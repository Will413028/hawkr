package handler

import (
	"github.com/Will413028/hawkr/gateway/internal/response"
	pb "github.com/Will413028/hawkr/go-pkg/proto/shipping"
	"github.com/gin-gonic/gin"
)

// ==================== Admin 異常查詢 API（§29 Admin 異常訂單管理）====================

// AdminShipmentSummaryResp 物流異常列表單筆
type AdminShipmentSummaryResp struct {
	ShipmentId      string `json:"shipmentId"`
	ShipmentNo      string `json:"shipmentNo"`
	OrderNo         string `json:"orderNo"`
	StoreId         string `json:"storeId"`
	ShippingMethod  string `json:"shippingMethod"`  // 合法值：3001=TAQBIN, 3002=SEVEN_ELEVEN_STORE_PICKUP
	IsReturn        string `json:"isReturn"`        // "true" / "false"
	Status          string `json:"status"`          // ShippingStatus value
	ProviderStatus  string `json:"providerStatus"`  // PayUni 原始貨態碼（正常軌跡證據，不代表異常）
	ProviderMessage string `json:"providerMessage"` // PayUni 原始貨態訊息
	// ProviderErrorCode 非空＝provider 層失敗，abnormalOnly 的判準（provider_status 不是）
	ProviderErrorCode    string `json:"providerErrorCode,omitempty"`
	ProviderErrorMessage string `json:"providerErrorMessage,omitempty"`
	ShipTradeNo          string `json:"shipTradeNo"`
	MerId                string `json:"merId"`
	CreatedAt            string `json:"createdAt"`
	UpdatedAt            string `json:"updatedAt"`
}

// AdminShipmentListResp 物流異常列表分頁
type AdminShipmentListResp struct {
	Total    string                     `json:"total"`
	Page     string                     `json:"page"`
	PageSize string                     `json:"pageSize"`
	Records  []AdminShipmentSummaryResp `json:"records"`
}

// AdminShipmentDetailResp 物流詳情
type AdminShipmentDetailResp struct {
	SenderName         string `json:"senderName"`
	ShipmentNo         string `json:"shipmentNo"`
	OrderNo            string `json:"orderNo"`
	StoreId            string `json:"storeId"` // 賣家 store 代理鍵
	BuyerId            string `json:"buyerId"` // 買家 buyer 代理鍵
	ShippingMethod     string `json:"shippingMethod"`
	IsReturn           string `json:"isReturn"`
	Status             string `json:"status"`
	ProviderStatus     string `json:"providerStatus"`
	ProviderMessage    string `json:"providerMessage"`
	ProviderStatusTime string `json:"providerStatusTime,omitempty"`
	// ProviderErrorCode 非空＝provider 層失敗，abnormalOnly 的判準（provider_status 不是）
	ProviderErrorCode    string   `json:"providerErrorCode,omitempty"`
	ProviderErrorMessage string   `json:"providerErrorMessage,omitempty"`
	ProviderErrorAt      string   `json:"providerErrorAt,omitempty"`
	ShipTradeNo          string   `json:"shipTradeNo"`
	MerId                string   `json:"merId"`
	ShipType             string   `json:"shipType,omitempty"`
	LgsType              string   `json:"lgsType,omitempty"`
	GoodsType            string   `json:"goodsType,omitempty"`
	TradeAmt             string   `json:"tradeAmt"`
	Spec                 string   `json:"spec,omitempty"`
	LabelPrintStatus     string   `json:"labelPrintStatus"` // LabelPrintStatus value: "4101"~"4104"
	SenderPhone          string   `json:"senderPhone"`
	RecipientName        string   `json:"recipientName"`
	RecipientPhone       string   `json:"recipientPhone"`
	RecipientAddress     string   `json:"recipientAddress,omitempty"`
	CvsStoreCode         string   `json:"cvsStoreCode,omitempty"` // 超商取貨門市代碼（原 storeId，與上方 storeId 代理鍵無關，純屬巧合曾同名）
	CvsStoreName         string   `json:"cvsStoreName,omitempty"` // 超商取貨門市名稱（原 storeName）
	PartnerId            string   `json:"partnerId,omitempty"`
	ValidationNo         string   `json:"validationNo,omitempty"`
	Odno                 string   `json:"odno,omitempty"`
	ShipmentId           string   `json:"shipmentId"`
	TcatCallStatus       string   `json:"tcatCallStatus"` // TcatCallStatus value: "4201"~"4204"
	TcatCallTime         string   `json:"tcatCallTime,omitempty"`
	LabelKey             string   `json:"labelKey,omitempty"` // 託運單 PDF objectKey（SSOT；49 §6.1）
	LabelUrl             string   `json:"labelUrl,omitempty"` // 託運單 PDF 顯示 URL（Gateway 組）
	UpdatedAt            string   `json:"updatedAt,omitempty"`
	LastReconciledAt     string   `json:"lastReconciledAt,omitempty"`
	CreatedAt            string   `json:"createdAt"`
	ShipmentPhotoKeys    []string `json:"shipmentPhotoKeys,omitempty"` // 出貨照片 objectKey（SSOT；49 §6.1）
	ShipmentPhotoUrls    []string `json:"shipmentPhotoUrls,omitempty"` // 出貨照片顯示 URL（Gateway 組）
}

