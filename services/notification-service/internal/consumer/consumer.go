package consumer

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	"github.com/segmentio/kafka-go"
	"github.com/vahan-sahakyan/distributed-social-network/notification-service/internal/service"
	postspb "github.com/vahan-sahakyan/distributed-social-network/pkg/grpc/posts"
	"google.golang.org/grpc"
)

// notification types keyed by source topic
var notifTypes = map[string]string{
	"like.created":    "like",
	"comment.created": "comment",
}

type Consumer struct {
	svc         *service.Service
	brokers     string
	postsClient postspb.PostsServiceClient
}

func New(svc *service.Service, brokers string, postsConn *grpc.ClientConn) *Consumer {
	return &Consumer{
		svc:         svc,
		brokers:     brokers,
		postsClient: postspb.NewPostsServiceClient(postsConn),
	}
}

func (c *Consumer) Start(ctx context.Context) {
	for topic := range notifTypes {
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

	resp, err := c.postsClient.GetPost(ctx, &postspb.GetPostRequest{Id: event.EntityID})
	if err != nil || resp.Post == nil || resp.Post.AuthorId == "" {
		log.Printf("error resolving author of %s: %v", event.EntityID, err)
		return
	}
	authorID := resp.Post.AuthorId
	if authorID == event.UserID {
		return
	}

	if err := c.svc.CreateNotification(ctx, authorID, notifTypes[topic], event.UserID, event.EntityID); err != nil {
		log.Printf("error creating notification: %v", err)
	}
}
