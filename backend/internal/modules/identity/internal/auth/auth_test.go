package auth

import (
	"encoding/base64"
	"testing"
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
