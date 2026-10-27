package forecast

import "fmt"

// FeatureConfig mirrors features.py's FeatureConfig (features.py:184-232).
// The 36-dim schema is a contract: any change to these defaults must be
// mirrored in model.json's config.feature_config or Go and Python will
// build vectors of different shapes and the parity test will fail.
//
// Python's constructor sorts price_lags/volume_lags/rolling_windows
// (features.py:192-199); Go does not replicate that auto-sort — the only two
// call sites in P3 (DefaultFeatureConfig and model.json's feature_config)
// are already sorted ascending, and adding silent reordering here would be
// speculative generality with no exercised caller.
type FeatureConfig struct {
	PriceLags             []int `json:"price_lags"`
	VolumeLags            []int `json:"volume_lags"`
	RollingWindows        []int `json:"rolling_windows"`
	MaxDaysSinceLastTrade int   `json:"max_days_since_last_trade"`
}

// DefaultFeatureConfig mirrors FeatureConfig()'s dataclass defaults
// (features.py:187-190).
func DefaultFeatureConfig() FeatureConfig {
	return FeatureConfig{
		PriceLags:             []int{1, 2, 3, 7, 14, 28},
		VolumeLags:            []int{1, 7, 14},
		RollingWindows:        []int{7, 14, 28},
		MaxDaysSinceLastTrade: 365,
	}
}

// Validate mirrors FeatureConfig.__post_init__'s checks (features.py:192-201).
func (c FeatureConfig) Validate() error {
	for name, values := range map[string][]int{
		"price_lags":      c.PriceLags,
		"volume_lags":     c.VolumeLags,
		"rolling_windows": c.RollingWindows,
	} {
		if len(values) == 0 {
			return fmt.Errorf("forecast: %s must contain positive integers", name)
		}
		seen := make(map[int]bool, len(values))
		for _, v := range values {
			if v <= 0 {
				return fmt.Errorf("forecast: %s must contain positive integers, got %d", name, v)
			}
			if seen[v] {
				return fmt.Errorf("forecast: %s must not contain duplicates, got %d twice", name, v)
			}
			seen[v] = true
		}
	}
	if c.MaxDaysSinceLastTrade <= 0 {
		return fmt.Errorf("forecast: max_days_since_last_trade must be positive, got %d", c.MaxDaysSinceLastTrade)
	}
	return nil
}

// MaxLookbackDays 鏡射 forecasting.py:186-196：訓練用到的最長回看窗口決定了
// as-of 查詢要往回撈多少天的歷史。usecase.Forecast（SeriesForecast/
// GroupForecast）與 cmd/pricebook-feature-health 都需要這個值來決定查詢窗，
// 兩處原本各自複製一份（P4 review followup）；促成 domain/forecast 的方法，
// 兩處呼叫同一份實作。
func (c FeatureConfig) MaxLookbackDays() int {
	m := c.MaxDaysSinceLastTrade
	for _, v := range c.PriceLags {
		if v > m {
			m = v
		}
	}
	for _, v := range c.VolumeLags {
		if v > m {
			m = v
		}
	}
	for _, v := range c.RollingWindows {
		if v > m {
			m = v
		}
	}
	return m
}

// FeatureNames returns the 36 feature names in contract order
// (features.py:203-224). BuildFeatureVectorFromDailyPoints (Task 3) errors
// if its output length ever diverges from len(FeatureNames()).
func (c FeatureConfig) FeatureNames() []string {
	names := make([]string, 0, 3+2*len(c.PriceLags)+2*len(c.VolumeLags)+4*len(c.RollingWindows)+3)
	names = append(names, "horizon_days", "target_weekday_sin", "target_weekday_cos")
	for _, lag := range c.PriceLags {
		names = append(names, fmt.Sprintf("price_lag_%d", lag), fmt.Sprintf("price_lag_%d_missing", lag))
	}
	for _, lag := range c.VolumeLags {
		names = append(names, fmt.Sprintf("volume_lag_%d", lag), fmt.Sprintf("volume_lag_%d_missing", lag))
	}
	for _, window := range c.RollingWindows {
		names = append(names,
			fmt.Sprintf("price_roll_mean_%d", window),
			fmt.Sprintf("price_roll_median_%d", window),
			fmt.Sprintf("volume_roll_mean_%d", window),
			fmt.Sprintf("observed_ratio_%d", window),
		)
	}
	names = append(names, "last_price", "last_price_missing", "days_since_last_trade")
	return names
}
