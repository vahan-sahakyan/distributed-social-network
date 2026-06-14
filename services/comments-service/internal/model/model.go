package model

import "time"

type Comment struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	EntityID  string    `json:"entity_id"`
	Text      string    `json:"text"`
	Likes     int       `json:"likes"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateCommentRequest struct {
	UserID   string `json:"user_id"`
	EntityID string `json:"entity_id"`
	Text     string `json:"text"`
}
