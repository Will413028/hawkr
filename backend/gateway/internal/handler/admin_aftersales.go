package handler

import (
	"github.com/Will413028/hawkr/gateway/internal/response"
	pb "github.com/Will413028/hawkr/go-pkg/proto/aftersales"
	"github.com/gin-gonic/gin"
)

// ==================== Admin 異常售後列表（§29 A1）====================

// AdminAfterSaleCaseSummaryResp 售後異常列表單筆
type AdminAfterSaleCaseSummaryResp struct {
	CaseNo           string `json:"caseNo"`
	OrderNo          string `json:"orderNo"`
	BuyerId          string `json:"buyerId"`
	StoreId          string `json:"storeId"`
	Status           string `json:"status"` // AfterSaleCaseStatus value；合法值：0=PENDING, 20=RETURN_PENDING, 25=RETURN_SHIPPING, 40=REFUND_PROCESSING, 50=CLOSED, 60=REJECTED, 70=CANCELLED, 71=CANCELLED_BY_CONFIRM_RECEIPT
	RefundableAmount string `json:"refundableAmount"`
	PaymentMethod    string `json:"paymentMethod"`
	ProviderStatus   string `json:"providerStatus"` // 依 case.status 聚合：40 取 Payment / 20,25 取 Shipping
	ProviderMessage  string `json:"providerMessage"`
	CreatedAt        string `json:"createdAt"`
	UpdatedAt        string `json:"updatedAt"`
}

// AdminAfterSaleCaseListResp 售後異常列表分頁
type AdminAfterSaleCaseListResp struct {
	Total    string                          `json:"total"`
	Page     string                          `json:"page"`
	PageSize string                          `json:"pageSize"`
	Records  []AdminAfterSaleCaseSummaryResp `json:"records"`
}

// ListAfterSaleCasesForAdmin §29 A1 列售後異常
//
// @Summary      列售後異常
// @Description  Admin 查詢 PayUni 退款失敗 / 退貨物流失敗 等售後異常案件。abnormal_only=true 時 filter REFUND_PROCESSING(40) / RETURN_PENDING(20) / RETURN_SHIPPING(25)
// @Tags         Admin 異常訂單 (Admin-Abnormal)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page              query  string  false  "頁碼"  default(1)
// @Param        page_size         query  string  false  "每頁筆數，最大 100"  default(20)
// @Param        abnormal_only     query  string  false  "僅異常；合法值：true / false"  default(true)
// @Param        provider_status   query  string  false  "PayUni 原始狀態碼 exact match"
// @Param        status            query  string  false  "AfterSaleCaseStatus value，可多值逗號分隔"
// @Param        buyer_id          query  string  false  "依買家過濾（傳入原始登入帳號代碼，gateway 解析成現任 buyer_id 代理鍵）"
// @Param        store_id          query  string  false  "依賣家過濾（傳入原始登入帳號代碼，gateway 解析成現任 store_id 代理鍵）"
// @Param        order_no          query  string  false  "依訂單號過濾"
// @Param        created_from      query  string  false  "ISO 8601 UTC"
// @Param        created_to        query  string  false  "ISO 8601 UTC"
// @Param        sort              query  string  false  "合法值：updated_at_desc / updated_at_asc / created_at_desc / created_at_asc"  default(updated_at_desc)
// @Success      200  {object}  object{code=int,message=string,data=AdminAfterSaleCaseListResp}
// @Router       /admin/after_sales [get]
func (h *Holder) ListAfterSaleCasesForAdmin(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok || accountID == 0 {
		response.Unauthorized(c)
		return
	}
	// ② buyer_id 為 admin 搜尋 filter，一律解析成現任 buyer_id（D7，
	// 跟 SearchOrders/ListAbnormalOrdersForAdmin 一致）。已核對
	// AfterSalesGrpcService.listAfterSaleCasesForAdmin：StringUtils.hasText +
	// wrapper.eq(Long.parseLong(...))，非空字串一律套用 filter，"-1" 保證查無
	// 結果（同 order-service 語意，非 "0 視為未篩選" 那套）。query key 已於
	// 2026-08 硬切為 buyer_id/store_id，不再相容舊版買家/賣家過濾用的舊查詢
	// 鍵名（F1，前端已同步遷移）。
	buyerIDFilter, ok := h.resolveOptionalBuyerFilterQuery(c, c.Query("buyer_id"))
	if !ok {
		return
	}
	storeIDFilter, ok := h.resolveOptionalSellerFilterQuery(c, c.Query("store_id"))
	if !ok {
		return
	}
	req := &pb.ListAfterSaleCasesForAdminRequest{
		AccountId:      accountID,
		Page:           c.DefaultQuery("page", "1"),
		PageSize:       c.DefaultQuery("page_size", "20"),
		AbnormalOnly:   c.DefaultQuery("abnormal_only", "true") == "true",
		ProviderStatus: c.Query("provider_status"),
		Status:         c.Query("status"),
		BuyerId:        buyerIDFilter,
		StoreId:        storeIDFilter,
		OrderNo:        c.Query("order_no"),
		CreatedFrom:    c.Query("created_from"),
		CreatedTo:      c.Query("created_to"),
		Sort:           c.DefaultQuery("sort", "updated_at_desc"),
	}
	resp, err := h.Aftersales.ListAfterSaleCasesForAdmin(c.Request.Context(), req)
	if err != nil {
		response.GrpcErr(c, "aftersales", err)
		return
	}
	if resp.GetCode() != 0 {
		response.GrpcResult(c, resp.GetCode(), resp.GetMessage(), nil)
		return
	}
	d := resp.GetData()
	records := make([]AdminAfterSaleCaseSummaryResp, 0, len(d.GetRecords()))
	for _, r := range d.GetRecords() {
		records = append(records, AdminAfterSaleCaseSummaryResp{
			CaseNo:           r.GetCaseNo(),
			OrderNo:          r.GetOrderNo(),
			BuyerId:          r.GetBuyerId(),
			StoreId:          r.GetStoreId(),
			Status:           r.GetStatus(),
			RefundableAmount: r.GetRefundableAmount(),
			PaymentMethod:    r.GetPaymentMethod(),
			ProviderStatus:   r.GetProviderStatus(),
			ProviderMessage:  r.GetProviderMessage(),
			CreatedAt:        r.GetCreatedAt(),
			UpdatedAt:        r.GetUpdatedAt(),
		})
	}
	response.OK(c, AdminAfterSaleCaseListResp{
		Records:  records,
		Total:    d.GetTotal(),
		Page:     d.GetPage(),
		PageSize: d.GetPageSize(),
	})
}

