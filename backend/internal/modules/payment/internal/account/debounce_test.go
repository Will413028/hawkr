package account

import (
	"testing"
	"time"
)

// 賣家觸發的重讀的搶名額門檻（D33）：now 往前推 ReadDebounce。時間都是注入的值，不睡眠。
// mutation 1：staleBefore 回 now（沒有 debounce）→ 失敗。
// mutation 2：方向改成 now.Add(ReadDebounce)（永遠搶不到）→ 失敗。
func TestStaleBeforeIsOneDebounceWindowBack(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	if got, want := staleBefore(now), now.Add(-10*time.Second); !got.Equal(want) {
		t.Fatalf("staleBefore = %v，要 %v", got, want)
	}
	if ReadDebounce != 10*time.Second {
		t.Fatalf("ReadDebounce = %v，D33 是 10 秒", ReadDebounce)
	}
}
