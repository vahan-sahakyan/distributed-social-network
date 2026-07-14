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

func (r *Repository) GetFeed(userID string) ([]model.FeedItem, error) {
	key := fmt.Sprintf("feed:%s", userID)
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

func (r *Repository) SetFeed(userID string, items []model.FeedItem) error {
	key := fmt.Sprintf("feed:%s", userID)
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

func (r *Repository) AppendToFeed(userID string, item *model.FeedItem) error {
	existing, err := r.GetFeed(userID)
	if err != nil {
		return err
	}
	items := append([]model.FeedItem{*item}, existing...)
	// Keep max 100 items in feed cache
	if len(items) > 100 {
		items = items[:100]
	}
	return r.SetFeed(userID, items)
}

// IncrementCount increments the like (isLike=true) or comment count for a post in a user's feed.
func (r *Repository) IncrementCount(userID, postID string, isLike bool) {
	items, err := r.GetFeed(userID)
	if err != nil || len(items) == 0 {
		return
	}
	for i, item := range items {
		if item.PostID == postID {
			if isLike {
				items[i].LikesCount++
			} else {
				items[i].CommentsCount++
			}
			r.SetFeed(userID, items) //nolint
			return
		}
	}
}
