package clerk

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/lestrrat-go/jwx/v3/jwa"
	jwxjwt "github.com/lestrrat-go/jwx/v3/jwt"
)

const testHost = "example-app-1.clerk.accounts.dev"

func testPublishableKey() string {
	return "pk_test_" + base64.StdEncoding.EncodeToString([]byte(testHost+"$"))
}

// fakeClerk 是本機的 frontend API（JWKS）與 Backend API。
type fakeClerk struct {
	key *rsa.PrivateKey
	api http.HandlerFunc
}

func newFakeClerk(t *testing.T) *fakeClerk {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return &fakeClerk{key: key}
}

// client 建立指向本機 server 的 Client：抓公鑰的請求由 transport 轉到本機，Backend API 以 APIURL 指過去。
func (f *fakeClerk) client(t *testing.T, parties ...string) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if f.api == nil {
			http.NotFound(w, r)
			return
		}
		f.api(w, r)
	}))
	t.Cleanup(srv.Close)
	hc := &http.Client{Transport: rewriteTo(srv.URL)}
	if len(parties) == 0 {
		parties = []string{"https://web.example"}
	}
	der, err := x509.MarshalPKIXPublicKey(&f.key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
	c, err := New(Config{PublishableKey: testPublishableKey(), SecretKey: "sk_test_x", JWTKey: keyPEM, AuthorizedParties: parties, APIURL: srv.URL, HTTPClient: hc})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

type rewriteTo string

func (base rewriteTo) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.URL.Scheme, r.URL.Host = "http", strings.TrimPrefix(string(base), "http://")
	return http.DefaultTransport.RoundTrip(r)
}

func (f *fakeClerk) sign(t *testing.T, c jwt.Claims, extra map[string]any) string {
	t.Helper()
	s, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: f.key}, (&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "k1"))
	if err != nil {
		t.Fatal(err)
	}
	b := jwt.Signed(s).Claims(c)
	if extra != nil {
		b = b.Claims(extra)
	}
	tok, err := b.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func sessionClaims(sub string) jwt.Claims {
	now := time.Now()
	return jwt.Claims{
		Issuer: "https://" + testHost, Subject: sub,
		IssuedAt: jwt.NewNumericDate(now), NotBefore: jwt.NewNumericDate(now.Add(-5 * time.Second)), Expiry: jwt.NewNumericDate(now.Add(time.Minute)),
	}
}

func TestFrontendAPIIsDecodedFromThePublishableKey(t *testing.T) {
	got, err := FrontendAPI(testPublishableKey())
	if err != nil || got != "https://"+testHost {
		t.Fatalf("FrontendAPI = %q, %v", got, err)
	}
	for _, bad := range []string{"", "sk_test_abc", "pk_test_%%%", "pk_test_" + base64.StdEncoding.EncodeToString([]byte("no-dollar"))} {
		if _, err := FrontendAPI(bad); err == nil {
			t.Errorf("FrontendAPI(%q) 應該失敗", bad)
		}
	}
}

func TestSessionTokensAreVerifiedLocally(t *testing.T) {
	f := newFakeClerk(t)
	c := f.client(t, "https://web.example")
	ctx := context.Background()

	sub, err := c.Verify(ctx, f.sign(t, sessionClaims("user_1"), nil))
	if err != nil || sub.Subject != "user_1" {
		t.Fatalf("Verify = %+v, %v", sub, err)
	}
	if sub, err := c.Verify(ctx, f.sign(t, sessionClaims("user_1"), map[string]any{"azp": "https://web.example"})); err != nil || sub.Subject != "user_1" {
		t.Fatalf("允許的 azp：Verify = %+v, %v", sub, err)
	}

	wrongIssuer := sessionClaims("user_1")
	wrongIssuer.Issuer = "https://other.clerk.accounts.dev"
	expired := sessionClaims("user_1")
	expired.Expiry = jwt.NewNumericDate(time.Now().Add(-time.Minute))
	for name, tok := range map[string]string{
		"issuer 不符":        f.sign(t, wrongIssuer, nil),
		"過期":               f.sign(t, expired, nil),
		"azp 不是允許的 origin": f.sign(t, sessionClaims("user_1"), map[string]any{"azp": "https://evil.example"}),
		"格式不對":             "not-a-jwt",
	} {
		if _, err := c.Verify(ctx, tok); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s：Verify 的錯誤 = %v，要是 ErrInvalid", name, err)
		}
	}
}

