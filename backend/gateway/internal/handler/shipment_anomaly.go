package handler

import (
	"strconv"

	"github.com/Will413028/hawkr/gateway/internal/response"
	pb "github.com/Will413028/hawkr/go-pkg/proto/shipping"
	"github.com/gin-gonic/gin"
)

// ==================== §5.21 配送異常賣家通知 — DTO ====================

// ShipmentAnomalyRecordResp 賣家配送異常列表單筆
// 資料來源：shipments LEFT JOIN shipment_anomaly_acknowledgments（SQL 推導，不寫第二份狀態表）
type ShipmentAnomalyRecordResp struct {
	ShipmentId       string `json:"shipmentId"`       // shipments.id（數字字串）
	OrderNo          string `json:"orderNo"`          // 訂單編號
	AnomalyType      string `json:"anomalyType"`      // ShipmentAnomalyType.value，合法值：TCAT_CALL_FAILED / CVS_LABEL_UNPRINTED / PROVIDER_STATUS_ABNORMAL / PACKAGE_LOST / DELIVERY_OVERDUE / PAYUNI_LGS_NO_CODE / RETURN_STUCK
	AnomalyTypeLabel string `json:"anomalyTypeLabel"` // ShipmentAnomalyType.label（如「黑貓叫件未完成」）
	Severity         string `json:"severity"`         // ShipmentAnomalySeverity.value，合法值：HIGH / MEDIUM / LOW
	ShippingMethod   string `json:"shippingMethod"`   // ShippingMethod.value，合法值：3001=TAQBIN / 3002=SEVEN_ELEVEN_STORE_PICKUP
	ShipmentStatus   string `json:"shipmentStatus"`   // ShippingStatus.value（當前 shipments.status）
	DetectedAt       string `json:"detectedAt"`       // ISO 8601 UTC（推導，shipments 相關時間欄位最大值）
	AckAction        string `json:"ackAction"`        // ShipmentAnomalyAckAction.value，空字串表未處理；合法值：ACKNOWLEDGED / IGNORED
	ActedAt          string `json:"actedAt"`          // ISO 8601 UTC，未處理時為空字串
}

// ListSellerShipmentAnomaliesResponse 賣家配送異常分頁
type ListSellerShipmentAnomaliesResponse struct {
	Records  []ShipmentAnomalyRecordResp `json:"records"`
	Total    string                      `json:"total"`
	Page     string                      `json:"page"`
	PageSize string                      `json:"pageSize"`
}

// AnomalyTypeCountResp 依 anomaly_type 分組計數
type AnomalyTypeCountResp struct {
	AnomalyType      string `json:"anomalyType"`
	AnomalyTypeLabel string `json:"anomalyTypeLabel"`
	Count            string `json:"count"` // 整數字串
}

// GetSellerShipmentAnomalyStatsResponse 賣家配送異常統計
type GetSellerShipmentAnomalyStatsResponse struct {
	Total               string                 `json:"total"`               // 全部異常單數（未 ack）
	HighSeverityCount   string                 `json:"highSeverityCount"`   // HIGH 嚴重度數量
	MediumSeverityCount string                 `json:"mediumSeverityCount"` // MEDIUM 嚴重度數量
	LowSeverityCount    string                 `json:"lowSeverityCount"`    // LOW 嚴重度數量
	ByType              []AnomalyTypeCountResp `json:"byType"`              // 依 anomaly_type 分組
}

// AcknowledgeShipmentAnomalyResponse 賣家標記異常為「已處理」/「已忽略」回應
type AcknowledgeShipmentAnomalyResponse struct {
	AckId   string `json:"ackId"`   // shipment_anomaly_acknowledgments.id（數字字串）
	ActedAt string `json:"actedAt"` // ISO 8601 UTC
}

