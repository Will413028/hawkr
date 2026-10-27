package llmprovider

import (
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/config"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"go.uber.org/zap"
)

// 這支檔案原本住在 internal/client/gemini（承載真正呼叫 Gemini SDK 的舊
// adapter，那份 SDK 用法讓金鑰走 URL query，#3519 Task 5 已把它與其唯一
// caller——Chat／ProductQA／GenerateImage／GenerateRecommendations 等死
// RPC——一併刪除）。gauge 本身隨 NewAssistant 搬到這裡：NewAssistant 是
// AIAssistantService（BuyerChat／SellerChat 等仍在服務的路徑）唯一的 LLM
// 建構點，也是唯一知道「GEMINI_API_KEY 有沒有真的接上」的地方。
//
// 🔴 兩個方向都要驗，且都要能被 sabotage 抓到：把 NewAssistant 裡任一個
// assistantProviderEnabled.Set(...) 拿掉或改錯值，這兩支測試裡至少有一支
// 要變紅。前置用 Set(-1) 卡哨兵值，排除「剛好已經是期望值」的偽陽性——
// 例如另一支測試或另一個分支殘留的值恰好等於這一支期望的值。

// GEMINI_API_KEY 未設定：gauge 必須真的被 Set 成 0，而不是「剛好是 0」。
func TestAssistantProviderEnabledGauge_KeyUnset(t *testing.T) {
	assistantProviderEnabled.Set(-1)
	a := NewAssistant(&config.Config{GeminiModel: "gemini-2.5-flash"}, zap.NewNop())
	if a.Enabled() {
		t.Fatal("key 未設定時 Assistant 不得回可用狀態")
	}
	if v := testutil.ToFloat64(assistantProviderEnabled); v != 0 {
		t.Fatalf("ai_assistant_provider_enabled 應為 0，實際 %v（-1 代表根本沒被 Set）", v)
	}
}

// GEMINI_API_KEY 已設定：gauge 必須真的被 Set 成 1。
func TestAssistantProviderEnabledGauge_KeySet(t *testing.T) {
	assistantProviderEnabled.Set(-1)
	a := NewAssistant(&config.Config{GeminiAPIKey: "k", GeminiModel: "gemini-2.5-flash"}, zap.NewNop())
	if !a.Enabled() {
		t.Fatal("key 已設定時 Assistant 應回可用狀態")
	}
	if v := testutil.ToFloat64(assistantProviderEnabled); v != 1 {
		t.Fatalf("ai_assistant_provider_enabled 應為 1，實際 %v（-1 代表根本沒被 Set）", v)
	}
}

// 停用的 LLM 必須在**生成時**才 fail-closed，不是「建構時就悄悄回一個能用的
// 東西」。這裡不重複驗證——disabledLLM 是 Procurement／Assistant 共用的同一個
// 型別、同一支 GenerateContent，已由 TestDisabledLLM_FailsClosedOnGenerate
// （provider_test.go）覆蓋；換一個建構它的呼叫端不會改變該方法本身的行為。
