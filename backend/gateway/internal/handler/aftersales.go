package handler

import (
	"net/http"
	"strconv"

	"github.com/Will413028/hawkr/gateway/internal/codemeta"
	"github.com/Will413028/hawkr/gateway/internal/response"
	pb "github.com/Will413028/hawkr/go-pkg/proto/aftersales"
	"github.com/gin-gonic/gin"
)

// ==================== DTO ====================

// CreateAfterSaleCaseRequest 申請售後請求
type CreateAfterSaleCaseRequest struct {
	OrderNo     string   `json:"orderNo" binding:"required"` // 訂單編號
	Reason      string   `json:"reason" binding:"required"`  // 退款原因（RefundReason value: 5001=商品瑕疵, 5002=寄錯商品, 5003=與描述不符, 5004=改變心意, 5005=其他）
	Description string   `json:"description"`                // 補充說明
	PhotoTokens []string `json:"photoTokens"`                // Media staged_token（最多 3 張）
}

// SupplementEvidenceRequest 補件請求
type SupplementEvidenceRequest struct {
	AdditionalDescription string   `json:"additionalDescription"` // 補充說明
	PhotoTokens           []string `json:"photoTokens"`           // Media staged_token
}

// ResolveAfterSaleCaseRequest 賣家提交決策請求
type ResolveAfterSaleCaseRequest struct {
	ResolutionType       string `json:"resolutionType" binding:"required"` // 決策類型（1=退貨退款, 2=部分退款, 3=全額免退, 4=優惠券補償, 5=拒絕）
	RefundAmount         string `json:"refundAmount"`                      // 退款金額（resolutionType=2 時必填）
	ReturnShippingMethod string `json:"returnShippingMethod"`              // 退貨物流方式（3001=黑貓宅急便, 3002=7-11店到店）（resolutionType=1 時必填）
	CouponAmount         string `json:"couponAmount"`                      // 優惠券金額（resolutionType=4 時必填）
	CouponMinSpend       string `json:"couponMinSpend"`                    // 優惠券門檻（resolutionType=4 時必填）
	CouponMessage        string `json:"couponMessage"`                     // 優惠券說明
	RejectReason         string `json:"rejectReason"`                      // 拒絕原因（AfterSaleRejectReason value: 9101=買家保存不當, 9102=超過申訴時效, 9103=證據不足, 9104=商品狀態正常, 9199=其他）（resolutionType=5 時必填）
	RejectMessage        string `json:"rejectMessage"`                     // 拒絕說明
	NegotiationReason    string `json:"negotiationReason"`                 // 協商說明
}

// CancelShipmentRequest 取消出貨請求
type CancelShipmentRequest struct {
	Reason string `json:"reason" binding:"required"` // 取消原因（SellerCancelReason value: 9201=天災因素, 9202=商品缺貨, 9203=品質問題, 9299=其他）
	Remark string `json:"remark"`                    // 備註
}

// ConfirmReturnReceiptRequest 確認收貨請求（含選填驗貨扣款）
// 全部欄位皆為選填，未傳 body 等同「無瑕疵退全額」（向後相容）
type ConfirmReturnReceiptRequest struct {
	ActualRefundAmount    string   `json:"actualRefundAmount,omitempty"`
	DeductionReason       string   `json:"deductionReason,omitempty"`
	InspectionNote        string   `json:"inspectionNote,omitempty"`
	InspectionPhotoTokens []string `json:"inspectionPhotoTokens,omitempty"`
}

// AfterSaleCaseSummaryResponse 售後案件摘要
type AfterSaleCaseSummaryResponse struct {
	TemperatureCode       string `json:"temperatureCode"`
	OrderNo               string `json:"orderNo"`
	BuyerName             string `json:"buyerName"`
	BuyerAvatarKey        string `json:"avatarKey"` // 買家頭像 objectKey（SSOT；49 §6.1）
	BuyerAvatarUrl        string `json:"avatarUrl"` // 買家頭像顯示 URL（Gateway 組）
	ShippingAddress       string `json:"shippingAddress"`
	ReturnShippingMethod  string `json:"returnShippingMethod"`
	CaseNo                string `json:"caseNo"`
	Status                string `json:"status"` // 案件狀態（AfterSaleCaseStatus value；合法值以 CodeMeta 業務狀態碼.md 為準）
	Reason                string `json:"reason"`
	RefundableAmount      string `json:"refundableAmount"`
	CreatedAt             string `json:"createdAt"`
	AppealState           string `json:"appealState"`                     // 申訴狀態（AfterSaleAppealState value：0=無申訴 / 10=已申訴 / 20=平台調查中 / 30=已裁決）
	RefundTaskStatus      string `json:"refundTaskStatus"`                // WorkerTaskStatus value。合法值：0=PENDING, 5=AWAITING_BUYER_REFUND_PAGE, 10=PROCESSING, 15=AWAITING_WEBHOOK, 20=DONE, 30=FAILED, 40=MANUAL_INTERVENTION；無 refund_task 時為空字串
	RefundFailureMessage  string `json:"refundFailureMessage"`            // 退款失敗訊息（PayUni 原話，直接透傳），成功時為空字串
	EstimatedRefundDate   string `json:"estimatedRefundDate"`             // 預計扣款日 ISO 8601 YYYY-MM-DD（僅 ATM 同步受理後有值，信用卡恆空）
	BankDate              string `json:"bankDate"`                        // 實際到帳日 ISO 8601 YYYY-MM-DD（僅 ATM webhook 匯款成功後有值，信用卡恆空）
	ProviderStatus        string `json:"providerStatus,omitempty"`        // 聚合 PayUni 原始狀態碼（收斂規範 §6.2；退款看 refund_tasks，退貨物流看 shipments）
	ProviderMessage       string `json:"providerMessage,omitempty"`       // 聚合 PayUni 原始訊息
	AutoConfirmAt         string `json:"autoConfirmAt,omitempty"`         // §5.3.10 自動確認收貨截止時間（僅 status=27 RETURN_DELIVERED 有值，前端列表也可秀「N 小時後自動確認」）
	StorePickupDeadlineAt string `json:"storePickupDeadlineAt,omitempty"` // 2026-05-22 超商待取逾時截止（僅 status=26 RETURN_AT_STORE 有值，前端依此倒數 72h）
	ShipBackDeadlineAt    string `json:"shipBackDeadlineAt,omitempty"`    // 2026-06-15 買家寄回退貨逾時截止（僅 status=20 RETURN_PENDING 有值，前端依此倒數 7 天）
	NeedRefundPage        bool   `json:"needRefundPage"`
}

