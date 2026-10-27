package domain

import (
	"fmt"
	"strconv"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
)

// TaxCategory 對齊 AMEGO ProductItem.TaxType。
// 值域刻意與 AMEGO 一致，避免在 provider adapter 再做一次映射。
type TaxCategory int16

const (
	TaxCategoryTaxable  TaxCategory = 1 // 應稅
	TaxCategoryZeroRate TaxCategory = 2 // 零稅率
	TaxCategoryExempt   TaxCategory = 3 // 免稅

	// TaxCategoryMixed 只允許出現在表頭（Header.TaxType）。行級（Line.TaxCategory）值域只有
	// 1/2/3，對應 §5 的兩條不同 CHECK：
	//   chk_invline_tax_category IN (1,2,3) vs chk_invdoc_tax_type IN (1,2,3,9)。
	// 同一個 Go 型別承載兩種值域是刻意的，代價是「行級不得為 9」只能由 A9 守住。
	TaxCategoryMixed TaxCategory = 9
)

// IsLineLevel 回報這個值是否可以出現在明細行上。
func (tc TaxCategory) IsLineLevel() bool {
	return tc == TaxCategoryTaxable || tc == TaxCategoryZeroRate || tc == TaxCategoryExempt
}

// IsHeaderLevel 回報這個值是否可以出現在表頭。
func (tc TaxCategory) IsHeaderLevel() bool {
	return tc.IsLineLevel() || tc == TaxCategoryMixed
}

func (tc TaxCategory) String() string { return strconv.Itoa(int(tc)) }

// TaxCategoryToDB 把 CodeMeta 常數換成 invoice_document_lines.tax_category 的存值。
//
// InvoiceTaxCategory 是全書唯一 Code != Value 的 enum（Code = TAXABLE/ZERO_RATED/TAX_EXEMPT、
// Value = "1"/"2"/"3"），也是唯一會踩到型別轉換的：registry struct 每個欄位都是 string，
// 而 DB 欄位是 smallint。轉換只寫在本檔一處。
func TaxCategoryToDB(tc codemeta.InvoiceTaxCategory) (int16, error) {
	n, err := strconv.Atoi(tc.Value) // Value 恆為 "1"/"2"/"3"
	if err != nil {
		return 0, fmt.Errorf("tax category value not numeric: %q", tc.Value)
	}
	return int16(n), nil
}

// TaxCategoryFromDB 把 smallint 存值換回 CodeMeta 常數。
func TaxCategoryFromDB(v int16) (codemeta.InvoiceTaxCategory, bool) {
	return codemeta.InvoiceTaxCategoryFromValue(strconv.Itoa(int(v)))
}

// 三個容易寫錯的地方：
//   ① 寫 DB 前先過 TaxCategoryToDB，不要直接 int16(row.TaxCategory)（後者拿到的是 Code 字串）
//   ② 讀回來要 strconv.Itoa 再 FromValue，拿 Code 去 FromValue 必然 ok=false
//   ③ 轉換失敗不要 fallback 成 1（應稅）—— 那會把一張免稅發票靜默開成應稅；
//      失敗一律 fail-closed 轉 AWAITING_MANUAL ＋ manual_reason_code='MIXED_TAX_CATEGORY'
//
// 送 AMEGO 用的是 tc.Value（字串 "1"）不是 TaxCategoryToDB 的結果（AMEGO 的 TaxType 是字元）。

// TaxCategoryToAmego 回傳送給 AMEGO 的 ProductItem.TaxType 字元。
func TaxCategoryToAmego(tc TaxCategory) (string, error) {
	meta, ok := TaxCategoryFromDB(int16(tc))
	if !ok {
		return "", fmt.Errorf("invoice: tax category has no CodeMeta entry: %d", int16(tc))
	}
	return meta.Value, nil
}

// TaxCategoryResolver 決定每一列 order_items 的課稅別。
//
//	Phase 1: SellerDefaultTaxResolver（每賣家一個值）
//	Phase 2: SkuTaxResolver（讀 order_invoice_line_snapshots.tax_category）
//
// 抽成介面的目的是讓 Phase 2 在 invoice-service 側零 schema 變更，只換實作與 fx 綁定。
type TaxCategoryResolver interface {
	// ResolveProduct 必須是純函式：所有外部資料在建構 resolver 時就已快照完畢，
	// Resolve 期間不得再查 DB／gRPC。
	ResolveProduct(skuID int64) TaxCategory
}

// SellerDefaultTaxResolver 是 Phase 1 唯一實作：整張訂單同一個課稅別。
//
// 唯一合法來源是 ADM 審核賣家稅務資格時依核定函所填的
// seller_tax_profiles.default_tax_category，且必須在建立 invoice_documents 的交易內快照——
// 該值可被 ADM 複審修改，而開票與付款之間有窗口、失敗還會重試數小時。
type SellerDefaultTaxResolver struct{ Default TaxCategory }

// ResolveProduct 回傳賣家層級的預設課稅別，與 skuID 無關。
func (r SellerDefaultTaxResolver) ResolveProduct(int64) TaxCategory { return r.Default }
