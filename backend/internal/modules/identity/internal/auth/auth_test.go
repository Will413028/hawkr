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

func TestAccountStatusRejectsOnlyDeletedAccounts(t *testing.T) {
	at := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	if got := apperr.KindOf(accountStatus(&at)); got != apperr.KindUnauthenticated {
		t.Fatalf("deleted kind=%v;want unauthenticated", got)
	}
	if err := accountStatus(nil); err != nil {
		t.Fatal(err)
	}
}
