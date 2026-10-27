package handler

import (
	"github.com/gin-gonic/gin"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	"github.com/Will413028/hawkr/gateway/internal/response"
	pb "github.com/Will413028/hawkr/go-pkg/proto/ai"
)

// idempotencyKeyHeader 是 #2538 四支寫入命令的穩定冪等鍵傳輸位置（wire 契約 §5.1，
// v6.0 V.11.5 逐字凍結）。刻意不放 body：body 收斂成剛好那幾個 CAS 欄位之後，
// 「client 不得傳金額」就是結構上不可能，而不是靠一條驗證規則守著。
//
// ⚠️ 既有 procurement 端點（rfq-price-previews／rfq-confirmations／rfq-publications）
// 維持 clientRequestId 放 body 不動——改它們是對已上線 client 的破壞性變更。
const idempotencyKeyHeader = "Idempotency-Key"

// requireIdempotencyKey 取出並正規化冪等鍵。缺 header 回 400，語意與既有命令的
// 「clientRequestId is required」一致。
func requireIdempotencyKey(c *gin.Context) (string, bool) {
	key := c.GetHeader(idempotencyKeyHeader)
	if key == "" {
		badProcurementRequest(c, "Idempotency-Key header is required")
		return "", false
	}
	return key, true
}

// ---- 共用 payload DTO ----

type ProcurementOfferedQualityDTO struct {
	Grade           string `json:"grade"`
	InspectionNotes string `json:"inspectionNotes"`
}

type ProcurementUnitBasisDTO struct {
	Quantity string `json:"quantity"`
	Unit     string `json:"unit"`
}

// ProcurementSellerQuotePayloadDTO mirrors aipb.SellerQuotePayload — the 17 fields
// a seller quote carries. Amounts and quantities are decimal strings on the wire;
// the server, not the client, decides whether total equals goods+shipping+tax.
type ProcurementSellerQuotePayloadDTO struct {
	ResponseType        string                       `json:"responseType"`
	OfferedQuality      ProcurementOfferedQualityDTO `json:"offeredQuality"`
	UnitBasis           ProcurementUnitBasisDTO      `json:"unitBasis"`
	OfferedQuantity     string                       `json:"offeredQuantity"`
	OfferedQuantityUnit string                       `json:"offeredQuantityUnit"`
	ConversionBasis     string                       `json:"conversionBasis"`
	Currency            string                       `json:"currency"`
	GoodsAmount         string                       `json:"goodsAmount"`
	ShippingAmount      string                       `json:"shippingAmount"`
	TaxAmount           string                       `json:"taxAmount"`
	TaxInclusionBasis   string                       `json:"taxInclusionBasis"`
	TotalAmount         string                       `json:"totalAmount"`
	LeadTimeDays        string                       `json:"leadTimeDays"`
	PromisedDeliveryAt  string                       `json:"promisedDeliveryAt"`
	ValidUntil          string                       `json:"validUntil"`
	TermsNote           string                       `json:"termsNote"`
	DeviationDisclosure string                       `json:"deviationDisclosure"`
}

func (d ProcurementSellerQuotePayloadDTO) toProto() *pb.SellerQuotePayload {
	return &pb.SellerQuotePayload{
		ResponseType: d.ResponseType,
		OfferedQuality: &pb.OfferedQuality{
			Grade: d.OfferedQuality.Grade, InspectionNotes: d.OfferedQuality.InspectionNotes,
		},
		UnitBasis:           &pb.UnitBasis{Quantity: d.UnitBasis.Quantity, Unit: d.UnitBasis.Unit},
		OfferedQuantity:     d.OfferedQuantity,
		OfferedQuantityUnit: d.OfferedQuantityUnit,
		ConversionBasis:     d.ConversionBasis,
		Currency:            d.Currency,
		GoodsAmount:         d.GoodsAmount,
		ShippingAmount:      d.ShippingAmount,
		TaxAmount:           d.TaxAmount,
		TaxInclusionBasis:   d.TaxInclusionBasis,
		TotalAmount:         d.TotalAmount,
		LeadTimeDays:        d.LeadTimeDays,
		PromisedDeliveryAt:  d.PromisedDeliveryAt,
		ValidUntil:          d.ValidUntil,
		TermsNote:           d.TermsNote,
		DeviationDisclosure: d.DeviationDisclosure,
	}
}

