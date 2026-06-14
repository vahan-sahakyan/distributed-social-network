package service

import (
	"context"
	"time"

	"github.com/vahan/distributed-social-network/notification-service/internal/model"
	"github.com/vahan/distributed-social-network/notification-service/internal/repository"
	"github.com/vahan/distributed-social-network/pkg/id"
)

type Service struct {
	repo *repository.Repository
}

func New(repo *repository.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateNotification(ctx context.Context, userID, notifType, actorID, entityID string) error {
	n := &model.Notification{
		ID:        id.New(),
		UserID:    userID,
		Type:      notifType,
		ActorID:   actorID,
		EntityID:  entityID,
		Read:      false,
		CreatedAt: time.Now().UTC(),
	}
	return s.repo.Create(ctx, n)
}

func (s *Service) GetByUserID(ctx context.Context, userID string) ([]model.Notification, error) {
	return s.repo.GetByUserID(ctx, userID)
}
