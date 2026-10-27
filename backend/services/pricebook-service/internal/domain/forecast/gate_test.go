package forecast

import (
	"math"
	"path/filepath"
	"testing"
	"time"
)

func dateptr(y, m, d int) *Date { v := NewDate(y, m, d); return &v }

// validRidgeHorizon 是一個血緣完全一致的 direct_ridge horizon，數值取自
// testdata/parity/model-demo.json 的 moa:109:A1 h1（sample_count 118 =
// proper_training_count 94 + calibration_count 24）。
func validRidgeHorizon() HorizonModel {
	return HorizonModel{
		Kind:                        "direct_ridge",
		SampleCount:                 118,
		CalibrationCount:            24,
		ProperTrainingCount:         94,
		CalibrationMethod:           "chronological_tail_strict_split",
		ProperTrainingEndTargetDate: dateptr(2026, 6, 24),
		CalibrationStartTargetDate:  dateptr(2026, 6, 25),
		CalibrationEndTargetDate:    dateptr(2026, 7, 18),
		ResidualQuantiles:           [3]float64{-0.93, 0.16, 1.16},
		FallbackPrice:               63.1,
		SeasonalLagDays:             7,
		Ridge:                       &RidgeParameters{Intercept: 1, Coefficients: []float64{1}, FeatureMean: []float64{0}, FeatureScale: []float64{1}},
	}
}

func validSeasonalHorizon() HorizonModel {
	return HorizonModel{
		Kind:                       "seasonal_naive",
		SampleCount:                112,
		CalibrationCount:           112,
		ProperTrainingCount:        0,
		CalibrationMethod:          "rolling_point_in_time_pseudo_residuals",
		CalibrationStartTargetDate: dateptr(2026, 3, 1),
		CalibrationEndTargetDate:   dateptr(2026, 7, 18),
		ResidualQuantiles:          [3]float64{-2, 0, 2},
		FallbackPrice:              64.4,
		SeasonalLagDays:            7,
	}
}

func TestCalibrationGateReason(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*HorizonModel)
		base   func() HorizonModel
		want   MissingReason
	}{
		{"ridge 血緣完整 → 過關", func(*HorizonModel) {}, validRidgeHorizon, ""},
		{"seasonal 血緣完整 → 過關", func(*HorizonModel) {}, validSeasonalHorizon, ""},
		{"calibration_count 低於門檻",
			func(m *HorizonModel) { m.CalibrationCount = 6; m.ProperTrainingCount = 112 },
			validRidgeHorizon, MissingReasonInsufficientCalibrationData},
		{"ridge 用了 seasonal 的 calibration_method",
			func(m *HorizonModel) { m.CalibrationMethod = "rolling_point_in_time_pseudo_residuals" },
			validRidgeHorizon, MissingReasonInvalidCalibrationLineage},
		{"seasonal 用了 ridge 的 calibration_method",
			func(m *HorizonModel) { m.CalibrationMethod = "chronological_tail_strict_split" },
			validSeasonalHorizon, MissingReasonInvalidCalibrationLineage},
		{"kind=direct_ridge 但沒有 ridge 參數",
			func(m *HorizonModel) { m.Ridge = nil },
			validRidgeHorizon, MissingReasonInvalidCalibrationLineage},
		{"proper_training_count < 2",
			func(m *HorizonModel) { m.ProperTrainingCount = 1; m.SampleCount = 25 },
			validRidgeHorizon, MissingReasonInvalidCalibrationLineage},
		{"proper_training + calibration != sample_count",
			func(m *HorizonModel) { m.SampleCount = 119 },
			validRidgeHorizon, MissingReasonInvalidCalibrationLineage},
		{"訓練尾端沒有早於 calibration 起點（look-ahead）",
			func(m *HorizonModel) { m.ProperTrainingEndTargetDate = dateptr(2026, 6, 25) },
			validRidgeHorizon, MissingReasonInvalidCalibrationLineage},
		{"calibration 區間反轉",
			func(m *HorizonModel) { m.CalibrationStartTargetDate = dateptr(2026, 7, 19) },
			validRidgeHorizon, MissingReasonInvalidCalibrationLineage},
		{"ridge 缺 proper_training_end_target_date",
			func(m *HorizonModel) { m.ProperTrainingEndTargetDate = nil },
			validRidgeHorizon, MissingReasonInvalidCalibrationLineage},
		{"seasonal 竟然帶了 ridge 參數",
			func(m *HorizonModel) {
				m.Ridge = &RidgeParameters{Intercept: 1, Coefficients: []float64{1}, FeatureMean: []float64{0}, FeatureScale: []float64{1}}
			},
			validSeasonalHorizon, MissingReasonInvalidCalibrationLineage},
		{"seasonal 的 proper_training_count 不是 0",
			func(m *HorizonModel) { m.ProperTrainingCount = 3 },
			validSeasonalHorizon, MissingReasonInvalidCalibrationLineage},
		{"seasonal 的 calibration_count != sample_count",
			func(m *HorizonModel) { m.CalibrationCount = 111 },
			validSeasonalHorizon, MissingReasonInvalidCalibrationLineage},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := tc.base()
			tc.mutate(&m)
			// minCalibrationSamples 用 demo artifact 的實際值 7
			if got := CalibrationGateReason(m, 7); got != tc.want {
				t.Errorf("CalibrationGateReason = %q，預期 %q", got, tc.want)
			}
		})
	}
}

