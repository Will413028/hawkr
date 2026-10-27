package vendorportal

import (
	"crypto/sha256"
	"encoding/base64"
	"slices"
	"testing"
)

func TestNewSecretIsOpaqueBase64URLAndHashMatches(t *testing.T) {
	seen := map[string]bool{}
	for range 50 {
		secret, hash, err := newSecret()
		if err != nil {
			t.Fatal(err)
		}
		if len(secret) != 43 || seen[secret] {
			t.Fatalf("secret=%q 長度或重複", secret)
		}
		seen[secret] = true
		raw, err := base64.RawURLEncoding.DecodeString(secret)
		if err != nil || len(raw) != 32 {
			t.Fatalf("secret=%q err=%v", secret, err)
		}
		sum := sha256.Sum256(raw)
		got, ok := hashSecret(secret)
		if !ok || !slices.Equal(got, sum[:]) || !slices.Equal(hash, sum[:]) {
			t.Fatal("hash 不一致")
		}
	}
}

func TestHashSecretRejectsAnythingButACanonical32ByteSecret(t *testing.T) {
	good, _, _ := newSecret()
	for name, s := range map[string]string{
		"空": "", "太短": "AAAA", "太長": good + "AA", "padding": good + "=", "非法字元": "!" + good[1:],
		"非標準的結尾位元": good[:42] + "B",
	} {
		if name == "非標準的結尾位元" && good[42] == 'B' {
			continue
		}
		if _, ok := hashSecret(s); ok && s != good {
			t.Errorf("%s：%q 不該被接受", name, s)
		}
	}
}
