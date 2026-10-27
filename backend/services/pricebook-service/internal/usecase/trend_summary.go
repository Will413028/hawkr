package usecase

import (
	"context"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/Will413028/hawkr/pricebook-service/internal/config"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/forecast"
)

// TrendSummaryResult is what Get returns — GetTrendSummary's usecase-layer
// DTO (proto mapping happens in the gRPC handler, Task 9, same layering
// every other RPC in this service already uses).
type TrendSummaryResult struct {
	SelectionKind string // "crop" | "group"
	Granularity   forecast.Granularity
	Points        []TrendSummaryPoint
	Comparisons   map[forecast.TrendComparisonKind]forecast.TrendComparison
	DataMeta      DataMeta
}

type TrendSummaryPoint struct {
	Bounds              forecast.PeriodBounds
	Status              forecast.PredictionStatus
	Value               *float64
	Volume              *float64
	SampleCount         int
	ObservedMarketCount int
	AggregationMethod   string
}

// TrendSummary serves GetTrendSummary (issue #2249 Slice B): resolve a
// direct crop_code (single market) or group_code (existing two-stage
// volume-weighted aggregate, same as ObservedPrice's group history path),
// bucket the observed series into day/week periods, and optionally compare
// the most recent period against its WoW/YoY364 baseline.
//
// Unlike ObservedPrice/ForecastCurve, there is no crop_name resolution path
// — see the proto RPC's own doc comment (Task 6) for why.
type TrendSummary struct {
	repo         domain.ForecastRepository
	forecastUC   *Forecast // group path only, for ComputeGroupVolumes (spatial weighting)
	freshnessCfg forecast.FreshnessConfig
	logger       *zap.Logger
}

func NewTrendSummary(repo domain.ForecastRepository, forecastUC *Forecast, cfg *config.Config, logger *zap.Logger) *TrendSummary {
	return &TrendSummary{repo: repo, forecastUC: forecastUC, freshnessCfg: freshnessConfigFrom(cfg.Inference), logger: logger}
}

// Get resolves and returns the trend series + comparisons. cropCode/
// groupCode: exactly one must be set (ErrConflictingResolutionInputs/
// ErrNoResolutionInput on violation, reusing the same sentinel errors
// ObservedPrice/ForecastCurve already use — the gRPC handler, Task 9,
// already knows how to map them to InvalidArgument). marketCode: required
// when cropCode is set, forbidden when groupCode is set (same
// ErrMarketCodeRequiredForCrop/ErrMarketCodeNotAllowedForGroup sentinels).
func (u *TrendSummary) Get(ctx context.Context, cropCode, groupCode, marketCode string, from, to forecast.Date, granularity forecast.Granularity, comparisons []forecast.TrendComparisonKind, now time.Time) (TrendSummaryResult, error) {
	if err := validateResolutionInputs("", cropCode, groupCode); err != nil {
		return TrendSummaryResult{}, err
	}

	periods := forecast.PeriodsInRange(from, to, granularity)
	if len(periods) == 0 {
		return TrendSummaryResult{}, nil
	}
	latest := periods[len(periods)-1]

	// Total read range: primary [from,to] plus, for each requested
	// comparison, the baseline window for the LAST period only (comparisons
	// are anchored to the most recent period, not computed per-point — see
	// the design doc's data-flow step 2). Each baseline window is at most
	// one period wide (<=7 days at week granularity), so this never becomes
	// a multi-year query.
	wideFrom, wideTo := from.ToTime(), to.ToTime()
	for _, kind := range comparisons {
		baseline := forecast.BaselinePeriod(latest, kind)
		if baseline.Start.ToTime().Before(wideFrom) {
			wideFrom = baseline.Start.ToTime()
		}
		if baseline.End.ToTime().After(wideTo) {
			wideTo = baseline.End.ToTime()
		}
	}

	var result TrendSummaryResult
	var err error
	switch {
	case cropCode != "":
		if marketCode == "" {
			return TrendSummaryResult{}, ErrMarketCodeRequiredForCrop
		}
		result, err = u.cropPath(ctx, cropCode, marketCode, periods, latest, comparisons, wideFrom, wideTo, now)
	case marketCode != "":
		return TrendSummaryResult{}, ErrMarketCodeNotAllowedForGroup
	default:
		result, err = u.groupPath(ctx, groupCode, periods, latest, comparisons, wideFrom, wideTo, now)
	}
	if err != nil {
		return TrendSummaryResult{}, err
	}
	// Granularity is request-echo, not derived from any repository read —
	// set once here rather than threading it through both path functions
	// just to stuff it into an otherwise-identical TrendSummaryResult{...}
	// literal twice.
	result.Granularity = granularity
	return result, nil
}

