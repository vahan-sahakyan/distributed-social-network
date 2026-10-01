package service

import (
	"context"
	"time"

	"github.com/vahan-sahakyan/distributed-social-network/pkg/events"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/id"
	"github.com/vahan-sahakyan/distributed-social-network/users-service/internal/model"
	"github.com/vahan-sahakyan/distributed-social-network/users-service/internal/repository"
)

type Service struct {
	repo *repository.Repository
}

func New(repo *repository.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateUser(ctx context.Context, req *model.CreateUserRequest) (*model.User, error) {
	user := &model.User{
		ID:        id.New(),
		Username:  req.Username,
		Bio:       req.Bio,
		CreatedAt: time.Now().UTC(),
	}

	err := s.repo.Tx(ctx, func(tx *repository.Repository) error {
		if err := tx.Create(ctx, user); err != nil {
			return err
		}
		return tx.Enqueue(ctx, events.UserCreated, user.ID, user)
	})
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (s *Service) GetUser(ctx context.Context, userID string) (*model.User, error) {
	return s.repo.GetByID(ctx, userID)
}

func (s *Service) GetUserByUsername(ctx context.Context, username string) (*model.User, error) {
	return s.repo.GetByUsername(ctx, username)
}

func (s *Service) Follow(ctx context.Context, followerID, followeeID string) error {
	return s.repo.Follow(ctx, followerID, followeeID)
}

func (s *Service) Unfollow(ctx context.Context, followerID, followeeID string) error {
	return s.repo.Unfollow(ctx, followerID, followeeID)
}

func (s *Service) GetFollowers(ctx context.Context, userID string) ([]string, error) {
	return s.repo.GetFollowers(ctx, userID)
}

func (s *Service) GetFollowing(ctx context.Context, userID string) ([]string, error) {
	return s.repo.GetFollowing(ctx, userID)
}
