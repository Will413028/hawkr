package payment

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"

	paymentpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/payment/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

type fakeAppModule struct {
	payment payment.Payment
	err     error
	buyers  []int64
	orders  []string
}

func (m *fakeAppModule) CreatePayment(_ context.Context, buyerID int64, orderNos []string) (payment.Payment, error) {
	m.buyers, m.orders = append(m.buyers, buyerID), append(m.orders, orderNos...)
	return m.payment, m.err
}

func (m *fakeAppModule) GetPayment(_ context.Context, buyerID int64, orderNo string) (payment.Payment, error) {
	m.buyers, m.orders = append(m.buyers, buyerID), append(m.orders, orderNo)
	return m.payment, m.err
}

type fakeBuyers struct{ byAccount map[int64]int64 }

func (b fakeBuyers) CurrentBuyerID(_ context.Context, accountID int64) (int64, error) {
	id, ok := b.byAccount[accountID]
	if !ok {
		return 0, errors.New("沒有買家")
	}
	return id, nil
}

func accountCtx(id int64) context.Context {
	return rpc.WithPrincipal(context.Background(), identity.Principal{Kind: identity.PrincipalAccount, ID: id})
}

func awaiting(provider, secret string) payment.Payment {
	return payment.Payment{Orders: []payment.PaymentOrderReceivable{{OrderNo: "O1", Amount: 120}}, Amount: 120, Provider: provider, Status: payment.PaymentAwaitingPayment, ClientSecret: secret}
}

func TestPaymentAppUsesTheCallersBuyerIDNotAnythingFromTheRequest(t *testing.T) {
	m := &fakeAppModule{payment: awaiting("stripe", "pi_1_secret_x")}
	h := NewApp(m, fakeBuyers{map[int64]int64{5: 50}}, "pk_test_1")
	if _, err := h.CreatePayment(accountCtx(5), connect.NewRequest(&paymentpb.CreatePaymentRequest{OrderNo: "O1"})); err != nil {
		t.Fatal(err)
	}
	if _, err := h.GetPayment(accountCtx(5), connect.NewRequest(&paymentpb.GetPaymentRequest{OrderNo: "O1"})); err != nil {
		t.Fatal(err)
	}
	if len(m.buyers) != 2 || m.buyers[0] != 50 || m.buyers[1] != 50 || m.orders[0] != "O1" {
		t.Fatalf("模組收到的買家 %v 訂單 %v：應該是呼叫者的買家 ID 50", m.buyers, m.orders)
	}
}

func TestPaymentAppRejectsACallerThatIsNotACustomerAccount(t *testing.T) {
	m := &fakeAppModule{payment: awaiting("stripe", "s")}
	h := NewApp(m, fakeBuyers{map[int64]int64{5: 50}}, "pk")
	adminCtx := rpc.WithPrincipal(context.Background(), identity.Principal{Kind: identity.PrincipalAdmin, ID: 9})
	_, err := h.CreatePayment(adminCtx, connect.NewRequest(&paymentpb.CreatePaymentRequest{OrderNo: "O1"}))
	if codeOf(err) != connect.CodePermissionDenied || len(m.orders) != 0 {
		t.Fatalf("err=%v 模組呼叫=%v", err, m.orders)
	}
}

