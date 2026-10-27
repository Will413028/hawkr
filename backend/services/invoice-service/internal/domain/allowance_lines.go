package domain

import (
	"fmt"
	"sort"
)

// AllowanceLine 是一列折讓明細。
//
// AMEGO 折讓的 UnitPrice / Amount 固定是未稅，與開立端 DetailVat=1 的含稅語意相反
// ⇒ 兩套 DTO 不可共用金額欄位。GrossAmount 是含稅折讓額，只在本層算式內流動、不送出。
type AllowanceLine struct {
	SourceLineSeq         int32 // 對應 invoice_document_lines.line_seq
	OriginalDescription   string
	OriginalInvoiceNumber string
	OriginalInvoiceDate   string // Ymd，台灣曆日
	TaxCategory           TaxCategory

	Quantity  int32 // 恆 1
	UnitPrice int64 // ＝ Amount
	Amount    int64 // 未稅
	Tax       int64

	GrossAmount int64 // 含稅折讓額 a_i（內部）
}

// AllowanceContext 是折讓需要的原發票資訊。
type AllowanceContext struct {
	DocumentTotalAmount    int64 // invoice_documents.total_amount
	DocumentTaxAmount      int64 // invoice_documents.tax_amount（B2C 為 0）
	ExistingAllowanceTotal int64 // Σ 已生效調整金額
	ProviderInvoiceNo      string
	InvoiceDateTaipeiYmd   string

	// B2CSplitTax 對應環境變數 INVOICE_ALLOWANCE_B2C_SPLIT_TAX（預設 true）。
	// 未決 U5：B2C 原發票 TaxAmount=0 時折讓單是否仍逐行拆稅，官方範例與 f0401 的規則互相矛盾。
	// 預設走 A 版（拆稅，依官方範例），B 版留成開關而不是寫死——這個答案只會是二選一。
	B2CSplitTax bool
}

// AllowanceDraft 是「即將送給 AMEGO 折讓」的完整草稿。
type AllowanceDraft struct {
	Context      AllowanceContext
	RefundAmount int64
	Lines        []AllowanceLine

	TaxAmount   int64 // Σ Tax_i
	TotalAmount int64 // Σ Amount_i（未稅合計）

	IsFullAllowance bool
}

// AllowanceBaseOf 回傳逐行的剩餘可折讓基數（只取 PRODUCT / SHIPPING 行）。
//
// base 必須是剩餘額而不是原始額——這是多次部分退款會踩的坑：第二次仍用原始 allowanceable_base
// 當權重時，一個已經折完的行會被再折一次，而 AMEGO 端的累計檢查 4040173 要到很後面才會發現。
func AllowanceBaseOf(lines []Line) (idx []int, base []int64) {
	for i, l := range lines {
		if l.LineType == lineTypeDiscount {
			continue
		}
		idx = append(idx, i)
		base = append(base, l.AllowanceableBase-l.AllowancedAmount)
	}
	return idx, base
}

// BuildAllowanceDraft 依 §5A.9 從原發票明細與退款金額組出折讓草稿。
//
// 逐行 allocation 是我方為了填 AMEGO 必填欄位而做的分攤，不是「買家退了哪幾件」的事實——
// 上游 aftersales 沒有部分品項退貨（建單時把訂單所有 items 原樣複製），折讓是金額折讓。
func BuildAllowanceDraft(docLines []Line, refundAmount int64, ctx AllowanceContext) (*AllowanceDraft, error) {
	if refundAmount <= 0 {
		return nil, fmt.Errorf("invoice: allowance refund amount must be positive, got %d", refundAmount)
	}
	idx, base := AllowanceBaseOf(docLines)

	var totalBase int64
	for _, b := range base {
		totalBase += b
	}
	// 守衛 I3：超額輸入在上游是合法且會發生的（aftersales 每開一張新售後單都拿
	// orders.total_actual_amount 當可退上限、不扣既退金額）。
	if refundAmount > totalBase {
		return nil, invariantErr(CodeAllowanceExceedsInvoice,
			"refund=%d exceeds remaining base=%d", refundAmount, totalBase)
	}

	alloc, err := AllocateLargestRemainder(base, refundAmount)
	if err != nil {
		return nil, fmt.Errorf("invoice: allowance allocation: %w", err)
	}

	d := &AllowanceDraft{
		Context:         ctx,
		RefundAmount:    refundAmount,
		IsFullAllowance: ctx.ExistingAllowanceTotal+refundAmount == ctx.DocumentTotalAmount,
	}
	splitTax := ctx.DocumentTaxAmount > 0 || ctx.B2CSplitTax
	for k, i := range idx {
		a := alloc[k]
		if a == 0 {
			continue // a_i == 0 的行不輸出
		}
		l := docLines[i]
		al := AllowanceLine{
			SourceLineSeq:         l.LineSeq,
			OriginalDescription:   l.Description, // 必須逐字照抄原發票品名
			OriginalInvoiceNumber: ctx.ProviderInvoiceNo,
			OriginalInvoiceDate:   ctx.InvoiceDateTaipeiYmd,
			TaxCategory:           l.TaxCategory,
			// Quantity 一律 1、UnitPrice = Amount：本平台的退款是金額退款，逐行折讓額與原品項
			// 數量沒有比例關係。硬填原數量會讓 UnitPrice 變小數，而折讓端沒有
			// DetailAmountRound 可以鎖整數。
			Quantity:    1,
			GrossAmount: a,
		}
		if l.TaxCategory == TaxCategoryTaxable && splitTax {
			al.Amount = Untax(a)
			al.Tax = a - al.Amount
		} else {
			al.Amount = a
			al.Tax = 0
		}
		al.UnitPrice = al.Amount
		d.Lines = append(d.Lines, al)
	}

	if err := d.absorbFullAllowanceRounding(); err != nil {
		return nil, err
	}
	d.recomputeHeader()
	return d, nil
}

