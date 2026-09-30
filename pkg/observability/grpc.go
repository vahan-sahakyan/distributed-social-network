package observability

import (
	"context"
	"time"

	grpcprom "github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	serverMetrics = grpcprom.NewServerMetrics(grpcprom.WithServerHandlingTimeHistogram())
	clientMetrics = grpcprom.NewClientMetrics(grpcprom.WithClientHandlingTimeHistogram())
)

func init() {
	prometheus.MustRegister(serverMetrics, clientMetrics)
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
// recorded as grpc_client_* metrics. Unary calls without a deadline get DefaultCallTimeout.
func GRPCDialOptions() []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
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
