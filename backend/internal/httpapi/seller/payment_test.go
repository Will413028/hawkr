package seller

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"connectrpc.com/connect"

	paymentpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/payment/v1alpha1"
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
	canEmbed   bool
	returnURL  string
	refreshURL string
	err        error
	session    *payment.OnboardingSession
	// syncedFor 記下 SyncAccountUnlessFresh 被呼叫的店家；syncErr 非 nil 時同步失敗。
	syncedFor []int64
	syncErr   error
}

func (m *paymentModule) Account(_ context.Context, storeID int64) (payment.Account, error) {
	return m.account, m.err
}

func (m *paymentModule) Capabilities() (payment.Capabilities, bool) {
	return m.caps, m.configured
}

func (m *paymentModule) SyncAccountUnlessFresh(_ context.Context, storeID int64) error {
	m.syncedFor = append(m.syncedFor, storeID)
	return m.syncErr
}

func (m *paymentModule) CreateOnboardingSession(_ context.Context, storeID int64, returnURL, refreshURL string, canEmbed bool) (payment.OnboardingSession, error) {
	m.sessionFor, m.returnURL, m.refreshURL, m.canEmbed = storeID, returnURL, refreshURL, canEmbed
	if m.session != nil {
		return *m.session, m.err
	}
	exp := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	return payment.OnboardingSession{Surface: payment.SurfaceHosted, URL: "https://onboarding.example.test/x", ExpiresAt: &exp}, m.err
}

func TestPaymentEnumsMatchTheModule(t *testing.T) {
	for name, got := range map[string][]string{
		"PaymentAccountStatus":  rpc.EnumMismatches[paymentpb.PaymentAccountStatus](payment.AccountStatusValues()),
		"RequirementCollection": rpc.EnumMismatches[paymentpb.RequirementCollection](payment.RequirementCollectionValues()),
		"OnboardingSurface":     rpc.EnumMismatches[paymentpb.OnboardingSurface]([]payment.OnboardingSurface{payment.SurfaceHosted, payment.SurfaceEmbedded}),
	} {
		if len(got) != 0 {
			t.Errorf("%s：%v", name, got)
		}
	}
}

