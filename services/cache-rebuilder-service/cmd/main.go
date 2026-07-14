package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/vahan-sahakyan/distributed-social-network/cache-rebuilder-service/internal/handler"
	"github.com/vahan-sahakyan/distributed-social-network/cache-rebuilder-service/internal/repository"
	"github.com/vahan-sahakyan/distributed-social-network/cache-rebuilder-service/internal/service"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/cache"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/database"

	"github.com/ansrivas/fiberprometheus/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	chAddr := os.Getenv("CLICKHOUSE_ADDR")
	if chAddr == "" {
		chAddr = "localhost:9000"
	}
	chDB := os.Getenv("CLICKHOUSE_DB")
	if chDB == "" {
		chDB = "default"
	}

	conn, err := database.NewClickHouse(ctx, chAddr, chDB)
	if err != nil {
		log.Fatalf("failed to connect to clickhouse: %v", err)
	}
	defer conn.Close()

	memcachedAddr := os.Getenv("MEMCACHED_ADDR")
	if memcachedAddr == "" {
		memcachedAddr = "localhost:11211"
	}
	mc := cache.NewMemcached(memcachedAddr)

	usersURL := os.Getenv("USERS_SERVICE_URL")
	if usersURL == "" {
		usersURL = "http://localhost:8085"
	}
	postsURL := os.Getenv("POSTS_SERVICE_URL")
	if postsURL == "" {
		postsURL = "http://localhost:8081"
	}

	repo := repository.New(conn)
	svc := service.New(repo, mc, usersURL, postsURL)
	h := handler.New(svc)

	app := fiber.New(fiber.Config{AppName: "cache-rebuilder-service"})
	prometheus := fiberprometheus.NewWithDefaultRegistry("cache-rebuilder-service")
	prometheus.RegisterAt(app, "/metrics")
	app.Use(prometheus.Middleware)
	app.Use(logger.New())

	app.Post("/api/v1/rebuild", h.TriggerRebuild)
	app.Post("/reset", func(c *fiber.Ctx) error {
		conn.Exec(c.Context(), "TRUNCATE TABLE IF EXISTS feed_events")
		conn.Exec(c.Context(), "TRUNCATE TABLE IF EXISTS current_post_state")
		mc.FlushAll()
		return c.JSON(fiber.Map{"status": "reset"})
	})
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8089"
	}

	go func() {
		if err := app.Listen(":" + port); err != nil {
			log.Fatalf("failed to start server: %v", err)
		}
	}()

	<-ctx.Done()
	_ = app.Shutdown()
}
