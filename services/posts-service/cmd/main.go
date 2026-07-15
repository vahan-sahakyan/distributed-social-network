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
	postspb "github.com/vahan-sahakyan/distributed-social-network/pkg/grpc/posts"
	grpcserver "github.com/vahan-sahakyan/distributed-social-network/posts-service/internal/grpcserver"
	"github.com/vahan-sahakyan/distributed-social-network/posts-service/internal/repository"
	"github.com/vahan-sahakyan/distributed-social-network/posts-service/internal/service"

	"github.com/ansrivas/fiberprometheus/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"google.golang.org/grpc"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	scyllaHosts := os.Getenv("SCYLLA_HOSTS")
	if scyllaHosts == "" {
		scyllaHosts = "localhost:9042"
	}
	scyllaKeyspace := os.Getenv("SCYLLA_KEYSPACE")
	if scyllaKeyspace == "" {
		scyllaKeyspace = "posts"
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
	grpcSrv := grpc.NewServer()
	postspb.RegisterPostsServiceServer(grpcSrv, grpcserver.New(svc, func(_ context.Context) error {
		db.Query("TRUNCATE posts").Exec()
		return nil
	}))
	go func() {
		log.Printf("gRPC server listening on :%s", grpcPort)
		if err := grpcSrv.Serve(lis); err != nil {
			log.Fatalf("gRPC server failed: %v", err)
		}
	}()

	// HTTP server (health + metrics only)
	app := fiber.New(fiber.Config{AppName: "posts-service"})
	prometheus := fiberprometheus.NewWithDefaultRegistry("posts-service")
	prometheus.RegisterAt(app, "/metrics")
	app.Use(prometheus.Middleware)
	app.Use(logger.New())

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

	<-ctx.Done()
	grpcSrv.GracefulStop()
	_ = app.Shutdown()
}