func (u *TrendSummary) cropPath(ctx context.Context, cropCode, marketCode string, periods []forecast.PeriodBounds, latest forecast.PeriodBounds, comparisons []forecast.TrendComparisonKind, wideFrom, wideTo time.Time, now time.Time) (TrendSummaryResult, error) {
	rows, err := u.repo.ObservedPricesByCrop(ctx, marketCode, cropCode, wideFrom, wideTo, now)
	if err != nil {
		return TrendSummaryResult{}, err
	}

	daily := make([]forecast.TrendDailyPoint, 0, len(rows))
	presentDates := map[forecast.Date]bool{}
	var categoryCode *string
	for _, r := range rows {
		if r.AvgPrice == nil {
			continue
		}
		d := forecast.DateFromTime(r.TradeDate)
		daily = append(daily, forecast.TrendDailyPoint{Date: d, Value: r.AvgPrice, Volume: r.VolumeKg})
		presentDates[d] = true
		if categoryCode == nil {
			categoryCode = r.CategoryCode
		}
	}

	// When the window has zero rows with a price (the whole point of the
	// market_closed status this feeds into — price_observation carries no
	// closed-market sentinel rows, see NewForecastRepository's doc comment),
	// categoryCode is still nil at this point: there is nothing in `rows` to
	// read it from. Fall back to a direct crop-table lookup so
	// ExpectedTradingDay/PeriodStatus still has a categoryCode to key its
	// closures/verified-calendar checks on — without this, a fully closed
	// period could never be distinguished from insufficient_data, since
	// IsVerified(..., nil, ...) is unconditionally false (calendar.go) and
	// every date would come back calendarUnverified. Only queried when the
	// cheaper rows-derived value is unavailable — the common case (real data
	// present) never pays this extra round trip.
	if categoryCode == nil {
		cropRef, err := u.repo.CropByCode(ctx, cropCode)
		if err != nil {
			return TrendSummaryResult{}, err
		}
		categoryCode = cropRef.CategoryCode
	}

	closures, verified, err := u.calendarFor(ctx, []string{marketCode}, wideFrom, wideTo)
	if err != nil {
		return TrendSummaryResult{}, err
	}

	// marketID must carry the same "moa:" prefix calendarFor uses internally
	// when populating closures/verified (calendarFor takes RAW market codes
	// for the repository lookups but keys its output with "moa:"+code — see
	// its own doc comment). Passing the raw marketCode here instead (as this
	// line used to) means closures.IsClosed/verified.IsVerified are queried
	// under a key ("109") that was never written (only "moa:109" was), so
	// every date is permanently calendarUnverified and PeriodStatus can only
	// ever return insufficient_data for the crop path — market_closed/ready/
	// partial become unreachable regardless of real RestDay/observation
	// data. groupPath already gets this right (its own marketIDs are built
	// with the "moa:" prefix, see below) — this mirrors that.
	marketID := "moa:" + marketCode

	// The per-period ObservedMarketCount diagnostic (via buildPoints) reads
	// from the raw ObservedPricesByCrop result `rows`, unfiltered on
	// AvgPrice — mirroring ObservedPrice.Get's buildDataMeta convention
	// (observed_price.go): a no_trade day's row still carries a real
	// source_digest/available_at worth surfacing. This is safe for a
	// single-market crop path since every row already belongs to the one
	// requested (marketCode, cropCode) pair — unlike the group path below,
	// there is no risk of leaking an unrelated crop's identity into this
	// signal. dataMeta itself uses a narrower, date-windowed subset — see
	// the I2 fix just below.
	points := u.buildPoints(periods, daily, presentDates, []string{marketID}, categoryCode, closures, verified, rows)
	comparisonResults := u.buildComparisons(latest, comparisons, daily, presentDates, []string{marketID}, categoryCode, closures, verified)

	// I2 fix (final review, 2026-08-08): dataMeta must describe only the
	// primary [from,to] window, not the widened wideFrom/wideTo range `rows`
	// was read with (which also carries WoW/YoY364 baseline rows) — see
	// filterRowsInPrimaryWindow's own doc comment.
	primaryFrom, primaryTo := periods[0].Start, periods[len(periods)-1].End
	primaryRows := filterRowsInPrimaryWindow(rows, primaryFrom, primaryTo)
	// M6 (final review, 2026-08-08): the design spec promises a Warn on any
	// repo call returning zero rows for a non-trivially-sized requested
	// range — signal for "is this crop actually inactive" vs "did something
	// break upstream" (Observability section). A single-day query
	// legitimately returning zero rows isn't suspicious the way a
	// multi-week zero-row result is.
	if len(primaryRows) == 0 && daysInclusive(primaryFrom, primaryTo) > 7 {
		u.logger.Warn("trend_summary: zero observed-price rows for a non-trivially-sized requested range",
			zap.String("crop_code", cropCode), zap.String("market_code", marketCode),
			zap.String("from", primaryFrom.String()), zap.String("to", primaryTo.String()))
	}
	meta := u.buildDataMeta(now, points, primaryRows)

	return TrendSummaryResult{SelectionKind: "crop", Points: points, Comparisons: comparisonResults, DataMeta: meta}, nil
}

