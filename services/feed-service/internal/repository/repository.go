package repository

import (
	"encoding/json"
	"fmt"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/vahan-sahakyan/distributed-social-network/feed-service/internal/model"
)

type Repository struct {
	mc *memcache.Client
}

func New(mc *memcache.Client) *Repository {
	return &Repository{mc: mc}
}

// HomeFeedKey is the cache key of the posts by userID and everyone they follow.
func HomeFeedKey(userID string) string {
	return fmt.Sprintf("feed:%s", userID)
}

// UserPostsKey is the cache key of the posts authored by userID.
func UserPostsKey(userID string) string {
	return fmt.Sprintf("userposts:%s", userID)
}

func (r *Repository) GetFeed(key string) ([]model.FeedItem, error) {
	item, err := r.mc.Get(key)
	if err == memcache.ErrCacheMiss {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var items []model.FeedItem
	if err := json.Unmarshal(item.Value, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *Repository) SetFeed(key string, items []model.FeedItem) error {
	data, err := json.Marshal(items)
	if err != nil {
		return err
	}
	return r.mc.Set(&memcache.Item{
		Key:        key,
		Value:      data,
		Expiration: 3600, // 1 hour TTL
	})
}

func (r *Repository) AppendToFeed(key string, item *model.FeedItem) error {
	existing, err := r.GetFeed(key)
	if err != nil {
		return err
	}
	items := append([]model.FeedItem{*item}, existing...)
	// Keep max 100 items in feed cache
	if len(items) > 100 {
		items = items[:100]
	}
	return r.SetFeed(key, items)
}

// AdjustCounts applies the like and comment deltas to a post in the feed stored under key.
func (r *Repository) AdjustCounts(key, postID string, likesDelta, commentsDelta int) {
	items, err := r.GetFeed(key)
	if err != nil || len(items) == 0 {
		return
	}
	for i, item := range items {
		if item.PostID == postID {
			items[i].LikesCount = max(items[i].LikesCount+likesDelta, 0)
			items[i].CommentsCount = max(items[i].CommentsCount+commentsDelta, 0)
			r.SetFeed(key, items) //nolint
			return
		}
	}
}
