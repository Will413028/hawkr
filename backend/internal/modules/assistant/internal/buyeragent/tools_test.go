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

func TestStatusLabelFallsBackToTheCode(t *testing.T) {
	if got := statusLabel("30"); got != "配送中" {
		t.Errorf("30 = %q", got)
	}
	if got := statusLabel("999"); got != "999" {
		t.Errorf("未知狀態碼要原樣回傳，得到 %q", got)
	}
}
