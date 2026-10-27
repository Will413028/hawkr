package usecase

import (
	"encoding/json"
	"testing"
)

// #3031：A06 第 3 層的金額拆解。
//
// 🔴 來源必須是 order_snapshot_json 的**結帳分解**（商品／運費／折扣），
// 不是 invoice_documents 的 **AMEGO 稅別分解**（sales／freeTax／zeroTax／tax）。
// 兩者是同一筆錢的兩種切法，拿後者去湊前者會得出看似合理、卻對不上結帳頁的數字。
func TestParseSnapshotAmounts(t *testing.T) {
	raw, err := json.Marshal(sourceSnapshot{
		OrderNo:             "O123",
		TotalOriginalAmount: 1000,
		ShippingFee:         60,
		DiscountAmount:      100,
		TotalActualAmount:   960,
		DefaultTaxCategory:  1,
	})
	if err != nil {
		t.Fatal(err)
	}

	got, ok := ParseSnapshotAmounts(raw)
	if !ok {
		t.Fatal("完整快照應該解得出來")
	}
	if got.ProductAmount != 1000 || got.ShippingAmount != 60 ||
		got.DiscountAmount != 100 || got.InvoiceAmount != 960 || got.TaxCategory != 1 {
		t.Fatalf("欄位對應錯了：%#v", got)
	}
	// 恆等式：invoice = product + shipping − discount（proto 逐字）。
	if got.ProductAmount+got.ShippingAmount-got.DiscountAmount != got.InvoiceAmount {
		t.Fatalf("拆解對不上總額：%#v", got)
	}
}

// 🔴 三種「沒有可用拆解」都必須回 ok=false，而不是一組零值。
//
// 回零值的後果是 A06 把它畫成「商品 0 元、運費 0 元」——一個看起來完全正常的錯誤畫面，
// 而管理員正是拿這一層去核對金額爭議。
func TestParseSnapshotAmounts_RefusesToFabricateZeroes(t *testing.T) {
	cases := map[string]json.RawMessage{
		"空快照":       nil,
		"解不開":       json.RawMessage(`{`),
		"早期快照沒有金額欄": json.RawMessage(`{"order_no":"O1","snapshot_version":1}`),
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if got, ok := ParseSnapshotAmounts(raw); ok {
				t.Fatalf("應回 ok=false，實得 %#v", got)
			}
		})
	}
}
