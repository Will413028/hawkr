package handler

import (
	"github.com/Will413028/hawkr/gateway/internal/response"
	pb "github.com/Will413028/hawkr/go-pkg/proto/shipping"
	"github.com/gin-gonic/gin"
)

// ==================== DTO ====================

// PrintCvsLabelReq 超商出貨單列印請求
type PrintCvsLabelReq struct {
	ShipDate string `json:"shipDate"` // 出貨日期，接受 yyyy-MM-dd 或 yyyyMMdd（首次列印必填，重複列印可不傳）
}

// ConfirmCvsShipmentReq 超商確認出貨請求
type ConfirmCvsShipmentReq struct {
	StagedTokens []string `json:"stagedTokens"` // 出貨照片 stagedToken（選填）
}

// GenerateHomeLabelReq 宅配取號 PDF 請求
type GenerateHomeLabelReq struct {
	ShipDate     string   `json:"shipDate" binding:"required"` // 出貨日期，接受 yyyy-MM-dd 或 yyyyMMdd
	DeliveryDate string   `json:"deliveryDate"`                // 選填（接受 yyyy-MM-dd 或 yyyyMMdd），未填則後端自動補 shipDate + 1 工作天
	StagedTokens []string `json:"stagedTokens"`                // 出貨照片 stagedToken（選填）
}

// CallTcatReq 呼叫黑貓預約取件請求
type CallTcatReq struct {
	IsContact string `json:"isContact"` // 是否需要事先電聯 Y/N（選填，預設 Y）
	IsTrolley string `json:"isTrolley"` // 是否需要推車 Y/N（選填，預設 N）
	Memo      string `json:"memo"`      // 給宅配人員備註（選填，最長 100 位）
}

// CvsLabelDataResp 超商出貨單列印回應（同步等待 Webhook 最多 55 秒）
type CvsLabelDataResp struct {
	ShipmentNo      string `json:"shipmentNo"`
	PartnerId       string `json:"partnerId"`       // 出貨便母代碼（3 碼，超時為空）
	Odno            string `json:"odno"`            // 出貨單編號（8 碼，超時為空）
	ValidationNo    string `json:"validationNo"`    // C2C 驗證碼（4 碼，超時為空）
	WebhookReceived bool   `json:"webhookReceived"` // 是否在等待期內收到 Webhook
}

// BatchPrintCvsLabelReq 批次超商出貨單列印請求
type BatchPrintCvsLabelReq struct {
	ShipDate string   `json:"shipDate" binding:"required"` // 出貨日期，接受 yyyy-MM-dd 或 yyyyMMdd
	OrderNos []string `json:"orderNos" binding:"required,min=1,max=50"`
}

// BatchCvsLabelDataResp 批次超商出貨單列印回應
type BatchCvsLabelDataResp struct {
	TotalCount   string                    `json:"totalCount"`
	SuccessCount string                    `json:"successCount"`
	Results      []BatchCvsLabelResultResp `json:"results"`
}

// BatchCvsLabelResultResp 單筆列印結果
type BatchCvsLabelResultResp struct {
	OrderNo    string `json:"orderNo"`
	ShipmentNo string `json:"shipmentNo"`
	Message    string `json:"message"`
	Success    bool   `json:"success"`
}

// HomeLabelDataResp 宅配託運單 PDF 回應
type HomeLabelDataResp struct {
	LabelKey   string `json:"labelKey"` // 託運單 PDF objectKey（SSOT；49 §6.1）
	LabelUrl   string `json:"labelUrl"` // 託運單 PDF 顯示 URL（Gateway 組）
	ShipmentNo string `json:"shipmentNo"`
}

// BatchHomeLabelItemReq 批次宅配取號單筆項目
type BatchHomeLabelItemReq struct {
	OrderNo      string `json:"orderNo" binding:"required"`
	ShipDate     string `json:"shipDate" binding:"required"` // 出貨日期，接受 yyyy-MM-dd 或 yyyyMMdd
	DeliveryDate string `json:"deliveryDate"`                // 選填（接受 yyyy-MM-dd 或 yyyyMMdd），未填則後端自動補 shipDate + 1 工作天
}

// BatchGenerateHomeLabelReq 批次宅配取號 PDF 請求（單次上限由後端 SHIPPING_BATCH_MAX_SIZE 控制）
type BatchGenerateHomeLabelReq struct {
	Items []BatchHomeLabelItemReq `json:"items" binding:"required,min=1,dive"`
}

// BatchHomeLabelDataResp 批次宅配取號 PDF 回應
type BatchHomeLabelDataResp struct {
	TotalCount   string                     `json:"totalCount"`
	SuccessCount string                     `json:"successCount"`
	Results      []BatchHomeLabelResultResp `json:"results"`
}

// BatchHomeLabelResultResp 批次宅配取號單筆結果
type BatchHomeLabelResultResp struct {
	OrderNo    string `json:"orderNo"`
	ShipmentNo string `json:"shipmentNo"`
	LabelKey   string `json:"labelKey"` // 託運單 PDF objectKey（SSOT；49 §6.1）
	LabelUrl   string `json:"labelUrl"` // 託運單 PDF 顯示 URL（Gateway 組）
	Message    string `json:"message"`
	Success    bool   `json:"success"`
}

// ImportHolidaysReq 匯入假日請求
type ImportHolidaysReq struct {
	CsvUrl string `json:"csvUrl" binding:"required"` // 政府 CSV 下載 URL
}

// ImportHolidaysDataResp 匯入假日回應
type ImportHolidaysDataResp struct {
	Year          string `json:"year"`
	ImportedCount string `json:"importedCount"`
	DeletedCount  string `json:"deletedCount"`
}

// ImportHolidaysFromContentReq 由 CSV 內容匯入假日請求（檔案上傳版）
type ImportHolidaysFromContentReq struct {
	Year       string `json:"year" binding:"required"`       // 顯式宣告年份（覆寫防護，取代隱式取 CSV 首筆）
	CsvContent string `json:"csvContent" binding:"required"` // 前端 FileReader 讀取的 CSV 全文
}

// HolidayYearCoverageResp 單一年度假日覆蓋現況
type HolidayYearCoverageResp struct {
	Year           string `json:"year"`           // 年份
	Count          string `json:"count"`          // 該年筆數
	MaxHolidayDate string `json:"maxHolidayDate"` // 該年最大假日日期（YYYY-MM-DD）
}

