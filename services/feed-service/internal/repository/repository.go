package repository

import (
	"encoding/json"
	"errors"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/vahan-sahakyan/distributed-social-network/feed-service/internal/model"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/cache"
)

// casAttempts bounds the compare-and-swap retry loop. Feeds are updated by four
// concurrent consumer goroutines, so a plain get/modify/set loses whichever
// delta lands second; CAS turns that race into a retry.
const casAttempts = 5

type Repository struct {
	mc *memcache.Client
}

func New(mc *memcache.Client) *Repository {
	return &Repository{mc: mc}
}

// HomeFeedKey is the cache key of the posts by userID and everyone they follow.
func HomeFeedKey(userID string) string {
	return cache.HomeFeedKey(userID)
}

// UserPostsKey is the cache key of the posts authored by userID.
func UserPostsKey(userID string) string {
	return cache.UserPostsKey(userID)
}

func (r *Repository) GetFeed(key string) ([]model.FeedItem, error) {
	items, _, err := r.getFeedItem(key)
	return items, err
}

// getFeedItem returns the decoded feed and the raw memcache item, which carries
// the CAS token needed to write it back safely. A cache miss yields a nil item.
func (r *Repository) getFeedItem(key string) ([]model.FeedItem, *memcache.Item, error) {
	item, err := r.mc.Get(key)
	if errors.Is(err, memcache.ErrCacheMiss) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}

	var items []model.FeedItem
	if err := json.Unmarshal(item.Value, &items); err != nil {
		return nil, nil, err
	}
	return items, item, nil
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
	return r.update(key, func(items []model.FeedItem) ([]model.FeedItem, bool) {
		items = append([]model.FeedItem{*item}, items...)
		// Keep max 100 items in feed cache
		if len(items) > 100 {
			items = items[:100]
		}
		return items, true
	})
}

// AdjustCounts applies the like and comment deltas to a post in the feed stored under key.
func (r *Repository) AdjustCounts(key, postID string, likesDelta, commentsDelta int) error {
	return r.update(key, func(items []model.FeedItem) ([]model.FeedItem, bool) {
		for i := range items {
			if items[i].PostID != postID {
				continue
			}
			items[i].LikesCount = max(items[i].LikesCount+likesDelta, 0)
			items[i].CommentsCount = max(items[i].CommentsCount+commentsDelta, 0)
			return items, true
		}
		// Post is not in this feed - nothing to write.
		return items, false
	})
}

// update applies mutate to the feed at key and writes it back atomically, using
// CompareAndSwap so a concurrent writer cannot have its delta overwritten. It
// retries on contention and gives up after casAttempts.
func (r *Repository) update(key string, mutate func([]model.FeedItem) ([]model.FeedItem, bool)) error {
	var lastErr error

	for attempt := 0; attempt < casAttempts; attempt++ {
		items, raw, err := r.getFeedItem(key)
		if err != nil {
			return err
		}

		updated, changed := mutate(items)
		if !changed {
			return nil
		}

		data, err := json.Marshal(updated)
		if err != nil {
			return err
		}

		if raw == nil {
			// No entry yet: Add fails if another writer created one meanwhile.
			err = r.mc.Add(&memcache.Item{Key: key, Value: data, Expiration: 3600})
			if err == nil {
				return nil
			}
			if !errors.Is(err, memcache.ErrNotStored) {
				return err
			}
		} else {
			raw.Value = data
			raw.Expiration = 3600
			err = r.mc.CompareAndSwap(raw)
			if err == nil {
				return nil
			}
			if !errors.Is(err, memcache.ErrCASConflict) && !errors.Is(err, memcache.ErrNotStored) {
				return err
			}
		}
		lastErr = err
	}

	return lastErr
}
