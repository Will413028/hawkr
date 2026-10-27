package forecast

import (
	"math"
	"strconv"
	"strings"
	"testing"
	"time"
)

// testArtifact 組一個最小可用的 artifact：1 條 series × 7 horizons，
// ridge 只有 1 維（測分支選擇不測數學，數學由 P3 的 ridge_test 與
// parity test 負責）。
func testArtifact(t *testing.T, kind string, mutate func(*HorizonModel)) ModelArtifact {
	t.Helper()
	cfg := FeatureConfig{PriceLags: []int{1}, VolumeLags: []int{1}, RollingWindows: []int{7}, MaxDaysSinceLastTrade: 365}
	horizons := map[string]HorizonModel{}
	for h := 1; h <= 7; h++ {
		var hm HorizonModel
		if kind == ModelKindDirectRidge {
			hm = validRidgeHorizon()
			hm.Ridge = &RidgeParameters{
				Intercept:    50,
				Coefficients: make([]float64, len(cfg.FeatureNames())),
				FeatureMean:  make([]float64, len(cfg.FeatureNames())),
				FeatureScale: ones(len(cfg.FeatureNames())),
			}
		} else {
			hm = validSeasonalHorizon()
		}
		if mutate != nil {
			mutate(&hm)
		}
		horizons[strconv.Itoa(h)] = hm
	}
	cat := "N04"
	return ModelArtifact{
		ArtifactSchemaVersion: ArtifactSchemaVersion,
		ModelVersion:          "test-model-v1",
		DatasetDigest:         strings.Repeat("a", 64),
		TrainedAt:             time.Date(2026, 7, 18, 0, 0, 0, 0, time.UTC),
		FeatureSchema:         cfg.FeatureNames(),
		Config: TrainingConfig{
			HorizonDays: 7, RidgeAlpha: 10, MinSamplesPerModel: 30,
			CalibrationFraction: 0.2, MinCalibrationSamples: 7, SeasonalLagDays: 7,
			AvailabilityPolicy: "latest_corrected_backfill", FeatureConfig: cfg,
		},
		Series: map[string]SeriesModel{
			"moa:109:LA1": {
				SeriesID: "moa:109:LA1", MarketID: "moa:109", CropID: "moa:LA1",
				CategoryCode: &cat, Horizons: horizons,
			},
		},
		Metadata: map[string]any{"calendar_timezone": "Asia/Taipei"},
	}
}

func ones(n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = 1
	}
	return out
}

// testHistory 產出 origin 前 60 天、每天都有成交的歷史。
func testHistory(originDate Date, price, volume float64, availableAt time.Time) map[Date]PricePoint {
	out := map[Date]PricePoint{}
	for offset := 0; offset < 60; offset++ {
		d := originDate.AddDays(-offset)
		p := price
		v := volume
		active := true
		pt, _ := NewPricePoint(PricePointInput{
			SeriesID: "moa:109:LA1", MarketID: "moa:109", CropID: "moa:LA1",
			TradeDate: d, AveragePrice: &p, Volume: &v, AvailableAt: availableAt,
			CategoryCode: func() *string { c := "N04"; return &c }(), IsActive: &active,
		})
		out[d] = pt
	}
	return out
}

func baseInput(t *testing.T, kind string, mutate func(*HorizonModel)) SevenDaysInput {
	t.Helper()
	origin := time.Date(2026, 7, 18, 15, 45, 0, 0, time.UTC)
	originDate := NewDate(2026, 7, 18)
	return SevenDaysInput{
		Artifact:        testArtifact(t, kind, mutate),
		HistoryBySeries: map[string]map[Date]PricePoint{"moa:109:LA1": testHistory(originDate, 50, 1000, origin.Add(-time.Hour))},
		Origin:          origin,
		Closures:        NewClosures(),
		Verified:        NewVerifiedCalendar([]VerifiedCalendarKey{{MarketID: "moa:109", CategoryCode: "N04", Year: 2026}}),
	}
}

