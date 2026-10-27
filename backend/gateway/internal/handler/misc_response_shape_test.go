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
	reviewpb "github.com/Will413028/hawkr/go-pkg/proto/review"
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

// ==================== Direction B: Page-trap 收斂（review stats）====================

// GetProductReviews 的 stats 與分頁四欄全部收在 data 之下（10 檔 §2 統一 Envelope）。
// 本測試釘住兩件事：(a) 頂層不得再出現 stats/records/total/page/pageSize（Page-trap 回歸），
// (b) stats 這個端點特有的區塊沒有在收斂過程中被弄丟。
func TestGetProductReviews_StatsAndPaginationUnderData(t *testing.T) {
	mock := &reviewMockClient{
		GetProductReviewsFn: func(ctx context.Context, in *reviewpb.GetProductReviewsRequest, opts ...grpc.CallOption) (*reviewpb.ProductReviewsResponse, error) {
			return &reviewpb.ProductReviewsResponse{
				Code: 0,
				Reviews: []*reviewpb.ReviewInfo{
					{Id: "1", BuyerId: "2", Rating: 5, Content: "good"},
				},
				Stats: &reviewpb.ReviewStats{
					TotalCount:    3,
					AverageRating: 4.5,
					Rating_5Count: 2,
					Rating_4Count: 1,
				},
				Total:    3,
				Page:     1,
				PageSize: 20,
			}, nil
		},
	}
	h := &Holder{Review: grpcclient.NewReviewClientForTest(mock)}
	router := setupReviewRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/products/42/reviews", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 0)

	if _, leaked := resp["stats"]; leaked {
		t.Errorf("envelope 頂層不得出現 stats（應在 data 之下）; body = %v", resp)
	}
	data := pageData(t, resp)
	if len(pageRecords(t, resp)) != 1 {
		t.Errorf("data.records len = %d, want 1", len(pageRecords(t, resp)))
	}
	stats, ok := data["stats"].(map[string]interface{})
	if !ok {
		t.Fatalf("data.stats 必須存在（收斂不得弄丟 stats）; data = %v", data)
	}
	if _, ok := stats["totalCount"]; !ok {
		t.Errorf("stats missing total_count; stats = %v", stats)
	}
	if got := data["total"]; got != "3" {
		t.Errorf("data.total = %v, want \"3\"", got)
	}
	if got := data["pageSize"]; got != "20" {
		t.Errorf("data.pageSize = %v, want \"20\"", got)
	}
}
