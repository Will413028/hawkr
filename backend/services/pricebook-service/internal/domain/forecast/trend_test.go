package forecast

import "testing"

func TestWeekStartIsMonday(t *testing.T) {
	cases := []struct {
		in, want Date
	}{
		{NewDate(2026, 7, 27), NewDate(2026, 7, 27)}, // already Monday
		{NewDate(2026, 7, 29), NewDate(2026, 7, 27)}, // Wednesday
		{NewDate(2026, 8, 2), NewDate(2026, 7, 27)},  // Sunday, same week
		{NewDate(2026, 8, 3), NewDate(2026, 8, 3)},   // next Monday
	}
	for _, c := range cases {
		if got := WeekStart(c.in); got != c.want {
			t.Errorf("WeekStart(%s) = %s, want %s", c.in, got, c.want)
		}
	}
}

func TestPeriodsInRangeDay(t *testing.T) {
	periods := PeriodsInRange(NewDate(2026, 7, 30), NewDate(2026, 8, 1), GranularityDay)
	want := []PeriodBounds{
		{Start: NewDate(2026, 7, 30), End: NewDate(2026, 7, 30)},
		{Start: NewDate(2026, 7, 31), End: NewDate(2026, 7, 31)},
		{Start: NewDate(2026, 8, 1), End: NewDate(2026, 8, 1)},
	}
	if len(periods) != len(want) {
		t.Fatalf("got %d periods, want %d", len(periods), len(want))
	}
	for i := range want {
		if periods[i] != want[i] {
			t.Errorf("period[%d] = %+v, want %+v", i, periods[i], want[i])
		}
	}
}

func TestPeriodsInRangeWeekClipsAtEdges(t *testing.T) {
	// from=2026-07-29 (Wed) to=2026-08-05 (Wed): spans parts of two ISO weeks
	// (week of 07-27..08-02, week of 08-03..08-09). First period must be
	// clipped to start at `from` (not silently extended back to Monday
	// 07-27 — that would misrepresent data we were never asked for as if it
	// were in range); last period clipped to end at `to`.
	periods := PeriodsInRange(NewDate(2026, 7, 29), NewDate(2026, 8, 5), GranularityWeek)
	want := []PeriodBounds{
		{Start: NewDate(2026, 7, 29), End: NewDate(2026, 8, 2)},
		{Start: NewDate(2026, 8, 3), End: NewDate(2026, 8, 5)},
	}
	if len(periods) != len(want) {
		t.Fatalf("got %d periods, want %d: %+v", len(periods), len(want), periods)
	}
	for i := range want {
		if periods[i] != want[i] {
			t.Errorf("period[%d] = %+v, want %+v", i, periods[i], want[i])
		}
	}
}

func TestPeriodsInRangeWeekAlignedExactly(t *testing.T) {
	// from/to exactly bound two full Monday-Sunday weeks — no clipping.
	periods := PeriodsInRange(NewDate(2026, 7, 27), NewDate(2026, 8, 9), GranularityWeek)
	want := []PeriodBounds{
		{Start: NewDate(2026, 7, 27), End: NewDate(2026, 8, 2)},
		{Start: NewDate(2026, 8, 3), End: NewDate(2026, 8, 9)},
	}
	if len(periods) != len(want) {
		t.Fatalf("got %d periods, want %d: %+v", len(periods), len(want), periods)
	}
	for i := range want {
		if periods[i] != want[i] {
			t.Errorf("period[%d] = %+v, want %+v", i, periods[i], want[i])
		}
	}
}

func TestAggregatePeriodValueVolumeWeighted(t *testing.T) {
	v := func(f float64) *float64 { return &f }
	points := []TrendDailyPoint{
		{Date: NewDate(2026, 7, 27), Value: v(40), Volume: v(100)},
		{Date: NewDate(2026, 7, 28), Value: v(50), Volume: v(300)},
	}
	// (40*100 + 50*300) / 400 = 47.5
	value, method, sampleCount := AggregatePeriodValue(points, PeriodBounds{Start: NewDate(2026, 7, 27), End: NewDate(2026, 8, 2)})
	if value == nil || *value != 47.5 {
		t.Errorf("value = %v, want 47.5", value)
	}
	if method != TrendAggregationVolumeWeightedV1 {
		t.Errorf("method = %q, want %q", method, TrendAggregationVolumeWeightedV1)
	}
	if sampleCount != 2 {
		t.Errorf("sampleCount = %d, want 2", sampleCount)
	}
}