// AdminSellerShipmentSettingSummaryResp 建店異常列表單筆
type AdminSellerShipmentSettingSummaryResp struct {
	SettingId          string `json:"settingId"`
	StoreId            string `json:"storeId"`
	ShippingType       string `json:"shippingType"` // 合法值：CVS_C2C / HOME_DELIVERY
	GoodsType          string `json:"goodsType,omitempty"`
	OnboardingStatus   string `json:"onboardingStatus"` // OnboardingStatus value
	ProviderStatus     string `json:"providerStatus"`
	ProviderMessage    string `json:"providerMessage"`
	MerId              string `json:"merId"`
	PartnerId          string `json:"partnerId,omitempty"`
	EshipId            string `json:"eshipId,omitempty"`
	PayuniSubmitStatus string `json:"payuniSubmitStatus"` // PayUniSubmitStatus value: "4301"~"4304"
	Enabled            string `json:"enabled"`            // "true" / "false"
	CreatedAt          string `json:"createdAt"`
	UpdatedAt          string `json:"updatedAt,omitempty"`
}

// AdminSellerShipmentSettingListResp 建店異常列表分頁
type AdminSellerShipmentSettingListResp struct {
	Total    string                                  `json:"total"`
	Page     string                                  `json:"page"`
	PageSize string                                  `json:"pageSize"`
	Records  []AdminSellerShipmentSettingSummaryResp `json:"records"`
}

// ListShipmentsForAdmin 列物流異常
//
// @Summary      列物流異常
// @Description  Admin 查詢 PayUni 物流建單失敗 / 異常的記錄。abnormal_only=true 時僅回異常態（CREATE_FAILED / ABNORMAL / 退貨異常等）
// @Tags         Admin 異常訂單 (Admin-Abnormal)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page              query  string  false  "頁碼，整數字串"  default(1)
// @Param        page_size         query  string  false  "每頁筆數，整數字串，最大 100"  default(20)
// @Param        abnormal_only     query  string  false  "僅異常；合法值：true / false"  default(true)
// @Param        provider_status   query  string  false  "PayUni 原始狀態碼 exact match（如 E001）"
// @Param        status            query  string  false  "平台 ShippingStatus value，多值逗號分隔"
// @Param        is_return         query  string  false  "是否退貨；合法值：true / false / 留空=全部"
// @Param        shipping_method   query  string  false  "配送方式；合法值：3001=TAQBIN / 3002=SEVEN_ELEVEN_STORE_PICKUP"
// @Param        store_id          query  string  false  "依賣家過濾"
// @Param        order_no          query  string  false  "依訂單號過濾"
// @Param        created_from      query  string  false  "建立時間起 (ISO 8601 UTC)"
// @Param        created_to        query  string  false  "建立時間迄 (ISO 8601 UTC)"
// @Param        sort              query  string  false  "排序；合法值：updated_at_desc / updated_at_asc / created_at_desc / created_at_asc"  default(updated_at_desc)
// @Success      200  {object}  object{code=int,message=string,data=AdminShipmentListResp}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Router       /admin/shipments [get]
func (h *Holder) ListShipmentsForAdmin(c *gin.Context) {
	storeIDFilter, ok := h.resolveOptionalSellerFilterQuery(c, c.Query("store_id"))
	if !ok {
		return
	}
	req := &pb.ListShipmentsForAdminRequest{
		Page:           c.DefaultQuery("page", "1"),
		PageSize:       c.DefaultQuery("page_size", "20"),
		AbnormalOnly:   c.DefaultQuery("abnormal_only", "true") == "true",
		ProviderStatus: c.Query("provider_status"),
		Status:         c.Query("status"),
		IsReturn:       c.Query("is_return"),
		ShippingMethod: c.Query("shipping_method"),
		StoreId:        storeIDFilter,
		OrderNo:        c.Query("order_no"),
		CreatedFrom:    c.Query("created_from"),
		CreatedTo:      c.Query("created_to"),
		Sort:           c.DefaultQuery("sort", "updated_at_desc"),
	}

	resp, err := h.Shipping.ListShipmentsForAdmin(c.Request.Context(), req)
	if err != nil {
		response.GrpcErr(c, "shipping", err)
		return
	}
	if resp.GetCode() != 0 {
		response.GrpcResult(c, resp.GetCode(), resp.GetMessage(), nil)
		return
	}

	data := resp.GetData()
	records := make([]AdminShipmentSummaryResp, 0, len(data.GetRecords()))
	for _, r := range data.GetRecords() {
		records = append(records, AdminShipmentSummaryResp{
			ShipmentId:           r.GetShipmentId(),
			ShipmentNo:           r.GetShipmentNo(),
			OrderNo:              r.GetOrderNo(),
			StoreId:              r.GetStoreId(),
			ShippingMethod:       r.GetShippingMethod(),
			IsReturn:             r.GetIsReturn(),
			Status:               r.GetStatus(),
			ProviderStatus:       r.GetProviderStatus(),
			ProviderErrorCode:    r.GetProviderErrorCode(),
			ProviderErrorMessage: r.GetProviderErrorMessage(),
			ProviderMessage:      r.GetProviderMessage(),
			ShipTradeNo:          r.GetShipTradeNo(),
			MerId:                r.GetMerId(),
			CreatedAt:            r.GetCreatedAt(),
			UpdatedAt:            r.GetUpdatedAt(),
		})
	}
	response.OK(c, AdminShipmentListResp{
		Records:  records,
		Total:    data.GetTotal(),
		Page:     data.GetPage(),
		PageSize: data.GetPageSize(),
	})
}

