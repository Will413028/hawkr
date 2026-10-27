package oidc

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
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	jwxjwt "github.com/lestrrat-go/jwx/v3/jwt"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func policyKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return k
}
func policyToken(t *testing.T, key *rsa.PrivateKey, kid string, changes map[string]any) string {
	t.Helper()
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key}, (&jose.SignerOptions{}).WithHeader("kid", kid))
	if err != nil {
		t.Fatal(err)
	}
	claims := map[string]any{"iss": "issuer", "sub": "user_1", "exp": now.Add(time.Hour).Unix(), "nbf": now.Add(-time.Minute).Unix(), "aud": []string{"application"}}
	for k, v := range changes {
		claims[k] = v
	}
	raw, err := jwt.Signed(signer).Claims(claims).Serialize()
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func TestPolicyClaimsAndPEMNetworkless(t *testing.T) {
	key := policyKey(t)
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	parties := []string{"https://web.example"}
	cfg := Config{Issuers: []string{"issuer"}, PublicKeyPEM: string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})), Audiences: []string{"application"}, Validators: []jwxjwt.Validator{jwxjwt.ValidatorFunc(func(_ context.Context, tok jwxjwt.Token) error {
		var azp any
		if tok.Has("azp") {
			if err := tok.Get("azp", &azp); err != nil || len(parties) == 0 || azp != parties[0] {
				return ErrInvalid
			}
		}
		var sts any
		if tok.Has("sts") {
			if err := tok.Get("sts", &sts); err != nil || sts != "active" {
				return ErrInvalid
			}
		}
		return nil
	})}, Now: func() time.Time { return now }, HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Error("PEM verifier accessed HTTP")
		return nil, errors.New("network forbidden")
	})}}
	v, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for name, changes := range map[string]map[string]any{
		"null azp": {"azp": nil}, "null sts": {"sts": nil}, "issuer": {"iss": "other"}, "exact issuer": {"iss": "accounts.google.com"}, "expired": {"exp": now.Add(-time.Minute).Unix()}, "missing exp": {"exp": nil}, "future nbf": {"nbf": now.Add(time.Minute).Unix()}, "missing sub": {"sub": ""}, "aud": {"aud": "other"}, "azp": {"azp": "https://evil.example"}, "empty azp": {"azp": ""}, "sts": {"sts": "pending"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := v.Verify(t.Context(), policyToken(t, key, "k1", changes)); !errors.Is(err, ErrInvalid) {
				t.Fatalf("accepted invalid claims: %v", err)
			}
		})
	}
	for _, extra := range []map[string]any{nil, {"azp": "https://web.example", "sts": "active", "email": "a@example.com", "name": "Alice"}} {
		c, err := v.Verify(t.Context(), policyToken(t, key, "k1", extra))
		if err != nil || c.Subject != "user_1" {
			t.Fatalf("valid: %+v %v", c, err)
		}
		if extra != nil && (c.Email != "a@example.com" || c.Name != "Alice") {
			t.Fatalf("custom claims: %+v", c)
		}
	}
	parties = nil
	if _, err := v.Verify(t.Context(), policyToken(t, key, "k1", map[string]any{"azp": "https://web.example"})); !errors.Is(err, ErrInvalid) {
		t.Fatal("missing authorized parties accepted")
	}
}
func TestPolicyRejectsWrongAlgorithms(t *testing.T) {
	key := policyKey(t)
	v, err := New(Config{Issuers: []string{"issuer"}, PublicKey: &key.PublicKey, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.HS256, Key: []byte(strings.Repeat("a", 32))}, nil)
	if err != nil {
		t.Fatal(err)
	}
	hs, err := jwt.Signed(signer).Claims(map[string]any{"iss": "issuer", "sub": "user_1", "exp": now.Add(time.Hour).Unix()}).Serialize()
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(policyToken(t, key, "k1", nil), ".")
	none := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`)) + "." + parts[1] + "."
	for _, raw := range []string{hs, none} {
		if _, err := v.Verify(t.Context(), raw); !errors.Is(err, ErrInvalid) {
			t.Fatalf("wrong alg: %v", err)
		}
	}
}
func floodVerifier(t *testing.T, hc *http.Client, url string) *Verifier {
	t.Helper()
	v, err := New(Config{Issuers: []string{"issuer"}, JWKSURL: url, Audiences: []string{"application"}, HTTPClient: hc, Now: func() time.Time { return now }, Context: t.Context()})
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func testFlood(t *testing.T, v *Verifier, key *rsa.PrivateKey, count *atomic.Int32) {
	t.Helper()
	for range 20 {
		if _, err := v.Verify(t.Context(), policyToken(t, key, "forged", nil)); !errors.Is(err, ErrInvalid) {
			t.Fatalf("forged kid: %v", err)
		}
	}
	if got := count.Load(); got != 1 {
		t.Fatalf("JWKS fetches = %d; want 1 per interval", got)
	}
	if _, err := v.Verify(t.Context(), policyToken(t, key, "k1", nil)); err != nil {
		t.Fatalf("cached valid key: %v", err)
	}
}
func TestPolicyUnknownKIDFlood(t *testing.T) {
	key := policyKey(t)
	var count atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		count.Add(1)
		_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "k1", Algorithm: "RS256", Use: "sig"}}})
	})
	hc := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w.Result(), nil
	})}
	testFlood(t, floodVerifier(t, hc, "https://jwks.test/keys"), key, &count)
}

func TestJWKSWithoutAlgorithmUsesRS256(t *testing.T) {
	key := policyKey(t)
	hc := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		w := httptest.NewRecorder()
		_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "k1", Use: "sig"}}})
		return w.Result(), nil
	})}
	v := floodVerifier(t, hc, "https://jwks.test/keys")
	if _, err := v.Verify(t.Context(), policyToken(t, key, "k1", nil)); err != nil {
		t.Fatal(err)
	}
}
func TestUnknownKIDFloodHTTPServer(t *testing.T) {
	key := policyKey(t)
	var count atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		count.Add(1)
		_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "k1", Algorithm: "RS256", Use: "sig"}}})
	}))
	t.Cleanup(server.Close)
	testFlood(t, floodVerifier(t, server.Client(), server.URL), key, &count)
}

func TestCachedKIDDoesNotWaitForForcedRefresh(t *testing.T) {
	key := policyKey(t)
	started := make(chan struct{})
	release := make(chan struct{})
	var count atomic.Int32
	hc := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if count.Add(1) == 2 {
			close(started)
			select {
			case <-release:
			case <-r.Context().Done():
				return nil, r.Context().Err()
			}
		}
		w := httptest.NewRecorder()
		_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "k1", Algorithm: "RS256", Use: "sig"}}})
		return w.Result(), nil
	})}
	var clock atomic.Int64
	clock.Store(now.Unix())
	v, err := New(Config{Issuers: []string{"issuer"}, JWKSURL: "https://jwks.test/keys", Audiences: []string{"application"}, HTTPClient: hc, Now: func() time.Time { return time.Unix(clock.Load(), 0) }, Context: t.Context()})
	if err != nil {
		t.Fatal(err)
	}
	known := policyToken(t, key, "k1", nil)
	unknown := policyToken(t, key, "forged", nil)
	if _, err := v.Verify(t.Context(), known); err != nil {
		t.Fatal(err)
	}
	clock.Store(now.Add(minRefreshInterval).Unix())
	refreshDone := make(chan struct{})
	go func() {
		_, _ = v.Verify(context.Background(), unknown)
		close(refreshDone)
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("forced refresh did not start")
	}
	knownDone := make(chan error, 1)
	go func() {
		_, err := v.Verify(context.Background(), known)
		knownDone <- err
	}()
	select {
	case err := <-knownDone:
		if err != nil {
			t.Errorf("cached key: %v", err)
		}
	case <-time.After(300 * time.Millisecond):
		t.Error("cached key waited for JWKS refresh")
	}
	close(release)
	<-refreshDone
}

func TestCanceledCallerDoesNotCancelSharedJWKSFetch(t *testing.T) {
	key := policyKey(t)
	started := make(chan struct{})
	release := make(chan struct{})
	hc := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		close(started)
		select {
		case <-release:
		case <-r.Context().Done():
			return nil, r.Context().Err()
		}
		w := httptest.NewRecorder()
		_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "k1", Algorithm: "RS256", Use: "sig"}}})
		return w.Result(), nil
	})}
	v := floodVerifier(t, hc, "https://jwks.test/keys")
	firstCtx, cancel := context.WithCancel(t.Context())
	first := make(chan error, 1)
	token := policyToken(t, key, "k1", nil)
	go func() {
		_, err := v.Verify(firstCtx, token)
		first <- err
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		cancel()
		close(release)
		t.Fatal("JWKS fetch did not start")
	}
	cancel()
	if err := <-first; !errors.Is(err, context.Canceled) {
		t.Fatalf("first caller: %v", err)
	}
	close(release)
	if _, err := v.Verify(t.Context(), token); err != nil {
		t.Fatalf("second caller: %v", err)
	}
}

func TestUnknownKIDWaitsForInflightRefresh(t *testing.T) {
	oldKey := policyKey(t)
	newKey := policyKey(t)
	started := make(chan struct{})
	release := make(chan struct{})
	var count atomic.Int32
	hc := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		kid := "old"
		key := oldKey
		if count.Add(1) == 2 {
			close(started)
			select {
			case <-release:
			case <-r.Context().Done():
				return nil, r.Context().Err()
			}
			kid = "new"
			key = newKey
		}
		w := httptest.NewRecorder()
		_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: kid, Algorithm: "RS256", Use: "sig"}}})
		return w.Result(), nil
	})}
	var clock atomic.Int64
	clock.Store(now.Unix())
	v, err := New(Config{Issuers: []string{"issuer"}, JWKSURL: "https://jwks.test/keys", Audiences: []string{"application"}, HTTPClient: hc, Now: func() time.Time { return time.Unix(clock.Load(), 0) }, Context: t.Context()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.Verify(t.Context(), policyToken(t, oldKey, "old", nil)); err != nil {
		t.Fatal(err)
	}
	clock.Store(now.Add(minRefreshInterval).Unix())
	token := policyToken(t, newKey, "new", nil)
	first := make(chan error, 1)
	go func() {
		_, err := v.Verify(t.Context(), token)
		first <- err
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("forced refresh did not start")
	}
	second := make(chan error, 1)
	go func() {
		_, err := v.Verify(t.Context(), token)
		second <- err
	}()
	select {
	case err := <-second:
		close(release)
		t.Fatalf("second caller returned before refresh: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	if err := <-first; err != nil {
		t.Fatalf("first caller: %v", err)
	}
	if err := <-second; err != nil {
		t.Fatalf("second caller: %v", err)
	}
}
