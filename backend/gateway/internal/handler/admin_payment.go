package handler

import (
	"github.com/Will413028/hawkr/gateway/internal/response"
	pb "github.com/Will413028/hawkr/go-pkg/proto/payment"
	"github.com/gin-gonic/gin"
)

// ==================== Admin 付款異常查詢（§29）====================

// AdminPaymentTransactionSummaryResp 付款異常列表單筆
type AdminPaymentTransactionSummaryResp struct {
	TradeNo         string `json:"tradeNo"`
	OrderNo         string `json:"orderNo"`
	BuyerId         string `json:"buyerId"`
	MerId           string `json:"merId"`
	Amount          string `json:"amount"`      // 新台幣元，整數字串
	PaymentType     string `json:"paymentType"` // 付款方式（PaymentMethod value；合法值以 CodeMeta 業務類型碼.md 為準，2001-2004）
	Status          string `json:"status"`      // PaymentTransactionStatus value；合法值：0=INIT,1=PAID,2=FAILED,3=EXPIRED,4=CONFLICT,5=CLOSED,6=CLOSED_BY_SWITCH
	ProviderStatus  string `json:"providerStatus"`
	ProviderMessage string `json:"providerMessage"`
	ProviderTradeNo string `json:"providerTradeNo"`
	CreatedAt       string `json:"createdAt"`
	UpdatedAt       string `json:"updatedAt"`
}

// AdminPaymentTransactionListResp 付款異常列表分頁
type AdminPaymentTransactionListResp struct {
	Total    string                               `json:"total"`
	Page     string                               `json:"page"`
	PageSize string                               `json:"pageSize"`
	Records  []AdminPaymentTransactionSummaryResp `json:"records"`
}

// AdminPaymentTransactionDetailResp 付款詳情
type AdminPaymentTransactionDetailResp struct {
	TradeNo              string `json:"tradeNo"`
	OrderNo              string `json:"orderNo"`
	BuyerId              string `json:"buyerId"`
	MerId                string `json:"merId"`
	Amount               string `json:"amount"`
	PaymentType          string `json:"paymentType"`
	ProductName          string `json:"productName"`
	Status               string `json:"status"`
	ProviderStatus       string `json:"providerStatus"`
	ProviderMessage      string `json:"providerMessage"`
	ProviderTradeNo      string `json:"providerTradeNo"`
	AtmBankCode          string `json:"atmBankCode,omitempty"`
	AtmVirtualAccount    string `json:"atmVirtualAccount,omitempty"`
	AtmExpireDate        string `json:"atmExpireDate,omitempty"`
	PayTime              string `json:"payTime,omitempty"`
	ProviderCreateAt     string `json:"providerCreateAt,omitempty"`
	ProviderPaidAt       string `json:"providerPaidAt,omitempty"`
	ExpireAt             string `json:"expireAt,omitempty"`
	ClosedAt             string `json:"closedAt,omitempty"`
	GrantedAt            string `json:"grantedAt,omitempty"`
	GrantStatus          string `json:"grantStatus,omitempty"`
	GrantMessage         string `json:"grantMessage,omitempty"`
	IsDuplicate          string `json:"isDuplicate"`
	DuplicateOfPaymentId string `json:"duplicateOfPaymentId,omitempty"`
	DuplicateSequence    string `json:"duplicateSequence,omitempty"`
	RescuedAt            string `json:"rescuedAt,omitempty"`
	RescueAction         string `json:"rescueAction,omitempty"`
	CreatedAt            string `json:"createdAt"`
	UpdatedAt            string `json:"updatedAt"`
}

// AdminPaymentRefundSummaryResp 退款列表單筆（也用於詳情）
type AdminPaymentRefundSummaryResp struct {
	RefundNo         string `json:"refundNo"`
	TradeNo          string `json:"tradeNo"`
	OrderNo          string `json:"orderNo"`
	Amount           string `json:"amount"`
	Status           string `json:"status"` // RefundStatus value；合法值：0=REQUESTED,1=PROCESSING,2=SUCCEEDED,3=FAILED
	ProviderStatus   string `json:"providerStatus"`
	ProviderMessage  string `json:"providerMessage"`
	ProviderRefundNo string `json:"providerRefundNo"`
	Reason           string `json:"reason"`
	CompletedAt      string `json:"completedAt,omitempty"`
	CreatedAt        string `json:"createdAt"`
	UpdatedAt        string `json:"updatedAt"`
}

