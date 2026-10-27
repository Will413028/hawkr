package forecast

import (
	"math"
	"strconv"
	"testing"
)

// artifactWithHorizon builds a minimal ModelArtifact with exactly one
// (series, horizon) so tests can hand-construct RidgeParameters without
// going through LoadArtifact's JSON/schema-version machinery — FeatureHealth
// operates on the in-memory ModelArtifact struct, not on a file.
func artifactWithHorizon(seriesID string, horizonDays int, hm HorizonModel, featureSchema []string) ModelArtifact {
	return ModelArtifact{
		FeatureSchema: featureSchema,
		Series: map[string]SeriesModel{
			seriesID: {
				SeriesID: seriesID,
				Horizons: map[string]HorizonModel{
					strconv.Itoa(horizonDays): hm,
				},
			},
		},
	}
}

func directRidgeHorizon(mean, scale []float64) HorizonModel {
	coefficients := make([]float64, len(mean))
	return HorizonModel{
		Kind: "direct_ridge",
		Ridge: &RidgeParameters{
			Intercept:    0,
			Coefficients: coefficients,
			FeatureMean:  mean,
			FeatureScale: scale,
		},
	}
}

func assertNoNaNOrInf(t *testing.T, report FeatureHealthReport) {
	t.Helper()
	for _, d := range report.Dimensions {
		if math.IsNaN(d.MeanZ) || math.IsInf(d.MeanZ, 0) {
			t.Errorf("dimension %d (%s): MeanZ is NaN/Inf: %v", d.Index, d.Name, d.MeanZ)
		}
		if math.IsNaN(d.MaxAbsZ) || math.IsInf(d.MaxAbsZ, 0) {
			t.Errorf("dimension %d (%s): MaxAbsZ is NaN/Inf: %v", d.Index, d.Name, d.MaxAbsZ)
		}
	}
}

// Case 1: 完全落在訓練分佈中心 —— MeanZ ≈ 0，Suspect 全 false。
func TestFeatureHealthVectorsAtTrainingCenterAreNotSuspect(t *testing.T) {
	mean := []float64{50.0, 10.0, 0.0}
	scale := []float64{5.0, 2.0, 1.0}
	names := []string{"price_lag_1", "volume_lag_1", "price_lag_1_missing"}
	artifact := artifactWithHorizon("moa:109:A1", 3, directRidgeHorizon(mean, scale), names)

	vectors := [][]float64{
		{50.0, 10.0, 0.0},
		{50.0, 10.0, 0.0},
		{50.0, 10.0, 0.0},
	}

	report, err := FeatureHealth(artifact, vectors, "moa:109:A1", 3, DefaultSkewZThreshold)
	if err != nil {
		t.Fatalf("FeatureHealth 回錯誤: %v", err)
	}
	if report.VectorN != 3 {
		t.Errorf("VectorN = %d，預期 3", report.VectorN)
	}
	if report.SuspectN != 0 {
		t.Errorf("SuspectN = %d，預期 0（所有樣本都在訓練中心）", report.SuspectN)
	}
	for _, d := range report.Dimensions {
		if d.Suspect {
			t.Errorf("dimension %d (%s) 不應被標 Suspect", d.Index, d.Name)
		}
		if math.Abs(d.MeanZ) > 1e-12 {
			t.Errorf("dimension %d (%s): MeanZ = %v，預期 ≈0", d.Index, d.Name, d.MeanZ)
		}
		if d.SampleN != 3 {
			t.Errorf("dimension %d (%s): SampleN = %d，預期 3", d.Index, d.Name, d.SampleN)
		}
	}
	assertNoNaNOrInf(t, report)
}

