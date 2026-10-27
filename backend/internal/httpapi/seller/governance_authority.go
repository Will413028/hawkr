package seller

import (
	"context"
	"strconv"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1/sellerv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
)

// 主管機關來函（step10e-reports-brief R5、設計 step10e-3cb-design §1.4、§4）。

// 模組輸入的欄位對應到請求的路徑。
var (
	createAuthorityOrderPaths = rpc.FieldPaths{
		"Agency": "agency", "DocumentNo": "document_no", "ReceivedAt": "receive_time", "Kind": "kind", "Summary": "summary", "Targets": "targets",
		"DueAt": "due_time", "RequestID": "request_id", "Targets[].AccountID": "targets[].account_id", "Targets[].StoreID": "targets[].store_id",
		"Targets[].ProductIDs": "targets[].product_ids",
	}
	completeAuthorityOrderPaths = rpc.FieldPaths{
		"ResolutionSummary": "resolution_summary", "ProvideTime": "provide_time", "ExpectedVersion": "etag", "TargetNotices": "target_notices",
		"TargetNotices[]": "target_notices[]", "TargetNotices[].Notice.Kind": "target_notices[].notice.kind",
		"TargetNotices[].Notice.Note": "target_notices[].notice.note", "TargetNotices[].Notice.NotifiedAt": "target_notices[].notice.notify_time",
		"TargetNotices[].Notice.RecheckOn": "target_notices[].notice.recheck_date",
	}
	extendAuthorityHoldPaths  = rpc.FieldPaths{"HoldUntil": "hold_until", "Reason": "reason", "ExpectedVersion": "etag"}
	revokeAuthorityOrderPaths = rpc.FieldPaths{"Reason": "reason", "ExpectedVersion": "etag"}
	recordTargetNoticePaths   = rpc.FieldPaths{
		"TargetPosition": "target_position", "ExpectedVersion": "etag", "Notice.Kind": "notice.kind", "Notice.Note": "notice.note",
		"Notice.NotifiedAt": "notice.notify_time", "Notice.RecheckOn": "notice.recheck_date",
	}
)

func toAuthorityNotice(n *model.AuthorityNotice) *adminpb.AuthorityTargetNotice {
	if n == nil {
		return nil
	}
	out := &adminpb.AuthorityTargetNotice{
		Kind: rpc.EnumToProto[adminpb.AuthorityTargetNoticeKind](n.Kind), NotifyTime: optionalTimestamp(n.NotifiedAt), Note: n.Note,
		RecordTime: timestamppb.New(n.RecordedAt), RecordedByAdminId: rpc.FormatID(n.RecordedByAdminID),
	}
	if n.RecheckOn != nil {
		out.RecheckDate = protoDate(*n.RecheckOn)
	}
	return out
}

// authorityUnavailabilities 把不能做的原因轉成契約；句子與指令被擋下時的錯誤是同一個函式（AuthorityOrderBlock.Text）。
func authorityUnavailabilities(blocks []model.AuthorityOrderBlock) []*adminpb.AuthorityOrderUnavailability {
	var out []*adminpb.AuthorityOrderUnavailability
	for _, b := range blocks {
		out = append(out, &adminpb.AuthorityOrderUnavailability{
			Reason: rpc.EnumToProto[adminpb.AuthorityOrderUnavailableReason](b), Message: rpc.LocalizedMessage(b.Text()),
		})
	}
	return out
}

