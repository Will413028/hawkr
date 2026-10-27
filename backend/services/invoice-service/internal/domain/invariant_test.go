package domain

import (
	"errors"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
)

// AllInvariantCodes 是 A 表 23 條的理由碼，順序即編號。測試用它斷言「變異數 ＝ 斷言數」。
var allInvariantCodes = []string{
	CodeAmountMismatch,          // A1
	CodeTaxSplitMismatch,        // A2
	CodeNegativeHeaderAmount,    // A3
	CodeNegativeBucket,          // A4
	CodeLineSignInvalid,         // A5
	CodeZeroLineEmitted,         // A6
	CodeB2CTaxNotZero,           // A7
	CodeB2BTaxMiscalc,           // A8
	CodeHeaderTaxTypeMismatch,   // A9
	CodeZeroRateNotSupported,    // A10
	CodeB2BMixedTaxUnverified,   // A11
	CodeLineCountInvalid,        // A12
	CodeDescriptionInvalid,      // A13
	CodeDetailVatInvalid,        // A14
	CodeBuyerInfoConflict,       // A15
	CodeAllowanceSplitMismatch,  // A16
	CodeAllowanceExceedsInvoice, // A17
	CodeAllowanceTaxMismatch,    // A18
	CodeAllowanceMultiInvoice,   // A19
	CodeAmountExceedsLimit,      // A20
	CodeSubtotalMismatch,        // A21
	CodePlatformFundedDiscount,  // A22
	CodeAllowanceBaseMismatch,   // A23
}

// 每個理由碼都必須查得回生成的 enum —— 手寫字面值漂移時這一條會轉紅。
func TestInvariantCodesAreCodeMetaValues(t *testing.T) {
	if len(allInvariantCodes) != 23 {
		t.Fatalf("invariant code list has %d entries, want 23", len(allInvariantCodes))
	}
	seen := map[string]struct{}{}
	for i, c := range allInvariantCodes {
		if _, dup := seen[c]; dup {
			t.Errorf("A%d: duplicate reason code %q", i+1, c)
		}
		seen[c] = struct{}{}
		if _, ok := codemeta.InvoiceManualReasonFromValue(c); !ok {
			t.Errorf("A%d: reason code %q not found in generated InvoiceManualReason", i+1, c)
		}
		if len(c) > 32 {
			t.Errorf("A%d: reason code %q is %d chars, exceeds varchar(32)", i+1, c, len(c))
		}
	}
}

func plainB2C(t *testing.T) *IssueDraft {
	t.Helper()
	return newExampleDraft(t, []exItem{
		{"A", 1, 333, TaxCategoryTaxable},
		{"B", 1, 333, TaxCategoryTaxable},
		{"C", 1, 334, TaxCategoryTaxable},
	}, 0, 100, B2CBuyerIdentifier)
}

func shippingB2C(t *testing.T) *IssueDraft {
	t.Helper()
	return newExampleDraft(t, []exItem{
		{"A", 1, 200, TaxCategoryTaxable},
		{"B", 1, 50, TaxCategoryTaxable},
	}, 150, 300, B2CBuyerIdentifier)
}

func plainB2B(t *testing.T) *IssueDraft {
	t.Helper()
	return newExampleDraft(t, []exItem{{"測試商品1", 1, 170, TaxCategoryTaxable}}, 0, 2, "45444444")
}

