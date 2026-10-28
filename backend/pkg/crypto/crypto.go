// Package crypto provides AES-256-GCM authenticated encryption shared across
// services that must encrypt sensitive data at rest (e.g. the identity module's
// apple_revoke_outbox refresh_token, the seller module's certification id_card_no).
//
// The key is derived from a config secret via SHA-256. Unlike a
// module-local implementation, this package is fail-closed: an empty
// secret is rejected at construction time so a mis-configured deployment can
// never silently downgrade to storing plaintext (issue #608 / #1396).
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// ErrEmptySecret is returned by New when the supplied secret is empty. Callers
// should surface this at Config.Validate() so the service refuses to start.
var ErrEmptySecret = errors.New("crypto: master key secret is empty (fail-closed; refusing to run without a key)")

// Encrypter is the byte-oriented interface satisfied by *Cipher. Kept so that
// existing consumers (the identity module) that adapt to their own usecase-defined
// interfaces can depend on an interface rather than the concrete type.
type Encrypter interface {
	Encrypt(plaintext []byte) ([]byte, error)
	Decrypt(ciphertext []byte) ([]byte, error)
}

// Cipher is an AES-256-GCM authenticated cipher. Safe for concurrent use.
type Cipher struct {
	aead cipher.AEAD
}

// New constructs a Cipher from a config secret. The 32-byte AES key is derived
// from the secret via SHA-256. Returns ErrEmptySecret if secret is empty
// (fail-closed — never falls back to a passthrough/no-op cipher).
func New(secret string) (*Cipher, error) {
	if secret == "" {
		return nil, ErrEmptySecret
	}
	digest := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(digest[:])
	if err != nil {
		return nil, fmt.Errorf("crypto: aes new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: aes new gcm: %w", err)
	}
	return &Cipher{aead: gcm}, nil
}

// Encrypt encrypts plaintext with a fresh random nonce, returning
// nonce || ciphertext || tag in a single byte slice.
func (c *Cipher) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("crypto: read nonce: %w", err)
	}
	return c.aead.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt is the inverse of Encrypt. It fails if the input is shorter than the
// nonce or if the authentication tag does not verify.
func (c *Cipher) Decrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < c.aead.NonceSize() {
		return nil, errors.New("crypto: ciphertext shorter than nonce size")
	}
	nonce := ciphertext[:c.aead.NonceSize()]
	body := ciphertext[c.aead.NonceSize():]
	return c.aead.Open(nil, nonce, body, nil)
}

// EncryptToBase64 encrypts a plaintext string and returns the standard Base64
// encoding of nonce||ciphertext||tag — convenient for storing in a varchar
// column. An empty plaintext returns an empty string (caller decides NULL vs
// empty semantics).
func (c *Cipher) EncryptToBase64(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	ct, err := c.Encrypt([]byte(plaintext))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(ct), nil
}

// DecryptFromBase64 is the inverse of EncryptToBase64. An empty input returns
// an empty string. A non-empty input that is not valid Base64 or fails GCM
// authentication returns an error.
func (c *Cipher) DecryptFromBase64(encoded string) (string, error) {
	if encoded == "" {
		return "", nil
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("crypto: base64 decode: %w", err)
	}
	pt, err := c.Decrypt(raw)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}

// LooksEncrypted reports whether encoded plausibly is an EncryptToBase64 output
// of this cipher: valid Base64 whose bytes GCM-authenticate under this key.
// Used by one-off backfill tooling to make re-encryption idempotent. A false
// result means "treat as plaintext"; it is not a security boundary.
func (c *Cipher) LooksEncrypted(encoded string) bool {
	if encoded == "" {
		return false
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return false
	}
	if _, err := c.Decrypt(raw); err != nil {
		return false
	}
	return true
}
