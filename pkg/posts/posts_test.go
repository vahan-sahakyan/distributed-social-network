package posts

import (
	"context"
	"testing"

	postspb "github.com/vahan-sahakyan/distributed-social-network/pkg/grpc/posts"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeClient struct {
	postspb.PostsServiceClient
	err error
}

func (f fakeClient) GetPost(context.Context, *postspb.GetPostRequest, ...grpc.CallOption) (*postspb.GetPostResponse, error) {
	return &postspb.GetPostResponse{}, f.err
}

func TestExists(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		want    bool
		wantErr bool
	}{
		{"found", nil, true, false},
		{"not found", status.Error(codes.NotFound, "post not found"), false, false},
		{"posts-service down", status.Error(codes.Unavailable, "down"), false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewChecker(fakeClient{err: tt.err}).Exists(context.Background(), "p1")
			if got != tt.want || (err != nil) != tt.wantErr {
				t.Errorf("Exists = %v, %v; want %v, err %v", got, err, tt.want, tt.wantErr)
			}
		})
	}
}
