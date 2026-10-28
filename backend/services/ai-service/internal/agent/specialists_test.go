package agent

import "testing"

func TestSellerSpecialistSpecs(t *testing.T) {
	specs := sellerSpecialistSpecs()
	wantKeys := map[string]string{ // key -> ButtonKey
		"order":     "", // /orders 特例，Phase 1 無按鈕
		"inventory": "inventory",
		"payment":   "payment",
	}
	if len(specs) != len(wantKeys) {
		t.Fatalf("seller specs 應有 %d 筆，得 %d", len(wantKeys), len(specs))
	}
	for _, s := range specs {
		btn, ok := wantKeys[s.Key]
		if !ok {
			t.Errorf("非預期 key %q", s.Key)
			continue
		}
		if s.ButtonKey != btn {
			t.Errorf("%s ButtonKey 應為 %q，得 %q", s.Key, btn, s.ButtonKey)
		}
		if s.Name == "" || s.Description == "" || s.Instruction == "" {
			t.Errorf("%s Name/Description/Instruction 不得為空", s.Key)
		}
		if s.buildTools == nil {
			t.Errorf("%s buildTools 不得為 nil", s.Key)
		}
	}
}

func TestBuyerSpecialistSpecs(t *testing.T) {
	specs := buyerSpecialistSpecs()
	want := map[string]string{"order": "orders", "tracking": "tracking", "seller_match": "seller-match"}
	if len(specs) != len(want) {
		t.Fatalf("buyer specs 應有 %d 筆，得 %d", len(want), len(specs))
	}
	for _, s := range specs {
		if btn, ok := want[s.Key]; !ok || s.ButtonKey != btn {
			t.Errorf("buyer spec %q ButtonKey 應為 %q，得 %q（ok=%v）", s.Key, btn, s.ButtonKey, ok)
		}
	}
}

// 驗證每個 spec 的 buildTools 產出「正確數量」的 tool（order specialist 掛 list+detail 共 2 個，
// 其餘各掛 1 個）。斷言精確數量而非非空，才能擋住 order spec 少掛第二個 tool 的靜默 regression。
func TestSpecialistSpecsBuildTools(t *testing.T) {
	deps := &ToolDeps{}
	for _, s := range append(sellerSpecialistSpecs(), buyerSpecialistSpecs()...) {
		want := 1
		if s.Key == "order" {
			want = 2
		}
		if got := len(s.buildTools(deps)); got != want {
			t.Errorf("%s buildTools 應產出 %d 個 tool，得 %d", s.Key, want, got)
		}
	}
}
