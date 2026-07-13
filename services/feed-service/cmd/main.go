package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/vahan-sahakyan/distributed-social-network/feed-service/internal/consumer"
	"github.com/vahan-sahakyan/distributed-social-network/feed-service/internal/handler"
	"github.com/vahan-sahakyan/distributed-social-network/feed-service/internal/repository"
	"github.com/vahan-sahakyan/distributed-social-network/feed-service/internal/service"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/cache"

	"github.com/ansrivas/fiberprometheus/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	memcachedAddr := os.Getenv("MEMCACHED_ADDR")
	if memcachedAddr == "" {
		memcachedAddr = "localhost:11211"
	}

	mc := cache.NewMemcached(memcachedAddr)

	repo := repository.New(mc)
	svc := service.New(repo)
	h := handler.New(svc)

	// start event consumer for fanout-on-write
	usersURL := os.Getenv("USERS_SERVICE_URL")
	if usersURL == "" {
		usersURL = "http://localhost:8085"
	}
	cons := consumer.New(svc, os.Getenv("KAFKA_BROKERS"), usersURL)
	go cons.Start(ctx)

	app := fiber.New(fiber.Config{AppName: "feed-service"})
	prometheus := fiberprometheus.NewWithDefaultRegistry("feed-service")
	prometheus.RegisterAt(app, "/metrics")
	app.Use(prometheus.Middleware)
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
