package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/vahan-sahakyan/distributed-social-network/notification-service/internal/consumer"
	grpcserver "github.com/vahan-sahakyan/distributed-social-network/notification-service/internal/grpcserver"
	"github.com/vahan-sahakyan/distributed-social-network/notification-service/internal/repository"
	"github.com/vahan-sahakyan/distributed-social-network/notification-service/internal/service"
	"github.com/vahan-sahakyan/distributed-social-network/notification-service/migrations"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/database"
	notificationspb "github.com/vahan-sahakyan/distributed-social-network/pkg/grpc/notifications"

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

	repo := repository.New(db)
	svc := service.New(repo)

	// start event consumers
	cons := consumer.New(svc, os.Getenv("KAFKA_BROKERS"))
	go cons.Start(ctx)

	// gRPC server
	grpcPort := os.Getenv("GRPC_PORT")
	if grpcPort == "" {
		grpcPort = "9087"
	}
	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		log.Fatalf("failed to listen on grpc port: %v", err)
	}
	grpcSrv := grpc.NewServer()
	notificationspb.RegisterNotificationServiceServer(grpcSrv, grpcserver.New(svc, func(ctx context.Context) error {
		_, err := db.Exec(ctx, "TRUNCATE notifications")
		return err
	}))
	go func() {
		log.Printf("gRPC server listening on :%s", grpcPort)
		if err := grpcSrv.Serve(lis); err != nil {
			log.Fatalf("gRPC server failed: %v", err)
		}
	}()

	// HTTP server (health + metrics only)
	app := fiber.New(fiber.Config{AppName: "notification-service"})
	prometheus := fiberprometheus.NewWithDefaultRegistry("notification-service")
	prometheus.RegisterAt(app, "/metrics")
	app.Use(prometheus.Middleware)
	app.Use(logger.New())

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8087"
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