// AdminPaymentRefundListResp 退款列表分頁
type AdminPaymentRefundListResp struct {
	Total    string                          `json:"total"`
	Page     string                          `json:"page"`
	PageSize string                          `json:"pageSize"`
	Records  []AdminPaymentRefundSummaryResp `json:"records"`
}

// AdminGrantSummaryResp 撥款異常列表單筆
type AdminGrantSummaryResp struct {
	TradeNo         string `json:"tradeNo"`
	OrderNo         string `json:"orderNo"`
	Amount          string `json:"amount"`
	GrantStatus     string `json:"grantStatus"`
	GrantMessage    string `json:"grantMessage"`
	GrantedAt       string `json:"grantedAt"`
	ProviderTradeNo string `json:"providerTradeNo"`
	CreatedAt       string `json:"createdAt"`
	UpdatedAt       string `json:"updatedAt"`
}

// AdminGrantListResp 撥款異常列表分頁
type AdminGrantListResp struct {
	Total    string                  `json:"total"`
	Page     string                  `json:"page"`
	PageSize string                  `json:"pageSize"`
	Records  []AdminGrantSummaryResp `json:"records"`
}

// ListPaymentTransactionsForAdmin 列付款異常
//
// @Summary      列付款異常
// @Description  Admin 查詢付款失敗 / 逾期 / 狀態衝突 / rescue 觸發的交易。
// @Tags         Admin 異常訂單 (Admin-Abnormal)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page              query  string  false  "頁碼（整數字串）"  default(1)
// @Param        page_size         query  string  false  "每頁筆數，最大 100"  default(20)
// @Param        abnormal_only     query  string  false  "僅異常；合法值：true / false"  default(true)
// @Param        provider_status   query  string  false  "PayUni 原始狀態碼 exact match"
// @Param        status            query  string  false  "PaymentTransactionStatus value，多值逗號分隔"
// @Param        order_no          query  string  false  "訂單號過濾"
// @Param        buyer_id          query  string  false  "依買家過濾"
// @Param        payment_type      query  string  false  "付款方式過濾（PaymentMethod value；合法值以 CodeMeta 業務類型碼.md 為準，2001-2004）"
// @Param        created_from      query  string  false  "ISO 8601 UTC"
// @Param        created_to        query  string  false  "ISO 8601 UTC"
// @Param        sort              query  string  false  "排序；合法值：updated_at_desc / updated_at_asc / created_at_desc / created_at_asc"  default(updated_at_desc)
// @Success      200  {object}  object{code=int,message=string,data=AdminPaymentTransactionListResp}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Router       /admin/payment_transactions [get]
func (h *Holder) ListPaymentTransactionsForAdmin(c *gin.Context) {
	buyerIDFilter, ok := h.resolveOptionalBuyerFilterQuery(c, c.Query("buyer_id"))
	if !ok {
		return
	}
	req := &pb.ListPaymentTransactionsForAdminRequest{
		Page:           c.DefaultQuery("page", "1"),
		PageSize:       c.DefaultQuery("page_size", "20"),
		AbnormalOnly:   c.DefaultQuery("abnormal_only", "true") == "true",
		ProviderStatus: c.Query("provider_status"),
		Status:         c.Query("status"),
		OrderNo:        c.Query("order_no"),
		BuyerId:        buyerIDFilter,
		PaymentType:    c.Query("payment_type"),
		CreatedFrom:    c.Query("created_from"),
		CreatedTo:      c.Query("created_to"),
		Sort:           c.DefaultQuery("sort", "updated_at_desc"),
	}

	resp, err := h.Payment.ListPaymentTransactionsForAdmin(c.Request.Context(), req)
	if err != nil {
		response.GrpcErr(c, "payment", err)
		return
	}
	if resp.GetCode() != 0 {
		response.GrpcResult(c, resp.GetCode(), resp.GetMessage(), nil)
		return
	}

	d := resp.GetData()
	records := make([]AdminPaymentTransactionSummaryResp, 0, len(d.GetRecords()))
	for _, r := range d.GetRecords() {
		records = append(records, AdminPaymentTransactionSummaryResp{
			TradeNo:         r.GetTradeNo(),
			OrderNo:         r.GetOrderNo(),
			BuyerId:         r.GetBuyerId(),
			MerId:           r.GetMerId(),
			Amount:          r.GetAmount(),
			PaymentType:     r.GetPaymentType(),
			Status:          r.GetStatus(),
			ProviderStatus:  r.GetProviderStatus(),
			ProviderMessage: r.GetProviderMessage(),
			ProviderTradeNo: r.GetProviderTradeNo(),
			CreatedAt:       r.GetCreatedAt(),
			UpdatedAt:       r.GetUpdatedAt(),
		})
	}
	response.OK(c, AdminPaymentTransactionListResp{
		Records:  records,
		Total:    d.GetTotal(),
		Page:     d.GetPage(),
		PageSize: d.GetPageSize(),
	})
}

