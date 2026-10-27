package forecast

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadArtifactDemoModel(t *testing.T) {
	artifact, err := LoadArtifact("../../../testdata/parity/model-demo.json")
	if err != nil {
		t.Fatalf("LoadArtifact 失敗: %v", err)
	}
	if artifact.ArtifactSchemaVersion != ArtifactSchemaVersion {
		t.Errorf("ArtifactSchemaVersion = %q，預期 %q", artifact.ArtifactSchemaVersion, ArtifactSchemaVersion)
	}
	if artifact.ModelVersion != "direct-ridge-v2-aa66e30432a2729a" {
		t.Errorf("ModelVersion = %q", artifact.ModelVersion)
	}
	if len(artifact.Series) != 2 {
		t.Fatalf("series 數 = %d，預期 2", len(artifact.Series))
	}
	fc := artifact.Config.FeatureConfig
	if fc.MaxDaysSinceLastTrade != 365 {
		t.Errorf("MaxDaysSinceLastTrade = %d，預期 365", fc.MaxDaysSinceLastTrade)
	}
	wantLags := []int{1, 2, 3, 7, 14, 28}
	if len(fc.PriceLags) != len(wantLags) {
		t.Fatalf("PriceLags 長度 = %d，預期 %d", len(fc.PriceLags), len(wantLags))
	}
	for i, w := range wantLags {
		if fc.PriceLags[i] != w {
			t.Errorf("PriceLags[%d] = %d，預期 %d", i, fc.PriceLags[i], w)
		}
	}

	series, ok := artifact.Series["moa:109:A1"]
	if !ok {
		t.Fatal("series moa:109:A1 不存在")
	}
	hm, ok := series.Horizon(1)
	if !ok {
		t.Fatal("horizon 1 不存在")
	}
	if hm.Kind != "direct_ridge" {
		t.Errorf("Kind = %q，預期 direct_ridge", hm.Kind)
	}
	if hm.SampleCount != 118 {
		t.Errorf("SampleCount = %d，預期 118", hm.SampleCount)
	}
	if hm.CalibrationCount != 24 {
		t.Errorf("CalibrationCount = %d，預期 24", hm.CalibrationCount)
	}
	assertClose(t, "FallbackPrice", hm.FallbackPrice, 63.1, 1e-12)
	if hm.Ridge == nil {
		t.Fatal("Ridge 不應為 nil")
	}
	assertClose(t, "Ridge.Intercept", hm.Ridge.Intercept, 55.93531914893608, 1e-9)
	if len(hm.Ridge.Coefficients) != 36 {
		t.Errorf("Coefficients 長度 = %d，預期 36", len(hm.Ridge.Coefficients))
	}
	if hm.CalibrationStartTargetDate == nil || hm.CalibrationStartTargetDate.String() != "2026-06-25" {
		t.Errorf("CalibrationStartTargetDate 不符: %v", hm.CalibrationStartTargetDate)
	}
}

func TestLoadArtifactFallbackModelHasBothKinds(t *testing.T) {
	artifact, err := LoadArtifact("../../../testdata/parity/model-fallback.json")
	if err != nil {
		t.Fatalf("LoadArtifact 失敗: %v", err)
	}
	if artifact.Config.MinSamplesPerModel != 115 {
		t.Errorf("MinSamplesPerModel = %d，預期 115", artifact.Config.MinSamplesPerModel)
	}
	series := artifact.Series["moa:109:A1"]
	h1, _ := series.Horizon(1)
	if h1.Kind != "direct_ridge" || h1.Ridge == nil {
		t.Errorf("horizon 1 應為 direct_ridge 且有 Ridge，得到 kind=%q ridge=%v", h1.Kind, h1.Ridge)
	}
	h5, _ := series.Horizon(5)
	if h5.Kind != "seasonal_naive" || h5.Ridge != nil {
		t.Errorf("horizon 5 應為 seasonal_naive 且 Ridge 應為 nil，得到 kind=%q ridge=%v", h5.Kind, h5.Ridge)
	}
	if h5.SampleCount != 114 {
		t.Errorf("h5.SampleCount = %d，預期 114", h5.SampleCount)
	}
}

func TestModelArtifact_HasSeries(t *testing.T) {
	artifact := ModelArtifact{Series: map[string]SeriesModel{
		"moa:109:A1": {SeriesID: "moa:109:A1", MarketID: "moa:109", CropID: "moa:A1"},
	}}
	if !artifact.HasSeries("109", "A1") {
		t.Error("HasSeries(109, A1) = false, want true for a present series")
	}
	if artifact.HasSeries("109", "B2") {
		t.Error("HasSeries(109, B2) = true, want false for an absent crop")
	}
	if artifact.HasSeries("999", "A1") {
		t.Error("HasSeries(999, A1) = true, want false for an absent market")
	}
}

func TestModelArtifact_HasAnySeriesForCrops(t *testing.T) {
	artifact := ModelArtifact{Series: map[string]SeriesModel{
		"moa:109:A1": {SeriesID: "moa:109:A1", MarketID: "moa:109", CropID: "moa:A1"},
	}}
	if !artifact.HasAnySeriesForCrops([]string{"B2", "A1"}) {
		t.Error("HasAnySeriesForCrops([B2 A1]) = false, want true — A1 matches")
	}
	if artifact.HasAnySeriesForCrops([]string{"B2", "C3"}) {
		t.Error("HasAnySeriesForCrops([B2 C3]) = true, want false — no crop matches")
	}
	if artifact.HasAnySeriesForCrops(nil) {
		t.Error("HasAnySeriesForCrops(nil) = true, want false for an empty crop list")
	}
}

func TestLoadArtifactRejectsUnsupportedSchemaVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad-model.json")
	if err := os.WriteFile(path, []byte(`{"artifact_schema_version":"hawkr.price-model/v1"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadArtifact(path); err == nil {
		t.Fatal("v1 schema 應回錯誤（mirrors training.py:318-323）")
	}
}

// feature_schema 是 36 維順序契約：RidgeParameters.Predict 用位置消費特徵
// 向量，若 model.json 的 feature_schema（人類可讀名稱）跟
// config.feature_config 實際會產生的 FeatureNames() 對不上，兩個陣列長度
// 仍都是 36，不會觸發長度不符錯誤，ridge 推論會安靜地算出錯誤數字。這裡
// 從真實 fixture 讀出 bytes、竄改 feature_schema，驗證 LoadArtifact 擋得下來。
func TestLoadArtifactRejectsFeatureSchemaMismatch(t *testing.T) {
	raw, err := os.ReadFile("../../../testdata/parity/model-demo.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	schema, ok := doc["feature_schema"].([]any)
	if !ok || len(schema) < 2 {
		t.Fatalf("feature_schema 型別或長度不符預期: %T, len=%d", doc["feature_schema"], len(schema))
	}
	// 反轉順序：長度不變（仍是 36），內容錯位，剛好對應 finding 描述的
	// 「長度相同、順序對不上」情境。
	for i, j := 0, len(schema)-1; i < j; i, j = i+1, j-1 {
		schema[i], schema[j] = schema[j], schema[i]
	}
	doc["feature_schema"] = schema
	mutated, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "mismatched-model.json")
	if err := os.WriteFile(path, mutated, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadArtifact(path); err == nil {
		t.Fatal("feature_schema 與 FeatureNames() 不一致時應回錯誤，實際為 nil")
	}
}
