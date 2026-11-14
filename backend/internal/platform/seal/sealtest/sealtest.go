// Package sealtest 給測試用的固定金鑰：不是任何真實的金鑰。
package sealtest

import (
	"encoding/base64"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/platform/seal"
)

// MAC 回傳測試用的 HMAC 金鑰環。
func MAC(tb testing.TB) *seal.MAC {
	tb.Helper()
	mac, err := seal.ParseMAC("test1:" + base64.StdEncoding.EncodeToString([]byte("test-kyc-hash-hmac-key-000000001")))
	if err != nil {
		tb.Fatal(err)
	}
	return mac
}