// GetPaymentTransactionForAdmin 取付款詳情
//
// @Summary      取付款詳情
// @Description  Admin 查詢單筆付款交易詳情，含撥款 (grant_*)、重複付款、rescue 等欄位
// @Tags         Admin 異常訂單 (Admin-Abnormal)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        trade_no  path  string  true  "交易編號（Payment trade_no）"
// @Success      200  {object}  object{code=int,message=string,data=AdminPaymentTransactionDetailResp}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /admin/payment_transactions/{trade_no} [get]
func (h *Holder) GetPaymentTransactionForAdmin(c *gin.Context) {
	tradeNo := c.Param("trade_no")
	if tradeNo == "" {
		response.BadRequest(c, "trade_no 為必填")
		return
	}
	resp, err := h.Payment.GetPaymentTransactionForAdmin(c.Request.Context(), &pb.GetPaymentTransactionForAdminRequest{
		TradeNo: tradeNo,
	})
	if err != nil {
		response.GrpcErr(c, "payment", err)
		return
	}
	if resp.GetCode() != 0 {
		response.GrpcResult(c, resp.GetCode(), resp.GetMessage(), nil)
		return
	}
	d := resp.GetData()
	response.OK(c, AdminPaymentTransactionDetailResp{
		TradeNo:              d.GetTradeNo(),
		OrderNo:              d.GetOrderNo(),
		BuyerId:              d.GetBuyerId(),
		MerId:                d.GetMerId(),
		Amount:               d.GetAmount(),
		PaymentType:          d.GetPaymentType(),
		ProductName:          d.GetProductName(),
		Status:               d.GetStatus(),
		ProviderStatus:       d.GetProviderStatus(),
		ProviderMessage:      d.GetProviderMessage(),
		ProviderTradeNo:      d.GetProviderTradeNo(),
		AtmBankCode:          d.GetAtmBankCode(),
		AtmVirtualAccount:    d.GetAtmVirtualAccount(),
		AtmExpireDate:        d.GetAtmExpireDate(),
		PayTime:              d.GetPayTime(),
		ProviderCreateAt:     d.GetProviderCreateAt(),
		ProviderPaidAt:       d.GetProviderPaidAt(),
		ExpireAt:             d.GetExpireAt(),
		ClosedAt:             d.GetClosedAt(),
		GrantedAt:            d.GetGrantedAt(),
		GrantStatus:          d.GetGrantStatus(),
		GrantMessage:         d.GetGrantMessage(),
		IsDuplicate:          d.GetIsDuplicate(),
		DuplicateOfPaymentId: d.GetDuplicateOfPaymentId(),
		DuplicateSequence:    d.GetDuplicateSequence(),
		RescuedAt:            d.GetRescuedAt(),
		RescueAction:         d.GetRescueAction(),
		CreatedAt:            d.GetCreatedAt(),
		UpdatedAt:            d.GetUpdatedAt(),
	})
}

