package model

import "time"

// Post is a posts index document; the id is the document _id.
type Post struct {
	ID        string    `json:"-"`
	AuthorID  string    `json:"author_id"`
	Text      string    `json:"text"`
	ImageID   string    `json:"image_id,omitempty"`
	Hashtags  []string  `json:"hashtags"`
	CreatedAt time.Time `json:"created_at"`
}

// User is a users index document; the id is the document _id.
type User struct {
	ID        string    `json:"-"`
	Username  string    `json:"username"`
	Bio       string    `json:"bio,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type PostHit struct {
	Post
	Highlight string
	Score     float64
}

type UserHit struct {
	User
	Score float64
}

type HashtagCount struct {
	Hashtag string
	Posts   int64
}
