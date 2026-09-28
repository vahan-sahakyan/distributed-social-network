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
	return s.repo.GetFeed(repository.HomeFeedKey(userID))
}

func (s *Service) GetUserFeed(userID string) ([]model.FeedItem, error) {
	return s.repo.GetFeed(repository.UserPostsKey(userID))
}

// FanoutPost distributes a new post to the author's own posts and to follower feed caches.
func (s *Service) FanoutPost(item *model.FeedItem, followerIDs []string) error {
	if err := s.repo.AppendToFeed(repository.UserPostsKey(item.AuthorID), item); err != nil {
		return err
	}
	for _, followerID := range followerIDs {
		if err := s.repo.AppendToFeed(repository.HomeFeedKey(followerID), item); err != nil {
			return err
		}
	}
	return nil
}

// AdjustCounts applies like and comment deltas to a post in its author's posts and the given users' feed caches.
func (s *Service) AdjustCounts(postID, authorID string, userIDs []string, likesDelta, commentsDelta int) {
	s.repo.AdjustCounts(repository.UserPostsKey(authorID), postID, likesDelta, commentsDelta)
	for _, userID := range userIDs {
		s.repo.AdjustCounts(repository.HomeFeedKey(userID), postID, likesDelta, commentsDelta)
	}
}
