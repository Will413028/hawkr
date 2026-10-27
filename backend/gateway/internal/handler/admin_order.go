package handler

import (
	"github.com/Will413028/hawkr/gateway/internal/response"
	pb "github.com/Will413028/hawkr/go-pkg/proto/order"
	"github.com/gin-gonic/gin"
)

// ==================== §29 Admin 異常訂單列表 ====================

// AdminAbnormalOrderSummaryResp 異常訂單列表單筆
type AdminAbnormalOrderSummaryResp struct {
	OrderNo           string `json:"orderNo"`
	BuyerId           string `json:"buyerId"`
	StoreId           string `json:"storeId"`
	Status            string `json:"status"` // OrderStatus value；預設列 35=SHIPPING_ABNORMAL；合法值另有 0=PENDING_PAYMENT / 10=PAID / 20=PENDING_SHIPMENT / 30=IN_TRANSIT / 32=WAITING_PICKUP / 35=SHIPPING_ABNORMAL / 40=DELIVERED / 50=COMPLETED / 60=PARTIALLY_REFUNDED / 70=FULLY_REFUNDED / 80=CANCELLED
	TotalActualAmount string `json:"totalActualAmount"`
	PaymentMethod     string `json:"paymentMethod"`
	ShippingMethod    string `json:"shippingMethod"`
	ProviderStatus    string `json:"providerStatus"`
	ProviderMessage   string `json:"providerMessage"`
	ProviderSource    string `json:"providerSource"` // CodeMeta ProviderSource：PAYMENT / PAYMENT_REFUND / SHIPPING；"" = 無第三方環節可對帳
	CreatedAt         string `json:"createdAt"`
	UpdatedAt         string `json:"updatedAt"`
}

// AdminAbnormalOrderListResp 異常訂單列表分頁
type AdminAbnormalOrderListResp struct {
	Total    string                          `json:"total"`
	Page     string                          `json:"page"`
	PageSize string                          `json:"pageSize"`
	Records  []AdminAbnormalOrderSummaryResp `json:"records"`
}

// ListAbnormalOrdersForAdmin §29 O1 列異常訂單（跨服務聚合）
//
// @Summary      列異常訂單
// @Description  Admin 查詢跨服務異常訂單（付款 / 物流 / 退款失敗）。預設 filter status=35 (SHIPPING_ABNORMAL)
// @Tags         Admin 異常訂單 (Admin-Abnormal)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page              query  string  false  "頁碼"  default(1)
// @Param        page_size         query  string  false  "每頁筆數，最大 100"  default(20)
// @Param        provider_status   query  string  false  "PayUni 原始狀態碼 exact match"
// @Param        status            query  string  false  "OrderStatus value，多值逗號分隔；未帶預設 35"
// @Param        buyer_id          query  string  false  "依買家過濾（現任 buyer_id 代理鍵）"
// @Param        store_id          query  string  false  "依賣家過濾（現任 store_id 代理鍵）"
// @Param        payment_method    query  string  false  "PaymentMethod value 過濾"
// @Param        shipping_method   query  string  false  "ShippingMethod value 過濾"
// @Param        created_from      query  string  false  "ISO 8601 UTC"
// @Param        created_to        query  string  false  "ISO 8601 UTC"
// @Param        sort              query  string  false  "合法值：updated_at_desc / updated_at_asc / created_at_desc / created_at_asc"  default(updated_at_desc)
// @Success      200  {object}  object{code=int,message=string,data=AdminAbnormalOrderListResp}
// @Router       /admin/orders/abnormal [get]
func (h *Holder) ListAbnormalOrdersForAdmin(c *gin.Context) {
	buyerIDFilter, ok := h.resolveOptionalBuyerFilterQuery(c, c.Query("buyer_id"))
	if !ok {
		return
	}
	// ② store_id 同為 admin 搜尋 filter，鏡像上面的買家端解析（D7，
	// section D：本篩選先前漏改，與 SearchOrders 同批修正）。
	storeIDFilter, ok := h.resolveOptionalSellerFilterQuery(c, c.Query("store_id"))
	if !ok {
		return
	}

	req := &pb.ListAbnormalOrdersForAdminRequest{
		Page:           c.DefaultQuery("page", "1"),
		PageSize:       c.DefaultQuery("page_size", "20"),
		ProviderStatus: c.Query("provider_status"),
		Status:         c.Query("status"),
		BuyerId:        buyerIDFilter,
		StoreId:        storeIDFilter,
		PaymentMethod:  c.Query("payment_method"),
		ShippingMethod: c.Query("shipping_method"),
		CreatedFrom:    c.Query("created_from"),
		CreatedTo:      c.Query("created_to"),
		Sort:           c.DefaultQuery("sort", "updated_at_desc"),
	}
	resp, err := h.Order.ListAbnormalOrdersForAdmin(c.Request.Context(), req)
	if err != nil {
		response.GrpcErr(c, "order", err)
		return
	}
	if resp.GetCode() != 0 {
		response.GrpcResult(c, resp.GetCode(), resp.GetMessage(), nil)
		return
	}
	d := resp.GetData()
	records := make([]AdminAbnormalOrderSummaryResp, 0, len(d.GetRecords()))
	for _, r := range d.GetRecords() {
		records = append(records, AdminAbnormalOrderSummaryResp{
			OrderNo:           r.GetOrderNo(),
			BuyerId:           r.GetBuyerId(),
			StoreId:           r.GetStoreId(),
			Status:            r.GetStatus(),
			TotalActualAmount: r.GetTotalActualAmount(),
			PaymentMethod:     r.GetPaymentMethod(),
			ShippingMethod:    r.GetShippingMethod(),
			ProviderStatus:    r.GetProviderStatus(),
			ProviderMessage:   r.GetProviderMessage(),
			ProviderSource:    r.GetProviderSource(),
			CreatedAt:         r.GetCreatedAt(),
			UpdatedAt:         r.GetUpdatedAt(),
		})
	}
	response.OK(c, AdminAbnormalOrderListResp{
		Records:  records,
		Total:    d.GetTotal(),
		Page:     d.GetPage(),
		PageSize: d.GetPageSize(),
	})
}

