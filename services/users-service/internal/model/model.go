package model

import (
	"errors"
	"time"
)

var (
	ErrUsernameTaken = errors.New("username already taken")
	ErrUserNotFound  = errors.New("user not found")
	ErrUserExists    = errors.New("profile already exists")
)

type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Bio       string    `json:"bio,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateUserRequest struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Bio      string `json:"bio,omitempty"`
}

type FollowRequest struct {
	FollowerID string `json:"follower_id"`
}