// AdminShipmentAnomalyRecordResp Admin 配送異常列表單筆
type AdminShipmentAnomalyRecordResp struct {
	ShipmentId       string `json:"shipmentId"`
	OrderNo          string `json:"orderNo"`
	StoreId          string `json:"storeId"`
	AnomalyType      string `json:"anomalyType"`
	AnomalyTypeLabel string `json:"anomalyTypeLabel"`
	Severity         string `json:"severity"`
	ShippingMethod   string `json:"shippingMethod"`
	ShipmentStatus   string `json:"shipmentStatus"`
	DetectedAt       string `json:"detectedAt"`
	AckAction        string `json:"ackAction"`       // admin 端處置：ACKNOWLEDGED / IGNORED / 空字串
	ActedAt          string `json:"actedAt"`         // admin 端處置時間（ISO 8601 UTC）
	SellerAckAction  string `json:"sellerAckAction"` // issue #1327：賣家端處置（唯讀揭露）
	SellerActedAt    string `json:"sellerActedAt"`   // issue #1327：賣家端處置時間（ISO 8601 UTC）
}

// ListShipmentAnomaliesForAdminResponse Admin 配送異常分頁
type ListShipmentAnomaliesForAdminResponse struct {
	Records  []AdminShipmentAnomalyRecordResp `json:"records"`
	Total    string                           `json:"total"`
	Page     string                           `json:"page"`
	PageSize string                           `json:"pageSize"`
}

// ==================== §5.21 配送異常賣家通知 — Handler ====================

// ListSellerShipmentAnomalies 列賣家配送異常 dashboard
//
// @Summary      列賣家配送異常 dashboard
// @Description  賣家查詢自己的配送異常列表，含 ack 狀態。資料由 shipments 表 LEFT JOIN ack 表 SQL 推導，不存第二份狀態表。預設僅回未 ack 的異常單；show_acknowledged=true 包含已處理/已忽略。
// @Tags         物流 (Shipping)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        anomaly_type       query  string  false  "ShipmentAnomalyType.value 過濾；合法值：TCAT_CALL_FAILED / CVS_LABEL_UNPRINTED / PROVIDER_STATUS_ABNORMAL / PACKAGE_LOST / DELIVERY_OVERDUE / PAYUNI_LGS_NO_CODE / RETURN_STUCK"
// @Param        severity           query  string  false  "ShipmentAnomalySeverity.value 過濾；合法值：HIGH / MEDIUM / LOW"
// @Param        show_acknowledged  query  string  false  "是否包含已 ack/ignored 的單；合法值：true / false"  default(false)
// @Param        page               query  string  false  "頁碼，整數字串"  default(1)
// @Param        page_size          query  string  false  "每頁筆數，整數字串"  default(20)
// @Success      200  {object}  object{code=int,message=string,data=ListSellerShipmentAnomaliesResponse}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      500  {object}  object{code=int,message=string}
// @Router       /app/shipments/anomalies [get]
func (h *Holder) ListSellerShipmentAnomalies(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	req := &pb.ListSellerShipmentAnomaliesRequest{
		StoreId:          strconv.FormatInt(int64(storeID), 10),
		AnomalyType:      c.Query("anomaly_type"),
		Severity:         c.Query("severity"),
		ShowAcknowledged: c.DefaultQuery("show_acknowledged", "false") == "true",
		Page:             c.DefaultQuery("page", "1"),
		PageSize:         c.DefaultQuery("page_size", "20"),
	}

	resp, err := h.Shipping.ListSellerShipmentAnomalies(c.Request.Context(), req)
	if err != nil {
		response.GrpcErr(c, "shipping", err)
		return
	}
	if resp.GetCode() != 0 {
		response.GrpcResult(c, resp.GetCode(), resp.GetMessage(), nil)
		return
	}

	data := resp.GetData()
	records := make([]ShipmentAnomalyRecordResp, 0, len(data.GetRecords()))
	for _, r := range data.GetRecords() {
		records = append(records, ShipmentAnomalyRecordResp{
			ShipmentId:       r.GetShipmentId(),
			OrderNo:          r.GetOrderNo(),
			AnomalyType:      r.GetAnomalyType(),
			AnomalyTypeLabel: r.GetAnomalyTypeLabel(),
			Severity:         r.GetSeverity(),
			ShippingMethod:   r.GetShippingMethod(),
			ShipmentStatus:   r.GetShipmentStatus(),
			DetectedAt:       r.GetDetectedAt(),
			AckAction:        r.GetAckAction(),
			ActedAt:          r.GetActedAt(),
		})
	}
	response.GrpcResult(c, resp.GetCode(), resp.GetMessage(), ListSellerShipmentAnomaliesResponse{
		Records:  records,
		Total:    data.GetTotal(),
		Page:     data.GetPage(),
		PageSize: data.GetPageSize(),
	})
}

