package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	grpcserver "github.com/vahan-sahakyan/distributed-social-network/cache-rebuilder-service/internal/grpcserver"
	"github.com/vahan-sahakyan/distributed-social-network/cache-rebuilder-service/internal/repository"
	"github.com/vahan-sahakyan/distributed-social-network/cache-rebuilder-service/internal/service"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/cache"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/database"
	cacherebpb "github.com/vahan-sahakyan/distributed-social-network/pkg/grpc/cache_rebuilder"
	postspb "github.com/vahan-sahakyan/distributed-social-network/pkg/grpc/posts"
	userspb "github.com/vahan-sahakyan/distributed-social-network/pkg/grpc/users"

	"github.com/ansrivas/fiberprometheus/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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
	mc := cache.NewMemcached(ctx, memcachedAddr)

	// gRPC clients for upstream services
	usersAddr := os.Getenv("USERS_SERVICE_GRPC_ADDR")
	if usersAddr == "" {
		usersAddr = "localhost:9085"
	}
	postsAddr := os.Getenv("POSTS_SERVICE_GRPC_ADDR")
	if postsAddr == "" {
		postsAddr = "localhost:9081"
	}

	usersConn, err := grpc.NewClient(usersAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to connect to users-service: %v", err)
	}
	defer usersConn.Close()

	postsConn, err := grpc.NewClient(postsAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to connect to posts-service: %v", err)
	}
	defer postsConn.Close()

	repo := repository.New(conn)
	svc := service.New(repo, mc, userspb.NewUsersServiceClient(usersConn), postspb.NewPostsServiceClient(postsConn))

	// gRPC server
	grpcPort := os.Getenv("GRPC_PORT")
	if grpcPort == "" {
		grpcPort = "9089"
	}
	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		log.Fatalf("failed to listen on grpc port: %v", err)
	}
	grpcSrv := grpc.NewServer()
	cacherebpb.RegisterCacheRebuilderServiceServer(grpcSrv, grpcserver.New(svc, func(ctx context.Context) error {
		conn.Exec(ctx, "TRUNCATE TABLE IF EXISTS feed_events")
		conn.Exec(ctx, "TRUNCATE TABLE IF EXISTS current_post_state")
		mc.FlushAll()
		return nil
	}))
	go func() {
		log.Printf("gRPC server listening on :%s", grpcPort)
		if err := grpcSrv.Serve(lis); err != nil {
			log.Fatalf("gRPC server failed: %v", err)
		}
	}()

	// HTTP server (health + metrics only)
	app := fiber.New(fiber.Config{AppName: "cache-rebuilder-service"})
	prometheus := fiberprometheus.NewWithDefaultRegistry("cache-rebuilder-service")
	prometheus.RegisterAt(app, "/metrics")
	app.Use(prometheus.Middleware)
	app.Use(logger.New())

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8089"
	}

	go func() {
		if err := app.Listen(":" + port); err != nil {
			log.Fatalf("failed to start HTTP server: %v", err)
		}
	}()

	<-ctx.Done()
	grpcSrv.GracefulStop()
	_ = app.Shutdown()
}
