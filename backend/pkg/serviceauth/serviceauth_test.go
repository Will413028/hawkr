package serviceauth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

const testKey = "01234567890123456789012345678901"

func testAuthConfig(now time.Time) Config {
	return Config{
		CurrentKey: testKey,
		Issuer:     "gateway",
		Audience:   "im-service",
		Purpose:    "im-chat-stream",
		TTL:        5 * time.Minute,
		Now:        func() time.Time { return now },
	}
}

func TestSignerVerifier_RoundTrip(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	cfg := testAuthConfig(now)
	signer, err := NewSigner(cfg)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	verifier, err := NewVerifier(cfg)
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}

	raw, err := signer.Sign()
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	claims, err := verifier.Verify(raw)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if claims.Issuer != cfg.Issuer || len(claims.Audience) != 1 || claims.Audience[0] != cfg.Audience {
		t.Fatalf("claims identity = issuer %q audience %v, want %q/%q", claims.Issuer, claims.Audience, cfg.Issuer, cfg.Audience)
	}
	if claims.Purpose != cfg.Purpose {
		t.Fatalf("purpose = %q, want %q", claims.Purpose, cfg.Purpose)
	}
	if claims.ID == "" {
		t.Fatal("jti must be present")
	}
	if _, err := uuid.Parse(claims.ID); err != nil {
		t.Fatalf("jti = %q is not a UUID: %v", claims.ID, err)
	}
	if got := claims.IssuedAt.Time; !got.Equal(now) {
		t.Fatalf("iat = %v, want %v", got, now)
	}
	if got := claims.ExpiresAt.Sub(now); got != cfg.TTL {
		t.Fatalf("ttl = %v, want %v", got, cfg.TTL)
	}
}

func TestVerifier_AcceptsPreviousKeyDuringRotation(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	oldCfg := testAuthConfig(now)
	oldSigner, err := NewSigner(oldCfg)
	if err != nil {
		t.Fatalf("NewSigner(old): %v", err)
	}
	raw, err := oldSigner.Sign()
	if err != nil {
		t.Fatalf("Sign(old): %v", err)
	}

	rotated := testAuthConfig(now)
	rotated.CurrentKey = "abcdefghijklmnopqrstuvwxyz123456"
	rotated.PreviousKey = testKey
	verifier, err := NewVerifier(rotated)
	if err != nil {
		t.Fatalf("NewVerifier(rotated): %v", err)
	}
	if _, err := verifier.Verify(raw); err != nil {
		t.Fatalf("previous key should verify during rotation: %v", err)
	}
}

func TestVerifier_RejectsInvalidClaims(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	cases := []struct {
		name   string
		mutate func(*Config)
	}{
		{name: "wrong issuer", mutate: func(c *Config) { c.Issuer = "not-gateway" }},
		{name: "wrong audience", mutate: func(c *Config) { c.Audience = "not-im" }},
		{name: "wrong purpose", mutate: func(c *Config) { c.Purpose = "other-purpose" }},
		{name: "expired", mutate: func(c *Config) { c.Now = func() time.Time { return now.Add(6 * time.Minute) } }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			signCfg := testAuthConfig(now)
			signer, err := NewSigner(signCfg)
			if err != nil {
				t.Fatalf("NewSigner: %v", err)
			}
			raw, err := signer.Sign()
			if err != nil {
				t.Fatalf("Sign: %v", err)
			}

			verifyCfg := testAuthConfig(now)
			tc.mutate(&verifyCfg)
			verifier, err := NewVerifier(verifyCfg)
			if err != nil {
				t.Fatalf("NewVerifier: %v", err)
			}
			if _, err := verifier.Verify(raw); err == nil {
				t.Fatal("Verify unexpectedly accepted invalid claims")
			}
		})
	}
}

func TestAuthConfig_RejectsMissingOrWeakKeys(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  Config
	}{
		{name: "missing current", cfg: Config{}},
		{name: "weak current", cfg: Config{CurrentKey: "short"}},
		{name: "weak previous", cfg: Config{CurrentKey: testKey, PreviousKey: "short"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewSigner(tc.cfg); err == nil {
				t.Fatal("NewSigner unexpectedly accepted invalid key config")
			}
			if _, err := NewVerifier(tc.cfg); err == nil {
				t.Fatal("NewVerifier unexpectedly accepted invalid key config")
			}
		})
	}
}
