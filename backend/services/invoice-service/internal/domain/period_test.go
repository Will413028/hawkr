package domain

import (
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/timeutil"
)

func utc(y int, m time.Month, d, h, min int) time.Time {
	return time.Date(y, m, d, h, min, 0, 0, time.UTC)
}

func TestAmegoPeriod(t *testing.T) {
	cases := []struct {
		name       string
		in         time.Time
		year, want int
	}{
		{"1 月 → 期別 0", utc(2026, time.January, 15, 3, 0), 2026, 0},
		{"3 月 → 期別 1", utc(2026, time.March, 15, 3, 0), 2026, 1},
		{"5 月 → 期別 2", utc(2026, time.May, 15, 3, 0), 2026, 2},
		{"7 月 → 期別 3", utc(2026, time.July, 15, 3, 0), 2026, 3},
		{"9 月 → 期別 4", utc(2026, time.September, 15, 3, 0), 2026, 4},
		{"11 月 → 期別 5", utc(2026, time.November, 15, 3, 0), 2026, 5},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			y, p := AmegoPeriod(c.in)
			if y != c.year || p != c.want {
				t.Fatalf("AmegoPeriod = (%d, %d), want (%d, %d)", y, p, c.year, c.want)
			}
		})
	}
}

// 跨期邊界的那八個小時：UTC 還在 2 月、台北已經是 3 月。
// 直接拿 UTC 的月份會落到前一期，而錯誤只在每兩個月的邊界日出現。
func TestAmegoPeriod_CrossPeriodBoundaryWindow(t *testing.T) {
	// 2026-02-28 16:00Z ＝ 2026-03-01 00:00 台北
	in := utc(2026, time.February, 28, 16, 0)
	if got := in.In(timeutil.LocationTaipei).Month(); got != time.March {
		t.Fatalf("fixture wrong: taipei month = %v", got)
	}
	y, p := AmegoPeriod(in)
	if y != 2026 || p != 1 {
		t.Fatalf("AmegoPeriod = (%d, %d), want (2026, 1) —— 用 UTC 月份會得到 0", y, p)
	}
	if got := TaxPeriodCode(in); got != "11502" {
		t.Fatalf("TaxPeriodCode = %q, want \"11502\" —— 用 UTC 月份會得到 \"11501\"", got)
	}
	if got := InvoiceDateTaipei(in); got.Year() != 2026 || got.Month() != time.March || got.Day() != 1 {
		t.Fatalf("InvoiceDateTaipei = %v, want 2026-03-01", got)
	}
	// 邊界前一分鐘仍屬上一期。
	before := utc(2026, time.February, 28, 15, 59)
	if _, p := AmegoPeriod(before); p != 0 {
		t.Fatalf("one minute earlier should still be period 0, got %d", p)
	}
	if got := TaxPeriodCode(before); got != "11501" {
		t.Fatalf("one minute earlier TaxPeriodCode = %q, want \"11501\"", got)
	}
}

// 跨年邊界：台北 2027-01-01 而 UTC 還在 2026-12-31。
func TestAmegoPeriod_YearBoundary(t *testing.T) {
	in := utc(2026, time.December, 31, 16, 0)
	y, p := AmegoPeriod(in)
	if y != 2027 || p != 0 {
		t.Fatalf("AmegoPeriod = (%d, %d), want (2027, 0)", y, p)
	}
	if got := TaxPeriodCode(in); got != "11601" {
		t.Fatalf("TaxPeriodCode = %q, want \"11601\"", got)
	}
}

