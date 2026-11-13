package account

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/stripe"
)

var _ Provider = (*Stripe)(nil)

const whsecret = "whsec_test"

// clock 是測試開始時的真實時間：SDK 以系統時間驗 webhook 簽章的時間戳。
var clock = time.Now()

// fixtures 是 platform/stripe/testdata 的實測回應（{status, body}，遮罩過）。
func fixtureResponder(t *testing.T, name string) http.HandlerFunc {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "platform", "stripe", "testdata", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Status int             `json:"status"`
		Body   json.RawMessage `json:"body"`
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(f.Status)
		_, _ = w.Write(f.Body)
	}
}

func stripeProvider(t *testing.T, collection model.RequirementCollection, h http.Handler) *Stripe {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c, err := stripe.New(stripe.Config{
		BaseURL: srv.URL, SecretKey: "sk_test_x", WebhookSecret: whsecret,
		Wait: func(context.Context, time.Duration) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := NewStripe(c, collection)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestStripeCapabilitiesFollowTheCollectionMode(t *testing.T) {
	c, _ := stripe.New(stripe.Config{SecretKey: "k", WebhookSecret: "w"})
	for mode, want := range map[model.RequirementCollection]Capabilities{
		model.CollectionProvider: {RequirementCollection: model.CollectionProvider, OnboardingSurface: model.SurfaceHosted},
		model.CollectionPlatform: {RequirementCollection: model.CollectionPlatform, OnboardingSurface: model.SurfaceEmbedded},
	} {
		p, err := NewStripe(c, mode)
		if err != nil || p.Capabilities() != want || p.Name() != "stripe" {
			t.Errorf("%s：%+v %v", mode, p, err)
		}
	}
	if _, err := NewStripe(c, ""); err == nil {
		t.Error("空的收件方式通過了")
	}
	if _, err := NewStripe(nil, model.CollectionProvider); err == nil {
		t.Error("沒有 client 通過了")
	}
}

// 收件方式取自帳戶列（EnsureInput.Collection），不是行程設定：adapter 設定成相反的值，建立的 dashboard 仍跟著列走。
func TestStripeEnsureAccountUsesTheDashboardOfTheStoredCollection(t *testing.T) {
	for stored, dash := range map[model.RequirementCollection]string{model.CollectionProvider: "express", model.CollectionPlatform: "none"} {
		configured := model.CollectionProvider
		if stored == model.CollectionProvider {
			configured = model.CollectionPlatform
		}
		var got map[string]any
		p := stripeProvider(t, configured, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&got)
			_, _ = w.Write([]byte(`{"id":"acct_1"}`))
		}))
		id, err := p.EnsureAccount(context.Background(), EnsureInput{StoreID: 5, IdempotencyKey: "k", ContactEmail: "seller@example.com", Collection: stored})
		if err != nil || id != "acct_1" || got["dashboard"] != dash || got["contact_email"] != "seller@example.com" {
			t.Errorf("列 %s：id=%q err=%v dashboard=%v contact_email=%v", stored, id, err, got["dashboard"], got["contact_email"])
		}
	}
	p := stripeProvider(t, model.CollectionProvider, http.NotFoundHandler())
	if _, err := p.EnsureAccount(context.Background(), EnsureInput{IdempotencyKey: "k"}); !errors.Is(err, ErrPermanent) {
		t.Errorf("空的收件方式：%v，預期 ErrPermanent", err)
	}
}

// 沒有聯絡信箱時不呼叫 Stripe：有收款設定的帳戶沒有信箱一定被拒，重試也不會成功。
func TestStripeEnsureAccountWithoutContactEmailIsPermanentAndSendsNothing(t *testing.T) {
	called := false
	p := stripeProvider(t, model.CollectionProvider, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		_, _ = w.Write([]byte(`{"id":"acct_1"}`))
	}))
	if _, err := p.EnsureAccount(context.Background(), EnsureInput{StoreID: 5, IdempotencyKey: "k", Collection: model.CollectionProvider}); !errors.Is(err, ErrPermanent) || called {
		t.Errorf("err=%v called=%v，預期 ErrPermanent 且沒有送出請求", err, called)
	}
}

