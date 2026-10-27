package handler

import (
	"regexp"

	"github.com/Will413028/hawkr/gateway/internal/response"
	pb "github.com/Will413028/hawkr/go-pkg/proto/order"
	"github.com/gin-gonic/gin"
)

// ==================== 超商取貨門市代號（cvsStoreCode） ====================

// shippingMethodCvsPickup 是唯一會用到超商門市代號的配送方式
// （`ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP`，7-11 店到店）。3001 黑貓宅配送到地址，
// 沒有門市可指定。
const shippingMethodCvsPickup = "3002"

// cvsStoreCodePattern：7-11 門市店號固定 6 碼數字（例 `252698`）。目前 CVS 只有
// 7-11 一家；若日後新增其他超商通路，這條規則要跟著 `ShippingMethod` 一起放寬。
var cvsStoreCodePattern = regexp.MustCompile(`^[0-9]{6}$`)

// resolveCvsStore 收斂 checkout 三個端點對「超商取貨門市」的解析與驗證。
//
// 🔴 這組欄位的歷史包袱：對外 JSON key 一直叫 `storeId`／`storeName`，跟平台的
// 賣家代理鍵 `store_id` **只有名字相同**——gateway 把它直接映進 proto 的
// `CvsStoreId`，最後落進 `orders.cvs_store_id`。誰照字面理解都會填成賣家
// store_id，而後端過去完全不驗：宅配單帶著賣家 store_id 也照收，靜靜寫進去。
// dev 上 2026-07-18～08-30 就這樣累積了 79 列宅配污染訂單（43 個買家，全部出自
// 驗證腳本；buyer app 與 website 兩個正式 client 都只在超商取貨時才帶，是對的）。
//
// 因此本函式做兩件事：
//  1. 正名——優先讀新的 `cvsStoreCode`／`cvsStoreName`；舊的 `storeId`／`storeName`
//     繼續接受（已上架的 App 直接呼叫這些端點，不能拔），只在文件上標 deprecated。
//  2. 驗證——非超商取貨不得帶門市代號；超商取貨的代號必須是 6 碼數字。兩者都直接
//     400，不做「靜默丟棄」：靜默丟棄會讓填錯的呼叫端永遠不知道自己填錯了，那正是
//     這批髒資料累積 6 週沒人發現的原因。
//
// 回傳 ok=false 時已經寫過 HTTP 回應，呼叫端直接 return。
func resolveCvsStore(c *gin.Context, shippingMethod, cvsStoreCode, cvsStoreName, legacyStoreID, legacyStoreName string) (code, name string, ok bool) {
	code = cvsStoreCode
	if code == "" {
		code = legacyStoreID
	}
	name = cvsStoreName
	if name == "" {
		name = legacyStoreName
	}

	if shippingMethod != shippingMethodCvsPickup {
		if code != "" {
			response.BadRequest(c, "非超商取貨不可指定門市代號（cvsStoreCode）")
			return "", "", false
		}
		// 沒有門市代號時，門市名稱也一併丟掉：留著只會寫出一列「有店名沒店號」的訂單。
		return "", "", true
	}

	// 超商取貨允許「還沒選門市」：買家在 App／網站上是先選 3002 看運費、再挑門市，
	// preview 這一步門市代號本來就可能還是空的（`checkout_legacy_golden_test.go`
	// 的 legacy fixture 就是這個形狀）。把它變成必填會直接打爛那一段流程。
	if code == "" {
		return "", name, true
	}
	if !cvsStoreCodePattern.MatchString(code) {
		response.BadRequest(c, "門市代號（cvsStoreCode）必須是 6 碼數字")
		return "", "", false
	}
	return code, name, true
}

// ==================== DTO ====================

// CheckoutValidateDataResponse 結帳驗證回應
type CheckoutValidateDataResponse struct {
	DeliveryTempIntersection []string                          `json:"deliveryTempIntersection"`
	AvailableShippingOptions []AvailableShippingOptionResponse `json:"availableShippingOptions"`
	Errors                   []CheckoutValidationErrorResponse `json:"errors"`
	UnavailableItems         []UnavailableItemResponse         `json:"unavailableItems"` // row 級不可購買清單（DELISTED / OUT_OF_STOCK / INSUFFICIENT_STOCK / LOOKUP_FAILED）。非空 → valid=false
	Valid                    bool                              `json:"valid"`
}

// AvailableShippingOptionResponse 可用配送選項
type AvailableShippingOptionResponse struct {
	ShippingMethod        string `json:"shippingMethod"`
	DeliveryTemp          string `json:"deliveryTemp"`
	Label                 string `json:"label"`
	Fee                   string `json:"fee"`
	FeePayer              string `json:"feePayer"`
	FreeShippingThreshold string `json:"freeShippingThreshold"`
	FreeShippingEligible  bool   `json:"freeShippingEligible"`
}

// CheckoutValidationErrorResponse 驗證錯誤項
type CheckoutValidationErrorResponse struct {
	ErrorCode    string `json:"errorCode"`
	ErrorMessage string `json:"errorMessage"`
}

// CheckoutShippingFeeDataResponse 運費精算回應
type CheckoutShippingFeeDataResponse struct {
	ShippingMethod        string `json:"shippingMethod"`
	DeliveryTemp          string `json:"deliveryTemp"`
	ShippingRoute         string `json:"shippingRoute"`
	ShippingFee           string `json:"shippingFee"`
	FeePayer              string `json:"feePayer"`
	FreeShippingThreshold string `json:"freeShippingThreshold"`
	OrderSubtotal         string `json:"orderSubtotal"`
	EstimatedTotal        string `json:"estimatedTotal"`
	FreeShippingEligible  bool   `json:"freeShippingEligible"`
}

