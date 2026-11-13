package stripe

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	sdk "github.com/stripe/stripe-go/v87"
)

type fixture struct {
	Status int             `json:"status"`
	Body   json.RawMessage `json:"body"`
}

func loadFixture(t *testing.T, name string) fixture {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var f fixture
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f fixture) object(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(f.Body, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

type recorded struct {
	Method, URI, ContentType, Version, Idempotency, Authorization string
	Body                                                          string
	Header                                                        http.Header
}

type stub struct {
	t    *testing.T
	mu   sync.Mutex
	reqs []recorded
	resp func(n int) (int, string, http.Header)
}

func (s *stub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	s.mu.Lock()
	s.reqs = append(s.reqs, recorded{r.Method, r.URL.RequestURI(), r.Header.Get("Content-Type"), r.Header.Get("Stripe-Version"), r.Header.Get("Idempotency-Key"), r.Header.Get("Authorization"), string(body), r.Header.Clone()})
	n := len(s.reqs)
	s.mu.Unlock()
	if v := r.Header.Get("Stripe-Version"); v != sdk.APIVersion {
		s.t.Errorf("Stripe-Version %q，預期 SDK 釘住的 %q", v, sdk.APIVersion)
	}
	status, out, hdr := s.resp(n)
	for k, v := range hdr {
		w.Header()[k] = v
	}
	w.WriteHeader(status)
	_, _ = w.Write([]byte(out))
}

func (s *stub) requests() []recorded {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]recorded(nil), s.reqs...)
}

func replay(f fixture) func(int) (int, string, http.Header) {
	return func(int) (int, string, http.Header) { return f.Status, string(f.Body), nil }
}

const testSecret = "whsec_test"

func newTestClient(t *testing.T, s *stub, mutate ...func(*Config)) *Client {
	t.Helper()
	s.t = t
	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)
	cfg := Config{BaseURL: srv.URL, SecretKey: "sk_test_x", WebhookSecret: testSecret, Wait: func(context.Context, time.Duration) error { return nil }}
	for _, m := range mutate {
		m(&cfg)
	}
	c, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func decode(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("本文不是 JSON：%v\n%s", err, s)
	}
	return m
}

func TestCreateAccountSendsTheSpikeBodyForBothDashboards(t *testing.T) {
	for dash, fx := range map[Dashboard]string{DashboardExpress: "create_provider", DashboardNone: "create_platform"} {
		s := &stub{resp: replay(loadFixture(t, fx))}
		c := newTestClient(t, s)
		id, err := c.CreateAccount(context.Background(), "42", dash, "seller@example.com")
		if err != nil || id != loadFixture(t, fx).object(t)["id"] || !strings.HasPrefix(id, "acct_") {
			t.Fatalf("%s：id=%q err=%v", dash, id, err)
		}
		r := s.requests()[0]
		if r.Method != "POST" || r.URI != "/v2/core/accounts" || r.Idempotency != "42" || r.Authorization != "Bearer sk_test_x" || !strings.HasPrefix(r.ContentType, "application/json") {
			t.Errorf("%s：請求 %+v", dash, r)
		}
		got := decode(t, r.Body)
		want := decode(t, `{
			"identity": {"country": "us", "entity_type": "individual"},
			"configuration": {"recipient": {"capabilities": {"stripe_balance": {"stripe_transfers": {"requested": true}}}}},
			"defaults": {"responsibilities": {"fees_collector": "application", "losses_collector": "application"}},
			"dashboard": "`+string(dash)+`",
			"contact_email": "seller@example.com"}`)
		if !jsonEqual(got, want) {
			t.Errorf("%s：本文 %s", dash, r.Body)
		}
	}
}

func jsonEqual(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}

func TestIdempotencyConflictIsAnAPIErrorThatIsPermanent(t *testing.T) {
	s := &stub{resp: replay(loadFixture(t, "create_provider_idem_conflict"))}
	_, err := newTestClient(t, s).CreateAccount(context.Background(), "42", DashboardExpress, "seller@example.com")
	var api *APIError
	if !errors.As(err, &api) || api.Status != 409 || api.Code != "idempotency_error" || !api.Permanent() {
		t.Fatalf("err=%v", err)
	}
	if n := len(s.requests()); n != 1 {
		t.Fatalf("409 重試了：%d 次", n)
	}
}

