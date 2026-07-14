package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/vahan-sahakyan/distributed-social-network/cache-rebuilder-service/internal/model"
	"github.com/vahan-sahakyan/distributed-social-network/cache-rebuilder-service/internal/repository"
)

type Service struct {
	repo     *repository.Repository
	mc       *memcache.Client
	usersURL string
	postsURL string
}

func New(repo *repository.Repository, mc *memcache.Client, usersURL, postsURL string) *Service {
	return &Service{repo: repo, mc: mc, usersURL: usersURL, postsURL: postsURL}
}

type feedItem struct {
	PostID        string    `json:"post_id"`
	AuthorID      string    `json:"author_id"`
	Text          string    `json:"text"`
	ImageURL      string    `json:"image_url"`
	LikesCount    int       `json:"likes_count"`
	CommentsCount int       `json:"comments_count"`
	CreatedAt     time.Time `json:"created_at"`
}

func (s *Service) loadPostStates(ctx context.Context) map[string]model.PostState {
	states, err := s.repo.GetPostStates(ctx)
	if err != nil {
		log.Printf("warning: could not load post states from ClickHouse: %v", err)
		return map[string]model.PostState{}
	}
	m := make(map[string]model.PostState, len(states))
	for _, st := range states {
		m[st.PostID] = st
	}
	return m
}

func (s *Service) RebuildCache(ctx context.Context) error {
	log.Println("starting cache rebuild...")

	events, err := s.repo.GetRecentPostEvents(ctx, 1000)
	if err != nil {
		return fmt.Errorf("failed to get post events: %w", err)
	}

	postStates := s.loadPostStates(ctx)

	// Build feed map: userID -> []feedItem
	feeds := map[string][]feedItem{}

	for _, event := range events {
		post, err := s.fetchPost(event.PostID)
		if err != nil {
			log.Printf("skipping post %s: %v", event.PostID, err)
			continue
		}

		st := postStates[event.PostID]
		item := feedItem{
			PostID:        post.ID,
			AuthorID:      post.AuthorID,
			Text:          post.Text,
			ImageURL:      post.ImageID,
			LikesCount:    int(st.Likes),
			CommentsCount: int(st.Comments),
			CreatedAt:     post.CreatedAt,
		}

		// Write to author's own feed
		feeds[event.UserID] = append(feeds[event.UserID], item)

		// Write to each follower's feed
		followers := s.fetchFollowers(event.UserID)
		for _, followerID := range followers {
			feeds[followerID] = append(feeds[followerID], item)
		}
	}

	// Write all feeds to Memcached
	for userID, items := range feeds {
		key := fmt.Sprintf("feed:%s", userID)
		data, err := json.Marshal(items)
		if err != nil {
			continue
		}
		s.mc.Set(&memcache.Item{Key: key, Value: data, Expiration: 3600})
	}

	log.Printf("cache rebuild complete: %d users' feeds populated from %d events", len(feeds), len(events))
	return nil
}

// RebuildUserFeed rebuilds the feed for a single user based on who they currently follow.
func (s *Service) RebuildUserFeed(ctx context.Context, userID string) error {
	log.Printf("rebuilding feed for user %s...", userID)

	following := s.fetchFollowing(userID)
	// include the user's own posts too
	authors := append(following, userID)

	events, err := s.repo.GetRecentPostEvents(ctx, 1000)
	if err != nil {
		return fmt.Errorf("failed to get events: %w", err)
	}

	authorSet := map[string]bool{}
	for _, a := range authors {
		authorSet[a] = true
	}

	postStates := s.loadPostStates(ctx)

	var items []feedItem
	for _, event := range events {
		if !authorSet[event.UserID] {
			continue
		}
		post, err := s.fetchPost(event.PostID)
		if err != nil {
			continue
		}
		st := postStates[event.PostID]
		items = append(items, feedItem{
			PostID:        post.ID,
			AuthorID:      post.AuthorID,
			Text:          post.Text,
			ImageURL:      post.ImageID,
			LikesCount:    int(st.Likes),
			CommentsCount: int(st.Comments),
			CreatedAt:     post.CreatedAt,
		})
	}

	key := fmt.Sprintf("feed:%s", userID)
	data, err := json.Marshal(items)
	if err != nil {
		return err
	}
	s.mc.Set(&memcache.Item{Key: key, Value: data, Expiration: 3600})
	log.Printf("user feed rebuilt: %d posts for %s", len(items), userID)
	return nil
}

type postResponse struct {
	ID        string    `json:"id"`
	Text      string    `json:"text"`
	AuthorID  string    `json:"author_id"`
	ImageID   string    `json:"image_id"`
	Likes     int       `json:"likes"`
	Comments  int       `json:"comments"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Service) fetchPost(postID string) (*postResponse, error) {
	resp, err := http.Get(s.postsURL + "/api/v1/posts/" + postID)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("posts-service returned %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	var post postResponse
	if err := json.Unmarshal(body, &post); err != nil {
		return nil, err
	}
	return &post, nil
}

func (s *Service) fetchFollowers(userID string) []string {
	resp, err := http.Get(s.usersURL + "/api/v1/users/" + userID + "/followers")
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Followers []string `json:"followers"`
	}
	json.Unmarshal(body, &result)
	return result.Followers
}

func (s *Service) fetchFollowing(userID string) []string {
	resp, err := http.Get(s.usersURL + "/api/v1/users/" + userID + "/following")
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Following []string `json:"following"`
	}
	json.Unmarshal(body, &result)
	return result.Following
}
