package payment

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"connectrpc.com/connect"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/payment/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/payment/v1alpha1/paymentv1alpha1connect"
	paymentpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/payment/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

type paymentAdminModule struct {
	account payment.Account
	// refreshedAccount 是 RefreshAccount 回的重讀後帳戶，和 account（Get 回的舊快取）分開，測試才分得出回應用的是哪一個。
	refreshedAccount payment.Account
	refreshed        int64
	retried          int64
	retriedAck       bool
	purgeRetried     int64
	mappingDeleted   int64
	// actors 是三個寫入方法收到的操作者（依呼叫順序）。
	actors []payment.AdminActor
	err    error
}

func (m *paymentAdminModule) RetryAccountPurge(_ context.Context, actor payment.AdminActor, id int64) (payment.Account, error) {
	m.purgeRetried = id
	m.actors = append(m.actors, actor)
	return m.account, m.err
}

func (m *paymentAdminModule) DeleteAccountMapping(_ context.Context, actor payment.AdminActor, id int64) error {
	m.mappingDeleted = id
	m.actors = append(m.actors, actor)
	return m.err
}

func (m *paymentAdminModule) Account(context.Context, int64) (payment.Account, error) {
	return m.account, m.err
}

func (m *paymentAdminModule) RefreshAccount(_ context.Context, id int64) (payment.Account, error) {
	m.refreshed = id
	return m.refreshedAccount, m.err
}

func (m *paymentAdminModule) RetryAccountCreate(_ context.Context, actor payment.AdminActor, id int64, ack bool) (payment.Account, error) {
	m.retried, m.retriedAck = id, ack
	m.actors = append(m.actors, actor)
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
	m.refreshedAccount = m.account
	m.refreshedAccount.Status = payment.StatusEnabled
	h := NewAdmin(m)
	adminCtx := rpc.WithPrincipal(context.Background(), identity.Principal{Kind: identity.PrincipalAdmin, ID: 77})
	got, err := h.GetPaymentAccount(context.Background(), connect.NewRequest(&adminpb.GetPaymentAccountRequest{StoreId: "11"}))
	if err != nil {
		t.Fatal(err)
	}
	a := got.Msg.GetAccount()
	if a.GetStoreId() != "11" || a.GetProvider() != "stripe" || a.GetExternalAccountId() != "acct_1" || a.GetStatus() != paymentpb.PaymentAccountStatus_PAYMENT_ACCOUNT_STATUS_DISABLED ||
		a.GetStatusReason() != reason || a.GetCreateFailedTime() == nil || a.GetFetchTime() != nil {
		t.Fatalf("account=%v", a)
	}
	refreshed, err := h.RefreshPaymentAccount(context.Background(), connect.NewRequest(&adminpb.RefreshPaymentAccountRequest{StoreId: "11"}))
	if err != nil || m.refreshed != 11 {
		t.Fatalf("refresh: %v %d", err, m.refreshed)
	}
	if ra := refreshed.Msg.GetAccount(); ra.GetStoreId() != "11" || ra.GetStatus() != paymentpb.PaymentAccountStatus_PAYMENT_ACCOUNT_STATUS_ENABLED {
		t.Fatalf("refresh 的回應沒有帶重讀後的帳戶：%v", ra)
	}
	if _, err := h.RetryPaymentAccountCreate(adminCtx, connect.NewRequest(&adminpb.RetryPaymentAccountCreateRequest{StoreId: "11"})); err != nil || m.retried != 11 || m.retriedAck {
		t.Fatalf("retry: %v %d ack=%v", err, m.retried, m.retriedAck)
	}
	if _, err := h.RetryPaymentAccountCreate(adminCtx, connect.NewRequest(&adminpb.RetryPaymentAccountCreateRequest{StoreId: "11", AcknowledgeNoProviderAccount: true})); err != nil || !m.retriedAck {
		t.Fatalf("retry 帶確認：%v ack=%v", err, m.retriedAck)
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
			_, err := h.RetryPaymentAccountCreate(adminCtx, connect.NewRequest(&adminpb.RetryPaymentAccountCreateRequest{StoreId: "0"}))
			return err
		},
	} {
		if apperr.KindOf(call()) != apperr.KindInvalidInput {
			t.Errorf("%s：壞的 store_id 不是 invalid argument", name)
		}
	}
}