func sellerQuotePayloadFromProto(p *pb.SellerQuotePayload) ProcurementSellerQuotePayloadDTO {
	if p == nil {
		return ProcurementSellerQuotePayloadDTO{}
	}
	return ProcurementSellerQuotePayloadDTO{
		ResponseType: p.GetResponseType(),
		OfferedQuality: ProcurementOfferedQualityDTO{
			Grade: p.GetOfferedQuality().GetGrade(), InspectionNotes: p.GetOfferedQuality().GetInspectionNotes(),
		},
		UnitBasis: ProcurementUnitBasisDTO{
			Quantity: p.GetUnitBasis().GetQuantity(), Unit: p.GetUnitBasis().GetUnit(),
		},
		OfferedQuantity:     p.GetOfferedQuantity(),
		OfferedQuantityUnit: p.GetOfferedQuantityUnit(),
		ConversionBasis:     p.GetConversionBasis(),
		Currency:            p.GetCurrency(),
		GoodsAmount:         p.GetGoodsAmount(),
		ShippingAmount:      p.GetShippingAmount(),
		TaxAmount:           p.GetTaxAmount(),
		TaxInclusionBasis:   p.GetTaxInclusionBasis(),
		TotalAmount:         p.GetTotalAmount(),
		LeadTimeDays:        p.GetLeadTimeDays(),
		PromisedDeliveryAt:  p.GetPromisedDeliveryAt(),
		ValidUntil:          p.GetValidUntil(),
		TermsNote:           p.GetTermsNote(),
		DeviationDisclosure: p.GetDeviationDisclosure(),
	}
}

// ---- 賣家面 request／response DTO ----

type ProcurementSellerResponseDraftRequest struct {
	ExpectedDraftVersion string                           `json:"expectedDraftVersion"`
	Draft                ProcurementSellerQuotePayloadDTO `json:"draft"`
}

type ProcurementSellerResponseDraftResponseDTO struct {
	DraftVersion string                           `json:"draftVersion"`
	UpdatedAt    string                           `json:"updatedAt"`
	Draft        ProcurementSellerQuotePayloadDTO `json:"draft"`
}

type ProcurementSubmitSellerResponseRequest struct {
	ExpectedPublicationVersion      string                           `json:"expectedPublicationVersion"`
	ExpectedSellerFacingPayloadHash string                           `json:"expectedSellerFacingPayloadHash"`
	ExpectedSellerResponseVersion   string                           `json:"expectedSellerResponseVersion"`
	Quote                           ProcurementSellerQuotePayloadDTO `json:"quote"`
}

type ProcurementWithdrawSellerResponseRequest struct {
	ExpectedSellerResponseVersion string `json:"expectedSellerResponseVersion"`
	ReasonCode                    string `json:"reasonCode"`
}

type ProcurementDeclineRfqPublicationRequest struct {
	ReasonCode string `json:"reasonCode"`
}

type ProcurementSellerPublicationDTO struct {
	PublicationID           string `json:"publicationId"`
	PublicationVersion      string `json:"publicationVersion"`
	InviteID                string `json:"inviteId"`
	Lane                    string `json:"lane"`
	SellerFacingPayloadHash string `json:"sellerFacingPayloadHash"`
	DeliveryState           string `json:"deliveryState"`
	PublishedAt             string `json:"publishedAt"`
}

// ProcurementSellerPublicationListItemDTO 與詳情同形：發布摘要 ＋ 需求內容。
// 一串不帶品名的 publication id 對賣家沒有用處。
type ProcurementSellerPublicationListItemDTO struct {
	Publication          ProcurementSellerPublicationDTO   `json:"publication"`
	SellerFacingPayload  ProcurementSellerFacingPayloadDTO `json:"sellerFacingPayload"`
	LatestResponseStatus string                            `json:"latestResponseStatus"`
	HasDraft             bool                              `json:"hasDraft"`
}

type ProcurementSellerPublicationListDTO struct {
	Publications []ProcurementSellerPublicationListItemDTO `json:"publications"`
}

// ProcurementSellerPublicationDetailDTO carries the publication plus THIS seller's
// own draft and current submitted version — the only source for the CAS fields
// (expectedDraftVersion / expectedSellerResponseVersion). Both are omitted when the
// seller has nothing yet. ⛔ Never any other seller's content.
type ProcurementSellerPublicationDetailDTO struct {
	Publication ProcurementSellerPublicationDTO `json:"publication"`
	// SellerFacingPayload 是賣家在報什麼價。少了它這張表單是空白的。
	SellerFacingPayload ProcurementSellerFacingPayloadDTO          `json:"sellerFacingPayload"`
	Draft               *ProcurementSellerResponseDraftResponseDTO `json:"draft,omitempty"`
	LatestResponse      *ProcurementSellerResponseVersionDTO       `json:"latestResponse,omitempty"`
}

type ProcurementSellerRequestedPriceRangeDTO struct {
	Currency         string `json:"currency"`
	PriceComponent   string `json:"priceComponent"`
	BasisQuantity    string `json:"basisQuantity"`
	BasisUnit        string `json:"basisUnit"`
	LowerBoundAmount string `json:"lowerBoundAmount"`
	UpperBoundAmount string `json:"upperBoundAmount"`
}

