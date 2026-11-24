package seller

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	paymentpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/payment/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1/sellerv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
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
	Configured() bool
}

// Payments 是賣家收款帳戶的來源（payment 模組，收款開通 ADR D30）：handler 先由能力檢查取得呼叫者的店家（擁有者），再以店家呼叫。
// SyncAccountUnlessFresh 是 RefreshMyPaymentAccount 的同步重讀（transaction 外的外部呼叫，可以與同步 job 並行，D32；上次讀取在 10 秒內不呼叫金流商，D33）。
type Payments interface {
	PaymentReader
	CreateOnboardingSession(ctx context.Context, storeID int64, returnURL, refreshURL string, runtimes []payment.EmbeddedRuntime) (payment.OnboardingSession, error)
	SyncAccountUnlessFresh(ctx context.Context, storeID int64) error
	// ListEarnings 與 OrderEarnings 是賣家入帳帳本（D15）：以 handler 從能力檢查取得的店家為範圍，模組只回這家店的列。
	ListEarnings(ctx context.Context, storeID int64, f payment.EarningsFilter, after *payment.RecordCursor, size int32) (payment.EarningsPage, error)
	OrderEarnings(ctx context.Context, storeID int64, orderNo string) (payment.OrderEarnings, error)
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
	returnURL, refreshURL, err := h.web.OnboardingReturnURLs(page, uitext.LocaleFrom(ctx))
	if err != nil {
		return nil, err
	}
	session, err := h.payments.CreateOnboardingSession(ctx, current.StoreID, returnURL, refreshURL, embeddedRuntimes(req.Msg.GetEmbeddedRuntimes()))
	if err != nil {
		return nil, err
	}
	out, err := sessionToProto(session)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(out), nil
}

// embeddedRuntimes 把呼叫端宣告的內嵌執行環境轉成 payment 的型別；UNSPECIFIED 與不認得的值忽略（呼叫端可能比伺服器新）。
func embeddedRuntimes(in []paymentpb.EmbeddedOnboardingRuntime) []payment.EmbeddedRuntime {
	var out []payment.EmbeddedRuntime
	for _, r := range in {
		if r.Descriptor().Values().ByNumber(r.Number()) == nil {
			continue
		}
		if v := rpc.EnumFromProto[payment.EmbeddedRuntime](r); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// sessionToProto 把 payment 的 session 轉成回應的 oneof；不認得的形狀是伺服器的錯（payment 新增形狀時要同步改這裡）。
func sessionToProto(session payment.OnboardingSession) (*pb.CreatePaymentOnboardingSessionResponse, error) {
	switch v := session.(type) {
	case payment.HostedSession:
		return &pb.CreatePaymentOnboardingSessionResponse{Session: &pb.CreatePaymentOnboardingSessionResponse_Hosted{
			Hosted: &pb.HostedOnboarding{Url: v.URL},
		}}, nil
	case payment.StripeEmbeddedSession:
		return &pb.CreatePaymentOnboardingSessionResponse{Session: &pb.CreatePaymentOnboardingSessionResponse_Embedded{
			Embedded: &pb.EmbeddedOnboarding{Provider: &pb.EmbeddedOnboarding_Stripe{
				Stripe: &pb.StripeEmbeddedOnboarding{ClientSecret: v.ClientSecret, PublishableKey: v.PublishableKey},
			}},
		}}, nil
	}
	return nil, fmt.Errorf("seller：不認得的開通 session 形狀 %T", session)
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

// earningsToProto 轉成帳本的一列；種類與狀態由模組的值對應到 proto 的列舉。
func earningsToProto(e payment.EarningsEntry) *paymentpb.EarningsEntry {
	return &paymentpb.EarningsEntry{
		Id:         e.ID.String(),
		Type:       rpc.EnumToProto[paymentpb.EarningsEntryType](e.Type),
		OrderNo:    e.OrderNo,
		Amount:     e.Amount,
		Status:     rpc.EnumToProto[paymentpb.EarningsEntryStatus](e.Status),
		CreateTime: timestamppb.New(e.CreatedAt),
	}
}

func optTime(t *timestamppb.Timestamp) *time.Time {
	if t == nil {
		return nil
	}
	v := t.AsTime()
	return &v
}

// ListMyEarnings 列出呼叫者店家的入帳帳本；店家只從能力檢查取得，請求不帶店家，所以別家店的列不可能被讀到。
func (h *App) ListMyEarnings(ctx context.Context, req *connect.Request[pb.ListMyEarningsRequest]) (*connect.Response[pb.ListMyEarningsResponse], error) {
	current, err := storeFrom(ctx)
	if err != nil {
		return nil, err
	}
	f := payment.EarningsFilter{OrderNo: req.Msg.GetOrderNo(), From: optTime(req.Msg.GetFromTime()), To: optTime(req.Msg.GetToTime())}
	scope := rpc.FilterScope(sellerv1alpha1connect.SellerAppServiceListMyEarningsProcedure,
		strconv.FormatInt(current.StoreID, 10), f.OrderNo, timeKey(f.From), timeKey(f.To))
	after, err := rpc.ValidCursor[payment.RecordCursor](scope, req.Msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	page, err := h.payments.ListEarnings(ctx, current.StoreID, f, after, req.Msg.GetPageSize())
	if err != nil {
		return nil, err
	}
	out := &pb.ListMyEarningsResponse{Entries: make([]*paymentpb.EarningsEntry, len(page.Entries))}
	for i, e := range page.Entries {
		out.Entries[i] = earningsToProto(e)
	}
	if page.Next != nil {
		out.NextPageToken = rpc.TokenFor(scope, page.Next)
	}
	return connect.NewResponse(out), nil
}

func timeKey(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

// GetMyOrderEarnings 回傳呼叫者店家一張訂單的入帳列與已完成列的淨額；別家店的訂單與沒有付款的訂單都回 not_found（由模組判定）。
func (h *App) GetMyOrderEarnings(ctx context.Context, req *connect.Request[pb.GetMyOrderEarningsRequest]) (*connect.Response[pb.GetMyOrderEarningsResponse], error) {
	current, err := storeFrom(ctx)
	if err != nil {
		return nil, err
	}
	got, err := h.payments.OrderEarnings(ctx, current.StoreID, req.Msg.GetOrderNo())
	if err != nil {
		return nil, err
	}
	out := &pb.GetMyOrderEarningsResponse{Entries: make([]*paymentpb.EarningsEntry, len(got.Entries)), NetAmount: got.NetAmount}
	for i, e := range got.Entries {
		out.Entries[i] = earningsToProto(e)
	}
	return connect.NewResponse(out), nil
}
