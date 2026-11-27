// Package notification 是 NotificationAppService 的 unary handler：登入的帳號讀寫自己的收件匣、未讀數、偏好與裝置 token。
//
// 收件身分是 (帳號, 受眾)：BUYER 是登入的帳號，STORE 要有現任店家（沒有回 not_found），賣家能力 CapNotification 在停權白名單內。
// 管理員與代行的憑證不走這條（service 是 AppService，interceptor 只放行顧客帳號；代行憑證走各模組的 DelegatedService）。
// 通知與裝置 token 的擁有者是帳號，已讀、刪除與 token 不帶受眾。
package notification

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/notification/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/notification/v1alpha1/notificationv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/notification"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/pagination"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// Notifications 是通知的來源（*notification.Module）。
type Notifications interface {
	ValidateAudience(ctx context.Context, a notification.Audience) error
	ListInbox(ctx context.Context, r notification.Recipient, category *notification.Category, l uitext.Locale, after *notification.InboxCursor, size int32) (notification.InboxPage, error)
	UnreadCount(ctx context.Context, r notification.Recipient) (int64, error)
	MarkInboxItemRead(ctx context.Context, accountID int64, id uuid.UUID) error
	MarkAllInboxItemsRead(ctx context.Context, r notification.Recipient) (int64, error)
	BatchDeleteInboxItems(ctx context.Context, accountID int64, ids []uuid.UUID) error
	NotificationPreferences(ctx context.Context, r notification.Recipient) ([]notification.Preference, error)
	UpdateNotificationPreference(ctx context.Context, r notification.Recipient, c notification.Category, ch notification.Channel, enabled bool) (notification.Preference, error)
	RegisterDeviceToken(ctx context.Context, accountID int64, app notification.DeviceApp, token string) error
	DeleteDeviceToken(ctx context.Context, accountID int64, token string) error
	// Watch 訂閱 (帳號, 受眾) 的訊號串流；帳號同時開著的串流達上限回 resource_exhausted。
	Watch(p notification.WatchParams) (notification.Watcher, error)
}

// Stores 是 STORE 受眾的店家來源（*seller.Module）：帳號此刻的店，只在有能力時成功。
type Stores interface {
	ActingStore(ctx context.Context, accountID int64, c seller.Capability) (seller.Store, error)
}

// App 實作 NotificationAppService。
type App struct {
	notifications Notifications
	stores        Stores
	images        rpc.Images
	log           *slog.Logger
	lifetime func() time.Duration
}

var _ notificationv1alpha1connect.NotificationAppServiceHandler = (*App)(nil)

// NewApp 建立 handler。能力宣告表（declarations）與 NotificationAppService 的方法對不上時回錯誤，不能啟動。
func NewApp(notifications Notifications, stores Stores, images rpc.Images, log *slog.Logger) (*App, error) {
	switch {
	case notifications == nil:
		return nil, fmt.Errorf("httpapi/notification：需要 Notifications")
	case stores == nil:
		return nil, fmt.Errorf("httpapi/notification：需要 Stores")
	case log == nil:
		return nil, fmt.Errorf("httpapi/notification：需要 logger")
	}
	if err := checkDeclarations(declarations, serviceDescriptor()); err != nil {
		return nil, err
	}
	return &App{notifications: notifications, stores: stores, images: images, log: log, lifetime: func() time.Duration { return streamLifetime }}, nil
}

const (
	defaultPageSize = 20
	maxPageSize     = 50
)

func (h *App) recipient(ctx context.Context, audience pb.NotificationAudience) (notification.Recipient, error) {
	p, err := rpc.RequireAccount(ctx)
	if err != nil {
		return notification.Recipient{}, err
	}
	a := rpc.EnumFromProto[notification.Audience](audience)
	if err := h.notifications.ValidateAudience(ctx, a); err != nil {
		return notification.Recipient{}, err
	}
	switch a {
	case notification.AudienceBuyer:
		return notification.Recipient{AccountID: p.ID, Audience: notification.AudienceBuyer}, nil
	case notification.AudienceStore:
		s, err := h.stores.ActingStore(ctx, p.ID, seller.CapNotification)
		if err != nil {
			return notification.Recipient{}, err
		}
		return notification.Recipient{AccountID: p.ID, Audience: notification.AudienceStore, StoreID: s.StoreID}, nil
	}
	return notification.Recipient{}, fmt.Errorf("受眾 %q 通過檢查卻沒有對應的收件人", a)
}

