package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/vahan-sahakyan/distributed-social-network/pkg/broker"
	searchpb "github.com/vahan-sahakyan/distributed-social-network/pkg/grpc/search"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/observability"
	"github.com/vahan-sahakyan/distributed-social-network/search-service/internal/consumer"
	grpcserver "github.com/vahan-sahakyan/distributed-social-network/search-service/internal/grpcserver"
	"github.com/vahan-sahakyan/distributed-social-network/search-service/internal/repository"

	"github.com/ansrivas/fiberprometheus/v2"
	"github.com/elastic/go-elasticsearch/v9"
	"github.com/gofiber/fiber/v2"
	"go.opentelemetry.io/otel"
	"google.golang.org/grpc"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	shutdown := observability.Init(ctx, "search-service")
	defer shutdown()

	esURL := os.Getenv("ELASTICSEARCH_URL")
	if esURL == "" {
		esURL = "http://localhost:9200"
	}
	es, err := elasticsearch.New(
		elasticsearch.WithAddresses(esURL),
		elasticsearch.WithInstrumentation(elasticsearch.NewOpenTelemetryInstrumentation(otel.GetTracerProvider(), false)),
	)
	if err != nil {
		log.Fatalf("failed to create elasticsearch client: %v", err)
	}
	repo := repository.New(es)

	// gRPC server
	grpcPort := os.Getenv("GRPC_PORT")
	if grpcPort == "" {
		grpcPort = "9091"
	}
	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		log.Fatalf("failed to listen on grpc port: %v", err)
	}
	grpcSrv := grpc.NewServer(observability.GRPCServerOptions()...)
	searchpb.RegisterSearchServiceServer(grpcSrv, grpcserver.New(repo))
	observability.InitGRPCMetrics(grpcSrv)
	go func() {
		log.Printf("gRPC server listening on :%s", grpcPort)
		if err := grpcSrv.Serve(lis); err != nil {
			log.Fatalf("gRPC server failed: %v", err)
		}
	}()

	// HTTP server (health + metrics only)
	app := fiber.New(fiber.Config{AppName: "search-service", DisableStartupMessage: true})
	prometheus := fiberprometheus.NewWithDefaultRegistry("search-service")
	prometheus.RegisterAt(app, "/metrics")
	app.Use(prometheus.Middleware)

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8091"
	}

	go func() {
		if err := app.Listen(":" + port); err != nil {
			log.Fatalf("failed to start HTTP server: %v", err)
		}
	}()

	// Indexing starts only once the indices exist: writing to a missing alias would
	// auto-create a plain index with guessed mappings.
	go func() {
		if err := repo.EnsureIndices(ctx); err != nil {
			log.Fatalf("failed to ensure indices: %v", err)
		}

		brokers := os.Getenv("KAFKA_BROKERS")
		if err := broker.EnsureTopics(ctx, brokers, consumer.Topics()...); err != nil {
			log.Fatalf("failed to ensure kafka topics: %v", err)
		}

		dlq := broker.NewProducer(brokers)
		defer dlq.Close()

		consumer.New(repo, brokers, dlq).Start(ctx)
	}()

	<-ctx.Done()
	grpcSrv.GracefulStop()
	_ = app.Shutdown()
}
