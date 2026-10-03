package repository

import (
	"context"

	"github.com/vahan-sahakyan/distributed-social-network/feed-service/internal/model"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/cache"
	"github.com/valkey-io/valkey-go"
)

// Repository keeps feeds in Valkey through the pkg/cache helpers, which
// cache-rebuilder writes with too.
type Repository struct {
	c valkey.Client
}

func New(c valkey.Client) *Repository {
	return &Repository{c: c}
}

// HomeFeedKey is the cache key of the posts by userID and everyone they follow.
func HomeFeedKey(userID string) string {
	return cache.HomeFeedKey(userID)
}

// UserPostsKey is the cache key of the posts authored by userID.
func UserPostsKey(userID string) string {
	return cache.UserPostsKey(userID)
}

func (r *Repository) GetFeed(ctx context.Context, key string) ([]model.FeedItem, error) {
	posts, err := cache.ReadFeed(ctx, r.c, key)
	if err != nil {
		return nil, err
	}
	items := make([]model.FeedItem, len(posts))
	for i, p := range posts {
		items[i] = model.FeedItem{
			PostID:        p.ID,
			AuthorID:      p.AuthorID,
			Text:          p.Text,
			LikesCount:    p.LikesCount,
			CommentsCount: p.CommentsCount,
			ImageURL:      p.ImageURL,
			CreatedAt:     p.CreatedAt,
		}
	}
	return items, nil
}

// AddPost caches item and puts it at the head of each feed.
func (r *Repository) AddPost(ctx context.Context, item *model.FeedItem, feedKeys ...string) error {
	return cache.AddPost(ctx, r.c, cache.Post{
		ID:            item.PostID,
		AuthorID:      item.AuthorID,
		Text:          item.Text,
		ImageURL:      item.ImageURL,
		CreatedAt:     item.CreatedAt,
		LikesCount:    item.LikesCount,
		CommentsCount: item.CommentsCount,
	}, feedKeys...)
}

// AdjustCounts applies like and comment deltas to a cached post, once for every feed showing it.
func (r *Repository) AdjustCounts(ctx context.Context, postID string, likesDelta, commentsDelta int) error {
	return cache.AdjustCounts(ctx, r.c, postID, likesDelta, commentsDelta)
}

func (r *Repository) Flush(ctx context.Context) error {
	return cache.Flush(ctx, r.c)
}
