package llmprovider

import (
	"go.uber.org/zap"

	"github.com/Will413028/hawkr/ai-service/internal/config"
)

// ProcurementForTest 讓傳輸層測試把 OpenAI 相容 provider 指向 httptest.Server。
//
// 只給測試用，正式路徑一律走 NewProcurement。存在的理由是「實際送上 wire 的
// request body 長什麼樣」必須可斷言——DeepSeek 的 reasoning.effort=none 一旦
// 漏掉，任何強制 tool_choice 的請求在正式端點都是 HTTP 400，而 400 不可重試，
// 表現出來只是「模型永遠失敗、一路退回模板」的靜默失敗。
//
// 放在非 _test.go 是因為呼叫端散在別的 package 的測試裡（usecase 等），
// _test.go 的內容只有同一個 package 看得到。
func ProcurementForTest(cfg *config.Config, baseURL string) Procurement {
	return newProcurement(cfg, zap.NewNop(), baseURL)
}
