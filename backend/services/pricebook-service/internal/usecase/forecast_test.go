package usecase

import (
	"context"
	"math"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/Will413028/hawkr/pricebook-service/internal/config"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/forecast"
)

// ==================== fake repo（比照 review-service/internal/grpc/handler_test.go 的
// struct-of-func-fields 慣例，不引入 gomock） ====================

type fakeForecastRepo struct {
	seriesObservationsFn func(ctx context.Context, marketCode, cropCode string, from, to, cutoff time.Time) ([]domain.Observation, error)
	groupObservationsFn  func(ctx context.Context, groupCode, marketCode string, from, to, cutoff time.Time) ([]domain.Observation, error)
	restDaysFn           func(ctx context.Context, marketCode string, from, to time.Time) ([]domain.RestDay, error)
	restDayCoverageFn    func(ctx context.Context, marketCode string) ([]domain.RestDayCoverage, error)
	cropsInGroupFn       func(ctx context.Context, groupCode string) ([]domain.CropRef, error)
	recentVolumeFn       func(ctx context.Context, groupCode string, from, to, cutoff time.Time) ([]domain.CropMarketVolume, error)
	// 2026-08-08 新增：RecentVolumeForCrop，issue #2248 跨市場 statistics
	// 的 crop path 用（見 domain.ForecastRepository.RecentVolumeForCrop）。
	recentVolumeForCropFn func(ctx context.Context, cropCode string, from, to, cutoff time.Time) ([]domain.MarketVolume, error)

	// Task 11 新增（trend-summary integration tests）：CropByCode，供
	// TrendSummary 的 crop path 在完全沒有觀測列時仍能解出 categoryCode
	// （見 domain.ForecastRepository.CropByCode 的介面註解）。本檔既有測試
	// 都不會踩到這個欄位（nil 時安全回零值），只有新加的 crop path
	// market-closed 情境需要設它。
	cropByCodeFn func(ctx context.Context, cropCode string) (domain.CropRef, error)

	// Task 3 新增：GetObservedPrice 用（本檔目前沒有任何 usecase 呼叫它們，
	// 只是滿足 domain.ForecastRepository 介面——observed_price.go 才會接上
	// 真正的呼叫）。cutoff 參數是 review Finding I1（2026-07-31）加的。
	observedPricesByCropFn  func(ctx context.Context, marketCode, cropCode string, from, to, cutoff time.Time) ([]domain.ObservedPrice, error)
	observedPricesByGroupFn func(ctx context.Context, groupCode, marketCode string, from, to, cutoff time.Time) ([]domain.ObservedPrice, error)
	cropGroupsByNameFn      func(ctx context.Context, name string) ([]domain.CropGroupMatch, error)

	// Task 2 新增：MarketByCode/SearchCatalog 用（本檔目前沒有任何 usecase
	// 呼叫它們，只是滿足 domain.ForecastRepository 介面——Task 6 的
	// SearchCropCatalog usecase 才會接上真正的呼叫）。
	marketByCodeFn     func(ctx context.Context, marketCode string) (domain.MarketRef, error)
	groupCropOptionsFn func(ctx context.Context, groupCode string, windowDays, limit int32) ([]domain.CatalogSearchRow, error)

	// 最終 review Finding I3 新增：MarketByCode 的批次版。SearchCropCatalog
	// 用它一次解析整個結果集的市場名稱；forecast_curve 的 marketComparison
	// 仍走單筆版（本波刻意不動它）。
	marketsByCodeFn func(ctx context.Context, marketCodes []string) ([]domain.MarketRef, error)

	// calledAny 用來證明陷阱 3 的防禦在碰任何一次 repo 呼叫之前就擋下無效輸入
	// （零值 Origin / 空字串 market-crop-group）——不是先打了 DB 才發現沒資料。
	calledAny bool
}

func (r *fakeForecastRepo) SeriesObservations(ctx context.Context, marketCode, cropCode string, from, to, cutoff time.Time) ([]domain.Observation, error) {
	r.calledAny = true
	if r.seriesObservationsFn == nil {
		return nil, nil
	}
	return r.seriesObservationsFn(ctx, marketCode, cropCode, from, to, cutoff)
}

func (r *fakeForecastRepo) GroupObservations(ctx context.Context, groupCode, marketCode string, from, to, cutoff time.Time) ([]domain.Observation, error) {
	r.calledAny = true
	if r.groupObservationsFn == nil {
		return nil, nil
	}
	return r.groupObservationsFn(ctx, groupCode, marketCode, from, to, cutoff)
}

func (r *fakeForecastRepo) RestDays(ctx context.Context, marketCode string, from, to time.Time) ([]domain.RestDay, error) {
	r.calledAny = true
	if r.restDaysFn == nil {
		return nil, nil
	}
	return r.restDaysFn(ctx, marketCode, from, to)
}

func (r *fakeForecastRepo) RestDayCoverage(ctx context.Context, marketCode string) ([]domain.RestDayCoverage, error) {
	r.calledAny = true
	if r.restDayCoverageFn == nil {
		return nil, nil
	}
	return r.restDayCoverageFn(ctx, marketCode)
}

func (r *fakeForecastRepo) CropsInGroup(ctx context.Context, groupCode string) ([]domain.CropRef, error) {
	r.calledAny = true
	if r.cropsInGroupFn == nil {
		return nil, nil
	}
	return r.cropsInGroupFn(ctx, groupCode)
}

func (r *fakeForecastRepo) RecentVolume(ctx context.Context, groupCode string, from, to, cutoff time.Time) ([]domain.CropMarketVolume, error) {
	r.calledAny = true
	if r.recentVolumeFn == nil {
		return nil, nil
	}
	return r.recentVolumeFn(ctx, groupCode, from, to, cutoff)
}

func (r *fakeForecastRepo) RecentVolumeForCrop(ctx context.Context, cropCode string, from, to, cutoff time.Time) ([]domain.MarketVolume, error) {
	r.calledAny = true
	if r.recentVolumeForCropFn == nil {
		return nil, nil
	}
	return r.recentVolumeForCropFn(ctx, cropCode, from, to, cutoff)
}

func (r *fakeForecastRepo) CropByCode(ctx context.Context, cropCode string) (domain.CropRef, error) {
	r.calledAny = true
	if r.cropByCodeFn == nil {
		return domain.CropRef{}, nil
	}
	return r.cropByCodeFn(ctx, cropCode)
}

func (r *fakeForecastRepo) ObservedPricesByCrop(ctx context.Context, marketCode, cropCode string, from, to, cutoff time.Time) ([]domain.ObservedPrice, error) {
	r.calledAny = true
	if r.observedPricesByCropFn == nil {
		return nil, nil
	}
	return r.observedPricesByCropFn(ctx, marketCode, cropCode, from, to, cutoff)
}

func (r *fakeForecastRepo) ObservedPricesByGroup(ctx context.Context, groupCode, marketCode string, from, to, cutoff time.Time) ([]domain.ObservedPrice, error) {
	r.calledAny = true
	if r.observedPricesByGroupFn == nil {
		return nil, nil
	}
	return r.observedPricesByGroupFn(ctx, groupCode, marketCode, from, to, cutoff)
}

func (r *fakeForecastRepo) CropGroupsByName(ctx context.Context, name string) ([]domain.CropGroupMatch, error) {
	r.calledAny = true
	if r.cropGroupsByNameFn == nil {
		return nil, nil
	}
	return r.cropGroupsByNameFn(ctx, name)
}

func (r *fakeForecastRepo) MarketByCode(ctx context.Context, marketCode string) (domain.MarketRef, error) {
	r.calledAny = true
	if r.marketByCodeFn == nil {
		return domain.MarketRef{}, nil
	}
	return r.marketByCodeFn(ctx, marketCode)
}

func (r *fakeForecastRepo) MarketsByCode(ctx context.Context, marketCodes []string) ([]domain.MarketRef, error) {
	r.calledAny = true
	if r.marketsByCodeFn == nil {
		return nil, nil
	}
	return r.marketsByCodeFn(ctx, marketCodes)
}

func (r *fakeForecastRepo) GroupCropOptionsByRecentVolume(ctx context.Context, groupCode string, windowDays, limit int32) ([]domain.CatalogSearchRow, error) {
	r.calledAny = true
	if r.groupCropOptionsFn == nil {
		return nil, nil
	}
	return r.groupCropOptionsFn(ctx, groupCode, windowDays, limit)
}

var _ domain.ForecastRepository = (*fakeForecastRepo)(nil)

// ==================== 測試用 artifact / observation 建構 helper ====================

// testLogger 回傳一個會記錄 Warn 以上等級的 logger，供斷言「陷阱防禦有沒有
// 真的留下可觀察的痕跡」使用（陷阱 2：coverage 空、artifact 血緣異常時必須
// Warn，不能靜默）。
func testLogger() (*zap.Logger, *observer.ObservedLogs) {
	core, logs := observer.New(zap.WarnLevel)
	return zap.New(core), logs
}

func defaultTestInference() config.InferenceConfig {
	return config.InferenceConfig{
		MinRidgeSampleCount: 100, FreshWithinHours: 36, StaleWithinHours: 72,
		GroupVolumeWindowDays: 28, ObservedWindowDays: 30,
		GroupMinMarketSupportCount: forecast.DefaultGroupMinMarketSupportCount,
	}
}

func testConfig(inf config.InferenceConfig) *config.Config {
	return &config.Config{Inference: inf}
}

// testFeatureConfig 是一份維度小、便於手算的 FeatureConfig（比照
// internal/domain/forecast/sevendays_test.go 的 testArtifact 同款縮小版）。
func testFeatureConfig() forecast.FeatureConfig {
	return forecast.FeatureConfig{PriceLags: []int{1}, VolumeLags: []int{1}, RollingWindows: []int{7}, MaxDaysSinceLastTrade: 365}
}

func datePtr(y, m, d int) *forecast.Date {
	v := forecast.NewDate(y, m, d)
	return &v
}

func strPtr(s string) *string { return &s }

func floatPtr(v float64) *float64 { return &v }

