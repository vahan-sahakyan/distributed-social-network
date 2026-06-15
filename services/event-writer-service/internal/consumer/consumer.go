package consumer

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/vahan-sahakyan/distributed-social-network/event-writer-service/internal/model"
	"github.com/vahan-sahakyan/distributed-social-network/event-writer-service/internal/repository"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/id"
)

type Consumer struct {
	repo    *repository.Repository
	brokers string
}

func New(repo *repository.Repository, brokers string) *Consumer {
	return &Consumer{repo: repo, brokers: brokers}
}

func (c *Consumer) Start(ctx context.Context) {
	topics := []string{"post.created", "like.created", "comment.created"}

	for _, topic := range topics {
		go c.consume(ctx, topic)
	}

	<-ctx.Done()
}

func (c *Consumer) consume(ctx context.Context, topic string) {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: strings.Split(c.brokers, ","),
		Topic:   topic,
		GroupID: "event-writer-service",
	})
	defer reader.Close()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			msg, err := reader.ReadMessage(ctx)
			if err != nil {
				log.Printf("[%s] error reading message: %v", topic, err)
				continue
			}
			c.handleEvent(ctx, topic, msg.Value)
		}
	}
}

func (c *Consumer) handleEvent(ctx context.Context, eventType string, data []byte) {
	var payload struct {
		ID       string `json:"id"`
		PostID   string `json:"post_id"`
		UserID   string `json:"user_id"`
		AuthorID string `json:"author_id"`
	}

	if err := json.Unmarshal(data, &payload); err != nil {
		log.Printf("error unmarshaling event: %v", err)
		return
	}

	event := &model.FeedEvent{
		EventID:   id.New(),
		EventType: eventType,
		PostID:    payload.PostID,
		UserID:    payload.UserID,
		CreatedAt: time.Now().UTC(),
	}

	// For post.created, the post ID is "id" and user is "author_id"
	if eventType == "post.created" {
		event.PostID = payload.ID
		event.UserID = payload.AuthorID
	}

	switch eventType {
	case "like.created":
		event.LikesDelta = 1
	case "comment.created":
		event.CommentsDelta = 1
	}

	if err := c.repo.InsertEvent(ctx, event); err != nil {
		log.Printf("error inserting event to clickhouse: %v", err)
	}
}
