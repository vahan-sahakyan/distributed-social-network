package service

import (
	"context"
	"time"

	"github.com/vahan-sahakyan/distributed-social-network/notification-service/internal/model"
	"github.com/vahan-sahakyan/distributed-social-network/notification-service/internal/repository"
)

type Service struct {
	repo *repository.Repository
}

func New(repo *repository.Repository) *Service {
	return &Service{repo: repo}
}

// CreateNotification stores the notification unless one with the same id exists.
func (s *Service) CreateNotification(ctx context.Context, notificationID, userID, notifType, actorID, entityID string) error {
	n := &model.Notification{
		ID:        notificationID,
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
