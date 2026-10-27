package agent

import (
	"strings"
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/config"
	"github.com/Will413028/hawkr/ai-service/internal/llmprovider"
	"go.uber.org/zap"
)

func TestProductListingInstruction_KeyPhrases(t *testing.T) {
	required := []struct {
		phrase string
		reason string
	}{
		{
			phrase: "批量",
			reason: "必須明確指示批量提取策略",
		},
		{
			phrase: "一次",
			reason: "update_draft 每輪只呼叫一次",
		},
		{
			phrase: "缺漏欄位一次列出",
			reason: "後續缺漏欄位應一次詢問，而非逐欄詢問",
		},
		{
			phrase: "search_category",
			reason: "分類必須走 search_category，禁止自行猜測",
		},
		{
			phrase: "update_draft",
			reason: "工具名稱必須存在",
		},
	}

	for _, r := range required {
		if !strings.Contains(productListingInstruction, r.phrase) {
			t.Errorf("productListingInstruction 缺少關鍵片段 %q（原因：%s）", r.phrase, r.reason)
		}
	}
}

func TestNewAgents_MapsPopulated(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() 非預期 error: %v", err)
	}
	if cfg.GeminiAPIKey == "" {
		t.Skip("GEMINI_API_KEY not set")
	}
	agents, err := NewAgents(llmprovider.NewAssistant(cfg, zap.NewNop()), &ToolDeps{})
	if err != nil {
		t.Fatalf("NewAgents: %v", err)
	}
	for _, k := range []string{"seller:order", "seller:inventory", "seller:payment", "seller:price",
		"buyer:order", "buyer:tracking", "buyer:seasonal", "buyer:seller_match"} {
		if agents.Specialists[k] == nil {
			t.Errorf("Specialists[%q] 為 nil", k)
		}
	}
	for _, k := range []string{"seller", "buyer"} {
		if agents.Coordinators[k] == nil {
			t.Errorf("Coordinators[%q] 為 nil", k)
		}
	}
	if agents.ProductAutofill == nil || agents.ProductListing == nil || agents.OrderPresenter == nil {
		t.Error("保留 agents 為 nil")
	}
}

func TestBuyerCoordinatorExcluded_OnlySellerMatch(t *testing.T) {
	for _, spec := range buyerSpecialistSpecs() {
		want := spec.Key == "seller_match"
		if got := buyerCoordinatorExcluded[spec.Key]; got != want {
			t.Errorf("buyerCoordinatorExcluded[%q] = %v, want %v", spec.Key, got, want)
		}
	}
}
