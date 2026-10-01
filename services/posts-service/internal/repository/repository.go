package repository

import (
	"context"
	"errors"

	"github.com/gocql/gocql"
	"github.com/vahan-sahakyan/distributed-social-network/posts-service/internal/model"
)

type Repository struct {
	db *gocql.Session
}

func New(db *gocql.Session) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, post *model.Post) error {
	return r.db.Query(
		`INSERT INTO posts (id, text, author_id, image_id, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		post.ID, post.Text, post.AuthorID, post.ImageID, post.CreatedAt,
	).WithContext(ctx).Exec()
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
