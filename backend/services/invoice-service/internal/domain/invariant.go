package domain

import (
	"fmt"
	"unicode/utf8"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
)

// InvariantCode 是斷言失敗的理由碼。值一律引用 CodeMeta 生成的常數，
// 不在本檔硬寫字串字面值（生成檔是 SSOT，手寫副本會漂移）。
//
// 開立側落 invoice_documents.manual_reason_code、調整側落
// invoice_adjustments.blocked_reason_code，兩者共用同一個 InvoiceManualReason 值域。
type InvariantCode string

// InvariantError 讓呼叫端用 errors.As 取出理由碼，決定落哪個 status。
type InvariantError struct {
	Code   InvariantCode // 落進 manual_reason_code / blocked_reason_code
	Detail string        // 給 log 與 ADM 卡片用的人類說明，不落 DB 欄位
}

func (e *InvariantError) Error() string { return string(e.Code) + ": " + e.Detail }

func invariantErr(code string, format string, args ...any) *InvariantError {
	return &InvariantError{Code: InvariantCode(code), Detail: fmt.Sprintf(format, args...)}
}

// A 表 23 條斷言的理由碼。編號一經指派就不再重排；要作廢某條就把該列標 retired 保留編號。
var (
	CodeAmountMismatch          = codemeta.InvoiceManualReasonAmountMismatch.Value          // A1
	CodeTaxSplitMismatch        = codemeta.InvoiceManualReasonTaxSplitMismatch.Value        // A2
	CodeNegativeHeaderAmount    = codemeta.InvoiceManualReasonNegativeHeaderAmount.Value    // A3
	CodeNegativeBucket          = codemeta.InvoiceManualReasonNegativeBucket.Value          // A4
	CodeLineSignInvalid         = codemeta.InvoiceManualReasonLineSignInvalid.Value         // A5
	CodeZeroLineEmitted         = codemeta.InvoiceManualReasonZeroLineEmitted.Value         // A6
	CodeB2CTaxNotZero           = codemeta.InvoiceManualReasonB2cTaxNotZero.Value           // A7
	CodeB2BTaxMiscalc           = codemeta.InvoiceManualReasonB2bTaxMiscalc.Value           // A8
	CodeHeaderTaxTypeMismatch   = codemeta.InvoiceManualReasonHeaderTaxtypeMismatch.Value   // A9
	CodeZeroRateNotSupported    = codemeta.InvoiceManualReasonZeroRateNotSupported.Value    // A10
	CodeB2BMixedTaxUnverified   = codemeta.InvoiceManualReasonB2bMixedTaxUnverified.Value   // A11
	CodeLineCountInvalid        = codemeta.InvoiceManualReasonLineCountInvalid.Value        // A12
	CodeDescriptionInvalid      = codemeta.InvoiceManualReasonDescriptionInvalid.Value      // A13
	CodeDetailVatInvalid        = codemeta.InvoiceManualReasonDetailVatInvalid.Value        // A14
	CodeBuyerInfoConflict       = codemeta.InvoiceManualReasonBuyerInfoConflict.Value       // A15
	CodeAllowanceSplitMismatch  = codemeta.InvoiceManualReasonAllowanceSplitMismatch.Value  // A16
	CodeAllowanceExceedsInvoice = codemeta.InvoiceManualReasonAllowanceExceedsInvoice.Value // A17
	CodeAllowanceTaxMismatch    = codemeta.InvoiceManualReasonAllowanceTaxMismatch.Value    // A18
	CodeAllowanceMultiInvoice   = codemeta.InvoiceManualReasonAllowanceMultiInvoice.Value   // A19
	CodeAmountExceedsLimit      = codemeta.InvoiceManualReasonAmountExceedsLimit.Value      // A20
	CodeSubtotalMismatch        = codemeta.InvoiceManualReasonSubtotalMismatch.Value        // A21
	CodePlatformFundedDiscount  = codemeta.InvoiceManualReasonPlatformFundedDiscount.Value  // A22
	CodeAllowanceBaseMismatch   = codemeta.InvoiceManualReasonAllowanceBaseMismatch.Value   // A23
)

// AmegoLineCountMax 是 AMEGO 對 ProductItem 陣列長度的上限。
const AmegoLineCountMax = 9999

