package expiry

import "testing"

func TestJobIsScheduledHourlyAndMergesWithinTheInterval(t *testing.T) {
	t.Parallel()
	if got := len(PeriodicJobs()); got != 1 {
		t.Fatalf("PeriodicJobs 有 %d 個，預期 1", got)
	}
	if o := (Args{}).InsertOpts(); o.UniqueOpts.ByPeriod != Interval {
		t.Fatalf("唯一鍵 = %+v，要 ByPeriod %s", o.UniqueOpts, Interval)
	}
}
