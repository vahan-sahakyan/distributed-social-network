package grpcserver

import (
	"context"

	userspb "github.com/vahan-sahakyan/distributed-social-network/pkg/grpc/users"
	"github.com/vahan-sahakyan/distributed-social-network/users-service/internal/model"
	"github.com/vahan-sahakyan/distributed-social-network/users-service/internal/service"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Server struct {
	userspb.UnimplementedUsersServiceServer
	svc     *service.Service
	resetFn func(ctx context.Context) error
}

func New(svc *service.Service, resetFn func(ctx context.Context) error) *Server {
	return &Server{svc: svc, resetFn: resetFn}
}

func (s *Server) CreateUser(ctx context.Context, req *userspb.CreateUserRequest) (*userspb.CreateUserResponse, error) {
	user, err := s.svc.CreateUser(ctx, &model.CreateUserRequest{
		Username: req.Username,
		Bio:      req.Bio,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create user: %v", err)
	}
	return &userspb.CreateUserResponse{User: toProto(user)}, nil
}

func (s *Server) GetUser(ctx context.Context, req *userspb.GetUserRequest) (*userspb.GetUserResponse, error) {
	user, err := s.svc.GetUser(ctx, req.Id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "user not found")
	}
	return &userspb.GetUserResponse{User: toProto(user)}, nil
}

func (s *Server) GetUserByUsername(ctx context.Context, req *userspb.GetUserByUsernameRequest) (*userspb.GetUserByUsernameResponse, error) {
	user, err := s.svc.GetUserByUsername(ctx, req.Username)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "user not found")
	}
	return &userspb.GetUserByUsernameResponse{User: toProto(user)}, nil
}

func (s *Server) FollowUser(ctx context.Context, req *userspb.FollowUserRequest) (*userspb.FollowUserResponse, error) {
	if err := s.svc.Follow(ctx, req.FollowerId, req.TargetId); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to follow: %v", err)
	}
	return &userspb.FollowUserResponse{}, nil
}

func (s *Server) UnfollowUser(ctx context.Context, req *userspb.UnfollowUserRequest) (*userspb.UnfollowUserResponse, error) {
	if err := s.svc.Unfollow(ctx, req.FollowerId, req.TargetId); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to unfollow: %v", err)
	}
	return &userspb.UnfollowUserResponse{}, nil
}

func (s *Server) GetFollowers(ctx context.Context, req *userspb.GetFollowersRequest) (*userspb.GetFollowersResponse, error) {
	followers, err := s.svc.GetFollowers(ctx, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get followers: %v", err)
	}
	return &userspb.GetFollowersResponse{Followers: followers}, nil
}

func (s *Server) GetFollowing(ctx context.Context, req *userspb.GetFollowingRequest) (*userspb.GetFollowingResponse, error) {
	following, err := s.svc.GetFollowing(ctx, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get following: %v", err)
	}
	return &userspb.GetFollowingResponse{Following: following}, nil
}

func (s *Server) Reset(ctx context.Context, _ *userspb.ResetRequest) (*userspb.ResetResponse, error) {
	if err := s.resetFn(ctx); err != nil {
		return nil, status.Errorf(codes.Internal, "reset failed: %v", err)
	}
	return &userspb.ResetResponse{Status: "reset"}, nil
}

func toProto(u *model.User) *userspb.User {
	return &userspb.User{
		Id:        u.ID,
		Username:  u.Username,
		Bio:       u.Bio,
		CreatedAt: timestamppb.New(u.CreatedAt),
	}
}
