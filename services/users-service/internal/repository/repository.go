package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vahan/distributed-social-network/users-service/internal/model"
)

type Repository struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, user *model.User) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO users (id, username, bio, created_at) VALUES ($1, $2, $3, $4)`,
		user.ID, user.Username, user.Bio, user.CreatedAt,
	)
	return err
}

func (r *Repository) GetByID(ctx context.Context, id string) (*model.User, error) {
	var user model.User
	err := r.db.QueryRow(ctx,
		`SELECT id, username, bio, created_at FROM users WHERE id = $1`, id,
	).Scan(&user.ID, &user.Username, &user.Bio, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) Follow(ctx context.Context, followerID, followeeID string) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO follows (follower_id, followee_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		followerID, followeeID,
	)
	return err
}

func (r *Repository) GetFollowers(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.db.Query(ctx,
		`SELECT follower_id FROM follows WHERE followee_id = $1`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (r *Repository) GetFollowing(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.db.Query(ctx,
		`SELECT followee_id FROM follows WHERE follower_id = $1`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}
