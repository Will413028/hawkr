package seller

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"connectrpc.com/connect"

	paymentpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/payment/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1/sellerv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

type paymentModule struct {
	account    payment.Account
	configured bool
	sessionFor int64
	runtimes   []payment.EmbeddedRuntime
	calls      int
	returnURL  string
	refreshURL string
	err        error
	// session 是 CreateOnboardingSession 回的 session；nil 時回託管連結。
	session payment.OnboardingSession
	// syncedFor 記下 SyncAccountUnlessFresh 被呼叫的店家；syncErr 非 nil 時同步失敗。
	syncedFor []int64
	syncErr   error
}

func (m *paymentModule) Account(_ context.Context, storeID int64) (payment.Account, error) {
	return m.account, m.err
}

func (m *paymentModule) Configured() bool { return m.configured }

func (m *paymentModule) SyncAccountUnlessFresh(_ context.Context, storeID int64) error {
	m.syncedFor = append(m.syncedFor, storeID)
	return m.syncErr
}

func (m *paymentModule) CreateOnboardingSession(_ context.Context, storeID int64, returnURL, refreshURL string, runtimes []payment.EmbeddedRuntime) (payment.OnboardingSession, error) {
	m.calls++
	m.sessionFor, m.returnURL, m.refreshURL, m.runtimes = storeID, returnURL, refreshURL, runtimes
	if m.err != nil {
		return nil, m.err
	}
	if m.session != nil {
		return m.session, nil
	}
	exp := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	return payment.HostedSession{URL: "https://onboarding.example.test/x", ExpiresAt: &exp}, nil
}

func TestPaymentEnumsMatchTheModule(t *testing.T) {
	for name, got := range map[string][]string{
		"PaymentAccountStatus":      rpc.EnumMismatches[paymentpb.PaymentAccountStatus](payment.AccountStatusValues()),
		"RequirementCollection":     rpc.EnumMismatches[paymentpb.RequirementCollection](payment.RequirementCollectionValues()),
		"EmbeddedOnboardingRuntime": rpc.EnumMismatches[paymentpb.EmbeddedOnboardingRuntime]([]payment.EmbeddedRuntime{payment.RuntimeStripeConnectJS}),
	} {
		if len(got) != 0 {
			t.Errorf("%s：%v", name, got)
		}
	}
}

func TestGetMyPaymentAccountReturnsTheNeutralStatus(t *testing.T) {
	fetched := time.Date(2026, 11, 2, 3, 0, 0, 0, time.UTC)
	m := &paymentModule{
		configured: true,
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
	if a.GetStatus() != paymentpb.PaymentAccountStatus_PAYMENT_ACCOUNT_STATUS_ACTION_REQUIRED || !a.GetPayoutsActive() || !a.GetFetchTime().AsTime().Equal(fetched) {
		t.Fatalf("account=%v", a)
	}
}

const (
	destCert = pb.OnboardingReturnDestination_ONBOARDING_RETURN_DESTINATION_WEBSITE_CERTIFICATION
	destApp  = pb.OnboardingReturnDestination_ONBOARDING_RETURN_DESTINATION_SELLER_APP
)

func TestCreatePaymentOnboardingSessionRejectsAnUnknownDestinationWithoutCallingTheModule(t *testing.T) {
	m := &paymentModule{}
	app := &App{payments: m, web: mustWebOrigin(t)}
	for name, dest := range map[string]pb.OnboardingReturnDestination{"未指定": 0, "不認得": 99} {
		_, err := app.CreatePaymentOnboardingSession(ctxWithStore(currentStore()), connect.NewRequest(&pb.CreatePaymentOnboardingSessionRequest{Destination: dest}))
		if apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("destination %s：err=%v", name, err)
		}
	}
	if m.calls != 0 {
		t.Fatal("請求不合仍然呼叫了模組")
	}
}

// 回跳網址的語系前綴取自請求 ctx 的語系（Accept-Language 協商的結果）；App 的回跳頁沒有語系前綴。
func TestCreatePaymentOnboardingSessionBuildsTheReturnUrlsFromTheRequestLocale(t *testing.T) {
	m := &paymentModule{}
	app := &App{payments: m, web: mustWebOrigin(t)}
	origin := mustWebOrigin(t).Origin()
	for name, c := range map[string]struct {
		dest             pb.OnboardingReturnDestination
		locale           uitext.Locale
		wantRet, wantRef string
	}{
		"網站 zh-TW":  {destCert, uitext.ZhTW, origin + "/seller-center/me/certifications?payment=return", origin + "/seller-center/me/certifications?payment=refresh"},
		"網站 en":     {destCert, uitext.EN, origin + "/en/seller-center/me/certifications?payment=return", origin + "/en/seller-center/me/certifications?payment=refresh"},
		"App zh-TW": {destApp, uitext.ZhTW, origin + "/seller-app/payment-return?state=done", origin + "/seller-app/payment-return?state=expired"},
		"App en":    {destApp, uitext.EN, origin + "/seller-app/payment-return?state=done", origin + "/seller-app/payment-return?state=expired"},
	} {
		m.sessionFor = 0
		ctx := uitext.WithLocale(ctxWithStore(currentStore()), c.locale)
		if _, err := app.CreatePaymentOnboardingSession(ctx, connect.NewRequest(&pb.CreatePaymentOnboardingSessionRequest{Destination: c.dest})); err != nil {
			t.Fatalf("%s：%v", name, err)
		}
		if m.sessionFor != 11 || m.returnURL != c.wantRet || m.refreshURL != c.wantRef {
			t.Errorf("%s：模組收到 store=%d return=%q refresh=%q", name, m.sessionFor, m.returnURL, m.refreshURL)
		}
	}
}