func TestAggregatePeriodValueExcludesNilDays(t *testing.T) {
	v := func(f float64) *float64 { return &f }
	points := []TrendDailyPoint{
		{Date: NewDate(2026, 7, 27), Value: v(40), Volume: v(100)},
		{Date: NewDate(2026, 7, 28), Value: nil, Volume: nil}, // no observation that day — must not count as a 0-value participant
	}
	value, _, sampleCount := AggregatePeriodValue(points, PeriodBounds{Start: NewDate(2026, 7, 27), End: NewDate(2026, 8, 2)})
	if value == nil || *value != 40 {
		t.Errorf("value = %v, want 40 (nil day must be excluded, not averaged in as 0)", value)
	}
	if sampleCount != 1 {
		t.Errorf("sampleCount = %d, want 1", sampleCount)
	}
}

func TestAggregatePeriodValueAllNilReturnsNilNotDivideByZero(t *testing.T) {
	points := []TrendDailyPoint{
		{Date: NewDate(2026, 7, 27), Value: nil, Volume: nil},
	}
	value, _, sampleCount := AggregatePeriodValue(points, PeriodBounds{Start: NewDate(2026, 7, 27), End: NewDate(2026, 8, 2)})
	if value != nil {
		t.Errorf("value = %v, want nil", value)
	}
	if sampleCount != 0 {
		t.Errorf("sampleCount = %d, want 0", sampleCount)
	}
}

func TestAggregatePeriodValueZeroVolumeFallsBackToEqualWeight(t *testing.T) {
	v := func(f float64) *float64 { return &f }
	points := []TrendDailyPoint{
		{Date: NewDate(2026, 7, 27), Value: v(40), Volume: v(0)},
		{Date: NewDate(2026, 7, 28), Value: v(60), Volume: v(0)},
	}
	// Both days report a real price but literal 0 volume — must degrade to
	// equal-weight (mean = 50), not divide by zero.
	value, _, _ := AggregatePeriodValue(points, PeriodBounds{Start: NewDate(2026, 7, 27), End: NewDate(2026, 8, 2)})
	if value == nil || *value != 50 {
		t.Errorf("value = %v, want 50 (equal-weight fallback)", value)
	}
}

func TestPeriodsInRangeOnlyDayOrWeek(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("want panic on unknown granularity — caller-contract violation, not a business outcome")
		}
	}()
	PeriodsInRange(NewDate(2026, 7, 27), NewDate(2026, 8, 2), Granularity("month"))
}

func TestExpectedTradingDayOpenAtOneOfManyMarkets(t *testing.T) {
	cat := "N04"
	closures := NewClosures()
	closures.AddMarketCategory("mA", cat, NewDate(2026, 7, 27)) // mA closed that day
	verified := NewVerifiedCalendar([]VerifiedCalendarKey{
		{MarketID: "mA", CategoryCode: cat, Year: 2026}, {MarketID: "mB", CategoryCode: cat, Year: 2026},
	})
	expected, unverified := ExpectedTradingDay(NewDate(2026, 7, 27), []string{"mA", "mB"}, &cat, closures, verified)
	if unverified {
		t.Fatal("want verified=true (mB has coverage)")
	}
	if !expected {
		t.Error("want expected=true — mB was open even though mA was closed")
	}
}

func TestExpectedTradingDayAllClosed(t *testing.T) {
	cat := "N04"
	closures := NewClosures()
	closures.AddMarketCategory("mA", cat, NewDate(2026, 7, 27))
	verified := NewVerifiedCalendar([]VerifiedCalendarKey{{MarketID: "mA", CategoryCode: cat, Year: 2026}})
	expected, unverified := ExpectedTradingDay(NewDate(2026, 7, 27), []string{"mA"}, &cat, closures, verified)
	if expected || unverified {
		t.Errorf("expected=%v unverified=%v, want expected=false unverified=false", expected, unverified)
	}
}

