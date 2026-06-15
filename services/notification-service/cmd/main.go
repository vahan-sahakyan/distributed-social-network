package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/vahan-sahakyan/distributed-social-network/notification-service/internal/consumer"
	"github.com/vahan-sahakyan/distributed-social-network/notification-service/internal/handler"
	"github.com/vahan-sahakyan/distributed-social-network/notification-service/internal/repository"
	"github.com/vahan-sahakyan/distributed-social-network/notification-service/internal/service"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/database"

	"github.com/ansrivas/fiberprometheus/v2"
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

	repo := repository.New(db)
	svc := service.New(repo)

	// start event consumers
	cons := consumer.New(svc, os.Getenv("KAFKA_BROKERS"))
	go cons.Start(ctx)

	h := handler.New(svc)
	app := fiber.New(fiber.Config{AppName: "notification-service"})
	prometheus := fiberprometheus.NewWithDefaultRegistry("notification-service")
	prometheus.RegisterAt(app, "/metrics")
	app.Use(prometheus.Middleware)
	app.Use(logger.New())

	api := app.Group("/api/v1/notifications")
	api.Get("/:user_id", h.GetNotifications)

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8087"
	}

	go func() {
		if err := app.Listen(":" + port); err != nil {
			log.Fatalf("failed to start server: %v", err)
		}
	}()

	<-ctx.Done()
	_ = app.Shutdown()
}
