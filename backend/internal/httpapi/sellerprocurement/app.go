// Package sellerprocurement 實作賣家採購 Connect API（SellerProcurementAppService）的傳輸：請求驗證、店家身分的推導、錯誤與欄位路徑對應、
// 模組結果轉成 proto。可見性、版本、冪等與停止寫入的規則都在 modules/procurement，賣家能力也由模組檢查，這裡不重做。
//
// 實作 6 支方法：ListSellerPublications、GetSellerPublication、UpsertSellerResponseDraft、SubmitSellerResponse、WithdrawSellerResponse、DeclineRfqPublication。
package sellerprocurement

import (
	"context"
	"fmt"
	"log/slog"

	"connectrpc.com/connect"
	"uuid"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/procurement/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/procurement/v1alpha1/procurementv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/requestid"
)

// Module 是賣家採購業務的介面（*procurement.Module）。
type Module interface {
	ListSellerPublications(ctx context.Context, storeID int64, pageSize int32, pageToken string) (procurement.SellerPublicationPage, error)
	GetSellerPublication(ctx context.Context, storeID int64, publicationID uuid.UUID) (procurement.SellerPublicationDetail, error)
	UpsertSellerResponseDraft(ctx context.Context, storeID int64, publicationID uuid.UUID, expectedDraftVersion string, draft procurement.SellerQuoteInput) (procurement.SellerDraft, error)
	SubmitSellerResponse(ctx context.Context, in procurement.SubmitSellerResponseInput) (procurement.SellerResponseVersion, error)
	WithdrawSellerResponse(ctx context.Context, storeID int64, publicationID, sellerResponseID uuid.UUID, expectedVersion, reasonCode string) (procurement.SellerResponseClosure, error)
	DeclineRfqPublication(ctx context.Context, storeID int64, publicationID uuid.UUID, reasonCode string) (procurement.SellerResponseClosure, error)
}

// Stores 是呼叫者目前的店的來源（*seller.Module）；賣家能力由 procurement 模組檢查，不在這裡。
type Stores interface {
	CurrentStore(ctx context.Context, accountID int64) (seller.Store, error)
}

// App 是 SellerProcurementAppService 的 handler。
type App struct {
	m      Module
	stores Stores
	log    *slog.Logger
}

var _ procurementv1alpha1connect.SellerProcurementAppServiceHandler = (*App)(nil)

// NewApp 建立 handler。能力宣告表（declarations）與 SellerProcurementAppService 的方法對不上時回錯誤，不能啟動。
func NewApp(m Module, stores Stores, log *slog.Logger) (*App, error) {
	return newApp(declarations, m, stores, log)
}

// newApp 的宣告表由參數給（測試用缺一列的表驗證啟動檢查）。
func newApp(table map[string]seller.Capability, m Module, stores Stores, log *slog.Logger) (*App, error) {
	switch {
	case log == nil:
		return nil, fmt.Errorf("httpapi/sellerprocurement：需要 logger")
	case m == nil:
		return nil, fmt.Errorf("httpapi/sellerprocurement：需要 Module")
	case stores == nil:
		return nil, fmt.Errorf("httpapi/sellerprocurement：需要 Stores")
	}
	if err := checkDeclarations(table, serviceDescriptor()); err != nil {
		return nil, err
	}
	return &App{m: m, stores: stores, log: log}, nil
}

// storeID 推導呼叫者目前的店家：沒有登入由 interceptor 擋，管理員回 permission_denied，呼叫者沒有店回 not_found。賣家能力（停權、前置條件）由模組檢查。
func (h *App) storeID(ctx context.Context) (int64, error) {
	p, err := rpc.RequireAccount(ctx)
	if err != nil {
		return 0, err
	}
	st, err := h.stores.CurrentStore(ctx, p.ID)
	if err != nil {
		return 0, err
	}
	return st.StoreID, nil
}

