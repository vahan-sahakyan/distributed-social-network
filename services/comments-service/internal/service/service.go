package service

import (
	"context"
	"time"

	"github.com/vahan-sahakyan/distributed-social-network/comments-service/internal/model"
	"github.com/vahan-sahakyan/distributed-social-network/comments-service/internal/repository"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/events"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/id"
)

type Service struct {
	repo *repository.Repository
}

func New(repo *repository.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateComment(ctx context.Context, req *model.CreateCommentRequest) (*model.Comment, error) {
	comment := &model.Comment{
		ID:        id.New(),
		UserID:    req.UserID,
		EntityID:  req.EntityID,
		Text:      req.Text,
		CreatedAt: time.Now().UTC(),
	}

	err := s.repo.Tx(ctx, func(tx *repository.Repository) error {
		if err := tx.Create(ctx, comment); err != nil {
			return err
		}
		return tx.Enqueue(ctx, events.CommentCreated, comment.EntityID, comment)
	})
	if err != nil {
		return nil, err
	}
	return comment, nil
}

func (s *Service) GetCommentsByEntity(ctx context.Context, entityID string) ([]model.Comment, error) {
	return s.repo.GetByEntityID(ctx, entityID)
}
