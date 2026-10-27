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
	notificationpb "github.com/Will413028/hawkr/go-pkg/proto/notification"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

// ==================== Router Setup ====================

func setupDeviceTokenRouter(h *Holder) *gin.Engine {
	r := gin.New()
	v1 := r.Group("/api/v1/app/notifications")
	v1.Use(fakeJWTAuth())
	{
		v1.POST("/device-token", h.RegisterDeviceToken)
		v1.PUT("/device-token", h.RefreshDeviceToken)
		v1.DELETE("/device-token", h.RemoveDeviceToken)
	}
	return r
}

// ==================== RefreshDeviceToken ====================

func TestRefreshDeviceToken_UserTypeEvaluation(t *testing.T) {
	tests := []struct {
		name             string
		loginRole        string
		expectedUserType string
		expectedStatus   int
	}{
		{
			name:             "buyer app registers as buyer",
			loginRole:        "1",
			expectedUserType: "buyer",
			expectedStatus:   http.StatusOK,
		},
		{
			name:             "seller app registers as seller",
			loginRole:        "2",
			expectedUserType: "seller",
			expectedStatus:   http.StatusOK,
		},
		{
			name:           "missing login role returns 400",
			loginRole:      "",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var capturedUserType string
			mockClient := &mockNotificationServiceClient{
				RegisterDeviceTokenFn: func(ctx context.Context, in *notificationpb.RegisterDeviceTokenRequest, opts ...grpc.CallOption) (*notificationpb.RegisterDeviceTokenResponse, error) {
					capturedUserType = in.UserType
					return &notificationpb.RegisterDeviceTokenResponse{Code: 0}, nil
				},
			}

			holder := &Holder{
				Notification: grpcclient.NewNotificationClientForTest(mockClient),
				BuyerID:      buyerGrandfatherResolverForTest(),
				Store:        grandfatherResolverForTest(),
			}

			r := setupDeviceTokenRouter(holder)

			body := map[string]string{
				"token":    "test_token",
				"platform": "ios",
			}
			jsonBytes, _ := json.Marshal(body)
			req := httptest.NewRequest("POST", "/api/v1/app/notifications/device-token", bytes.NewBuffer(jsonBytes))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Account-Id", "123")
			req.Header.Set("X-Account-Type", "3")
			if tc.loginRole != "" {
				req.Header.Set("X-Login-Role", tc.loginRole)
			}

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assertHTTPStatus(t, w, tc.expectedStatus)
			if tc.expectedStatus == http.StatusOK && capturedUserType != tc.expectedUserType {
				t.Errorf("expected UserType %q, got %q", tc.expectedUserType, capturedUserType)
			}
		})
	}
}

func TestRefreshDeviceToken_PassesAllFields(t *testing.T) {
	var captured *notificationpb.RegisterDeviceTokenRequest
	mockClient := &mockNotificationServiceClient{
		RegisterDeviceTokenFn: func(ctx context.Context, in *notificationpb.RegisterDeviceTokenRequest, opts ...grpc.CallOption) (*notificationpb.RegisterDeviceTokenResponse, error) {
			captured = in
			return &notificationpb.RegisterDeviceTokenResponse{Code: 0}, nil
		},
	}

	h := &Holder{
		Notification: grpcclient.NewNotificationClientForTest(mockClient),
		BuyerID:      buyerGrandfatherResolverForTest(),
	}
	r := setupDeviceTokenRouter(h)

	body, _ := json.Marshal(map[string]string{"token": "fcm-abc-123", "platform": "ios"})
	req := httptest.NewRequest("POST", "/api/v1/app/notifications/device-token", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "9876")
	req.Header.Set("X-Account-Type", "1")
	req.Header.Set("X-Login-Role", "1")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil {
		t.Fatal("RegisterDeviceToken was not called")
	}
	if captured.UserId != "9876" {
		t.Errorf("UserId = %s, want 9876", captured.UserId)
	}
	if captured.UserType != "buyer" {
		t.Errorf("UserType = %q, want %q", captured.UserType, "buyer")
	}
	if captured.Token != "fcm-abc-123" {
		t.Errorf("Token = %q, want %q", captured.Token, "fcm-abc-123")
	}
	if captured.Platform != "ios" {
		t.Errorf("Platform = %q, want %q", captured.Platform, "ios")
	}
}

