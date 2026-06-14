package main

import (
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/proxy"
)

func main() {
	app := fiber.New(fiber.Config{
		AppName: "gateway-service",
	})

	app.Use(logger.New())

	registerRoutes(app)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Fatal(app.Listen(":" + port))
}

func registerRoutes(app *fiber.App) {
	postsURL := envOrDefault("POSTS_SERVICE_URL", "http://localhost:8081")
	feedURL := envOrDefault("FEED_SERVICE_URL", "http://localhost:8082")
	commentsURL := envOrDefault("COMMENTS_SERVICE_URL", "http://localhost:8083")
	likesURL := envOrDefault("LIKES_SERVICE_URL", "http://localhost:8084")
	usersURL := envOrDefault("USERS_SERVICE_URL", "http://localhost:8085")
	mediaURL := envOrDefault("MEDIA_SERVICE_URL", "http://localhost:8086")

	app.All("/api/v1/posts/*", func(c *fiber.Ctx) error {
		return proxy.Forward(postsURL + c.Path())(c)
	})

	app.All("/api/v1/feed/*", func(c *fiber.Ctx) error {
		return proxy.Forward(feedURL + c.Path())(c)
	})

	app.All("/api/v1/comments/*", func(c *fiber.Ctx) error {
		return proxy.Forward(commentsURL + c.Path())(c)
	})

	app.All("/api/v1/likes/*", func(c *fiber.Ctx) error {
		return proxy.Forward(likesURL + c.Path())(c)
	})

	app.All("/api/v1/users/*", func(c *fiber.Ctx) error {
		return proxy.Forward(usersURL + c.Path())(c)
	})

	app.All("/api/v1/media/*", func(c *fiber.Ctx) error {
		return proxy.Forward(mediaURL + c.Path())(c)
	})

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})
}

func envOrDefault(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
