package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/vahan-sahakyan/distributed-social-network/event-writer-service/internal/consumer"
	"github.com/vahan-sahakyan/distributed-social-network/event-writer-service/internal/repository"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/database"

	"github.com/gofiber/fiber/v2"
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

	repo := repository.New(conn)
	cons := consumer.New(repo, os.Getenv("KAFKA_BROKERS"))

	go cons.Start(ctx)

	// Health endpoint
	app := fiber.New(fiber.Config{AppName: "event-writer-service"})
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8088"
	}

	go func() {
		if err := app.Listen(":" + port); err != nil {
			log.Fatalf("failed to start server: %v", err)
		}
	}()

	<-ctx.Done()
	_ = app.Shutdown()
}
