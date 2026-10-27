package forecast

import "testing"

// 手算範例：intercept=1, coefficients=[2,-1], feature_mean=[0,0],
// feature_scale=[1,1], features=[3,4]
// → center = 1 + (3-0)/1*2 + (4-0)/1*(-1) = 1 + 6 - 4 = 3
func TestRidgeParametersPredictHandComputedExample(t *testing.T) {
	r := RidgeParameters{
		Intercept:    1.0,
		Coefficients: []float64{2.0, -1.0},
		FeatureMean:  []float64{0.0, 0.0},
		FeatureScale: []float64{1.0, 1.0},
	}
	got, err := r.Predict([]float64{3.0, 4.0})
	if err != nil {
		t.Fatalf("Predict 回錯誤: %v", err)
	}
	assertClose(t, "predict", got, 3.0, 1e-12)
}

func TestRidgeParametersPredictRejectsLengthMismatch(t *testing.T) {
	r := RidgeParameters{Intercept: 0, Coefficients: []float64{1, 2, 3}, FeatureMean: []float64{0, 0, 0}, FeatureScale: []float64{1, 1, 1}}
	if _, err := r.Predict([]float64{1, 2}); err == nil {
		t.Fatal("特徵維度與係數維度不符應回錯誤")
	}
}
