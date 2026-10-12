package main

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	commentspb "github.com/vahan-sahakyan/distributed-social-network/pkg/grpc/comments"
	feedpb "github.com/vahan-sahakyan/distributed-social-network/pkg/grpc/feed"
	likespb "github.com/vahan-sahakyan/distributed-social-network/pkg/grpc/likes"
	notificationspb "github.com/vahan-sahakyan/distributed-social-network/pkg/grpc/notifications"
	postspb "github.com/vahan-sahakyan/distributed-social-network/pkg/grpc/posts"
	userspb "github.com/vahan-sahakyan/distributed-social-network/pkg/grpc/users"
)

// fakes answer Reset only; any other call panics on the nil embedded client
type (
	fakeUsers struct {
		userspb.UsersServiceClient
	}
	fakePosts struct {
		postspb.PostsServiceClient
	}
	fakeComments struct {
		commentspb.CommentsServiceClient
	}
	fakeLikes struct {
		likespb.LikesServiceClient
	}
	fakeNotifications struct {
		notificationspb.NotificationServiceClient
	}
	fakeFeed struct {
		feedpb.FeedServiceClient
	}
)

func (fakeUsers) Reset(context.Context, *userspb.ResetRequest, ...grpc.CallOption) (*userspb.ResetResponse, error) {
	return &userspb.ResetResponse{}, nil
}

func (fakePosts) Reset(context.Context, *postspb.ResetRequest, ...grpc.CallOption) (*postspb.ResetResponse, error) {
	return &postspb.ResetResponse{}, nil
}

// alice is the author of every post
func (fakePosts) DeletePost(_ context.Context, req *postspb.DeletePostRequest, _ ...grpc.CallOption) (*postspb.DeletePostResponse, error) {
	if req.RequesterId != "alice" {
		return nil, status.Error(codes.PermissionDenied, "only the author can delete a post")
	}
	return &postspb.DeletePostResponse{}, nil
}

func (fakePosts) DeleteUserPosts(_ context.Context, req *postspb.DeleteUserPostsRequest, _ ...grpc.CallOption) (*postspb.DeleteUserPostsResponse, error) {
	if req.AuthorId != "alice" {
		return &postspb.DeleteUserPostsResponse{}, nil
	}
	return &postspb.DeleteUserPostsResponse{Deleted: 3}, nil
}

func (fakeComments) Reset(context.Context, *commentspb.ResetRequest, ...grpc.CallOption) (*commentspb.ResetResponse, error) {
	return &commentspb.ResetResponse{}, nil
}

func (fakeLikes) Reset(context.Context, *likespb.ResetRequest, ...grpc.CallOption) (*likespb.ResetResponse, error) {
	return &likespb.ResetResponse{}, nil
}

func (fakeNotifications) Reset(context.Context, *notificationspb.ResetRequest, ...grpc.CallOption) (*notificationspb.ResetResponse, error) {
	return &notificationspb.ResetResponse{}, nil
}

func (fakeFeed) Reset(context.Context, *feedpb.ResetRequest, ...grpc.CallOption) (*feedpb.ResetResponse, error) {
	return &feedpb.ResetResponse{}, nil
}

// appWithoutOptional has search and cache-rebuilder switched off
func appWithoutOptional(t *testing.T) *fiber.App {
	t.Helper()
	t.Setenv("ALLOW_RESET", "true")
	app := fiber.New()
	cl := &clients{
		users:         fakeUsers{},
		posts:         fakePosts{},
		comments:      fakeComments{},
		likes:         fakeLikes{},
		notifications: fakeNotifications{},
		feed:          fakeFeed{},
	}
	registerRoutes(app, cl, func(c *fiber.Ctx) error { return c.Next() })
	return app
}

func TestSwitchedOffServicesAnswer503(t *testing.T) {
	app := appWithoutOptional(t)
	cases := []struct{ method, path, want string }{
		{"GET", "/api/v1/search/posts?q=go", "search is disabled"},
		{"GET", "/api/v1/search/users?q=al", "search is disabled"},
		{"GET", "/api/v1/search/hashtags/trending", "search is disabled"},
		{"POST", "/api/v1/rebuild", "event store is disabled"},
	}
	for _, tc := range cases {
		resp, err := app.Test(httptest.NewRequest(tc.method, tc.path, nil))
		if err != nil {
			t.Fatal(err)
		}
		var body map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != fiber.StatusServiceUnavailable || body["error"] != tc.want {
			t.Errorf("%s %s: got %d %q, want 503 %q", tc.method, tc.path, resp.StatusCode, body["error"], tc.want)
		}
	}
}

func TestResetSkipsSwitchedOffServices(t *testing.T) {
	app := appWithoutOptional(t)
	resp, err := app.Test(httptest.NewRequest("POST", "/api/v1/reset", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("got %d, want 200", resp.StatusCode)
	}
}

func TestOptionalDial(t *testing.T) {
	t.Setenv("SEARCH_SERVICE_GRPC_ADDR", "")
	if conn := optionalDial("SEARCH_SERVICE_GRPC_ADDR", "localhost:9091"); conn != nil {
		t.Error("set but empty: want nil, the service is off")
	}
	t.Setenv("SEARCH_SERVICE_GRPC_ADDR", "search-service:9091")
	if conn := optionalDial("SEARCH_SERVICE_GRPC_ADDR", "localhost:9091"); conn == nil {
		t.Error("set: want a connection")
	}
}

// the caller comes from the X-User header in place of a token
func appAs(t *testing.T) *fiber.App {
	t.Helper()
	app := fiber.New()
	registerRoutes(app, &clients{posts: fakePosts{}}, func(c *fiber.Ctx) error {
		c.Locals(identityKey, identity{ID: c.Get("X-User")})
		return c.Next()
	})
	return app
}

func TestDeletePost(t *testing.T) {
	app := appAs(t)
	cases := []struct {
		user string
		want int
	}{
		{"alice", fiber.StatusNoContent},
		{"bob", fiber.StatusForbidden},
	}
	for _, tc := range cases {
		req := httptest.NewRequest("DELETE", "/api/v1/posts/p1", nil)
		req.Header.Set("X-User", tc.user)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != tc.want {
			t.Errorf("%s: got %d, want %d", tc.user, resp.StatusCode, tc.want)
		}
	}
}

func TestDeleteAllOwnPosts(t *testing.T) {
	app := appAs(t)
	req := httptest.NewRequest("DELETE", "/api/v1/posts/", nil)
	req.Header.Set("X-User", "alice")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]int
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK || body["deleted"] != 3 {
		t.Errorf("got %d %v, want 200 deleted=3 (the caller's posts)", resp.StatusCode, body)
	}
}
