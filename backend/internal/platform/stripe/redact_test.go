package stripe

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func capture() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	return slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})), &buf
}

func mustNotContain(t *testing.T, logs string, values ...string) {
	t.Helper()
	for _, v := range values {
		if strings.Contains(logs, v) {
			t.Errorf("記錄含遮罩前的值 %q：\n%s", v, logs)
		}
	}
}

func mustContain(t *testing.T, logs string, values ...string) {
	t.Helper()
	for _, v := range values {
		if !strings.Contains(logs, v) {
			t.Errorf("記錄缺 %q：\n%s", v, logs)
		}
	}
}

func roundTrip(t *testing.T, method, path, contentType, body string, respStatus int, respBody string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Request-Id", "req_ABC123")
		w.WriteHeader(respStatus)
		_, _ = w.Write([]byte(respBody))
	}))
	t.Cleanup(srv.Close)
	lg, buf := capture()
	hc := &http.Client{Transport: recorder{next: http.DefaultTransport, log: lg}}
	req, err := http.NewRequestWithContext(context.Background(), method, srv.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer sk_test_SECRETKEY123")
	req.Header.Set("Idempotency-Key", "idem-key-77")
	resp, err := hc.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	return buf.String()
}

func TestRecorderLogsTheRequestSummaryAndNeverTheCredential(t *testing.T) {
	logs := roundTrip(t, "POST", "/v1/account_sessions", "application/x-www-form-urlencoded", "account=acct_1&components[account_onboarding][enabled]=true", 200, `{"id":"acct_1"}`)
	mustContain(t, logs, "method=POST", "path=/v1/account_sessions", "status=200", "request_id=req_ABC123", "idempotency_key=idem-key-77", "duration=", "account=acct_1", `acct_1`)
	mustNotContain(t, logs, "sk_test_SECRETKEY123", "SECRETKEY123", "Authorization", "Bearer")
}

func TestSecretKeyAndWebhookSecretNeverAppearEvenInsideFreeText(t *testing.T) {
	t.Run("secret key", func(t *testing.T) {
		logs := roundTrip(t, "POST", "/v1/x", "application/x-www-form-urlencoded", "metadata[note]=used sk_live_ABCDEF123456 and rk_test_ZZZ999", 400,
			`{"error":{"message":"Invalid API Key provided: sk_live_ABCDEF123456"}}`)
		mustNotContain(t, logs, "sk_live_ABCDEF123456", "rk_test_ZZZ999")
	})
	t.Run("webhook secret", func(t *testing.T) {
		logs := roundTrip(t, "POST", "/v1/x", "application/x-www-form-urlencoded", "metadata[note]=whsec_WEBHOOKSECRET42", 200, `{"note":"whsec_WEBHOOKSECRET42"}`)
		mustNotContain(t, logs, "whsec_WEBHOOKSECRET42")
	})
	t.Run("client secret value", func(t *testing.T) {
		logs := roundTrip(t, "POST", "/v1/x", "application/json", `{"metadata":{"note":"pi_3Abc_secret_XyZ987"}}`, 200, `{"metadata":{"note":"pi_3Abc_secret_XyZ987"}}`)
		mustNotContain(t, logs, "pi_3Abc_secret_XyZ987", "XyZ987")
	})
}

func TestPatternsRedactValuesUnderInnocuousKeys(t *testing.T) {
	t.Run("pan", func(t *testing.T) {
		logs := roundTrip(t, "POST", "/v1/x", "application/x-www-form-urlencoded", "metadata[note]=4242 4242 4242 4242 and 4000-0566-5566-5556", 200, `{"note":"5555555555554444"}`)
		mustNotContain(t, logs, "4242 4242 4242 4242", "4242", "4000-0566", "5555555555554444")
	})
	t.Run("email", func(t *testing.T) {
		logs := roundTrip(t, "POST", "/v1/x", "application/json", `{"metadata":{"note":"mail buyer@example.com"}}`, 200, `{"note":"seller@example.org"}`)
		mustNotContain(t, logs, "buyer@example.com", "seller@example.org", "example.com")
	})
	t.Run("phone", func(t *testing.T) {
		logs := roundTrip(t, "POST", "/v1/x", "application/json", `{"metadata":{"note":"call +886 912 345 678"}}`, 200, `{"note":"+1 (415) 555-0132"}`)
		mustNotContain(t, logs, "912 345 678", "415) 555-0132")
	})
}

