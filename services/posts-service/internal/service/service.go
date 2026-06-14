package service

import (
	"context"
	"time"

	"github.com/vahan/distributed-social-network/pkg/broker"
	"github.com/vahan/distributed-social-network/pkg/id"
	"github.com/vahan/distributed-social-network/posts-service/internal/model"
	"github.com/vahan/distributed-social-network/posts-service/internal/repository"
)

type Service struct {
	repo     *repository.Repository
	producer *broker.Producer
}

func New(repo *repository.Repository, producer *broker.Producer) *Service {
	return &Service{repo: repo, producer: producer}
}

func (s *Service) CreatePost(ctx context.Context, req *model.CreatePostRequest) (*model.Post, error) {
	post := &model.Post{
		ID:        id.New(),
		Text:      req.Text,
		AuthorID:  req.AuthorID,
		ImageID:   req.ImageID,
		CreatedAt: time.Now().UTC(),
	}

	if err := s.repo.Create(ctx, post); err != nil {
		return nil, err
	}

	_ = s.producer.Publish(ctx, "post.created", post)

	return post, nil
}

func (s *Service) GetPost(ctx context.Context, postID string) (*model.Post, error) {
	return s.repo.GetByID(ctx, postID)
}
