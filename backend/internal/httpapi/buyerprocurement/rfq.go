package buyerprocurement

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/protobuf/types/known/timestamppb"
	"uuid"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/procurement/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// RFQ 發布鏈三支方法的欄位違規對照：模組的欄位名對到請求的 proto 路徑。
var (
	pricePaths   = rpc.FieldPaths{"LowerBoundAmount": "buyer_requested_range.lower_bound_amount", "UpperBoundAmount": "buyer_requested_range.upper_bound_amount"}
	publishPaths = rpc.FieldPaths{
		"LowerBoundAmount": "buyer_requested_range.lower_bound_amount", "UpperBoundAmount": "buyer_requested_range.upper_bound_amount", "BuyerNote": "buyer_note",
		"AudienceMode": "audience_mode", "CandidateSetID": "candidate_set_id", "SelectedCandidateIDs[]": "selected_candidate_ids[]",
		"ResponseDeadline": "response_deadline", "client_request_id": "client_request_id",
	}
	executePaths = rpc.FieldPaths{"client_request_id": "client_request_id"}
)

// PreviewRfqPrice 依買家預期的單價區間算出詢價總額區間（純計算，不寫入）。
func (h *App) PreviewRfqPrice(ctx context.Context, req *connect.Request[pb.PreviewRfqPriceRequest]) (*connect.Response[pb.PreviewRfqPriceResponse], error) {
	buyerID, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	requestID, err := rpc.ParseUUID("request_id", msg.GetRequestId())
	if err != nil {
		return nil, err
	}
	expected, err := rpc.ParseUUID("expected_revision_id", msg.GetExpectedRevisionId())
	if err != nil {
		return nil, err
	}
	r := msg.GetBuyerRequestedRange()
	if r == nil {
		return nil, rpc.RequiredField("buyer_requested_range", "buyer_requested_range 不能是空的")
	}
	p, err := h.m.PreviewRfqPrice(ctx, procurement.PreviewRfqPriceInput{
		BuyerID: buyerID, RequestID: requestID, ExpectedRevisionID: expected, LowerBoundAmount: r.GetLowerBoundAmount(), UpperBoundAmount: r.GetUpperBoundAmount(),
	})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, pricePaths)
	}
	out := &pb.PreviewRfqPriceResponse{
		BuyerRequestedRange: &pb.BuyerRequestedRange{
			LowerBoundAmount: p.LowerBoundAmount, UpperBoundAmount: p.UpperBoundAmount, BasisQuantity: p.BasisQuantity, BasisUnit: p.BasisUnit,
		},
	}
	if t := p.Total; t != nil {
		out.QuoteTotalRange = &pb.QuoteTotalRange{LowerBoundAmount: t.LowerBoundAmount, UpperBoundAmount: t.UpperBoundAmount, Quantity: t.Quantity, QuantityUnit: t.Unit}
	}
	return connect.NewResponse(out), nil
}

// PreviewRfqPublication 編出要發給賣家的內容並建立不可變、只能執行一次的發布核准。
func (h *App) PreviewRfqPublication(ctx context.Context, req *connect.Request[pb.PreviewRfqPublicationRequest]) (*connect.Response[pb.PreviewRfqPublicationResponse], error) {
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
	expected, err := rpc.ParseUUID("expected_revision_id", msg.GetExpectedRevisionId())
	if err != nil {
		return nil, err
	}
	r := msg.GetBuyerRequestedRange()
	if r == nil {
		return nil, rpc.RequiredField("buyer_requested_range", "buyer_requested_range 不能是空的")
	}
	mode, err := audienceModeFromProto(msg.GetAudienceMode())
	if err != nil {
		return nil, err
	}
	in := procurement.PreviewRfqPublicationInput{
		BuyerID: buyerID, RequestID: requestID, ClientRequestID: cid, ExpectedRevisionID: expected,
		LowerBoundAmount: r.GetLowerBoundAmount(), UpperBoundAmount: r.GetUpperBoundAmount(), BuyerNote: msg.GetBuyerNote(), AudienceMode: mode,
	}
	if raw := msg.GetCandidateSetId(); raw != "" {
		id, err := rpc.ParseUUID("candidate_set_id", raw)
		if err != nil {
			return nil, err
		}
		in.CandidateSetID = &id
	}
	for i, raw := range msg.GetSelectedCandidateIds() {
		id, err := uuid.Parse(raw)
		if err != nil || len(raw) != 36 {
			return nil, rpc.WithFieldPaths(apperr.InvalidInputOn([]string{apperr.IndexedField("SelectedCandidateIDs", i)}, "selected_candidate_ids[%d] 必須是有效的 ID", i), publishPaths)
		}
		in.SelectedCandidateIDs = append(in.SelectedCandidateIDs, id)
	}
	if in.ResponseDeadline, err = dateFromProto("response_deadline", msg.ResponseDeadline); err != nil {
		return nil, err
	}
	p, err := h.m.PreviewRfqPublication(ctx, in)
	if err != nil {
		return nil, rpc.WithFieldPaths(err, publishPaths)
	}
	return connect.NewResponse(&pb.PreviewRfqPublicationResponse{
		ApprovalId: p.ApprovalID.String(), ApprovalVersion: strconv.FormatInt(int64(p.ApprovalVersion), 10), AudienceHash: p.AudienceHash,
		SellerFacingPayloadHash: p.SellerFacingHash, SellerFacingPayload: sellerFacingToProto(p.Payload), ExpireTime: timestamppb.New(p.ExpiresAt),
		RecipientCount: p.RecipientCount,
	}), nil
}

