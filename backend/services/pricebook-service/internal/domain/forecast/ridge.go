package forecast

import "fmt"

// RidgeParameters is the trained linear model for one (series, horizon),
// mirroring baseline.py's RidgeParameters (baseline.py:11-16). It is
// inference-only: Go never fits a ridge model (that stays in Python — see
// baseline.py:49-86's fit_ridge, a training-time function); Go only replays
// baseline.py:18-29's predict() against coefficients already trained and
// stored in model.json.
type RidgeParameters struct {
	Intercept    float64   `json:"intercept"`
	Coefficients []float64 `json:"coefficients"`
	FeatureMean  []float64 `json:"feature_mean"`
	FeatureScale []float64 `json:"feature_scale"`
}

// Predict ports RidgeParameters.predict (baseline.py:18-29) for the
// single-row case — P3 only ever predicts one (series, horizon) at a time,
// so the batched 2D branch in the Python original is not ported.
// feature_scale is assumed to already carry baseline.py:70's
// `scale = np.where(scale < 1e-12, 1.0, scale)` training-time protection —
// it does, model.json stores the post-protection value — so Predict does
// not re-apply it.
func (r RidgeParameters) Predict(features []float64) (float64, error) {
	if len(features) != len(r.Coefficients) {
		return 0, fmt.Errorf("forecast: feature vector has %d dims, ridge trained on %d", len(features), len(r.Coefficients))
	}
	if len(r.FeatureMean) != len(r.Coefficients) || len(r.FeatureScale) != len(r.Coefficients) {
		return 0, fmt.Errorf("forecast: ridge feature_mean/feature_scale/coefficients length mismatch (%d/%d/%d)", len(r.FeatureMean), len(r.FeatureScale), len(r.Coefficients))
	}
	center := r.Intercept
	for i, x := range features {
		center += ((x - r.FeatureMean[i]) / r.FeatureScale[i]) * r.Coefficients[i]
	}
	return center, nil
}
