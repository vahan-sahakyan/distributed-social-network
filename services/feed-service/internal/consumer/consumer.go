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
	postspb "github.com/vahan-sahakyan/distributed-social-network/pkg/grpc/posts"
	userspb "github.com/vahan-sahakyan/distributed-social-network/pkg/grpc/users"
	"google.golang.org/grpc"
)

type Consumer struct {
	svc         *service.Service
	brokers     string
	usersClient userspb.UsersServiceClient
	postsClient postspb.PostsServiceClient
}

func New(svc *service.Service, brokers string, usersConn, postsConn *grpc.ClientConn) *Consumer {
	return &Consumer{
		svc:         svc,
		brokers:     brokers,
		usersClient: userspb.NewUsersServiceClient(usersConn),
		postsClient: postspb.NewPostsServiceClient(postsConn),
	}
}

func (c *Consumer) Start(ctx context.Context) {
	go c.startReader(ctx, "post.created", "feed-service-posts", c.handlePostCreated)
	go c.startReader(ctx, "like.created", "feed-service-likes", c.handleLikeCreated)
	c.startReader(ctx, "comment.created", "feed-service-comments", c.handleCommentCreated)
}

func (c *Consumer) startReader(ctx context.Context, topic, groupID string, handler func([]byte)) {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: strings.Split(c.brokers, ","),
		Topic:   topic,
		GroupID: groupID,
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
			handler(msg.Value)
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

	followerIDs := c.fetchFollowers(post.AuthorID)
	followerIDs = append(followerIDs, post.AuthorID)

	if err := c.svc.FanoutPost(item, followerIDs); err != nil {
		log.Printf("error fanning out post: %v", err)
	}
}

func (c *Consumer) handleLikeCreated(data []byte) {
	var event struct {
		UserID   string `json:"user_id"`
		EntityID string `json:"entity_id"`
	}
	if err := json.Unmarshal(data, &event); err != nil {
		return
	}
	authorID, ok := c.fetchPostAuthor(event.EntityID)
	if !ok {
		return
	}
	users := append(c.fetchFollowers(authorID), authorID)
	c.svc.IncrementLikes(event.EntityID, users)
}

func (c *Consumer) handleCommentCreated(data []byte) {
	var event struct {
		UserID   string `json:"user_id"`
		EntityID string `json:"entity_id"`
	}
	if err := json.Unmarshal(data, &event); err != nil {
		return
	}
	authorID, ok := c.fetchPostAuthor(event.EntityID)
	if !ok {
		return
	}
	users := append(c.fetchFollowers(authorID), authorID)
	c.svc.IncrementComments(event.EntityID, users)
}

func (c *Consumer) fetchFollowers(userID string) []string {
	resp, err := c.usersClient.GetFollowers(context.Background(), &userspb.GetFollowersRequest{UserId: userID})
	if err != nil {
		log.Printf("error fetching followers for %s: %v", userID, err)
		return nil
	}
	return resp.Followers
}

func (c *Consumer) fetchPostAuthor(postID string) (string, bool) {
	resp, err := c.postsClient.GetPost(context.Background(), &postspb.GetPostRequest{Id: postID})
	if err != nil || resp.Post == nil || resp.Post.AuthorId == "" {
		return "", false
	}
	return resp.Post.AuthorId, true
}