// ExecuteRfqPublication 以發布核准的版本執行發布。
func (h *App) ExecuteRfqPublication(ctx context.Context, req *connect.Request[pb.ExecuteRfqPublicationRequest]) (*connect.Response[pb.ExecuteRfqPublicationResponse], error) {
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
	approvalID, err := rpc.ParseUUID("approval_id", msg.GetApprovalId())
	if err != nil {
		return nil, err
	}
	version, err := strconv.ParseInt(msg.GetApprovalVersion(), 10, 32)
	if err != nil || version < 1 {
		return nil, rpc.RequiredField("approval_version", "approval_version 必須是預覽回的核准版本")
	}
	expected, err := rpc.ParseUUID("expected_revision_id", msg.GetExpectedRevisionId())
	if err != nil {
		return nil, err
	}
	if msg.GetPayloadHash() == "" {
		return nil, rpc.RequiredField("payload_hash", "payload_hash 不能是空的")
	}
	pub, err := h.m.ExecuteRfqPublication(ctx, procurement.ExecuteRfqPublicationInput{
		BuyerID: buyerID, RequestID: requestID, ClientRequestID: cid, ApprovalID: approvalID, ApprovalVersion: int32(version), //nolint:gosec // ParseInt 的 bitSize 32 已限制範圍
		ExpectedRevisionID: expected, PayloadHash: msg.GetPayloadHash(),
	})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, executePaths)
	}
	state, err := publicationStateToProto(pub.State)
	if err != nil {
		return nil, err
	}
	mode, err := audienceModeToProto(pub.AudienceMode)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.ExecuteRfqPublicationResponse{
		PublicationId: pub.ID.String(), PublicationVersion: strconv.FormatInt(int64(pub.Version), 10), State: state, AudienceMode: mode,
		AudienceHash: pub.AudienceHash, SellerFacingPayloadHash: pub.SellerFacingHash, PublishTime: timestamppb.New(pub.PublishedAt), Replayed: pub.Replayed,
	}), nil
}

// audienceModeFromProto 把 proto 的受眾模式轉成模組的代碼；UNSPECIFIED 與不認得的值是請求錯誤。
func audienceModeFromProto(m pb.AudienceMode) (string, error) {
	switch m {
	case pb.AudienceMode_AUDIENCE_MODE_DIRECT_RECIPIENTS:
		return procurement.AudienceDirectRecipients, nil
	case pb.AudienceMode_AUDIENCE_MODE_UNSPECIFIED:
		return "", rpc.RequiredField("audience_mode", "audience_mode 不能是空的")
	default:
		return "", rpc.RequiredField("audience_mode", "audience_mode 不認得")
	}
}

func audienceModeToProto(code string) (pb.AudienceMode, error) {
	v, ok := pb.AudienceMode_value["AUDIENCE_MODE_"+code]
	if !ok || v == int32(pb.AudienceMode_AUDIENCE_MODE_UNSPECIFIED) {
		return 0, fmt.Errorf("buyerprocurement：受眾模式 %q 沒有對應的 AudienceMode", code)
	}
	return pb.AudienceMode(v), nil
}

func publicationStateToProto(code string) (pb.PublicationState, error) {
	v, ok := pb.PublicationState_value["PUBLICATION_STATE_"+strings.ToUpper(code)]
	if !ok || v == int32(pb.PublicationState_PUBLICATION_STATE_UNSPECIFIED) {
		return 0, fmt.Errorf("buyerprocurement：發布狀態 %q 沒有對應的 PublicationState", code)
	}
	return pb.PublicationState(v), nil
}

// dateFromProto 把 google.type.Date 轉成當日 UTC 午夜；nil 是沒設，不合法的日曆日期是請求錯誤。
func dateFromProto(path string, d *date.Date) (*time.Time, error) {
	if d == nil {
		return nil, nil
	}
	t := time.Date(int(d.GetYear()), time.Month(d.GetMonth()), int(d.GetDay()), 0, 0, 0, 0, time.UTC)
	if d.GetYear() < 1 || d.GetYear() > 9999 || t.Year() != int(d.GetYear()) || t.Month() != time.Month(d.GetMonth()) || t.Day() != int(d.GetDay()) {
		return nil, rpc.RequiredField(path, path+" 不是合法的日期")
	}
	return &t, nil
}

func dateToProto(t *time.Time) *date.Date {
	if t == nil {
		return nil
	}
	return &date.Date{Year: int32(t.Year()), Month: int32(t.Month()), Day: int32(t.Day())} //nolint:gosec // time.Time 的日曆欄位在 Date 的範圍內
}

func sellerFacingToProto(p procurement.SellerFacing) *pb.SellerFacingRfq {
	return &pb.SellerFacingRfq{
		Item: p.Item, ItemLabel: p.ItemLabel, Quantity: p.Quantity, Unit: p.Unit, Packaging: p.Packaging, DeliveryCity: p.DeliveryCity,
		DeliveryDate: dateToProto(p.DeliveryDate), ResponseDeadline: dateToProto(p.ResponseDate), BuyerNote: p.BuyerNote,
		BuyerRequestedRange: &pb.SellerFacingRequestedRange{
			BasisQuantity: p.BasisQuantity, BasisUnit: p.BasisUnit, LowerBoundAmount: p.LowerBoundAmount, UpperBoundAmount: p.UpperBoundAmount,
		},
	}
}
