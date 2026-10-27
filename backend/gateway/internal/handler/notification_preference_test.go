package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	pb "github.com/Will413028/hawkr/go-pkg/proto/notification"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

// ==================== Mock NotificationServiceClient ====================

type mockNotificationServiceClient struct {
	GetNotificationPreferencesFn   func(ctx context.Context, in *pb.GetNotificationPreferencesRequest, opts ...grpc.CallOption) (*pb.GetNotificationPreferencesResponse, error)
	UpdateNotificationPreferenceFn func(ctx context.Context, in *pb.UpdateNotificationPreferenceRequest, opts ...grpc.CallOption) (*pb.UpdateNotificationPreferenceResponse, error)
	RegisterDeviceTokenFn          func(ctx context.Context, in *pb.RegisterDeviceTokenRequest, opts ...grpc.CallOption) (*pb.RegisterDeviceTokenResponse, error)
	RemoveDeviceTokenFn            func(ctx context.Context, in *pb.RemoveDeviceTokenRequest, opts ...grpc.CallOption) (*pb.RemoveDeviceTokenResponse, error)
	ReloadTemplatesFn              func(ctx context.Context, in *pb.ReloadTemplatesRequest, opts ...grpc.CallOption) (*pb.ReloadTemplatesResponse, error)
	GetTemplateFn                  func(ctx context.Context, in *pb.GetTemplateRequest, opts ...grpc.CallOption) (*pb.GetTemplateResponse, error)
	UpdateTemplateEnabledFn        func(ctx context.Context, in *pb.UpdateTemplateEnabledRequest, opts ...grpc.CallOption) (*pb.UpdateTemplateEnabledResponse, error)
	PreviewTemplateFn              func(ctx context.Context, in *pb.PreviewTemplateRequest, opts ...grpc.CallOption) (*pb.PreviewTemplateResponse, error)
	TestSendTemplateFn             func(ctx context.Context, in *pb.TestSendTemplateRequest, opts ...grpc.CallOption) (*pb.TestSendTemplateResponse, error)
	BroadcastAnnouncementFn        func(ctx context.Context, in *pb.BroadcastAnnouncementRequest, opts ...grpc.CallOption) (*pb.BroadcastAnnouncementResponse, error)
	ListNotificationsFn            func(ctx context.Context, in *pb.ListNotificationsRequest, opts ...grpc.CallOption) (*pb.ListNotificationsResponse, error)
	GetUnreadCountFn               func(ctx context.Context, in *pb.GetUnreadCountRequest, opts ...grpc.CallOption) (*pb.GetUnreadCountResponse, error)
	MarkAsReadFn                   func(ctx context.Context, in *pb.MarkAsReadRequest, opts ...grpc.CallOption) (*pb.MarkAsReadResponse, error)
	MarkAllAsReadFn                func(ctx context.Context, in *pb.MarkAllAsReadRequest, opts ...grpc.CallOption) (*pb.MarkAllAsReadResponse, error)
	DeleteNotificationsFn          func(ctx context.Context, in *pb.DeleteNotificationsRequest, opts ...grpc.CallOption) (*pb.DeleteNotificationsResponse, error)
}