// ==================== Admin 撥款重發（收斂規範 §8.4 人工介入閉環）====================

// RetryGrantRequest Admin 重發撥款請求
type RetryGrantRequest struct {
	OperatorReason string `json:"operatorReason,omitempty"` // 重發原因（選填，稽核用）
}

// RetryGrantResponseData 重發撥款結果
type RetryGrantResponseData struct {
	OrderNo            string `json:"orderNo"`
	TaskStatus         string `json:"taskStatus"`         // 重設後 task 狀態（= PENDING）
	PreviousRetryCount string `json:"previousRetryCount"` // 重設前 retry_count 值
}

// RetryGrant Admin 重發撥款任務
//
// 撥款失敗後提供 Admin 手動重新觸發，Worker 下一輪掃描會重新執行。
// 與其他 PayUni 呼叫不同，撥款為唯一允許的重試入口（收斂規範 §8.4 撥款例外）。
//
// @Summary      Admin 重發撥款
// @Description  重新觸發既有 grant_payment_tasks 的執行；僅 Admin 可用。要求 task 目前為 FAILED 或 MANUAL_INTERVENTION 狀態，DONE 不可重發。
// @Tags         Admin (Admin)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        order_no  path      string             true  "訂單編號"
// @Param        body      body      RetryGrantRequest  false "重發原因"
// @Success      200  {object}  object{code=int,message=string,data=RetryGrantResponseData}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Failure      409  {object}  object{code=int,message=string}
// @Router       /admin/orders/{order_no}/retry_grant [post]
func (h *Holder) RetryGrant(c *gin.Context) {
	orderNo := c.Param("order_no")
	if orderNo == "" {
		response.BadRequest(c, "order_no 為必填")
		return
	}

	var req RetryGrantRequest
	_ = c.ShouldBindJSON(&req) // body 全選填

	operatorID := c.GetHeader("x-account-id")
	if operatorID == "" {
		response.Unauthorized(c)
		return
	}

	resp, err := h.Order.RetryGrant(c.Request.Context(), &pb.RetryGrantRequest{
		OrderNo:        orderNo,
		OperatorId:     operatorID,
		OperatorReason: req.OperatorReason,
	})
	if err != nil {
		response.GrpcErr(c, "order", err)
		return
	}

	if resp.GetCode() != 0 {
		response.GrpcResult(c, resp.GetCode(), resp.GetMessage(), nil)
		return
	}

	data := resp.GetData()
	response.OK(c, RetryGrantResponseData{
		OrderNo:            data.GetOrderNo(),
		TaskStatus:         data.GetTaskStatus(),
		PreviousRetryCount: data.GetPreviousRetryCount(),
	})
}
