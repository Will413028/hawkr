package domain

import (
	"errors"
	"fmt"
	"sort"
	"unicode/utf8"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
)

var (
	lineTypeProduct  = codemeta.InvoiceLineTypeProduct.Value
	lineTypeShipping = codemeta.InvoiceLineTypeShipping.Value
	lineTypeDiscount = codemeta.InvoiceLineTypeDiscount.Value
)

// DescriptionMaxRunes 是 AMEGO 對 ProductItem.Description 的長度上限（實測 256 PASS／257 → 3040139）。
const DescriptionMaxRunes = 256

// ShippingDescription 與 DiscountDescription 是兩種非商品行的固定品名。
const (
	ShippingDescription = "運費"
	DiscountDescription = "優惠折抵"
)

// ErrZeroAmount 表示訂單實付金額為 0（全額折抵）⇒ 不開票。
//
// 這是正常分支不是失敗：呼叫端要建 invoice_documents 一列、status='NOT_APPLICABLE'、
// not_applicable_reason='ZERO_AMOUNT'，訂單詳情顯示原因。刻意不走 InvariantError。
var ErrZeroAmount = errors.New("invoice: order total actual amount is zero")

// DiscountFunding 回答「這筆折扣是誰出的錢」。
//
// 零值必須是 Unknown（＝擋下來）而不是 Seller：未來新增一條建構快照的路徑而忘了填時，
// 行為是 fail-closed 而不是 fail-open。靜默把平台補貼的那一段從賣家銷售額扣掉 ＝ 賣家
// 短漏報銷售額，那是稅務錯誤，要等國稅局查帳才浮現。
type DiscountFunding int

const (
	DiscountFundingUnknown  DiscountFunding = 0
	DiscountFundingSeller   DiscountFunding = 1
	DiscountFundingPlatform DiscountFunding = 2
)

// OrderItem 是建立明細行所需的單列 order_items 快照。
//
// 刻意不收 actual_pay_amount／share_discount：明細行一律從 unit_price × quantity、shipping_fee、
// discount_amount 三個原始欄位重建。既有分攤器 CheckoutService.allocateShareDiscount 用浮點
// 四捨五入、尾差歸最後一筆、負值夾成 0，三者都會讓加總不變式失效；收了那兩個欄位就會有人拿去算金額。
type OrderItem struct {
	OrderItemID int64
	SkuID       string // catalog SKU UUID；空字串表示 non-catalog 行
	ProductName string
	SkuName     string
	Quantity    int32
	UnitPrice   int64 // 含稅，成交價（不是 list_price）
}

// OrderSnapshot 是建立一份發票文件所需的訂單快照。
//
// TotalOriginalAmount 欄名誤導：orders 那一欄叫 original 但存的是成交價小計
// （Σ unit_price × quantity），原價在 order_items.list_price、僅供前端刪除線。
type OrderSnapshot struct {
	TotalOriginalAmount int64 // S
	ShippingFee         int64 // F，已是買家實際被收的金額；免運成立時自己就是 0
	DiscountAmount      int64 // D
	TotalActualAmount   int64 // ＝ max(0, S + F − D)，也是發票總額的唯一權威
	DiscountFunding     DiscountFunding
	Items               []OrderItem
}

// Line 是一列 invoice_document_lines。
//
// NetAmount 就是本章正文說的 gross_amount：同一個欄位、同一個值（＝ quantity × unit_price、
// 送 AMEGO 的 ProductItem.Amount）。寫 SQL／sqlc query／struct tag 時一律 net_amount。
type Line struct {
	LineSeq     int32
	LineType    string // codemeta InvoiceLineType 的 Value
	OrderItemID int64  // 0 表示 NULL（SHIPPING／DISCOUNT）
	SkuID       string // 空字串表示 NULL
	Description string
	Quantity    int32
	UnitPrice   int64
	NetAmount   int64 // ＝ Quantity × UnitPrice
	TaxCategory TaxCategory

	// AllocatedDiscount 是分攤到本行的折扣；AllowanceableBase = NetAmount − AllocatedDiscount。
	// SHIPPING 行的 AllocatedDiscount 不可省略——溢出額已被算進 DISCOUNT 行的負數，
	// 不同時從 SHIPPING 行的基數扣掉，同一筆折扣就被兩處各自認帳，而折讓守衛 I3／A17
	// 用的正是同一組 base ⇒ 它擋不住自己算錯的數。唯一攔截點是 A23。
	AllocatedDiscount int64
	AllowanceableBase int64

	// AllowancedAmount 是已折讓累計。沒有它就只能每次拿原始基數當權重，
	// 已折完的行會被再折一次（§5A.9.2）。建立文件時恆為 0。
	AllowancedAmount int64
}

// Emitted 回報本行是否會出現在送給 AMEGO 的 ProductItem 陣列中。
//
// gross_amount == 0 的行一律不輸出，但仍落庫供對帳 ⇒ 有 0 元行時 line_seq 與陣列索引必然錯開，
// 兩者不得互相推導（折讓端用 invoice_document_lines 逐行對應、不用索引）。
func (l Line) Emitted() bool { return l.NetAmount != 0 }

// EmittedLines 回傳會被送出的行，維持原本的 line_seq 順序。
func EmittedLines(lines []Line) []Line {
	out := make([]Line, 0, len(lines))
	for _, l := range lines {
		if l.Emitted() {
			out = append(out, l)
		}
	}
	return out
}

func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	i := 0
	for idx := range s {
		if i == n {
			return s[:idx]
		}
		i++
	}
	return s
}