func TestAPIErrorPermanence(t *testing.T) {
	for status, want := range map[int]bool{400: true, 401: true, 404: true, 408: false, 429: false, 500: false, 503: false} {
		if got := (&APIError{Status: status}).Permanent(); got != want {
			t.Errorf("%d：%v，want %v", status, got, want)
		}
	}
}

// 409 只有冪等鍵搭配不同參數（idempotency_error）是永久的；同一把鍵的另一個請求還在執行（https://docs.stripe.com/error-low-level）要重試。
func TestOnlyAnIdempotencyMismatchConflictIsPermanent(t *testing.T) {
	for code, want := range map[string]bool{"idempotency_error": true, "idempotency_key_in_use": false, "": false} {
		if got := (&APIError{Status: 409, Code: code}).Permanent(); got != want {
			t.Errorf("409 %q：%v，want %v", code, got, want)
		}
	}
}

func TestRetrieveAccountSendsIncludesAndKeepsOnlyTheOnboardingFields(t *testing.T) {
	fx := loadFixture(t, "retrieve_provider_new")
	s := &stub{resp: replay(fx)}
	a, err := newTestClient(t, s).RetrieveAccount(context.Background(), "acct_1UXO50EWUjgLkPE9")
	if err != nil {
		t.Fatal(err)
	}
	r := s.requests()[0]
	u, perr := url.Parse(r.URI)
	if perr != nil || r.Method != "GET" || u.Path != "/v2/core/accounts/acct_1UXO50EWUjgLkPE9" || r.Body != "" ||
		u.Query().Get("include[0]") != "configuration.recipient" || u.Query().Get("include[1]") != "requirements" || len(u.Query()) != 2 {
		t.Errorf("請求 %+v", r)
	}
	body := fx.object(t)
	bal := body["configuration"].(map[string]any)["recipient"].(map[string]any)["capabilities"].(map[string]any)["stripe_balance"].(map[string]any)
	wantTransfers := bal["stripe_transfers"].(map[string]any)
	wantPayouts := bal["payouts"].(map[string]any)
	if a.ID != body["id"] || a.Transfers.Status != wantTransfers["status"] || !jsonEqual(a.Transfers.Details, wantTransfers["status_details"]) ||
		a.Payouts.Status != wantPayouts["status"] || !jsonEqual(a.Payouts.Details, wantPayouts["status_details"]) {
		t.Errorf("帳戶 %+v", a)
	}
	if len(a.Transfers.Details) == 0 {
		t.Fatalf("fixture 沒有 status_details，細節沒被驗到")
	}
	if !jsonEqual(decode(t, string(a.Requirements)), body["requirements"]) {
		t.Errorf("requirements 不是回應本文裡的原始物件：%s", a.Requirements)
	}
	if _, ok := decode(t, string(a.Requirements))["entries"]; !ok {
		t.Errorf("requirements 缺 entries：%s", a.Requirements)
	}
}

func TestRetrieveAccountWithoutTheTransfersCapabilityIsAMalformedResponse(t *testing.T) {
	for name, body := range map[string]string{
		"recipient 是 null":    `{"id":"acct_1","object":"v2.core.account","configuration":{"recipient":null}}`,
		"沒有 stripe_transfers": `{"id":"acct_1","configuration":{"recipient":{"capabilities":{"stripe_balance":{"payouts":{"status":"active"}}}}}}`,
	} {
		s := &stub{resp: func(int) (int, string, http.Header) { return 200, body, nil }}
		a, err := newTestClient(t, s).RetrieveAccount(context.Background(), "acct_1")
		var api *APIError
		if err == nil || errors.As(err, &api) || a.ID != "" {
			t.Errorf("%s：a=%+v err=%v，預期一般錯誤", name, a, err)
		}
	}
}

func TestRetrieveNotFoundFixtureIsAPermanentAPIError(t *testing.T) {
	s := &stub{resp: replay(loadFixture(t, "retrieve_not_found"))}
	_, err := newTestClient(t, s).RetrieveAccount(context.Background(), "acct_doesnotexist000")
	var api *APIError
	if !errors.As(err, &api) || api.Status != 400 || !api.Permanent() {
		t.Fatalf("err=%v", err)
	}
}

