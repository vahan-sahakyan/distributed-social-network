package service

import (
	"errors"
	"fmt"
	"log"

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

// FanoutPost distributes a new post to the author's own posts and to follower
// feed caches. One failing feed must not cancel the rest: the caller retries the
// whole message, so stopping early would leave every later follower unwritten.
func (s *Service) FanoutPost(item *model.FeedItem, followerIDs []string) error {
	var errs []error

	if err := s.repo.AppendToFeed(repository.UserPostsKey(item.AuthorID), item); err != nil {
		errs = append(errs, fmt.Errorf("author posts %s: %w", item.AuthorID, err))
	}

	for _, followerID := range followerIDs {
		if err := s.repo.AppendToFeed(repository.HomeFeedKey(followerID), item); err != nil {
			errs = append(errs, fmt.Errorf("follower feed %s: %w", followerID, err))
		}
	}

	return errors.Join(errs...)
}

// AdjustCounts applies like and comment deltas to a post in its author's posts and the given users' feed caches.
func (s *Service) AdjustCounts(postID, authorID string, userIDs []string, likesDelta, commentsDelta int) error {
	var errs []error

	if err := s.repo.AdjustCounts(repository.UserPostsKey(authorID), postID, likesDelta, commentsDelta); err != nil {
		errs = append(errs, fmt.Errorf("author posts %s: %w", authorID, err))
	}

	for _, userID := range userIDs {
		if err := s.repo.AdjustCounts(repository.HomeFeedKey(userID), postID, likesDelta, commentsDelta); err != nil {
			errs = append(errs, fmt.Errorf("home feed %s: %w", userID, err))
		}
	}

	if err := errors.Join(errs...); err != nil {
		log.Printf("adjusting counts for post %s: %v", postID, err)
		return err
	}
	return nil
}
