package payuni

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"
)

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
	_ = r.Wait(context.Background())
	if got := r.currentQPS(); got <= 5 || got >= 10 {
		t.Fatalf("回升中的 QPS = %v, want 介於 5 與 10 之間", got)
	}

	*clock = now.Add(rateRecoveryWindow + time.Second)
	_ = r.Wait(context.Background())
	if got := r.currentQPS(); got != 10 {
		t.Fatalf("回升完成後 QPS = %v, want 10", got)
	}
}

func TestRateLimiterWaitHonoursCallerContext(t *testing.T) {
	t.Parallel()
	r := newRateLimiter(1, time.Now)
	if err := r.Wait(context.Background()); err != nil {
		t.Fatalf("第一次 Wait 不該排隊: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := r.Wait(ctx); err == nil {
		t.Fatal("桶已用光且 ctx 逾時，Wait 必須回傳錯誤而不是無限等待")
	}
}

func TestRateLimiterFiresCallbacks(t *testing.T) {
	t.Parallel()
	throttled, degraded := 0, 0
	r := newRateLimiter(1, time.Now)
	r.onThrottle = func() { throttled++ }
	r.onDegrade = func() { degraded++ }

	if err := r.Wait(context.Background()); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if throttled != 0 {
		t.Errorf("onThrottle 不該在還有 token 時觸發，got %d", throttled)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_ = r.Wait(ctx)
	if throttled != 1 {
		t.Errorf("onThrottle 呼叫次數 = %d, want 1", throttled)
	}

	r.Degrade()
	if degraded != 1 {
		t.Errorf("onDegrade 呼叫次數 = %d, want 1", degraded)
	}
}

func TestLooksRateLimited(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil 不是被擋", nil, false},
		{"429 是被擋", fmt.Errorf("payuni api status 429: rate limited"), true},
		{"503 是被擋", fmt.Errorf("payuni api status 503: unavailable"), true},
		{"連線被拒是被擋（IP 層阻擋的典型外觀）", &net.OpError{Op: "dial", Err: errors.New("connection refused")}, true},
		{"連線被重置是被擋", errors.New("read tcp: connection reset by peer"), true},
		{"400 不是被擋", fmt.Errorf("payuni api status 400: bad request"), false},
		{"500 不是被擋", fmt.Errorf("payuni api status 500: internal error"), false},
		{"一般業務錯誤不是被擋", errors.New("AGENT04017 商店代號已存在"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := looksRateLimited(tt.err); got != tt.want {
				t.Errorf("looksRateLimited(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