func (m *mockNotificationServiceClient) GetNotificationPreferences(ctx context.Context, in *pb.GetNotificationPreferencesRequest, opts ...grpc.CallOption) (*pb.GetNotificationPreferencesResponse, error) {
	if m.GetNotificationPreferencesFn != nil {
		return m.GetNotificationPreferencesFn(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockNotificationServiceClient) UpdateNotificationPreference(ctx context.Context, in *pb.UpdateNotificationPreferenceRequest, opts ...grpc.CallOption) (*pb.UpdateNotificationPreferenceResponse, error) {
	if m.UpdateNotificationPreferenceFn != nil {
		return m.UpdateNotificationPreferenceFn(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockNotificationServiceClient) RegisterDeviceToken(ctx context.Context, in *pb.RegisterDeviceTokenRequest, opts ...grpc.CallOption) (*pb.RegisterDeviceTokenResponse, error) {
	if m.RegisterDeviceTokenFn != nil {
		return m.RegisterDeviceTokenFn(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockNotificationServiceClient) RemoveDeviceToken(ctx context.Context, in *pb.RemoveDeviceTokenRequest, opts ...grpc.CallOption) (*pb.RemoveDeviceTokenResponse, error) {
	if m.RemoveDeviceTokenFn != nil {
		return m.RemoveDeviceTokenFn(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockNotificationServiceClient) ReloadTemplates(ctx context.Context, in *pb.ReloadTemplatesRequest, opts ...grpc.CallOption) (*pb.ReloadTemplatesResponse, error) {
	if m.ReloadTemplatesFn != nil {
		return m.ReloadTemplatesFn(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockNotificationServiceClient) ListTemplates(ctx context.Context, in *pb.ListTemplatesRequest, opts ...grpc.CallOption) (*pb.ListTemplatesResponse, error) {
	return nil, nil
}

func (m *mockNotificationServiceClient) GetTemplate(ctx context.Context, in *pb.GetTemplateRequest, opts ...grpc.CallOption) (*pb.GetTemplateResponse, error) {
	if m.GetTemplateFn != nil {
		return m.GetTemplateFn(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockNotificationServiceClient) UpdateTemplateEnabled(ctx context.Context, in *pb.UpdateTemplateEnabledRequest, opts ...grpc.CallOption) (*pb.UpdateTemplateEnabledResponse, error) {
	if m.UpdateTemplateEnabledFn != nil {
		return m.UpdateTemplateEnabledFn(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockNotificationServiceClient) PreviewTemplate(ctx context.Context, in *pb.PreviewTemplateRequest, opts ...grpc.CallOption) (*pb.PreviewTemplateResponse, error) {
	if m.PreviewTemplateFn != nil {
		return m.PreviewTemplateFn(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockNotificationServiceClient) TestSendTemplate(ctx context.Context, in *pb.TestSendTemplateRequest, opts ...grpc.CallOption) (*pb.TestSendTemplateResponse, error) {
	if m.TestSendTemplateFn != nil {
		return m.TestSendTemplateFn(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockNotificationServiceClient) ListNotifications(ctx context.Context, in *pb.ListNotificationsRequest, opts ...grpc.CallOption) (*pb.ListNotificationsResponse, error) {
	if m.ListNotificationsFn != nil {
		return m.ListNotificationsFn(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockNotificationServiceClient) GetUnreadCount(ctx context.Context, in *pb.GetUnreadCountRequest, opts ...grpc.CallOption) (*pb.GetUnreadCountResponse, error) {
	if m.GetUnreadCountFn != nil {
		return m.GetUnreadCountFn(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockNotificationServiceClient) MarkAsRead(ctx context.Context, in *pb.MarkAsReadRequest, opts ...grpc.CallOption) (*pb.MarkAsReadResponse, error) {
	if m.MarkAsReadFn != nil {
		return m.MarkAsReadFn(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockNotificationServiceClient) MarkAllAsRead(ctx context.Context, in *pb.MarkAllAsReadRequest, opts ...grpc.CallOption) (*pb.MarkAllAsReadResponse, error) {
	if m.MarkAllAsReadFn != nil {
		return m.MarkAllAsReadFn(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockNotificationServiceClient) DeleteNotifications(ctx context.Context, in *pb.DeleteNotificationsRequest, opts ...grpc.CallOption) (*pb.DeleteNotificationsResponse, error) {
	if m.DeleteNotificationsFn != nil {
		return m.DeleteNotificationsFn(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockNotificationServiceClient) BroadcastAnnouncement(ctx context.Context, in *pb.BroadcastAnnouncementRequest, opts ...grpc.CallOption) (*pb.BroadcastAnnouncementResponse, error) {
	if m.BroadcastAnnouncementFn != nil {
		return m.BroadcastAnnouncementFn(ctx, in, opts...)
	}
	return nil, nil
}

// ==================== Router Setup ====================

func setupNotificationPrefRouter(h *Holder) *gin.Engine {
	r := gin.New()
	v1 := r.Group("/api/v1/app/sellers/me")
	v1.Use(fakeJWTAuth())
	{
		v1.GET("/notification-preferences", h.GetNotificationPreferences)
		v1.PUT("/notification-preferences", h.UpdateNotificationPreference)
	}
	return r
}

func setupBuyerNotificationPrefRouter(h *Holder) *gin.Engine {
	r := gin.New()
	v1 := r.Group("/api/v1/app/buyers/me")
	v1.Use(fakeJWTAuth())
	{
		v1.GET("/notification-preferences", h.GetBuyerNotificationPreferences)
		v1.PUT("/notification-preferences", h.UpdateBuyerNotificationPreference)
	}
	return r
}

// ==================== Tests ====================

func TestGetNotificationPreferences_Success(t *testing.T) {
	mock := &mockNotificationServiceClient{
		GetNotificationPreferencesFn: func(ctx context.Context, in *pb.GetNotificationPreferencesRequest, opts ...grpc.CallOption) (*pb.GetNotificationPreferencesResponse, error) {
			if in.UserId != "42" {
				t.Errorf("UserId = %s, want 42", in.UserId)
			}
			if in.UserType != "seller" {
				t.Errorf("UserType = %q, want seller", in.UserType)
			}
			return &pb.GetNotificationPreferencesResponse{
				Code: 0,
				Preferences: []*pb.NotificationPreferenceItem{
					{Category: "order_update", Enabled: true, Title: "新訂單更新", Description: "新訂單與款項撥付通知"},
					{Category: "chat", Enabled: false, Title: "聊天通知", Description: "聊天訊息提醒"},
				},
			}, nil
		},
	}
	h := &Holder{Notification: grpcclient.NewNotificationClientForTest(mock), Store: grandfatherResolverForTest()}

	r := setupNotificationPrefRouter(h)
	req, _ := http.NewRequest("GET", "/api/v1/app/sellers/me/notification-preferences", nil)
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 0)

	data := body["data"].(map[string]interface{})
	prefs := data["preferences"].([]interface{})
	if len(prefs) != 2 {
		t.Fatalf("len(preferences) = %d, want 2", len(prefs))
	}

	p0 := prefs[0].(map[string]interface{})
	if p0["category"] != "order_update" {
		t.Errorf("preferences[0].category = %v, want order_update", p0["category"])
	}
	if p0["enabled"] != true {
		t.Errorf("preferences[0].enabled = %v, want true", p0["enabled"])
	}

	p1 := prefs[1].(map[string]interface{})
	if p1["enabled"] != false {
		t.Errorf("preferences[1].enabled = %v, want false", p1["enabled"])
	}
}

func TestGetNotificationPreferences_Unauthorized(t *testing.T) {
	h := &Holder{}
	r := setupNotificationPrefRouter(h)
	req, _ := http.NewRequest("GET", "/api/v1/app/sellers/me/notification-preferences", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusUnauthorized)
}

func TestGetNotificationPreferences_GrpcError(t *testing.T) {
	mock := &mockNotificationServiceClient{
		GetNotificationPreferencesFn: func(ctx context.Context, in *pb.GetNotificationPreferencesRequest, opts ...grpc.CallOption) (*pb.GetNotificationPreferencesResponse, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}
	h := &Holder{Notification: grpcclient.NewNotificationClientForTest(mock), Store: grandfatherResolverForTest()}

	r := setupNotificationPrefRouter(h)
	req, _ := http.NewRequest("GET", "/api/v1/app/sellers/me/notification-preferences", nil)
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusServiceUnavailable)
}

// TestGetNotificationPreferences_UsesResolvedStoreID 是 C2 修復的直接迴歸測試：
// 賣家路徑必須送出**解析後的 store_id**，不是原始 X-Account-Id（42）——否則
// notification_preferences 原子遷移到 store_id 鍵之後，分歧帳號（store_id !=
// account_id）的偏好永遠查不到，fail-open 送出使用者已關閉的通知類別。
func TestGetNotificationPreferences_UsesResolvedStoreID(t *testing.T) {
	var captured *pb.GetNotificationPreferencesRequest
	mock := &mockNotificationServiceClient{
		GetNotificationPreferencesFn: func(ctx context.Context, in *pb.GetNotificationPreferencesRequest, opts ...grpc.CallOption) (*pb.GetNotificationPreferencesResponse, error) {
			captured = in
			return &pb.GetNotificationPreferencesResponse{Code: 0}, nil
		},
	}
	h := &Holder{Notification: grpcclient.NewNotificationClientForTest(mock), Store: storeResolverForTest(7001)}

	r := setupNotificationPrefRouter(h)
	req, _ := http.NewRequest("GET", "/api/v1/app/sellers/me/notification-preferences", nil)
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil {
		t.Fatal("GetNotificationPreferences was not called")
	}
	if captured.UserId != "7001" {
		t.Errorf("UserId = %s, want 7001 (resolved store id, not raw accountId 42)", captured.UserId)
	}
}

func TestGetNotificationPreferences_NoCurrentSeller403(t *testing.T) {
	mock := &mockNotificationServiceClient{}
	h := &Holder{Notification: grpcclient.NewNotificationClientForTest(mock), Store: noStoreResolverForTest()}

	r := setupNotificationPrefRouter(h)
	req, _ := http.NewRequest("GET", "/api/v1/app/sellers/me/notification-preferences", nil)
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusForbidden)
	body := parseJSON(t, w.Body)
	if body["message"] != "seller access required" {
		t.Errorf("message = %v, want %q", body["message"], "seller access required")
	}
}

func TestUpdateNotificationPreference_Success(t *testing.T) {
	mock := &mockNotificationServiceClient{
		UpdateNotificationPreferenceFn: func(ctx context.Context, in *pb.UpdateNotificationPreferenceRequest, opts ...grpc.CallOption) (*pb.UpdateNotificationPreferenceResponse, error) {
			if in.Category != "chat" {
				t.Errorf("Category = %q, want chat", in.Category)
			}
			if in.Enabled != false {
				t.Errorf("Enabled = %v, want false", in.Enabled)
			}
			return &pb.UpdateNotificationPreferenceResponse{Code: 0, Message: "ok"}, nil
		},
	}
	h := &Holder{Notification: grpcclient.NewNotificationClientForTest(mock), Store: grandfatherResolverForTest()}

	reqBody, _ := json.Marshal(map[string]interface{}{
		"category": "chat",
		"enabled":  false,
	})

	r := setupNotificationPrefRouter(h)
	req, _ := http.NewRequest("PUT", "/api/v1/app/sellers/me/notification-preferences", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 0)
}

// TestUpdateNotificationPreference_UsesResolvedStoreID mirrors
// TestGetNotificationPreferences_UsesResolvedStoreID for the write path.
func TestUpdateNotificationPreference_UsesResolvedStoreID(t *testing.T) {
	var captured *pb.UpdateNotificationPreferenceRequest
	mock := &mockNotificationServiceClient{
		UpdateNotificationPreferenceFn: func(ctx context.Context, in *pb.UpdateNotificationPreferenceRequest, opts ...grpc.CallOption) (*pb.UpdateNotificationPreferenceResponse, error) {
			captured = in
			return &pb.UpdateNotificationPreferenceResponse{Code: 0, Message: "ok"}, nil
		},
	}
	h := &Holder{Notification: grpcclient.NewNotificationClientForTest(mock), Store: storeResolverForTest(7001)}

	reqBody, _ := json.Marshal(map[string]interface{}{
		"category": "chat",
		"enabled":  false,
	})

	r := setupNotificationPrefRouter(h)
	req, _ := http.NewRequest("PUT", "/api/v1/app/sellers/me/notification-preferences", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil {
		t.Fatal("UpdateNotificationPreference was not called")
	}
	if captured.UserId != "7001" {
		t.Errorf("UserId = %s, want 7001 (resolved store id, not raw accountId 42)", captured.UserId)
	}
}

func TestUpdateNotificationPreference_NoCurrentSeller403(t *testing.T) {
	mock := &mockNotificationServiceClient{}
	h := &Holder{Notification: grpcclient.NewNotificationClientForTest(mock), Store: noStoreResolverForTest()}

	reqBody, _ := json.Marshal(map[string]interface{}{
		"category": "chat",
		"enabled":  false,
	})

	r := setupNotificationPrefRouter(h)
	req, _ := http.NewRequest("PUT", "/api/v1/app/sellers/me/notification-preferences", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusForbidden)
	body := parseJSON(t, w.Body)
	if body["message"] != "seller access required" {
		t.Errorf("message = %v, want %q", body["message"], "seller access required")
	}
}

func TestUpdateNotificationPreference_InvalidCategory(t *testing.T) {
	mock := &mockNotificationServiceClient{}
	h := &Holder{Notification: grpcclient.NewNotificationClientForTest(mock), Store: grandfatherResolverForTest()}

	reqBody, _ := json.Marshal(map[string]interface{}{
		"category": "invalid_cat",
		"enabled":  false,
	})

	r := setupNotificationPrefRouter(h)
	req, _ := http.NewRequest("PUT", "/api/v1/app/sellers/me/notification-preferences", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusBadRequest)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 400)
}

func TestUpdateNotificationPreference_Unauthorized(t *testing.T) {
	h := &Holder{}
	r := setupNotificationPrefRouter(h)
	req, _ := http.NewRequest("PUT", "/api/v1/app/sellers/me/notification-preferences", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusUnauthorized)
}

// ==================== Buyer Tests ====================

func TestGetBuyerNotificationPreferences_Success(t *testing.T) {
	mock := &mockNotificationServiceClient{
		GetNotificationPreferencesFn: func(ctx context.Context, in *pb.GetNotificationPreferencesRequest, opts ...grpc.CallOption) (*pb.GetNotificationPreferencesResponse, error) {
			if in.UserId != "42" {
				t.Errorf("UserId = %s, want 42", in.UserId)
			}
			if in.UserType != "buyer" {
				t.Errorf("UserType = %q, want buyer", in.UserType)
			}
			return &pb.GetNotificationPreferencesResponse{
				Code: 0,
				Preferences: []*pb.NotificationPreferenceItem{
					{Category: "order_update", Enabled: true, Title: "新訂單更新", Description: "當訂單狀態改變時發送通知"},
					{Category: "coupon", Enabled: false, Title: "優惠卷通知", Description: "優惠卷發送通知與到期通知"},
					{Category: "message", Enabled: true, Title: "賣家留言通知", Description: "留言提醒"},
				},
			}, nil
		},
	}
	h := &Holder{
		Notification: grpcclient.NewNotificationClientForTest(mock),
		BuyerID:      buyerGrandfatherResolverForTest(),
	}

	r := setupBuyerNotificationPrefRouter(h)
	req, _ := http.NewRequest("GET", "/api/v1/app/buyers/me/notification-preferences", nil)
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 0)

	data := body["data"].(map[string]interface{})
	prefs := data["preferences"].([]interface{})
	if len(prefs) != 3 {
		t.Fatalf("len(preferences) = %d, want 3", len(prefs))
	}

	p1 := prefs[1].(map[string]interface{})
	if p1["category"] != "coupon" {
		t.Errorf("preferences[1].category = %v, want coupon", p1["category"])
	}
	if p1["enabled"] != false {
		t.Errorf("preferences[1].enabled = %v, want false", p1["enabled"])
	}
}

func TestGetBuyerNotificationPreferences_Unauthorized(t *testing.T) {
	h := &Holder{}
	r := setupBuyerNotificationPrefRouter(h)
	req, _ := http.NewRequest("GET", "/api/v1/app/buyers/me/notification-preferences", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusUnauthorized)
}

func TestGetBuyerNotificationPreferences_GrpcError(t *testing.T) {
	mock := &mockNotificationServiceClient{
		GetNotificationPreferencesFn: func(ctx context.Context, in *pb.GetNotificationPreferencesRequest, opts ...grpc.CallOption) (*pb.GetNotificationPreferencesResponse, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}
	h := &Holder{
		Notification: grpcclient.NewNotificationClientForTest(mock),
		BuyerID:      buyerGrandfatherResolverForTest(),
	}

	r := setupBuyerNotificationPrefRouter(h)
	req, _ := http.NewRequest("GET", "/api/v1/app/buyers/me/notification-preferences", nil)
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusServiceUnavailable)
}

func TestGetBuyerNotificationPreferences_UsesResolvedBuyerID(t *testing.T) {
	var captured *pb.GetNotificationPreferencesRequest
	mock := &mockNotificationServiceClient{
		GetNotificationPreferencesFn: func(ctx context.Context, in *pb.GetNotificationPreferencesRequest, opts ...grpc.CallOption) (*pb.GetNotificationPreferencesResponse, error) {
			captured = in
			return &pb.GetNotificationPreferencesResponse{Code: 0}, nil
		},
	}
	h := &Holder{
		Notification: grpcclient.NewNotificationClientForTest(mock),
		BuyerID:      buyerResolverForTest(9999),
	}

	r := setupBuyerNotificationPrefRouter(h)
	req, _ := http.NewRequest("GET", "/api/v1/app/buyers/me/notification-preferences", nil)
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil {
		t.Fatal("GetNotificationPreferences was not called")
	}
	if captured.UserId != "9999" {
		t.Errorf("UserId = %s, want 9999 (resolved buyer id, not raw accountId 42)", captured.UserId)
	}
}

func TestUpdateBuyerNotificationPreference_Success(t *testing.T) {
	mock := &mockNotificationServiceClient{
		UpdateNotificationPreferenceFn: func(ctx context.Context, in *pb.UpdateNotificationPreferenceRequest, opts ...grpc.CallOption) (*pb.UpdateNotificationPreferenceResponse, error) {
			if in.UserType != "buyer" {
				t.Errorf("UserType = %q, want buyer", in.UserType)
			}
			if in.Category != "coupon" {
				t.Errorf("Category = %q, want coupon", in.Category)
			}
			if in.Enabled != false {
				t.Errorf("Enabled = %v, want false", in.Enabled)
			}
			return &pb.UpdateNotificationPreferenceResponse{Code: 0, Message: "ok"}, nil
		},
	}
	h := &Holder{
		Notification: grpcclient.NewNotificationClientForTest(mock),
		BuyerID:      buyerGrandfatherResolverForTest(),
	}

	reqBody, _ := json.Marshal(map[string]interface{}{
		"category": "coupon",
		"enabled":  false,
	})

	r := setupBuyerNotificationPrefRouter(h)
	req, _ := http.NewRequest("PUT", "/api/v1/app/buyers/me/notification-preferences", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 0)
}

func TestUpdateBuyerNotificationPreference_InvalidCategory(t *testing.T) {
	mock := &mockNotificationServiceClient{}
	h := &Holder{
		Notification: grpcclient.NewNotificationClientForTest(mock),
		BuyerID:      buyerGrandfatherResolverForTest(),
	}

	reqBody, _ := json.Marshal(map[string]interface{}{
		"category": "follower",
		"enabled":  true,
	})

	r := setupBuyerNotificationPrefRouter(h)
	req, _ := http.NewRequest("PUT", "/api/v1/app/buyers/me/notification-preferences", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusBadRequest)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 400)
}

func TestUpdateBuyerNotificationPreference_MessageCategory(t *testing.T) {
	mock := &mockNotificationServiceClient{
		UpdateNotificationPreferenceFn: func(ctx context.Context, in *pb.UpdateNotificationPreferenceRequest, opts ...grpc.CallOption) (*pb.UpdateNotificationPreferenceResponse, error) {
			if in.Category != "message" {
				t.Errorf("Category = %q, want message", in.Category)
			}
			return &pb.UpdateNotificationPreferenceResponse{Code: 0, Message: "ok"}, nil
		},
	}
	h := &Holder{
		Notification: grpcclient.NewNotificationClientForTest(mock),
		BuyerID:      buyerGrandfatherResolverForTest(),
	}

	reqBody, _ := json.Marshal(map[string]interface{}{
		"category": "message",
		"enabled":  true,
	})

	r := setupBuyerNotificationPrefRouter(h)
	req, _ := http.NewRequest("PUT", "/api/v1/app/buyers/me/notification-preferences", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 0)
}

func TestUpdateBuyerNotificationPreference_UsesResolvedBuyerID(t *testing.T) {
	var captured *pb.UpdateNotificationPreferenceRequest
	mock := &mockNotificationServiceClient{
		UpdateNotificationPreferenceFn: func(ctx context.Context, in *pb.UpdateNotificationPreferenceRequest, opts ...grpc.CallOption) (*pb.UpdateNotificationPreferenceResponse, error) {
			captured = in
			return &pb.UpdateNotificationPreferenceResponse{Code: 0, Message: "ok"}, nil
		},
	}
	h := &Holder{
		Notification: grpcclient.NewNotificationClientForTest(mock),
		BuyerID:      buyerResolverForTest(9999),
	}

	reqBody, _ := json.Marshal(map[string]interface{}{
		"category": "coupon",
		"enabled":  false,
	})

	r := setupBuyerNotificationPrefRouter(h)
	req, _ := http.NewRequest("PUT", "/api/v1/app/buyers/me/notification-preferences", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil {
		t.Fatal("UpdateNotificationPreference was not called")
	}
	if captured.UserId != "9999" {
		t.Errorf("UserId = %s, want 9999 (resolved buyer id, not raw accountId 42)", captured.UserId)
	}
}

func TestUpdateBuyerNotificationPreference_Unauthorized(t *testing.T) {
	h := &Holder{}
	r := setupBuyerNotificationPrefRouter(h)
	req, _ := http.NewRequest("PUT", "/api/v1/app/buyers/me/notification-preferences", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusUnauthorized)
}