// IssueDraft 是「即將送給 AMEGO 開立」的完整草稿。
//
// Items 是實際會送出的 ProductItem 行（＝ gross != 0 的行），Lines 是全部落庫行（含 0 元行）。
// 兩者刻意分開存而不是每次由 Lines 推導：A6 要斷言的正是「送出去的那份沒有 0 元行」，
// 而由 Lines 推導的話那條斷言恆真、等於沒有斷言。
type IssueDraft struct {
	Order  OrderSnapshot
	Lines  []Line
	Items  []Line
	Header Header

	BuyerIdentifier string // B2C 固定 "0000000000"
	CarrierType     string // AMEGO 原生詞彙："3J0002"／"CQ0001"／""
	CarrierValue    string
	DonationCode    string
}

// NewIssueDraft 從訂單快照組出完整草稿（不含 Validate）。
func NewIssueDraft(snap OrderSnapshot, resolver TaxCategoryResolver, buyerIdentifier string) (*IssueDraft, error) {
	lines, err := BuildLines(snap, resolver)
	if err != nil {
		return nil, err
	}
	items := EmittedLines(lines)
	return &IssueDraft{
		Order:           snap,
		Lines:           lines,
		Items:           items,
		Header:          BuildHeader(lines, buyerIdentifier),
		BuyerIdentifier: buyerIdentifier,
	}, nil
}