// CheckoutPreviewDataResponse 結帳預覽回應
type CheckoutPreviewDataResponse struct {
	CheckoutID          string                      `json:"checkoutId"`
	TotalOriginalAmount string                      `json:"totalOriginalAmount"`
	TotalShippingFee    string                      `json:"totalShippingFee"`
	TotalDiscount       string                      `json:"totalDiscount"`
	TotalActualAmount   string                      `json:"totalActualAmount"`
	PaymentMerID        string                      `json:"paymentMerId"` // 本次交易付款對應 PayUni MerID（單賣家=seller.mer / 和單=代理商 mer）。前端 Google Pay 用此值
	OrderGroups         []OrderGroupPreviewResponse `json:"orderGroups"`
	UnavailableItems    []UnavailableItemResponse   `json:"unavailableItems,omitempty"` // row 級不可購買清單。非空 → envelope code=4001
	// 依 totalActualAmount 算出的各付款方式可用性（20 設計書 §3.4.1）。前端據此 render（禁用 + 理由），不自行用金額判斷。totalActualAmount==0（零元）→ 空清單，前端顯示「無需付款」
	AvailablePaymentMethods []AvailablePaymentMethodResponse `json:"availablePaymentMethods,omitempty"`
	// 結帳頁的發票區（58 §6.2.4 order ②、58A §B01）。
	// 🔴 omitempty ＋ 指標：invoice-service 拿不到時 order 不設這一欄，前端據此**不渲染**發票區，
	// 而不是渲染一個空的。回 `{}` 與「沒有這個 key」對前端是兩件事。
	Invoice *CheckoutInvoicePreviewResponse `json:"invoice,omitempty"`
}

// CheckoutInvoicePreviewResponse 是 58A §B01 的資料來源。
//
// 逐欄取自 proto，不在 gateway 做任何推導：invoiceMode 決定四個分支、disclosure 決定
// 顯示哪一段告知、intakeLimit.blocked 決定不可下單 banner —— 三者都是發票服務的判定。
type CheckoutInvoicePreviewResponse struct {
	// InvoiceFulfillmentMode：AMEGO_AUTO / PLATFORM_RECEIPT / SELLER_SELF_ISSUE / LEGACY_MANUAL
	InvoiceMode string `json:"invoiceMode"`
	// 由 order-service 覆蓋（I-50：它是 snapshot_version 的唯一寫入者）
	SnapshotVersion string `json:"snapshotVersion"`
	// ISO 8601 UTC；前端不得自算 TTL
	PreviewExpiresAt string                          `json:"previewExpiresAt"`
	Amount           *CheckoutInvoiceAmountResponse  `json:"amount,omitempty"`
	Disclosure       *CheckoutInvoiceDisclosureResp  `json:"disclosure,omitempty"`
	DefaultProfile   *CheckoutInvoiceProfileResponse `json:"defaultProfile,omitempty"`
	IntakeLimit      *CheckoutInvoiceIntakeResponse  `json:"intakeLimit,omitempty"`
}

// CheckoutInvoiceAmountResponse 發票金額拆解。
// ⚠️ 四欄逐字對齊 58A §B01 的明細列；前端**不得**拿 totalActualAmount 充當 invoiceAmount
// （兩者在 Phase 1 相等是業務決策不是恆等式）。
type CheckoutInvoiceAmountResponse struct {
	ProductAmount  string `json:"productAmount"`
	ShippingAmount string `json:"shippingAmount"`
	DiscountAmount string `json:"discountAmount"` // 正數；前端顯示為 −NT$ X
	InvoiceAmount  string `json:"invoiceAmount"`
	TaxCategory    string `json:"taxCategory"`
}

// CheckoutInvoiceDisclosureResp 法定告知。
// ⚠️ required 自 2026-08-31（DEC-20260831-210934）起**只決定要不要顯示**，不再是付款前置條件。
type CheckoutInvoiceDisclosureResp struct {
	Required     bool   `json:"required"`
	TermsVersion string `json:"termsVersion"`
	Title        string `json:"title"`
	Body         string `json:"body"`
}

// CheckoutInvoiceProfileResponse 買家的預設常用發票資料。
// displayValue 是**遮罩後**的字串（DB 存密文，明碼永不出服務邊界）。
type CheckoutInvoiceProfileResponse struct {
	ProfileID    string `json:"profileId"`
	CarrierType  string `json:"carrierType"`
	DisplayValue string `json:"displayValue"`
	BuyerTaxID   string `json:"buyerTaxId,omitempty"`
	BuyerName    string `json:"buyerName,omitempty"`
	IsDefault    bool   `json:"isDefault"`
	Label        string `json:"label,omitempty"`
}

// CheckoutInvoiceIntakeResponse 賣家待審期間的接單額度；blocked=true ⇒ B01 顯示不可下單 banner（9326）。
type CheckoutInvoiceIntakeResponse struct {
	OrderCountUsed  string `json:"orderCountUsed"`
	OrderCountLimit string `json:"orderCountLimit"`
	AmountUsed      string `json:"amountUsed"`
	AmountLimit     string `json:"amountLimit"`
	Blocked         bool   `json:"blocked"`
}

