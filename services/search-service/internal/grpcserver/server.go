package grpcserver

import (
	"context"

	searchpb "github.com/vahan-sahakyan/distributed-social-network/pkg/grpc/search"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/validate"
	"github.com/vahan-sahakyan/distributed-social-network/search-service/internal/repository"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Server struct {
	searchpb.UnimplementedSearchServiceServer
	repo *repository.Repository
}

func New(repo *repository.Repository) *Server {
	return &Server{repo: repo}
}

// clamp returns def for an unset value and caps it at max.
func clamp(v int32, def, max int) int {
	if v <= 0 {
		return def
	}
	return min(int(v), max)
}

func (s *Server) SearchPosts(ctx context.Context, req *searchpb.SearchPostsRequest) (*searchpb.SearchPostsResponse, error) {
	if err := validate.Required("query", req.Query); err != nil {
		return nil, err
	}
	hits, total, err := s.repo.SearchPosts(ctx, req.Query, clamp(req.Limit, 20, 50))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "searching posts: %v", err)
	}
	resp := &searchpb.SearchPostsResponse{Total: total, Posts: make([]*searchpb.PostHit, len(hits))}
	for i, h := range hits {
		resp.Posts[i] = &searchpb.PostHit{
			Id:        h.ID,
			AuthorId:  h.AuthorID,
			Text:      h.Text,
			ImageId:   h.ImageID,
			Hashtags:  h.Hashtags,
			CreatedAt: timestamppb.New(h.CreatedAt),
			Highlight: h.Highlight,
			Score:     float32(h.Score),
		}
	}
	return resp, nil
}

func (s *Server) SearchUsers(ctx context.Context, req *searchpb.SearchUsersRequest) (*searchpb.SearchUsersResponse, error) {
	if err := validate.Required("query", req.Query); err != nil {
		return nil, err
	}
	hits, total, err := s.repo.SearchUsers(ctx, req.Query, clamp(req.Limit, 20, 50))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "searching users: %v", err)
	}
	resp := &searchpb.SearchUsersResponse{Total: total, Users: make([]*searchpb.UserHit, len(hits))}
	for i, h := range hits {
		resp.Users[i] = &searchpb.UserHit{
			Id:        h.ID,
			Username:  h.Username,
			Bio:       h.Bio,
			CreatedAt: timestamppb.New(h.CreatedAt),
			Score:     float32(h.Score),
		}
	}
	return resp, nil
}

func (s *Server) TrendingHashtags(ctx context.Context, req *searchpb.TrendingHashtagsRequest) (*searchpb.TrendingHashtagsResponse, error) {
	counts, err := s.repo.TrendingHashtags(ctx, clamp(req.Hours, 24, 24*30), clamp(req.Limit, 10, 50))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "trending hashtags: %v", err)
	}
	resp := &searchpb.TrendingHashtagsResponse{Hashtags: make([]*searchpb.HashtagCount, len(counts))}
	for i, c := range counts {
		resp.Hashtags[i] = &searchpb.HashtagCount{Hashtag: c.Hashtag, Posts: c.Posts}
	}
	return resp, nil
}

func (s *Server) Reset(ctx context.Context, _ *searchpb.ResetRequest) (*searchpb.ResetResponse, error) {
	if err := s.repo.Reset(ctx); err != nil {
		return nil, status.Errorf(codes.Internal, "reset failed: %v", err)
	}
	return &searchpb.ResetResponse{Status: "reset"}, nil
}
