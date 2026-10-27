package domain

import (
	"testing"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
)

func TestCarrierTypeToAmego(t *testing.T) {
	cases := []struct {
		in   codemeta.InvoiceCarrierType
		want string
	}{
		{codemeta.InvoiceCarrierTypeMobileBarcode, "3J0002"},
		{codemeta.InvoiceCarrierTypeCitizenCert, "CQ0001"},
		{codemeta.InvoiceCarrierTypeTaxId, ""},
		{codemeta.InvoiceCarrierTypeDonation, ""},
	}
	for _, c := range cases {
		got, ok := CarrierTypeToAmego(c.in)
		if !ok || got != c.want {
			t.Fatalf("CarrierTypeToAmego(%s) = (%q, %v), want %q", c.in.Code, got, ok, c.want)
		}
	}
	if _, ok := CarrierTypeToAmego(codemeta.InvoiceCarrierType{Code: "NOT_A_CARRIER"}); ok {
		t.Fatal("unknown carrier type must return ok=false")
	}
}

// invoice_documents.carrier_type 存的是 AMEGO 原生詞彙，不是 enum 的 Value ——
// 兩者對手機條碼是 "3J0002" vs "MOBILE_BARCODE"，混用不會編譯錯。
func TestCarrierAmegoVocabularyIsNotEnumValue(t *testing.T) {
	got, _ := CarrierTypeToAmego(codemeta.InvoiceCarrierTypeMobileBarcode)
	if got == codemeta.InvoiceCarrierTypeMobileBarcode.Value {
		t.Fatal("AMEGO 原生詞彙不應等於 enum Value")
	}
	if len(got) > 6 {
		t.Fatalf("carrier_type varchar(6) 放不下 %q", got)
	}
}

// 反向不是函式：TAX_ID 與 DONATION 的 AMEGO 詞彙都是空字串。
func TestCarrierTypeFromAmego(t *testing.T) {
	if got, ok := CarrierTypeFromAmego("3J0002"); !ok || got.Code != codemeta.InvoiceCarrierTypeMobileBarcode.Code {
		t.Fatalf("= (%+v, %v)", got, ok)
	}
	if got, ok := CarrierTypeFromAmego("CQ0001"); !ok || got.Code != codemeta.InvoiceCarrierTypeCitizenCert.Code {
		t.Fatalf("= (%+v, %v)", got, ok)
	}
	if _, ok := CarrierTypeFromAmego(""); ok {
		t.Fatal("空字串必須回 false —— 回任一個猜測值就是在憑證上寫錯買受人身分")
	}
	if _, ok := CarrierTypeFromAmego("XX0000"); ok {
		t.Fatal("unknown vocabulary must return ok=false")
	}
}

func TestCarrierUsesAmegoCarrierField(t *testing.T) {
	for _, c := range []struct {
		in   codemeta.InvoiceCarrierType
		want bool
	}{
		{codemeta.InvoiceCarrierTypeMobileBarcode, true},
		{codemeta.InvoiceCarrierTypeCitizenCert, true},
		{codemeta.InvoiceCarrierTypeTaxId, false},
		{codemeta.InvoiceCarrierTypeDonation, false},
	} {
		if got := CarrierUsesAmegoCarrierField(c.in); got != c.want {
			t.Errorf("CarrierUsesAmegoCarrierField(%s) = %v, want %v", c.in.Code, got, c.want)
		}
	}
}
