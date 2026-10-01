// Package posts answers whether a post exists, for services that accept writes
// referring to one.
package posts

import (
	"context"

	postspb "github.com/vahan-sahakyan/distributed-social-network/pkg/grpc/posts"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Checker struct {
	client postspb.PostsServiceClient
}

func NewChecker(client postspb.PostsServiceClient) *Checker {
	return &Checker{client: client}
}

// Exists reports whether the post exists. Other failures, such as posts-service
// being unreachable, are returned as errors rather than as a missing post.
func (c *Checker) Exists(ctx context.Context, id string) (bool, error) {
	_, err := c.client.GetPost(ctx, &postspb.GetPostRequest{Id: id})
	if status.Code(err) == codes.NotFound {
		return false, nil
	}
	return err == nil, err
}
