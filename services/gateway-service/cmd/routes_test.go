package main

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"google.golang.org/grpc"

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
