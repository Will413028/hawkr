package domain

import "testing"

// mapResolver 讓測試表達 Phase 2 的逐 SKU 稅別（例 2 需要）。
type mapResolver struct {
	byS map[int64]TaxCategory
	def TaxCategory
}

func (r mapResolver) ResolveProduct(skuID int64) TaxCategory {
	if c, ok := r.byS[skuID]; ok {
		return c
	}
	return r.def
}

type exItem struct {
	name  string
	qty   int32
	price int64
	cat   TaxCategory
}

func buildExample(t *testing.T, items []exItem, shippingFee, discount int64, buyerID string) *IssueDraft {
	t.Helper()
	d := newExampleDraft(t, items, shippingFee, discount, buyerID)
	if err := d.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	return d
}

// newExampleDraft 只組草稿、不驗證，供變異測試注入違反用。
func newExampleDraft(t *testing.T, items []exItem, shippingFee, discount int64, buyerID string) *IssueDraft {
	t.Helper()
	snap := OrderSnapshot{
		ShippingFee:     shippingFee,
		DiscountAmount:  discount,
		DiscountFunding: DiscountFundingSeller,
	}
	byS := map[int64]TaxCategory{}
	for i, it := range items {
		id := int64(i + 1)
		snap.Items = append(snap.Items, OrderItem{
			OrderItemID: id, SkuID: id, ProductName: it.name, Quantity: it.qty, UnitPrice: it.price,
		})
		snap.TotalOriginalAmount += int64(it.qty) * it.price
		byS[id] = it.cat
	}
	snap.TotalActualAmount = snap.TotalOriginalAmount + shippingFee - discount
	if snap.TotalActualAmount < 0 {
		snap.TotalActualAmount = 0
	}
	d, err := NewIssueDraft(snap, mapResolver{byS: byS, def: TaxCategoryTaxable}, buyerID)
	if err != nil {
		t.Fatalf("NewIssueDraft: %v", err)
	}
	return d
}

type wantLine struct {
	lineType string
	desc     string
	qty      int32
	unit     int64
	net      int64
	cat      TaxCategory
	alloc    int64
	base     int64
}

func assertLines(t *testing.T, got []Line, want []wantLine) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("line count = %d, want %d", len(got), len(want))
	}
	for i := range want {
		g, w := got[i], want[i]
		if int(g.LineSeq) != i+1 {
			t.Errorf("line[%d].LineSeq = %d, want %d", i, g.LineSeq, i+1)
		}
		if g.LineType != w.lineType || g.Description != w.desc || g.Quantity != w.qty ||
			g.UnitPrice != w.unit || g.NetAmount != w.net || g.TaxCategory != w.cat ||
			g.AllocatedDiscount != w.alloc || g.AllowanceableBase != w.base {
			t.Errorf("line[%d] = %+v\nwant type=%s desc=%q qty=%d unit=%d net=%d cat=%d alloc=%d base=%d",
				i, g, w.lineType, w.desc, w.qty, w.unit, w.net, int16(w.cat), w.alloc, w.base)
		}
	}
}

func assertHeader(t *testing.T, h Header, sales, free, zero, tax, total int64, taxType TaxCategory) {
	t.Helper()
	if h.SalesAmount != sales || h.FreeTaxSalesAmount != free || h.ZeroTaxSalesAmount != zero ||
		h.TaxAmount != tax || h.TotalAmount != total || h.TaxType != taxType {
		t.Errorf("header = %+v\nwant sales=%d free=%d zero=%d tax=%d total=%d taxType=%d",
			h, sales, free, zero, tax, total, int16(taxType))
	}
	if h.TaxRate != TaxRateLiteral || h.DetailVat != DetailVatInclusive || h.DetailAmountRound != DetailAmountRoundOn {
		t.Errorf("header fixed fields = %+v", h)
	}
}

// 例 1：尾差（同一稅別、三行、B2C）。§5A.7。
func TestExample1_RoundingRemainder(t *testing.T) {
	d := buildExample(t, []exItem{
		{"A", 1, 333, TaxCategoryTaxable},
		{"B", 1, 333, TaxCategoryTaxable},
		{"C", 1, 334, TaxCategoryTaxable},
	}, 0, 100, B2CBuyerIdentifier)

	assertLines(t, d.Lines, []wantLine{
		{lineTypeProduct, "A", 1, 333, 333, TaxCategoryTaxable, 33, 300},
		{lineTypeProduct, "B", 1, 333, 333, TaxCategoryTaxable, 33, 300},
		{lineTypeProduct, "C", 1, 334, 334, TaxCategoryTaxable, 34, 300},
		{lineTypeDiscount, DiscountDescription, 1, -100, -100, TaxCategoryTaxable, 0, 0},
	})
	assertHeader(t, d.Header, 900, 0, 0, 0, 900, TaxCategoryTaxable)
}

// 例 2：混合稅 ＋ 桶層平手。排序鍵第二段（weights DESC）非要不可的證據。§5A.7。
func TestExample2_BucketTieBrokenByWeight(t *testing.T) {
	d := buildExample(t, []exItem{
		{"免稅A", 3, 250, TaxCategoryExempt},
		{"加工B", 2, 180, TaxCategoryTaxable},
		{"禮盒C", 1, 890, TaxCategoryTaxable},
	}, 120, 300, B2CBuyerIdentifier)

	assertLines(t, d.Lines, []wantLine{
		{lineTypeProduct, "免稅A", 3, 250, 750, TaxCategoryExempt, 112, 638},
		{lineTypeProduct, "加工B", 2, 180, 360, TaxCategoryTaxable, 54, 306},
		{lineTypeProduct, "禮盒C", 1, 890, 890, TaxCategoryTaxable, 134, 756},
		{lineTypeShipping, ShippingDescription, 1, 120, 120, TaxCategoryTaxable, 0, 120},
		{lineTypeDiscount, DiscountDescription, 1, -188, -188, TaxCategoryTaxable, 0, 0},
		{lineTypeDiscount, DiscountDescription, 1, -112, -112, TaxCategoryExempt, 0, 0},
	})
	assertHeader(t, d.Header, 1182, 638, 0, 0, 1820, TaxCategoryMixed)
}

