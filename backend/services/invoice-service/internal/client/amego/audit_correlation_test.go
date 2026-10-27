package amego

import (
	"context"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
)

// 🔴 這一格守的是 #3031 的實際失效模式，不是 helper 的三個分支。
//
// 修復第一版只做到「ctx 有就讀」，而 invoice_provider_audit_logs 的絕大多數列是
// **排程 worker** 寫的：worker 沒有請求可種 correlation id，本服務也沒有任何地方
// 起 OTel span ⇒ 那一欄部署之後仍然全空（2026-09-02 dev 實測 6 列全 NULL）。
// 所以真正要斷言的是「**什麼都沒有**的 ctx 也必須拿得到一個 id」。
func TestEnsureCorrelationID_SeedsWhenContextHasNothing(t *testing.T) {
	got := interceptor.CorrelationIDFromContext(ensureCorrelationID(context.Background()))
	if got == "" {
		t.Fatal("空 ctx 沒有被種進 correlation id —— 排程路徑的稽核列會恆為 NULL，" +
			"而 ProviderExchangeItem.request_id 與 last_request_id 兩個對外欄位以它為唯一來源")
	}
}

// 已有值時不得覆蓋：同一次請求打多次 AMEGO 應共用同一個 id，
// 否則 ADM 上同一次操作的數列往返會看起來互不相干。
func TestEnsureCorrelationID_KeepsExisting(t *testing.T) {
	ctx := interceptor.WithCorrelationID(context.Background(), "req-abc")
	if got := interceptor.CorrelationIDFromContext(ensureCorrelationID(ctx)); got != "req-abc" {
		t.Fatalf("correlation id 被覆蓋成 %q，應保留 req-abc", got)
	}
}

// 每一次往返各自一個 id（沒有請求脈絡時）。相同代表拿了某個全域值，
// 那會讓所有列指向同一個識別碼，等於沒有識別。
func TestEnsureCorrelationID_DistinctPerCall(t *testing.T) {
	a := interceptor.CorrelationIDFromContext(ensureCorrelationID(context.Background()))
	b := interceptor.CorrelationIDFromContext(ensureCorrelationID(context.Background()))
	if a == b {
		t.Fatalf("兩次呼叫拿到同一個 id %q", a)
	}
}
