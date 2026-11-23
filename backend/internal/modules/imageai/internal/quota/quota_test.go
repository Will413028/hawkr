package quota

import (
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/platform/twtime"
)

func TestWeekStartIsMondayMidnightInTaipei(t *testing.T) {
	monday := time.Date(2026, 11, 23, 0, 0, 0, 0, twtime.Taipei)
	for name, in := range map[string]time.Time{
		"Monday 00:00":                      monday,
		"Wednesday":                         time.Date(2026, 11, 25, 15, 4, 5, 0, twtime.Taipei),
		"Sunday last second":                time.Date(2026, 11, 29, 23, 59, 59, 0, twtime.Taipei),
		"UTC Sunday 16:00 is Taipei Monday": time.Date(2026, 11, 22, 16, 0, 0, 0, time.UTC),
	} {
		if got := WeekStart(in); !got.Equal(monday) {
			t.Errorf("%s：WeekStart = %v，預期 %v", name, got, monday)
		}
	}
	if got := WeekStart(time.Date(2026, 11, 22, 15, 59, 59, 0, time.UTC)); !got.Equal(monday.AddDate(0, 0, -7)) {
		t.Errorf("週日最後一秒屬於上一週：%v", got)
	}
}

func TestNextWeekStartIsSevenDaysLater(t *testing.T) {
	start := time.Date(2026, 11, 23, 0, 0, 0, 0, twtime.Taipei)
	if got := NextWeekStart(start); !got.Equal(time.Date(2026, 11, 30, 0, 0, 0, 0, twtime.Taipei)) {
		t.Fatalf("NextWeekStart = %v", got)
	}
}