func (h *App) ListInboxItems(ctx context.Context, req *connect.Request[pb.ListInboxItemsRequest]) (*connect.Response[pb.ListInboxItemsResponse], error) {
	size, err := pagination.PageSize(req.Msg.GetPageSize(), defaultPageSize, maxPageSize)
	if err != nil {
		return nil, err
	}
	r, err := h.recipient(ctx, req.Msg.GetAudience())
	if err != nil {
		return nil, err
	}
	var category *notification.Category
	categoryName := ""
	if req.Msg.Category != nil {
		c := rpc.EnumFromProto[notification.Category](req.Msg.GetCategory())
		category, categoryName = &c, string(c)
	}
	scope := rpc.FilterScope(notificationv1alpha1connect.NotificationAppServiceListInboxItemsProcedure, rpc.FormatID(r.AccountID), string(r.Audience), rpc.FormatID(r.StoreID), categoryName)
	after, err := rpc.ValidCursor[notification.InboxCursor](scope, req.Msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	page, err := h.notifications.ListInbox(ctx, r, category, uitext.LocaleFrom(ctx), after, size)
	if err != nil {
		return nil, err
	}
	resp := &pb.ListInboxItemsResponse{Items: make([]*pb.InboxItem, 0, len(page.Items)), TotalSize: count32(page.Total)}
	for _, it := range page.Items {
		resp.Items = append(resp.Items, h.itemPB(it))
	}
	if page.Next != nil {
		resp.NextPageToken = rpc.TokenFor(scope, page.Next)
	}
	return connect.NewResponse(resp), nil
}

func count32(n int64) int32 {
	if n > math.MaxInt32 {
		return math.MaxInt32
	}
	return int32(n) //nolint:gosec // clamped above
}

func (h *App) itemPB(it notification.InboxItem) *pb.InboxItem {
	out := &pb.InboxItem{
		Id:           it.ID.String(),
		Category:     rpc.EnumToProto[pb.NotificationCategory](it.Category),
		Presentation: rpc.EnumToProto[pb.InboxPresentation](it.Presentation),
		Title:        it.Title,
		Body:         it.Body,
		DeepLink:     it.DeepLink,
		Image:        h.images.Image(it.ImageKey),
		Read:         it.ReadAt != nil,
		CreateTime:   timestamppb.New(it.Time),
	}
	if it.ReadAt != nil {
		out.ReadTime = timestamppb.New(*it.ReadAt)
	}
	return out
}

func (h *App) GetUnreadCount(ctx context.Context, req *connect.Request[pb.GetUnreadCountRequest]) (*connect.Response[pb.GetUnreadCountResponse], error) {
	r, err := h.recipient(ctx, req.Msg.GetAudience())
	if err != nil {
		return nil, err
	}
	n, err := h.notifications.UnreadCount(ctx, r)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.GetUnreadCountResponse{UnreadCount: count32(n)}), nil
}

func (h *App) MarkInboxItemRead(ctx context.Context, req *connect.Request[pb.MarkInboxItemReadRequest]) (*connect.Response[pb.MarkInboxItemReadResponse], error) {
	p, err := rpc.RequireAccount(ctx)
	if err != nil {
		return nil, err
	}
	id, err := rpc.ParseUUID("id", req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	if err := h.notifications.MarkInboxItemRead(ctx, p.ID, id); err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.MarkInboxItemReadResponse{}), nil
}

func (h *App) MarkAllInboxItemsRead(ctx context.Context, req *connect.Request[pb.MarkAllInboxItemsReadRequest]) (*connect.Response[pb.MarkAllInboxItemsReadResponse], error) {
	r, err := h.recipient(ctx, req.Msg.GetAudience())
	if err != nil {
		return nil, err
	}
	n, err := h.notifications.MarkAllInboxItemsRead(ctx, r)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.MarkAllInboxItemsReadResponse{UpdatedCount: count32(n)}), nil
}

