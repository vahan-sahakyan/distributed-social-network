package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/vahan-sahakyan/distributed-social-network/pkg/database"
	"github.com/vahan-sahakyan/distributed-social-network/users-service/internal/handler"
	"github.com/vahan-sahakyan/distributed-social-network/users-service/internal/repository"
	"github.com/vahan-sahakyan/distributed-social-network/users-service/internal/service"

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
	h := handler.New(svc)

	app := fiber.New(fiber.Config{AppName: "users-service"})
	prometheus := fiberprometheus.NewWithDefaultRegistry("users-service")
	prometheus.RegisterAt(app, "/metrics")
	app.Use(prometheus.Middleware)
	app.Use(logger.New())

	api := app.Group("/api/v1/users")
	api.Post("/", h.CreateUser)
	api.Get("/by-username/:username", h.GetUserByUsername)
	api.Get("/:id", h.GetUser)
	api.Post("/:id/follow", h.FollowUser)
	api.Delete("/:id/follow", h.UnfollowUser)
	api.Get("/:id/followers", h.GetFollowers)
	api.Get("/:id/following", h.GetFollowing)

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8085"
	}

	go func() {
		if err := app.Listen(":" + port); err != nil {
			log.Fatalf("failed to start server: %v", err)
		}
	}()

	<-ctx.Done()
	_ = app.Shutdown()
}
