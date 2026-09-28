package repository

import (
	"fmt"
	"testing"

	"github.com/vahan-sahakyan/distributed-social-network/feed-service/internal/model"
)

func TestPrependItem(t *testing.T) {
	feed := []model.FeedItem{{PostID: "p1"}}

	got, changed := prependItem(feed, &model.FeedItem{PostID: "p2"})
	if !changed || len(got) != 2 || got[0].PostID != "p2" {
		t.Fatalf("new post: got %v changed=%v, want p2 at the head", got, changed)
	}

	// A redelivered post.created must not duplicate the post.
	again, changed := prependItem(got, &model.FeedItem{PostID: "p2"})
	if changed || len(again) != 2 {
		t.Fatalf("redelivery: got %d items changed=%v, want 2 unchanged", len(again), changed)
	}
}

func TestPrependItemCapsFeed(t *testing.T) {
	var feed []model.FeedItem
	for i := range 100 {
		feed = append(feed, model.FeedItem{PostID: fmt.Sprint(i)})
	}
	got, _ := prependItem(feed, &model.FeedItem{PostID: "new"})
	if len(got) != 100 || got[0].PostID != "new" || got[99].PostID != "98" {
		t.Fatalf("got len %d head %q tail %q, want 100 with oldest dropped", len(got), got[0].PostID, got[len(got)-1].PostID)
	}
}
