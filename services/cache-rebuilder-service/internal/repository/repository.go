package repository

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/vahan-sahakyan/distributed-social-network/cache-rebuilder-service/internal/model"
)

type Repository struct {
	conn driver.Conn
}

func New(conn driver.Conn) *Repository {
	return &Repository{conn: conn}
}

func (r *Repository) GetRecentPostEvents(ctx context.Context, limit int) ([]model.FeedEvent, error) {
	rows, err := r.conn.Query(ctx,
		`SELECT event_id, event_type, post_id, user_id, created_at
		 FROM feed_events
		 WHERE event_type = 'post.created'
		 ORDER BY created_at DESC
		 LIMIT ?`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []model.FeedEvent
	for rows.Next() {
		var e model.FeedEvent
		if err := rows.Scan(&e.EventID, &e.EventType, &e.PostID, &e.UserID, &e.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, nil
}

func (r *Repository) GetPostStates(ctx context.Context) ([]model.PostState, error) {
	rows, err := r.conn.Query(ctx, `
		SELECT
			post_id,
			countIf(event_type = 'like.created')    AS likes,
			countIf(event_type = 'comment.created') AS comments,
			max(created_at) AS last_update
		FROM feed_events
		WHERE post_id != ''
		GROUP BY post_id
		ORDER BY last_update DESC
		LIMIT 1000`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var states []model.PostState
	for rows.Next() {
		var s model.PostState
		if err := rows.Scan(&s.PostID, &s.Likes, &s.Comments, &s.LastUpdate); err != nil {
			return nil, err
		}
		states = append(states, s)
	}
	return states, nil
}
