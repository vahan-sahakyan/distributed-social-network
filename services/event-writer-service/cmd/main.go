package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/vahan-sahakyan/distributed-social-network/event-writer-service/internal/consumer"
	"github.com/vahan-sahakyan/distributed-social-network/event-writer-service/internal/repository"
	"github.com/vahan-sahakyan/distributed-social-network/event-writer-service/migrations"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/broker"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/database"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/events"

	"github.com/ansrivas/fiberprometheus/v2"
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

	if err := database.MigrateClickHouse(ctx, conn, migrations.SQL); err != nil {
		log.Fatalf("failed to run migration: %v", err)
	}

	repo := repository.New(conn)

	// Health endpoint
	app := fiber.New(fiber.Config{AppName: "event-writer-service"})
	prometheus := fiberprometheus.NewWithDefaultRegistry("event-writer-service")
	prometheus.RegisterAt(app, "/metrics")
	app.Use(prometheus.Middleware)
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

	// Kafka setup runs after /health is serving so a cold broker does not fail the liveness probe.
	go func() {
		brokers := os.Getenv("KAFKA_BROKERS")
		if err := broker.EnsureTopics(ctx, brokers, events.All...); err != nil {
			log.Fatalf("failed to ensure kafka topics: %v", err)
		}

		dlq := broker.NewProducer(brokers)
		defer dlq.Close()

		consumer.New(repo, brokers, dlq).Start(ctx)
	}()

	<-ctx.Done()
	_ = app.Shutdown()
}
