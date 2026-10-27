package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// HMACLookup returns the lowercase hex HMAC-SHA256 (64 characters) of plaintext
// under secret.
//
// This is a *lookup key*, not a message authentication code: it exists so that
// rows encrypted with AES-GCM can still be compared for equality without being
// decrypted. GCM picks a fresh random nonce per encryption, so two ciphertexts
// of the same plaintext never match — an equality check has to run against a
// deterministic derivative instead.
//
// Why HMAC and not a bare SHA-256: the values this is applied to have small,
// known-format keyspaces (invoice-service feeds it a 20-character alphanumeric
// AMEGO AppKey, 58 §8.1.1(f)). A bare hash of such a value can be brute-forced
// offline the moment the column leaks, and a leaked column could also be joined
// against a list of known keys to answer "is this key registered on that
// platform?". HMAC binds the ability to recompute to a secret that only lives
// in the deployment's Secret store.
//
// The secret MUST be distinct from any at-rest encryption master key. Two
// reasons, and the second is the operational one: key separation as a principle,
// and lifecycle — a master key can be rotated at will, whereas rotating a lookup
// key invalidates every stored fingerprint at once (recomputing them requires
// decrypting the whole table). Sharing one key makes a routine master-key
// rotation detonate a full-table recompute.
//
// Callers own the plaintext framing. invoice-service uses
// provider_tax_id + "\n" + app_key so that "B pasted A's tax id together with
// A's AppKey" collides on the unique index; see 58 §8.1.1(f) for why the tax id
// is part of the input rather than the AppKey alone.
func HMACLookup(secret string, plaintext string) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(plaintext))
	return hex.EncodeToString(m.Sum(nil))
}
