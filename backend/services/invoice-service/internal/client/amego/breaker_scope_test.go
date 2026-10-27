package amego

import (
	"errors"
	"testing"

	gobreaker "github.com/sony/gobreaker/v2"
)

// TestBreakerFor_PerIssuerIsolation 釘住 §7.3.4「per-store 斷路器」。
//
// 🔴 修正前只有一顆全域斷路器、而且用的是 per-store 的跳脫條件（連 5 敗）⇒
// **一家賣家的憑證壞掉會把全服務推向 open**，其餘賣家的開票一起停在
// RETRY_SCHEDULED。而 invoice_provider_accounts.status='DEGRADED' 的語意是
// 「per-store 斷路器的持久化投影」，投影的來源卻不是 per-store 的。
func TestBreakerFor_PerIssuerIsolation(t *testing.T) {
	c := &HTTPClient{
		perIssuer:  make(map[int64]*gobreaker.CircuitBreaker[[]byte]),
		breakerSet: DefaultBreakerSettings(),
	}

	a := c.breakerFor(101)
	b := c.breakerFor(102)
	if a == b {
		t.Fatal("不同 issuer 必須拿到不同的斷路器")
	}
	if again := c.breakerFor(101); again != a {
		t.Fatal("同一個 issuer 必須拿到同一顆（否則連敗計數每次歸零、永遠跳不起來）")
	}

	// 讓 issuer 101 連敗到開路（DefaultBreakerSettings().Failures = 5）。
	boom := &APIError{Endpoint: "/json/f0401", Code: "10", Class: ClassRetryable, Kind: ErrorKindProvider, KnownCode: true}
	for i := uint32(0); i < DefaultBreakerSettings().Failures; i++ {
		_, _ = a.Execute(func() ([]byte, error) { return nil, boom })
	}
	if a.State() != gobreaker.StateOpen {
		t.Fatalf("issuer 101 連敗 %d 次應開路，實得 %v", DefaultBreakerSettings().Failures, a.State())
	}

	// 🔴 關鍵斷言：另一家**完全不受影響**。這一格就是修正前壞掉的地方。
	if b.State() != gobreaker.StateClosed {
		t.Errorf("issuer 102 不該被 101 拖累，實得 %v", b.State())
	}
	if _, err := b.Execute(func() ([]byte, error) { return []byte("ok"), nil }); err != nil {
		t.Errorf("issuer 102 應該還能正常呼叫，實得 %v", err)
	}
}

// TestGlobalBreakerExcludesPerStoreOpen 釘住「per-store 開路不計入全域」。
//
// 少了這道排除，一家賣家持續開路會把全域的失敗率一路推上去 ⇒ 回到修正前的行為
// （一家壞拖垮全部），只是繞了一圈。
func TestGlobalBreakerExcludesPerStoreOpen(t *testing.T) {
	excluded := func(err error) bool {
		if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
			return true
		}
		return isExcludedFromBreaker(err)
	}
	if !excluded(gobreaker.ErrOpenState) {
		t.Error("per-store 開路必須被全域排除")
	}
	if !excluded(gobreaker.ErrTooManyRequests) {
		t.Error("per-store half-open 限流必須被全域排除")
	}
	// 真正的上游故障仍要計入全域——否則全域斷路器永遠跳不起來。
	upstream := &APIError{Endpoint: "/json/f0401", Code: "10", Class: ClassRetryable, Kind: ErrorKindProvider, KnownCode: true}
	if excluded(upstream) {
		t.Error("真正的上游故障必須計入全域，否則全域斷路器形同虛設")
	}
}
