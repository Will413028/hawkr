// Package notification 實作 notification 的 Connect 傳輸。admin.go 是後台公告（NotificationAdminService）：欄位與錯誤路徑對應、列舉與時間轉成 proto；
// 驗證、冪等與派送都在 modules/notification，這裡不重做。
package notification

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/notification/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/notification/v1alpha1/notificationv1alpha1connect"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/notification/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/notification"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
)

// Announcements 是公告的來源與處理者（*notification.Module）。
type Announcements interface {
	CreateAnnouncement(ctx context.Context, in notification.CreateAnnouncementInput) (notification.Announcement, error)
	ListAnnouncements(ctx context.Context, after *notification.AnnouncementCursor, size int32) (notification.AnnouncementPage, error)
}

// Admin 實作 NotificationAdminService。授權（只有管理員、逐方法的權限、audited）由 rpc 的 interceptor 與 adminauth 的宣告表檢查。
type Admin struct{ m Announcements }

var _ notificationv1alpha1connect.NotificationAdminServiceHandler = (*Admin)(nil)

// NewAdmin 建立後台的 handler。
func NewAdmin(m Announcements) *Admin { return &Admin{m: m} }

var (
	createPaths = rpc.FieldPaths{"Audiences": "audiences", "Title": "title", "Body": "body", "RequestID": "request_id"}
	listPaths   = rpc.FieldPaths{"PageSize": "page_size"}
)

// CreateAnnouncement 建立一則公告並排入派送。
func (h *Admin) CreateAnnouncement(ctx context.Context, req *connect.Request[adminpb.CreateAnnouncementRequest]) (*connect.Response[adminpb.CreateAnnouncementResponse], error) {
	admin, err := rpc.RequireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	requestID, err := rpc.ParseRequestID(msg.GetRequestId())
	if err != nil {
		return nil, err
	}
	audiences := make([]string, len(msg.GetAudiences()))
	for i, a := range msg.GetAudiences() {
		audiences[i] = rpc.EnumFromProto[string](a)
	}
	a, err := h.m.CreateAnnouncement(ctx, notification.CreateAnnouncementInput{
		Actor:     adminaudit.Actor{AdminID: admin, Action: req.Spec().Procedure},
		Audiences: audiences, Title: msg.GetTitle(), Body: msg.GetBody(), RequestID: requestID,
	})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, createPaths)
	}
	return connect.NewResponse(&adminpb.CreateAnnouncementResponse{Announcement: announcementPB(a)}), nil
}

// ListAnnouncements 列出公告，新到舊。
func (h *Admin) ListAnnouncements(ctx context.Context, req *connect.Request[adminpb.ListAnnouncementsRequest]) (*connect.Response[adminpb.ListAnnouncementsResponse], error) {
	scope := rpc.FilterScope(notificationv1alpha1connect.NotificationAdminServiceListAnnouncementsProcedure)
	after, err := rpc.ValidCursor[notification.AnnouncementCursor](scope, req.Msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	page, err := h.m.ListAnnouncements(ctx, after, req.Msg.GetPageSize())
	if err != nil {
		return nil, rpc.WithFieldPaths(err, listPaths)
	}
	out := &adminpb.ListAnnouncementsResponse{Announcements: make([]*adminpb.Announcement, len(page.Announcements))}
	for i, a := range page.Announcements {
		out.Announcements[i] = announcementPB(a)
	}
	if page.Next != nil {
		out.NextPageToken = rpc.TokenFor(scope, page.Next)
	}
	return connect.NewResponse(out), nil
}

func announcementPB(a notification.Announcement) *adminpb.Announcement {
	out := &adminpb.Announcement{
		Id: a.ID.String(), Title: a.Title, Body: a.Body, CreateTime: timestamppb.New(a.CreateTime),
		InboxCount: a.InboxCount, Dispatched: a.Dispatched,
		Audiences: make([]pb.NotificationAudience, len(a.Audiences)),
	}
	for i, aud := range a.Audiences {
		out.Audiences[i] = rpc.EnumToProto[pb.NotificationAudience](aud)
	}
	return out
}
