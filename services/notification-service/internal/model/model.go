package model

import "time"

type Notification struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Type      string    `json:"type"`
	ActorID   string    `json:"actor_id"`
	EntityID  string    `json:"entity_id"`
	Read      bool      `json:"read"`
	CreatedAt time.Time `json:"created_at"`
}