// testHorizonModel 是一個血緣完全一致、通過 CalibrationGateReason 的
// direct_ridge horizon。
//
// 🔴 review Finding 1（2026-07-30）：原本 Ridge 係數全 0，Predict 恆等於
// intercept，跟餵進去的歷史觀測完全無關——這讓任何「驗證歷史有沒有被正確
// 接線」的測試（例如 GroupObservations 是否真的按市場分組）對它宣稱要擋的
// bug 完全不敏感：即使歷史被接錯（例如兩個市場的觀測互相蓋掉），predict
// 出來的值仍然只由 artifact 裡的 intercept 決定，跟接沒接對無關。
//
// 修法：在 "last_price" 這個真的會被 BuildFeatureVectorFromDailyPoints 讀
// 進來的特徵上給係數 1（feature_mean=0、feature_scale=1），使
// Predict = intercept + last_price。呼叫端若想讓預測值純粹反映歷史，傳
// intercept=0 即可（TestGroupForecast_GroupsObservationsByMarketAndCrop
// 就是這樣用的）；其餘既有測試都不斷言精確數值，只斷言 status/單調性，不受影響。
func testHorizonModel(sampleCount int, intercept float64) forecast.HorizonModel {
	names := testFeatureConfig().FeatureNames()
	dims := len(names)
	coef := make([]float64, dims)
	mean := make([]float64, dims)
	scale := make([]float64, dims)
	for i := range scale {
		scale[i] = 1
	}
	for i, name := range names {
		if name == "last_price" {
			coef[i] = 1
		}
	}
	return forecast.HorizonModel{
		Kind: forecast.ModelKindDirectRidge, SampleCount: sampleCount, CalibrationCount: 24,
		ProperTrainingCount:         sampleCount - 24,
		CalibrationMethod:           "chronological_tail_strict_split",
		ProperTrainingEndTargetDate: datePtr(2026, 6, 24),
		CalibrationStartTargetDate:  datePtr(2026, 6, 25),
		CalibrationEndTargetDate:    datePtr(2026, 7, 18),
		ResidualQuantiles:           [3]float64{-1, 0, 1},
		FallbackPrice:               intercept,
		SeasonalLagDays:             7,
		Ridge:                       &forecast.RidgeParameters{Intercept: intercept, Coefficients: coef, FeatureMean: mean, FeatureScale: scale},
	}
}

func testSeriesModel(seriesID, marketID, cropID, categoryCode string, sampleCount int, intercept float64) forecast.SeriesModel {
	horizons := make(map[string]forecast.HorizonModel, 7)
	for h := 1; h <= 7; h++ {
		horizons[strconv.Itoa(h)] = testHorizonModel(sampleCount, intercept)
	}
	return forecast.SeriesModel{SeriesID: seriesID, MarketID: marketID, CropID: cropID, CategoryCode: strPtr(categoryCode), Horizons: horizons}
}

func testArtifact(series ...forecast.SeriesModel) forecast.ModelArtifact {
	cfg := testFeatureConfig()
	seriesMap := make(map[string]forecast.SeriesModel, len(series))
	for _, s := range series {
		seriesMap[s.SeriesID] = s
	}
	return forecast.ModelArtifact{
		ArtifactSchemaVersion: forecast.ArtifactSchemaVersion,
		ModelVersion:          "test-model-v1",
		DatasetDigest:         strings.Repeat("a", 64),
		TrainedAt:             time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
		FeatureSchema:         cfg.FeatureNames(),
		Config: forecast.TrainingConfig{
			HorizonDays: 7, RidgeAlpha: 10, MinSamplesPerModel: 30,
			CalibrationFraction: 0.2, MinCalibrationSamples: 7, SeasonalLagDays: 7,
			AvailabilityPolicy: "latest_corrected_backfill", FeatureConfig: cfg,
		},
		Series:   seriesMap,
		Metadata: map[string]any{"calendar_timezone": "UTC"},
	}
}

func testObservation(marketCode, cropCode string, categoryCode *string, tradeDate time.Time, avgPrice, volumeKg *float64, availableAt time.Time) domain.Observation {
	return domain.Observation{
		MarketCode: marketCode, CropCode: cropCode, CategoryCode: categoryCode,
		MarketActive: true, CropActive: true, TradeDate: tradeDate,
		AvgPrice: avgPrice, VolumeKg: volumeKg, AvailableAt: availableAt, RevisionNo: 1,
	}
}

// dailyObservations 產出 [from, to]（含頭尾）逐日一筆的觀測，每筆都有價格與成交量。
func dailyObservations(marketCode, cropCode string, categoryCode *string, from, to forecast.Date, price, volume float64, availableAt time.Time) []domain.Observation {
	var out []domain.Observation
	for d := from; !d.After(to); d = d.AddDays(1) {
		out = append(out, testObservation(marketCode, cropCode, categoryCode, d.ToTime(), floatPtr(price), floatPtr(volume), availableAt))
	}
	return out
}

// ==================== 陷阱 3：Origin 零值 / 空字串必須在碰 DB 前被擋下 ====================

func TestSeriesForecast_RejectsZeroOrigin(t *testing.T) {
	repo := &fakeForecastRepo{}
	logger, _ := testLogger()
	fc := NewForecast(repo, testConfig(defaultTestInference()), logger)

	_, err := fc.SeriesForecast(context.Background(), SeriesForecastInput{
		Artifact:   testArtifact(testSeriesModel("moa:109:A1", "moa:109", "moa:A1", "N05", 118, 60)),
		MarketCode: "109", CropCode: "A1", Origin: time.Time{},
	})
	if err == nil {
		t.Fatal("零值 Origin 應報錯")
	}
	if repo.calledAny {
		t.Error("零值 Origin 會讓 pgxutil.TimestamptzParam/DateParam 靜默變成 SQL NULL、查詢靜默回 0 列——" +
			"必須在碰任何一次 repo 呼叫之前就被擋下，不能先打了 DB 才發現查不到東西")
	}
}

func TestSeriesForecast_RejectsEmptyMarketOrCrop(t *testing.T) {
	for _, tc := range []struct {
		name       string
		marketCode string
		cropCode   string
	}{
		{"empty market", "", "A1"},
		{"empty crop", "109", ""},
		{"both empty", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeForecastRepo{}
			logger, _ := testLogger()
			fc := NewForecast(repo, testConfig(defaultTestInference()), logger)

			_, err := fc.SeriesForecast(context.Background(), SeriesForecastInput{
				Artifact: testArtifact(), MarketCode: tc.marketCode, CropCode: tc.cropCode,
				Origin: time.Date(2026, 7, 25, 8, 0, 0, 0, time.UTC),
			})
			if err == nil {
				t.Fatal("預期報錯")
			}
			if repo.calledAny {
				t.Error("空字串 market/crop 應在碰 DB 前被擋下")
			}
		})
	}
}

func TestGroupForecast_RejectsZeroOrigin(t *testing.T) {
	repo := &fakeForecastRepo{}
	logger, _ := testLogger()
	fc := NewForecast(repo, testConfig(defaultTestInference()), logger)

	_, err := fc.GroupForecast(context.Background(), GroupForecastInput{
		Artifact: testArtifact(), GroupCode: "GRP-A", Origin: time.Time{},
	})
	if err == nil {
		t.Fatal("零值 Origin 應報錯")
	}
	if repo.calledAny {
		t.Error("零值 Origin 應在碰任何一次 repo 呼叫之前就被擋下")
	}
}

func TestGroupForecast_RejectsEmptyGroupCode(t *testing.T) {
	repo := &fakeForecastRepo{}
	logger, _ := testLogger()
	fc := NewForecast(repo, testConfig(defaultTestInference()), logger)

	_, err := fc.GroupForecast(context.Background(), GroupForecastInput{
		Artifact: testArtifact(), GroupCode: "", Origin: time.Date(2026, 7, 25, 8, 0, 0, 0, time.UTC),
	})
	if err == nil {
		t.Fatal("預期報錯")
	}
	if repo.calledAny {
		t.Error("空字串 group code 應在碰 DB 前被擋下")
	}
}

// TestSeriesForecast_UnknownSeriesReturnsError 釘住 task-11-brief.md Step 4 的
// 「demo artifact 只有兩條 series，查其他組合會回錯誤——這是預期行為」。
func TestSeriesForecast_UnknownSeriesReturnsError(t *testing.T) {
	repo := &fakeForecastRepo{}
	logger, _ := testLogger()
	fc := NewForecast(repo, testConfig(defaultTestInference()), logger)

	_, err := fc.SeriesForecast(context.Background(), SeriesForecastInput{
		Artifact:   testArtifact(testSeriesModel("moa:109:A1", "moa:109", "moa:A1", "N05", 118, 60)),
		MarketCode: "999", CropCode: "ZZ", Origin: time.Date(2026, 7, 25, 8, 0, 0, 0, time.UTC),
	})
	if err == nil {
		t.Fatal("artifact 沒有這條 series 應報錯")
	}
	if repo.calledAny {
		t.Error("artifact 缺 series 應在碰 DB 前就擋下——不需要查資料庫就能判斷")
	}
}

// ==================== 陷阱 2：VerifiedCalendar 必須顯式建構，coverage 空要 Warn ====================

func TestSeriesForecast_EmptyRestDayCoverage_MarksInsufficientDataAndWarns(t *testing.T) {
	origin := time.Date(2026, 7, 25, 8, 0, 0, 0, time.UTC)
	originDate := forecast.NewDate(2026, 7, 25)
	marketCode, cropCode, category := "109", "A1", "N05"

	repo := &fakeForecastRepo{
		seriesObservationsFn: func(ctx context.Context, mc, cc string, from, to, cutoff time.Time) ([]domain.Observation, error) {
			return dailyObservations(marketCode, cropCode, strPtr(category), originDate.AddDays(-40), originDate, 60, 1000, origin.Add(-time.Hour)), nil
		},
		restDaysFn: func(ctx context.Context, mc string, from, to time.Time) ([]domain.RestDay, error) {
			return nil, nil
		},
		restDayCoverageFn: func(ctx context.Context, mc string) ([]domain.RestDayCoverage, error) {
			// 🔴 陷阱 2 的核心情境：完全沒有休市日曆覆蓋。
			return nil, nil
		},
	}
	logger, logs := testLogger()
	fc := NewForecast(repo, testConfig(defaultTestInference()), logger)

	out, err := fc.SeriesForecast(context.Background(), SeriesForecastInput{
		Artifact:   testArtifact(testSeriesModel("moa:109:A1", "moa:109", "moa:A1", category, 118, 60)),
		MarketCode: marketCode, CropCode: cropCode, Origin: origin,
	})
	if err != nil {
		t.Fatalf("SeriesForecast: %v", err)
	}
	if len(out.Forecast) != 7 {
		t.Fatalf("列數 = %d，預期 7", len(out.Forecast))
	}
	for _, p := range out.Forecast {
		if p.Status != forecast.PredictionStatusInsufficientData {
			t.Errorf("D+%d status = %q，預期 %q（沒有休市日曆覆蓋時，calendar_unverified 必須讓每個點回 insufficient_data，"+
				"不能因為 VerifiedCalendar 落回零值而 fail-open 成 ready）", p.HorizonDays, p.Status, forecast.PredictionStatusInsufficientData)
		}
		if p.MissingReason != forecast.MissingReasonCalendarUnverified {
			t.Errorf("D+%d missing_reason = %q，預期 %q", p.HorizonDays, p.MissingReason, forecast.MissingReasonCalendarUnverified)
		}
	}

	found := false
	for _, entry := range logs.All() {
		if strings.Contains(entry.Message, "no rest-day calendar coverage") {
			found = true
		}
	}
	if !found {
		t.Error("coverage 為空必須留下可觀察的 Warn log，不能靜默發生")
	}
}

