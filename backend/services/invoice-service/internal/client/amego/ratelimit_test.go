package amego

import (
	"context"
	"testing"
	"time"
)

// TestRateLimiterDegradeAndLinearRecovery：觸發 code 21 時砍半、5 分鐘後線性回升
// （避免整批同時被擋、也避免一恢復就再次觸頂）。
func TestRateLimiterDegradeAndLinearRecovery(t *testing.T) {
	t.Parallel()
	now := time.Unix(1787577174, 0).UTC()
	clock := &now
	r := newRateLimiter(10, func() time.Time { return *clock })

	if got := r.currentQPS(); got != 10 {
		t.Fatalf("初始 QPS = %v, want 10", got)
	}
	r.Degrade()
	if got := r.currentQPS(); got != 5 {
		t.Fatalf("降檔後 QPS = %v, want 5", got)
	}

	*clock = now.Add(rateRecoveryWindow / 2)
	_ = r.Wait(context.Background()) // Wait 內部觸發 recover
	if got := r.currentQPS(); got <= 5 || got >= 10 {
		t.Fatalf("回升中的 QPS = %v, want 介於 5 與 10 之間", got)
	}

	*clock = now.Add(rateRecoveryWindow + time.Second)
	_ = r.Wait(context.Background())
	if got := r.currentQPS(); got != 10 {
		t.Fatalf("回升完成後 QPS = %v, want 10", got)
	}
}

// TestRateLimiterWaitHonoursCallerContext。
//
// 🔴 限速排隊必須吃**該次呼叫自己的 ctx**：傳 context.Background 會讓排隊時間跑到
// 租約外面去偷租約 ⇒「單筆最壞耗時 < 租約」在高負載時無聲失效
// （《多副本排程搶佔規範》C1 的形狀）。
func TestRateLimiterWaitHonoursCallerContext(t *testing.T) {
	t.Parallel()
	r := newRateLimiter(1, time.Now)
	// 先把桶用光。
	if err := r.Wait(context.Background()); err != nil {
		t.Fatalf("第一次 Wait 不該排隊: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := r.Wait(ctx)
	if err == nil {
		t.Fatal("排隊超過 ctx deadline 時必須回錯，而不是無視 deadline 繼續等")
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("Wait 等了 %v，明顯超過呼叫端的 20ms deadline", elapsed)
	}
}

// TestRateLimiterLocalThrottleHookFires 對應
// invoice_amego_rate_limited_total{source="local"}。
func TestRateLimiterLocalThrottleHookFires(t *testing.T) {
	t.Parallel()
	r := newRateLimiter(1, time.Now)
	fired := 0
	r.onLocalThrottle = func() { fired++ }
	_ = r.Wait(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_ = r.Wait(ctx)
	if fired == 0 {
		t.Fatal("排隊時必須觸發 local throttle hook")
	}
}
