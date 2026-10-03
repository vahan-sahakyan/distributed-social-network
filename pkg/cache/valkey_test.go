package cache

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/valkey-io/valkey-go"
)

func testClient(t *testing.T) valkey.Client {
	t.Helper()
	addr := os.Getenv("CACHE_TEST_VALKEY_ADDR")
	if addr == "" {
		t.Skip("CACHE_TEST_VALKEY_ADDR not set")
	}
	c, err := valkey.NewClient(valkey.ClientOption{InitAddress: []string{addr}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Close)
	return c
}

// unique keeps tests apart without flushing a shared server
func unique(t *testing.T, s string) string {
	return fmt.Sprintf("%s-%s-%d", t.Name(), s, time.Now().UnixNano())
}

func post(t *testing.T, id string, at time.Time) Post {
	return Post{ID: unique(t, id), AuthorID: "a1", Text: "text " + id, CreatedAt: at}
}

func TestAddPostRedeliveryKeepsCountsAndAppearsOnce(t *testing.T) {
	ctx, c := context.Background(), testClient(t)
	feed, p := HomeFeedKey(unique(t, "u")), post(t, "p", time.Now())
	if err := AddPost(ctx, c, p, feed); err != nil {
		t.Fatal(err)
	}
	if err := AdjustCounts(ctx, c, p.ID, 2, 1); err != nil {
		t.Fatal(err)
	}
	if err := AddPost(ctx, c, p, feed); err != nil {
		t.Fatal(err)
	}
	got, err := ReadFeed(ctx, c, feed)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].LikesCount != 2 || got[0].CommentsCount != 1 {
		t.Fatalf("got %+v, want the post once with 2 likes and 1 comment", got)
	}
}

func TestFeedKeepsNewestFeedCapPostsNewestFirst(t *testing.T) {
	ctx, c := context.Background(), testClient(t)
	feed, start := HomeFeedKey(unique(t, "u")), time.Now()
	var ids []string
	for i := range FeedCap + 5 {
		p := post(t, fmt.Sprint(i), start.Add(time.Duration(i)*time.Second))
		ids = append(ids, p.ID)
		if err := AddPost(ctx, c, p, feed); err != nil {
			t.Fatal(err)
		}
	}
	got, err := ReadFeed(ctx, c, feed)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != FeedCap || got[0].ID != ids[len(ids)-1] || got[FeedCap-1].ID != ids[5] {
		t.Fatalf("got %d posts, head %q tail %q; want %d, newest first, oldest 5 dropped", len(got), got[0].ID, got[len(got)-1].ID, FeedCap)
	}
}

func TestAdjustCountsIsOrderIndependentAndSkipsUncachedPosts(t *testing.T) {
	ctx, c := context.Background(), testClient(t)
	for _, order := range [][]int{{1, -1}, {-1, 1}} {
		feed, p := HomeFeedKey(unique(t, "u")), post(t, "p", time.Now())
		if err := AddPost(ctx, c, p, feed); err != nil {
			t.Fatal(err)
		}
		for _, d := range order {
			if err := AdjustCounts(ctx, c, p.ID, d, 0); err != nil {
				t.Fatal(err)
			}
		}
		got, err := ReadFeed(ctx, c, feed)
		if err != nil {
			t.Fatal(err)
		}
		if got[0].LikesCount != 0 {
			t.Errorf("deltas %v ended at %d likes, want 0", order, got[0].LikesCount)
		}
	}
	missing := unique(t, "missing")
	if err := AdjustCounts(ctx, c, missing, 1, 0); err != nil {
		t.Fatal(err)
	}
	if n, _ := c.Do(ctx, c.B().Exists().Key(PostKey(missing)).Build()).AsInt64(); n != 0 {
		t.Error("an uncached post must not get a counts-only hash")
	}
}

func TestReadFeedSkipsEvictedPosts(t *testing.T) {
	ctx, c := context.Background(), testClient(t)
	feed, now := HomeFeedKey(unique(t, "u")), time.Now()
	kept, evicted := post(t, "kept", now), post(t, "evicted", now.Add(time.Second))
	for _, p := range []Post{kept, evicted} {
		if err := AddPost(ctx, c, p, feed); err != nil {
			t.Fatal(err)
		}
	}
	c.Do(ctx, c.B().Del().Key(PostKey(evicted.ID)).Build())
	got, err := ReadFeed(ctx, c, feed)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != kept.ID {
		t.Fatalf("got %+v, want only the post still cached", got)
	}
}

func TestReplaceFeedSetsContentAndCounts(t *testing.T) {
	ctx, c := context.Background(), testClient(t)
	feed, now := HomeFeedKey(unique(t, "u")), time.Now()
	stale := post(t, "stale", now)
	if err := AddPost(ctx, c, stale, feed); err != nil {
		t.Fatal(err)
	}
	fresh := post(t, "fresh", now.Add(time.Second))
	fresh.LikesCount = 7
	if err := ReplaceFeed(ctx, c, feed, []Post{fresh}); err != nil {
		t.Fatal(err)
	}
	got, err := ReadFeed(ctx, c, feed)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != fresh.ID || got[0].LikesCount != 7 || !got[0].CreatedAt.Equal(fresh.CreatedAt) {
		t.Fatalf("got %+v, want only the fresh post with its 7 likes", got)
	}
	if err := ReplaceFeed(ctx, c, feed, nil); err != nil {
		t.Fatal(err)
	}
	if got, _ := ReadFeed(ctx, c, feed); len(got) != 0 {
		t.Fatalf("got %d posts, want an empty feed", len(got))
	}
}
