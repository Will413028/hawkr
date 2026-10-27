package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Will413028/hawkr/gateway/internal/authz"
	"github.com/Will413028/hawkr/gateway/internal/buyerid"
	"github.com/Will413028/hawkr/gateway/internal/codemeta"
	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/Will413028/hawkr/go-pkg/identity"
	orderpb "github.com/Will413028/hawkr/go-pkg/proto/order"
	pb "github.com/Will413028/hawkr/go-pkg/proto/payment"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

func (h *Holder) resolvePaymentBuyerID(c *gin.Context, accountID int64) (int64, bool) {
	if accountID == 0 {
		return 0, true
	}
	resolved, err := h.BuyerID.CurrentBuyerID(c.Request.Context(), identity.AccountID(accountID))
	if err != nil {
		if errors.Is(err, buyerid.ErrNoCurrentBuyer) {
			return accountID, true
		}
		response.GrpcErr(c, "buyer", err)
		return 0, false
	}
	return int64(resolved), true
}

// ==================== DTO ====================

// PaymentResultResponse 建立支付回應
type PaymentResultResponse struct {
	TradeNo           string               `json:"tradeNo,omitempty"`
	OrderID           string               `json:"orderId,omitempty"`
	PaymentType       string               `json:"paymentType,omitempty"` // 付款方式（PaymentMethod value；合法值以 CodeMeta 業務類型碼.md 為準，2001-2004）
	Amount            string               `json:"amount,omitempty"`
	Status            string               `json:"status,omitempty"` // 交易狀態 dbValue（0=待付款, 1=已付款, 2=付款失敗, 3=已逾期, 4=狀態衝突, 5=已關閉）
	UppFormData       *UppFormDataResponse `json:"uppFormData,omitempty"`
	AtmBankCode       string               `json:"atmBankCode,omitempty"`
	AtmVirtualAccount string               `json:"atmVirtualAccount,omitempty"`
	AtmExpireDate     string               `json:"atmExpireDate,omitempty"`
	ExpireAt          string               `json:"expireAt,omitempty"`
	RedirectUrl       string               `json:"redirectUrl,omitempty"` // 完整的 WebView 開啟 URL（由 Payment Service 組裝）
}

// PaymentDetailResponse 支付詳情回應
type PaymentDetailResponse struct {
	AtmVirtualAccount string `json:"atmVirtualAccount,omitempty"`
	ProviderMessage   string `json:"providerMessage,omitempty"`
	MerID             string `json:"merId,omitempty"`
	ProviderStatus    string `json:"providerStatus,omitempty"`
	UpdatedAt         string `json:"updatedAt,omitempty"`
	Amount            string `json:"amount,omitempty"`
	ProductName       string `json:"productName,omitempty"`
	CreatedAt         string `json:"createdAt,omitempty"`
	OrderID           string `json:"orderId,omitempty"`
	ProviderTradeNo   string `json:"providerTradeNo,omitempty"`
	BuyerID           string `json:"buyerId,omitempty"`
	AtmBankCode       string `json:"atmBankCode,omitempty"`
	TradeNo           string `json:"tradeNo,omitempty"`
	AtmExpireDate     string `json:"atmExpireDate,omitempty"`
	PayTime           string `json:"payTime,omitempty"`
	ClosedAt          string `json:"closedAt,omitempty"`
	ExpireAt          string `json:"expireAt,omitempty"`
	Status            string `json:"status,omitempty"`      // 交易狀態 dbValue（0=待付款, 1=已付款, 2=付款失敗, 3=已逾期, 4=狀態衝突, 5=已關閉, 6=因切換付款方式而關閉）
	PaymentType       string `json:"paymentType,omitempty"` // 支付方式 dbValue（2001=信用卡, 2002=ApplePay, 2003=GooglePay, 2004=ATM, 2900=零元）
	// §2A.6 重複付款顯示
	DuplicateSequence string `json:"duplicateSequence,omitempty"` // 重複付款序號（"1", "2", "3"...），主 row 為空字串
	DisplayTradeNo    string `json:"displayTradeNo,omitempty"`    // 前端顯示用：副 row 為 "{tradeNo}-DUP{seq}"，主 row 為原 tradeNo
	IsDuplicate       bool   `json:"isDuplicate"`                 // 是否為重複付款的副 row（true=已被退款）
}

