package bizcalendar

import (
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/platform/twtime"
)

func d(s string) time.Time {
	t, err := time.ParseInLocation(time.DateOnly, s, time.UTC)
	if err != nil {
		panic(err)
	}
	return t
}

func mustParse(t *testing.T, year int, content string) []Day {
	t.Helper()
	days, err := ParseOfficeCalendar(year, content)
	if err != nil {
		t.Fatal(err)
	}
	return days
}

// testCalendar 涵蓋 2026、2027（政府的實際檔）與 2030（合成：2030-01-05 週六補班、2030-01-02 週三放假）；2028、2029、2031 沒有涵蓋。
func testCalendar(t *testing.T) *Calendar {
	t.Helper()
	days := mustParse(t, 2026, readFixture(t, "115.csv"))
	days = append(days, mustParse(t, 2027, readFixture(t, "116.csv"))...)
	days = append(days, mustParse(t, 2030, csvOf(2030, func(date, line string) string {
		switch date {
		case "20300105":
			return "20300105,六,0,補班"
		case "20300102":
			return "20300102,三,2,颱風假"
		}
		return line
	}))...)
	c, err := NewCalendar(days)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestIsBusinessDay(t *testing.T) {
	c := testCalendar(t)
	tests := []struct {
		day              string
		business, covers bool
	}{
		{"2026-01-01", false, true},  // 週四的國定假日
		{"2026-01-02", true, true},   // 週五
		{"2026-01-03", false, true},  // 週六
		{"2030-01-05", true, true},   // 補班的週六：旗標 0 就是工作天
		{"2030-01-02", false, true},  // 平日的假日：旗標 2
		{"2030-01-07", true, true},   // 一般週一
		{"2025-12-31", false, false}, // 第一個涵蓋年之前
		{"2028-01-01", false, false}, // 中間沒有涵蓋的年
		{"2027-12-31", false, true},  // 最後一個連續涵蓋年的最後一天（旗標 2）
		{"2031-01-01", false, false}, // 最後一個涵蓋年之後
	}
	for _, tc := range tests {
		business, ok := c.IsBusinessDay(d(tc.day))
		if business != tc.business || ok != tc.covers {
			t.Errorf("%s：business=%v ok=%v，要 %v、%v", tc.day, business, ok, tc.business, tc.covers)
		}
	}
}

func TestUncoveredDayIsNeverReportedAsBusinessDay(t *testing.T) {
	c := testCalendar(t)
	// 2028-01-03 是週一：不能因為是平日就回 true。
	if business, ok := c.IsBusinessDay(d("2028-01-03")); ok || business {
		t.Fatalf("business=%v ok=%v", business, ok)
	}
	if _, ok := (&Calendar{}).IsBusinessDay(d("2026-01-02")); ok {
		t.Fatal("零值 Calendar 沒有涵蓋任何年份")
	}
}

func TestIsBusinessDayUsesTheTaipeiDay(t *testing.T) {
	c := testCalendar(t)
	// UTC 2026-12-31 16:00 已是台北 2027-01-01 00:00（放假）；差一分鐘還是 12 月 31 日（上班）。
	if b, ok := c.IsBusinessDay(time.Date(2026, 12, 31, 16, 0, 0, 0, time.UTC)); !ok || b {
		t.Errorf("台北 2027-01-01：business=%v ok=%v", b, ok)
	}
	if b, ok := c.IsBusinessDay(time.Date(2026, 12, 31, 15, 59, 0, 0, time.UTC)); !ok || !b {
		t.Errorf("台北 2026-12-31：business=%v ok=%v", b, ok)
	}
	if b, ok := c.IsBusinessDay(time.Date(2026, 1, 2, 0, 30, 0, 0, twtime.Taipei)); !ok || !b {
		t.Errorf("台北 2026-01-02 凌晨：business=%v ok=%v", b, ok)
	}
}

func TestAddBusinessDays(t *testing.T) {
	c := testCalendar(t)
	tests := []struct {
		name string
		from string
		n    int
		want string
		ok   bool
	}{
		{"n=1 跳過週末", "2026-01-09", 1, "2026-01-12", true},
		{"from 本身不算", "2026-01-12", 1, "2026-01-13", true},
		{"n=3", "2026-01-12", 3, "2026-01-15", true},
		{"n=0 回 from 的日曆日", "2026-01-03", 0, "2026-01-03", true},
		{"跨兩個涵蓋年與年假", "2026-12-29", 3, "2027-01-04", true},
		{"補班的週六算工作天", "2030-01-04", 1, "2030-01-05", true},
		{"平日的假日不算", "2030-01-01", 1, "2030-01-03", true},
		{"from 在涵蓋年之前、第一個被數的日子已涵蓋", "2025-12-31", 1, "2026-01-02", true},
		{"數到沒有涵蓋的年", "2027-12-30", 1, "", false},
		{"全在涵蓋年內的最後一個工作天", "2027-12-29", 1, "2027-12-30", true},
		{"from 在沒有涵蓋的年", "2028-06-01", 1, "", false},
		{"n 為負", "2026-01-12", -1, "", false},
		{"數得太遠超出涵蓋", "2026-01-12", 1000, "", false},
	}
	for _, tc := range tests {
		got, ok := c.AddBusinessDays(d(tc.from), tc.n)
		if ok != tc.ok || (ok && !got.Equal(d(tc.want))) {
			t.Errorf("%s：AddBusinessDays(%s, %d) = %s, %v；要 %s, %v", tc.name, tc.from, tc.n, got.Format(time.DateOnly), ok, tc.want, tc.ok)
		}
	}
}

func TestBusinessDaysBetween(t *testing.T) {
	c := testCalendar(t)
	tests := []struct {
		name     string
		from, to string
		want     int
		ok       bool
	}{
		{"同一天", "2026-01-12", "2026-01-12", 0, true},
		{"(from, to]", "2026-01-12", "2026-01-15", 3, true},
		{"反向取負", "2026-01-15", "2026-01-12", -3, true},
		{"跨年", "2026-12-29", "2027-01-04", 3, true},
		{"跨年反向", "2027-01-04", "2026-12-29", -3, true},
		{"補班週六", "2030-01-04", "2030-01-07", 2, true},
		{"起點在第一個涵蓋年之前", "2025-12-25", "2026-01-05", 0, false},
		{"終點在最後涵蓋年之後", "2027-12-29", "2028-01-05", 0, false},
		{"跨過沒有涵蓋的年", "2027-12-29", "2030-01-07", 0, false},
		{"反向跨出涵蓋", "2028-01-05", "2027-12-29", 0, false},
	}
	for _, tc := range tests {
		got, ok := c.BusinessDaysBetween(d(tc.from), d(tc.to))
		if got != tc.want || ok != tc.ok {
			t.Errorf("%s：%s → %s = %d, %v；要 %d, %v", tc.name, tc.from, tc.to, got, ok, tc.want, tc.ok)
		}
	}
}

func TestCoverageAndHolidays(t *testing.T) {
	c := testCalendar(t)
	if got := c.Years(); len(got) != 3 || got[0] != 2026 || got[1] != 2027 || got[2] != 2030 {
		t.Fatalf("Years = %v", got)
	}
	cov := c.Coverage()
	if len(cov) != 3 || cov[0] != (YearCoverage{2026, 365, 120}) || cov[1] != (YearCoverage{2027, 365, 121}) {
		t.Fatalf("Coverage = %+v", cov)
	}
	got := c.HolidaysBetween(d("2026-12-30"), d("2027-01-04"))
	want := []string{"2027-01-01", "2027-01-02", "2027-01-03"}
	if len(got) != len(want) {
		t.Fatalf("HolidaysBetween = %v", got)
	}
	for i, w := range want {
		if !got[i].Equal(d(w)) {
			t.Errorf("[%d] = %v，要 %s", i, got[i], w)
		}
	}
	if got := c.HolidaysBetween(d("2028-01-01"), d("2029-12-31")); len(got) != 0 {
		t.Errorf("沒有涵蓋的範圍不會有結果：%v", got)
	}
}

func TestNewCalendarRejectsInconsistentRows(t *testing.T) {
	days := mustParse(t, 2026, readFixture(t, "115.csv"))
	if _, err := NewCalendar(days[1:]); err == nil || !strings.Contains(err.Error(), "364 天") {
		t.Errorf("缺一天：%v", err)
	}
	if _, err := NewCalendar(append(append([]Day{}, days...), days[0])); err == nil || !strings.Contains(err.Error(), "出現兩次") {
		t.Errorf("重複：%v", err)
	}
	if c, err := NewCalendar(nil); err != nil || len(c.Years()) != 0 {
		t.Errorf("空：%v", err)
	}
}

// 期望值全部硬寫日期。
func TestCoveredThrough(t *testing.T) {
	c := testCalendar(t) // 2026、2027、2030
	tests := []struct {
		name string
		from string
		want string // 空字串 = 不涵蓋
	}{
		{"first year runs through the contiguous second", "2026-11-22", "2027-12-31"},
		{"last year of the run", "2027-06-01", "2027-12-31"},
		{"stops before the gap", "2026-01-01", "2027-12-31"},
		{"isolated year", "2030-03-01", "2030-12-31"},
		{"year in the gap", "2028-06-01", ""},
		{"before every imported year", "2025-12-31", ""},
		{"after every imported year", "2031-01-01", ""},
	}
	for _, tc := range tests {
		got, ok := c.CoveredThrough(d(tc.from))
		if tc.want == "" {
			if ok || !got.IsZero() {
				t.Errorf("%s：CoveredThrough(%s) = %v, %v；要不涵蓋", tc.name, tc.from, got, ok)
			}
			continue
		}
		if !ok || got.Format(time.DateOnly) != tc.want {
			t.Errorf("%s：CoveredThrough(%s) = %v, %v；要 %s", tc.name, tc.from, got, ok, tc.want)
		}
	}
	if _, ok := (&Calendar{}).CoveredThrough(d("2026-11-22")); ok {
		t.Fatal("空行事曆沒有涵蓋")
	}
}

func TestCoveredThroughUsesTheTaipeiDay(t *testing.T) {
	c := testCalendar(t)
	// 2026-12-31 17:00 UTC 是台北 2027-01-01：以台北日算，2027 年涵蓋到 2027-12-31，仍是 2027-12-31；
	// 2029-12-31 17:00 UTC 是台北 2030-01-01，落在 2030 年涵蓋的區段。
	if got, ok := c.CoveredThrough(time.Date(2029, 12, 31, 17, 0, 0, 0, time.UTC)); !ok || got.Format(time.DateOnly) != "2030-12-31" {
		t.Fatalf("got %v %v", got, ok)
	}
	if _, ok := c.CoveredThrough(time.Date(2029, 12, 31, 15, 0, 0, 0, time.UTC)); ok {
		t.Fatal("台北 2029-12-31 的年份沒有涵蓋")
	}
}
