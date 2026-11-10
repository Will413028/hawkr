package account

import (
	"testing"
	"time"
)

// 賣家觸發的重讀只在快取不夠新時才向 provider 讀（D33）。時間都是注入的值，不睡眠。
// mutation 1：readIsFresh 永遠回 false（沒有 debounce）→ 窗內的列失敗。
// mutation 2：readIsFresh 永遠回 true（永遠不讀）→ 沒讀過與滿窗的列失敗。
// mutation 3：邊界改成 <= → 剛好滿 ReadDebounce 的列失敗。
func TestReadIsFreshOnlyInsideTheDebounceWindow(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	at := func(d time.Duration) *time.Time { v := now.Add(-d); return &v }
	cases := []struct {
		name      string
		fetchedAt *time.Time
		fresh     bool
	}{
		{"never read", nil, false},
		{"read just now", at(0), true},
		{"inside the window", at(ReadDebounce - time.Nanosecond), true},
		{"exactly the window", at(ReadDebounce), false},
		{"long ago", at(time.Hour), false},
		{"fetched_at ahead of this instance's clock", at(-time.Minute), true},
	}
	for _, c := range cases {
		if got := readIsFresh(c.fetchedAt, now); got != c.fresh {
			t.Errorf("%s：readIsFresh = %v，要 %v", c.name, got, c.fresh)
		}
	}
}
