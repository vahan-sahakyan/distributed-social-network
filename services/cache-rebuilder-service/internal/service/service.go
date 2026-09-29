package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/vahan-sahakyan/distributed-social-network/cache-rebuilder-service/internal/model"
	"github.com/vahan-sahakyan/distributed-social-network/cache-rebuilder-service/internal/repository"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/cache"
	postspb "github.com/vahan-sahakyan/distributed-social-network/pkg/grpc/posts"
	userspb "github.com/vahan-sahakyan/distributed-social-network/pkg/grpc/users"
)

type Service struct {
	repo        *repository.Repository
	mc          *memcache.Client
	usersClient userspb.UsersServiceClient
	postsClient postspb.PostsServiceClient
}

func New(repo *repository.Repository, mc *memcache.Client, usersClient userspb.UsersServiceClient, postsClient postspb.PostsServiceClient) *Service {
	return &Service{repo: repo, mc: mc, usersClient: usersClient, postsClient: postsClient}
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
		slog.WarnContext(ctx, "loading post states from clickhouse", "error", err)
		return map[string]model.PostState{}
	}
	m := make(map[string]model.PostState, len(states))
	for _, st := range states {
		m[st.PostID] = st
	}
	return m
}

func (s *Service) RebuildCache(ctx context.Context) error {
	slog.InfoContext(ctx, "starting cache rebuild")

	events, err := s.repo.GetRecentPostEvents(ctx, 1000)
	if err != nil {
		return fmt.Errorf("failed to get post events: %w", err)
	}

	postStates := s.loadPostStates(ctx)

	// Build feed map: cache key -> []feedItem
	feeds := map[string][]feedItem{}

	for _, event := range events {
		post, err := s.fetchPost(ctx, event.PostID)
		if err != nil {
			slog.WarnContext(ctx, "skipping post", "post_id", event.PostID, "error", err)
			continue
		}

		item := newFeedItem(post, postStates[event.PostID])

		// Write to author's own feed and posts
		feeds[cache.HomeFeedKey(event.UserID)] = append(feeds[cache.HomeFeedKey(event.UserID)], item)
		feeds[cache.UserPostsKey(event.UserID)] = append(feeds[cache.UserPostsKey(event.UserID)], item)

		// Write to each follower's feed
		followers := s.fetchFollowers(ctx, event.UserID)
		for _, followerID := range followers {
			feeds[cache.HomeFeedKey(followerID)] = append(feeds[cache.HomeFeedKey(followerID)], item)
		}
	}

	// Write all feeds to Memcached
	for key, items := range feeds {
		if err := s.setFeed(key, items); err != nil {
			slog.ErrorContext(ctx, "writing feed", "key", key, "error", err)
		}
	}

	slog.InfoContext(ctx, "cache rebuild complete", "feeds", len(feeds), "events", len(events))
	return nil
}

// RebuildUserFeed rebuilds the feed for a single user based on who they currently follow.
func (s *Service) RebuildUserFeed(ctx context.Context, userID string) error {
	slog.InfoContext(ctx, "rebuilding user feed", "user_id", userID)

	following := s.fetchFollowing(ctx, userID)
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

	// feed-service serves two caches per user: the home feed and the profile's
	// own posts. Rebuilding only the first left profiles empty once the second
	// expired, with nothing able to repopulate it short of a full rebuild.
	homeItems := []feedItem{}
	ownItems := []feedItem{}
	for _, event := range events {
		if !authorSet[event.UserID] {
			continue
		}
		post, err := s.fetchPost(ctx, event.PostID)
		if err != nil {
			continue
		}
		item := newFeedItem(post, postStates[event.PostID])
		homeItems = append(homeItems, item)
		if event.UserID == userID {
			ownItems = append(ownItems, item)
		}
	}

	if err := s.setFeed(cache.HomeFeedKey(userID), homeItems); err != nil {
		return err
	}
	if err := s.setFeed(cache.UserPostsKey(userID), ownItems); err != nil {
		return err
	}
	slog.InfoContext(ctx, "user feed rebuilt", "user_id", userID, "feed_posts", len(homeItems), "own_posts", len(ownItems))
	return nil
}

func (s *Service) setFeed(key string, items []feedItem) error {
	data, err := json.Marshal(items)
	if err != nil {
		return err
	}
	return s.mc.Set(&memcache.Item{Key: key, Value: data, Expiration: 3600})
}

// newFeedItem builds a cached feed entry. Counts are clamped at zero: summed
// deltas go negative when an unlike outlives its like (e.g. after feed_events
// was truncated), and the UI would otherwise render "-1 likes".
func newFeedItem(post *postResponse, st model.PostState) feedItem {
	return feedItem{
		PostID:        post.ID,
		AuthorID:      post.AuthorID,
		Text:          post.Text,
		ImageURL:      post.ImageID,
		LikesCount:    max(int(st.Likes), 0),
		CommentsCount: max(int(st.Comments), 0),
		CreatedAt:     post.CreatedAt,
	}
}

type postResponse struct {
	ID        string    `json:"id"`
	Text      string    `json:"text"`
	AuthorID  string    `json:"author_id"`
	ImageID   string    `json:"image_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Service) fetchPost(ctx context.Context, postID string) (*postResponse, error) {
	resp, err := s.postsClient.GetPost(ctx, &postspb.GetPostRequest{Id: postID})
	if err != nil {
		return nil, err
	}
	if resp.Post == nil {
		return nil, fmt.Errorf("post not found: %s", postID)
	}
	p := resp.Post
	post := &postResponse{
		ID:       p.Id,
		Text:     p.Text,
		AuthorID: p.AuthorId,
		ImageID:  p.ImageId,
	}
	if p.CreatedAt != nil {
		post.CreatedAt = p.CreatedAt.AsTime()
	}
	return post, nil
}

func (s *Service) fetchFollowers(ctx context.Context, userID string) []string {
	resp, err := s.usersClient.GetFollowers(ctx, &userspb.GetFollowersRequest{UserId: userID})
	if err != nil {
		slog.WarnContext(ctx, "fetching followers", "user_id", userID, "error", err)
		return nil
	}
	return resp.Followers
}

func (s *Service) fetchFollowing(ctx context.Context, userID string) []string {
	resp, err := s.usersClient.GetFollowing(ctx, &userspb.GetFollowingRequest{UserId: userID})
	if err != nil {
		slog.WarnContext(ctx, "fetching following", "user_id", userID, "error", err)
		return nil
	}
	return resp.Following
}
