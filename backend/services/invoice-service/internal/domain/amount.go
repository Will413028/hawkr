package domain

import "sort"

// B2CBuyerIdentifier 是二聯式（買受人為個人）的統編佔位值：固定 10 個 0。
const B2CBuyerIdentifier = "0000000000"

// 表頭固定欄位。
const (
	// TaxRateLiteral 恆帶 "0.05"（String）。未決 U2：純免稅發票（表頭 TaxType=3）是否也必須帶
	// 它，官方所有範例都是應稅、沒有免稅範例而欄位標必填 ⇒ 未查證前跟隨所有官方範例恆帶。
	TaxRateLiteral = "0.05"

	// DetailVatInclusive 恆為 1：本平台的價格都是含稅價（台灣 B2C 標價一律含稅，營業稅法
	// §32 II），orders/order_items 也沒有任何未稅欄位。連 B2B 也走 1——官方 B2B 範例即含稅明細
	// 只在表頭拆稅；DetailVat=0 會讓 UnitPrice 變小數，且會踩到一個未查證的捨入模式。
	DetailVatInclusive = 1

	// TaxRateBps 是營業稅率的 basis point 表示（5% ＝ 500），落 invoice_documents.tax_rate_bps
	// 與 invoice_document_lines.tax_rate_bps（兩者的 DB DEFAULT 皆為 500）。
	// 送 AMEGO 時用的是 TaxRateLiteral（"0.05"）而不是本值——兩種表示法不得互相推導：
	// bps 是整數存庫用，字串是 provider 契約，中間差一次除法就會出現 0.049999。
	TaxRateBps = 500

	// DetailAmountRoundOn 恆為 1。我方 Amount 本來就都是整數，此旗標是恆等操作；
	// 零成本的未來保險：導入秤重小數數量後，它把 AMEGO 的重算鎖在整數。
	DetailAmountRoundOn = 1
)

// Header 是 AMEGO 開立 request 的表頭金額四欄與其相依旗標。
type Header struct {
	SalesAmount        int64 // 應稅銷售額
	FreeTaxSalesAmount int64 // 免稅銷售額
	ZeroTaxSalesAmount int64 // 零稅率銷售額
	TaxAmount          int64
	TotalAmount        int64

	TaxType           TaxCategory // 1/2/3/9，由實際輸出的行推導
	TaxRate           string
	DetailVat         int
	DetailAmountRound int
}

// IsB2B 回報這份表頭對應的是不是三聯式（買受人打了統編）。
func IsB2B(buyerIdentifier string) bool {
	return buyerIdentifier != "" && buyerIdentifier != B2CBuyerIdentifier
}

// BuildHeader 依 §5A.6.4 由「實際輸出的行」算出表頭四欄。
//
// 先分攤再算稅：全程以含稅整數做完分攤，只在表頭做唯一一次除法。官方算式就是「先把 Amount
// 依稅別加總，再拆稅」⇒ 這樣才與 AMEGO 的重算逐位元一致。反過來（先算稅再分攤）會讓每行各
// 產生一個尾差，而 AMEGO 用表頭重算後會以 3040177／3040178 否決。
func BuildHeader(lines []Line, buyerIdentifier string) Header {
	var g, z, fx int64
	seen := make(map[TaxCategory]struct{}, 3)
	for _, l := range lines {
		if !l.Emitted() {
			continue
		}
		seen[l.TaxCategory] = struct{}{}
		switch l.TaxCategory {
		case TaxCategoryTaxable:
			g += l.NetAmount // 含稅、恆 >= 0（DISCOUNT 行為負，已相抵）
		case TaxCategoryZeroRate:
			z += l.NetAmount
		case TaxCategoryExempt:
			fx += l.NetAmount
		}
	}

	h := Header{
		FreeTaxSalesAmount: fx,
		ZeroTaxSalesAmount: z,
		TaxType:            HeaderTaxType(lines),
		TaxRate:            TaxRateLiteral,
		DetailVat:          DetailVatInclusive,
		DetailAmountRound:  DetailAmountRoundOn,
	}
	if IsB2B(buyerIdentifier) {
		h.TaxAmount = TaxOf(g)
		h.SalesAmount = g - h.TaxAmount
	} else {
		// 沒打統編的發票 TaxAmount 一律帶 0（[AMEGO]:464 逐字）。誤帶 5 會回 3040174
		// 「SalesAmount 計算錯誤」——訊息指向錯誤的欄位，所以一定要在我方擋（A7）。
		h.TaxAmount = 0
		h.SalesAmount = g
	}
	h.TotalAmount = h.SalesAmount + h.FreeTaxSalesAmount + h.ZeroTaxSalesAmount + h.TaxAmount
	return h
}

// HeaderTaxType 由實際輸出的行的稅別集合推導表頭 TaxType：
// 集合大小 == 1 ⇒ 該稅別；>= 2 ⇒ 9（混合）。無輸出行時回 0（由 A12 攔）。
//
// 不是由賣家設定推導：免稅商品 ＋ 應稅運費的發票若取賣家設定會標成純免稅 ⇒ 應稅銷售額短報，
// 而表頭與明細自洽、AMEGO 不會拒絕，要等國稅局查帳才浮現。
func HeaderTaxType(lines []Line) TaxCategory {
	set := TaxCategorySet(lines)
	switch len(set) {
	case 0:
		return 0
	case 1:
		return set[0]
	default:
		return TaxCategoryMixed
	}
}

// TaxCategorySet 回傳實際輸出的行的稅別集合，升冪、去重。
func TaxCategorySet(lines []Line) []TaxCategory {
	seen := make(map[TaxCategory]struct{}, 3)
	for _, l := range lines {
		if l.Emitted() {
			seen[l.TaxCategory] = struct{}{}
		}
	}
	out := make([]TaxCategory, 0, len(seen))
	for c := range seen {
		out = append(out, c)
	}
	sort.Slice(out, func(a, b int) bool { return out[a] < out[b] })
	return out
}

// SumNetAmount 回傳所有行（含未輸出的 0 元行，加 0 不影響）的 gross 合計。
func SumNetAmount(lines []Line) int64 {
	var sum int64
	for _, l := range lines {
		sum += l.NetAmount
	}
	return sum
}

// SumAllowanceableBase 回傳 PRODUCT ＋ SHIPPING 行的 allowanceable_base 合計。
//
// 恆等式 Σ base = Σ(PRODUCT gross) + F − Σ allocated_discount = S + F − D = total_actual_amount，
// 成立的前提是 SHIPPING 行也被賦值 allocated_discount = shippingDiscount。這個恆等式是跨列
// 加總、寫不成 DB CHECK ⇒ 守護者是 APP ＋ A23 ＋ TEST。
func SumAllowanceableBase(lines []Line) int64 {
	var sum int64
	for _, l := range lines {
		if l.LineType == lineTypeDiscount {
			continue
		}
		sum += l.AllowanceableBase
	}
	return sum
}