// ProcurementSellerFacingPayloadDTO 是賣家可見的需求內容（deny-by-default 白名單）。
// 內容與 vendor portal 的 VendorPortalRFQDTO 相同，但**刻意是獨立型別**——public 與
// app 兩個 namespace 不共用 DTO（API 設計規範 §十）。
type ProcurementSellerFacingPayloadDTO struct {
	// 空字串＝本欄加入前建立的 approval，前端當「未知」處理，不得補值。
	QualityKnowledgeState string                                   `json:"qualityKnowledgeState"`
	Item                  string                                   `json:"item"`
	ItemLabel             string                                   `json:"itemLabel"`
	Quantity              string                                   `json:"quantity"`
	Unit                  string                                   `json:"unit"`
	Packaging             string                                   `json:"packaging"`
	DeliveryCity          string                                   `json:"deliveryCity"`
	DeliveryDate          string                                   `json:"deliveryDate"`
	ResponseDeadline      string                                   `json:"responseDeadline"`
	BuyerNote             string                                   `json:"buyerNote"`
	BuyerRequestedRange   *ProcurementSellerRequestedPriceRangeDTO `json:"buyerRequestedRange"`
}

func sellerFacingPayloadFromProto(p *pb.RfqPublicationInviteSellerViewResponse) ProcurementSellerFacingPayloadDTO {
	return ProcurementSellerFacingPayloadDTO{
		QualityKnowledgeState: p.GetQualityKnowledgeState(),
		Item:                  p.GetItem(),
		ItemLabel:             p.GetItemLabel(),
		Quantity:              p.GetQuantity(),
		Unit:                  p.GetUnit(),
		Packaging:             p.GetPackaging(),
		DeliveryCity:          p.GetDeliveryCity(),
		DeliveryDate:          p.GetDeliveryDate(),
		ResponseDeadline:      p.GetResponseDeadline(),
		BuyerNote:             p.GetBuyerNote(),
		BuyerRequestedRange:   sellerRequestedRangeFromProto(p.GetBuyerRequestedRange()),
	}
}

func sellerRequestedRangeFromProto(r *pb.SellerFacingRequestedRange) *ProcurementSellerRequestedPriceRangeDTO {
	if r == nil {
		return nil
	}
	return &ProcurementSellerRequestedPriceRangeDTO{
		Currency:         r.GetCurrency(),
		PriceComponent:   r.GetPriceComponent(),
		BasisQuantity:    r.GetBasisQuantity(),
		BasisUnit:        r.GetBasisUnit(),
		LowerBoundAmount: r.GetLowerBoundAmount(),
		UpperBoundAmount: r.GetUpperBoundAmount(),
	}
}

// ProcurementSellerResponseVersionDTO mirrors aipb.SellerResponseVersionResponse.
// OutsideBuyerRequestedRange is advisory only — it must never be used to alter or
// block the seller's own amounts.
type ProcurementSellerResponseVersionDTO struct {
	SellerResponseID           string                           `json:"sellerResponseId"`
	SellerResponseVersion      string                           `json:"sellerResponseVersion"`
	PublicationID              string                           `json:"publicationId"`
	Status                     string                           `json:"status"`
	PayloadHash                string                           `json:"payloadHash"`
	SubmittedAt                string                           `json:"submittedAt"`
	SupersededByVersion        string                           `json:"supersededByVersion,omitempty"`
	OutsideBuyerRequestedRange bool                             `json:"outsideBuyerRequestedRange"`
	Quote                      ProcurementSellerQuotePayloadDTO `json:"quote"`
	Replayed                   bool                             `json:"replayed"`
}

type ProcurementSellerResponseClosureDTO struct {
	SellerResponseID  string `json:"sellerResponseId"`
	PublicationID     string `json:"publicationId"`
	LatestVersion     string `json:"latestVersion"`
	WithdrawnVersion  string `json:"withdrawnVersion,omitempty"`
	WriteClosedAt     string `json:"writeClosedAt,omitempty"`
	WriteClosedReason string `json:"writeClosedReason,omitempty"`
}

// ---- 買家面 request／response DTO ----

type ProcurementSellerIdentityDTO struct {
	// DisplayName 對站外業者恆為空字串：ai-service 沒有安全可用的名稱快照。
	// ⛔ 不得帶 email、KYC 狀態或聯絡方式。
	DisplayName string `json:"displayName"`
	PartyType   string `json:"partyType"`
	StoreCode   string `json:"storeCode,omitempty"`
}

