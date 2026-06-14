package consumer

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	"github.com/segmentio/kafka-go"
	"github.com/vahan/distributed-social-network/notification-service/internal/service"
)

type Consumer struct {
	svc     *service.Service
	brokers string
}

func New(svc *service.Service, brokers string) *Consumer {
	return &Consumer{svc: svc, brokers: brokers}
}

func (c *Consumer) Start(ctx context.Context) {
	topics := []string{"like.created", "comment.created"}

	for _, topic := range topics {
		go c.consume(ctx, topic)
	}

	<-ctx.Done()
}

func (c *Consumer) consume(ctx context.Context, topic string) {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: strings.Split(c.brokers, ","),
		Topic:   topic,
		GroupID: "notification-service",
	})
	defer reader.Close()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			msg, err := reader.ReadMessage(ctx)
			if err != nil {
				log.Printf("error reading from %s: %v", topic, err)
				continue
			}
			c.handle(ctx, topic, msg.Value)
		}
	}
}

func (c *Consumer) handle(ctx context.Context, topic string, data []byte) {
	var event struct {
		UserID   string `json:"user_id"`
		EntityID string `json:"entity_id"`
	}

	if err := json.Unmarshal(data, &event); err != nil {
		log.Printf("error unmarshaling event: %v", err)
		return
	}

	notifType := strings.Replace(topic, ".", "_", -1)

	// TODO: resolve target user from entity
	_ = c.svc.CreateNotification(ctx, "", notifType, event.UserID, event.EntityID)
}
