package payment

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	paymentpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/payment/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/payment/v1alpha1/paymentv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/payment"
)

// AppModule 是買家 handler 需要的模組方法：付款只讀寫呼叫者自己的（模組以買家 ID 比對歸屬，他人的訂單回 NotFound）。
type AppModule interface {
	CreatePayment(ctx context.Context, buyerID int64, orderNo string) (payment.Payment, error)
	GetPayment(ctx context.Context, buyerID int64, orderNo string) (payment.Payment, error)
}

// 設定或資料壞掉、不是使用者的錯（沒有分類，回 internal）。
var (
	errNoPublishableKey = errors.New("httpapi/payment：Stripe 付款沒有設定公開金鑰")
	errUnknownProvider  = errors.New("httpapi/payment：付款的 provider 沒有對應的 client session")
)

// App 是 PaymentAppService 的 handler。授權（只有顧客帳號、停權的帳號不能建立付款）由 rpc 的 interceptor 與 customerEffects 檢查。
type App struct {
	m              AppModule
	buyers         rpc.BuyerLookup
	publishableKey string
}

var _ paymentv1alpha1connect.PaymentAppServiceHandler = (*App)(nil)

// NewApp 建立買家的 handler。publishableKey 是 Stripe 的公開金鑰（不是 secret），隨 AWAITING_PAYMENT 的付款回給 client 初始化 Stripe.js 與 PaymentSheet；
// 沒有設定時 Stripe 付款的 session 回 internal（設定錯誤），其他狀態不受影響。
func NewApp(m AppModule, buyers rpc.BuyerLookup, publishableKey string) *App {
	return &App{m: m, buyers: buyers, publishableKey: publishableKey}
}

// providerStripe 是 Stripe 付款 provider 的代碼（付款列的 provider 欄）。
const providerStripe = "stripe"

func (h *App) toPayment(p payment.Payment) (*paymentpb.Payment, error) {
	out := &paymentpb.Payment{
		OrderNo: p.OrderNo,
		Status:  rpc.EnumToProto[paymentpb.PaymentStatus](p.Status),
		Amount:  p.Amount,
	}
	if p.PaidAt != nil {
		out.PaidTime = timestamppb.New(*p.PaidAt)
	}
	// secret 只在模組判定可以付款（AWAITING_PAYMENT 且 provider 仍在等待）時才有，其餘情況不帶 session。
	if p.ClientSecret == "" {
		return out, nil
	}
	switch p.Provider {
	case providerStripe:
		if h.publishableKey == "" {
			return nil, errNoPublishableKey
		}
		out.Session = &paymentpb.PaymentSession{Provider: &paymentpb.PaymentSession_Stripe{
			Stripe: &paymentpb.StripePaymentSession{ClientSecret: p.ClientSecret, PublishableKey: h.publishableKey},
		}}
	default:
		return nil, errUnknownProvider
	}
	return out, nil
}

// CreatePayment 對呼叫者自己的待付款訂單建立付款；歸屬、可付與金額都由模組向訂單一方確認，這裡只轉換。
func (h *App) CreatePayment(ctx context.Context, req *connect.Request[paymentpb.CreatePaymentRequest]) (*connect.Response[paymentpb.CreatePaymentResponse], error) {
	buyerID, err := rpc.AccountBuyerID(ctx, h.buyers)
	if err != nil {
		return nil, err
	}
	p, err := h.m.CreatePayment(ctx, buyerID, req.Msg.GetOrderNo())
	if err != nil {
		return nil, err
	}
	out, err := h.toPayment(p)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&paymentpb.CreatePaymentResponse{Payment: out}), nil
}

// GetPayment 讀呼叫者自己的付款；他人的付款回 not_found（模組以買家 ID 比對）。
func (h *App) GetPayment(ctx context.Context, req *connect.Request[paymentpb.GetPaymentRequest]) (*connect.Response[paymentpb.GetPaymentResponse], error) {
	buyerID, err := rpc.AccountBuyerID(ctx, h.buyers)
	if err != nil {
		return nil, err
	}
	p, err := h.m.GetPayment(ctx, buyerID, req.Msg.GetOrderNo())
	if err != nil {
		return nil, err
	}
	out, err := h.toPayment(p)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&paymentpb.GetPaymentResponse{Payment: out}), nil
}
