package grpcserver

import (
	"context"

	"github.com/bradfitz/gomemcache/memcache"
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
	mc  *memcache.Client
}

func New(svc *service.Service, mc *memcache.Client) *Server {
	return &Server{svc: svc, mc: mc}
}

func (s *Server) GetHomeFeed(_ context.Context, req *feedpb.GetHomeFeedRequest) (*feedpb.GetFeedResponse, error) {
	items, err := s.svc.GetHomeFeed(req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get feed: %v", err)
	}
	return &feedpb.GetFeedResponse{Items: toProtoItems(items)}, nil
}

func (s *Server) GetUserFeed(_ context.Context, req *feedpb.GetUserFeedRequest) (*feedpb.GetFeedResponse, error) {
	items, err := s.svc.GetUserFeed(req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get feed: %v", err)
	}
	return &feedpb.GetFeedResponse{Items: toProtoItems(items)}, nil
}

func (s *Server) Reset(_ context.Context, _ *feedpb.ResetRequest) (*feedpb.ResetResponse, error) {
	s.mc.FlushAll()
	return &feedpb.ResetResponse{Status: "reset"}, nil
}

func toProtoItems(items []model.FeedItem) []*feedpb.FeedItem {
	out := make([]*feedpb.FeedItem, len(items))
	for i, item := range items {
		out[i] = &feedpb.FeedItem{
			PostId:        item.PostID,
			AuthorId:      item.AuthorID,
			Text:          item.Text,
			LikesCount:    int32(item.LikesCount),
			CommentsCount: int32(item.CommentsCount),
			ImageUrl:      item.ImageURL,
			CreatedAt:     timestamppb.New(item.CreatedAt),
		}
	}
	return out
}
