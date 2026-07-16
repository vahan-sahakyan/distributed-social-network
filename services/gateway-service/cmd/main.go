package main

import (
	"io"
	"log"
	"os"

	cacherebpb "github.com/vahan-sahakyan/distributed-social-network/pkg/grpc/cache_rebuilder"
	commentspb "github.com/vahan-sahakyan/distributed-social-network/pkg/grpc/comments"
	feedpb "github.com/vahan-sahakyan/distributed-social-network/pkg/grpc/feed"
	likespb "github.com/vahan-sahakyan/distributed-social-network/pkg/grpc/likes"
	mediapb "github.com/vahan-sahakyan/distributed-social-network/pkg/grpc/media"
	notificationspb "github.com/vahan-sahakyan/distributed-social-network/pkg/grpc/notifications"
	postspb "github.com/vahan-sahakyan/distributed-social-network/pkg/grpc/posts"
	userspb "github.com/vahan-sahakyan/distributed-social-network/pkg/grpc/users"

	"github.com/ansrivas/fiberprometheus/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type clients struct {
	users          userspb.UsersServiceClient
	posts          postspb.PostsServiceClient
	comments       commentspb.CommentsServiceClient
	likes          likespb.LikesServiceClient
	feed           feedpb.FeedServiceClient
	media          mediapb.MediaServiceClient
	notifications  notificationspb.NotificationServiceClient
	cacheRebuilder cacherebpb.CacheRebuilderServiceClient
}

func grpcErrStatus(c *fiber.Ctx, err error) error {
	if st, ok := status.FromError(err); ok {
		switch st.Code() {
		case codes.NotFound:
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": st.Message()})
		case codes.InvalidArgument:
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": st.Message()})
		case codes.AlreadyExists:
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": st.Message()})
		}
	}
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
}

func mustDial(addr string) *grpc.ClientConn {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to dial %s: %v", addr, err)
	}
	return conn
}