var expectedDeniedNames = []string{
	"authorization", "secret", "secret_key", "api_key", "token", "password", "url",
	"card", "number", "cvc", "cvv", "exp_month", "exp_year", "last4", "bank_account", "iban", "account_number", "routing_number",
	"email", "phone",
	"address", "line1", "line2", "postal_code", "zip", "city", "state", "town",
	"name", "surname",
	"individual", "person", "billing_details", "shipping", "attestations",
	"dob", "id_number", "ssn_last_4", "tax_id", "ip", "user_agent",
}

// 每個遮罩名稱，在表單（巢狀路徑）與 JSON（巢狀物件）都要遮掉值；值刻意不符合任何值比對 pattern。
func TestEveryDeniedFieldNameIsRedactedInFormAndJSON(t *testing.T) {
	for _, k := range expectedDeniedNames {
		t.Run(k, func(t *testing.T) {
			val := "plainvalue-" + k
			formLogs := roundTrip(t, "POST", "/v1/x", "application/x-www-form-urlencoded", url.QueryEscape("outer["+k+"][inner]")+"="+val, 200, `{}`)
			mustNotContain(t, formLogs, val)
			jsonLogs := roundTrip(t, "POST", "/v1/x", "application/json", `{"outer":{"`+k+`":{"inner":"`+val+`"}}}`, 200, `{"outer":[{"`+k+`":"`+val+`"}]}`)
			mustNotContain(t, jsonLogs, val)
			mustContain(t, jsonLogs, redacted)
		})
	}
}

// production 的表新增名稱時，要同步補進 expectedDeniedNames。
func TestTheDenyTablesAndTheExpectedNamesAgree(t *testing.T) {
	want := map[string]bool{}
	for _, k := range expectedDeniedNames {
		want[k] = true
	}
	got := map[string]bool{}
	for _, list := range [][]string{credentialKeys, cardKeys, contactKeys, addressKeys, nameKeys, containerKeys, identityKeys} {
		for _, k := range list {
			got[k] = true
		}
	}
	for k := range got {
		if !want[k] {
			t.Errorf("遮罩表有 %q，測試的 expectedDeniedNames 沒有", k)
		}
	}
	for k := range want {
		if !got[k] {
			t.Errorf("expectedDeniedNames 有 %q，遮罩表沒有", k)
		}
	}
}

func TestSensitiveNameSuffixesAreRedacted(t *testing.T) {
	for _, k := range []string{"client_secret", "contact_email", "receipt_email", "mobile_phone", "payment_intent_client_secret", "legal_name", "first_name", "last_name", "given_name", "full_name", "display_name", "billing_address", "access_token"} {
		t.Run(k, func(t *testing.T) {
			val := "plainvalue-" + k
			logs := roundTrip(t, "POST", "/v1/x", "application/json", `{"`+k+`":"`+val+`"}`, 200, `{"`+k+`":"`+val+`"}`)
			mustNotContain(t, logs, val)
		})
	}
}

func TestUnknownFieldsInsideAPIIdentityContainerAreRedactedWithoutBeingListed(t *testing.T) {
	logs := roundTrip(t, "POST", "/v2/core/accounts/acct_1", "application/json",
		`{"identity":{"individual":{"some_new_field":"mystery-pii","nested":{"deep":"mystery-deep"}}},"metadata":{"store_id":"42"}}`,
		200, `{"shipping":{"carrier":"mystery-carrier"},"billing_details":{"whatever":"mystery-bd"}}`)
	mustNotContain(t, logs, "mystery-pii", "mystery-deep", "mystery-carrier", "mystery-bd")
	mustContain(t, logs, "store_id", "42")
}

