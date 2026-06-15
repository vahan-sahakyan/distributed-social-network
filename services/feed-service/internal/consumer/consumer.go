package consumer

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/vahan-sahakyan/distributed-social-network/feed-service/internal/model"
	"github.com/vahan-sahakyan/distributed-social-network/feed-service/internal/service"
)

type Consumer struct {
	svc     *service.Service
	brokers string
}

func New(svc *service.Service, brokers string) *Consumer {
	return &Consumer{svc: svc, brokers: brokers}
}

func (c *Consumer) Start(ctx context.Context) {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: strings.Split(c.brokers, ","),
		Topic:   "post.created",
		GroupID: "feed-service",
	})
	defer reader.Close()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			msg, err := reader.ReadMessage(ctx)
			if err != nil {
				log.Printf("error reading message: %v", err)
				continue
			}
			c.handlePostCreated(msg.Value)
		}
	}
}

func (c *Consumer) handlePostCreated(data []byte) {
	var post struct {
		ID        string    `json:"id"`
		Text      string    `json:"text"`
		AuthorID  string    `json:"author_id"`
		ImageID   string    `json:"image_id"`
		CreatedAt time.Time `json:"created_at"`
	}

	if err := json.Unmarshal(data, &post); err != nil {
		log.Printf("error unmarshaling post: %v", err)
		return
	}

	item := &model.FeedItem{
		PostID:    post.ID,
		AuthorID:  post.AuthorID,
		Text:      post.Text,
		ImageURL:  post.ImageID,
		CreatedAt: post.CreatedAt,
	}

	// TODO: fetch followers from users-service
	followerIDs := []string{}

	if err := c.svc.FanoutPost(item, followerIDs); err != nil {
		log.Printf("error fanning out post: %v", err)
	}
}