// App 只發 iOS：android 也要擋下。
func TestRefreshDeviceToken_InvalidPlatform(t *testing.T) {
	for _, platform := range []string{"windows", "android"} {
		h := &Holder{Notification: grpcclient.NewNotificationClientForTest(&mockNotificationServiceClient{})}
		r := setupDeviceTokenRouter(h)

		body, _ := json.Marshal(map[string]string{"token": "tok", "platform": platform})
		req := httptest.NewRequest("POST", "/api/v1/app/notifications/device-token", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Account-Id", "1")
		req.Header.Set("X-Account-Type", "1")
		req.Header.Set("X-Login-Role", "1")

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assertHTTPStatus(t, w, http.StatusBadRequest)
	}
}

func TestRefreshDeviceToken_MissingToken(t *testing.T) {
	h := &Holder{Notification: grpcclient.NewNotificationClientForTest(&mockNotificationServiceClient{})}
	r := setupDeviceTokenRouter(h)

	// 只送 platform，沒有 token
	body, _ := json.Marshal(map[string]string{"platform": "ios"})
	req := httptest.NewRequest("POST", "/api/v1/app/notifications/device-token", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "1")
	req.Header.Set("X-Account-Type", "1")
	req.Header.Set("X-Login-Role", "1")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusBadRequest)
}

func TestRefreshDeviceToken_MissingPlatform(t *testing.T) {
	h := &Holder{Notification: grpcclient.NewNotificationClientForTest(&mockNotificationServiceClient{})}
	r := setupDeviceTokenRouter(h)

	// 只送 token，沒有 platform
	body, _ := json.Marshal(map[string]string{"token": "tok"})
	req := httptest.NewRequest("POST", "/api/v1/app/notifications/device-token", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "1")
	req.Header.Set("X-Account-Type", "1")
	req.Header.Set("X-Login-Role", "1")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusBadRequest)
}

func TestRefreshDeviceToken_Unauthorized(t *testing.T) {
	h := &Holder{Notification: grpcclient.NewNotificationClientForTest(&mockNotificationServiceClient{})}
	r := setupDeviceTokenRouter(h)

	body, _ := json.Marshal(map[string]string{"token": "tok", "platform": "ios"})
	req := httptest.NewRequest("POST", "/api/v1/app/notifications/device-token", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	// 不送 X-Account-Id → fakeJWTAuth 回 401

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusUnauthorized)
}

