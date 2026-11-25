package seller

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"uuid"

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
	// 入帳帳本：記下收到的店家、篩選、游標與筆數；page 與 order 是回應。
	earningsStore  int64
	earningsFilter payment.EarningsFilter
	earningsAfter  *payment.RecordCursor
	earningsSize   int32
	page           payment.EarningsPage
	order          payment.OrderEarnings
}

func (m *paymentModule) ListEarnings(_ context.Context, storeID int64, f payment.EarningsFilter, after *payment.RecordCursor, size int32) (payment.EarningsPage, error) {
	m.earningsStore, m.earningsFilter, m.earningsAfter, m.earningsSize = storeID, f, after, size
	return m.page, m.err
}

func (m *paymentModule) OrderEarnings(_ context.Context, storeID int64, _ string) (payment.OrderEarnings, error) {
	m.earningsStore = storeID
	return m.order, m.err
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
	return payment.HostedSession{URL: "https://onboarding.example.test/x"}, nil
}

func TestPaymentEnumsMatchTheModule(t *testing.T) {
	for name, got := range map[string][]string{
		"PaymentAccountStatus":      rpc.EnumMismatches[paymentpb.PaymentAccountStatus](payment.AccountStatusValues()),
		"RequirementCollection":     rpc.EnumMismatches[paymentpb.RequirementCollection](payment.RequirementCollectionValues()),
		"EmbeddedOnboardingRuntime": rpc.EnumMismatches[paymentpb.EmbeddedOnboardingRuntime](payment.EmbeddedRuntimeValues()),
		"EarningsEntryType":         rpc.EnumMismatches[paymentpb.EarningsEntryType](payment.EarningsTypeValues()),
		"EarningsEntryStatus":       rpc.EnumMismatches[paymentpb.EarningsEntryStatus](payment.EarningsStatusValues()),
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

// 契約的每個內嵌執行環境（UNSPECIFIED 除外）都轉成 payment 的值；不認得的數字不會傳下去。
func TestEmbeddedRuntimesMapsEveryProtoValue(t *testing.T) {
	vs := paymentpb.EmbeddedOnboardingRuntime(0).Descriptor().Values()
	for i := range vs.Len() {
		e := paymentpb.EmbeddedOnboardingRuntime(vs.Get(i).Number())
		got := embeddedRuntimes([]paymentpb.EmbeddedOnboardingRuntime{e})
		if e == 0 {
			if len(got) != 0 {
				t.Errorf("UNSPECIFIED 不該傳下去：%v", got)
			}
			continue
		}
		if len(got) != 1 || !slices.Contains(payment.EmbeddedRuntimeValues(), got[0]) {
			t.Errorf("%s：轉成 %v", e, got)
		}
	}
	if got := embeddedRuntimes([]paymentpb.EmbeddedOnboardingRuntime{99}); len(got) != 0 {
		t.Errorf("不認得的值傳下去了：%v", got)
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

func earningsFixture() []payment.EarningsEntry {
	at := time.Date(2026, 11, 20, 3, 0, 0, 0, time.UTC)
	return []payment.EarningsEntry{
		{ID: uuid.MustParse("01930000-0000-7000-8000-000000000003"), Type: payment.EarningsReversal, OrderNo: "A1", Amount: -300, Status: payment.EarningsCompleted, CreatedAt: at},
		{ID: uuid.MustParse("01930000-0000-7000-8000-000000000002"), Type: payment.EarningsTopUp, OrderNo: "A1", Amount: 100, Status: payment.EarningsPending, CreatedAt: at},
		{ID: uuid.MustParse("01930000-0000-7000-8000-000000000001"), Type: payment.EarningsTransfer, OrderNo: "A1", Amount: 1000, Status: payment.EarningsFailed, CreatedAt: at},
	}
}

// 店家只從能力檢查的 ctx 取得：請求不帶店家，模組收到的就是呼叫者的店家；收回是負數，種類與狀態轉成 proto 的列舉。
func TestListMyEarningsUsesTheCallersStoreAndMapsEntries(t *testing.T) {
	m := &paymentModule{page: payment.EarningsPage{Entries: earningsFixture()}}
	app := &App{payments: m}
	got, err := app.ListMyEarnings(ctxWithStore(currentStore()), connect.NewRequest(&pb.ListMyEarningsRequest{OrderNo: "A1", PageSize: 5}))
	if err != nil {
		t.Fatal(err)
	}
	if m.earningsStore != currentStore().StoreID || m.earningsFilter.OrderNo != "A1" || m.earningsSize != 5 {
		t.Fatalf("模組收到 store=%d filter=%+v size=%d", m.earningsStore, m.earningsFilter, m.earningsSize)
	}
	es := got.Msg.GetEntries()
	if len(es) != 3 || got.Msg.GetNextPageToken() != "" {
		t.Fatalf("entries=%d token=%q", len(es), got.Msg.GetNextPageToken())
	}
	if es[0].GetType() != paymentpb.EarningsEntryType_EARNINGS_ENTRY_TYPE_REVERSAL || es[0].GetAmount() != -300 || es[0].GetStatus() != paymentpb.EarningsEntryStatus_EARNINGS_ENTRY_STATUS_COMPLETED {
		t.Errorf("收回：%v", es[0])
	}
	if es[1].GetType() != paymentpb.EarningsEntryType_EARNINGS_ENTRY_TYPE_TOP_UP || es[1].GetStatus() != paymentpb.EarningsEntryStatus_EARNINGS_ENTRY_STATUS_PENDING {
		t.Errorf("補差額：%v", es[1])
	}
	if es[2].GetType() != paymentpb.EarningsEntryType_EARNINGS_ENTRY_TYPE_TRANSFER || es[2].GetStatus() != paymentpb.EarningsEntryStatus_EARNINGS_ENTRY_STATUS_FAILED || es[2].GetId() == "" || es[2].GetCreateTime() == nil {
		t.Errorf("轉入：%v", es[2])
	}
}

// 下一頁的 token 帶回模組的游標；token 只能用在同一個店家與同一組篩選，換了篩選回 invalid_argument。
func TestListMyEarningsPageTokenIsBoundToTheFilter(t *testing.T) {
	last := earningsFixture()[2].ID
	m := &paymentModule{page: payment.EarningsPage{Entries: earningsFixture(), Next: &payment.RecordCursor{ID: last}}}
	app := &App{payments: m}
	ctx := ctxWithStore(currentStore())
	first, err := app.ListMyEarnings(ctx, connect.NewRequest(&pb.ListMyEarningsRequest{OrderNo: "A1"}))
	if err != nil || first.Msg.GetNextPageToken() == "" {
		t.Fatalf("err=%v token=%q", err, first.Msg.GetNextPageToken())
	}
	if _, err := app.ListMyEarnings(ctx, connect.NewRequest(&pb.ListMyEarningsRequest{OrderNo: "A1", PageToken: first.Msg.GetNextPageToken()})); err != nil {
		t.Fatal(err)
	}
	if m.earningsAfter == nil || m.earningsAfter.ID != last {
		t.Fatalf("游標沒有帶給模組：%v", m.earningsAfter)
	}
	_, err = app.ListMyEarnings(ctx, connect.NewRequest(&pb.ListMyEarningsRequest{OrderNo: "B2", PageToken: first.Msg.GetNextPageToken()}))
	if !isTransportInvalidArgument(err) {
		t.Fatalf("換了篩選的 token 沒有被擋：%v", err)
	}
	other := currentStore()
	other.StoreID = 12
	_, err = app.ListMyEarnings(ctxWithStore(other), connect.NewRequest(&pb.ListMyEarningsRequest{OrderNo: "A1", PageToken: first.Msg.GetNextPageToken()}))
	if !isTransportInvalidArgument(err) {
		t.Fatalf("別家店的 token 沒有被擋：%v", err)
	}
}

func TestListMyEarningsRequiresAStoreInTheContext(t *testing.T) {
	app := &App{payments: &paymentModule{}}
	if _, err := app.ListMyEarnings(context.Background(), connect.NewRequest(&pb.ListMyEarningsRequest{})); err == nil {
		t.Fatal("沒有店家仍然讀了帳本")
	}
	if _, err := app.GetMyOrderEarnings(context.Background(), connect.NewRequest(&pb.GetMyOrderEarningsRequest{OrderNo: "A1"})); err == nil {
		t.Fatal("沒有店家仍然讀了訂單的帳本")
	}
}

func TestGetMyOrderEarningsReturnsEntriesAndNet(t *testing.T) {
	m := &paymentModule{order: payment.OrderEarnings{Entries: earningsFixture(), NetAmount: -300}}
	app := &App{payments: m}
	got, err := app.GetMyOrderEarnings(ctxWithStore(currentStore()), connect.NewRequest(&pb.GetMyOrderEarningsRequest{OrderNo: "A1"}))
	if err != nil {
		t.Fatal(err)
	}
	if m.earningsStore != currentStore().StoreID || len(got.Msg.GetEntries()) != 3 || got.Msg.GetNetAmount() != -300 {
		t.Fatalf("store=%d got=%v", m.earningsStore, got.Msg)
	}
}

func TestGetMyOrderEarningsPassesNotFoundThrough(t *testing.T) {
	app := &App{payments: &paymentModule{err: apperr.NotFound("找不到這張訂單的入帳")}}
	_, err := app.GetMyOrderEarnings(ctxWithStore(currentStore()), connect.NewRequest(&pb.GetMyOrderEarningsRequest{OrderNo: "X"}))
	if apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("err=%v", err)
	}
}

// handler 直接回傳的是 interceptor 轉換之前的傳輸層錯誤，錯誤文字以代碼開頭。
func isTransportInvalidArgument(err error) bool {
	return err != nil && strings.HasPrefix(err.Error(), "invalid_argument")
}