func TestAccountSessionIsFormEncodedAndReturnsTheClientSecret(t *testing.T) {
	fx := loadFixture(t, "account_session_platform")
	s := &stub{resp: replay(fx)}
	sess, err := newTestClient(t, s).CreateAccountSession(context.Background(), "acct_1")
	if err != nil {
		t.Fatal(err)
	}
	r := s.requests()[0]
	if r.Method != "POST" || r.URI != "/v1/account_sessions" || r.ContentType != "application/x-www-form-urlencoded" {
		t.Errorf("請求 %+v", r)
	}
	form, perr := url.ParseQuery(r.Body)
	want := url.Values{
		"account": {"acct_1"},
		"components[account_onboarding][enabled]":                                      {"true"},
		"components[account_onboarding][features][disable_stripe_user_authentication]": {"true"},
	}
	if perr != nil || !jsonEqual(form, want) {
		t.Errorf("本文 %q", r.Body)
	}
	body := fx.object(t)
	if sess.ClientSecret != body["client_secret"] || sess.ExpiresAt == nil || sess.ExpiresAt.Unix() != int64(body["expires_at"].(float64)) || sess.ExpiresAt.Location() != time.UTC {
		t.Errorf("session %+v", sess)
	}
}

// 回應是 sandbox 錄下的 account_link_platform；請求形狀是這個測試要守的。
func TestAccountLinkRequestHasNoConfigurationsField(t *testing.T) {
	fx := loadFixture(t, "account_link_platform")
	s := &stub{resp: replay(fx)}
	link, err := newTestClient(t, s).CreateAccountLink(context.Background(), "acct_1", "https://app.test/return", "https://app.test/refresh")
	if err != nil {
		t.Fatal(err)
	}
	r := s.requests()[0]
	if r.Method != "POST" || r.URI != "/v2/core/account_links" || !strings.HasPrefix(r.ContentType, "application/json") {
		t.Errorf("請求 %+v", r)
	}
	want := decode(t, `{"account":"acct_1","use_case":{"type":"account_onboarding","account_onboarding":{"return_url":"https://app.test/return","refresh_url":"https://app.test/refresh"}}}`)
	if !jsonEqual(decode(t, r.Body), want) || strings.Contains(r.Body, "configurations") {
		t.Errorf("本文 %s", r.Body)
	}
	wantExpiry, terr := time.Parse(time.RFC3339, fx.object(t)["expires_at"].(string))
	if terr != nil || link.URL != fx.object(t)["url"] || link.ExpiresAt == nil || !link.ExpiresAt.Equal(wantExpiry) {
		t.Errorf("link %+v", link)
	}
}

func TestResponsesMissingRequiredFieldsAreErrorsNotValues(t *testing.T) {
	s := &stub{resp: func(int) (int, string, http.Header) { return 200, `{}`, nil }}
	c := newTestClient(t, s)
	ctx := context.Background()
	if id, err := c.CreateAccount(ctx, "1", DashboardNone, "seller@example.com"); err == nil || id != "" {
		t.Errorf("CreateAccount id=%q err=%v", id, err)
	}
	if l, err := c.CreateAccountLink(ctx, "a", "r", "f"); err == nil || l.URL != "" {
		t.Errorf("CreateAccountLink %+v err=%v", l, err)
	}
	if sess, err := c.CreateAccountSession(ctx, "a"); err == nil || sess.ClientSecret != "" {
		t.Errorf("CreateAccountSession %+v err=%v", sess, err)
	}
	if _, err := c.RetrieveAccount(ctx, "a"); err == nil {
		t.Error("RetrieveAccount 沒有 id 卻成功")
	}
}

func TestRateLimitIsRetriedInsideTheClientWithBackoffThenSucceeds(t *testing.T) {
	var waits []time.Duration
	s := &stub{resp: func(n int) (int, string, http.Header) {
		if n < 3 {
			return 429, `{"error":{"code":"rate_limit","message":"slow down"}}`, nil
		}
		return 200, `{"id":"acct_9"}`, nil
	}}
	c := newTestClient(t, s, func(cfg *Config) {
		cfg.Wait = func(_ context.Context, d time.Duration) error { waits = append(waits, d); return nil }
	})
	id, err := c.CreateAccount(context.Background(), "7", DashboardExpress, "seller@example.com")
	if err != nil || id != "acct_9" {
		t.Fatalf("id=%q err=%v", id, err)
	}
	reqs := s.requests()
	if len(reqs) != 3 || reqs[0].Idempotency != "7" || reqs[2].Idempotency != "7" || reqs[0].Body != reqs[2].Body {
		t.Errorf("重試的請求 %+v", reqs)
	}
	if len(waits) != 2 || waits[0] != baseBackoff || waits[1] != 2*baseBackoff {
		t.Errorf("退避 %v", waits)
	}
}