// 模組欄位違規的欄位名對到請求的 proto 路徑；報價內容在草稿的 draft 底下、正式回價的 quote 底下。
func paths(quotePrefix string) rpc.FieldPaths {
	p := rpc.FieldPaths{
		"PageSize": "page_size", "page_token": "page_token", "expected_draft_version": "expected_draft_version",
		"expected_publication_version": "expected_publication_version", "expected_seller_facing_payload_hash": "expected_seller_facing_payload_hash",
		"expected_seller_response_version": "expected_seller_response_version", "client_request_id": "client_request_id", "reason_code": "reason_code",
	}
	if quotePrefix == "" {
		return p
	}
	p["Draft"] = quotePrefix
	for field, path := range map[string]string{
		"ResponseType": "response_type", "SpecNote": "spec_note", "UnitBasisQuantity": "unit_basis.quantity", "UnitBasisUnit": "unit_basis.unit",
		"OfferedQuantity": "offered_quantity", "OfferedQuantityUnit": "offered_quantity_unit", "ConversionBasis": "conversion_basis",
		"GoodsAmount": "goods_amount", "ShippingAmount": "shipping_amount", "TaxAmount": "tax_amount", "TaxInclusionBasis": "tax_inclusion_basis",
		"TotalAmount": "total_amount", "LeadTimeDays": "lead_time_days", "PromisedDelivery": "promised_delivery_time", "ValidUntil": "valid_until_time",
		"TermsNote": "terms_note", "DeviationDisclosure": "deviation_disclosure",
	} {
		p[field] = quotePrefix + "." + path
	}
	return p
}

var (
	listPaths     = paths("")
	draftPaths    = paths("draft")
	submitPaths   = paths("quote")
	withdrawPaths = paths("")
	declinePaths  = paths("")
)

// clientRequestID 解析必填的冪等鍵：UUIDv4。
func clientRequestID(raw string) (uuid.UUID, error) {
	if raw == "" {
		return uuid.UUID{}, rpc.RequiredField("client_request_id", "client_request_id 不能是空的")
	}
	id, err := uuid.Parse(raw)
	if err != nil || len(raw) != 36 || !requestid.IsV4(id) {
		return uuid.UUID{}, rpc.RequiredField("client_request_id", "client_request_id 必須是 UUIDv4")
	}
	return id, nil
}

// ListSellerPublications 列出發給呼叫者的詢價，最新的在前。
func (h *App) ListSellerPublications(ctx context.Context, req *connect.Request[pb.ListSellerPublicationsRequest]) (*connect.Response[pb.ListSellerPublicationsResponse], error) {
	storeID, err := h.storeID(ctx)
	if err != nil {
		return nil, err
	}
	page, err := h.m.ListSellerPublications(ctx, storeID, req.Msg.GetPageSize(), req.Msg.GetPageToken())
	if err != nil {
		return nil, rpc.WithFieldPaths(err, listPaths)
	}
	out := &pb.ListSellerPublicationsResponse{NextPageToken: page.NextPageToken}
	for _, it := range page.Items {
		item, err := itemToProto(it)
		if err != nil {
			return nil, err
		}
		out.Publications = append(out.Publications, item)
	}
	return connect.NewResponse(out), nil
}

// GetSellerPublication 回傳一筆詢價的內容，加上呼叫者自己的草稿與目前的正式回價。
func (h *App) GetSellerPublication(ctx context.Context, req *connect.Request[pb.GetSellerPublicationRequest]) (*connect.Response[pb.GetSellerPublicationResponse], error) {
	publicationID, err := rpc.ParseUUID("publication_id", req.Msg.GetPublicationId())
	if err != nil {
		return nil, err
	}
	storeID, err := h.storeID(ctx)
	if err != nil {
		return nil, err
	}
	d, err := h.m.GetSellerPublication(ctx, storeID, publicationID)
	if err != nil {
		return nil, err
	}
	out, err := detailToProto(d)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(out), nil
}