// convertCheckoutInvoicePreview 把 proto 折成 DTO。回 nil ⇒ 上層不設 invoice 欄。
func convertCheckoutInvoicePreview(in *pb.CheckoutInvoicePreview) *CheckoutInvoicePreviewResponse {
	if in == nil {
		return nil
	}
	out := &CheckoutInvoicePreviewResponse{
		InvoiceMode:      in.GetInvoiceMode(),
		SnapshotVersion:  in.GetSnapshotVersion(),
		PreviewExpiresAt: in.GetPreviewExpiresAt(),
	}
	if a := in.GetAmount(); a != nil {
		out.Amount = &CheckoutInvoiceAmountResponse{
			ProductAmount:  a.GetProductAmount(),
			ShippingAmount: a.GetShippingAmount(),
			DiscountAmount: a.GetDiscountAmount(),
			InvoiceAmount:  a.GetInvoiceAmount(),
			TaxCategory:    a.GetTaxCategory(),
		}
	}
	if d := in.GetDisclosure(); d != nil {
		out.Disclosure = &CheckoutInvoiceDisclosureResp{
			Required:     d.GetRequired(),
			TermsVersion: d.GetTermsVersion(),
			Title:        d.GetTitle(),
			Body:         d.GetBody(),
		}
	}
	// profileId 為空代表買家沒有預設 profile（proto 的空 message）⇒ 不帶這一欄。
	if pf := in.GetDefaultProfile(); pf != nil && pf.GetProfileId() != "" {
		out.DefaultProfile = &CheckoutInvoiceProfileResponse{
			ProfileID:    pf.GetProfileId(),
			CarrierType:  pf.GetCarrierType(),
			DisplayValue: pf.GetDisplayValue(),
			BuyerTaxID:   pf.GetBuyerTaxId(),
			BuyerName:    pf.GetBuyerName(),
			IsDefault:    pf.GetIsDefault(),
			Label:        pf.GetLabel(),
		}
	}
	if l := in.GetIntakeLimit(); l != nil {
		out.IntakeLimit = &CheckoutInvoiceIntakeResponse{
			OrderCountUsed:  l.GetOrderCountUsed(),
			OrderCountLimit: l.GetOrderCountLimit(),
			AmountUsed:      l.GetAmountUsed(),
			AmountLimit:     l.GetAmountLimit(),
			Blocked:         l.GetBlocked(),
		}
	}
	return out
}

// AvailablePaymentMethodResponse 付款方式可用性（結帳預覽 + 訂單詳情再次付款共用）。20 設計書 §3.4.1
type AvailablePaymentMethodResponse struct {
	PaymentType      string `json:"paymentType"`                // PaymentMethod value（2001 信用卡 / 2002 Apple Pay / 2003 Google Pay / 2004 ATM）
	Eligible         bool   `json:"eligible"`                   // 金額是否落在該方式合法區間
	IneligibleReason string `json:"ineligibleReason,omitempty"` // PaymentMethodIneligibleReason value（2101 未達下限 / 2102 超過上限）；eligible=true 時空字串
}

// convertAvailablePaymentMethods 將 proto AvailablePaymentMethod 列表轉 Gateway DTO（結帳預覽 + 訂單詳情共用）
func convertAvailablePaymentMethods(items []*pb.AvailablePaymentMethod) []AvailablePaymentMethodResponse {
	if len(items) == 0 {
		return nil
	}
	out := make([]AvailablePaymentMethodResponse, 0, len(items))
	for _, m := range items {
		out = append(out, AvailablePaymentMethodResponse{
			PaymentType:      m.GetPaymentType(),
			Eligible:         m.GetEligible(),
			IneligibleReason: m.GetIneligibleReason(),
		})
	}
	return out
}

// OrderGroupPreviewResponse 訂單分組預覽
type OrderGroupPreviewResponse struct {
	GroupIndex                    string                        `json:"groupIndex"`
	StoreID                       string                        `json:"storeId"`
	SellerName                    string                        `json:"sellerName"`
	StoreCode                     string                        `json:"storeCode"` // 賣家 canonical 短碼；storeId 是內部代理鍵，賣家頁連結走 store_code。與 sellerName 同源
	ShippingMethod                string                        `json:"shippingMethod"`
	DeliveryTemp                  string                        `json:"deliveryTemp"`
	ShippingLabel                 string                        `json:"shippingLabel"`
	ShippingFee                   string                        `json:"shippingFee"`
	ShippingFeePayer              string                        `json:"shippingFeePayer"`
	SubtotalOriginalAmount        string                        `json:"subtotalOriginalAmount"`
	DiscountAmount                string                        `json:"discountAmount"`
	Items                         []CheckoutItemPreviewResponse `json:"items"`
	FreeShippingEligible          bool                          `json:"freeShippingEligible"`          // 最終免運判定（折扣後）
	TentativeFreeShippingEligible bool                          `json:"tentativeFreeShippingEligible"` // 暫定免運判定（折扣前；前端比對 tentative != final 提示「使用此券將失去免運」）
}