func TestRateLimitHonoursRetryAfterAndStopsAfterTheLastAttempt(t *testing.T) {
	var waits []time.Duration
	s := &stub{resp: func(int) (int, string, http.Header) {
		return 429, `{"error":{"code":"rate_limit","message":"x"}}`, http.Header{"Retry-After": {"2"}}
	}}
	c := newTestClient(t, s, func(cfg *Config) {
		cfg.Wait = func(_ context.Context, d time.Duration) error { waits = append(waits, d); return nil }
	})
	_, err := c.RetrieveAccount(context.Background(), "acct_1")
	var api *APIError
	if !errors.As(err, &api) || api.Status != 429 || api.Permanent() {
		t.Fatalf("err=%v", err)
	}
	if len(s.requests()) != maxAttempts || len(waits) != maxAttempts-1 || waits[0] != 2*time.Second {
		t.Errorf("請求 %d 次，退避 %v", len(s.requests()), waits)
	}
}

func TestServerErrorsAreNotRetriedInsideTheClientAndAreNotPermanent(t *testing.T) {
	s := &stub{resp: func(int) (int, string, http.Header) { return 503, `{"error":{"message":"down"}}`, nil }}
	_, err := newTestClient(t, s).RetrieveAccount(context.Background(), "acct_1")
	var api *APIError
	if !errors.As(err, &api) || api.Permanent() || len(s.requests()) != 1 {
		t.Fatalf("err=%v 請求=%d", err, len(s.requests()))
	}
}

// 帶 Stripe-Should-Retry: true 的 409 與 5xx 是 SDK 預設會自動重試的情況；這裡一律只送一次，重試交給呼叫方。
func TestSDKAutomaticRetriesAreDisabled(t *testing.T) {
	for name, resp := range map[string]func(int) (int, string, http.Header){
		"409 in use": func(int) (int, string, http.Header) { return 409, `{"error":{"code":"idempotency_key_in_use"}}`, nil },
		"500 should retry": func(int) (int, string, http.Header) {
			return 500, `{"error":{}}`, http.Header{"Stripe-Should-Retry": {"true"}}
		},
	} {
		s := &stub{resp: resp}
		_, err := newTestClient(t, s).CreateAccount(context.Background(), "k", DashboardNone, "a@example.com")
		var api *APIError
		if !errors.As(err, &api) || len(s.requests()) != 1 {
			t.Errorf("%s：err=%v 請求=%d", name, err, len(s.requests()))
		}
	}
}

func TestConnectionFailureIsNotRetriedAndIsNotAnAPIError(t *testing.T) {
	var mu sync.Mutex
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		hits++
		mu.Unlock()
		conn, _, err := w.(http.Hijacker).Hijack()
		if err == nil {
			_ = conn.Close()
		}
	}))
	t.Cleanup(srv.Close)
	c, err := New(Config{BaseURL: srv.URL, SecretKey: "sk_test_x", WebhookSecret: testSecret})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.RetrieveAccount(context.Background(), "acct_1")
	var api *APIError
	mu.Lock()
	defer mu.Unlock()
	if err == nil || errors.As(err, &api) || hits != 1 {
		t.Fatalf("err=%v 請求=%d", err, hits)
	}
}

func TestNoTelemetryHeaderIsSent(t *testing.T) {
	s := &stub{resp: func(int) (int, string, http.Header) { return 200, `{}`, nil }}
	c := newTestClient(t, s)
	for range 2 {
		_, _ = c.CreateAccountSession(context.Background(), "a")
	}
	reqs := s.requests()
	if len(reqs) != 2 {
		t.Fatalf("請求 %d", len(reqs))
	}
	for _, r := range reqs {
		if v := r.Header.Get("X-Stripe-Client-Telemetry"); v != "" {
			t.Errorf("送了遙測：%s", v)
		}
	}
}

