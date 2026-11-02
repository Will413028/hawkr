package seller

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
)

// maxReturnURLLength 是開通後回到的網址的長度上限。
const maxReturnURLLength = 2048

// checkReturnURL 檢查開通 session 的回跳網址：絕對的 https 網址，或本機開發的 http://localhost。金流商會把賣家導向這個網址，不接受其他 scheme、帶帳密的網址。
func checkReturnURL(path, raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return rpc.RequiredField(path, path+" 必填")
	}
	u, err := url.Parse(raw)
	invalid := err != nil || len(raw) > maxReturnURLLength || u.Host == "" || u.User != nil || u.Fragment != ""
	if !invalid {
		local := u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1"
		invalid = u.Scheme != "https" && (u.Scheme != "http" || !local)
	}
	if invalid {
		return rpc.RequiredField(path, path+" 必須是 https 網址（本機開發可用 http://localhost）")
	}
	return nil
}

func toPaymentRequirements(raw json.RawMessage) *structpb.Struct {
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

// GetMyPaymentAccount 讀呼叫者店家的收款帳戶。
func (h *App) GetMyPaymentAccount(ctx context.Context, _ *connect.Request[pb.GetMyPaymentAccountRequest]) (*connect.Response[pb.GetMyPaymentAccountResponse], error) {
	current, err := storeFrom(ctx)
	if err != nil {
		return nil, err
	}
	acct, err := h.m.PaymentAccountOf(ctx, current.StoreID)
	if err != nil {
		return nil, err
	}
	out := &pb.PaymentAccount{
		Status:                rpc.EnumToProto[pb.PaymentAccountStatus](acct.Status),
		RequirementCollection: rpc.EnumToProto[pb.RequirementCollection](acct.RequirementCollection),
		TransfersActive:       acct.TransfersActive, PayoutsActive: acct.PayoutsActive,
		Requirements: toPaymentRequirements(acct.Requirements), FetchTime: optTimestamp(acct.FetchedAt),
	}
	if caps, ok := h.m.PaymentCapabilities(); ok {
		out.OnboardingSurface = rpc.EnumToProto[pb.OnboardingSurface](caps.OnboardingSurface)
	}
	return connect.NewResponse(&pb.GetMyPaymentAccountResponse{Account: out}), nil
}

// CreatePaymentOnboardingSession 為呼叫者店家的收款帳戶產生開通 session。
func (h *App) CreatePaymentOnboardingSession(ctx context.Context, req *connect.Request[pb.CreatePaymentOnboardingSessionRequest]) (*connect.Response[pb.CreatePaymentOnboardingSessionResponse], error) {
	current, err := storeFrom(ctx)
	if err != nil {
		return nil, err
	}
	if err := checkReturnURL("return_url", req.Msg.GetReturnUrl()); err != nil {
		return nil, err
	}
	if err := checkReturnURL("refresh_url", req.Msg.GetRefreshUrl()); err != nil {
		return nil, err
	}
	session, err := h.m.CreatePaymentOnboardingSession(ctx, current.StoreID, strings.TrimSpace(req.Msg.GetReturnUrl()), strings.TrimSpace(req.Msg.GetRefreshUrl()))
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.CreatePaymentOnboardingSessionResponse{
		Surface: rpc.EnumToProto[pb.OnboardingSurface](session.Surface), Url: session.URL, ClientSecret: session.ClientSecret,
		ExpireTime: optTimestamp(session.ExpiresAt),
	}), nil
}

func toAdminPaymentAccount(a model.PaymentAccount) *adminpb.PaymentAccount {
	out := &adminpb.PaymentAccount{
		StoreId: rpc.FormatID(a.StoreID), Provider: a.Provider, ExternalAccountId: deref(a.ExternalAccountID),
		Status:                rpc.EnumToProto[pb.PaymentAccountStatus](a.Status),
		RequirementCollection: rpc.EnumToProto[pb.RequirementCollection](a.RequirementCollection),
		TransfersActive:       a.TransfersActive, PayoutsActive: a.PayoutsActive, Requirements: toPaymentRequirements(a.Requirements),
		StatusReason: deref(a.StatusReason), FetchTime: optTimestamp(a.FetchedAt), CreateFailedTime: optTimestamp(a.CreateFailedAt),
		CreateTime: timestamppb.New(a.CreatedAt), UpdateTime: timestamppb.New(a.UpdatedAt),
	}
	return out
}

// GetPaymentAccount 讀店家的收款帳戶。
func (h *Admin) GetPaymentAccount(ctx context.Context, req *connect.Request[adminpb.GetPaymentAccountRequest]) (*connect.Response[adminpb.GetPaymentAccountResponse], error) {
	storeID, err := rpc.ParseID("store_id", req.Msg.GetStoreId())
	if err != nil {
		return nil, err
	}
	acct, err := h.m.PaymentAccountOf(ctx, storeID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&adminpb.GetPaymentAccountResponse{Account: toAdminPaymentAccount(acct)}), nil
}

// RefreshPaymentAccount 排入一次重讀外部帳戶的背景工作。
func (h *Admin) RefreshPaymentAccount(ctx context.Context, req *connect.Request[adminpb.RefreshPaymentAccountRequest]) (*connect.Response[adminpb.RefreshPaymentAccountResponse], error) {
	storeID, err := rpc.ParseID("store_id", req.Msg.GetStoreId())
	if err != nil {
		return nil, err
	}
	if err := h.m.RefreshPaymentAccount(ctx, storeID); err != nil {
		return nil, err
	}
	return connect.NewResponse(&adminpb.RefreshPaymentAccountResponse{}), nil
}

// RetryPaymentAccountCreate 對建立失敗的收款帳戶重新排入建立：沒有 etag 也不要理由，狀態檢查在模組，重送時第二次回 failed_precondition。
func (h *Admin) RetryPaymentAccountCreate(ctx context.Context, req *connect.Request[adminpb.RetryPaymentAccountCreateRequest]) (*connect.Response[adminpb.RetryPaymentAccountCreateResponse], error) {
	storeID, err := rpc.ParseID("store_id", req.Msg.GetStoreId())
	if err != nil {
		return nil, err
	}
	acct, err := h.m.RetryPaymentAccountCreate(ctx, storeID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&adminpb.RetryPaymentAccountCreateResponse{Account: toAdminPaymentAccount(acct)}), nil
}
