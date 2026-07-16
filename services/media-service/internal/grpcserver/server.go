package grpcserver

import (
	"bytes"
	"context"
	"io"

	"github.com/vahan-sahakyan/distributed-social-network/media-service/internal/service"
	mediapb "github.com/vahan-sahakyan/distributed-social-network/pkg/grpc/media"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	mediapb.UnimplementedMediaServiceServer
	svc *service.Service
}

func New(svc *service.Service) *Server {
	return &Server{svc: svc}
}

func (s *Server) Upload(stream mediapb.MediaService_UploadServer) error {
	// receive metadata from first message
	msg, err := stream.Recv()
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "failed to receive metadata: %v", err)
	}
	meta := msg.GetMeta()
	if meta == nil {
		return status.Error(codes.InvalidArgument, "first message must be metadata")
	}

	// read remaining chunks into a buffer
	var buf bytes.Buffer
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return status.Errorf(codes.Internal, "error receiving chunk: %v", err)
		}
		buf.Write(msg.GetChunkData())
	}

	img, err := s.svc.Upload(stream.Context(), &buf, meta.Size, meta.ContentType)
	if err != nil {
		return status.Errorf(codes.Internal, "upload failed: %v", err)
	}

	return stream.SendAndClose(&mediapb.UploadResponse{
		Id:  img.ID,
		Url: img.URL,
	})
}

func (s *Server) GetURL(ctx context.Context, req *mediapb.GetURLRequest) (*mediapb.GetURLResponse, error) {
	url, err := s.svc.GetURL(ctx, req.Id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "media not found")
	}
	return &mediapb.GetURLResponse{Id: req.Id, Url: url}, nil
}