func TestSevenDaysAlwaysReturnsSevenRowsPerSeries(t *testing.T) {
	got, err := SevenDays(baseInput(t, ModelKindDirectRidge, nil))
	if err != nil {
		t.Fatalf("SevenDays: %v", err)
	}
	if len(got) != 7 {
		t.Fatalf("列數 = %d，預期 7（每條 series 固定 7 列，任何分支都不得少列——"+
			"少列會讓前端的 D+1~D+7 表格出現無法解釋的空格）", len(got))
	}
	for i, p := range got {
		if p.HorizonDays != i+1 {
			t.Errorf("第 %d 列的 horizon = %d，預期 %d（順序必須是 1..7）", i, p.HorizonDays, i+1)
		}
		if p.TargetSemantics != TargetSemanticsPriceGivenTrade {
			t.Errorf("target_semantics = %q，預期 %q（每列都必須帶，這是誠實標示的一部分）", p.TargetSemantics, TargetSemanticsPriceGivenTrade)
		}
	}
}

func TestSevenDaysBranchClosedTakesPrecedenceOverCalibrationGate(t *testing.T) {
	// 上游的分支順序是「先判休市、再判 calibration」（forecasting.py:250 在 :276 之前）。
	// 反過來的話，休市日遇上未校準的 horizon 會回報 not_predicted 而不是 market_closed，
	// UI 就會把「今天沒開市」說成「資料不足」。
	in := baseInput(t, ModelKindDirectRidge, func(m *HorizonModel) { m.CalibrationCount = 1 })
	in.Closures.AddMarketCategory("moa:109", "N04", NewDate(2026, 7, 21))

	got, err := SevenDays(in)
	if err != nil {
		t.Fatalf("SevenDays: %v", err)
	}
	closed := got[2] // D+3 = 2026-07-21
	if closed.TargetDate != NewDate(2026, 7, 21) {
		t.Fatalf("D+3 的 target_date = %s，預期 2026-07-21", closed.TargetDate)
	}
	if closed.PointStatus != PointStatusNotApplicable || closed.MissingReason != MissingReasonMarketClosed {
		t.Errorf("休市點 = (%q, %q)，預期 (%q, %q)", closed.PointStatus, closed.MissingReason, PointStatusNotApplicable, MissingReasonMarketClosed)
	}
	if closed.CalendarStatus != CalendarStatusVerifiedClosed || closed.MarketStatus != MarketStatusClosed {
		t.Errorf("休市點的 calendar/market status = (%q, %q)，預期 (%q, %q)", closed.CalendarStatus, closed.MarketStatus, CalendarStatusVerifiedClosed, MarketStatusClosed)
	}
	if closed.P50 != nil || closed.MaxFeatureAvailableAt != nil {
		t.Error("休市短路發生在建特徵之前，不該有價格也不該有 max_feature_available_at")
	}
	// 非休市的日子仍然要走 calibration gate
	if got[0].PointStatus != PointStatusNotPredicted || got[0].MissingReason != MissingReasonInsufficientCalibrationData {
		t.Errorf("D+1 = (%q, %q)，預期 calibration gate 擋下", got[0].PointStatus, got[0].MissingReason)
	}
}

func TestSevenDaysBranchNoRecentData(t *testing.T) {
	in := baseInput(t, ModelKindDirectRidge, nil)
	in.HistoryBySeries["moa:109:LA1"] = map[Date]PricePoint{} // 完全沒有歷史

	got, err := SevenDays(in)
	if err != nil {
		t.Fatalf("SevenDays: %v", err)
	}
	for _, p := range got {
		if p.PointStatus != PointStatusNotPredicted || p.MissingReason != MissingReasonInsufficientRecentData {
			t.Errorf("D+%d = (%q, %q)，預期 (%q, %q)", p.HorizonDays, p.PointStatus, p.MissingReason, PointStatusNotPredicted, MissingReasonInsufficientRecentData)
		}
		if p.ModelKind != ModelKindNoRecentData {
			t.Errorf("D+%d 的 model_kind = %q，預期 %q（forecasting.py:333 在這條分支覆寫 model_kind）", p.HorizonDays, p.ModelKind, ModelKindNoRecentData)
		}
		if p.P50 != nil {
			t.Errorf("D+%d 不該有價格", p.HorizonDays)
		}
	}
}

