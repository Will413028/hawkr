package seller

import (
	"context"
	"strconv"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1/sellerv1alpha1connect"
	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
)

// 案件的詢問（step10e-reports-brief R4、設計 step10e-3cb-design §1.2、§4）。

// 模組輸入的欄位對應到請求的路徑。
var (
	createInquiryPaths      = rpc.FieldPaths{"Kind": "kind", "Message": "message", "DueAt": "due_time", "RequestID": "request_id"}
	recordInquiryReplyPaths = rpc.FieldPaths{"Channel": "channel", "ReceivedAt": "receive_time", "Body": "body", "EvidenceTokens[]": "evidence_tokens[]"}
)

// caseInquiryEvidenceURI 是詢問回覆附件的檢視網址（EvidenceInquiryRoute 的實例）。
func caseInquiryEvidenceURI(inquiryID uuid.UUID, position int) string {
	return "/api/files/admin/seller/case_inquiries/" + inquiryID.String() + "/evidence/" + strconv.Itoa(position)
}

// toCaseInquiry 把後台看到的詢問轉成契約：期限只在還沒結束時輸出（義務還在時才輸出）；can_* 取自同一個 Availability（不輸出原因）；
// 回覆附件的上限是模組的 MaxEvidence（RecordCaseInquiryReply 驗證的同一個值）。
func toCaseInquiry(i model.AdminInquiry) *adminpb.CaseInquiry {
	out := &adminpb.CaseInquiry{
		Id: i.ID.String(), CaseId: i.CaseID.String(), StoreId: rpc.FormatID(i.StoreID),
		Kind: rpc.EnumToProto[adminpb.CaseInquiryKind](i.Kind), Message: i.Message,
		CreateTime: timestamppb.New(i.CreatedAt), CreatedByAdminId: rpc.FormatID(i.CreatedByAdminID),
		State:          rpc.EnumToProto[adminpb.CaseInquiryState](i.State()),
		CanRecordReply: len(i.Availability.RecordReply) == 0, CanWithdraw: len(i.Availability.Withdraw) == 0,
		ReplyEvidenceMax: model.MaxEvidence,
	}
	if i.Open() {
		// 賣家的回覆期限：建立時存下的 due_at（不是營運的審閱政策），開始是建立時間。
		out.ReplyDeadline = &commonpb.Deadline{StartTime: timestamppb.New(i.CreatedAt), DueTime: timestamppb.New(i.DueAt)}
	}
	if r := i.Reply; r != nil {
		out.Reply = &adminpb.CaseInquiryReply{
			Body: r.Body, Channel: rpc.EnumToProto[adminpb.CaseInquiryReplyChannel](r.Channel),
			ReceiveTime: timestamppb.New(r.ReceivedAt), RecordTime: timestamppb.New(r.RecordedAt), RecordedByAdminId: rpc.FormatID(r.RecordedByAdminID),
			Evidence: toEvidence(len(r.Evidence), func(n int) string { return caseInquiryEvidenceURI(i.ID, n) }),
		}
	}
	if w := i.Withdrawn; w != nil {
		out.WithdrawCause = rpc.EnumToProto[adminpb.CaseInquiryWithdrawCause](w.Cause)
		out.WithdrawTime = timestamppb.New(w.At)
		out.WithdrawnByAdminId = rpc.FormatID(w.AdminID)
	}
	return out
}

func (h *Governance) CreateCaseInquiry(ctx context.Context, req *connect.Request[adminpb.CreateCaseInquiryRequest]) (*connect.Response[adminpb.CreateCaseInquiryResponse], error) {
	adminID, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	caseID, err := rpc.ParseUUID("case_id", msg.GetCaseId())
	if err != nil {
		return nil, err
	}
	reqID, err := requestID(msg.GetRequestId())
	if err != nil {
		return nil, err
	}
	result, err := h.m.CreateCaseInquiry(ctx, model.CreateCaseInquiryInput{
		CaseID: caseID, AdminID: adminID, Kind: rpc.EnumFromProto[model.InquiryKind](msg.GetKind()), Message: msg.GetMessage(),
		DueAt: optionalTime(msg.GetDueTime()), RequestID: reqID,
	})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, createInquiryPaths)
	}
	return connect.NewResponse(&adminpb.CreateCaseInquiryResponse{Inquiry: toCaseInquiry(result.Inquiry), Case: toCase(result.Case)}), nil
}

// ListCaseInquiries 是案件詢問的分頁讀取（舊到新）。page token 綁定案件，換一個案件就從第一頁開始。
func (h *Governance) ListCaseInquiries(ctx context.Context, req *connect.Request[adminpb.ListCaseInquiriesRequest]) (*connect.Response[adminpb.ListCaseInquiriesResponse], error) {
	msg := req.Msg
	caseID, err := rpc.ParseUUID("case_id", msg.GetCaseId())
	if err != nil {
		return nil, err
	}
	scope := rpc.FilterScope(sellerv1alpha1connect.SellerGovernanceAdminServiceListCaseInquiriesProcedure, msg.GetCaseId())
	after, err := rpc.ValidCursor[model.InquiryCursor](scope, msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	page, err := h.m.CaseInquiries(ctx, model.CaseInquiryListInput{CaseID: caseID, PageSize: msg.GetPageSize(), After: after})
	if err != nil {
		return nil, err
	}
	out := &adminpb.ListCaseInquiriesResponse{Inquiries: make([]*adminpb.CaseInquiry, len(page.Items))}
	for i, item := range page.Items {
		out.Inquiries[i] = toCaseInquiry(item)
	}
	if page.Next != nil {
		out.NextPageToken = rpc.TokenFor(scope, page.Next)
	}
	return connect.NewResponse(out), nil
}

func (h *Governance) RecordCaseInquiryReply(ctx context.Context, req *connect.Request[adminpb.RecordCaseInquiryReplyRequest]) (*connect.Response[adminpb.RecordCaseInquiryReplyResponse], error) {
	adminID, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	id, err := rpc.ParseUUID("id", msg.GetId())
	if err != nil {
		return nil, err
	}
	var received time.Time
	if t := optionalTime(msg.GetReceiveTime()); t != nil {
		received = *t
	}
	result, err := h.m.RecordCaseInquiryReply(ctx, model.RecordCaseInquiryReplyInput{
		InquiryID: id, AdminID: adminID, Channel: rpc.EnumFromProto[model.InquiryReplyChannel](msg.GetChannel()), ReceivedAt: received,
		Body: msg.GetBody(), EvidenceTokens: msg.GetEvidenceTokens(),
	})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, recordInquiryReplyPaths)
	}
	return connect.NewResponse(&adminpb.RecordCaseInquiryReplyResponse{
		Inquiry: toCaseInquiry(result.Inquiry), Case: toCase(result.Case), AlreadyRecorded: result.AlreadyRecorded,
	}), nil
}

func (h *Governance) WithdrawCaseInquiry(ctx context.Context, req *connect.Request[adminpb.WithdrawCaseInquiryRequest]) (*connect.Response[adminpb.WithdrawCaseInquiryResponse], error) {
	adminID, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	id, err := rpc.ParseUUID("id", req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	result, err := h.m.WithdrawCaseInquiry(ctx, model.WithdrawCaseInquiryInput{InquiryID: id, AdminID: adminID})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&adminpb.WithdrawCaseInquiryResponse{
		Inquiry: toCaseInquiry(result.Inquiry), Case: toCase(result.Case), AlreadyWithdrawn: result.AlreadyWithdrawn,
	}), nil
}
