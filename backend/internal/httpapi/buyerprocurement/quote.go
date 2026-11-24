package buyerprocurement

import (
	"context"
	"strconv"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"
	"uuid"

	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/procurement/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/procurementwire"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement"
)

// 賣家回價比較與接受（A9-3b-3d）的欄位違規：模組的欄位名對到請求的 proto 路徑（模組直接用請求的欄位名，這裡只是明列）。
var quotePaths = rpc.FieldPaths{
	"page_size": "page_size", "page_token": "page_token", "seller_response_version": "seller_response_version",
	"expected_seller_response_version": "expected_seller_response_version", "expected_seller_facing_hash": "expected_seller_facing_hash",
	"client_request_id": "client_request_id",
}

// sellerResponseVersion 解析必填的回價版本號：正整數的十進位字串（版本是不透明的字串，伺服器用整數發）。
func sellerResponseVersion(path, raw string) (int32, error) {
	if raw == "" {
		return 0, rpc.RequiredField(path, path+" 不能是空的")
	}
	v, err := strconv.ParseInt(raw, 10, 32)
	if err != nil || v <= 0 {
		return 0, rpc.RequiredField(path, path+" 必須是有效的版本")
	}
	return int32(v), nil
}

// ListSellerResponses 列出一次發布收到的賣家回價（每位賣家的目前版本）；店家卡片在這裡組（沒公開過的店沒有卡片，回價照列、不設 store）。
func (h *App) ListSellerResponses(ctx context.Context, req *connect.Request[pb.ListSellerResponsesRequest]) (*connect.Response[pb.ListSellerResponsesResponse], error) {
	buyerID, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	requestID, err := rpc.ParseUUID("request_id", msg.GetRequestId())
	if err != nil {
		return nil, err
	}
	publicationID, err := rpc.ParseUUID("publication_id", msg.GetPublicationId())
	if err != nil {
		return nil, err
	}
	page, err := h.m.ListSellerResponses(ctx, procurement.ListSellerResponsesInput{
		BuyerID: buyerID, RequestID: requestID, PublicationID: publicationID, PageSize: msg.GetPageSize(), PageToken: msg.GetPageToken(),
	})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, quotePaths)
	}
	ids := make([]int64, len(page.Responses))
	for i, r := range page.Responses {
		ids[i] = r.StoreID
	}
	cards, err := h.storeCards(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := &pb.ListSellerResponsesResponse{NextPageToken: page.NextPageToken}
	for _, r := range page.Responses {
		v, err := sellerResponseToProto(r, cards[r.StoreID])
		if err != nil {
			return nil, err
		}
		out.SellerResponses = append(out.SellerResponses, v)
	}
	return connect.NewResponse(out), nil
}

// PutShortlistEntry 把回價的一個版本放進入圍清單；同一則回價已在清單時換成這個版本。
func (h *App) PutShortlistEntry(ctx context.Context, req *connect.Request[pb.PutShortlistEntryRequest]) (*connect.Response[pb.PutShortlistEntryResponse], error) {
	buyerID, requestID, responseID, version, err := h.shortlistArgs(ctx, req.Msg.GetRequestId(), req.Msg.GetSellerResponseId(), req.Msg.GetSellerResponseVersion())
	if err != nil {
		return nil, err
	}
	e, err := h.m.PutShortlistEntry(ctx, buyerID, requestID, responseID, version)
	if err != nil {
		return nil, rpc.WithFieldPaths(err, quotePaths)
	}
	return connect.NewResponse(&pb.PutShortlistEntryResponse{
		SellerResponseId: e.SellerResponseID.String(), SellerResponseVersion: strconv.Itoa(int(e.Version)), Present: e.Present,
	}), nil
}

// DeleteShortlistEntry 把回價從入圍清單移除；冪等，不在清單時回目前的狀態（present 為 false）。
func (h *App) DeleteShortlistEntry(ctx context.Context, req *connect.Request[pb.DeleteShortlistEntryRequest]) (*connect.Response[pb.DeleteShortlistEntryResponse], error) {
	buyerID, requestID, responseID, version, err := h.shortlistArgs(ctx, req.Msg.GetRequestId(), req.Msg.GetSellerResponseId(), req.Msg.GetSellerResponseVersion())
	if err != nil {
		return nil, err
	}
	e, err := h.m.DeleteShortlistEntry(ctx, buyerID, requestID, responseID, version)
	if err != nil {
		return nil, rpc.WithFieldPaths(err, quotePaths)
	}
	return connect.NewResponse(&pb.DeleteShortlistEntryResponse{
		SellerResponseId: e.SellerResponseID.String(), SellerResponseVersion: strconv.Itoa(int(e.Version)), Present: e.Present,
	}), nil
}