// GetSellerShipmentAnomalyStats 賣家配送異常統計
//
// @Summary      賣家配送異常統計
// @Description  賣家配送異常的統計概覽，依 anomaly_type 與 severity 分組計數，供賣家 dashboard 紅點/分類條使用。僅計算未 ack 的異常單。
// @Tags         物流 (Shipping)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  object{code=int,message=string,data=GetSellerShipmentAnomalyStatsResponse}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      500  {object}  object{code=int,message=string}
// @Router       /app/shipments/anomalies/stats [get]
func (h *Holder) GetSellerShipmentAnomalyStats(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	req := &pb.GetSellerShipmentAnomalyStatsRequest{
		StoreId: strconv.FormatInt(int64(storeID), 10),
	}

	resp, err := h.Shipping.GetSellerShipmentAnomalyStats(c.Request.Context(), req)
	if err != nil {
		response.GrpcErr(c, "shipping", err)
		return
	}
	if resp.GetCode() != 0 {
		response.GrpcResult(c, resp.GetCode(), resp.GetMessage(), nil)
		return
	}

	data := resp.GetData()
	byType := make([]AnomalyTypeCountResp, 0, len(data.GetByType()))
	for _, t := range data.GetByType() {
		byType = append(byType, AnomalyTypeCountResp{
			AnomalyType:      t.GetAnomalyType(),
			AnomalyTypeLabel: t.GetAnomalyTypeLabel(),
			Count:            t.GetCount(),
		})
	}
	response.GrpcResult(c, resp.GetCode(), resp.GetMessage(), GetSellerShipmentAnomalyStatsResponse{
		Total:               data.GetTotal(),
		HighSeverityCount:   data.GetHighSeverityCount(),
		MediumSeverityCount: data.GetMediumSeverityCount(),
		LowSeverityCount:    data.GetLowSeverityCount(),
		ByType:              byType,
	})
}

// AcknowledgeShipmentAnomaly 賣家標記異常為「已處理」
//
// @Summary      賣家標記異常為「已處理」
// @Description  賣家對特定 shipment 的特定 anomaly_type 標記為「已處理」（upsert shipment_anomaly_acknowledgments）。複合 key 為 (shipment_id, anomaly_type)，不需事前查詢 ack id。
// @Tags         物流 (Shipping)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        shipment_id   path  string  true  "shipments.id（TSID 數字字串）"
// @Param        anomaly_type  path  string  true  "ShipmentAnomalyType.value；合法值：TCAT_CALL_FAILED / CVS_LABEL_UNPRINTED / PROVIDER_STATUS_ABNORMAL / PACKAGE_LOST / DELIVERY_OVERDUE / PAYUNI_LGS_NO_CODE / RETURN_STUCK"
// @Success      200  {object}  object{code=int,message=string,data=AcknowledgeShipmentAnomalyResponse}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      500  {object}  object{code=int,message=string}
// @Router       /app/shipments/anomalies/{shipment_id}/{anomaly_type}/acknowledge [post]
func (h *Holder) AcknowledgeShipmentAnomaly(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	shipmentID := c.Param("shipment_id")
	if shipmentID == "" {
		response.BadRequest(c, "shipment_id is required")
		return
	}
	anomalyType := c.Param("anomaly_type")
	if anomalyType == "" {
		response.BadRequest(c, "anomaly_type is required")
		return
	}

	req := &pb.AcknowledgeShipmentAnomalyRequest{
		StoreId:     strconv.FormatInt(int64(storeID), 10),
		ShipmentId:  shipmentID,
		AnomalyType: anomalyType,
	}

	resp, err := h.Shipping.AcknowledgeShipmentAnomaly(c.Request.Context(), req)
	if err != nil {
		response.GrpcErr(c, "shipping", err)
		return
	}
	if resp.GetCode() != 0 {
		response.GrpcResult(c, resp.GetCode(), resp.GetMessage(), nil)
		return
	}

	data := resp.GetData()
	response.GrpcResult(c, resp.GetCode(), resp.GetMessage(), AcknowledgeShipmentAnomalyResponse{
		AckId:   data.GetAckId(),
		ActedAt: data.GetActedAt(),
	})
}