func TestExpectedTradingDayNoCoverageIsUnverifiedNotClosed(t *testing.T) {
	cat := "N04"
	closures := NewClosures()
	verified := NewVerifiedCalendar([]VerifiedCalendarKey{{MarketID: "mOther", CategoryCode: cat, Year: 2026}}) // mA not covered
	expected, unverified := ExpectedTradingDay(NewDate(2026, 7, 27), []string{"mA"}, &cat, closures, verified)
	if expected {
		t.Error("want expected=false")
	}
	if !unverified {
		t.Error("want unverified=true — absence of coverage must not be read as either closed or open")
	}
}

func TestPeriodStatusReady(t *testing.T) {
	cat := "N04"
	closures := NewClosures()
	verified := NewVerifiedCalendar([]VerifiedCalendarKey{{MarketID: "mA", CategoryCode: cat, Year: 2026}})
	bounds := PeriodBounds{Start: NewDate(2026, 7, 27), End: NewDate(2026, 7, 28)}
	present := map[Date]bool{NewDate(2026, 7, 27): true, NewDate(2026, 7, 28): true}
	got := PeriodStatus(bounds, present, []string{"mA"}, &cat, closures, verified)
	if got != PredictionStatusReady {
		t.Errorf("status = %s, want ready", got)
	}
}

func TestPeriodStatusPartial(t *testing.T) {
	cat := "N04"
	closures := NewClosures()
	verified := NewVerifiedCalendar([]VerifiedCalendarKey{{MarketID: "mA", CategoryCode: cat, Year: 2026}})
	bounds := PeriodBounds{Start: NewDate(2026, 7, 27), End: NewDate(2026, 7, 28)}
	present := map[Date]bool{NewDate(2026, 7, 27): true} // 7-28 expected but missing
	got := PeriodStatus(bounds, present, []string{"mA"}, &cat, closures, verified)
	if got != PredictionStatusPartial {
		t.Errorf("status = %s, want partial", got)
	}
}

func TestPeriodStatusMarketClosed(t *testing.T) {
	cat := "N04"
	closures := NewClosures()
	closures.AddMarketCategory("mA", cat, NewDate(2026, 7, 27))
	closures.AddMarketCategory("mA", cat, NewDate(2026, 7, 28))
	verified := NewVerifiedCalendar([]VerifiedCalendarKey{{MarketID: "mA", CategoryCode: cat, Year: 2026}})
	bounds := PeriodBounds{Start: NewDate(2026, 7, 27), End: NewDate(2026, 7, 28)}
	got := PeriodStatus(bounds, map[Date]bool{}, []string{"mA"}, &cat, closures, verified)
	if got != PredictionStatusMarketClosed {
		t.Errorf("status = %s, want market_closed", got)
	}
}

func TestPeriodStatusInsufficientDataNotAllClosed(t *testing.T) {
	cat := "N04"
	closures := NewClosures() // nothing closed
	verified := NewVerifiedCalendar([]VerifiedCalendarKey{{MarketID: "mA", CategoryCode: cat, Year: 2026}})
	bounds := PeriodBounds{Start: NewDate(2026, 7, 27), End: NewDate(2026, 7, 28)}
	got := PeriodStatus(bounds, map[Date]bool{}, []string{"mA"}, &cat, closures, verified) // expected but zero present
	if got != PredictionStatusInsufficientData {
		t.Errorf("status = %s, want insufficient_data", got)
	}
}

func TestPeriodStatusUnverifiedDefaultsToInsufficientData(t *testing.T) {
	cat := "N04"
	closures := NewClosures()
	// UnenforcedVerifiedCalendar() would not produce a real gap here:
	// enforced=false makes IsVerified always return true. Use a real
	// verified calendar with no keys instead, so nothing is verified.
	verified := NewVerifiedCalendar(nil) // no keys at all — nothing is verified
	bounds := PeriodBounds{Start: NewDate(2026, 7, 27), End: NewDate(2026, 7, 28)}
	got := PeriodStatus(bounds, map[Date]bool{}, []string{"mA"}, &cat, closures, verified)
	if got != PredictionStatusInsufficientData {
		t.Errorf("status = %s, want insufficient_data (conservative default when calendar is unverified)", got)
	}
}