// ProcurementBuyerFacingSellerResponseDTO mirrors aipb.BuyerFacingSellerResponse.
// Acceptability is typed and server-decided; the client must not derive it from
// validUntil or version ordering.
type ProcurementBuyerFacingSellerResponseDTO struct {
	SellerResponseID           string                           `json:"sellerResponseId"`
	SellerResponseVersion      string                           `json:"sellerResponseVersion"`
	Status                     string                           `json:"status"`
	PayloadHash                string                           `json:"payloadHash"`
	SubmittedAt                string                           `json:"submittedAt"`
	Seller                     ProcurementSellerIdentityDTO     `json:"seller"`
	Quote                      ProcurementSellerQuotePayloadDTO `json:"quote"`
	OutsideBuyerRequestedRange bool                             `json:"outsideBuyerRequestedRange"`
	Shortlisted                bool                             `json:"shortlisted"`
	Acceptability              string                           `json:"acceptability"`
}

type ProcurementSellerResponseListDTO struct {
	Responses []ProcurementBuyerFacingSellerResponseDTO `json:"responses"`
}

type ProcurementShortlistEntryDTO struct {
	SellerResponseID      string `json:"sellerResponseId"`
	SellerResponseVersion string `json:"sellerResponseVersion"`
	Present               bool   `json:"present"`
}

// ProcurementBuyerQuoteAcceptanceRequest is deliberately exactly three CAS fields.
// ⛔ No amount may appear here: acceptedDealPrice is copied server-side from the
// exact version being accepted.
// ⛔ 三個欄位都是 required：缺任何一個就沒有可 CAS 的對象，而 server 不得拿自己讀到
// 的權威值補齊——doc 64 §3.2「少任何一個都有一條真實的錯配路徑」。
type ProcurementBuyerQuoteAcceptanceRequest struct {
	SellerResponseID              string `json:"sellerResponseId" binding:"required"`
	ExpectedSellerResponseVersion string `json:"expectedSellerResponseVersion" binding:"required"`
	ExpectedRfqSnapshotHash       string `json:"expectedRfqSnapshotHash" binding:"required"`
}

// ProcurementBuyerQuoteAcceptanceDTO mirrors aipb.BuyerQuoteAcceptanceResponse.
// OrderNo/PaymentDeadline stay empty until orderProvisioningState is ORDER_CREATED
// (decision D3) — CREATING_ORDER is a buyer-visible state and must not be rendered
// as 已成交 or 已下單.
type ProcurementBuyerQuoteAcceptanceDTO struct {
	BuyerAcceptanceID      string `json:"buyerAcceptanceId"`
	SellerResponseID       string `json:"sellerResponseId"`
	SellerResponseVersion  string `json:"sellerResponseVersion"`
	AcceptedDealPrice      string `json:"acceptedDealPrice"`
	Currency               string `json:"currency"`
	AcceptedAt             string `json:"acceptedAt"`
	OrderProvisioningState string `json:"orderProvisioningState"`
	OrderNo                string `json:"orderNo"`
	PaymentDeadline        string `json:"paymentDeadline"`
	Replayed               bool   `json:"replayed"`
}

// ---- 賣家面 handler ----

// ProxySellerProcurementPublications lists the publications this seller can see.
//
// @Summary      Hawkr 幫找 — 賣家可見的發布清單
// @Tags         AI
// @Produce      json
// @Security     BearerAuth
// @Param        limit  query  string  false  "最多回幾筆（預設 50，上限 100）"
// @Success      200  {object}  object{code=int,message=string,data=ProcurementSellerPublicationListDTO}
// @Router       /app/sellers/me/procurement/publications [get]
func (h *Holder) ProxySellerProcurementPublications(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}
	ctx := grpcclient.WithStoreID(c.Request.Context(), storeID)
	result, err := h.AIProcurement.ListSellerPublications(ctx, &pb.ListSellerPublicationsRequest{Limit: c.Query("limit")})
	if err != nil {
		writeProcurementError(c, err)
		return
	}
	out := ProcurementSellerPublicationListDTO{
		Publications: make([]ProcurementSellerPublicationListItemDTO, 0, len(result.GetPublications())),
	}
	for _, p := range result.GetPublications() {
		out.Publications = append(out.Publications, ProcurementSellerPublicationListItemDTO{
			Publication:          sellerPublicationFromProto(p.GetPublication()),
			SellerFacingPayload:  sellerFacingPayloadFromProto(p.GetSellerFacingPayload()),
			LatestResponseStatus: p.GetLatestResponseStatus(),
			HasDraft:             p.GetHasDraft(),
		})
	}
	response.OK(c, out)
}