func (h *App) BatchDeleteInboxItems(ctx context.Context, req *connect.Request[pb.BatchDeleteInboxItemsRequest]) (*connect.Response[pb.BatchDeleteInboxItemsResponse], error) {
	p, err := rpc.RequireAccount(ctx)
	if err != nil {
		return nil, err
	}
	raw := req.Msg.GetIds()
	ids := make([]uuid.UUID, len(raw))
	for i, s := range raw {
		if ids[i], err = rpc.ParseUUID("ids", s); err != nil {
			return nil, err
		}
	}
	if err := h.notifications.BatchDeleteInboxItems(ctx, p.ID, ids); err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.BatchDeleteInboxItemsResponse{}), nil
}

func preferencePB(p notification.Preference) *pb.PreferenceEntry {
	return &pb.PreferenceEntry{
		Category:   rpc.EnumToProto[pb.NotificationCategory](p.Category),
		Channel:    rpc.EnumToProto[pb.NotificationChannel](p.Channel),
		Enabled:    p.Enabled,
		CanDisable: p.CanDisable,
	}
}

func (h *App) GetNotificationPreferences(ctx context.Context, req *connect.Request[pb.GetNotificationPreferencesRequest]) (*connect.Response[pb.GetNotificationPreferencesResponse], error) {
	r, err := h.recipient(ctx, req.Msg.GetAudience())
	if err != nil {
		return nil, err
	}
	prefs, err := h.notifications.NotificationPreferences(ctx, r)
	if err != nil {
		return nil, err
	}
	resp := &pb.GetNotificationPreferencesResponse{Entries: make([]*pb.PreferenceEntry, 0, len(prefs))}
	for _, p := range prefs {
		resp.Entries = append(resp.Entries, preferencePB(p))
	}
	return connect.NewResponse(resp), nil
}

func (h *App) UpdateNotificationPreference(ctx context.Context, req *connect.Request[pb.UpdateNotificationPreferenceRequest]) (*connect.Response[pb.UpdateNotificationPreferenceResponse], error) {
	r, err := h.recipient(ctx, req.Msg.GetAudience())
	if err != nil {
		return nil, err
	}
	category := rpc.EnumFromProto[notification.Category](req.Msg.GetCategory())
	channel := rpc.EnumFromProto[notification.Channel](req.Msg.GetChannel())
	p, err := h.notifications.UpdateNotificationPreference(ctx, r, category, channel, req.Msg.GetEnabled())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.UpdateNotificationPreferenceResponse{Entry: preferencePB(p)}), nil
}

func deviceApp(a pb.NotificationApp) notification.DeviceApp {
	switch a {
	case pb.NotificationApp_NOTIFICATION_APP_BUYER_APP:
		return notification.DeviceAppBuyer
	case pb.NotificationApp_NOTIFICATION_APP_SELLER_APP:
		return notification.DeviceAppSeller
	case pb.NotificationApp_NOTIFICATION_APP_UNSPECIFIED:
	}
	return ""
}

func (h *App) RegisterDeviceToken(ctx context.Context, req *connect.Request[pb.RegisterDeviceTokenRequest]) (*connect.Response[pb.RegisterDeviceTokenResponse], error) {
	p, err := rpc.RequireAccount(ctx)
	if err != nil {
		return nil, err
	}
	token := req.Msg.GetToken()
	app := deviceApp(req.Msg.GetApp())
	if err := h.notifications.RegisterDeviceToken(ctx, p.ID, app, token); err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.RegisterDeviceTokenResponse{}), nil
}

func (h *App) DeleteDeviceToken(ctx context.Context, req *connect.Request[pb.DeleteDeviceTokenRequest]) (*connect.Response[pb.DeleteDeviceTokenResponse], error) {
	p, err := rpc.RequireAccount(ctx)
	if err != nil {
		return nil, err
	}
	token := req.Msg.GetToken()
	if err := h.notifications.DeleteDeviceToken(ctx, p.ID, token); err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.DeleteDeviceTokenResponse{}), nil
}