// HolidayCoverageDataResp 假日覆蓋現況回應
type HolidayCoverageDataResp struct {
	CoverageDays string                    `json:"coverageDays"` // 距耗盡天數（與監測 metric 同源公式）；空表 "-1"
	Years        []HolidayYearCoverageResp `json:"years"`        // 各年度覆蓋狀態
}

// BusinessDayResp 工作天
type BusinessDayResp struct {
	Date      string `json:"date"`      // YYYY-MM-DD
	DayOfWeek string `json:"dayOfWeek"` // 一/二/三/四/五
}

// BusinessDaysDataResp 工作天列表回應
type BusinessDaysDataResp struct {
	BusinessDays []BusinessDayResp `json:"businessDays"`
}

// CallTcatDataResp 呼叫黑貓回應
type CallTcatDataResp struct {
	CallTime   string `json:"callTime"`
	ShipmentNo string `json:"shipmentNo"`
}

// OnboardCvsReq 超商建店請求
type OnboardCvsReq struct {
	ReturnStoreCode    string `json:"returnStoreCode" binding:"required"`
	ReturnStoreName    string `json:"returnStoreName"`
	ReturnStoreAddress string `json:"returnStoreAddress"`
	SenderName         string `json:"senderName" binding:"required"`
	SenderPhone        string `json:"senderPhone" binding:"required"`
}

// OnboardHomeReq 宅配建店請求
type OnboardHomeReq struct {
	ProductCategory     string `json:"productCategory" binding:"required"`
	SenderName          string `json:"senderName" binding:"required"`
	SenderPhone         string `json:"senderPhone" binding:"required"`
	SenderCity          string `json:"senderCity" binding:"required"`
	SenderDistrict      string `json:"senderDistrict" binding:"required"`
	SenderAddressDetail string `json:"senderAddressDetail" binding:"required"`
	SenderPostalCode    string `json:"senderPostalCode" binding:"required"`
	ReturnName          string `json:"returnName"`
	ReturnPhone         string `json:"returnPhone"`
	ReturnCity          string `json:"returnCity"`
	ReturnDistrict      string `json:"returnDistrict"`
	ReturnAddressDetail string `json:"returnAddressDetail"`
	ReturnPostalCode    string `json:"returnPostalCode"`
}

// ToggleShippingOnboardingReq 切換建店設定啟用/停用請求
type ToggleShippingOnboardingReq struct {
	Enabled bool `json:"enabled"` // true=啟用, false=停用
}

// ShippingOnboardingViewResp 建店設定回應
type ShippingOnboardingViewResp struct {
	OnboardingId        string `json:"onboardingId"`
	ShippingType        string `json:"shippingType"`     // 物流類型（CVS_C2C=超商店到店, HOME_DELIVERY=宅配）
	OnboardingStatus    string `json:"onboardingStatus"` // 建店狀態碼
	ProviderStatus      string `json:"providerStatus"`   // PayUni 原始啟用狀態（收斂規範 §4.1 SoT；欄位原名 setStatus 已改名）
	ProviderMessage     string `json:"providerMessage"`  // PayUni 原始訊息
	PartnerId           string `json:"partnerId"`
	EshipId             string `json:"eshipId"`
	ReturnStoreCode     string `json:"returnStoreCode"`    // 退貨門市代碼（超商才有）
	ReturnStoreName     string `json:"returnStoreName"`    // 退貨門市名稱（超商才有）
	ReturnStoreAddress  string `json:"returnStoreAddress"` // 退貨門市地址（超商才有）
	ProductCategory     string `json:"productCategory"`
	LabelCompanyName    string `json:"labelCompanyName"`
	Enabled             string `json:"enabled"`            // "true" / "false"
	PayuniSubmitStatus  string `json:"payuniSubmitStatus"` // PayUniSubmitStatus value: "4301"~"4304"；前端依 SUCCEEDED 判斷宅配地址 readonly
	SenderName          string `json:"senderName"`
	SenderPhone         string `json:"senderPhone"`
	SenderCity          string `json:"senderCity"`
	SenderDistrict      string `json:"senderDistrict"`
	SenderAddressDetail string `json:"senderAddressDetail"`
	SenderPostalCode    string `json:"senderPostalCode"`
	ReturnSenderName    string `json:"returnName"`
	ReturnSenderPhone   string `json:"returnPhone"`
	ReturnCity          string `json:"returnCity"`
	ReturnDistrict      string `json:"returnDistrict"`
	ReturnAddressDetail string `json:"returnAddressDetail"`
	ReturnPostalCode    string `json:"returnPostalCode"`
	CreatedAt           string `json:"createdAt"`
	UpdatedAt           string `json:"updatedAt"`
}

// ==================== Helper ====================

// ==================== Handler ====================

