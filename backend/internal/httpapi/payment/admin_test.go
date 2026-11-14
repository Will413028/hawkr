package payment

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/payment/v1alpha1"
	paymentpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/payment/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

type paymentAdminModule struct {
	account   payment.Account
	refreshed int64
	retried   int64
	err       error
}

func (m *paymentAdminModule) Account(context.Context, int64) (payment.Account, error) {
	return m.account, m.err
}

func (m *paymentAdminModule) RefreshAccount(_ context.Context, id int64) error {
	m.refreshed = id
	return m.err
}

func (m *paymentAdminModule) RetryAccountCreate(_ context.Context, id int64) (payment.Account, error) {
	m.retried = id
	return m.account, m.err
}

func TestAdminPaymentAccountHandlersMapTheAccountAndPassTheStore(t *testing.T) {
	ext := "acct_1"
	reason := "rejected.fraud"
	failed := time.Date(2026, 11, 2, 1, 0, 0, 0, time.UTC)
	m := &paymentAdminModule{account: payment.Account{
		StoreID: 11, Provider: "stripe", ExternalAccountID: &ext, Status: payment.StatusDisabled, RequirementCollection: payment.CollectionProvider,
		StatusReason: &reason, CreateFailedAt: &failed, CreatedAt: failed, UpdatedAt: failed,
	}}
	h := NewAdmin(m)
	got, err := h.GetPaymentAccount(context.Background(), connect.NewRequest(&adminpb.GetPaymentAccountRequest{StoreId: "11"}))
	if err != nil {
		t.Fatal(err)
	}
	a := got.Msg.GetAccount()
	if a.GetStoreId() != "11" || a.GetProvider() != "stripe" || a.GetExternalAccountId() != "acct_1" || a.GetStatus() != paymentpb.PaymentAccountStatus_PAYMENT_ACCOUNT_STATUS_DISABLED ||
		a.GetStatusReason() != reason || a.GetCreateFailedTime() == nil || a.GetFetchTime() != nil {
		t.Fatalf("account=%v", a)
	}
	if _, err := h.RefreshPaymentAccount(context.Background(), connect.NewRequest(&adminpb.RefreshPaymentAccountRequest{StoreId: "11"})); err != nil || m.refreshed != 11 {
		t.Fatalf("refresh: %v %d", err, m.refreshed)
	}
	if _, err := h.RetryPaymentAccountCreate(context.Background(), connect.NewRequest(&adminpb.RetryPaymentAccountCreateRequest{StoreId: "11"})); err != nil || m.retried != 11 {
		t.Fatalf("retry: %v %d", err, m.retried)
	}
	for name, call := range map[string]func() error{
		"Get": func() error {
			_, err := h.GetPaymentAccount(context.Background(), connect.NewRequest(&adminpb.GetPaymentAccountRequest{StoreId: "x"}))
			return err
		},
		"Refresh": func() error {
			_, err := h.RefreshPaymentAccount(context.Background(), connect.NewRequest(&adminpb.RefreshPaymentAccountRequest{}))
			return err
		},
		"Retry": func() error {
			_, err := h.RetryPaymentAccountCreate(context.Background(), connect.NewRequest(&adminpb.RetryPaymentAccountCreateRequest{StoreId: "0"}))
			return err
		},
	} {
		if apperr.KindOf(call()) != apperr.KindInvalidInput {
			t.Errorf("%s：壞的 store_id 不是 invalid argument", name)
		}
	}
}
