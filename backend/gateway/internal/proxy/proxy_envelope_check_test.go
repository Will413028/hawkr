package proxy

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// #2198：上游不可達時原本回 {"code":502,"msg":"..."}，欄名不合契約，且是字串串接組出來的
// ——err 裡有雙引號就會產出壞掉的 JSON。這裡同時釘住欄名與可解析性。
func TestWriteUpstreamFailureEnvelope_ShapeAndEscaping(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/app/whatever", nil)
	writeUpstreamFailureEnvelope(w, r, FailClientFacing, errors.New(`dial tcp "10.0.0.1:9999": refused`))

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("回應不是合法 JSON（err 內的雙引號未 escape）：%v；body=%s", err, w.Body.String())
	}
	if _, leaked := body["msg"]; leaked {
		t.Errorf("仍使用 msg 欄名（契約只有 message）；body=%v", body)
	}
	if _, ok := body["message"].(string); !ok {
		t.Errorf("缺少 message；body=%v", body)
	}
	if body["code"] != float64(424) {
		t.Errorf("code = %v, want 424（鏡射 HTTP status）", body["code"])
	}
}

// 上游錯誤的**內容**不得出現在回應 body。
//
// 這條是 424 那個改動直接帶出來的：502 時代 Cloudflare 會把整個 body 換掉，所以
// `"service unavailable: " + err.Error()` 從來沒有真的送到使用者面前；改成 4xx 之後
// envelope 是真的送達的，而 seller App 的 AppException 直接把 `message` 當文案顯示
// （`lib/core/exceptions/app_exception.dart:26-44`）⇒ 使用者會在畫面上看到
// `dial tcp 10.x.x.x:9936: connect: connection refused`，內部拓撲就這樣外洩。
//
// 兩個 status 都要驗：PayUni 那條雖然被 CF 擋著，但那是巧合不是設計。
func TestWriteUpstreamFailureEnvelope_DoesNotLeakUpstreamError(t *testing.T) {
	for _, status := range []UpstreamFailureStatus{FailClientFacing, FailThirdPartyWebhook} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/app/media/secure_download", nil)
		writeUpstreamFailureEnvelope(w, r, status,
			errors.New(`dial tcp 10.42.7.13:9936: connect: connection refused`))

		raw := w.Body.String()
		for _, secret := range []string{"10.42.7.13", "9936", "dial tcp", "connection refused"} {
			if strings.Contains(raw, secret) {
				t.Errorf("status=%d 的回應 body 洩漏上游錯誤內容 %q —— 這句會直接顯示在使用者畫面上；"+
					"err 只准進 log，body 用固定文案。body=%s", int(status), secret, raw)
			}
		}
		var body struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("回應不是合法 JSON：%v", err)
		}
		if body.Message != upstreamFailureMessage {
			t.Errorf("status=%d 的 message = %q, want 固定文案 %q", int(status), body.Message, upstreamFailureMessage)
		}
	}
}

// 上游不可達時回什麼 status，取決於**這條路由的消費者是誰**，兩類的正確答案剛好相反。
//
//   - 我們自己的前端 → 424：Cloudflare 會把 5xx 的 body 整個換成 text/plain 錯誤頁，
//     前端的 response.json() 直接拋錯，envelope 永遠送不到（同 9280 的形狀，b723abc4c）。
//   - 第三方 webhook 送信端 → **必須維持 5xx**：5xx 對 PayUni 是「等一下重送」、4xx 是
//     「別再送了」。而 10 號設計書 §4.5.3／§4.5.5.D 的整套失敗回收（T2 失敗 → PayUni
//     重送 → dedup 命中 FAILED → replay 路徑接手）就是靠重送觸發的；改成 4xx 等於把一個
//     可恢復的中斷變成付款狀態永久不同步，而且沒有任何錯誤訊號。
//
// 這支測試存在的理由就是後者：424 一旦成為「先例」，下一個做同類收斂的人很容易把
// webhook 這條一起改掉，而那個回歸在測試與 CI 上完全不會有徵兆。
func TestUpstreamFailureStatus_ByConsumerClass(t *testing.T) {
	cases := []struct {
		name       string
		status     UpstreamFailureStatus
		wantStatus int
	}{
		{"client-facing（web/admin/App）→ 424，避開 Cloudflare 換 body", FailClientFacing, http.StatusFailedDependency},
		{"third-party webhook（PayUni）→ 502，保住對方的重送行為", FailThirdPartyWebhook, http.StatusBadGateway},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/whatever", nil)
			writeUpstreamFailureEnvelope(w, r, tc.status, errors.New("boom"))

			if w.Code != tc.wantStatus {
				t.Fatalf("HTTP status = %d, want %d", w.Code, tc.wantStatus)
			}
			var body map[string]interface{}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("回應不是合法 JSON：%v；body=%s", err, w.Body.String())
			}
			if body["code"] != float64(tc.wantStatus) {
				t.Errorf("envelope code = %v, want %d（鏡射 HTTP status）", body["code"], tc.wantStatus)
			}
		})
	}
}

