package rfqhash

import (
	"testing"
	"time"
)

func responseBase() SellerResponseInput {
	return SellerResponseInput{
		SchemaVersion: SchemaV2, ResponseType: "MANUAL_QUOTE", SpecNote: "規格如圖", UnitBasisQuantity: "1.000", UnitBasisUnit: "KG",
		OfferedQuantity: "3.000", OfferedQuantityUnit: "KG", ConversionBasis: "一箱三公斤", Currency: "TWD", GoodsAmount: 900, ShippingAmount: 100, TaxAmount: 50,
		TaxInclusionBasis: "TAX_EXCLUDED", TotalAmount: 1050, LeadTimeDays: ptr(int32(3)), PromisedDelivery: ptr(time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)),
		ValidUntil: time.Date(2026, 12, 5, 8, 0, 0, 0, time.UTC), TermsNote: "月結", DeviationDisclosure: "無",
	}
}

func TestSellerResponseIsDeterministicAndPinned(t *testing.T) {
	a, err := SellerResponse(responseBase())
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := SellerResponse(responseBase()); a != b || len(a) != 64 {
		t.Fatalf("同輸入要同 hash：%q %q", a, b)
	}
	const golden = "8fd699858a71ddb27c97622e13f6a57d13af7a8fa0403ac98197f5633eb6b2c9"
	if a != golden {
		t.Errorf("golden 變了：%s", a)
	}
}

func TestSellerResponseRejectsUnknownSchemaVersions(t *testing.T) {
	for _, v := range []string{"", "v0", "v1", "v3", "V2"} {
		in := responseBase()
		in.SchemaVersion = v
		if got, err := SellerResponse(in); err == nil {
			t.Errorf("%q 要回錯誤，得到 %s", v, got)
		}
	}
}

func TestEverySellerResponseFieldChangesTheHash(t *testing.T) {
	want, _ := SellerResponse(responseBase())
	for name, f := range map[string]func(*SellerResponseInput){
		"response type":       func(i *SellerResponseInput) { i.ResponseType = "ACCEPT_NON_PRICE_TERMS_WITH_QUOTE" },
		"spec note":           func(i *SellerResponseInput) { i.SpecNote = "規格不同" },
		"spec note empty":     func(i *SellerResponseInput) { i.SpecNote = "" },
		"unit basis quantity": func(i *SellerResponseInput) { i.UnitBasisQuantity = "2.000" },
		"unit basis unit":     func(i *SellerResponseInput) { i.UnitBasisUnit = "G" },
		"offered quantity":    func(i *SellerResponseInput) { i.OfferedQuantity = "4.000" },
		"offered unit":        func(i *SellerResponseInput) { i.OfferedQuantityUnit = "G" },
		"conversion basis":    func(i *SellerResponseInput) { i.ConversionBasis = "x" },
		"currency":            func(i *SellerResponseInput) { i.Currency = "USD" },
		"goods":               func(i *SellerResponseInput) { i.GoodsAmount++ },
		"shipping":            func(i *SellerResponseInput) { i.ShippingAmount++ },
		"tax":                 func(i *SellerResponseInput) { i.TaxAmount++ },
		"tax basis":           func(i *SellerResponseInput) { i.TaxInclusionBasis = "TAX_INCLUDED" },
		"total":               func(i *SellerResponseInput) { i.TotalAmount++ },
		"lead time":           func(i *SellerResponseInput) { i.LeadTimeDays = ptr(int32(4)) },
		"lead time none":      func(i *SellerResponseInput) { i.LeadTimeDays = nil },
		"promised delivery":   func(i *SellerResponseInput) { i.PromisedDelivery = ptr(time.Date(2026, 12, 2, 0, 0, 0, 0, time.UTC)) },
		"promised none":       func(i *SellerResponseInput) { i.PromisedDelivery = nil },
		"valid until":         func(i *SellerResponseInput) { i.ValidUntil = i.ValidUntil.Add(time.Hour) },
		"terms note":          func(i *SellerResponseInput) { i.TermsNote = "現金" },
		"deviation":           func(i *SellerResponseInput) { i.DeviationDisclosure = "有" },
	} {
		in := responseBase()
		f(&in)
		if got, _ := SellerResponse(in); got == want {
			t.Errorf("改 %s，hash 沒變", name)
		}
	}
}

func TestSellerResponseHashIgnoresTheTimeZoneOfTheInstant(t *testing.T) {
	want, _ := SellerResponse(responseBase())
	in := responseBase()
	in.ValidUntil = in.ValidUntil.In(time.FixedZone("TPE", 8*3600))
	if got, _ := SellerResponse(in); got != want {
		t.Error("同一個時間點換時區，hash 不該變")
	}
}