func TestCalibrationCountGateTakesPrecedenceOverLineage(t *testing.T) {
	// 上游的順序是先查 calibration_count 再查血緣（forecasting.py:126-133），
	// 兩者同時壞掉時回報的是前者。順序反了會讓「資料不足」被誤報成「artifact 損壞」。
	m := validRidgeHorizon()
	m.CalibrationCount = 1
	m.CalibrationMethod = "bogus"
	if got := CalibrationGateReason(m, 7); got != MissingReasonInsufficientCalibrationData {
		t.Errorf("CalibrationGateReason = %q，預期 %q（calibration_count 的檢查在血緣之前）", got, MissingReasonInsufficientCalibrationData)
	}
}

func predictedPoint() SeriesPoint {
	p10, p50, p90 := 40.0, 50.0, 60.0
	return SeriesPoint{
		SeriesID: "moa:109:LA1", MarketID: "moa:109", CropID: "moa:LA1",
		HorizonDays: 1, TargetDate: NewDate(2026, 7, 19),
		CalendarStatus: CalendarStatusVerifiedOpen, MarketStatus: MarketStatusOpen,
		PointStatus: PointStatusPredicted, ModelKind: ModelKindDirectRidge,
		P10: &p10, P50: &p50, P90: &p90,
		ModelVersion:  "direct-ridge-v2-aa66e30432a2729a",
		DatasetDigest: "02b9fd8cef27a5bff8a4be4ebf846f8e7d4e7f12617cda2ba6556ad7c740fffe",
		SampleCount:   118, CalibrationCount: 24,
		TargetSemantics: TargetSemanticsPriceGivenTrade,
	}
}

func TestEvaluatePointGates(t *testing.T) {
	cfg := GateConfig{MinRidgeSampleCount: 100}

	for _, tc := range []struct {
		name   string
		mutate func(*SeriesPoint)
		want   MissingReason
	}{
		{"正常點 → 過關", func(*SeriesPoint) {}, ""},
		{"p50 = 0 → 擋（上游沒有這條，0 元建議售價會讓成本公式除出荒謬結果）",
			func(p *SeriesPoint) { p.P10, p.P50, p.P90 = price(0), price(0), price(0) }, MissingReasonNonPositiveP50},
		{"p50 為 NaN → 擋",
			func(p *SeriesPoint) { p.P50 = price(math.NaN()) }, MissingReasonNonFinitePrice},
		{"p90 為 +Inf → 擋",
			func(p *SeriesPoint) { p.P90 = price(math.Inf(1)) }, MissingReasonNonFinitePrice},
		{"分位數非單調 → 擋",
			func(p *SeriesPoint) { p.P10 = price(55) }, MissingReasonQuantilesNotMonotonic},
		{"model_version 空 → 擋",
			func(p *SeriesPoint) { p.ModelVersion = "" }, MissingReasonModelLineageMissing},
		{"dataset_digest 空 → 擋",
			func(p *SeriesPoint) { p.DatasetDigest = "" }, MissingReasonModelLineageMissing},
		{"dataset_digest 不是 64 位 hex → 擋（上游 postgres_publication.py:329 同一條）",
			func(p *SeriesPoint) { p.DatasetDigest = "abc" }, MissingReasonModelLineageMissing},
		{"ridge 樣本量低於門檻 → 擋（§14 line 558，N_7 < 100 時 ridge 比 naive 更差）",
			func(p *SeriesPoint) { p.SampleCount = 99 }, MissingReasonBelowRidgeSampleThreshold},
		{"ridge 樣本量剛好等於門檻 → 過關",
			func(p *SeriesPoint) { p.SampleCount = 100 }, ""},
		{"seasonal_naive 不受 ridge 樣本量門檻約束",
			func(p *SeriesPoint) { p.ModelKind = ModelKindSeasonalNaive; p.SampleCount = 12 }, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := predictedPoint()
			tc.mutate(&p)
			got := EvaluatePointGates(p, cfg)
			if got.Reason != tc.want || got.Passed != (tc.want == "") {
				t.Errorf("EvaluatePointGates = {Passed:%v Reason:%q}，預期 Reason=%q", got.Passed, got.Reason, tc.want)
			}
		})
	}
}

