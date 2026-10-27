package usecase

import (
	"testing"
	"time"
)

func TestCalendarWeekBounds(t *testing.T) {
	tests := []struct {
		now          time.Time
		wantThisWeek time.Time
		wantLastWeek time.Time
		name         string
	}{
		{
			name:         "週五（週中）",
			now:          time.Date(2026, 4, 3, 14, 30, 0, 0, time.UTC),
			wantThisWeek: time.Date(2026, 3, 29, 0, 0, 0, 0, time.UTC),
			wantLastWeek: time.Date(2026, 3, 22, 0, 0, 0, 0, time.UTC),
		},
		{
			name:         "週日（週的第一天）",
			now:          time.Date(2026, 3, 29, 8, 0, 0, 0, time.UTC),
			wantThisWeek: time.Date(2026, 3, 29, 0, 0, 0, 0, time.UTC),
			wantLastWeek: time.Date(2026, 3, 22, 0, 0, 0, 0, time.UTC),
		},
		{
			name:         "週六（週的最後一天）",
			now:          time.Date(2026, 4, 4, 23, 59, 0, 0, time.UTC),
			wantThisWeek: time.Date(2026, 3, 29, 0, 0, 0, 0, time.UTC),
			wantLastWeek: time.Date(2026, 3, 22, 0, 0, 0, 0, time.UTC),
		},
		{
			name:         "跨月（週一，本週日在上個月）",
			now:          time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC),
			wantThisWeek: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
			wantLastWeek: time.Date(2026, 2, 22, 0, 0, 0, 0, time.UTC),
		},
		{
			name:         "跨年（週四，本週日在去年）",
			now:          time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
			wantThisWeek: time.Date(2025, 12, 28, 0, 0, 0, 0, time.UTC),
			wantLastWeek: time.Date(2025, 12, 21, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			thisWeek, lastWeek := calendarWeekBounds(tt.now)
			if !thisWeek.Equal(tt.wantThisWeek) {
				t.Errorf("thisWeekStart = %v, want %v", thisWeek, tt.wantThisWeek)
			}
			if !lastWeek.Equal(tt.wantLastWeek) {
				t.Errorf("lastWeekStart = %v, want %v", lastWeek, tt.wantLastWeek)
			}
			// 本週日必定是 Sunday
			if thisWeek.Weekday() != time.Sunday {
				t.Errorf("thisWeekStart weekday = %v, want Sunday", thisWeek.Weekday())
			}
			if lastWeek.Weekday() != time.Sunday {
				t.Errorf("lastWeekStart weekday = %v, want Sunday", lastWeek.Weekday())
			}
		})
	}
}
