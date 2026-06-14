package model

import "time"

type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Bio       string    `json:"bio,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateUserRequest struct {
	Username string `json:"username"`
	Bio      string `json:"bio,omitempty"`
}

type FollowRequest struct {
	FollowerID string `json:"follower_id"`
}
