package main

import (
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/platform/config"
)

// 統編查詢是公開的政府資料，不需要金鑰：都沒設定時用預設網址與逾時；網址格式不合會讓啟動失敗。
func TestGCISConfig(t *testing.T) {
	e := config.FromMap(map[string]string{})
	if client := readGCIS(e); client == nil || e.Err() != nil {
		t.Fatalf("預設設定：client=%v err=%v", client, e.Err())
	}
	e = config.FromMap(map[string]string{"GCIS_BASE_URL": "http://127.0.0.1:9", "GCIS_TIMEOUT": "2s"})
	if client := readGCIS(e); client == nil || e.Err() != nil {
		t.Fatalf("自訂設定：client=%v err=%v", client, e.Err())
	}
	e = config.FromMap(map[string]string{"GCIS_BASE_URL": "data.gcis.nat.gov.tw"})
	readGCIS(e)
	if err := e.Err(); err == nil || !strings.Contains(err.Error(), "GCIS") {
		t.Fatalf("沒有 scheme 的網址通過了：%v", err)
	}
}