// ProxySellerProcurementPublicationDetail returns one publication's seller-facing summary.
//
// @Summary      Hawkr 幫找 — 賣家看某一張發布
// @Tags         AI
// @Produce      json
// @Security     BearerAuth
// @Param        publication_id  path  string  true  "發布 ID（TSID 字串）"
// @Success      200  {object}  object{code=int,message=string,data=ProcurementSellerPublicationDetailDTO}
// @Failure      404  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Router       /app/sellers/me/procurement/publications/{publication_id} [get]
func (h *Holder) ProxySellerProcurementPublicationDetail(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}
	ctx := grpcclient.WithStoreID(c.Request.Context(), storeID)
	result, err := h.AIProcurement.GetSellerPublication(ctx, &pb.GetSellerPublicationRequest{
		PublicationId: c.Param("publication_id"),
	})
	if err != nil {
		writeProcurementError(c, err)
		return
	}
	out := ProcurementSellerPublicationDetailDTO{
		Publication:         sellerPublicationFromProto(result.GetPublication()),
		SellerFacingPayload: sellerFacingPayloadFromProto(result.GetSellerFacingPayload()),
	}
	if d := result.GetDraft(); d != nil {
		out.Draft = &ProcurementSellerResponseDraftResponseDTO{
			DraftVersion: d.GetDraftVersion(), UpdatedAt: d.GetUpdatedAt(),
			Draft: sellerQuotePayloadFromProto(d.GetDraft()),
		}
	}
	if l := result.GetLatestResponse(); l != nil {
		latest := sellerResponseVersionFromProto(l)
		out.LatestResponse = &latest
	}
	response.OK(c, out)
}

// ProxySellerProcurementResponseDraft upserts the seller's draft quote.
//
// @Summary      Hawkr 幫找 — 賣家回價草稿
// @Description  草稿可不限次改，不計正式一次、不驗金額恆等式，買家讀不到。CAS 對象是 expectedDraftVersion，因此不需要 Idempotency-Key。
// @Tags         AI
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        publication_id  path  string  true  "發布 ID（TSID 字串）"
// @Param        request  body  ProcurementSellerResponseDraftRequest  true  "草稿內容"
// @Success      200  {object}  object{code=int,message=string,data=ProcurementSellerResponseDraftResponseDTO}
// @Failure      409  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Router       /app/sellers/me/procurement/publications/{publication_id}/response-draft [put]
func (h *Holder) ProxySellerProcurementResponseDraft(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}
	var req ProcurementSellerResponseDraftRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badProcurementRequest(c, "invalid request body")
		return
	}
	ctx := grpcclient.WithStoreID(c.Request.Context(), storeID)
	result, err := h.AIProcurement.UpsertSellerResponseDraft(ctx, &pb.UpsertSellerResponseDraftRequest{
		PublicationId:        c.Param("publication_id"),
		ExpectedDraftVersion: req.ExpectedDraftVersion,
		Draft:                req.Draft.toProto(),
	})
	if err != nil {
		writeProcurementError(c, err)
		return
	}
	response.OK(c, ProcurementSellerResponseDraftResponseDTO{
		DraftVersion: result.GetDraftVersion(), UpdatedAt: result.GetUpdatedAt(),
		Draft: sellerQuotePayloadFromProto(result.GetDraft()),
	})
}

// ProxySellerProcurementSubmitResponse appends the next immutable quote version.
//
// @Summary      Hawkr 幫找 — 賣家正式提交／改版回價
// @Description  提交與改版是同一支端點，靠 expectedSellerResponseVersion 區分："" 代表尚無正式回價，"3" 代表要在第 3 版之上改版。
// @Description  穩定冪等鍵走 HTTP `Idempotency-Key` header，不放 body。
// @Tags         AI
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        publication_id   path    string  true  "發布 ID（TSID 字串）"
// @Param        Idempotency-Key  header  string  true  "穩定冪等鍵"
// @Param        request  body  ProcurementSubmitSellerResponseRequest  true  "回價內容與 CAS 欄位"
// @Success      200  {object}  object{code=int,message=string,data=ProcurementSellerResponseVersionDTO}
// @Failure      400  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Failure      409  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Failure      422  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Router       /app/sellers/me/procurement/publications/{publication_id}/seller-responses [post]
func (h *Holder) ProxySellerProcurementSubmitResponse(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}
	key, ok := requireIdempotencyKey(c)
	if !ok {
		return
	}
	var req ProcurementSubmitSellerResponseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badProcurementRequest(c, "invalid request body")
		return
	}
	ctx := grpcclient.WithStoreID(c.Request.Context(), storeID)
	result, err := h.AIProcurement.SubmitSellerResponse(ctx, &pb.SubmitSellerResponseRequest{
		PublicationId:                   c.Param("publication_id"),
		ClientRequestId:                 key,
		ExpectedPublicationVersion:      req.ExpectedPublicationVersion,
		ExpectedSellerFacingPayloadHash: req.ExpectedSellerFacingPayloadHash,
		ExpectedSellerResponseVersion:   req.ExpectedSellerResponseVersion,
		Quote:                           req.Quote.toProto(),
	})
	if err != nil {
		writeProcurementError(c, err)
		return
	}
	response.OK(c, sellerResponseVersionFromProto(result))
}

