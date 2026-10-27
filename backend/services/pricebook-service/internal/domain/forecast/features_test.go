package forecast

import (
	"math"
	"testing"
	"time"
)

// mustPoint mirrors tests/test_features.py's point() helper (series moa:104:LA1).
func mustPoint(t *testing.T, day int, priceVal *float64, availableDay int, closed bool) PricePoint {
	t.Helper()
	var volume *float64
	if priceVal != nil {
		v := 100.0 + float64(day)
		volume = &v
	}
	p, err := NewPricePoint(PricePointInput{
		SeriesID: "moa:104:LA1", MarketID: "moa:104", CropID: "moa:LA1",
		TradeDate:      NewDate(2025, 1, day),
		AveragePrice:   priceVal,
		Volume:         volume,
		AvailableAt:    time.Date(2025, 1, availableDay, 18, 0, 0, 0, time.UTC),
		IsMarketClosed: closed,
	})
	if err != nil {
		t.Fatalf("mustPoint(day=%d) 失敗: %v", day, err)
	}
	return p
}

func assertClose(t *testing.T, label string, got, want, tol float64) {
	t.Helper()
	diff := got - want
	if diff < 0 {
		diff = -diff
	}
	// NaN 檢查獨立於 diff > tol：NaN 的比較永遠是 false（NaN - want、以及
	// NaN > tol 皆是），少了這行 got=NaN 會被誤判「通過」。
	if math.IsNaN(got) || math.IsNaN(diff) || diff > tol {
		t.Errorf("%s = %v，預期 %v（容差 %v，實差 %v）", label, got, want, tol, diff)
	}
}

func indexByDate(points ...PricePoint) map[Date]PricePoint {
	m := make(map[Date]PricePoint, len(points))
	for _, p := range points {
		m[p.TradeDate] = p
	}
	return m
}

func TestFeatureVectorValueLooksUpByName(t *testing.T) {
	fv := FeatureVector{Names: []string{"a", "last_price_missing"}, Values: []float64{1, 0}}
	if v, ok := fv.Value("last_price_missing"); !ok || v != 0 {
		t.Errorf("Value(last_price_missing) = (%v, %v)，預期 (0, true)", v, ok)
	}
	if _, ok := fv.Value("nope"); ok {
		t.Error("未知名稱必須回 false，不能回零值假裝找到了")
	}
}

// 對應 test_features_are_calendar_aligned_and_expose_missing_masks
// （tests/test_features.py:37-65）：1 月 9 日刻意缺資料，缺的那天不能讓後面
// 日期補位變成 lag 1。
func TestBuildFeatureVectorFromDailyPoints_MissingDayDoesNotShiftLag(t *testing.T) {
	config := FeatureConfig{PriceLags: []int{1, 2, 7}, VolumeLags: []int{1, 2}, RollingWindows: []int{3}, MaxDaysSinceLastTrade: 365}
	points := indexByDate(
		mustPoint(t, 7, price(17.0), 7, false),
		mustPoint(t, 8, price(18.0), 8, false),
		// 1 月 9 日刻意缺席
		mustPoint(t, 10, price(20.0), 10, false),
	)
	origin := time.Date(2025, 1, 10, 23, 0, 0, 0, time.UTC)

	fv, err := BuildFeatureVectorFromDailyPoints(points, origin, 1, config, "UTC")
	if err != nil {
		t.Fatalf("BuildFeatureVectorFromDailyPoints 回錯誤: %v", err)
	}
	get := func(name string) float64 {
		v, ok := fv.Value(name)
		if !ok {
			t.Fatalf("找不到特徵 %s", name)
		}
		return v
	}
	assertClose(t, "price_lag_1", get("price_lag_1"), 0.0, 0)
	assertClose(t, "price_lag_1_missing", get("price_lag_1_missing"), 1.0, 0)
	assertClose(t, "price_lag_2", get("price_lag_2"), 18.0, 0)
	assertClose(t, "price_lag_2_missing", get("price_lag_2_missing"), 0.0, 0)
	assertClose(t, "price_roll_mean_3", get("price_roll_mean_3"), 17.5, 1e-12)
	assertClose(t, "observed_ratio_3", get("observed_ratio_3"), 2.0/3.0, 1e-12)
	assertClose(t, "last_price", get("last_price"), 20.0, 0)
	assertClose(t, "days_since_last_trade", get("days_since_last_trade"), 0.0, 0)
}