// 重試沿用同一把鍵：adapter 原樣把 EnsureInput 的鍵送成 Idempotency-Key。
func TestStripeEnsureAccountSendsTheGivenKeyOnEveryAttempt(t *testing.T) {
	var keys []string
	p := stripeProvider(t, model.CollectionProvider, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		keys = append(keys, r.Header.Get("Idempotency-Key"))
		_, _ = w.Write([]byte(`{"id":"acct_1"}`))
	}))
	for range 2 {
		if _, err := p.EnsureAccount(context.Background(), EnsureInput{StoreID: 5, IdempotencyKey: "6f1c0a52-0000-4000-8000-000000000001", ContactEmail: "seller@example.com", Collection: model.CollectionProvider}); err != nil {
			t.Fatal(err)
		}
	}
	if fmt.Sprint(keys) != "[6f1c0a52-0000-4000-8000-000000000001 6f1c0a52-0000-4000-8000-000000000001]" {
		t.Errorf("Idempotency-Key %v", keys)
	}
}

func TestStripeEnsureAccountClassifiesErrors(t *testing.T) {
	create := func(h http.HandlerFunc) error {
		_, err := stripeProvider(t, model.CollectionProvider, h).EnsureAccount(context.Background(), EnsureInput{StoreID: 5, IdempotencyKey: "5", ContactEmail: "seller@example.com", Collection: model.CollectionProvider})
		return err
	}
	if err := create(fixtureResponder(t, "create_provider_idem_conflict")); !errors.Is(err, ErrPermanent) {
		t.Errorf("409 idempotency_error：%v，預期 ErrPermanent", err)
	}
	inProgress := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":{"code":"idempotency_key_in_use","message":"A request with this key is still being processed."}}`))
	}
	if err := create(inProgress); err == nil || errors.Is(err, ErrPermanent) {
		t.Errorf("409 同一把鍵的請求還在執行：%v，預期一般錯誤（重試）", err)
	}
	for _, status := range []int{500, 502, 429} {
		err := create(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(status) })
		if err == nil || errors.Is(err, ErrPermanent) {
			t.Errorf("%d：%v，預期一般錯誤（重試）", status, err)
		}
	}
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()
	c, _ := stripe.New(stripe.Config{BaseURL: url, SecretKey: "k", WebhookSecret: "w"})
	p, _ := NewStripe(c, model.CollectionProvider)
	if _, err := p.EnsureAccount(context.Background(), EnsureInput{IdempotencyKey: "5", ContactEmail: "seller@example.com", Collection: model.CollectionProvider}); err == nil || errors.Is(err, ErrPermanent) {
		t.Errorf("傳輸錯誤：%v，預期一般錯誤", err)
	}
	if _, err := p.EnsureAccount(context.Background(), EnsureInput{}); !errors.Is(err, ErrPermanent) {
		t.Errorf("沒有冪等鍵：%v", err)
	}
}

func TestStripeFetchNewAccountFixtureIsActionRequired(t *testing.T) {
	for _, fx := range []string{"retrieve_provider_new", "retrieve_platform_new"} {
		p := stripeProvider(t, model.CollectionProvider, fixtureResponder(t, fx))
		snap, err := p.FetchAccount(context.Background(), "acct_1")
		if err != nil {
			t.Fatal(err)
		}
		status, reason, _ := StatusOf(snap)
		if status != model.PaymentActionRequired || reason != "requirements_past_due" {
			t.Errorf("%s：%s %q", fx, status, reason)
		}
		want := Snapshot{Transfers: CapabilityRestricted, Details: []StatusDetail{{Resolution: ResolutionSellerAction, Reason: "requirements_past_due"}}}
		if snap.Transfers != want.Transfers || len(snap.Details) != 1 || snap.Details[0] != want.Details[0] || snap.PayoutsActive {
			t.Errorf("%s：快照 %+v", fx, snap)
		}
		var req map[string]json.RawMessage
		if json.Unmarshal(snap.Requirements, &req) != nil || req["entries"] == nil {
			t.Errorf("%s：requirements 不是原始物件", fx)
		}
	}
}