// 經正式的 Connect handler 與 rpc 的 interceptor：Accept-Language: en 的回跳網址帶 /en，沒有 header 沒有前綴（zh-TW）。
func TestCreatePaymentOnboardingSessionReturnUrlsFollowTheAcceptLanguageHeader(t *testing.T) {
	m := &paymentModule{}
	app := &App{payments: m, web: mustWebOrigin(t)}
	mux := http.NewServeMux()
	mux.Handle(sellerv1alpha1connect.NewSellerAppServiceHandler(app, connect.WithInterceptors(rpc.NewInterceptor(slog.New(slog.DiscardHandler)))))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := rpc.WithPrincipal(r.Context(), identity.Principal{Kind: identity.PrincipalAccount, ID: 7})
		mux.ServeHTTP(w, r.WithContext(context.WithValue(ctx, storeKey{}, currentStore())))
	}))
	t.Cleanup(srv.Close)
	client := sellerv1alpha1connect.NewSellerAppServiceClient(srv.Client(), srv.URL)
	origin := mustWebOrigin(t).Origin()
	for name, c := range map[string]struct{ header, want string }{
		"en":        {"en", origin + "/en/seller-center/me/certifications?payment=return"},
		"沒有 header": {"", origin + "/seller-center/me/certifications?payment=return"},
		"zh-TW":     {"zh-TW", origin + "/seller-center/me/certifications?payment=return"},
	} {
		req := connect.NewRequest(&pb.CreatePaymentOnboardingSessionRequest{Destination: destCert})
		if c.header != "" {
			req.Header().Set("Accept-Language", c.header)
		}
		if _, err := client.CreatePaymentOnboardingSession(context.Background(), req); err != nil {
			t.Fatalf("%s：%v", name, err)
		}
		if m.returnURL != c.want {
			t.Errorf("%s：return=%q，預期 %q", name, m.returnURL, c.want)
		}
	}
}

// 呼叫端宣告的內嵌執行環境原樣交給 payment；UNSPECIFIED 與不認得的值不會傳下去。
func TestCreatePaymentOnboardingSessionPassesOnlyKnownRuntimesToPayment(t *testing.T) {
	m := &paymentModule{}
	app := &App{payments: m, web: mustWebOrigin(t)}
	connectJS := paymentpb.EmbeddedOnboardingRuntime_EMBEDDED_ONBOARDING_RUNTIME_STRIPE_CONNECT_JS
	for name, c := range map[string]struct {
		in   []paymentpb.EmbeddedOnboardingRuntime
		want []payment.EmbeddedRuntime
	}{
		"空清單":                   {nil, nil},
		"宣告 Connect.js":         {[]paymentpb.EmbeddedOnboardingRuntime{connectJS}, []payment.EmbeddedRuntime{payment.RuntimeStripeConnectJS}},
		"只有 UNSPECIFIED 與不認得的值": {[]paymentpb.EmbeddedOnboardingRuntime{0, 99}, nil},
		"混合":                    {[]paymentpb.EmbeddedOnboardingRuntime{99, connectJS, 0}, []payment.EmbeddedRuntime{payment.RuntimeStripeConnectJS}},
	} {
		req := &pb.CreatePaymentOnboardingSessionRequest{Destination: destCert, EmbeddedRuntimes: c.in}
		if _, err := app.CreatePaymentOnboardingSession(ctxWithStore(currentStore()), connect.NewRequest(req)); err != nil {
			t.Fatalf("%s：%v", name, err)
		}
		if !slices.Equal(m.runtimes, c.want) {
			t.Errorf("%s：payment 收到 %v，預期 %v", name, m.runtimes, c.want)
		}
	}
}

// 回應的 oneof：託管連結只有 hosted，Stripe 內嵌只有 embedded.stripe；兩種形狀互斥；payment 回不認得的形狀是伺服器的錯；前置條件失敗原樣回。
func TestCreatePaymentOnboardingSessionAnswersWithExactlyOneSessionShape(t *testing.T) {
	m := &paymentModule{}
	app := &App{payments: m, web: mustWebOrigin(t)}
	call := func() (*pb.CreatePaymentOnboardingSessionResponse, error) {
		got, err := app.CreatePaymentOnboardingSession(ctxWithStore(currentStore()), connect.NewRequest(&pb.CreatePaymentOnboardingSessionRequest{Destination: destCert}))
		if err != nil {
			return nil, err
		}
		return got.Msg, nil
	}
	got, err := call()
	if err != nil || got.GetHosted().GetUrl() != "https://onboarding.example.test/x" || got.GetEmbedded() != nil {
		t.Fatalf("託管：%v %v", got, err)
	}
	m.session = payment.StripeEmbeddedSession{ClientSecret: "cs_1", PublishableKey: "pk_test_x"}
	got, err = call()
	if err != nil || got.GetHosted() != nil || got.GetEmbedded().GetStripe().GetClientSecret() != "cs_1" || got.GetEmbedded().GetStripe().GetPublishableKey() != "pk_test_x" {
		t.Fatalf("內嵌：%v %v", got, err)
	}
	m.session = unknownSession{}
	if _, err := call(); err == nil {
		t.Fatal("不認得的 session 形狀被當成成功")
	}
	m.session, m.err = nil, apperr.FailedPrecondition("收款帳戶還沒有建立")
	if _, err := call(); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("前置條件失敗 err=%v", err)
	}
}

// unknownSession 模擬 payment 新增了 handler 還沒處理的 session 形狀。payment.OnboardingSession 是封閉的 interface，
// 這裡以巢狀的內嵌型別取得它的未匯出方法。
type unknownSession struct{ payment.HostedSession }