// 例 3：折扣溢出到運費（全應稅）。A23 守的那一格。§5A.7。
func TestExample3_DiscountOverflowsIntoShipping(t *testing.T) {
	d := buildExample(t, []exItem{
		{"A", 1, 200, TaxCategoryTaxable},
		{"B", 1, 50, TaxCategoryTaxable},
	}, 150, 300, B2CBuyerIdentifier)

	assertLines(t, d.Lines, []wantLine{
		{lineTypeProduct, "A", 1, 200, 200, TaxCategoryTaxable, 200, 0},
		{lineTypeProduct, "B", 1, 50, 50, TaxCategoryTaxable, 50, 0},
		{lineTypeShipping, ShippingDescription, 1, 150, 150, TaxCategoryTaxable, 50, 100},
		{lineTypeDiscount, DiscountDescription, 1, -300, -300, TaxCategoryTaxable, 0, 0},
	})
	assertHeader(t, d.Header, 100, 0, 0, 0, 100, TaxCategoryTaxable)
	if got := SumAllowanceableBase(d.Lines); got != 100 {
		t.Fatalf("sum(allowanceable_base) = %d, want 100", got)
	}
}

// 例 3b：同例 3 但商品免稅 ⇒ 兩個 DISCOUNT 行、表頭 9 而 FreeTaxSalesAmount = 0（未決 U4）。
func TestExample3b_TwoDiscountBucketsWithZeroFreeTax(t *testing.T) {
	d := buildExample(t, []exItem{
		{"A", 1, 200, TaxCategoryExempt},
		{"B", 1, 50, TaxCategoryExempt},
	}, 150, 300, B2CBuyerIdentifier)

	assertLines(t, d.Lines, []wantLine{
		{lineTypeProduct, "A", 1, 200, 200, TaxCategoryExempt, 200, 0},
		{lineTypeProduct, "B", 1, 50, 50, TaxCategoryExempt, 50, 0},
		{lineTypeShipping, ShippingDescription, 1, 150, 150, TaxCategoryTaxable, 50, 100},
		{lineTypeDiscount, DiscountDescription, 1, -50, -50, TaxCategoryTaxable, 0, 0},
		{lineTypeDiscount, DiscountDescription, 1, -250, -250, TaxCategoryExempt, 0, 0},
	})
	assertHeader(t, d.Header, 100, 0, 0, 0, 100, TaxCategoryMixed)
}

// 例 4：Phase 1 的典型形態（免稅生鮮 ＋ 應稅運費 ⇒ 混合稅）。§5A.7。
func TestExample4_ExemptGoodsPlusTaxableShipping(t *testing.T) {
	d := buildExample(t, []exItem{
		{"生鮮A", 2, 380, TaxCategoryExempt},
		{"生鮮B", 1, 450, TaxCategoryExempt},
		{"生鮮C", 1, 290, TaxCategoryExempt},
	}, 150, 100, B2CBuyerIdentifier)

	assertLines(t, d.Lines, []wantLine{
		{lineTypeProduct, "生鮮A", 2, 380, 760, TaxCategoryExempt, 51, 709},
		{lineTypeProduct, "生鮮B", 1, 450, 450, TaxCategoryExempt, 30, 420},
		{lineTypeProduct, "生鮮C", 1, 290, 290, TaxCategoryExempt, 19, 271},
		{lineTypeShipping, ShippingDescription, 1, 150, 150, TaxCategoryTaxable, 0, 150},
		{lineTypeDiscount, DiscountDescription, 1, -100, -100, TaxCategoryExempt, 0, 0},
	})
	assertHeader(t, d.Header, 150, 1400, 0, 0, 1550, TaxCategoryMixed)
}

// 例 5：AMEGO 官方 B2B 範例，逐位元對照 [AMEGO]:538-548。§5A.7。
func TestExample5_OfficialB2BSample(t *testing.T) {
	d := buildExample(t, []exItem{
		{"測試商品1", 1, 170, TaxCategoryTaxable},
	}, 0, 2, "45444444")

	assertLines(t, d.Lines, []wantLine{
		{lineTypeProduct, "測試商品1", 1, 170, 170, TaxCategoryTaxable, 2, 168},
		{lineTypeDiscount, DiscountDescription, 1, -2, -2, TaxCategoryTaxable, 0, 0},
	})
	assertHeader(t, d.Header, 160, 0, 0, 8, 168, TaxCategoryTaxable)
}

// 例 1 的行層分攤直接對 AllocateLargestRemainder 再驗一次（設計書列出了逐項 num/q/rem）。
func TestExample1_AllocationTable(t *testing.T) {
	got, err := AllocateLargestRemainder([]int64{333, 333, 334}, 100)
	if err != nil {
		t.Fatalf("allocate: %v", err)
	}
	want := []int64{33, 33, 34}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("alloc = %v, want %v", got, want)
		}
	}
}
