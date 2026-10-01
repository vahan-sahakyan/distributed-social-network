package service

import (
	"context"
	"time"

	"github.com/vahan-sahakyan/distributed-social-network/pkg/events"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/id"
	"github.com/vahan-sahakyan/distributed-social-network/posts-service/internal/model"
	"github.com/vahan-sahakyan/distributed-social-network/posts-service/internal/repository"
)

type Service struct {
	repo *repository.Repository
}

func New(repo *repository.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreatePost(ctx context.Context, req *model.CreatePostRequest) (*model.Post, error) {
	post := &model.Post{
		ID:        id.New(),
		Text:      req.Text,
		AuthorID:  req.AuthorID,
		ImageID:   req.ImageID,
		CreatedAt: time.Now().UTC(),
	}

	if err := s.repo.CreateWithEvent(ctx, post, events.PostCreated); err != nil {
		return nil, err
	}
	return post, nil
}

func (s *Service) GetPost(ctx context.Context, postID string) (*model.Post, error) {
	return s.repo.GetByID(ctx, postID)
}