// ProxySellerProcurementWithdrawResponse withdraws the seller's current quote.
//
// @Summary      Hawkr 幫找 — 賣家撤回回價
// @Description  撤回住身分列，舊版本 payload 不改寫。穩定冪等鍵走 `Idempotency-Key` header。
// @Tags         AI
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        publication_id      path    string  true  "發布 ID（TSID 字串）"
// @Param        seller_response_id  path    string  true  "回價身分 ID（TSID 字串）"
// @Param        Idempotency-Key     header  string  true  "穩定冪等鍵"
// @Param        request  body  ProcurementWithdrawSellerResponseRequest  true  "CAS 欄位與理由"
// @Success      200  {object}  object{code=int,message=string,data=ProcurementSellerResponseClosureDTO}
// @Failure      409  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Router       /app/sellers/me/procurement/publications/{publication_id}/seller-responses/{seller_response_id}/withdraw [post]
func (h *Holder) ProxySellerProcurementWithdrawResponse(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}
	key, ok := requireIdempotencyKey(c)
	if !ok {
		return
	}
	var req ProcurementWithdrawSellerResponseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badProcurementRequest(c, "invalid request body")
		return
	}
	ctx := grpcclient.WithStoreID(c.Request.Context(), storeID)
	result, err := h.AIProcurement.WithdrawSellerResponse(ctx, &pb.WithdrawSellerResponseRequest{
		PublicationId:                 c.Param("publication_id"),
		SellerResponseId:              c.Param("seller_response_id"),
		ClientRequestId:               key,
		ExpectedSellerResponseVersion: req.ExpectedSellerResponseVersion,
		ReasonCode:                    req.ReasonCode,
	})
	if err != nil {
		writeProcurementError(c, err)
		return
	}
	response.OK(c, sellerResponseClosureFromProto(result))
}

// ProxySellerProcurementDecline declines a publication for this seller.
//
// @Summary      Hawkr 幫找 — 賣家明確拒絕
// @Description  終止該賣家在此發布的寫入，冪等，理由可選。穩定冪等鍵走 `Idempotency-Key` header。
// @Tags         AI
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        publication_id   path    string  true  "發布 ID（TSID 字串）"
// @Param        Idempotency-Key  header  string  true  "穩定冪等鍵"
// @Param        request  body  ProcurementDeclineRfqPublicationRequest  true  "拒絕理由（可選）"
// @Success      200  {object}  object{code=int,message=string,data=ProcurementSellerResponseClosureDTO}
// @Router       /app/sellers/me/procurement/publications/{publication_id}/decline [post]
func (h *Holder) ProxySellerProcurementDecline(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}
	key, ok := requireIdempotencyKey(c)
	if !ok {
		return
	}
	var req ProcurementDeclineRfqPublicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badProcurementRequest(c, "invalid request body")
		return
	}
	ctx := grpcclient.WithStoreID(c.Request.Context(), storeID)
	result, err := h.AIProcurement.DeclineRfqPublication(ctx, &pb.DeclineRfqPublicationRequest{
		PublicationId: c.Param("publication_id"), ClientRequestId: key, ReasonCode: req.ReasonCode,
	})
	if err != nil {
		writeProcurementError(c, err)
		return
	}
	response.OK(c, sellerResponseClosureFromProto(result))
}

// ---- 買家面 handler ----

// ProxyBuyerProcurementSellerResponses lists every immutable quote version on this request.
//
// @Summary      Hawkr 幫找 — 買家比較清單
// @Description  已被取代／撤回／逾期的版本仍留在清單裡（audit），可否接受由 typed `acceptability` 表達，前端不得自行推導。
// @Tags         AI
// @Produce      json
// @Security     BearerAuth
// @Param        id              path   string  true   "需求 ID（TSID 字串）"
// @Param        publication_id  query  string  false  "只看某一次發布的回價"
// @Success      200  {object}  object{code=int,message=string,data=ProcurementSellerResponseListDTO}
// @Router       /app/buyers/me/procurement/requests/{id}/seller-responses [get]
func (h *Holder) ProxyBuyerProcurementSellerResponses(c *gin.Context) {
	buyerID, ok := h.resolveBuyerID(c)
	if !ok {
		return
	}
	ctx := grpcclient.WithBuyerID(c.Request.Context(), buyerID)
	result, err := h.AIProcurement.ListSellerResponses(ctx, &pb.ListSellerResponsesRequest{
		RequestId: c.Param("id"), PublicationId: c.Query("publication_id"),
	})
	if err != nil {
		writeProcurementError(c, err)
		return
	}
	out := ProcurementSellerResponseListDTO{
		Responses: make([]ProcurementBuyerFacingSellerResponseDTO, 0, len(result.GetResponses())),
	}
	for _, r := range result.GetResponses() {
		out.Responses = append(out.Responses, buyerFacingSellerResponseFromProto(r))
	}
	response.OK(c, out)
}

