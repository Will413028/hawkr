package seller

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

type paymentModule struct {
	Module
	account    model.PaymentAccount
	caps       model.PaymentCapabilities
	configured bool
	sessionFor int64
	returnURL  string
	refreshURL string
	err        error
}

func (m *paymentModule) PaymentAccountOf(_ context.Context, storeID int64) (model.PaymentAccount, error) {
	return m.account, m.err
}

func (m *paymentModule) PaymentCapabilities() (model.PaymentCapabilities, bool) {
	return m.caps, m.configured
}

func (m *paymentModule) CreatePaymentOnboardingSession(_ context.Context, storeID int64, returnURL, refreshURL string) (model.OnboardingSession, error) {
	m.sessionFor, m.returnURL, m.refreshURL = storeID, returnURL, refreshURL
	exp := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	return model.OnboardingSession{Surface: model.SurfaceHosted, URL: "https://onboarding.example.test/x", ExpiresAt: &exp}, m.err
}

type paymentAdminModule struct {
	AdminModule
	account   model.PaymentAccount
	refreshed int64
	retried   int64
	err       error
}

func (m *paymentAdminModule) PaymentAccountOf(context.Context, int64) (model.PaymentAccount, error) {
	return m.account, m.err
}

func (m *paymentAdminModule) RefreshPaymentAccount(_ context.Context, id int64) error {
	m.refreshed = id
	return m.err
}

func (m *paymentAdminModule) RetryPaymentAccountCreate(_ context.Context, id int64) (model.PaymentAccount, error) {
	m.retried = id
	return m.account, m.err
}

func TestPaymentEnumsMatchTheModule(t *testing.T) {
	for name, got := range map[string][]string{
		"PaymentAccountStatus":  rpc.EnumMismatches[pb.PaymentAccountStatus](model.PaymentAccountStatusValues()),
		"RequirementCollection": rpc.EnumMismatches[pb.RequirementCollection]([]model.RequirementCollection{model.CollectionProvider, model.CollectionPlatform}),
		"OnboardingSurface":     rpc.EnumMismatches[pb.OnboardingSurface]([]model.OnboardingSurface{model.SurfaceHosted, model.SurfaceEmbedded}),
	} {
		if len(got) != 0 {
			t.Errorf("%s：%v", name, got)
		}
	}
}

func TestCheckReturnURLAcceptsOnlyAbsoluteHTTPSOrLocalHTTP(t *testing.T) {
	for raw, ok := range map[string]bool{
		"https://web.example.test/seller/payment?x=1": true,
		"http://localhost:3000/seller/payment":        true,
		"http://127.0.0.1:3000/x":                     true,
		"":                                            false,
		"   ":                                         false,
		"http://web.example.test/x":                   false,
		"javascript:alert(1)":                         false,
		"hawkr://seller/payment":                      false,
		"/seller/payment":                             false,
		"https://user:pw@web.example.test/x":          false,
		"https://web.example.test/x#frag":             false,
		"https://" + strings.Repeat("a", 2100) + ".test/": false,
	} {
		err := checkReturnURL("return_url", raw)
		if (err == nil) != ok {
			t.Errorf("%.40q：err=%v，want ok=%v", raw, err, ok)
		}
		if err != nil && apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%.40q：err=%v 不是 invalid argument", raw, err)
		}
	}
}

