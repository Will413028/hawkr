package apple

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

type fakeApple struct {
	key    *ecdsa.PrivateKey
	forms  []url.Values
	paths  []string
	status int
	body   string
}

func TestClientSecretSignatureAndClaims(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	c := &Client{cfg: Config{TeamID: "TEAM", KeyID: "KEY", Now: func() time.Time { return now }}, key: key}
	raw, err := c.clientSecret("io.github.will413028.hawkr.buyer")
	if err != nil {
		t.Fatal(err)
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.Split(raw, ".")[1])
	if err != nil {
		t.Fatal(err)
	}
	var rawClaims map[string]json.RawMessage
	if err := json.Unmarshal(payload, &rawClaims); err != nil {
		t.Fatal(err)
	}
	if got := string(rawClaims["aud"]); got != `"https://appleid.apple.com"` {
		t.Fatalf("raw aud = %s, want a single JSON string", got)
	}
	signed, err := jwt.ParseSigned(raw, []jose.SignatureAlgorithm{jose.ES256})
	if err != nil {
		t.Fatal(err)
	}
	var claims jwt.Claims
	if err := signed.Claims(&key.PublicKey, &claims); err != nil {
		t.Fatal(err)
	}
	if claims.Issuer != "TEAM" || claims.Subject != "io.github.will413028.hawkr.buyer" || !claims.Audience.Contains(BaseURL) || claims.IssuedAt.Time().Unix() != now.Unix() || claims.Expiry.Time().Unix() != now.Add(clientSecretTTL).Unix() || signed.Headers[0].KeyID != "KEY" {
		t.Fatalf("claims=%+v iat=%v exp=%v kid=%q", claims, claims.IssuedAt.Time(), claims.Expiry.Time(), signed.Headers[0].KeyID)
	}
}

func newFake(t *testing.T) (*fakeApple, *Client) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeApple{key: key, status: http.StatusOK, body: `{"refresh_token":"rt-1","id_token":"idt-1"}`}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		f.forms = append(f.forms, r.PostForm)
		f.paths = append(f.paths, r.URL.Path)
		w.WriteHeader(f.status)
		_, _ = w.Write([]byte(f.body))
	}))
	t.Cleanup(srv.Close)
	c, err := New(Config{
		TeamID: "TEAM", KeyID: "KEY",
		PrivateKeyPEM: string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})),
		RedirectURIs:  map[string]string{"web.services.id": "https://hawkr.example/auth/apple"},
		BaseURL:       srv.URL,
		Now:           func() time.Time { return time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatal(err)
	}
	return f, c
}

func TestExchangeSendsASignedClientSecret(t *testing.T) {
	f, c := newFake(t)
	rt, idToken, err := c.Exchange(context.Background(), "io.github.will413028.hawkr.buyer", "code-1")
	if err != nil || rt != "rt-1" || idToken != "idt-1" {
		t.Fatalf("= %q, %q, %v", rt, idToken, err)
	}
	form := f.forms[0]
	if f.paths[0] != "/auth/token" || form.Get("code") != "code-1" || form.Get("grant_type") != "authorization_code" ||
		form.Get("client_id") != "io.github.will413028.hawkr.buyer" || form.Has("redirect_uri") {
		t.Fatalf("form = %v", form)
	}
	tok, err := jwt.ParseSigned(form.Get("client_secret"), []jose.SignatureAlgorithm{jose.ES256})
	if err != nil {
		t.Fatal(err)
	}
	var claims jwt.Claims
	if err := tok.Claims(&f.key.PublicKey, &claims); err != nil {
		t.Fatalf("client secret 的簽章：%v", err)
	}
	if claims.Issuer != "TEAM" || claims.Subject != "io.github.will413028.hawkr.buyer" || !claims.Audience.Contains(BaseURL) || tok.Headers[0].KeyID != "KEY" {
		t.Fatalf("client secret = %+v（kid %q）", claims, tok.Headers[0].KeyID)
	}

	// web 的 client 帶授權時的 redirect URI。
	if _, _, err := c.Exchange(context.Background(), "web.services.id", "code-2"); err != nil {
		t.Fatal(err)
	}
	if got := f.forms[1].Get("redirect_uri"); got != "https://hawkr.example/auth/apple" {
		t.Fatalf("redirect_uri = %q", got)
	}
}

func TestRevoke(t *testing.T) {
	f, c := newFake(t)
	f.body = ""
	if err := c.Revoke(context.Background(), "io.github.will413028.hawkr.seller", "rt-1"); err != nil {
		t.Fatal(err)
	}
	if f.paths[0] != "/auth/revoke" || f.forms[0].Get("token") != "rt-1" || f.forms[0].Get("token_type_hint") != "refresh_token" {
		t.Fatalf("form = %v", f.forms[0])
	}
	f.status, f.body = http.StatusBadRequest, `{"error":"invalid_grant"}`
	if err := c.Revoke(context.Background(), "io.github.will413028.hawkr.seller", "rt-1"); err != nil {
		t.Fatalf("已經失效的 token：%v", err)
	}
	f.body = `{"error":"invalid_client"}`
	if err := c.Revoke(context.Background(), "io.github.will413028.hawkr.seller", "rt-1"); !errors.Is(err, ErrRejected) {
		t.Fatalf("設定錯誤：err = %v，預期 ErrRejected", err)
	}
	f.status = http.StatusServiceUnavailable
	if err := c.Revoke(context.Background(), "io.github.will413028.hawkr.seller", "rt-1"); err == nil || errors.Is(err, ErrRejected) {
		t.Fatalf("5xx：err = %v，預期可以重試的錯誤", err)
	}
}

func TestNewRejectsABadKey(t *testing.T) {
	if _, err := New(Config{TeamID: "T", KeyID: "K", PrivateKeyPEM: "not a key"}); err == nil {
		t.Fatal("預期錯誤")
	}
}