func (u *TrendSummary) groupPath(ctx context.Context, groupCode string, periods []forecast.PeriodBounds, latest forecast.PeriodBounds, comparisons []forecast.TrendComparisonKind, wideFrom, wideTo time.Time, now time.Time) (TrendSummaryResult, error) {
	crops, err := u.repo.CropsInGroup(ctx, groupCode)
	if err != nil {
		return TrendSummaryResult{}, err
	}
	activeCropCodes := map[string]bool{}
	var categoryCode *string
	var categoryCodeFromActive bool
	for _, c := range crops {
		if c.Active {
			activeCropCodes[c.CropCode] = true
		}
		// An active crop's CategoryCode always wins over one already picked
		// from an earlier inactive crop; only fall back to an inactive
		// crop's category when no active crop has been seen yet (crops.CropRef
		// order is not guaranteed to put actives first). This matters because
		// categoryCode feeds ExpectedTradingDay/PeriodStatus below — the
		// wrong category can make a period wrongly report insufficient_data/
		// market_closed even though it has real data. The naive "same
		// crop_group therefore same category_code" assumption is NOT a DB
		// constraint (schema.sql has no FK/check tying crop.category_code to
		// its group): it only holds because price_ingest.go sets both from
		// the same upstream TcType in the same upsert — application
		// convention, not a guarantee this usecase should silently rely on.
		if categoryCode == nil || (!categoryCodeFromActive && c.Active && c.CategoryCode != nil) {
			categoryCode = c.CategoryCode
			categoryCodeFromActive = c.Active
		}
	}

	rows, err := u.repo.ObservedPricesByGroup(ctx, groupCode, "", wideFrom, wideTo, now)
	if err != nil {
		return TrendSummaryResult{}, err
	}

	// vols must be computed BEFORE marketSet below is finalized (not just
	// before AggregateGroupObservations needs it) — see I1's doc comment on
	// the marketSet-seeding loop just below for why.
	vols, err := u.forecastUC.ComputeGroupVolumes(ctx, groupCode, forecast.DateFromTime(now), now)
	if err != nil {
		return TrendSummaryResult{}, err
	}

	// contributingRows is the subset of `rows` that actually feeds
	// variantPoints below (active crop + a real price) — mirrors
	// GroupObservedSeries' identical filter (forecast.go) and the
	// established "must not leak inactive-crop signal" rule it exists to
	// enforce (see TestObservedPrice_DataMeta_GroupHistoryAggregated_
	// UsesContributingRowsSignals in observed_price_test.go). Both the
	// per-period ObservedMarketCount diagnostic (via buildPoints) and
	// dataMeta (via buildDataMeta) read from this filtered set, not the raw
	// `rows` — an inactive crop's later available_at or a distinct
	// source_digest from a crop that never contributed to the aggregated
	// Value must not leak into either signal.
	// I3 fix (final review, 2026-08-08): pass r.VolumeKg straight through,
	// no nil→0.0 conversion. forecast.DailyVariantPrice.Volume is *float64
	// precisely so a row whose volume_kg is genuinely unknown (SQL NULL)
	// stays nil all the way through AggregateGroupObservations instead of
	// being fabricated into a real-looking &0.0 on the wire — the crop path
	// (above) already does this correctly (Volume: r.VolumeKg).
	variantPoints := make([]forecast.DailyVariantPrice, 0, len(rows))
	contributingRows := make([]domain.ObservedPrice, 0, len(rows))
	marketSet := map[string]bool{}
	for _, r := range rows {
		if !activeCropCodes[r.CropCode] || r.AvgPrice == nil {
			continue
		}
		variantPoints = append(variantPoints, forecast.DailyVariantPrice{
			MarketID: "moa:" + r.MarketCode, CropID: "moa:" + r.CropCode,
			TradeDate: forecast.DateFromTime(r.TradeDate), Price: *r.AvgPrice, Volume: r.VolumeKg,
		})
		contributingRows = append(contributingRows, r)
		marketSet[r.MarketCode] = true
	}

	// I1 fix (final review, 2026-08-08): union marketSet with vols.ByMarket's
	// keys (stripped of their "moa:" prefix back to raw market codes) — vols
	// carries a data-INDEPENDENT market roster (sourced from a `now`-relative
	// recent-volume window, see ComputeGroupVolumes/groupVolumes' own doc
	// comment), unlike marketSet built from `rows` above, which is empty
	// whenever the query window has zero contributing rows (e.g. every
	// market genuinely closed all week — Lunar New Year, typhoon closures).
	// Without this union, an all-closed window leaves marketSet/marketIDs
	// empty, calendarFor below makes zero repo calls, ExpectedTradingDay
	// returns unverified for every date, and PeriodStatus can only ever
	// report insufficient_data — never market_closed — exactly the defect
	// class Task 11 already root-caused and fixed on the crop path (via
	// CropByCode), now fixed on its sibling. Known, accepted limitation:
	// since vols reflects TODAY's market roster (not necessarily the
	// historical one for a deep-historical query), this is strictly better
	// than an empty list, not a claim of full historical accuracy.
	for marketID := range vols.ByMarket {
		marketSet[strings.TrimPrefix(marketID, "moa:")] = true
	}

	marketIDs := make([]string, 0, len(marketSet))
	rawMarketCodes := make([]string, 0, len(marketSet))
	for m := range marketSet {
		marketIDs = append(marketIDs, "moa:"+m)
		rawMarketCodes = append(rawMarketCodes, m)
	}

	aggregated := forecast.AggregateGroupObservations(variantPoints, vols)

	daily := make([]forecast.TrendDailyPoint, 0, len(aggregated))
	presentDates := map[forecast.Date]bool{}
	for _, p := range aggregated {
		if p.Value == nil {
			continue
		}
		daily = append(daily, forecast.TrendDailyPoint{Date: p.TradeDate, Value: p.Value, Volume: p.Volume})
		presentDates[p.TradeDate] = true
	}

	closures, verified, err := u.calendarFor(ctx, rawMarketCodes, wideFrom, wideTo)
	if err != nil {
		return TrendSummaryResult{}, err
	}

	points := u.buildPoints(periods, daily, presentDates, marketIDs, categoryCode, closures, verified, contributingRows)
	comparisonResults := u.buildComparisons(latest, comparisons, daily, presentDates, marketIDs, categoryCode, closures, verified)

	// I2 fix (final review, 2026-08-08) — same reasoning as cropPath's
	// identical block above: dataMeta must describe only the primary
	// [from,to] window, not the widened wideFrom/wideTo range
	// `contributingRows` was read with.
	primaryFrom, primaryTo := periods[0].Start, periods[len(periods)-1].End
	primaryContributingRows := filterRowsInPrimaryWindow(contributingRows, primaryFrom, primaryTo)
	// M6 (final review, 2026-08-08) — same reasoning as cropPath's identical
	// block above.
	if len(primaryContributingRows) == 0 && daysInclusive(primaryFrom, primaryTo) > 7 {
		u.logger.Warn("trend_summary: zero observed-price rows for a non-trivially-sized requested range",
			zap.String("group_code", groupCode),
			zap.String("from", primaryFrom.String()), zap.String("to", primaryTo.String()))
	}
	meta := u.buildDataMeta(now, points, primaryContributingRows)

	return TrendSummaryResult{SelectionKind: "group", Points: points, Comparisons: comparisonResults, DataMeta: meta}, nil
}

