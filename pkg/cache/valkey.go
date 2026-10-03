package cache

import (
	"context"
	"errors"
	"log"
	"strconv"
	"time"

	"github.com/valkey-io/valkey-go"
)

// Feeds are sorted sets of post IDs scored by creation time; each post and its
// counts live once, in a hash under PostKey. A like is then one write, however
// many feeds show the post.
const (
	TTL     = time.Hour
	FeedCap = 100
)

// Post is a cached post, as feeds show it.
type Post struct {
	ID            string
	AuthorID      string
	Text          string
	ImageURL      string
	CreatedAt     time.Time
	LikesCount    int
	CommentsCount int
}

// NewValkey returns a connected Valkey client, retrying until the server is reachable or ctx is done.
func NewValkey(ctx context.Context, addr string) valkey.Client {
	for {
		c, err := valkey.NewClient(valkey.ClientOption{InitAddress: []string{addr}})
		if err == nil {
			return c
		}
		log.Printf("waiting for valkey at %s: %v", addr, err)
		select {
		case <-ctx.Done():
			log.Fatalf("context cancelled waiting for valkey")
		case <-time.After(2 * time.Second):
		}
	}
}

// AddPost caches p and puts it at the head of each feed, capped at FeedCap. A
// redelivered post keeps the counts it has gathered since, and lands once per
// feed: a sorted set holds each post ID once.
func AddPost(ctx context.Context, c valkey.Client, p Post, feedKeys ...string) error {
	key := PostKey(p.ID)
	cmds := valkey.Commands{
		c.B().Hset().Key(key).FieldValue().
			FieldValue("author_id", p.AuthorID).
			FieldValue("text", p.Text).
			FieldValue("image_url", p.ImageURL).
			FieldValue("created_at", p.CreatedAt.Format(time.RFC3339Nano)).Build(),
		c.B().Hsetnx().Key(key).Field("likes_count").Value(strconv.Itoa(p.LikesCount)).Build(),
		c.B().Hsetnx().Key(key).Field("comments_count").Value(strconv.Itoa(p.CommentsCount)).Build(),
		c.B().Expire().Key(key).Seconds(int64(TTL.Seconds())).Build(),
	}
	for _, feed := range feedKeys {
		cmds = append(cmds, feedCmds(c, feed, p)...)
	}
	return joinErrs(c.DoMulti(ctx, cmds...))
}

// ReplaceFeed caches posts with their counts as given and makes them the whole
// feed at key. The feed is built aside and renamed into place, so readers never
// see it half written.
func ReplaceFeed(ctx context.Context, c valkey.Client, key string, posts []Post) error {
	tmp := key + ":rebuild"
	cmds := valkey.Commands{c.B().Del().Key(tmp).Build()}
	for _, p := range posts {
		pk := PostKey(p.ID)
		cmds = append(cmds,
			c.B().Hset().Key(pk).FieldValue().
				FieldValue("author_id", p.AuthorID).
				FieldValue("text", p.Text).
				FieldValue("image_url", p.ImageURL).
				FieldValue("created_at", p.CreatedAt.Format(time.RFC3339Nano)).
				FieldValue("likes_count", strconv.Itoa(p.LikesCount)).
				FieldValue("comments_count", strconv.Itoa(p.CommentsCount)).Build(),
			c.B().Expire().Key(pk).Seconds(int64(TTL.Seconds())).Build(),
			c.B().Zadd().Key(tmp).ScoreMember().ScoreMember(score(p), p.ID).Build(),
		)
	}
	if len(posts) == 0 {
		// RENAME needs a source; an empty feed is no key at all
		cmds = append(cmds, c.B().Del().Key(key).Build())
		return joinErrs(c.DoMulti(ctx, cmds...))
	}
	cmds = append(cmds,
		c.B().Zremrangebyrank().Key(tmp).Start(0).Stop(-FeedCap-1).Build(),
		c.B().Rename().Key(tmp).Newkey(key).Build(),
		c.B().Expire().Key(key).Seconds(int64(TTL.Seconds())).Build(),
	)
	return joinErrs(c.DoMulti(ctx, cmds...))
}

// adjustCounts adds deltas to a cached post only: a post that isn't cached has
// nothing to correct, and creating it here would leave a hash with counts only
var adjustCounts = valkey.NewLuaScript(`
if redis.call('EXISTS', KEYS[1]) == 0 then return 0 end
redis.call('HINCRBY', KEYS[1], 'likes_count', ARGV[1])
redis.call('HINCRBY', KEYS[1], 'comments_count', ARGV[2])
return 1`)

// AdjustCounts applies like and comment deltas to a cached post. Sums are stored
// raw, never clamped: like and unlike arrive on separate topics in any order, and
// only an unclamped sum ends right. Readers clamp negatives.
func AdjustCounts(ctx context.Context, c valkey.Client, postID string, likesDelta, commentsDelta int) error {
	return adjustCounts.Exec(ctx, c, []string{PostKey(postID)},
		[]string{strconv.Itoa(likesDelta), strconv.Itoa(commentsDelta)}).Error()
}

// ReadFeed returns the feed at key, newest first. Posts evicted from the cache are
// left out; a missing feed is empty.
func ReadFeed(ctx context.Context, c valkey.Client, key string) ([]Post, error) {
	ids, err := c.Do(ctx, c.B().Zrange().Key(key).Min("0").Max(strconv.Itoa(FeedCap-1)).Rev().Build()).AsStrSlice()
	if err != nil || len(ids) == 0 {
		return nil, err
	}
	cmds := make(valkey.Commands, len(ids))
	for i, id := range ids {
		cmds[i] = c.B().Hgetall().Key(PostKey(id)).Build()
	}
	posts := make([]Post, 0, len(ids))
	for i, resp := range c.DoMulti(ctx, cmds...) {
		fields, err := resp.AsStrMap()
		if err != nil {
			return nil, err
		}
		if len(fields) == 0 {
			continue
		}
		posts = append(posts, postFromHash(ids[i], fields))
	}
	return posts, nil
}

// Flush empties the cache.
func Flush(ctx context.Context, c valkey.Client) error {
	return c.Do(ctx, c.B().Flushdb().Build()).Error()
}

func feedCmds(c valkey.Client, feed string, p Post) valkey.Commands {
	return valkey.Commands{
		c.B().Zadd().Key(feed).ScoreMember().ScoreMember(score(p), p.ID).Build(),
		c.B().Zremrangebyrank().Key(feed).Start(0).Stop(-FeedCap - 1).Build(),
		c.B().Expire().Key(feed).Seconds(int64(TTL.Seconds())).Build(),
	}
}

func score(p Post) float64 {
	return float64(p.CreatedAt.UnixMilli())
}

func postFromHash(id string, f map[string]string) Post {
	createdAt, _ := time.Parse(time.RFC3339Nano, f["created_at"])
	likes, _ := strconv.Atoi(f["likes_count"])
	comments, _ := strconv.Atoi(f["comments_count"])
	return Post{
		ID:            id,
		AuthorID:      f["author_id"],
		Text:          f["text"],
		ImageURL:      f["image_url"],
		CreatedAt:     createdAt,
		LikesCount:    likes,
		CommentsCount: comments,
	}
}

func joinErrs(resps []valkey.ValkeyResult) error {
	var errs []error
	for _, r := range resps {
		if err := r.Error(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
