package grpcserver

import (
	"context"

	postspb "github.com/vahan-sahakyan/distributed-social-network/pkg/grpc/posts"
	"github.com/vahan-sahakyan/distributed-social-network/posts-service/internal/model"
	"github.com/vahan-sahakyan/distributed-social-network/posts-service/internal/service"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Server struct {
	postspb.UnimplementedPostsServiceServer
	svc     *service.Service
	resetFn func(ctx context.Context) error
}

func New(svc *service.Service, resetFn func(ctx context.Context) error) *Server {
	return &Server{svc: svc, resetFn: resetFn}
}

func (s *Server) CreatePost(ctx context.Context, req *postspb.CreatePostRequest) (*postspb.CreatePostResponse, error) {
	post, err := s.svc.CreatePost(ctx, &model.CreatePostRequest{
		Text:     req.Text,
		AuthorID: req.AuthorId,
		ImageID:  req.ImageId,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create post: %v", err)
	}
	return &postspb.CreatePostResponse{Post: toProto(post)}, nil
}

func (s *Server) GetPost(ctx context.Context, req *postspb.GetPostRequest) (*postspb.GetPostResponse, error) {
	post, err := s.svc.GetPost(ctx, req.Id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "post not found")
	}
	return &postspb.GetPostResponse{Post: toProto(post)}, nil
}

func (s *Server) Reset(ctx context.Context, _ *postspb.ResetRequest) (*postspb.ResetResponse, error) {
	if err := s.resetFn(ctx); err != nil {
		return nil, status.Errorf(codes.Internal, "reset failed: %v", err)
	}
	return &postspb.ResetResponse{Status: "reset"}, nil
}

func toProto(p *model.Post) *postspb.Post {
	return &postspb.Post{
		Id:        p.ID,
		Text:      p.Text,
		AuthorId:  p.AuthorID,
		ImageId:   p.ImageID,
		Likes:     int32(p.Likes),
		Comments:  int32(p.Comments),
		CreatedAt: timestamppb.New(p.CreatedAt),
	}
}