// absorbFullAllowanceRounding 執行 §5A.9.4 的全額折讓尾差吸收。
//
// 只在「全額折讓」且「原發票 TaxAmount > 0」時執行：B2C 原發票 tax_amount = 0、沒有基準可對。
// 全額折讓時逐品項 Tax 四捨五入後加總會 != 原發票 TaxAmount，AMEGO 後台會跳
// 「折讓單的營業稅額已大於原發票的營業稅額」。
func (d *AllowanceDraft) absorbFullAllowanceRounding() error {
	if !d.IsFullAllowance || d.Context.DocumentTaxAmount <= 0 {
		return nil
	}
	var sumTax int64
	for _, l := range d.Lines {
		sumTax += l.Tax
	}
	delta := d.Context.DocumentTaxAmount - sumTax
	if delta == 0 {
		return nil
	}

	target := -1
	for i, l := range d.Lines {
		if l.TaxCategory != TaxCategoryTaxable {
			continue
		}
		if target < 0 {
			target = i
			continue
		}
		// 應稅行中 a_i 最大者；a_i 相同時取 line_seq 最大者。
		t := d.Lines[target]
		if l.GrossAmount > t.GrossAmount || (l.GrossAmount == t.GrossAmount && l.SourceLineSeq > t.SourceLineSeq) {
			target = i
		}
	}
	if target < 0 {
		return invariantErr(CodeAllowanceTaxMismatch,
			"tax delta=%d but no taxable allowance line to absorb it", delta)
	}

	// 護欄：吸收後不得讓任一欄位變負（Amount 為負會撞 3040152／4040141；Tax 為負無合法語意）。
	// |delta| <= 應稅行數 而 target 是 a_i 最大者 ⇒ 正常情況恆安全，但那是推論不是保證——
	// 它依賴「行數遠小於單行金額」這個沒有任何約束在守的前提。護欄成本是兩個比較。
	if d.Lines[target].Amount-delta < 0 || d.Lines[target].Tax+delta < 0 {
		return invariantErr(CodeAllowanceTaxMismatch,
			"absorbing delta=%d on line_seq=%d would make amount=%d tax=%d negative",
			delta, d.Lines[target].SourceLineSeq, d.Lines[target].Amount-delta, d.Lines[target].Tax+delta)
	}
	d.Lines[target].Tax += delta
	d.Lines[target].Amount -= delta // 維持 Amount + Tax == a_i 不變
	d.Lines[target].UnitPrice = d.Lines[target].Amount
	return nil
}

func (d *AllowanceDraft) recomputeHeader() {
	d.TaxAmount, d.TotalAmount = 0, 0
	for _, l := range d.Lines {
		d.TaxAmount += l.Tax
		d.TotalAmount += l.Amount
	}
}

// Validate 執行折讓側的 A16–A19，fail-closed。
func (d *AllowanceDraft) Validate() error {
	// A16：TaxAmount + TotalAmount == refund_amount
	if got := d.TaxAmount + d.TotalAmount; got != d.RefundAmount {
		return invariantErr(CodeAllowanceSplitMismatch,
			"tax=%d total=%d sum=%d refund=%d", d.TaxAmount, d.TotalAmount, got, d.RefundAmount)
	}

	// A17：Σ 已生效調整 + refund_amount <= invoice_documents.total_amount
	if cum := d.Context.ExistingAllowanceTotal + d.RefundAmount; cum > d.Context.DocumentTotalAmount {
		return invariantErr(CodeAllowanceExceedsInvoice,
			"cumulative=%d document_total=%d", cum, d.Context.DocumentTotalAmount)
	}

	// A18：全額折讓且原發票 tax_amount > 0 ⇒ TaxAmount == 原發票 tax_amount；
	//      且尾差吸收後每行 Amount >= 0 && Tax >= 0
	for _, l := range d.Lines {
		if l.Amount < 0 || l.Tax < 0 {
			return invariantErr(CodeAllowanceTaxMismatch,
				"line_seq=%d amount=%d tax=%d", l.SourceLineSeq, l.Amount, l.Tax)
		}
	}
	if d.IsFullAllowance && d.Context.DocumentTaxAmount > 0 && d.TaxAmount != d.Context.DocumentTaxAmount {
		return invariantErr(CodeAllowanceTaxMismatch,
			"full allowance tax=%d document tax=%d", d.TaxAmount, d.Context.DocumentTaxAmount)
	}

	// A19：一張折讓單只綁一張原發票（AMEGO 硬約束 4040132）
	seen := map[string]struct{}{}
	for _, l := range d.Lines {
		if l.OriginalInvoiceNumber == "" {
			return invariantErr(CodeAllowanceMultiInvoice, "line_seq=%d has empty original invoice number", l.SourceLineSeq)
		}
		seen[l.OriginalInvoiceNumber] = struct{}{}
	}
	if len(seen) != 1 {
		nos := make([]string, 0, len(seen))
		for n := range seen {
			nos = append(nos, n)
		}
		sort.Strings(nos)
		return invariantErr(CodeAllowanceMultiInvoice, "allowance spans %d invoices: %v", len(seen), nos)
	}
	return nil
}
