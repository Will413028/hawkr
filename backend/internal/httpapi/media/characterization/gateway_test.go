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

// 這裡的測試打本機起的單體（入口，scripts/characterize.sh media）：舊的 REST 由單體轉給 gateway、再到 legacy.sh
// 起的 Java media-service（沒有物件儲存設定，B1-D12），只用 Postgres 與 Redis。身分是腳本以 hawkr dev-token 簽的 token。

// TestMain 起假的 media-service：單體的 MEDIA_SERVICE_ADDR 指向它（單體其他模組的 media port 要有人接），與被測的
// Java media-service 是兩回事。
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

// TestLegacy 經入口對舊的 gateway 與 Java media-service 跑情境（D8：先對舊實作跑過）。
func TestLegacy(t *testing.T) {
	d := &restDriver{rest: ch.REST{Base: ch.RequireEnv(t, ch.EnvEntryURL), Tokens: ch.EnvTokens(t), HTTP: &http.Client{Timeout: 10 * time.Second}}}
	ch.Run[Driver](t, d, scenarios)
}