// ListRefundsForAdmin 列退款異常
//
// @Summary      列退款異常
// @Description  Admin 查詢退款失敗 / 處理超時 24h 的退款紀錄
// @Tags         Admin 異常訂單 (Admin-Abnormal)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page              query  string  false  "頁碼"  default(1)
// @Param        page_size         query  string  false  "每頁筆數"  default(20)
// @Param        abnormal_only     query  string  false  "合法值：true / false"  default(true)
// @Param        provider_status   query  string  false  "PayUni 原始狀態碼"
// @Param        status            query  string  false  "RefundStatus value，可多值逗號分隔"
// @Param        trade_no          query  string  false  "原交易號過濾"
// @Param        order_no          query  string  false  "訂單號過濾"
// @Param        created_from      query  string  false  "ISO 8601 UTC"
// @Param        created_to        query  string  false  "ISO 8601 UTC"
// @Param        sort              query  string  false  "合法值：updated_at_desc 等"  default(updated_at_desc)
// @Success      200  {object}  object{code=int,message=string,data=AdminPaymentRefundListResp}
// @Router       /admin/payment_refunds [get]
func (h *Holder) ListRefundsForAdmin(c *gin.Context) {
	req := &pb.ListRefundsForAdminRequest{
		Page:           c.DefaultQuery("page", "1"),
		PageSize:       c.DefaultQuery("page_size", "20"),
		AbnormalOnly:   c.DefaultQuery("abnormal_only", "true") == "true",
		ProviderStatus: c.Query("provider_status"),
		Status:         c.Query("status"),
		TradeNo:        c.Query("trade_no"),
		OrderNo:        c.Query("order_no"),
		CreatedFrom:    c.Query("created_from"),
		CreatedTo:      c.Query("created_to"),
		Sort:           c.DefaultQuery("sort", "updated_at_desc"),
	}
	resp, err := h.Payment.ListRefundsForAdmin(c.Request.Context(), req)
	if err != nil {
		response.GrpcErr(c, "payment", err)
		return
	}
	if resp.GetCode() != 0 {
		response.GrpcResult(c, resp.GetCode(), resp.GetMessage(), nil)
		return
	}
	d := resp.GetData()
	records := make([]AdminPaymentRefundSummaryResp, 0, len(d.GetRecords()))
	for _, r := range d.GetRecords() {
		records = append(records, AdminPaymentRefundSummaryResp{
			RefundNo:         r.GetRefundNo(),
			TradeNo:          r.GetTradeNo(),
			OrderNo:          r.GetOrderNo(),
			Amount:           r.GetAmount(),
			Status:           r.GetStatus(),
			ProviderStatus:   r.GetProviderStatus(),
			ProviderMessage:  r.GetProviderMessage(),
			ProviderRefundNo: r.GetProviderRefundNo(),
			Reason:           r.GetReason(),
			CompletedAt:      r.GetCompletedAt(),
			CreatedAt:        r.GetCreatedAt(),
			UpdatedAt:        r.GetUpdatedAt(),
		})
	}
	response.OK(c, AdminPaymentRefundListResp{
		Records:  records,
		Total:    d.GetTotal(),
		Page:     d.GetPage(),
		PageSize: d.GetPageSize(),
	})
}