func (h *App) shortlistArgs(ctx context.Context, rawRequest, rawResponse, rawVersion string) (buyerID int64, requestID, responseID uuid.UUID, version int32, err error) {
	if buyerID, err = h.caller(ctx); err != nil {
		return
	}
	if requestID, err = rpc.ParseUUID("request_id", rawRequest); err != nil {
		return
	}
	if responseID, err = rpc.ParseUUID("seller_response_id", rawResponse); err != nil {
		return
	}
	version, err = sellerResponseVersion("seller_response_version", rawVersion)
	return
}

// AcceptSellerQuote 接受回價的指定版本並凍結成交價，同時排入建立待付款訂單的工作；建單本身不在這個請求裡，所以回應的訂單狀態是 CREATING_ORDER、沒有 order_no。
func (h *App) AcceptSellerQuote(ctx context.Context, req *connect.Request[pb.AcceptSellerQuoteRequest]) (*connect.Response[pb.AcceptSellerQuoteResponse], error) {
	buyerID, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	requestID, err := rpc.ParseUUID("request_id", msg.GetRequestId())
	if err != nil {
		return nil, err
	}
	cid, err := clientRequestID(msg.GetClientRequestId())
	if err != nil {
		return nil, err
	}
	responseID, err := rpc.ParseUUID("seller_response_id", msg.GetSellerResponseId())
	if err != nil {
		return nil, err
	}
	version, err := sellerResponseVersion("expected_seller_response_version", msg.GetExpectedSellerResponseVersion())
	if err != nil {
		return nil, err
	}
	if msg.GetExpectedSellerFacingHash() == "" {
		return nil, rpc.RequiredField("expected_seller_facing_hash", "expected_seller_facing_hash 不能是空的")
	}
	a, err := h.m.AcceptSellerQuote(ctx, procurement.AcceptSellerQuoteInput{
		BuyerID: buyerID, RequestID: requestID, ClientRequestID: cid, ResponseID: responseID, ExpectedVersion: version,
		ExpectedSellerFacingHash: msg.GetExpectedSellerFacingHash(),
	})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, quotePaths)
	}
	state, err := procurementwire.Enum[pb.OrderProvisioningState](pb.OrderProvisioningState_value, "ORDER_PROVISIONING_STATE_", "建單狀態", a.Provisioning)
	if err != nil {
		return nil, err
	}
	out := &pb.AcceptSellerQuoteResponse{
		BuyerAcceptanceId: a.ID.String(), SellerResponseId: a.SellerResponseID.String(), SellerResponseVersion: strconv.Itoa(int(a.Version)),
		AcceptedDealPrice: a.AcceptedDealPrice, AcceptTime: timestamppb.New(a.AcceptedAt), OrderProvisioningState: state, Replayed: a.Replayed,
	}
	if a.OrderNo != nil {
		out.OrderNo = *a.OrderNo
	}
	if a.PaymentDeadline != nil {
		out.PaymentDeadline = timestamppb.New(*a.PaymentDeadline)
	}
	return connect.NewResponse(out), nil
}

func sellerResponseToProto(r procurement.SellerResponse, card *commonpb.StoreCard) (*pb.BuyerFacingSellerResponse, error) {
	status, err := procurementwire.Enum[pb.SellerResponseStatus](pb.SellerResponseStatus_value, "SELLER_RESPONSE_STATUS_", "回價狀態", r.Status)
	if err != nil {
		return nil, err
	}
	acceptability, err := procurementwire.Enum[pb.QuoteAcceptability](pb.QuoteAcceptability_value, "QUOTE_ACCEPTABILITY_", "可否接受", r.Acceptability)
	if err != nil {
		return nil, err
	}
	quote, err := procurementwire.SellerQuote(r.Quote)
	if err != nil {
		return nil, err
	}
	return &pb.BuyerFacingSellerResponse{
		SellerResponseId: r.ID.String(), SellerResponseVersion: strconv.Itoa(int(r.Version)), Status: status, PayloadHash: r.PayloadHash,
		SubmitTime: timestamppb.New(r.SubmittedAt), Store: card, Quote: quote, OutsideBuyerRequestedRange: r.OutsideBuyerRequestedRange,
		Shortlisted: r.Shortlisted, Acceptability: acceptability,
	}, nil
}
