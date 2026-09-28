package service

import (
	"context"
	"log"

	"github.com/vahan-sahakyan/distributed-social-network/likes-service/internal/model"
	"github.com/vahan-sahakyan/distributed-social-network/likes-service/internal/repository"
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

func (s *Service) HasLiked(ctx context.Context, userID, entityID string) (bool, error) {
	return s.repo.HasLiked(ctx, userID, entityID)
}

func (s *Service) Unlike(ctx context.Context, userID, entityID string) error {
	deleted, err := s.repo.Delete(ctx, userID, entityID)
	if err != nil || !deleted {
		return err
	}

	s.publish(ctx, events.LikeDeleted, &model.Like{UserID: userID, EntityID: entityID})

	return nil
}

func (s *Service) CreateLike(ctx context.Context, req *model.CreateLikeRequest) (*model.Like, error) {
	like := &model.Like{
		ID:       id.New(),
		UserID:   req.UserID,
		EntityID: req.EntityID,
	}

	created, err := s.repo.Create(ctx, like)
	if err != nil {
		return nil, err
	}

	// a repeated like is a no-op, so it must not emit another event
	if created {
		s.publish(ctx, events.LikeCreated, like)
	}

	return like, nil
}

func (s *Service) publish(ctx context.Context, topic string, like *model.Like) {
	if err := s.producer.Publish(ctx, topic, like.EntityID, like); err != nil {
		log.Printf("failed to publish %s for %s: %v", topic, like.EntityID, err)
	}
}
