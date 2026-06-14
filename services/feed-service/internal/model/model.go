package model

import "time"

type FeedItem struct {
	UserID        string    `json:"user_id"`
	PostID        string    `json:"post_id"`
	AuthorID      string    `json:"author_id"`
	Text          string    `json:"text"`
	LikesCount    int       `json:"likes_count"`
	CommentsCount int       `json:"comments_count"`
	ImageURL      string    `json:"image_url,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}
