package crypto

import (
	"regexp"
	"testing"
)

// invoiceFingerprint mirrors invoice-service's only assembly form
// (58 §8.1.1(f)): provider_tax_id + "\n" + app_key.
func invoiceFingerprint(secret, taxID, appKey string) string {
	return HMACLookup(secret, taxID+"\n"+appKey)
}

const testSecret = "unit-test-lookup-hmac-key-32-bytes!!"

func TestHMACLookup_SameTaxIDAndKeyGivesSameFingerprint(t *testing.T) {
	// The property the unique index relies on: two rows carrying the same
	// (tax id, AppKey) pair must collide, even though their AES-GCM ciphertexts
	// differ on every write.
	a := invoiceFingerprint(testSecret, "12345678", "AbCdEfGhIjKlMnOpQrSt")
	b := invoiceFingerprint(testSecret, "12345678", "AbCdEfGhIjKlMnOpQrSt")
	if a != b {
		t.Fatalf("same tax id + same key must collide:\n  a = %s\n  b = %s", a, b)
	}
}

func TestHMACLookup_SameKeyDifferentTaxIDGivesDifferentFingerprint(t *testing.T) {
	// The tax id is part of the input precisely so that "B pasted A's tax id
	// together with A's AppKey" collides while two genuinely different issuers
	// sharing a re-issued AppKey do not.
	a := invoiceFingerprint(testSecret, "12345678", "AbCdEfGhIjKlMnOpQrSt")
	b := invoiceFingerprint(testSecret, "87654321", "AbCdEfGhIjKlMnOpQrSt")
	if a == b {
		t.Fatalf("different tax id must not collide, both produced %s", a)
	}
}

func TestHMACLookup_DifferentSecretGivesDifferentFingerprint(t *testing.T) {
	// Without this the two tests above would still pass if the secret were
	// silently ignored — the exact failure mode that turns the fingerprint back
	// into a bare hash.
	a := invoiceFingerprint(testSecret, "12345678", "AbCdEfGhIjKlMnOpQrSt")
	b := invoiceFingerprint(testSecret+"-rotated", "12345678", "AbCdEfGhIjKlMnOpQrSt")
	if a == b {
		t.Fatalf("secret must affect the output, both produced %s", a)
	}
}

func TestHMACLookup_SeparatorIsNotAmbiguous(t *testing.T) {
	// Concatenating without a separator would make ("1234", "5678xyz") and
	// ("12345678", "xyz") the same input. The newline is what keeps the framing
	// injective for the fixed-width tax id the caller actually passes.
	a := invoiceFingerprint(testSecret, "1234", "5678AbCdEfGhIjKlMnOp")
	b := invoiceFingerprint(testSecret, "12345678", "AbCdEfGhIjKlMnOp")
	if a == b {
		t.Fatalf("framing must be injective, both produced %s", a)
	}
}

func TestHMACLookup_ShapeIs64LowercaseHex(t *testing.T) {
	// credential_fingerprint is a fixed-width column; a shape change is a
	// silent write failure rather than a compile error.
	got := invoiceFingerprint(testSecret, "12345678", "AbCdEfGhIjKlMnOpQrSt")
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(got) {
		t.Fatalf("expected 64 lowercase hex characters, got %q (len %d)", got, len(got))
	}
}