// calendarFor fetches RestDays/RestDayCoverage for every market involved and
// folds them into one Closures + VerifiedCalendar pair, scoped to an
// explicit market list instead of a single artifact-driven market, since the
// group path can span several markets.
//
// I4 fix (final review, 2026-08-08): delegates the per-market fold to the
// SAME addRestDaysToClosures/appendVerifiedCalendarKeys helpers forecast.go
// already uses for SeriesForecast/GroupForecast (this function used to
// hand-roll an equivalent loop, which silently dropped the unknown-
// market_type Warn those helpers' own doc comments say is mandatory —
// "無法把 market_type 換算成 category code 的列一律記 Warn 並跳過...上游改了
// 代碼時要看得見"). Reusing them also restores appendVerifiedCalendarKeys'
// zero-coverage Warn for free, keyed the same way GroupForecast keys it
// (extraFields carries market_code, mirroring GroupForecast's group_code).
//
// from/to must be the actual query window (this task's callers always pass
// wideFrom/wideTo, the same window used for the observed-price read) — NOT
// the zero time.Time{}. pgxutil.DateParam maps a zero time.Time to
// pgtype.Date{Valid: false} (SQL NULL), and RestDaysForMarket's `rest_date
// BETWEEN sqlc.arg(from_date) AND sqlc.arg(to_date)` is unsatisfiable when
// either bound is NULL (`BETWEEN NULL AND NULL` is SQL NULL, never TRUE) —
// so a zero-time call would silently and permanently return zero rest days,
// the same "zero time.Time silently becomes SQL NULL" trap SeriesForecast/
// GroupForecast already guard against for their own Origin parameter (see
// their doc comments in this package), just reached through a different
// parameter here.
func (u *TrendSummary) calendarFor(ctx context.Context, marketCodes []string, from, to time.Time) (*forecast.Closures, forecast.VerifiedCalendar, error) {
	closures := forecast.NewClosures()
	var keys []forecast.VerifiedCalendarKey
	for _, marketCode := range marketCodes {
		restDays, err := u.repo.RestDays(ctx, marketCode, from, to)
		if err != nil {
			return nil, forecast.VerifiedCalendar{}, err
		}
		addRestDaysToClosures(u.logger, closures, restDays)

		coverage, err := u.repo.RestDayCoverage(ctx, marketCode)
		if err != nil {
			return nil, forecast.VerifiedCalendar{}, err
		}
		keys = appendVerifiedCalendarKeys(u.logger, keys, "trend_summary", coverage, zap.String("market_code", marketCode))
	}
	return closures, forecast.NewVerifiedCalendar(keys), nil
}