// Case 2: 單一維系統性偏移 10 個 sigma —— 只有那一維 Suspect。
func TestFeatureHealthSingleDimensionShiftIsSuspectAlone(t *testing.T) {
	mean := []float64{50.0, 10.0, 0.0}
	scale := []float64{5.0, 2.0, 1.0}
	names := []string{"price_lag_1", "volume_lag_1", "price_lag_1_missing"}
	artifact := artifactWithHorizon("moa:109:A1", 1, directRidgeHorizon(mean, scale), names)

	// dim0: x = 50 + 10*5 = 100 → z = 10（超過門檻）。dim1/dim2 都在訓練中心。
	vectors := [][]float64{
		{100.0, 10.0, 0.0},
		{100.0, 10.0, 0.0},
	}

	report, err := FeatureHealth(artifact, vectors, "moa:109:A1", 1, DefaultSkewZThreshold)
	if err != nil {
		t.Fatalf("FeatureHealth 回錯誤: %v", err)
	}
	if report.SuspectN != 1 {
		t.Fatalf("SuspectN = %d，預期 1", report.SuspectN)
	}
	if !report.Dimensions[0].Suspect {
		t.Errorf("dim0 應被標 Suspect（MeanZ = %v）", report.Dimensions[0].MeanZ)
	}
	if math.Abs(report.Dimensions[0].MeanZ-10.0) > 1e-9 {
		t.Errorf("dim0 MeanZ = %v，預期 10.0", report.Dimensions[0].MeanZ)
	}
	for _, d := range report.Dimensions[1:] {
		if d.Suspect {
			t.Errorf("dim%d (%s) 不應被標 Suspect（MeanZ = %v）", d.Index, d.Name, d.MeanZ)
		}
	}
	assertNoNaNOrInf(t, report)
}

// Case 3: feature_scale 為 1.0 的常數維（model-demo.json 的 price_lag_1_missing，
// index 4：feature_scale=1.0、coefficients=0.0）—— 不得產出 NaN/Inf。
func TestFeatureHealthConstantDimensionScaleOneDoesNotProduceNaNOrInf(t *testing.T) {
	mean := []float64{55.8, 0.0} // dim1 mirrors price_lag_1_missing: mean=0.0, scale=1.0
	scale := []float64{2.59, 1.0}
	names := []string{"price_lag_1", "price_lag_1_missing"}
	artifact := artifactWithHorizon("moa:109:A1", 1, directRidgeHorizon(mean, scale), names)

	// dim1 從沒缺過值，樣本永遠是 0.0（missing flag 永遠不觸發）。
	vectors := [][]float64{
		{56.0, 0.0},
		{54.5, 0.0},
		{55.2, 0.0},
	}

	report, err := FeatureHealth(artifact, vectors, "moa:109:A1", 1, DefaultSkewZThreshold)
	if err != nil {
		t.Fatalf("FeatureHealth 回錯誤: %v", err)
	}
	assertNoNaNOrInf(t, report)
	constDim := report.Dimensions[1]
	if constDim.Name != "price_lag_1_missing" {
		t.Fatalf("dim1 名稱 = %q，預期 price_lag_1_missing", constDim.Name)
	}
	if constDim.MeanZ != 0.0 || constDim.MaxAbsZ != 0.0 {
		t.Errorf("常數維 MeanZ=%v MaxAbsZ=%v，預期皆為 0（x 恆等於 mean）", constDim.MeanZ, constDim.MaxAbsZ)
	}
	if constDim.Suspect {
		t.Errorf("常數維不應被標 Suspect")
	}
}

// Case 4: 空輸入 —— 回 VectorN=0，不除以零，不產生 NaN。
func TestFeatureHealthEmptyVectorsReturnsZeroVectorNWithoutDivideByZero(t *testing.T) {
	mean := []float64{50.0, 0.0}
	scale := []float64{5.0, 1.0}
	names := []string{"price_lag_1", "price_lag_1_missing"}
	artifact := artifactWithHorizon("moa:109:A1", 1, directRidgeHorizon(mean, scale), names)

	report, err := FeatureHealth(artifact, nil, "moa:109:A1", 1, DefaultSkewZThreshold)
	if err != nil {
		t.Fatalf("FeatureHealth 回錯誤: %v", err)
	}
	if report.VectorN != 0 {
		t.Errorf("VectorN = %d，預期 0", report.VectorN)
	}
	if report.SuspectN != 0 {
		t.Errorf("SuspectN = %d，預期 0（沒有樣本可判斷）", report.SuspectN)
	}
	for _, d := range report.Dimensions {
		if d.SampleN != 0 {
			t.Errorf("dim%d SampleN = %d，預期 0", d.Index, d.SampleN)
		}
		if d.Suspect {
			t.Errorf("dim%d 空輸入不應被標 Suspect", d.Index)
		}
	}
	assertNoNaNOrInf(t, report)
}

