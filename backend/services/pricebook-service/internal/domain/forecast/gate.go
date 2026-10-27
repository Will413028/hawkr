package forecast

import "math"

// MissingReason 是「這個點為什麼沒有價格」的原因碼。
//
// 前 5 個逐字沿用上游 forecasting.py 的字面值（移植對象，改動會讓 parity test
// 失去比對基準）；其後是 P4 自訂的品質閘門原因（設計書 §8 line 425-427，上游
// 沒有對應概念）。
//
// ⚠️ 這組值將於 Plan C 隨 proto 進 CodeMeta（設計書 §11 line 489）；屆時 SSOT
// 移到 docs/代碼管理（CodeMeta）/，此處改為 hand-mirror（比照
// gateway/internal/codemeta/codemeta.go 與
// services/seller-service/internal/domain/seller_suspension.go 的既有做法）。
// P4 不註冊的理由：CodeMeta README:53-54 的判準是「跨服務／前端會依該值分支
// 才納管」，而 P4 沒有任何值跨出服務邊界（無 proto、無 HTTP 回應、無 MQ）——
// 同 internal/domain/constants.go:3-7 對 ingest_kind/ingest_status 的判斷。
// Plan C 註冊時要先確認值的形狀：repo 內 97 個 CodeMeta enum 有 0 個用純小寫
// Value（52 個數字字串、45 個帶點路由鍵、20+ 個 UPPER_SNAKE）。
type MissingReason string

const (
	// 以下 5 個逐字對應上游 forecasting.py。
	MissingReasonMarketClosed                MissingReason = "market_closed"                 // forecasting.py:264
	MissingReasonInsufficientCalibrationData MissingReason = "insufficient_calibration_data" // forecasting.py:127
	MissingReasonInvalidCalibrationLineage   MissingReason = "invalid_calibration_lineage"   // forecasting.py:134/146/156
	MissingReasonInsufficientRecentData      MissingReason = "insufficient_recent_data"      // forecasting.py:329
	MissingReasonCalendarUnverified          MissingReason = "calendar_unverified"           // forecasting.py:374

	// 以下是 P4 自訂（設計書 §8 line 425-427 + §14 line 558），上游無對應。
	MissingReasonNonFinitePrice            MissingReason = "non_finite_price"
	MissingReasonNonPositiveP50            MissingReason = "non_positive_p50"
	MissingReasonQuantilesNotMonotonic     MissingReason = "quantiles_not_monotonic"
	MissingReasonModelLineageMissing       MissingReason = "model_lineage_missing"
	MissingReasonHorizonSetIncomplete      MissingReason = "horizon_set_incomplete"
	MissingReasonBelowRidgeSampleThreshold MissingReason = "below_ridge_sample_threshold"
)

// calibration method 的合法值，由訓練期依 kind 決定（training.py:756/775）。
const (
	calibrationMethodRidge    = "chronological_tail_strict_split"
	calibrationMethodSeasonal = "rolling_point_in_time_pseudo_residuals"
)

// 模型種類，由訓練期決定並寫進 model.json（training.py:736-782）。
// 推論期永遠不重新選擇——這是刻意的：kind 決定於訓練時看得到的樣本量，
// 執行期沒有那個資訊。
const (
	ModelKindDirectRidge   = "direct_ridge"
	ModelKindSeasonalNaive = "seasonal_naive"
	// ModelKindNoRecentData 是 forecasting.py:333 在 last_price_missing==1 時
	// 覆寫的 model_kind，不是訓練期的 kind。
	ModelKindNoRecentData = "no_recent_data"
)

