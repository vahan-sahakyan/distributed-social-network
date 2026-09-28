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
		 LIMIT 1 BY event_id
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
	// The inner query collapses rows sharing an event_id. event-writer derives
	// event_id from the Kafka offset, so a message redelivered after a failed
	// commit is written twice with the same id and must only count once.
	rows, err := r.conn.Query(ctx, `
		SELECT
			pid                AS post_id,
			sum(dlikes)        AS likes,
			sum(dcomments)     AS comments,
			max(first_seen)    AS last_update
		FROM (
			SELECT
				event_id,
				any(post_id)        AS pid,
				any(likes_delta)    AS dlikes,
				any(comments_delta) AS dcomments,
				min(created_at)     AS first_seen
			FROM feed_events
			WHERE post_id != ''
			GROUP BY event_id
		)
		GROUP BY pid
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
