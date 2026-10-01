package consumer

import (
	"context"
	"encoding/json"
	"fmt"
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
		go broker.ConsumeBatch(ctx, broker.ConsumerConfig{
			Brokers: c.brokers,
			Topic:   topic,
			GroupID: "event-writer-service",
			DLQ:     c.dlq,
		}, c.handler(topic))
	}

	<-ctx.Done()
}

func (c *Consumer) handler(topic string) broker.BatchHandler {
	return func(ctx context.Context, msgs []kafka.Message) error {
		return c.handleEvents(ctx, topic, msgs)
	}
}

func (c *Consumer) handleEvents(ctx context.Context, eventType string, msgs []kafka.Message) error {
	events := make([]*model.FeedEvent, len(msgs))
	for i, msg := range msgs {
		event, err := buildEvent(eventType, msg)
		if err != nil {
			return fmt.Errorf("offset %d: %w", msg.Offset, err)
		}
		events[i] = event
	}
	if err := c.repo.InsertEvents(ctx, events); err != nil {
		return fmt.Errorf("inserting %d events to clickhouse: %w", len(events), err)
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
		// Derived from the event's identity rather than generated, so a redelivery or
		// a republish by the outbox writes the same event_id and readers collapse the
		// duplicate instead of counting the like twice.
		EventID:   id.Deterministic(broker.DedupeKey(msg)...),
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
