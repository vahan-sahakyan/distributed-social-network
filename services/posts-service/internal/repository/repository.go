package repository

import (
	"context"
	"errors"

	"github.com/gocql/gocql"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/outbox"
	"github.com/vahan-sahakyan/distributed-social-network/posts-service/internal/model"
)

type Repository struct {
	db *gocql.Session
}

func New(db *gocql.Session) *Repository {
	return &Repository{db: db}
}

// CreateWithEvent writes the post and its event in one LOGGED batch: both apply or neither.
func (r *Repository) CreateWithEvent(ctx context.Context, post *model.Post, topic string) error {
	b := r.db.NewBatch(gocql.LoggedBatch).WithContext(ctx)
	b.Query(`INSERT INTO posts (id, text, author_id, image_id, created_at) VALUES (?, ?, ?, ?, ?)`,
		post.ID, post.Text, post.AuthorID, post.ImageID, post.CreatedAt)
	if err := outbox.AddToBatch(ctx, b, topic, post.ID, post); err != nil {
		return err
	}
	return r.db.ExecuteBatch(b)
}

func (r *Repository) GetByID(ctx context.Context, id string) (*model.Post, error) {
	var post model.Post
	err := r.db.Query(
		`SELECT id, text, author_id, image_id, created_at FROM posts WHERE id = ?`, id,
	).WithContext(ctx).Scan(&post.ID, &post.Text, &post.AuthorID, &post.ImageID, &post.CreatedAt)
	if errors.Is(err, gocql.ErrNotFound) {
		return nil, model.ErrPostNotFound
	}
	if err != nil {
		return nil, err
	}
	return &post, nil
}
