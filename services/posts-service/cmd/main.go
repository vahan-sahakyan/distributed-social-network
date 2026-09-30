package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/vahan-sahakyan/distributed-social-network/pkg/broker"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/database"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/events"
	postspb "github.com/vahan-sahakyan/distributed-social-network/pkg/grpc/posts"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/observability"
	grpcserver "github.com/vahan-sahakyan/distributed-social-network/posts-service/internal/grpcserver"
	"github.com/vahan-sahakyan/distributed-social-network/posts-service/internal/repository"
	"github.com/vahan-sahakyan/distributed-social-network/posts-service/internal/service"
	"github.com/vahan-sahakyan/distributed-social-network/posts-service/migrations"

	"github.com/ansrivas/fiberprometheus/v2"
	"github.com/gofiber/fiber/v2"
	"google.golang.org/grpc"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	shutdown := observability.Init(ctx, "posts-service")
	defer shutdown()

	scyllaHosts := os.Getenv("SCYLLA_HOSTS")
	if scyllaHosts == "" {
		scyllaHosts = "localhost:9042"
	}
	scyllaKeyspace := os.Getenv("SCYLLA_KEYSPACE")
	if scyllaKeyspace == "" {
		scyllaKeyspace = "posts"
	}

	if err := database.MigrateScylla(scyllaHosts, migrations.SQL); err != nil {
		log.Fatalf("failed to run scylla migration: %v", err)
	}

	db, err := database.NewScyllaDB(scyllaHosts, scyllaKeyspace)
	if err != nil {
		log.Fatalf("failed to connect to scylladb: %v", err)
	}
	defer db.Close()

	producer := broker.NewProducer(os.Getenv("KAFKA_BROKERS"))
	defer producer.Close()

	repo := repository.New(db)
	svc := service.New(repo, producer)

	// gRPC server
	grpcPort := os.Getenv("GRPC_PORT")
	if grpcPort == "" {
		grpcPort = "9081"
	}
	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		log.Fatalf("failed to listen on grpc port: %v", err)
	}
	grpcSrv := grpc.NewServer(observability.GRPCServerOptions()...)
	postspb.RegisterPostsServiceServer(grpcSrv, grpcserver.New(svc, func(ctx context.Context) error {
		return db.Query("TRUNCATE posts").WithContext(ctx).Exec()
	}))
	observability.InitGRPCMetrics(grpcSrv)
	go func() {
		log.Printf("gRPC server listening on :%s", grpcPort)
		if err := grpcSrv.Serve(lis); err != nil {
			log.Fatalf("gRPC server failed: %v", err)
		}
	}()

	// HTTP server (health + metrics only)
	app := fiber.New(fiber.Config{AppName: "posts-service", DisableStartupMessage: true})
	prometheus := fiberprometheus.NewWithDefaultRegistry("posts-service")
	prometheus.RegisterAt(app, "/metrics")
	app.Use(prometheus.Middleware)

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	go func() {
		if err := app.Listen(":" + port); err != nil {
			log.Fatalf("failed to start HTTP server: %v", err)
		}
	}()

	// Topics are created after /health is serving so a cold broker does not fail
	// the liveness probe. Publishes fail while Kafka is unreachable either way.
	go func() {
		if err := broker.EnsureTopics(ctx, os.Getenv("KAFKA_BROKERS"), events.PostCreated); err != nil {
			log.Fatalf("failed to ensure kafka topics: %v", err)
		}
	}()

	<-ctx.Done()
	grpcSrv.GracefulStop()
	_ = app.Shutdown()
}
