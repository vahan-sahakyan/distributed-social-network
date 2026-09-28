package model

import "time"

type PostState struct {
	PostID     string    `json:"post_id"`
	Likes      int64     `json:"likes"`
	Comments   int64     `json:"comments"`
	LastUpdate time.Time `json:"last_update"`
}

type FeedEvent struct {
	EventID   string    `json:"event_id"`
	EventType string    `json:"event_type"`
	PostID    string    `json:"post_id"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}