// TestSeriesForecast_UnknownMarketTypeInCoverage_SkipsAndWarns 釘住「上游改了
// market_type 代碼」的可觀察性：CategoryCodeForMarketType 查不到時要 Warn 並跳過，
// 不能靜默當成這一年沒有休市日曆（那樣會製造假的 unverified）也不能 panic。
func TestSeriesForecast_UnknownMarketTypeInCoverage_SkipsAndWarns(t *testing.T) {
	origin := time.Date(2026, 7, 25, 8, 0, 0, 0, time.UTC)
	originDate := forecast.NewDate(2026, 7, 25)
	marketCode, cropCode, category := "109", "A1", "N05"

	repo := &fakeForecastRepo{
		seriesObservationsFn: func(ctx context.Context, mc, cc string, from, to, cutoff time.Time) ([]domain.Observation, error) {
			return dailyObservations(marketCode, cropCode, strPtr(category), originDate.AddDays(-40), originDate, 60, 1000, origin.Add(-time.Hour)), nil
		},
		restDayCoverageFn: func(ctx context.Context, mc string) ([]domain.RestDayCoverage, error) {
			return []domain.RestDayCoverage{{MarketCode: marketCode, MarketType: "X", Year: 2026}}, nil
		},
	}
	logger, logs := testLogger()
	fc := NewForecast(repo, testConfig(defaultTestInference()), logger)

	out, err := fc.SeriesForecast(context.Background(), SeriesForecastInput{
		Artifact:   testArtifact(testSeriesModel("moa:109:A1", "moa:109", "moa:A1", category, 118, 60)),
		MarketCode: marketCode, CropCode: cropCode, Origin: origin,
	})
	if err != nil {
		t.Fatalf("SeriesForecast: %v", err)
	}
	for _, p := range out.Forecast {
		if p.MissingReason != forecast.MissingReasonCalendarUnverified {
			t.Errorf("D+%d missing_reason = %q，預期 %q（未知 market_type 應被跳過，等同沒有覆蓋）", p.HorizonDays, p.MissingReason, forecast.MissingReasonCalendarUnverified)
		}
	}
	found := false
	for _, entry := range logs.All() {
		if strings.Contains(entry.Message, "unknown market_type") {
			found = true
		}
	}
	if !found {
		t.Error("未知 market_type 必須留下可觀察的 Warn log")
	}
}

// ==================== 陷阱 1：GroupObservations 必須按 (market, crop) 分組 ====================

// TestGroupForecast_GroupsObservationsByMarketAndCrop 是三個陷阱裡最容易被
// silently 弄錯的一個：GroupObservations(marketCode="") 對同一個
// (crop, trade_date) 會回多個市場各一列。若 usecase 誤把它們用「只看
// trade_date」的鍵收斂（例如 map[Date]Observation 覆寫），其中一個市場的
// 觀測會被另一個市場悄悄蓋掉，聚合出來的價格會是「某個不相干市場剛好蓋過去」
// 的結果，而不是兩個市場都真的參與加權。
//
// 兩個市場、同一個作物、同一天：市場 109 價格 100（量 300）、市場 110 價格 200
// （量 100）。這個作物是本大類唯一的子品項，所以市場內加權（stage 1）就等於
// 各市場自己的價格；跨市場加權（stage 2）= (100*300 + 200*100) / 400 = 125。
// 若市場鍵被弄丟，結果會是 100 或 200 其中一個，而不是 125——這就是本測試的
// 判準。
//
// 🔴 review Finding 1（2026-07-30）：series 的 intercept 刻意設成 0——若非 0，
// Predict = intercept + last_price 兩者都會影響結果，intercept 的固定貢獻會
// 稀釋掉「歷史被接錯」造成的差異，可能讓斷言的容錯區間意外放行一個接錯歷史
// 的結果。intercept=0 讓 P50 純粹由 last_price（也就是餵進去的歷史）決定，
// 這正是本測試要驗證的東西。
func TestGroupForecast_GroupsObservationsByMarketAndCrop(t *testing.T) {
	origin := time.Date(2026, 7, 25, 8, 0, 0, 0, time.UTC)
	originDate := forecast.NewDate(2026, 7, 25)
	groupCode, cropCode, category := "GRP-A", "A1", "N05"

	series109 := testSeriesModel("moa:109:A1", "moa:109", "moa:A1", category, 118, 0)
	series110 := testSeriesModel("moa:110:A1", "moa:110", "moa:A1", category, 118, 0)

	repo := &fakeForecastRepo{
		cropsInGroupFn: func(ctx context.Context, gc string) ([]domain.CropRef, error) {
			return []domain.CropRef{{CropCode: cropCode, Name: "測試作物", Active: true, CategoryCode: strPtr(category)}}, nil
		},
		groupObservationsFn: func(ctx context.Context, gc, mc string, from, to, cutoff time.Time) ([]domain.Observation, error) {
			if mc != "" {
				t.Fatalf("GroupForecast 應以 marketCode=\"\" 呼叫 GroupObservations（不縮限市場），got %q", mc)
			}
			out := dailyObservations("109", cropCode, strPtr(category), originDate.AddDays(-40), originDate, 100, 300, origin.Add(-time.Hour))
			out = append(out, dailyObservations("110", cropCode, strPtr(category), originDate.AddDays(-40), originDate, 200, 100, origin.Add(-time.Hour))...)
			return out, nil
		},
		restDayCoverageFn: func(ctx context.Context, mc string) ([]domain.RestDayCoverage, error) {
			return []domain.RestDayCoverage{{MarketCode: mc, MarketType: "F", Year: 2026}}, nil
		},
		recentVolumeFn: func(ctx context.Context, gc string, from, to, cutoff time.Time) ([]domain.CropMarketVolume, error) {
			return []domain.CropMarketVolume{
				{MarketCode: "109", CropCode: cropCode, Volume: 300},
				{MarketCode: "110", CropCode: cropCode, Volume: 100},
			}, nil
		},
	}
	logger, _ := testLogger()
	fc := NewForecast(repo, testConfig(defaultTestInference()), logger)

	out, err := fc.GroupForecast(context.Background(), GroupForecastInput{
		Artifact: testArtifact(series109, series110), GroupCode: groupCode, Origin: origin,
	})
	if err != nil {
		t.Fatalf("GroupForecast: %v", err)
	}
	if len(out.Forecast) != 7 {
		t.Fatalf("列數 = %d，預期 7", len(out.Forecast))
	}
	wantMaxAvailableAt := origin.Add(-time.Hour)
	if out.MaxFeatureAvailableAt == nil || !out.MaxFeatureAvailableAt.Equal(wantMaxAvailableAt) {
		t.Errorf("MaxFeatureAvailableAt = %v，預期 %v（GroupForecastOutput 也要帶這個欄位，"+
			"同 SeriesForecastOutput，供 issue #2249 Slice A dataMeta.maxFeatureAvailableAt 使用）",
			out.MaxFeatureAvailableAt, wantMaxAvailableAt)
	}
	for _, p := range out.Forecast {
		if p.MarketSupportCount != 2 {
			t.Fatalf("D+%d market_support_count = %d，預期 2（兩個市場都該參與，"+
				"若市場鍵被弄丟只會剩 1）", p.HorizonDays, p.MarketSupportCount)
		}
		if p.Status != forecast.PredictionStatusReady {
			t.Fatalf("D+%d status = %q，預期 ready（測試資料應該乾淨到能出價）：missing_reason=%q", p.HorizonDays, p.Status, p.MissingReason)
		}
		if p.P50 == nil {
			t.Fatalf("D+%d p50 為 nil", p.HorizonDays)
		}
		const want = 125.0
		if diff := *p.P50 - want; diff > 1e-6 || diff < -1e-6 {
			t.Errorf("D+%d p50 = %v，預期 %v（(100*300+200*100)/400——"+
				"若市場鍵被弄丟，其中一個市場的觀測會悄悄蓋掉另一個，結果會是 100 或 200 而不是 125）",
				p.HorizonDays, *p.P50, want)
		}
	}
}

// TestGroupForecast_UnknownMarketTypeInCoverage_SkipsAndWarns 證明 GroupForecast
// 與 SeriesForecast 經 appendVerifiedCalendarKeys 共用同一份行為：未知
// market_type 記 Warn 並跳過，不 panic、不誤判成有覆蓋（P4 review followup——
// 修正前這段映射邏輯在 GroupForecast 裡自己重寫一份，沒有測試釘住兩者一致）。
func TestGroupForecast_UnknownMarketTypeInCoverage_SkipsAndWarns(t *testing.T) {
	origin := time.Date(2026, 7, 25, 8, 0, 0, 0, time.UTC)
	originDate := forecast.NewDate(2026, 7, 25)
	groupCode, cropCode, category := "GRP-A", "A1", "N05"

	repo := &fakeForecastRepo{
		cropsInGroupFn: func(ctx context.Context, gc string) ([]domain.CropRef, error) {
			return []domain.CropRef{{CropCode: cropCode, Name: "測試作物", Active: true, CategoryCode: strPtr(category)}}, nil
		},
		groupObservationsFn: func(ctx context.Context, gc, mc string, from, to, cutoff time.Time) ([]domain.Observation, error) {
			return dailyObservations("109", cropCode, strPtr(category), originDate.AddDays(-40), originDate, 60, 1000, origin.Add(-time.Hour)), nil
		},
		restDayCoverageFn: func(ctx context.Context, mc string) ([]domain.RestDayCoverage, error) {
			return []domain.RestDayCoverage{{MarketCode: mc, MarketType: "X", Year: 2026}}, nil
		},
		recentVolumeFn: func(ctx context.Context, gc string, from, to, cutoff time.Time) ([]domain.CropMarketVolume, error) {
			return []domain.CropMarketVolume{{MarketCode: "109", CropCode: cropCode, Volume: 300}}, nil
		},
	}
	logger, logs := testLogger()
	fc := NewForecast(repo, testConfig(defaultTestInference()), logger)

	out, err := fc.GroupForecast(context.Background(), GroupForecastInput{
		Artifact:  testArtifact(testSeriesModel("moa:109:A1", "moa:109", "moa:A1", category, 118, 60)),
		GroupCode: groupCode, Origin: origin,
	})
	if err != nil {
		t.Fatalf("GroupForecast: %v", err)
	}
	for _, p := range out.Forecast {
		if p.MissingReason != forecast.MissingReasonCalendarUnverified {
			t.Errorf("D+%d missing_reason = %q，預期 %q（未知 market_type 應被跳過，等同沒有覆蓋）", p.HorizonDays, p.MissingReason, forecast.MissingReasonCalendarUnverified)
		}
	}
	found := false
	for _, entry := range logs.All() {
		if strings.Contains(entry.Message, "unknown market_type") {
			found = true
		}
	}
	if !found {
		t.Error("未知 market_type 必須留下可觀察的 Warn log")
	}
}