func main() {
	app := fiber.New(fiber.Config{
		AppName:   "gateway-service",
		BodyLimit: 60 * 1024 * 1024, // 60MB for media uploads
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

	cl := &clients{
		users:          userspb.NewUsersServiceClient(mustDial(envOrDefault("USERS_SERVICE_GRPC_ADDR", "localhost:9085"))),
		posts:          postspb.NewPostsServiceClient(mustDial(envOrDefault("POSTS_SERVICE_GRPC_ADDR", "localhost:9081"))),
		comments:       commentspb.NewCommentsServiceClient(mustDial(envOrDefault("COMMENTS_SERVICE_GRPC_ADDR", "localhost:9083"))),
		likes:          likespb.NewLikesServiceClient(mustDial(envOrDefault("LIKES_SERVICE_GRPC_ADDR", "localhost:9084"))),
		feed:           feedpb.NewFeedServiceClient(mustDial(envOrDefault("FEED_SERVICE_GRPC_ADDR", "localhost:9082"))),
		media:          mediapb.NewMediaServiceClient(mustDial(envOrDefault("MEDIA_SERVICE_GRPC_ADDR", "localhost:9086"))),
		notifications:  notificationspb.NewNotificationServiceClient(mustDial(envOrDefault("NOTIFICATIONS_SERVICE_GRPC_ADDR", "localhost:9087"))),
		cacheRebuilder: cacherebpb.NewCacheRebuilderServiceClient(mustDial(envOrDefault("CACHE_REBUILDER_SERVICE_GRPC_ADDR", "localhost:9089"))),
	}

	registerRoutes(app, cl)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Fatal(app.Listen(":" + port))
}

func registerRoutes(app *fiber.App, cl *clients) {
	// --- users ---
	app.Post("/api/v1/users/", func(c *fiber.Ctx) error {
		var req userspb.CreateUserRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
		}
		resp, err := cl.users.CreateUser(c.Context(), &req)
		if err != nil {
			return grpcErrStatus(c, err)
		}
		return c.Status(fiber.StatusCreated).JSON(resp.User)
	})
	app.Get("/api/v1/users/by-username/:username", func(c *fiber.Ctx) error {
		resp, err := cl.users.GetUserByUsername(c.Context(), &userspb.GetUserByUsernameRequest{Username: c.Params("username")})
		if err != nil {
			return grpcErrStatus(c, err)
		}
		return c.JSON(resp.User)
	})
	app.Get("/api/v1/users/:id", func(c *fiber.Ctx) error {
		resp, err := cl.users.GetUser(c.Context(), &userspb.GetUserRequest{Id: c.Params("id")})
		if err != nil {
			return grpcErrStatus(c, err)
		}
		return c.JSON(resp.User)
	})
	app.Post("/api/v1/users/:id/follow", func(c *fiber.Ctx) error {
		var req userspb.FollowUserRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
		}
		req.TargetId = c.Params("id")
		if _, err := cl.users.FollowUser(c.Context(), &req); err != nil {
			return grpcErrStatus(c, err)
		}
		return c.SendStatus(fiber.StatusNoContent)
	})
	app.Delete("/api/v1/users/:id/follow", func(c *fiber.Ctx) error {
		var req userspb.UnfollowUserRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
		}
		req.TargetId = c.Params("id")
		if _, err := cl.users.UnfollowUser(c.Context(), &req); err != nil {
			return grpcErrStatus(c, err)
		}
		return c.SendStatus(fiber.StatusNoContent)
	})
	app.Get("/api/v1/users/:id/followers", func(c *fiber.Ctx) error {
		resp, err := cl.users.GetFollowers(c.Context(), &userspb.GetFollowersRequest{UserId: c.Params("id")})
		if err != nil {
			return grpcErrStatus(c, err)
		}
		return c.JSON(fiber.Map{"followers": resp.Followers})
	})
	app.Get("/api/v1/users/:id/following", func(c *fiber.Ctx) error {
		resp, err := cl.users.GetFollowing(c.Context(), &userspb.GetFollowingRequest{UserId: c.Params("id")})
		if err != nil {
			return grpcErrStatus(c, err)
		}
		return c.JSON(fiber.Map{"following": resp.Following})
	})

	// --- posts ---
	app.Post("/api/v1/posts/", func(c *fiber.Ctx) error {
		var req postspb.CreatePostRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
		}
		resp, err := cl.posts.CreatePost(c.Context(), &req)
		if err != nil {
			return grpcErrStatus(c, err)
		}
		return c.Status(fiber.StatusCreated).JSON(resp.Post)
	})
	app.Get("/api/v1/posts/:id", func(c *fiber.Ctx) error {
		resp, err := cl.posts.GetPost(c.Context(), &postspb.GetPostRequest{Id: c.Params("id")})
		if err != nil {
			return grpcErrStatus(c, err)
		}
		return c.JSON(resp.Post)
	})

	// --- comments ---
	app.Post("/api/v1/comments/", func(c *fiber.Ctx) error {
		var req commentspb.CreateCommentRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
		}
		resp, err := cl.comments.CreateComment(c.Context(), &req)
		if err != nil {
			return grpcErrStatus(c, err)
		}
		return c.Status(fiber.StatusCreated).JSON(resp.Comment)
	})
	app.Get("/api/v1/comments/entity/:entity_id", func(c *fiber.Ctx) error {
		resp, err := cl.comments.GetComments(c.Context(), &commentspb.GetCommentsRequest{EntityId: c.Params("entity_id")})
		if err != nil {
			return grpcErrStatus(c, err)
		}
		return c.JSON(resp.Comments)
	})

	// --- likes ---
	app.Post("/api/v1/likes/", func(c *fiber.Ctx) error {
		var req likespb.CreateLikeRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
		}
		resp, err := cl.likes.CreateLike(c.Context(), &req)
		if err != nil {
			return grpcErrStatus(c, err)
		}
		return c.Status(fiber.StatusCreated).JSON(resp.Like)
	})
	app.Get("/api/v1/likes/check", func(c *fiber.Ctx) error {
		resp, err := cl.likes.HasLiked(c.Context(), &likespb.HasLikedRequest{
			UserId:   c.Query("user_id"),
			EntityId: c.Query("entity_id"),
		})
		if err != nil {
			return grpcErrStatus(c, err)
		}
		return c.JSON(fiber.Map{"liked": resp.Liked})
	})
	app.Delete("/api/v1/likes/", func(c *fiber.Ctx) error {
		var req likespb.UnlikeRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
		}
		if _, err := cl.likes.Unlike(c.Context(), &req); err != nil {
			return grpcErrStatus(c, err)
		}
		return c.SendStatus(fiber.StatusNoContent)
	})

	// --- feed ---
	app.Get("/api/v1/feed/home", func(c *fiber.Ctx) error {
		resp, err := cl.feed.GetHomeFeed(c.Context(), &feedpb.GetHomeFeedRequest{UserId: c.Query("user_id")})
		if err != nil {
			return grpcErrStatus(c, err)
		}
		return c.JSON(resp.Items)
	})
	app.Get("/api/v1/feed/user/:user_id", func(c *fiber.Ctx) error {
		resp, err := cl.feed.GetUserFeed(c.Context(), &feedpb.GetUserFeedRequest{UserId: c.Params("user_id")})
		if err != nil {
			return grpcErrStatus(c, err)
		}
		return c.JSON(resp.Items)
	})

	// --- media ---
	app.Post("/api/v1/media/upload", func(c *fiber.Ctx) error {
		file, err := c.FormFile("file")
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "file required"})
		}
		f, err := file.Open()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to read file"})
		}
		defer f.Close()

		stream, err := cl.media.Upload(c.Context())
		if err != nil {
			return grpcErrStatus(c, err)
		}
		if err := stream.Send(&mediapb.UploadChunk{
			Content: &mediapb.UploadChunk_Meta{
				Meta: &mediapb.UploadMeta{
					Filename:    file.Filename,
					ContentType: file.Header.Get("Content-Type"),
					Size:        file.Size,
				},
			},
		}); err != nil {
			return grpcErrStatus(c, err)
		}
		buf := make([]byte, 32*1024)
		for {
			n, readErr := f.Read(buf)
			if n > 0 {
				if err := stream.Send(&mediapb.UploadChunk{
					Content: &mediapb.UploadChunk_ChunkData{ChunkData: buf[:n]},
				}); err != nil {
					return grpcErrStatus(c, err)
				}
			}
			if readErr == io.EOF {
				break
			}
			if readErr != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "read error"})
			}
		}
		resp, err := stream.CloseAndRecv()
		if err != nil {
			return grpcErrStatus(c, err)
		}
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{"id": resp.Id, "url": resp.Url})
	})
	app.Get("/api/v1/media/:id", func(c *fiber.Ctx) error {
		resp, err := cl.media.GetURL(c.Context(), &mediapb.GetURLRequest{Id: c.Params("id")})
		if err != nil {
			return grpcErrStatus(c, err)
		}
		return c.JSON(fiber.Map{"id": resp.Id, "url": resp.Url})
	})

	// --- notifications ---
	app.Get("/api/v1/notifications/:user_id", func(c *fiber.Ctx) error {
		resp, err := cl.notifications.GetNotifications(c.Context(), &notificationspb.GetNotificationsRequest{UserId: c.Params("user_id")})
		if err != nil {
			return grpcErrStatus(c, err)
		}
		return c.JSON(fiber.Map{"notifications": resp.Notifications})
	})

	// --- cache rebuild ---
	app.Post("/api/v1/rebuild", func(c *fiber.Ctx) error {
		resp, err := cl.cacheRebuilder.TriggerRebuild(c.Context(), &cacherebpb.TriggerRebuildRequest{
			UserId: c.Query("user_id"),
		})
		if err != nil {
			return grpcErrStatus(c, err)
		}
		return c.JSON(fiber.Map{"status": resp.Status})
	})

	// --- reset (dev only) ---
	app.Post("/api/v1/reset", func(c *fiber.Ctx) error {
		ctx := c.Context()
		cl.users.Reset(ctx, &userspb.ResetRequest{})
		cl.posts.Reset(ctx, &postspb.ResetRequest{})
		cl.comments.Reset(ctx, &commentspb.ResetRequest{})
		cl.likes.Reset(ctx, &likespb.ResetRequest{})
		cl.notifications.Reset(ctx, &notificationspb.ResetRequest{})
		cl.feed.Reset(ctx, &feedpb.ResetRequest{})
		cl.cacheRebuilder.Reset(ctx, &cacherebpb.ResetRequest{})
		return c.JSON(fiber.Map{"status": "reset complete"})
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