// FailClientFacing 必須落在 4xx——這是「Cloudflare 不換 body」的硬前提，不是風格偏好。
// 對齊 response.TestBizHTTPStatus_NoServerErrorStatuses 的同一條規則。
func TestFailClientFacing_IsClientErrorStatus(t *testing.T) {
	if FailClientFacing < 400 || FailClientFacing > 499 {
		t.Fatalf("FailClientFacing = %d，必須是 4xx：origin 回 5xx 時 Cloudflare 會把 body "+
			"換成自己的錯誤頁，前端拿不到 envelope", int(FailClientFacing))
	}
}

// 反向：FailThirdPartyWebhook 必須留在 5xx。
func TestFailThirdPartyWebhook_IsServerErrorStatus(t *testing.T) {
	if FailThirdPartyWebhook < 500 || FailThirdPartyWebhook > 599 {
		t.Fatalf("FailThirdPartyWebhook = %d，必須是 5xx：第三方送信端只有對 5xx 才會重送，"+
			"而 10 號設計書 §4.5.5.D 的失敗回收完全依賴 PayUni 重送", int(FailThirdPartyWebhook))
	}
}

// #2227：502/424 都是錯誤回應，必須帶 traceId（response.Err 系列一律帶；proxy 這條路徑原本
// 漏了，讓上游不可達的請求在 Loki 上串不起來）。TracingMiddleware 把 traceId 寫在**回應**
// header，所以這裡首選 w.Header()；tracer 未啟用時退回請求 header。
func TestWriteUpstreamFailureEnvelope_TraceID(t *testing.T) {
	t.Run("from response header (TracingMiddleware)", func(t *testing.T) {
		w := httptest.NewRecorder()
		w.Header().Set("x-trace-id", "resp-trace-1")
		r := httptest.NewRequest(http.MethodGet, "/api/v1/app/whatever", nil)
		r.Header.Set("x-trace-id", "req-trace-1")
		writeUpstreamFailureEnvelope(w, r, FailClientFacing, errors.New("boom"))

		var body map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("回應不是合法 JSON：%v", err)
		}
		if body["traceId"] != "resp-trace-1" {
			t.Errorf("traceId = %v, want resp-trace-1", body["traceId"])
		}
	})

	t.Run("fallback to request header", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/app/whatever", nil)
		r.Header.Set("x-trace-id", "req-trace-2")
		writeUpstreamFailureEnvelope(w, r, FailClientFacing, errors.New("boom"))

		var body map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("回應不是合法 JSON：%v", err)
		}
		if body["traceId"] != "req-trace-2" {
			t.Errorf("traceId = %v, want req-trace-2", body["traceId"])
		}
	})

	t.Run("absent when unknown", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/app/whatever", nil)
		writeUpstreamFailureEnvelope(w, r, FailThirdPartyWebhook, errors.New("boom"))

		var body map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("回應不是合法 JSON：%v", err)
		}
		if _, present := body["traceId"]; present {
			t.Errorf("取不到 traceId 時不應輸出該欄位；body=%v", body)
		}
	})
}
