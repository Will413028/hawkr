package expiry

import (
	"testing"
	"time"
)

func TestHorizonIsEndOfTheNextUTCDay(t *testing.T) {
	t.Parallel()
	taipei := time.FixedZone("UTC+8", 8*3600)
	cases := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{"UTC 日內任一刻同一個上界", time.Date(2026, 12, 1, 17, 30, 0, 0, time.UTC), time.Date(2026, 12, 3, 0, 0, 0, 0, time.UTC)},
		{"UTC 零時整", time.Date(2026, 12, 2, 0, 0, 0, 0, time.UTC), time.Date(2026, 12, 4, 0, 0, 0, 0, time.UTC)},
		{"UTC 23:59:59", time.Date(2026, 12, 1, 23, 59, 59, 0, time.UTC), time.Date(2026, 12, 3, 0, 0, 0, 0, time.UTC)},
		{"非 UTC 時區的 now 仍以 UTC 日計（台北 12/2 01:30 = UTC 12/1 17:30）", time.Date(2026, 12, 2, 1, 30, 0, 0, taipei), time.Date(2026, 12, 3, 0, 0, 0, 0, time.UTC)},
	}
	for _, c := range cases {
		if got := Horizon(c.now); !got.Equal(c.want) {
			t.Errorf("%s：Horizon(%s) = %s，要 %s", c.name, c.now, got, c.want)
		}
	}
}

func TestJobIsScheduledHourlyAndMergesWithinTheInterval(t *testing.T) {
	t.Parallel()
	if got := len(PeriodicJobs()); got != 1 {
		t.Fatalf("PeriodicJobs 有 %d 個，預期 1", got)
	}
	if o := (Args{}).InsertOpts(); o.UniqueOpts.ByPeriod != Interval {
		t.Fatalf("唯一鍵 = %+v，要 ByPeriod %s", o.UniqueOpts, Interval)
	}
}
