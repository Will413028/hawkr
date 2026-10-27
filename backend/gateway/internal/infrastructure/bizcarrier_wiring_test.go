package infrastructure

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Will413028/hawkr/gateway/internal/bizcarrier"
	"github.com/Will413028/hawkr/gateway/internal/config"
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// bizcarrier.Middleware 必須掛在 newEngine 的全域鏈上。
//
// 為什麼這需要一支專屬的測試：carrier 不在場時 EnvelopeGuardInterceptor 會**撤銷**
// preserveData 特權（Arm 回 false ⇒ 落回一般錯誤映射）。那是刻意的 fail-closed
// 降級——不會產生假成功——但它會讓 9331／4001 這些碼的 data 在 gRPC 邊界蒸發，
// 前端拿不到 providerRequestId／unavailableItems，症狀是「按鈕點了沒反應」。
//
// 沒有這一行，那個降級是靜默的：服務照常啟動、所有既有測試照常綠（handler 層的
// e2e 測試自己掛 carrier），只有真實流量會少掉 data。
//
// 斷言方式是**行為**不是名稱比對：直接在 engine 上跑一次 Arm。名稱比對
// （runtime.FuncForPC）證明得了「鏈上有一個叫 Middleware 的東西」，證明不了
// 「它真的把 holder 放進了 c.Request 的 context」。
func TestNewEngine_MountsBizCarrier(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := newEngine(&config.Config{}, zap.NewNop(), &handler.Holder{})

	// 註冊在 newEngine 之後仍會套用全域鏈：gin 在註冊路由的當下才把 engine.Handlers
	// 複製進該路由，而 r.Use(...) 全部發生在 newEngine 內。
	var armed bool
	r.GET("/__bizcarrier_probe", func(c *gin.Context) {
		armed = bizcarrier.Arm(c.Request.Context(), "/probe.Service/Probe", 9331, "probe")
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/__bizcarrier_probe", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("probe 沒被執行：HTTP %d（若是 404，表示全域鏈上有東西提前 abort）", w.Code)
	}
	if !armed {
		t.Fatal("bizcarrier.Middleware 不在 newEngine 的全域鏈上：" +
			"preserveData 業務碼會在 gRPC 邊界降級成無 data 的錯誤（見 internal/bizcarrier 的 package doc）")
	}
}