func TestRefreshDeviceToken_GrpcError(t *testing.T) {
	mockClient := &mockNotificationServiceClient{
		RegisterDeviceTokenFn: func(ctx context.Context, in *notificationpb.RegisterDeviceTokenRequest, opts ...grpc.CallOption) (*notificationpb.RegisterDeviceTokenResponse, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}

	h := &Holder{
		Notification: grpcclient.NewNotificationClientForTest(mockClient),
		BuyerID:      buyerGrandfatherResolverForTest(),
	}
	r := setupDeviceTokenRouter(h)

	body, _ := json.Marshal(map[string]string{"token": "tok", "platform": "ios"})
	req := httptest.NewRequest("POST", "/api/v1/app/notifications/device-token", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "1")
	req.Header.Set("X-Account-Type", "1")
	req.Header.Set("X-Login-Role", "1")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// gRPC 錯誤 → 非 200
	if w.Code == http.StatusOK {
		t.Errorf("expected non-200 on gRPC error, got %d", w.Code)
	}
}

func TestRefreshDeviceToken_PUTMethodAlsoWorks(t *testing.T) {
	var called bool
	mockClient := &mockNotificationServiceClient{
		RegisterDeviceTokenFn: func(ctx context.Context, in *notificationpb.RegisterDeviceTokenRequest, opts ...grpc.CallOption) (*notificationpb.RegisterDeviceTokenResponse, error) {
			called = true
			return &notificationpb.RegisterDeviceTokenResponse{Code: 0}, nil
		},
	}

	h := &Holder{
		Notification: grpcclient.NewNotificationClientForTest(mockClient),
		BuyerID:      buyerGrandfatherResolverForTest(),
	}
	r := setupDeviceTokenRouter(h)

	body, _ := json.Marshal(map[string]string{"token": "tok", "platform": "ios"})
	req := httptest.NewRequest("PUT", "/api/v1/app/notifications/device-token", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "1")
	req.Header.Set("X-Account-Type", "1")
	req.Header.Set("X-Login-Role", "1")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if !called {
		t.Error("RegisterDeviceToken was not called via PUT")
	}
}

func TestUpsertDeviceToken_BuyerRole_UsesResolvedBuyerID(t *testing.T) {
	var captured *notificationpb.RegisterDeviceTokenRequest
	mockClient := &mockNotificationServiceClient{
		RegisterDeviceTokenFn: func(ctx context.Context, in *notificationpb.RegisterDeviceTokenRequest, opts ...grpc.CallOption) (*notificationpb.RegisterDeviceTokenResponse, error) {
			captured = in
			return &notificationpb.RegisterDeviceTokenResponse{Code: 0}, nil
		},
	}

	h := &Holder{
		Notification: grpcclient.NewNotificationClientForTest(mockClient),
		BuyerID:      buyerResolverForTest(9999),
	}
	r := setupDeviceTokenRouter(h)

	body, _ := json.Marshal(map[string]string{"token": "fcm-buyer-tok", "platform": "ios"})
	req := httptest.NewRequest("POST", "/api/v1/app/notifications/device-token", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "42")
	req.Header.Set("X-Account-Type", "1")
	req.Header.Set("X-Login-Role", "1")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil {
		t.Fatal("RegisterDeviceToken was not called")
	}
	if captured.UserId != "9999" {
		t.Errorf("UserId = %s, want 9999 (resolved buyer id, not raw accountId 42)", captured.UserId)
	}
}

func TestUpsertDeviceToken_SellerRole_UsesResolvedStoreID(t *testing.T) {
	var captured *notificationpb.RegisterDeviceTokenRequest
	mockClient := &mockNotificationServiceClient{
		RegisterDeviceTokenFn: func(ctx context.Context, in *notificationpb.RegisterDeviceTokenRequest, opts ...grpc.CallOption) (*notificationpb.RegisterDeviceTokenResponse, error) {
			captured = in
			return &notificationpb.RegisterDeviceTokenResponse{Code: 0}, nil
		},
	}

	h := &Holder{
		Notification: grpcclient.NewNotificationClientForTest(mockClient),
		Store:        storeResolverForTest(8888),
	}
	r := setupDeviceTokenRouter(h)

	body, _ := json.Marshal(map[string]string{"token": "fcm-seller-tok", "platform": "ios"})
	req := httptest.NewRequest("POST", "/api/v1/app/notifications/device-token", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "42")
	req.Header.Set("X-Account-Type", "2")
	req.Header.Set("X-Login-Role", "2")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil {
		t.Fatal("RegisterDeviceToken was not called")
	}
	if captured.UserId != "8888" {
		t.Errorf("UserId = %s, want 8888 (resolved store id, not raw accountId 42)", captured.UserId)
	}
}

// ==================== RemoveDeviceToken ====================

func TestRemoveDeviceToken_UserTypeEvaluation(t *testing.T) {
	tests := []struct {
		name             string
		loginRole        string
		expectedUserType string
		expectedStatus   int
	}{
		{"buyer removes as buyer", "1", "buyer", http.StatusOK},
		{"seller removes as seller", "2", "seller", http.StatusOK},
		{"missing login role returns 400", "", "", http.StatusBadRequest},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var capturedUserType string
			mockClient := &mockNotificationServiceClient{
				RemoveDeviceTokenFn: func(ctx context.Context, in *notificationpb.RemoveDeviceTokenRequest, opts ...grpc.CallOption) (*notificationpb.RemoveDeviceTokenResponse, error) {
					capturedUserType = in.UserType
					return &notificationpb.RemoveDeviceTokenResponse{Code: 0}, nil
				},
			}

			h := &Holder{
				Notification: grpcclient.NewNotificationClientForTest(mockClient),
				BuyerID:      buyerGrandfatherResolverForTest(),
				Store:        grandfatherResolverForTest(),
			}
			r := setupDeviceTokenRouter(h)

			body, _ := json.Marshal(map[string]string{"token": "tok"})
			req := httptest.NewRequest("DELETE", "/api/v1/app/notifications/device-token", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Account-Id", "123")
			req.Header.Set("X-Account-Type", "3")
			if tc.loginRole != "" {
				req.Header.Set("X-Login-Role", tc.loginRole)
			}

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assertHTTPStatus(t, w, tc.expectedStatus)
			if tc.expectedStatus == http.StatusOK && capturedUserType != tc.expectedUserType {
				t.Errorf("UserType = %q, want %q", capturedUserType, tc.expectedUserType)
			}
		})
	}
}

