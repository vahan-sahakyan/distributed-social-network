package repository

import (
	"context"
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/vahan-sahakyan/distributed-social-network/event-writer-service/internal/model"
)

type Repository struct {
	conn driver.Conn
}

func New(conn driver.Conn) *Repository {
	return &Repository{conn: conn}
}

// InsertEvents writes events in one insert. Async insert lets the server merge small
// batches into one part; waiting for it keeps the write durable before the commit.
func (r *Repository) InsertEvents(ctx context.Context, events []*model.FeedEvent) error {
	ctx = clickhouse.Context(ctx, clickhouse.WithSettings(clickhouse.Settings{
		"async_insert":          1,
		"wait_for_async_insert": 1,
	}))
	batch, err := r.conn.PrepareBatch(ctx,
		`INSERT INTO feed_events (event_id, event_type, post_id, user_id, likes_delta, comments_delta, created_at)`)
	if err != nil {
		return fmt.Errorf("preparing batch: %w", err)
	}
	defer batch.Abort()

	for _, e := range events {
		if err := batch.Append(e.EventID, e.EventType, e.PostID, e.UserID, e.LikesDelta, e.CommentsDelta, e.CreatedAt); err != nil {
			return fmt.Errorf("appending event %s: %w", e.EventID, err)
		}
	}
	return batch.Send()
}
