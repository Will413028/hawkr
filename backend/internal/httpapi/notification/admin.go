// Package notification 實作 notification 的 Connect 傳輸。admin.go 是後台公告（NotificationAdminService）：欄位與錯誤路徑對應、列舉與時間轉成 proto；
// 驗證、冪等與派送都在 modules/notification，這裡不重做。
package notification

import (
	"context"
	"strconv"
	"strings"
	"time"

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

// Deliveries 是投遞紀錄的重送與未觸達查詢的處理者（*notification.Module）。
type Deliveries interface {
	ResendDeliveries(ctx context.Context, in notification.ResendInput) (notification.ResendResult, error)
	ListUnreachedNotifications(ctx context.Context, in notification.UnreachedInput) (notification.UnreachedPage, error)
}

// AdminSource 是 Admin 需要的全部來源（*notification.Module）。
type AdminSource interface {
	Announcements
	Deliveries
}

// Admin 實作 NotificationAdminService。授權（只有管理員、逐方法的權限、audited）由 rpc 的 interceptor 與 adminauth 的宣告表檢查。
type Admin struct{ m AdminSource }

var _ notificationv1alpha1connect.NotificationAdminServiceHandler = (*Admin)(nil)

// NewAdmin 建立後台的 handler。
func NewAdmin(m AdminSource) *Admin { return &Admin{m: m} }

var (
	createPaths = rpc.FieldPaths{"Audiences": "audiences", "Title": "title", "Body": "body", "RequestID": "request_id"}
	listPaths   = rpc.FieldPaths{"PageSize": "page_size"}
	resendPaths = rpc.FieldPaths{
		"FailTimeStart": "fail_time_start", "FailTimeEnd": "fail_time_end", "Category": "category", "Channel": "channel",
		"Reasons": "reasons", "AccountID": "account_id", "MaxCount": "max_count",
	}
	unreachedPaths = rpc.FieldPaths{"Days": "days", "PageSize": "page_size"}
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

// ResendDeliveries 重排永久失敗且原因可重送的投遞。
func (h *Admin) ResendDeliveries(ctx context.Context, req *connect.Request[adminpb.ResendDeliveriesRequest]) (*connect.Response[adminpb.ResendDeliveriesResponse], error) {
	admin, err := rpc.RequireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	reasons := make([]string, len(msg.GetReasons()))
	for i, r := range msg.GetReasons() {
		reasons[i] = strings.ToLower(rpc.EnumFromProto[string](r))
	}
	res, err := h.m.ResendDeliveries(ctx, notification.ResendInput{
		Actor:     adminaudit.Actor{AdminID: admin, Action: req.Spec().Procedure},
		FailStart: timeOf(msg.GetFailTimeStart()), FailEnd: timeOf(msg.GetFailTimeEnd()),
		Category: rpc.EnumFromProto[notification.Category](msg.GetCategory()), Channel: rpc.EnumFromProto[notification.Channel](msg.GetChannel()),
		Reasons: reasons, AccountID: msg.GetAccountId(), MaxCount: int(msg.GetMaxCount()), DryRun: msg.GetDryRun(),
	})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, resendPaths)
	}
	return connect.NewResponse(&adminpb.ResendDeliveriesResponse{ResendCount: int32(res.Count), HasMore: res.HasMore}), nil //nolint:gosec // 筆數上限 500
}

func timeOf(ts *timestamppb.Timestamp) time.Time {
	if ts == nil {
		return time.Time{}
	}
	return ts.AsTime()
}

// ListUnreachedNotifications 列出近期交易必要通知中沒有觸達的收件人。
func (h *Admin) ListUnreachedNotifications(ctx context.Context, req *connect.Request[adminpb.ListUnreachedNotificationsRequest]) (*connect.Response[adminpb.ListUnreachedNotificationsResponse], error) {
	scope := rpc.FilterScope(notificationv1alpha1connect.NotificationAdminServiceListUnreachedNotificationsProcedure, strconv.Itoa(int(req.Msg.GetDays())))
	after, err := rpc.ValidCursor[notification.UnreachedCursor](scope, req.Msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	page, err := h.m.ListUnreachedNotifications(ctx, notification.UnreachedInput{Days: int(req.Msg.GetDays()), After: after, PageSize: req.Msg.GetPageSize()})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, unreachedPaths)
	}
	out := &adminpb.ListUnreachedNotificationsResponse{UnreachedCount: page.Count, Notifications: make([]*adminpb.UnreachedNotification, len(page.Notifications))}
	for i, n := range page.Notifications {
		item := &adminpb.UnreachedNotification{
			DeliveryId: n.DeliveryID.String(), IntentId: n.IntentID.String(), Type: n.Type,
			Category:  rpc.EnumToProto[pb.NotificationCategory](n.Category),
			Audience:  rpc.EnumToProto[pb.NotificationAudience](n.Audience),
			AccountId: n.AccountID, StoreId: n.StoreID, CreateTime: timestamppb.New(n.CreatedAt),
			Channels: make([]*adminpb.UnreachedChannel, len(n.Channels)),
		}
		for j, c := range n.Channels {
			item.Channels[j] = &adminpb.UnreachedChannel{
				Channel:       rpc.EnumToProto[pb.NotificationChannel](c.Channel),
				FailureReason: rpc.EnumToProto[adminpb.DeliveryFailureReason](strings.ToUpper(c.Reason)),
			}
		}
		out.Notifications[i] = item
	}
	if page.Next != nil {
		out.NextPageToken = rpc.TokenFor(scope, page.Next)
	}
	return connect.NewResponse(out), nil
}