// IgnoreShipmentAnomaly 賣家標記異常為「已忽略」
//
// @Summary      賣家標記異常為「已忽略」
// @Description  賣家對特定 shipment 的特定 anomaly_type 標記為「已忽略」（upsert shipment_anomaly_acknowledgments，action=IGNORED）。複合 key 為 (shipment_id, anomaly_type)，不需事前查詢 ack id。
// @Tags         物流 (Shipping)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        shipment_id   path  string  true  "shipments.id（TSID 數字字串）"
// @Param        anomaly_type  path  string  true  "ShipmentAnomalyType.value；合法值：TCAT_CALL_FAILED / CVS_LABEL_UNPRINTED / PROVIDER_STATUS_ABNORMAL / PACKAGE_LOST / DELIVERY_OVERDUE / PAYUNI_LGS_NO_CODE / RETURN_STUCK"
// @Success      200  {object}  object{code=int,message=string,data=AcknowledgeShipmentAnomalyResponse}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      500  {object}  object{code=int,message=string}
// @Router       /app/shipments/anomalies/{shipment_id}/{anomaly_type}/ignore [post]
func (h *Holder) IgnoreShipmentAnomaly(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	shipmentID := c.Param("shipment_id")
	if shipmentID == "" {
		response.BadRequest(c, "shipment_id is required")
		return
	}
	anomalyType := c.Param("anomaly_type")
	if anomalyType == "" {
		response.BadRequest(c, "anomaly_type is required")
		return
	}

	req := &pb.IgnoreShipmentAnomalyRequest{
		StoreId:     strconv.FormatInt(int64(storeID), 10),
		ShipmentId:  shipmentID,
		AnomalyType: anomalyType,
	}

	resp, err := h.Shipping.IgnoreShipmentAnomaly(c.Request.Context(), req)
	if err != nil {
		response.GrpcErr(c, "shipping", err)
		return
	}
	if resp.GetCode() != 0 {
		response.GrpcResult(c, resp.GetCode(), resp.GetMessage(), nil)
		return
	}

	data := resp.GetData()
	response.GrpcResult(c, resp.GetCode(), resp.GetMessage(), AcknowledgeShipmentAnomalyResponse{
		AckId:   data.GetAckId(),
		ActedAt: data.GetActedAt(),
	})
}