// AfterSaleCaseDetailResponse 售後案件詳情
type AfterSaleCaseDetailResponse struct {
	PartnerId             string `json:"partnerId"`
	ReturnShipmentNo      string `json:"returnShipmentNo"`
	BuyerId               string `json:"buyerId"`
	StoreId               string `json:"storeId"`
	Status                string `json:"status"` // 案件狀態（AfterSaleCaseStatus value；合法值以 CodeMeta 業務狀態碼.md 為準）
	Reason                string `json:"reason"`
	Description           string `json:"description"`
	RefundableAmount      string `json:"refundableAmount"`
	ValidationNo          string `json:"validationNo"`
	ReturnShippingMethod  string `json:"returnShippingMethod"`
	ShipTradeNo           string `json:"shipTradeNo"`
	OrderNo               string `json:"orderNo"`
	AppealDeadline        string `json:"appealDeadline,omitempty"`
	CaseNo                string `json:"caseNo"`
	CreatedAt             string `json:"createdAt"`
	ClosedAt              string `json:"closedAt"`
	AppealState           string `json:"appealState"` // 申訴狀態（AfterSaleAppealState value：0=無申訴 / 10=已申訴 / 20=平台調查中 / 30=已裁決）
	Odno                  string `json:"odno"`
	RefundTaskStatus      string `json:"refundTaskStatus"`                // WorkerTaskStatus value，無 refund_task 時空字串
	RefundFailureMessage  string `json:"refundFailureMessage"`            // 退款失敗訊息（PayUni 原話）
	EstimatedRefundDate   string `json:"estimatedRefundDate"`             // 預計扣款日 YYYY-MM-DD（僅 ATM 受理後）
	BankDate              string `json:"bankDate"`                        // 實際到帳日 YYYY-MM-DD（僅 ATM 成功匯款後）
	ProviderStatus        string `json:"providerStatus,omitempty"`        // 聚合 PayUni 原始狀態碼（收斂規範 §6.2）
	ProviderMessage       string `json:"providerMessage,omitempty"`       // 聚合 PayUni 原始訊息
	ProcessDeadline       string `json:"processDeadline,omitempty"`       // §5.10 賣家處理截止時間 ISO 8601 UTC；僅 status=PENDING 才有值；前端依此倒數
	AutoEscalated         bool   `json:"autoEscalated"`                   // §5.10 是否系統自動升級至平台仲裁
	ProcessDeadlineHours  int32  `json:"processDeadlineHours"`            // §5.10 配置的處理期限小時數（前端展示用，例 24）
	AutoConfirmAt         string `json:"autoConfirmAt,omitempty"`         // §5.3.10 自動確認收貨截止時間（僅 status=27 RETURN_DELIVERED 才有值；前端依此倒數）
	ReturnDeliveredAt     string `json:"returnDeliveredAt,omitempty"`     // §5.3.10 退貨送達時間（status>=27 才有值）
	ReturnProviderStatus  string `json:"returnProviderStatus,omitempty"`  // §5.3.11 PayUni 退貨物流原始狀態碼（status=28 RETURN_FAILED 才有值）
	ReturnProviderMessage string `json:"returnProviderMessage,omitempty"` // §5.3.11 PayUni 解密後錯誤訊息原文（status=28 才有值，前端直接顯示）
	// 2026-05-22 逆物流退貨政策修訂欄位
	StoreArrivedAt        string `json:"storeArrivedAt,omitempty"`        // 超商到店時間 ISO 8601 UTC（PayUni 碼 32 觸發 25→26 stamp）
	StorePickupDeadlineAt string `json:"storePickupDeadlineAt,omitempty"` // 超商待取逾時截止 ISO 8601 UTC（僅 status=26 RETURN_AT_STORE 有值，前端依此倒數 72h）
	SellerPickedUpAt      string `json:"sellerPickedUpAt,omitempty"`      // 超商賣家實際取貨時間 ISO 8601 UTC（PayUni 碼 11 觸發 26→27 stamp）
	AutoCompleteAtStamp   string `json:"autoCompleteAtStamp,omitempty"`   // 系統實際自動完成的時間戳 ISO 8601 UTC（status=31/32 才有值）
	// 2026-06-15 待寄回逾時政策修訂
	ShipBackDeadlineAt string                      `json:"shipBackDeadlineAt,omitempty"` // 買家寄回退貨逾時截止 ISO 8601 UTC（僅 status=20 RETURN_PENDING 有值，前端依此倒數 7 天）
	Resolutions        []ResolutionResponse        `json:"resolutions"`
	ShipmentPhotoKeys  []string                    `json:"shipmentPhotoKeys"` // 出貨照片 objectKey（SSOT，前端 buildImageUrl 組顯示；49 §6.1）
	ShipmentPhotoUrls  []string                    `json:"shipmentPhotoUrls"` // 出貨照片顯示 URL（Gateway 組）
	Evidence           []EvidenceResponse          `json:"evidence"`
	Items              []AfterSaleCaseItemResponse `json:"items"`
	NeedRefundPage     bool                        `json:"needRefundPage"`
	CanAppeal          bool                        `json:"canAppeal"`
}

// AfterSaleCaseItemResponse 售後案件明細項目
type AfterSaleCaseItemResponse struct {
	OrderItemId     string `json:"orderItemId"`
	SkuId           string `json:"skuId"`
	SkuName         string `json:"skuName"`
	Quantity        string `json:"quantity"`
	OriginalPrice   string `json:"originalPrice"`
	ActualPayAmount string `json:"actualPayAmount"`
	ImageKey        string `json:"imageKey"` // 商品圖 objectKey（SSOT；49 §6.1）
	ImageUrl        string `json:"imageUrl"` // 商品圖顯示 URL（Gateway 組）
}

// EvidenceResponse 舉證回應
type EvidenceResponse struct {
	AssetId  string `json:"assetId"`
	ImageKey string `json:"imageKey"` // 佐證圖 objectKey（SSOT；49 §6.1）
	ImageUrl string `json:"imageUrl"` // 佐證圖顯示 URL（Gateway 組）
	// 舉證類型碼（CodeMeta AfterSaleEvidenceType Value：8601=PHOTO 買家舉證／8602=INSPECTION_PHOTO 賣家驗貨照片）。
	// 值是數字碼字串，不是 BUYER／SELLER 這類角色字面值——前端依角色分組必須比對 8601／8602。
	EvidenceType string `json:"evidenceType"`
	CreatedAt    string `json:"createdAt"`
}

// ResolutionResponse 決策記錄回應
type ResolutionResponse struct {
	ResolutionType    string `json:"resolutionType"` // 決策類型（1=退貨退款, 2=部分退款, 3=全額免退, 4=優惠券補償, 5=拒絕, 6=驗貨扣款）
	RefundAmount      string `json:"refundAmount"`
	CouponAmount      string `json:"couponAmount"`
	CouponMinSpend    string `json:"couponMinSpend"`
	CouponMessage     string `json:"couponMessage"`
	RejectReason      string `json:"rejectReason"` // 拒絕原因（AfterSaleRejectReason value: 9101/9102/9103/9104/9199）
	RejectMessage     string `json:"rejectMessage"`
	NegotiationReason string `json:"negotiationReason"`
	CreatedAt         string `json:"createdAt"`
	// 驗貨扣款（僅 resolutionType=6 INSPECTION_DEDUCTION 有值）
	DeductionReason string `json:"deductionReason,omitempty"` // 扣款原因
	InspectionNote  string `json:"inspectionNote,omitempty"`  // 驗貨備註
}

// ResolveResultResponse 決策結果回應
type ResolveResultResponse struct {
	CaseNo           string `json:"caseNo"`
	Status           string `json:"status"`           // 案件狀態（AfterSaleCaseStatus value；合法值以 CodeMeta 業務狀態碼.md 為準）
	ReturnShipmentNo string `json:"returnShipmentNo"` // 退貨物流單號（退貨退款時有值）
	NeedRefundPage   bool   `json:"needRefundPage"`   // 是否需前往 ATM 退款頁面
}

// RefundPageUrlDataResponse ATM 退款頁面 URL 回應
type RefundPageUrlDataResponse struct {
	RedirectUrl string `json:"redirectUrl"` // PayUni 退款頁面 URL
}

// GetRefundPageUrlRequest ATM 退款頁面 URL 請求
type GetRefundPageUrlRequest struct {
	ClientType string `json:"clientType"` // CodeMeta ClientType value（MOBILE_APP / WEB_BROWSER），空字串由 Service 套用 MOBILE_APP 預設
}

// CancelShipmentResultResponse 取消出貨結果回應
type CancelShipmentResultResponse struct {
	OrderNo        string `json:"orderNo"`
	RefundAmount   string `json:"refundAmount"`
	NeedRefundPage bool   `json:"needRefundPage"` // 是否需前往 ATM 退款頁面
}

