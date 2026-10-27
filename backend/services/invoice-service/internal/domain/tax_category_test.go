package domain

import (
	"testing"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
)

// InvoiceTaxCategory 是全書唯一 Code != Value 的 enum：轉換走 Value，拿 Code 去 FromValue 必然失敗。
func TestTaxCategoryToDBAndBack(t *testing.T) {
	cases := []struct {
		meta codemeta.InvoiceTaxCategory
		db   int16
	}{
		{codemeta.InvoiceTaxCategoryTaxable, 1},
		{codemeta.InvoiceTaxCategoryZeroRated, 2},
		{codemeta.InvoiceTaxCategoryTaxExempt, 3},
	}
	for _, c := range cases {
		got, err := TaxCategoryToDB(c.meta)
		if err != nil || got != c.db {
			t.Fatalf("TaxCategoryToDB(%s) = (%d, %v), want %d", c.meta.Code, got, err, c.db)
		}
		back, ok := TaxCategoryFromDB(c.db)
		if !ok || back.Code != c.meta.Code {
			t.Fatalf("TaxCategoryFromDB(%d) = (%+v, %v), want %s", c.db, back, ok, c.meta.Code)
		}
	}
}

func TestTaxCategoryFromDB_UnknownIsFailClosed(t *testing.T) {
	// 轉換失敗不得 fallback 成 1（應稅）—— 那會把一張免稅發票靜默開成應稅。
	if meta, ok := TaxCategoryFromDB(9); ok {
		t.Fatalf("TaxCategoryFromDB(9) = %+v, want ok=false（9 只能出現在表頭）", meta)
	}
	if _, ok := TaxCategoryFromDB(0); ok {
		t.Fatal("TaxCategoryFromDB(0) must be ok=false")
	}
}

// 拿 Code 去 FromValue 必然 ok=false —— 這是 §C.10 明列的第二個容易寫錯的地方。
func TestTaxCategoryFromValue_RejectsCode(t *testing.T) {
	if _, ok := codemeta.InvoiceTaxCategoryFromValue(codemeta.InvoiceTaxCategoryTaxable.Code); ok {
		t.Fatal("InvoiceTaxCategoryFromValue(\"TAXABLE\") should be false")
	}
}

func TestTaxCategoryToAmego(t *testing.T) {
	for _, c := range []struct {
		in   TaxCategory
		want string
	}{{TaxCategoryTaxable, "1"}, {TaxCategoryZeroRate, "2"}, {TaxCategoryExempt, "3"}} {
		got, err := TaxCategoryToAmego(c.in)
		if err != nil || got != c.want {
			t.Fatalf("TaxCategoryToAmego(%d) = (%q, %v), want %q", int16(c.in), got, err, c.want)
		}
	}
	if _, err := TaxCategoryToAmego(TaxCategoryMixed); err == nil {
		t.Fatal("表頭專用的 9 不得被送成 ProductItem.TaxType")
	}
}

func TestTaxCategoryLevels(t *testing.T) {
	for _, c := range []TaxCategory{1, 2, 3} {
		if !c.IsLineLevel() || !c.IsHeaderLevel() {
			t.Fatalf("%d should be both line- and header-level", int16(c))
		}
	}
	if TaxCategoryMixed.IsLineLevel() {
		t.Fatal("9 must not be line-level")
	}
	if !TaxCategoryMixed.IsHeaderLevel() {
		t.Fatal("9 must be header-level")
	}
	if TaxCategory(0).IsHeaderLevel() {
		t.Fatal("0 is not a valid tax category")
	}
}

func TestSellerDefaultTaxResolver(t *testing.T) {
	r := SellerDefaultTaxResolver{Default: TaxCategoryExempt}
	for _, sku := range []int64{0, 1, 999} {
		if got := r.ResolveProduct(sku); got != TaxCategoryExempt {
			t.Fatalf("ResolveProduct(%d) = %d, want 3", sku, int16(got))
		}
	}
	var _ TaxCategoryResolver = r
}
