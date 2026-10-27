package forecast

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const parityTestdataDir = "../../../testdata/parity"

type historyFixture struct {
	Points []historyPoint `json:"points"`
}

type historyPoint struct {
	SeriesID            string   `json:"series_id"`
	MarketID            string   `json:"market_id"`
	CropID              string   `json:"crop_id"`
	TradeDate           string   `json:"trade_date"`
	AveragePrice        *float64 `json:"average_price"`
	Volume              *float64 `json:"volume"`
	AvailableAt         string   `json:"available_at"`
	IsMarketClosed      bool     `json:"is_market_closed"`
	AvailabilityQuality string   `json:"availability_quality"`
	CategoryCode        *string  `json:"category_code"`
	IsActive            bool     `json:"is_active"`
}

type parityFixtureFile struct {
	Source struct {
		ForecastOriginLocal string `json:"forecast_origin_local"`
		CalendarTimezone    string `json:"calendar_timezone"`
	} `json:"source"`
	Cases []parityCase `json:"cases"`
}

type parityCase struct {
	SeriesID              string    `json:"series_id"`
	HorizonDays           int       `json:"horizon_days"`
	TargetDate            string    `json:"target_date"`
	PredictionStatus      string    `json:"prediction_status"`
	Features              []float64 `json:"features"`
	MaxFeatureAvailableAt *string   `json:"max_feature_available_at"`
	RidgeCenter           *float64  `json:"ridge_center"`
	PriceP10              *float64  `json:"price_p10"`
	PriceP50              *float64  `json:"price_p50"`
	PriceP90              *float64  `json:"price_p90"`
	MissingReason         *string   `json:"missing_reason"`
	CalendarStatus        string    `json:"calendar_status"`
	MarketStatus          string    `json:"market_status"`
	ModelKind             string    `json:"model_kind"`
}

func loadParityHistory(t *testing.T) map[string]map[Date]PricePoint {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(parityTestdataDir, "history.json"))
	if err != nil {
		t.Fatalf("讀 history.json 失敗: %v（先跑 Task 1 產生它）", err)
	}
	var fx historyFixture
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatalf("解析 history.json 失敗: %v", err)
	}

	bySeries := map[string]map[Date]PricePoint{}
	for _, hp := range fx.Points {
		tradeDate, err := ParseDate(hp.TradeDate)
		if err != nil {
			t.Fatalf("解析 trade_date %q 失敗: %v", hp.TradeDate, err)
		}
		availableAt, err := time.Parse(time.RFC3339, hp.AvailableAt)
		if err != nil {
			t.Fatalf("解析 available_at %q 失敗: %v", hp.AvailableAt, err)
		}
		isActive := hp.IsActive
		point, err := NewPricePoint(PricePointInput{
			SeriesID: hp.SeriesID, MarketID: hp.MarketID, CropID: hp.CropID,
			TradeDate: tradeDate, AveragePrice: hp.AveragePrice, Volume: hp.Volume,
			AvailableAt: availableAt, IsMarketClosed: hp.IsMarketClosed,
			AvailabilityQuality: hp.AvailabilityQuality, CategoryCode: hp.CategoryCode,
			IsActive: &isActive,
		})
		if err != nil {
			t.Fatalf("建立 %s@%s 的 PricePoint 失敗: %v", hp.SeriesID, hp.TradeDate, err)
		}
		if bySeries[hp.SeriesID] == nil {
			bySeries[hp.SeriesID] = map[Date]PricePoint{}
		}
		bySeries[hp.SeriesID][tradeDate] = point
	}
	return bySeries
}