func TestTaxPeriodCode(t *testing.T) {
	cases := []struct {
		in   time.Time
		want string
	}{
		{utc(2026, time.January, 2, 3, 0), "11501"},
		{utc(2026, time.April, 2, 3, 0), "11502"},
		{utc(2026, time.August, 15, 3, 0), "11504"},
		{utc(2026, time.December, 1, 3, 0), "11506"},
	}
	for _, c := range cases {
		if got := TaxPeriodCode(c.in); got != c.want {
			t.Errorf("TaxPeriodCode(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

// 兩套編碼差 1、年基準也不同 ⇒ 不得互相推導。
func TestTwoPeriodEncodingsAreNotInterchangeable(t *testing.T) {
	in := utc(2026, time.August, 15, 3, 0)
	code := TaxPeriodCode(in) // "11504"
	_, amego := AmegoPeriod(in)
	if amego != 3 {
		t.Fatalf("AmegoPeriod = %d, want 3", amego)
	}
	if got := int(code[4] - '0'); got-1 != amego {
		t.Fatalf("期別半段差 1 的關係被破壞：code=%q amego=%d", code, amego)
	}
	_, roc, err := parseForTest(code)
	if err != nil || roc != 4 {
		t.Fatalf("ParseTaxPeriodCode(%q) period = %d, err = %v", code, roc, err)
	}
}

func parseForTest(code string) (year, period int, err error) { return ParseTaxPeriodCode(code) }

func TestParseTaxPeriodCode(t *testing.T) {
	y, p, err := ParseTaxPeriodCode("11504")
	if err != nil || y != 2026 || p != 4 {
		t.Fatalf("= (%d, %d, %v), want (2026, 4, nil)", y, p, err)
	}
	for _, bad := range []string{"", "1150", "115040", "1150a", "11500", "11507"} {
		if _, _, err := ParseTaxPeriodCode(bad); err == nil {
			t.Errorf("ParseTaxPeriodCode(%q) = nil error, want error", bad)
		}
	}
}

func taipei(y int, m time.Month, d, h, mi, s int) time.Time {
	return time.Date(y, m, d, h, mi, s, 0, timeutil.LocationTaipei)
}

func TestPeriodBounds(t *testing.T) {
	cases := []struct {
		code                    string
		end, filingDue, closeDL time.Time
	}{
		{"11501",
			taipei(2026, time.February, 28, 23, 59, 59),
			taipei(2026, time.March, 15, 23, 59, 59),
			taipei(2026, time.March, 5, 23, 59, 59)},
		{"11504",
			taipei(2026, time.August, 31, 23, 59, 59),
			taipei(2026, time.September, 15, 23, 59, 59),
			taipei(2026, time.September, 5, 23, 59, 59)},
		// 期別 6 跨年
		{"11506",
			taipei(2026, time.December, 31, 23, 59, 59),
			taipei(2027, time.January, 15, 23, 59, 59),
			taipei(2027, time.January, 5, 23, 59, 59)},
		// 閏年 2 月
		{"11301",
			taipei(2024, time.February, 29, 23, 59, 59),
			taipei(2024, time.March, 15, 23, 59, 59),
			taipei(2024, time.March, 5, 23, 59, 59)},
	}
	for _, c := range cases {
		t.Run(c.code, func(t *testing.T) {
			end, err := PeriodEndTaipei(c.code)
			if err != nil || !end.Equal(c.end) {
				t.Errorf("PeriodEndTaipei = %v (%v), want %v", end, err, c.end)
			}
			due, err := PeriodFilingDueTaipei(c.code)
			if err != nil || !due.Equal(c.filingDue) {
				t.Errorf("PeriodFilingDueTaipei = %v (%v), want %v", due, err, c.filingDue)
			}
			dl, err := ProviderCloseDeadlineTaipei(c.code)
			if err != nil || !dl.Equal(c.closeDL) {
				t.Errorf("ProviderCloseDeadlineTaipei = %v (%v), want %v", dl, err, c.closeDL)
			}
		})
	}
}

// 關帳日的硬牆：每單月 6 日 00:00:00 起上期發票被鎖。
func TestWithinVoidWindow(t *testing.T) {
	cases := []struct {
		name string
		now  time.Time
		want bool
	}{
		{"期內", taipei(2026, time.February, 10, 12, 0, 0), true},
		{"跨期後第 5 日最後一秒", taipei(2026, time.March, 5, 23, 59, 59), true},
		{"第 6 日 00:00:00 —— 硬牆", taipei(2026, time.March, 6, 0, 0, 0), false},
		{"更晚", taipei(2026, time.March, 20, 0, 0, 0), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := WithinVoidWindow("11501", c.now)
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Fatalf("WithinVoidWindow = %v, want %v", got, c.want)
			}
		})
	}
	// 以 UTC 傳入同一時刻也必須得到同樣答案（函式自己轉時區）。
	got, err := WithinVoidWindow("11501", taipei(2026, time.March, 6, 0, 0, 0).UTC())
	if err != nil || got {
		t.Fatalf("UTC 傳入應得同一答案，got %v (%v)", got, err)
	}
}

func TestReportedStatus(t *testing.T) {
	cases := []struct {
		name string
		now  time.Time
		want string
	}{
		{"期別尚未終結", taipei(2026, time.February, 1, 0, 0, 0), ReportedStatusNotReported},
		{"期末最後一秒仍未終結", taipei(2026, time.February, 28, 23, 59, 58), ReportedStatusNotReported},
		{"期已結、限未到 ⇒ fail-closed", taipei(2026, time.March, 1, 0, 0, 0), ReportedStatusUnknown},
		{"申報期限當日最後一秒仍是 UNKNOWN", taipei(2026, time.March, 15, 23, 59, 59), ReportedStatusUnknown},
		{"申報期限已過", taipei(2026, time.March, 16, 0, 0, 0), ReportedStatusReported},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ReportedStatus("11501", c.now)
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Fatalf("ReportedStatus = %q, want %q", got, c.want)
			}
		})
	}
	if _, err := ReportedStatus("bad", time.Now()); err == nil {
		t.Fatal("bad period code should error, not silently return a status")
	}
}
