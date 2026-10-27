package forecast

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// FeatureVector is the 36-dim input to RidgeParameters.Predict, mirroring
// Python's FeatureVector (features.py:245-254).
type FeatureVector struct {
	Names                 []string
	Values                []float64
	MaxFeatureAvailableAt *time.Time
}

// Value 按特徵名稱取值。編排層需要 last_price_missing 來選分支
// （forecasting.py:317 用 feature.as_dict()["last_price_missing"]），
// 而寫死 index 34 會在 FeatureConfig 改動時安靜取到別的維度。
func (f FeatureVector) Value(name string) (float64, bool) {
	for i, n := range f.Names {
		if n == name {
			return f.Values[i], true
		}
	}
	return 0, false
}

func validPrice(p PricePoint, ok bool) (float64, bool) {
	if !ok || !p.IsActive || p.IsMarketClosed || p.AveragePrice == nil {
		return 0, false
	}
	return *p.AveragePrice, true
}

func validVolume(p PricePoint, ok bool) (float64, bool) {
	if !ok || !p.IsActive || p.IsMarketClosed || p.Volume == nil {
		return 0, false
	}
	return *p.Volume, true
}

// visiblePoint mirrors the visible_point closure that appears independently
// inside both build_feature_vector_from_daily_points (features.py:331-335)
// and seasonal_naive_from_daily_points (features.py:454-458) — those two
// Python closures are byte-for-byte identical, so Go keeps one shared
// implementation instead of the duplication.
func visiblePoint(pointsByDate map[Date]PricePoint, originDate Date, cutoff time.Time, tradeDate Date) (PricePoint, bool) {
	p, ok := pointsByDate[tradeDate]
	if !ok || p.TradeDate.After(originDate) || p.AvailableAt.After(cutoff) {
		return PricePoint{}, false
	}
	return p, true
}

// BuildFeatureVectorFromDailyPoints ports build_feature_vector_from_daily_points
// (features.py:310-407). pointsByDate must already hold at most one
// PricePoint per calendar date — this function does not resolve revisions
// (that is latest_points_as_of's job; out of scope for P3, see plan's
// 範圍決策 section).
func BuildFeatureVectorFromDailyPoints(
	pointsByDate map[Date]PricePoint,
	origin time.Time,
	horizonDays int,
	config FeatureConfig,
	calendarTimezone string,
) (FeatureVector, error) {
	if horizonDays < 1 || horizonDays > 7 {
		return FeatureVector{}, fmt.Errorf("forecast: horizon_days must be between 1 and 7, got %d", horizonDays)
	}
	if err := config.Validate(); err != nil {
		return FeatureVector{}, err
	}
	originDate, cutoff, err := OriginDateAndCutoff(origin, calendarTimezone)
	if err != nil {
		return FeatureVector{}, err
	}

	targetDate := originDate.AddDays(horizonDays)
	weekdayAngle := 2.0 * math.Pi * float64(targetDate.PyWeekday()) / 7.0
	values := []float64{
		float64(horizonDays),
		math.Sin(weekdayAngle),
		math.Cos(weekdayAngle),
	}

	for _, lag := range config.PriceLags {
		p, ok := visiblePoint(pointsByDate, originDate, cutoff, originDate.AddDays(-lag))
		if value, present := validPrice(p, ok); present {
			values = append(values, value, 0.0)
		} else {
			values = append(values, 0.0, 1.0)
		}
	}

	for _, lag := range config.VolumeLags {
		p, ok := visiblePoint(pointsByDate, originDate, cutoff, originDate.AddDays(-lag))
		if value, present := validVolume(p, ok); present {
			values = append(values, value, 0.0)
		} else {
			values = append(values, 0.0, 1.0)
		}
	}

	for _, window := range config.RollingWindows {
		var prices, volumes []float64
		for lag := 1; lag <= window; lag++ {
			p, ok := visiblePoint(pointsByDate, originDate, cutoff, originDate.AddDays(-lag))
			if value, present := validPrice(p, ok); present {
				prices = append(prices, value)
			}
			if value, present := validVolume(p, ok); present {
				volumes = append(volumes, value)
			}
		}
		values = append(values,
			meanOrZero(prices),
			medianOrZero(prices),
			meanOrZero(volumes),
			float64(len(prices))/float64(window),
		)
	}

	lastPrice, lastPriceMissing, daysSinceLastTrade := 0.0, 1.0, float64(config.MaxDaysSinceLastTrade)
	for offset := 0; offset <= config.MaxDaysSinceLastTrade; offset++ {
		p, ok := visiblePoint(pointsByDate, originDate, cutoff, originDate.AddDays(-offset))
		if value, present := validPrice(p, ok); present {
			lastPrice, lastPriceMissing, daysSinceLastTrade = value, 0.0, float64(offset)
			break
		}
	}
	values = append(values, lastPrice, lastPriceMissing, daysSinceLastTrade)

	maxLookback := config.MaxDaysSinceLastTrade
	for _, lag := range concatLags(config) {
		if lag > maxLookback {
			maxLookback = lag
		}
	}
	var maxAvailableAt *time.Time
	for offset := 0; offset <= maxLookback; offset++ {
		p, ok := visiblePoint(pointsByDate, originDate, cutoff, originDate.AddDays(-offset))
		if !ok {
			continue
		}
		if maxAvailableAt == nil || p.AvailableAt.After(*maxAvailableAt) {
			at := p.AvailableAt
			maxAvailableAt = &at
		}
	}

	names := config.FeatureNames()
	if len(names) != len(values) {
		return FeatureVector{}, fmt.Errorf("forecast: feature schema and feature values are out of sync: %d names, %d values", len(names), len(values))
	}
	return FeatureVector{Names: names, Values: values, MaxFeatureAvailableAt: maxAvailableAt}, nil
}

