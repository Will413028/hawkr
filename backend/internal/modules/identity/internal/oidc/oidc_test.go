package oidc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

type issuer struct {
	key    *rsa.PrivateKey
	client *http.Client
	down   bool
}

func newIssuer(t *testing.T) *issuer {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	is := &issuer{key: key}
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if is.down {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "k1", Algorithm: "RS256", Use: "sig"}}})
	})
	is.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w.Result(), nil
	})}
	return is
}

var now = time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)

func (is *issuer) sign(t *testing.T, claims map[string]any) string {
	t.Helper()
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: is.key}, (&jose.SignerOptions{}).WithHeader("kid", "k1").WithType("JWT"))
	if err != nil {
		t.Fatal(err)
	}
	base := map[string]any{
		"iss": "https://accounts.google.com", "sub": "sub-1", "aud": "buyer-web",
		"iat": now.Add(-time.Minute).Unix(), "exp": now.Add(time.Hour).Unix(),
	}
	for k, v := range claims {
		base[k] = v
	}
	raw, err := jwt.Signed(signer).Claims(base).Serialize()
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func (is *issuer) verifier(t *testing.T) *Verifier {
	t.Helper()
	v, err := New(Config{Issuers: []string{GoogleIssuer, GoogleIssuerNoScheme}, JWKSURL: "https://jwks.test/keys", Audiences: []string{"buyer-web", "seller-ios"}, HTTPClient: is.client, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestVerifyReturnsTheClaims(t *testing.T) {
	is := newIssuer(t)
	v := is.verifier(t)
	c, err := v.Verify(context.Background(), is.sign(t, map[string]any{
		"aud": []string{"other", "seller-ios"}, "email": "a@example.com", "email_verified": true, "hd": "example.com", "name": "王小明",
	}))
	if err != nil {
		t.Fatal(err)
	}
	want := Claims{Subject: "sub-1", Email: "a@example.com", EmailVerified: true, Audience: "seller-ios", Name: "王小明", IssuedAt: now.Add(-time.Minute)}
	if c != want {
		t.Fatalf("= %+v", c)
	}
	// Apple 的 email_verified 可能是字串。
	c, err = v.Verify(context.Background(), is.sign(t, map[string]any{"email_verified": "true"}))
	if err != nil || !c.EmailVerified {
		t.Fatalf("= %+v, %v", c, err)
	}
	c, err = v.Verify(context.Background(), is.sign(t, map[string]any{"email_verified": "false"}))
	if err != nil || c.EmailVerified {
		t.Fatalf("= %+v, %v", c, err)
	}
}

func TestVerifyReturnsNonceAndIssuedAt(t *testing.T) {
	is := newIssuer(t)
	v := is.verifier(t)
	c, err := v.Verify(context.Background(), is.sign(t, map[string]any{"nonce": "n-1", "iat": now.Add(-9 * time.Minute).Unix()}))
	if err != nil || c.Nonce != "n-1" || !c.IssuedAt.Equal(now.Add(-9*time.Minute)) {
		t.Fatalf("= %+v, %v", c, err)
	}
	c, err = v.Verify(context.Background(), is.sign(t, nil))
	if err != nil || c.Nonce != "" {
		t.Fatalf("沒有 nonce：%+v, %v", c, err)
	}
	if _, err := v.Verify(context.Background(), is.sign(t, map[string]any{"nonce": 7})); !errors.Is(err, ErrInvalid) {
		t.Fatalf("nonce 不是字串：%v", err)
	}
}

// iat 在未來的容忍度是 claimLeeway（5 秒）：超過就由解析拒絕，不另外放寬。
func TestIssuedAtInTheFutureBeyondTheLeewayIsRejected(t *testing.T) {
	is := newIssuer(t)
	v := is.verifier(t)
	if _, err := v.Verify(context.Background(), is.sign(t, map[string]any{"iat": now.Add(4 * time.Second).Unix()})); err != nil {
		t.Fatalf("4 秒內的時間差要容忍：%v", err)
	}
	if _, err := v.Verify(context.Background(), is.sign(t, map[string]any{"iat": now.Add(6 * time.Second).Unix()})); !errors.Is(err, ErrInvalid) {
		t.Fatalf("超過 5 秒的未來 iat 要拒絕：%v", err)
	}
}

func TestGoogleSchemeLessIssuer(t *testing.T) {
	is := newIssuer(t)
	v := is.verifier(t)
	claims, err := v.Verify(context.Background(), is.sign(t, map[string]any{"iss": GoogleIssuerNoScheme}))
	if err != nil || claims.Subject != "sub-1" {
		t.Fatalf("claims = %+v, err = %v", claims, err)
	}
}

func TestInvalidTokensAreRejected(t *testing.T) {
	is := newIssuer(t)
	v := is.verifier(t)
	other := newIssuer(t)
	cases := map[string]string{
		"別人簽的":           other.sign(t, nil),
		"過期":             is.sign(t, map[string]any{"exp": now.Add(-time.Minute).Unix()}),
		"issuer 不對":      is.sign(t, map[string]any{"iss": "https://evil.example"}),
		"audience 不在白名單": is.sign(t, map[string]any{"aud": "someone-else"}),
		"不是 JWT":         "not-a-jwt",
	}
	for name, tok := range cases {
		if _, err := v.Verify(context.Background(), tok); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s：err = %v，預期 ErrInvalid", name, err)
		}
	}
}

func TestUnreachableKeysAreNotAnInvalidToken(t *testing.T) {
	is := newIssuer(t)
	v := is.verifier(t)
	is.down = true
	_, err := v.Verify(context.Background(), is.sign(t, nil))
	if err == nil || errors.Is(err, ErrInvalid) {
		t.Fatalf("抓不到公鑰時 err = %v，預期不是 ErrInvalid", err)
	}
}
