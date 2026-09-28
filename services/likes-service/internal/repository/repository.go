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

// Create inserts the like, returning false if the user already liked the entity.
func (r *Repository) Create(ctx context.Context, like *model.Like) (bool, error) {
	tag, err := r.db.Exec(ctx,
		`INSERT INTO likes (id, user_id, entity_id) VALUES ($1, $2, $3)
		 ON CONFLICT (user_id, entity_id) DO NOTHING`,
		like.ID, like.UserID, like.EntityID,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (r *Repository) HasLiked(ctx context.Context, userID, entityID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM likes WHERE user_id=$1 AND entity_id=$2)`,
		userID, entityID,
	).Scan(&exists)
	return exists, err
}

// Delete removes the like, returning false if there was nothing to remove.
func (r *Repository) Delete(ctx context.Context, userID, entityID string) (bool, error) {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM likes WHERE user_id=$1 AND entity_id=$2`,
		userID, entityID,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}
