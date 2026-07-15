package grpcserver
package grpcserver

import (
	"context"

	notificationspb "github.com/vahan-sahakyan/distributed-social-network/pkg/grpc/notifications"
	"github.com/vahan-sahakyan/distributed-social-network/notification-service/internal/model"
	"github.com/vahan-sahakyan/distributed-social-network/notification-service/internal/service"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Server struct {
	notificationspb.UnimplementedNotificationServiceServer
	svc     *service.Service
	resetFn func(ctx context.Context) error
}

func New(svc *service.Service, resetFn func(ctx context.Context) error) *Server {
	return &Server{svc: svc, resetFn: resetFn}
}

func (s *Server) GetNotifications(ctx context.Context, req *notificationspb.GetNotificationsRequest) (*notificationspb.GetNotificationsResponse, error) {
	notifications, err := s.svc.GetByUserID(ctx, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get notifications: %v", err)
	}
	pbNotifs := make([]*notificationspb.Notification, len(notifications))
	for i, n := range notifications {
		pbNotifs[i] = toProto(&n)
	}
	return &notificationspb.GetNotificationsResponse{Notifications: pbNotifs}, nil
}

func (s *Server) Reset(ctx context.Context, _ *notificationspb.ResetRequest) (*notificationspb.ResetResponse, error) {
	if err := s.resetFn(ctx); err != nil {
		return nil, status.Errorf(codes.Internal, "reset failed: %v", err)
	}
	return &notificationspb.ResetResponse{Status: "reset"}, nil
}

func toProto(n *model.Notification) *notificationspb.Notification {
	return &notificationspb.Notification{
		Id:        n.ID,
		UserId:    n.UserID,
		Type:      n.Type,
		ActorId:   n.ActorID,
		EntityId:  n.EntityID,
		Read:      n.Read,
		CreatedAt: timestamppb.New(n.CreatedAt),
	}
}