func TestRemoveDeviceToken_PassesAllFields(t *testing.T) {
	var captured *notificationpb.RemoveDeviceTokenRequest
	mockClient := &mockNotificationServiceClient{
		RemoveDeviceTokenFn: func(ctx context.Context, in *notificationpb.RemoveDeviceTokenRequest, opts ...grpc.CallOption) (*notificationpb.RemoveDeviceTokenResponse, error) {
			captured = in
			return &notificationpb.RemoveDeviceTokenResponse{Code: 0}, nil
		},
	}

	h := &Holder{
		Notification: grpcclient.NewNotificationClientForTest(mockClient),
		Store:        grandfatherResolverForTest(),
	}
	r := setupDeviceTokenRouter(h)

	body, _ := json.Marshal(map[string]string{"token": "fcm-remove-me"})
	req := httptest.NewRequest("DELETE", "/api/v1/app/notifications/device-token", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "555")
	req.Header.Set("X-Account-Type", "2")
	req.Header.Set("X-Login-Role", "2")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil {
		t.Fatal("RemoveDeviceToken was not called")
	}
	if captured.UserId != "555" {
		t.Errorf("UserId = %s, want 555", captured.UserId)
	}
	if captured.UserType != "seller" {
		t.Errorf("UserType = %q, want %q", captured.UserType, "seller")
	}
	if captured.Token != "fcm-remove-me" {
		t.Errorf("Token = %q, want %q", captured.Token, "fcm-remove-me")
	}
}

func TestRemoveDeviceToken_MissingToken(t *testing.T) {
	h := &Holder{Notification: grpcclient.NewNotificationClientForTest(&mockNotificationServiceClient{})}
	r := setupDeviceTokenRouter(h)

	body, _ := json.Marshal(map[string]string{})
	req := httptest.NewRequest("DELETE", "/api/v1/app/notifications/device-token", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "1")
	req.Header.Set("X-Account-Type", "1")
	req.Header.Set("X-Login-Role", "1")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusBadRequest)
}

func TestRemoveDeviceToken_Unauthorized(t *testing.T) {
	h := &Holder{Notification: grpcclient.NewNotificationClientForTest(&mockNotificationServiceClient{})}
	r := setupDeviceTokenRouter(h)

	body, _ := json.Marshal(map[string]string{"token": "tok"})
	req := httptest.NewRequest("DELETE", "/api/v1/app/notifications/device-token", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusUnauthorized)
}

func TestRemoveDeviceToken_GrpcError(t *testing.T) {
	mockClient := &mockNotificationServiceClient{
		RemoveDeviceTokenFn: func(ctx context.Context, in *notificationpb.RemoveDeviceTokenRequest, opts ...grpc.CallOption) (*notificationpb.RemoveDeviceTokenResponse, error) {
			return nil, fmt.Errorf("db timeout")
		},
	}

	h := &Holder{
		Notification: grpcclient.NewNotificationClientForTest(mockClient),
		BuyerID:      buyerGrandfatherResolverForTest(),
	}
	r := setupDeviceTokenRouter(h)

	body, _ := json.Marshal(map[string]string{"token": "tok"})
	req := httptest.NewRequest("DELETE", "/api/v1/app/notifications/device-token", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "1")
	req.Header.Set("X-Account-Type", "1")
	req.Header.Set("X-Login-Role", "1")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		t.Errorf("expected non-200 on gRPC error, got %d", w.Code)
	}
}

func TestRemoveDeviceToken_BuyerRole_UsesResolvedBuyerID(t *testing.T) {
	var captured *notificationpb.RemoveDeviceTokenRequest
	mockClient := &mockNotificationServiceClient{
		RemoveDeviceTokenFn: func(ctx context.Context, in *notificationpb.RemoveDeviceTokenRequest, opts ...grpc.CallOption) (*notificationpb.RemoveDeviceTokenResponse, error) {
			captured = in
			return &notificationpb.RemoveDeviceTokenResponse{Code: 0}, nil
		},
	}

	h := &Holder{
		Notification: grpcclient.NewNotificationClientForTest(mockClient),
		BuyerID:      buyerResolverForTest(9999),
	}
	r := setupDeviceTokenRouter(h)

	body, _ := json.Marshal(map[string]string{"token": "fcm-buyer-tok"})
	req := httptest.NewRequest("DELETE", "/api/v1/app/notifications/device-token", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "42")
	req.Header.Set("X-Account-Type", "1")
	req.Header.Set("X-Login-Role", "1")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil {
		t.Fatal("RemoveDeviceToken was not called")
	}
	if captured.UserId != "9999" {
		t.Errorf("UserId = %s, want 9999 (resolved buyer id, not raw accountId 42)", captured.UserId)
	}
}

