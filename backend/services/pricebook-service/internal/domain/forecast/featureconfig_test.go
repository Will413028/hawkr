package forecast

import "testing"

func TestDefaultFeatureConfigFeatureNamesHas36Entries(t *testing.T) {
	names := DefaultFeatureConfig().FeatureNames()
	if len(names) != 36 {
		t.Fatalf("FeatureNames() 回 %d 個名稱，預期 36（設計書 §5）", len(names))
	}
	wantFirst3 := []string{"horizon_days", "target_weekday_sin", "target_weekday_cos"}
	for i, w := range wantFirst3 {
		if names[i] != w {
			t.Errorf("names[%d] = %q，預期 %q", i, names[i], w)
		}
	}
	wantLast3 := []string{"last_price", "last_price_missing", "days_since_last_trade"}
	for i, w := range wantLast3 {
		if names[33+i] != w {
			t.Errorf("names[%d] = %q，預期 %q", 33+i, names[33+i], w)
		}
	}
}

func TestFeatureConfigValidateRejectsDuplicateLags(t *testing.T) {
	c := FeatureConfig{PriceLags: []int{1, 1}, VolumeLags: []int{1}, RollingWindows: []int{7}, MaxDaysSinceLastTrade: 365}
	if err := c.Validate(); err == nil {
		t.Fatal("重複的 price_lags 應回錯誤")
	}
}

func TestFeatureConfigValidateRejectsNonPositiveMaxDays(t *testing.T) {
	c := DefaultFeatureConfig()
	c.MaxDaysSinceLastTrade = 0
	if err := c.Validate(); err == nil {
		t.Fatal("max_days_since_last_trade=0 應回錯誤")
	}
}

// TestFeatureConfigMaxLookbackDaysPicksLargestAcrossAllFour 釘住
// MaxLookbackDays 真的取四個維度（MaxDaysSinceLastTrade/PriceLags/VolumeLags/
// RollingWindows）的最大值，而不是只看其中一兩個——usecase.Forecast 與
// cmd/pricebook-feature-health 都靠這個值決定歷史查詢窗，之前兩處各自複製
// 一份實作卻沒有任何測試釘住邏輯本身（P4 review followup）。
func TestFeatureConfigMaxLookbackDaysPicksLargestAcrossAllFour(t *testing.T) {
	cases := []struct {
		name string
		cfg  FeatureConfig
		want int
	}{
		{"MaxDaysSinceLastTrade 最大", FeatureConfig{MaxDaysSinceLastTrade: 365, PriceLags: []int{1, 2}, VolumeLags: []int{1}, RollingWindows: []int{7}}, 365},
		{"PriceLags 最大", FeatureConfig{MaxDaysSinceLastTrade: 10, PriceLags: []int{1, 400}, VolumeLags: []int{1}, RollingWindows: []int{7}}, 400},
		{"VolumeLags 最大", FeatureConfig{MaxDaysSinceLastTrade: 10, PriceLags: []int{1}, VolumeLags: []int{1, 500}, RollingWindows: []int{7}}, 500},
		{"RollingWindows 最大", FeatureConfig{MaxDaysSinceLastTrade: 10, PriceLags: []int{1}, VolumeLags: []int{1}, RollingWindows: []int{7, 600}}, 600},
		{"預設值", DefaultFeatureConfig(), 365}, // MaxDaysSinceLastTrade=365 全面蓋過 lags/windows
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.cfg.MaxLookbackDays(); got != tc.want {
				t.Errorf("MaxLookbackDays() = %d，預期 %d", got, tc.want)
			}
		})
	}
}
