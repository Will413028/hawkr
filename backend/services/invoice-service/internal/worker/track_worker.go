package worker

import (
	"context"
	"time"

	"github.com/Will413028/hawkr/go-pkg/timeutil"
)

const trackTick = time.Hour

const periodStartFastHours = 4

func isTrackScanSlot(now time.Time, baseHourUTC int) bool {
	h := now.UTC().Hour()
	lt := now.In(timeutil.LocationTaipei)
	periodStart := int(lt.Month())%2 == 1 && lt.Day() >= 1 && lt.Day() <= 5
	if periodStart {
		return ((h-baseHourUTC)%periodStartFastHours+periodStartFastHours)%periodStartFastHours == 0
	}
	return h == baseHourUTC
}

type trackRunner interface {
	RunOnce(ctx context.Context) error
}

type trackWorker struct {
	uc          trackRunner
	baseHourUTC int
	now         func() time.Time
}

// RunOnce 由排程器每小時呼叫；不是掃描時點就直接返回。
func (w *trackWorker) RunOnce(ctx context.Context) error {
	if !isTrackScanSlot(w.now(), w.baseHourUTC) {
		return nil
	}
	return w.uc.RunOnce(ctx)
}

type trackObserver struct{}

func (trackObserver) ObserveTrackScan(remainingMin, lowSellers, exhaustedSellers, nextPeriodInsufficient int64) {
	trackRemainingMin.Set(float64(remainingMin))
	trackLowSellers.Set(float64(lowSellers))
	trackExhaustedSellers.Set(float64(exhaustedSellers))
	trackNextPeriodInsufficientSellers.Set(float64(nextPeriodInsufficient))
}