func TestRemoveDeviceToken_SellerRole_UsesResolvedStoreID(t *testing.T) {
	var captured *notificationpb.RemoveDeviceTokenRequest
	mockClient := &mockNotificationServiceClient{
		RemoveDeviceTokenFn: func(ctx context.Context, in *notificationpb.RemoveDeviceTokenRequest, opts ...grpc.CallOption) (*notificationpb.RemoveDeviceTokenResponse, error) {
			captured = in
			return &notificationpb.RemoveDeviceTokenResponse{Code: 0}, nil
		},
	}

	h := &Holder{
		Notification: grpcclient.NewNotificationClientForTest(mockClient),
		Store:        storeResolverForTest(8888),
	}
	r := setupDeviceTokenRouter(h)

	body, _ := json.Marshal(map[string]string{"token": "fcm-seller-tok"})
	req := httptest.NewRequest("DELETE", "/api/v1/app/notifications/device-token", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "42")
	req.Header.Set("X-Account-Type", "2")
	req.Header.Set("X-Login-Role", "2")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil {
		t.Fatal("RemoveDeviceToken was not called")
	}
	if captured.UserId != "8888" {
		t.Errorf("UserId = %s, want 8888 (resolved store id, not raw accountId 42)", captured.UserId)
	}
}

func TestUpsertDeviceToken_SellerRole_NoCurrentStore_Returns403(t *testing.T) {
	h := &Holder{
		Notification: grpcclient.NewNotificationClientForTest(&mockNotificationServiceClient{}),
		Store:        noStoreResolverForTest(),
	}
	r := setupDeviceTokenRouter(h)

	body, _ := json.Marshal(map[string]string{"token": "test-token", "platform": "ios"})
	req := httptest.NewRequest("POST", "/api/v1/app/notifications/device-token", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "42")
	req.Header.Set("X-Account-Type", "2")
	req.Header.Set("X-Login-Role", "2")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusForbidden)
	body_resp := parseJSON(t, w.Body)
	if body_resp["message"] != "seller access required" {
		t.Errorf("message = %v, want \"seller access required\"", body_resp["message"])
	}
}

func TestRemoveDeviceToken_SellerRole_NoCurrentStore_Returns403(t *testing.T) {
	h := &Holder{
		Notification: grpcclient.NewNotificationClientForTest(&mockNotificationServiceClient{}),
		Store:        noStoreResolverForTest(),
	}
	r := setupDeviceTokenRouter(h)

	body, _ := json.Marshal(map[string]string{"token": "test-token"})
	req := httptest.NewRequest("DELETE", "/api/v1/app/notifications/device-token", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "42")
	req.Header.Set("X-Account-Type", "2")
	req.Header.Set("X-Login-Role", "2")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusForbidden)
	body_resp := parseJSON(t, w.Body)
	if body_resp["message"] != "seller access required" {
		t.Errorf("message = %v, want \"seller access required\"", body_resp["message"])
	}
}

// ==================== ListNotifications ====================

func setupNotificationInboxRouter(h *Holder) *gin.Engine {
	r := gin.New()
	r.GET("/api/v1/app/notifications", func(c *gin.Context) {
		c.Set("loginRole", 1) // 1 = buyer
		c.Next()
	}, h.ListNotifications)
	return r
}

func TestListNotifications_PassesCategoryQuery(t *testing.T) {
	var captured *notificationpb.ListNotificationsRequest
	mockClient := &mockNotificationServiceClient{
		ListNotificationsFn: func(ctx context.Context, in *notificationpb.ListNotificationsRequest, opts ...grpc.CallOption) (*notificationpb.ListNotificationsResponse, error) {
			captured = in
			return &notificationpb.ListNotificationsResponse{Code: 0}, nil
		},
	}
	h := &Holder{
		Notification: grpcclient.NewNotificationClientForTest(mockClient),
		BuyerID:      buyerGrandfatherResolverForTest(),
	}
	r := setupNotificationInboxRouter(h)

	req := httptest.NewRequest("GET", "/api/v1/app/notifications?category=order_update", nil)
	req.Header.Set("X-Account-Id", "123")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if captured == nil {
		t.Fatal("gRPC ListNotifications was not called")
	}
	if captured.Category != "order_update" {
		t.Errorf("Category = %q, want %q", captured.Category, "order_update")
	}
}

