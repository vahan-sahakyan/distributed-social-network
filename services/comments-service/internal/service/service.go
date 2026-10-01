package service

import (
	"context"
	"fmt"
	"time"

	"github.com/vahan-sahakyan/distributed-social-network/comments-service/internal/model"
	"github.com/vahan-sahakyan/distributed-social-network/comments-service/internal/repository"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/events"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/id"
)

type Service struct {
	repo  *repository.Repository
	posts PostChecker
}

// PostChecker reports whether a post exists; *posts.Checker in production.
type PostChecker interface {
	Exists(ctx context.Context, id string) (bool, error)
}

func New(repo *repository.Repository, posts PostChecker) *Service {
	return &Service{repo: repo, posts: posts}
}

func (s *Service) CreateComment(ctx context.Context, req *model.CreateCommentRequest) (*model.Comment, error) {
	if exists, err := s.posts.Exists(ctx, req.EntityID); err != nil {
		return nil, fmt.Errorf("checking post %s: %w", req.EntityID, err)
	} else if !exists {
		return nil, model.ErrPostNotFound
	}

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
