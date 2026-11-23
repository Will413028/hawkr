//go:build gateway

package characterization

import (
	"net/http"
	"testing"
	"time"

	ch "github.com/Will413028/hawkr/backend/internal/httpapi/characterize"
)

// 這裡的測試打本機起的單體（入口，scripts/characterize.sh coupon）：舊的 REST 由單體轉給 gateway、再到 legacy.sh
// 起的 Java coupon-service，只用 Postgres、Redis 與 RabbitMQ。身分是 legacy.sh 以 hawkr dev-token 簽的 token
// （兩位買家、兩家店的賣家）加腳本簽的管理員。

// TestLegacy 經入口對舊的 gateway 與 Java coupon-service 跑情境（D8：先對舊實作跑過）。
func TestLegacy(t *testing.T) {
	tokens := ch.EnvTokens(t)
	for _, c := range []ch.Caller{ch.Seller, ch.Seller2, ch.Buyer2} {
		if tokens[c] == "" {
			t.Fatalf("缺少呼叫者 %d 的 token：用 scripts/characterize.sh coupon 執行", c)
		}
	}
	d := newRESTDriver(ch.RequireEnv(t, ch.EnvEntryURL), tokens, &http.Client{Timeout: 20 * time.Second})
	ch.Run[Driver](t, d, scenarios)
}