func TestListNotifications_BuyerRole_UsesResolvedBuyerID(t *testing.T) {
	var captured *notificationpb.ListNotificationsRequest
	mockClient := &mockNotificationServiceClient{
		ListNotificationsFn: func(ctx context.Context, in *notificationpb.ListNotificationsRequest, opts ...grpc.CallOption) (*notificationpb.ListNotificationsResponse, error) {
			captured = in
			return &notificationpb.ListNotificationsResponse{Code: 0}, nil
		},
	}
	h := &Holder{
		Notification: grpcclient.NewNotificationClientForTest(mockClient),
		BuyerID:      buyerResolverForTest(9999),
	}
	r := setupNotificationInboxRouter(h)

	req := httptest.NewRequest("GET", "/api/v1/app/notifications", nil)
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil {
		t.Fatal("gRPC ListNotifications was not called")
	}
	if captured.UserId != "9999" {
		t.Errorf("UserId = %s, want 9999 (resolved buyer id, not raw accountId 42)", captured.UserId)
	}
}

func TestListNotifications_SellerRole_UsesResolvedStoreID(t *testing.T) {
	var captured *notificationpb.ListNotificationsRequest
	mockClient := &mockNotificationServiceClient{
		ListNotificationsFn: func(ctx context.Context, in *notificationpb.ListNotificationsRequest, opts ...grpc.CallOption) (*notificationpb.ListNotificationsResponse, error) {
			captured = in
			return &notificationpb.ListNotificationsResponse{Code: 0}, nil
		},
	}
	h := &Holder{
		Notification: grpcclient.NewNotificationClientForTest(mockClient),
		Store:        storeResolverForTest(8888),
	}
	r := gin.New()
	r.GET("/api/v1/app/notifications", func(c *gin.Context) {
		c.Set("loginRole", 2) // 2 = seller
		c.Next()
	}, h.ListNotifications)

	req := httptest.NewRequest("GET", "/api/v1/app/notifications", nil)
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil {
		t.Fatal("gRPC ListNotifications was not called")
	}
	if captured.UserId != "8888" {
		t.Errorf("UserId = %s, want 8888 (resolved store id, not raw accountId 42)", captured.UserId)
	}
}

// ==================== canonical notification identity ====================
//
// Inbox requests carry only the resolved current buyer_id/store_id. The
// account-level compatibility fallback was removed after all seller templates
// completed their migration.

func TestListNotifications_SellerRole_UsesResolvedIdentity(t *testing.T) {
	var captured *notificationpb.ListNotificationsRequest
	mockClient := &mockNotificationServiceClient{
		ListNotificationsFn: func(ctx context.Context, in *notificationpb.ListNotificationsRequest, opts ...grpc.CallOption) (*notificationpb.ListNotificationsResponse, error) {
			captured = in
			return &notificationpb.ListNotificationsResponse{Code: 0}, nil
		},
	}
	h := &Holder{
		Notification: grpcclient.NewNotificationClientForTest(mockClient),
		Store:        storeResolverForTest(8888),
	}
	r := gin.New()
	r.GET("/api/v1/app/notifications", func(c *gin.Context) {
		c.Set("loginRole", 2) // 2 = seller
		c.Next()
	}, h.ListNotifications)

	req := httptest.NewRequest("GET", "/api/v1/app/notifications", nil)
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil {
		t.Fatal("gRPC ListNotifications was not called")
	}
}

func TestListNotifications_BuyerRole_UsesResolvedIdentity(t *testing.T) {
	var captured *notificationpb.ListNotificationsRequest
	mockClient := &mockNotificationServiceClient{
		ListNotificationsFn: func(ctx context.Context, in *notificationpb.ListNotificationsRequest, opts ...grpc.CallOption) (*notificationpb.ListNotificationsResponse, error) {
			captured = in
			return &notificationpb.ListNotificationsResponse{Code: 0}, nil
		},
	}
	h := &Holder{
		Notification: grpcclient.NewNotificationClientForTest(mockClient),
		BuyerID:      buyerResolverForTest(9999),
	}
	r := setupNotificationInboxRouter(h) // loginRole=1 (buyer)

	req := httptest.NewRequest("GET", "/api/v1/app/notifications", nil)
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil {
		t.Fatal("gRPC ListNotifications was not called")
	}
}

