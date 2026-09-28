package service

import (
	"context"
	"log"
	"time"

	"github.com/vahan-sahakyan/distributed-social-network/pkg/broker"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/events"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/id"
	"github.com/vahan-sahakyan/distributed-social-network/posts-service/internal/model"
	"github.com/vahan-sahakyan/distributed-social-network/posts-service/internal/repository"
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

	if err := s.producer.Publish(ctx, events.PostCreated, post.ID, post); err != nil {
		log.Printf("failed to publish post.created for %s: %v", post.ID, err)
	}

	return post, nil
}

func (s *Service) GetPost(ctx context.Context, postID string) (*model.Post, error) {
	return s.repo.GetByID(ctx, postID)
}