// Validate 執行開立側的 A1–A15、A20–A23，fail-closed。
//
// 在 usecase 組完 draft、落庫之前呼叫。provider adapter 不重複驗（debug 模式下可再跑一次同一個
// Validate 做交叉檢查）。回傳 *InvariantError，呼叫端用 errors.As 取出理由碼落
// manual_reason_code ＋ status='AWAITING_MANUAL'。
func (d *IssueDraft) Validate() error {
	h := d.Header

	if sum := SumNetAmount(d.Lines); sum != h.TotalAmount || h.TotalAmount != d.Order.TotalActualAmount {
		return invariantErr(CodeAmountMismatch,
			"sum(lines)=%d header.total=%d order.total_actual=%d", sum, h.TotalAmount, d.Order.TotalActualAmount)
	}

	if got := h.SalesAmount + h.FreeTaxSalesAmount + h.ZeroTaxSalesAmount + h.TaxAmount; got != h.TotalAmount {
		return invariantErr(CodeTaxSplitMismatch,
			"sales=%d free=%d zero=%d tax=%d sum=%d total=%d",
			h.SalesAmount, h.FreeTaxSalesAmount, h.ZeroTaxSalesAmount, h.TaxAmount, got, h.TotalAmount)
	}

	if h.SalesAmount < 0 || h.FreeTaxSalesAmount < 0 || h.ZeroTaxSalesAmount < 0 || h.TotalAmount <= 0 {
		return invariantErr(CodeNegativeHeaderAmount,
			"sales=%d free=%d zero=%d total=%d", h.SalesAmount, h.FreeTaxSalesAmount, h.ZeroTaxSalesAmount, h.TotalAmount)
	}

	buckets := map[TaxCategory]int64{}
	for _, l := range d.Items {
		buckets[l.TaxCategory] += l.NetAmount
	}
	for _, c := range []TaxCategory{TaxCategoryTaxable, TaxCategoryZeroRate, TaxCategoryExempt} {
		if v, ok := buckets[c]; ok && v < 0 {
			return invariantErr(CodeNegativeBucket, "tax category %d bucket sum=%d", int16(c), v)
		}
	}

	for _, l := range d.Lines {
		switch l.LineType {
		case lineTypeProduct, lineTypeShipping:
			if l.NetAmount < 0 {
				return invariantErr(CodeLineSignInvalid, "line_seq=%d type=%s net=%d", l.LineSeq, l.LineType, l.NetAmount)
			}
		case lineTypeDiscount:
			if l.NetAmount >= 0 {
				return invariantErr(CodeLineSignInvalid, "line_seq=%d type=%s net=%d", l.LineSeq, l.LineType, l.NetAmount)
			}
		default:
			return invariantErr(CodeLineSignInvalid, "line_seq=%d unknown line_type=%q", l.LineSeq, l.LineType)
		}
	}

	for _, l := range d.Items {
		if l.NetAmount == 0 {
			return invariantErr(CodeZeroLineEmitted, "line_seq=%d has zero gross but is emitted", l.LineSeq)
		}
	}

	var g int64
	for _, l := range d.Items {
		if l.TaxCategory == TaxCategoryTaxable {
			g += l.NetAmount
		}
	}
	if IsB2B(d.BuyerIdentifier) {
		wantTax := TaxOf(g)
		if h.TaxAmount != wantTax || h.SalesAmount != g-wantTax {
			return invariantErr(CodeB2BTaxMiscalc,
				"taxable_gross=%d want_tax=%d got_tax=%d want_sales=%d got_sales=%d",
				g, wantTax, h.TaxAmount, g-wantTax, h.SalesAmount)
		}
	} else if h.TaxAmount != 0 {
		return invariantErr(CodeB2CTaxNotZero, "b2c tax_amount=%d", h.TaxAmount)
	}

	for _, l := range d.Lines {
		if !l.TaxCategory.IsLineLevel() {
			return invariantErr(CodeHeaderTaxTypeMismatch,
				"line_seq=%d tax_category=%d is not line-level (9 only allowed on header)", l.LineSeq, int16(l.TaxCategory))
		}
	}
	if want := HeaderTaxType(d.Items); h.TaxType != want {
		return invariantErr(CodeHeaderTaxTypeMismatch, "header tax_type=%d want=%d", int16(h.TaxType), int16(want))
	}

	set := TaxCategorySet(d.Items)

	for _, c := range set {
		if c == TaxCategoryZeroRate {
			return invariantErr(CodeZeroRateNotSupported, "zero-rated line present")
		}
	}

	if IsB2B(d.BuyerIdentifier) && len(set) > 1 {
		return invariantErr(CodeB2BMixedTaxUnverified, "b2b with %d tax categories", len(set))
	}

	if len(d.Items) < 1 || len(d.Items) > AmegoLineCountMax {
		return invariantErr(CodeLineCountInvalid, "emitted line count=%d", len(d.Items))
	}

	for _, l := range d.Items {
		if l.Description == "" {
			return invariantErr(CodeDescriptionInvalid, "line_seq=%d empty description", l.LineSeq)
		}
		if n := utf8.RuneCountInString(l.Description); n > DescriptionMaxRunes {
			return invariantErr(CodeDescriptionInvalid, "line_seq=%d description rune count=%d", l.LineSeq, n)
		}
	}

	if h.DetailVat != DetailVatInclusive {
		return invariantErr(CodeDetailVatInvalid, "detail_vat=%d", h.DetailVat)
	}

	picked := 0
	if IsB2B(d.BuyerIdentifier) {
		picked++
	}
	if d.CarrierValue != "" {
		picked++
	}
	if d.DonationCode != "" {
		picked++
	}
	if picked > 1 {
		return invariantErr(CodeBuyerInfoConflict,
			"buyer_identifier=%q carrier=%q donation=%q", d.BuyerIdentifier, d.CarrierType, d.DonationCode)
	}

	if h.TotalAmount > MaxAmount {
		return invariantErr(CodeAmountExceedsLimit, "total=%d max=%d", h.TotalAmount, MaxAmount)
	}

	var subtotal int64
	for _, it := range d.Order.Items {
		subtotal += int64(it.Quantity) * it.UnitPrice
	}
	if subtotal != d.Order.TotalOriginalAmount {
		return invariantErr(CodeSubtotalMismatch,
			"sum(unit_price*qty)=%d total_original_amount=%d", subtotal, d.Order.TotalOriginalAmount)
	}

	if d.Order.DiscountAmount > 0 && d.Order.DiscountFunding != DiscountFundingSeller {
		return invariantErr(CodePlatformFundedDiscount,
			"discount=%d funding=%d", d.Order.DiscountAmount, int(d.Order.DiscountFunding))
	}

	if base := SumAllowanceableBase(d.Lines); base != h.TotalAmount {
		return invariantErr(CodeAllowanceBaseMismatch, "sum(allowanceable_base)=%d total=%d", base, h.TotalAmount)
	}

	return nil
}