// toAuthorityOrder 把後台看到的來函轉成契約：期限只在處理中時輸出（義務還在時才輸出）；can_* 與 *_unavailabilities 取自同一個 Availability。
func toAuthorityOrder(o model.AdminAuthorityOrder) *adminpb.AuthorityOrder {
	out := &adminpb.AuthorityOrder{
		Id: o.ID.String(), Agency: o.Agency, DocumentNo: o.DocumentNo, ReceiveTime: timestamppb.New(o.ReceivedAt),
		Kind: rpc.EnumToProto[adminpb.AuthorityOrderKind](o.Kind), Summary: o.Summary, State: rpc.EnumToProto[adminpb.AuthorityOrderState](o.State()),
		HoldUntil: timestamppb.New(o.HoldUntil), HoldActive: o.HoldActive, RegisteredByAdminId: rpc.FormatID(o.RegisteredByAdminID),
		CreateTime: timestamppb.New(o.RegisteredAt), Etag: rpc.Etag(o.Version),
		CanComplete: len(o.Availability.Complete) == 0, CanExtendHold: len(o.Availability.ExtendHold) == 0,
		CanRevoke: len(o.Availability.Revoke) == 0, RevokeUnavailabilities: authorityUnavailabilities(o.Availability.Revoke),
	}
	out.Deadline = toDeadline(o.Deadline)
	if c := o.Completed; c != nil {
		out.CompleteTime, out.CompletedByAdminId, out.ResolutionSummary = timestamppb.New(c.At), rpc.FormatID(c.AdminID), c.Summary
	}
	if r := o.Revoked; r != nil {
		out.RevokeTime, out.RevokedByAdminId, out.RevokeReason = timestamppb.New(r.At), rpc.FormatID(r.AdminID), r.Reason
	}
	for i, t := range o.Targets {
		var noticeBlocks []model.AuthorityOrderBlock
		if i < len(o.Availability.RecordNotice) {
			noticeBlocks = o.Availability.RecordNotice[i]
		}
		pt := &adminpb.AuthorityOrderTarget{
			AccountId: rpc.FormatID(t.AccountID), ProductIds: t.ProductIDs, NoSellerData: t.NoSellerData, Notice: toAuthorityNotice(t.Notice),
			CanRecordNotice: len(noticeBlocks) == 0, RecordNoticeUnavailabilities: authorityUnavailabilities(noticeBlocks),
		}
		if t.StoreID != nil {
			pt.StoreId = rpc.FormatID(*t.StoreID)
		}
		// 指名了單一店家時 store_name 是那一家的名字；不指定店家時不輸出（逐家的名字在 covered_stores）。
		for _, s := range t.Stores {
			name := derefString(s.StoreName)
			pt.CoveredStores = append(pt.CoveredStores, &adminpb.AuthorityTargetStore{
				StoreId: rpc.FormatID(s.StoreID), StoreName: name, Coverage: rpc.EnumToProto[adminpb.AuthorityStoreCoverage](s.Coverage),
				CanCreateMeasure: s.CanCreateMeasure, CanCreateTermination: s.CanCreateTermination,
			})
			if t.StoreID != nil {
				pt.StoreName = name
			}
		}
		out.Targets = append(out.Targets, pt)
	}
	for _, e := range o.HoldExtensions {
		out.HoldExtensions = append(out.HoldExtensions, &adminpb.AuthorityHoldExtension{
			FromTime: timestamppb.New(e.From), ToTime: timestamppb.New(e.To), Cause: rpc.EnumToProto[adminpb.AuthorityHoldExtensionCause](e.Cause),
			Reason: e.Reason, AdminId: rpc.FormatID(e.AdminID), CreateTime: timestamppb.New(e.CreatedAt),
		})
	}
	return out
}

// noticeInputFrom 轉換請求裡的告知；告知的日期不是有效日期時 RecheckOn 是零值（模組回「請填寫確認日期」）。
func noticeInputFrom(n *adminpb.AuthorityTargetNotice) model.AuthorityNoticeInput {
	return model.AuthorityNoticeInput{
		Kind: rpc.EnumFromProto[model.AuthorityNoticeKind](n.GetKind()), NotifiedAt: optionalTime(n.GetNotifyTime()), Note: n.GetNote(),
		RecheckOn: civilDate(n.GetRecheckDate()),
	}
}

