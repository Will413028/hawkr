package forecast

import (
	"fmt"
	"math"
)

// DefaultSkewZThreshold 是判定「這一維系統性偏移」的 |z| 門檻。
//
// 3 是常見的離群值門檻，5 才是「這一維幾乎確定對不上」。用 5：這個報告的
// 用途是抓語意差（修訂選取、缺值處理、單位），不是抓個別離群觀測，而
// 農產品價格本身就有 3-sigma 級的正常波動。Level 0 ADR
// （wiki ADR 2026-07-29-pricebook-python-boundary-level-0）明講這個判準是
// 經驗值、不像原本的逐筆對帳 gate 那樣預先承諾。
const DefaultSkewZThreshold = 5.0

// FeatureDimensionHealth 是單一特徵維度的 z 分佈摘要。
type FeatureDimensionHealth struct {
	Index   int
	Name    string
	MeanZ   float64 // 我方樣本標準化後的平均 z
	MaxAbsZ float64
	SampleN int  // 這一維實際貢獻到 MeanZ/MaxAbsZ 的樣本數（< VectorN 代表有樣本因 feature_scale==0 被跳過，見下方 FeatureHealth 註解）
	Suspect bool // |MeanZ| > threshold，或這一維在任何樣本上遇到 feature_scale==0
}

// FeatureHealthReport 是對單一 (series, horizon) 的健檢結果。
//
// 只收單一 horizon：每個 (series, horizon) 各有自己的一組
// feature_mean/feature_scale（training.py 逐 horizon 各訓練一個 ridge），
// 門檻又是逐維判定——把 7 個 horizon 的 z 混在一起算平均，不同 horizon
// 方向相反的偏移會互相抵銷，掩蓋真正的 skew。呼叫端（cmd/pricebook-feature-health）
// 對 7 個 horizon 各呼叫一次 FeatureHealth，各自報告、互不混算。
type FeatureHealthReport struct {
	SeriesID    string
	HorizonDays int
	VectorN     int
	Threshold   float64
	Dimensions  []FeatureDimensionHealth
	SuspectN    int
}

// FeatureHealth 拿 artifact 裡 (seriesID, horizonDays) 這組 ridge 的
// feature_mean/feature_scale，對 vectors 逐維算標準化後的 z，回傳每一維的
// 平均值／最大絕對值／是否可疑。
//
// z 的公式必須與 RidgeParameters.Predict 內的標準化是同一個算法
// （ridge.go:32-36：center += ((x - FeatureMean[i]) / FeatureScale[i]) *
// Coefficients[i]）——這裡只借用同一個 (x-mean)/scale 標準化步驟，不重新發明
// 一種度量。健檢量的就是 Predict 實際吃到的那個 z，這樣「z 分佈正常」才等於
// 「Predict 吃到的輸入分佈跟訓練時一致」。
//
// feature_scale 已經是訓練期保護過的值（上游 baseline.py:70 的
// scale = np.where(scale < 1e-12, 1.0, scale)，model.json 存的就是保護後的
// 值——同 ridge.go:21-24 對 Predict 的說明），FeatureHealth 因此不重新套用
// 一次保護。但這個函式不信任輸入一定滿足這個前提：真的遇到某一維
// feature_scale==0，會跳過那個樣本對該維的貢獻（不做除以零），並把
// SampleN 壓到低於 VectorN、同時把該維標成 Suspect——不會靜默吃進 NaN／Inf
// 污染其他維的統計，也不會假裝那一維「看起來正常」。
//
// 只有 kind=="direct_ridge" 的 horizon 有 Ridge（seasonal_naive 的 Ridge 是
// nil）；遇到 nil 回傳 error 而不是 panic，呼叫端據此跳過並在輸出說明。
func FeatureHealth(artifact ModelArtifact, vectors [][]float64, seriesID string, horizonDays int, threshold float64) (FeatureHealthReport, error) {
	series, ok := artifact.Series[seriesID]
	if !ok {
		return FeatureHealthReport{}, fmt.Errorf("forecast: artifact has no series %q", seriesID)
	}
	hm, ok := series.Horizon(horizonDays)
	if !ok {
		return FeatureHealthReport{}, fmt.Errorf("forecast: series %q has no horizon %d (must be 1-7)", seriesID, horizonDays)
	}
	if hm.Ridge == nil {
		return FeatureHealthReport{}, fmt.Errorf("forecast: series %q horizon %d has kind %q, not direct_ridge — no ridge parameters to health-check", seriesID, horizonDays, hm.Kind)
	}
	ridge := *hm.Ridge
	dims := len(ridge.Coefficients)
	if len(ridge.FeatureMean) != dims || len(ridge.FeatureScale) != dims {
		return FeatureHealthReport{}, fmt.Errorf("forecast: ridge feature_mean/feature_scale/coefficients length mismatch (%d/%d/%d) for series %q horizon %d", len(ridge.FeatureMean), len(ridge.FeatureScale), dims, seriesID, horizonDays)
	}
	names := artifact.FeatureSchema
	if len(names) != dims {
		return FeatureHealthReport{}, fmt.Errorf("forecast: feature_schema has %d names, ridge for series %q horizon %d trained on %d dims", len(names), seriesID, horizonDays, dims)
	}

	sums := make([]float64, dims)
	maxAbs := make([]float64, dims)
	sampleN := make([]int, dims)

	for vi, v := range vectors {
		if len(v) != dims {
			return FeatureHealthReport{}, fmt.Errorf("forecast: vectors[%d] has %d dims, ridge trained on %d", vi, len(v), dims)
		}
		for i, x := range v {
			scale := ridge.FeatureScale[i]
			if scale == 0 {
				// 防禦性分支：見上方函式註解，理論上不該發生（訓練期已保護），
				// 發生時跳過這個樣本對這一維的貢獻，不除以零。
				continue
			}
			z := (x - ridge.FeatureMean[i]) / scale
			sums[i] += z
			if abs := math.Abs(z); abs > maxAbs[i] {
				maxAbs[i] = abs
			}
			sampleN[i]++
		}
	}

	report := FeatureHealthReport{
		SeriesID:    seriesID,
		HorizonDays: horizonDays,
		VectorN:     len(vectors),
		Threshold:   threshold,
		Dimensions:  make([]FeatureDimensionHealth, dims),
	}
	for i := 0; i < dims; i++ {
		var meanZ float64
		if sampleN[i] > 0 {
			meanZ = sums[i] / float64(sampleN[i])
		}
		degenerateScale := len(vectors) > 0 && sampleN[i] < len(vectors)
		suspect := degenerateScale || math.Abs(meanZ) > threshold
		report.Dimensions[i] = FeatureDimensionHealth{
			Index: i, Name: names[i], MeanZ: meanZ, MaxAbsZ: maxAbs[i], SampleN: sampleN[i], Suspect: suspect,
		}
		if suspect {
			report.SuspectN++
		}
	}
	return report, nil
}
