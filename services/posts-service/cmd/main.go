package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/vahan/distributed-social-network/pkg/broker"
	"github.com/vahan/distributed-social-network/pkg/database"
	"github.com/vahan/distributed-social-network/posts-service/internal/handler"
	"github.com/vahan/distributed-social-network/posts-service/internal/repository"
	"github.com/vahan/distributed-social-network/posts-service/internal/service"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	db, err := database.NewPostgres(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	producer := broker.NewProducer(os.Getenv("KAFKA_BROKERS"))
	defer producer.Close()

	repo := repository.New(db)
	svc := service.New(repo, producer)
	h := handler.New(svc)

	app := fiber.New(fiber.Config{AppName: "posts-service"})
	app.Use(logger.New())

	api := app.Group("/api/v1/posts")
	api.Post("/", h.CreatePost)
	api.Get("/:id", h.GetPost)

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	go func() {
		if err := app.Listen(":" + port); err != nil {
			log.Fatalf("failed to start server: %v", err)
		}
	}()

	<-ctx.Done()
	_ = app.Shutdown()
}
