package service

import (
	"context"

	"github.com/redis/go-redis/v9"
	"github.com/vahan/distributed-social-network/feed-service/internal/model"
	"github.com/vahan/distributed-social-network/feed-service/internal/repository"
)

const defaultFeedLimit = 50

type Service struct {
	repo  *repository.Repository
	redis *redis.Client
}

func New(repo *repository.Repository, redis *redis.Client) *Service {
	return &Service{repo: repo, redis: redis}
}

func (s *Service) GetHomeFeed(ctx context.Context, userID string) ([]model.FeedItem, error) {
	return s.repo.GetHomeFeed(ctx, userID, defaultFeedLimit)
}

func (s *Service) GetUserFeed(ctx context.Context, userID string) ([]model.FeedItem, error) {
	return s.repo.GetUserFeed(ctx, userID, defaultFeedLimit)
}

// FanoutPost distributes a new post to all follower feeds.
func (s *Service) FanoutPost(ctx context.Context, item *model.FeedItem, followerIDs []string) error {
	for _, followerID := range followerIDs {
		feedItem := *item
		feedItem.UserID = followerID
		if err := s.repo.InsertFeedItem(ctx, &feedItem); err != nil {
			return err
		}
	}
	return nil
}