// visiblePoint 的 available_at 守門（features.py:333 的
// `point.available_at > cutoff`）：即使 map 裡有這個日期的點，只要它的
// available_at 晚於 cutoff 就必須被當成不存在。revision 之間的取捨
// （latest_points_as_of，features.py:271-290）不在 P3 範圍內，見本 Task
// 開頭的對應表。
func TestBuildFeatureVectorFromDailyPoints_LateAvailabilityExcluded(t *testing.T) {
	config := DefaultFeatureConfig()
	origin := time.Date(2025, 1, 10, 23, 0, 0, 0, time.UTC)

	visible := indexByDate(mustPoint(t, 7, price(17.0), 7, false))
	fvVisible, err := BuildFeatureVectorFromDailyPoints(visible, origin, 1, config, "UTC")
	if err != nil {
		t.Fatalf("BuildFeatureVectorFromDailyPoints 失敗: %v", err)
	}

	lateOnly := indexByDate(mustPoint(t, 7, price(17.0), 12, false)) // available_day=12 > cutoff(day10)
	fvLate, err := BuildFeatureVectorFromDailyPoints(lateOnly, origin, 1, config, "UTC")
	if err != nil {
		t.Fatalf("BuildFeatureVectorFromDailyPoints 失敗: %v", err)
	}

	visibleLag3, _ := fvVisible.Value("price_lag_3")
	lateLag3, _ := fvLate.Value("price_lag_3")
	lateLag3Missing, _ := fvLate.Value("price_lag_3_missing")
	assertClose(t, "可見時 price_lag_3", visibleLag3, 17.0, 0)
	assertClose(t, "遲到修訂 price_lag_3", lateLag3, 0.0, 0)
	assertClose(t, "遲到修訂 price_lag_3_missing", lateLag3Missing, 1.0, 0)
}

// 對應 test_taipei_midnight_features_use_taipei_trade_date
// （tests/test_features.py:104-134）：origin 在 Taipei 凌晨 00:30，必須用
// Taipei 當地日曆日期算特徵，不是 UTC 日期——跨日邊界最容易錯的地方。
func TestBuildFeatureVectorFromDailyPoints_TaipeiMidnightUsesLocalTradeDate(t *testing.T) {
	config := FeatureConfig{PriceLags: []int{1}, VolumeLags: []int{1}, RollingWindows: []int{2}, MaxDaysSinceLastTrade: 365}
	taipei, err := time.LoadLocation("Asia/Taipei")
	if err != nil {
		t.Fatal(err)
	}
	points := map[Date]PricePoint{}
	for _, day := range []int{19, 20, 21} {
		v := 100.0
		p, err := NewPricePoint(PricePointInput{
			SeriesID: "moa:104:LA1", MarketID: "moa:104", CropID: "moa:LA1",
			TradeDate: NewDate(2025, 3, day), AveragePrice: price(float64(day)), Volume: &v,
			AvailableAt: time.Date(2025, 3, day, 0, 15, 0, 0, taipei),
		})
		if err != nil {
			t.Fatal(err)
		}
		points[p.TradeDate] = p
	}
	origin := time.Date(2025, 3, 21, 0, 30, 0, 0, taipei)

	fv, err := BuildFeatureVectorFromDailyPoints(points, origin, 1, config, "Asia/Taipei")
	if err != nil {
		t.Fatalf("BuildFeatureVectorFromDailyPoints 回錯誤: %v", err)
	}
	lag1, _ := fv.Value("price_lag_1")
	lastPrice, _ := fv.Value("last_price")
	assertClose(t, "price_lag_1", lag1, 20.0, 0)
	assertClose(t, "last_price", lastPrice, 21.0, 0)
}