// feature_scale == 0（違反訓練期保護的防禦性分支）—— 不得除以零／產生
// NaN/Inf，該維的樣本被跳過（SampleN < VectorN）並整體標記 Suspect，讓人
// 看得見「這一維算不出可信的 z」，而不是靜默吞掉。
func TestFeatureHealthFeatureScaleZeroDoesNotDivideByZero(t *testing.T) {
	mean := []float64{50.0, 10.0}
	scale := []float64{0.0, 2.0} // dim0 broken
	names := []string{"broken_dim", "volume_lag_1"}
	artifact := artifactWithHorizon("moa:109:A1", 1, directRidgeHorizon(mean, scale), names)

	vectors := [][]float64{
		{999.0, 10.0},
		{-999.0, 10.0},
	}

	report, err := FeatureHealth(artifact, vectors, "moa:109:A1", 1, DefaultSkewZThreshold)
	if err != nil {
		t.Fatalf("FeatureHealth 回錯誤: %v", err)
	}
	assertNoNaNOrInf(t, report)
	broken := report.Dimensions[0]
	if broken.SampleN != 0 {
		t.Errorf("broken_dim SampleN = %d，預期 0（feature_scale==0 的樣本全被跳過）", broken.SampleN)
	}
	if !broken.Suspect {
		t.Errorf("broken_dim 應被標 Suspect（feature_scale==0 無法判斷）")
	}
	if broken.MeanZ != 0.0 || broken.MaxAbsZ != 0.0 {
		t.Errorf("broken_dim MeanZ=%v MaxAbsZ=%v，預期皆為 0（沒有任何樣本貢獻）", broken.MeanZ, broken.MaxAbsZ)
	}
	healthy := report.Dimensions[1]
	if healthy.SampleN != 2 {
		t.Errorf("volume_lag_1 SampleN = %d，預期 2（未受 dim0 影響）", healthy.SampleN)
	}
	if healthy.Suspect {
		t.Errorf("volume_lag_1 不應被標 Suspect")
	}
}

// 只有 kind=="direct_ridge" 的 horizon 有 Ridge；seasonal_naive 的 Ridge 是
// nil，遇到要回 error 而不是 panic。
func TestFeatureHealthReturnsErrorWhenHorizonHasNoRidge(t *testing.T) {
	artifact := artifactWithHorizon("moa:109:A1", 1, HorizonModel{Kind: "seasonal_naive", Ridge: nil}, []string{"x"})

	_, err := FeatureHealth(artifact, [][]float64{{1.0}}, "moa:109:A1", 1, DefaultSkewZThreshold)
	if err == nil {
		t.Fatal("seasonal_naive horizon（Ridge=nil）應回錯誤，不應 panic 或靜默通過")
	}
}

func TestFeatureHealthReturnsErrorWhenSeriesMissing(t *testing.T) {
	artifact := artifactWithHorizon("moa:109:A1", 1, directRidgeHorizon([]float64{0}, []float64{1}), []string{"x"})

	_, err := FeatureHealth(artifact, [][]float64{{1.0}}, "moa:999:ZZ", 1, DefaultSkewZThreshold)
	if err == nil {
		t.Fatal("artifact 沒有這條 series 時應回錯誤")
	}
}

func TestFeatureHealthReturnsErrorWhenHorizonMissing(t *testing.T) {
	artifact := artifactWithHorizon("moa:109:A1", 1, directRidgeHorizon([]float64{0}, []float64{1}), []string{"x"})

	_, err := FeatureHealth(artifact, [][]float64{{1.0}}, "moa:109:A1", 5, DefaultSkewZThreshold)
	if err == nil {
		t.Fatal("series 沒有這個 horizon 時應回錯誤")
	}
}

func TestFeatureHealthReturnsErrorWhenVectorDimsMismatch(t *testing.T) {
	artifact := artifactWithHorizon("moa:109:A1", 1, directRidgeHorizon([]float64{0, 0}, []float64{1, 1}), []string{"a", "b"})

	_, err := FeatureHealth(artifact, [][]float64{{1.0}}, "moa:109:A1", 1, DefaultSkewZThreshold)
	if err == nil {
		t.Fatal("向量維度與 ridge 訓練維度不符時應回錯誤")
	}
}