func (u *TrendSummary) buildPoints(periods []forecast.PeriodBounds, daily []forecast.TrendDailyPoint, presentDates map[forecast.Date]bool, marketIDs []string, categoryCode *string, closures *forecast.Closures, verified forecast.VerifiedCalendar, rawRows []domain.ObservedPrice) []TrendSummaryPoint {
	out := make([]TrendSummaryPoint, 0, len(periods))
	for _, bounds := range periods {
		value, method, sampleCount := forecast.AggregatePeriodValue(daily, bounds)
		status := forecast.PeriodStatus(bounds, presentDates, marketIDs, categoryCode, closures, verified)
		var volume *float64
		var totalVol float64
		var haveVol bool
		for _, p := range daily {
			if p.Date.Before(bounds.Start) || p.Date.After(bounds.End) || p.Volume == nil {
				continue
			}
			totalVol += *p.Volume
			haveVol = true
		}
		if haveVol {
			volume = &totalVol
		}
		out = append(out, TrendSummaryPoint{
			Bounds: bounds, Status: status, Value: value, Volume: volume, SampleCount: sampleCount,
			ObservedMarketCount: countMarketsInPeriod(rawRows, bounds), AggregationMethod: method,
		})
	}
	return out
}

func countMarketsInPeriod(rows []domain.ObservedPrice, bounds forecast.PeriodBounds) int {
	seen := map[string]bool{}
	for _, r := range rows {
		d := forecast.DateFromTime(r.TradeDate)
		if d.Before(bounds.Start) || d.After(bounds.End) {
			continue
		}
		seen[r.MarketCode] = true
	}
	return len(seen)
}

