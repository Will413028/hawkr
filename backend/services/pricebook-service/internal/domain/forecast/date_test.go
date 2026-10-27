package forecast

import (
	"testing"
	"time"
)

func TestDatePyWeekday(t *testing.T) {
	tests := []struct {
		name string
		date Date
		want int
	}{
		{"星期一", NewDate(2026, 7, 20), 0},
		{"星期二", NewDate(2026, 7, 21), 1},
		{"星期六", NewDate(2026, 7, 25), 5},
		{"星期日", NewDate(2026, 7, 26), 6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.date.PyWeekday(); got != tt.want {
				t.Errorf("%s.PyWeekday() = %d，預期 %d（Python date.weekday() Monday=0）", tt.date, got, tt.want)
			}
		})
	}
}

func TestDateAddDaysCrossesMonthBoundary(t *testing.T) {
	got := NewDate(2026, 7, 29).AddDays(3)
	want := NewDate(2026, 8, 1)
	if got != want {
		t.Errorf("AddDays(3) = %s，預期 %s", got, want)
	}
}

func TestDateParseDateRoundTrip(t *testing.T) {
	d, err := ParseDate("2026-07-18")
	if err != nil {
		t.Fatalf("ParseDate 回錯誤: %v", err)
	}
	if d.String() != "2026-07-18" {
		t.Errorf("String() = %s，預期 2026-07-18", d.String())
	}
}

// TestDateToTimeAndDateFromTimeRoundTrip 釘住 ToTime/DateFromTime 互為反函式，
// 且 ToTime() 固定落在 UTC 午夜——usecase.Forecast 與 cmd/pricebook-feature-health
// 都靠這兩個轉換決定 repository 查詢窗，之前兩處各自複製一份卻沒有測試釘住
// （P4 review followup）。
func TestDateToTimeAndDateFromTimeRoundTrip(t *testing.T) {
	d := NewDate(2026, 7, 18)
	tm := d.ToTime()
	if tm.Hour() != 0 || tm.Minute() != 0 || tm.Second() != 0 || tm.Location() != time.UTC {
		t.Errorf("ToTime() = %v，預期 UTC 午夜", tm)
	}
	if got := DateFromTime(tm); got != d {
		t.Errorf("DateFromTime(d.ToTime()) = %s，預期 %s（往返應該還原）", got, d)
	}
	// 時間部分無意義，DateFromTime 只取日期。
	tmWithClock := time.Date(2026, 7, 18, 15, 30, 0, 0, time.UTC)
	if got := DateFromTime(tmWithClock); got != d {
		t.Errorf("DateFromTime(有時分秒的 time.Time) = %s，預期 %s（時間部分應被捨棄）", got, d)
	}
}
