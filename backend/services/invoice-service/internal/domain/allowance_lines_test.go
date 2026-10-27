package domain

import (
	"errors"
	"testing"
)

// b2bThreeLines 造出 §5A.9.4 的實例：三行各 300 元、全應稅、B2B。
func b2bThreeLines(t *testing.T) (*IssueDraft, AllowanceContext) {
	t.Helper()
	d := newExampleDraft(t, []exItem{
		{"甲", 1, 300, TaxCategoryTaxable},
		{"乙", 1, 300, TaxCategoryTaxable},
		{"丙", 1, 300, TaxCategoryTaxable},
	}, 0, 0, "45444444")
	if err := d.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if d.Header.TaxAmount != 43 || d.Header.SalesAmount != 857 {
		t.Fatalf("header = %+v, want tax=43 sales=857", d.Header)
	}
	return d, AllowanceContext{
		DocumentTotalAmount:  d.Header.TotalAmount,
		DocumentTaxAmount:    d.Header.TaxAmount,
		ProviderInvoiceNo:    "AB12345678",
		InvoiceDateTaipeiYmd: "20260829",
		B2CSplitTax:          true,
	}
}

// §5A.9.4 的實例逐位元對照：delta = +1 吸收到 line_seq 最大的那一行。
func TestAllowance_FullAllowanceTailAbsorption(t *testing.T) {
	d, ctx := b2bThreeLines(t)
	ad, err := BuildAllowanceDraft(d.Lines, 900, ctx)
	if err != nil {
		t.Fatalf("BuildAllowanceDraft: %v", err)
	}
	if err := ad.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if !ad.IsFullAllowance {
		t.Fatal("want IsFullAllowance")
	}
	want := []struct{ seq, amount, tax int64 }{{1, 286, 14}, {2, 286, 14}, {3, 285, 15}}
	if len(ad.Lines) != 3 {
		t.Fatalf("len(lines) = %d, want 3", len(ad.Lines))
	}
	for i, w := range want {
		l := ad.Lines[i]
		if int64(l.SourceLineSeq) != w.seq || l.Amount != w.amount || l.Tax != w.tax || l.UnitPrice != w.amount || l.Quantity != 1 {
			t.Errorf("line[%d] = %+v, want seq=%d amount=%d tax=%d qty=1", i, l, w.seq, w.amount, w.tax)
		}
		if l.OriginalDescription == "" || l.OriginalInvoiceNumber != "AB12345678" || l.OriginalInvoiceDate != "20260829" {
			t.Errorf("line[%d] original fields = %+v", i, l)
		}
	}
	if ad.TaxAmount != 43 || ad.TotalAmount != 857 {
		t.Fatalf("header tax=%d total=%d, want 43 / 857", ad.TaxAmount, ad.TotalAmount)
	}
	if ad.TaxAmount+ad.TotalAmount != 900 {
		t.Fatalf("tax+total = %d, want 900", ad.TaxAmount+ad.TotalAmount)
	}
}

// base 必須是剩餘額而不是原始額：第二次部分退款不得讓已折完的行再被折一次。
func TestAllowance_SecondPartialUsesRemainingBase(t *testing.T) {
	d, ctx := b2bThreeLines(t)
	lines := append([]Line(nil), d.Lines...)

	first, err := BuildAllowanceDraft(lines, 600, ctx)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := first.Validate(); err != nil {
		t.Fatalf("first validate: %v", err)
	}
	// 寫回 allowanced_amount（usecase 在同一交易內做的事）。
	for _, al := range first.Lines {
		lines[al.SourceLineSeq-1].AllowancedAmount += al.GrossAmount
	}

	ctx.ExistingAllowanceTotal = 600
	second, err := BuildAllowanceDraft(lines, 300, ctx)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if err := second.Validate(); err != nil {
		t.Fatalf("second validate: %v", err)
	}
	for i := range lines {
		if lines[i].LineType == lineTypeDiscount {
			continue
		}
		got := lines[i].AllowancedAmount
		for _, al := range second.Lines {
			if al.SourceLineSeq == lines[i].LineSeq {
				got += al.GrossAmount
			}
		}
		if got > lines[i].AllowanceableBase {
			t.Fatalf("line_seq=%d allowanced=%d exceeds base=%d", lines[i].LineSeq, got, lines[i].AllowanceableBase)
		}
	}
	if second.TaxAmount+second.TotalAmount != 300 {
		t.Fatalf("second tax+total = %d, want 300", second.TaxAmount+second.TotalAmount)
	}
}

// 上游確實會產生超額輸入（aftersales 每張新單都拿 total_actual_amount 當上限、不扣既退）。
func TestAllowance_ExceedsRemainingBaseIsBlockedAtBuild(t *testing.T) {
	d, ctx := b2bThreeLines(t)
	_, err := BuildAllowanceDraft(d.Lines, 901, ctx)
	var ie *InvariantError
	if !errors.As(err, &ie) || string(ie.Code) != CodeAllowanceExceedsInvoice {
		t.Fatalf("err = %v, want %s", err, CodeAllowanceExceedsInvoice)
	}
}

