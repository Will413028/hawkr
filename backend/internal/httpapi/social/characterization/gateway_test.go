//go:build gateway

package characterization

import (
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	ch "github.com/Will413028/hawkr/backend/internal/httpapi/characterize"
)

// 這裡的測試打本機起的單體（入口，scripts/characterize.sh social）：舊的 REST 由單體轉給 gateway、再到舊的
// social-service（legacy.sh 起它），只用本機的 Postgres 與 Redis；呼叫 media-service 的是這個測試聽的假服務。
// 身分是腳本以 hawkr dev-token 簽的 token（兩位買家、兩家店的賣家、管理員）。單體的那一份在 A7-2 加上。

// TestMain 起假的 media-service，位址由腳本同時設定給單體與舊服務。
func TestMain(m *testing.M) {
	stop, err := ch.ServeFakeMedia(os.Getenv(ch.EnvFakeMediaAddr))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	code := m.Run()
	stop()
	os.Exit(code)
}

// TestLegacy 經入口對舊的 gateway＋social-service 跑同一份情境（D8：先對舊實作跑過）。
func TestLegacy(t *testing.T) {
	tokens := ch.EnvTokens(t)
	for _, c := range []ch.Caller{ch.Seller, ch.Seller2, ch.Buyer2} {
		if tokens[c] == "" {
			t.Fatalf("缺少呼叫者 %d 的 token：用 scripts/characterize.sh social 執行", c)
		}
	}
	d := newRESTDriver(ch.RequireEnv(t, ch.EnvEntryURL), tokens, &http.Client{Timeout: 10 * time.Second})
	ch.Run[Driver](t, d, scenarios)
}
