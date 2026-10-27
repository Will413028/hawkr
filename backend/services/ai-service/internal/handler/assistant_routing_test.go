package handler

import (
	"testing"

	agentpkg "github.com/Will413028/hawkr/ai-service/internal/agent"
	"github.com/Will413028/hawkr/ai-service/internal/config"
	"google.golang.org/adk/v2/runner"
)

// TestChatRunner_AlwaysCoordinator 鎖定 all-tools fallback 移除後的行為：chat 端點
// 一律走該 role 的 coordinator，沒有第二條路徑可選。
//
// 移除理由見 docs/adr/2026-09-20-coordinator-vs-all-tools-fallback.md：實測 28 條
// query，coordinator 27/28、fallback 20/28，且 fallback 的主要失敗是一個 tool 都不叫、
// 直接用 prompt 回答。用 sentinel *runner.Runner 比對指標身分，不需 API key。
func TestChatRunner_AlwaysCoordinator(t *testing.T) {
	coordSeller := &runner.Runner{}
	coordBuyer := &runner.Runner{}
	runners := &agentpkg.Runners{
		Coordinators: map[string]*runner.Runner{"seller": coordSeller, "buyer": coordBuyer},
	}

	for _, tc := range []struct {
		role string
		want *runner.Runner
	}{
		{"seller", coordSeller},
		{"buyer", coordBuyer},
	} {
		t.Run(tc.role, func(t *testing.T) {
			if got := ChatRunner(&config.Config{}, runners, tc.role); got != tc.want {
				t.Errorf("ChatRunner(%q) 應回該 role 的 coordinator", tc.role)
			}
		})
	}
}

// TestButtonSpecialistKey_Contract 鎖定按鈕 endpointKey → Specialists map key 的契約。
func TestButtonSpecialistKey_Contract(t *testing.T) {
	want := map[string]string{
		"inventory":    "seller:inventory",
		"payment":      "seller:payment",
		"price":        "seller:price",
		"orders":       "buyer:order",
		"tracking":     "buyer:tracking",
		"seasonal":     "buyer:seasonal",
		"seller-match": "buyer:seller_match",
	}
	if len(ButtonSpecialistKey) != len(want) {
		t.Fatalf("ButtonSpecialistKey 應有 %d 筆，得 %d", len(want), len(ButtonSpecialistKey))
	}
	for ep, key := range want {
		if ButtonSpecialistKey[ep] != key {
			t.Errorf("ButtonSpecialistKey[%q]=%q，應為 %q", ep, ButtonSpecialistKey[ep], key)
		}
	}
	// 每個按鈕 endpoint 都要有預設訊息（DefaultMessages 已於 init() merge buyer keys）
	for ep := range ButtonSpecialistKey {
		if _, ok := DefaultMessages[ep]; !ok {
			t.Errorf("按鈕 endpoint %q 缺 DefaultMessages（送空 body 時會 400）", ep)
		}
	}
}
