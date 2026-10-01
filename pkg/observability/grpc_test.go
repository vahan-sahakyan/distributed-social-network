package observability

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
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

// An upstream that is restarting must not fail calls that have time left.
func TestDialOptionsWaitForAnUpstreamComingUp(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := lis.Addr().String()
	_ = lis.Close()

	conn, err := grpc.NewClient(addr, GRPCDialOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	srv := grpc.NewServer()
	healthpb.RegisterHealthServer(srv, health.NewServer())
	defer srv.Stop()
	go func() {
		time.Sleep(700 * time.Millisecond)
		lis, err := net.Listen("tcp", addr)
		if err != nil {
			t.Error(err)
			return
		}
		_ = srv.Serve(lis)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	if _, err := healthpb.NewHealthClient(conn).Check(ctx, &healthpb.HealthCheckRequest{}); err != nil {
		t.Fatalf("call during the upstream's startup failed: %v", err)
	}
}
