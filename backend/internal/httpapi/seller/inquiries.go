package seller

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1/sellerv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
)

// 詢問的方法以帳號為範圍（宣告為豁免，不要求賣家角色或店家營運中）：詢問開放中，已關閉的店家也能回覆（案件結案時系統撤回）。帳號是登入的呼叫者；
// 詢問是不是這個帳號的店家的，由模組以 account ID 檢查，別人的與不存在的一律 not_found。

var (
	inquiryListPaths  = rpc.FieldPaths{"State": "state"}
	inquiryReplyPaths = rpc.FieldPaths{"InquiryID": "inquiry_id", "Body": "body", "EvidenceTokens[]": "evidence_staged_tokens[]"}
)

// toSellerInquiry 轉成賣家看到的詢問：不輸出案件、店家、管理員的識別與撤回的原因；期限只在還沒結束時輸出（開始是建立時間，期限是建立時存下的）；
// can_reply 取自模組的同一個 Availability；回覆只帶內容、管道、收到時間與附件數（賣家不讀回附件，不輸出物件鍵或網址）。
func toSellerInquiry(i model.SellerInquiry) *pb.Inquiry {
	out := &pb.Inquiry{
		Id: i.ID.String(), Kind: rpc.EnumToProto[pb.InquiryKind](i.Kind), Message: i.Message, CreateTime: timestamppb.New(i.CreatedAt),
		State: rpc.EnumToProto[pb.InquiryState](i.State()), CanReply: i.CanReply, ReplyAttachmentMax: model.MaxEvidence,
	}
	if i.Open() {
		out.ReplyDeadline = &commonpb.Deadline{StartTime: timestamppb.New(i.CreatedAt), DueTime: timestamppb.New(i.DueAt)}
	}
	if r := i.Reply; r != nil {
		out.Reply = &pb.InquiryReply{
			Body: r.Body, Channel: rpc.EnumToProto[pb.InquiryReplyChannel](r.Channel), ReceiveTime: timestamppb.New(r.ReceivedAt),
			AttachmentCount: int32(i.AttachmentCount), //nolint:gosec // 附件最多 MaxEvidence
		}
	}
	if w := i.Withdrawn; w != nil {
		out.WithdrawTime = timestamppb.New(w.At)
	}
	return out
}

func (h *App) ListMyInquiries(ctx context.Context, req *connect.Request[pb.ListMyInquiriesRequest]) (*connect.Response[pb.ListMyInquiriesResponse], error) {
	p, ok := rpc.PrincipalFrom(ctx)
	if !ok {
		return nil, errNoPrincipal
	}
	msg := req.Msg
	scope := rpc.FilterScope(sellerv1alpha1connect.SellerAppServiceListMyInquiriesProcedure, msg.GetState().String())
	scope = rpc.PageScope(scope, p.ID)
	after, err := rpc.ValidCursor[model.InquiryCursor](scope, msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	in := model.AccountInquiryListInput{AccountID: p.ID, PageSize: msg.GetPageSize(), After: after}
	if state := rpc.EnumFromProto[model.InquiryState](msg.GetState()); state != "" {
		in.State = &state
	}
	page, err := h.m.AccountInquiries(ctx, in)
	if err != nil {
		return nil, rpc.WithFieldPaths(err, inquiryListPaths)
	}
	out := &pb.ListMyInquiriesResponse{Inquiries: make([]*pb.Inquiry, len(page.Items)), PendingReplyCount: page.PendingReplyCount}
	for i, item := range page.Items {
		out.Inquiries[i] = toSellerInquiry(item)
	}
	if page.Next != nil {
		out.NextPageToken = rpc.TokenFor(scope, page.Next)
	}
	return connect.NewResponse(out), nil
}

func (h *App) GetMyInquiry(ctx context.Context, req *connect.Request[pb.GetMyInquiryRequest]) (*connect.Response[pb.GetMyInquiryResponse], error) {
	p, ok := rpc.PrincipalFrom(ctx)
	if !ok {
		return nil, errNoPrincipal
	}
	id, err := rpc.ParseUUID("inquiry_id", req.Msg.GetInquiryId())
	if err != nil {
		return nil, err
	}
	i, err := h.m.AccountInquiry(ctx, p.ID, id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.GetMyInquiryResponse{Inquiry: toSellerInquiry(i)}), nil
}

func (h *App) ReplyToInquiry(ctx context.Context, req *connect.Request[pb.ReplyToInquiryRequest]) (*connect.Response[pb.ReplyToInquiryResponse], error) {
	p, ok := rpc.PrincipalFrom(ctx)
	if !ok {
		return nil, errNoPrincipal
	}
	id, err := rpc.ParseUUID("inquiry_id", req.Msg.GetInquiryId())
	if err != nil {
		return nil, err
	}
	i, err := h.m.ReplyToInquiry(ctx, model.ReplyToInquiryInput{
		AccountID: p.ID, InquiryID: id, Body: req.Msg.GetBody(), EvidenceTokens: req.Msg.GetEvidenceStagedTokens(),
	})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, inquiryReplyPaths)
	}
	return connect.NewResponse(&pb.ReplyToInquiryResponse{Inquiry: toSellerInquiry(i)}), nil
}
