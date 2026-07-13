package main

import (
	"log"
	"os"

	"github.com/ansrivas/fiberprometheus/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/proxy"
)

func main() {
	app := fiber.New(fiber.Config{
		AppName: "gateway-service",
	})

	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,PUT,DELETE,PATCH,OPTIONS",
		AllowHeaders: "Content-Type,Authorization",
	}))

	app.Use(logger.New())

	prometheus := fiberprometheus.NewWithDefaultRegistry("gateway-service")
	prometheus.RegisterAt(app, "/metrics")
	app.Use(prometheus.Middleware)

	registerRoutes(app)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Fatal(app.Listen(":" + port))
}

func requestURI(c *fiber.Ctx) string {
	uri := c.Path()
	if qs := string(c.Request().URI().QueryString()); qs != "" {
		uri += "?" + qs
	}
	return uri
}

func registerRoutes(app *fiber.App) {
	postsURL := envOrDefault("POSTS_SERVICE_URL", "http://localhost:8081")
	feedURL := envOrDefault("FEED_SERVICE_URL", "http://localhost:8082")
	commentsURL := envOrDefault("COMMENTS_SERVICE_URL", "http://localhost:8083")
	likesURL := envOrDefault("LIKES_SERVICE_URL", "http://localhost:8084")
	usersURL := envOrDefault("USERS_SERVICE_URL", "http://localhost:8085")
	mediaURL := envOrDefault("MEDIA_SERVICE_URL", "http://localhost:8086")
	notificationsURL := envOrDefault("NOTIFICATIONS_SERVICE_URL", "http://localhost:8087")
	cacheRebuilderURL := envOrDefault("CACHE_REBUILDER_SERVICE_URL", "http://localhost:8089")

	app.All("/api/v1/posts/*", func(c *fiber.Ctx) error {
		return proxy.Forward(postsURL + requestURI(c))(c)
	})

	app.All("/api/v1/feed/*", func(c *fiber.Ctx) error {
		return proxy.Forward(feedURL + requestURI(c))(c)
	})

	app.All("/api/v1/comments/*", func(c *fiber.Ctx) error {
		return proxy.Forward(commentsURL + requestURI(c))(c)
	})

	app.All("/api/v1/likes/*", func(c *fiber.Ctx) error {
		return proxy.Forward(likesURL + requestURI(c))(c)
	})

	app.All("/api/v1/users/*", func(c *fiber.Ctx) error {
		return proxy.Forward(usersURL + requestURI(c))(c)
	})

	app.All("/api/v1/media/*", func(c *fiber.Ctx) error {
		return proxy.Forward(mediaURL + requestURI(c))(c)
	})

	app.All("/api/v1/notifications/*", func(c *fiber.Ctx) error {
		return proxy.Forward(notificationsURL + requestURI(c))(c)
	})

	app.All("/api/v1/rebuild", func(c *fiber.Ctx) error {
		return proxy.Forward(cacheRebuilderURL + requestURI(c))(c)
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
