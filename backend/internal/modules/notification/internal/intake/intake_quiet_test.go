package intake

import (
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/platform/twtime"
)

func TestQuietHoursEnd(t *testing.T) {
	at := func(d, h, m int) time.Time { return time.Date(2026, 11, d, h, m, 0, 0, twtime.Taipei) }
	cases := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{"22:00 開始", at(24, 22, 0), at(25, 9, 0)},
		{"23:00", at(24, 23, 0), at(25, 9, 0)},
		{"午夜後", at(25, 0, 30), at(25, 9, 0)},
		{"08:59", at(25, 8, 59), at(25, 9, 0)},
		{"09:00 不延後", at(25, 9, 0), time.Time{}},
		{"10:00", at(25, 10, 0), time.Time{}},
		{"21:59", at(25, 21, 59), time.Time{}},
		{"以 UTC 表示的 23:00 台北", at(24, 23, 0).UTC(), at(25, 9, 0)},
		{"月底跨月", time.Date(2026, 11, 30, 23, 0, 0, 0, twtime.Taipei), time.Date(2026, 12, 1, 9, 0, 0, 0, twtime.Taipei)},
	}
	for _, c := range cases {
		if got := QuietHoursEnd(c.now); !got.Equal(c.want) {
			t.Errorf("%s：QuietHoursEnd(%v) = %v，預期 %v", c.name, c.now, got, c.want)
		}
	}
}
