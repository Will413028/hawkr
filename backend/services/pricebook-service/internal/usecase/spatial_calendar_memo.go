// services/pricebook-service/internal/usecase/spatial_calendar_memo.go
package usecase

import (
	"context"
	"sync"
	"time"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
)

// calendarMemoRepo is a per-request decorator that answers a market's
// rest-day calendar and calendar-coverage queries once instead of once per
// sub-variant.
//
// Why it exists: SpatialSurface.Get runs SeriesForecast once per
// (market, crop). SeriesForecast issues three reads per call —
// SeriesObservations (crop-dependent), RestDays and RestDayCoverage (both
// crop-INdependent, keyed only by market and the request's own origin
// date). On the group path that makes the two calendar reads run once per
// sub-variant per market: a 24-market group with 5 modelled sub-variants
// issues 240 identical calendar queries where 48 would do. GroupForecast
// already avoids this by looping markets rather than series
// (forecast.go's marketCodes loop); this reaches the same query count
// without copying SeriesForecast's calendar/quality-gate logic into the
// spatial usecase, which estimated_item_price.go's doc comment explicitly
// forbids.
//
// Correctness: only the two crop-independent reads are memoised, each
// under a key containing every argument that could change the answer, and
// the memo lives exactly as long as one Get call. Two calls that would
// have returned different rows therefore cannot share an entry — the memo
// changes how many times an answer is fetched, never what it is.
// SeriesObservations and every write path are passed straight through.
//
// The mutex is not for concurrency inside one request (aggregateMarkets is
// sequential) but because *Forecast makes no single-goroutine promise and a
// silently racy decorator would be a trap for whoever parallelises the
// per-market loop later.
type calendarMemoRepo struct {
	domain.ForecastRepository

	mu       sync.Mutex
	restDays map[restDaysKey][]domain.RestDay
	coverage map[string][]domain.RestDayCoverage
}

type restDaysKey struct {
	marketCode string
	from, to   time.Time
}

func newCalendarMemoRepo(inner domain.ForecastRepository) *calendarMemoRepo {
	return &calendarMemoRepo{
		ForecastRepository: inner,
		restDays:           map[restDaysKey][]domain.RestDay{},
		coverage:           map[string][]domain.RestDayCoverage{},
	}
}

func (r *calendarMemoRepo) RestDays(ctx context.Context, marketCode string, from, to time.Time) ([]domain.RestDay, error) {
	key := restDaysKey{marketCode: marketCode, from: from, to: to}
	r.mu.Lock()
	cached, ok := r.restDays[key]
	r.mu.Unlock()
	if ok {
		return cached, nil
	}
	days, err := r.ForecastRepository.RestDays(ctx, marketCode, from, to)
	if err != nil {
		// Deliberately not cached: a failed read must be retried, not
		// remembered as "this market has no rest days".
		return nil, err
	}
	r.mu.Lock()
	r.restDays[key] = days
	r.mu.Unlock()
	return days, nil
}

func (r *calendarMemoRepo) RestDayCoverage(ctx context.Context, marketCode string) ([]domain.RestDayCoverage, error) {
	r.mu.Lock()
	cached, ok := r.coverage[marketCode]
	r.mu.Unlock()
	if ok {
		return cached, nil
	}
	rows, err := r.ForecastRepository.RestDayCoverage(ctx, marketCode)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	r.coverage[marketCode] = rows
	r.mu.Unlock()
	return rows, nil
}

var _ domain.ForecastRepository = (*calendarMemoRepo)(nil)