func TestBaselinePeriodWoW(t *testing.T) {
	latest := PeriodBounds{Start: NewDate(2026, 7, 27), End: NewDate(2026, 8, 2)}
	got := BaselinePeriod(latest, ComparisonWoW)
	want := PeriodBounds{Start: NewDate(2026, 7, 20), End: NewDate(2026, 7, 26)}
	if got != want {
		t.Errorf("BaselinePeriod(wow) = %+v, want %+v", got, want)
	}
}

func TestBaselinePeriodYoY364StaysWeekAligned(t *testing.T) {
	latest := PeriodBounds{Start: NewDate(2026, 7, 27), End: NewDate(2026, 8, 2)} // Monday-Sunday
	got := BaselinePeriod(latest, ComparisonYoY364)
	// 364 days = exactly 52 weeks back, so the baseline period's Start must
	// still land on a Monday (2025-07-29) — the whole reason yoy364 uses
	// 364 days instead of a naive "same month/day last year" rule.
	if got.Start.PyWeekday() != 0 {
		t.Errorf("BaselinePeriod(yoy364).Start = %s is not a Monday", got.Start)
	}
	want := PeriodBounds{Start: NewDate(2025, 7, 28), End: NewDate(2025, 8, 3)}
	if got != want {
		t.Errorf("BaselinePeriod(yoy364) = %+v, want %+v", got, want)
	}
}

func TestComputeComparisonReady(t *testing.T) {
	v := func(f float64) *float64 { return &f }
	c := ComputeComparison(v(42.5), PredictionStatusReady, v(40), PredictionStatusReady)
	if c.ChangePercent == nil || *c.ChangePercent != 6.25 {
		t.Errorf("ChangePercent = %v, want 6.25", c.ChangePercent)
	}
	if c.ChangeAmount == nil || *c.ChangeAmount != 2.5 {
		t.Errorf("ChangeAmount = %v, want 2.5", c.ChangeAmount)
	}
	if c.ReasonCode != nil {
		t.Errorf("ReasonCode = %v, want nil", c.ReasonCode)
	}
}

func TestComputeComparisonMissingBaselineNeverFakesZeroPercent(t *testing.T) {
	v := func(f float64) *float64 { return &f }
	c := ComputeComparison(v(42.5), PredictionStatusReady, nil, PredictionStatusInsufficientData)
	if c.ChangePercent != nil {
		t.Errorf("ChangePercent = %v, want nil", c.ChangePercent)
	}
	if c.ReasonCode == nil || *c.ReasonCode != "missing_baseline" {
		t.Errorf("ReasonCode = %v, want \"missing_baseline\"", c.ReasonCode)
	}
}

func TestComputeComparisonZeroBaselineNeverDividesByZero(t *testing.T) {
	v := func(f float64) *float64 { return &f }
	c := ComputeComparison(v(42.5), PredictionStatusReady, v(0), PredictionStatusReady)
	if c.ChangePercent != nil {
		t.Errorf("ChangePercent = %v, want nil", c.ChangePercent)
	}
	if c.ReasonCode == nil || *c.ReasonCode != "zero_baseline" {
		t.Errorf("ReasonCode = %v, want \"zero_baseline\"", c.ReasonCode)
	}
}

func TestComputeComparisonStatusIsWorstOfCurrentAndBaseline(t *testing.T) {
	v := func(f float64) *float64 { return &f }
	// Current period only partially elapsed (e.g. this week, 2 of 7 days in)
	// while the baseline (2 weeks ago) is fully ready — the comparison must
	// surface partial, not silently report ready just because the baseline
	// side happens to be complete.
	c := ComputeComparison(v(42.5), PredictionStatusPartial, v(40), PredictionStatusReady)
	if c.Status != PredictionStatusPartial {
		t.Errorf("Status = %s, want partial (current's worse status must win)", c.Status)
	}
}

func TestComputeComparisonMissingCurrentDataIsLabeledDistinctlyFromMissingBaseline(t *testing.T) {
	v := func(f float64) *float64 { return &f }
	c := ComputeComparison(nil, PredictionStatusInsufficientData, v(40), PredictionStatusReady)
	if c.ReasonCode == nil || *c.ReasonCode != "missing_current_data" {
		t.Errorf("ReasonCode = %v, want \"missing_current_data\"", c.ReasonCode)
	}
	if c.ChangePercent != nil {
		t.Errorf("ChangePercent = %v, want nil", c.ChangePercent)
	}
}