// ==================== 轉換函式 ====================

func convertAfterSaleSummary(s *pb.AfterSaleCaseSummary, mediaBase string) AfterSaleCaseSummaryResponse {
	if s == nil {
		return AfterSaleCaseSummaryResponse{}
	}
	return AfterSaleCaseSummaryResponse{
		CaseNo:                s.CaseNo,
		OrderNo:               s.OrderNo,
		BuyerName:             s.BuyerName,
		BuyerAvatarKey:        s.BuyerAvatarKey,
		BuyerAvatarUrl:        buildURL(mediaBase, s.BuyerAvatarKey),
		ShippingAddress:       s.ShippingAddress,
		ReturnShippingMethod:  s.ReturnShippingMethod,
		TemperatureCode:       s.TemperatureCode,
		Status:                s.Status,
		Reason:                s.Reason,
		RefundableAmount:      s.RefundableAmount,
		CreatedAt:             s.CreatedAt,
		NeedRefundPage:        s.NeedRefundPage,
		AppealState:           s.AppealState,
		RefundTaskStatus:      s.RefundTaskStatus,
		RefundFailureMessage:  s.RefundFailureMessage,
		EstimatedRefundDate:   s.EstimatedRefundDate,
		BankDate:              s.BankDate,
		ProviderStatus:        s.GetProviderStatus(),
		ProviderMessage:       s.GetProviderMessage(),
		AutoConfirmAt:         s.GetAutoConfirmAt(),
		StorePickupDeadlineAt: s.GetStorePickupDeadlineAt(),
		ShipBackDeadlineAt:    s.GetShipBackDeadlineAt(),
	}
}

func convertAfterSaleSummaries(list []*pb.AfterSaleCaseSummary, mediaBase string) []AfterSaleCaseSummaryResponse {
	result := make([]AfterSaleCaseSummaryResponse, 0, len(list))
	for _, s := range list {
		result = append(result, convertAfterSaleSummary(s, mediaBase))
	}
	return result
}

func convertAfterSaleDetail(d *pb.AfterSaleCaseDetailView, mediaBase string) AfterSaleCaseDetailResponse {
	if d == nil {
		return AfterSaleCaseDetailResponse{}
	}
	items := make([]AfterSaleCaseItemResponse, 0, len(d.Items))
	for _, item := range d.Items {
		items = append(items, AfterSaleCaseItemResponse{
			OrderItemId:     item.OrderItemId,
			SkuId:           item.SkuId,
			SkuName:         item.SkuName,
			Quantity:        item.Quantity,
			OriginalPrice:   item.OriginalPrice,
			ActualPayAmount: item.ActualPayAmount,
			ImageKey:        item.ImageKey,
			ImageUrl:        buildURL(mediaBase, item.ImageKey),
		})
	}
	evidence := make([]EvidenceResponse, 0, len(d.Evidence))
	for _, e := range d.Evidence {
		evidence = append(evidence, EvidenceResponse{
			AssetId:      e.AssetId,
			ImageKey:     e.ImageKey,
			ImageUrl:     buildURL(mediaBase, e.ImageKey),
			EvidenceType: e.EvidenceType,
			CreatedAt:    e.CreatedAt,
		})
	}
	resolutions := make([]ResolutionResponse, 0, len(d.Resolutions))
	for _, r := range d.Resolutions {
		resolutions = append(resolutions, ResolutionResponse{
			ResolutionType:    r.ResolutionType,
			RefundAmount:      r.RefundAmount,
			CouponAmount:      r.CouponAmount,
			CouponMinSpend:    r.CouponMinSpend,
			CouponMessage:     r.CouponMessage,
			RejectReason:      r.RejectReason,
			RejectMessage:     r.RejectMessage,
			NegotiationReason: r.NegotiationReason,
			CreatedAt:         r.CreatedAt,
			DeductionReason:   r.DeductionReason,
			InspectionNote:    r.InspectionNote,
		})
	}
	return AfterSaleCaseDetailResponse{
		CaseNo:                d.CaseNo,
		OrderNo:               d.OrderNo,
		BuyerId:               d.BuyerId,
		StoreId:               d.StoreId,
		Status:                d.Status,
		Reason:                d.Reason,
		Description:           d.Description,
		RefundableAmount:      d.RefundableAmount,
		Items:                 items,
		Evidence:              evidence,
		Resolutions:           resolutions,
		ReturnShipmentNo:      d.ReturnShipmentNo,
		ReturnShippingMethod:  d.ReturnShippingMethod,
		ShipTradeNo:           d.ShipTradeNo,
		PartnerId:             d.PartnerId,
		Odno:                  d.Odno,
		ValidationNo:          d.ValidationNo,
		ShipmentPhotoKeys:     d.ShipmentPhotoKeys,
		ShipmentPhotoUrls:     buildURLs(mediaBase, d.ShipmentPhotoKeys),
		NeedRefundPage:        d.NeedRefundPage,
		CreatedAt:             d.CreatedAt,
		ClosedAt:              d.ClosedAt,
		AppealState:           d.AppealState,
		AppealDeadline:        d.AppealDeadline,
		CanAppeal:             d.CanAppeal,
		RefundTaskStatus:      d.RefundTaskStatus,
		RefundFailureMessage:  d.RefundFailureMessage,
		EstimatedRefundDate:   d.EstimatedRefundDate,
		BankDate:              d.BankDate,
		ProviderStatus:        d.GetProviderStatus(),
		ProviderMessage:       d.GetProviderMessage(),
		ProcessDeadline:       d.GetProcessDeadline(),
		AutoEscalated:         d.GetAutoEscalated(),
		ProcessDeadlineHours:  d.GetProcessDeadlineHours(),
		AutoConfirmAt:         d.GetAutoConfirmAt(),
		ReturnDeliveredAt:     d.GetReturnDeliveredAt(),
		ReturnProviderStatus:  d.GetReturnProviderStatus(),
		ReturnProviderMessage: d.GetReturnProviderMessage(),
		// 2026-05-22 逆物流退貨政策修訂欄位
		StoreArrivedAt:        d.GetStoreArrivedAt(),
		StorePickupDeadlineAt: d.GetStorePickupDeadlineAt(),
		SellerPickedUpAt:      d.GetSellerPickedUpAt(),
		AutoCompleteAtStamp:   d.GetAutoCompleteAtStamp(),
		// 2026-06-15 待寄回逾時政策修訂
		ShipBackDeadlineAt: d.GetShipBackDeadlineAt(),
	}
}

func mapResolutionType(s string) string {
	return s
}

// ==================== 買家端 Handler ====================

