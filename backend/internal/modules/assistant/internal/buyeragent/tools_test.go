package buyeragent

import (
	"testing"
	"time"
)

func TestFilterByDateRange(t *testing.T) {
	loc := time.FixedZone("test", 8*3600)
	now := time.Date(2026, 11, 20, 15, 0, 0, 0, loc)
	at := func(d time.Time) Order { return Order{OrderNo: d.Format(time.RFC3339), CreateTime: d} }
	startOfToday := time.Date(2026, 11, 20, 0, 0, 0, 0, loc)
	orders := []Order{
		at(startOfToday),                        // 今天 00:00 剛好算今天
		at(startOfToday.Add(-time.Millisecond)), // 昨天最後一毫秒
		at(now.AddDate(0, 0, -7)),               // 剛好 7 天前
		at(now.AddDate(0, 0, -7).Add(-time.Millisecond)),
	}
	cases := []struct {
		dateRange string
		want      int
	}{
		{"today", 1},
		{"last_7_days", 3},
		{"all", 4},
		{"", 4},
		{"unknown", 4}, // 照搬：不認得的範圍等於不篩
	}
	for _, c := range cases {
		if got := len(filterByDateRange(orders, c.dateRange, now)); got != c.want {
			t.Errorf("%q：%d 筆，要 %d", c.dateRange, got, c.want)
		}
	}
}

func TestFilterByDateRangeTodayIsTheTaipeiDay(t *testing.T) {
	// 2026-11-20 23:00 UTC 是台北 11-21 07:00；今天從台北 11-21 00:00（UTC 11-20 16:00）起算。
	now := time.Date(2026, 11, 20, 23, 0, 0, 0, time.UTC)
	at := func(d time.Time) Order { return Order{OrderNo: d.String(), CreateTime: d} }
	orders := []Order{
		at(time.Date(2026, 11, 20, 16, 0, 0, 0, time.UTC)),                        // 台北今天 00:00
		at(time.Date(2026, 11, 20, 16, 0, 0, 0, time.UTC).Add(-time.Millisecond)), // 台北昨天最後一毫秒
		at(time.Date(2026, 11, 20, 8, 0, 0, 0, time.UTC)),                         // 台北昨天 16:00
	}
	if got := len(filterByDateRange(orders, "today", now)); got != 1 {
		t.Errorf("today：%d 筆，要 1", got)
	}
}
