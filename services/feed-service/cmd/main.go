package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/vahan-sahakyan/distributed-social-network/feed-service/internal/consumer"
	grpcserver "github.com/vahan-sahakyan/distributed-social-network/feed-service/internal/grpcserver"
	"github.com/vahan-sahakyan/distributed-social-network/feed-service/internal/repository"
	"github.com/vahan-sahakyan/distributed-social-network/feed-service/internal/service"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/broker"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/cache"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/events"
	feedpb "github.com/vahan-sahakyan/distributed-social-network/pkg/grpc/feed"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/observability"

	"github.com/ansrivas/fiberprometheus/v2"
	"github.com/gofiber/fiber/v2"
	"google.golang.org/grpc"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	shutdown := observability.Init(ctx, "feed-service")
	defer shutdown()

	valkeyAddr := os.Getenv("VALKEY_ADDR")
	if valkeyAddr == "" {
		valkeyAddr = "localhost:6379"
	}

	vk := cache.NewValkey(ctx, valkeyAddr)
	defer vk.Close()

	repo := repository.New(vk)
	svc := service.New(repo)

	// followers for the fanout of new posts
	usersAddr := os.Getenv("USERS_SERVICE_GRPC_ADDR")
	if usersAddr == "" {
		usersAddr = "localhost:9085"
	}

	usersConn, err := grpc.NewClient(usersAddr, observability.GRPCDialOptions()...)
	if err != nil {
		log.Fatalf("failed to connect to users-service: %v", err)
	}
	defer usersConn.Close()

	// gRPC server
	grpcPort := os.Getenv("GRPC_PORT")
	if grpcPort == "" {
		grpcPort = "9082"
	}
	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		log.Fatalf("failed to listen on grpc port: %v", err)
	}
	grpcSrv := grpc.NewServer(observability.GRPCServerOptions()...)
	feedpb.RegisterFeedServiceServer(grpcSrv, grpcserver.New(svc))
	observability.InitGRPCMetrics(grpcSrv)
	go func() {
		log.Printf("gRPC server listening on :%s", grpcPort)
		if err := grpcSrv.Serve(lis); err != nil {
			log.Fatalf("gRPC server failed: %v", err)
		}
	}()

	// HTTP server (health + metrics only)
	app := fiber.New(fiber.Config{AppName: "feed-service", DisableStartupMessage: true})
	prometheus := fiberprometheus.NewWithDefaultRegistry("feed-service")
	prometheus.RegisterAt(app, "/metrics")
	app.Use(prometheus.Middleware)

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}

	go func() {
		if err := app.Listen(":" + port); err != nil {
			log.Fatalf("failed to start HTTP server: %v", err)
		}
	}()

	// Kafka setup runs after the health endpoint is already serving: waiting on a
	// cold broker here would otherwise leave the liveness probe unanswered and get
	// the pod killed before it ever finished starting.
	go func() {
		brokers := os.Getenv("KAFKA_BROKERS")
		if err := broker.EnsureTopics(ctx, brokers, events.All...); err != nil {
			log.Fatalf("failed to ensure kafka topics: %v", err)
		}

		dlq := broker.NewProducer(brokers)
		defer dlq.Close()

		consumer.New(svc, brokers, dlq, usersConn).Start(ctx)
	}()

	<-ctx.Done()
	grpcSrv.GracefulStop()
	_ = app.Shutdown()
}
