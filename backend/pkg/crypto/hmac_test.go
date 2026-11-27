package crypto

import (
	"regexp"
	"testing"
)

func invoiceFingerprint(secret, taxID, appKey string) string {
	return HMACLookup(secret, taxID+"\n"+appKey)
}

const testSecret = "unit-test-lookup-hmac-key-32-bytes!!"

func TestHMACLookup_SameTaxIDAndKeyGivesSameFingerprint(t *testing.T) {
	a := invoiceFingerprint(testSecret, "12345678", "AbCdEfGhIjKlMnOpQrSt")
	b := invoiceFingerprint(testSecret, "12345678", "AbCdEfGhIjKlMnOpQrSt")
	if a != b {
		t.Fatalf("same tax id + same key must collide:\n  a = %s\n  b = %s", a, b)
	}
}

func TestHMACLookup_SameKeyDifferentTaxIDGivesDifferentFingerprint(t *testing.T) {
	a := invoiceFingerprint(testSecret, "12345678", "AbCdEfGhIjKlMnOpQrSt")
	b := invoiceFingerprint(testSecret, "87654321", "AbCdEfGhIjKlMnOpQrSt")
	if a == b {
		t.Fatalf("different tax id must not collide, both produced %s", a)
	}
}

func TestHMACLookup_DifferentSecretGivesDifferentFingerprint(t *testing.T) {
	a := invoiceFingerprint(testSecret, "12345678", "AbCdEfGhIjKlMnOpQrSt")
	b := invoiceFingerprint(testSecret+"-rotated", "12345678", "AbCdEfGhIjKlMnOpQrSt")
	if a == b {
		t.Fatalf("secret must affect the output, both produced %s", a)
	}
}

func TestHMACLookup_SeparatorIsNotAmbiguous(t *testing.T) {
	a := invoiceFingerprint(testSecret, "1234", "5678AbCdEfGhIjKlMnOp")
	b := invoiceFingerprint(testSecret, "12345678", "AbCdEfGhIjKlMnOp")
	if a == b {
		t.Fatalf("framing must be injective, both produced %s", a)
	}
}

func TestHMACLookup_ShapeIs64LowercaseHex(t *testing.T) {
	got := invoiceFingerprint(testSecret, "12345678", "AbCdEfGhIjKlMnOpQrSt")
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(got) {
		t.Fatalf("expected 64 lowercase hex characters, got %q (len %d)", got, len(got))
	}
}
