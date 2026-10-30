package provider

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestSignDevTokenAudienceJSONShape(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name     string
		audience []string
		want     string
	}{
		{name: "none"},
		{name: "single", audience: []string{"hawkr-dev-admin"}, want: `"hawkr-dev-admin"`},
		{name: "multiple", audience: []string{"one", "two"}, want: `["one","two"]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := signDevToken(key, DevAdminIssuer, "dev_ops", time.Hour, time.Now(), tc.audience...)
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

func TestSignDevTokenRequiresDevPrefix(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := signDevToken(key, DevAdminIssuer, "user_real", time.Hour, time.Now()); err == nil {
		t.Fatal("非 dev_ 開頭的使用者被簽發")
	}
}
