package payment

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/payment/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/payment/v1alpha1/paymentv1alpha1connect"
	paymentpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/payment/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/payment"
)

// AdminModule 是後台 handler 需要的模組方法。
type AdminModule interface {
	Account(ctx context.Context, storeID int64) (payment.Account, error)
	RefreshAccount(ctx context.Context, storeID int64) (payment.Account, error)
	RetryAccountCreate(ctx context.Context, storeID int64) (payment.Account, error)
	RetryAccountPurge(ctx context.Context, storeID int64) (payment.Account, error)
	DeleteAccountMapping(ctx context.Context, storeID, adminID int64) error
}

// errNoPrincipal 是請求通過了 rpc 的授權卻沒有管理員呼叫者：授權鏈壞了，沒有分類（internal），不是使用者的錯。
var errNoPrincipal = errors.New("httpapi/payment：ctx 裡沒有管理員")

// adminFrom 取出呼叫的管理員 ID（interceptor 已經依 service 名稱擋下非管理員）。
func adminFrom(ctx context.Context) (int64, error) {
	p, ok := rpc.PrincipalFrom(ctx)
	if !ok || p.Kind != identity.PrincipalAdmin {
		return 0, errNoPrincipal
	}
	return p.ID, nil
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
		PurgeBlockedTime: optTimestamp(a.PurgeBlockedAt), PurgeBlockedReason: deref(a.PurgeBlockedReason),
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
	acct, err := h.m.RetryAccountCreate(ctx, storeID)
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
	acct, err := h.m.RetryAccountPurge(ctx, storeID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&adminpb.RetryPaymentAccountPurgeResponse{Account: toAdminAccount(acct)}), nil
}

// DeletePaymentAccountMapping 是「已在金流商處理，刪除對應」：操作的管理員從 ctx 取，紀錄與刪列在模組的同一個 transaction；不呼叫金流商。
func (h *Admin) DeletePaymentAccountMapping(ctx context.Context, req *connect.Request[adminpb.DeletePaymentAccountMappingRequest]) (*connect.Response[adminpb.DeletePaymentAccountMappingResponse], error) {
	storeID, err := rpc.ParseID("store_id", req.Msg.GetStoreId())
	if err != nil {
		return nil, err
	}
	adminID, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.m.DeleteAccountMapping(ctx, storeID, adminID); err != nil {
		return nil, err
	}
	return connect.NewResponse(&adminpb.DeletePaymentAccountMappingResponse{}), nil
}
