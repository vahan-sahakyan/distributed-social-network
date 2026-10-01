package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	grpcserver "github.com/vahan-sahakyan/distributed-social-network/likes-service/internal/grpcserver"
	"github.com/vahan-sahakyan/distributed-social-network/likes-service/internal/repository"
	"github.com/vahan-sahakyan/distributed-social-network/likes-service/internal/service"
	"github.com/vahan-sahakyan/distributed-social-network/likes-service/migrations"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/broker"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/database"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/events"
	likespb "github.com/vahan-sahakyan/distributed-social-network/pkg/grpc/likes"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/observability"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/outbox"

	"github.com/ansrivas/fiberprometheus/v2"
	"github.com/gofiber/fiber/v2"
	"google.golang.org/grpc"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	shutdown := observability.Init(ctx, "likes-service")
	defer shutdown()

	db, err := database.NewPostgres(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := database.MigratePostgres(ctx, db, migrations.SQL+";"+outbox.Schema); err != nil {
		log.Fatalf("failed to run migration: %v", err)
	}

	producer := broker.NewProducer(os.Getenv("KAFKA_BROKERS"))
	defer producer.Close()

	repo := repository.New(db)
	svc := service.New(repo)
	go outbox.NewRelay(db, producer, "likes-service").Run(ctx)

	// gRPC server
	grpcPort := os.Getenv("GRPC_PORT")
	if grpcPort == "" {
		grpcPort = "9084"
	}
	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		log.Fatalf("failed to listen on grpc port: %v", err)
	}
	grpcSrv := grpc.NewServer(observability.GRPCServerOptions()...)
	likespb.RegisterLikesServiceServer(grpcSrv, grpcserver.New(svc, func(ctx context.Context) error {
		_, err := db.Exec(ctx, "TRUNCATE likes, outbox")
		return err
	}))
	observability.InitGRPCMetrics(grpcSrv)
	go func() {
		log.Printf("gRPC server listening on :%s", grpcPort)
		if err := grpcSrv.Serve(lis); err != nil {
			log.Fatalf("gRPC server failed: %v", err)
		}
	}()

	// HTTP server (health + metrics only)
	app := fiber.New(fiber.Config{AppName: "likes-service", DisableStartupMessage: true})
	prometheus := fiberprometheus.NewWithDefaultRegistry("likes-service")
	prometheus.RegisterAt(app, "/metrics")
	app.Use(prometheus.Middleware)

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8084"
	}

	go func() {
		if err := app.Listen(":" + port); err != nil {
			log.Fatalf("failed to start HTTP server: %v", err)
		}
	}()

	// Topics are created after /health is serving so a cold broker does not fail
	// the liveness probe. Publishes fail while Kafka is unreachable either way.
	go func() {
		if err := broker.EnsureTopics(ctx, os.Getenv("KAFKA_BROKERS"), events.LikeCreated, events.LikeDeleted); err != nil {
			log.Fatalf("failed to ensure kafka topics: %v", err)
		}
	}()

	<-ctx.Done()
	grpcSrv.GracefulStop()
	_ = app.Shutdown()
}
