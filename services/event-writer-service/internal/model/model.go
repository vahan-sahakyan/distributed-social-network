package model

import "time"

type FeedEvent struct {
	EventID       string    `json:"event_id"`
	EventType     string    `json:"event_type"`
	PostID        string    `json:"post_id"`
	UserID        string    `json:"user_id"`
	LikesDelta    int32     `json:"likes_delta"`
	CommentsDelta int32     `json:"comments_delta"`
	CreatedAt     time.Time `json:"created_at"`
}
