package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vahan/distributed-social-network/notification-service/internal/model"
)

type Repository struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, n *model.Notification) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO notifications (id, user_id, type, actor_id, entity_id, read, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		n.ID, n.UserID, n.Type, n.ActorID, n.EntityID, n.Read, n.CreatedAt,
	)
	return err
}

func (r *Repository) GetByUserID(ctx context.Context, userID string) ([]model.Notification, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, user_id, type, actor_id, entity_id, read, created_at
		 FROM notifications WHERE user_id = $1 ORDER BY created_at DESC LIMIT 50`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notifications []model.Notification
	for rows.Next() {
		var n model.Notification
		if err := rows.Scan(&n.ID, &n.UserID, &n.Type, &n.ActorID, &n.EntityID, &n.Read, &n.CreatedAt); err != nil {
			return nil, err
		}
		notifications = append(notifications, n)
	}
	return notifications, nil
}
