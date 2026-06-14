package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/vahan/distributed-social-network/media-service/internal/handler"
	"github.com/vahan/distributed-social-network/media-service/internal/service"
	"github.com/vahan/distributed-social-network/media-service/internal/storage"
	"github.com/vahan/distributed-social-network/pkg/broker"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	store, err := storage.NewMinio(
		os.Getenv("MINIO_ENDPOINT"),
		os.Getenv("MINIO_ACCESS_KEY"),
		os.Getenv("MINIO_SECRET_KEY"),
		os.Getenv("MINIO_BUCKET"),
	)
	if err != nil {
		log.Fatalf("failed to initialize storage: %v", err)
	}

	producer := broker.NewProducer(os.Getenv("KAFKA_BROKERS"))
	defer producer.Close()

	svc := service.New(store, producer)
	h := handler.New(svc)

	app := fiber.New(fiber.Config{
		AppName:   "media-service",
		BodyLimit: 50 * 1024 * 1024, // 50MB
	})
	app.Use(logger.New())

	api := app.Group("/api/v1/media")
	api.Post("/upload", h.Upload)
	api.Get("/:id", h.Get)

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8086"
	}

	go func() {
		if err := app.Listen(":" + port); err != nil {
			log.Fatalf("failed to start server: %v", err)
		}
	}()

	<-ctx.Done()
	_ = app.Shutdown()
}