// ProxyBuyerProcurementShortlistPut adds one exact version to the comparison shortlist.
//
// @Summary      Hawkr 幫找 — 加入比較
// @Description  冪等、零 side effect，因此不需要 Idempotency-Key。
// @Tags         AI
// @Produce      json
// @Security     BearerAuth
// @Param        id                       path  string  true  "需求 ID（TSID 字串）"
// @Param        seller_response_id       path  string  true  "回價身分 ID（TSID 字串）"
// @Param        seller_response_version  path  string  true  "回價版本（十進位字串）"
// @Success      200  {object}  object{code=int,message=string,data=ProcurementShortlistEntryDTO}
// @Failure      404  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Router       /app/buyers/me/procurement/requests/{id}/shortlist/{seller_response_id}/versions/{seller_response_version} [put]
func (h *Holder) ProxyBuyerProcurementShortlistPut(c *gin.Context) {
	buyerID, ok := h.resolveBuyerID(c)
	if !ok {
		return
	}
	ctx := grpcclient.WithBuyerID(c.Request.Context(), buyerID)
	result, err := h.AIProcurement.PutShortlistEntry(ctx, shortlistRequestFromPath(c))
	if err != nil {
		writeProcurementError(c, err)
		return
	}
	response.OK(c, shortlistEntryFromProto(result))
}

// ProxyBuyerProcurementShortlistDelete removes one exact version from the shortlist.
//
// @Summary      Hawkr 幫找 — 移除比較
// @Description  冪等：本來就不在清單裡也回成功。
// @Tags         AI
// @Produce      json
// @Security     BearerAuth
// @Param        id                       path  string  true  "需求 ID（TSID 字串）"
// @Param        seller_response_id       path  string  true  "回價身分 ID（TSID 字串）"
// @Param        seller_response_version  path  string  true  "回價版本（十進位字串）"
// @Success      200  {object}  object{code=int,message=string,data=ProcurementShortlistEntryDTO}
// @Router       /app/buyers/me/procurement/requests/{id}/shortlist/{seller_response_id}/versions/{seller_response_version} [delete]
func (h *Holder) ProxyBuyerProcurementShortlistDelete(c *gin.Context) {
	buyerID, ok := h.resolveBuyerID(c)
	if !ok {
		return
	}
	ctx := grpcclient.WithBuyerID(c.Request.Context(), buyerID)
	result, err := h.AIProcurement.DeleteShortlistEntry(ctx, shortlistRequestFromPath(c))
	if err != nil {
		writeProcurementError(c, err)
		return
	}
	response.OK(c, shortlistEntryFromProto(result))
}

// ProxyBuyerProcurementQuoteAcceptance accepts one exact seller quote version.
//
// @Summary      Hawkr 幫找 — 買家接受報價（server 權威 ACK ＝ 商務 award）
// @Description  body 只有三個 CAS 欄位，⛔ 不得帶任何金額——`acceptedDealPrice` 由 server 從被接受的那一版取。
// @Description  穩定冪等鍵走 `Idempotency-Key` header。
// @Description  ACK 不帶 `orderNo`：`orderProvisioningState=CREATING_ORDER` 是買家看得到的狀態，不得畫成「已成交」或「已下單」。
// @Tags         AI
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id               path    string  true  "需求 ID（TSID 字串）"
// @Param        Idempotency-Key  header  string  true  "穩定冪等鍵"
// @Param        request  body  ProcurementBuyerQuoteAcceptanceRequest  true  "要接受的 exact version"
// @Success      200  {object}  object{code=int,message=string,data=ProcurementBuyerQuoteAcceptanceDTO}
// @Failure      400  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Failure      404  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Failure      409  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Router       /app/buyers/me/procurement/requests/{id}/buyer-quote-acceptances [post]
func (h *Holder) ProxyBuyerProcurementQuoteAcceptance(c *gin.Context) {
	buyerID, ok := h.resolveBuyerID(c)
	if !ok {
		return
	}
	key, ok := requireIdempotencyKey(c)
	if !ok {
		return
	}
	var req ProcurementBuyerQuoteAcceptanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badProcurementRequest(c, "invalid request body")
		return
	}
	ctx := grpcclient.WithBuyerID(c.Request.Context(), buyerID)
	result, err := h.AIProcurement.AcceptSellerQuote(ctx, &pb.BuyerQuoteAcceptanceRequest{
		RequestId:                     c.Param("id"),
		ClientRequestId:               key,
		SellerResponseId:              req.SellerResponseID,
		ExpectedSellerResponseVersion: req.ExpectedSellerResponseVersion,
		ExpectedRfqSnapshotHash:       req.ExpectedRfqSnapshotHash,
	})
	if err != nil {
		writeProcurementError(c, err)
		return
	}
	response.OK(c, ProcurementBuyerQuoteAcceptanceDTO{
		BuyerAcceptanceID:      result.GetBuyerAcceptanceId(),
		SellerResponseID:       result.GetSellerResponseId(),
		SellerResponseVersion:  result.GetSellerResponseVersion(),
		AcceptedDealPrice:      result.GetAcceptedDealPrice(),
		Currency:               result.GetCurrency(),
		AcceptedAt:             result.GetAcceptedAt(),
		OrderProvisioningState: result.GetOrderProvisioningState(),
		OrderNo:                result.GetOrderNo(),
		PaymentDeadline:        result.GetPaymentDeadline(),
		Replayed:               result.GetReplayed(),
	})
}