// GetShipmentForAdmin 取物流詳情（Admin）
//
// @Summary      取物流詳情
// @Description  Admin 查詢單筆物流單詳情，含完整 provider_status / provider_message 與寄收件人資訊
// @Tags         Admin 異常訂單 (Admin-Abnormal)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Shipment TSID"
// @Success      200  {object}  object{code=int,message=string,data=AdminShipmentDetailResp}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /admin/shipments/{id} [get]
func (h *Holder) GetShipmentForAdmin(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.BadRequest(c, "id 為必填")
		return
	}

	resp, err := h.Shipping.GetShipmentForAdmin(c.Request.Context(), &pb.GetShipmentForAdminRequest{
		ShipmentId: id,
	})
	if err != nil {
		response.GrpcErr(c, "shipping", err)
		return
	}
	if resp.GetCode() != 0 {
		response.GrpcResult(c, resp.GetCode(), resp.GetMessage(), nil)
		return
	}

	d := resp.GetData()
	response.OK(c, AdminShipmentDetailResp{
		ShipmentId:           d.GetShipmentId(),
		ShipmentNo:           d.GetShipmentNo(),
		OrderNo:              d.GetOrderNo(),
		StoreId:              d.GetStoreId(),
		BuyerId:              d.GetBuyerId(),
		ShippingMethod:       d.GetShippingMethod(),
		IsReturn:             d.GetIsReturn(),
		Status:               d.GetStatus(),
		ProviderStatus:       d.GetProviderStatus(),
		ProviderMessage:      d.GetProviderMessage(),
		ProviderStatusTime:   d.GetProviderStatusTime(),
		ProviderErrorCode:    d.GetProviderErrorCode(),
		ProviderErrorMessage: d.GetProviderErrorMessage(),
		ProviderErrorAt:      d.GetProviderErrorAt(),
		ShipTradeNo:          d.GetShipTradeNo(),
		MerId:                d.GetMerId(),
		ShipType:             d.GetShipType(),
		LgsType:              d.GetLgsType(),
		GoodsType:            d.GetGoodsType(),
		Spec:                 d.GetSpec(),
		TradeAmt:             d.GetTradeAmt(),
		SenderName:           d.GetSenderName(),
		SenderPhone:          d.GetSenderPhone(),
		RecipientName:        d.GetRecipientName(),
		RecipientPhone:       d.GetRecipientPhone(),
		RecipientAddress:     d.GetRecipientAddress(),
		CvsStoreCode:         d.GetCvsStoreCode(),
		CvsStoreName:         d.GetStoreName(),
		PartnerId:            d.GetPartnerId(),
		ValidationNo:         d.GetValidationNo(),
		Odno:                 d.GetOdno(),
		LabelPrintStatus:     d.GetLabelPrintStatus(),
		TcatCallStatus:       d.GetTcatCallStatus(),
		TcatCallTime:         d.GetTcatCallTime(),
		LabelKey:             d.GetLabelKey(),
		LabelUrl:             buildURL(h.MediaBaseURL, d.GetLabelKey()),
		ShipmentPhotoKeys:    d.GetShipmentPhotoKeys(),
		ShipmentPhotoUrls:    buildURLs(h.MediaBaseURL, d.GetShipmentPhotoKeys()),
		LastReconciledAt:     d.GetLastReconciledAt(),
		CreatedAt:            d.GetCreatedAt(),
		UpdatedAt:            d.GetUpdatedAt(),
	})
}