// TestGroupForecast_EmptyRestDayCoverage_WarnsWithGroupCode 釘住共用
// appendVerifiedCalendarKeys 之後沒有弄丟 group_code 這個可觀察性欄位——多個
// 大類共用同一市場時，這是唯一能從 log 分辨是哪個大類觸發「沒有休市日曆覆蓋」
// 的線索（P4 review followup：對抗式 review 抓到共用化時弄丟了這個欄位）。
func TestGroupForecast_EmptyRestDayCoverage_WarnsWithGroupCode(t *testing.T) {
	origin := time.Date(2026, 7, 25, 8, 0, 0, 0, time.UTC)
	originDate := forecast.NewDate(2026, 7, 25)
	groupCode, cropCode, category := "GRP-A", "A1", "N05"

	repo := &fakeForecastRepo{
		cropsInGroupFn: func(ctx context.Context, gc string) ([]domain.CropRef, error) {
			return []domain.CropRef{{CropCode: cropCode, Name: "測試作物", Active: true, CategoryCode: strPtr(category)}}, nil
		},
		groupObservationsFn: func(ctx context.Context, gc, mc string, from, to, cutoff time.Time) ([]domain.Observation, error) {
			return dailyObservations("109", cropCode, strPtr(category), originDate.AddDays(-40), originDate, 60, 1000, origin.Add(-time.Hour)), nil
		},
		restDayCoverageFn: func(ctx context.Context, mc string) ([]domain.RestDayCoverage, error) {
			return nil, nil // 完全沒有覆蓋
		},
		recentVolumeFn: func(ctx context.Context, gc string, from, to, cutoff time.Time) ([]domain.CropMarketVolume, error) {
			return []domain.CropMarketVolume{{MarketCode: "109", CropCode: cropCode, Volume: 300}}, nil
		},
	}
	logger, logs := testLogger()
	fc := NewForecast(repo, testConfig(defaultTestInference()), logger)

	_, err := fc.GroupForecast(context.Background(), GroupForecastInput{
		Artifact:  testArtifact(testSeriesModel("moa:109:A1", "moa:109", "moa:A1", category, 118, 60)),
		GroupCode: groupCode, Origin: origin,
	})
	if err != nil {
		t.Fatalf("GroupForecast: %v", err)
	}

	found := false
	for _, entry := range logs.All() {
		if strings.Contains(entry.Message, "no rest-day calendar coverage") {
			found = true
			if got := entry.ContextMap()["group_code"]; got != groupCode {
				t.Errorf("group_code = %v，預期 %q", got, groupCode)
			}
		}
	}
	if !found {
		t.Error("沒有休市日曆覆蓋時必須留下可觀察的 Warn log")
	}
}

// ==================== MIN_RIDGE_SAMPLE_COUNT=0 停用整條閘門 ====================

// TestSeriesForecast_MinRidgeSampleCountZero_DisablesGate 端到端證明
// config.InferenceConfig.MinRidgeSampleCount=0 真的停用了 ridge 樣本量閘門：
// 同一份 sample_count=50（遠低於預設門檻 100）的資料，門檻=100 時被擋成
// insufficient_data／below_ridge_sample_threshold，門檻=0 時放行成 ready。
func TestSeriesForecast_MinRidgeSampleCountZero_DisablesGate(t *testing.T) {
	origin := time.Date(2026, 7, 25, 8, 0, 0, 0, time.UTC)
	originDate := forecast.NewDate(2026, 7, 25)
	marketCode, cropCode, category := "109", "A1", "N05"

	buildRepo := func() *fakeForecastRepo {
		return &fakeForecastRepo{
			seriesObservationsFn: func(ctx context.Context, mc, cc string, from, to, cutoff time.Time) ([]domain.Observation, error) {
				return dailyObservations(marketCode, cropCode, strPtr(category), originDate.AddDays(-40), originDate, 60, 1000, origin.Add(-time.Hour)), nil
			},
			restDayCoverageFn: func(ctx context.Context, mc string) ([]domain.RestDayCoverage, error) {
				return []domain.RestDayCoverage{{MarketCode: mc, MarketType: "F", Year: 2026}}, nil
			},
		}
	}
	artifact := testArtifact(testSeriesModel("moa:109:A1", "moa:109", "moa:A1", category, 50, 60))

	t.Run("default threshold 100 blocks sample_count 50", func(t *testing.T) {
		logger, _ := testLogger()
		fc := NewForecast(buildRepo(), testConfig(defaultTestInference()), logger)
		out, err := fc.SeriesForecast(context.Background(), SeriesForecastInput{Artifact: artifact, MarketCode: marketCode, CropCode: cropCode, Origin: origin})
		if err != nil {
			t.Fatalf("SeriesForecast: %v", err)
		}
		for _, p := range out.Forecast {
			if p.Status != forecast.PredictionStatusInsufficientData || p.MissingReason != forecast.MissingReasonBelowRidgeSampleThreshold {
				t.Errorf("D+%d = (%q, %q)，預期 (%q, %q)", p.HorizonDays, p.Status, p.MissingReason,
					forecast.PredictionStatusInsufficientData, forecast.MissingReasonBelowRidgeSampleThreshold)
			}
		}
	})

	t.Run("MIN_RIDGE_SAMPLE_COUNT=0 disables the gate", func(t *testing.T) {
		inf := defaultTestInference()
		inf.MinRidgeSampleCount = 0
		logger, _ := testLogger()
		fc := NewForecast(buildRepo(), testConfig(inf), logger)
		out, err := fc.SeriesForecast(context.Background(), SeriesForecastInput{Artifact: artifact, MarketCode: marketCode, CropCode: cropCode, Origin: origin})
		if err != nil {
			t.Fatalf("SeriesForecast: %v", err)
		}
		for _, p := range out.Forecast {
			if p.Status != forecast.PredictionStatusReady {
				t.Errorf("D+%d status = %q，預期 ready（MIN_RIDGE_SAMPLE_COUNT=0 應停用整條閘門）：missing_reason=%q", p.HorizonDays, p.Status, p.MissingReason)
			}
		}
	})
}

// ==================== Happy path：Observed 窗分類 + 七日輸出形狀 ====================

// TestSeriesForecast_HappyPath 驗證一條乾淨的 series：七列齊全、target_date
// 連續、至少一列 ready 且 p10<=p50<=p90、Observed 窗四種 market_status
// （closed/no_trade/missing/open）依規則分類、data_through_date 抓到最新的
// trade_date。
func TestSeriesForecast_HappyPath(t *testing.T) {
	origin := time.Date(2026, 7, 25, 8, 0, 0, 0, time.UTC)
	originDate := forecast.NewDate(2026, 7, 25) // ObservedWindowDays=4 → 07-22..07-25
	marketCode, cropCode, category := "109", "A1", "N05"

	repo := &fakeForecastRepo{
		seriesObservationsFn: func(ctx context.Context, mc, cc string, from, to, cutoff time.Time) ([]domain.Observation, error) {
			// 60 天正常歷史（供特徵計算），再疊上 Observed 窗要測的四種狀態：
			// 07-22 open（正常價格）、07-23 missing（不回這天）、
			// 07-24 no_trade（有列但 avg_price=NULL）、07-25 closed（休市日曆命中）。
			base := dailyObservations(marketCode, cropCode, strPtr(category), originDate.AddDays(-60), originDate.AddDays(-4), 55, 900, origin.Add(-2*time.Hour))
			base = append(base, testObservation(marketCode, cropCode, strPtr(category), originDate.AddDays(-3).ToTime(), floatPtr(58), floatPtr(950), origin.Add(-time.Hour)))
			// -2 天（07-23）刻意不回傳，對應 missing。
			base = append(base, testObservation(marketCode, cropCode, strPtr(category), originDate.AddDays(-1).ToTime(), nil, nil, origin.Add(-time.Hour)))
			base = append(base, testObservation(marketCode, cropCode, strPtr(category), originDate.ToTime(), floatPtr(61), floatPtr(1000), origin.Add(-30*time.Minute)))
			return base, nil
		},
		restDaysFn: func(ctx context.Context, mc string, from, to time.Time) ([]domain.RestDay, error) {
			// originDate 本身休市（落在 [originDate, originDate+7] 查詢窗內）。
			return []domain.RestDay{{MarketCode: mc, MarketType: "F", RestDate: originDate.ToTime()}}, nil
		},
		restDayCoverageFn: func(ctx context.Context, mc string) ([]domain.RestDayCoverage, error) {
			return []domain.RestDayCoverage{{MarketCode: mc, MarketType: "F", Year: 2026}}, nil
		},
	}
	logger, _ := testLogger()
	inf := defaultTestInference()
	inf.ObservedWindowDays = 4
	fc := NewForecast(repo, testConfig(inf), logger)

	out, err := fc.SeriesForecast(context.Background(), SeriesForecastInput{
		Artifact:   testArtifact(testSeriesModel("moa:109:A1", "moa:109", "moa:A1", category, 118, 60)),
		MarketCode: marketCode, CropCode: cropCode, Origin: origin,
	})
	if err != nil {
		t.Fatalf("SeriesForecast: %v", err)
	}

	if len(out.Forecast) != 7 {
		t.Fatalf("列數 = %d，預期 7", len(out.Forecast))
	}
	for i, p := range out.Forecast {
		if p.HorizonDays != i+1 {
			t.Errorf("第 %d 列 horizon = %d，預期 %d", i, p.HorizonDays, i+1)
		}
		wantDate := originDate.AddDays(i + 1)
		if p.TargetDate != wantDate {
			t.Errorf("第 %d 列 target_date = %s，預期 %s", i, p.TargetDate, wantDate)
		}
	}
	readyCount := 0
	for _, p := range out.Forecast {
		if p.Status == forecast.PredictionStatusReady {
			readyCount++
			if p.P10 == nil || p.P50 == nil || p.P90 == nil {
				t.Fatalf("D+%d status=ready 但分位數不齊全", p.HorizonDays)
			}
			if !(*p.P10 <= *p.P50 && *p.P50 <= *p.P90) {
				t.Errorf("D+%d 分位數不單調: p10=%v p50=%v p90=%v", p.HorizonDays, *p.P10, *p.P50, *p.P90)
			}
		}
	}
	if readyCount == 0 {
		t.Fatal("預期至少一列 ready")
	}

	if out.ModelVersion == "" || out.DatasetDigest == "" {
		t.Error("model_version / dataset_digest 不應為空")
	}
	if out.DataThroughDate == nil || *out.DataThroughDate != originDate {
		t.Errorf("data_through_date = %v，預期 %s（最新一筆觀測就在 originDate）", out.DataThroughDate, originDate)
	}
	if out.Freshness != forecast.FreshnessFresh {
		t.Errorf("freshness = %q，預期 fresh（最新觀測 30 分鐘前才可見）", out.Freshness)
	}
	wantMaxAvailableAt := origin.Add(-30 * time.Minute)
	if out.MaxFeatureAvailableAt == nil || !out.MaxFeatureAvailableAt.Equal(wantMaxAvailableAt) {
		t.Errorf("MaxFeatureAvailableAt = %v，預期 %v（issue #2249 Slice A dataMeta.maxFeatureAvailableAt 要用的"+
			"「知識時間截止」，等於本次用來算 freshness 的同一個 maxAvailableAt——不是另開一條邏輯）", out.MaxFeatureAvailableAt, wantMaxAvailableAt)
	}

	if len(out.Observed) != 4 {
		t.Fatalf("Observed 列數 = %d，預期 4（ObservedWindowDays=4）", len(out.Observed))
	}
	wantStatus := map[forecast.Date]string{
		originDate.AddDays(-3): "open",
		originDate.AddDays(-2): "missing",
		originDate.AddDays(-1): "no_trade",
		originDate:             "closed",
	}
	for _, op := range out.Observed {
		want, ok := wantStatus[op.TradeDate]
		if !ok {
			t.Fatalf("Observed 出現非預期日期 %s", op.TradeDate)
		}
		if op.MarketStatus != want {
			t.Errorf("%s market_status = %q，預期 %q", op.TradeDate, op.MarketStatus, want)
		}
	}
}

