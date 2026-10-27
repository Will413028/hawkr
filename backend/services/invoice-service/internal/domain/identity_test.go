package domain

import (
	"testing"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
)

// §5.3.2：兩個正交的版本軸都必須在鍵裡。
//
// 少了實體軸（issue_attempt_seq），作廢重開時內容軸可能一個字都沒變 ⇒ 算出同一個
// AMEGO OrderId ⇒ 回 3040171 而我方以為「已成功」⇒ 原票已作廢、新票沒開出來。
func TestProviderRequestNoForOrder_BothVersionAxesAreInTheKey(t *testing.T) {
	base := ProviderRequestNoForOrder("O000123456", 1, 1)
	if base != "O000123456-1-1" {
		t.Fatalf("= %q", base)
	}
	if same := ProviderRequestNoForOrder("O000123456", 1, 2); same == base {
		t.Error("issue_attempt_seq 沒有進鍵：作廢重開會撞號")
	}
	if same := ProviderRequestNoForOrder("O000123456", 2, 1); same == base {
		t.Error("snapshot_version 沒有進鍵")
	}
	if n := len(ProviderRequestNoForOrder("O000123456", 999, 999)); n > 40 {
		t.Errorf("長度 %d > 40，會撞 chk_invdoc_request_no_len", n)
	}
}

// 六種 operation_type 各有一條推導規則，一條都不能少：少一條的症狀不是編譯錯，
// 是那一類 operation 沒有 operation_key 可寫（NOT NULL）⇒ 第一次建立就 500。
func TestOperationKeyFor_EveryOperationTypeHasARule(t *testing.T) {
	seen := map[string]string{}
	for _, meta := range codemeta.AllInvoiceOperationTypes {
		key, ok := OperationKeyFor(meta.Value, 111, 222, 3, 4)
		if !ok {
			t.Errorf("operation_type %q 沒有推導規則", meta.Value)
			continue
		}
		if prev, dup := seen[key]; dup {
			t.Errorf("%q 與 %q 產生同一把鍵 %q —— 三者可並存，前綴不得相同", meta.Value, prev, key)
		}
		seen[key] = meta.Value
	}
	if _, ok := OperationKeyFor("NOT_A_TYPE", 1, 2, 3, 4); ok {
		t.Error("未知 operation_type 必須回 false，不得 fallback 成任一前綴")
	}
}

func TestOperationKeyFor_Shapes(t *testing.T) {
	cases := map[string]string{
		codemeta.InvoiceOperationTypeIssue.Value:         "issue:111:3",
		codemeta.InvoiceOperationTypeReissue.Value:       "reissue:111:3",
		codemeta.InvoiceOperationTypeQuery.Value:         "query:111:3:4",
		codemeta.InvoiceOperationTypeVoid.Value:          "void:222",
		codemeta.InvoiceOperationTypeAllowance.Value:     "allowance:222",
		codemeta.InvoiceOperationTypeVoidAllowance.Value: "void_allowance:222",
	}
	for opType, want := range cases {
		got, _ := OperationKeyFor(opType, 111, 222, 3, 4)
		if got != want {
			t.Errorf("%s → %q, want %q", opType, got, want)
		}
	}
}

func TestRequestSnapshotHash_ChangesWithEveryFieldThatReachesAmego(t *testing.T) {
	snap := OrderSnapshot{
		TotalOriginalAmount: 100, TotalActualAmount: 100,
		DiscountFunding: DiscountFundingSeller,
		Items:           []OrderItem{{OrderItemID: 1, SkuID: 1, ProductName: "菜", Quantity: 1, UnitPrice: 100}},
	}
	base, err := NewIssueDraft(snap, SellerDefaultTaxResolver{Default: TaxCategoryTaxable}, B2CBuyerIdentifier)
	if err != nil {
		t.Fatalf("NewIssueDraft: %v", err)
	}
	h0 := RequestSnapshotHash(base)
	if h0 == "" || h0 == RequestSnapshotHash(nil) {
		t.Fatal("hash 不得為空，也不得與 nil 草稿相同")
	}
	if RequestSnapshotHash(base) != h0 {
		t.Fatal("同一份草稿必須算出同一個 hash（決定性）")
	}

	changed := *base
	changed.CarrierValue = "/ABC+123"
	if RequestSnapshotHash(&changed) == h0 {
		t.Error("載具值變了 hash 沒變 —— 那是「有人改了快照」偵測不到的形狀")
	}

	changed2 := *base
	changed2.Header.TotalAmount = 101
	if RequestSnapshotHash(&changed2) == h0 {
		t.Error("表頭金額變了 hash 沒變")
	}
}

func TestIsBusinessTaxID(t *testing.T) {
	cases := map[string]bool{
		"36333333":   true,
		"12345678":   true,
		"":           false,
		"0000000000": false, // B2C 佔位值不是統編
		"3633333":    false,
		"363333330":  false,
		"3633333a":   false,
		"3633 3333":  false, // 含空白（IsBusinessTaxID 只收 8 位純數字）
	}
	for in, want := range cases {
		if got := IsBusinessTaxID(in); got != want {
			t.Errorf("IsBusinessTaxID(%q) = %v, want %v", in, got, want)
		}
	}
}
