package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

var envelopeTopLevelKeys = map[string]bool{
	"code": true, "message": true, "data": true, "traceId": true, "messageZh": true,
}

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