func TestPaymentAppReturnsTheStripeSessionOnlyWhenTheModuleReturnedASecret(t *testing.T) {
	paid := time.Date(2026, 11, 20, 3, 0, 0, 0, time.UTC)
	cases := []struct {
		name       string
		p          payment.Payment
		wantStatus paymentpb.PaymentStatus
		wantKey    string
	}{
		{"待付款帶 session", awaiting("stripe", "pi_1_secret_x"), paymentpb.PaymentStatus_PAYMENT_STATUS_AWAITING_PAYMENT, "pk_test_1"},
		{"待付款但 provider 已往前，沒有 secret", payment.Payment{Orders: []payment.PaymentOrderReceivable{{OrderNo: "O1", Amount: 120}}, Amount: 120, Provider: "stripe", Status: payment.PaymentAwaitingPayment}, paymentpb.PaymentStatus_PAYMENT_STATUS_AWAITING_PAYMENT, ""},
		{"已入帳", payment.Payment{Orders: []payment.PaymentOrderReceivable{{OrderNo: "O1", Amount: 120}}, Amount: 120, Provider: "stripe", Status: payment.PaymentSucceeded, PaidAt: &paid}, paymentpb.PaymentStatus_PAYMENT_STATUS_SUCCEEDED, ""},
		{"建立中", payment.Payment{Orders: []payment.PaymentOrderReceivable{{OrderNo: "O1", Amount: 120}}, Amount: 120, Provider: "stripe", Status: payment.PaymentCreating}, paymentpb.PaymentStatus_PAYMENT_STATUS_CREATING, ""},
		{"建立失敗", payment.Payment{Orders: []payment.PaymentOrderReceivable{{OrderNo: "O1", Amount: 120}}, Amount: 120, Provider: "stripe", Status: payment.PaymentCreateFailed}, paymentpb.PaymentStatus_PAYMENT_STATUS_CREATE_FAILED, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := NewApp(&fakeAppModule{payment: c.p}, fakeBuyers{map[int64]int64{5: 50}}, "pk_test_1")
			got, err := h.GetPayment(accountCtx(5), connect.NewRequest(&paymentpb.GetPaymentRequest{OrderNo: "O1"}))
			if err != nil {
				t.Fatal(err)
			}
			p := got.Msg.GetPayment()
			if p.GetStatus() != c.wantStatus || p.GetAmount() != 120 || p.GetOrderNo() != "O1" {
				t.Fatalf("payment=%v", p)
			}
			if (p.GetPaidTime() != nil) != (c.p.PaidAt != nil) {
				t.Fatalf("paid_time=%v", p.GetPaidTime())
			}
			s := p.GetSession().GetStripe()
			if s.GetPublishableKey() != c.wantKey || (c.wantKey != "") != (s.GetClientSecret() != "") {
				t.Fatalf("session=%v", p.GetSession())
			}
		})
	}
}

func TestPaymentAppFailsInsteadOfReturningASessionTheClientCannotUse(t *testing.T) {
	buyers := fakeBuyers{map[int64]int64{5: 50}}
	_, err := NewApp(&fakeAppModule{payment: awaiting("stripe", "s")}, buyers, "").GetPayment(accountCtx(5), connect.NewRequest(&paymentpb.GetPaymentRequest{OrderNo: "O1"}))
	if !errors.Is(err, errNoPublishableKey) {
		t.Fatalf("沒有公開金鑰時 err=%v", err)
	}
	_, err = NewApp(&fakeAppModule{payment: awaiting("fake", "s")}, buyers, "pk").GetPayment(accountCtx(5), connect.NewRequest(&paymentpb.GetPaymentRequest{OrderNo: "O1"}))
	if !errors.Is(err, errUnknownProvider) {
		t.Fatalf("不認得的 provider err=%v", err)
	}
}

func TestPaymentAppPassesModuleErrorsThrough(t *testing.T) {
	h := NewApp(&fakeAppModule{err: apperr.NotFound("找不到這張訂單")}, fakeBuyers{map[int64]int64{5: 50}}, "pk")
	if _, err := h.CreatePayment(accountCtx(5), connect.NewRequest(&paymentpb.CreatePaymentRequest{OrderNo: "O9"})); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("err=%v", err)
	}
}

func TestPaymentEnumsMatchTheModuleValues(t *testing.T) {
	check := func(name string, got []string) {
		if len(got) > 0 {
			t.Errorf("%s：%v", name, got)
		}
	}
	check("PaymentStatus", rpc.EnumMismatches[paymentpb.PaymentStatus](payment.PaymentStatusValues()))
	check("PaymentRefundStatus", rpc.EnumMismatches[paymentpb.PaymentRefundStatus](payment.PaymentRefundStatusValues()))
	check("PaymentTransferStatus", rpc.EnumMismatches[paymentpb.PaymentTransferStatus](payment.PaymentTransferStatusValues()))
	check("PaymentReversalStatus", rpc.EnumMismatches[paymentpb.PaymentReversalStatus](payment.PaymentReversalStatusValues()))
	check("PaymentTopupStatus", rpc.EnumMismatches[paymentpb.PaymentTopupStatus](payment.PaymentTopupStatusValues()))
	check("PaymentAnomalyOutcome", rpc.EnumMismatches[paymentpb.PaymentAnomalyOutcome]([]payment.AnomalyOutcome{payment.AnomalySucceeded, payment.AnomalyFailed}))
	seen := map[payment.AnomalyKind]bool{}
	vs := paymentpb.PaymentAnomalyKind(0).Descriptor().Values()
	for i := range vs.Len() {
		n := vs.Get(i).Number()
		if n == 0 {
			continue
		}
		k, ok := anomalyKinds[paymentpb.PaymentAnomalyKind(n)]
		if !ok || seen[k] {
			t.Errorf("PaymentAnomalyKind %v 沒有對應或重複", vs.Get(i).Name())
		}
		seen[k] = true
	}
}

func codeOf(err error) connect.Code {
	return connect.CodeOf(rpc.ConnectError(context.Background(), err))
}
