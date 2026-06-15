package repository

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/vahan-sahakyan/distributed-social-network/event-writer-service/internal/model"
)

type Repository struct {
	conn driver.Conn
}

func New(conn driver.Conn) *Repository {
	return &Repository{conn: conn}
}

func (r *Repository) InsertEvent(ctx context.Context, event *model.FeedEvent) error {
	return r.conn.Exec(ctx,
		`INSERT INTO feed_events (event_id, event_type, post_id, user_id, likes_delta, comments_delta, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		event.EventID, event.EventType, event.PostID, event.UserID, event.LikesDelta, event.CommentsDelta, event.CreatedAt,
	)
}