func TestGetMyPaymentAccountReturnsTheNeutralStatusAndTheProviderSurface(t *testing.T) {
	fetched := time.Date(2026, 11, 2, 3, 0, 0, 0, time.UTC)
	m := &paymentModule{
		configured: true, caps: payment.Capabilities{RequirementCollection: payment.CollectionPlatform, OnboardingSurface: payment.SurfaceEmbedded},
		account: payment.Account{
			StoreID: 11, Status: payment.StatusActionRequired, RequirementCollection: payment.CollectionPlatform, TransfersActive: false, PayoutsActive: true,
			Requirements: json.RawMessage(`{"currently_due":["individual.id_number"]}`), FetchedAt: &fetched,
		},
	}
	app := &App{payments: m}
	got, err := app.GetMyPaymentAccount(ctxWithStore(currentStore()), connect.NewRequest(&pb.GetMyPaymentAccountRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	a := got.Msg.GetAccount()
	if a.GetStatus() != paymentpb.PaymentAccountStatus_PAYMENT_ACCOUNT_STATUS_ACTION_REQUIRED ||
		a.GetOnboardingSurface() != paymentpb.OnboardingSurface_ONBOARDING_SURFACE_EMBEDDED || !a.GetPayoutsActive() || !a.GetFetchTime().AsTime().Equal(fetched) {
		t.Fatalf("account=%v", a)
	}
	m.configured = false
	got, err = app.GetMyPaymentAccount(ctxWithStore(currentStore()), connect.NewRequest(&pb.GetMyPaymentAccountRequest{}))
	if err != nil || got.Msg.GetAccount().GetOnboardingSurface() != paymentpb.OnboardingSurface_ONBOARDING_SURFACE_UNSPECIFIED {
		t.Fatalf("沒有設定收款開通：%v %v", got, err)
	}
}

func TestCreatePaymentOnboardingSessionBuildsTheReturnUrlsOnTheServer(t *testing.T) {
	m := &paymentModule{}
	app := &App{payments: m, web: mustWebOrigin(t)}
	origin := mustWebOrigin(t).Origin()
	cert := pb.OnboardingReturnDestination_ONBOARDING_RETURN_DESTINATION_WEBSITE_CERTIFICATION
	appDest := pb.OnboardingReturnDestination_ONBOARDING_RETURN_DESTINATION_SELLER_APP
	for name, req := range map[string]*pb.CreatePaymentOnboardingSessionRequest{
		"destination 未指定":  {Locale: "zh-TW"},
		"destination 不認得":  {Destination: 99, Locale: "zh-TW"},
		"沒有 locale":        {Destination: cert},
		"不支援的 locale":      {Destination: cert, Locale: "fr"},
		"App 目的地也驗 locale": {Destination: appDest, Locale: "fr"},
		"locale 大小寫不同":     {Destination: cert, Locale: "EN"},
	} {
		if _, err := app.CreatePaymentOnboardingSession(ctxWithStore(currentStore()), connect.NewRequest(req)); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s：err=%v", name, err)
		}
	}
	if m.sessionFor != 0 {
		t.Fatal("請求不合仍然呼叫了模組")
	}
	for name, c := range map[string]struct {
		dest             pb.OnboardingReturnDestination
		locale           string
		wantRet, wantRef string
	}{
		"網站 zh-TW":  {cert, "zh-TW", origin + "/seller-center/me/certifications?payment=return", origin + "/seller-center/me/certifications?payment=refresh"},
		"網站 en":     {cert, "en", origin + "/en/seller-center/me/certifications?payment=return", origin + "/en/seller-center/me/certifications?payment=refresh"},
		"App zh-TW": {appDest, "zh-TW", origin + "/seller-app/payment-return?state=done", origin + "/seller-app/payment-return?state=expired"},
		"App en":    {appDest, "en", origin + "/seller-app/payment-return?state=done", origin + "/seller-app/payment-return?state=expired"},
	} {
		m.sessionFor = 0
		got, err := app.CreatePaymentOnboardingSession(ctxWithStore(currentStore()), connect.NewRequest(&pb.CreatePaymentOnboardingSessionRequest{Destination: c.dest, Locale: c.locale}))
		if err != nil {
			t.Fatalf("%s：%v", name, err)
		}
		if m.sessionFor != 11 || m.returnURL != c.wantRet || m.refreshURL != c.wantRef {
			t.Errorf("%s：模組收到 store=%d return=%q refresh=%q", name, m.sessionFor, m.returnURL, m.refreshURL)
		}
		if got.Msg.GetSurface() != paymentpb.OnboardingSurface_ONBOARDING_SURFACE_HOSTED || got.Msg.GetUrl() == "" || got.Msg.GetClientSecret() != "" {
			t.Errorf("%s：response=%v", name, got.Msg)
		}
	}
}

// 是否到網站完成由 payment 依帳戶列的介面決定；handler 只告知呼叫端能否內嵌（App 不能），並在收到「到網站完成」時填網站認證頁。
func TestCreatePaymentOnboardingSessionFillsTheWebsiteURLWhenPaymentSaysCompleteOnTheWebsite(t *testing.T) {
	m := &paymentModule{session: &payment.OnboardingSession{Surface: payment.SurfaceEmbedded, CompleteOnWebsite: true}}
	app := &App{payments: m, web: mustWebOrigin(t)}
	origin := mustWebOrigin(t).Origin()
	appDest := pb.OnboardingReturnDestination_ONBOARDING_RETURN_DESTINATION_SELLER_APP
	for locale, want := range map[string]string{
		"zh-TW": origin + "/seller-center/me/certifications",
		"en":    origin + "/en/seller-center/me/certifications",
	} {
		m.sessionFor = 0
		got, err := app.CreatePaymentOnboardingSession(ctxWithStore(currentStore()), connect.NewRequest(&pb.CreatePaymentOnboardingSessionRequest{Destination: appDest, Locale: locale}))
		if err != nil {
			t.Fatal(err)
		}
		if m.sessionFor != 11 || m.canEmbed {
			t.Fatalf("%s：App 目的地應以 canEmbed=false 交給 payment，sessionFor=%d canEmbed=%v", locale, m.sessionFor, m.canEmbed)
		}
		g := got.Msg
		if g.GetUrl() != want || g.GetClientSecret() != "" || g.GetProvider() != "" || g.GetPublishableKey() != "" ||
			g.GetSurface() != paymentpb.OnboardingSurface_ONBOARDING_SURFACE_EMBEDDED {
			t.Errorf("%s：response=%v", locale, g)
		}
	}
	// 網站目的地以 canEmbed=true 交給 payment，內嵌 session 原樣回。
	m.session = &payment.OnboardingSession{Surface: payment.SurfaceEmbedded, ClientSecret: "cs_1", Provider: "stripe", PublishableKey: "pk_test_x"}
	m.sessionFor = 0
	got, err := app.CreatePaymentOnboardingSession(ctxWithStore(currentStore()), connect.NewRequest(&pb.CreatePaymentOnboardingSessionRequest{
		Destination: pb.OnboardingReturnDestination_ONBOARDING_RETURN_DESTINATION_WEBSITE_CERTIFICATION, Locale: "zh-TW"}))
	if err != nil || m.sessionFor != 11 || !m.canEmbed || got.Msg.GetClientSecret() != "cs_1" {
		t.Fatalf("網站目的地：%v %v sessionFor=%d canEmbed=%v", got, err, m.sessionFor, m.canEmbed)
	}
	// payment 的前置條件失敗原樣回，不填網站網址。
	m.session, m.err = nil, apperr.FailedPrecondition("收款帳戶還沒有建立")
	if _, err := app.CreatePaymentOnboardingSession(ctxWithStore(currentStore()), connect.NewRequest(&pb.CreatePaymentOnboardingSessionRequest{Destination: appDest, Locale: "zh-TW"})); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("前置條件失敗 err=%v", err)
	}
}