func TestNonRedactableBodiesAreOmittedNotLogged(t *testing.T) {
	logs := roundTrip(t, "POST", "/v1/x", "application/json", `not json buyer 4242424242424242`, 500, `<html>oops buyer@example.com</html>`)
	mustNotContain(t, logs, "4242424242424242", "buyer@example.com", "oops")
	mustContain(t, logs, "body omitted", "status=500")
}

func TestUnknownContentTypesAreOmittedNotLogged(t *testing.T) {
	logs := roundTrip(t, "POST", "/v1/x", "text/plain", "buyer 4242424242424242 plaintext-body", 200, `{}`)
	mustNotContain(t, logs, "4242424242424242", "plaintext-body")
	mustContain(t, logs, "body omitted")
}

func TestNonSecretIdentifiersAreKept(t *testing.T) {
	logs := roundTrip(t, "POST", "/v1/transfers", "application/x-www-form-urlencoded", "amount=1200&currency=usd&destination=acct_1Abc&metadata[store_id]=42", 200,
		`{"id":"tr_1Abc","object":"transfer","amount":1200,"created":1767225600}`)
	mustContain(t, logs, "amount=1200", "currency=usd", "destination=acct_1Abc", "metadata[store_id]=42", "tr_1Abc", "1767225600")
}

func TestLongBodiesAreTruncated(t *testing.T) {
	logs := roundTrip(t, "POST", "/v1/x", "application/json", `{"a":"`+strings.Repeat("x", 3*maxLogBodyBytes)+`"}`, 200, `{}`)
	mustContain(t, logs, "[truncated]")
	if len(logs) > 3*maxLogBodyBytes {
		t.Errorf("記錄 %d bytes，沒有截斷", len(logs))
	}
}

// 經完整 client：v2 JSON 請求的聯絡信箱、回應的 client_secret 與 Authorization 都不進記錄；冪等鍵與 Request-Id 在。
func TestClientCallsLogRedactedSummariesEndToEnd(t *testing.T) {
	clientSecret := "accss_secret__" + "AbCdEf123456"
	lg, buf := capture()
	s := &stub{resp: func(int) (int, string, http.Header) {
		return 200, `{"client_secret":"` + clientSecret + `","expires_at":1767225600,"livemode":false}`, http.Header{"Request-Id": {"req_E2E"}, "Content-Type": {"application/json"}}
	}}
	c := newTestClient(t, s, func(cfg *Config) { cfg.SecretKey = "sk_test_E2ESECRET"; cfg.Logger = lg })
	if _, err := c.CreateAccountSession(context.Background(), "acct_1"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.CreateAccountShell(context.Background(), "idem-shell-1", 7); err != nil {
		t.Log(err)
	}
	if err := c.UpdateAccount(context.Background(), "acct_1", DashboardNone, "owner@example.com"); err != nil {
		t.Log(err)
	}
	logs := buf.String()
	mustNotContain(t, logs, "sk_test_E2ESECRET", clientSecret, "AbCdEf123456", "owner@example.com")
	mustContain(t, logs, "method=POST", "path=/v1/account_sessions", "status=200", "request_id=req_E2E", "idempotency_key=idem-shell-1", "acct_1")
}

func TestRetriesLogEachAttempt(t *testing.T) {
	lg, buf := capture()
	s := &stub{resp: func(n int) (int, string, http.Header) {
		if n == 1 {
			return 429, `{"error":{"code":"rate_limit","message":"slow down"}}`, http.Header{"Retry-After": {"0"}, "Request-Id": {"req_R1"}, "Content-Type": {"application/json"}}
		}
		return 200, `{"client_secret":"accss_secret__x","expires_at":1}`, http.Header{"Request-Id": {"req_R2"}, "Content-Type": {"application/json"}}
	}}
	c := newTestClient(t, s, func(cfg *Config) { cfg.Logger = lg })
	if _, err := c.CreateAccountSession(context.Background(), "acct_1"); err != nil {
		t.Fatal(err)
	}
	mustContain(t, buf.String(), "status=429", "request_id=req_R1", "status=200", "request_id=req_R2")
}

func TestTransportErrorsAreLoggedWithoutLeakingTheRequestBody(t *testing.T) {
	lg, buf := capture()
	hc := &http.Client{Transport: recorder{next: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("dial failed for sk_test_LEAKED99")
	}), log: lg}}
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "http://stripe.invalid/v1/x", strings.NewReader("metadata[note]=a"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := hc.Do(req)
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("預期錯誤")
	}
	mustContain(t, buf.String(), "stripe request failed", "method=POST", "dial failed")
	mustNotContain(t, buf.String(), "sk_test_LEAKED99")
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestNilLoggerIsSilent(t *testing.T) {
	s := &stub{resp: replay(loadFixture(t, "account_session_platform"))}
	c := newTestClient(t, s)
	if _, err := c.CreateAccountSession(context.Background(), "acct_1"); err != nil {
		t.Fatal(err)
	}
}