func productDescription(it OrderItem) string {
	name := it.ProductName
	if it.SkuName != "" && it.SkuName != it.ProductName {
		name = it.ProductName + " " + it.SkuName
	}
	return truncateRunes(name, DescriptionMaxRunes)
}

// BuildLines 依 §5A.3／§5A.5 從訂單快照組出全部明細行（含 gross == 0 的落庫行）。
//
// 產生順序即 line_seq（1-based），決定性、不依賴 map 迭代序：
//
//	1..N   PRODUCT   每列 order_items 一行，依 order_items.id 升冪
//	N+1    SHIPPING  shipping_fee > 0 時恰好一行，TaxType 恆為 1（應稅）
//	N+2..  DISCOUNT  依稅別聚合，每個「分到折扣 > 0」的稅別一行，稅別碼升冪
//
// 實付為 0 時回 ErrZeroAmount（正常分支）。
func BuildLines(snap OrderSnapshot, resolver TaxCategoryResolver) ([]Line, error) {
	if resolver == nil {
		return nil, fmt.Errorf("invoice: BuildLines requires a TaxCategoryResolver")
	}
	if snap.TotalActualAmount == 0 {
		return nil, ErrZeroAmount
	}
	if snap.TotalActualAmount < 0 || snap.ShippingFee < 0 || snap.DiscountAmount < 0 {
		return nil, fmt.Errorf("invoice: negative order amounts in snapshot")
	}

	items := make([]OrderItem, len(snap.Items))
	copy(items, snap.Items)
	sort.SliceStable(items, func(a, b int) bool { return items[a].OrderItemID < items[b].OrderItemID })

	lines := make([]Line, 0, len(items)+2)
	var productSubtotal int64
	for _, it := range items {
		cat := resolver.ResolveProduct(it.SkuID)
		gross := int64(it.Quantity) * it.UnitPrice
		productSubtotal += gross
		lines = append(lines, Line{
			LineSeq:     int32(len(lines) + 1),
			LineType:    lineTypeProduct,
			OrderItemID: it.OrderItemID,
			SkuID:       it.SkuID,
			Description: productDescription(it),
			Quantity:    it.Quantity,
			UnitPrice:   it.UnitPrice,
			NetAmount:   gross,
			TaxCategory: cat,
		})
	}
	productCount := len(lines)

	productDiscount := snap.DiscountAmount
	if productDiscount > productSubtotal {
		productDiscount = productSubtotal
	}
	shippingDiscount := snap.DiscountAmount - productDiscount

	if snap.ShippingFee > 0 {
		lines = append(lines, Line{
			LineSeq:           int32(len(lines) + 1),
			LineType:          lineTypeShipping,
			Description:       ShippingDescription,
			Quantity:          1,
			UnitPrice:         snap.ShippingFee,
			NetAmount:         snap.ShippingFee,
			TaxCategory:       TaxCategoryTaxable,
			AllocatedDiscount: shippingDiscount,
			AllowanceableBase: snap.ShippingFee - shippingDiscount,
		})
	}

	bucketCats, bucketGross := bucketsOf(lines[:productCount])
	bucketAlloc, err := AllocateLargestRemainder(bucketGross, productDiscount)
	if err != nil {
		return nil, fmt.Errorf("invoice: bucket allocation: %w", err)
	}

	discountByCategory := make(map[TaxCategory]int64, len(bucketCats)+1)
	for bi, cat := range bucketCats {
		discountByCategory[cat] += bucketAlloc[bi]

		idx := make([]int, 0, productCount)
		w := make([]int64, 0, productCount)
		for i := 0; i < productCount; i++ {
			if lines[i].TaxCategory == cat && lines[i].NetAmount > 0 {
				idx = append(idx, i)
				w = append(w, lines[i].NetAmount)
			}
		}
		lineAlloc, err := AllocateLargestRemainder(w, bucketAlloc[bi])
		if err != nil {
			return nil, fmt.Errorf("invoice: line allocation (tax category %d): %w", int16(cat), err)
		}
		for k, i := range idx {
			lines[i].AllocatedDiscount = lineAlloc[k]
			lines[i].AllowanceableBase = lines[i].NetAmount - lineAlloc[k]
		}
	}
	if shippingDiscount > 0 {
		discountByCategory[TaxCategoryTaxable] += shippingDiscount
	}

	cats := make([]TaxCategory, 0, len(discountByCategory))
	for c, v := range discountByCategory {
		if v > 0 {
			cats = append(cats, c)
		}
	}
	sort.Slice(cats, func(a, b int) bool { return cats[a] < cats[b] })
	for _, c := range cats {
		amount := discountByCategory[c]
		lines = append(lines, Line{
			LineSeq:           int32(len(lines) + 1),
			LineType:          lineTypeDiscount,
			Description:       DiscountDescription,
			Quantity:          1,
			UnitPrice:         -amount,
			NetAmount:         -amount,
			TaxCategory:       c,
			AllocatedDiscount: 0,
			AllowanceableBase: 0,
		})
	}
	return lines, nil
}

func bucketsOf(productLines []Line) ([]TaxCategory, []int64) {
	sums := make(map[TaxCategory]int64, 3)
	for _, l := range productLines {
		sums[l.TaxCategory] += l.NetAmount
	}
	cats := make([]TaxCategory, 0, len(sums))
	for c, v := range sums {
		if v > 0 {
			cats = append(cats, c)
		}
	}
	sort.Slice(cats, func(a, b int) bool { return cats[a] < cats[b] })
	gross := make([]int64, len(cats))
	for i, c := range cats {
		gross[i] = sums[c]
	}
	return cats, gross
}
