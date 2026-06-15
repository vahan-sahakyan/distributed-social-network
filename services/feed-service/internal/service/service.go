package service

import (
	"github.com/vahan-sahakyan/distributed-social-network/feed-service/internal/model"
	"github.com/vahan-sahakyan/distributed-social-network/feed-service/internal/repository"
)

type Service struct {
	repo *repository.Repository
}

func New(repo *repository.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetHomeFeed(userID string) ([]model.FeedItem, error) {
	return s.repo.GetFeed(userID)
}

func (s *Service) GetUserFeed(userID string) ([]model.FeedItem, error) {
	return s.repo.GetFeed(userID)
}

// FanoutPost distributes a new post to follower feed caches.
func (s *Service) FanoutPost(item *model.FeedItem, followerIDs []string) error {
	for _, followerID := range followerIDs {
		if err := s.repo.AppendToFeed(followerID, item); err != nil {
			return err
		}
	}
	return nil
}