func TestStripeFetchNotFoundFixtureIsPermanent(t *testing.T) {
	p := stripeProvider(t, model.CollectionProvider, fixtureResponder(t, "retrieve_not_found"))
	if _, err := p.FetchAccount(context.Background(), "acct_doesnotexist000"); !errors.Is(err, ErrPermanent) {
		t.Fatalf("err=%v", err)
	}
	if _, err := p.FetchAccount(context.Background(), ""); !errors.Is(err, ErrPermanent) {
		t.Fatalf("空的帳戶 ID：%v", err)
	}
}

func account(transfers, payouts string) string {
	return `{"id":"acct_1","configuration":{"recipient":{"capabilities":{"stripe_balance":{` + transfers + `,` + payouts + `}}}},"requirements":{"entries":[]}}`
}

func TestStripeFetchMapsCapabilityStatesAndResolutions(t *testing.T) {
	capJSON := func(key, status string, details ...string) string {
		s := fmt.Sprintf(`%q:{"status":%q,"status_details":[`, key, status)
		for i, d := range details {
			if i > 0 {
				s += ","
			}
			s += d
		}
		return s + "]}"
	}
	detail := func(code, res string) string { return fmt.Sprintf(`{"code":%q,"resolution":%q}`, code, res) }
	for name, tc := range map[string]struct {
		body       string
		wantState  CapabilityState
		wantRes    []Resolution
		wantPayout bool
		wantStatus model.PaymentAccountStatus
	}{
		"active 加撥款 active": {account(capJSON("stripe_transfers", "active"), capJSON("payouts", "active")), CapabilityActive, nil, true, model.PaymentEnabled},
		"active 撥款受限":       {account(capJSON("stripe_transfers", "active"), capJSON("payouts", "restricted", detail("x", "provide_info"))), CapabilityActive, nil, false, model.PaymentEnabled},
		"pending 加 provide_info": {account(capJSON("stripe_transfers", "pending", detail("requirements_pending_verification", "provide_info")), capJSON("payouts", "pending")),
			CapabilityPending, []Resolution{ResolutionSellerAction}, false, model.PaymentActionRequired},
		"pending 沒有細節": {account(capJSON("stripe_transfers", "pending"), capJSON("payouts", "pending")), CapabilityPending, nil, false, model.PaymentUnderReview},
		"restricted 加 contact_stripe": {account(capJSON("stripe_transfers", "restricted", detail("risk", "contact_stripe")), capJSON("payouts", "restricted")),
			CapabilityRestricted, []Resolution{ResolutionPlatformAction}, false, model.PaymentDisabled},
		"restricted 加 no_resolution": {account(capJSON("stripe_transfers", "restricted", detail("risk", "no_resolution")), capJSON("payouts", "restricted")),
			CapabilityRestricted, []Resolution{ResolutionNone}, false, model.PaymentDisabled},
		"rejected": {account(capJSON("stripe_transfers", "rejected", detail("rejected_fraud", "no_resolution")), capJSON("payouts", "rejected")),
			CapabilityRejected, []Resolution{ResolutionNone}, false, model.PaymentDisabled},
		"unsupported": {account(capJSON("stripe_transfers", "unsupported", detail("unsupported_country", "no_resolution")), capJSON("payouts", "unsupported")),
			CapabilityUnsupported, []Resolution{ResolutionNone}, false, model.PaymentDisabled},
		"不認得的狀態與行動方": {account(capJSON("stripe_transfers", "brand_new", detail("c", "new_resolution")), capJSON("payouts", "brand_new")),
			CapabilityUnknown, []Resolution{ResolutionUnknown}, false, model.PaymentUnderReview},
		"多筆細節": {account(capJSON("stripe_transfers", "restricted", detail("a", "contact_stripe"), detail("b", "provide_info")), capJSON("payouts", "restricted")),
			CapabilityRestricted, []Resolution{ResolutionPlatformAction, ResolutionSellerAction}, false, model.PaymentActionRequired},
	} {
		p := stripeProvider(t, model.CollectionProvider, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(tc.body)) }))
		snap, err := p.FetchAccount(context.Background(), "acct_1")
		if err != nil {
			t.Fatalf("%s：%v", name, err)
		}
		var res []Resolution
		for _, d := range snap.Details {
			res = append(res, d.Resolution)
		}
		if snap.Transfers != tc.wantState || fmt.Sprint(res) != fmt.Sprint(tc.wantRes) || snap.PayoutsActive != tc.wantPayout {
			t.Errorf("%s：快照 %+v", name, snap)
		}
		if got, _, _ := StatusOf(snap); got != tc.wantStatus {
			t.Errorf("%s：狀態 %s，want %s", name, got, tc.wantStatus)
		}
	}
}