func TestGetMyPaymentAccountReturnsTheNeutralStatusAndTheProviderSurface(t *testing.T) {
	fetched := time.Date(2026, 11, 2, 3, 0, 0, 0, time.UTC)
	m := &paymentModule{
		configured: true, caps: model.PaymentCapabilities{RequirementCollection: model.CollectionPlatform, OnboardingSurface: model.SurfaceEmbedded},
		account: model.PaymentAccount{
			StoreID: 11, Status: model.PaymentActionRequired, RequirementCollection: model.CollectionPlatform, TransfersActive: false, PayoutsActive: false,
			Requirements: json.RawMessage(`{"currently_due":["individual.id_number"]}`), FetchedAt: &fetched,
		},
	}
	app := &App{m: m}
	got, err := app.GetMyPaymentAccount(ctxWithStore(currentStore()), connect.NewRequest(&pb.GetMyPaymentAccountRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	a := got.Msg.GetAccount()
	if a.GetStatus() != pb.PaymentAccountStatus_PAYMENT_ACCOUNT_STATUS_ACTION_REQUIRED || a.GetRequirementCollection() != pb.RequirementCollection_REQUIREMENT_COLLECTION_PLATFORM ||
		a.GetOnboardingSurface() != pb.OnboardingSurface_ONBOARDING_SURFACE_EMBEDDED || a.GetTransfersActive() || !a.GetFetchTime().AsTime().Equal(fetched) {
		t.Fatalf("account=%v", a)
	}
	if due := a.GetRequirements().AsMap()["currently_due"]; due == nil {
		t.Fatalf("requirements=%v", a.GetRequirements())
	}
	m.configured = false
	got, err = app.GetMyPaymentAccount(ctxWithStore(currentStore()), connect.NewRequest(&pb.GetMyPaymentAccountRequest{}))
	if err != nil || got.Msg.GetAccount().GetOnboardingSurface() != pb.OnboardingSurface_ONBOARDING_SURFACE_UNSPECIFIED {
		t.Fatalf("沒有設定收款開通：%v %v", got, err)
	}
}

func TestCreatePaymentOnboardingSessionValidatesTheUrlsBeforeCallingTheModule(t *testing.T) {
	m := &paymentModule{}
	app := &App{m: m}
	for name, req := range map[string]*pb.CreatePaymentOnboardingSessionRequest{
		"沒有 return_url":  {RefreshUrl: "https://web.example.test/again"},
		"沒有 refresh_url": {ReturnUrl: "https://web.example.test/ok"},
		"不是 https":       {ReturnUrl: "http://web.example.test/ok", RefreshUrl: "https://web.example.test/again"},
	} {
		if _, err := app.CreatePaymentOnboardingSession(ctxWithStore(currentStore()), connect.NewRequest(req)); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s：err=%v", name, err)
		}
	}
	if m.sessionFor != 0 {
		t.Fatal("網址不合仍然呼叫了模組")
	}
	got, err := app.CreatePaymentOnboardingSession(ctxWithStore(currentStore()), connect.NewRequest(&pb.CreatePaymentOnboardingSessionRequest{
		ReturnUrl: " https://web.example.test/ok ", RefreshUrl: "https://web.example.test/again",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if m.sessionFor != 11 || m.returnURL != "https://web.example.test/ok" || m.refreshURL != "https://web.example.test/again" {
		t.Fatalf("模組收到 store=%d return=%q refresh=%q", m.sessionFor, m.returnURL, m.refreshURL)
	}
	if got.Msg.GetSurface() != pb.OnboardingSurface_ONBOARDING_SURFACE_HOSTED || got.Msg.GetUrl() == "" || got.Msg.GetClientSecret() != "" || got.Msg.GetExpireTime() == nil {
		t.Fatalf("response=%v", got.Msg)
	}
}

func TestAdminPaymentAccountHandlersMapTheAccountAndPassTheStore(t *testing.T) {
	ext := "acct_1"
	reason := "rejected.fraud"
	failed := time.Date(2026, 11, 2, 1, 0, 0, 0, time.UTC)
	m := &paymentAdminModule{account: model.PaymentAccount{
		StoreID: 11, Provider: "stripe", ExternalAccountID: &ext, Status: model.PaymentDisabled, RequirementCollection: model.CollectionProvider,
		StatusReason: &reason, CreateFailedAt: &failed, CreatedAt: failed, UpdatedAt: failed,
	}}
	h := NewAdmin(m, rpc.Images{})
	got, err := h.GetPaymentAccount(adminCtx(1), connect.NewRequest(&adminpb.GetPaymentAccountRequest{StoreId: "11"}))
	if err != nil {
		t.Fatal(err)
	}
	a := got.Msg.GetAccount()
	if a.GetStoreId() != "11" || a.GetProvider() != "stripe" || a.GetExternalAccountId() != "acct_1" || a.GetStatus() != pb.PaymentAccountStatus_PAYMENT_ACCOUNT_STATUS_DISABLED ||
		a.GetStatusReason() != reason || a.GetCreateFailedTime() == nil || a.GetFetchTime() != nil {
		t.Fatalf("account=%v", a)
	}
	if _, err := h.RefreshPaymentAccount(adminCtx(1), connect.NewRequest(&adminpb.RefreshPaymentAccountRequest{StoreId: "11"})); err != nil || m.refreshed != 11 {
		t.Fatalf("refresh: %v %d", err, m.refreshed)
	}
	if _, err := h.RetryPaymentAccountCreate(adminCtx(1), connect.NewRequest(&adminpb.RetryPaymentAccountCreateRequest{StoreId: "11"})); err != nil || m.retried != 11 {
		t.Fatalf("retry: %v %d", err, m.retried)
	}
	for name, call := range map[string]func() error{
		"Get": func() error {
			_, err := h.GetPaymentAccount(adminCtx(1), connect.NewRequest(&adminpb.GetPaymentAccountRequest{StoreId: "x"}))
			return err
		},
		"Refresh": func() error {
			_, err := h.RefreshPaymentAccount(adminCtx(1), connect.NewRequest(&adminpb.RefreshPaymentAccountRequest{}))
			return err
		},
		"Retry": func() error {
			_, err := h.RetryPaymentAccountCreate(adminCtx(1), connect.NewRequest(&adminpb.RetryPaymentAccountCreateRequest{StoreId: "0"}))
			return err
		},
	} {
		if apperr.KindOf(call()) != apperr.KindInvalidInput {
			t.Errorf("%s：壞的 store_id 不是 invalid argument", name)
		}
	}
}
