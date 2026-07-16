package grpcserver

import (
	"context"

	"github.com/vahan-sahakyan/distributed-social-network/likes-service/internal/model"
	"github.com/vahan-sahakyan/distributed-social-network/likes-service/internal/service"
	likespb "github.com/vahan-sahakyan/distributed-social-network/pkg/grpc/likes"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	likespb.UnimplementedLikesServiceServer
	svc     *service.Service
	resetFn func(ctx context.Context) error
}

func New(svc *service.Service, resetFn func(ctx context.Context) error) *Server {
	return &Server{svc: svc, resetFn: resetFn}
}

func (s *Server) CreateLike(ctx context.Context, req *likespb.CreateLikeRequest) (*likespb.CreateLikeResponse, error) {
	like, err := s.svc.CreateLike(ctx, &model.CreateLikeRequest{
		UserID:   req.UserId,
		EntityID: req.EntityId,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create like: %v", err)
	}
	return &likespb.CreateLikeResponse{Like: &likespb.Like{
		Id:       like.ID,
		UserId:   like.UserID,
		EntityId: like.EntityID,
	}}, nil
}

func (s *Server) HasLiked(ctx context.Context, req *likespb.HasLikedRequest) (*likespb.HasLikedResponse, error) {
	liked, err := s.svc.HasLiked(ctx, req.UserId, req.EntityId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to check like: %v", err)
	}
	return &likespb.HasLikedResponse{Liked: liked}, nil
}

func (s *Server) Unlike(ctx context.Context, req *likespb.UnlikeRequest) (*likespb.UnlikeResponse, error) {
	if err := s.svc.Unlike(ctx, req.UserId, req.EntityId); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to unlike: %v", err)
	}
	return &likespb.UnlikeResponse{}, nil
}

func (s *Server) Reset(ctx context.Context, _ *likespb.ResetRequest) (*likespb.ResetResponse, error) {
	if err := s.resetFn(ctx); err != nil {
		return nil, status.Errorf(codes.Internal, "reset failed: %v", err)
	}
	return &likespb.ResetResponse{Status: "reset"}, nil
}
