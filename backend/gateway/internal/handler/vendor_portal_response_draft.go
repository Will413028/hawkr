package handler

import (
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/Will413028/hawkr/gateway/internal/response"
	aipb "github.com/Will413028/hawkr/go-pkg/proto/ai"
	"github.com/gin-gonic/gin"
)

type VendorPortalOfferedQualityDTO struct {
	Grade           string `json:"grade"`
	InspectionNotes string `json:"inspectionNotes"`
}

type VendorPortalUnitBasisDTO struct {
	Quantity string `json:"quantity"`
	Unit     string `json:"unit"`
}

type VendorPortalQuotePayloadDTO struct {
	ResponseType        string                        `json:"responseType"`
	OfferedQuality      VendorPortalOfferedQualityDTO `json:"offeredQuality"`
	UnitBasis           VendorPortalUnitBasisDTO      `json:"unitBasis"`
	OfferedQuantity     string                        `json:"offeredQuantity"`
	OfferedQuantityUnit string                        `json:"offeredQuantityUnit"`
	ConversionBasis     string                        `json:"conversionBasis"`
	Currency            string                        `json:"currency"`
	GoodsAmount         string                        `json:"goodsAmount"`
	ShippingAmount      string                        `json:"shippingAmount"`
	TaxAmount           string                        `json:"taxAmount"`
	TaxInclusionBasis   string                        `json:"taxInclusionBasis"`
	TotalAmount         string                        `json:"totalAmount"`
	LeadTimeDays        string                        `json:"leadTimeDays"`
	PromisedDeliveryAt  string                        `json:"promisedDeliveryAt"`
	ValidUntil          string                        `json:"validUntil"`
	TermsNote           string                        `json:"termsNote"`
	DeviationDisclosure string                        `json:"deviationDisclosure"`
}

func (d VendorPortalQuotePayloadDTO) toProto() *aipb.SellerQuotePayload {
	return &aipb.SellerQuotePayload{
		ResponseType: d.ResponseType,
		OfferedQuality: &aipb.OfferedQuality{
			Grade: d.OfferedQuality.Grade, InspectionNotes: d.OfferedQuality.InspectionNotes,
		},
		UnitBasis: &aipb.UnitBasis{
			Quantity: d.UnitBasis.Quantity, Unit: d.UnitBasis.Unit,
		},
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

func vendorPortalQuotePayloadFromProto(p *aipb.SellerQuotePayload) VendorPortalQuotePayloadDTO {
	return VendorPortalQuotePayloadDTO{
		ResponseType: p.GetResponseType(),
		OfferedQuality: VendorPortalOfferedQualityDTO{
			Grade:           p.GetOfferedQuality().GetGrade(),
			InspectionNotes: p.GetOfferedQuality().GetInspectionNotes(),
		},
		UnitBasis: VendorPortalUnitBasisDTO{
			Quantity: p.GetUnitBasis().GetQuantity(),
			Unit:     p.GetUnitBasis().GetUnit(),
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

type VendorPortalUpsertDraftRequest struct {
	ExpectedDraftVersion string                      `json:"expectedDraftVersion"`
	Draft                VendorPortalQuotePayloadDTO `json:"draft"`
}

type VendorPortalDraftResponseDTO struct {
	DraftVersion string                      `json:"draftVersion"`
	UpdatedAt    string                      `json:"updatedAt"`
	Draft        VendorPortalQuotePayloadDTO `json:"draft"`
}

func vendorPortalBoundRecipientID(c *gin.Context) (string, bool) {
	boundID := middleware.VendorPortalRecipientIDFrom(c)
	if boundID == "" || boundID != c.Param("rfq_recipient_id") {
		response.NotFound(c, "invite not found")
		return "", false
	}
	return boundID, true
}

// @Summary      Hawkr 幫找 — 站外業者存回價草稿
// @Description  草稿不驗金額恆等式、不驗品相必填、不驗 validUntil 是否未來——那些是正式提交才擋的（草稿的意義就是還沒想清楚也能先存）。
// @Description  CAS 對象是 expectedDraftVersion；不需要 Idempotency-Key。草稿不得被買家讀到。
// @Tags         VendorPortal
// @Accept       json
// @Produce      json
// @Param        rfq_recipient_id  path  string  true  "邀請收件人 ID（TSID 字串）"
// @Param        request  body  VendorPortalUpsertDraftRequest  true  "草稿內容與 CAS 欄位"
// @Success      200  {object}  object{code=int,message=string,data=VendorPortalDraftResponseDTO}
// @Failure      400  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Failure      404  {object}  object{code=int,message=string,traceId=string}
// @Failure      409  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Router       /public/vendor_portal/rfqs/{rfq_recipient_id}/response_draft [put]
func (h *Holder) VendorPortalUpsertResponseDraft(c *gin.Context) {
	inviteID, ok := vendorPortalBoundRecipientID(c)
	if !ok {
		return
	}
	var req VendorPortalUpsertDraftRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badProcurementRequest(c, "invalid request body")
		return
	}
	result, err := h.AIProcurement.UpsertVendorPortalResponseDraft(c.Request.Context(),
		&aipb.UpsertVendorPortalResponseDraftRequest{
			RfqRecipientId:       inviteID,
			ExpectedDraftVersion: req.ExpectedDraftVersion,
			Draft:                req.Draft.toProto(),
		})
	if err != nil {
		writeProcurementError(c, err)
		return
	}
	response.OK(c, VendorPortalDraftResponseDTO{
		DraftVersion: result.GetDraftVersion(),
		UpdatedAt:    result.GetUpdatedAt(),
		Draft:        vendorPortalQuotePayloadFromProto(result.GetDraft()),
	})
}

// @Summary      Hawkr 幫找 — 站外業者讀回自己的回價草稿
// @Description  沒存過草稿時回 404（不是 200 空物件）——「沒有草稿」與「草稿是空的」是兩件事。
// @Tags         VendorPortal
// @Produce      json
// @Param        rfq_recipient_id  path  string  true  "邀請收件人 ID（TSID 字串）"
// @Success      200  {object}  object{code=int,message=string,data=VendorPortalDraftResponseDTO}
// @Failure      404  {object}  object{code=int,message=string,traceId=string}
// @Router       /public/vendor_portal/rfqs/{rfq_recipient_id}/response_draft [get]
func (h *Holder) VendorPortalResponseDraft(c *gin.Context) {
	inviteID, ok := vendorPortalBoundRecipientID(c)
	if !ok {
		return
	}
	result, err := h.AIProcurement.GetVendorPortalResponseDraft(c.Request.Context(),
		&aipb.GetVendorPortalResponseDraftRequest{RfqRecipientId: inviteID})
	if err != nil {
		writeProcurementError(c, err)
		return
	}
	response.OK(c, VendorPortalDraftResponseDTO{
		DraftVersion: result.GetDraftVersion(),
		UpdatedAt:    result.GetUpdatedAt(),
		Draft:        vendorPortalQuotePayloadFromProto(result.GetDraft()),
	})
}