// ==================== Admin 強制確認收貨（§5.3.12）====================

// AdminForceConfirmReceiptRequest Admin 強制確認收貨請求
type AdminForceConfirmReceiptRequest struct {
	Reason             string `json:"reason"                       binding:"required" example:"PayUni 漏發送達通知，賣家提供簽收照"`
	ActualRefundAmount string `json:"actualRefundAmount,omitempty" example:"1200"`      // 選填，預設全額退款
	DeductionReason    string `json:"deductionReason,omitempty"    example:"驗貨發現外包裝破損"` // 選填，actualRefundAmount < original 時必填
}

// AdminForceConfirmReceiptResp Admin 強制確認收貨回應
type AdminForceConfirmReceiptResp struct {
	CaseNo            string `json:"caseNo"`
	PriorStatus       string `json:"priorStatus"`       // 強制前的 status（記錄用）
	RefundTaskId      string `json:"refundTaskId"`      // 建立的退款任務 ID
	FinalRefundAmount string `json:"finalRefundAmount"` // 實際退款金額
}

// ForceConfirmReceiptForAdmin §5.3.12 Admin 強制確認收貨
//
// @Summary      Admin 強制確認收貨
// @Description  Admin 在 PayUni 漏發送達 / 物流失敗 / 賣家失聯時介入，強制推進案件至 RETURN_RECEIVED(30) → REFUND_PROCESSING(40) 並啟動退款。允許 status ∈ 20/25/27/28。
// @Tags         Admin 售後 (Admin-AfterSales)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        case_no path     string                           true  "售後單編號"
// @Param        body    body     AdminForceConfirmReceiptRequest  true  "強制確認收貨請求"
// @Success      200     {object} object{code=int,message=string,data=AdminForceConfirmReceiptResp}
// @Failure      400     {object} object{code=int,message=string}  "INVALID_PARAM"
// @Failure      404     {object} object{code=int,message=string}  "AFTERSALE_NOT_FOUND"
// @Failure      409     {object} object{code=int,message=string}  "AFTERSALE_INVALID_STATUS (status 不在 20/25/27/28)"
// @Router       /admin/after_sales/{case_no}/force_confirm_receipt [post]
func (h *Holder) ForceConfirmReceiptForAdmin(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok || accountID == 0 {
		response.Unauthorized(c)
		return
	}
	caseNo := c.Param("case_no")
	if caseNo == "" {
		response.BadRequest(c, "case_no is required")
		return
	}
	var body AdminForceConfirmReceiptRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	req := &pb.ForceConfirmReceiptForAdminRequest{
		AccountId:          accountID,
		CaseNo:             caseNo,
		Reason:             body.Reason,
		ActualRefundAmount: body.ActualRefundAmount,
		DeductionReason:    body.DeductionReason,
	}
	resp, err := h.Aftersales.ForceConfirmReceiptForAdmin(c.Request.Context(), req)
	if err != nil {
		response.GrpcErr(c, "aftersales", err)
		return
	}
	if resp.GetCode() != 0 {
		response.GrpcResult(c, resp.GetCode(), resp.GetMessage(), nil)
		return
	}
	d := resp.GetData()
	response.OK(c, AdminForceConfirmReceiptResp{
		CaseNo:            d.GetCaseNo(),
		PriorStatus:       d.GetPriorStatus(),
		RefundTaskId:      d.GetRefundTaskId(),
		FinalRefundAmount: d.GetFinalRefundAmount(),
	})
}