// CheckoutItemPreviewResponse 結帳商品預覽
type CheckoutItemPreviewResponse struct {
	SkuID           string `json:"skuId"`
	ProductID       string `json:"productId"`
	SkuName         string `json:"skuName"`
	Quantity        string `json:"quantity"`
	Price           string `json:"price"`                     // 實際成交單價（特價優先）
	ListPrice       string `json:"listPrice,omitempty"`       // 原價快照（給前端 strike-through）
	SalePrice       string `json:"salePrice,omitempty"`       // 特價（無特價=空字串）
	ShareDiscount   string `json:"shareDiscount,omitempty"`   // 該品項分攤的優惠券折扣
	ActualPayAmount string `json:"actualPayAmount,omitempty"` // 該品項最終實付（= subtotal − shareDiscount）
	Subtotal        string `json:"subtotal"`
	StockQty        string `json:"stockQty"`
	Temperature     string `json:"temperature"`
}

// CheckoutConfirmDataResponse 確認結帳回應
type CheckoutConfirmDataResponse struct {
	CheckoutID       string                    `json:"checkoutId"`
	TotalPayAmount   string                    `json:"totalPayAmount"`
	OrderNos         []string                  `json:"orderNos"`
	UnavailableItems []UnavailableItemResponse `json:"unavailableItems,omitempty"` // 取代舊 insufficientItems。row 級不可購買清單（DELISTED / OUT_OF_STOCK / INSUFFICIENT_STOCK / LOOKUP_FAILED）
}

// PriceChangedDetailResponse 金額變動明細（code=4091 時回傳）
type PriceChangedDetailResponse struct {
	OldTotalAmount string `json:"oldTotalAmount"` // Preview 時顯示的實付金額
	NewTotalAmount string `json:"newTotalAmount"` // Confirm 重算後的實付金額
	ChangeReasons  string `json:"changeReasons"`  // 變動原因（逗號分隔，如 "PRODUCT_PRICE,SHIPPING_FEE"）
	NewSubtotal    string `json:"newSubtotal"`    // 重算後商品小計
	NewShippingFee string `json:"newShippingFee"` // 重算後運費
	NewDiscount    string `json:"newDiscount"`    // 重算後折扣
}

// UnavailableItemResponse row 級不可購買明細（Validate / Preview / Confirm 共用）
type UnavailableItemResponse struct {
	SkuID        string `json:"skuId"`
	SkuName      string `json:"skuName"`      // 商品名稱（cart_items.sku_name_snapshot；lookup 失敗時前端可 fallback「商品已下架」）
	Reason       string `json:"reason"`       // CartItemUnavailableReason value（DELISTED / OUT_OF_STOCK / INSUFFICIENT_STOCK / LOOKUP_FAILED）
	RequestedQty string `json:"requestedQty"` // 買家本次勾選量
	AvailableQty string `json:"availableQty"` // 即時可購買量；INSUFFICIENT_STOCK 時 0<available<requested；OUT_OF_STOCK 時 "0"；DELISTED/LOOKUP_FAILED 時空字串
}

// CheckoutValidateRequest 結帳驗證請求
type CheckoutValidateRequest struct {
	Items []CheckoutItemRequest `json:"items" binding:"required"`
}

// CheckoutShippingFeeRequest 運費精算請求
type CheckoutShippingFeeRequest struct {
	ShippingMethod  string                `json:"shippingMethod" binding:"required"`
	DeliveryTemp    string                `json:"deliveryTemp" binding:"required"`
	ReceiverZipCode string                `json:"receiverZipCode"`
	CvsStoreCode    string                `json:"cvsStoreCode"` // 超商取貨門市店號，6 碼數字；僅 shippingMethod=3002 可帶
	StoreID         string                `json:"storeId"`      // Deprecated：cvsStoreCode 的舊名，語意完全相同（**不是**賣家 store_id）
	Items           []CheckoutItemRequest `json:"items" binding:"required"`
}

// CheckoutPreviewRequest 結帳預覽請求
type CheckoutPreviewRequest struct {
	ShippingMethod   string                `json:"shippingMethod" binding:"required"`
	DeliveryTemp     string                `json:"deliveryTemp" binding:"required"`
	ReceiverZipCode  string                `json:"receiverZipCode"`
	CvsStoreCode     string                `json:"cvsStoreCode"`     // 超商取貨門市店號，6 碼數字；僅 shippingMethod=3002 可帶
	StoreID          string                `json:"storeId"`          // Deprecated：cvsStoreCode 的舊名，語意完全相同（**不是**賣家 store_id）
	CouponTemplateID string                `json:"couponTemplateId"` // 使用者選擇的優惠券模板 ID
	Items            []CheckoutItemRequest `json:"items" binding:"required"`
	Consolidated     bool                  `json:"consolidated,omitempty"` // 是否為和單（多賣家代理商代收）。default false=單賣家走 seller mer，true=取 env PAYUNI_AGENT_MER_ID。20 §4.13
}

