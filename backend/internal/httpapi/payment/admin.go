package payment

import (
	"context"
	"encoding/json"
	"strconv"
	"time"
	"unicode/utf8"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"uuid"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/payment/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/payment/v1alpha1/paymentv1alpha1connect"
	paymentpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/payment/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
)

// AdminModule 是後台 handler 需要的模組方法。
type AdminModule interface {
	Account(ctx context.Context, storeID int64) (payment.Account, error)
	RefreshAccount(ctx context.Context, storeID int64) (payment.Account, error)
	RetryAccountCreate(ctx context.Context, actor payment.AdminActor, storeID int64, acknowledgeNoProviderAccount bool) (payment.Account, error)
	RetryAccountPurge(ctx context.Context, actor payment.AdminActor, storeID int64) (payment.Account, error)
	DeleteAccountMapping(ctx context.Context, actor payment.AdminActor, storeID int64) error
	ListPayments(ctx context.Context, f payment.PaymentFilter, after *payment.RecordCursor, size int32) (payment.PaymentRecordPage, error)
	ListPaymentTransfers(ctx context.Context, f payment.TransferFilter, after *payment.RecordCursor, size int32) (payment.TransferRecordPage, error)
	ListPaymentRefunds(ctx context.Context, f payment.RefundFilter, after *payment.RecordCursor, size int32) (payment.RefundRecordPage, error)
	ResolveAnomaly(ctx context.Context, actor payment.AdminActor, kind payment.AnomalyKind, id uuid.UUID, outcome payment.AnomalyOutcome, reason payment.AnomalyResolutionReason, note, evidence string) error
}

// actorFrom 是後台紀錄的操作者：呼叫的管理員 ID 與這個 RPC 的 procedure 名稱（D62）。
func actorFrom(ctx context.Context, procedure string) (payment.AdminActor, error) {
	id, err := rpc.RequireAdmin(ctx)
	if err != nil {
		return payment.AdminActor{}, err
	}
	return payment.AdminActor{AdminID: id, Action: procedure}, nil
}

// Admin 是 PaymentAdminService 的 handler。授權（只有管理員、各方法的權限）由 rpc 的 interceptor 與 adminauth 檢查。
type Admin struct{ m AdminModule }

var _ paymentv1alpha1connect.PaymentAdminServiceHandler = (*Admin)(nil)

// NewAdmin 建立後台的 handler。
func NewAdmin(m AdminModule) *Admin { return &Admin{m: m} }

// requirements 把金流商回報的待補條件原始 JSON 轉成 Struct；不是物件時回空物件。
func requirements(raw json.RawMessage) *structpb.Struct {
	out := &structpb.Struct{}
	if len(raw) == 0 || out.UnmarshalJSON(raw) != nil {
		return &structpb.Struct{Fields: map[string]*structpb.Value{}}
	}
	return out
}

func optTimestamp(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func toAdminAccount(a payment.Account) *adminpb.PaymentAccount {
	return &adminpb.PaymentAccount{
		StoreId: rpc.FormatID(a.StoreID), Provider: a.Provider, ExternalAccountId: deref(a.ExternalAccountID),
		Status:                rpc.EnumToProto[paymentpb.PaymentAccountStatus](a.Status),
		RequirementCollection: rpc.EnumToProto[paymentpb.RequirementCollection](a.RequirementCollection),
		TransfersActive:       a.TransfersActive, PayoutsActive: a.PayoutsActive, Requirements: requirements(a.Requirements),
		StatusReason: deref(a.StatusReason), FetchTime: optTimestamp(a.FetchedAt), CreateFailedTime: optTimestamp(a.CreateFailedAt),
		PurgeBlockedTime: optTimestamp(a.PurgeBlockedAt), PurgeBlockedReason: deref(a.PurgeBlockedReason), CreateUnresolved: a.CreateUnresolved, PurgeReplayPending: a.PurgeReplayPending, PurgePendingTime: optTimestamp(a.PurgePendingAt),
		CreateTime: timestamppb.New(a.CreatedAt), UpdateTime: timestamppb.New(a.UpdatedAt),
	}
}

// GetPaymentAccount 讀店家的收款帳戶。
func (h *Admin) GetPaymentAccount(ctx context.Context, req *connect.Request[adminpb.GetPaymentAccountRequest]) (*connect.Response[adminpb.GetPaymentAccountResponse], error) {
	storeID, err := rpc.ParseID("store_id", req.Msg.GetStoreId())
	if err != nil {
		return nil, err
	}
	acct, err := h.m.Account(ctx, storeID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&adminpb.GetPaymentAccountResponse{Account: toAdminAccount(acct)}), nil
}

// RefreshPaymentAccount 同步向金流商重讀外部帳戶並回最新帳戶；間隔與名額在 payment 的公開 API（與賣家的重讀同一條路徑），這裡不重複。
func (h *Admin) RefreshPaymentAccount(ctx context.Context, req *connect.Request[adminpb.RefreshPaymentAccountRequest]) (*connect.Response[adminpb.RefreshPaymentAccountResponse], error) {
	storeID, err := rpc.ParseID("store_id", req.Msg.GetStoreId())
	if err != nil {
		return nil, err
	}
	acct, err := h.m.RefreshAccount(ctx, storeID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&adminpb.RefreshPaymentAccountResponse{Account: toAdminAccount(acct)}), nil
}

// RetryPaymentAccountCreate 對建立失敗的收款帳戶重新排入建立：沒有 etag 也不要理由，狀態檢查在模組，重送時第二次回 failed_precondition。
func (h *Admin) RetryPaymentAccountCreate(ctx context.Context, req *connect.Request[adminpb.RetryPaymentAccountCreateRequest]) (*connect.Response[adminpb.RetryPaymentAccountCreateResponse], error) {
	storeID, err := rpc.ParseID("store_id", req.Msg.GetStoreId())
	if err != nil {
		return nil, err
	}
	actor, err := actorFrom(ctx, req.Spec().Procedure)
	if err != nil {
		return nil, err
	}
	acct, err := h.m.RetryAccountCreate(ctx, actor, storeID, req.Msg.GetAcknowledgeNoProviderAccount())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&adminpb.RetryPaymentAccountCreateResponse{Account: toAdminAccount(acct)}), nil
}