func (h *Governance) CreateAuthorityOrder(ctx context.Context, req *connect.Request[adminpb.CreateAuthorityOrderRequest]) (*connect.Response[adminpb.CreateAuthorityOrderResponse], error) {
	adminID, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	reqID, err := requestID(msg.GetRequestId())
	if err != nil {
		return nil, err
	}
	var received time.Time
	if t := optionalTime(msg.GetReceiveTime()); t != nil {
		received = *t
	}
	in := model.CreateAuthorityOrderInput{
		AdminID: adminID, RequestID: reqID, Agency: msg.GetAgency(), DocumentNo: msg.GetDocumentNo(), ReceivedAt: received,
		Kind: rpc.EnumFromProto[model.AuthorityOrderKind](msg.GetKind()), Summary: msg.GetSummary(), DueAt: optionalTime(msg.GetDueTime()),
	}
	for i, t := range msg.GetTargets() {
		account, err := rpc.ParseID("targets["+strconv.Itoa(i)+"].account_id", t.GetAccountId())
		if err != nil {
			return nil, err
		}
		target := model.AuthorityTargetInput{AccountID: account, ProductIDs: t.GetProductIds()}
		if raw := t.GetStoreId(); raw != "" {
			store, err := rpc.ParseID("targets["+strconv.Itoa(i)+"].store_id", raw)
			if err != nil {
				return nil, err
			}
			target.StoreID = &store
		}
		in.Targets = append(in.Targets, target)
	}
	result, err := h.m.CreateAuthorityOrder(ctx, in)
	if err != nil {
		return nil, rpc.WithFieldPaths(err, createAuthorityOrderPaths)
	}
	return connect.NewResponse(&adminpb.CreateAuthorityOrderResponse{Order: toAuthorityOrder(result.Order)}), nil
}

// ListAuthorityOrders 是來函的分頁讀取。page token 綁定篩選（狀態、帳號），換了篩選就從第一頁開始。
func (h *Governance) ListAuthorityOrders(ctx context.Context, req *connect.Request[adminpb.ListAuthorityOrdersRequest]) (*connect.Response[adminpb.ListAuthorityOrdersResponse], error) {
	msg := req.Msg
	in := model.AuthorityOrderListInput{PageSize: msg.GetPageSize()}
	if state := rpc.EnumFromProto[model.AuthorityOrderState](msg.GetState()); state != "" {
		in.State = &state
	}
	if msg.GetAccountId() != "" {
		id, err := rpc.ParseID("account_id", msg.GetAccountId())
		if err != nil {
			return nil, err
		}
		in.AccountID = id
	}
	scope := rpc.FilterScope(sellerv1alpha1connect.SellerGovernanceAdminServiceListAuthorityOrdersProcedure, msg.GetState().String(), msg.GetAccountId())
	after, err := rpc.ValidCursor[model.AuthorityCursor](scope, msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	in.After = after
	page, err := h.m.AuthorityOrders(ctx, in)
	if err != nil {
		return nil, rpc.WithFieldPaths(err, rpc.FieldPaths{"State": "state", "AccountID": "account_id"})
	}
	out := &adminpb.ListAuthorityOrdersResponse{Orders: make([]*adminpb.AuthorityOrder, len(page.Items))}
	for i, item := range page.Items {
		out.Orders[i] = toAuthorityOrder(item)
	}
	if page.Next != nil {
		out.NextPageToken = rpc.TokenFor(scope, page.Next)
	}
	return connect.NewResponse(out), nil
}

func (h *Governance) GetAuthorityOrder(ctx context.Context, req *connect.Request[adminpb.GetAuthorityOrderRequest]) (*connect.Response[adminpb.GetAuthorityOrderResponse], error) {
	id, err := rpc.ParseUUID("id", req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	order, err := h.m.AuthorityOrder(ctx, id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&adminpb.GetAuthorityOrderResponse{Order: toAuthorityOrder(order)}), nil
}

func (h *Governance) CompleteAuthorityOrder(ctx context.Context, req *connect.Request[adminpb.CompleteAuthorityOrderRequest]) (*connect.Response[adminpb.CompleteAuthorityOrderResponse], error) {
	adminID, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	id, err := rpc.ParseUUID("id", msg.GetId())
	if err != nil {
		return nil, err
	}
	version, err := rpc.RequiredVersionField(msg.GetEtag(), "etag")
	if err != nil {
		return nil, err
	}
	in := model.CompleteAuthorityOrderInput{
		ID: id, AdminID: adminID, ExpectedVersion: version, ResolutionSummary: msg.GetResolutionSummary(), ProvideTime: optionalTime(msg.GetProvideTime()),
	}
	for _, n := range msg.GetTargetNotices() {
		in.TargetNotices = append(in.TargetNotices, model.TargetNoticeInput{Position: int(n.GetTargetPosition()), Notice: noticeInputFrom(n.GetNotice())})
	}
	order, err := h.m.CompleteAuthorityOrder(ctx, in)
	if err != nil {
		return nil, rpc.WithFieldPaths(err, completeAuthorityOrderPaths)
	}
	return connect.NewResponse(&adminpb.CompleteAuthorityOrderResponse{Order: toAuthorityOrder(order)}), nil
}

func (h *Governance) ExtendAuthorityOrderHold(ctx context.Context, req *connect.Request[adminpb.ExtendAuthorityOrderHoldRequest]) (*connect.Response[adminpb.ExtendAuthorityOrderHoldResponse], error) {
	adminID, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	id, err := rpc.ParseUUID("id", msg.GetId())
	if err != nil {
		return nil, err
	}
	version, err := rpc.RequiredVersionField(msg.GetEtag(), "etag")
	if err != nil {
		return nil, err
	}
	until := optionalTime(msg.GetHoldUntil())
	if until == nil {
		return nil, rpc.RequiredField("hold_until", "hold_until 必填")
	}
	order, err := h.m.ExtendAuthorityOrderHold(ctx, model.ExtendAuthorityOrderHoldInput{
		ID: id, AdminID: adminID, ExpectedVersion: version, HoldUntil: *until, Reason: msg.GetReason(),
	})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, extendAuthorityHoldPaths)
	}
	return connect.NewResponse(&adminpb.ExtendAuthorityOrderHoldResponse{Order: toAuthorityOrder(order)}), nil
}