// TestSeriesForecast_FreshnessUnavailable_DoesNotOverridePointStatus
// 釘住 2026-09-14 的決定（見 ADR
// docs/adr/2026-09-14-pricebook-freshness-describes-age-not-usability.md）：新鮮度只回答
// 「這份數字有多舊」，不回答「能不能用」。>72h 時 Freshness 仍如實回報
// unavailable，但逐點狀態必須維持品質閘門自己算出來的結果——否則模型算得出
// 來的預測會被整條丟掉（實測 moa:338:LA1 被丟棄的 D+1 對照隔日實際成交誤差
// 僅 0.5%），而桃園市這類只有單一批發市場的縣市會因此完全拿不到參考價。
func TestSeriesForecast_FreshnessUnavailable_DoesNotOverridePointStatus(t *testing.T) {
	origin := time.Date(2026, 7, 25, 8, 0, 0, 0, time.UTC)
	originDate := forecast.NewDate(2026, 7, 25)
	marketCode, cropCode, category := "109", "A1", "N05"

	repo := &fakeForecastRepo{
		seriesObservationsFn: func(ctx context.Context, mc, cc string, from, to, cutoff time.Time) ([]domain.Observation, error) {
			// 最新一筆觀測的 available_at 是 100 小時前——遠超過 72h stale 上限。
			return dailyObservations(marketCode, cropCode, strPtr(category), originDate.AddDays(-40), originDate, 60, 1000, origin.Add(-100*time.Hour)), nil
		},
		restDayCoverageFn: func(ctx context.Context, mc string) ([]domain.RestDayCoverage, error) {
			return []domain.RestDayCoverage{{MarketCode: mc, MarketType: "F", Year: 2026}}, nil
		},
	}
	logger, _ := testLogger()
	fc := NewForecast(repo, testConfig(defaultTestInference()), logger)

	out, err := fc.SeriesForecast(context.Background(), SeriesForecastInput{
		Artifact:   testArtifact(testSeriesModel("moa:109:A1", "moa:109", "moa:A1", category, 118, 60)),
		MarketCode: marketCode, CropCode: cropCode, Origin: origin,
	})
	if err != nil {
		t.Fatalf("SeriesForecast: %v", err)
	}
	// 新鮮度本身必須誠實——這是消費端判斷「要不要用」的唯一依據，不得一併拿掉。
	if out.Freshness != forecast.FreshnessUnavailable {
		t.Fatalf("freshness = %q，預期 unavailable（新鮮度訊號必須保留）", out.Freshness)
	}
	usable := 0
	for _, p := range out.Forecast {
		if p.Status == forecast.PredictionStatusUnavailable {
			t.Errorf("D+%d status = unavailable，預期由品質閘門決定而非被新鮮度覆寫", p.HorizonDays)
		}
		if usablePredictionStatus(p.Status) {
			usable++
			if p.P50 == nil {
				t.Errorf("D+%d status = %q 但 P50 為 nil", p.HorizonDays, p.Status)
			}
		}
	}
	if usable == 0 {
		t.Fatal("沒有任何一點可用；本 fixture 的資料足以出價，覆寫移除後應至少有一點可用")
	}
}

// TestGroupForecast_FreshnessUnavailable_DoesNotOverridePointStatus 是
// TestSeriesForecast_FreshnessUnavailable_DoesNotOverridePointStatus 的大類版本。
// 兩處覆寫必須一起移除：只移 series 端會造成「單一市場曲線有值、全國大類曲線
// 消失」的不一致。
func TestGroupForecast_FreshnessUnavailable_DoesNotOverridePointStatus(t *testing.T) {
	origin := time.Date(2026, 7, 25, 8, 0, 0, 0, time.UTC)
	originDate := forecast.NewDate(2026, 7, 25)
	groupCode, cropCode, category := "GRP-A", "A1", "N05"

	repo := &fakeForecastRepo{
		cropsInGroupFn: func(ctx context.Context, gc string) ([]domain.CropRef, error) {
			return []domain.CropRef{{CropCode: cropCode, Name: "測試作物", Active: true, CategoryCode: strPtr(category)}}, nil
		},
		groupObservationsFn: func(ctx context.Context, gc, mc string, from, to, cutoff time.Time) ([]domain.Observation, error) {
			// 最新一筆觀測的 available_at 是 100 小時前——遠超過 72h stale 上限，
			// 比照 series 端同款測試。
			return dailyObservations("109", cropCode, strPtr(category), originDate.AddDays(-40), originDate, 100, 300, origin.Add(-100*time.Hour)), nil
		},
		restDayCoverageFn: func(ctx context.Context, mc string) ([]domain.RestDayCoverage, error) {
			return []domain.RestDayCoverage{{MarketCode: mc, MarketType: "F", Year: 2026}}, nil
		},
		recentVolumeFn: func(ctx context.Context, gc string, from, to, cutoff time.Time) ([]domain.CropMarketVolume, error) {
			return []domain.CropMarketVolume{{MarketCode: "109", CropCode: cropCode, Volume: 300}}, nil
		},
	}
	logger, _ := testLogger()
	fc := NewForecast(repo, testConfig(defaultTestInference()), logger)

	out, err := fc.GroupForecast(context.Background(), GroupForecastInput{
		Artifact:  testArtifact(testSeriesModel("moa:109:A1", "moa:109", "moa:A1", category, 118, 0)),
		GroupCode: groupCode, Origin: origin,
	})
	if err != nil {
		t.Fatalf("GroupForecast: %v", err)
	}
	if out.Freshness != forecast.FreshnessUnavailable {
		t.Fatalf("freshness = %q，預期 unavailable（新鮮度訊號必須保留）", out.Freshness)
	}
	if len(out.Forecast) != 7 {
		t.Fatalf("列數 = %d，預期 7", len(out.Forecast))
	}
	usable := 0
	for _, p := range out.Forecast {
		if p.Status == forecast.PredictionStatusUnavailable {
			t.Errorf("D+%d status = unavailable，預期由品質閘門決定而非被新鮮度覆寫", p.HorizonDays)
		}
		if usablePredictionStatus(p.Status) {
			usable++
			if p.P50 == nil {
				t.Errorf("D+%d status = %q 但 P50 為 nil", p.HorizonDays, p.Status)
			}
		}
	}
	if usable == 0 {
		t.Fatal("沒有任何一點可用；本 fixture 的資料足以出價，覆寫移除後應至少有一點可用")
	}
}

// TestSeriesForecast_FreshnessDoesNotChangePredictedValues 釘住「新鮮度與預測值
// 完全正交」：同一組觀測，只改 available_at（100h 前 vs 40h 前），
// p10/p50/p90、Status、MissingReason 必須逐點完全一致，只有 Freshness 不同。
//
// 這支測試存在的理由是它會在「有人重新引入任何一種以新鮮度改寫預測結果的邏輯」
// 時立刻轉紅——不論那個改寫是覆寫 Status、清空 P50，還是加寬區間。
func TestSeriesForecast_FreshnessDoesNotChangePredictedValues(t *testing.T) {
	origin := time.Date(2026, 7, 25, 8, 0, 0, 0, time.UTC)
	originDate := forecast.NewDate(2026, 7, 25)
	marketCode, cropCode, category := "109", "A1", "N05"

	run := func(availableAt time.Time) *SeriesForecastOutput {
		t.Helper()
		repo := &fakeForecastRepo{
			seriesObservationsFn: func(ctx context.Context, mc, cc string, from, to, cutoff time.Time) ([]domain.Observation, error) {
				return dailyObservations(marketCode, cropCode, strPtr(category), originDate.AddDays(-40), originDate, 60, 1000, availableAt), nil
			},
			restDayCoverageFn: func(ctx context.Context, mc string) ([]domain.RestDayCoverage, error) {
				return []domain.RestDayCoverage{{MarketCode: mc, MarketType: "F", Year: 2026}}, nil
			},
		}
		logger, _ := testLogger()
		fc := NewForecast(repo, testConfig(defaultTestInference()), logger)
		out, err := fc.SeriesForecast(context.Background(), SeriesForecastInput{
			Artifact:   testArtifact(testSeriesModel("moa:109:A1", "moa:109", "moa:A1", category, 118, 60)),
			MarketCode: marketCode, CropCode: cropCode, Origin: origin,
		})
		if err != nil {
			t.Fatalf("SeriesForecast: %v", err)
		}
		return out
	}

	old := run(origin.Add(-100 * time.Hour))   // >72h → unavailable
	recent := run(origin.Add(-40 * time.Hour)) // 36h~72h → stale

	if old.Freshness != forecast.FreshnessUnavailable {
		t.Fatalf("old.Freshness = %q，預期 unavailable", old.Freshness)
	}
	if recent.Freshness != forecast.FreshnessStale {
		t.Fatalf("recent.Freshness = %q，預期 stale", recent.Freshness)
	}
	if len(old.Forecast) != len(recent.Forecast) {
		t.Fatalf("列數不同：old=%d recent=%d", len(old.Forecast), len(recent.Forecast))
	}
	for i := range old.Forecast {
		o, r := old.Forecast[i], recent.Forecast[i]
		if o.Status != r.Status {
			t.Errorf("D+%d status 不同：old=%q recent=%q", o.HorizonDays, o.Status, r.Status)
		}
		if o.MissingReason != r.MissingReason {
			t.Errorf("D+%d missing_reason 不同：old=%q recent=%q", o.HorizonDays, o.MissingReason, r.MissingReason)
		}
		assertSameFloatPtr(t, o.HorizonDays, "p10", o.P10, r.P10)
		assertSameFloatPtr(t, o.HorizonDays, "p50", o.P50, r.P50)
		assertSameFloatPtr(t, o.HorizonDays, "p90", o.P90, r.P90)
	}
}