func TestEveryCallRunsUnderTheConfiguredDeadline(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	t.Cleanup(func() { close(release); srv.Close() })
	c, err := New(Config{BaseURL: srv.URL, SecretKey: "sk_test_x", WebhookSecret: testSecret, Timeout: 20 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.RetrieveAccount(context.Background(), "acct_1")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err=%v，預期 DeadlineExceeded", err)
	}
	var api *APIError
	if errors.As(err, &api) {
		t.Fatal("逾時被當成業務錯誤")
	}
}

func TestNewRejectsMissingSecretsAndBadBaseURL(t *testing.T) {
	for name, cfg := range map[string]Config{
		"沒有金鑰":       {WebhookSecret: "w"},
		"沒有 webhook": {SecretKey: "k"},
		"網址不合":       {SecretKey: "k", WebhookSecret: "w", BaseURL: "api.stripe.com"},
	} {
		if _, err := New(cfg); err == nil {
			t.Errorf("%s：通過了", name)
		}
	}
}

// ListBankAccountFingerprints 帶 bank_account 篩選，回傳回應裡每個項目的 fingerprint（期望值從 fixture 讀，不寫死）。
// mutation：不帶 object=bank_account、或回傳 last4 → 本測試失敗。
func TestListBankAccountFingerprintsReadsTheFingerprintFieldOnTheV1Version(t *testing.T) {
	fx := loadFixture(t, "external_accounts_bank")
	var list struct {
		Data []struct {
			Fingerprint string `json:"fingerprint"`
		} `json:"data"`
	}
	if err := json.Unmarshal(fx.Body, &list); err != nil || len(list.Data) == 0 || list.Data[0].Fingerprint == "" {
		t.Fatalf("fixture 沒有 fingerprint：%v", err)
	}
	s := &stub{resp: replay(fx)}
	got, err := newTestClient(t, s).ListBankAccountFingerprints(context.Background(), "acct_1")
	if err != nil || len(got) != len(list.Data) || got[0] != list.Data[0].Fingerprint {
		t.Fatalf("got=%v err=%v", got, err)
	}
	r := s.requests()
	if len(r) != 1 || r[0].Method != "GET" || r[0].Body != "" ||
		r[0].URI != "/v1/accounts/acct_1/external_accounts?limit=100&object=bank_account" {
		t.Errorf("請求 %+v", r)
	}
}

// 翻頁：has_more 為真時以最後一項的 id 當 starting_after 讀下一頁，沒有 fingerprint 的項目略過、不當成空字串值回傳。
// mutation：不翻頁 → 少一個值；不略過空 fingerprint → 回傳含空字串。
func TestListBankAccountFingerprintsFollowsPagesAndSkipsEntriesWithoutAFingerprint(t *testing.T) {
	s := &stub{resp: func(n int) (int, string, http.Header) {
		if n == 1 {
			return 200, `{"object":"list","has_more":true,"data":[{"id":"ba_1","fingerprint":"fp_a"},{"id":"ba_2","fingerprint":""}]}`, nil
		}
		return 200, `{"object":"list","has_more":false,"data":[{"id":"ba_3","fingerprint":"fp_c"},{"id":"ba_4"}]}`, nil
	}}
	got, err := newTestClient(t, s).ListBankAccountFingerprints(context.Background(), "acct_1")
	if err != nil || !jsonEqual(got, []string{"fp_a", "fp_c"}) {
		t.Fatalf("got=%v err=%v", got, err)
	}
	r := s.requests()
	if len(r) != 2 || !strings.Contains(r[1].URI, "starting_after=ba_2") || strings.Contains(r[0].URI, "starting_after") {
		t.Errorf("請求 %+v", r)
	}
}

func TestListBankAccountFingerprintsEmptyListIsEmptyNotAnError(t *testing.T) {
	s := &stub{resp: func(int) (int, string, http.Header) { return 200, `{"object":"list","has_more":false,"data":[]}`, nil }}
	got, err := newTestClient(t, s).ListBankAccountFingerprints(context.Background(), "acct_1")
	if err != nil || len(got) != 0 {
		t.Fatalf("got=%v err=%v", got, err)
	}
}

// 錯誤不被吞掉：5xx 回 APIError（非永久）、has_more 但沒有可接續的項目是錯誤而不是無限翻頁。
func TestListBankAccountFingerprintsPropagatesErrorsAndRejectsAnUnfollowablePage(t *testing.T) {
	s := &stub{resp: func(int) (int, string, http.Header) { return 503, `{"error":{"message":"down"}}`, nil }}
	_, err := newTestClient(t, s).ListBankAccountFingerprints(context.Background(), "acct_1")
	var api *APIError
	if !errors.As(err, &api) || api.Permanent() {
		t.Fatalf("err=%v", err)
	}
	s = &stub{resp: func(int) (int, string, http.Header) { return 200, `{"has_more":true,"data":[]}`, nil }}
	if _, err := newTestClient(t, s).ListBankAccountFingerprints(context.Background(), "acct_1"); err == nil || len(s.requests()) != 1 {
		t.Fatalf("err=%v 請求=%d", err, len(s.requests()))
	}
}

func closeFixtureBody(closed bool) string {
	if closed {
		return `{"id":"acct_1","object":"v2.core.account","closed":true,"applied_configurations":["recipient"]}`
	}
	return `{"id":"acct_1","object":"v2.core.account","closed":false,"applied_configurations":["recipient"]}`
}

const forbiddenBody = `{"error":{"code":"forbidden","message":"closed"}}`

func TestCloseAccountPostsTheRecipientConfiguration(t *testing.T) {
	s := &stub{resp: func(int) (int, string, http.Header) { return 200, closeFixtureBody(true), nil }}
	if err := newTestClient(t, s).CloseAccount(context.Background(), "acct_1"); err != nil {
		t.Fatal(err)
	}
	reqs := s.requests()
	if len(reqs) != 1 || reqs[0].Method != "POST" || reqs[0].URI != "/v2/core/accounts/acct_1/close" || !strings.HasPrefix(reqs[0].ContentType, "application/json") ||
		!jsonEqual(decode(t, reqs[0].Body), map[string]any{"applied_configurations": []any{"recipient"}}) {
		t.Fatalf("請求 %+v", reqs)
	}
}

// 已關閉的帳戶再關回 403：讀帳戶，closed 為真才算成功。
func TestCloseAccountOnAnAlreadyClosedAccountSucceedsOnlyWhenTheAccountIsClosed(t *testing.T) {
	s := &stub{resp: func(n int) (int, string, http.Header) {
		if n == 1 {
			return 403, forbiddenBody, nil
		}
		return 200, closeFixtureBody(true), nil
	}}
	if err := newTestClient(t, s).CloseAccount(context.Background(), "acct_1"); err != nil {
		t.Fatalf("err=%v", err)
	}
	reqs := s.requests()
	if len(reqs) != 2 || reqs[1].Method != "GET" || !strings.HasPrefix(reqs[1].URI, "/v2/core/accounts/acct_1") || strings.HasSuffix(reqs[1].URI, "/close") {
		t.Fatalf("請求 %+v", reqs)
	}
}

func TestCloseAccountForbiddenOnAnOpenAccountReturnsTheOriginalError(t *testing.T) {
	for name, second := range map[string]func() (int, string){
		"帳戶沒關":        func() (int, string) { return 200, closeFixtureBody(false) },
		"回應沒有 closed": func() (int, string) { return 200, `{"id":"acct_1"}` },
	} {
		s := &stub{resp: func(n int) (int, string, http.Header) {
			if n == 1 {
				return 403, forbiddenBody, nil
			}
			status, body := second()
			return status, body, nil
		}}
		err := newTestClient(t, s).CloseAccount(context.Background(), "acct_1")
		var api *APIError
		if !errors.As(err, &api) || api.Status != 403 || api.Code != "forbidden" || len(s.requests()) != 2 {
			t.Errorf("%s：err=%v 請求=%d", name, err, len(s.requests()))
		}
	}
}

// 403 之後讀帳戶本身失敗：不知道帳戶是不是已關閉，回讀取的錯誤（暫時性的讀取失敗由呼叫方重試），不回永久的 403。
func TestCloseAccountForbiddenThenFailedLookupReturnsTheLookupError(t *testing.T) {
	s := &stub{resp: func(n int) (int, string, http.Header) {
		if n == 1 {
			return 403, forbiddenBody, nil
		}
		return 503, `{"error":{"message":"down"}}`, nil
	}}
	err := newTestClient(t, s).CloseAccount(context.Background(), "acct_1")
	var api *APIError
	if !errors.As(err, &api) || api.Status != 503 || api.Permanent() || len(s.requests()) != 2 {
		t.Fatalf("err=%v 請求=%d，預期讀取的 503 且不是永久錯誤", err, len(s.requests()))
	}
}

func TestCloseAccountUnknownIDIsAPermanentErrorWithoutALookup(t *testing.T) {
	s := &stub{resp: replay(loadFixture(t, "retrieve_not_found"))}
	err := newTestClient(t, s).CloseAccount(context.Background(), "acct_doesnotexist000")
	var api *APIError
	if !errors.As(err, &api) || api.Status != 400 || !api.Permanent() || len(s.requests()) != 1 {
		t.Fatalf("err=%v 請求=%d", err, len(s.requests()))
	}
}