// 沒有轉帳能力的回應是格式錯誤：回一般錯誤（重試），不產生快照，不能被存成審核中。
func TestStripeFetchWithoutTheTransfersCapabilityIsARetryableError(t *testing.T) {
	for name, body := range map[string]string{
		"沒有 recipient 設定":     `{"id":"acct_1","configuration":{"recipient":null},"requirements":null}`,
		"沒有 stripe_transfers": `{"id":"acct_1","configuration":{"recipient":{"capabilities":{"stripe_balance":{"payouts":{"status":"active"}}}}},"requirements":{}}`,
	} {
		p := stripeProvider(t, model.CollectionProvider, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) }))
		snap, err := p.FetchAccount(context.Background(), "acct_1")
		if err == nil || errors.Is(err, ErrPermanent) || snap.Transfers != "" {
			t.Errorf("%s：snap=%+v err=%v，預期一般錯誤", name, snap, err)
		}
	}
}

func TestStripeOnboardingSessionByCollectionMode(t *testing.T) {
	var paths []string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.URL.Path == "/v1/account_sessions" {
			fixtureResponder(t, "account_session_platform")(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"object":"v2.core.account_link","url":"https://connect.stripe.com/x","expires_at":"2026-11-04T01:02:03Z"}`))
	})
	in := SessionInput{ExternalAccountID: "acct_1", ReturnURL: "https://a.test/r", RefreshURL: "https://a.test/f"}

	// 介面取自帳戶列的收件方式；adapter 設定成相反的值，結果仍跟著列走。
	in.Collection = model.CollectionProvider
	hosted, err := stripeProvider(t, model.CollectionPlatform, handler).CreateOnboardingSession(context.Background(), in)
	if err != nil || hosted.Surface != model.SurfaceHosted || hosted.URL != "https://connect.stripe.com/x" || hosted.ClientSecret != "" || hosted.ExpiresAt == nil {
		t.Errorf("託管 %+v err=%v", hosted, err)
	}
	in.Collection = model.CollectionPlatform
	embedded, err := stripeProvider(t, model.CollectionProvider, handler).CreateOnboardingSession(context.Background(), in)
	if err != nil || embedded.Surface != model.SurfaceEmbedded || embedded.ClientSecret == "" || embedded.URL != "" || embedded.ExpiresAt == nil {
		t.Errorf("內嵌 %+v err=%v", embedded, err)
	}
	if fmt.Sprint(paths) != "[/v2/core/account_links /v1/account_sessions]" {
		t.Errorf("呼叫的路徑 %v", paths)
	}
	if _, err := stripeProvider(t, model.CollectionProvider, handler).CreateOnboardingSession(context.Background(), SessionInput{Collection: model.CollectionProvider}); !errors.Is(err, ErrPermanent) {
		t.Errorf("沒有帳戶 ID：%v", err)
	}
	if _, err := stripeProvider(t, model.CollectionProvider, handler).CreateOnboardingSession(context.Background(), SessionInput{ExternalAccountID: "acct_1"}); !errors.Is(err, ErrPermanent) {
		t.Errorf("空的收件方式：%v", err)
	}
}

func signedEvent(typ, relatedType string, secret string, at time.Time) (http.Header, []byte) {
	body := fmt.Sprintf(`{"id":"evt_1","object":"v2.core.event","type":%q,"created":"2026-11-03T12:00:00.000Z","livemode":false,"context":null,"reason":null,"related_object":{"id":"acct_1","type":%q,"url":"/v2/core/accounts/acct_1"}}`, typ, relatedType)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strconv.FormatInt(at.Unix(), 10) + "." + body))
	return http.Header{"Stripe-Signature": {fmt.Sprintf("t=%d,v1=%s", at.Unix(), hex.EncodeToString(mac.Sum(nil)))}}, []byte(body)
}

func TestStripeParseEventMapsSignatureIgnoredAndAccepted(t *testing.T) {
	p := stripeProvider(t, model.CollectionProvider, http.NotFoundHandler())

	h, body := signedEvent("v2.core.account[configuration.recipient].capability_status_updated", "v2.core.account", whsecret, clock)
	if ev, err := p.ParseEvent(h, body); err != nil || ev != (Event{ID: "evt_1", ExternalAccountID: "acct_1"}) {
		t.Errorf("接受：%+v %v", ev, err)
	}
	h, body = signedEvent("v2.core.account.created", "v2.core.account", "whsec_other", clock)
	if _, err := p.ParseEvent(h, body); !errors.Is(err, ErrInvalidEvent) || errors.Is(err, ErrIgnoredEvent) {
		t.Errorf("簽章不符：%v", err)
	}
	h, body = signedEvent("v2.core.account_person.created", "v2.core.account_person", whsecret, clock)
	if _, err := p.ParseEvent(h, body); !errors.Is(err, ErrIgnoredEvent) || errors.Is(err, ErrInvalidEvent) {
		t.Errorf("不處理的型別：%v", err)
	}
	h, body = signedEvent("v1.account.updated", "v1.account", whsecret, clock)
	if _, err := p.ParseEvent(h, body); !errors.Is(err, ErrIgnoredEvent) {
		t.Errorf("v1 事件：%v", err)
	}
	if _, err := p.ParseEvent(http.Header{}, body); !errors.Is(err, ErrInvalidEvent) {
		t.Errorf("沒有簽章：%v", err)
	}
}

// FetchAccount 把外部銀行帳號的 fingerprint 放進快照；讀 fingerprint 失敗（5xx）整個呼叫失敗且是一般錯誤（job 重試），不產生沒有 fingerprint 的快照。
// mutation：adapter 不呼叫 ListBankAccountFingerprints → 快照沒有 fingerprint；吞掉列表錯誤 → 第二段沒有錯誤；把錯誤轉成 ErrPermanent → errors.Is 檢查失敗。
func TestStripeFetchFillsBankFingerprintsAndFailsWholeWhenTheListFails(t *testing.T) {
	retrieve := fixtureResponder(t, "retrieve_provider_new")
	list := fixtureResponder(t, "external_accounts_bank")
	listStatus := 0
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/accounts/acct_1/external_accounts":
			if listStatus != 0 {
				w.WriteHeader(listStatus)
				return
			}
			list(w, r)
		default:
			retrieve(w, r)
		}
	})
	p := stripeProvider(t, model.CollectionProvider, h)
	snap, err := p.FetchAccount(context.Background(), "acct_1")
	if err != nil || len(snap.BankFingerprints) != 1 || snap.BankFingerprints[0] == "" {
		t.Fatalf("snap=%+v err=%v", snap, err)
	}
	listStatus = 503
	snap, err = p.FetchAccount(context.Background(), "acct_1")
	if err == nil || errors.Is(err, ErrPermanent) || snap.Transfers != "" || snap.BankFingerprints != nil {
		t.Fatalf("snap=%+v err=%v，預期一般錯誤且沒有快照", snap, err)
	}
	listStatus = 403
	if _, err := p.FetchAccount(context.Background(), "acct_1"); !errors.Is(err, ErrPermanent) {
		t.Fatalf("4xx 的列表錯誤是永久失敗：%v", err)
	}
}
