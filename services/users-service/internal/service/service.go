package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/vahan-sahakyan/distributed-social-network/pkg/broker"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/events"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/id"
	"github.com/vahan-sahakyan/distributed-social-network/users-service/internal/model"
	"github.com/vahan-sahakyan/distributed-social-network/users-service/internal/repository"
)

type Service struct {
	repo     *repository.Repository
	producer *broker.Producer
}

func New(repo *repository.Repository, producer *broker.Producer) *Service {
	return &Service{repo: repo, producer: producer}
}

func (s *Service) CreateUser(ctx context.Context, req *model.CreateUserRequest) (*model.User, error) {
	user := &model.User{
		ID:        id.New(),
		Username:  req.Username,
		Bio:       req.Bio,
		CreatedAt: time.Now().UTC(),
	}

	if err := s.repo.Create(ctx, user); err != nil {
		return nil, err
	}

	if err := s.producer.Publish(ctx, events.UserCreated, user.ID, user); err != nil {
		slog.ErrorContext(ctx, "publishing event", "topic", events.UserCreated, "user_id", user.ID, "error", err)
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
