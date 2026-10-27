package refsync

import (
	"testing"
	"time"
)

func TestNextRetryFollowsRiverBackoffUntilTheCap(t *testing.T) {
	at := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	for attempt, want := range map[int]time.Duration{1: time.Second, 2: 16 * time.Second, 7: 2401 * time.Second, 8: MaxBackoff, 25: MaxBackoff} {
		if got := NextRetry(at, attempt).Sub(at); got != want {
			t.Errorf("attempt %d：間隔 = %v，應為 %v", attempt, got, want)
		}
	}
}

// media 長時間故障後恢復，下一次重試不能落在 GC 的寬限（從資產建立起算 7 天）之後：任一時刻離下一次重試都不超過 MaxBackoff。
func TestNextRetryNeverWaitsPastTheGCGrace(t *testing.T) {
	const grace = 7 * 24 * time.Hour
	at := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	elapsed := time.Duration(0)
	for attempt := 1; attempt <= MaxAttempts; attempt++ {
		gap := NextRetry(at.Add(elapsed), attempt).Sub(at.Add(elapsed))
		if gap > MaxBackoff {
			t.Fatalf("attempt %d：間隔 %v 超過 MaxBackoff", attempt, gap)
		}
		elapsed += gap
	}
	if MaxBackoff >= grace {
		t.Fatalf("MaxBackoff %v 不小於 GC 寬限 %v", MaxBackoff, grace)
	}
}
