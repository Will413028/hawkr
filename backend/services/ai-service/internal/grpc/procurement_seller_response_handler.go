package grpc

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"

	"google.golang.org/grpc/codes"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/commerce"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/identity"
	aipb "github.com/Will413028/hawkr/go-pkg/proto/ai"
	"github.com/Will413028/hawkr/go-pkg/timeutil"
)

func init() {
	seller := MethodPolicy{Actor: ActorSeller}
	buyer := MethodPolicy{Actor: ActorBuyer}
	registerMethodPolicy("/hawkr.ai.AIProcurementService/ListSellerPublications", seller)
	registerMethodPolicy("/hawkr.ai.AIProcurementService/GetSellerPublication", seller)
	registerMethodPolicy("/hawkr.ai.AIProcurementService/UpsertSellerResponseDraft", seller)
	external := MethodPolicy{Actor: ActorNone}
	registerMethodPolicy("/hawkr.ai.AIProcurementService/GetVendorPortalResponseDraft", external)
	registerMethodPolicy("/hawkr.ai.AIProcurementService/UpsertVendorPortalResponseDraft", external)
	// 站外業者的三支正式寫入。⛔ 它們走 ActorNone 是因為站外業者**沒有平台帳號**
	// （EXTERNAL lane 的前提），身分改由入口驗過的 vendor portal session 斷言；
	// 真正擋住「拿到邀請信就能代該業者承諾價格」的是 usecase 要求完整的 Google
	// 驗證紀錄（缺任何欄位回 PermissionDenied），不是這裡的 policy。
	registerMethodPolicy("/hawkr.ai.AIProcurementService/SubmitVendorPortalResponse", external)
	registerMethodPolicy("/hawkr.ai.AIProcurementService/WithdrawVendorPortalResponse", external)
	registerMethodPolicy("/hawkr.ai.AIProcurementService/DeclineVendorPortalRfqPublication", external)
	registerMethodPolicy("/hawkr.ai.AIProcurementService/SubmitSellerResponse", seller)
	registerMethodPolicy("/hawkr.ai.AIProcurementService/WithdrawSellerResponse", seller)
	registerMethodPolicy("/hawkr.ai.AIProcurementService/DeclineRfqPublication", seller)
	registerMethodPolicy("/hawkr.ai.AIProcurementService/ListSellerResponses", buyer)
	registerMethodPolicy("/hawkr.ai.AIProcurementService/PutShortlistEntry", buyer)
	registerMethodPolicy("/hawkr.ai.AIProcurementService/DeleteShortlistEntry", buyer)
	registerMethodPolicy("/hawkr.ai.AIProcurementService/AcceptSellerQuote", buyer)
}

// sellerPrincipalFromContext 把 gateway 斷言的賣家身分換成 usecase 的 principal。
// invite_id 非空即 EXTERNAL：gateway 已先做過 vendor portal session 與 path id 的
// 相等檢查（同 GetRfqPublicationInviteSellerView 的既有形狀）。
func sellerPrincipalFromContext(ctx context.Context, inviteID string) (commerce.SellerPrincipal, error) {
	if inviteID != "" {
		id, err := parseInt64(inviteID)
		if err != nil {
			return commerce.SellerPrincipal{}, procurementStatus(codes.InvalidArgument, "", "invalid inviteId")
		}
		return commerce.SellerPrincipal{InviteID: id}, nil
	}
	a, ok := ActorFromContext(ctx)
	if !ok {
		return commerce.SellerPrincipal{}, procurementStatus(codes.PermissionDenied, "", "actor not resolved")
	}
	storeID, err := strconv.ParseInt(a.UserID, 10, 64)
	if err != nil {
		return commerce.SellerPrincipal{}, procurementStatus(codes.Internal, "", "invalid store id in resolved actor")
	}
	return commerce.SellerPrincipal{StoreID: storeID}, nil
}

func publicationIDFrom(p commerce.SellerPrincipal, raw string) (int64, error) {
	if raw == "" && p.IsExternal() {
		return 0, nil
	}
	return parseInt64(raw)
}