func TestQueryStringIsRedactedByKey(t *testing.T) {
	logs := roundTrip(t, "GET", "/v1/x?email=plain-mail-value&client_secret=plain-cs-value&limit=3", "", "", 200, `{}`)
	mustNotContain(t, logs, "plain-mail-value", "plain-cs-value")
	mustContain(t, logs, "limit=3")
}

func TestAttestationContainerAndIPFieldsAreRedacted(t *testing.T) {
	logs := roundTrip(t, "POST", "/v2/core/accounts/acct_1", "application/json",
		`{"identity":{"attestations":{"terms_of_service":{"account":{"ip":"plain-ip-value","user_agent":"plain-ua-value","date":"plain-date-value"}}}}}`,
		200, `{"x":{"ip":"plain-ip-resp","user_agent":"plain-ua-resp"}}`)
	mustNotContain(t, logs, "plain-ip-value", "plain-ua-value", "plain-date-value", "plain-ip-resp", "plain-ua-resp")
}

func TestJSONObjectKeysAreScrubbed(t *testing.T) {
	logs := roundTrip(t, "POST", "/v1/x", "application/json", `{"owner@example.com":"v","sk_test_KEYASNAME":"v"}`, 200, `{}`)
	mustNotContain(t, logs, "owner@example.com", "sk_test_KEYASNAME")
}

func TestRequestWithoutGetBodyIsSentIntactAndBodyIsNotLogged(t *testing.T) {
	lg, buf := capture()
	var sent []byte
	hc := &http.Client{Transport: recorder{next: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		sent, _ = io.ReadAll(r.Body)
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("{}"))}, nil
	}), log: lg}}
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "http://stripe.invalid/v1/x", io.NopCloser(strings.NewReader("note=plain-note-value")))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if req.GetBody != nil {
		t.Fatal("測試前提：GetBody 應為 nil")
	}
	resp, err := hc.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if string(sent) != "note=plain-note-value" {
		t.Fatalf("送出的本文被改動：%q", sent)
	}
	mustContain(t, buf.String(), "request_body=[unavailable]")
	mustNotContain(t, buf.String(), "plain-note-value")
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("boom sk_test_READFAIL") }

func TestResponseReadFailureAndOversizeAreLogged(t *testing.T) {
	for name, body := range map[string]io.Reader{
		"read-error": failingReader{},
		"oversize":   strings.NewReader(strings.Repeat("a", maxResponseBytes+1)),
	} {
		t.Run(name, func(t *testing.T) {
			lg, buf := capture()
			hc := &http.Client{Transport: recorder{next: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 502, Header: http.Header{}, Body: io.NopCloser(body)}, nil
			}), log: lg}}
			req, _ := http.NewRequestWithContext(context.Background(), "GET", "http://stripe.invalid/v1/x", nil)
			req.Header.Set("Idempotency-Key", "idem-rf")
			if resp, err := hc.Do(req); err == nil {
				_ = resp.Body.Close()
				t.Fatal("預期錯誤")
			}
			mustContain(t, buf.String(), "stripe response unreadable", "status=502", "method=GET", "idempotency_key=idem-rf")
			mustNotContain(t, buf.String(), "sk_test_READFAIL")
		})
	}
}
