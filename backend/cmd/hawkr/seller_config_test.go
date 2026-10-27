package main

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/platform/config"
)

func b64(b byte, n int) string { return base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{b}, n)) }

func TestKYCKeysAreRequiredAndChecked(t *testing.T) {
	enc, mac := "e1:"+b64(1, 32), "h1:"+b64(2, 32)
	for name, tc := range map[string]struct {
		enc, mac string
		wantKeys bool
		wantErr  string
	}{
		"都沒有設定":      {"", "", false, "必須設定"},
		"兩個都設定":      {enc, mac, true, ""},
		"只設加密金鑰":     {enc, "", false, "必須設定"},
		"只設 HMAC 金鑰": {"", mac, false, "必須設定"},
		"HMAC 金鑰太短":  {enc, "h1:" + b64(2, 16), false, "KYC_HASH_KEYS"},
		"加密金鑰的長度不對":  {"e1:" + b64(1, 16), mac, false, "KYC_ENCRYPTION_KEYS"},
		"HMAC 金鑰是空的": {enc, "h1:", false, "KYC_HASH_KEYS"},
		// 啟動檢查：HMAC 金鑰不得與加密金鑰相同（同一把金鑰兩種用途）。
		"HMAC 金鑰與加密金鑰相同": {enc, "h1:" + b64(1, 32), false, "相同"},
		"換金鑰期間其中一把相同":    {enc, "h2:" + b64(3, 32) + ",h1:" + b64(1, 32), false, "相同"},
	} {
		t.Run(name, func(t *testing.T) {
			env := config.FromMap(map[string]string{"KYC_ENCRYPTION_KEYS": tc.enc, "KYC_HASH_KEYS": tc.mac})
			keys := readKYCKeys(env)
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
