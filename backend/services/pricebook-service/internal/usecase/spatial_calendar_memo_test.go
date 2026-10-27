package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
)

// calendarMemoRepo's contract is "changes how many times an answer is
// fetched, never what it is", which holds only if its keys contain every
// argument that could change the answer.
//
// TestSpatialSurface_Get_CalendarMemoDoesNotChangeResults exercises the
// market_code half of that through the real usecase, but it cannot reach
// the window half: within one SpatialSurface.Get every SeriesForecast run
// derives its rest-day window from the same origin date, so a single
// request only ever asks for ONE window per market and a key that dropped
// from/to would still answer correctly there. That component is therefore
// pinned here instead, at the decorator's own seam, where two windows can
// actually be requested.
func TestCalendarMemoRepo_KeysOnEveryArgumentThatChangesTheAnswer(t *testing.T) {
	window1From, window1To := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC), time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)
	window2From, window2To := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC)

	var restDayCalls, coverageCalls int
	inner := &fakeForecastRepo{
		// The answer encodes both the market and the window it was asked
		// for, so a lost key component shows up as a wrong row rather than
		// merely a wrong call count.
		restDaysFn: func(ctx context.Context, marketCode string, from, to time.Time) ([]domain.RestDay, error) {
			restDayCalls++
			return []domain.RestDay{{MarketCode: marketCode, MarketType: "V", RestDate: from}}, nil
		},
		restDayCoverageFn: func(ctx context.Context, marketCode string) ([]domain.RestDayCoverage, error) {
			coverageCalls++
			return []domain.RestDayCoverage{{MarketCode: marketCode, MarketType: "V", Year: 2026}}, nil
		},
	}
	memo := newCalendarMemoRepo(inner)
	ctx := context.Background()

	requireRestDay := func(marketCode string, from, to time.Time) {
		t.Helper()
		got, err := memo.RestDays(ctx, marketCode, from, to)
		require.NoError(t, err)
		require.Len(t, got, 1)
		require.Equal(t, marketCode, got[0].MarketCode, "memo served another market's rest days")
		require.Equal(t, from, got[0].RestDate, "memo served another window's rest days")
	}

	// Same market, two windows: each must get its own answer.
	requireRestDay("104", window1From, window1To)
	requireRestDay("104", window2From, window2To)
	require.Equal(t, 2, restDayCalls, "two distinct windows are two distinct questions")

	// Two markets, same window: likewise.
	requireRestDay("220", window1From, window1To)
	require.Equal(t, 3, restDayCalls)

	// Every repeat is served from the memo, not the repository.
	requireRestDay("104", window1From, window1To)
	requireRestDay("104", window2From, window2To)
	requireRestDay("220", window1From, window1To)
	require.Equal(t, 3, restDayCalls, "a repeated question must not reach the repository again")

	// Coverage is keyed by market alone because market IS its whole
	// argument list.
	for _, marketCode := range []string{"104", "220", "104", "220"} {
		got, err := memo.RestDayCoverage(ctx, marketCode)
		require.NoError(t, err)
		require.Len(t, got, 1)
		require.Equal(t, marketCode, got[0].MarketCode, "memo served another market's coverage")
	}
	require.Equal(t, 2, coverageCalls)
}

// A failed read must be retried, not remembered as "this market has no rest
// days" — the one case where the memo deliberately does not cache.
func TestCalendarMemoRepo_DoesNotCacheErrors(t *testing.T) {
	from, to := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC), time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)
	boom := errors.New("connection reset")

	var restDayCalls, coverageCalls int
	fail := true
	inner := &fakeForecastRepo{
		restDaysFn: func(ctx context.Context, marketCode string, f, tt time.Time) ([]domain.RestDay, error) {
			restDayCalls++
			if fail {
				return nil, boom
			}
			return []domain.RestDay{{MarketCode: marketCode, MarketType: "V", RestDate: f}}, nil
		},
		restDayCoverageFn: func(ctx context.Context, marketCode string) ([]domain.RestDayCoverage, error) {
			coverageCalls++
			if fail {
				return nil, boom
			}
			return []domain.RestDayCoverage{{MarketCode: marketCode, MarketType: "V", Year: 2026}}, nil
		},
	}
	memo := newCalendarMemoRepo(inner)
	ctx := context.Background()

	_, err := memo.RestDays(ctx, "104", from, to)
	require.ErrorIs(t, err, boom)
	_, err = memo.RestDayCoverage(ctx, "104")
	require.ErrorIs(t, err, boom)

	fail = false
	days, err := memo.RestDays(ctx, "104", from, to)
	require.NoError(t, err)
	require.Len(t, days, 1, "the retry must reach the repository, not a cached empty answer")
	coverage, err := memo.RestDayCoverage(ctx, "104")
	require.NoError(t, err)
	require.Len(t, coverage, 1)

	require.Equal(t, 2, restDayCalls)
	require.Equal(t, 2, coverageCalls)
}
