package model

import "time"

type Post struct {
	ID        string    `json:"id"`
	Text      string    `json:"text"`
	AuthorID  string    `json:"author_id"`
	ImageID   string    `json:"image_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type CreatePostRequest struct {
	Text     string `json:"text"`
	AuthorID string `json:"author_id"`
	ImageID  string `json:"image_id,omitempty"`
}