func TestEvaluatePointGatesSkipsPriceChecksOnPointsWithoutPrices(t *testing.T) {
	// 沒有價格的點（休市、calibration gate、近期無資料）不該被價格閘門再擋一次——
	// 它們的 missing_reason 已經是最精確的解釋，被覆寫成 non_positive_p50
	// 會讓「今天沒開市」變成「價格異常」。
	for _, status := range []PointStatus{PointStatusNotApplicable, PointStatusNotPredicted} {
		p := predictedPoint()
		p.PointStatus = status
		p.P10, p.P50, p.P90 = nil, nil, nil
		if got := EvaluatePointGates(p, GateConfig{MinRidgeSampleCount: 100}); !got.Passed {
			t.Errorf("status=%q 的無價格點被擋下（Reason=%q）", status, got.Reason)
		}
	}
}

func TestEvaluateHorizonSet(t *testing.T) {
	mk := func(horizons ...int) []SeriesPoint {
		out := make([]SeriesPoint, 0, len(horizons))
		for _, h := range horizons {
			p := predictedPoint()
			p.HorizonDays = h
			p.TargetDate = NewDate(2026, 7, 18).AddDays(h)
			out = append(out, p)
		}
		return out
	}
	if got := EvaluateHorizonSet(mk(1, 2, 3, 4, 5, 6, 7)); !got.Passed {
		t.Errorf("完整七日被擋: %q", got.Reason)
	}
	for _, tc := range []struct {
		name  string
		input []SeriesPoint
	}{
		{"缺一日", mk(1, 2, 3, 4, 5, 6)},
		{"重複", mk(1, 2, 3, 4, 5, 6, 6)},
		{"多一日", mk(1, 2, 3, 4, 5, 6, 7, 8)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := EvaluateHorizonSet(tc.input)
			if got.Passed || got.Reason != MissingReasonHorizonSetIncomplete {
				t.Errorf("EvaluateHorizonSet = {Passed:%v Reason:%q}，預期擋下並回 %q", got.Passed, got.Reason, MissingReasonHorizonSetIncomplete)
			}
		})
	}
}

func TestEvaluateHorizonSetCatchesDuplicateTargetDates(t *testing.T) {
	// horizon 1..7 齊全但 target_date 撞在一起——這是大類加權 merge 多個子品項時
	// 唯一會出現的形狀（SevenDays 的 originDate.AddDays(h) 結構上不可能撞）。
	points := make([]SeriesPoint, 0, 7)
	for h := 1; h <= 7; h++ {
		p := predictedPoint()
		p.HorizonDays = h
		p.TargetDate = NewDate(2026, 7, 19) // 全部同一天
		points = append(points, p)
	}
	if got := EvaluateHorizonSet(points); got.Passed {
		t.Error("七個 horizon 指向同一個 target_date 必須擋下")
	}
}

// TestParityFixturesPassAllGatesAtDefaultThreshold 是回歸防線：
// MIN_RIDGE_SAMPLE_COUNT 的預設值若訂得太高，會讓既有 28 case 的 parity
// fixture 全部被擋成 insufficient_data，那時候編排層 parity 仍然綠（它直接
// 呼叫 SevenDays、不過閘門），但整條線上路徑會安靜地什麼都不出。
func TestParityFixturesPassAllGatesAtDefaultThreshold(t *testing.T) {
	history := loadParityHistory(t)
	closures, verified := loadParityCalendar(t)
	for _, f := range []struct{ fixture, model string }{
		{"parity-fixture.json", "model-demo.json"},
		{"parity-fixture-fallback.json", "model-fallback.json"},
	} {
		t.Run(f.model, func(t *testing.T) {
			artifact, err := LoadArtifact(filepath.Join(parityTestdataDir, f.model))
			if err != nil {
				t.Fatalf("LoadArtifact: %v", err)
			}
			origin := time.Date(2026, 7, 18, 15, 45, 0, 0, time.UTC)
			points, err := SevenDays(SevenDaysInput{
				Artifact: artifact, HistoryBySeries: history, Origin: origin,
				Closures: closures, Verified: verified,
			})
			if err != nil {
				t.Fatalf("SevenDays: %v", err)
			}
			cfg := GateConfig{MinRidgeSampleCount: DefaultMinRidgeSampleCount}
			for _, p := range points {
				if v := EvaluatePointGates(p, cfg); !v.Passed {
					t.Errorf("%s D+%d 被閘門擋下（Reason=%q, ModelKind=%q, SampleCount=%d）——"+
						"demo/fallback artifact 的 sample_count 是 112~118，預設門檻 %d 不該擋到它們",
						p.SeriesID, p.HorizonDays, v.Reason, p.ModelKind, p.SampleCount, DefaultMinRidgeSampleCount)
				}
			}
		})
	}
}