// RetryPaymentAccountPurge 對清理進了終局的收款帳戶重試關閉：沒有 etag 也不要理由，狀態檢查在模組，重送時第二次回 failed_precondition。
func (h *Admin) RetryPaymentAccountPurge(ctx context.Context, req *connect.Request[adminpb.RetryPaymentAccountPurgeRequest]) (*connect.Response[adminpb.RetryPaymentAccountPurgeResponse], error) {
	storeID, err := rpc.ParseID("store_id", req.Msg.GetStoreId())
	if err != nil {
		return nil, err
	}
	actor, err := actorFrom(ctx, req.Spec().Procedure)
	if err != nil {
		return nil, err
	}
	acct, err := h.m.RetryAccountPurge(ctx, actor, storeID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&adminpb.RetryPaymentAccountPurgeResponse{Account: toAdminAccount(acct)}), nil
}

// DeletePaymentAccountMapping 是「已在金流商處理，刪除對應」：操作的管理員從 ctx 取、動作是 procedure 名稱，後台紀錄與刪列在模組的同一個 transaction；不呼叫金流商。
func (h *Admin) DeletePaymentAccountMapping(ctx context.Context, req *connect.Request[adminpb.DeletePaymentAccountMappingRequest]) (*connect.Response[adminpb.DeletePaymentAccountMappingResponse], error) {
	storeID, err := rpc.ParseID("store_id", req.Msg.GetStoreId())
	if err != nil {
		return nil, err
	}
	actor, err := actorFrom(ctx, req.Spec().Procedure)
	if err != nil {
		return nil, err
	}
	if err := h.m.DeleteAccountMapping(ctx, actor, storeID); err != nil {
		return nil, err
	}
	return connect.NewResponse(&adminpb.DeletePaymentAccountMappingResponse{}), nil
}

// listPaths 是清單請求的欄位路徑：模組回的欄位錯誤（InvalidInput 的欄位）對到請求。
var listPaths = rpc.FieldPaths{"Status": "status", "OrderNo": "order_no", "PageSize": "page_size"}

func toAdminPayment(r payment.PaymentRecord) *adminpb.Payment {
	orders := make([]*adminpb.PaymentOrder, len(r.Orders))
	for i, o := range r.Orders {
		orders[i] = &adminpb.PaymentOrder{Id: o.ID.String(), OrderNo: o.OrderNo, StoreId: rpc.FormatID(o.StoreID), Amount: o.Amount}
	}
	return &adminpb.Payment{
		Id: r.ID.String(), Orders: orders, BuyerId: rpc.FormatID(r.BuyerID), Amount: r.Amount, Provider: r.Provider,
		Status:            rpc.EnumToProto[paymentpb.PaymentStatus](r.Status),
		ExternalPaymentId: r.ExternalPaymentID, PaidTime: optTimestamp(r.PaidAt), LastError: r.LastError, FailureReason: r.FailureReason,
		CreateTime: timestamppb.New(r.CreatedAt), UpdateTime: timestamppb.New(r.UpdatedAt),
	}
}