// ListShipmentAnomaliesForAdmin Admin 平台級配送異常監控
//
// @Summary      Admin 平台級配送異常監控
// @Description  Admin 查詢全平台配送異常列表，可依 store_id / anomaly_type / severity 過濾。資料由 shipments LEFT JOIN ack 表 SQL 推導，與賣家端共用同一資料來源。
// @Tags         Admin 異常訂單 (Admin-Abnormal)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        store_id           query  string  false  "依賣家過濾"
// @Param        anomaly_type       query  string  false  "ShipmentAnomalyType.value 過濾；合法值：TCAT_CALL_FAILED / CVS_LABEL_UNPRINTED / PROVIDER_STATUS_ABNORMAL / PACKAGE_LOST / DELIVERY_OVERDUE / PAYUNI_LGS_NO_CODE / RETURN_STUCK"
// @Param        severity           query  string  false  "ShipmentAnomalySeverity.value 過濾；合法值：HIGH / MEDIUM / LOW"
// @Param        show_acknowledged  query  string  false  "是否包含已 ack/ignored 的單；合法值：true / false"  default(false)
// @Param        page               query  string  false  "頁碼，整數字串"  default(1)
// @Param        page_size          query  string  false  "每頁筆數，整數字串"  default(20)
// @Success      200  {object}  object{code=int,message=string,data=ListShipmentAnomaliesForAdminResponse}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      500  {object}  object{code=int,message=string}
// @Router       /admin/shipments/anomalies [get]
func (h *Holder) ListShipmentAnomaliesForAdmin(c *gin.Context) {
	storeIDFilter, ok := h.resolveOptionalSellerFilterQuery(c, c.Query("store_id"))
	if !ok {
		return
	}
	if storeIDFilter == noCurrentSellerFilterSentinel {
		response.OK(c, ListShipmentAnomaliesForAdminResponse{
			Records:  []AdminShipmentAnomalyRecordResp{},
			Total:    "0",
			Page:     c.DefaultQuery("page", "1"),
			PageSize: c.DefaultQuery("page_size", "20"),
		})
		return
	}

	req := &pb.ListShipmentAnomaliesForAdminRequest{
		StoreId:          storeIDFilter,
		AnomalyType:      c.Query("anomaly_type"),
		Severity:         c.Query("severity"),
		ShowAcknowledged: c.DefaultQuery("show_acknowledged", "false") == "true",
		Page:             c.DefaultQuery("page", "1"),
		PageSize:         c.DefaultQuery("page_size", "20"),
	}

	resp, err := h.Shipping.ListShipmentAnomaliesForAdmin(c.Request.Context(), req)
	if err != nil {
		response.GrpcErr(c, "shipping", err)
		return
	}
	if resp.GetCode() != 0 {
		response.GrpcResult(c, resp.GetCode(), resp.GetMessage(), nil)
		return
	}

	data := resp.GetData()
	records := make([]AdminShipmentAnomalyRecordResp, 0, len(data.GetRecords()))
	for _, r := range data.GetRecords() {
		records = append(records, AdminShipmentAnomalyRecordResp{
			ShipmentId:       r.GetShipmentId(),
			OrderNo:          r.GetOrderNo(),
			StoreId:          r.GetStoreId(),
			AnomalyType:      r.GetAnomalyType(),
			AnomalyTypeLabel: r.GetAnomalyTypeLabel(),
			Severity:         r.GetSeverity(),
			ShippingMethod:   r.GetShippingMethod(),
			ShipmentStatus:   r.GetShipmentStatus(),
			DetectedAt:       r.GetDetectedAt(),
			AckAction:        r.GetAckAction(),
			ActedAt:          r.GetActedAt(),
			SellerAckAction:  r.GetSellerAckAction(),
			SellerActedAt:    r.GetSellerActedAt(),
		})
	}
	response.GrpcResult(c, resp.GetCode(), resp.GetMessage(), ListShipmentAnomaliesForAdminResponse{
		Records:  records,
		Total:    data.GetTotal(),
		Page:     data.GetPage(),
		PageSize: data.GetPageSize(),
	})
}

func resolveAdminAccountID(c *gin.Context) (int64, bool) {
	raw := c.GetHeader("x-account-id")
	if raw == "" {
		response.Unauthorized(c)
		return 0, false
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		response.Unauthorized(c)
		return 0, false
	}
	return id, true
}

