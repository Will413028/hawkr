package main

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/platform/config"
)

func b64(b byte, n int) string { return base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{b}, n)) }

func TestKYCHashKeysAreRequiredAndChecked(t *testing.T) {
	mac := "h1:" + b64(2, 32)
	for name, tc := range map[string]struct {
		mac      string
		wantKeys bool
		wantErr  string
	}{
		"沒有設定":       {"", false, "必須設定"},
		"設定":         {mac, true, ""},
		"換金鑰期間兩把":    {"h2:" + b64(3, 32) + "," + mac, true, ""},
		"HMAC 金鑰太短":  {"h1:" + b64(2, 16), false, "KYC_HASH_KEYS"},
		"HMAC 金鑰是空的": {"h1:", false, "KYC_HASH_KEYS"},
		"金鑰 ID 重複":   {mac + ",h1:" + b64(4, 32), false, "KYC_HASH_KEYS"},
	} {
		t.Run(name, func(t *testing.T) {
			env := config.FromMap(map[string]string{"KYC_HASH_KEYS": tc.mac})
			keys := readKYCHashKeys(env)
			err := env.Err()
			if (keys != nil) != tc.wantKeys {
				t.Fatalf("keys = %v，預期有金鑰 = %v（err = %v）", keys, tc.wantKeys, err)
			}
			if tc.wantErr == "" && err != nil {
				t.Fatalf("不該失敗：%v", err)
			}
			if tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Fatalf("err = %v，預期包含 %q", err, tc.wantErr)
			}
		})
	}
}
