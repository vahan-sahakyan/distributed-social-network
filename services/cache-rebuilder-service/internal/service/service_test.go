package service

import (
	"testing"

	"github.com/vahan-sahakyan/distributed-social-network/cache-rebuilder-service/internal/model"
)

func TestNewFeedItemClampsNegativeCounts(t *testing.T) {
	post := &postResponse{ID: "p1", AuthorID: "u1"}

	item := newFeedItem(post, model.PostState{Likes: -1, Comments: -3})
	if item.LikesCount != 0 || item.CommentsCount != 0 {
		t.Errorf("got likes=%d comments=%d, want both clamped to 0", item.LikesCount, item.CommentsCount)
	}

	item = newFeedItem(post, model.PostState{Likes: 4, Comments: 2})
	if item.LikesCount != 4 || item.CommentsCount != 2 {
		t.Errorf("got likes=%d comments=%d, want 4/2", item.LikesCount, item.CommentsCount)
	}
}