// CheckoutConfirmRequest 確認結帳請求
type CheckoutConfirmRequest struct {
	CheckoutID       string                `json:"checkoutId" binding:"required"`
	ShippingMethod   string                `json:"shippingMethod" binding:"required"`
	DeliveryTemp     string                `json:"deliveryTemp" binding:"required"`
	ReceiverZipCode  string                `json:"receiverZipCode"`
	RecipientName    string                `json:"recipientName" binding:"required"`
	RecipientPhone   string                `json:"recipientPhone" binding:"required"`
	ShippingAddress  string                `json:"shippingAddress"`
	ShippingNote     string                `json:"shippingNote"`
	OrderRemark      string                `json:"orderRemark"`
	CvsStoreCode     string                `json:"cvsStoreCode"`     // 超商取貨門市店號，6 碼數字；僅 shippingMethod=3002 可帶
	CvsStoreName     string                `json:"cvsStoreName"`     // 超商取貨門市名稱
	StoreID          string                `json:"storeId"`          // Deprecated：cvsStoreCode 的舊名，語意完全相同（**不是**賣家 store_id）
	StoreName        string                `json:"storeName"`        // Deprecated：cvsStoreName 的舊名
	CouponTemplateID string                `json:"couponTemplateId"` // 使用者選擇的優惠券模板 ID
	Items            []CheckoutItemRequest `json:"items" binding:"required"`
	// Invoice 買家發票選擇（**選填**，58 號 §4.1「03 訂單」）。
	// 省略（或送 null）⇒ 後端走 USE_BUYER_DEFAULT，由 invoice-service 在開票時解析
	// 「買家指定的預設 → 無載具發票」。🔴 已上架的 hawkr_buyer App 直接呼叫本端點
	// 且不知道這個欄位存在，任何把它變成必填的改動都會立刻打爛線上結帳。
	Invoice *CheckoutInvoiceRequest `json:"invoice,omitempty"`
}

// CheckoutInvoiceRequest 結帳時採集的買家發票選擇（58 號 §7.1.2 步驟 1）。
//
// 只收「order_invoice_snapshots 存得下」的欄位——該表是零 PII 表，載具明碼
// （手機條碼／自然人憑證）不落 order-service，買家臨櫃輸入的載具值必須先經
// invoice-service 的常用發票資料端點存成一列 preference，結帳時只帶 profileId。
type CheckoutInvoiceRequest struct {
	ProfileID    string `json:"profileId"`    // invoice-service buyer_invoice_preferences.id（數字字串）；"" 或 "0" ＝ 未指定
	BuyerTaxID   string `json:"buyerTaxId"`   // 統編 8 碼數字 ⇒ 三聯式（B2B），需同時給 buyerTitle
	BuyerTitle   string `json:"buyerTitle"`   // 買方抬頭（上限 60 字元），僅 B2B
	DonationCode string `json:"donationCode"` // 捐贈碼 3–7 碼數字
	NotifyEmail  string `json:"notifyEmail"`  // 發票通知信箱（選填）
}

// CheckoutItemRequest 結帳商品項目
type CheckoutItemRequest struct {
	SkuID     string `json:"skuId" binding:"required"`
	Quantity  string `json:"quantity" binding:"required"`
	ProductID string `json:"productId" binding:"required"`
}

// ==================== 轉換函式 ====================

// convertCheckoutInvoice 把選填的發票區塊轉成 proto。
// nil-in / nil-out：proto3 的 message 欄位有 presence，不設就是「買家沒帶」。
func convertCheckoutInvoice(in *CheckoutInvoiceRequest) *pb.CheckoutInvoiceRequest {
	if in == nil {
		return nil
	}
	return &pb.CheckoutInvoiceRequest{
		ProfileId:    in.ProfileID,
		BuyerTaxId:   in.BuyerTaxID,
		BuyerTitle:   in.BuyerTitle,
		DonationCode: in.DonationCode,
		NotifyEmail:  in.NotifyEmail,
	}
}

func convertCheckoutValidateData(d *pb.CheckoutValidateData) *CheckoutValidateDataResponse {
	if d == nil {
		return nil
	}
	r := &CheckoutValidateDataResponse{
		Valid:                    d.Valid,
		DeliveryTempIntersection: d.DeliveryTempIntersection,
	}
	if r.DeliveryTempIntersection == nil {
		r.DeliveryTempIntersection = []string{}
	}
	r.AvailableShippingOptions = make([]AvailableShippingOptionResponse, 0, len(d.AvailableShippingOptions))
	for _, opt := range d.AvailableShippingOptions {
		r.AvailableShippingOptions = append(r.AvailableShippingOptions, AvailableShippingOptionResponse{
			ShippingMethod:        opt.ShippingMethod,
			DeliveryTemp:          opt.DeliveryTemp,
			Label:                 opt.Label,
			Fee:                   opt.Fee,
			FeePayer:              opt.FeePayer,
			FreeShippingThreshold: opt.FreeShippingThreshold,
			FreeShippingEligible:  opt.TentativeFreeShippingEligible,
		})
	}
	r.Errors = make([]CheckoutValidationErrorResponse, 0, len(d.Errors))
	for _, e := range d.Errors {
		r.Errors = append(r.Errors, CheckoutValidationErrorResponse{
			ErrorCode:    e.ErrorCode,
			ErrorMessage: e.ErrorMessage,
		})
	}
	r.UnavailableItems = convertUnavailableItems(d.UnavailableItems)
	return r
}

// convertUnavailableItems 將 proto UnavailableItem 列表轉成 Gateway DTO（Validate / Preview / Confirm 共用）
func convertUnavailableItems(items []*pb.UnavailableItem) []UnavailableItemResponse {
	out := make([]UnavailableItemResponse, 0, len(items))
	for _, it := range items {
		out = append(out, UnavailableItemResponse{
			SkuID:        it.SkuId,
			SkuName:      it.SkuName,
			Reason:       it.Reason,
			RequestedQty: it.RequestedQty,
			AvailableQty: it.AvailableQty,
		})
	}
	return out
}