func TestSevenDaysBranchPredictedRidgeAndSeasonal(t *testing.T) {
	for _, tc := range []struct{ kind, wantModelKind string }{
		{ModelKindDirectRidge, ModelKindDirectRidge},
		{ModelKindSeasonalNaive, ModelKindSeasonalNaive},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			got, err := SevenDays(baseInput(t, tc.kind, nil))
			if err != nil {
				t.Fatalf("SevenDays: %v", err)
			}
			for _, p := range got {
				if p.PointStatus != PointStatusPredicted {
					t.Fatalf("D+%d = %q，預期 %q", p.HorizonDays, p.PointStatus, PointStatusPredicted)
				}
				if p.MissingReason != "" {
					t.Errorf("D+%d 有價格卻帶 missing_reason %q", p.HorizonDays, p.MissingReason)
				}
				if p.ModelKind != tc.wantModelKind {
					t.Errorf("D+%d model_kind = %q，預期 %q", p.HorizonDays, p.ModelKind, tc.wantModelKind)
				}
				if p.P10 == nil || p.P50 == nil || p.P90 == nil {
					t.Fatalf("D+%d 缺分位數", p.HorizonDays)
				}
				if !(*p.P10 <= *p.P50 && *p.P50 <= *p.P90) {
					t.Errorf("D+%d 分位數非單調: %v %v %v", p.HorizonDays, *p.P10, *p.P50, *p.P90)
				}
			}
		})
	}
}

func TestSevenDaysUnverifiedCalendarStillPredictsButFlags(t *testing.T) {
	// 上游對未驗證日曆的處理是「照樣出價，但把 market_status 標成 unknown、
	// missing_reason 標成 calendar_unverified」（forecasting.py:371-374）。
	// 設計書 §8 line 421 要求最終對外回 insufficient_data —— 那是外層映射
	// （Task 8）的事，內層必須保留上游語意，否則 parity 失去基準。
	in := baseInput(t, ModelKindDirectRidge, nil)
	in.Verified = NewVerifiedCalendar(nil) // 啟用驗證但沒有任何覆蓋

	got, err := SevenDays(in)
	if err != nil {
		t.Fatalf("SevenDays: %v", err)
	}
	for _, p := range got {
		if p.PointStatus != PointStatusPredicted {
			t.Errorf("D+%d = %q，預期內層仍然出價", p.HorizonDays, p.PointStatus)
		}
		if p.CalendarStatus != CalendarStatusUnverified || p.MarketStatus != MarketStatusUnknown {
			t.Errorf("D+%d = (%q, %q)，預期 (%q, %q)", p.HorizonDays, p.CalendarStatus, p.MarketStatus, CalendarStatusUnverified, MarketStatusUnknown)
		}
		if p.MissingReason != MissingReasonCalendarUnverified {
			t.Errorf("D+%d missing_reason = %q，預期 %q", p.HorizonDays, p.MissingReason, MissingReasonCalendarUnverified)
		}
	}
}

func TestSevenDaysRejectsOriginBeforeTrainedAt(t *testing.T) {
	// forecasting.py:180-181。origin 早於 trained_at 意味著用未來的模型預測過去，
	// 是 backtest 才有的操作，線上路徑走到這裡代表時鐘或參數錯了。
	in := baseInput(t, ModelKindDirectRidge, nil)
	in.Origin = time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	if _, err := SevenDays(in); err == nil {
		t.Fatal("origin 早於 trained_at 必須回錯誤")
	}
}

func TestSevenDaysMissingHorizonIsAnError(t *testing.T) {
	in := baseInput(t, ModelKindDirectRidge, nil)
	series := in.Artifact.Series["moa:109:LA1"]
	delete(series.Horizons, "4")
	in.Artifact.Series["moa:109:LA1"] = series
	if _, err := SevenDays(in); err == nil {
		t.Fatal("缺 horizon 必須回錯誤而不是少一列——上游靠 SeriesModel.__post_init__ " +
			"（training.py:258-260）保證 1..7 齊全，Go 端沒有那個建構期檢查，只能在這裡擋")
	}
}