func (u *TrendSummary) buildComparisons(latest forecast.PeriodBounds, kinds []forecast.TrendComparisonKind, daily []forecast.TrendDailyPoint, presentDates map[forecast.Date]bool, marketIDs []string, categoryCode *string, closures *forecast.Closures, verified forecast.VerifiedCalendar) map[forecast.TrendComparisonKind]forecast.TrendComparison {
	if len(kinds) == 0 {
		return nil
	}
	currentValue, _, _ := forecast.AggregatePeriodValue(daily, latest)
	currentStatus := forecast.PeriodStatus(latest, presentDates, marketIDs, categoryCode, closures, verified)

	out := make(map[forecast.TrendComparisonKind]forecast.TrendComparison, len(kinds))
	for _, kind := range kinds {
		baseline := forecast.BaselinePeriod(latest, kind)
		baselineValue, _, _ := forecast.AggregatePeriodValue(daily, baseline)
		baselineStatus := forecast.PeriodStatus(baseline, presentDates, marketIDs, categoryCode, closures, verified)
		out[kind] = forecast.ComputeComparison(currentValue, currentStatus, baselineValue, baselineStatus)
	}
	return out
}

func (u *TrendSummary) buildDataMeta(now time.Time, points []TrendSummaryPoint, rows []domain.ObservedPrice) DataMeta {
	statuses := make([]forecast.PredictionStatus, len(points))
	for i, p := range points {
		statuses[i] = p.Status
	}
	maxAvailableAt := observedRowsMaxAvailableAt(rows)
	latestTradeDate := observedRowsLatestTradeDate(rows)
	digest := forecast.DatasetDigestFromSourceDigests(observedRowsSourceDigests(rows))
	return trendSummaryDataMeta(now, u.freshnessCfg, statuses, maxAvailableAt, latestTradeDate, digest)
}

// filterRowsInPrimaryWindow narrows rows to just the primary [from,to]
// window (I2 fix, final review 2026-08-08) — dataMeta must describe only
// what's in points[], not the WoW/YoY364 baseline windows also pulled in by
// the widened wideFrom/wideTo range cropPath/groupPath actually read the
// repository with (design spec's dataMeta section: "computed from the raw
// rows that actually contributed to points[]... not the baseline windows —
// those are a separate comparison concern"). Without this filter,
// snapshotId/datasetDigest changed depending on whether `comparisons` was
// requested, for an otherwise-identical primary response — breaking any
// client-side "did the data change" check.
func filterRowsInPrimaryWindow(rows []domain.ObservedPrice, from, to forecast.Date) []domain.ObservedPrice {
	out := make([]domain.ObservedPrice, 0, len(rows))
	for _, r := range rows {
		d := forecast.DateFromTime(r.TradeDate)
		if d.Before(from) || d.After(to) {
			continue
		}
		out = append(out, r)
	}
	return out
}

// daysInclusive is the number of calendar days in [from,to], both ends
// inclusive — used by the M6 zero-rows Warn (cropPath/groupPath) to
// distinguish a trivially small range (a single-day query legitimately
// returning zero rows isn't suspicious) from a multi-week one.
func daysInclusive(from, to forecast.Date) int {
	return int(to.ToTime().Sub(from.ToTime()).Hours()/24) + 1
}
