package model

import (
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/twtime"
)

func date(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

// 日以台北時間（UTC+8）為界。
func TestViewDayUsesTaipeiBoundaries(t *testing.T) {
	for _, tt := range []struct {
		name  string
		now   time.Time
		today time.Time
	}{
		{"台北 00:00 是新的一天", time.Date(2026, 2, 1, 0, 0, 0, 0, twtime.Taipei), date(2026, 2, 1)},
		{"台北 23:59:59 還是同一天", time.Date(2026, 1, 31, 23, 59, 59, 0, twtime.Taipei), date(2026, 1, 31)},
		// 2026-01-31 16:00 UTC 已經是台北的 2/1 00:00：UTC 的日曆日還是 1/31。
		{"UTC 16:00 是台北的隔天", time.Date(2026, 1, 31, 16, 0, 0, 0, time.UTC), date(2026, 2, 1)},
		{"UTC 15:59:59 還是台北的同一天", time.Date(2026, 1, 31, 15, 59, 59, 0, time.UTC), date(2026, 1, 31)},
		{"跨年", time.Date(2027, 1, 1, 8, 0, 0, 0, twtime.Taipei), date(2027, 1, 1)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := ViewDay(tt.now); !got.Equal(tt.today) {
				t.Fatalf("ViewDay=%s, want %s", got, tt.today)
			}
		})
	}
}

// 查詢的期間 [start, end)：兩端必須是台北 00:00，起點早於終點；回傳對應的日曆日範圍。
func TestViewRange(t *testing.T) {
	tp := func(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, twtime.Taipei) }
	first, end, err := ViewRange(tp(2026, 10, 3), tp(2026, 10, 10))
	if err != nil || !first.Equal(date(2026, 10, 3)) || !end.Equal(date(2026, 10, 10)) {
		t.Fatalf("一週: %s %s %v", first, end, err)
	}
	// 同一個瞬間以 UTC 表示（前一天 16:00）也是台北 00:00。
	if first, end, err = ViewRange(tp(2026, 10, 3).UTC(), tp(2026, 10, 4).UTC()); err != nil || !first.Equal(date(2026, 10, 3)) || !end.Equal(date(2026, 10, 4)) {
		t.Fatalf("UTC 表示: %s %s %v", first, end, err)
	}
	for name, tc := range map[string][2]time.Time{
		"起點不是台北 00:00": {tp(2026, 10, 3).Add(time.Hour), tp(2026, 10, 5)},
		"終點不是台北 00:00": {tp(2026, 10, 3), tp(2026, 10, 5).Add(-time.Nanosecond)},
		"UTC 午夜不是台北午夜": {date(2026, 10, 3), date(2026, 10, 5)},
		"空的期間":         {tp(2026, 10, 3), tp(2026, 10, 3)},
		"倒置":           {tp(2026, 10, 4), tp(2026, 10, 3)},
	} {
		if _, _, err := ViewRange(tc[0], tc[1]); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// 重算的區間：不可倒置、不可在未來，起日的台北 00:00 不可早於 now − 7 天。
func TestViewRebuildRange(t *testing.T) {
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, twtime.Taipei)
	day := func(d int) time.Time { return time.Date(2026, 10, d, 0, 0, 0, 0, time.UTC) }
	for _, tc := range []struct {
		name     string
		from, to time.Time
		ok       bool
	}{
		{"保留期內", day(4), day(10), true},
		{"起日的時間部分被捨去", day(3).Add(13 * time.Hour), day(10), false}, // 10/3 00:00 早於 10/3 12:00
		{"起日早於保留期", day(3), day(5), false},
		{"倒置", day(8), day(7), false},
		{"迄日在未來", day(8), day(11), false},
		{"單日", day(10), day(10), true},
	} {
		from, to, err := ViewRebuildRange(now, tc.from, tc.to)
		if (err == nil) != tc.ok {
			t.Fatalf("%s: err=%v", tc.name, err)
		}
		if err == nil && (from.Hour() != 0 || to.Location() != time.UTC) {
			t.Fatalf("%s: dates not normalized: %v %v", tc.name, from, to)
		}
	}
}
