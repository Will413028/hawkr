package grpc

import (
	"context"
	"testing"

	pb "github.com/Will413028/hawkr/go-pkg/proto/notification"
	"github.com/Will413028/hawkr/notification-service/internal/domain"
	"github.com/Will413028/hawkr/notification-service/internal/usecase"
)

type inboxCapturingRepo struct {
	gotUserID   int64
	gotUserType string
}

func (r *inboxCapturingRepo) Create(_ context.Context, _ *domain.InboxItem) (int64, error) {
	return 0, nil
}
func (r *inboxCapturingRepo) ListByUser(_ context.Context, userID int64, userType string, _ int64, _ int32, _ string) ([]*domain.InboxItem, error) {
	r.gotUserID, r.gotUserType = userID, userType
	return nil, nil
}
func (r *inboxCapturingRepo) CountByUser(_ context.Context, userID int64, userType string) (int64, error) {
	r.gotUserID, r.gotUserType = userID, userType
	return 0, nil
}
func (r *inboxCapturingRepo) CountUnread(_ context.Context, userID int64, userType string) (int64, error) {
	r.gotUserID, r.gotUserType = userID, userType
	return 0, nil
}
func (r *inboxCapturingRepo) MarkAsRead(_ context.Context, _, userID int64, userType string) (int64, error) {
	r.gotUserID, r.gotUserType = userID, userType
	return 0, nil
}
func (r *inboxCapturingRepo) MarkAllAsRead(_ context.Context, userID int64, userType string) (int64, error) {
	r.gotUserID, r.gotUserType = userID, userType
	return 0, nil
}
func (r *inboxCapturingRepo) SoftDeleteByIDs(_ context.Context, _ []int64, userID int64, userType string) (int64, error) {
	r.gotUserID, r.gotUserType = userID, userType
	return 0, nil
}
func (r *inboxCapturingRepo) RedactChatBySender(_ context.Context, _ []int64) (int64, error) {
	return 0, nil
}
func (r *inboxCapturingRepo) SoftDeleteByHistoricalBuyerIDs(_ context.Context, _ []int64) (int64, error) {
	return 0, nil
}
func (r *inboxCapturingRepo) SoftDeleteByHistoricalStoreIDs(_ context.Context, _ []int64) (int64, error) {
	return 0, nil
}

func newInboxTestServer() (*Server, *inboxCapturingRepo) {
	repo := &inboxCapturingRepo{}
	uc := usecase.NewNotificationInboxUseCase(repo, nil, nil, nil, nil)
	return &Server{inboxUC: uc}, repo
}

func TestInboxRPCsUseCanonicalUserID(t *testing.T) {
	tests := []struct {
		name string
		call func(*Server) error
	}{
		{
			name: "list",
			call: func(s *Server) error {
				_, err := s.ListNotifications(context.Background(), &pb.ListNotificationsRequest{
					UserId: "7001", UserType: "seller",
				})
				return err
			},
		},
		{
			name: "unread count",
			call: func(s *Server) error {
				_, err := s.GetUnreadCount(context.Background(), &pb.GetUnreadCountRequest{
					UserId: "7001", UserType: "seller",
				})
				return err
			},
		},
		{
			name: "mark as read",
			call: func(s *Server) error {
				_, err := s.MarkAsRead(context.Background(), &pb.MarkAsReadRequest{
					Id: "1", UserId: "7001", UserType: "seller",
				})
				return err
			},
		},
		{
			name: "mark all as read",
			call: func(s *Server) error {
				_, err := s.MarkAllAsRead(context.Background(), &pb.MarkAllAsReadRequest{
					UserId: "7001", UserType: "seller",
				})
				return err
			},
		},
		{
			name: "delete",
			call: func(s *Server) error {
				_, err := s.DeleteNotifications(context.Background(), &pb.DeleteNotificationsRequest{
					Ids: []string{"1"}, UserId: "7001", UserType: "seller",
				})
				return err
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, repo := newInboxTestServer()
			if err := tc.call(s); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if repo.gotUserID != 7001 || repo.gotUserType != "seller" {
				t.Fatalf("repo identity=(%d,%q), want (7001,seller)", repo.gotUserID, repo.gotUserType)
			}
		})
	}
}
