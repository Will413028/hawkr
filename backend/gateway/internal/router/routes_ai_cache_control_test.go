package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/gin-gonic/gin"
)

// aiNoStorePrefixes 是「回應一律 private, no-store」的路由前綴（#2447 BE-01A）。
// 下面的測試直接掃 engine.Routes() 再用這組前綴過濾，而不是手抄一份路由清單：
// 之後在**這些前綴底下**新增的端點會自動被納入檢查。前綴之外新增的 AI 端點仍需
// 有人來加一條——這個測試守不到，靠的是 API 設計規範「回應快取政策」那節。
//
// 前綴用 appPrefix／APIV1Path 組出來而不是寫死 "/api/v1/app"：namespace 常數改了
// 這裡要跟著壞，不能靜默失配成 0 條。
var aiNoStorePrefixes = []string{
	appPrefix + "/sellers/me/assistant",
	appPrefix + "/buyers/me/assistant",
	appPrefix + "/buyers/me/procurement",
	appPrefix + AIStylesPath,
}

// aiNoStoreRouteFloor 是上述前綴目前的路由數（8 賣家助手 + 7 買家助手 + 20 採購
// （issue #2403 Slice 2 新增 POST rfq-price-previews，Slice 3 新增 POST
// rfq-confirmations；Task 14 新增 POST runs/:runId/cancel／resume）+ 1 賣家報價
// 確認 + 1 public 報價確認 + 1 AI 風格列表）。這個下限守的是過濾器本身：路由前綴
// 被改名或群組被搬走時，過濾結果會靜默變成 0，測試卻照樣綠。新增路由讓實際數字
// 超過下限不會失敗（新路由已自動被檢查）；數字掉下來才失敗。
const aiNoStoreRouteFloor = 38

// TestAIRoutes_ResponsesArePrivateNoStore 釘住 AI／採購端點的回應快取政策。
//
// 這是唯一走真實中介層鏈的層級：handler 包的測試（ai_procurement_unary_test.go 的
// setupProcurementUnaryRouter 等）都自建 gin.New() 只掛 fakeJWTAuth，中介層漏掛在
// 那裡看不出來。這裡刻意不帶憑證：多數路由會在 JWTAuth 早退成 401，而「被共享快取
// 留下的 401」正是這張票要防的東西——PrivateNoStore 必須排在 JWTAuth 之前才會綠。
func TestAIRoutes_ResponsesArePrivateNoStore(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// legacy 的 public 報價確認路由已隨 C3 退場，這裡不再有免認證、會真的打進 client
	// 的路由，所以不需要接 AIProcurement stub。
	h := &handler.Holder{}
	r := gin.New()
	app := r.Group("/api/v1/app")
	admin := r.Group("/api/v1/admin")
	registerAIRoutes(app, admin, h)
	registerProcurementQuoteRoutes(app, h)

	checked := 0
	for _, ri := range r.Routes() {
		if !underAnyPrefix(ri.Path, aiNoStorePrefixes) {
			continue
		}
		checked++

		var body *strings.Reader
		switch ri.Method {
		case http.MethodPost, http.MethodPatch, http.MethodPut:
			// public 報價確認以 body 裡的 token 為憑證；其餘路由在 JWTAuth 就退掉，
			// body 內容不影響斷言。
			body = strings.NewReader(`{"token":"x"}`)
		default:
			body = strings.NewReader("")
		}
		req := httptest.NewRequest(ri.Method, fillRouteParams(ri.Path), body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if got := w.Header().Get("Cache-Control"); got != "private, no-store" {
			t.Errorf("%s %s: Cache-Control = %q, want %q (status was %d)",
				ri.Method, ri.Path, got, "private, no-store", w.Code)
		}
	}

	if checked < aiNoStoreRouteFloor {
		t.Errorf("only %d AI/procurement routes matched %v, want at least %d — 前綴被改名或群組被搬走了？",
			checked, aiNoStorePrefixes, aiNoStoreRouteFloor)
	}
}

// fillRouteParams 把 gin 的 :param 佔位符換成任意值，讓請求真的命中該路由。
func fillRouteParams(path string) string {
	segments := strings.Split(path, "/")
	for i, seg := range segments {
		if strings.HasPrefix(seg, ":") {
			segments[i] = "x1"
		}
	}
	return strings.Join(segments, "/")
}