// CalibrationGateReason 移植 _calibration_gate_reason（forecasting.py:122-157）。
// 回傳 "" 代表過關。
//
// 檢查順序有意義：calibration_count 在血緣之前。兩者同時壞掉時，「樣本不足」
// 是比「artifact 損壞」更常見也更可行動的解釋。
func CalibrationGateReason(m HorizonModel, minCalibrationSamples int) MissingReason {
	if m.CalibrationCount < minCalibrationSamples {
		return MissingReasonInsufficientCalibrationData
	}
	expectedMethod := calibrationMethodSeasonal
	if m.Kind == ModelKindDirectRidge {
		expectedMethod = calibrationMethodRidge
	}
	if m.CalibrationMethod != expectedMethod {
		return MissingReasonInvalidCalibrationLineage
	}
	if m.Kind == ModelKindDirectRidge {
		if m.Ridge == nil ||
			m.ProperTrainingCount < 2 ||
			m.ProperTrainingEndTargetDate == nil ||
			m.CalibrationStartTargetDate == nil ||
			m.CalibrationEndTargetDate == nil ||
			// proper_training_end >= calibration_start 代表訓練資料吃到了
			// calibration 區間 —— 校準殘差因此不是 out-of-sample，區間會過窄。
			!m.ProperTrainingEndTargetDate.Before(*m.CalibrationStartTargetDate) ||
			m.CalibrationEndTargetDate.Before(*m.CalibrationStartTargetDate) ||
			m.ProperTrainingCount+m.CalibrationCount != m.SampleCount {
			return MissingReasonInvalidCalibrationLineage
		}
		return ""
	}
	if m.Ridge != nil ||
		m.ProperTrainingCount != 0 ||
		m.ProperTrainingEndTargetDate != nil ||
		m.CalibrationCount != m.SampleCount ||
		m.CalibrationStartTargetDate == nil ||
		m.CalibrationEndTargetDate == nil ||
		m.CalibrationEndTargetDate.Before(*m.CalibrationStartTargetDate) {
		return MissingReasonInvalidCalibrationLineage
	}
	return ""
}

// DefaultMinRidgeSampleCount 是 direct_ridge 的最小訓練樣本數門檻。
//
// 依據：設計書 §14 line 557 的 5 年 hold-out backtest（2021-07-29..2026-07-28，
// 5,118,825 列），依 N_7 分層的 MAE ratio（ridge / seasonal_naive，< 1 才是贏）：
//
//	N_7  30–46  → 1.176（50.9% 勝 naive，P10–P90 實測覆蓋率 55.3%）
//	N_7  47–99  → 1.169（40.2%，61.3%）
//	N_7 100–299 → 0.968（54.4%，70.3%）
//	N_7 300–699 → 0.911（62.8%，76.0%）
//	N_7 700+    → 0.885（73.1%，73.7%）
//
// 也就是 N_7 ≥ 100 之後 ridge 才真的贏。訓練期門檻是
// config.min_samples_per_model = 30（demo artifact 實測值），而 Level 0 決策
// 下我方不重訓、改不了那個值，所以在服務端擋。
//
// 這個值是「G1 尚未執行」的暫代：G1（multi-fold rolling-origin backtest）會給出
// 正式的門檻值，目前這個數字來自單一 hold-out 窗（2026 年六七月單一產季）。
const DefaultMinRidgeSampleCount = 100

// DefaultGroupMinMarketSupportCount 是大類聚合 usabilityStatus（issue #2249
// Slice A dataMeta）判 usable 所需的最低支援市場數（GroupPoint.
// MarketSupportCount）。
//
// 這是「正式門檻尚未確認」的暫代值——比照 DefaultMinRidgeSampleCount 的模式
// （G1 backtest 前的暫代），先給保守值、寫進 config、明確標記待確認，不是
// 悄悄猜一個數字定案。依據見
// 私人封存的 ADR 2026-08-07-pricebook-group-readiness-gate-resolution
// D3：C1（完全不擋）等於讓 marketSupportCount==1（單一市場冒充全臺行情）也
// 標 usable，違反前端 epic #292 §20 的把關意圖；但正式門檻缺市場覆蓋分布
// 實證，2 是保守值，不是拍板值。
//
// Followup（同 ADR）：待營運／產品正式確認 marketSupportCount 門檻數字
// （issue website #292 §26 原題未決）。
const DefaultGroupMinMarketSupportCount = 2

// GateConfig 是品質閘門的可調參數。
type GateConfig struct {
	MinRidgeSampleCount int
}

// GateVerdict 是一次閘門評估的結果。Reason 為空 iff Passed。
type GateVerdict struct {
	Passed bool
	Reason MissingReason
}

