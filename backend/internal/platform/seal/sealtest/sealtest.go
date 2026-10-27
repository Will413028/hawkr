// Package sealtest 給測試用的固定金鑰：加密與 HMAC 是不同的 32 bytes，不是任何真實的金鑰。
package sealtest

import (
	"encoding/base64"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/platform/seal"
)

// Keys 回傳測試用的 KYC 加密金鑰環與 HMAC 金鑰環。
func Keys(tb testing.TB) (*seal.Keyring, *seal.MAC) {
	tb.Helper()
	enc, err := seal.Parse("test1:" + base64.StdEncoding.EncodeToString([]byte("test-kyc-encryption-key-00000001")))
	if err != nil {
		tb.Fatal(err)
	}
	mac, err := seal.ParseMAC("test1:" + base64.StdEncoding.EncodeToString([]byte("test-kyc-hash-hmac-key-000000001")))
	if err != nil {
		tb.Fatal(err)
	}
	return enc, mac
}
