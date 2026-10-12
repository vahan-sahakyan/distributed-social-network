package service

import (
	"context"
	"errors"
	"testing"

	"github.com/vahan-sahakyan/distributed-social-network/pkg/events"
	"github.com/vahan-sahakyan/distributed-social-network/posts-service/internal/model"
)

// fakeStore keeps posts in memory and records each delete's topic
type fakeStore struct {
	posts     map[string]model.Post
	deleted   []string
	topics    []string
	failAfter int
}

func newFakeStore(posts ...model.Post) *fakeStore {
	f := &fakeStore{posts: map[string]model.Post{}, failAfter: -1}
	for _, p := range posts {
		f.posts[p.ID] = p
	}
	return f
}

func (f *fakeStore) CreateWithEvent(context.Context, *model.Post, string) error { return nil }

func (f *fakeStore) DeleteWithEvent(_ context.Context, post *model.Post, topic string) error {
	if f.failAfter == len(f.deleted) {
		return errors.New("scylla unavailable")
	}
	delete(f.posts, post.ID)
	f.deleted = append(f.deleted, post.ID)
	f.topics = append(f.topics, topic)
	return nil
}

func (f *fakeStore) GetByID(_ context.Context, id string) (*model.Post, error) {
	p, ok := f.posts[id]
	if !ok {
		return nil, model.ErrPostNotFound
	}
	return &p, nil
}

func (f *fakeStore) ListByAuthor(_ context.Context, authorID string) ([]model.Post, error) {
	var out []model.Post
	for _, p := range f.posts {
		if p.AuthorID == authorID {
			out = append(out, p)
		}
	}
	return out, nil
}

func TestDeletePostByAuthor(t *testing.T) {
	store := newFakeStore(model.Post{ID: "p1", AuthorID: "alice"})
	if err := New(store).DeletePost(context.Background(), "p1", "alice"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if len(store.deleted) != 1 || store.topics[0] != events.PostDeleted {
		t.Fatalf("got deletes %v on %v, want p1 on %s", store.deleted, store.topics, events.PostDeleted)
	}
}

func TestDeletePostRejectsOthers(t *testing.T) {
	store := newFakeStore(model.Post{ID: "p1", AuthorID: "alice"})
	if err := New(store).DeletePost(context.Background(), "p1", "bob"); !errors.Is(err, model.ErrNotAuthor) {
		t.Fatalf("got %v, want ErrNotAuthor", err)
	}
	if len(store.deleted) != 0 {
		t.Fatalf("deleted %v, want nothing", store.deleted)
	}
}

func TestDeletePostUnknown(t *testing.T) {
	if err := New(newFakeStore()).DeletePost(context.Background(), "nope", "alice"); !errors.Is(err, model.ErrPostNotFound) {
		t.Fatalf("got %v, want ErrPostNotFound", err)
	}
}

func TestDeleteUserPostsOnlyTheirs(t *testing.T) {
	store := newFakeStore(
		model.Post{ID: "a1", AuthorID: "alice"},
		model.Post{ID: "a2", AuthorID: "alice"},
		model.Post{ID: "b1", AuthorID: "bob"},
	)
	n, err := New(store).DeleteUserPosts(context.Background(), "alice")
	if err != nil || n != 2 {
		t.Fatalf("got %d, %v; want 2, nil", n, err)
	}
	if _, ok := store.posts["b1"]; !ok || len(store.posts) != 1 {
		t.Fatalf("left %v, want only b1", store.posts)
	}
}

// a failure stops the run and reports how many went; a retry finishes the rest
func TestDeleteUserPostsResumes(t *testing.T) {
	store := newFakeStore(
		model.Post{ID: "a1", AuthorID: "alice"},
		model.Post{ID: "a2", AuthorID: "alice"},
		model.Post{ID: "a3", AuthorID: "alice"},
	)
	store.failAfter = 1
	svc := New(store)
	if n, err := svc.DeleteUserPosts(context.Background(), "alice"); err == nil || n != 1 {
		t.Fatalf("got %d, %v; want 1 and an error", n, err)
	}
	store.failAfter = -1
	if n, err := svc.DeleteUserPosts(context.Background(), "alice"); err != nil || n != 2 {
		t.Fatalf("retry got %d, %v; want 2, nil", n, err)
	}
	if len(store.posts) != 0 {
		t.Fatalf("left %v, want none", store.posts)
	}
}
