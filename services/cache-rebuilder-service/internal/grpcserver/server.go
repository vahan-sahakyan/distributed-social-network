package grpcserver

import (
	"context"

	"github.com/vahan-sahakyan/distributed-social-network/cache-rebuilder-service/internal/service"
	cacherebpb "github.com/vahan-sahakyan/distributed-social-network/pkg/grpc/cache_rebuilder"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	cacherebpb.UnimplementedCacheRebuilderServiceServer
	svc     *service.Service
	resetFn func(ctx context.Context) error
}

func New(svc *service.Service, resetFn func(ctx context.Context) error) *Server {
	return &Server{svc: svc, resetFn: resetFn}
}

func (s *Server) TriggerRebuild(ctx context.Context, req *cacherebpb.TriggerRebuildRequest) (*cacherebpb.TriggerRebuildResponse, error) {
	if req.UserId != "" {
		if err := s.svc.RebuildUserFeed(ctx, req.UserId); err != nil {
			return nil, status.Errorf(codes.Internal, "rebuild failed: %v", err)
		}
		return &cacherebpb.TriggerRebuildResponse{Status: "user feed rebuilt"}, nil
	}
	if err := s.svc.RebuildCache(ctx); err != nil {
		return nil, status.Errorf(codes.Internal, "rebuild failed: %v", err)
	}
	return &cacherebpb.TriggerRebuildResponse{Status: "rebuild complete"}, nil
}

func (s *Server) Reset(ctx context.Context, _ *cacherebpb.ResetRequest) (*cacherebpb.ResetResponse, error) {
	if err := s.resetFn(ctx); err != nil {
		return nil, status.Errorf(codes.Internal, "reset failed: %v", err)
	}
	return &cacherebpb.ResetResponse{Status: "reset"}, nil
}