// PrintCvsLabel 超商出貨單列印
// @Summary      超商出貨單列印
// @Description  賣家對已建立的超商物流單觸發 PayUni 列印，同步等待最多 55 秒取得交貨便代碼。若超時則回傳 webhookReceived=false，前端可在訂單詳情頁後續查詢。
// @Tags         物流 (Shipping)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        order_no  path  string  true  "訂單編號"
// @Param        body      body  PrintCvsLabelReq  true  "出貨日期（yyyy-MM-dd 或 yyyyMMdd）"
// @Success      200  {object}  object{code=int,data=CvsLabelDataResp}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /app/shipments/{order_no}/print_cvs_label [post]
func (h *Holder) PrintCvsLabel(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	orderNo := c.Param("order_no")
	if orderNo == "" {
		response.BadRequest(c, "order_no is required")
		return
	}

	var req PrintCvsLabelReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	resp, err := h.Shipping.PrintCvsLabel(c.Request.Context(), &pb.PrintCvsLabelRequest{
		StoreId:  int64(storeID),
		OrderNo:  orderNo,
		ShipDate: req.ShipDate,
	})
	if err != nil {
		response.GrpcErr(c, "shipping", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}
	data := CvsLabelDataResp{
		ShipmentNo:      resp.Data.GetShipmentNo(),
		PartnerId:       resp.Data.GetPartnerId(),
		Odno:            resp.Data.GetOdno(),
		ValidationNo:    resp.Data.GetValidationNo(),
		WebhookReceived: resp.Data.GetWebhookReceived(),
	}
	response.GrpcResult(c, resp.Code, resp.Message, data)
}

// BatchPrintCvsLabel 批次超商出貨單列印
// @Summary      批次超商出貨單列印
// @Description  賣家一次觸發多筆超商物流單的 PayUni 列印，同批次自動依溫層/物流商分組呼叫。出貨便代碼由 Webhook 逐筆非同步回填。單次最多 50 筆，採部分成功模式。
// @Tags         物流 (Shipping)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  BatchPrintCvsLabelReq  true  "訂單編號列表與出貨日期（yyyy-MM-dd 或 yyyyMMdd）"
// @Success      200  {object}  object{code=int,data=BatchCvsLabelDataResp}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /app/shipments/batch_print_cvs_label [post]
func (h *Holder) BatchPrintCvsLabel(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	var req BatchPrintCvsLabelReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	resp, err := h.Shipping.BatchPrintCvsLabel(c.Request.Context(), &pb.BatchPrintCvsLabelRequest{
		StoreId:  int64(storeID),
		OrderNos: req.OrderNos,
		ShipDate: req.ShipDate,
	})
	if err != nil {
		response.GrpcErr(c, "shipping", err)
		return
	}

	results := make([]BatchCvsLabelResultResp, 0, len(resp.Data.Results))
	for _, r := range resp.Data.Results {
		results = append(results, BatchCvsLabelResultResp{
			OrderNo:    r.GetOrderNo(),
			ShipmentNo: r.GetShipmentNo(),
			Success:    r.GetSuccess(),
			Message:    r.GetMessage(),
		})
	}

	data := BatchCvsLabelDataResp{
		TotalCount:   resp.Data.GetTotalCount(),
		SuccessCount: resp.Data.GetSuccessCount(),
		Results:      results,
	}
	response.GrpcResult(c, resp.Code, resp.Message, data)
}

// BatchGenerateHomeLabel 批次宅配取號 PDF
// @Summary      批次宅配取號 PDF
// @Description  賣家一次觸發多筆宅配物流單取號 + PDF 上傳 OSS。後端 fan-out 並行（Virtual Threads + Semaphore），同步等待全部完成。單次上限與並行數由環境變數控制（預設 50 / 10）。採部分成功模式，每筆獨立回傳結果。**不接受出貨照片**（請改用單筆 API）。
// @Tags         物流 (Shipping)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  BatchGenerateHomeLabelReq  true  "批次宅配取號項目（每筆含 orderNo + shipDate + deliveryDate?）"
// @Success      200  {object}  object{code=int,data=BatchHomeLabelDataResp}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /app/shipments/batch_generate_home_label [post]
func (h *Holder) BatchGenerateHomeLabel(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	var req BatchGenerateHomeLabelReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	items := make([]*pb.BatchHomeLabelItem, 0, len(req.Items))
	for _, it := range req.Items {
		items = append(items, &pb.BatchHomeLabelItem{
			OrderNo:      it.OrderNo,
			ShipDate:     it.ShipDate,
			DeliveryDate: it.DeliveryDate,
		})
	}

	resp, err := h.Shipping.BatchGenerateHomeLabelPdf(c.Request.Context(), &pb.BatchGenerateHomeLabelPdfRequest{
		StoreId: int64(storeID),
		Items:   items,
	})
	if err != nil {
		response.GrpcErr(c, "shipping", err)
		return
	}

	results := make([]BatchHomeLabelResultResp, 0, len(resp.Data.Results))
	for _, r := range resp.Data.Results {
		results = append(results, BatchHomeLabelResultResp{
			OrderNo:    r.GetOrderNo(),
			ShipmentNo: r.GetShipmentNo(),
			LabelKey:   r.GetLabelKey(),
			LabelUrl:   buildURL(h.MediaBaseURL, r.GetLabelKey()),
			Success:    r.GetSuccess(),
			Message:    r.GetMessage(),
		})
	}

	data := BatchHomeLabelDataResp{
		TotalCount:   resp.Data.GetTotalCount(),
		SuccessCount: resp.Data.GetSuccessCount(),
		Results:      results,
	}
	response.GrpcResult(c, resp.Code, resp.Message, data)
}

// GenerateHomeLabel 產宅配託運單 PDF
// @Summary      產宅配託運單 PDF
// @Description  賣家對已建立的宅配物流單產生託運單 PDF，後端呼叫 PayUni 取號並上傳 CDN，回傳 PDF URL
// @Tags         物流 (Shipping)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        order_no  path  string  true  "訂單編號"
// @Param        body      body  GenerateHomeLabelReq  true  "出貨日期與配達日期（yyyy-MM-dd 或 yyyyMMdd）"
// @Success      200  {object}  object{code=int,data=HomeLabelDataResp}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /app/shipments/{order_no}/generate_home_label [post]
func (h *Holder) GenerateHomeLabel(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	orderNo := c.Param("order_no")
	if orderNo == "" {
		response.BadRequest(c, "order_no is required")
		return
	}

	var req GenerateHomeLabelReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	resp, err := h.Shipping.GenerateHomeLabelPdf(c.Request.Context(), &pb.GenerateHomeLabelPdfRequest{
		StoreId:      int64(storeID),
		OrderNo:      orderNo,
		ShipDate:     req.ShipDate,
		DeliveryDate: req.DeliveryDate,
		StagedTokens: req.StagedTokens,
	})
	if err != nil {
		response.GrpcErr(c, "shipping", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}
	data := HomeLabelDataResp{
		LabelKey:   resp.Data.GetLabelKey(),
		LabelUrl:   buildURL(h.MediaBaseURL, resp.Data.GetLabelKey()),
		ShipmentNo: resp.Data.GetShipmentNo(),
	}
	response.GrpcResult(c, resp.Code, resp.Message, data)
}

// CallTcat 呼叫黑貓預約取件
// @Summary      呼叫黑貓預約取件
// @Description  賣家對已取號的宅配物流單呼叫黑貓預約取件，需先完成 generate_home_label 取號。取件選項會存入 DB 再傳給 PayUni。
// @Tags         物流 (Shipping)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        order_no  path  string  true  "訂單編號"
// @Param        body      body  CallTcatReq  false  "預約取件選項（全部選填）"
// @Success      200  {object}  object{code=int,data=CallTcatDataResp}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /app/shipments/{order_no}/call_tcat [post]
func (h *Holder) CallTcat(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	orderNo := c.Param("order_no")
	if orderNo == "" {
		response.BadRequest(c, "order_no is required")
		return
	}

	var req CallTcatReq
	// CallTcat 全部欄位選填，允許空 body
	_ = c.ShouldBindJSON(&req)

	resp, err := h.Shipping.CallTcat(c.Request.Context(), &pb.CallTcatRequest{
		StoreId:   int64(storeID),
		OrderNo:   orderNo,
		IsContact: req.IsContact,
		IsTrolley: req.IsTrolley,
		Memo:      req.Memo,
	})
	if err != nil {
		response.GrpcErr(c, "shipping", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}
	data := CallTcatDataResp{
		CallTime:   resp.Data.GetCallTime(),
		ShipmentNo: resp.Data.GetShipmentNo(),
	}
	response.GrpcResult(c, resp.Code, resp.Message, data)
}

// ==================== 建店 Handler ====================

// OnboardCvsShipping 超商 C2C 建店
// @Summary      超商 C2C 建店
// @Description  賣家申請超商 C2C 物流建店（7-ELEVEN），呼叫 PayUni 建店 API 取得物流代碼
// @Tags         物流 (Shipping)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  OnboardCvsReq  true  "returnStoreCode: 6碼退貨門市，senderName: 寄件人姓名，senderPhone: 寄件人電話"
// @Success      200  {object}  object{code=int,message=string}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /app/shipping_onboardings/cvs [post]
func (h *Holder) OnboardCvsShipping(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	var req OnboardCvsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body: returnStoreCode, senderName and senderPhone are required")
		return
	}

	resp, err := h.Shipping.OnboardCvsShipping(c.Request.Context(), &pb.OnboardCvsShippingRequest{
		StoreId:            int64(storeID),
		GoodsType:          ambientShippingTemperature,
		ReturnStoreCode:    req.ReturnStoreCode,
		ReturnStoreName:    req.ReturnStoreName,
		ReturnStoreAddress: req.ReturnStoreAddress,
		SenderName:         req.SenderName,
		SenderPhone:        req.SenderPhone,
	})
	if err != nil {
		response.GrpcErr(c, "shipping", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, nil)
}

// OnboardHomeDeliveryShipping 宅配建店
// @Summary      宅配建店
// @Description  賣家申請宅配物流建店（黑貓），呼叫 PayUni 建店 API 取得物流代碼
// @Tags         物流 (Shipping)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  OnboardHomeReq  true  "商品類別代碼（0001~0015）+ 寄件人/退貨地址資訊"
// @Success      200  {object}  object{code=int,message=string}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /app/shipping_onboardings/home_delivery [post]
func (h *Holder) OnboardHomeDeliveryShipping(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	var req OnboardHomeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body: productCategory, senderName, senderPhone, senderCity, senderDistrict, senderAddressDetail and senderPostalCode are required")
		return
	}

	resp, err := h.Shipping.OnboardHomeDeliveryShipping(c.Request.Context(), &pb.OnboardHomeDeliveryRequest{
		StoreId:             int64(storeID),
		ProductCategory:     req.ProductCategory,
		SenderName:          req.SenderName,
		SenderPhone:         req.SenderPhone,
		SenderCity:          req.SenderCity,
		SenderDistrict:      req.SenderDistrict,
		SenderAddressDetail: req.SenderAddressDetail,
		SenderPostalCode:    req.SenderPostalCode,
		ReturnName:          req.ReturnName,
		ReturnPhone:         req.ReturnPhone,
		ReturnCity:          req.ReturnCity,
		ReturnDistrict:      req.ReturnDistrict,
		ReturnAddressDetail: req.ReturnAddressDetail,
		ReturnPostalCode:    req.ReturnPostalCode,
	})
	if err != nil {
		response.GrpcErr(c, "shipping", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, nil)
}

// ListShippingOnboardings 列出建店設定
// @Summary      列出建店設定
// @Description  賣家取得自己所有物流建店設定（超商/宅配）
// @Tags         物流 (Shipping)
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  object{code=int,data=[]ShippingOnboardingViewResp}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /app/shipping_onboardings [get]
func (h *Holder) ListShippingOnboardings(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	resp, err := h.Shipping.ListShippingOnboardings(c.Request.Context(), &pb.ListShippingOnboardingsRequest{
		StoreId: int64(storeID),
	})
	if err != nil {
		response.GrpcErr(c, "shipping", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	var items []ShippingOnboardingViewResp
	for _, v := range resp.Onboardings {
		if !isAmbientOrUntyped(v.GetGoodsType()) {
			continue
		}
		items = append(items, convertOnboardingView(v))
	}
	response.GrpcResult(c, resp.Code, resp.Message, items)
}

// GetShippingOnboarding 查詢建店詳情
// @Summary      查詢建店詳情
// @Description  賣家查詢單一物流建店設定詳情
// @Tags         物流 (Shipping)
// @Produce      json
// @Security     BearerAuth
// @Param        onboarding_id  path  string  true  "建店設定 ID"
// @Success      200  {object}  object{code=int,data=ShippingOnboardingViewResp}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /app/shipping_onboardings/{onboarding_id} [get]
func (h *Holder) GetShippingOnboarding(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	onboardingID := c.Param("onboarding_id")
	if onboardingID == "" {
		response.BadRequest(c, "onboarding_id is required")
		return
	}

	resp, err := h.Shipping.GetShippingOnboarding(c.Request.Context(), &pb.GetShippingOnboardingRequest{
		StoreId:      int64(storeID),
		OnboardingId: onboardingID,
	})
	if err != nil {
		response.GrpcErr(c, "shipping", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, convertOnboardingView(resp.Data))
}

// ToggleShippingOnboarding 切換建店設定啟用/停用
// @Summary      切換建店設定啟用/停用
// @Description  賣家切換物流建店設定的啟用/停用狀態，純平台內部操作，不與 PayUni 溝通
// @Tags         物流 (Shipping)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        onboarding_id  path  string                       true  "建店設定 ID"
// @Param        body           body  ToggleShippingOnboardingReq  true  "啟用/停用"
// @Success      200  {object}  object{code=int,message=string}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /app/shipping_onboardings/{onboarding_id}/toggle [post]
func (h *Holder) ToggleShippingOnboarding(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	onboardingID := c.Param("onboarding_id")
	if onboardingID == "" {
		response.BadRequest(c, "onboarding_id is required")
		return
	}

	var req ToggleShippingOnboardingReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	resp, err := h.Shipping.ToggleShippingOnboarding(c.Request.Context(), &pb.ToggleShippingOnboardingRequest{
		StoreId:      int64(storeID),
		OnboardingId: onboardingID,
		Enabled:      req.Enabled,
	})
	if err != nil {
		response.GrpcErr(c, "shipping", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, nil)
}

// ==================== 賣家配送方式 DTO ====================

// SellerShippingMethodResp 賣家配送方式回應
type SellerShippingMethodResp struct {
	ShippingMethod     string `json:"shippingMethod"`
	DisplayName        string `json:"displayName"`
	Description        string `json:"description"`
	OnboardingStatus   string `json:"onboardingStatus"`
	OnboardingId       string `json:"onboardingId"`
	Enabled            string `json:"enabled"` // "true" / "false" / ""（未建店）
	Available          bool   `json:"available"`
	OnboardingRequired bool   `json:"onboardingRequired"`
}

// ==================== 賣家配送方式 Handler ====================

// ListSellerShippingMethods 查詢賣家可用配送方式
// @Summary      查詢賣家可用配送方式
// @Description  回傳賣家所有可能的配送方式槽位（宅配/超商），含各自的啟用狀態與建店資訊
// @Tags         物流 (Shipping)
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  object{code=int,data=[]SellerShippingMethodResp}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /app/shipping_methods [get]
func (h *Holder) ListSellerShippingMethods(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}
	storeID, ok := h.resolveStoreIDFor(c, accountID)
	if !ok {
		return
	}

	accountType := getAccountType(c)

	resp, err := h.Shipping.ListSellerShippingMethods(c.Request.Context(), &pb.ListSellerShippingMethodsRequest{
		StoreId:     int64(storeID),
		AccountType: int32(accountType),
	})
	if err != nil {
		response.GrpcErr(c, "shipping", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	var items []SellerShippingMethodResp
	for _, m := range resp.Methods {
		if !isAmbientOrUntyped(m.GetGoodsType()) {
			continue
		}
		items = append(items, SellerShippingMethodResp{
			ShippingMethod:     m.GetShippingMethod(),
			DisplayName:        m.GetDisplayName(),
			Description:        m.GetDescription(),
			Available:          m.GetAvailable(),
			OnboardingRequired: m.GetOnboardingRequired(),
			OnboardingStatus:   m.GetOnboardingStatus(),
			OnboardingId:       m.GetOnboardingId(),
			Enabled:            m.GetEnabled(),
		})
	}
	response.GrpcResult(c, resp.Code, resp.Message, items)
}

// ==================== 買家常用門市 DTO ====================

// AddStorePickupReq 新增常用門市請求
type AddStorePickupReq struct {
	StoreCode     string `json:"storeCode" binding:"required"`
	StoreName     string `json:"storeName" binding:"required"`
	StoreAddress  string `json:"storeAddress" binding:"required"`
	ReceiverName  string `json:"receiverName" binding:"required"`
	ReceiverPhone string `json:"receiverPhone" binding:"required"`
	IsDefault     bool   `json:"isDefault"`
}

// UpdateStorePickupReq 修改常用門市請求
type UpdateStorePickupReq struct {
	StoreCode     string `json:"storeCode" binding:"required"`
	StoreName     string `json:"storeName" binding:"required"`
	StoreAddress  string `json:"storeAddress" binding:"required"`
	ReceiverName  string `json:"receiverName" binding:"required"`
	ReceiverPhone string `json:"receiverPhone" binding:"required"`
	IsDefault     bool   `json:"isDefault"`
}

// BuyerStorePickupResp 買家常用門市回應
type BuyerStorePickupResp struct {
	Id            string `json:"id"`
	StoreCode     string `json:"storeCode"`
	StoreName     string `json:"storeName"`
	StoreAddress  string `json:"storeAddress"`
	ReceiverName  string `json:"receiverName"`
	ReceiverPhone string `json:"receiverPhone"`
	IsDefault     string `json:"isDefault"` // "true" / "false"
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
}

// ==================== 買家常用門市 Handler ====================

// AddBuyerStorePickup 新增常用門市
// @Summary      新增常用門市
// @Description  買家新增常用取貨門市，每人最多 20 筆。設為預設時自動清除其他預設門市。
// @Tags         物流 (Shipping)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  AddStorePickupReq  true  "門市資訊"
// @Success      200  {object}  object{code=int,data=BuyerStorePickupResp}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /app/store_pickups [post]
func (h *Holder) AddBuyerStorePickup(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}
	buyerID, ok := h.resolveBuyerIDFor(c, accountID)
	if !ok {
		return
	}

	var req AddStorePickupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	resp, err := h.Shipping.AddBuyerStorePickup(c.Request.Context(), &pb.AddBuyerStorePickupRequest{
		BuyerId:       int64(buyerID),
		StoreCode:     req.StoreCode,
		StoreName:     req.StoreName,
		StoreAddress:  req.StoreAddress,
		ReceiverName:  req.ReceiverName,
		ReceiverPhone: req.ReceiverPhone,
		IsDefault:     req.IsDefault,
	})
	if err != nil {
		response.GrpcErr(c, "shipping", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, convertPickupView(resp.Data))
}

// UpdateBuyerStorePickup 修改常用門市
// @Summary      修改常用門市
// @Description  買家修改已儲存的常用取貨門市。設為預設時自動清除其他預設門市。
// @Tags         物流 (Shipping)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        store_pickup_id  path  string  true  "門市記錄 ID"
// @Param        body             body  UpdateStorePickupReq  true  "門市資訊"
// @Success      200  {object}  object{code=int,data=BuyerStorePickupResp}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /app/store_pickups/{store_pickup_id} [put]
func (h *Holder) UpdateBuyerStorePickup(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}
	buyerID, ok := h.resolveBuyerIDFor(c, accountID)
	if !ok {
		return
	}

	storePickupID := c.Param("store_pickup_id")
	if storePickupID == "" {
		response.BadRequest(c, "store_pickup_id is required")
		return
	}

	var req UpdateStorePickupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	resp, err := h.Shipping.UpdateBuyerStorePickup(c.Request.Context(), &pb.UpdateBuyerStorePickupRequest{
		BuyerId:       int64(buyerID),
		StorePickupId: storePickupID,
		StoreCode:     req.StoreCode,
		StoreName:     req.StoreName,
		StoreAddress:  req.StoreAddress,
		ReceiverName:  req.ReceiverName,
		ReceiverPhone: req.ReceiverPhone,
		IsDefault:     req.IsDefault,
	})
	if err != nil {
		response.GrpcErr(c, "shipping", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, convertPickupView(resp.Data))
}

// ListBuyerStorePickups 取得常用門市列表
// @Summary      取得常用門市列表
// @Description  買家取得自己所有常用取貨門市，依預設優先、建立時間降序排列
// @Tags         物流 (Shipping)
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  object{code=int,data=[]BuyerStorePickupResp}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /app/store_pickups [get]
func (h *Holder) ListBuyerStorePickups(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}
	buyerID, ok := h.resolveBuyerIDFor(c, accountID)
	if !ok {
		return
	}

	resp, err := h.Shipping.ListBuyerStorePickups(c.Request.Context(), &pb.ListBuyerStorePickupsRequest{
		BuyerId: int64(buyerID),
	})
	if err != nil {
		response.GrpcErr(c, "shipping", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	var items []BuyerStorePickupResp
	for _, v := range resp.Data {
		items = append(items, convertPickupView(v))
	}
	response.GrpcResult(c, resp.Code, resp.Message, items)
}

// DeleteBuyerStorePickup 刪除常用門市
// @Summary      刪除常用門市
// @Description  買家刪除已儲存的常用取貨門市
// @Tags         物流 (Shipping)
// @Produce      json
// @Security     BearerAuth
// @Param        store_pickup_id  path  string  true  "門市記錄 ID"
// @Success      200  {object}  object{code=int,message=string}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /app/store_pickups/{store_pickup_id} [delete]
func (h *Holder) DeleteBuyerStorePickup(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}
	buyerID, ok := h.resolveBuyerIDFor(c, accountID)
	if !ok {
		return
	}

	storePickupID := c.Param("store_pickup_id")
	if storePickupID == "" {
		response.BadRequest(c, "store_pickup_id is required")
		return
	}

	resp, err := h.Shipping.DeleteBuyerStorePickup(c.Request.Context(), &pb.DeleteBuyerStorePickupRequest{
		BuyerId:       int64(buyerID),
		StorePickupId: storePickupID,
	})
	if err != nil {
		response.GrpcErr(c, "shipping", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, nil)
}

// convertPickupView 轉換 Proto 常用門市為回應 DTO
func convertPickupView(v *pb.BuyerStorePickupView) BuyerStorePickupResp {
	if v == nil {
		return BuyerStorePickupResp{}
	}
	return BuyerStorePickupResp{
		Id:            v.GetId(),
		StoreCode:     v.GetStoreCode(),
		StoreName:     v.GetStoreName(),
		StoreAddress:  v.GetStoreAddress(),
		ReceiverName:  v.GetReceiverName(),
		ReceiverPhone: v.GetReceiverPhone(),
		IsDefault:     v.GetIsDefault(),
		CreatedAt:     v.GetCreatedAt(),
		UpdatedAt:     v.GetUpdatedAt(),
	}
}

// convertOnboardingView 轉換 Proto 建店設定為回應 DTO
func convertOnboardingView(v *pb.ShippingOnboardingView) ShippingOnboardingViewResp {
	if v == nil {
		return ShippingOnboardingViewResp{}
	}
	return ShippingOnboardingViewResp{
		OnboardingId:        v.GetOnboardingId(),
		ShippingType:        v.GetShippingType(),
		OnboardingStatus:    v.GetOnboardingStatus(),
		ProviderStatus:      v.GetProviderStatus(),
		ProviderMessage:     v.GetProviderMessage(),
		PartnerId:           v.GetPartnerId(),
		EshipId:             v.GetEshipId(),
		ReturnStoreCode:     v.GetReturnStoreCode(),
		ReturnStoreName:     v.GetReturnStoreName(),
		ReturnStoreAddress:  v.GetReturnStoreAddress(),
		ProductCategory:     v.GetProductCategory(),
		LabelCompanyName:    v.GetLabelCompanyName(),
		Enabled:             v.GetEnabled(),
		PayuniSubmitStatus:  v.GetPayuniSubmitStatus(),
		SenderName:          v.GetSenderName(),
		SenderPhone:         v.GetSenderPhone(),
		SenderCity:          v.GetSenderCity(),
		SenderDistrict:      v.GetSenderDistrict(),
		SenderAddressDetail: v.GetSenderAddressDetail(),
		SenderPostalCode:    v.GetSenderPostalCode(),
		ReturnSenderName:    v.GetReturnName(),
		ReturnSenderPhone:   v.GetReturnPhone(),
		ReturnCity:          v.GetReturnCity(),
		ReturnDistrict:      v.GetReturnDistrict(),
		ReturnAddressDetail: v.GetReturnAddressDetail(),
		ReturnPostalCode:    v.GetReturnPostalCode(),
		CreatedAt:           v.GetCreatedAt(),
		UpdatedAt:           v.GetUpdatedAt(),
	}
}

// ==================== 假日管理 ====================

// ImportHolidays 匯入假日
// @Summary      匯入假日
// @Description  傳入政府 CSV URL，解析後寫入 DB（先刪除該年份舊資料再匯入）
// @Tags         假日管理 (Holiday)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  ImportHolidaysReq  true  "CSV URL"
// @Success      200  {object}  object{code=int,data=ImportHolidaysDataResp}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /admin/holidays/import [post]
func (h *Holder) ImportHolidays(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}

	var req ImportHolidaysReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "csvUrl is required")
		return
	}

	resp, err := h.Shipping.ImportHolidays(c.Request.Context(), &pb.ImportHolidaysRequest{
		AdminAccountId: accountID,
		CsvUrl:         req.CsvUrl,
	})
	if err != nil {
		response.GrpcErr(c, "shipping", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}
	data := ImportHolidaysDataResp{
		Year:          resp.Data.GetYear(),
		ImportedCount: resp.Data.GetImportedCount(),
		DeletedCount:  resp.Data.GetDeletedCount(),
	}
	response.GrpcResult(c, resp.Code, resp.Message, data)
}

// ImportHolidaysFromContent 由 CSV 內容匯入假日
// @Summary      由 CSV 內容匯入假日（檔案上傳版）
// @Description  前端讀取 CSV 檔成文字後傳入，搭配顯式宣告年份，過濾至該年並先刪除舊資料再匯入。回傳 importedCount / deletedCount 供二次確認。
// @Tags         假日管理 (Holiday)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  ImportHolidaysFromContentReq  true  "宣告年份與 CSV 內容"
// @Success      200  {object}  object{code=int,data=ImportHolidaysDataResp}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /admin/holidays/import_content [post]
func (h *Holder) ImportHolidaysFromContent(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}

	var req ImportHolidaysFromContentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "year and csvContent are required")
		return
	}

	resp, err := h.Shipping.ImportHolidaysFromContent(c.Request.Context(), &pb.ImportHolidaysFromContentRequest{
		AdminAccountId: accountID,
		Year:           req.Year,
		CsvContent:     req.CsvContent,
	})
	if err != nil {
		response.GrpcErr(c, "shipping", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}
	data := ImportHolidaysDataResp{
		Year:          resp.Data.GetYear(),
		ImportedCount: resp.Data.GetImportedCount(),
		DeletedCount:  resp.Data.GetDeletedCount(),
	}
	response.GrpcResult(c, resp.Code, resp.Message, data)
}

// GetHolidayCoverage 查詢假日覆蓋現況
// @Summary      查詢假日覆蓋現況
// @Description  回傳各年度假日筆數與最大假日日期，以及距耗盡天數（coverageDays，與監測 metric 同源公式；空表回 "-1"），供 admin 判斷是否需補匯入次年假日。
// @Tags         假日管理 (Holiday)
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  object{code=int,data=HolidayCoverageDataResp}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /admin/holidays/coverage [get]
func (h *Holder) GetHolidayCoverage(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}

	resp, err := h.Shipping.GetHolidayCoverage(c.Request.Context(), &pb.GetHolidayCoverageRequest{
		AdminAccountId: accountID,
	})
	if err != nil {
		response.GrpcErr(c, "shipping", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	years := make([]HolidayYearCoverageResp, 0, len(resp.Data.GetYears()))
	for _, y := range resp.Data.GetYears() {
		years = append(years, HolidayYearCoverageResp{
			Year:           y.GetYear(),
			Count:          y.GetCount(),
			MaxHolidayDate: y.GetMaxHolidayDate(),
		})
	}
	data := HolidayCoverageDataResp{
		CoverageDays: resp.Data.GetCoverageDays(),
		Years:        years,
	}
	response.GrpcResult(c, resp.Code, resp.Message, data)
}

// ListBusinessDays 查詢可出貨工作天
// @Summary      查詢可出貨工作天
// @Description  回傳明日起 7 個工作天，供前台日曆元件標記可選日期
// @Tags         物流 (Shipping)
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  object{code=int,data=BusinessDaysDataResp}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /app/shipping_business_days [get]
func (h *Holder) ListBusinessDays(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}

	resp, err := h.Shipping.ListBusinessDays(c.Request.Context(), &pb.ListBusinessDaysRequest{
		AccountId: accountID,
	})
	if err != nil {
		response.GrpcErr(c, "shipping", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	days := make([]BusinessDayResp, 0, len(resp.BusinessDays))
	for _, d := range resp.BusinessDays {
		days = append(days, BusinessDayResp{
			Date:      d.GetDate(),
			DayOfWeek: d.GetDayOfWeek(),
		})
	}
	response.GrpcResult(c, resp.Code, resp.Message, BusinessDaysDataResp{BusinessDays: days})
}

// ConfirmCvsShipment 超商確認出貨
// @Summary      超商確認出貨
// @Description  超商賣家確認出貨，推進訂單狀態為配送中。出貨照片選填。
// @Tags         物流 (Shipping)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        order_no  path  string  true  "訂單編號"
// @Param        body      body  ConfirmCvsShipmentReq  true  "確認出貨參數"
// @Success      200  {object}  object{code=int,message=string}
// @Failure      400  {object}  object{code=int,message=string}
// @Router       /app/shipments/{order_no}/confirm [post]
func (h *Holder) ConfirmCvsShipment(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	orderNo := c.Param("order_no")
	if orderNo == "" {
		response.BadRequest(c, "order_no is required")
		return
	}

	var req ConfirmCvsShipmentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		// body 選填，綁定失敗不阻擋
		req = ConfirmCvsShipmentReq{}
	}

	resp, err := h.Shipping.ConfirmCvsShipment(c.Request.Context(), &pb.ConfirmCvsShipmentRequest{
		StoreId:      int64(storeID),
		OrderNo:      orderNo,
		StagedTokens: req.StagedTokens,
	})
	if err != nil {
		response.GrpcErr(c, "shipping", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, nil)
}

// ==================== 買家物流追蹤 ====================

// OrderLogisticsResp 買家物流追蹤回應（單點狀態，PayUni 無多筆軌跡）
// 最小揭露：不含 recipientPhone / recipientAddress / labelUrl / sender 資訊
type OrderLogisticsResp struct {
	OrderNo         string `json:"orderNo"`
	ShipTradeNo     string `json:"shipTradeNo"`            // 託運單號（PayUni 物流序號）
	ShippingMethod  string `json:"shippingMethod"`         // CONVENIENCE_STORE / HOME_DELIVERY
	Status          string `json:"status"`                 // 內部物流狀態碼（數字字串）
	StatusLabel     string `json:"statusLabel"`            // 狀態人類可讀文案（ShippingStatus.label）
	ProviderStatus  string `json:"providerStatus"`         // PayUni 原始狀態碼
	ProviderMessage string `json:"providerMessage"`        // PayUni 狀態說明
	StatusTime      string `json:"statusTime"`             // 最新狀態更新時間（ISO 8601）
	StoreName       string `json:"storeName"`              // 超商門市名稱（超商配送）
	RecipientName   string `json:"recipientName"`          // 收件人姓名（買家本人）
	CreatedAt       string `json:"createdAt"`              // 物流單建立時間（ISO 8601）
	CvsStoreCode    string `json:"cvsStoreCode,omitempty"` // 超商門市代號（取代語意不誠實的 store_id）

	// AllowedActions 是「這筆配送單現在能做什麼」的唯一出口。
	// 🔴 前端**不得**自行從 status 推導可做什麼，也不得自行組 URL——
	// 後者同樣需要 provider 知識，而 href 由後端給就等於新增 provider 時前端零改動。
	// 業務判斷（能不能按、為什麼不能）來自 shipping-service；href + method 由本層補上。
	AllowedActions []AllowedAction `json:"allowedActions"`
}

// allowedActionsOf 把 shipping-service 算好的可用性補上路由，成為對外契約。
//
// 🔴 **後端還沒開始送時回空陣列而不是 nil**：JSON 的 `null` 與 `[]` 在前端是兩種不同的處理
// （`null` 常常讓 `.map()` 直接炸），而這個欄位在 provider capability 尚未全部搬移完成前
// 本來就可能是空的。回 `[]` 讓前端一律走同一條路徑。
//
// ⚠️ 被丟棄的 action（未登錄路由、或不可用卻沒帶原因碼）目前只是不出現在回應裡。
// 那是刻意的 fail-closed——猜一個路徑給前端打，症狀會出現在使用者面前而不是這裡。
// 但**丟棄本身沒有告警**：若某個 action 一直被丟棄，沒有人會知道。
// 要補的話是在這裡加一個 counter，而不是讓它通過。
func allowedActionsOf(d interface {
	GetAllowedActions() []*pb.ActionAvailability
	GetOrderNo() string
	GetShippingMethod() string
}, viewer string) []AllowedAction {
	src := d.GetAllowedActions()
	input := make([]ActionInput, 0, len(src))
	for _, a := range src {
		input = append(input, ActionInput{
			Action:             a.GetAction(),
			Enabled:            a.GetEnabled(),
			DisabledReasonCode: a.GetDisabledReasonCode(),
			ActorAccountTypes:  a.GetActorAccountTypes(),
		})
	}
	actions, _ := BuildAllowedActions(d.GetOrderNo(), d.GetShippingMethod(), viewer, input)
	if actions == nil {
		return []AllowedAction{}
	}
	return actions
}

// GetOrderLogistics 買家查看訂單物流追蹤
// @Summary      查看訂單物流追蹤
// @Description  買家查看自己訂單的物流追蹤，後端即時向 PayUni 同步當前狀態並回傳單點狀態。Ownership 由 shipping-service 驗證；無託運單號時回 DB 當前狀態。
// @Tags         物流 (Shipping)
// @Produce      json
// @Security     BearerAuth
// @Param        order_no  path  string  true  "訂單編號"
// @Success      200  {object}  object{code=int,data=OrderLogisticsResp}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /app/orders/{order_no}/logistics [get]
func (h *Holder) GetOrderLogistics(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}

	buyerID, ok := h.resolveBuyerIDFor(c, accountID)
	if !ok {
		return
	}

	orderNo := c.Param("order_no")
	if orderNo == "" {
		response.BadRequest(c, "order_no is required")
		return
	}

	resp, err := h.Shipping.GetShipmentTracking(c.Request.Context(), &pb.GetShipmentTrackingRequest{
		OrderNo: orderNo,
		BuyerId: int64(buyerID),
	})
	if err != nil {
		response.GrpcErr(c, "shipping", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	d := resp.Data
	data := OrderLogisticsResp{
		OrderNo:         d.GetOrderNo(),
		ShipTradeNo:     d.GetShipTradeNo(),
		ShippingMethod:  d.GetShippingMethod(),
		Status:          d.GetStatus(),
		StatusLabel:     d.GetStatusLabel(),
		ProviderStatus:  d.GetProviderStatus(),
		ProviderMessage: d.GetProviderMessage(),
		StatusTime:      d.GetStatusTime(),
		StoreName:       d.GetStoreName(),
		RecipientName:   d.GetRecipientName(),
		CreatedAt:       d.GetCreatedAt(),
		CvsStoreCode:    d.GetCvsStoreCode(),
		// 🔴 買家端：只放 actor=買家 的動作。修復前這裡回的是全部，
		//    包含三條掛 seller capability 的 href，買家跟隨即 403。
		AllowedActions: allowedActionsOf(d, ViewerBuyer),
	}
	response.GrpcResult(c, resp.Code, resp.Message, data)
}

// GetSellerShipmentTracking 賣家查看訂單物流追蹤
// @Summary      賣家查看訂單物流追蹤
// @Description  賣家查看自己訂單的物流追蹤，後端即時向 PayUni 同步當前狀態並回傳單點狀態。Ownership 由 shipping-service 驗 store_id。is_return 可選正向/退貨單。
// @Tags         物流 (Shipping)
// @Produce      json
// @Security     BearerAuth
// @Param        order_no   path   string  true   "訂單編號"
// @Param        is_return  query  bool    false  "true=退貨單, 預設 false=正向單"
// @Success      200  {object}  object{code=int,data=OrderLogisticsResp}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /app/shipments/{order_no}/tracking [get]
func (h *Holder) GetSellerShipmentTracking(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	orderNo := c.Param("order_no")
	if orderNo == "" {
		response.BadRequest(c, "order_no is required")
		return
	}

	resp, err := h.Shipping.GetSellerShipmentTracking(c.Request.Context(), &pb.GetSellerShipmentTrackingRequest{
		OrderNo:  orderNo,
		StoreId:  int64(storeID),
		IsReturn: c.Query("is_return") == "true",
	})
	if err != nil {
		response.GrpcErr(c, "shipping", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	d := resp.Data
	data := OrderLogisticsResp{
		OrderNo:         d.GetOrderNo(),
		ShipTradeNo:     d.GetShipTradeNo(),
		ShippingMethod:  d.GetShippingMethod(),
		Status:          d.GetStatus(),
		StatusLabel:     d.GetStatusLabel(),
		ProviderStatus:  d.GetProviderStatus(),
		ProviderMessage: d.GetProviderMessage(),
		StatusTime:      d.GetStatusTime(),
		StoreName:       d.GetStoreName(),
		RecipientName:   d.GetRecipientName(),
		CreatedAt:       d.GetCreatedAt(),
		CvsStoreCode:    d.GetCvsStoreCode(),
		// 🔴 賣家端：只放 actor=賣家 的動作（履約動作的實際歸屬端）。
		AllowedActions: allowedActionsOf(d, ViewerSeller),
	}
	response.GrpcResult(c, resp.Code, resp.Message, data)
}
