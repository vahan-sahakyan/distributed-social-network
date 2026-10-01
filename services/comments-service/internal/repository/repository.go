package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vahan-sahakyan/distributed-social-network/comments-service/internal/model"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/database"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/outbox"
)

type Repository struct {
	pool *pgxpool.Pool
	db   database.Querier
}

func New(db *pgxpool.Pool) *Repository {
	return &Repository{pool: db, db: db}
}

// Tx runs fn with a repository bound to one transaction, committed if fn returns nil.
func (r *Repository) Tx(ctx context.Context, fn func(tx *Repository) error) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		return fn(&Repository{pool: r.pool, db: tx})
	})
}

// Enqueue stores an event that is published once the transaction commits.
func (r *Repository) Enqueue(ctx context.Context, topic, key string, payload any) error {
	return outbox.Enqueue(ctx, r.db, topic, key, payload)
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
