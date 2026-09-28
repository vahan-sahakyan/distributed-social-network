package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/vahan-sahakyan/distributed-social-network/event-writer-service/internal/model"
	"github.com/vahan-sahakyan/distributed-social-network/event-writer-service/internal/repository"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/broker"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/events"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/id"
)

type Consumer struct {
	repo    *repository.Repository
	brokers string
	dlq     *broker.Producer
}

func New(repo *repository.Repository, brokers string, dlq *broker.Producer) *Consumer {
	return &Consumer{repo: repo, brokers: brokers, dlq: dlq}
}

func (c *Consumer) Start(ctx context.Context) {
	for _, topic := range events.All {
		go broker.Consume(ctx, broker.ConsumerConfig{
			Brokers: c.brokers,
			Topic:   topic,
			GroupID: "event-writer-service",
			DLQ:     c.dlq,
		}, c.handler(topic))
	}

	<-ctx.Done()
}

func (c *Consumer) handler(topic string) broker.Handler {
	return func(ctx context.Context, msg kafka.Message) error {
		return c.handleEvent(ctx, topic, msg)
	}
}

func (c *Consumer) handleEvent(ctx context.Context, eventType string, msg kafka.Message) error {
	event, err := buildEvent(eventType, msg)
	if err != nil {
		return err
	}
	if err := c.repo.InsertEvent(ctx, event); err != nil {
		return fmt.Errorf("inserting event to clickhouse: %w", err)
	}
	return nil
}

// buildEvent maps a Kafka message to the feed_events row it produces.
func buildEvent(eventType string, msg kafka.Message) (*model.FeedEvent, error) {
	var payload struct {
		ID       string `json:"id"`
		PostID   string `json:"post_id"`
		EntityID string `json:"entity_id"` // likes and comments use entity_id
		UserID   string `json:"user_id"`
		AuthorID string `json:"author_id"`
	}

	if err := json.Unmarshal(msg.Value, &payload); err != nil {
		return nil, fmt.Errorf("unmarshaling event: %w", err)
	}

	event := &model.FeedEvent{
		// Derived from the message's Kafka coordinates rather than generated, so a
		// redelivery after a failed commit writes the same event_id and readers can
		// collapse the duplicate instead of counting the like twice.
		EventID:   id.Deterministic(msg.Topic, strconv.Itoa(msg.Partition), strconv.FormatInt(msg.Offset, 10)),
		EventType: eventType,
		PostID:    payload.PostID,
		UserID:    payload.UserID,
		CreatedAt: time.Now().UTC(),
	}

	switch eventType {
	case events.PostCreated:
		event.PostID = payload.ID
		event.UserID = payload.AuthorID
	case events.LikeCreated, events.LikeDeleted, events.CommentCreated:
		event.PostID = payload.EntityID
	}

	switch eventType {
	case events.LikeCreated:
		event.LikesDelta = 1
	case events.LikeDeleted:
		event.LikesDelta = -1
	case events.CommentCreated:
		event.CommentsDelta = 1
	}

	return event, nil
}
