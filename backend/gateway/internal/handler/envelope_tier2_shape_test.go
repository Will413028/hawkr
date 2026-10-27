package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	notificationpb "github.com/Will413028/hawkr/go-pkg/proto/notification"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

// Tier-2 envelope 收斂（issue #2198）：Page-trap 之外還有一批端點把業務欄位掛在
// envelope 頂層與 code/message 並列，或用 `msg` 當欄名。收斂後一律
// {code, message, data:{...}}，唯一登記在案的例外是 /health（10 檔 §2 例外清單）。
//
// 這組測試對每支端點打真實路由，斷言兩件事：
//   - envelope 頂層只允許 code/message/data/traceId/messageZh，業務欄位不得外洩
//   - 該業務欄位確實出現在 data 之下、值正確
//
// 弄壞任何一支（把欄位搬回頂層、或把 message 改回 msg）都會讓對應 case 轉紅。

// envelopeTopLevelKeys 是 envelope 頂層允許出現的 key 全集。
var envelopeTopLevelKeys = map[string]bool{
	"code": true, "message": true, "data": true, "traceId": true, "messageZh": true,
}

// assertEnvelopeData 斷言頂層乾淨並回傳 data 物件。
func assertEnvelopeData(t *testing.T, body map[string]interface{}) map[string]interface{} {
	t.Helper()
	for k := range body {
		if !envelopeTopLevelKeys[k] {
			t.Fatalf("envelope 頂層出現非契約欄位 %q（10 檔 §2：業務資料一律在 data 之下）；body = %v", k, body)
		}
	}
	if _, isMsg := body["msg"]; isMsg {
		t.Fatalf("envelope 用了 msg 欄名（只有 message 是合法的）；body = %v", body)
	}
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("data 不是物件；body = %v", body)
	}
	return data
}

func doGetOn(t *testing.T, r *gin.Engine, path string) (*httptest.ResponseRecorder, map[string]interface{}) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w, parseJSON(t, w.Body)
}

func TestEnvelopeTier2_AdminReloadTemplates(t *testing.T) {
	mock := &mockNotificationServiceClient{
		ReloadTemplatesFn: func(ctx context.Context, in *notificationpb.ReloadTemplatesRequest, opts ...grpc.CallOption) (*notificationpb.ReloadTemplatesResponse, error) {
			return &notificationpb.ReloadTemplatesResponse{Code: 0, ReloadedCount: "5"}, nil
		},
	}
	h := &Holder{Notification: grpcclient.NewNotificationClientForTest(mock)}
	r := gin.New()
	r.POST("/api/v1/admin/notification-templates/reload", h.AdminReloadTemplates)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/notification-templates/reload", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	data := assertEnvelopeData(t, body)
	if data["reloadedCount"] != "5" {
		t.Errorf("data.reloadedCount = %v, want \"5\"", data["reloadedCount"])
	}
}

// TestEnvelopeTier2_HealthIsTheOnlyException 釘住 10 檔 §2 例外清單：
// /health 刻意不是 envelope（k8s probe 形狀）。這支若哪天被「順手收斂」成 envelope，
// 探針的判讀就變了——所以反向斷言它維持 probe 形狀。
func TestEnvelopeTier2_HealthIsTheOnlyException(t *testing.T) {
	h := &Holder{}
	r := gin.New()
	r.GET("/health", h.HealthSelf)
	w, body := doGetOn(t, r, "/health")
	assertHTTPStatus(t, w, http.StatusOK)
	if body["status"] != "ok" {
		t.Errorf("health 應維持 {\"status\":\"ok\"} probe 形狀；got %v", body)
	}
	if _, hasCode := body["code"]; hasCode {
		t.Errorf("health 不該被收斂成 envelope（10 檔 §2 例外清單第 1 條）；got %v", body)
	}
}
