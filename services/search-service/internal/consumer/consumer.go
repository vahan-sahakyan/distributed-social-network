package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/broker"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/events"
	"github.com/vahan-sahakyan/distributed-social-network/search-service/internal/model"
	"github.com/vahan-sahakyan/distributed-social-network/search-service/internal/repository"
)

// Topics is every topic this consumer reads.
func Topics() []string {
	return []string{events.PostCreated, events.UserCreated}
}

type Consumer struct {
	repo    *repository.Repository
	brokers string
	dlq     *broker.Producer
}

func New(repo *repository.Repository, brokers string, dlq *broker.Producer) *Consumer {
	return &Consumer{repo: repo, brokers: brokers, dlq: dlq}
}

func (c *Consumer) Start(ctx context.Context) {
	cfg := func(topic string) broker.ConsumerConfig {
		return broker.ConsumerConfig{Brokers: c.brokers, Topic: topic, GroupID: "search-service", DLQ: c.dlq}
	}
	go broker.ConsumeBatch(ctx, cfg(events.PostCreated), c.indexPosts)
	go broker.ConsumeBatch(ctx, cfg(events.UserCreated), c.indexUsers)

	<-ctx.Done()
}

func (c *Consumer) indexPosts(ctx context.Context, msgs []kafka.Message) error {
	posts := make([]model.Post, len(msgs))
	for i, msg := range msgs {
		p, err := buildPost(msg.Value)
		if err != nil {
			return fmt.Errorf("offset %d: %w", msg.Offset, err)
		}
		posts[i] = p
	}
	return c.repo.IndexPosts(ctx, posts)
}

func (c *Consumer) indexUsers(ctx context.Context, msgs []kafka.Message) error {
	users := make([]model.User, len(msgs))
	for i, msg := range msgs {
		u, err := buildUser(msg.Value)
		if err != nil {
			return fmt.Errorf("offset %d: %w", msg.Offset, err)
		}
		users[i] = u
	}
	return c.repo.IndexUsers(ctx, users)
}

func buildPost(data []byte) (model.Post, error) {
	var event struct {
		ID        string    `json:"id"`
		Text      string    `json:"text"`
		AuthorID  string    `json:"author_id"`
		ImageID   string    `json:"image_id"`
		CreatedAt time.Time `json:"created_at"`
	}
	if err := json.Unmarshal(data, &event); err != nil {
		return model.Post{}, fmt.Errorf("unmarshaling post: %w", err)
	}
	if event.ID == "" {
		return model.Post{}, fmt.Errorf("post event without id")
	}
	return model.Post{
		ID:        event.ID,
		AuthorID:  event.AuthorID,
		Text:      event.Text,
		ImageID:   event.ImageID,
		Hashtags:  hashtags(event.Text),
		CreatedAt: event.CreatedAt,
	}, nil
}

func buildUser(data []byte) (model.User, error) {
	var event struct {
		ID        string    `json:"id"`
		Username  string    `json:"username"`
		Bio       string    `json:"bio"`
		CreatedAt time.Time `json:"created_at"`
	}
	if err := json.Unmarshal(data, &event); err != nil {
		return model.User{}, fmt.Errorf("unmarshaling user: %w", err)
	}
	if event.ID == "" {
		return model.User{}, fmt.Errorf("user event without id")
	}
	return model.User{ID: event.ID, Username: event.Username, Bio: event.Bio, CreatedAt: event.CreatedAt}, nil
}

var hashtagPattern = regexp.MustCompile(`#([\p{L}\p{N}_]+)`)

// hashtags returns the distinct lowercased tags in text, in order of appearance.
func hashtags(text string) []string {
	tags := []string{}
	seen := map[string]bool{}
	for _, m := range hashtagPattern.FindAllStringSubmatch(text, -1) {
		tag := strings.ToLower(m[1])
		if !seen[tag] {
			seen[tag] = true
			tags = append(tags, tag)
		}
	}
	return tags
}