// UppFormDataResponse PAYUNi UPP 表單資料
type UppFormDataResponse struct {
	ActionURL   string `json:"actionUrl,omitempty"`
	MerID       string `json:"merId,omitempty"`
	Version     string `json:"version,omitempty"`
	EncryptInfo string `json:"encryptInfo,omitempty"`
	HashInfo    string `json:"hashInfo,omitempty"`
	IsPlatform  string `json:"isPlatform,omitempty"`
}

// RefundDetailResponse 退款詳情回應
type RefundDetailResponse struct {
	RefundNo            string `json:"refundNo,omitempty"`
	TradeNo             string `json:"tradeNo,omitempty"`
	OrderID             string `json:"orderId,omitempty"`
	Amount              string `json:"amount,omitempty"`
	Reason              string `json:"reason,omitempty"`
	ProviderRefundNo    string `json:"providerRefundNo,omitempty"`
	ProviderStatus      string `json:"providerStatus,omitempty"`
	ProviderMessage     string `json:"providerMessage,omitempty"`
	EstimatedRefundDate string `json:"estimatedRefundDate,omitempty"` // ATM 預計扣款日 YYYY-MM-DD（僅 ATM sync success 有值）
	BankDate            string `json:"bankDate,omitempty"`            // ATM 實際到帳日 YYYY-MM-DD（僅 ATM async RefundStatus=1 有值）
	CompletedAt         string `json:"completedAt,omitempty"`
	CreatedAt           string `json:"createdAt,omitempty"`
	Status              string `json:"status,omitempty"` // 退款狀態 dbValue
}

// ==================== 轉換函式 ====================

func convertPaymentResult(r *pb.PaymentResult) *PaymentResultResponse {
	if r == nil {
		return nil
	}
	resp := &PaymentResultResponse{
		TradeNo:           r.TradeNo,
		OrderID:           r.OrderId,
		PaymentType:       r.PaymentType,
		Amount:            r.Amount,
		Status:            r.Status,
		AtmBankCode:       r.AtmBankCode,
		AtmVirtualAccount: r.AtmVirtualAccount,
		AtmExpireDate:     r.AtmExpireDate,
		ExpireAt:          r.ExpireAt,
		RedirectUrl:       r.RedirectUrl,
	}
	if r.UppFormData != nil {
		resp.UppFormData = convertUppFormData(r.UppFormData)
	}
	return resp
}

func convertPaymentDetail(d *pb.PaymentDetail) *PaymentDetailResponse {
	if d == nil {
		return nil
	}
	// §2A.6 displayTradeNo：副 row 加 -DUP{seq} 後綴給前端區分
	displayTradeNo := d.TradeNo
	if d.IsDuplicate && d.DuplicateSequence != "" {
		displayTradeNo = d.TradeNo + "-DUP" + d.DuplicateSequence
	}
	return &PaymentDetailResponse{
		TradeNo:           d.TradeNo,
		OrderID:           d.OrderId,
		MerID:             d.MerId,
		BuyerID:           d.BuyerId,
		PaymentType:       d.PaymentType,
		Amount:            d.Amount,
		ProductName:       d.ProductName,
		Status:            d.Status,
		ProviderTradeNo:   d.ProviderTradeNo,
		ProviderStatus:    d.ProviderStatus,
		ProviderMessage:   d.ProviderMessage,
		AtmBankCode:       d.AtmBankCode,
		AtmVirtualAccount: d.AtmVirtualAccount,
		AtmExpireDate:     d.AtmExpireDate,
		PayTime:           d.PayTime,
		ClosedAt:          d.ClosedAt,
		ExpireAt:          d.ExpireAt,
		CreatedAt:         d.CreatedAt,
		UpdatedAt:         d.UpdatedAt,
		IsDuplicate:       d.IsDuplicate,
		DuplicateSequence: d.DuplicateSequence,
		DisplayTradeNo:    displayTradeNo,
	}
}

