package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vahan/distributed-social-network/posts-service/internal/model"
)

type Repository struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, post *model.Post) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO posts (id, text, author_id, image_id, likes, comments, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		post.ID, post.Text, post.AuthorID, post.ImageID, post.Likes, post.Comments, post.CreatedAt,
	)
	return err
}

func (r *Repository) GetByID(ctx context.Context, id string) (*model.Post, error) {
	var post model.Post
	err := r.db.QueryRow(ctx,
		`SELECT id, text, author_id, image_id, likes, comments, created_at FROM posts WHERE id = $1`, id,
	).Scan(&post.ID, &post.Text, &post.AuthorID, &post.ImageID, &post.Likes, &post.Comments, &post.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &post, nil
}
