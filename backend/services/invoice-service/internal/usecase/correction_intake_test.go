package usecase

import "testing"

// ---------------------------------------------------------------------------
// carrierShapeValid：TAX_ID ＋ buyerTaxId 是同一個來源（2026-09-04，#3085 收尾）
//
// 這一組在修復前**一支都不存在**，而缺口是 dev 端到端才發現的：
// 打統編的更正 200 但立刻 REJECTED / CARRIER_INVALID，而只送 buyerTaxId 回 9305。
// 兩種寫法都不通 ⇒ 那條路整條走不到，且沒有任何單元測試會紅。
// ---------------------------------------------------------------------------

func TestCarrierShapeValid(t *testing.T) {
	cases := []struct {
		name string
		spec CheckoutInvoiceSpec
		want bool
	}{
		// 🔴 本組的存在理由：結帳側逐字要求「統編必須明說 carrierType=TAX_ID」，
		// 所以這是**唯一**合法的打統編形狀。修復前它被判成兩個來源。
		{"打統編（TAX_ID ＋ 統編 ＋ 抬頭）", CheckoutInvoiceSpec{
			CarrierType: "TAX_ID", BuyerTaxID: "22223223", BuyerName: "測試股份有限公司"}, true},
		{"載具（手機條碼）", CheckoutInvoiceSpec{CarrierType: "MOBILE_BARCODE", Value: "/ABC1234"}, true},
		{"捐贈", CheckoutInvoiceSpec{CarrierType: "DONATION", Value: "123"}, true},
		{"常用發票資料", CheckoutInvoiceSpec{ProfileID: 42}, true},
		{"什麼都不帶", CheckoutInvoiceSpec{}, true},

		// ── 真正要擋的：兩個**不同**來源同時出現 ──
		{"常用資料 ＋ 統編", CheckoutInvoiceSpec{ProfileID: 42, BuyerTaxID: "22223223"}, false},
		{"載具 ＋ 統編", CheckoutInvoiceSpec{
			CarrierType: "MOBILE_BARCODE", Value: "/ABC1234", BuyerTaxID: "22223223"}, false},
		{"捐贈 ＋ 統編", CheckoutInvoiceSpec{
			CarrierType: "DONATION", Value: "123", BuyerTaxID: "22223223"}, false},
		// 豁免只對 TAX_ID 成立，而且不得把 profileId 一起放行：
		// 「指定既有常用資料」與「這次要打統編」仍然是兩個來源。
		{"TAX_ID ＋ 統編 ＋ 常用資料", CheckoutInvoiceSpec{
			CarrierType: "TAX_ID", BuyerTaxID: "22223223", ProfileID: 42}, false},
	}
	for _, c := range cases {
		if got := carrierShapeValid(c.spec); got != c.want {
			t.Errorf("%s: carrierShapeValid = %v, want %v", c.name, got, c.want)
		}
	}
}