func convertUppFormData(u *pb.UppFormData) *UppFormDataResponse {
	if u == nil {
		return nil
	}
	return &UppFormDataResponse{
		ActionURL:   u.ActionUrl,
		MerID:       u.MerId,
		Version:     u.Version,
		EncryptInfo: u.EncryptInfo,
		HashInfo:    u.HashInfo,
		IsPlatform:  u.IsPlatform,
	}
}

func convertRefundDetail(d *pb.RefundDetail) *RefundDetailResponse {
	if d == nil {
		return nil
	}
	return &RefundDetailResponse{
		RefundNo:            d.RefundNo,
		TradeNo:             d.TradeNo,
		OrderID:             d.OrderId,
		Amount:              d.Amount,
		Status:              d.Status,
		Reason:              d.Reason,
		ProviderRefundNo:    d.ProviderRefundNo,
		ProviderStatus:      d.ProviderStatus,
		ProviderMessage:     d.ProviderMessage,
		EstimatedRefundDate: d.GetEstimatedRefundDate(),
		BankDate:            d.GetBankDate(),
		CompletedAt:         d.CompletedAt,
		CreatedAt:           d.CreatedAt,
	}
}

func convertRefundDetails(refunds []*pb.RefundDetail) []RefundDetailResponse {
	result := make([]RefundDetailResponse, 0, len(refunds))
	for _, r := range refunds {
		if r != nil {
			result = append(result, *convertRefundDetail(r))
		}
	}
	return result
}

// ==================== 請求結構 ====================

// CreatePaymentRequest 建立支付請求
type CreatePaymentRequest struct {
	OrderNo        string `json:"orderNo" binding:"required"`
	PaymentType    string `json:"paymentType" binding:"required"`
	ExpireDays     string `json:"expireDays"`
	ApplePayToken  string `json:"applePayToken"`
	GooglePayToken string `json:"googlePayToken"`
	BankType       string `json:"bankType"`
	UserEmail      string `json:"userEmail"`
	ClientType     string `json:"clientType"`
}

// snapshotPaymentInfoToOrder 快照付款資訊到訂單（best-effort）
func (h *Holder) snapshotPaymentInfoToOrder(c *gin.Context, data *pb.PaymentResult, orderNo string) {
	_, _ = h.Order.SnapshotPaymentInfo(c.Request.Context(), &orderpb.SnapshotPaymentInfoRequest{
		OrderNo:           orderNo,
		TradeNo:           data.TradeNo,
		PaymentMethod:     data.PaymentType,
		AtmBankCode:       data.AtmBankCode,
		AtmVirtualAccount: data.AtmVirtualAccount,
		AtmExpireDate:     data.AtmExpireDate,
	})
}