func assertSameFloatPtr(t *testing.T, horizon int, name string, a, b *float64) {
	t.Helper()
	if (a == nil) != (b == nil) {
		t.Errorf("D+%d %s 一邊為 nil：old=%v recent=%v", horizon, name, a, b)
		return
	}
	if a != nil && *a != *b {
		t.Errorf("D+%d %s 不同：old=%v recent=%v", horizon, name, *a, *b)
	}
}

// ==================== GroupForecast 額外邊界 ====================

func TestGroupForecast_NoCropsInGroupReturnsError(t *testing.T) {
	repo := &fakeForecastRepo{
		cropsInGroupFn: func(ctx context.Context, gc string) ([]domain.CropRef, error) {
			return nil, nil
		},
	}
	logger, _ := testLogger()
	fc := NewForecast(repo, testConfig(defaultTestInference()), logger)

	_, err := fc.GroupForecast(context.Background(), GroupForecastInput{
		Artifact: testArtifact(), GroupCode: "GRP-EMPTY", Origin: time.Date(2026, 7, 25, 8, 0, 0, 0, time.UTC),
	})
	if err == nil {
		t.Fatal("大類沒有任何子品項應報錯")
	}
}

// ==================== review 2026-07-30 五個 Finding 的回歸測試 ====================

// TestSeriesForecast_ObservedWindow_HistoricalClosedDayIsClosedNotMissing 是
// Finding 2 的回歸測試：Observed 窗回看的是過去 ObservedWindowDays 天，但
// RestDays 原本只查 [originDate, originDate+7]（只夠涵蓋 D+1..D+7 推論窗），
// 導致 originDate 之前的休市日全部被誤判成 missing。這裡種一個 5 天前
// （在 Observed 窗內、但在原本的查詢窗外）的休市日，market_status 必須是
// closed，不是 missing；同時直接斷言 RestDays 的查詢下界確實涵蓋到那一天
// （不是只看最終分類結果，連 SQL 查詢範圍本身都釘住）。
func TestSeriesForecast_ObservedWindow_HistoricalClosedDayIsClosedNotMissing(t *testing.T) {
	origin := time.Date(2026, 7, 25, 8, 0, 0, 0, time.UTC)
	originDate := forecast.NewDate(2026, 7, 25)
	marketCode, cropCode, category := "109", "A1", "N05"
	historicalClosedDate := originDate.AddDays(-5) // 2026-07-20，遠在 originDate 之前

	repo := &fakeForecastRepo{
		seriesObservationsFn: func(ctx context.Context, mc, cc string, from, to, cutoff time.Time) ([]domain.Observation, error) {
			return dailyObservations(marketCode, cropCode, strPtr(category), originDate.AddDays(-60), originDate, 60, 1000, origin.Add(-time.Hour)), nil
		},
		restDaysFn: func(ctx context.Context, mc string, from, to time.Time) ([]domain.RestDay, error) {
			if from.After(historicalClosedDate.ToTime()) {
				t.Fatalf("RestDays 查詢窗 from=%s 沒有涵蓋歷史休市日 %s——Observed 窗需要往回查這麼遠", from, historicalClosedDate)
			}
			return []domain.RestDay{{MarketCode: mc, MarketType: "F", RestDate: historicalClosedDate.ToTime()}}, nil
		},
		restDayCoverageFn: func(ctx context.Context, mc string) ([]domain.RestDayCoverage, error) {
			return []domain.RestDayCoverage{{MarketCode: mc, MarketType: "F", Year: 2026}}, nil
		},
	}
	logger, _ := testLogger()
	inf := defaultTestInference()
	inf.ObservedWindowDays = 10 // 涵蓋 originDate-9..originDate，確保歷史休市日落在窗內
	fc := NewForecast(repo, testConfig(inf), logger)

	out, err := fc.SeriesForecast(context.Background(), SeriesForecastInput{
		Artifact:   testArtifact(testSeriesModel("moa:109:A1", "moa:109", "moa:A1", category, 118, 0)),
		MarketCode: marketCode, CropCode: cropCode, Origin: origin,
	})
	if err != nil {
		t.Fatalf("SeriesForecast: %v", err)
	}
	var found bool
	for _, op := range out.Observed {
		if op.TradeDate == historicalClosedDate {
			found = true
			if op.MarketStatus != "closed" {
				t.Errorf("歷史休市日 %s 的 market_status = %q，預期 %q（不能因為 RestDays 查詢窗沒往回涵蓋過去就誤判成 missing）",
					historicalClosedDate, op.MarketStatus, "closed")
			}
		}
	}
	if !found {
		t.Fatalf("Observed 窗沒有包含 %s，測試設定有誤", historicalClosedDate)
	}
}

// TestSeriesForecast_ObservedWindow_NoTradeAndClosedDaysKeepVolumeAndRevision
// 是 Finding 3 的回歸測試：no_trade 分支原本只設 RevisionNo、丟掉
// VolumeKg（即使該列 volume_kg 非 NULL）；closed 分支連 RevisionNo 都沒填。
// schema 上 avg_price 與 volume_kg 是各自獨立可為 NULL 的欄位，不該因為
// market_status 落哪個分支就把本來有的欄位變不見。
func TestSeriesForecast_ObservedWindow_NoTradeAndClosedDaysKeepVolumeAndRevision(t *testing.T) {
	origin := time.Date(2026, 7, 25, 8, 0, 0, 0, time.UTC)
	originDate := forecast.NewDate(2026, 7, 25)
	marketCode, cropCode, category := "109", "A1", "N05"
	noTradeDate := originDate.AddDays(-1)       // 有列，avg_price=NULL，volume_kg 非 NULL
	closedDateWithRow := originDate.AddDays(-2) // 休市，但剛好也有一筆觀測列

	repo := &fakeForecastRepo{
		seriesObservationsFn: func(ctx context.Context, mc, cc string, from, to, cutoff time.Time) ([]domain.Observation, error) {
			base := dailyObservations(marketCode, cropCode, strPtr(category), originDate.AddDays(-60), originDate.AddDays(-3), 55, 900, origin.Add(-2*time.Hour))
			base = append(base, testObservation(marketCode, cropCode, strPtr(category), closedDateWithRow.ToTime(), floatPtr(58), floatPtr(950), origin.Add(-time.Hour)))
			base = append(base, testObservation(marketCode, cropCode, strPtr(category), noTradeDate.ToTime(), nil, floatPtr(500), origin.Add(-time.Hour)))
			base = append(base, testObservation(marketCode, cropCode, strPtr(category), originDate.ToTime(), floatPtr(61), floatPtr(1000), origin.Add(-30*time.Minute)))
			return base, nil
		},
		restDaysFn: func(ctx context.Context, mc string, from, to time.Time) ([]domain.RestDay, error) {
			return []domain.RestDay{{MarketCode: mc, MarketType: "F", RestDate: closedDateWithRow.ToTime()}}, nil
		},
		restDayCoverageFn: func(ctx context.Context, mc string) ([]domain.RestDayCoverage, error) {
			return []domain.RestDayCoverage{{MarketCode: mc, MarketType: "F", Year: 2026}}, nil
		},
	}
	logger, _ := testLogger()
	inf := defaultTestInference()
	inf.ObservedWindowDays = 4
	fc := NewForecast(repo, testConfig(inf), logger)

	out, err := fc.SeriesForecast(context.Background(), SeriesForecastInput{
		Artifact:   testArtifact(testSeriesModel("moa:109:A1", "moa:109", "moa:A1", category, 118, 0)),
		MarketCode: marketCode, CropCode: cropCode, Origin: origin,
	})
	if err != nil {
		t.Fatalf("SeriesForecast: %v", err)
	}

	byDate := map[forecast.Date]ObservedPoint{}
	for _, op := range out.Observed {
		byDate[op.TradeDate] = op
	}

	noTrade, ok := byDate[noTradeDate]
	if !ok {
		t.Fatalf("Observed 窗缺 %s", noTradeDate)
	}
	if noTrade.MarketStatus != "no_trade" {
		t.Fatalf("%s market_status = %q，預期 no_trade", noTradeDate, noTrade.MarketStatus)
	}
	if noTrade.VolumeKg == nil || *noTrade.VolumeKg != 500 {
		t.Errorf("no_trade 分支丟了 VolumeKg：got %v，預期 500（該列 volume_kg 非 NULL，不該因為 avg_price 是 NULL 就連帶消失）", noTrade.VolumeKg)
	}
	if noTrade.RevisionNo != 1 {
		t.Errorf("no_trade 分支的 RevisionNo = %d，預期 1", noTrade.RevisionNo)
	}

	closed, ok := byDate[closedDateWithRow]
	if !ok {
		t.Fatalf("Observed 窗缺 %s", closedDateWithRow)
	}
	if closed.MarketStatus != "closed" {
		t.Fatalf("%s market_status = %q，預期 closed", closedDateWithRow, closed.MarketStatus)
	}
	if closed.RevisionNo != 1 {
		t.Errorf("closed 分支的 RevisionNo = %d，預期 1（原本 closed 分支完全沒填這個欄位）", closed.RevisionNo)
	}
	if closed.VolumeKg == nil || *closed.VolumeKg != 950 {
		t.Errorf("closed 分支的 VolumeKg = %v，預期 950", closed.VolumeKg)
	}
}