// multiSeriesArtifactForOrderingTest 建一個 6 條 series 的 artifact，series_id
// 刻意用非字典序插入（Go map 迭代順序是隨機的——如果 SevenDays 忘記排序改用裸
// range，這個測試才有機會抓到）。所有 horizon 的 CalibrationCount 故意設 0
// （< MinCalibrationSamples=7），讓每個點都在 calibration gate（分支 B）短路，
// 不需要建歷史資料——這個測試只關心 SeriesID/HorizonDays 的排序契約，不關心
// 有沒有出價。
//
// 用 6 條而非 3 條：n 條 series 隨機序恰好等於排序結果的機率是 1/n!——n=3 時
// 約 16.7%，實測 -count=200 曾出現 12% false-PASS（把 slices.Sorted 換成裸
// slices.Collect 抓不到）。n=6 時 1/6! ≈ 0.14%，才足以在 CI 單次跑穩定抓到
// 排序契約被破壞的退化。
func multiSeriesArtifactForOrderingTest(t *testing.T) ModelArtifact {
	t.Helper()
	cfg := FeatureConfig{PriceLags: []int{1}, VolumeLags: []int{1}, RollingWindows: []int{7}, MaxDaysSinceLastTrade: 365}
	// 刻意非字典序插入（對排序結果 A<B<C<D<E<F 做完全錯位排列，六個位置無一
	// 落在自己的排序位置上，避免湊巧與 map 迭代順序一致而漏測）。
	insertOrder := []string{
		"moa:110:AA", "moa:109:AA", "moa:104:ZZ",
		"moa:109:ZZ", "moa:104:AA", "moa:107:BB",
	}
	series := map[string]SeriesModel{}
	cat := "N04"
	for _, sid := range insertOrder {
		horizons := map[string]HorizonModel{}
		for h := 1; h <= 7; h++ {
			hm := validRidgeHorizon()
			hm.CalibrationCount = 0
			horizons[strconv.Itoa(h)] = hm
		}
		series[sid] = SeriesModel{SeriesID: sid, MarketID: "moa:109", CropID: "moa:X", CategoryCode: &cat, Horizons: horizons}
	}
	return ModelArtifact{
		ArtifactSchemaVersion: ArtifactSchemaVersion,
		ModelVersion:          "test-model-v1",
		DatasetDigest:         strings.Repeat("a", 64),
		TrainedAt:             time.Date(2026, 7, 18, 0, 0, 0, 0, time.UTC),
		FeatureSchema:         cfg.FeatureNames(),
		Config: TrainingConfig{
			HorizonDays: 7, RidgeAlpha: 10, MinSamplesPerModel: 30,
			CalibrationFraction: 0.2, MinCalibrationSamples: 7, SeasonalLagDays: 7,
			AvailabilityPolicy: "latest_corrected_backfill", FeatureConfig: cfg,
		},
		Series:   series,
		Metadata: map[string]any{"calendar_timezone": "Asia/Taipei"},
	}
}

// Finding 1（review）：sevendays.go 的 doc comment 承諾「順序是 (series_id 升冪,
// horizon 1..7)」，但兩支 parity test 都先塞進 map 再比對，順序完全不可見。這裡
// 直接斷言回傳序列本身，逐列比對 SeriesID 與 HorizonDays，抓的是「有人把
// slices.Sorted(maps.Keys(...)) 改成裸 range 或 slices.Collect」這種退化。
func TestSevenDaysOrdersBySeriesIDThenHorizon(t *testing.T) {
	in := SevenDaysInput{
		Artifact:        multiSeriesArtifactForOrderingTest(t),
		HistoryBySeries: map[string]map[Date]PricePoint{},
		Origin:          time.Date(2026, 7, 18, 15, 45, 0, 0, time.UTC),
		Closures:        NewClosures(),
		Verified:        NewVerifiedCalendar(nil),
	}
	got, err := SevenDays(in)
	if err != nil {
		t.Fatalf("SevenDays: %v", err)
	}
	wantSeriesOrder := []string{
		"moa:104:AA", "moa:104:ZZ", "moa:107:BB",
		"moa:109:AA", "moa:109:ZZ", "moa:110:AA",
	} // 字典序
	if len(got) != len(wantSeriesOrder)*7 {
		t.Fatalf("列數 = %d，預期 %d", len(got), len(wantSeriesOrder)*7)
	}
	for i, wantSeries := range wantSeriesOrder {
		for h := 0; h < 7; h++ {
			p := got[i*7+h]
			if p.SeriesID != wantSeries {
				t.Errorf("第 %d 列 SeriesID = %q，預期 %q（series 必須按 series_id 升冪排序，"+
					"不能依賴 map 迭代順序——Go map 迭代順序是隨機的）", i*7+h, p.SeriesID, wantSeries)
			}
			if p.HorizonDays != h+1 {
				t.Errorf("第 %d 列（series=%s）HorizonDays = %d，預期 %d", i*7+h, p.SeriesID, p.HorizonDays, h+1)
			}
		}
	}
}

