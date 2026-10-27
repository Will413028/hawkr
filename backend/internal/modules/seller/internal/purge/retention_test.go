package purge

import (
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/platform/twtime"
)

// 審核紀錄的保存期間是日曆上的 2 年（台北時區），不是 730 天：跨過閏日時兩者差一天，界線以日曆為準。
func TestFoodReviewCutoffIsTwoCalendarYearsInTaipei(t *testing.T) {
	cases := []struct{ now, want time.Time }{
		{time.Date(2028, 3, 1, 0, 0, 0, 0, twtime.Taipei), time.Date(2026, 3, 1, 0, 0, 0, 0, twtime.Taipei)},
		{time.Date(2026, 10, 16, 12, 30, 0, 0, twtime.Taipei), time.Date(2024, 10, 16, 12, 30, 0, 0, twtime.Taipei)},
		// UTC 的 2026-12-31 20:00 在台北已經是 2027-01-01 04:00。
		{time.Date(2026, 12, 31, 20, 0, 0, 0, time.UTC), time.Date(2025, 1, 1, 4, 0, 0, 0, twtime.Taipei)},
	}
	for _, c := range cases {
		if got := FoodReviewCutoff(c.now); !got.Equal(c.want) {
			t.Errorf("now=%v cutoff=%v want %v", c.now, got, c.want)
		}
		if old := c.now.Add(-2 * 365 * 24 * time.Hour); c.now.Year() == 2028 && old.Equal(FoodReviewCutoff(c.now)) {
			t.Errorf("閏日之後 730 天不等於 2 個日曆年：%v", old)
		}
	}
}