// GetRefundForAdmin 取退款詳情
//
// @Summary      取退款詳情
// @Description  Admin 查詢單筆退款紀錄詳情
// @Tags         Admin 異常訂單 (Admin-Abnormal)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        refund_no  path  string  true  "退款編號（Payment refund_no）"
// @Success      200  {object}  object{code=int,message=string,data=AdminPaymentRefundSummaryResp}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /admin/payment_refunds/{refund_no} [get]
func (h *Holder) GetRefundForAdmin(c *gin.Context) {
	refundNo := c.Param("refund_no")
	if refundNo == "" {
		response.BadRequest(c, "refund_no 為必填")
		return
	}
	resp, err := h.Payment.GetRefundForAdmin(c.Request.Context(), &pb.GetRefundForAdminRequest{
		RefundNo: refundNo,
	})
	if err != nil {
		response.GrpcErr(c, "payment", err)
		return
	}
	if resp.GetCode() != 0 {
		response.GrpcResult(c, resp.GetCode(), resp.GetMessage(), nil)
		return
	}
	r := resp.GetData()
	response.OK(c, AdminPaymentRefundSummaryResp{
		RefundNo:         r.GetRefundNo(),
		TradeNo:          r.GetTradeNo(),
		OrderNo:          r.GetOrderNo(),
		Amount:           r.GetAmount(),
		Status:           r.GetStatus(),
		ProviderStatus:   r.GetProviderStatus(),
		ProviderMessage:  r.GetProviderMessage(),
		ProviderRefundNo: r.GetProviderRefundNo(),
		Reason:           r.GetReason(),
		CompletedAt:      r.GetCompletedAt(),
		CreatedAt:        r.GetCreatedAt(),
		UpdatedAt:        r.GetUpdatedAt(),
	})
}

// ListGrantsForAdmin 列撥款異常
//
// @Summary      列撥款異常
// @Description  Admin 查詢 PayUni 撥款失敗或異常狀態的交易紀錄
// @Tags         Admin 異常訂單 (Admin-Abnormal)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page           query  string  false  "頁碼"  default(1)
// @Param        page_size      query  string  false  "每頁筆數"  default(20)
// @Param        abnormal_only  query  string  false  "合法值：true / false"  default(true)
// @Param        grant_status   query  string  false  "PayUni 撥款狀態 exact match"
// @Param        order_no       query  string  false  "訂單號過濾"
// @Param        granted_from   query  string  false  "ISO 8601 UTC"
// @Param        granted_to     query  string  false  "ISO 8601 UTC"
// @Param        sort           query  string  false  "合法值：updated_at_desc 等"  default(updated_at_desc)
// @Success      200  {object}  object{code=int,message=string,data=AdminGrantListResp}
// @Router       /admin/grants [get]
func (h *Holder) ListGrantsForAdmin(c *gin.Context) {
	req := &pb.ListGrantsForAdminRequest{
		Page:         c.DefaultQuery("page", "1"),
		PageSize:     c.DefaultQuery("page_size", "20"),
		AbnormalOnly: c.DefaultQuery("abnormal_only", "true") == "true",
		GrantStatus:  c.Query("grant_status"),
		OrderNo:      c.Query("order_no"),
		GrantedFrom:  c.Query("granted_from"),
		GrantedTo:    c.Query("granted_to"),
		Sort:         c.DefaultQuery("sort", "updated_at_desc"),
	}
	resp, err := h.Payment.ListGrantsForAdmin(c.Request.Context(), req)
	if err != nil {
		response.GrpcErr(c, "payment", err)
		return
	}
	if resp.GetCode() != 0 {
		response.GrpcResult(c, resp.GetCode(), resp.GetMessage(), nil)
		return
	}
	d := resp.GetData()
	records := make([]AdminGrantSummaryResp, 0, len(d.GetRecords()))
	for _, r := range d.GetRecords() {
		records = append(records, AdminGrantSummaryResp{
			TradeNo:         r.GetTradeNo(),
			OrderNo:         r.GetOrderNo(),
			Amount:          r.GetAmount(),
			GrantStatus:     r.GetGrantStatus(),
			GrantMessage:    r.GetGrantMessage(),
			GrantedAt:       r.GetGrantedAt(),
			ProviderTradeNo: r.GetProviderTradeNo(),
			CreatedAt:       r.GetCreatedAt(),
			UpdatedAt:       r.GetUpdatedAt(),
		})
	}
	response.OK(c, AdminGrantListResp{
		Records:  records,
		Total:    d.GetTotal(),
		Page:     d.GetPage(),
		PageSize: d.GetPageSize(),
	})
}