func TestBackendAPIErrorsAreClassified(t *testing.T) {
	f := newFakeClerk(t)
	status := 0
	f.api = func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"errors":[{"code":"some_code","message":"含使用者資料的訊息"}]}`))
	}
	c := f.client(t)
	ctx := context.Background()
	for _, tc := range []struct {
		status   int
		wantNil  bool
		rejected bool
	}{
		{http.StatusOK, true, false},
		{http.StatusNotFound, true, false}, // 已經不存在：刪除算成功
		{http.StatusUnprocessableEntity, false, true},
		{http.StatusTooManyRequests, false, false},
		{http.StatusInternalServerError, false, false},
	} {
		status = tc.status
		for name, call := range map[string]func() error{
			"DeleteUser": func() error { return c.DeleteUser(ctx, "user_1") },
		} {
			err := call()
			if (err == nil) != tc.wantNil || errors.Is(err, ErrRejected) != tc.rejected {
				t.Errorf("%s 在 %d：err = %v", name, tc.status, err)
			}
			if err != nil && strings.Contains(err.Error(), "含使用者資料") {
				t.Errorf("%s：錯誤訊息帶進了 Clerk 回應的內容：%v", name, err)
			}
		}
	}
}

func TestDevTokensAreSignedAndVerifiedLocally(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	d, err := NewDev(&key.PublicKey, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	tok, err := SignDev(key, DevIssuer, "dev_buyer", time.Hour, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if sub, err := d.Verify(ctx, tok); err != nil || sub.Subject != "dev_buyer" {
		t.Fatalf("Verify = %+v, %v", sub, err)
	}
	admin, _ := SignDev(key, "hawkr-dev-admin", "dev_admin", time.Hour, time.Now())
	if _, err := d.Verify(ctx, admin); !errors.Is(err, ErrInvalid) {
		t.Fatalf("後台的 dev token 不能打顧客：%v", err)
	}
	if _, err := SignDev(key, DevIssuer, "user_real", time.Hour, time.Now()); err == nil {
		t.Fatal("dev token 不能冒用 Clerk 的使用者 ID")
	}
	other, _ := rsa.GenerateKey(rand.Reader, 2048)
	forged, _ := SignDev(other, DevIssuer, "dev_buyer", time.Hour, time.Now())
	if _, err := d.Verify(ctx, forged); !errors.Is(err, ErrInvalid) {
		t.Fatalf("別的金鑰簽的 token 要被拒：%v", err)
	}
	if u, err := d.Verify(ctx, tok); err != nil || u.Email != "dev_buyer@dev.localhost" {
		t.Fatalf("User = %+v, %v", u, err)
	}
	parsed, err := jwxjwt.Parse([]byte(tok), jwxjwt.WithKey(jwa.RS256(), &key.PublicKey), jwxjwt.WithValidate(false))
	if err != nil {
		t.Fatal(err)
	}
	if err := parsed.Set("sts", "pending"); err != nil {
		t.Fatal(err)
	}
	pending, err := jwxjwt.Sign(parsed, jwxjwt.WithKey(jwa.RS256(), key))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Verify(ctx, string(pending)); !errors.Is(err, ErrInvalid) {
		t.Fatalf("pending session accepted: %v", err)
	}
}

type rejectHTTP struct{ t *testing.T }

func (r rejectHTTP) RoundTrip(*http.Request) (*http.Response, error) {
	r.t.Error("Clerk verification must not call HTTP")
	return nil, errors.New("unexpected HTTP")
}
func TestPEMSessionClaimsWithoutBackendAPI(t *testing.T) {
	f := newFakeClerk(t)
	der, err := x509.MarshalPKIXPublicKey(&f.key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{PublishableKey: testPublishableKey(), SecretKey: "sk_test_x", JWTKey: string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})), AuthorizedParties: []string{"https://web.example"}, HTTPClient: &http.Client{Transport: rejectHTTP{t}}}
	c, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := c.Verify(t.Context(), f.sign(t, sessionClaims("user_1"), map[string]any{"email": "a@example.com", "name": "Alice"}))
	if err != nil || claims.Subject != "user_1" || claims.Email != "a@example.com" || claims.Name != "Alice" {
		t.Fatalf("claims=%+v err=%v", claims, err)
	}
	claims, err = c.Verify(t.Context(), f.sign(t, sessionClaims("user_line"), nil))
	if err != nil || claims.Email != "" || claims.Name != "" {
		t.Fatalf("missing custom claims=%+v err=%v", claims, err)
	}
	cfg.AuthorizedParties = nil
	if _, err := New(cfg); err == nil {
		t.Fatal("Clerk configured without authorized parties")
	}
	cfg.AuthorizedParties = []string{"https://web.example"}
	cfg.JWTKey = ""
	if _, err := New(cfg); err == nil {
		t.Fatal("Clerk configured without PEM")
	}
}

func TestSessionPolicyRejectsUnauthorizedPartyWithoutNetwork(t *testing.T) {
	f := newFakeClerk(t)
	der, err := x509.MarshalPKIXPublicKey(&f.key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	c, err := New(Config{PublishableKey: testPublishableKey(), SecretKey: "sk_test_x", JWTKey: string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})), AuthorizedParties: []string{"https://web.example"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, extra := range []map[string]any{{"azp": "https://evil.example"}, {"sts": "pending"}} {
		if _, err := c.Verify(t.Context(), f.sign(t, sessionClaims("user_1"), extra)); !errors.Is(err, ErrInvalid) {
			t.Fatalf("accepted invalid session %v: %v", extra, err)
		}
	}
}

func TestSignDevAudienceJSONShape(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name     string
		audience []string
		want     string
	}{
		{name: "customer"},
		{name: "admin", audience: []string{"hawkr-dev-admin"}, want: `"hawkr-dev-admin"`},
		{name: "multiple", audience: []string{"one", "two"}, want: `["one","two"]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := SignDev(key, DevIssuer, "dev_buyer", time.Hour, time.Now(), tc.audience...)
			if err != nil {
				t.Fatal(err)
			}
			payload, err := base64.RawURLEncoding.DecodeString(strings.Split(raw, ".")[1])
			if err != nil {
				t.Fatal(err)
			}
			var claims map[string]json.RawMessage
			if err := json.Unmarshal(payload, &claims); err != nil {
				t.Fatal(err)
			}
			if got := string(claims["aud"]); got != tc.want {
				t.Fatalf("raw aud = %s, want %s", got, tc.want)
			}
		})
	}
}