func shortlistRequestFromPath(c *gin.Context) *pb.ShortlistEntryRequest {
	return &pb.ShortlistEntryRequest{
		RequestId:             c.Param("id"),
		SellerResponseId:      c.Param("seller_response_id"),
		SellerResponseVersion: c.Param("seller_response_version"),
	}
}

func sellerPublicationFromProto(p *pb.SellerPublicationSummary) ProcurementSellerPublicationDTO {
	return ProcurementSellerPublicationDTO{
		PublicationID: p.GetPublicationId(), PublicationVersion: p.GetPublicationVersion(),
		InviteID: p.GetInviteId(), Lane: p.GetLane(),
		SellerFacingPayloadHash: p.GetSellerFacingPayloadHash(),
		DeliveryState:           p.GetDeliveryState(), PublishedAt: p.GetPublishedAt(),
	}
}

func sellerResponseVersionFromProto(p *pb.SellerResponseVersionResponse) ProcurementSellerResponseVersionDTO {
	return ProcurementSellerResponseVersionDTO{
		SellerResponseID: p.GetSellerResponseId(), SellerResponseVersion: p.GetSellerResponseVersion(),
		PublicationID: p.GetPublicationId(), Status: p.GetStatus(), PayloadHash: p.GetPayloadHash(),
		SubmittedAt: p.GetSubmittedAt(), SupersededByVersion: p.GetSupersededByVersion(),
		OutsideBuyerRequestedRange: p.GetOutsideBuyerRequestedRange(),
		Quote:                      sellerQuotePayloadFromProto(p.GetQuote()),
		Replayed:                   p.GetReplayed(),
	}
}

func sellerResponseClosureFromProto(p *pb.SellerResponseClosureResponse) ProcurementSellerResponseClosureDTO {
	return ProcurementSellerResponseClosureDTO{
		SellerResponseID: p.GetSellerResponseId(), PublicationID: p.GetPublicationId(),
		LatestVersion: p.GetLatestVersion(), WithdrawnVersion: p.GetWithdrawnVersion(),
		WriteClosedAt: p.GetWriteClosedAt(), WriteClosedReason: p.GetWriteClosedReason(),
	}
}

func buyerFacingSellerResponseFromProto(p *pb.BuyerFacingSellerResponse) ProcurementBuyerFacingSellerResponseDTO {
	return ProcurementBuyerFacingSellerResponseDTO{
		SellerResponseID: p.GetSellerResponseId(), SellerResponseVersion: p.GetSellerResponseVersion(),
		Status: p.GetStatus(), PayloadHash: p.GetPayloadHash(), SubmittedAt: p.GetSubmittedAt(),
		Seller: ProcurementSellerIdentityDTO{
			DisplayName: p.GetSeller().GetDisplayName(), PartyType: p.GetSeller().GetPartyType(),
			StoreCode: p.GetSeller().GetStoreCode(),
		},
		Quote:                      sellerQuotePayloadFromProto(p.GetQuote()),
		OutsideBuyerRequestedRange: p.GetOutsideBuyerRequestedRange(),
		Shortlisted:                p.GetShortlisted(),
		Acceptability:              p.GetAcceptability(),
	}
}

func shortlistEntryFromProto(p *pb.ShortlistEntryResponse) ProcurementShortlistEntryDTO {
	return ProcurementShortlistEntryDTO{
		SellerResponseID: p.GetSellerResponseId(), SellerResponseVersion: p.GetSellerResponseVersion(),
		Present: p.GetPresent(),
	}
}