func concatLags(config FeatureConfig) []int {
	out := make([]int, 0, len(config.PriceLags)+len(config.VolumeLags)+len(config.RollingWindows))
	out = append(out, config.PriceLags...)
	out = append(out, config.VolumeLags...)
	out = append(out, config.RollingWindows...)
	return out
}

func meanOrZero(xs []float64) float64 {
	if len(xs) == 0 {
		return 0.0
	}
	sum := 0.0
	for _, x := range xs {
		sum += x
	}
	return sum / float64(len(xs))
}

// medianOrZero mirrors numpy.median's linear-interpolation midpoint for even
// counts (features.py:362 uses np.median with no override).
func medianOrZero(xs []float64) float64 {
	if len(xs) == 0 {
		return 0.0
	}
	sorted := append([]float64{}, xs...)
	sort.Float64s(sorted)
	n := len(sorted)
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2.0
}

// SeasonalNaiveFromDailyPoints ports seasonal_naive_from_daily_points
// (features.py:440-467).
func SeasonalNaiveFromDailyPoints(
	pointsByDate map[Date]PricePoint,
	origin time.Time,
	targetDate Date,
	fallbackPrice *float64,
	seasonalLagDays int,
	maxDaysSinceLastTrade int,
	calendarTimezone string,
) (*float64, error) {
	originDate, cutoff, err := OriginDateAndCutoff(origin, calendarTimezone)
	if err != nil {
		return nil, err
	}
	visiblePrice := func(tradeDate Date) (float64, bool) {
		p, ok := visiblePoint(pointsByDate, originDate, cutoff, tradeDate)
		return validPrice(p, ok)
	}
	if seasonal, ok := visiblePrice(targetDate.AddDays(-seasonalLagDays)); ok {
		return &seasonal, nil
	}
	for offset := 0; offset <= maxDaysSinceLastTrade; offset++ {
		if latest, ok := visiblePrice(originDate.AddDays(-offset)); ok {
			return &latest, nil
		}
	}
	return fallbackPrice, nil
}
