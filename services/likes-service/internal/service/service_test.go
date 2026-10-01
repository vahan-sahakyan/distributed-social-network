package service

import (
	"context"
	"errors"
	"testing"

	"github.com/vahan-sahakyan/distributed-social-network/likes-service/internal/model"
)

type fakePosts struct {
	exists bool
	err    error
}

func (f fakePosts) Exists(context.Context, string) (bool, error) { return f.exists, f.err }

// the existence check runs before the database is touched, so no repository is needed
func TestCreateLikeRejectsUnknownPost(t *testing.T) {
	svc := New(nil, fakePosts{exists: false})
	if _, err := svc.CreateLike(context.Background(), &model.CreateLikeRequest{UserID: "u1", EntityID: "p1"}); !errors.Is(err, model.ErrPostNotFound) {
		t.Fatalf("got %v, want ErrPostNotFound", err)
	}
}

func TestCreateLikeReportsPostsServiceFailure(t *testing.T) {
	svc := New(nil, fakePosts{err: errors.New("posts-service unavailable")})
	_, err := svc.CreateLike(context.Background(), &model.CreateLikeRequest{UserID: "u1", EntityID: "p1"})
	if err == nil || errors.Is(err, model.ErrPostNotFound) {
		t.Fatalf("got %v, want the lookup error, not a missing post", err)
	}
}