func TestCreatePaymentOnboardingSessionCarriesTheProviderAndPublishableKeyOnlyWhenEmbedded(t *testing.T) {
	m := &paymentModule{session: &payment.OnboardingSession{Surface: payment.SurfaceEmbedded, ClientSecret: "cs_1", Provider: "stripe", PublishableKey: "pk_test_x"}}
	app := &App{payments: m, web: mustWebOrigin(t)}
	req := connect.NewRequest(&pb.CreatePaymentOnboardingSessionRequest{Destination: pb.OnboardingReturnDestination_ONBOARDING_RETURN_DESTINATION_WEBSITE_CERTIFICATION, Locale: "zh-TW"})
	got, err := app.CreatePaymentOnboardingSession(ctxWithStore(currentStore()), req)
	if err != nil {
		t.Fatal(err)
	}
	if got.Msg.GetClientSecret() != "cs_1" || got.Msg.GetProvider() != "stripe" || got.Msg.GetPublishableKey() != "pk_test_x" || got.Msg.GetUrl() != "" {
		t.Fatalf("內嵌：%v", got.Msg)
	}
	m.session = nil
	got, err = app.CreatePaymentOnboardingSession(ctxWithStore(currentStore()), req)
	if err != nil {
		t.Fatal(err)
	}
	if got.Msg.GetProvider() != "" || got.Msg.GetPublishableKey() != "" || got.Msg.GetUrl() == "" {
		t.Fatalf("託管：%v", got.Msg)
	}
}
