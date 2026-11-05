package seller

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

type paymentModule struct {
	account    payment.Account
	caps       payment.Capabilities
	configured bool
	sessionFor int64
	returnURL  string
	refreshURL string
	err        error
}

func (m *paymentModule) Account(_ context.Context, storeID int64) (payment.Account, error) {
	return m.account, m.err
}

func (m *paymentModule) Capabilities() (payment.Capabilities, bool) {
	return m.caps, m.configured
}

func (m *paymentModule) CreateOnboardingSession(_ context.Context, storeID int64, returnURL, refreshURL string) (payment.OnboardingSession, error) {
	m.sessionFor, m.returnURL, m.refreshURL = storeID, returnURL, refreshURL
	exp := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	return payment.OnboardingSession{Surface: payment.SurfaceHosted, URL: "https://onboarding.example.test/x", ExpiresAt: &exp}, m.err
}

func TestPaymentEnumsMatchTheModule(t *testing.T) {
	for name, got := range map[string][]string{
		"PaymentAccountStatus":  rpc.EnumMismatches[pb.PaymentAccountStatus](payment.AccountStatusValues()),
		"RequirementCollection": rpc.EnumMismatches[pb.RequirementCollection](payment.RequirementCollectionValues()),
		"OnboardingSurface":     rpc.EnumMismatches[pb.OnboardingSurface]([]payment.OnboardingSurface{payment.SurfaceHosted, payment.SurfaceEmbedded}),
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
		configured: true, caps: payment.Capabilities{RequirementCollection: payment.CollectionPlatform, OnboardingSurface: payment.SurfaceEmbedded},
		account: payment.Account{
			StoreID: 11, Status: payment.StatusActionRequired, RequirementCollection: payment.CollectionPlatform, TransfersActive: false, PayoutsActive: false,
			Requirements: json.RawMessage(`{"currently_due":["individual.id_number"]}`), FetchedAt: &fetched,
		},
	}
	app := &App{payments: m}
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
	app := &App{payments: m}
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
