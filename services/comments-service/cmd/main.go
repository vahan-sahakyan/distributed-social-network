package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	grpcserver "github.com/vahan-sahakyan/distributed-social-network/comments-service/internal/grpcserver"
	"github.com/vahan-sahakyan/distributed-social-network/comments-service/internal/repository"
	"github.com/vahan-sahakyan/distributed-social-network/comments-service/internal/service"
	"github.com/vahan-sahakyan/distributed-social-network/comments-service/migrations"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/broker"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/database"
	commentspb "github.com/vahan-sahakyan/distributed-social-network/pkg/grpc/comments"

	"github.com/ansrivas/fiberprometheus/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"google.golang.org/grpc"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	db, err := database.NewPostgres(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := database.MigratePostgres(ctx, db, migrations.SQL); err != nil {
		log.Fatalf("failed to run migration: %v", err)
	}

	producer := broker.NewProducer(os.Getenv("KAFKA_BROKERS"))
	defer producer.Close()

	repo := repository.New(db)
	svc := service.New(repo, producer)

	// gRPC server
	grpcPort := os.Getenv("GRPC_PORT")
	if grpcPort == "" {
		grpcPort = "9083"
	}
	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		log.Fatalf("failed to listen on grpc port: %v", err)
	}
	grpcSrv := grpc.NewServer()
	commentspb.RegisterCommentsServiceServer(grpcSrv, grpcserver.New(svc, func(ctx context.Context) error {
		_, err := db.Exec(ctx, "TRUNCATE comments")
		return err
	}))
	go func() {
		log.Printf("gRPC server listening on :%s", grpcPort)
		if err := grpcSrv.Serve(lis); err != nil {
			log.Fatalf("gRPC server failed: %v", err)
		}
	}()

	// HTTP server (health + metrics only)
	app := fiber.New(fiber.Config{AppName: "comments-service"})
	prometheus := fiberprometheus.NewWithDefaultRegistry("comments-service")
	prometheus.RegisterAt(app, "/metrics")
	app.Use(prometheus.Middleware)
	app.Use(logger.New())

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8083"
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
