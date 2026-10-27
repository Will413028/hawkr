package postgres

import (
	"strings"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/crypto"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres/gen"
)

// legacyBankAccountColumnLen is the pre-#2073 width of
// certification_applications.bank_account. Kept here so
// TestBankAccountCiphertextOutgrowsLegacyColumn can prove the migration that
// widened it was mandatory rather than precautionary.
const legacyBankAccountColumnLen = 50

func newTestCipher(t *testing.T) *crypto.Cipher {
	t.Helper()
	c, err := crypto.New("unit-test-master-key-2073")
	if err != nil {
		t.Fatalf("crypto.New: %v", err)
	}
	return c
}

// TestBankAccountRoundTrip pins the basic contract: what goes into the column is
// not the account number, and what comes back out is.
func TestBankAccountRoundTrip(t *testing.T) {
	enc := newTestCipher(t)
	for _, plaintext := range []string{"1", "0001234567890", "12345678901234"} {
		stored, err := encryptBankAccount(enc, plaintext)
		if err != nil {
			t.Fatalf("encryptBankAccount(%d chars): %v", len(plaintext), err)
		}
		if stored == plaintext {
			t.Fatalf("stored value equals plaintext (%d chars) — not encrypted", len(plaintext))
		}
		got, err := decryptBankAccount(enc, stored)
		if err != nil {
			t.Fatalf("decryptBankAccount: %v", err)
		}
		if got != plaintext {
			t.Fatalf("round-trip mismatch: got %d chars, want %d chars", len(got), len(plaintext))
		}
	}
}

// TestBankAccountEmptyStaysEmpty — draft rows with no bank account must stay
// NULL, not store an encrypted "".
func TestBankAccountEmptyStaysEmpty(t *testing.T) {
	enc := newTestCipher(t)
	stored, err := encryptBankAccount(enc, "")
	if err != nil || stored != "" {
		t.Fatalf("encryptBankAccount(\"\") = %q, %v; want \"\", nil", stored, err)
	}
	got, err := decryptBankAccount(enc, "")
	if err != nil || got != "" {
		t.Fatalf("decryptBankAccount(\"\") = %q, %v; want \"\", nil", got, err)
	}
}

// TestBankAccountCiphertextOutgrowsLegacyColumn is the regression guard for the
// trap called out in #2073: encrypting before widening the column would have
// produced write failures / truncated undecryptable ciphertext. If this ever
// fails, the varchar(255) migration assumption needs revisiting.
func TestBankAccountCiphertextOutgrowsLegacyColumn(t *testing.T) {
	enc := newTestCipher(t)
	// 14 chars is the longest real value observed in dev and prod.
	stored, err := encryptBankAccount(enc, "12345678901234")
	if err != nil {
		t.Fatalf("encryptBankAccount: %v", err)
	}
	if len(stored) <= legacyBankAccountColumnLen {
		t.Fatalf("ciphertext is %d chars, expected > %d — the widening migration would be unnecessary, re-check the assumption",
			len(stored), legacyBankAccountColumnLen)
	}
}

// TestDecryptBankAccountRejectsPlaintext is the guard that replaced the pre-#2073
// compatibility layer. The backfill encrypted every plaintext row in every
// environment, so a plaintext-shaped value in this column can now only mean a
// write that skipped encryption — and that has to be loud. While the old
// compatibility branches existed these very inputs were returned as-is, which is
// exactly why a silently-unencrypted row could never be detected.
func TestDecryptBankAccountRejectsPlaintext(t *testing.T) {
	enc := newTestCipher(t)
	cases := []struct {
		name   string
		stored string
	}{
		// Shapes that were actually present pre-backfill: 6-14 digits.
		{"shortest observed", "123456"},
		{"longest observed", "12345678901234"},
		{"leading zeros", "0001234567890"},
		// A human-typed value long enough to have reached the old digits-only
		// escape hatch (old branch 3).
		{"grouped and over 40 chars", strings.Repeat("1234-", 9)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := decryptBankAccount(enc, c.stored)
			if err == nil {
				t.Fatalf("plaintext was accepted (returned %d chars) — the compatibility layer is back", len(got))
			}
			if strings.Contains(err.Error(), c.stored) {
				t.Fatal("error message leaks the stored value")
			}
		})
	}
}

// TestDecryptBankAccountRejectsUndecryptableCiphertext — a value that looks like
// ciphertext but will not decrypt (corruption, wrong/rotated key) must fail
// loudly. Returning it as-is would surface garbage as an account number.
func TestDecryptBankAccountRejectsUndecryptableCiphertext(t *testing.T) {
	enc := newTestCipher(t)
	other, err := crypto.New("a-different-master-key")
	if err != nil {
		t.Fatalf("crypto.New: %v", err)
	}
	foreign, err := other.EncryptToBase64("12345678901234")
	if err != nil {
		t.Fatalf("EncryptToBase64: %v", err)
	}
	if _, err := decryptBankAccount(enc, foreign); err == nil {
		t.Fatal("expected an error for ciphertext encrypted under a different key, got nil")
	}

	// Truncated ciphertext — the exact corruption the pre-widening column would
	// have produced.
	valid, err := enc.EncryptToBase64("12345678901234")
	if err != nil {
		t.Fatalf("EncryptToBase64: %v", err)
	}
	if _, err := decryptBankAccount(enc, valid[:legacyBankAccountColumnLen]); err == nil {
		t.Fatal("expected an error for truncated ciphertext, got nil")
	}
}