func toAdminTransfer(r payment.TransferRecord) *adminpb.PaymentTransfer {
	return &adminpb.PaymentTransfer{
		Id: r.ID.String(), PaymentId: r.PaymentID.String(), PaymentOrderId: r.PaymentOrderID.String(), OrderNo: r.OrderNo, StoreId: rpc.FormatID(r.StoreID), Amount: r.Amount, ReversedAmount: r.ReversedAmount,
		Status:             rpc.EnumToProto[paymentpb.PaymentTransferStatus](r.Status),
		ExternalTransferId: r.ExternalTransferID, FailureReason: r.FailureReason, CreateTime: timestamppb.New(r.CreatedAt),
	}
}

func toAdminRefund(r payment.RefundRecord) *adminpb.PaymentRefund {
	out := &adminpb.PaymentRefund{
		Id: r.ID.String(), PaymentId: r.PaymentID.String(), PaymentOrderId: r.PaymentOrderID.String(), OrderNo: r.OrderNo, Amount: r.Amount, Auto: r.Auto,
		Status:           rpc.EnumToProto[paymentpb.PaymentRefundStatus](r.Status),
		ReversalAmount:   r.ReversalAmount,
		ReversalStatus:   rpc.EnumToProto[paymentpb.PaymentReversalStatus](r.ReversalStatus),
		TopupStatus:      rpc.EnumToProto[paymentpb.PaymentTopupStatus](r.TopupStatus),
		ExternalRefundId: r.ExternalRefundID, FailureReason: r.FailureReason, RequestKey: r.RequestKey, CreateTime: timestamppb.New(r.CreatedAt),
		Reversals: make([]*adminpb.PaymentReversal, len(r.Reversals)),
	}
	for i, v := range r.Reversals {
		out.Reversals[i] = &adminpb.PaymentReversal{
			Id: v.ID.String(), Amount: v.Amount, Status: rpc.EnumToProto[paymentpb.PaymentReversalStatus](v.Status),
			Reason: v.Reason, ExternalId: v.ExternalID, CreateTime: timestamppb.New(v.CreatedAt),
		}
	}
	return out
}

// pageCursor 解出 page token 的排序鍵；scope 綁定這支 RPC 與請求的篩選，換了篩選的 token 回 invalid_argument。
func pageCursor(scope, token string) (*payment.RecordCursor, error) {
	return rpc.ValidCursor[payment.RecordCursor](scope, token)
}