// ListSellerShipmentSettingsForAdmin 列建店異常
//
// @Summary      列建店異常
// @Description  Admin 查詢 PayUni 建店審核失敗的賣家物流設定。預設 filter onboarding_status=REJECTED
// @Tags         Admin 異常訂單 (Admin-Abnormal)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page              query  string  false  "頁碼，整數字串"  default(1)
// @Param        page_size         query  string  false  "每頁筆數，整數字串，最大 100"  default(20)
// @Param        abnormal_only     query  string  false  "僅異常；合法值：true / false"  default(true)
// @Param        provider_status   query  string  false  "PayUni 原始狀態碼 exact match"
// @Param        onboarding_status query  string  false  "OnboardingStatus value，多值逗號分隔"
// @Param        shipping_type     query  string  false  "合法值：CVS_C2C / HOME_DELIVERY"
// @Param        store_id          query  string  false  "依賣家過濾"
// @Param        created_from      query  string  false  "建立時間起 (ISO 8601 UTC)"
// @Param        created_to        query  string  false  "建立時間迄 (ISO 8601 UTC)"
// @Param        sort              query  string  false  "排序；合法值：updated_at_desc / updated_at_asc / created_at_desc / created_at_asc"  default(updated_at_desc)
// @Success      200  {object}  object{code=int,message=string,data=AdminSellerShipmentSettingListResp}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Router       /admin/seller_shipment_settings [get]
func (h *Holder) ListSellerShipmentSettingsForAdmin(c *gin.Context) {
	req := &pb.ListSellerShipmentSettingsForAdminRequest{
		Page:             c.DefaultQuery("page", "1"),
		PageSize:         c.DefaultQuery("page_size", "20"),
		AbnormalOnly:     c.DefaultQuery("abnormal_only", "true") == "true",
		ProviderStatus:   c.Query("provider_status"),
		OnboardingStatus: c.Query("onboarding_status"),
		ShippingType:     c.Query("shipping_type"),
		StoreId:          c.Query("store_id"),
		CreatedFrom:      c.Query("created_from"),
		CreatedTo:        c.Query("created_to"),
		Sort:             c.DefaultQuery("sort", "updated_at_desc"),
	}

	resp, err := h.Shipping.ListSellerShipmentSettingsForAdmin(c.Request.Context(), req)
	if err != nil {
		response.GrpcErr(c, "shipping", err)
		return
	}
	if resp.GetCode() != 0 {
		response.GrpcResult(c, resp.GetCode(), resp.GetMessage(), nil)
		return
	}

	data := resp.GetData()
	records := make([]AdminSellerShipmentSettingSummaryResp, 0, len(data.GetRecords()))
	for _, r := range data.GetRecords() {
		records = append(records, AdminSellerShipmentSettingSummaryResp{
			SettingId:          r.GetSettingId(),
			StoreId:            r.GetStoreId(),
			ShippingType:       r.GetShippingType(),
			GoodsType:          r.GetGoodsType(),
			OnboardingStatus:   r.GetOnboardingStatus(),
			ProviderStatus:     r.GetProviderStatus(),
			ProviderMessage:    r.GetProviderMessage(),
			MerId:              r.GetMerId(),
			PartnerId:          r.GetPartnerId(),
			EshipId:            r.GetEshipId(),
			PayuniSubmitStatus: r.GetPayuniSubmitStatus(),
			Enabled:            r.GetEnabled(),
			CreatedAt:          r.GetCreatedAt(),
			UpdatedAt:          r.GetUpdatedAt(),
		})
	}
	response.OK(c, AdminSellerShipmentSettingListResp{
		Records:  records,
		Total:    data.GetTotal(),
		Page:     data.GetPage(),
		PageSize: data.GetPageSize(),
	})
}