// AcknowledgeShipmentAnomalyForAdmin Admin 標記異常為「已處理」
//
// @Summary      Admin 標記配送異常為「已處理」
// @Description  Admin 對特定 shipment 的特定 anomaly_type 標記為「已處理」（actor_type=ADMIN，與賣家 ack 互不覆蓋）。複合 key 為 (shipment_id, anomaly_type)，不需事前查詢 ack id。
// @Tags         Admin 異常訂單 (Admin-Abnormal)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        shipment_id   path  string  true  "shipments.id（TSID 數字字串）"
// @Param        anomaly_type  path  string  true  "ShipmentAnomalyType.value；合法值：TCAT_CALL_FAILED / CVS_LABEL_UNPRINTED / PROVIDER_STATUS_ABNORMAL / PACKAGE_LOST / DELIVERY_OVERDUE / PAYUNI_LGS_NO_CODE / RETURN_STUCK"
// @Success      200  {object}  object{code=int,message=string,data=AcknowledgeShipmentAnomalyResponse}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      500  {object}  object{code=int,message=string}
// @Router       /admin/shipments/anomalies/{shipment_id}/{anomaly_type}/acknowledge [post]
func (h *Holder) AcknowledgeShipmentAnomalyForAdmin(c *gin.Context) {
	accountID, ok := resolveAdminAccountID(c)
	if !ok {
		return
	}

	shipmentID := c.Param("shipment_id")
	if shipmentID == "" {
		response.BadRequest(c, "shipment_id is required")
		return
	}
	anomalyType := c.Param("anomaly_type")
	if anomalyType == "" {
		response.BadRequest(c, "anomaly_type is required")
		return
	}

	req := &pb.AcknowledgeShipmentAnomalyForAdminRequest{
		AccountId:   accountID,
		ShipmentId:  shipmentID,
		AnomalyType: anomalyType,
	}

	resp, err := h.Shipping.AcknowledgeShipmentAnomalyForAdmin(c.Request.Context(), req)
	if err != nil {
		response.GrpcErr(c, "shipping", err)
		return
	}
	if resp.GetCode() != 0 {
		response.GrpcResult(c, resp.GetCode(), resp.GetMessage(), nil)
		return
	}

	data := resp.GetData()
	response.GrpcResult(c, resp.GetCode(), resp.GetMessage(), AcknowledgeShipmentAnomalyResponse{
		AckId:   data.GetAckId(),
		ActedAt: data.GetActedAt(),
	})
}

// IgnoreShipmentAnomalyForAdmin Admin 標記異常為「已忽略」
//
// @Summary      Admin 標記配送異常為「已忽略」
// @Description  Admin 對特定 shipment 的特定 anomaly_type 標記為「已忽略」（actor_type=ADMIN，action=IGNORED，與賣家 ack 互不覆蓋）。複合 key 為 (shipment_id, anomaly_type)，不需事前查詢 ack id。
// @Tags         Admin 異常訂單 (Admin-Abnormal)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        shipment_id   path  string  true  "shipments.id（TSID 數字字串）"
// @Param        anomaly_type  path  string  true  "ShipmentAnomalyType.value；合法值：TCAT_CALL_FAILED / CVS_LABEL_UNPRINTED / PROVIDER_STATUS_ABNORMAL / PACKAGE_LOST / DELIVERY_OVERDUE / PAYUNI_LGS_NO_CODE / RETURN_STUCK"
// @Success      200  {object}  object{code=int,message=string,data=AcknowledgeShipmentAnomalyResponse}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      500  {object}  object{code=int,message=string}
// @Router       /admin/shipments/anomalies/{shipment_id}/{anomaly_type}/ignore [post]
func (h *Holder) IgnoreShipmentAnomalyForAdmin(c *gin.Context) {
	accountID, ok := resolveAdminAccountID(c)
	if !ok {
		return
	}

	shipmentID := c.Param("shipment_id")
	if shipmentID == "" {
		response.BadRequest(c, "shipment_id is required")
		return
	}
	anomalyType := c.Param("anomaly_type")
	if anomalyType == "" {
		response.BadRequest(c, "anomaly_type is required")
		return
	}

	req := &pb.IgnoreShipmentAnomalyForAdminRequest{
		AccountId:   accountID,
		ShipmentId:  shipmentID,
		AnomalyType: anomalyType,
	}

	resp, err := h.Shipping.IgnoreShipmentAnomalyForAdmin(c.Request.Context(), req)
	if err != nil {
		response.GrpcErr(c, "shipping", err)
		return
	}
	if resp.GetCode() != 0 {
		response.GrpcResult(c, resp.GetCode(), resp.GetMessage(), nil)
		return
	}

	data := resp.GetData()
	response.GrpcResult(c, resp.GetCode(), resp.GetMessage(), AcknowledgeShipmentAnomalyResponse{
		AckId:   data.GetAckId(),
		ActedAt: data.GetActedAt(),
	})
}