// ListPayments 列付款，新的在前。
func (h *Admin) ListPayments(ctx context.Context, req *connect.Request[adminpb.ListPaymentsRequest]) (*connect.Response[adminpb.ListPaymentsResponse], error) {
	f := payment.PaymentFilter{OrderNo: req.Msg.GetOrderNo()}
	if req.Msg.Status != nil {
		f.Status = rpc.EnumFromProto[payment.PaymentStatus](req.Msg.GetStatus())
		if f.Status == "" {
			return nil, rpc.Error(connect.CodeInvalidArgument, "%s", "status 不合")
		}
	}
	scope := rpc.FilterScope(paymentv1alpha1connect.PaymentAdminServiceListPaymentsProcedure, string(f.Status), f.OrderNo)
	after, err := pageCursor(scope, req.Msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	page, err := h.m.ListPayments(ctx, f, after, req.Msg.GetPageSize())
	if err != nil {
		return nil, rpc.WithFieldPaths(err, listPaths)
	}
	out := &adminpb.ListPaymentsResponse{Payments: make([]*adminpb.Payment, len(page.Records))}
	for i, r := range page.Records {
		out.Payments[i] = toAdminPayment(r)
	}
	if page.Next != nil {
		out.NextPageToken = rpc.TokenFor(scope, page.Next)
	}
	return connect.NewResponse(out), nil
}

// ListPaymentTransfers 列轉帳，新的在前。
func (h *Admin) ListPaymentTransfers(ctx context.Context, req *connect.Request[adminpb.ListPaymentTransfersRequest]) (*connect.Response[adminpb.ListPaymentTransfersResponse], error) {
	f := payment.TransferFilter{OrderNo: req.Msg.GetOrderNo()}
	if req.Msg.Status != nil {
		f.Status = rpc.EnumFromProto[payment.PaymentTransferStatus](req.Msg.GetStatus())
		if f.Status == "" {
			return nil, rpc.Error(connect.CodeInvalidArgument, "%s", "status 不合")
		}
	}
	scope := rpc.FilterScope(paymentv1alpha1connect.PaymentAdminServiceListPaymentTransfersProcedure, string(f.Status), f.OrderNo)
	after, err := pageCursor(scope, req.Msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	page, err := h.m.ListPaymentTransfers(ctx, f, after, req.Msg.GetPageSize())
	if err != nil {
		return nil, rpc.WithFieldPaths(err, listPaths)
	}
	out := &adminpb.ListPaymentTransfersResponse{Transfers: make([]*adminpb.PaymentTransfer, len(page.Records))}
	for i, r := range page.Records {
		out.Transfers[i] = toAdminTransfer(r)
	}
	if page.Next != nil {
		out.NextPageToken = rpc.TokenFor(scope, page.Next)
	}
	return connect.NewResponse(out), nil
}

// ListPaymentRefunds 列退款，新的在前，每筆附撤回列。
func (h *Admin) ListPaymentRefunds(ctx context.Context, req *connect.Request[adminpb.ListPaymentRefundsRequest]) (*connect.Response[adminpb.ListPaymentRefundsResponse], error) {
	f := payment.RefundFilter{OrderNo: req.Msg.GetOrderNo(), AnomalyOnly: req.Msg.GetAnomalyOnly()}
	if req.Msg.Status != nil {
		f.Status = rpc.EnumFromProto[payment.PaymentRefundStatus](req.Msg.GetStatus())
		if f.Status == "" {
			return nil, rpc.Error(connect.CodeInvalidArgument, "%s", "status 不合")
		}
	}
	scope := rpc.FilterScope(paymentv1alpha1connect.PaymentAdminServiceListPaymentRefundsProcedure, string(f.Status), f.OrderNo, strconv.FormatBool(f.AnomalyOnly))
	after, err := pageCursor(scope, req.Msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	page, err := h.m.ListPaymentRefunds(ctx, f, after, req.Msg.GetPageSize())
	if err != nil {
		return nil, rpc.WithFieldPaths(err, listPaths)
	}
	out := &adminpb.ListPaymentRefundsResponse{Refunds: make([]*adminpb.PaymentRefund, len(page.Records))}
	for i, r := range page.Records {
		out.Refunds[i] = toAdminRefund(r)
	}
	if page.Next != nil {
		out.NextPageToken = rpc.TokenFor(scope, page.Next)
	}
	return connect.NewResponse(out), nil
}

// anomalyKinds 是契約的物件種類到模組的值（模組的值是小寫，不能以去前綴的通則轉換）。
var anomalyKinds = map[paymentpb.PaymentAnomalyKind]payment.AnomalyKind{
	paymentpb.PaymentAnomalyKind_PAYMENT_ANOMALY_KIND_PAYMENT:  payment.AnomalyPayment,
	paymentpb.PaymentAnomalyKind_PAYMENT_ANOMALY_KIND_TRANSFER: payment.AnomalyTransfer,
	paymentpb.PaymentAnomalyKind_PAYMENT_ANOMALY_KIND_REFUND:   payment.AnomalyRefund,
	paymentpb.PaymentAnomalyKind_PAYMENT_ANOMALY_KIND_REVERSAL: payment.AnomalyReversal,
	paymentpb.PaymentAnomalyKind_PAYMENT_ANOMALY_KIND_TOPUP:    payment.AnomalyTopup,
}

var resolvePaths = rpc.FieldPaths{"Evidence": "evidence", "Reason": "reason", "Note": "note"}

// ResolvePaymentAnomaly 是 ANOMALY 的人工結案：操作的管理員取自呼叫者、動作是 procedure 名稱，後台紀錄與領域變更在模組的同一個 transaction。
func (h *Admin) ResolvePaymentAnomaly(ctx context.Context, req *connect.Request[adminpb.ResolvePaymentAnomalyRequest]) (*connect.Response[adminpb.ResolvePaymentAnomalyResponse], error) {
	kind, ok := anomalyKinds[req.Msg.GetKind()]
	if !ok {
		return nil, rpc.Error(connect.CodeInvalidArgument, "%s", "kind 不合")
	}
	id, err := rpc.ParseUUID("id", req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	outcome := rpc.EnumFromProto[payment.AnomalyOutcome](req.Msg.GetOutcome())
	if outcome == "" {
		return nil, rpc.Error(connect.CodeInvalidArgument, "%s", "outcome 不合")
	}
	reason := rpc.EnumFromProto[payment.AnomalyResolutionReason](req.Msg.GetReason())
	if reason == "" {
		return nil, rpc.RequiredField("reason", "reason 必填")
	}
	note := trimtext.Trim(req.Msg.GetNote())
	if note == "" {
		return nil, rpc.RequiredField("note", "note 必填")
	}
	if utf8.RuneCountInString(note) > payment.AnomalyResolutionNoteMaxRunes {
		return nil, rpc.WithFieldPaths(apperr.InvalidInputOn([]string{"note"}, "note 不得超過 %d 個字元", payment.AnomalyResolutionNoteMaxRunes), rpc.FieldPaths{"note": "note"})
	}
	actor, err := actorFrom(ctx, req.Spec().Procedure)
	if err != nil {
		return nil, err
	}
	if err := h.m.ResolveAnomaly(ctx, actor, kind, id, outcome, reason, note, req.Msg.GetEvidence()); err != nil {
		return nil, rpc.WithFieldPaths(err, resolvePaths)
	}
	return connect.NewResponse(&adminpb.ResolvePaymentAnomalyResponse{}), nil
}
