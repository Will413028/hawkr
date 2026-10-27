package main

import (
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/platform/config"
)

// 測試用的值：長度符合規格，網址是保留的 .invalid 網域（不指向任何主機）。
func TestPayUniConfigIsAllOrNothingWithNoDefaults(t *testing.T) {
	const (
		url = "https://payuni.invalid"
		key = "0123456789abcdef0123456789abcdef" // gitleaks:allow（虛構值 0123…）
		iv  = "0123456789abcdef"
	)
	for name, tc := range map[string]struct {
		env        map[string]string
		wantClient bool
		wantErr    string
	}{
		"都沒有設定（預設是關閉，沒有任何預設網址）": {map[string]string{}, false, ""},
		"四個都設定": {map[string]string{
			"PAYUNI_BASE_URL": url, "PAYUNI_AGENT_ID": "agent", "PAYUNI_HASH_KEY": key, "PAYUNI_HASH_IV": iv,
		}, true, ""},
		"只設網址": {map[string]string{"PAYUNI_BASE_URL": url}, false, "必須一起設定"},
		"少一個金鑰": {map[string]string{
			"PAYUNI_BASE_URL": url, "PAYUNI_AGENT_ID": "agent", "PAYUNI_HASH_KEY": key,
		}, false, "必須一起設定"},
		"金鑰長度不對": {map[string]string{
			"PAYUNI_BASE_URL": url, "PAYUNI_AGENT_ID": "agent", "PAYUNI_HASH_KEY": "short", "PAYUNI_HASH_IV": iv,
		}, false, "HashKey"},
		"網址沒有 scheme": {map[string]string{
			"PAYUNI_BASE_URL": "payuni.invalid", "PAYUNI_AGENT_ID": "agent", "PAYUNI_HASH_KEY": key, "PAYUNI_HASH_IV": iv,
		}, false, "BaseURL"},
	} {
		t.Run(name, func(t *testing.T) {
			e := config.FromMap(tc.env)
			setup := readPayUni(e, slog.New(slog.NewTextHandler(io.Discard, nil)))
			err := e.Err()
			if (setup.client != nil) != tc.wantClient {
				t.Fatalf("client = %v，預期有 client = %v（err = %v）", setup.client, tc.wantClient, err)
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
