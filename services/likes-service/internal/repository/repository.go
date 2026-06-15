package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vahan-sahakyan/distributed-social-network/likes-service/internal/model"
)

type Repository struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, like *model.Like) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO likes (id, user_id, entity_id) VALUES ($1, $2, $3)
		 ON CONFLICT (user_id, entity_id) DO NOTHING`,
		like.ID, like.UserID, like.EntityID,
	)
	return err
}
