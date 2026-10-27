package handler

import (
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/Will413028/hawkr/gateway/internal/response"
	aipb "github.com/Will413028/hawkr/go-pkg/proto/ai"
	"github.com/gin-gonic/gin"
)

// ==================== DTO ====================

type VendorPortalReferencePriceRangeDTO struct {
	LowerBoundAmount string `json:"lowerBoundAmount"`
	UpperBoundAmount string `json:"upperBoundAmount"`
	BasisQuantity    string `json:"basisQuantity"`
	BasisUnit        string `json:"basisUnit"`
}

type VendorPortalRequestedPriceRangeDTO struct {
	Currency         string `json:"currency"`
	PriceComponent   string `json:"priceComponent"`
	BasisQuantity    string `json:"basisQuantity"`
	BasisUnit        string `json:"basisUnit"`
	LowerBoundAmount string `json:"lowerBoundAmount"`
	UpperBoundAmount string `json:"upperBoundAmount"`
}

// VendorPortalRFQDTO 是站外業者看得到的 RFQ 摘要（deny-by-default 白名單）。
//
// ⛔ 逐欄明列（API 設計規範 §3.1）。加欄位之前先回答：「這是這張詢價單的商業
// 資訊，還是買家的個人資料？」後者一律不進。
type VendorPortalRFQDTO struct {
	// 空字串＝本欄加入前建立的 approval，前端當「未知」處理，不得補值。
	QualityKnowledgeState string `json:"qualityKnowledgeState"`
	Item                  string `json:"item"`
	ItemLabel             string `json:"itemLabel"`
	Quantity              string `json:"quantity"`
	Unit                  string `json:"unit"`
	Packaging             string `json:"packaging"`
	DeliveryCity          string `json:"deliveryCity"`
	DeliveryDate          string `json:"deliveryDate"`
	ResponseDeadline      string `json:"responseDeadline"`
	// 空字串＝買家沒有補充說明。原文照登，不是品相宣告。
	BuyerNote string `json:"buyerNote"`
	// PublicationVersion 是白名單的第 13 欄（十進位字串）。業者正式送出報價時原樣帶
	// 回來做 CAS：把報價綁在「他當時看到的那一版 RFQ」上。
	// ⛔ seller_facing_payload_hash 不在這裡，理由見 ai.proto 同名欄位的註解。
	PublicationVersion  string                              `json:"publicationVersion"`
	ReferencePriceRange *VendorPortalReferencePriceRangeDTO `json:"referencePriceRange"`
	BuyerRequestedRange *VendorPortalRequestedPriceRangeDTO `json:"buyerRequestedRange"`
}

func vendorPortalRFQDTOFromProto(r *aipb.RfqPublicationInviteSellerViewResponse) VendorPortalRFQDTO {
	return VendorPortalRFQDTO{
		QualityKnowledgeState: r.GetQualityKnowledgeState(),
		Item:                  r.GetItem(),
		ItemLabel:             r.GetItemLabel(),
		Quantity:              r.GetQuantity(),
		Unit:                  r.GetUnit(),
		Packaging:             r.GetPackaging(),
		DeliveryCity:          r.GetDeliveryCity(),
		DeliveryDate:          r.GetDeliveryDate(),
		ResponseDeadline:      r.GetResponseDeadline(),
		BuyerNote:             r.GetBuyerNote(),
		PublicationVersion:    r.GetPublicationVersion(),
		ReferencePriceRange:   vendorPortalReferenceRangeFromProto(r.GetReferencePriceRange()),
		BuyerRequestedRange:   vendorPortalRequestedRangeFromProto(r.GetBuyerRequestedRange()),
	}
}

func vendorPortalReferenceRangeFromProto(r *aipb.SellerFacingReferenceRange) *VendorPortalReferencePriceRangeDTO {
	if r == nil {
		return nil
	}
	return &VendorPortalReferencePriceRangeDTO{
		LowerBoundAmount: r.GetLowerBoundAmount(),
		UpperBoundAmount: r.GetUpperBoundAmount(),
		BasisQuantity:    r.GetBasisQuantity(),
		BasisUnit:        r.GetBasisUnit(),
	}
}

func vendorPortalRequestedRangeFromProto(r *aipb.SellerFacingRequestedRange) *VendorPortalRequestedPriceRangeDTO {
	if r == nil {
		return nil
	}
	return &VendorPortalRequestedPriceRangeDTO{
		Currency:         r.GetCurrency(),
		PriceComponent:   r.GetPriceComponent(),
		BasisQuantity:    r.GetBasisQuantity(),
		BasisUnit:        r.GetBasisUnit(),
		LowerBoundAmount: r.GetLowerBoundAmount(),
		UpperBoundAmount: r.GetUpperBoundAmount(),
	}
}

// VendorPortalRFQ 讀取站外業者可見的 RFQ 摘要。
//
// @Summary      站外業者 Portal — 讀取 RFQ 摘要
// @Description  需要 vendor portal session（cookie `__Host-hk_vp_<rfq_recipient_id>`（path 指名的那張邀請的 cookie），單體驗證後以 X-Vendor-Portal-* header 轉給 gateway）。path 的 rfq_recipient_id 與 session 綁定的不同一律回 404（不是 403），且不回述請求的 id。qualityKnowledgeState 為空字串代表該筆 approval 建立於本欄加入之前，呼叫端當「未知」處理。
// @Tags         VendorPortal
// @Produce      json
// @Param        rfq_recipient_id  path  string  true  "RFQ 收件人 ID"
// @Success      200  {object}  object{code=int,message=string,data=VendorPortalRFQDTO}
// @Failure      401  {object}  object{code=int,message=string,traceId=string}
// @Failure      404  {object}  object{code=int,message=string,traceId=string}
// @Router       /public/vendor_portal/rfqs/{rfq_recipient_id} [get]
func (h *Holder) VendorPortalRFQ(c *gin.Context) {
	boundID := middleware.VendorPortalRecipientIDFrom(c)
	pathID := c.Param("rfq_recipient_id")

	if boundID == "" || boundID != pathID {
		response.NotFound(c, "invite not found")
		return
	}

	result, err := h.AIProcurement.GetRfqPublicationInviteSellerView(c.Request.Context(),
		&aipb.GetRfqPublicationInviteSellerViewRequest{InviteId: pathID})
	if err != nil {
		response.GrpcErr(c, "ai", err)
		return
	}
	response.OK(c, vendorPortalRFQDTOFromProto(result))
}
