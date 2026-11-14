package seller

import (
	"context"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	paymentpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/payment/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/payment"
)

// returnPage 轉成 rpc 的回跳頁；UNSPECIFIED 與不認得的值回 invalid_argument。
func returnPage(d pb.OnboardingReturnDestination) (rpc.ReturnPage, error) {
	switch d {
	case pb.OnboardingReturnDestination_ONBOARDING_RETURN_DESTINATION_WEBSITE_CERTIFICATION:
		return rpc.ReturnPageCertification, nil
	case pb.OnboardingReturnDestination_ONBOARDING_RETURN_DESTINATION_SELLER_APP:
		return rpc.ReturnPageSellerApp, nil
	case pb.OnboardingReturnDestination_ONBOARDING_RETURN_DESTINATION_UNSPECIFIED:
	}
	return 0, rpc.RequiredField("destination", "destination 必須是有效的回跳目的地")
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
		Status:        rpc.EnumToProto[paymentpb.PaymentAccountStatus](acct.Status),
		PayoutsActive: acct.PayoutsActive, FetchTime: optTimestamp(acct.FetchedAt),
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
	page, err := returnPage(req.Msg.GetDestination())
	if err != nil {
		return nil, err
	}
	locale, ok := rpc.ValidWebLocale(req.Msg.GetLocale())
	if !ok {
		return nil, rpc.RequiredField("locale", "locale 必須是 zh-TW 或 en")
	}
	returnURL, refreshURL, err := h.web.OnboardingReturnURLs(page, locale)
	if err != nil {
		return nil, err
	}
	session, err := h.payments.CreateOnboardingSession(ctx, current.StoreID, returnURL, refreshURL)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.CreatePaymentOnboardingSessionResponse{
		Surface: rpc.EnumToProto[paymentpb.OnboardingSurface](session.Surface), Url: session.URL, ClientSecret: session.ClientSecret,
		Provider: session.Provider, PublishableKey: session.PublishableKey,
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
