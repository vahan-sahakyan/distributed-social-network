package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vahan-sahakyan/distributed-social-network/comments-service/internal/model"
)

type Repository struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, comment *model.Comment) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO comments (id, user_id, entity_id, text, likes, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		comment.ID, comment.UserID, comment.EntityID, comment.Text, comment.Likes, comment.CreatedAt,
	)
	return err
}

func (r *Repository) GetByEntityID(ctx context.Context, entityID string) ([]model.Comment, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, user_id, entity_id, text, likes, created_at FROM comments WHERE entity_id = $1 ORDER BY created_at DESC`,
		entityID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []model.Comment
	for rows.Next() {
		var c model.Comment
		if err := rows.Scan(&c.ID, &c.UserID, &c.EntityID, &c.Text, &c.Likes, &c.CreatedAt); err != nil {
			return nil, err
		}
		comments = append(comments, c)
	}
	return comments, nil
}
