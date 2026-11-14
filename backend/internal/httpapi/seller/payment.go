package seller

import (
	"context"
	"net/url"
	"strings"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	paymentpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/payment/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	httppayment "github.com/Will413028/hawkr/backend/internal/httpapi/payment"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/payment"
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

// PaymentReader 是收款帳戶的讀取來源（賣家端與後台的開通任務共用）。
type PaymentReader interface {
	Account(ctx context.Context, storeID int64) (payment.Account, error)
	Capabilities() (payment.Capabilities, bool)
}

// Payments 是賣家收款帳戶的來源（payment 模組，Stripe 計畫 D30）：handler 先由能力檢查取得呼叫者的店家（擁有者），再以店家呼叫。
// SyncAccountUnlessFresh 是 RefreshMyPaymentAccount 的同步重讀（transaction 外的外部呼叫，可以與同步 job 並行，D32；上次讀取在 10 秒內不呼叫金流商，D33）。
type Payments interface {
	PaymentReader
	CreateOnboardingSession(ctx context.Context, storeID int64, returnURL, refreshURL string) (payment.OnboardingSession, error)
	SyncAccountUnlessFresh(ctx context.Context, storeID int64) error
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
	acct, err := h.payments.Account(ctx, current.StoreID)
	if err != nil {
		return nil, err
	}
	out := &pb.PaymentAccount{
		Status:                rpc.EnumToProto[paymentpb.PaymentAccountStatus](acct.Status),
		RequirementCollection: rpc.EnumToProto[paymentpb.RequirementCollection](acct.RequirementCollection),
		TransfersActive:       acct.TransfersActive, PayoutsActive: acct.PayoutsActive,
		Requirements: httppayment.Requirements(acct.Requirements), FetchTime: optTimestamp(acct.FetchedAt),
	}
	if caps, ok := h.payments.Capabilities(); ok {
		out.OnboardingSurface = rpc.EnumToProto[paymentpb.OnboardingSurface](caps.OnboardingSurface)
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
	session, err := h.payments.CreateOnboardingSession(ctx, current.StoreID, strings.TrimSpace(req.Msg.GetReturnUrl()), strings.TrimSpace(req.Msg.GetRefreshUrl()))
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.CreatePaymentOnboardingSessionResponse{
		Surface: rpc.EnumToProto[paymentpb.OnboardingSurface](session.Surface), Url: session.URL, ClientSecret: session.ClientSecret,
		ExpireTime: optTimestamp(session.ExpiresAt),
	}), nil
}

// RefreshMyPaymentAccount 同步向金流商重讀呼叫者店家的收款帳戶並更新快取（D33）；重讀失敗是這個明確動作的失敗，原樣回給 client
// （暫時性是 unavailable，client 可以重試；API 設計規範第八節）。間隔限制在 payment 的公開 API，這裡不重複。
func (h *App) RefreshMyPaymentAccount(ctx context.Context, _ *connect.Request[pb.RefreshMyPaymentAccountRequest]) (*connect.Response[pb.RefreshMyPaymentAccountResponse], error) {
	current, err := storeFrom(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.payments.SyncAccountUnlessFresh(ctx, current.StoreID); err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.RefreshMyPaymentAccountResponse{}), nil
}
