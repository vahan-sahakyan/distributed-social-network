package service

import (
	"context"
	"fmt"
	"time"

	"github.com/vahan-sahakyan/distributed-social-network/pkg/events"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/id"
	"github.com/vahan-sahakyan/distributed-social-network/posts-service/internal/model"
)

// Store keeps posts and their outbox events; *repository.Repository in production.
type Store interface {
	CreateWithEvent(ctx context.Context, post *model.Post, topic string) error
	DeleteWithEvent(ctx context.Context, post *model.Post, topic string) error
	GetByID(ctx context.Context, id string) (*model.Post, error)
	ListByAuthor(ctx context.Context, authorID string) ([]model.Post, error)
}

type Service struct {
	repo Store
}

func New(repo Store) *Service {
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

// DeletePost removes one of requesterID's posts. Its post.deleted event carries the whole
// post, so every read model (feeds, search, event store, notifications) can drop it.
func (s *Service) DeletePost(ctx context.Context, postID, requesterID string) error {
	post, err := s.repo.GetByID(ctx, postID)
	if err != nil {
		return err
	}
	if post.AuthorID != requesterID {
		return model.ErrNotAuthor
	}
	return s.repo.DeleteWithEvent(ctx, post, events.PostDeleted)
}

// DeleteUserPosts removes every post of authorID's, each with its own post.deleted, and
// returns how many went. Safe to retry after a failure: the deleted ones are no longer listed.
func (s *Service) DeleteUserPosts(ctx context.Context, authorID string) (int, error) {
	posts, err := s.repo.ListByAuthor(ctx, authorID)
	if err != nil {
		return 0, fmt.Errorf("listing posts of %s: %w", authorID, err)
	}
	for i := range posts {
		if err := s.repo.DeleteWithEvent(ctx, &posts[i], events.PostDeleted); err != nil {
			return i, fmt.Errorf("deleting post %s: %w", posts[i].ID, err)
		}
	}
	return len(posts), nil
}
