package observability

import (
	"context"
	"time"

	grpcprom "github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/resolver"
)

var (
	serverMetrics = grpcprom.NewServerMetrics(grpcprom.WithServerHandlingTimeHistogram())
	clientMetrics = grpcprom.NewClientMetrics(grpcprom.WithClientHandlingTimeHistogram())
)

func init() {
	prometheus.MustRegister(serverMetrics, clientMetrics)
	// Hand addresses to the dialer, which resolves them on every connect. gRPC's
	// dns resolver retries a failed lookup (a stopped container) on its own backoff,
	// up to 120s, so callers kept failing long after the upstream was back.
	// Upstreams are single addresses (compose names, Kubernetes Services), so its
	// multi-address balancing isn't used.
	resolver.SetDefaultScheme("passthrough")
}

// GRPCServerOptions traces every call and records grpc_server_* metrics.
func GRPCServerOptions() []grpc.ServerOption {
	return []grpc.ServerOption{
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(serverMetrics.UnaryServerInterceptor()),
		grpc.ChainStreamInterceptor(serverMetrics.StreamServerInterceptor()),
	}
}

// InitGRPCMetrics zero-fills the metrics of every registered method, so idle methods
// show up in dashboards. Call it after registering the services.
func InitGRPCMetrics(srv *grpc.Server) {
	serverMetrics.InitializeMetrics(srv)
}

// DefaultCallTimeout bounds unary calls whose context has no deadline.
const DefaultCallTimeout = 5 * time.Second

// GRPCDialOptions is the plaintext dial config for in-cluster calls, traced and
// recorded as grpc_client_* metrics. Unary calls without a deadline get
// DefaultCallTimeout, and wait for a connection within it instead of failing while
// the upstream restarts; reconnect backoff is capped at 3s.
func GRPCDialOptions() []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.WaitForReady(true)),
		grpc.WithConnectParams(grpc.ConnectParams{
			Backoff:           backoff.Config{BaseDelay: 200 * time.Millisecond, Multiplier: 1.6, Jitter: 0.2, MaxDelay: 3 * time.Second},
			MinConnectTimeout: 5 * time.Second,
		}),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		grpc.WithChainUnaryInterceptor(withDefaultTimeout(DefaultCallTimeout), clientMetrics.UnaryClientInterceptor()),
		grpc.WithChainStreamInterceptor(clientMetrics.StreamClientInterceptor()),
	}
}

// withDefaultTimeout makes a hung upstream fail the call instead of holding it forever.
func withDefaultTimeout(d time.Duration) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if _, ok := ctx.Deadline(); !ok {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, d)
			defer cancel()
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}
