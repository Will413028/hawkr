//go:build integration

package postgres_test

import (
	"testing"

	"github.com/Will413028/hawkr/go-pkg/crypto"
)

// testCipher returns a deterministic AES-256-GCM cipher for integration tests
// (#608). The key is fixed so round-trips within a test are stable.
func testCipher(t *testing.T) *crypto.Cipher {
	t.Helper()
	c, err := crypto.New("integration-test-master-key")
	if err != nil {
		t.Fatalf("crypto.New: %v", err)
	}
	return c
}