func (s *AIProcurementGrpcServer) ListSellerPublications(ctx context.Context, req *aipb.ListSellerPublicationsRequest) (*aipb.ListSellerPublicationsResponse, error) {
	p, err := sellerPrincipalFromContext(ctx, "")
	if err != nil {
		return nil, err
	}
	limit := int32(0)
	if req.GetLimit() != "" {
		v, convErr := strconv.ParseInt(req.GetLimit(), 10, 32)
		if convErr != nil {
			return nil, procurementStatus(codes.InvalidArgument, "", "invalid limit")
		}
		limit = int32(v)
	}
	rows, err := s.sellerCommands.SellerResponse.ListPublications(ctx, p.StoreID, limit)
	if err != nil {
		return nil, procurementUsecaseErrorStatus(ctx, err)
	}
	out := make([]*aipb.SellerPublicationListItem, len(rows))
	for i, row := range rows {
		out[i] = &aipb.SellerPublicationListItem{
			Publication:          toSellerPublicationSummaryProto(row.Row.Publication, row.Row.Invite),
			SellerFacingPayload:  toSellerFacingPayloadProto(row.Row.SellerView),
			LatestResponseStatus: row.LatestResponseStatus,
			HasDraft:             row.HasDraft,
		}
	}
	return &aipb.ListSellerPublicationsResponse{Publications: out}, nil
}

func (s *AIProcurementGrpcServer) GetSellerPublication(ctx context.Context, req *aipb.GetSellerPublicationRequest) (*aipb.SellerPublicationDetailResponse, error) {
	p, err := sellerPrincipalFromContext(ctx, req.GetInviteId())
	if err != nil {
		return nil, err
	}
	publicationID, err := publicationIDFrom(p, req.GetPublicationId())
	if err != nil {
		return nil, procurementStatus(codes.InvalidArgument, "", "invalid publicationId")
	}
	detail, err := s.sellerCommands.SellerResponse.GetPublicationDetail(ctx, p, publicationID)
	if err != nil {
		return nil, procurementUsecaseErrorStatus(ctx, err)
	}
	out := &aipb.SellerPublicationDetailResponse{
		Publication:         toSellerPublicationSummaryProto(detail.Publication, detail.Invite),
		SellerFacingPayload: toSellerFacingPayloadProto(detail.SellerView),
	}
	if detail.HasDraft {
		var draft domain.SellerQuotePayload
		// 草稿是原樣存下的 payload JSON，解不開就當成沒有草稿——它不驗金額恆等式，
		// 一份壞掉的草稿不該讓賣家連 publication 詳情都讀不到。
		if err := json.Unmarshal(detail.Draft.Payload, &draft); err == nil {
			out.Draft = &aipb.SellerResponseDraftResponse{
				DraftVersion: itoa32(detail.Draft.Version),
				UpdatedAt:    timeutil.FormatISO8601(detail.Draft.UpdatedAt),
				Draft:        toSellerQuotePayloadProto(draft),
			}
		}
	}
	if detail.HasLatest {
		out.LatestResponse = toSellerResponseVersionProto(commerce.SellerResponseVersionResult{
			Identity: detail.Identity, Version: detail.Latest, Status: detail.LatestStatus,
		})
	}
	return out, nil
}