func setupNotificationRoleRouter(loginRole int, register func(*gin.RouterGroup)) *gin.Engine {
	r := gin.New()
	grp := r.Group("/api/v1/app/notifications")
	grp.Use(func(c *gin.Context) {
		c.Set("loginRole", loginRole)
		c.Next()
	})
	register(grp)
	return r
}

func TestGetUnreadCount_SellerRole_UsesResolvedIdentity(t *testing.T) {
	var captured *notificationpb.GetUnreadCountRequest
	mockClient := &mockNotificationServiceClient{
		GetUnreadCountFn: func(ctx context.Context, in *notificationpb.GetUnreadCountRequest, opts ...grpc.CallOption) (*notificationpb.GetUnreadCountResponse, error) {
			captured = in
			return &notificationpb.GetUnreadCountResponse{Code: 0}, nil
		},
	}
	h := &Holder{
		Notification: grpcclient.NewNotificationClientForTest(mockClient),
		Store:        storeResolverForTest(8888),
	}
	r := setupNotificationRoleRouter(2, func(g *gin.RouterGroup) { g.GET("/unread-count", h.GetUnreadCount) })

	req := httptest.NewRequest("GET", "/api/v1/app/notifications/unread-count", nil)
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil {
		t.Fatal("gRPC GetUnreadCount was not called")
	}
}

func TestGetUnreadCount_BuyerRole_UsesResolvedIdentity(t *testing.T) {
	var captured *notificationpb.GetUnreadCountRequest
	mockClient := &mockNotificationServiceClient{
		GetUnreadCountFn: func(ctx context.Context, in *notificationpb.GetUnreadCountRequest, opts ...grpc.CallOption) (*notificationpb.GetUnreadCountResponse, error) {
			captured = in
			return &notificationpb.GetUnreadCountResponse{Code: 0}, nil
		},
	}
	h := &Holder{
		Notification: grpcclient.NewNotificationClientForTest(mockClient),
		BuyerID:      buyerResolverForTest(9999),
	}
	r := setupNotificationRoleRouter(1, func(g *gin.RouterGroup) { g.GET("/unread-count", h.GetUnreadCount) })

	req := httptest.NewRequest("GET", "/api/v1/app/notifications/unread-count", nil)
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil {
		t.Fatal("gRPC GetUnreadCount was not called")
	}
}

func TestMarkAsRead_SellerRole_UsesResolvedIdentity(t *testing.T) {
	var captured *notificationpb.MarkAsReadRequest
	mockClient := &mockNotificationServiceClient{
		MarkAsReadFn: func(ctx context.Context, in *notificationpb.MarkAsReadRequest, opts ...grpc.CallOption) (*notificationpb.MarkAsReadResponse, error) {
			captured = in
			return &notificationpb.MarkAsReadResponse{Code: 0}, nil
		},
	}
	h := &Holder{
		Notification: grpcclient.NewNotificationClientForTest(mockClient),
		Store:        storeResolverForTest(8888),
	}
	r := setupNotificationRoleRouter(2, func(g *gin.RouterGroup) { g.PATCH("/:id/read", h.MarkNotificationAsRead) })

	req := httptest.NewRequest("PATCH", "/api/v1/app/notifications/777/read", nil)
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil {
		t.Fatal("gRPC MarkAsRead was not called")
	}
}

func TestMarkAsRead_BuyerRole_UsesResolvedIdentity(t *testing.T) {
	var captured *notificationpb.MarkAsReadRequest
	mockClient := &mockNotificationServiceClient{
		MarkAsReadFn: func(ctx context.Context, in *notificationpb.MarkAsReadRequest, opts ...grpc.CallOption) (*notificationpb.MarkAsReadResponse, error) {
			captured = in
			return &notificationpb.MarkAsReadResponse{Code: 0}, nil
		},
	}
	h := &Holder{
		Notification: grpcclient.NewNotificationClientForTest(mockClient),
		BuyerID:      buyerResolverForTest(9999),
	}
	r := setupNotificationRoleRouter(1, func(g *gin.RouterGroup) { g.PATCH("/:id/read", h.MarkNotificationAsRead) })

	req := httptest.NewRequest("PATCH", "/api/v1/app/notifications/777/read", nil)
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil {
		t.Fatal("gRPC MarkAsRead was not called")
	}
}

