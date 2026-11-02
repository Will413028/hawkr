package stripe

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
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

type recorded struct {
	Method, URI, ContentType, Version, Idempotency, User string
	Body                                                 string
}

type stub struct {
	mu   sync.Mutex
	reqs []recorded
	resp func(n int) (int, string, http.Header)
}

func (s *stub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	user, _, _ := r.BasicAuth()
	s.mu.Lock()
	s.reqs = append(s.reqs, recorded{r.Method, r.URL.RequestURI(), r.Header.Get("Content-Type"), r.Header.Get("Stripe-Version"), r.Header.Get("Idempotency-Key"), user, string(body)})
	n := len(s.reqs)
	s.mu.Unlock()
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
		id, err := c.CreateAccount(context.Background(), "42", dash)
		if err != nil || !strings.HasPrefix(id, "acct_") {
			t.Fatalf("%s：id=%q err=%v", dash, id, err)
		}
		r := s.requests()[0]
		if r.Method != "POST" || r.URI != "/v2/core/accounts" || r.Version != APIVersion || r.Idempotency != "42" || r.User != "sk_test_x" || r.ContentType != "application/json" {
			t.Errorf("%s：請求 %+v", dash, r)
		}
		got := decode(t, r.Body)
		want := decode(t, `{
			"identity": {"country": "us", "entity_type": "individual"},
			"configuration": {"recipient": {"capabilities": {"stripe_balance": {"stripe_transfers": {"requested": true}}}}},
			"defaults": {"responsibilities": {"fees_collector": "application", "losses_collector": "application"}},
			"dashboard": "`+string(dash)+`"}`)
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
	_, err := newTestClient(t, s).CreateAccount(context.Background(), "42", DashboardExpress)
	var api *APIError
	if !errors.As(err, &api) || api.Status != 409 || api.Code != "idempotency_error" || !api.Permanent() {
		t.Fatalf("err=%v", err)
	}
	if n := len(s.requests()); n != 1 {
		t.Fatalf("409 重試了：%d 次", n)
	}
}

func TestAPIErrorPermanence(t *testing.T) {
	for status, want := range map[int]bool{400: true, 401: true, 404: true, 409: true, 408: false, 429: false, 500: false, 503: false} {
		if got := (&APIError{Status: status}).Permanent(); got != want {
			t.Errorf("%d：%v，want %v", status, got, want)
		}
	}
}

func TestRetrieveAccountSendsIncludesAndKeepsOnlyTheOnboardingFields(t *testing.T) {
	s := &stub{resp: replay(loadFixture(t, "retrieve_provider_new"))}
	a, err := newTestClient(t, s).RetrieveAccount(context.Background(), "acct_1UXO50EWUjgLkPE9")
	if err != nil {
		t.Fatal(err)
	}
	r := s.requests()[0]
	if r.Method != "GET" || r.URI != "/v2/core/accounts/acct_1UXO50EWUjgLkPE9?include=configuration.recipient&include=requirements" || r.Version != APIVersion || r.Body != "" {
		t.Errorf("請求 %+v", r)
	}
	wantDetails := []StatusDetail{{Code: "requirements_past_due", Resolution: "provide_info"}}
	if !a.Transfers.Present || a.Transfers.Status != "restricted" || !jsonEqual(a.Transfers.Details, wantDetails) || !a.Payouts.Present || a.Payouts.Status != "restricted" {
		t.Errorf("能力 %+v / %+v", a.Transfers, a.Payouts)
	}
	req := decode(t, string(a.Requirements))
	if _, ok := req["entries"]; !ok {
		t.Errorf("requirements 不是原始物件：%s", a.Requirements)
	}
}

func TestRetrieveAccountWithoutRecipientConfigurationHasNoCapabilities(t *testing.T) {
	s := &stub{resp: func(int) (int, string, http.Header) {
		return 200, `{"id":"acct_1","object":"v2.core.account","configuration":{"recipient":null}}`, nil
	}}
	a, err := newTestClient(t, s).RetrieveAccount(context.Background(), "acct_1")
	if err != nil || a.Transfers.Present || a.Payouts.Present || a.Requirements != nil {
		t.Fatalf("a=%+v err=%v", a, err)
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
	s := &stub{resp: replay(loadFixture(t, "account_session_platform"))}
	sess, err := newTestClient(t, s).CreateAccountSession(context.Background(), "acct_1")
	if err != nil {
		t.Fatal(err)
	}
	r := s.requests()[0]
	if r.Method != "POST" || r.URI != "/v1/account_sessions" || r.ContentType != "application/x-www-form-urlencoded" || r.Version != APIVersion {
		t.Errorf("請求 %+v", r)
	}
	if r.Body != "account=acct_1&components%5Baccount_onboarding%5D%5Benabled%5D=true" {
		t.Errorf("本文 %q", r.Body)
	}
	if sess.ClientSecret != "<CLIENT-SECRET-MASKED>" || sess.ExpiresAt == nil || sess.ExpiresAt.Unix() != 1793636969 {
		t.Errorf("session %+v", sess)
	}
}

// 合成的回應（sandbox 尚未錄製）：請求形狀是這個測試要守的。
func TestAccountLinkRequestHasNoConfigurationsField(t *testing.T) {
	s := &stub{resp: func(int) (int, string, http.Header) {
		return 200, `{"object":"v2.core.account_link","url":"https://connect.stripe.com/x","expires_at":"2026-11-04T01:02:03Z"}`, nil
	}}
	link, err := newTestClient(t, s).CreateAccountLink(context.Background(), "acct_1", "https://app.test/return", "https://app.test/refresh")
	if err != nil {
		t.Fatal(err)
	}
	r := s.requests()[0]
	if r.Method != "POST" || r.URI != "/v2/core/account_links" || r.Version != APIVersion || r.ContentType != "application/json" || r.Idempotency != "" {
		t.Errorf("請求 %+v", r)
	}
	want := decode(t, `{"account":"acct_1","use_case":{"type":"account_onboarding","account_onboarding":{"return_url":"https://app.test/return","refresh_url":"https://app.test/refresh"}}}`)
	if !jsonEqual(decode(t, r.Body), want) || strings.Contains(r.Body, "configurations") {
		t.Errorf("本文 %s", r.Body)
	}
	if link.URL != "https://connect.stripe.com/x" || link.ExpiresAt == nil || !link.ExpiresAt.Equal(time.Date(2026, 11, 4, 1, 2, 3, 0, time.UTC)) {
		t.Errorf("link %+v", link)
	}
}

func TestResponsesMissingRequiredFieldsAreErrorsNotValues(t *testing.T) {
	s := &stub{resp: func(int) (int, string, http.Header) { return 200, `{}`, nil }}
	c := newTestClient(t, s)
	ctx := context.Background()
	if id, err := c.CreateAccount(ctx, "1", DashboardNone); err == nil || id != "" {
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
	id, err := c.CreateAccount(context.Background(), "7", DashboardExpress)
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