// 對應 test_latest_inactive_revision_removes_price_from_features
// （tests/test_features.py:137-164）：resolved 之後的點若 is_active=false
// （tombstone），該天的價格必須視為缺失。這裡直接把 tombstone 放進 map
// （模擬「revision 解完之後留下的就是這筆」），驗證 from_daily_points 自己
// 對 is_active 的處理（_valid_price，features.py:293-301）。
func TestBuildFeatureVectorFromDailyPoints_InactiveRevisionMasksPrice(t *testing.T) {
	config := FeatureConfig{PriceLags: []int{1}, VolumeLags: []int{1}, RollingWindows: []int{2}, MaxDaysSinceLastTrade: 365}
	inactive := false
	v := 119.0
	tombstone, err := NewPricePoint(PricePointInput{
		SeriesID: "moa:104:LA1", MarketID: "moa:104", CropID: "moa:LA1",
		TradeDate: NewDate(2025, 1, 9), AveragePrice: price(19.0), Volume: &v,
		AvailableAt: time.Date(2025, 1, 10, 20, 0, 0, 0, time.UTC),
		IsActive:    &inactive,
	})
	if err != nil {
		t.Fatal(err)
	}
	points := indexByDate(tombstone)
	origin := time.Date(2025, 1, 10, 23, 0, 0, 0, time.UTC)

	fv, err := BuildFeatureVectorFromDailyPoints(points, origin, 1, config, "UTC")
	if err != nil {
		t.Fatalf("BuildFeatureVectorFromDailyPoints 回錯誤: %v", err)
	}
	lag1Missing, _ := fv.Value("price_lag_1_missing")
	lastPriceMissing, _ := fv.Value("last_price_missing")
	assertClose(t, "price_lag_1_missing", lag1Missing, 1.0, 0)
	assertClose(t, "last_price_missing", lastPriceMissing, 1.0, 0)
}

// 對應 test_reconstructed_daily_index_matches_revision_path_before_end_of_day
// 的數值部分（tests/test_features.py:167-222，10 天、origin 在最後一天的
// 23:45、observations 在各自日終時刻 23:59:59.999999 發布、
// availability_quality=reconstructed）：只搬「日終時刻的 last_price /
// days_since_last_trade / seasonal_naive」這幾個數值斷言，不搬
// build_feature_vector 與 build_feature_vector_from_daily_points 的等價性
// 比較（那需要 latest_points_as_of，P3 不移植）。
func TestBuildFeatureVectorFromDailyPoints_EndOfDayLastPriceAndSeasonalNaive(t *testing.T) {
	taipei, err := time.LoadLocation("Asia/Taipei")
	if err != nil {
		t.Fatal(err)
	}
	points := map[Date]PricePoint{}
	for day := 1; day <= 10; day++ {
		v := 100.0 + float64(day)
		endOfDay := time.Date(2025, 1, day, 23, 59, 59, 999999000, taipei)
		p, err := NewPricePoint(PricePointInput{
			SeriesID: "moa:104:LA1", MarketID: "moa:104", CropID: "moa:LA1",
			TradeDate: NewDate(2025, 1, day), AveragePrice: price(float64(day)), Volume: &v,
			AvailableAt:         endOfDay,
			AvailabilityQuality: "reconstructed",
		})
		if err != nil {
			t.Fatal(err)
		}
		points[p.TradeDate] = p
	}
	origin := time.Date(2025, 1, 10, 23, 45, 0, 0, taipei)
	config := FeatureConfig{PriceLags: []int{1, 7}, VolumeLags: []int{1}, RollingWindows: []int{7}, MaxDaysSinceLastTrade: 365}

	fv, err := BuildFeatureVectorFromDailyPoints(points, origin, 7, config, "Asia/Taipei")
	if err != nil {
		t.Fatalf("BuildFeatureVectorFromDailyPoints 回錯誤: %v", err)
	}
	lastPrice, _ := fv.Value("last_price")
	daysSince, _ := fv.Value("days_since_last_trade")
	assertClose(t, "last_price", lastPrice, 9.0, 0)
	assertClose(t, "days_since_last_trade", daysSince, 1.0, 0)

	fallback := 0.0
	naive, err := SeasonalNaiveFromDailyPoints(points, origin, NewDate(2025, 1, 17), &fallback, 7, 365, "Asia/Taipei")
	if err != nil {
		t.Fatalf("SeasonalNaiveFromDailyPoints 回錯誤: %v", err)
	}
	if naive == nil {
		t.Fatal("預期非 nil")
	}
	assertClose(t, "seasonal_naive", *naive, 9.0, 0)

	// MaxFeatureAvailableAt 是所有「visible_point」(不只 _valid_price 通過的
	// 那些) 裡 available_at 的最大值，掃完整個 max_lookback+1 天、不提前
	// break（features.py:383-398）。Jan10 的點 available_at 晚於
	// cutoff(Jan10 23:45 Taipei) 所以被排除，watermark 停在 Jan9 23:59:59.999999
	// Taipei，換算 UTC 是 Jan9 15:59:59.999999。
	if fv.MaxFeatureAvailableAt == nil {
		t.Fatal("預期 MaxFeatureAvailableAt 非 nil")
	}
	wantWatermark := time.Date(2025, 1, 9, 15, 59, 59, 999999000, time.UTC)
	if !fv.MaxFeatureAvailableAt.Equal(wantWatermark) {
		t.Errorf("MaxFeatureAvailableAt = %v，預期 %v", *fv.MaxFeatureAvailableAt, wantWatermark)
	}
}

