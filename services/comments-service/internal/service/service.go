package service

import (
	"context"
	"time"

	"github.com/vahan/distributed-social-network/comments-service/internal/model"
	"github.com/vahan/distributed-social-network/comments-service/internal/repository"
	"github.com/vahan/distributed-social-network/pkg/broker"
	"github.com/vahan/distributed-social-network/pkg/id"
)

type Service struct {
	repo     *repository.Repository
	producer *broker.Producer
}

func New(repo *repository.Repository, producer *broker.Producer) *Service {
	return &Service{repo: repo, producer: producer}
}

func (s *Service) CreateComment(ctx context.Context, req *model.CreateCommentRequest) (*model.Comment, error) {
	comment := &model.Comment{
		ID:        id.New(),
		UserID:    req.UserID,
		EntityID:  req.EntityID,
		Text:      req.Text,
		CreatedAt: time.Now().UTC(),
	}

	if err := s.repo.Create(ctx, comment); err != nil {
		return nil, err
	}

	_ = s.producer.Publish(ctx, "comment.created", comment)

	return comment, nil
}

func (s *Service) GetCommentsByEntity(ctx context.Context, entityID string) ([]model.Comment, error) {
	return s.repo.GetByEntityID(ctx, entityID)
}