// runParityFixture 跑一份 fixture 的所有 case。wantNotApplicable/wantDirectRidge/
// wantSeasonalNaive 是這份 fixture「應該有幾個」各類 case 的守門值——沒有這層檢查，
// 未來 testdata/parity/regenerate/ 重跑後若 cases 意外變空（或某個 kind，尤其
// seasonal_naive 這種數量最少的分支，悄悄歸零），t.Run 對 0 個 case 迭代仍會印
// `ok`，完全看不出 28 個比對點消失了。
func runParityFixture(t *testing.T, fixtureFile, modelFile string, wantNotApplicable, wantDirectRidge, wantSeasonalNaive int) {
	t.Helper()
	history := loadParityHistory(t)

	raw, err := os.ReadFile(filepath.Join(parityTestdataDir, fixtureFile))
	if err != nil {
		t.Fatalf("讀 %s 失敗: %v", fixtureFile, err)
	}
	var fx parityFixtureFile
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatalf("解析 %s 失敗: %v", fixtureFile, err)
	}

	artifact, err := LoadArtifact(filepath.Join(parityTestdataDir, modelFile))
	if err != nil {
		t.Fatalf("LoadArtifact(%s) 失敗: %v", modelFile, err)
	}

	wantTotal := wantNotApplicable + wantDirectRidge + wantSeasonalNaive
	if len(fx.Cases) != wantTotal {
		t.Fatalf("%s 的 case 數 = %d，預期 %d（not_applicable=%d + direct_ridge=%d + seasonal_naive=%d）；fixture 可能重新產生後意外變空或欄位改名讀不到", fixtureFile, len(fx.Cases), wantTotal, wantNotApplicable, wantDirectRidge, wantSeasonalNaive)
	}
	gotNotApplicable, gotDirectRidge, gotSeasonalNaive := 0, 0, 0
	for _, c := range fx.Cases {
		if c.PredictionStatus == "not_applicable" {
			gotNotApplicable++
			continue
		}
		series, ok := artifact.Series[c.SeriesID]
		if !ok {
			t.Fatalf("artifact 缺少 series %s（case 計數階段）", c.SeriesID)
		}
		horizonModel, ok := series.Horizon(c.HorizonDays)
		if !ok {
			t.Fatalf("series %s 缺少 horizon %d（case 計數階段）", c.SeriesID, c.HorizonDays)
		}
		switch horizonModel.Kind {
		case "direct_ridge":
			gotDirectRidge++
		case "seasonal_naive":
			gotSeasonalNaive++
		default:
			t.Fatalf("case %s D+%d 的 kind %q 不是已知值（case 計數階段）", c.SeriesID, c.HorizonDays, horizonModel.Kind)
		}
	}
	if gotNotApplicable != wantNotApplicable || gotDirectRidge != wantDirectRidge || gotSeasonalNaive != wantSeasonalNaive {
		t.Fatalf("%s 的分類計數 = not_applicable=%d direct_ridge=%d seasonal_naive=%d，預期 not_applicable=%d direct_ridge=%d seasonal_naive=%d", fixtureFile, gotNotApplicable, gotDirectRidge, gotSeasonalNaive, wantNotApplicable, wantDirectRidge, wantSeasonalNaive)
	}

	origin, err := time.Parse(time.RFC3339, fx.Source.ForecastOriginLocal)
	if err != nil {
		t.Fatalf("解析 forecast_origin_local 失敗: %v", err)
	}
	tz := fx.Source.CalendarTimezone

	for _, c := range fx.Cases {
		c := c
		t.Run(fmt.Sprintf("%s_D+%d", c.SeriesID, c.HorizonDays), func(t *testing.T) {
			if c.PredictionStatus == "not_applicable" {
				if c.Features != nil {
					t.Fatalf("not_applicable case 不該有 features（市場休市短路發生在建特徵之前，forecasting.py:250-274）")
				}
				return // P3 不實作 market_closed 判斷（P4 範圍），這裡只驗證 fixture 形狀
			}

			targetDate, err := ParseDate(c.TargetDate)
			if err != nil {
				t.Fatalf("解析 target_date 失敗: %v", err)
			}
			series, ok := artifact.Series[c.SeriesID]
			if !ok {
				t.Fatalf("artifact 缺少 series %s", c.SeriesID)
			}
			horizonModel, ok := series.Horizon(c.HorizonDays)
			if !ok {
				t.Fatalf("series %s 缺少 horizon %d", c.SeriesID, c.HorizonDays)
			}

			seriesHistory := history[c.SeriesID]
			fv, err := BuildFeatureVectorFromDailyPoints(seriesHistory, origin, c.HorizonDays, artifact.Config.FeatureConfig, tz)
			if err != nil {
				t.Fatalf("BuildFeatureVectorFromDailyPoints 失敗: %v", err)
			}
			if len(fv.Values) != len(c.Features) {
				t.Fatalf("特徵維度 = %d，fixture 預期 %d", len(fv.Values), len(c.Features))
			}
			for i, want := range c.Features {
				assertClose(t, fmt.Sprintf("features[%d]=%s", i, fv.Names[i]), fv.Values[i], want, 1e-12)
			}
			if c.MaxFeatureAvailableAt != nil {
				want, err := time.Parse(time.RFC3339, *c.MaxFeatureAvailableAt)
				if err != nil {
					t.Fatalf("解析 max_feature_available_at 失敗: %v", err)
				}
				if fv.MaxFeatureAvailableAt == nil || !fv.MaxFeatureAvailableAt.Equal(want) {
					t.Errorf("max_feature_available_at = %v，預期 %v", fv.MaxFeatureAvailableAt, want)
				}
			}

			var center float64
			switch horizonModel.Kind {
			case "direct_ridge":
				if horizonModel.Ridge == nil {
					t.Fatalf("kind=direct_ridge 但 Ridge 為 nil")
				}
				center, err = horizonModel.Ridge.Predict(fv.Values)
				if err != nil {
					t.Fatalf("Predict 失敗: %v", err)
				}
				if c.RidgeCenter == nil {
					t.Fatalf("fixture 缺 ridge_center")
				}
				assertClose(t, "ridge_center", center, *c.RidgeCenter, 1e-9)
			case "seasonal_naive":
				fallback := horizonModel.FallbackPrice
				centerPtr, err := SeasonalNaiveFromDailyPoints(seriesHistory, origin, targetDate, &fallback, horizonModel.SeasonalLagDays, artifact.Config.FeatureConfig.MaxDaysSinceLastTrade, tz)
				if err != nil {
					t.Fatalf("SeasonalNaiveFromDailyPoints 失敗: %v", err)
				}
				if centerPtr == nil {
					t.Fatalf("預期非 nil（有 fallback_price 保底）")
				}
				center = *centerPtr
			default:
				t.Fatalf("未知的 kind %q", horizonModel.Kind)
			}

			p10, p50, p90, err := OrderedNonNegative(center, horizonModel.ResidualQuantiles)
			if err != nil {
				t.Fatalf("OrderedNonNegative 失敗: %v", err)
			}
			if c.PriceP10 == nil || c.PriceP50 == nil || c.PriceP90 == nil {
				t.Fatalf("fixture 缺 price_p10/50/90")
			}
			assertClose(t, "price_p10", p10, *c.PriceP10, 1e-9)
			assertClose(t, "price_p50", p50, *c.PriceP50, 1e-9)
			assertClose(t, "price_p90", p90, *c.PriceP90, 1e-9)
		})
	}
}

func TestParityFixtureDemo(t *testing.T) {
	// 14 case = 1 not_applicable + 13 direct_ridge + 0 seasonal_naive（demo 模型
	// 兩個 series 皆為 direct_ridge，實測見 task-6-report.md）。
	runParityFixture(t, "parity-fixture.json", "model-demo.json", 1, 13, 0)
}

func TestParityFixtureFallback(t *testing.T) {
	// 14 case = 1 not_applicable + 7 direct_ridge + 6 seasonal_naive（fallback
	// 模型混合兩種 kind，實測見 task-6-report.md）。
	runParityFixture(t, "parity-fixture-fallback.json", "model-fallback.json", 1, 7, 6)
}
