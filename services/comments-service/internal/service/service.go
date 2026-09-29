package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/vahan-sahakyan/distributed-social-network/comments-service/internal/model"
	"github.com/vahan-sahakyan/distributed-social-network/comments-service/internal/repository"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/broker"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/events"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/id"
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

	if err := s.producer.Publish(ctx, events.CommentCreated, comment.EntityID, comment); err != nil {
		slog.ErrorContext(ctx, "publishing event", "topic", events.CommentCreated, "comment_id", comment.ID, "error", err)
	}

	return comment, nil
}

func (s *Service) GetCommentsByEntity(ctx context.Context, entityID string) ([]model.Comment, error) {
	return s.repo.GetByEntityID(ctx, entityID)
}
