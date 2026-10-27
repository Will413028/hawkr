package forecast

import (
	"fmt"
	"time"
)

// PredictionStatus 是對外的預測狀態，值取自設計書 §8 line 419 與 §4 的 proto
// 註解。
//
// 🔴 這是「外層」詞彙，與 PointStatus（內層，逐字對應上游）是不同的兩套。
// 上游的 serving 層（api/catalog.py）也有 ready / unavailable / stale，但
// insufficient_data 在上游全 repo 不存在——那是我方自己的值。
//
// ⚠️ 這組值將於 Plan C 隨 proto 進 CodeMeta（設計書 §11 line 489）；屆時 SSOT
// 移到 docs/代碼管理（CodeMeta）/。註冊前要先確認值的形狀：repo 內 97 個
// CodeMeta enum 有 0 個用純小寫 Value（52 個數字字串、45 個帶點路由鍵、
// 20+ 個 UPPER_SNAKE）。這裡先照設計書 §4 line 262 的小寫字面值。
type PredictionStatus string

const (
	PredictionStatusReady            PredictionStatus = "ready"
	PredictionStatusPartial          PredictionStatus = "partial"
	PredictionStatusMarketClosed     PredictionStatus = "market_closed"
	PredictionStatusInsufficientData PredictionStatus = "insufficient_data"
	PredictionStatusNotPredicted     PredictionStatus = "not_predicted"
	PredictionStatusStale            PredictionStatus = "stale"
	PredictionStatusUnavailable      PredictionStatus = "unavailable"
)

// OuterStatus 的映射表見 plan 的 Task 8 段落。回傳的第二個值是對外要一併帶上的
// 原因碼（沿用內層或閘門的原因，不重新造詞）。
func OuterStatus(p SeriesPoint, v GateVerdict) (PredictionStatus, MissingReason) {
	switch p.PointStatus {
	case PointStatusNotApplicable:
		return PredictionStatusMarketClosed, p.MissingReason

	case PointStatusNotPredicted:
		// insufficient_recent_data 有自己的對外值（§4 line 262 的 not_predicted），
		// 語意是「這條 series 近期完全沒有成交」——與「模型不夠好」是不同的事，
		// 前端的文案也不同（「近期無成交紀錄」vs「資料不足無法預測」）。
		if p.MissingReason == MissingReasonInsufficientRecentData {
			return PredictionStatusNotPredicted, p.MissingReason
		}
		return PredictionStatusInsufficientData, p.MissingReason

	case PointStatusPredicted:
		// 設計書 §8 line 421：缺日曆資料的日期一律回 insufficient_data，
		// 不可假設「沒資料就是休市」。與上游不同（上游照樣出價、只把
		// market_status 標 unknown）。
		//
		// ⚠️ 已知副作用：D+1~D+7 跨年而 MOA 尚未發布次年休市日曆時，整週會變成
		// insufficient_data。已實測民國 115 年（2026）完整、116 年未查。需要指標
		// 與告警（P8），不是可以靜靜接受的狀態。
		if p.MissingReason == MissingReasonCalendarUnverified {
			return PredictionStatusInsufficientData, p.MissingReason
		}
		if !v.Passed {
			return PredictionStatusInsufficientData, v.Reason
		}
		return PredictionStatusReady, ""

	default:
		// 未知的內層狀態一律保守處理。預設回 ready 會讓一個沒人想過的狀態
		// 直接出價給賣家。
		return PredictionStatusInsufficientData, p.MissingReason
	}
}

// FreshnessLabel 是資料新鮮度分級（設計書 §8 line 423）。
type FreshnessLabel string

const (
	FreshnessFresh       FreshnessLabel = "fresh"
	FreshnessStale       FreshnessLabel = "stale"
	FreshnessUnavailable FreshnessLabel = "unavailable"
)

// FreshnessSeverity ranks FreshnessLabel from least to most concerning, the
// same role forecast.PredictionStatusSeverity plays for PredictionStatus —
// used to fold several contributing signals into the one a caller should
// trust least.
func FreshnessSeverity(f FreshnessLabel) int {
	switch f {
	case FreshnessFresh:
		return 0
	case FreshnessStale:
		return 1
	default: // FreshnessUnavailable, or any future/unknown value
		return 2
	}
}

// FreshnessConfig 是新鮮度門檻。設計書 §8 line 423 要求「門檻放 service config，
// 不散落在元件」。
type FreshnessConfig struct {
	// FreshWithin 之內為 fresh（設計書：發布後 36h）
	FreshWithin time.Duration
	// StaleWithin 之內為 stale，超過為 unavailable（設計書：36–72h stale，>72h unavailable）
	StaleWithin time.Duration
}

func DefaultFreshnessConfig() FreshnessConfig {
	return FreshnessConfig{FreshWithin: 36 * time.Hour, StaleWithin: 72 * time.Hour}
}

func (c FreshnessConfig) Validate() error {
	if c.FreshWithin <= 0 || c.StaleWithin <= 0 {
		return fmt.Errorf("forecast: freshness thresholds must be positive, got fresh=%s stale=%s", c.FreshWithin, c.StaleWithin)
	}
	if c.FreshWithin >= c.StaleWithin {
		return fmt.Errorf("forecast: fresh_within (%s) must be less than stale_within (%s), otherwise the stale band disappears", c.FreshWithin, c.StaleWithin)
	}
	return nil
}

// Classify 依「最新一筆觀測的可見時間」分級。
//
// latestAvailableAt 用 available_at（我方可見時間）而不是 trade_date：新鮮度要
// 回答的是「這份數字有多舊」，而 trade_date 到 available_at 之間本來就有
// MOA 的發布延遲。P3 的 FeatureVector.MaxFeatureAvailableAt 正是這個值。
//
// nil（完全沒有可用觀測）→ unavailable，不是 fresh。
func (c FreshnessConfig) Classify(now time.Time, latestAvailableAt *time.Time) FreshnessLabel {
	if latestAvailableAt == nil {
		return FreshnessUnavailable
	}
	age := now.Sub(*latestAvailableAt)
	switch {
	case age < c.FreshWithin:
		// age 可能是負數（容器時鐘與 DB 時鐘不同步，或 available_at 由上游填成
		// 未來時間）。當 fresh 處理：把時鐘誤差當成「資料太舊」會誤把好資料下架。
		return FreshnessFresh
	case age < c.StaleWithin:
		return FreshnessStale
	default:
		return FreshnessUnavailable
	}
}
