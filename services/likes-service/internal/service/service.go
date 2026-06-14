package service

import (
	"context"

	"github.com/vahan/distributed-social-network/likes-service/internal/model"
	"github.com/vahan/distributed-social-network/likes-service/internal/repository"
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

func (s *Service) CreateLike(ctx context.Context, req *model.CreateLikeRequest) (*model.Like, error) {
	like := &model.Like{
		ID:       id.New(),
		UserID:   req.UserID,
		EntityID: req.EntityID,
	}

	if err := s.repo.Create(ctx, like); err != nil {
		return nil, err
	}

	_ = s.producer.Publish(ctx, "like.created", like)

	return like, nil
}