// Finding 2（review）：非有限價格降級成 missing_reason 而非中止全批，是計畫三個
// 刻意偏離上游的設計決定之一（上游 _ordered_non_negative 對非有限值直接 raise，
// forecasting.py:114-115，會讓一條壞 series 殺掉整批數千條）——但這條分支存在
// 的唯一理由（壞 series 不能拖垮全站）在改動前完全沒有測試覆蓋。這裡刻意讓一條
// series 的某個 horizon 產出非有限 residual quantile，斷言：不回 error、不減
// 列、壞點降級成 non_finite_price、而且同一批裡另一條健康的 series 仍正常出價。
func TestSevenDaysNonFinitePriceDowngradesToMissingReasonWithoutAbortingBatch(t *testing.T) {
	cfg := FeatureConfig{PriceLags: []int{1}, VolumeLags: []int{1}, RollingWindows: []int{7}, MaxDaysSinceLastTrade: 365}
	brokenHorizons := map[string]HorizonModel{}
	healthyHorizons := map[string]HorizonModel{}
	for h := 1; h <= 7; h++ {
		hm := validRidgeHorizon()
		hm.Ridge = &RidgeParameters{
			Intercept:    50,
			Coefficients: make([]float64, len(cfg.FeatureNames())),
			FeatureMean:  make([]float64, len(cfg.FeatureNames())),
			FeatureScale: ones(len(cfg.FeatureNames())),
		}
		healthyHorizons[strconv.Itoa(h)] = hm
		broken := hm
		if h == 3 {
			broken.ResidualQuantiles = [3]float64{math.Inf(1), 0, 0}
		}
		brokenHorizons[strconv.Itoa(h)] = broken
	}
	cat := "N04"
	artifact := ModelArtifact{
		ArtifactSchemaVersion: ArtifactSchemaVersion,
		ModelVersion:          "test-model-v1",
		DatasetDigest:         strings.Repeat("a", 64),
		TrainedAt:             time.Date(2026, 7, 18, 0, 0, 0, 0, time.UTC),
		FeatureSchema:         cfg.FeatureNames(),
		Config: TrainingConfig{
			HorizonDays: 7, RidgeAlpha: 10, MinSamplesPerModel: 30,
			CalibrationFraction: 0.2, MinCalibrationSamples: 7, SeasonalLagDays: 7,
			AvailabilityPolicy: "latest_corrected_backfill", FeatureConfig: cfg,
		},
		Series: map[string]SeriesModel{
			"moa:109:BROKEN": {SeriesID: "moa:109:BROKEN", MarketID: "moa:109", CropID: "moa:BROKEN", CategoryCode: &cat, Horizons: brokenHorizons},
			"moa:109:LA1":    {SeriesID: "moa:109:LA1", MarketID: "moa:109", CropID: "moa:LA1", CategoryCode: &cat, Horizons: healthyHorizons},
		},
		Metadata: map[string]any{"calendar_timezone": "Asia/Taipei"},
	}

	origin := time.Date(2026, 7, 18, 15, 45, 0, 0, time.UTC)
	originDate := NewDate(2026, 7, 18)
	in := SevenDaysInput{
		Artifact: artifact,
		HistoryBySeries: map[string]map[Date]PricePoint{
			"moa:109:BROKEN": testHistory(originDate, 50, 1000, origin.Add(-time.Hour)),
			"moa:109:LA1":    testHistory(originDate, 50, 1000, origin.Add(-time.Hour)),
		},
		Origin:   origin,
		Closures: NewClosures(),
		Verified: NewVerifiedCalendar([]VerifiedCalendarKey{{MarketID: "moa:109", CategoryCode: "N04", Year: 2026}}),
	}

	got, err := SevenDays(in)
	if err != nil {
		t.Fatalf("SevenDays 不該回 error——非有限價格必須降級成該點的 missing_reason，"+
			"不能中止全批（forecasting.py:114-115 上游會 raise，我方刻意不移植這個行為）: %v", err)
	}
	if len(got) != 14 {
		t.Fatalf("列數 = %d，預期 14（2 series × 7 horizon，壞點不減列）", len(got))
	}

	var brokenD3, healthyD3 *SeriesPoint
	for i := range got {
		switch {
		case got[i].SeriesID == "moa:109:BROKEN" && got[i].HorizonDays == 3:
			brokenD3 = &got[i]
		case got[i].SeriesID == "moa:109:LA1" && got[i].HorizonDays == 3:
			healthyD3 = &got[i]
		}
	}
	if brokenD3 == nil {
		t.Fatal("找不到 moa:109:BROKEN D+3")
	}
	if healthyD3 == nil {
		t.Fatal("找不到 moa:109:LA1 D+3")
	}
	if brokenD3.PointStatus != PointStatusNotPredicted || brokenD3.MissingReason != MissingReasonNonFinitePrice {
		t.Errorf("壞點 = (%q, %q)，預期 (%q, %q)", brokenD3.PointStatus, brokenD3.MissingReason, PointStatusNotPredicted, MissingReasonNonFinitePrice)
	}
	if brokenD3.P10 != nil || brokenD3.P50 != nil || brokenD3.P90 != nil {
		t.Error("壞點不該有價格")
	}
	if healthyD3.PointStatus != PointStatusPredicted {
		t.Errorf("同一 artifact 裡另一條正常 series 必須仍然正常出價（壞 series 不能殺全批），得到 %q", healthyD3.PointStatus)
	}
	if healthyD3.P50 == nil {
		t.Error("正常 series 缺價格")
	}
}