func gatePassed() GateVerdict { return GateVerdict{Passed: true} }

func gateFailed(reason MissingReason) GateVerdict {
	return GateVerdict{Passed: false, Reason: reason}
}

// EvaluatePointGates 執行設計書 §8 line 425-427 的品質閘門（單點部分）加上
// §14 line 558 的 ridge 樣本量保守閘門。
//
// 沒有價格的點直接放行：它們的 MissingReason 已經是最精確的解釋，被價格閘門
// 覆寫會讓「今天沒開市」變成「價格異常」。
//
// 刻意不重複 calibration_count 的檢查：那條已由 CalibrationGateReason 在
// SevenDays 分支 B 執行，且上游版本比 §8 的簡述更完整（另查 calibration_method
// 與血緣一致性）。在這裡再查一次會是死碼。
func EvaluatePointGates(p SeriesPoint, cfg GateConfig) GateVerdict {
	// 血緣欄位對所有點都必須存在——沒有 model_version / dataset_digest 就無法
	// 分辨這個數字是哪個模型、哪份資料產的，而設計書 §2「未來 challenger 的退路」
	// 明講回應必須攜帶這兩者才能辨識兩種產法。
	if p.ModelVersion == "" || !isSHA256Hex(p.DatasetDigest) {
		return gateFailed(MissingReasonModelLineageMissing)
	}
	if p.P10 == nil || p.P50 == nil || p.P90 == nil {
		return gatePassed()
	}
	for _, v := range []float64{*p.P10, *p.P50, *p.P90} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return gateFailed(MissingReasonNonFinitePrice)
		}
	}
	if *p.P50 <= 0 {
		return gateFailed(MissingReasonNonPositiveP50)
	}
	if *p.P10 > *p.P50 || *p.P50 > *p.P90 {
		return gateFailed(MissingReasonQuantilesNotMonotonic)
	}
	if p.ModelKind == ModelKindDirectRidge && cfg.MinRidgeSampleCount > 0 && p.SampleCount < cfg.MinRidgeSampleCount {
		return gateFailed(MissingReasonBelowRidgeSampleThreshold)
	}
	return gatePassed()
}

// isSHA256Hex 檢查 dataset_digest 的形狀（64 位小寫 hex），比照上游發布期的
// 檢查（postgres_publication.py:329）。
func isSHA256Hex(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

// EvaluateHorizonSet 檢查設計書 §8 line 427 的「七日缺日或重複」。
//
// 目前唯一的 production 呼叫點是 usecase/forecast.go 的 series 路徑
// （internal/usecase/forecast.go:353），而對 SevenDays 的輸出這是結構上不
// 可達的（SevenDays 內部 for horizon := 1; horizon <= 7 保證恆為七日齊全、
// 不重複）——這裡是 defense-in-depth，不是活躍防線。
//
// 大類路徑（group.go）不經過這個函式：AggregateGroup 自己另外實作了一份
// 等價的七日完整性檢查（group.go:91-107 的同一條不變量），但那份實作失敗
// 時回的是 error，不是這裡的 MissingReasonHorizonSetIncomplete（也就是
// §8 line 427 要求的 insufficient_data 狀態）——這是與設計書的已知差異，
// 已列為 Plan C followup，不在本檔修正範圍內。
//
// 同時檢查 target_date 沒有重複——horizon 齊全但日期撞在一起是 merge 出錯的
// 典型形狀。
func EvaluateHorizonSet(points []SeriesPoint) GateVerdict {
	if len(points) != 7 {
		return gateFailed(MissingReasonHorizonSetIncomplete)
	}
	seenHorizon := make(map[int]bool, 7)
	seenDate := make(map[Date]bool, 7)
	for _, p := range points {
		if p.HorizonDays < 1 || p.HorizonDays > 7 || seenHorizon[p.HorizonDays] {
			return gateFailed(MissingReasonHorizonSetIncomplete)
		}
		if seenDate[p.TargetDate] {
			return gateFailed(MissingReasonHorizonSetIncomplete)
		}
		seenHorizon[p.HorizonDays] = true
		seenDate[p.TargetDate] = true
	}
	return gatePassed()
}