// 免稅／零稅率行不拆稅（AMEGO 折讓固定未稅，但免稅行的未稅額 ＝ 含稅額）。
func TestAllowance_ExemptLineNotSplit(t *testing.T) {
	d := newExampleDraft(t, []exItem{{"生鮮", 1, 500, TaxCategoryExempt}}, 0, 0, B2CBuyerIdentifier)
	if err := d.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	ad, err := BuildAllowanceDraft(d.Lines, 500, AllowanceContext{
		DocumentTotalAmount: 500, DocumentTaxAmount: 0,
		ProviderInvoiceNo: "AB12345678", InvoiceDateTaipeiYmd: "20260829", B2CSplitTax: true,
	})
	if err != nil {
		t.Fatalf("BuildAllowanceDraft: %v", err)
	}
	if err := ad.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if ad.TaxAmount != 0 || ad.TotalAmount != 500 {
		t.Fatalf("tax=%d total=%d, want 0 / 500", ad.TaxAmount, ad.TotalAmount)
	}
}

// 未決 U5 的 B 版開關：B2C 原發票 tax_amount = 0 時可關掉逐行拆稅。
func TestAllowance_B2CSplitTaxSwitch(t *testing.T) {
	d := newExampleDraft(t, []exItem{{"甲", 1, 900, TaxCategoryTaxable}}, 0, 0, B2CBuyerIdentifier)
	base := AllowanceContext{
		DocumentTotalAmount: 900, DocumentTaxAmount: 0,
		ProviderInvoiceNo: "AB12345678", InvoiceDateTaipeiYmd: "20260829",
	}
	on := base
	on.B2CSplitTax = true
	adOn, err := BuildAllowanceDraft(d.Lines, 900, on)
	if err != nil {
		t.Fatalf("split on: %v", err)
	}
	if adOn.TaxAmount != 43 || adOn.TotalAmount != 857 {
		t.Fatalf("split on: tax=%d total=%d, want 43 / 857", adOn.TaxAmount, adOn.TotalAmount)
	}
	adOff, err := BuildAllowanceDraft(d.Lines, 900, base)
	if err != nil {
		t.Fatalf("split off: %v", err)
	}
	if adOff.TaxAmount != 0 || adOff.TotalAmount != 900 {
		t.Fatalf("split off: tax=%d total=%d, want 0 / 900", adOff.TaxAmount, adOff.TotalAmount)
	}
	for _, ad := range []*AllowanceDraft{adOn, adOff} {
		if err := ad.Validate(); err != nil {
			t.Fatalf("Validate: %v", err)
		}
	}
}

// 逐條把 A16–A19 各弄壞一次。
func TestAllowanceDraftValidate_MutationPerAssertion(t *testing.T) {
	build := func(t *testing.T) *AllowanceDraft {
		t.Helper()
		d, ctx := b2bThreeLines(t)
		ad, err := BuildAllowanceDraft(d.Lines, 900, ctx)
		if err != nil {
			t.Fatalf("BuildAllowanceDraft: %v", err)
		}
		return ad
	}

	cases := []struct {
		name   string
		code   string
		mutate func(*AllowanceDraft)
	}{
		{"A16 未稅合計加稅額不等於退款額", CodeAllowanceSplitMismatch, func(d *AllowanceDraft) {
			d.TaxAmount++
		}},
		{"A17 累計折讓超過發票金額", CodeAllowanceExceedsInvoice, func(d *AllowanceDraft) {
			d.Context.ExistingAllowanceTotal = d.Context.DocumentTotalAmount
		}},
		{"A18 尾差吸收後金額為負", CodeAllowanceTaxMismatch, func(d *AllowanceDraft) {
			d.Lines[0].Tax += d.Lines[0].Amount + 5
			d.Lines[0].Amount = -5 // 維持 A16（Amount + Tax 不變）
		}},
		{"A18 全額折讓稅額不等於原發票", CodeAllowanceTaxMismatch, func(d *AllowanceDraft) {
			d.Context.DocumentTaxAmount = 44
		}},
		{"A19 折讓橫跨兩張原發票", CodeAllowanceMultiInvoice, func(d *AllowanceDraft) {
			d.Lines = append(d.Lines, AllowanceLine{
				SourceLineSeq: 9, OriginalInvoiceNumber: "ZZ99999999", OriginalDescription: "另一張",
			})
		}},
		{"A19 原發票號碼為空", CodeAllowanceMultiInvoice, func(d *AllowanceDraft) {
			d.Lines[0].OriginalInvoiceNumber = ""
		}},
	}

	covered := map[string]struct{}{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ad := build(t)
			tc.mutate(ad)
			err := ad.Validate()
			var ie *InvariantError
			if !errors.As(err, &ie) {
				t.Fatalf("Validate() = %v, want *InvariantError with code %s", err, tc.code)
			}
			if string(ie.Code) != tc.code {
				t.Fatalf("Validate() code = %s, want %s (detail: %s)", ie.Code, tc.code, ie.Detail)
			}
		})
		covered[tc.code] = struct{}{}
	}
	for i := 15; i <= 18; i++ { // A16–A19
		if _, ok := covered[allInvariantCodes[i]]; !ok {
			t.Errorf("A%d (%s) has no mutation case", i+1, allInvariantCodes[i])
		}
	}
}
