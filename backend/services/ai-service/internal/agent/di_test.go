package agent

import "testing"

func TestButtonRunnerKeys(t *testing.T) {
	// 只有 ButtonKey != "" 的 specialist 會有 button runner
	got := map[string]bool{}
	for _, s := range sellerSpecialistSpecs() {
		if s.ButtonKey != "" {
			got["seller:"+s.Key] = true
		}
	}
	for _, s := range buyerSpecialistSpecs() {
		if s.ButtonKey != "" {
			got["buyer:"+s.Key] = true
		}
	}
	want := []string{"seller:inventory", "seller:payment",
		"buyer:order", "buyer:tracking", "buyer:seller_match"}
	if len(got) != len(want) {
		t.Fatalf("button runner 應有 %d 個，得 %d：%v", len(want), len(got), got)
	}
	for _, k := range want {
		if !got[k] {
			t.Errorf("缺 button runner key %q", k)
		}
	}
	// seller:order 不應有按鈕（/orders 特例）
	if got["seller:order"] {
		t.Error("seller:order 不應有 button runner（/orders 為特例流程）")
	}
}