// CreatePayment 建立支付交易
// @Summary      建立支付交易
// @Description  建立新的支付交易，支援信用卡、ATM、Apple Pay、Google Pay 等付款方式。
// @Description
// @Description  **付款方式金額範圍（PayUni 硬性限制）**：
// @Description  - 信用卡 / Apple Pay / Google Pay：1 ~ 199,999 元
// @Description  - ATM 轉帳：15 ~ 49,999 元
// @Description  - 零元訂單（amount=0）：不走 PayUni，直接由 payment-service 標記為 PAID（不需呼叫本 API）
// @Description
// @Description  超範圍會回 `8021 PAYMENT_METHOD_AMOUNT_OUT_OF_RANGE`；訂單建立時若超過 199,999 已於 order-service 早期攔截，回 `8026 ORDER_AMOUNT_EXCEEDS_LIMIT`。
// @Tags         支付 (Payment)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  CreatePaymentRequest  true  "建立支付請求"
// @Success      200  {object}  object{code=int,data=PaymentResultResponse}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Failure      409  {object}  object{code=int,message=string} "8021 PAYMENT_METHOD_AMOUNT_OUT_OF_RANGE 付款方式金額超出範圍"
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /app/payments [post]
func (h *Holder) CreatePayment(c *gin.Context) {
	var req CreatePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	accountId, _ := c.Get("accountId")
	var accountIdInt64 int64
	if accountId != nil {
		switch v := accountId.(type) {
		case float64:
			accountIdInt64 = int64(v)
		case int64:
			accountIdInt64 = v
		case int:
			accountIdInt64 = int64(v)
		case string:
			accountIdInt64, _ = strconv.ParseInt(v, 10, 64)
		}
	}

	buyerID, ok := h.resolvePaymentBuyerID(c, accountIdInt64)
	if !ok {
		return
	}

	resp, err := h.Payment.CreatePayment(c.Request.Context(), &pb.CreatePaymentRequest{
		OrderNo:        req.OrderNo,
		BuyerId:        buyerID,
		PaymentType:    req.PaymentType,
		ExpireDays:     req.ExpireDays,
		ApplePayToken:  req.ApplePayToken,
		GooglePayToken: req.GooglePayToken,
		BankType:       req.BankType,
		UserEmail:      req.UserEmail,
		ClientType:     req.ClientType,
		AppType:        codemeta.AppTypeFromAccountType(getAccountType(c)),
	})

	if err != nil {
		response.GrpcErr(c, "payment", err)
		return
	}

	// 快照付款資訊到訂單（best-effort，失敗不影響付款流程）
	if resp.Code == 0 && resp.Data != nil {
		h.snapshotPaymentInfoToOrder(c, resp.Data, req.OrderNo)
	}

	// 透傳微服務回傳值（redirectUrl 由 Payment Service 組裝）
	converted := convertPaymentResult(resp.Data)
	if converted != nil && converted.RedirectUrl != "" {
		// 有 redirectUrl 時加入回應
		response.GrpcResult(c, resp.Code, resp.Message, gin.H{
			"tradeNo":     converted.TradeNo,
			"orderId":     converted.OrderID,
			"paymentType": converted.PaymentType,
			"amount":      converted.Amount,
			"status":      converted.Status,
			"uppFormData": converted.UppFormData,
			"redirectUrl": converted.RedirectUrl,
		})
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, converted)
}

