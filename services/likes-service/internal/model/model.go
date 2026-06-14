package model

type Like struct {
	ID       string `json:"id"`
	UserID   string `json:"user_id"`
	EntityID string `json:"entity_id"`
}

type CreateLikeRequest struct {
	UserID   string `json:"user_id"`
	EntityID string `json:"entity_id"`
}