func TestAdminPurgeHandlersPassTheStoreAndTheCallingAdmin(t *testing.T) {
	blocked := time.Date(2026, 11, 14, 1, 0, 0, 0, time.UTC)
	reason := "provider_refused：no"
	m := &paymentAdminModule{account: payment.Account{StoreID: 11, Provider: "stripe", PurgeBlockedAt: &blocked, PurgeBlockedReason: &reason, CreatedAt: blocked, UpdatedAt: blocked}}
	h := NewAdmin(m)
	adminCtx := rpc.WithPrincipal(context.Background(), identity.Principal{Kind: identity.PrincipalAdmin, ID: 77})
	got, err := h.RetryPaymentAccountPurge(adminCtx, connect.NewRequest(&adminpb.RetryPaymentAccountPurgeRequest{StoreId: "11"}))
	if err != nil || m.purgeRetried != 11 || got.Msg.GetAccount().GetPurgeBlockedReason() != reason || got.Msg.GetAccount().GetPurgeBlockedTime() == nil {
		t.Fatalf("retry purge: %v %d %v", err, m.purgeRetried, got)
	}
	ctx := adminCtx
	if _, err := h.DeletePaymentAccountMapping(ctx, connect.NewRequest(&adminpb.DeletePaymentAccountMappingRequest{StoreId: "11"})); err != nil || m.mappingDeleted != 11 || m.actors[len(m.actors)-1].AdminID != 77 {
		t.Fatalf("delete mapping: %v %v %v", err, m.mappingDeleted, m.actors)
	}
	if _, err := h.DeletePaymentAccountMapping(context.Background(), connect.NewRequest(&adminpb.DeletePaymentAccountMappingRequest{StoreId: "11"})); !errors.Is(err, errNoPrincipal) {
		t.Fatalf("沒有管理員 principal 時預期 errNoPrincipal：%v", err)
	}
	if _, err := h.DeletePaymentAccountMapping(ctx, connect.NewRequest(&adminpb.DeletePaymentAccountMappingRequest{StoreId: "0"})); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("壞的 store_id 不是 invalid argument：%v", err)
	}
}

// 三個寫入方法把管理員 ID 與「這個 RPC 的 procedure 名稱」交給模組當後台紀錄的動作（D62）；procedure 名稱來自 Connect 的 req.Spec()，所以走真的 Connect handler。
// mutation：handler 把 Action 寫死或留空 → 動作不是 procedure 名稱，失敗；少傳管理員 ID → AdminID 不是 77，失敗。
func TestWriteHandlersPassTheAdminAndTheProcedureNameAsTheAuditAction(t *testing.T) {
	m := &paymentAdminModule{account: payment.Account{StoreID: 11, Provider: "stripe", CreatedAt: time.Now(), UpdatedAt: time.Now()}}
	path, handler := paymentv1alpha1connect.NewPaymentAdminServiceHandler(NewAdmin(m))
	mux := http.NewServeMux()
	mux.Handle(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r.WithContext(rpc.WithPrincipal(r.Context(), identity.Principal{Kind: identity.PrincipalAdmin, ID: 77})))
	}))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	c := paymentv1alpha1connect.NewPaymentAdminServiceClient(srv.Client(), srv.URL)
	ctx := context.Background()
	if _, err := c.RetryPaymentAccountCreate(ctx, connect.NewRequest(&adminpb.RetryPaymentAccountCreateRequest{StoreId: "11"})); err != nil {
		t.Fatal(err)
	}
	if _, err := c.RetryPaymentAccountPurge(ctx, connect.NewRequest(&adminpb.RetryPaymentAccountPurgeRequest{StoreId: "11"})); err != nil {
		t.Fatal(err)
	}
	if _, err := c.DeletePaymentAccountMapping(ctx, connect.NewRequest(&adminpb.DeletePaymentAccountMappingRequest{StoreId: "11"})); err != nil {
		t.Fatal(err)
	}
	want := []payment.AdminActor{
		{AdminID: 77, Action: paymentv1alpha1connect.PaymentAdminServiceRetryPaymentAccountCreateProcedure},
		{AdminID: 77, Action: paymentv1alpha1connect.PaymentAdminServiceRetryPaymentAccountPurgeProcedure},
		{AdminID: 77, Action: paymentv1alpha1connect.PaymentAdminServiceDeletePaymentAccountMappingProcedure},
	}
	if !slices.Equal(m.actors, want) {
		t.Fatalf("actors=%v，預期 %v", m.actors, want)
	}
}
