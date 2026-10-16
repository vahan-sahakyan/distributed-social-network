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

// DeleteWithEvent removes the post and records its event in one LOGGED batch: both apply or neither.
func (r *Repository) DeleteWithEvent(ctx context.Context, post *model.Post, topic string) error {
	b := r.db.NewBatch(gocql.LoggedBatch).WithContext(ctx)
	b.Query(`DELETE FROM posts WHERE id = ?`, post.ID)
	if err := outbox.AddToBatch(ctx, b, topic, post.ID, post); err != nil {
		return err
	}
	return r.db.ExecuteBatch(b)
}

// ListByAuthor returns every post of authorID's, through the author_id index.
func (r *Repository) ListByAuthor(ctx context.Context, authorID string) ([]model.Post, error) {
	iter := r.db.Query(
		`SELECT id, text, author_id, image_id, created_at FROM posts WHERE author_id = ?`, authorID,
	).WithContext(ctx).Iter()
	var posts []model.Post
	var p model.Post
	for iter.Scan(&p.ID, &p.Text, &p.AuthorID, &p.ImageID, &p.CreatedAt) {
		posts = append(posts, p)
	}
	return posts, iter.Close()
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
