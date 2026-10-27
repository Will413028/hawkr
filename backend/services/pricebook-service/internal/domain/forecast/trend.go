package forecast

// Granularity is trend-summary's day/week bucketing unit (issue #2249
// Slice B).
type Granularity string

const (
	GranularityDay  Granularity = "day"
	GranularityWeek Granularity = "week"
)

// TrendAggregationVolumeWeightedV1 tags trend-summary's temporal (day→period)
// volume-weighted reduction. Deliberately a distinct version string from
// AggregationMethodVolumeWeightedTwoStage/AggregationMethodEqualWeightFallback
// (group.go's spatial axis) — the two aggregation axes (space vs time) are
// independent and must never be confused with each other in a response.
const TrendAggregationVolumeWeightedV1 = "volume_weighted/v1"

// WeekStart returns the Monday on or before d (PyWeekday()==0 is Monday).
func WeekStart(d Date) Date {
	return d.AddDays(-d.PyWeekday())
}

// PeriodBounds is a period's inclusive [Start, End] calendar-date range.
type PeriodBounds struct {
	Start, End Date
}

// PeriodsInRange splits [from, to] (inclusive) into consecutive periods.
// GranularityDay: one 1-day period per calendar date. GranularityWeek:
// Monday-Sunday buckets, clipped to [from, to] at the first/last period —
// a period at either edge of the requested range may span fewer than 7
// calendar days; that is a caller-visible fact (PeriodStatus, Task 4, is
// what tells the caller a period is partial — this function does not
// silently widen the request to a full calendar week it wasn't asked for).
// Panics on any Granularity other than the two constants above — an unknown
// granularity is a caller-contract violation the gRPC layer must catch
// before this is ever called (see Task 9's eager validation), not a
// business outcome this function should degrade gracefully for.
func PeriodsInRange(from, to Date, g Granularity) []PeriodBounds {
	var out []PeriodBounds
	switch g {
	case GranularityDay:
		for d := from; !d.After(to); d = d.AddDays(1) {
			out = append(out, PeriodBounds{Start: d, End: d})
		}
	case GranularityWeek:
		cursor := from
		for !cursor.After(to) {
			weekEnd := WeekStart(cursor).AddDays(6)
			end := weekEnd
			if end.After(to) {
				end = to
			}
			out = append(out, PeriodBounds{Start: cursor, End: end})
			cursor = end.AddDays(1)
		}
	default:
		panic("forecast: unknown granularity " + string(g))
	}
	return out
}

// TrendDailyPoint is one calendar date's already spatially-aggregated value
// and weight — the common shape both the crop path (one raw row per date,
// already single-market) and the group path (AggregateGroupObservations'
// GroupObservedPoint, Task 2) converge to before temporal bucketing. nil
// Value means no observation that date; Volume is nil exactly when Value is
// nil (mirrors GroupObservedPoint's own nil-pairing), non-nil-but-zero is a
// distinct, legitimate "priced but volumeless" state.
type TrendDailyPoint struct {
	Date   Date
	Value  *float64
	Volume *float64
}

// AggregatePeriodValue reduces the daily points whose Date falls inside
// bounds into one volume-weighted period value, reusing WeightedAverage
// (group.go) — the same positive-weight-sum-zero-falls-back-to-equal-weight
// behavior the spatial aggregation already relies on, so a period whose
// contributing days all report a real price but literal 0 volume degrades
// to an equal-weight mean instead of dividing by zero. Days with a nil
// Value are excluded entirely from the average (never treated as a
// zero-value participant). sampleCount is the count of non-nil days
// contributing, independent of method's fallback status. Returns
// (nil, "", 0) when zero days in bounds have data.
func AggregatePeriodValue(points []TrendDailyPoint, bounds PeriodBounds) (value *float64, method string, sampleCount int) {
	var values, weights []float64
	for _, p := range points {
		if p.Date.Before(bounds.Start) || p.Date.After(bounds.End) {
			continue
		}
		if p.Value == nil {
			continue
		}
		values = append(values, *p.Value)
		weight := 0.0
		if p.Volume != nil {
			weight = *p.Volume
		}
		weights = append(weights, weight)
	}
	if len(values) == 0 {
		return nil, "", 0
	}
	avg, _ := WeightedAverage(values, weights)
	return &avg, TrendAggregationVolumeWeightedV1, len(values)
}

// ExpectedTradingDay reports whether at least one of marketIDs was open
// (not a declared rest day) for categoryCode on date d. calendarUnverified
// is true when NONE of marketIDs has verified rest-day coverage for d's
// year — the caller must then treat d as neither closed nor confirmed open
// (mirrors the existing calendar_unverified precedent the forecast pipeline
// already applies; see MissingReasonCalendarUnverified / VerifiedCalendar's
// own doc comment). A market with verified coverage that is NOT closed
// short-circuits to expected=true immediately — the group path only needs
// ONE of its markets open to produce a meaningful aggregate that day.
func ExpectedTradingDay(d Date, marketIDs []string, categoryCode *string, closures *Closures, verified VerifiedCalendar) (expected bool, calendarUnverified bool) {
	anyVerified := false
	for _, marketID := range marketIDs {
		if !verified.IsVerified(marketID, categoryCode, d.Year) {
			continue
		}
		anyVerified = true
		if !closures.IsClosed(marketID, categoryCode, d) {
			return true, false
		}
	}
	if !anyVerified {
		return false, true
	}
	return false, false
}

