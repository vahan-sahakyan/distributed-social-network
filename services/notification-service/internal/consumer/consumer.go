package consumer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/segmentio/kafka-go"
	"github.com/vahan-sahakyan/distributed-social-network/notification-service/internal/service"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/broker"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/events"
	postspb "github.com/vahan-sahakyan/distributed-social-network/pkg/grpc/posts"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/id"
	"google.golang.org/grpc"
)

// notification types keyed by source topic
var notifTypes = map[string]string{
	events.LikeCreated:    "like",
	events.CommentCreated: "comment",
}

// Topics is every topic this consumer reads.
func Topics() []string {
	return []string{events.LikeCreated, events.CommentCreated}
}

type Consumer struct {
	svc         *service.Service
	brokers     string
	dlq         *broker.Producer
	postsClient postspb.PostsServiceClient
}

func New(svc *service.Service, brokers string, dlq *broker.Producer, postsConn *grpc.ClientConn) *Consumer {
	return &Consumer{
		svc:         svc,
		brokers:     brokers,
		dlq:         dlq,
		postsClient: postspb.NewPostsServiceClient(postsConn),
	}
}

func (c *Consumer) Start(ctx context.Context) {
	for _, topic := range Topics() {
		go broker.Consume(ctx, broker.ConsumerConfig{
			Brokers: c.brokers,
			Topic:   topic,
			GroupID: "notification-service",
			DLQ:     c.dlq,
		}, c.handler(topic))
	}

	<-ctx.Done()
}

func (c *Consumer) handler(topic string) broker.Handler {
	return func(ctx context.Context, msg kafka.Message) error {
		return c.handle(ctx, topic, msg)
	}
}

// handle returns an error when the notification could not be created, so a
// posts-service blip is retried instead of losing the notification.
func (c *Consumer) handle(ctx context.Context, topic string, msg kafka.Message) error {
	var event struct {
		UserID   string `json:"user_id"`
		EntityID string `json:"entity_id"`
	}

	if err := json.Unmarshal(msg.Value, &event); err != nil {
		return fmt.Errorf("unmarshaling event: %w", err)
	}

	resp, err := c.postsClient.GetPost(ctx, &postspb.GetPostRequest{Id: event.EntityID})
	if err != nil {
		return fmt.Errorf("resolving author of %s: %w", event.EntityID, err)
	}
	if resp.Post == nil || resp.Post.AuthorId == "" {
		return fmt.Errorf("resolving author of %s: post not found", event.EntityID)
	}
	authorID := resp.Post.AuthorId
	if authorID == event.UserID {
		return nil
	}

	if err := c.svc.CreateNotification(ctx, notificationID(msg), authorID, notifTypes[topic], event.UserID, event.EntityID); err != nil {
		return fmt.Errorf("creating notification: %w", err)
	}
	return nil
}

// notificationID is derived from the event's identity, so a redelivery or a
// republish by the outbox produces the same id and the insert is skipped.
func notificationID(msg kafka.Message) string {
	return id.Deterministic(broker.DedupeKey(msg)...)
}