// CreateAfterSaleCase 申請售後
// @Summary      申請售後
// @Description  買家對已完成/已送達的訂單申請售後服務（退款/退貨退款）
// @Tags         售後 (AfterSales)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  CreateAfterSaleCaseRequest  true  "售後申請內容"
// @Success      200  {object}  object{code=int,message=string,data=AfterSaleCaseSummaryResponse}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Router       /app/after_sales [post]
func (h *Holder) CreateAfterSaleCase(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok || accountID == 0 {
		response.Unauthorized(c)
		return
	}
	buyerID, ok := h.resolveBuyerIDFor(c, accountID)
	if !ok {
		return
	}

	var req CreateAfterSaleCaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.Aftersales.CreateAfterSaleCase(c.Request.Context(), &pb.CreateAfterSaleCaseRequest{
		BuyerId:     int64(buyerID),
		OrderNo:     req.OrderNo,
		Reason:      req.Reason,
		Description: req.Description,
		PhotoTokens: req.PhotoTokens,
	})
	if err != nil {
		response.GrpcErr(c, "aftersales", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, convertAfterSaleSummary(resp.Data, h.MediaBaseURL))
}

// GetMyAfterSaleCases 我的售後列表
// @Summary      我的售後列表
// @Description  買家查詢自己的售後案件列表，支援分頁和狀態篩選
// @Tags         售後 (AfterSales)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page       query  string  false  "頁碼（預設 1）"
// @Param        page_size  query  string  false  "每頁筆數（預設 10，最大 50）"
// @Param        status     query  string  false  "狀態篩選（AfterSaleCaseStatus value，支援逗號分隔多值；合法值以 CodeMeta 業務狀態碼.md 為準）"
// @Success      200  {object}  object{code=int,message=string,data=object{records=[]AfterSaleCaseSummaryResponse,total=string,page=string,pageSize=string}}
// @Failure      401  {object}  object{code=int,message=string}
// @Router       /app/after_sales [get]
func (h *Holder) GetMyAfterSaleCases(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok || accountID == 0 {
		response.Unauthorized(c)
		return
	}

	buyerID, ok := h.resolveBuyerIDFor(c, accountID)
	if !ok {
		return
	}

	page := c.DefaultQuery("page", defaultPageStr)
	pageSize := c.DefaultQuery("page_size", defaultPageSizeStr)
	status := c.DefaultQuery("status", "")

	resp, err := h.Aftersales.GetMyAfterSaleCases(c.Request.Context(), &pb.GetMyAfterSaleCasesRequest{
		BuyerId:  int64(buyerID),
		Page:     page,
		PageSize: pageSize,
		Status:   status,
	})
	if err != nil {
		response.GrpcErr(c, "aftersales", err)
		return
	}

	data := resp.GetData()
	response.GrpcResult(c, resp.Code, resp.Message, response.PageData{
		Records:  convertAfterSaleSummaries(data.GetRecords(), h.MediaBaseURL),
		Total:    data.GetTotal(),
		Page:     data.GetPage(),
		PageSize: data.GetPageSize(),
	})
}

// GetAfterSaleCaseDetail 售後詳情
// @Summary      售後詳情
// @Description  買家查詢售後案件詳情（含明細、舉證、決策記錄）
// @Tags         售後 (AfterSales)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        case_no  path  string  true  "案件編號"
// @Success      200  {object}  object{code=int,message=string,data=AfterSaleCaseDetailResponse}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /app/after_sales/{case_no} [get]
func (h *Holder) GetAfterSaleCaseDetail(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok || accountID == 0 {
		response.Unauthorized(c)
		return
	}
	buyerID, ok := h.resolveBuyerIDFor(c, accountID)
	if !ok {
		return
	}

	caseNo := c.Param("case_no")
	if caseNo == "" {
		response.BadRequest(c, "case_no is required")
		return
	}

	resp, err := h.Aftersales.GetAfterSaleCaseDetail(c.Request.Context(), &pb.GetAfterSaleCaseDetailRequest{
		BuyerId: int64(buyerID),
		CaseNo:  caseNo,
	})
	if err != nil {
		response.GrpcErr(c, "aftersales", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, convertAfterSaleDetail(resp.Data, h.MediaBaseURL))
}

// SupplementEvidence 補件
// @Summary      補件
// @Description  買家為售後案件補充舉證照片或描述
// @Tags         售後 (AfterSales)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        case_no  path  string                     true  "案件編號"
// @Param        request  body  SupplementEvidenceRequest  true  "補件內容"
// @Success      200  {object}  object{code=int,message=string}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Router       /app/after_sales/{case_no}/supplement [post]
func (h *Holder) SupplementEvidence(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok || accountID == 0 {
		response.Unauthorized(c)
		return
	}
	buyerID, ok := h.resolveBuyerIDFor(c, accountID)
	if !ok {
		return
	}

	caseNo := c.Param("case_no")
	if caseNo == "" {
		response.BadRequest(c, "case_no is required")
		return
	}

	var req SupplementEvidenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.Aftersales.SupplementEvidence(c.Request.Context(), &pb.SupplementEvidenceRequest{
		BuyerId:               int64(buyerID),
		CaseNo:                caseNo,
		PhotoTokens:           req.PhotoTokens,
		AdditionalDescription: req.AdditionalDescription,
	})
	if err != nil {
		response.GrpcErr(c, "aftersales", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, nil)
}

// CancelAfterSaleCase 取消售後
// @Summary      取消售後
// @Description  買家取消尚未處理的售後案件
// @Tags         售後 (AfterSales)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        case_no  path  string  true  "案件編號"
// @Success      200  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Router       /app/after_sales/{case_no}/cancel [post]
func (h *Holder) CancelAfterSaleCase(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok || accountID == 0 {
		response.Unauthorized(c)
		return
	}
	buyerID, ok := h.resolveBuyerIDFor(c, accountID)
	if !ok {
		return
	}

	caseNo := c.Param("case_no")
	if caseNo == "" {
		response.BadRequest(c, "case_no is required")
		return
	}

	resp, err := h.Aftersales.CancelAfterSaleCase(c.Request.Context(), &pb.CancelAfterSaleCaseRequest{
		BuyerId: int64(buyerID),
		CaseNo:  caseNo,
	})
	if err != nil {
		response.GrpcErr(c, "aftersales", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, nil)
}

// GetRefundPageUrl ATM 退款頁面
// @Summary      取得 ATM 退款頁面 URL
// @Description  取得 PayUni ATM 退款頁面 URL，前端用 WebView 開啟讓買家填寫銀行資訊（售後退款 + 賣家取消出貨通用）
// @Tags         售後 (AfterSales)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        order_no  path  string  true  "訂單編號"
// @Success      200  {object}  object{code=int,message=string,data=RefundPageUrlDataResponse}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /app/orders/{order_no}/refund_page [post]
func (h *Holder) GetRefundPageUrl(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok || accountID == 0 {
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

	// 容許空 body（既有呼叫端可能未帶 body），僅綁定可解析的欄位
	var req GetRefundPageUrlRequest
	_ = c.ShouldBindJSON(&req)

	resp, err := h.Aftersales.GetRefundPageUrl(c.Request.Context(), &pb.GetRefundPageUrlRequest{
		BuyerId:    int64(buyerID),
		OrderNo:    orderNo,
		ClientType: req.ClientType,
		AppType:    codemeta.AppTypeFromAccountType(getAccountType(c)),
	})
	if err != nil {
		response.GrpcErr(c, "aftersales", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, RefundPageUrlDataResponse{
		RedirectUrl: resp.GetData().GetRedirectUrl(),
	})
}

// ==================== 賣家端 Handler ====================

// GetSellerAfterSaleCases 賣家售後列表
// @Summary      賣家售後列表
// @Description  賣家查詢收到的售後案件列表，支援分頁和狀態篩選
// @Tags         售後 (AfterSales)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page       query  string  false  "頁碼（預設 1）"
// @Param        page_size  query  string  false  "每頁筆數（預設 10，最大 50）"
// @Param        status     query  string  false  "狀態篩選（AfterSaleCaseStatus value，支援逗號分隔多值；合法值以 CodeMeta 業務狀態碼.md 為準）"
// @Success      200  {object}  object{code=int,message=string,data=object{records=[]AfterSaleCaseSummaryResponse,total=string,page=string,pageSize=string}}
// @Failure      401  {object}  object{code=int,message=string}
// @Router       /app/seller/after_sales [get]
func (h *Holder) GetSellerAfterSaleCases(c *gin.Context) {
	sellerID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	page := c.DefaultQuery("page", defaultPageStr)
	pageSize := c.DefaultQuery("page_size", defaultPageSizeStr)
	status := c.DefaultQuery("status", "")

	resp, err := h.Aftersales.GetSellerAfterSaleCases(c.Request.Context(), &pb.GetSellerAfterSaleCasesRequest{
		StoreId:  int64(sellerID),
		Page:     page,
		PageSize: pageSize,
		Status:   status,
	})
	if err != nil {
		response.GrpcErr(c, "aftersales", err)
		return
	}

	data := resp.GetData()
	response.GrpcResult(c, resp.Code, resp.Message, response.PageData{
		Records:  convertAfterSaleSummaries(data.GetRecords(), h.MediaBaseURL),
		Total:    data.GetTotal(),
		Page:     data.GetPage(),
		PageSize: data.GetPageSize(),
	})
}

// GetSellerAfterSaleCaseDetail 售後詳情（賣家視角）
// @Summary      售後詳情（賣家視角）
// @Description  賣家查詢售後案件詳情（含明細、舉證、決策記錄）
// @Tags         售後 (AfterSales)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        case_no  path  string  true  "案件編號"
// @Success      200  {object}  object{code=int,message=string,data=AfterSaleCaseDetailResponse}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /app/seller/after_sales/{case_no} [get]
func (h *Holder) GetSellerAfterSaleCaseDetail(c *gin.Context) {
	sellerID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	caseNo := c.Param("case_no")
	if caseNo == "" {
		response.BadRequest(c, "case_no is required")
		return
	}

	resp, err := h.Aftersales.GetSellerAfterSaleCaseDetail(c.Request.Context(), &pb.GetSellerAfterSaleCaseDetailRequest{
		StoreId: int64(sellerID),
		CaseNo:  caseNo,
	})
	if err != nil {
		response.GrpcErr(c, "aftersales", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, convertAfterSaleDetail(resp.Data, h.MediaBaseURL))
}

// ResolveAfterSaleCase 提交決策
// @Summary      提交售後決策
// @Description  賣家對售後案件提交處理決策（退貨退款/部分退款/全額免退/優惠券補償/拒絕）
// @Tags         售後 (AfterSales)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        case_no  path  string                        true  "案件編號"
// @Param        request  body  ResolveAfterSaleCaseRequest   true  "決策內容"
// @Success      200  {object}  object{code=int,message=string,data=ResolveResultResponse}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Router       /app/seller/after_sales/{case_no}/resolve [post]
func (h *Holder) ResolveAfterSaleCase(c *gin.Context) {
	sellerID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	caseNo := c.Param("case_no")
	if caseNo == "" {
		response.BadRequest(c, "case_no is required")
		return
	}

	var req ResolveAfterSaleCaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.Aftersales.ResolveAfterSaleCase(c.Request.Context(), &pb.ResolveAfterSaleCaseRequest{
		StoreId:              int64(sellerID),
		CaseNo:               caseNo,
		ResolutionType:       mapResolutionType(req.ResolutionType),
		RefundAmount:         req.RefundAmount,
		ReturnShippingMethod: req.ReturnShippingMethod,
		CouponAmount:         req.CouponAmount,
		CouponMinSpend:       req.CouponMinSpend,
		CouponMessage:        req.CouponMessage,
		RejectReason:         req.RejectReason,
		RejectMessage:        req.RejectMessage,
		NegotiationReason:    req.NegotiationReason,
	})
	if err != nil {
		response.GrpcErr(c, "aftersales", err)
		return
	}

	d := resp.GetData()
	response.GrpcResult(c, resp.Code, resp.Message, ResolveResultResponse{
		CaseNo:           d.GetCaseNo(),
		Status:           d.GetStatus(),
		ReturnShipmentNo: d.GetReturnShipmentNo(),
		NeedRefundPage:   d.GetNeedRefundPage(),
	})
}

// ConfirmReturnReceipt 確認收貨（含選填驗貨扣款）
// @Summary      確認收到退貨（含驗貨扣款）
// @Description  賣家確認已收到買家退回的商品。可選填實際退款金額（驗貨扣款）、扣款原因、驗貨照片、備註。詳見設計書 §5.3.3 / §2.5.2。
// @Description  空 body = 無瑕疵退全額（向後相容）；若傳 actualRefundAmount 則必須 > 0 且 ≤ 決議階段金額，<決議金額時 deductionReason 必填。
// @Tags         售後 (AfterSales)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        case_no  path  string                       true   "案件編號"
// @Param        request  body  ConfirmReturnReceiptRequest  false  "驗貨扣款參數（全為選填，未傳 = 全額退款）"
// @Success      200  {object}  object{code=int,message=string}
// @Failure      400  {object}  object{code=int,message=string}  "參數錯誤（actualRefundAmount ≤ 0 或 deductionReason 缺失）"
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}  "9010 無權操作此售後單"
// @Failure      404  {object}  object{code=int,message=string}  "9001 售後單不存在"
// @Failure      409  {object}  object{code=int,message=string}  "9004 售後單狀態不允許此操作 / 9006 退款金額超過可退款上限"
// @Router       /app/seller/after_sales/{case_no}/confirm_receipt [post]
func (h *Holder) ConfirmReturnReceipt(c *gin.Context) {
	sellerID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	caseNo := c.Param("case_no")
	if caseNo == "" {
		response.BadRequest(c, "case_no is required")
		return
	}

	// optional body：空 body 走全額退款路徑（向後相容）
	var req ConfirmReturnReceiptRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
	}

	resp, err := h.Aftersales.ConfirmReturnReceipt(c.Request.Context(), &pb.ConfirmReturnReceiptRequest{
		StoreId:               int64(sellerID),
		CaseNo:                caseNo,
		ActualRefundAmount:    req.ActualRefundAmount,
		DeductionReason:       req.DeductionReason,
		InspectionPhotoTokens: req.InspectionPhotoTokens,
		InspectionNote:        req.InspectionNote,
	})
	if err != nil {
		response.GrpcErr(c, "aftersales", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, nil)
}

// RetryPrintReturnLabel admin 人工重試 7-11 退貨列印（§5.3.6 ops 介入用）
// @Summary      人工重試 7-11 退貨出貨便列印（Admin）
// @Description  用於系統自動重試仍未取得出貨便代碼時的平台管理員介入。重置 label_retry_count 並再次觸發 PayUni print_label。
// @Description  僅允許 7-11 退貨（returnShippingMethod=3002）、狀態 20/25、尚未回填代碼（partnerId 為空）的案件使用。
// @Tags         售後 (AfterSales)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        case_no  path  string  true  "案件編號"
// @Success      200  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 token 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非管理員（admin access required）"
// @Failure      404  {object}  object{code=int,message=string}  "9001 售後單不存在"
// @Failure      409  {object}  object{code=int,message=string}  "9013 出貨便代碼已產生，無需重試 / 9014 售後案件狀態不允許重試列印"
// @Router       /admin/after_sales/{case_no}/retry_print_return_label [post]
func (h *Holder) RetryPrintReturnLabel(c *gin.Context) {
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

	resp, err := h.Aftersales.RetryPrintReturnLabel(c.Request.Context(), &pb.RetryPrintReturnLabelRequest{
		AccountId: accountID,
		CaseNo:    caseNo,
	})
	if err != nil {
		response.GrpcErr(c, "aftersales", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, nil)
}

// CancelShipment 取消出貨
// @Summary      賣家取消出貨
// @Description  賣家取消已付款未出貨的訂單（僅限 status=10(已付款) / 20(待出貨) 狀態），自動啟動退款流程
// @Tags         售後 (AfterSales)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        order_no  path  string                  true  "訂單編號"
// @Param        request   body  CancelShipmentRequest   true  "取消原因"
// @Success      200  {object}  object{code=int,message=string,data=CancelShipmentResultResponse}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Router       /app/orders/{order_no}/seller_cancellations [post]
func (h *Holder) CancelShipment(c *gin.Context) {
	sellerID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	orderNo := c.Param("order_no")
	if orderNo == "" {
		response.BadRequest(c, "order_no is required")
		return
	}

	var req CancelShipmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.Aftersales.CancelShipment(c.Request.Context(), &pb.CancelShipmentRequest{
		StoreId: int64(sellerID),
		OrderNo: orderNo,
		Reason:  req.Reason,
		Remark:  req.Remark,
	})
	if err != nil {
		response.GrpcErr(c, "aftersales", err)
		return
	}

	d := resp.GetData()
	response.GrpcResult(c, resp.Code, resp.Message, CancelShipmentResultResponse{
		OrderNo:        d.GetOrderNo(),
		RefundAmount:   d.GetRefundAmount(),
		NeedRefundPage: d.GetNeedRefundPage(),
	})
}

// ==================== ATM 退款回調 ====================

// AtmRefundReturn ATM 退款頁面完成回調（PayUni Form POST）
// PayUni ATM 退款頁面完成後以 Form POST 將 EncryptInfo + HashInfo 導回此端點
// Gateway 提取加密參數 → AfterSales gRPC 解密 + 更新退款任務 → 302 redirect Deep Link
// @Summary      ATM 退款頁面回調
// @Description  PayUni ATM 退款頁面完成後的 Form POST 回調，解密退款結果、更新退款任務、302 redirect 到 Deep Link 導回 App
// @Tags         售後 (AfterSales)
// @Accept       x-www-form-urlencoded
// @Param        EncryptInfo  formData  string  true  "PayUni 加密資料"
// @Param        HashInfo     formData  string  true  "PayUni 簽章"
// @Success      302  "Redirect to Deep Link"
// @Failure      400  {string}  string  "參數錯誤"
// @Failure      503  {string}  string  "服務不可用"
// @Router       /atm-refund/return [post]
func (h *Holder) AtmRefundReturn(c *gin.Context) {
	encryptInfo := c.PostForm("EncryptInfo")
	hashInfo := c.PostForm("HashInfo")

	if encryptInfo == "" || hashInfo == "" {
		c.String(http.StatusBadRequest, "缺少加密參數")
		return
	}

	// return_url 帶了 ?order_no=xxx 作 fallback：PayUni 失敗時 MerTradeNo 可能為空，
	// AfterSales 需靠 order_no lookup refund_task 才能寫 payuni_status/message snapshot
	orderNo := c.Query("order_no")

	resp, err := h.Aftersales.HandleRefundPageReturn(c.Request.Context(), &pb.HandleRefundPageReturnRequest{
		EncryptInfo: encryptInfo,
		HashInfo:    hashInfo,
		OrderNo:     orderNo,
	})
	if err != nil {
		c.String(http.StatusServiceUnavailable, "處理退款結果失敗")
		return
	}

	// 302 redirect 到 AfterSales Service 組裝的 Deep Link
	c.Redirect(http.StatusFound, resp.GetRedirectUrl())
}

// 避免 import 未使用警告
var _ = strconv.Itoa

// ==================== §10 申訴與平台裁決 ====================

// SubmitAppealRequest 買家申訴 Request
type SubmitAppealRequest struct {
	Reason              string   `json:"reason" binding:"required,max=128"`   // 申訴理由分類或簡述
	Description         string   `json:"description" binding:"max=1000"`      // 詳細說明（選填）
	EvidencePhotoTokens []string `json:"evidencePhotoTokens" binding:"max=5"` // Media stagedToken，最多 5 張
}

// SubmitAppealResponse 買家申訴回應
type SubmitAppealResponse struct {
	AppealId    string `json:"appealId"`    // = caseNo（一案一申訴）
	AppealState string `json:"appealState"` // APPEALED=10
}

// ArbitrationResponse 裁決查詢回應（買賣家共用）
type ArbitrationResponse struct {
	Decision *PlatformDecisionResponse `json:"decision,omitempty"`
	State    string                    `json:"state"`
	Timeline []AppealEventResponse     `json:"timeline"`
}

// AppealEventResponse 申訴事件項目
type AppealEventResponse struct {
	EventType   string `json:"eventType"` // APPEAL_SUBMITTED=1 / DECISION_MADE=2
	Timestamp   string `json:"timestamp"` // ISO 8601 UTC
	Label       string `json:"label"`
	Description string `json:"description"`
}

// PlatformDecisionResponse 平台裁決結果
type PlatformDecisionResponse struct {
	Outcome      string `json:"outcome"`                // FULL_REFUND=1 / PARTIAL_REFUND=2 / REJECT_APPEAL=3
	RefundAmount string `json:"refundAmount,omitempty"` // 僅 PARTIAL_REFUND 有值
	Reason       string `json:"reason"`
	DecidedAt    string `json:"decidedAt"` // ISO 8601 UTC
}

// AppealSummaryResponse Admin 列表摘要
type AppealSummaryResponse struct {
	CaseNo            string `json:"caseNo"`
	OrderNo           string `json:"orderNo"`
	BuyerId           string `json:"buyerId"`
	StoreId           string `json:"storeId"`
	AppealState       string `json:"appealState"` // 申訴狀態（AfterSaleAppealState value：10=已申訴 / 20=平台調查中 / 30=已裁決）
	AppealSubmittedAt string `json:"appealSubmittedAt"`
	LastRejectedAt    string `json:"lastRejectedAt"`
	RefundableAmount  string `json:"refundableAmount"`
	// §10.6.6 Profile 欄位（降級：查詢失敗回空字串，不 fail 整個請求）
	BuyerName       string `json:"buyerName"`
	BuyerAvatarKey  string `json:"avatarKey"` // 買家頭像 objectKey（SSOT；49 §6.1）
	BuyerAvatarUrl  string `json:"avatarUrl"` // 買家頭像顯示 URL（Gateway 組）
	SellerName      string `json:"sellerName"`
	SellerAvatarKey string `json:"logoKey"` // 商家 logo objectKey（SSOT；49 §6.1）
	SellerAvatarUrl string `json:"logoUrl"` // 商家 logo 顯示 URL（Gateway 組）
}

// ==================== §10.6.6 Admin 單案詳情 DTO ====================

// AdminAppealDetailResponse Admin 單案詳情四層結構（§29 加 providerContext）
type AdminAppealDetailResponse struct {
	ProviderContext *AdminProviderContextResponse `json:"providerContext,omitempty"`
	AdminContext    AdminContextResponse          `json:"adminContext"`
	Arbitration     ArbitrationResponse           `json:"arbitration"`
	CaseDetail      AfterSaleCaseDetailResponse   `json:"caseDetail"`
}

// AdminProviderContextResponse §29 Admin 對帳 provider 資訊
type AdminProviderContextResponse struct {
	ProviderSource   string `json:"providerSource"`             // CodeMeta ProviderSource 子集：PAYMENT_REFUND / SHIPPING；"" = 無第三方環節可對帳
	ProviderStatus   string `json:"providerStatus"`             // PayUni 原始狀態碼
	ProviderMessage  string `json:"providerMessage"`            // PayUni 原始訊息
	ProviderRefundNo string `json:"providerRefundNo,omitempty"` // 僅 PAYMENT_REFUND 有值
	ShipTradeNo      string `json:"shipTradeNo,omitempty"`      // 僅 SHIPPING 有值
}

// AdminContextResponse Admin-only 欄位
type AdminContextResponse struct {
	DecidedByAdminId       string `json:"decidedByAdminId,omitempty"` // 僅 RESOLVED 有值
	BuyerAppealReason      string `json:"buyerAppealReason"`          // 從 aftersale_cases.appeal_reason
	BuyerAppealDescription string `json:"buyerAppealDescription"`     // 從 aftersale_cases.appeal_description
	SellerRejectReason     string `json:"sellerRejectReason"`         // 最新一筆 REJECT resolution.reject_reason（AfterSaleRejectReason value）
	SellerRejectMessage    string `json:"sellerRejectMessage"`        // 最新一筆 REJECT resolution.reject_message
	BuyerName              string `json:"buyerName"`                  // 快照
	BuyerAvatarKey         string `json:"avatarKey"`                  // 買家頭像 objectKey（SSOT；49 §6.1）
	BuyerAvatarUrl         string `json:"avatarUrl"`                  // 買家頭像顯示 URL（Gateway 組）
	SellerName             string `json:"sellerName"`                 // live lookup 降級
	SellerAvatarKey        string `json:"logoKey"`                    // 商家 logo objectKey（SSOT；49 §6.1）
	SellerAvatarUrl        string `json:"logoUrl"`                    // 商家 logo 顯示 URL（Gateway 組）
}

// DecideAppealRequest Admin 裁決 Request
type DecideAppealRequest struct {
	Outcome      string `json:"outcome" binding:"required"`         // 合法值: FULL_REFUND / PARTIAL_REFUND / REJECT_APPEAL
	RefundAmount string `json:"refundAmount,omitempty"`             // 僅 PARTIAL_REFUND 必填，整數字串；0 < amount ≤ refundableAmount
	Reason       string `json:"reason" binding:"required,max=1000"` // 裁決理由（對買賣家可見）
}

// DecideAppealResponse Admin 裁決回應
type DecideAppealResponse struct {
	CaseNo       string `json:"caseNo"`
	Outcome      string `json:"outcome"`
	RefundAmount string `json:"refundAmount,omitempty"`
}

// convertArbitration 組裝買家 / 賣家的 arbitration response
func convertArbitration(data *pb.ArbitrationData) ArbitrationResponse {
	if data == nil {
		return ArbitrationResponse{}
	}
	timeline := make([]AppealEventResponse, 0, len(data.GetTimeline()))
	for _, ev := range data.GetTimeline() {
		timeline = append(timeline, AppealEventResponse{
			EventType:   ev.GetEventType(),
			Timestamp:   ev.GetTimestamp(),
			Label:       ev.GetLabel(),
			Description: ev.GetDescription(),
		})
	}

	resp := ArbitrationResponse{
		State:    data.GetState(),
		Timeline: timeline,
	}

	// state=RESOLVED 時 decision 才有效（proto3 預設值判斷：outcome 非空字串）
	if d := data.GetDecision(); d != nil && d.GetOutcome() != "" {
		resp.Decision = &PlatformDecisionResponse{
			Outcome:      d.GetOutcome(),
			RefundAmount: d.GetRefundAmount(),
			Reason:       d.GetReason(),
			DecidedAt:    d.GetDecidedAt(),
		}
	}
	return resp
}

// SubmitAppeal 買家提交申訴
// @Summary      提交申訴
// @Description  買家對已被賣家拒絕的售後案件向平台提交申訴，72 小時內限一次
// @Tags         售後 (AfterSales)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        case_no  path  string               true  "案件編號"
// @Param        body     body  SubmitAppealRequest  true  "申訴請求"
// @Success      200  {object}  object{code=int,message=string,data=SubmitAppealResponse}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      409  {object}  object{code=int,message=string}
// @Router       /app/after_sales/{case_no}/appeal [post]
func (h *Holder) SubmitAppeal(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok || accountID == 0 {
		response.Unauthorized(c)
		return
	}
	buyerID, ok := h.resolveBuyerIDFor(c, accountID)
	if !ok {
		return
	}
	caseNo := c.Param("case_no")
	if caseNo == "" {
		response.BadRequest(c, "case_no is required")
		return
	}
	var req SubmitAppealRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	resp, err := h.Aftersales.SubmitAppeal(c.Request.Context(), &pb.SubmitAppealRequest{
		BuyerId:             int64(buyerID),
		CaseNo:              caseNo,
		Reason:              req.Reason,
		Description:         req.Description,
		EvidencePhotoTokens: req.EvidencePhotoTokens,
	})
	if err != nil {
		response.GrpcErr(c, "aftersales", err)
		return
	}
	d := resp.GetData()
	response.GrpcResult(c, resp.GetCode(), resp.GetMessage(), SubmitAppealResponse{
		AppealId:    d.GetAppealId(),
		AppealState: d.GetAppealState(),
	})
}

// GetArbitration 買家查詢裁決
// @Summary      查詢申訴裁決
// @Description  買家查詢申訴的 timeline 與裁決結果
// @Tags         售後 (AfterSales)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        case_no  path  string  true  "案件編號"
// @Success      200  {object}  object{code=int,message=string,data=ArbitrationResponse}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      409  {object}  object{code=int,message=string}
// @Router       /app/after_sales/{case_no}/arbitration [get]
func (h *Holder) GetArbitration(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok || accountID == 0 {
		response.Unauthorized(c)
		return
	}
	buyerID, ok := h.resolveBuyerIDFor(c, accountID)
	if !ok {
		return
	}
	caseNo := c.Param("case_no")
	if caseNo == "" {
		response.BadRequest(c, "case_no is required")
		return
	}
	resp, err := h.Aftersales.GetArbitration(c.Request.Context(), &pb.GetArbitrationRequest{
		BuyerId: int64(buyerID),
		CaseNo:  caseNo,
	})
	if err != nil {
		response.GrpcErr(c, "aftersales", err)
		return
	}
	response.GrpcResult(c, resp.GetCode(), resp.GetMessage(), convertArbitration(resp.GetData()))
}

// GetSellerArbitration 賣家查詢裁決（read-only）
// @Summary      賣家查詢申訴裁決
// @Description  賣家 read-only 查看自己售後案件的申訴狀態與裁決結果
// @Tags         售後-賣家 (AfterSales-Seller)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        case_no  path  string  true  "案件編號"
// @Success      200  {object}  object{code=int,message=string,data=ArbitrationResponse}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      409  {object}  object{code=int,message=string}
// @Router       /app/seller/after_sales/{case_no}/arbitration [get]
func (h *Holder) GetSellerArbitration(c *gin.Context) {
	sellerID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}
	caseNo := c.Param("case_no")
	if caseNo == "" {
		response.BadRequest(c, "case_no is required")
		return
	}
	resp, err := h.Aftersales.GetSellerArbitration(c.Request.Context(), &pb.GetSellerArbitrationRequest{
		StoreId: int64(sellerID),
		CaseNo:  caseNo,
	})
	if err != nil {
		response.GrpcErr(c, "aftersales", err)
		return
	}
	response.GrpcResult(c, resp.GetCode(), resp.GetMessage(), convertArbitration(resp.GetData()))
}

// appealStateFilterAll 代表不限 appeal_state（含已裁決）；非 CodeMeta 值，是本端點專屬的 filter 關鍵字。
const appealStateFilterAll = "all"

// allowedAppealStateFilters 是 /admin/after_sales/appeals 的 appeal_state 合法值集合。
// 一律使用 AfterSaleAppealState 的 Value——回應欄位 appealState 與 DB 都是 Value，
// 請求端沒有理由用 Name（收 Name 回 Value 的不對稱正是 #2104 的成因）。
// NONE(0) 不在集合內：申訴列表本身就排除未申訴案件。
var allowedAppealStateFilters = map[string]struct{}{
	"":                                    {}, // 未帶 → 由 aftersales-service 套預設 APPEALED(10)
	codemeta.AfterSaleAppealStateAppealed: {},
	codemeta.AfterSaleAppealStateInvestigating: {},
	codemeta.AfterSaleAppealStateResolved:      {},
	appealStateFilterAll:                       {},
}

// ListAppealsForAdmin Admin 待裁決列表
// @Summary      Admin 待裁決列表
// @Description  Admin 查詢待裁決的申訴案件列表，預設 appeal_state=10（已申訴）
// @Tags         售後-Admin (AfterSales-Admin)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        appeal_state  query  string  false  "AfterSaleAppealState value: 10=已申訴（預設）/ 20=平台調查中 / 30=已裁決 / all=不限"
// @Param        page          query  string  false  "頁碼，預設 1"
// @Param        page_size     query  string  false  "每頁筆數，預設 20，最大 100"
// @Success      200  {object}  object{code=int,message=string,data=object{records=[]AppealSummaryResponse,total=string,page=string,pageSize=string}}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Router       /admin/after_sales/appeals [get]
func (h *Holder) ListAppealsForAdmin(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok || accountID == 0 {
		response.Unauthorized(c)
		return
	}
	appealState := c.DefaultQuery("appeal_state", "")
	if _, valid := allowedAppealStateFilters[appealState]; !valid {
		response.BadRequest(c, "invalid appeal_state: must be one of 10 / 20 / 30 / all")
		return
	}
	page := c.DefaultQuery("page", defaultPageStr)
	pageSize := c.DefaultQuery("page_size", defaultPageSizeStr)

	resp, err := h.Aftersales.ListAppealsForAdmin(c.Request.Context(), &pb.ListAppealsForAdminRequest{
		AccountId:   accountID,
		AppealState: appealState,
		Page:        page,
		PageSize:    pageSize,
	})
	if err != nil {
		response.GrpcErr(c, "aftersales", err)
		return
	}
	data := resp.GetData()
	records := make([]AppealSummaryResponse, 0, len(data.GetRecords()))
	for _, r := range data.GetRecords() {
		records = append(records, AppealSummaryResponse{
			CaseNo:            r.GetCaseNo(),
			OrderNo:           r.GetOrderNo(),
			BuyerId:           r.GetBuyerId(),
			StoreId:           r.GetStoreId(),
			AppealState:       r.GetAppealState(),
			AppealSubmittedAt: r.GetAppealSubmittedAt(),
			LastRejectedAt:    r.GetLastRejectedAt(),
			RefundableAmount:  r.GetRefundableAmount(),
			BuyerName:         r.GetBuyerName(),
			BuyerAvatarKey:    r.GetBuyerAvatarKey(),
			BuyerAvatarUrl:    buildURL(h.MediaBaseURL, r.GetBuyerAvatarKey()),
			SellerName:        r.GetSellerName(),
			SellerAvatarKey:   r.GetSellerAvatarKey(),
			SellerAvatarUrl:   buildURL(h.MediaBaseURL, r.GetSellerAvatarKey()),
		})
	}
	response.GrpcResult(c, resp.GetCode(), resp.GetMessage(), response.PageData{
		Records:  records,
		Total:    data.GetTotal(),
		Page:     data.GetPage(),
		PageSize: data.GetPageSize(),
	})
}

// DecideAppealForAdmin Admin 裁決
// @Summary      Admin 裁決申訴
// @Description  Admin 對申訴案件做出裁決（FULL_REFUND / PARTIAL_REFUND / REJECT_APPEAL）
// @Tags         售後-Admin (AfterSales-Admin)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        case_no  path  string               true  "案件編號"
// @Param        body     body  DecideAppealRequest  true  "裁決請求"
// @Success      200  {object}  object{code=int,message=string,data=DecideAppealResponse}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      409  {object}  object{code=int,message=string}
// @Router       /admin/after_sales/{case_no}/decide [post]
func (h *Holder) DecideAppealForAdmin(c *gin.Context) {
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
	var req DecideAppealRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	resp, err := h.Aftersales.DecideAppealForAdmin(c.Request.Context(), &pb.DecideAppealForAdminRequest{
		AccountId:    accountID,
		CaseNo:       caseNo,
		Outcome:      req.Outcome,
		RefundAmount: req.RefundAmount,
		Reason:       req.Reason,
	})
	if err != nil {
		response.GrpcErr(c, "aftersales", err)
		return
	}
	d := resp.GetData()
	response.GrpcResult(c, resp.GetCode(), resp.GetMessage(), DecideAppealResponse{
		CaseNo:       d.GetCaseNo(),
		Outcome:      d.GetOutcome(),
		RefundAmount: d.GetRefundAmount(),
	})
}

// GetAppealDetailForAdmin §10.6.6 Admin 單案詳情
// @Summary      Admin 單案詳情
// @Description  Admin 查詢單一售後案件完整資訊，含買家申訴理由 / 證據照片 / 賣家原始拒絕理由等裁決依據。查詢範圍不限 appeal_state。
// @Tags         售後-Admin (AfterSales-Admin)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        case_no  path  string  true  "案件編號"
// @Success      200  {object}  object{code=int,message=string,data=AdminAppealDetailResponse}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /admin/after_sales/{case_no} [get]
func (h *Holder) GetAppealDetailForAdmin(c *gin.Context) {
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
	resp, err := h.Aftersales.GetAppealDetailForAdmin(c.Request.Context(), &pb.GetAppealDetailForAdminRequest{
		AccountId: accountID,
		CaseNo:    caseNo,
	})
	if err != nil {
		response.GrpcErr(c, "aftersales", err)
		return
	}
	data := resp.GetData()
	if data == nil {
		response.GrpcResult(c, resp.GetCode(), resp.GetMessage(), AdminAppealDetailResponse{})
		return
	}

	detail := AdminAppealDetailResponse{
		CaseDetail:  convertAfterSaleDetail(data.GetCaseDetail(), h.MediaBaseURL),
		Arbitration: convertArbitration(data.GetArbitration()),
	}
	if ac := data.GetAdminContext(); ac != nil {
		detail.AdminContext = AdminContextResponse{
			DecidedByAdminId:       ac.GetDecidedByAdminId(),
			BuyerAppealReason:      ac.GetBuyerAppealReason(),
			BuyerAppealDescription: ac.GetBuyerAppealDescription(),
			SellerRejectReason:     ac.GetSellerRejectReason(),
			SellerRejectMessage:    ac.GetSellerRejectMessage(),
			BuyerName:              ac.GetBuyerName(),
			BuyerAvatarKey:         ac.GetBuyerAvatarKey(),
			BuyerAvatarUrl:         buildURL(h.MediaBaseURL, ac.GetBuyerAvatarKey()),
			SellerName:             ac.GetSellerName(),
			SellerAvatarKey:        ac.GetSellerAvatarKey(),
			SellerAvatarUrl:        buildURL(h.MediaBaseURL, ac.GetSellerAvatarKey()),
		}
	}
	if pc := data.GetProviderContext(); pc != nil && pc.GetProviderSource() != "" {
		detail.ProviderContext = &AdminProviderContextResponse{
			ProviderSource:   pc.GetProviderSource(),
			ProviderStatus:   pc.GetProviderStatus(),
			ProviderMessage:  pc.GetProviderMessage(),
			ProviderRefundNo: pc.GetProviderRefundNo(),
			ShipTradeNo:      pc.GetShipTradeNo(),
		}
	}
	response.GrpcResult(c, resp.GetCode(), resp.GetMessage(), detail)
}
