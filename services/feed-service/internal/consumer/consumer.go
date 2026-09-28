package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/vahan-sahakyan/distributed-social-network/feed-service/internal/model"
	"github.com/vahan-sahakyan/distributed-social-network/feed-service/internal/service"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/broker"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/events"
	postspb "github.com/vahan-sahakyan/distributed-social-network/pkg/grpc/posts"
	userspb "github.com/vahan-sahakyan/distributed-social-network/pkg/grpc/users"
	"google.golang.org/grpc"
)

type Consumer struct {
	svc         *service.Service
	brokers     string
	dlq         *broker.Producer
	usersClient userspb.UsersServiceClient
	postsClient postspb.PostsServiceClient
}

func New(svc *service.Service, brokers string, dlq *broker.Producer, usersConn, postsConn *grpc.ClientConn) *Consumer {
	return &Consumer{
		svc:         svc,
		brokers:     brokers,
		dlq:         dlq,
		usersClient: userspb.NewUsersServiceClient(usersConn),
		postsClient: postspb.NewPostsServiceClient(postsConn),
	}
}

func (c *Consumer) Start(ctx context.Context) {
	go c.consume(ctx, events.PostCreated, "feed-service-posts", c.handlePostCreated)
	go c.consume(ctx, events.LikeCreated, "feed-service-likes", c.countsHandler(1, 0))
	go c.consume(ctx, events.LikeDeleted, "feed-service-unlikes", c.countsHandler(-1, 0))
	c.consume(ctx, events.CommentCreated, "feed-service-comments", c.countsHandler(0, 1))
}

func (c *Consumer) consume(ctx context.Context, topic, groupID string, handler broker.Handler) {
	broker.Consume(ctx, broker.ConsumerConfig{
		Brokers: c.brokers,
		Topic:   topic,
		GroupID: groupID,
		DLQ:     c.dlq,
	}, handler)
}

func (c *Consumer) handlePostCreated(ctx context.Context, msg kafka.Message) error {
	var post struct {
		ID        string    `json:"id"`
		Text      string    `json:"text"`
		AuthorID  string    `json:"author_id"`
		ImageID   string    `json:"image_id"`
		CreatedAt time.Time `json:"created_at"`
	}
	if err := json.Unmarshal(msg.Value, &post); err != nil {
		// Malformed payloads never succeed on retry; park them immediately.
		return fmt.Errorf("unmarshaling post: %w", err)
	}

	item := &model.FeedItem{
		PostID:    post.ID,
		AuthorID:  post.AuthorID,
		Text:      post.Text,
		ImageURL:  post.ImageID,
		CreatedAt: post.CreatedAt,
	}

	followerIDs, err := c.fetchFollowers(ctx, post.AuthorID)
	if err != nil {
		return err
	}
	followerIDs = append(followerIDs, post.AuthorID)

	return c.svc.FanoutPost(item, followerIDs)
}

// countsHandler returns a handler for like/comment events that applies the given deltas to cached feeds.
func (c *Consumer) countsHandler(likesDelta, commentsDelta int) broker.Handler {
	return func(ctx context.Context, msg kafka.Message) error {
		var event struct {
			UserID   string `json:"user_id"`
			EntityID string `json:"entity_id"`
		}
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			return fmt.Errorf("unmarshaling event: %w", err)
		}

		authorID, err := c.fetchPostAuthor(ctx, event.EntityID)
		if err != nil {
			return err
		}

		followers, err := c.fetchFollowers(ctx, authorID)
		if err != nil {
			return err
		}

		// Lookup failures above are retried, but once deltas start landing a retry
		// would apply them twice to the feeds that already took them. A partial
		// failure is logged by AdjustCounts and left for the cache rebuild.
		_ = c.svc.AdjustCounts(event.EntityID, authorID, append(followers, authorID), likesDelta, commentsDelta)
		return nil
	}
}

func (c *Consumer) fetchFollowers(ctx context.Context, userID string) ([]string, error) {
	resp, err := c.usersClient.GetFollowers(ctx, &userspb.GetFollowersRequest{UserId: userID})
	if err != nil {
		return nil, fmt.Errorf("fetching followers for %s: %w", userID, err)
	}
	return resp.Followers, nil
}

func (c *Consumer) fetchPostAuthor(ctx context.Context, postID string) (string, error) {
	resp, err := c.postsClient.GetPost(ctx, &postspb.GetPostRequest{Id: postID})
	if err != nil {
		return "", fmt.Errorf("fetching post %s: %w", postID, err)
	}
	if resp.Post == nil || resp.Post.AuthorId == "" {
		return "", fmt.Errorf("post %s has no author", postID)
	}
	return resp.Post.AuthorId, nil
}