// Finding 3（review）：calibration gate 分支（B）的 calendar_status/market_status
// 是依 calendar_verified 算出來的（forecasting.py:291-292），不是寫死
// verified_open/open——但既有測試走到分支 B 時 Verified 剛好都是已驗證狀態，
// 沒有任何測試在「gate 擋下 + 日曆未驗證」的交集下斷言這兩個欄位。
func TestSevenDaysCalibrationGateBranchReflectsUnverifiedCalendarStatus(t *testing.T) {
	in := baseInput(t, ModelKindDirectRidge, func(m *HorizonModel) { m.CalibrationCount = 1 })
	in.Verified = NewVerifiedCalendar(nil) // 啟用驗證但零覆蓋 → 全部視為未驗證

	got, err := SevenDays(in)
	if err != nil {
		t.Fatalf("SevenDays: %v", err)
	}
	p := got[0] // D+1，calibration gate 擋下
	if p.PointStatus != PointStatusNotPredicted || p.MissingReason != MissingReasonInsufficientCalibrationData {
		t.Fatalf("D+1 = (%q, %q)，預期 calibration gate 擋下 (%q, %q)", p.PointStatus, p.MissingReason, PointStatusNotPredicted, MissingReasonInsufficientCalibrationData)
	}
	if p.CalendarStatus != CalendarStatusUnverified || p.MarketStatus != MarketStatusUnknown {
		t.Errorf("calibration gate 分支的 calendar/market status = (%q, %q)，預期 (%q, %q)"+
			"（這兩個欄位是依 calendar_verified 算出來的，不是寫死 verified_open/open）",
			p.CalendarStatus, p.MarketStatus, CalendarStatusUnverified, MarketStatusUnknown)
	}
}

func TestSevenDaysClosedDayStillCarriesConditionalPrice(t *testing.T) {
	in := baseInput(t, ModelKindDirectRidge, nil)
	in.Closures.AddMarketCategory("moa:109", "N04", NewDate(2026, 7, 21))

	got, err := SevenDays(in)
	if err != nil {
		t.Fatalf("SevenDays: %v", err)
	}
	closed := got[2] // D+3 = 2026-07-21
	if closed.TargetDate != NewDate(2026, 7, 21) {
		t.Fatalf("D+3 的 target_date = %s，預期 2026-07-21", closed.TargetDate)
	}
	if closed.PointStatus != PointStatusNotApplicable || closed.MissingReason != MissingReasonMarketClosed {
		t.Errorf("休市點 = (%q, %q)，預期 (%q, %q)", closed.PointStatus, closed.MissingReason, PointStatusNotApplicable, MissingReasonMarketClosed)
	}
	if closed.CalendarStatus != CalendarStatusVerifiedClosed || closed.MarketStatus != MarketStatusClosed {
		t.Errorf("休市點的 calendar/market status = (%q, %q)，預期 (%q, %q)", closed.CalendarStatus, closed.MarketStatus, CalendarStatusVerifiedClosed, MarketStatusClosed)
	}
	if closed.P10 == nil || closed.P50 == nil || closed.P90 == nil {
		t.Fatalf("休市點缺價格：p10=%v p50=%v p90=%v", closed.P10, closed.P50, closed.P90)
	}
	if !(*closed.P10 <= *closed.P50 && *closed.P50 <= *closed.P90) {
		t.Errorf("分位數未遞增：%v / %v / %v", *closed.P10, *closed.P50, *closed.P90)
	}
	if closed.MaxFeatureAvailableAt == nil {
		t.Error("有建特徵就該帶 max_feature_available_at")
	}
	open := got[0]
	if open.PointStatus != PointStatusPredicted || open.P50 == nil {
		t.Errorf("D+1（開市）= (%q, p50=%v)，預期照常出價", open.PointStatus, open.P50)
	}
}
