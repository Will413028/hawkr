package usecase

import "testing"

func TestCarrierShapeValid(t *testing.T) {
	cases := []struct {
		name string
		spec CheckoutInvoiceSpec
		want bool
	}{
		{"打統編（TAX_ID ＋ 統編 ＋ 抬頭）", CheckoutInvoiceSpec{
			CarrierType: "TAX_ID", BuyerTaxID: "22223223", BuyerName: "測試股份有限公司"}, true},
		{"載具（手機條碼）", CheckoutInvoiceSpec{CarrierType: "MOBILE_BARCODE", Value: "/ABC1234"}, true},
		{"捐贈", CheckoutInvoiceSpec{CarrierType: "DONATION", Value: "123"}, true},
		{"常用發票資料", CheckoutInvoiceSpec{ProfileID: 42}, true},
		{"什麼都不帶", CheckoutInvoiceSpec{}, true},

		{"常用資料 ＋ 統編", CheckoutInvoiceSpec{ProfileID: 42, BuyerTaxID: "22223223"}, false},
		{"載具 ＋ 統編", CheckoutInvoiceSpec{
			CarrierType: "MOBILE_BARCODE", Value: "/ABC1234", BuyerTaxID: "22223223"}, false},
		{"捐贈 ＋ 統編", CheckoutInvoiceSpec{
			CarrierType: "DONATION", Value: "123", BuyerTaxID: "22223223"}, false},
		{"TAX_ID ＋ 統編 ＋ 常用資料", CheckoutInvoiceSpec{
			CarrierType: "TAX_ID", BuyerTaxID: "22223223", ProfileID: 42}, false},
	}
	for _, c := range cases {
		if got := carrierShapeValid(c.spec); got != c.want {
			t.Errorf("%s: carrierShapeValid = %v, want %v", c.name, got, c.want)
		}
	}
}