// TestDecryptBankAccountErrorDoesNotLeakValue — the error surfaces in logs and
// gRPC status messages, so it must never carry the stored value.
func TestDecryptBankAccountErrorDoesNotLeakValue(t *testing.T) {
	enc := newTestCipher(t)
	other, _ := crypto.New("a-different-master-key")
	foreign, _ := other.EncryptToBase64("12345678901234")
	_, err := decryptBankAccount(enc, foreign)
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), foreign) {
		t.Fatal("error message leaks the stored value")
	}
}

// TestCreateAndUpdateParamsEncryptBankAccount is the test that goes red if the
// encryption is ever removed from the write choke points: it asserts on the
// value actually bound to the SQL parameter, not on the domain entity.
func TestCreateAndUpdateParamsEncryptBankAccount(t *testing.T) {
	enc := newTestCipher(t)
	const plaintext = "0001234567890"
	app := &domain.CertificationApplication{
		ID:          1,
		AccountID:   2,
		Status:      domain.CertStatusDraft,
		IDCardNo:    "A123456789",
		BankAccount: plaintext,
	}

	create, err := toCreateApplicationParams(app, enc)
	if err != nil {
		t.Fatalf("toCreateApplicationParams: %v", err)
	}
	assertStoredIsEncrypted(t, enc, *create.BankAccount, plaintext, "CreateApplicationParams")

	update, err := toUpdateApplicationParams(app, enc)
	if err != nil {
		t.Fatalf("toUpdateApplicationParams: %v", err)
	}
	assertStoredIsEncrypted(t, enc, *update.BankAccount, plaintext, "UpdateApplicationParams")

	// Two writes of the same value must not produce identical ciphertext —
	// otherwise the column becomes a lookup table for guessable accounts.
	second, err := toCreateApplicationParams(app, enc)
	if err != nil {
		t.Fatalf("toCreateApplicationParams (2nd): %v", err)
	}
	if *second.BankAccount == *create.BankAccount {
		t.Fatal("two encryptions of the same account produced identical ciphertext (nonce reuse)")
	}
}

func assertStoredIsEncrypted(t *testing.T, enc *crypto.Cipher, stored, plaintext, what string) {
	t.Helper()
	if stored == plaintext {
		t.Fatalf("%s: bank_account bound as plaintext", what)
	}
	if !enc.LooksEncrypted(stored) {
		t.Fatalf("%s: bank_account is not ciphertext of this cipher", what)
	}
	got, err := enc.DecryptFromBase64(stored)
	if err != nil {
		t.Fatalf("%s: stored value does not decrypt: %v", what, err)
	}
	if got != plaintext {
		t.Fatalf("%s: decrypted value does not match the original", what)
	}
}

// TestToDomainDecryptsBankAccount covers the read choke point that all 5 read
// repo methods share: an encrypted row resolves, and a plaintext row surfaces as
// an error instead of being quietly passed through.
func TestToDomainDecryptsBankAccount(t *testing.T) {
	enc := newTestCipher(t)
	const plaintext = "0001234567890"
	ciphertext, err := enc.EncryptToBase64(plaintext)
	if err != nil {
		t.Fatalf("EncryptToBase64: %v", err)
	}
	encryptedID, err := enc.EncryptToBase64("A123456789")
	if err != nil {
		t.Fatalf("EncryptToBase64: %v", err)
	}

	newRow := func(stored string) gen.CertificationApplication {
		return gen.CertificationApplication{
			ID:          7,
			Status:      domain.CertStatusDraft.String(),
			IDCardNo:    &encryptedID,
			BankAccount: &stored,
		}
	}

	t.Run("encrypted row decrypts", func(t *testing.T) {
		app, err := toDomainCertificationApplication(newRow(ciphertext), enc)
		if err != nil {
			t.Fatalf("toDomainCertificationApplication: %v", err)
		}
		if app.BankAccount != plaintext {
			t.Fatalf("domain entity did not receive the plaintext account (got %d chars, want %d)",
				len(app.BankAccount), len(plaintext))
		}
	})

	t.Run("unencrypted row is surfaced, not swallowed", func(t *testing.T) {
		if _, err := toDomainCertificationApplication(newRow(plaintext), enc); err == nil {
			t.Fatal("toDomainCertificationApplication accepted an unencrypted bank_account")
		}
	})
}
