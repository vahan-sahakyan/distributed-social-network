package observability

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc"
)

func TestWithDefaultTimeout(t *testing.T) {
	var got time.Time
	var hasDeadline bool
	invoker := func(ctx context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
		got, hasDeadline = ctx.Deadline()
		return nil
	}
	intercept := withDefaultTimeout(time.Minute)

	if err := intercept(context.Background(), "/m", nil, nil, nil, invoker); err != nil {
		t.Fatal(err)
	}
	if !hasDeadline || time.Until(got) > time.Minute {
		t.Errorf("no default deadline applied: %v %v", hasDeadline, got)
	}

	own, cancel := context.WithTimeout(context.Background(), time.Hour)
	defer cancel()
	want, _ := own.Deadline()
	if err := intercept(own, "/m", nil, nil, nil, invoker); err != nil {
		t.Fatal(err)
	}
	if !got.Equal(want) {
		t.Errorf("caller deadline overridden: got %v, want %v", got, want)
	}
}