// TestGroupForecast_InactiveCropDoesNotBlockReady 是 Finding 4 的回歸測試：
// CropsInGroup 不過濾 active，一個已退場、沒有近期成交的子品項若被計進
// totalVariantCount，會讓 EffectiveVariantCount < TotalVariantCount 恆成立，
// 大類永遠回 partial、永遠不會是 ready。大類含一個 inactive 子品項時，
// 其餘全 ready 應得到 ready 而非 partial。
func TestGroupForecast_InactiveCropDoesNotBlockReady(t *testing.T) {
	origin := time.Date(2026, 7, 25, 8, 0, 0, 0, time.UTC)
	originDate := forecast.NewDate(2026, 7, 25)
	groupCode, activeCrop, inactiveCrop, category := "GRP-B", "A1", "ZZ", "N05"

	activeSeries := testSeriesModel("moa:109:A1", "moa:109", "moa:A1", category, 118, 0)

	repo := &fakeForecastRepo{
		cropsInGroupFn: func(ctx context.Context, gc string) ([]domain.CropRef, error) {
			return []domain.CropRef{
				{CropCode: activeCrop, Name: "現役作物", Active: true, CategoryCode: strPtr(category)},
				{CropCode: inactiveCrop, Name: "已退場作物", Active: false, CategoryCode: strPtr(category)},
			}, nil
		},
		groupObservationsFn: func(ctx context.Context, gc, mc string, from, to, cutoff time.Time) ([]domain.Observation, error) {
			// inactiveCrop 完全沒有觀測（已退場的典型形狀）；活躍作物有乾淨歷史。
			return dailyObservations("109", activeCrop, strPtr(category), originDate.AddDays(-40), originDate, 60, 500, origin.Add(-time.Hour)), nil
		},
		restDayCoverageFn: func(ctx context.Context, mc string) ([]domain.RestDayCoverage, error) {
			return []domain.RestDayCoverage{{MarketCode: mc, MarketType: "F", Year: 2026}}, nil
		},
		recentVolumeFn: func(ctx context.Context, gc string, from, to, cutoff time.Time) ([]domain.CropMarketVolume, error) {
			return []domain.CropMarketVolume{{MarketCode: "109", CropCode: activeCrop, Volume: 500}}, nil
		},
	}
	logger, _ := testLogger()
	fc := NewForecast(repo, testConfig(defaultTestInference()), logger)

	out, err := fc.GroupForecast(context.Background(), GroupForecastInput{
		Artifact: testArtifact(activeSeries), GroupCode: groupCode, Origin: origin,
	})
	if err != nil {
		t.Fatalf("GroupForecast: %v", err)
	}
	for _, p := range out.Forecast {
		if p.TotalVariantCount != 1 {
			t.Fatalf("D+%d total_variant_count = %d，預期 1（inactive 子品項不該計入分母）", p.HorizonDays, p.TotalVariantCount)
		}
		if p.Status != forecast.PredictionStatusReady {
			t.Errorf("D+%d status = %q，預期 ready（唯一的 active 子品項資料乾淨，不該被一個已退場、"+
				"artifact 裡根本沒有對應 series 的子品項拖成 partial）：missing_reason=%q", p.HorizonDays, p.Status, p.MissingReason)
		}
	}
}

// TestGroupForecast_TotalVariantCount_OnlyCountsCropsWithModelCoverage 是 D1
// 的回歸測試（ADR
// 私人封存的 ADR 2026-08-07-pricebook-group-readiness-gate-resolution）：
// TotalVariantCount 改成只算「DB-active 且上游模型/artifact 也有 series 覆蓋」
// 的子品項，不是全部 DB-active 子品項。
//
// 兩個都是 DB-active 的子品項，但只有一個在 artifact 裡有對應 series——
// 舊定義（len(activeCrops)）會把 TotalVariantCount 算成 2，讓
// EffectiveVariantCount(1) < TotalVariantCount(2) 恆成立、大類永遠 partial；
// 新定義下 TotalVariantCount 應收斂成 1（只有真的有模型覆蓋的那個），讓
// EffectiveVariantCount == TotalVariantCount，正確得到 ready。
func TestGroupForecast_TotalVariantCount_OnlyCountsCropsWithModelCoverage(t *testing.T) {
	origin := time.Date(2026, 7, 25, 8, 0, 0, 0, time.UTC)
	originDate := forecast.NewDate(2026, 7, 25)
	groupCode, coveredCrop, uncoveredCrop, category := "GRP-C", "A1", "B2", "N05"

	coveredSeries := testSeriesModel("moa:109:A1", "moa:109", "moa:A1", category, 118, 0)

	repo := &fakeForecastRepo{
		cropsInGroupFn: func(ctx context.Context, gc string) ([]domain.CropRef, error) {
			return []domain.CropRef{
				{CropCode: coveredCrop, Name: "有模型覆蓋", Active: true, CategoryCode: strPtr(category)},
				{CropCode: uncoveredCrop, Name: "DB active 但模型未覆蓋", Active: true, CategoryCode: strPtr(category)},
			}, nil
		},
		groupObservationsFn: func(ctx context.Context, gc, mc string, from, to, cutoff time.Time) ([]domain.Observation, error) {
			// uncoveredCrop 完全沒有觀測列——它有 DB active 列，只是模型沒有
			// 對應 series，跟已退場作物的形狀不同，刻意用來證明「有觀測資料」
			// 不是 D1 判準，「有 series 覆蓋」才是。
			return dailyObservations("109", coveredCrop, strPtr(category), originDate.AddDays(-40), originDate, 60, 500, origin.Add(-time.Hour)), nil
		},
		restDayCoverageFn: func(ctx context.Context, mc string) ([]domain.RestDayCoverage, error) {
			return []domain.RestDayCoverage{{MarketCode: mc, MarketType: "F", Year: 2026}}, nil
		},
		recentVolumeFn: func(ctx context.Context, gc string, from, to, cutoff time.Time) ([]domain.CropMarketVolume, error) {
			return []domain.CropMarketVolume{{MarketCode: "109", CropCode: coveredCrop, Volume: 500}}, nil
		},
	}
	logger, _ := testLogger()
	fc := NewForecast(repo, testConfig(defaultTestInference()), logger)

	out, err := fc.GroupForecast(context.Background(), GroupForecastInput{
		Artifact: testArtifact(coveredSeries), GroupCode: groupCode, Origin: origin,
	})
	if err != nil {
		t.Fatalf("GroupForecast: %v", err)
	}
	for _, p := range out.Forecast {
		if p.TotalVariantCount != 1 {
			t.Fatalf("D+%d total_variant_count = %d，預期 1（D1：DB-active 但模型未覆蓋的子品項不該計入分母）", p.HorizonDays, p.TotalVariantCount)
		}
		if p.Status != forecast.PredictionStatusReady {
			t.Errorf("D+%d status = %q，預期 ready（唯一有模型覆蓋的子品項資料乾淨，不該被一個模型未覆蓋的"+
				"子品項拖成 partial）：missing_reason=%q", p.HorizonDays, p.Status, p.MissingReason)
		}
	}
}

// ==================== calendarTimezoneOf 三份副本的分岔測試 ====================

// TestSeriesForecast_CalendarTimezoneMismatch_ProducesTaipeiNotUTCQueryWindow
// 釘住 calendarTimezoneOf 的其中一份副本（internal/usecase/forecast.go:96-101）
// ——它有三份幾乎一樣的拷貝（另兩份在 cmd/pricebook-feature-health/main.go 與
// internal/domain/forecast/sevendays.go），三處註解都寫「改動需同步」，但在
// 這條測試補上之前零測試把它們釘在一起。唯一的 integration fixture
// （cmd/pricebook-forecast）用 origin=08:00Z，UTC 與 artifact 宣告的
// Asia/Taipei 在那個時刻剛好是同一天，分岔完全不可見。
//
// 這裡改用一個讓 UTC 與 Asia/Taipei 落在不同日期的 origin
// （2026-07-25T17:30:00Z = 台北時間 07-26 01:30），斷言兩件事：
//  1. usecase 傳給 repo.SeriesObservations 的查詢窗 to 是台北日期 07-26 的
//     UTC 午夜，不是 UTC 日期 07-25 的午夜——這就是 usecase 自己那份
//     calendarTimezoneOf 副本的輸出。
//  2. SevenDays（sevendays.go 內建自己另一份副本）算出的 D+1 target_date 是
//     台北日期 07-27——用同一個 artifact，兩處副本各自獨立算，若其中一處被
//     改壞（例如漏改、退化成硬編 "UTC"），這裡就會分岔。
//
// 對應設計書描述的線上後果：SevenDays 的 originDate 比 usecase 算的晚一天時，
// 查詢窗 to 少一天、offset 0 那天的觀測被排除、Observed 窗與 D+1 之間憑空
// 多一天空隙——全程不報錯。
func TestSeriesForecast_CalendarTimezoneMismatch_ProducesTaipeiNotUTCQueryWindow(t *testing.T) {
	origin := time.Date(2026, 7, 25, 17, 30, 0, 0, time.UTC) // 台北時間 2026-07-26 01:30
	marketCode, cropCode, category := "109", "A1", "N05"

	var gotTo time.Time
	repo := &fakeForecastRepo{
		seriesObservationsFn: func(ctx context.Context, mc, cc string, from, to, cutoff time.Time) ([]domain.Observation, error) {
			gotTo = to
			return nil, nil
		},
		restDayCoverageFn: func(ctx context.Context, mc string) ([]domain.RestDayCoverage, error) {
			return []domain.RestDayCoverage{{MarketCode: mc, MarketType: "F", Year: 2026}}, nil
		},
	}
	logger, _ := testLogger()
	fc := NewForecast(repo, testConfig(defaultTestInference()), logger)

	artifact := testArtifact(testSeriesModel("moa:109:A1", "moa:109", "moa:A1", category, 118, 60))
	artifact.Metadata = map[string]any{"calendar_timezone": "Asia/Taipei"}

	out, err := fc.SeriesForecast(context.Background(), SeriesForecastInput{
		Artifact: artifact, MarketCode: marketCode, CropCode: cropCode, Origin: origin,
	})
	if err != nil {
		t.Fatalf("SeriesForecast: %v", err)
	}

	wantTo := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC) // 台北日期 07-26 的 UTC 午夜
	if !gotTo.Equal(wantTo) {
		t.Errorf("傳給 SeriesObservations 的查詢窗 to = %s，預期 %s（台北日期，不是 UTC 日期 "+
			"07-25 的午夜）——usecase 的 calendarTimezoneOf 副本沒有正確讀到 artifact 宣告的 "+
			"Asia/Taipei", gotTo, wantTo)
	}

	if len(out.Forecast) != 7 {
		t.Fatalf("列數 = %d，預期 7", len(out.Forecast))
	}
	wantD1 := forecast.NewDate(2026, 7, 27) // 台北 originDate（07-26）+1
	if out.Forecast[0].TargetDate != wantD1 {
		t.Errorf("D+1 target_date = %s，預期 %s（SevenDays 內部那份 calendarTimezoneOf 副本算出的"+
			"台北日期，跟 usecase 那份對不上就是設計書描述的分岔）", out.Forecast[0].TargetDate, wantD1)
	}
}

// ==================== GroupObservedSeries ====================

