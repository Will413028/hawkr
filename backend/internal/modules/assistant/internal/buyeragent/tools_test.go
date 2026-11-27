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
		at(startOfToday),
		at(startOfToday.Add(-time.Millisecond)),
		at(now.AddDate(0, 0, -7)),
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
		{"unknown", 4},
	}
	for _, c := range cases {
		if got := len(filterByDateRange(orders, c.dateRange, now)); got != c.want {
			t.Errorf("%q：%d 筆，要 %d", c.dateRange, got, c.want)
		}
	}
}

func TestFilterByDateRangeTodayIsTheTaipeiDay(t *testing.T) {
	now := time.Date(2026, 11, 20, 23, 0, 0, 0, time.UTC)
	at := func(d time.Time) Order { return Order{OrderNo: d.String(), CreateTime: d} }
	orders := []Order{
		at(time.Date(2026, 11, 20, 16, 0, 0, 0, time.UTC)),
		at(time.Date(2026, 11, 20, 16, 0, 0, 0, time.UTC).Add(-time.Millisecond)),
		at(time.Date(2026, 11, 20, 8, 0, 0, 0, time.UTC)),
	}
	if got := len(filterByDateRange(orders, "today", now)); got != 1 {
		t.Errorf("today：%d 筆，要 1", got)
	}
}