// toSellerQuotePayloadProto 是草稿路徑用的轉換：草稿住 JSON 不住版本列，所以來源
// 是 domain 型別而不是 repository.SellerResponseVersion。
func toSellerQuotePayloadProto(d domain.SellerQuotePayload) *aipb.SellerQuotePayload {
	return &aipb.SellerQuotePayload{
		ResponseType: d.ResponseType,
		OfferedQuality: &aipb.OfferedQuality{
			Grade: d.OfferedQuality.Grade, InspectionNotes: d.OfferedQuality.InspectionNotes,
		},
		UnitBasis:           &aipb.UnitBasis{Quantity: d.UnitBasis.Quantity, Unit: d.UnitBasis.Unit},
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

func (s *AIProcurementGrpcServer) UpsertSellerResponseDraft(ctx context.Context, req *aipb.UpsertSellerResponseDraftRequest) (*aipb.SellerResponseDraftResponse, error) {
	p, err := sellerPrincipalFromContext(ctx, req.GetInviteId())
	if err != nil {
		return nil, err
	}
	publicationID, err := publicationIDFrom(p, req.GetPublicationId())
	if err != nil {
		return nil, procurementStatus(codes.InvalidArgument, "", "invalid publicationId")
	}
	draft, err := s.sellerCommands.SellerResponse.UpsertDraft(ctx, commerce.UpsertSellerResponseDraftInput{
		Principal: p, PublicationID: publicationID,
		ExpectedDraftVersion: req.GetExpectedDraftVersion(),
		Draft:                fromSellerQuoteProto(req.GetDraft()),
	})
	if err != nil {
		return nil, procurementUsecaseErrorStatus(ctx, err)
	}
	return &aipb.SellerResponseDraftResponse{
		DraftVersion: itoa32(draft.Version),
		UpdatedAt:    timeutil.FormatISO8601(draft.UpdatedAt),
		Draft:        req.GetDraft(),
	}, nil
}

func vendorPortalPrincipal(rfqRecipientID string) (commerce.SellerPrincipal, error) {
	id, err := parseInt64(rfqRecipientID)
	if err != nil || id == 0 {
		return commerce.SellerPrincipal{}, procurementStatus(codes.InvalidArgument, "", "invalid rfqRecipientId")
	}
	return commerce.SellerPrincipal{InviteID: id}, nil
}

func (s *AIProcurementGrpcServer) UpsertVendorPortalResponseDraft(ctx context.Context, req *aipb.UpsertVendorPortalResponseDraftRequest) (*aipb.SellerResponseDraftResponse, error) {
	p, err := vendorPortalPrincipal(req.GetRfqRecipientId())
	if err != nil {
		return nil, err
	}
	draft, err := s.sellerCommands.SellerResponse.UpsertDraft(ctx, commerce.UpsertSellerResponseDraftInput{
		Principal: p, PublicationID: 0,
		ExpectedDraftVersion: req.GetExpectedDraftVersion(),
		Draft:                fromSellerQuoteProto(req.GetDraft()),
	})
	if err != nil {
		return nil, procurementUsecaseErrorStatus(ctx, err)
	}
	return &aipb.SellerResponseDraftResponse{
		DraftVersion: itoa32(draft.Version),
		UpdatedAt:    timeutil.FormatISO8601(draft.UpdatedAt),
		Draft:        req.GetDraft(),
	}, nil
}

func (s *AIProcurementGrpcServer) GetVendorPortalResponseDraft(ctx context.Context, req *aipb.GetVendorPortalResponseDraftRequest) (*aipb.SellerResponseDraftResponse, error) {
	p, err := vendorPortalPrincipal(req.GetRfqRecipientId())
	if err != nil {
		return nil, err
	}
	row, err := s.sellerCommands.SellerResponse.GetDraft(ctx, p, 0)
	if err != nil {
		return nil, procurementUsecaseErrorStatus(ctx, err)
	}
	var draft domain.SellerQuotePayload
	if err := json.Unmarshal(row.Payload, &draft); err != nil {
		return nil, procurementStatus(codes.Internal, "", "stored draft payload is not readable")
	}
	return &aipb.SellerResponseDraftResponse{
		DraftVersion: itoa32(row.Version),
		UpdatedAt:    timeutil.FormatISO8601(row.UpdatedAt),
		Draft:        toSellerQuotePayloadProto(draft),
	}, nil
}

// SubmitVendorPortalResponse 是站外業者的正式送出（ActorNone）。
//
// 與 SubmitSellerResponse 共用同一個 usecase，差別只有身分鍵的來源：這裡是
// vendor portal session 的 rfq_recipient_id，publication 由 usecase 從 invite 解出
// （PublicationID 傳 0，沿用 UpsertVendorPortalResponseDraft 的既有形狀）。
func (s *AIProcurementGrpcServer) SubmitVendorPortalResponse(ctx context.Context, req *aipb.SubmitVendorPortalResponseRequest) (*aipb.SellerResponseVersionResponse, error) {
	p, err := vendorPortalPrincipal(req.GetRfqRecipientId())
	if err != nil {
		return nil, err
	}
	result, err := s.sellerCommands.SellerResponse.Submit(ctx, commerce.SubmitSellerResponseInput{
		Principal: p, PublicationID: 0,
		ClientRequestID:            req.GetClientRequestId(),
		ExpectedPublicationVersion: req.GetExpectedPublicationVersion(),
		// ⛔ 站外這條 lane 的 CAS 是 **version-only**，hash 傳空字串（checkPublicationCAS
		// 對空值跳過該項）。hash 不對站外方暴露——它是我方的內部完整性摘要，而它唯一
		// 的價值是「client 證言我看到的就是這一份」；讓 server 拿自己的 hash 跟自己比
		// 是恆真式。⚠️ 代價：擋得住「買家改版後業者才送出」，擋不住「payload 變了但
		// version 沒 bump」，後者本身是 producer 端的 bug。
		ExpectedSellerFacingPayloadHash: "",
		ExpectedSellerResponseVersion:   req.GetExpectedSellerResponseVersion(),
		Quote:                           fromSellerQuoteProto(req.GetQuote()),
		Verification:                    fromVendorPortalVerificationProto(req.GetVerification()),
	})
	if err != nil {
		return nil, procurementUsecaseErrorStatus(ctx, err)
	}
	return toSellerResponseVersionProto(result), nil
}

// WithdrawVendorPortalResponse 是站外業者的撤回（ActorNone）。
func (s *AIProcurementGrpcServer) WithdrawVendorPortalResponse(ctx context.Context, req *aipb.WithdrawVendorPortalResponseRequest) (*aipb.SellerResponseClosureResponse, error) {
	p, err := vendorPortalPrincipal(req.GetRfqRecipientId())
	if err != nil {
		return nil, err
	}
	sellerResponseID, err := parseInt64(req.GetSellerResponseId())
	if err != nil {
		return nil, procurementStatus(codes.InvalidArgument, "", "invalid sellerResponseId")
	}
	out, err := s.sellerCommands.SellerResponse.Withdraw(ctx, commerce.WithdrawSellerResponseInput{
		Principal: p, PublicationID: 0, SellerResponseID: sellerResponseID,
		ClientRequestID:               req.GetClientRequestId(),
		ExpectedSellerResponseVersion: req.GetExpectedSellerResponseVersion(),
		ReasonCode:                    req.GetReasonCode(),
		Verification:                  fromVendorPortalVerificationProto(req.GetVerification()),
	})
	if err != nil {
		return nil, procurementUsecaseErrorStatus(ctx, err)
	}
	return toSellerResponseClosureProto(out), nil
}

// DeclineVendorPortalRfqPublication 是站外業者的婉拒（ActorNone）。
func (s *AIProcurementGrpcServer) DeclineVendorPortalRfqPublication(ctx context.Context, req *aipb.DeclineVendorPortalRfqPublicationRequest) (*aipb.SellerResponseClosureResponse, error) {
	p, err := vendorPortalPrincipal(req.GetRfqRecipientId())
	if err != nil {
		return nil, err
	}
	out, err := s.sellerCommands.SellerResponse.Decline(ctx, commerce.DeclineRfqPublicationInput{
		Principal: p, PublicationID: 0,
		ClientRequestID: req.GetClientRequestId(), ReasonCode: req.GetReasonCode(),
		Verification: fromVendorPortalVerificationProto(req.GetVerification()),
	})
	if err != nil {
		return nil, procurementUsecaseErrorStatus(ctx, err)
	}
	return toSellerResponseClosureProto(out), nil
}

func fromVendorPortalVerificationProto(v *aipb.VendorPortalVerification) *repository.SellerResponseVerification {
	if v == nil {
		return nil
	}
	out := &repository.SellerResponseVerification{
		Method:    v.GetMethod(),
		Email:     v.GetEmail(),
		SessionID: v.GetSessionId(),
	}
	if at := v.GetVerifiedAt(); at != nil {
		out.VerifiedAt = at.AsTime()
	}
	return out
}

func (s *AIProcurementGrpcServer) SubmitSellerResponse(ctx context.Context, req *aipb.SubmitSellerResponseRequest) (*aipb.SellerResponseVersionResponse, error) {
	p, err := sellerPrincipalFromContext(ctx, req.GetInviteId())
	if err != nil {
		return nil, err
	}
	publicationID, err := publicationIDFrom(p, req.GetPublicationId())
	if err != nil {
		return nil, procurementStatus(codes.InvalidArgument, "", "invalid publicationId")
	}
	result, err := s.sellerCommands.SellerResponse.Submit(ctx, commerce.SubmitSellerResponseInput{
		Principal: p, PublicationID: publicationID,
		ClientRequestID:                 req.GetClientRequestId(),
		ExpectedPublicationVersion:      req.GetExpectedPublicationVersion(),
		ExpectedSellerFacingPayloadHash: req.GetExpectedSellerFacingPayloadHash(),
		ExpectedSellerResponseVersion:   req.GetExpectedSellerResponseVersion(),
		Quote:                           fromSellerQuoteProto(req.GetQuote()),
	})
	if err != nil {
		return nil, procurementUsecaseErrorStatus(ctx, err)
	}
	return toSellerResponseVersionProto(result), nil
}

func (s *AIProcurementGrpcServer) WithdrawSellerResponse(ctx context.Context, req *aipb.WithdrawSellerResponseRequest) (*aipb.SellerResponseClosureResponse, error) {
	p, err := sellerPrincipalFromContext(ctx, req.GetInviteId())
	if err != nil {
		return nil, err
	}
	publicationID, err := publicationIDFrom(p, req.GetPublicationId())
	if err != nil {
		return nil, procurementStatus(codes.InvalidArgument, "", "invalid publicationId")
	}
	sellerResponseID, err := parseInt64(req.GetSellerResponseId())
	if err != nil {
		return nil, procurementStatus(codes.InvalidArgument, "", "invalid sellerResponseId")
	}
	out, err := s.sellerCommands.SellerResponse.Withdraw(ctx, commerce.WithdrawSellerResponseInput{
		Principal: p, PublicationID: publicationID, SellerResponseID: sellerResponseID,
		ClientRequestID:               req.GetClientRequestId(),
		ExpectedSellerResponseVersion: req.GetExpectedSellerResponseVersion(),
		ReasonCode:                    req.GetReasonCode(),
	})
	if err != nil {
		return nil, procurementUsecaseErrorStatus(ctx, err)
	}
	return toSellerResponseClosureProto(out), nil
}

func (s *AIProcurementGrpcServer) DeclineRfqPublication(ctx context.Context, req *aipb.DeclineRfqPublicationRequest) (*aipb.SellerResponseClosureResponse, error) {
	p, err := sellerPrincipalFromContext(ctx, req.GetInviteId())
	if err != nil {
		return nil, err
	}
	publicationID, err := publicationIDFrom(p, req.GetPublicationId())
	if err != nil {
		return nil, procurementStatus(codes.InvalidArgument, "", "invalid publicationId")
	}
	out, err := s.sellerCommands.SellerResponse.Decline(ctx, commerce.DeclineRfqPublicationInput{
		Principal: p, PublicationID: publicationID,
		ClientRequestID: req.GetClientRequestId(), ReasonCode: req.GetReasonCode(),
	})
	if err != nil {
		return nil, procurementUsecaseErrorStatus(ctx, err)
	}
	return toSellerResponseClosureProto(out), nil
}

func (s *AIProcurementGrpcServer) ListSellerResponses(ctx context.Context, req *aipb.ListSellerResponsesRequest) (*aipb.ListSellerResponsesResponse, error) {
	buyerID, err := buyerIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	requestID, err := parseInt64(req.GetRequestId())
	if err != nil {
		return nil, procurementStatus(codes.InvalidArgument, "", "invalid requestId")
	}
	rows, err := s.sellerCommands.BuyerAcceptance.ListSellerResponses(ctx, buyerID, requestID)
	if err != nil {
		return nil, procurementUsecaseErrorStatus(ctx, err)
	}
	// publication_id 是選填的收窄條件：買家在同一張 request 上可能發過多次
	// publication，UI 一次只比較其中一批。
	var filter int64
	if req.GetPublicationId() != "" {
		if filter, err = parseInt64(req.GetPublicationId()); err != nil {
			return nil, procurementStatus(codes.InvalidArgument, "", "invalid publicationId")
		}
	}
	out := make([]*aipb.BuyerFacingSellerResponse, 0, len(rows))
	for _, row := range rows {
		if filter != 0 && row.Identity.PublicationID != filter {
			continue
		}
		out = append(out, toBuyerFacingSellerResponseProto(row))
	}
	return &aipb.ListSellerResponsesResponse{Responses: out}, nil
}

func (s *AIProcurementGrpcServer) PutShortlistEntry(ctx context.Context, req *aipb.ShortlistEntryRequest) (*aipb.ShortlistEntryResponse, error) {
	buyerID, requestID, sellerResponseID, version, err := parseShortlistRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	entry, err := s.sellerCommands.BuyerAcceptance.PutShortlist(ctx, buyerID, requestID, sellerResponseID, version)
	if err != nil {
		return nil, procurementUsecaseErrorStatus(ctx, err)
	}
	return &aipb.ShortlistEntryResponse{
		SellerResponseId:      itoa64(entry.SellerResponseID),
		SellerResponseVersion: itoa32(entry.SellerResponseVersion),
		Present:               true,
	}, nil
}

func (s *AIProcurementGrpcServer) DeleteShortlistEntry(ctx context.Context, req *aipb.ShortlistEntryRequest) (*aipb.ShortlistEntryResponse, error) {
	buyerID, requestID, sellerResponseID, version, err := parseShortlistRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	if _, err := s.sellerCommands.BuyerAcceptance.DeleteShortlist(ctx, buyerID, requestID, sellerResponseID); err != nil {
		return nil, procurementUsecaseErrorStatus(ctx, err)
	}
	// present=false 是刪除後的狀態，不是「本來就不在」的回報——DELETE 冪等，
	// 兩種情況對呼叫端是同一個結果。
	return &aipb.ShortlistEntryResponse{
		SellerResponseId:      itoa64(sellerResponseID),
		SellerResponseVersion: itoa32(version),
		Present:               false,
	}, nil
}

func parseShortlistRequest(ctx context.Context, req *aipb.ShortlistEntryRequest) (identity.BuyerID, int64, int64, int32, error) {
	buyerID, err := buyerIDFromContext(ctx)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	requestID, err := parseInt64(req.GetRequestId())
	if err != nil {
		return 0, 0, 0, 0, procurementStatus(codes.InvalidArgument, "", "invalid requestId")
	}
	sellerResponseID, err := parseInt64(req.GetSellerResponseId())
	if err != nil {
		return 0, 0, 0, 0, procurementStatus(codes.InvalidArgument, "", "invalid sellerResponseId")
	}
	version, err := strconv.ParseInt(req.GetSellerResponseVersion(), 10, 32)
	if err != nil || version <= 0 {
		return 0, 0, 0, 0, procurementStatus(codes.InvalidArgument, "", "invalid sellerResponseVersion")
	}
	return buyerID, requestID, sellerResponseID, int32(version), nil
}

func (s *AIProcurementGrpcServer) AcceptSellerQuote(ctx context.Context, req *aipb.BuyerQuoteAcceptanceRequest) (*aipb.BuyerQuoteAcceptanceResponse, error) {
	buyerID, err := buyerIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	requestID, err := parseInt64(req.GetRequestId())
	if err != nil {
		return nil, procurementStatus(codes.InvalidArgument, "", "invalid requestId")
	}
	sellerResponseID, err := parseInt64(req.GetSellerResponseId())
	if err != nil {
		return nil, procurementStatus(codes.InvalidArgument, "", "invalid sellerResponseId")
	}
	version, err := strconv.ParseInt(req.GetExpectedSellerResponseVersion(), 10, 32)
	if err != nil || version <= 0 {
		return nil, procurementStatus(codes.InvalidArgument, "", "invalid expectedSellerResponseVersion")
	}
	result, err := s.sellerCommands.BuyerAcceptance.AcceptSellerQuote(ctx, commerce.AcceptSellerQuoteInput{
		BuyerID: buyerID, RequestID: requestID,
		ClientRequestID:               req.GetClientRequestId(),
		SellerResponseID:              sellerResponseID,
		ExpectedSellerResponseVersion: int32(version),
		ExpectedRfqSnapshotHash:       req.GetExpectedRfqSnapshotHash(),
	})
	if err != nil {
		return nil, procurementUsecaseErrorStatus(ctx, err)
	}
	return toBuyerQuoteAcceptanceProto(result), nil
}

// ---- proto 轉換 ----

func itoa32(v int32) string { return strconv.FormatInt(int64(v), 10) }

func fromSellerQuoteProto(p *aipb.SellerQuotePayload) domain.SellerQuotePayload {
	if p == nil {
		return domain.SellerQuotePayload{}
	}
	return domain.SellerQuotePayload{
		ResponseType: p.GetResponseType(),
		OfferedQuality: domain.OfferedQuality{
			Grade: p.GetOfferedQuality().GetGrade(), InspectionNotes: p.GetOfferedQuality().GetInspectionNotes(),
		},
		UnitBasis: domain.UnitBasis{
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

func toSellerQuoteProto(v repository.SellerResponseVersion) *aipb.SellerQuotePayload {
	out := &aipb.SellerQuotePayload{
		ResponseType: v.ResponseType,
		OfferedQuality: &aipb.OfferedQuality{
			Grade: v.OfferedQualityGrade, InspectionNotes: v.OfferedQualityInspectionNotes,
		},
		UnitBasis:           &aipb.UnitBasis{Quantity: v.UnitBasisQuantity, Unit: v.UnitBasisUnit},
		OfferedQuantity:     v.OfferedQuantity,
		OfferedQuantityUnit: v.OfferedQuantityUnit,
		ConversionBasis:     v.ConversionBasis,
		Currency:            v.Currency,
		GoodsAmount:         strconv.FormatInt(v.GoodsAmount, 10),
		ShippingAmount:      strconv.FormatInt(v.ShippingAmount, 10),
		TaxAmount:           strconv.FormatInt(v.TaxAmount, 10),
		TaxInclusionBasis:   v.TaxInclusionBasis,
		TotalAmount:         strconv.FormatInt(v.TotalAmount, 10),
		ValidUntil:          timeutil.FormatISO8601(v.ValidUntil),
		TermsNote:           v.TermsNote,
		DeviationDisclosure: v.DeviationDisclosure,
	}
	if v.LeadTimeDays != nil {
		out.LeadTimeDays = itoa32(*v.LeadTimeDays)
	}
	if v.PromisedDeliveryAt != nil {
		out.PromisedDeliveryAt = timeutil.FormatISO8601(*v.PromisedDeliveryAt)
	}
	return out
}

func toSellerFacingPayloadProto(v repository.InviteSellerView) *aipb.RfqPublicationInviteSellerViewResponse {
	return &aipb.RfqPublicationInviteSellerViewResponse{
		Item:             v.Item,
		ItemLabel:        v.ItemLabel,
		Quantity:         v.Quantity,
		Unit:             v.Unit,
		Packaging:        v.Packaging,
		DeliveryCity:     v.DeliveryCity,
		DeliveryDate:     v.DeliveryDate,
		ResponseDeadline: v.ResponseDeadline,
		BuyerNote:        v.BuyerNote,
	}
}

func toSellerPublicationSummaryProto(pub repository.Publication, invite repository.PublicationInvite) *aipb.SellerPublicationSummary {
	return &aipb.SellerPublicationSummary{
		PublicationId:           itoa64(pub.ID),
		PublicationVersion:      strconv.Itoa(pub.Version),
		InviteId:                itoa64(invite.ID),
		Lane:                    string(invite.Lane),
		SellerFacingPayloadHash: pub.SellerFacingPayloadHash,
		DeliveryState:           invite.DeliveryState,
		PublishedAt:             timeutil.FormatISO8601(pub.PublishedAt),
	}
}

func toSellerResponseVersionProto(r commerce.SellerResponseVersionResult) *aipb.SellerResponseVersionResponse {
	out := &aipb.SellerResponseVersionResponse{
		SellerResponseId:           itoa64(r.Version.SellerResponseID),
		SellerResponseVersion:      itoa32(r.Version.Version),
		PublicationId:              itoa64(r.Identity.PublicationID),
		Status:                     r.Status,
		PayloadHash:                r.Version.PayloadHash,
		SubmittedAt:                timeutil.FormatISO8601(r.Version.SubmittedAt),
		OutsideBuyerRequestedRange: r.Version.OutsideBuyerRequestedRange,
		Quote:                      toSellerQuoteProto(r.Version),
		Replayed:                   r.Replayed,
	}
	if r.Version.Version < r.Identity.LatestVersion {
		out.SupersededByVersion = itoa32(r.Identity.LatestVersion)
	}
	return out
}

func toSellerResponseClosureProto(id repository.SellerResponseIdentity) *aipb.SellerResponseClosureResponse {
	out := &aipb.SellerResponseClosureResponse{
		SellerResponseId: itoa64(id.ID),
		PublicationId:    itoa64(id.PublicationID),
		LatestVersion:    itoa32(id.LatestVersion),
	}
	if id.WithdrawnVersion != nil {
		out.WithdrawnVersion = itoa32(*id.WithdrawnVersion)
	}
	if id.WriteClosedAt != nil {
		out.WriteClosedAt = timeutil.FormatISO8601(*id.WriteClosedAt)
	}
	if id.WriteClosedReason != nil {
		out.WriteClosedReason = *id.WriteClosedReason
	}
	return out
}

func toBuyerFacingSellerResponseProto(r commerce.BuyerFacingSellerResponse) *aipb.BuyerFacingSellerResponse {
	return &aipb.BuyerFacingSellerResponse{
		SellerResponseId:      itoa64(r.Version.SellerResponseID),
		SellerResponseVersion: itoa32(r.Version.Version),
		Status:                r.Status,
		PayloadHash:           r.Version.PayloadHash,
		SubmittedAt:           timeutil.FormatISO8601(r.Version.SubmittedAt),
		Seller: &aipb.SellerIdentityView{
			DisplayName: r.Seller.DisplayName, PartyType: r.Seller.PartyType, StoreCode: r.Seller.StoreCode,
		},
		Quote:                      toSellerQuoteProto(r.Version),
		OutsideBuyerRequestedRange: r.Version.OutsideBuyerRequestedRange,
		Shortlisted:                r.Shortlisted,
		Acceptability:              r.Acceptability,
	}
}

func toBuyerQuoteAcceptanceProto(r commerce.AcceptSellerQuoteResult) *aipb.BuyerQuoteAcceptanceResponse {
	a := r.Acceptance
	out := &aipb.BuyerQuoteAcceptanceResponse{
		BuyerAcceptanceId:      itoa64(a.ID),
		SellerResponseId:       itoa64(a.SellerResponseID),
		SellerResponseVersion:  itoa32(a.SellerResponseVersion),
		AcceptedDealPrice:      strconv.FormatInt(a.AcceptedDealPrice, 10),
		Currency:               a.Currency,
		AcceptedAt:             timeutil.FormatISO8601(a.AcceptedAt),
		OrderProvisioningState: a.OrderProvisioningState,
		Replayed:               r.Replayed,
	}
	// orderNo／paymentDeadline 空字串直到 ORDER_CREATED（D3）：ACK 的權威性只
	// 落在已成立的那一半，建單在另一個服務、另一個交易。
	if a.OrderNo != nil {
		out.OrderNo = *a.OrderNo
	}
	if a.PaymentDeadline != nil {
		out.PaymentDeadline = timeutil.FormatISO8601(*a.PaymentDeadline)
	}
	return out
}

// sellerResponseErrorStatus 把 #2538 的 usecase／repository 哨兵錯誤映到 §11 的
// 錯誤碼。回 nil 代表這一層不認得，交回共用的 default 分支。
func sellerResponseErrorStatus(err error) error {
	switch {
	case errors.Is(err, commerce.ErrIdempotencyKeyRequired):
		return procurementStatus(codes.InvalidArgument, "", "Idempotency-Key is required")
	case errors.Is(err, commerce.ErrRfqSnapshotHashRequired):
		return procurementStatus(codes.InvalidArgument, "", "expectedRfqSnapshotHash is required")
	case errors.Is(err, repository.ErrSellerResponseNotFound),
		errors.Is(err, repository.ErrPublicationNotFound),
		errors.Is(err, repository.ErrInviteNotFound),
		errors.Is(err, repository.ErrBuyerQuoteAcceptanceNotFound):
		// 跨賣家／跨買家猜 id 與亂數 id 回同一個外形（doc 64 §9.3：403 會洩漏存在性）。
		return procurementStatus(codes.NotFound, codemeta.ProcurementErrorReasonResourceNotFound.Value, "resource not found")
	case errors.Is(err, commerce.ErrStaleRfqSnapshot):
		return procurementStatus(codes.Aborted, codemeta.ProcurementErrorReasonStaleRfqSnapshot.Value, "詢價快照已變動，請重新確認後再試")
	case errors.Is(err, commerce.ErrStalePublication):
		return procurementStatus(codes.Aborted, codemeta.ProcurementErrorReasonStalePublication.Value, "發布內容已變動，請重新載入")
	case errors.Is(err, repository.ErrSellerResponseDraftStale):
		return procurementStatus(codes.Aborted, codemeta.ProcurementErrorReasonStaleSellerResponse.Value,
			"你的草稿在別的地方已經更新過，請重新載入後再儲存")
	case errors.Is(err, repository.ErrSellerResponseStale):
		return procurementStatus(codes.Aborted, codemeta.ProcurementErrorReasonStaleSellerResponse.Value, "供應商報價已變動，請重新確認後再決定")
	case errors.Is(err, commerce.ErrSellerResponseAlreadySubmitted):
		return procurementStatus(codes.Aborted, codemeta.ProcurementErrorReasonRfqResponseAlreadySubmitted.Value, "已提交過正式回價，改版請帶目前版本")
	case errors.Is(err, commerce.ErrVerificationRequired):
		return procurementStatus(codes.PermissionDenied, "", "vendor verification evidence is required")
	case errors.Is(err, commerce.ErrRfqResponseExpired):
		return procurementStatus(codes.Aborted, codemeta.ProcurementErrorReasonRfqResponseExpired.Value, "此報價版本已逾有效期")
	case errors.Is(err, repository.ErrSellerResponseWriteClosed), errors.Is(err, commerce.ErrRfqRecipientClosed):
		return procurementStatus(codes.Aborted, codemeta.ProcurementErrorReasonRfqRecipientClosed.Value, "此收件人已停止寫入")
	case errors.Is(err, commerce.ErrRfqCheckoutAlreadyActive):
		return procurementStatus(codes.Aborted, codemeta.ProcurementErrorReasonRfqCheckoutAlreadyActive.Value, "已有進行中的成交，請前往付款")
	case errors.Is(err, commerce.ErrRfqAlreadyAwarded):
		return procurementStatus(codes.Aborted, codemeta.ProcurementErrorReasonRfqAlreadyAwarded.Value, "本詢價已成交")
	case errors.Is(err, domain.ErrQuoteAmountInconsistent):
		return procurementStatus(codes.FailedPrecondition, codemeta.ProcurementErrorReasonQuoteAmountInconsistent.Value,
			"總金額需等於貨款＋運費＋稅額")
	case errors.Is(err, domain.ErrQuoteQualityRequired):
		return procurementStatus(codes.FailedPrecondition, codemeta.ProcurementErrorReasonQuoteQualityRequired.Value,
			"請填寫實際品相等級與驗收說明")
	case errors.Is(err, domain.ErrQuoteFieldInvalid):
		return procurementStatus(codes.InvalidArgument, "", err.Error())
	}
	return nil
}