func TestGroupObservedSeries_TwoStageWeightsRawDailyPrices(t *testing.T) {
	d1 := forecast.NewDate(2026, 8, 1)
	availableAt1 := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	availableAt2 := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC) // later — must win MaxAvailableAt
	repo := &fakeForecastRepo{
		cropsInGroupFn: func(ctx context.Context, groupCode string) ([]domain.CropRef, error) {
			return []domain.CropRef{{CropCode: "crop1", Active: true}, {CropCode: "crop2", Active: true}}, nil
		},
		observedPricesByGroupFn: func(ctx context.Context, groupCode, marketCode string, from, to, cutoff time.Time) ([]domain.ObservedPrice, error) {
			return []domain.ObservedPrice{
				{MarketCode: "A", CropCode: "crop1", TradeDate: d1.ToTime(), AvgPrice: floatPtr(100), SourceDigest: repeatHexTest('1'), AvailableAt: availableAt1},
				{MarketCode: "A", CropCode: "crop2", TradeDate: d1.ToTime(), AvgPrice: floatPtr(200), SourceDigest: repeatHexTest('2'), AvailableAt: availableAt2},
			}, nil
		},
		recentVolumeFn: func(ctx context.Context, groupCode string, from, to, cutoff time.Time) ([]domain.CropMarketVolume, error) {
			return []domain.CropMarketVolume{
				{MarketCode: "A", CropCode: "crop1", Volume: 300},
				{MarketCode: "A", CropCode: "crop2", Volume: 100},
			}, nil
		},
	}
	logger, _ := testLogger()
	u := NewForecast(repo, testConfig(defaultTestInference()), logger)

	out, err := u.GroupObservedSeries(context.Background(), GroupObservedSeriesInput{
		GroupCode: "group1", From: d1.ToTime(), To: d1.ToTime(), Now: d1.ToTime(),
	})

	if err != nil {
		t.Fatalf("GroupObservedSeries: unexpected error %v", err)
	}
	if len(out.Points) != 1 {
		t.Fatalf("len(out.Points) = %d, want 1", len(out.Points))
	}
	want := (100.0*300 + 200.0*100) / 400.0
	if out.Points[0].Value == nil || math.Abs(*out.Points[0].Value-want) > 1e-9 {
		t.Errorf("Points[0].Value = %v, want %v", out.Points[0].Value, want)
	}

	// issue #2249 Slice A dataMeta 前置：group-aggregated 觀測路徑的
	// synthesized point 本身沒有 available_at/source_digest（見
	// GroupObservedSeriesOutput.Points 的型別），這兩個信號要從
	// GroupObservedSeriesOutput 本身額外帶出來，取自實際貢獻進聚合結果的
	// raw rows（inactive/nil-price 的列不算——同一份過濾條件）。
	if out.MaxAvailableAt == nil || !out.MaxAvailableAt.Equal(availableAt2) {
		t.Errorf("MaxAvailableAt = %v, want %v (the later of the two contributing rows)", out.MaxAvailableAt, availableAt2)
	}
	wantDigests := map[string]bool{repeatHexTest('1'): true, repeatHexTest('2'): true}
	if len(out.SourceDigests) != 2 {
		t.Fatalf("SourceDigests = %v, want 2 entries", out.SourceDigests)
	}
	for _, d := range out.SourceDigests {
		if !wantDigests[d] {
			t.Errorf("SourceDigests contains unexpected digest %q", d)
		}
	}
}

func TestGroupObservedSeries_InactiveCropExcluded(t *testing.T) {
	d1 := forecast.NewDate(2026, 8, 1)
	repo := &fakeForecastRepo{
		cropsInGroupFn: func(ctx context.Context, groupCode string) ([]domain.CropRef, error) {
			return []domain.CropRef{{CropCode: "crop1", Active: true}, {CropCode: "crop2", Active: false}}, nil
		},
		observedPricesByGroupFn: func(ctx context.Context, groupCode, marketCode string, from, to, cutoff time.Time) ([]domain.ObservedPrice, error) {
			return []domain.ObservedPrice{
				{MarketCode: "A", CropCode: "crop1", TradeDate: d1.ToTime(), AvgPrice: floatPtr(100), SourceDigest: repeatHexTest('1'), AvailableAt: time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)},
				// inactive — must not pull the average toward this, AND must not leak into MaxAvailableAt/SourceDigests either.
				{MarketCode: "A", CropCode: "crop2", TradeDate: d1.ToTime(), AvgPrice: floatPtr(9999), SourceDigest: repeatHexTest('9'), AvailableAt: time.Date(2026, 8, 1, 23, 0, 0, 0, time.UTC)},
			}, nil
		},
		recentVolumeFn: func(ctx context.Context, groupCode string, from, to, cutoff time.Time) ([]domain.CropMarketVolume, error) {
			return []domain.CropMarketVolume{{MarketCode: "A", CropCode: "crop1", Volume: 1}}, nil
		},
	}
	logger, _ := testLogger()
	u := NewForecast(repo, testConfig(defaultTestInference()), logger)

	out, err := u.GroupObservedSeries(context.Background(), GroupObservedSeriesInput{
		GroupCode: "group1", From: d1.ToTime(), To: d1.ToTime(), Now: d1.ToTime(),
	})

	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if len(out.Points) != 1 || out.Points[0].Value == nil || *out.Points[0].Value != 100 {
		t.Fatalf("got %+v, want a single point with Value=100 (inactive crop2 excluded)", out.Points)
	}
	wantMaxAvailableAt := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	if out.MaxAvailableAt == nil || !out.MaxAvailableAt.Equal(wantMaxAvailableAt) {
		t.Errorf("MaxAvailableAt = %v, want %v (inactive crop2's later available_at must not leak in)", out.MaxAvailableAt, wantMaxAvailableAt)
	}
	if len(out.SourceDigests) != 1 || out.SourceDigests[0] != repeatHexTest('1') {
		t.Errorf("SourceDigests = %v, want exactly [%q] (inactive crop2's digest must not leak in)", out.SourceDigests, repeatHexTest('1'))
	}
}

// repeatHexTest 是 domain/forecast 套件內 repeatHex 的獨立小拷貝（不同
// package，無法直接重用測試專用 helper）——回傳 64 位重複字元的假 hex
// digest，僅供測試固定資料使用。
func repeatHexTest(r byte) string {
	b := make([]byte, 64)
	for i := range b {
		b[i] = r
	}
	return string(b)
}

func TestGroupObservedSeries_ZeroNowIsRejected(t *testing.T) {
	logger, _ := testLogger()
	u := NewForecast(&fakeForecastRepo{}, testConfig(defaultTestInference()), logger)
	_, err := u.GroupObservedSeries(context.Background(), GroupObservedSeriesInput{GroupCode: "group1"})
	if err == nil {
		t.Fatal("GroupObservedSeries with zero Now: want error, got nil")
	}
}

// TestGroupObservedSeries_MarketCode_BlendsOnlyThatMarketsCrops proves the
// single-market filter reuses AggregateGroupObservations unchanged: with
// two crops trading in market "109" at different prices/volumes and a third
// crop only trading in a different market "110", filtering to MarketCode:
// "109" must blend only the two "109" crops (volume-weighted), excluding
// the "110"-only crop entirely — not zero-filling it, not blending it in.
func TestGroupObservedSeries_MarketCode_BlendsOnlyThatMarketsCrops(t *testing.T) {
	d1 := forecast.NewDate(2026, 8, 1)
	repo := &fakeForecastRepo{
		cropsInGroupFn: func(ctx context.Context, groupCode string) ([]domain.CropRef, error) {
			return []domain.CropRef{
				{CropCode: "crop1", Active: true},
				{CropCode: "crop2", Active: true},
				{CropCode: "crop3", Active: true},
			}, nil
		},
		observedPricesByGroupFn: func(ctx context.Context, groupCode, marketCode string, from, to, cutoff time.Time) ([]domain.ObservedPrice, error) {
			if marketCode != "109" {
				t.Fatalf("ObservedPricesByGroup called with marketCode=%q, want \"109\" (the filter must reach the repo call)", marketCode)
			}
			// Repo is expected to already filter server-side to the requested
			// market — this fake only returns market-109 rows, proving the
			// usecase passes marketCode through rather than filtering client-side.
			return []domain.ObservedPrice{
				{MarketCode: "109", CropCode: "crop1", TradeDate: d1.ToTime(), AvgPrice: floatPtr(30)},
				{MarketCode: "109", CropCode: "crop2", TradeDate: d1.ToTime(), AvgPrice: floatPtr(70)},
			}, nil
		},
		recentVolumeFn: func(ctx context.Context, groupCode string, from, to, cutoff time.Time) ([]domain.CropMarketVolume, error) {
			return []domain.CropMarketVolume{
				{MarketCode: "109", CropCode: "crop1", Volume: 1},
				{MarketCode: "109", CropCode: "crop2", Volume: 1},
				{MarketCode: "110", CropCode: "crop3", Volume: 999},
			}, nil
		},
	}
	logger, _ := testLogger()
	uc := NewForecast(repo, testConfig(defaultTestInference()), logger)

	out, err := uc.GroupObservedSeries(context.Background(), GroupObservedSeriesInput{
		GroupCode: "group1", MarketCode: "109", From: d1.ToTime(), To: d1.ToTime(), Now: d1.ToTime(),
	})
	if err != nil {
		t.Fatalf("GroupObservedSeries: unexpected error %v", err)
	}
	if len(out.Points) != 1 {
		t.Fatalf("len(Points) = %d, want 1 (one blended point for the single requested date)", len(out.Points))
	}
	if out.Points[0].Value == nil || *out.Points[0].Value != 50 { // (30*1 + 70*1) / 2
		t.Errorf("Points[0].Value = %v, want 50", out.Points[0].Value)
	}
}

// TestGroupObservedSeries_MarketCode_Empty_UnchangedFullAggregate is a
// regression guard: the existing all-markets behavior (MarketCode == "")
// must be byte-for-byte unchanged by this task — MarketCode is additive,
// not a replacement for the existing signature's meaning.
func TestGroupObservedSeries_MarketCode_Empty_UnchangedFullAggregate(t *testing.T) {
	d1 := forecast.NewDate(2026, 8, 1)
	repo := &fakeForecastRepo{
		cropsInGroupFn: func(ctx context.Context, groupCode string) ([]domain.CropRef, error) {
			return []domain.CropRef{{CropCode: "crop1", Active: true}}, nil
		},
		observedPricesByGroupFn: func(ctx context.Context, groupCode, marketCode string, from, to, cutoff time.Time) ([]domain.ObservedPrice, error) {
			if marketCode != "" {
				t.Fatalf("ObservedPricesByGroup called with marketCode=%q, want \"\" (empty MarketCode must still mean all markets)", marketCode)
			}
			return []domain.ObservedPrice{
				{MarketCode: "109", CropCode: "crop1", TradeDate: d1.ToTime(), AvgPrice: floatPtr(30)},
			}, nil
		},
		recentVolumeFn: func(ctx context.Context, groupCode string, from, to, cutoff time.Time) ([]domain.CropMarketVolume, error) {
			return []domain.CropMarketVolume{{MarketCode: "109", CropCode: "crop1", Volume: 1}}, nil
		},
	}
	logger, _ := testLogger()
	uc := NewForecast(repo, testConfig(defaultTestInference()), logger)

	out, err := uc.GroupObservedSeries(context.Background(), GroupObservedSeriesInput{
		GroupCode: "group1", From: d1.ToTime(), To: d1.ToTime(), Now: d1.ToTime(), // MarketCode left zero-value
	})
	if err != nil {
		t.Fatalf("GroupObservedSeries: unexpected error %v", err)
	}
	if len(out.Points) != 1 || out.Points[0].Value == nil || *out.Points[0].Value != 30 {
		t.Fatalf("Points = %+v, want one point at 30", out.Points)
	}
}
