package provider

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/oidc"
	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

type jwksTransport func(*http.Request) (*http.Response, error)

func (t jwksTransport) RoundTrip(r *http.Request) (*http.Response, error) { return t(r) }

func TestAccessJWTValidation(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	jwk := jose.JSONWebKey{Key: &key.PublicKey, KeyID: "local-test", Algorithm: "RS256", Use: "sig"}
	keys, err := json.Marshal(map[string]any{"keys": []jose.JSONWebKey{jwk}})
	if err != nil {
		t.Fatal(err)
	}
	const issuer = "https://team.cloudflareaccess.test"
	client := &http.Client{Transport: jwksTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != issuer+"/cdn-cgi/access/certs" {
			t.Fatalf("JWKS URL = %s", r.URL)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(string(keys))),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Request:    r,
		}, nil
	})}
	v, err := NewAccess(model.AccessConfig{TeamDomain: issuer, Audience: "admin-aud", HTTPClient: client})
	if err != nil {
		t.Fatal(err)
	}
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key}, (&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "local-test"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	sign := func(iss, aud string, expiry, notBefore time.Time) string {
		t.Helper()
		raw, err := jwt.Signed(signer).Claims(jwt.Claims{
			Issuer: iss, Subject: "access-subject", Audience: jwt.Audience{aud},
			IssuedAt: jwt.NewNumericDate(now), NotBefore: jwt.NewNumericDate(notBefore), Expiry: jwt.NewNumericDate(expiry),
		}).Claims(map[string]any{"email": "ops@gmail.com"}).Serialize()
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	for name, raw := range map[string]string{
		"wrong audience": sign(issuer, "other-aud", now.Add(time.Hour), now.Add(-time.Minute)),
		"wrong issuer":   sign("https://other-team.test", "admin-aud", now.Add(time.Hour), now.Add(-time.Minute)),
		"expired":        sign(issuer, "admin-aud", now.Add(-time.Hour), now.Add(-2*time.Hour)),
		"future nbf":     sign(issuer, "admin-aud", now.Add(2*time.Hour), now.Add(time.Hour)),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := v.Verify(context.Background(), raw)
			if err == nil {
				t.Fatal("無效的 Access JWT 被接受")
			}
		})
	}
	good := sign(issuer, "admin-aud", now.Add(time.Hour), now.Add(-time.Minute))
	claims, err := v.Verify(context.Background(), good)
	if err != nil || claims != (model.AdminIdentity{Subject: "access-subject", Email: "ops@gmail.com"}) {
		t.Fatalf("有效的 Access JWT = %+v, %v", claims, err)
	}
}

func TestDevAdminUsesAccessClaims(t *testing.T) {
	dev, err := NewDevAuth(nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := dev.Admins().(accessVerifier); !ok {
		t.Fatal("dev admin 必須使用 Access verifier")
	}
	token, err := dev.AdminToken("dev_ops")
	if err != nil {
		t.Fatal(err)
	}
	got, err := dev.Admins().Verify(context.Background(), token)
	if err != nil || got != (model.AdminIdentity{Subject: "dev_ops", Email: "dev_ops@dev.localhost"}) {
		t.Fatalf("dev admin = %+v, %v", got, err)
	}
	clerkShaped, err := SignDevToken(dev.key, DevAdminIssuer, "dev_ops", time.Hour, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dev.Admins().Verify(context.Background(), clerkShaped); !errors.Is(err, model.ErrInvalidToken) {
		t.Fatalf("Clerk-shaped admin token = %v", err)
	}
}

func TestGoogleIssuerVariants(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	cfg := googleConfig([]string{"buyer-web"})
	cfg.PublicKey = &key.PublicKey
	cfg.JWKSURL = ""
	v, err := oidc.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key}, (&jose.SignerOptions{}).WithType("JWT"))
	if err != nil {
		t.Fatal(err)
	}
	for _, issuer := range []string{oidc.GoogleIssuer, oidc.GoogleIssuerNoScheme} {
		raw, err := jwt.Signed(signer).Claims(jwt.Claims{
			Issuer: issuer, Subject: "google-user", Audience: jwt.Audience{"buyer-web"},
			Expiry: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		}).Serialize()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := v.Verify(context.Background(), raw); err != nil {
			t.Errorf("issuer %q: %v", issuer, err)
		}
	}
}