func (h *Governance) RecordAuthorityTargetNotice(ctx context.Context, req *connect.Request[adminpb.RecordAuthorityTargetNoticeRequest]) (*connect.Response[adminpb.RecordAuthorityTargetNoticeResponse], error) {
	adminID, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	id, err := rpc.ParseUUID("id", msg.GetId())
	if err != nil {
		return nil, err
	}
	version, err := rpc.RequiredVersionField(msg.GetEtag(), "etag")
	if err != nil {
		return nil, err
	}
	order, err := h.m.RecordAuthorityTargetNotice(ctx, model.RecordAuthorityTargetNoticeInput{
		ID: id, AdminID: adminID, ExpectedVersion: version, Position: int(msg.GetTargetPosition()), Notice: noticeInputFrom(msg.GetNotice()),
	})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, recordTargetNoticePaths)
	}
	return connect.NewResponse(&adminpb.RecordAuthorityTargetNoticeResponse{Order: toAuthorityOrder(order)}), nil
}

func (h *Governance) RevokeAuthorityOrder(ctx context.Context, req *connect.Request[adminpb.RevokeAuthorityOrderRequest]) (*connect.Response[adminpb.RevokeAuthorityOrderResponse], error) {
	adminID, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	id, err := rpc.ParseUUID("id", msg.GetId())
	if err != nil {
		return nil, err
	}
	version, err := rpc.RequiredVersionField(msg.GetEtag(), "etag")
	if err != nil {
		return nil, err
	}
	result, err := h.m.RevokeAuthorityOrder(ctx, model.RevokeAuthorityOrderInput{ID: id, AdminID: adminID, ExpectedVersion: version, Reason: msg.GetReason()})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, revokeAuthorityOrderPaths)
	}
	return connect.NewResponse(&adminpb.RevokeAuthorityOrderResponse{Order: toAuthorityOrder(result.Order), AlreadyRevoked: result.AlreadyRevoked}), nil
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