// validPrice/validVolume 的 p.IsMarketClosed 守門：全部既有測試的 mustPoint
// 呼叫都傳 closed=false，這條分支從沒被任何測試真的執行過。休市當天即使有
// AveragePrice 也必須被當成缺失（跟 TestBuildFeatureVectorFromDailyPoints_
// InactiveRevisionMasksPrice 同一種「該天有點但這天的價格不算數」形狀，
// 差在守門條件是 IsMarketClosed 而不是 IsActive）。
func TestBuildFeatureVectorFromDailyPoints_MarketClosedMasksPrice(t *testing.T) {
	config := FeatureConfig{PriceLags: []int{1}, VolumeLags: []int{1}, RollingWindows: []int{2}, MaxDaysSinceLastTrade: 365}
	points := indexByDate(mustPoint(t, 9, price(19.0), 9, true)) // closed=true
	origin := time.Date(2025, 1, 10, 23, 0, 0, 0, time.UTC)

	fv, err := BuildFeatureVectorFromDailyPoints(points, origin, 1, config, "UTC")
	if err != nil {
		t.Fatalf("BuildFeatureVectorFromDailyPoints 回錯誤: %v", err)
	}
	get := func(name string) float64 {
		v, ok := fv.Value(name)
		if !ok {
			t.Fatalf("找不到特徵 %s", name)
		}
		return v
	}
	assertClose(t, "price_lag_1", get("price_lag_1"), 0.0, 0)
	assertClose(t, "price_lag_1_missing", get("price_lag_1_missing"), 1.0, 0)
}

// validPrice/validVolume 的 nil AveragePrice/Volume 守門：pricepoint.go 明文
// 說 *float64 存在就是為了區分「沒有成交紀錄」(nil) 與「成交價為零」(非 nil
// 0.0)，但全部既有測試傳的價格都非 nil，這個設計決策從沒被驗證過。
// mustPoint 在 priceVal 為 nil 時會自動把 volume 也設成 nil（見該 helper
// 本體），所以這裡同時驗到 price 與 volume 兩條 missing 分支。
func TestBuildFeatureVectorFromDailyPoints_NilPriceAndVolumeAreMissing(t *testing.T) {
	config := FeatureConfig{PriceLags: []int{1}, VolumeLags: []int{1}, RollingWindows: []int{2}, MaxDaysSinceLastTrade: 365}
	points := indexByDate(mustPoint(t, 9, nil, 9, false)) // nil price → mustPoint 也把 volume 設成 nil
	origin := time.Date(2025, 1, 10, 23, 0, 0, 0, time.UTC)

	fv, err := BuildFeatureVectorFromDailyPoints(points, origin, 1, config, "UTC")
	if err != nil {
		t.Fatalf("BuildFeatureVectorFromDailyPoints 回錯誤: %v", err)
	}
	get := func(name string) float64 {
		v, ok := fv.Value(name)
		if !ok {
			t.Fatalf("找不到特徵 %s", name)
		}
		return v
	}
	assertClose(t, "price_lag_1_missing", get("price_lag_1_missing"), 1.0, 0)
	assertClose(t, "volume_lag_1_missing", get("volume_lag_1_missing"), 1.0, 0)
}

// FeatureConfig.Validate() 的錯誤必須真的擋下 BuildFeatureVectorFromDailyPoints
// ——RollingWindows 含 0 會讓 observed_ratio 變成 0.0/0.0=NaN、PriceLags 含 0
// 會讓 AddDays(-0)=origin_date 本身，直接破壞這個 task 的 offset 契約，兩者
// 都不該無聲通過。錯誤訊息內容已在 Task 2 的 featureconfig_test.go 測過，
// 這裡只驗證有擋下來。
func TestBuildFeatureVectorFromDailyPoints_RejectsInvalidFeatureConfig(t *testing.T) {
	config := FeatureConfig{PriceLags: []int{1}, VolumeLags: []int{1}, RollingWindows: []int{0}, MaxDaysSinceLastTrade: 365}
	origin := time.Date(2025, 1, 10, 23, 0, 0, 0, time.UTC)

	_, err := BuildFeatureVectorFromDailyPoints(map[Date]PricePoint{}, origin, 1, config, "UTC")
	if err == nil {
		t.Fatal("預期 FeatureConfig 驗證失敗時回傳 error，實際為 nil")
	}
}
