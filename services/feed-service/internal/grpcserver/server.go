package grpcserver

import (
	"context"

	"github.com/vahan-sahakyan/distributed-social-network/feed-service/internal/model"
	"github.com/vahan-sahakyan/distributed-social-network/feed-service/internal/service"
	feedpb "github.com/vahan-sahakyan/distributed-social-network/pkg/grpc/feed"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Server struct {
	feedpb.UnimplementedFeedServiceServer
	svc *service.Service
}

func New(svc *service.Service) *Server {
	return &Server{svc: svc}
}

func (s *Server) GetHomeFeed(ctx context.Context, req *feedpb.GetHomeFeedRequest) (*feedpb.GetFeedResponse, error) {
	items, err := s.svc.GetHomeFeed(ctx, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get feed: %v", err)
	}
	return &feedpb.GetFeedResponse{Items: toProtoItems(items)}, nil
}

func (s *Server) GetUserFeed(ctx context.Context, req *feedpb.GetUserFeedRequest) (*feedpb.GetFeedResponse, error) {
	items, err := s.svc.GetUserFeed(ctx, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get feed: %v", err)
	}
	return &feedpb.GetFeedResponse{Items: toProtoItems(items)}, nil
}

func (s *Server) Reset(ctx context.Context, _ *feedpb.ResetRequest) (*feedpb.ResetResponse, error) {
	if err := s.svc.Reset(ctx); err != nil {
		return nil, status.Errorf(codes.Internal, "reset failed: %v", err)
	}
	return &feedpb.ResetResponse{Status: "reset"}, nil
}

func toProtoItems(items []model.FeedItem) []*feedpb.FeedItem {
	out := make([]*feedpb.FeedItem, len(items))
	for i, item := range items {
		out[i] = &feedpb.FeedItem{
			PostId:   item.PostID,
			AuthorId: item.AuthorID,
			Text:     item.Text,
			// a transient negative while an unlike overtook its like
			LikesCount:    int32(max(item.LikesCount, 0)),
			CommentsCount: int32(max(item.CommentsCount, 0)),
			ImageUrl:      item.ImageURL,
			CreatedAt:     timestamppb.New(item.CreatedAt),
		}
	}
	return out
}