func convertCheckoutShippingFeeData(d *pb.CheckoutShippingFeeData) *CheckoutShippingFeeDataResponse {
	if d == nil {
		return nil
	}
	return &CheckoutShippingFeeDataResponse{
		ShippingMethod:        d.ShippingMethod,
		DeliveryTemp:          d.DeliveryTemp,
		ShippingRoute:         d.ShippingRoute,
		ShippingFee:           d.ShippingFee,
		FeePayer:              d.FeePayer,
		FreeShippingEligible:  d.TentativeFreeShippingEligible,
		FreeShippingThreshold: d.FreeShippingThreshold,
		OrderSubtotal:         d.OrderSubtotal,
		EstimatedTotal:        d.EstimatedTotal,
	}
}

func convertCheckoutPreviewData(d *pb.CheckoutPreviewData) *CheckoutPreviewDataResponse {
	if d == nil {
		return nil
	}
	r := &CheckoutPreviewDataResponse{
		CheckoutID:          d.CheckoutId,
		TotalOriginalAmount: d.TotalOriginalAmount,
		TotalShippingFee:    d.TotalShippingFee,
		TotalDiscount:       d.TotalDiscount,
		TotalActualAmount:   d.TotalActualAmount,
		PaymentMerID:        d.PaymentMerId,
		// 58 §6.2.4 order ②：發票區。order 拿不到時不設這一欄 ⇒ 這裡也是 nil ⇒
		// JSON 沒有 `invoice` key，前端據此不渲染（而不是渲染一個空的）。
		Invoice: convertCheckoutInvoicePreview(d.GetInvoice()),
	}
	r.OrderGroups = make([]OrderGroupPreviewResponse, 0, len(d.OrderGroups))
	for _, g := range d.OrderGroups {
		group := OrderGroupPreviewResponse{
			GroupIndex:                    g.GroupIndex,
			StoreID:                       g.StoreId,
			SellerName:                    g.SellerName,
			StoreCode:                     g.StoreCode,
			ShippingMethod:                g.ShippingMethod,
			DeliveryTemp:                  g.DeliveryTemp,
			ShippingLabel:                 g.ShippingLabel,
			ShippingFee:                   g.ShippingFee,
			ShippingFeePayer:              g.ShippingFeePayer,
			FreeShippingEligible:          g.FreeShippingEligible,
			TentativeFreeShippingEligible: g.TentativeFreeShippingEligible,
			SubtotalOriginalAmount:        g.SubtotalOriginalAmount,
			DiscountAmount:                g.DiscountAmount,
		}
		group.Items = make([]CheckoutItemPreviewResponse, 0, len(g.Items))
		for _, item := range g.Items {
			group.Items = append(group.Items, CheckoutItemPreviewResponse{
				SkuID:           item.SkuId,
				ProductID:       item.ProductId,
				SkuName:         item.SkuName,
				Quantity:        item.Quantity,
				Price:           item.Price,
				ListPrice:       item.ListPrice,
				SalePrice:       item.SalePrice,
				ShareDiscount:   item.ShareDiscount,
				ActualPayAmount: item.ActualPayAmount,
				Subtotal:        item.Subtotal,
				StockQty:        item.StockQty,
				Temperature:     item.Temperature,
			})
		}
		r.OrderGroups = append(r.OrderGroups, group)
	}
	r.UnavailableItems = convertUnavailableItems(d.UnavailableItems)
	r.AvailablePaymentMethods = convertAvailablePaymentMethods(d.AvailablePaymentMethods)
	return r
}

func convertPriceChangedDetail(d *pb.CheckoutPriceChangedDetail) *PriceChangedDetailResponse {
	if d == nil {
		return nil
	}
	return &PriceChangedDetailResponse{
		OldTotalAmount: d.OldTotalAmount,
		NewTotalAmount: d.NewTotalAmount,
		ChangeReasons:  d.ChangeReasons,
		NewSubtotal:    d.NewSubtotal,
		NewShippingFee: d.NewShippingFee,
		NewDiscount:    d.NewDiscount,
	}
}

func convertCheckoutConfirmData(d *pb.CheckoutConfirmData) *CheckoutConfirmDataResponse {
	if d == nil {
		return nil
	}
	orderNos := d.OrderNos
	if orderNos == nil {
		orderNos = []string{}
	}
	r := &CheckoutConfirmDataResponse{
		CheckoutID:     d.CheckoutId,
		OrderNos:       orderNos,
		TotalPayAmount: d.TotalPayAmount,
	}
	if len(d.UnavailableItems) > 0 {
		r.UnavailableItems = convertUnavailableItems(d.UnavailableItems)
	}
	return r
}

func convertCheckoutItems(items []CheckoutItemRequest) []*pb.CheckoutItem {
	result := make([]*pb.CheckoutItem, 0, len(items))
	for _, item := range items {
		result = append(result, &pb.CheckoutItem{
			SkuId:     item.SkuID,
			Quantity:  item.Quantity,
			ProductId: item.ProductID,
		})
	}
	return result
}

// ==================== Handler ====================

