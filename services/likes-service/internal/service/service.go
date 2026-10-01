package service

import (
	"context"
	"fmt"

	"github.com/vahan-sahakyan/distributed-social-network/likes-service/internal/model"
	"github.com/vahan-sahakyan/distributed-social-network/likes-service/internal/repository"
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

func (s *Service) HasLiked(ctx context.Context, userID, entityID string) (bool, error) {
	return s.repo.HasLiked(ctx, userID, entityID)
}

func (s *Service) Unlike(ctx context.Context, userID, entityID string) error {
	return s.repo.Tx(ctx, func(tx *repository.Repository) error {
		deleted, err := tx.Delete(ctx, userID, entityID)
		if err != nil || !deleted {
			return err
		}
		return tx.Enqueue(ctx, events.LikeDeleted, entityID, &model.Like{UserID: userID, EntityID: entityID})
	})
}

func (s *Service) CreateLike(ctx context.Context, req *model.CreateLikeRequest) (*model.Like, error) {
	if exists, err := s.posts.Exists(ctx, req.EntityID); err != nil {
		return nil, fmt.Errorf("checking post %s: %w", req.EntityID, err)
	} else if !exists {
		return nil, model.ErrPostNotFound
	}

	like := &model.Like{
		ID:       id.New(),
		UserID:   req.UserID,
		EntityID: req.EntityID,
	}

	err := s.repo.Tx(ctx, func(tx *repository.Repository) error {
		created, err := tx.Create(ctx, like)
		// a repeated like is a no-op, so it must not emit another event
		if err != nil || !created {
			return err
		}
		return tx.Enqueue(ctx, events.LikeCreated, like.EntityID, like)
	})
	if err != nil {
		return nil, err
	}
	return like, nil
}
