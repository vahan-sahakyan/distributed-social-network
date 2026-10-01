package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	grpcserver "github.com/vahan-sahakyan/distributed-social-network/media-service/internal/grpcserver"
	"github.com/vahan-sahakyan/distributed-social-network/media-service/internal/service"
	"github.com/vahan-sahakyan/distributed-social-network/media-service/internal/storage"
	mediapb "github.com/vahan-sahakyan/distributed-social-network/pkg/grpc/media"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/observability"

	"github.com/ansrivas/fiberprometheus/v2"
	"github.com/gofiber/fiber/v2"
	"google.golang.org/grpc"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	shutdown := observability.Init(ctx, "media-service")
	defer shutdown()

	store, err := storage.NewMinio(
		os.Getenv("MINIO_ENDPOINT"),
		os.Getenv("MINIO_ACCESS_KEY"),
		os.Getenv("MINIO_SECRET_KEY"),
		os.Getenv("MINIO_BUCKET"),
	)
	if err != nil {
		log.Fatalf("failed to initialize storage: %v", err)
	}

	svc := service.New(store)

	// gRPC server
	grpcPort := os.Getenv("GRPC_PORT")
	if grpcPort == "" {
		grpcPort = "9086"
	}
	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		log.Fatalf("failed to listen on grpc port: %v", err)
	}
	grpcSrv := grpc.NewServer(append(observability.GRPCServerOptions(), grpc.MaxRecvMsgSize(60*1024*1024))...) // 60MB max
	mediapb.RegisterMediaServiceServer(grpcSrv, grpcserver.New(svc))
	observability.InitGRPCMetrics(grpcSrv)
	go func() {
		log.Printf("gRPC server listening on :%s", grpcPort)
		if err := grpcSrv.Serve(lis); err != nil {
			log.Fatalf("gRPC server failed: %v", err)
		}
	}()

	// HTTP server (health + metrics only)
	app := fiber.New(fiber.Config{AppName: "media-service", DisableStartupMessage: true})
	prometheus := fiberprometheus.NewWithDefaultRegistry("media-service")
	prometheus.RegisterAt(app, "/metrics")
	app.Use(prometheus.Middleware)

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8086"
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