// CheckoutValidate 結帳即時驗證
// @Summary      結帳即時驗證
// @Description  購物車勾選商品後呼叫，驗證溫層相容性與可用配送選項
// @Tags         結帳 (Checkout)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  CheckoutValidateRequest  true  "驗證請求"
// @Success      200  {object}  object{code=int,data=CheckoutValidateDataResponse}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Failure      409  {object}  object{code=int,message=string}
// @Router       /app/checkout/validate [post]
func (h *Holder) CheckoutValidate(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok || accountID == 0 {
		response.Unauthorized(c)
		return
	}
	buyerID, ok := h.resolveBuyerIDFor(c, accountID)
	if !ok {
		return
	}

	var req struct {
		Items []CheckoutItemRequest `json:"items" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	resp, err := h.Order.CheckoutValidate(c.Request.Context(), &pb.CheckoutValidateRequest{
		BuyerId: int64(buyerID),
		Items:   convertCheckoutItems(req.Items),
	})
	if err != nil {
		response.GrpcErr(c, "order", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, convertCheckoutValidateData(resp.Data))
}

// CheckoutShippingFee 結帳運費精算
// @Summary      運費精算
// @Description  選擇配送方式與填寫地址後呼叫，精確計算運費
// @Tags         結帳 (Checkout)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  CheckoutShippingFeeRequest  true  "運費精算請求"
// @Success      200  {object}  object{code=int,data=CheckoutShippingFeeDataResponse}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /app/checkout/shipping_fee [post]
func (h *Holder) CheckoutShippingFee(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok || accountID == 0 {
		response.Unauthorized(c)
		return
	}
	buyerID, ok := h.resolveBuyerIDFor(c, accountID)
	if !ok {
		return
	}

	var req struct {
		ShippingMethod  string                `json:"shippingMethod" binding:"required"`
		DeliveryTemp    string                `json:"deliveryTemp" binding:"required"`
		ReceiverZipCode string                `json:"receiverZipCode"`
		CvsStoreCode    string                `json:"cvsStoreCode"`
		StoreID         string                `json:"storeId"` // Deprecated：cvsStoreCode 的舊名
		Items           []CheckoutItemRequest `json:"items" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	cvsStoreCode, _, ok := resolveCvsStore(c, req.ShippingMethod, req.CvsStoreCode, "", req.StoreID, "")
	if !ok {
		return
	}

	resp, err := h.Order.CheckoutShippingFee(c.Request.Context(), &pb.CheckoutShippingFeeRequest{
		BuyerId:         int64(buyerID),
		Items:           convertCheckoutItems(req.Items),
		ShippingMethod:  req.ShippingMethod,
		DeliveryTemp:    req.DeliveryTemp,
		ReceiverZipCode: req.ReceiverZipCode,
		CvsStoreId:      cvsStoreCode,
	})
	if err != nil {
		response.GrpcErr(c, "order", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, convertCheckoutShippingFeeData(resp.Data))
}

// CheckoutPreview 結帳預覽
// @Summary      結帳預覽
// @Description  完整驗證 + 精算總額，生成 checkout_id，不建單
// @Tags         結帳 (Checkout)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  CheckoutPreviewRequest  true  "預覽請求"
// @Success      200  {object}  object{code=int,data=CheckoutPreviewDataResponse}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Failure      409  {object}  object{code=int,message=string}
// @Router       /app/checkout/preview [post]
func (h *Holder) CheckoutPreview(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok || accountID == 0 {
		response.Unauthorized(c)
		return
	}
	// ② 買家身分 = 現任 buyer_id（resolver）。
	buyerID, ok := h.resolveBuyerIDFor(c, accountID)
	if !ok {
		return
	}

	var req struct {
		ShippingMethod   string                `json:"shippingMethod" binding:"required"`
		DeliveryTemp     string                `json:"deliveryTemp" binding:"required"`
		ReceiverZipCode  string                `json:"receiverZipCode"`
		CvsStoreCode     string                `json:"cvsStoreCode"`
		StoreID          string                `json:"storeId"` // Deprecated：cvsStoreCode 的舊名
		CouponTemplateID string                `json:"couponTemplateId"`
		Items            []CheckoutItemRequest `json:"items" binding:"required"`
		Consolidated     bool                  `json:"consolidated,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	cvsStoreCode, _, ok := resolveCvsStore(c, req.ShippingMethod, req.CvsStoreCode, "", req.StoreID, "")
	if !ok {
		return
	}

	resp, err := h.Order.CheckoutPreview(c.Request.Context(), &pb.CheckoutPreviewRequest{
		BuyerId:          int64(buyerID),
		Items:            convertCheckoutItems(req.Items),
		ShippingMethod:   req.ShippingMethod,
		DeliveryTemp:     req.DeliveryTemp,
		ReceiverZipCode:  req.ReceiverZipCode,
		CvsStoreId:       cvsStoreCode,
		CouponTemplateId: req.CouponTemplateID,
		Consolidated:     req.Consolidated,
	})
	if err != nil {
		response.GrpcErr(c, "order", err)
		return
	}

	// issue #1127：4001 商品不可購買時，envelope code=4001 + data.unavailableItems[] 透傳給前端 dialog。
	// 走原始 c.JSON 而非 response.GrpcResult（後者對 code≠0 會丟 data）。
	if resp.Code != 0 && resp.Data != nil && len(resp.Data.UnavailableItems) > 0 {
		response.Biz(c, resp.Code, resp.Message, gin.H{
			"unavailableItems": convertUnavailableItems(resp.Data.UnavailableItems),
		})
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, convertCheckoutPreviewData(resp.Data))
}

// CheckoutConfirm 確認結帳（建單）
// @Summary      確認結帳
// @Description  重新驗證 + 建立訂單，回傳訂單編號與付款金額。業務錯誤碼：
// @Description  - 4006 CHECKOUT_EXPIRED：checkoutId 已使用或過期（同 checkoutId 重送會觸發）
// @Description  - 4091 CHECKOUT_PRICE_CHANGED：Preview 與 Confirm 之間金額變動，需重新 preview
// @Description  - 4092 ORDER_ALREADY_CREATED：buyer 已有未付款訂單包含相同 sku，message 含 orderNo，前端應導向付款頁
// @Description  - 6005 CHECKOUT_LOCK_FAILED：購物車鎖定失敗（真併發或殘留 LOCKED，罕見）
// @Description  - 8026 ORDER_AMOUNT_EXCEEDS_LIMIT：訂單實付金額超出系統支援上限（199,999 元），請分批下單
// @Description  - 9305 INVOICE_CARRIER_INVALID：invoice 區塊的統編／捐贈碼／抬頭／通知信箱格式不正確
// @Description  - 9306 INVOICE_TARGET_CONFLICT：invoice 區塊同時給了統編、常用資料與捐贈碼中的兩項以上
// @Tags         結帳 (Checkout)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  CheckoutConfirmRequest  true  "確認結帳請求"
// @Success      200  {object}  object{code=int,data=CheckoutConfirmDataResponse}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Failure      409  {object}  object{code=int,message=string}  "業務碼 4092 ORDER_ALREADY_CREATED / 6005 CHECKOUT_LOCK_FAILED / 8026 ORDER_AMOUNT_EXCEEDS_LIMIT 皆映射 HTTP 409"
// @Router       /app/checkout/confirm [post]
func (h *Holder) CheckoutConfirm(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok || accountID == 0 {
		response.Unauthorized(c)
		return
	}
	buyerID, ok := h.resolveBuyerIDFor(c, accountID)
	if !ok {
		return
	}

	var req struct {
		CheckoutID       string                `json:"checkoutId" binding:"required"`
		ShippingMethod   string                `json:"shippingMethod" binding:"required"`
		DeliveryTemp     string                `json:"deliveryTemp" binding:"required"`
		ReceiverZipCode  string                `json:"receiverZipCode"`
		RecipientName    string                `json:"recipientName" binding:"required"`
		RecipientPhone   string                `json:"recipientPhone" binding:"required"`
		ShippingAddress  string                `json:"shippingAddress"`
		ShippingNote     string                `json:"shippingNote"`
		OrderRemark      string                `json:"orderRemark"`
		CvsStoreCode     string                `json:"cvsStoreCode"`
		CvsStoreName     string                `json:"cvsStoreName"`
		StoreID          string                `json:"storeId"`   // Deprecated：cvsStoreCode 的舊名
		StoreName        string                `json:"storeName"` // Deprecated：cvsStoreName 的舊名
		CouponTemplateID string                `json:"couponTemplateId"`
		Items            []CheckoutItemRequest `json:"items" binding:"required"`
		// 選填；nil ⇒ 不設 proto 的 invoice 欄位 ⇒ order-service 走 USE_BUYER_DEFAULT
		Invoice *CheckoutInvoiceRequest `json:"invoice,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	cvsStoreCode, cvsStoreName, ok := resolveCvsStore(c, req.ShippingMethod, req.CvsStoreCode, req.CvsStoreName, req.StoreID, req.StoreName)
	if !ok {
		return
	}

	resp, err := h.Order.CheckoutConfirm(c.Request.Context(), &pb.CheckoutConfirmRequest{
		BuyerId:          int64(buyerID),
		CheckoutId:       req.CheckoutID,
		Items:            convertCheckoutItems(req.Items),
		ShippingMethod:   req.ShippingMethod,
		DeliveryTemp:     req.DeliveryTemp,
		ReceiverZipCode:  req.ReceiverZipCode,
		RecipientName:    req.RecipientName,
		RecipientPhone:   req.RecipientPhone,
		ShippingAddress:  req.ShippingAddress,
		ShippingNote:     req.ShippingNote,
		OrderRemark:      req.OrderRemark,
		CvsStoreId:       cvsStoreCode,
		CvsStoreName:     cvsStoreName,
		CouponTemplateId: req.CouponTemplateID,
		// 🔴 nil 時**不設**這個欄位（proto3 message presence）：order-service 用
		// hasInvoice() 區分「沒帶」與「帶了空值」，這裡若一律 new 一個空 message，
		// 兩者就分不開了——而「沒帶」正是線上 App 的行為。
		Invoice: convertCheckoutInvoice(req.Invoice),
	})
	if err != nil {
		response.GrpcErr(c, "order", err)
		return
	}

	// 金額變動時回傳 code=4091 + data.priceChanged 明細
	if resp.PriceChanged != nil && resp.PriceChanged.NewTotalAmount != "" {
		response.Biz(c, resp.Code, resp.Message, gin.H{
			"priceChanged": convertPriceChangedDetail(resp.PriceChanged),
		})
		return
	}

	// issue #1127：4001 商品不可購買時，envelope code=4001 + data.unavailableItems[] 透傳給前端 dialog。
	if resp.Code != 0 && resp.Data != nil && len(resp.Data.UnavailableItems) > 0 {
		response.Biz(c, resp.Code, resp.Message, gin.H{
			"unavailableItems": convertUnavailableItems(resp.Data.UnavailableItems),
		})
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, convertCheckoutConfirmData(resp.Data))
}
