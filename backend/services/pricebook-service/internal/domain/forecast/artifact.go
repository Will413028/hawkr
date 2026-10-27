package forecast

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strconv"
	"time"
)

const ArtifactSchemaVersion = "hawkr.price-model/v2"

// HorizonModel mirrors training.py's HorizonModel.to_dict()/from_dict()
// (training.py:175-244). P3 loads it read-only; Go never re-derives or
// re-validates calibration lineage (that belongs to P4's quality gates).
type HorizonModel struct {
	Kind                        string           `json:"kind"`
	SampleCount                 int              `json:"sample_count"`
	CalibrationCount            int              `json:"calibration_count"`
	ResidualQuantiles           [3]float64       `json:"residual_quantiles"`
	FallbackPrice               float64          `json:"fallback_price"`
	SeasonalLagDays             int              `json:"seasonal_lag_days"`
	ProperTrainingCount         int              `json:"proper_training_count"`
	CalibrationMethod           string           `json:"calibration_method"`
	ProperTrainingEndTargetDate *Date            `json:"proper_training_end_target_date"`
	CalibrationStartTargetDate  *Date            `json:"calibration_start_target_date"`
	CalibrationEndTargetDate    *Date            `json:"calibration_end_target_date"`
	Ridge                       *RidgeParameters `json:"ridge"`
}

// SeriesModel mirrors training.py's SeriesModel (training.py:247-289).
// Horizons is keyed by the string "1".."7", matching how
// SeriesModel.to_dict() serializes it (training.py:271).
type SeriesModel struct {
	SeriesID     string                  `json:"series_id"`
	MarketID     string                  `json:"market_id"`
	CropID       string                  `json:"crop_id"`
	CategoryCode *string                 `json:"category_code"`
	Horizons     map[string]HorizonModel `json:"horizons"`
}

// Horizon looks up the trained model for one forecast horizon (1-7).
func (s SeriesModel) Horizon(days int) (HorizonModel, bool) {
	hm, ok := s.Horizons[strconv.Itoa(days)]
	return hm, ok
}

// TrainingConfig mirrors training.py's TrainingConfig (training.py:37-76).
// P3 does not re-derive or re-validate these — they are read-only training
// metadata alongside the coefficients.
type TrainingConfig struct {
	HorizonDays           int           `json:"horizon_days"`
	RidgeAlpha            float64       `json:"ridge_alpha"`
	MinSamplesPerModel    int           `json:"min_samples_per_model"`
	CalibrationFraction   float64       `json:"calibration_fraction"`
	MinCalibrationSamples int           `json:"min_calibration_samples"`
	SeasonalLagDays       int           `json:"seasonal_lag_days"`
	AvailabilityPolicy    string        `json:"availability_policy"`
	FeatureConfig         FeatureConfig `json:"feature_config"`
}

// ModelArtifact mirrors training.py's ModelArtifact (training.py:291-337).
type ModelArtifact struct {
	ArtifactSchemaVersion string                 `json:"artifact_schema_version"`
	ModelVersion          string                 `json:"model_version"`
	TrainedAt             time.Time              `json:"trained_at"`
	DatasetDigest         string                 `json:"dataset_digest"`
	FeatureSchema         []string               `json:"feature_schema"`
	Config                TrainingConfig         `json:"config"`
	Series                map[string]SeriesModel `json:"series"`
	Metadata              map[string]any         `json:"metadata"`
}

// HasSeries reports whether the artifact has a trained model for exactly
// this (market, crop) pair — same series-ID construction SeriesForecast
// itself uses (internal/usecase/forecast.go's "moa:"+market+":"+crop key).
// A pure lookup, used by EstimatedItemPrice as a pre-check to distinguish
// "model doesn't cover this crop" (a business unavailable outcome) from an
// infra failure, without duplicating SeriesForecast's calendar/quality-gate
// logic — see estimated_item_price.go's own doc comment for why that
// logic must not be duplicated.
func (a ModelArtifact) HasSeries(marketCode, cropCode string) bool {
	_, ok := a.Series["moa:"+marketCode+":"+cropCode]
	return ok
}

// HasAnySeriesForCrops reports whether the artifact has at least one
// trained series, in any market, for any of the given crop codes —
// mirrors GroupForecast's own relevantSeries filter (internal/usecase/
// forecast.go, matches series.CropID against "moa:"+cropCode for each
// crop in the group). Used by EstimatedItemPrice as the group-path
// equivalent of HasSeries.
func (a ModelArtifact) HasAnySeriesForCrops(cropCodes []string) bool {
	want := make(map[string]bool, len(cropCodes))
	for _, c := range cropCodes {
		want["moa:"+c] = true
	}
	for _, series := range a.Series {
		if want[series.CropID] {
			return true
		}
	}
	return false
}

// LoadArtifact reads and validates a model.json produced by
// hawkr_price_ai.training.save_artifact (training.py:895-906), mirroring
// load_artifact/ModelArtifact.from_dict's schema-version gate
// (training.py:316-323).
func LoadArtifact(path string) (ModelArtifact, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ModelArtifact{}, fmt.Errorf("forecast: reading %s: %w", path, err)
	}
	var artifact ModelArtifact
	if err := json.Unmarshal(raw, &artifact); err != nil {
		return ModelArtifact{}, fmt.Errorf("forecast: parsing %s: %w", path, err)
	}
	if artifact.ArtifactSchemaVersion != ArtifactSchemaVersion {
		return ModelArtifact{}, fmt.Errorf("forecast: unsupported artifact schema %q in %s; expected %q", artifact.ArtifactSchemaVersion, path, ArtifactSchemaVersion)
	}
	if err := artifact.Config.FeatureConfig.Validate(); err != nil {
		return ModelArtifact{}, fmt.Errorf("forecast: %s has invalid feature_config: %w", path, err)
	}
	if wantNames := artifact.Config.FeatureConfig.FeatureNames(); !slices.Equal(artifact.FeatureSchema, wantNames) {
		return ModelArtifact{}, fmt.Errorf("forecast: %s feature_schema does not match config.feature_config's derived FeatureNames() (got %v, want %v)", path, artifact.FeatureSchema, wantNames)
	}
	return artifact, nil
}