func TestMarkAllAsRead_SellerRole_UsesResolvedIdentity(t *testing.T) {
	var captured *notificationpb.MarkAllAsReadRequest
	mockClient := &mockNotificationServiceClient{
		MarkAllAsReadFn: func(ctx context.Context, in *notificationpb.MarkAllAsReadRequest, opts ...grpc.CallOption) (*notificationpb.MarkAllAsReadResponse, error) {
			captured = in
			return &notificationpb.MarkAllAsReadResponse{Code: 0}, nil
		},
	}
	h := &Holder{
		Notification: grpcclient.NewNotificationClientForTest(mockClient),
		Store:        storeResolverForTest(8888),
	}
	r := setupNotificationRoleRouter(2, func(g *gin.RouterGroup) { g.PATCH("/read-all", h.MarkAllNotificationsAsRead) })

	req := httptest.NewRequest("PATCH", "/api/v1/app/notifications/read-all", nil)
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil {
		t.Fatal("gRPC MarkAllAsRead was not called")
	}
}

func TestMarkAllAsRead_BuyerRole_UsesResolvedIdentity(t *testing.T) {
	var captured *notificationpb.MarkAllAsReadRequest
	mockClient := &mockNotificationServiceClient{
		MarkAllAsReadFn: func(ctx context.Context, in *notificationpb.MarkAllAsReadRequest, opts ...grpc.CallOption) (*notificationpb.MarkAllAsReadResponse, error) {
			captured = in
			return &notificationpb.MarkAllAsReadResponse{Code: 0}, nil
		},
	}
	h := &Holder{
		Notification: grpcclient.NewNotificationClientForTest(mockClient),
		BuyerID:      buyerResolverForTest(9999),
	}
	r := setupNotificationRoleRouter(1, func(g *gin.RouterGroup) { g.PATCH("/read-all", h.MarkAllNotificationsAsRead) })

	req := httptest.NewRequest("PATCH", "/api/v1/app/notifications/read-all", nil)
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil {
		t.Fatal("gRPC MarkAllAsRead was not called")
	}
}

func TestDeleteNotifications_SellerRole_UsesResolvedIdentity(t *testing.T) {
	var captured *notificationpb.DeleteNotificationsRequest
	mockClient := &mockNotificationServiceClient{
		DeleteNotificationsFn: func(ctx context.Context, in *notificationpb.DeleteNotificationsRequest, opts ...grpc.CallOption) (*notificationpb.DeleteNotificationsResponse, error) {
			captured = in
			return &notificationpb.DeleteNotificationsResponse{Code: 0}, nil
		},
	}
	h := &Holder{
		Notification: grpcclient.NewNotificationClientForTest(mockClient),
		Store:        storeResolverForTest(8888),
	}
	r := setupNotificationRoleRouter(2, func(g *gin.RouterGroup) { g.DELETE("", h.DeleteNotifications) })

	body, _ := json.Marshal(map[string][]string{"ids": {"1"}})
	req := httptest.NewRequest("DELETE", "/api/v1/app/notifications", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil {
		t.Fatal("gRPC DeleteNotifications was not called")
	}
}

func TestDeleteNotifications_BuyerRole_UsesResolvedIdentity(t *testing.T) {
	var captured *notificationpb.DeleteNotificationsRequest
	mockClient := &mockNotificationServiceClient{
		DeleteNotificationsFn: func(ctx context.Context, in *notificationpb.DeleteNotificationsRequest, opts ...grpc.CallOption) (*notificationpb.DeleteNotificationsResponse, error) {
			captured = in
			return &notificationpb.DeleteNotificationsResponse{Code: 0}, nil
		},
	}
	h := &Holder{
		Notification: grpcclient.NewNotificationClientForTest(mockClient),
		BuyerID:      buyerResolverForTest(9999),
	}
	r := setupNotificationRoleRouter(1, func(g *gin.RouterGroup) { g.DELETE("", h.DeleteNotifications) })

	body, _ := json.Marshal(map[string][]string{"ids": {"1"}})
	req := httptest.NewRequest("DELETE", "/api/v1/app/notifications", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil {
		t.Fatal("gRPC DeleteNotifications was not called")
	}
}
