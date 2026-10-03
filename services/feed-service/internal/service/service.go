package service

import (
	"context"

	"github.com/vahan-sahakyan/distributed-social-network/feed-service/internal/model"
	"github.com/vahan-sahakyan/distributed-social-network/feed-service/internal/repository"
)

type Service struct {
	repo *repository.Repository
}

func New(repo *repository.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetHomeFeed(ctx context.Context, userID string) ([]model.FeedItem, error) {
	return s.repo.GetFeed(ctx, repository.HomeFeedKey(userID))
}

func (s *Service) GetUserFeed(ctx context.Context, userID string) ([]model.FeedItem, error) {
	return s.repo.GetFeed(ctx, repository.UserPostsKey(userID))
}

// FanoutPost puts a new post in its author's posts and in the given users' home
// feeds, in one pipeline. Redelivery is safe: each feed holds a post once.
func (s *Service) FanoutPost(ctx context.Context, item *model.FeedItem, followerIDs []string) error {
	keys := []string{repository.UserPostsKey(item.AuthorID)}
	for _, id := range followerIDs {
		keys = append(keys, repository.HomeFeedKey(id))
	}
	return s.repo.AddPost(ctx, item, keys...)
}

// AdjustCounts applies like and comment deltas to a post; every feed showing it reads the same counts.
func (s *Service) AdjustCounts(ctx context.Context, postID string, likesDelta, commentsDelta int) error {
	return s.repo.AdjustCounts(ctx, postID, likesDelta, commentsDelta)
}

func (s *Service) Reset(ctx context.Context) error {
	return s.repo.Flush(ctx)
}
