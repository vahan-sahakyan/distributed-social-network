package model

import "errors"

// ErrPostNotFound is returned for writes that refer to a post that doesn't exist.
var ErrPostNotFound = errors.New("post not found")

type Like struct {
	ID       string `json:"id"`
	UserID   string `json:"user_id"`
	EntityID string `json:"entity_id"`
}

type CreateLikeRequest struct {
	UserID   string `json:"user_id"`
	EntityID string `json:"entity_id"`
}
