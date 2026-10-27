package auth

import (
	"encoding/base64"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"testing"
	"time"
)

func TestUnverifiedIssuerOnlySelectsAnApplication(t *testing.T) {
	claims := base64.RawURLEncoding.EncodeToString([]byte(`{"iss":"https://issuer.test","sub":"user_1"}`))
	if got := unverifiedIssuer("e30." + claims + ".signature"); got != "https://issuer.test" {
		t.Fatalf("issuer = %q", got)
	}
	for _, token := range []string{"", "one.part", "e30.!.signature", "e30." + base64.RawURLEncoding.EncodeToString([]byte(`[]`)) + ".signature"} {
		if got := unverifiedIssuer(token); got != "" {
			t.Fatalf("malformed token %q selected %q", token, got)
		}
	}
}

func TestAccountStatusErrors(t *testing.T) {
	at := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name               string
		deleted, suspended *time.Time
		kind               apperr.Kind
	}{
		{"suspended", nil, &at, apperr.KindPermissionDenied},
		{"deleted", &at, nil, apperr.KindUnauthenticated},
		{"deleted and suspended", &at, &at, apperr.KindUnauthenticated},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := apperr.KindOf(accountStatus(tc.deleted, tc.suspended)); got != tc.kind {
				t.Fatalf("kind=%v;want %v", got, tc.kind)
			}
		})
	}
	if err := accountStatus(nil, nil); err != nil {
		t.Fatal(err)
	}
}
