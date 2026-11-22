package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	notificationpb "github.com/Will413028/hawkr/go-pkg/proto/notification"
	"google.golang.org/grpc"
)

// ==================== Direction A: Tier-1 msg→message ====================

// After converging the notification/misc emitters to response.*, every Tier-1
// success/error envelope must carry the canonical "message" field and must NOT
// emit the legacy "msg" key. This guards the RemoveDeviceToken success path
// (response.OKMsg(c, "ok", nil)) which previously emitted {"code":0,"msg":"ok"}.
//
// #2227 收緊一階：成功回應的 message 恆為 "success"（10 檔 §2），Gateway 不得自造
// 成功訊息——原本這裡回的 "ok" 就是那個 class 的一員，已改用 response.OK。
func TestRemoveDeviceToken_UsesMessageNotMsg(t *testing.T) {
	mockClient := &mockNotificationServiceClient{
		RemoveDeviceTokenFn: func(ctx context.Context, in *notificationpb.RemoveDeviceTokenRequest, opts ...grpc.CallOption) (*notificationpb.RemoveDeviceTokenResponse, error) {
			return &notificationpb.RemoveDeviceTokenResponse{Code: 0}, nil
		},
	}
	h := &Holder{
		Notification: grpcclient.NewNotificationClientForTest(mockClient),
		BuyerID:      buyerGrandfatherResolverForTest(),
	}
	r := setupDeviceTokenRouter(h)

	body, _ := json.Marshal(map[string]string{"token": "test_token"})
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/app/notifications/device-token", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "123")
	req.Header.Set("X-Login-Role", "1")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 0)

	if _, hasMsg := resp["msg"]; hasMsg {
		t.Errorf("response must not contain legacy \"msg\" key; body = %v", resp)
	}
	msg, ok := resp["message"].(string)
	if !ok {
		t.Fatalf("response missing \"message\" field; body = %v", resp)
	}
	if msg != "success" {
		t.Errorf("message = %q, want %q", msg, "success")
	}
}
