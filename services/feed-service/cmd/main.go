package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/vahan/distributed-social-network/feed-service/internal/consumer"
	"github.com/vahan/distributed-social-network/feed-service/internal/handler"
	"github.com/vahan/distributed-social-network/feed-service/internal/repository"
	"github.com/vahan/distributed-social-network/feed-service/internal/service"
	"github.com/vahan/distributed-social-network/pkg/cache"
	"github.com/vahan/distributed-social-network/pkg/database"

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

	redisClient := cache.NewRedis(os.Getenv("REDIS_URL"))
	defer redisClient.Close()

	repo := repository.New(db)
	svc := service.New(repo, redisClient)
	h := handler.New(svc)

	// start event consumer for fanout-on-write
	cons := consumer.New(svc, os.Getenv("KAFKA_BROKERS"))
	go cons.Start(ctx)

	app := fiber.New(fiber.Config{AppName: "feed-service"})
	app.Use(logger.New())

	api := app.Group("/api/v1/feed")
	api.Get("/home", h.GetHomeFeed)
	api.Get("/user/:user_id", h.GetUserFeed)

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}

	go func() {
		if err := app.Listen(":" + port); err != nil {
			log.Fatalf("failed to start server: %v", err)
		}
	}()

	<-ctx.Done()
	_ = app.Shutdown()
}