// PeriodStatus derives issue #2249 Slice B's period-level status by
// comparing which calendar dates inside bounds actually have observation
// data (presentDates) against which dates were expected trading days (via
// ExpectedTradingDay for every date in bounds). Mirrors group.go's
// aggregateOneHorizon status derivation (all-closed vs zero-effective vs
// partial vs ready), generalized from "7 fixed forecast horizons" to "an
// arbitrary period's calendar dates". Dates whose calendar coverage is
// unverified are excluded from both the expected and present tallies —
// unverified stays conservative and does not manufacture false partial-ness.
func PeriodStatus(bounds PeriodBounds, presentDates map[Date]bool, marketIDs []string, categoryCode *string, closures *Closures, verified VerifiedCalendar) PredictionStatus {
	expectedCount, presentExpectedCount, verifiedCount := 0, 0, 0
	for d := bounds.Start; !d.After(bounds.End); d = d.AddDays(1) {
		expected, unverified := ExpectedTradingDay(d, marketIDs, categoryCode, closures, verified)
		if unverified {
			continue
		}
		verifiedCount++
		if expected {
			expectedCount++
			if presentDates[d] {
				presentExpectedCount++
			}
		}
	}
	switch {
	case verifiedCount == 0:
		return PredictionStatusInsufficientData
	case expectedCount == 0:
		return PredictionStatusMarketClosed
	case presentExpectedCount == 0:
		return PredictionStatusInsufficientData
	case presentExpectedCount < expectedCount:
		return PredictionStatusPartial
	default:
		return PredictionStatusReady
	}
}

// TrendComparisonKind selects which baseline offset a comparison uses — WoW
// and YoY364 differ only in that offset (7 vs 364 calendar days back), so
// callers share BaselinePeriod/ComputeComparison via this parameter instead
// of two near-duplicate code paths.
type TrendComparisonKind string

const (
	ComparisonWoW    TrendComparisonKind = "wow"
	ComparisonYoY364 TrendComparisonKind = "yoy364"
)

// BaselinePeriod returns the equal-length period immediately preceding
// latest, shifted back by 7 (WoW) or 364 (YoY364) calendar days. 364 is
// exactly 52×7, so shifting a Monday-start week back 364 days always lands
// on another Monday — this is what "364-day aligned YoY" (issue #2249
// Slice B) means, as opposed to a naive "same month/day last year" rule
// that would drift off week boundaries.
func BaselinePeriod(latest PeriodBounds, kind TrendComparisonKind) PeriodBounds {
	offset := 7
	if kind == ComparisonYoY364 {
		offset = 364
	}
	return PeriodBounds{Start: latest.Start.AddDays(-offset), End: latest.End.AddDays(-offset)}
}

// TrendComparison is one WoW/YoY364 comparison result. Status reuses the
// same PredictionStatus vocabulary PeriodStatus produces for the baseline
// period — ready/partial/insufficient_data/market_closed — so a caller can
// tell "baseline exists but is only partial" from "baseline genuinely
// doesn't exist" instead of collapsing both into one flag.
type TrendComparison struct {
	Status        PredictionStatus
	CurrentValue  *float64
	BaselineValue *float64
	ChangeAmount  *float64
	ChangePercent *float64
	ReasonCode    *string
}

// PredictionStatusSeverity ranks PredictionStatus by how much a caller
// should distrust a number carried alongside it — Ready best,
// InsufficientData worst. MarketClosed ranks above InsufficientData: a
// closed market is a known, legitimate business state (same distinction
// group.go's aggregateOneHorizon already draws), while insufficient data
// signals something unexpected that should have been there.
//
// Exported because more than one caller now needs to fold several statuses
// into "the worst of these": ComputeComparison (current vs baseline period)
// and internal/usecase/spatial_surface.go (a group's sub-variants at one
// market). Two hand-rolled orderings would drift, and the drift would be
// silent — both callers use the answer to decide how confident a number
// shown to a seller looks.
func PredictionStatusSeverity(s PredictionStatus) int {
	switch s {
	case PredictionStatusReady:
		return 0
	case PredictionStatusPartial:
		return 1
	case PredictionStatusMarketClosed:
		return 2
	default: // PredictionStatusInsufficientData, or any future/unknown value
		return 3
	}
}

// ComputeComparison never fabricates a 0% change: a missing or zero
// baseline always produces ChangePercent=nil plus a ReasonCode explaining
// why, per issue #2249 Slice B's explicit rule ("baseline 缺失或為零時
// percent 必須 null + reasonCode，不得顯示 0%"). Status is the worse of
// currentStatus/baselineStatus (PredictionStatusSeverity, same
// worst-of-rollup philosophy as Task 7's worstUsability): a partial current
// period must not be masked by a fully-ready baseline into looking like a
// trustworthy comparison. ReasonCode distinguishes which side is actually
// missing data — "missing_current_data" vs "missing_baseline" — instead of
// collapsing both into one label.
func ComputeComparison(currentValue *float64, currentStatus PredictionStatus, baselineValue *float64, baselineStatus PredictionStatus) TrendComparison {
	status := baselineStatus
	if PredictionStatusSeverity(currentStatus) > PredictionStatusSeverity(baselineStatus) {
		status = currentStatus
	}
	c := TrendComparison{Status: status, CurrentValue: currentValue, BaselineValue: baselineValue}
	if currentValue == nil {
		reason := "missing_current_data"
		c.ReasonCode = &reason
		return c
	}
	if baselineValue == nil {
		reason := "missing_baseline"
		c.ReasonCode = &reason
		return c
	}
	if *baselineValue == 0 {
		reason := "zero_baseline"
		c.ReasonCode = &reason
		return c
	}
	amount := *currentValue - *baselineValue
	pct := (amount / *baselineValue) * 100
	c.ChangeAmount = &amount
	c.ChangePercent = &pct
	return c
}