// UpsertSellerResponseDraft 建立或更新回價草稿。
func (h *App) UpsertSellerResponseDraft(ctx context.Context, req *connect.Request[pb.UpsertSellerResponseDraftRequest]) (*connect.Response[pb.UpsertSellerResponseDraftResponse], error) {
	msg := req.Msg
	publicationID, err := rpc.ParseUUID("publication_id", msg.GetPublicationId())
	if err != nil {
		return nil, err
	}
	if msg.GetDraft() == nil {
		return nil, rpc.RequiredField("draft", "draft 不能是空的")
	}
	draft, err := quoteInputFromProto(msg.GetDraft(), "draft")
	if err != nil {
		return nil, err
	}
	storeID, err := h.storeID(ctx)
	if err != nil {
		return nil, err
	}
	d, err := h.m.UpsertSellerResponseDraft(ctx, storeID, publicationID, msg.GetExpectedDraftVersion(), draft)
	if err != nil {
		return nil, rpc.WithFieldPaths(err, draftPaths)
	}
	pd, err := draftToProto(&d)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.UpsertSellerResponseDraftResponse{Draft: pd}), nil
}

// SubmitSellerResponse 送出正式回價，產生不可變的新版本。
func (h *App) SubmitSellerResponse(ctx context.Context, req *connect.Request[pb.SubmitSellerResponseRequest]) (*connect.Response[pb.SubmitSellerResponseResponse], error) {
	msg := req.Msg
	publicationID, err := rpc.ParseUUID("publication_id", msg.GetPublicationId())
	if err != nil {
		return nil, err
	}
	cid, err := clientRequestID(msg.GetClientRequestId())
	if err != nil {
		return nil, err
	}
	if msg.GetQuote() == nil {
		return nil, rpc.RequiredField("quote", "quote 不能是空的")
	}
	quote, err := quoteInputFromProto(msg.GetQuote(), "quote")
	if err != nil {
		return nil, err
	}
	storeID, err := h.storeID(ctx)
	if err != nil {
		return nil, err
	}
	v, err := h.m.SubmitSellerResponse(ctx, procurement.SubmitSellerResponseInput{
		StoreID: storeID, PublicationID: publicationID, ClientRequestID: cid,
		ExpectedPublicationVersion: msg.GetExpectedPublicationVersion(), ExpectedSellerFacingHash: msg.GetExpectedSellerFacingPayloadHash(),
		ExpectedResponseVersion: msg.GetExpectedSellerResponseVersion(), Quote: quote,
	})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, submitPaths)
	}
	pv, err := versionToProto(v)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.SubmitSellerResponseResponse{SellerResponse: pv}), nil
}

// WithdrawSellerResponse 撤回指定版本的正式回價（目標狀態的方法）。
func (h *App) WithdrawSellerResponse(ctx context.Context, req *connect.Request[pb.WithdrawSellerResponseRequest]) (*connect.Response[pb.WithdrawSellerResponseResponse], error) {
	msg := req.Msg
	publicationID, err := rpc.ParseUUID("publication_id", msg.GetPublicationId())
	if err != nil {
		return nil, err
	}
	responseID, err := rpc.ParseUUID("seller_response_id", msg.GetSellerResponseId())
	if err != nil {
		return nil, err
	}
	storeID, err := h.storeID(ctx)
	if err != nil {
		return nil, err
	}
	c, err := h.m.WithdrawSellerResponse(ctx, storeID, publicationID, responseID, msg.GetExpectedSellerResponseVersion(), msg.GetReasonCode())
	if err != nil {
		return nil, rpc.WithFieldPaths(err, withdrawPaths)
	}
	return connect.NewResponse(&pb.WithdrawSellerResponseResponse{Closure: closureToProto(c)}), nil
}

// DeclineRfqPublication 婉拒一筆詢價（目標狀態的方法）。
func (h *App) DeclineRfqPublication(ctx context.Context, req *connect.Request[pb.DeclineRfqPublicationRequest]) (*connect.Response[pb.DeclineRfqPublicationResponse], error) {
	msg := req.Msg
	publicationID, err := rpc.ParseUUID("publication_id", msg.GetPublicationId())
	if err != nil {
		return nil, err
	}
	storeID, err := h.storeID(ctx)
	if err != nil {
		return nil, err
	}
	c, err := h.m.DeclineRfqPublication(ctx, storeID, publicationID, msg.GetReasonCode())
	if err != nil {
		return nil, rpc.WithFieldPaths(err, declinePaths)
	}
	return connect.NewResponse(&pb.DeclineRfqPublicationResponse{Closure: closureToProto(c)}), nil
}