// 逐條把 A1–A15、A20–A23 各弄壞一次，斷言回對應的理由碼。
// 只驗正例（送對的輸入看它回 nil）的斷言測試等於沒有斷言。
func TestIssueDraftValidate_MutationPerAssertion(t *testing.T) {
	cases := []struct {
		name   string
		code   string
		draft  func(*testing.T) *IssueDraft
		mutate func(*IssueDraft)
	}{
		{"A1 表頭總額對不上訂單實付", CodeAmountMismatch, plainB2C, func(d *IssueDraft) {
			d.Order.TotalActualAmount++
		}},
		{"A2 四欄加總不等於總額", CodeTaxSplitMismatch, plainB2C, func(d *IssueDraft) {
			d.Header.SalesAmount++ // TotalAmount 不動 ⇒ A1 仍成立
		}},
		{"A3 銷售額欄位為負", CodeNegativeHeaderAmount, plainB2C, func(d *IssueDraft) {
			d.Header.FreeTaxSalesAmount = -1
			d.Header.SalesAmount++ // 維持 A2
		}},
		{"A4 稅別桶合計為負", CodeNegativeBucket, plainB2C, func(d *IssueDraft) {
			d.Items = append(d.Items, Line{
				LineSeq: 99, LineType: lineTypeDiscount, NetAmount: -100000, TaxCategory: TaxCategoryTaxable,
			})
		}},
		{"A5 PRODUCT 行為負", CodeLineSignInvalid, plainB2C, func(d *IssueDraft) {
			d.Lines[3].LineType = lineTypeProduct // 折扣行改標成商品行 ⇒ 負數的 PRODUCT
		}},
		{"A6 送出 0 元行", CodeZeroLineEmitted, plainB2C, func(d *IssueDraft) {
			d.Items = append(d.Items, Line{
				LineSeq: 99, LineType: lineTypeProduct, Description: "贈品", Quantity: 1,
				NetAmount: 0, TaxCategory: TaxCategoryTaxable,
			})
		}},
		{"A7 B2C 稅額非零", CodeB2CTaxNotZero, plainB2C, func(d *IssueDraft) {
			d.Header.TaxAmount = 5
			d.Header.SalesAmount -= 5 // 維持 A2
		}},
		{"A8 B2B 稅額算錯", CodeB2BTaxMiscalc, plainB2B, func(d *IssueDraft) {
			d.Header.TaxAmount = 9
			d.Header.SalesAmount = 159 // 維持 A2（9+159 = 168）
		}},
		{"A9 表頭稅別與輸出行不符", CodeHeaderTaxTypeMismatch, plainB2C, func(d *IssueDraft) {
			d.Header.TaxType = TaxCategoryMixed
		}},
		{"A9 行級出現 9", CodeHeaderTaxTypeMismatch, plainB2C, func(d *IssueDraft) {
			d.Lines[0].TaxCategory = TaxCategoryMixed
		}},
		{"A10 出現零稅率行", CodeZeroRateNotSupported, func(t *testing.T) *IssueDraft {
			t.Helper()
			return newExampleDraft(t, []exItem{{"外銷A", 1, 500, TaxCategoryZeroRate}}, 0, 0, B2CBuyerIdentifier)
		}, func(*IssueDraft) {}},
		{"A11 打統編且混合稅別", CodeB2BMixedTaxUnverified, func(t *testing.T) *IssueDraft {
			t.Helper()
			return newExampleDraft(t, []exItem{
				{"生鮮A", 2, 380, TaxCategoryExempt},
				{"生鮮B", 1, 450, TaxCategoryExempt},
				{"生鮮C", 1, 290, TaxCategoryExempt},
			}, 150, 100, "45444444")
		}, func(*IssueDraft) {}},
		{"A12 沒有任何輸出行", CodeLineCountInvalid, plainB2C, func(d *IssueDraft) {
			d.Items = nil
			d.Header.TaxType = 0 // 讓 A9 仍成立，把失敗留給 A12
		}},
		{"A13 品名為空", CodeDescriptionInvalid, plainB2C, func(d *IssueDraft) {
			d.Items[0].Description = ""
		}},
		{"A13 品名超過 256 runes", CodeDescriptionInvalid, plainB2C, func(d *IssueDraft) {
			d.Items[0].Description = strings.Repeat("鮮", 257)
		}},
		{"A14 DetailVat 非 1", CodeDetailVatInvalid, plainB2C, func(d *IssueDraft) {
			d.Header.DetailVat = 0
		}},
		{"A15 統編與載具同時存在", CodeBuyerInfoConflict, plainB2B, func(d *IssueDraft) {
			d.CarrierType = AmegoCarrierMobileBarcode
			d.CarrierValue = "/ABC1234"
		}},
		{"A20 總額超過業務上限", CodeAmountExceedsLimit, func(t *testing.T) *IssueDraft {
			t.Helper()
			return newExampleDraft(t, []exItem{{"高單價", 1, 200_000, TaxCategoryTaxable}}, 0, 0, B2CBuyerIdentifier)
		}, func(*IssueDraft) {}},
		{"A21 訂單小計自相矛盾", CodeSubtotalMismatch, plainB2C, func(d *IssueDraft) {
			d.Order.TotalOriginalAmount++
		}},
		{"A22 折扣出資方非賣家", CodePlatformFundedDiscount, plainB2C, func(d *IssueDraft) {
			d.Order.DiscountFunding = DiscountFundingUnknown
		}},
		{"A23 SHIPPING 行漏扣分攤折扣", CodeAllowanceBaseMismatch, shippingB2C, func(d *IssueDraft) {
			// 這正是 5A.5.2 說的漏賦值：chk_invline_allowance_shape 照過（base = net − 0），
			// 只有 A23 擋得住。
			d.Lines[2].AllocatedDiscount = 0
			d.Lines[2].AllowanceableBase = d.Lines[2].NetAmount
		}},
	}

	covered := map[string]struct{}{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := tc.draft(t)
			tc.mutate(d)
			err := d.Validate()
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

	// 變異數 ＝ 斷言數：之後新增斷言而忘了加變異時，這裡會轉紅。
	for i, c := range allInvariantCodes {
		if i >= 15 && i <= 18 {
			continue // A16–A19 是折讓側，在 TestAllowanceDraftValidate_MutationPerAssertion
		}
		if _, ok := covered[c]; !ok {
			t.Errorf("A%d (%s) has no mutation case", i+1, c)
		}
	}
}

// 正例：六個算例的 draft 全部通過 Validate（在 example_5a7_test.go 內已各驗一次），
// 這裡另驗「乾淨的草稿回 nil」與「B2C 不帶任何買受人資訊也合法」。
func TestIssueDraftValidate_HappyPath(t *testing.T) {
	if err := plainB2C(t).Validate(); err != nil {
		t.Fatalf("plain B2C: %v", err)
	}
	if err := plainB2B(t).Validate(); err != nil {
		t.Fatalf("plain B2B: %v", err)
	}
	d := plainB2C(t)
	d.CarrierType = AmegoCarrierMobileBarcode
	d.CarrierValue = "/ABC1234"
	if err := d.Validate(); err != nil {
		t.Fatalf("B2C with carrier only should pass A15: %v", err)
	}
}

func TestBuildLines_ZeroAmountIsNotAFailure(t *testing.T) {
	_, err := BuildLines(OrderSnapshot{TotalActualAmount: 0}, SellerDefaultTaxResolver{Default: TaxCategoryTaxable})
	if !errors.Is(err, ErrZeroAmount) {
		t.Fatalf("BuildLines = %v, want ErrZeroAmount", err)
	}
	var ie *InvariantError
	if errors.As(err, &ie) {
		t.Fatalf("ErrZeroAmount must not be an InvariantError (正常分支不是失敗)")
	}
}

// 0 元 PRODUCT 行仍落庫、但不輸出 ⇒ line_seq 與送出陣列索引必然錯開，兩者不得互相推導。
func TestBuildLines_ZeroGrossLinePersistedButNotEmitted(t *testing.T) {
	d := newExampleDraft(t, []exItem{
		{"贈品", 1, 0, TaxCategoryTaxable},
		{"A", 1, 500, TaxCategoryTaxable},
	}, 0, 0, B2CBuyerIdentifier)
	if err := d.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if len(d.Lines) != 2 || len(d.Items) != 1 {
		t.Fatalf("len(Lines)=%d len(Items)=%d, want 2 and 1", len(d.Lines), len(d.Items))
	}
	if d.Items[0].LineSeq != 2 {
		t.Fatalf("emitted line_seq = %d, want 2 (索引 0 但 line_seq 2)", d.Items[0].LineSeq)
	}
}
