package grpcserver

import (
	"context"

	"github.com/vahan-sahakyan/distributed-social-network/comments-service/internal/model"
	"github.com/vahan-sahakyan/distributed-social-network/comments-service/internal/service"
	commentspb "github.com/vahan-sahakyan/distributed-social-network/pkg/grpc/comments"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Server struct {
	commentspb.UnimplementedCommentsServiceServer
	svc     *service.Service
	resetFn func(ctx context.Context) error
}

func New(svc *service.Service, resetFn func(ctx context.Context) error) *Server {
	return &Server{svc: svc, resetFn: resetFn}
}

func (s *Server) CreateComment(ctx context.Context, req *commentspb.CreateCommentRequest) (*commentspb.CreateCommentResponse, error) {
	comment, err := s.svc.CreateComment(ctx, &model.CreateCommentRequest{
		UserID:   req.UserId,
		EntityID: req.EntityId,
		Text:     req.Text,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create comment: %v", err)
	}
	return &commentspb.CreateCommentResponse{Comment: toProto(comment)}, nil
}

func (s *Server) GetComments(ctx context.Context, req *commentspb.GetCommentsRequest) (*commentspb.GetCommentsResponse, error) {
	comments, err := s.svc.GetCommentsByEntity(ctx, req.EntityId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get comments: %v", err)
	}
	pbComments := make([]*commentspb.Comment, len(comments))
	for i, c := range comments {
		pbComments[i] = toProto(&c)
	}
	return &commentspb.GetCommentsResponse{Comments: pbComments}, nil
}

func (s *Server) Reset(ctx context.Context, _ *commentspb.ResetRequest) (*commentspb.ResetResponse, error) {
	if err := s.resetFn(ctx); err != nil {
		return nil, status.Errorf(codes.Internal, "reset failed: %v", err)
	}
	return &commentspb.ResetResponse{Status: "reset"}, nil
}

func toProto(c *model.Comment) *commentspb.Comment {
	return &commentspb.Comment{
		Id:        c.ID,
		UserId:    c.UserID,
		EntityId:  c.EntityID,
		Text:      c.Text,
		Likes:     int32(c.Likes),
		CreatedAt: timestamppb.New(c.CreatedAt),
	}
}