// GetPayment 查詢支付詳情
// @Summary      查詢支付詳情
// @Description  根據交易編號查詢支付交易的詳細資訊
// @Tags         支付 (Payment)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        trade_no  path  string  true  "交易編號"
// @Success      200  {object}  object{code=int,data=PaymentDetailResponse}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /app/payments/{trade_no} [get]
func (h *Holder) GetPayment(c *gin.Context) {
	tradeNo := c.Param("trade_no")
	if tradeNo == "" {
		response.BadRequest(c, "trade_no is required")
		return
	}

	accountID, _ := getAccountID(c)
	buyerID, ok := h.resolvePaymentBuyerID(c, accountID)
	if !ok {
		return
	}

	resp, err := h.Payment.GetPayment(c.Request.Context(), buyerID, tradeNo)
	if err != nil {
		response.GrpcErr(c, "payment", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, convertPaymentDetail(resp.Data))
}

// GetPaymentHistory §2A.3.6 查詢訂單付款歷程
// @Summary      查詢訂單付款歷程
// @Description  根據訂單編號查詢所有付款嘗試記錄，含切換而關閉的舊 row（CLOSED_BY_SWITCH）與重複付款的副 row（is_duplicate=true）。依 created_at DESC 排序。預設為買家視角（僅本人訂單）；view=seller 切換為賣家視角，以現任 store_id 驗店家歸屬並遮蔽買家 PII（buyerId/atmVirtualAccount/providerTradeNo）
// @Tags         支付 (Payment)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        order_id  path   string  true   "訂單編號（如 O382716459）"
// @Param        view      query  string  false  "視角：空值=買家（僅本人訂單）；seller=賣家（僅自己店家的訂單，遮蔽買家 PII）"
// @Success      200  {object}  object{code=int,data=object{records=[]PaymentDetailResponse}}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /app/payments/orders/{order_id}/history [get]
func (h *Holder) GetPaymentHistory(c *gin.Context) {
	orderId := c.Param("order_id")
	if orderId == "" {
		response.BadRequest(c, "order_id is required")
		return
	}

	// #2501 賣家視角（?view=seller）：由 server 解出的現任 store_id 走 store-scoped RPC，
	// payment-service 以 canonical order 反解 store 歸屬並遮蔽買家 PII。一般路徑收斂為 buyer-only。
	if authz.IsSellerView(c.Query("view")) {
		storeID, ok := h.resolveStoreID(c)
		if !ok {
			return
		}
		resp, err := h.Payment.GetSellerPaymentHistory(c.Request.Context(), int64(storeID), orderId)
		if err != nil {
			response.GrpcErr(c, "payment", err)
			return
		}
		records := make([]*PaymentDetailResponse, 0, len(resp.Records))
		for _, r := range resp.Records {
			records = append(records, convertPaymentDetail(r))
		}
		response.GrpcResult(c, resp.Code, resp.Message, gin.H{"records": records})
		return
	}

	accountID, _ := getAccountID(c)
	buyerID, ok := h.resolvePaymentBuyerID(c, accountID)
	if !ok {
		return
	}

	resp, err := h.Payment.GetPaymentHistory(c.Request.Context(), buyerID, orderId)
	if err != nil {
		response.GrpcErr(c, "payment", err)
		return
	}

	records := make([]*PaymentDetailResponse, 0, len(resp.Records))
	for _, r := range resp.Records {
		records = append(records, convertPaymentDetail(r))
	}
	response.GrpcResult(c, resp.Code, resp.Message, gin.H{"records": records})
}

// SyncPaymentStatus 同步支付狀態
// @Summary      同步支付狀態
// @Description  手動同步指定交易的支付狀態，從 PAYUNi 取得最新狀態
// @Tags         支付 (Payment)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        trade_no  path  string  true  "交易編號"
// @Success      200  {object}  object{code=int,data=PaymentDetailResponse}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /app/payments/{trade_no}/status_syncs [post]
func (h *Holder) SyncPaymentStatus(c *gin.Context) {
	tradeNo := c.Param("trade_no")
	if tradeNo == "" {
		response.BadRequest(c, "trade_no is required")
		return
	}

	accountID, _ := getAccountID(c)
	buyerID, ok := h.resolvePaymentBuyerID(c, accountID)
	if !ok {
		return
	}

	resp, err := h.Payment.SyncPaymentStatus(c.Request.Context(), buyerID, tradeNo)
	if err != nil {
		response.GrpcErr(c, "payment", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, convertPaymentDetail(resp.Data))
}

// ClosePayment 關閉交易
// @Summary      關閉交易
// @Description  關閉指定的支付交易，交易關閉後無法再進行付款
// @Tags         支付 (Payment)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        trade_no  path  string  true  "交易編號"
// @Success      200  {object}  object{code=int,message=string}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Failure      409  {object}  object{code=int,message=string}
// @Router       /app/payments/{trade_no} [patch]
func (h *Holder) ClosePayment(c *gin.Context) {
	tradeNo := c.Param("trade_no")
	if tradeNo == "" {
		response.BadRequest(c, "trade_no is required")
		return
	}

	accountID, _ := getAccountID(c)
	buyerID, ok := h.resolvePaymentBuyerID(c, accountID)
	if !ok {
		return
	}

	resp, err := h.Payment.ClosePayment(c.Request.Context(), buyerID, tradeNo)
	if err != nil {
		response.GrpcErr(c, "payment", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, nil)
}

// GetRefund 查詢退款詳情
// @Summary      查詢退款詳情
// @Description  根據退款編號查詢退款的詳細資訊
// @Tags         支付 (Payment)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        refund_no  path  string  true  "退款編號"
// @Success      200  {object}  object{code=int,data=RefundDetailResponse}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /app/payments/refunds/{refund_no} [get]
func (h *Holder) GetRefund(c *gin.Context) {
	refundNo := c.Param("refund_no")
	if refundNo == "" {
		response.BadRequest(c, "refund_no is required")
		return
	}

	accountID, _ := getAccountID(c)
	buyerID, ok := h.resolvePaymentBuyerID(c, accountID)
	if !ok {
		return
	}

	resp, err := h.Payment.GetRefund(c.Request.Context(), buyerID, refundNo)
	if err != nil {
		response.GrpcErr(c, "payment", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, convertRefundDetail(resp.Data))
}

// GetRefundsByTradeNo 查詢交易的所有退款
// @Summary      查詢交易的所有退款
// @Description  根據交易編號查詢該筆交易下的所有退款紀錄
// @Tags         支付 (Payment)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        trade_no  path  string  true  "交易編號"
// @Success      200  {object}  object{code=int,data=[]RefundDetailResponse}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /app/payments/{trade_no}/refunds [get]
func (h *Holder) GetRefundsByTradeNo(c *gin.Context) {
	tradeNo := c.Param("trade_no")
	if tradeNo == "" {
		response.BadRequest(c, "trade_no is required")
		return
	}

	accountID, _ := getAccountID(c)
	buyerID, ok := h.resolvePaymentBuyerID(c, accountID)
	if !ok {
		return
	}

	resp, err := h.Payment.GetRefundsByTradeNo(c.Request.Context(), buyerID, tradeNo)
	if err != nil {
		response.GrpcErr(c, "payment", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, convertRefundDetails(resp.Refunds))
}

// RedirectToPayUni 重導向到 PAYUNi 付款頁
// GET /payment/redirect?id={opaqueId}
// 此端點供 APP WebView 直接開啟。#1394：改讀 opaque id → gRPC 取回後端 canonical
// 表單 → html/template 安全 render（舊 base64 `?data=` 反射 XSS 路徑已物理刪除）。
// @Summary      重導向到 PAYUNi 付款頁
// @Description  以後端產生的一次性 opaque id 取回 canonical 表單資料並自動 Form POST 到 PAYUNi，供 APP WebView 直接開啟使用
// @Tags         支付 (Payment)
// @Produce      html
// @Param        id  query  string  true  "後端產生的一次性 redirect id"
// @Success      200  {string}  string  "自動提交到 PAYUNi 的 HTML 頁面"
// @Failure      400  {string}  string  "缺少參數"
// @Failure      404  {string}  string  "連結已失效或不存在"
// @Failure      503  {string}  string  "服務不可用"
// @Router       /payment/redirect [get]
func (h *Holder) RedirectToPayUni(c *gin.Context) {
	redirectID := c.Query("id")
	if redirectID == "" {
		renderPayUniRedirectError(c, http.StatusBadRequest, "缺少必要參數")
		return
	}

	resp, err := h.Payment.GetPaymentRedirectForm(c.Request.Context(), redirectID)
	if err != nil {
		if grpcstatus.Code(err) == codes.NotFound {
			renderPayUniRedirectError(c, http.StatusNotFound, "付款連結已失效或不存在，請重新發起付款")
			return
		}
		renderPayUniRedirectError(c, http.StatusServiceUnavailable, "付款服務暫時無法使用")
		return
	}

	// is_platform 在 proto 為 bool（非數字語意）；PayUni Form 欄位為字串（代理商模式固定 "1"）。
	isPlatform := "0"
	if resp.GetIsPlatform() {
		isPlatform = "1"
	}
	renderPayUniRedirect(c, payUniRedirectForm{
		Title:       "正在跳轉到付款頁",
		FormID:      "payuniForm",
		ActionURL:   resp.GetActionUrl(),
		MerID:       resp.GetMerId(),
		Version:     resp.GetVersion(),
		EncryptInfo: resp.GetEncryptInfo(),
		HashInfo:    resp.GetHashInfo(),
		IsPlatForm:  isPlatform,
	})
}

// PaymentReturn 付款結果回調頁面
// PayUni 信用卡付款完成後，將使用者瀏覽器導回此頁面
// Gateway 呼叫 Payment Service 取得完整 HTML（含狀態映射、Deep Link），直接透傳
// @Summary      付款結果回調頁面
// @Description  PayUni 付款完成後的回調頁面，顯示付款結果（成功/失敗/處理中），自動透過 deep link 導回 App
// @Tags         支付 (Payment)
// @Produce      html
// @Param        tradeNo  query  string  true  "交易編號"
// @Success      200  {string}  string  "付款結果 HTML 頁面"
// @Failure      400  {string}  string  "參數錯誤"
// @Failure      503  {string}  string  "服務不可用"
// @Router       /payment/return [get]
func (h *Holder) PaymentReturn(c *gin.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")

	tradeNo := c.Query("tradeNo")
	if tradeNo == "" {
		c.String(http.StatusBadRequest, "缺少交易編號")
		return
	}

	// 呼叫 Payment Service 取得完整 HTML（含狀態映射 + Deep Link）
	resp, err := h.Payment.GetPaymentReturnPage(c.Request.Context(), &pb.GetPaymentReturnPageRequest{
		TradeNo: tradeNo,
	})
	if err != nil {
		c.String(http.StatusServiceUnavailable, "查詢付款結果失敗")
		return
	}

	// 透傳 Payment Service 回傳的完整 HTML 頁面
	c.String(http.StatusOK, resp.GetHtmlContent())
}

// PaymentReturnPost 信用卡付款完成回調（PayUni Form POST）
// PayUni 付款完成後以 Form POST 將 EncryptInfo + HashInfo 導回此端點
// Gateway 提取加密參數 → Payment Service gRPC 解密 + 更新 DB → 302 redirect Deep Link
// @Summary      信用卡付款完成回調
// @Description  PayUni UPP 付款完成後的 Form POST 回調，解密交易結果、更新 DB、302 redirect 到 Deep Link 導回 App
// @Tags         支付 (Payment)
// @Accept       x-www-form-urlencoded
// @Param        EncryptInfo  formData  string  true  "PayUni 加密資料"
// @Param        HashInfo     formData  string  true  "PayUni 簽章"
// @Success      302  "Redirect to Deep Link"
// @Failure      400  {string}  string  "參數錯誤"
// @Failure      503  {string}  string  "服務不可用"
// @Router       /payment/return [post]
func (h *Holder) PaymentReturnPost(c *gin.Context) {
	encryptInfo := c.PostForm("EncryptInfo")
	hashInfo := c.PostForm("HashInfo")

	if encryptInfo == "" || hashInfo == "" {
		c.String(http.StatusBadRequest, "缺少加密參數")
		return
	}

	resp, err := h.Payment.HandlePaymentPageReturn(c.Request.Context(), &pb.HandlePaymentPageReturnRequest{
		EncryptInfo: encryptInfo,
		HashInfo:    hashInfo,
	})
	if err != nil {
		c.String(http.StatusServiceUnavailable, "處理付款結果失敗")
		return
	}

	// 302 redirect 到 Payment Service 組裝的 Deep Link
	c.Redirect(http.StatusFound, resp.GetRedirectUrl())
}
