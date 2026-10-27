package crypto

import (
	"errors"
	"testing"
)

func TestNew_EmptySecretFailClosed(t *testing.T) {
	c, err := New("")
	if !errors.Is(err, ErrEmptySecret) {
		t.Fatalf("expected ErrEmptySecret, got %v", err)
	}
	if c != nil {
		t.Fatalf("expected nil cipher on empty secret")
	}
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	c, err := New("test-master-key")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	cases := []string{"A123456789", "", "身分證字號中文", "x"}
	for _, plain := range cases {
		enc, err := c.EncryptToBase64(plain)
		if err != nil {
			t.Fatalf("encrypt %q: %v", plain, err)
		}
		if plain != "" && enc == plain {
			t.Fatalf("ciphertext equals plaintext for %q", plain)
		}
		got, err := c.DecryptFromBase64(enc)
		if err != nil {
			t.Fatalf("decrypt %q: %v", plain, err)
		}
		if got != plain {
			t.Fatalf("round-trip mismatch: want %q got %q", plain, got)
		}
	}
}

func TestEncrypt_NonDeterministicNonce(t *testing.T) {
	c, _ := New("k")
	a, _ := c.EncryptToBase64("A123456789")
	b, _ := c.EncryptToBase64("A123456789")
	if a == b {
		t.Fatalf("expected distinct ciphertexts due to random nonce")
	}
}

func TestDecrypt_WrongKeyFails(t *testing.T) {
	c1, _ := New("key-one")
	c2, _ := New("key-two")
	enc, _ := c1.EncryptToBase64("A123456789")
	if _, err := c2.DecryptFromBase64(enc); err == nil {
		t.Fatalf("expected auth failure decrypting with wrong key")
	}
}

func TestLooksEncrypted(t *testing.T) {
	c, _ := New("k")
	enc, _ := c.EncryptToBase64("A123456789")
	if !c.LooksEncrypted(enc) {
		t.Fatalf("expected LooksEncrypted true for real ciphertext")
	}
	if c.LooksEncrypted("A123456789") {
		t.Fatalf("expected LooksEncrypted false for plaintext id")
	}
	if c.LooksEncrypted("") {
		t.Fatalf("expected LooksEncrypted false for empty")
	}
}
