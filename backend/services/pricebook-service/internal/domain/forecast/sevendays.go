package forecast

import (
	"fmt"
	"maps"
	"slices"
	"time"
)

// PointStatus 是單一 (series, horizon) 點的內層狀態，逐字對應上游
// forecast_seven_days 產出的 prediction_status（forecasting.py:263/293/328/373）。
//
// 🔴 這是「內層」狀態：只有 3 個值，而且與設計書 §8 line 419 的 7 個對外值
// （ready / partial / market_closed / insufficient_data / not_predicted /
// stale / unavailable）是**不同的兩套詞彙**。上游那 7 個值只存在於它的 serving
// 層（api/catalog.py），而 insufficient_data 在上游全 repo 不存在。
// 兩層之間的映射在 status.go（Task 8）。混成一層會讓 parity test 失去比對基準。
type PointStatus string

const (
	PointStatusNotApplicable PointStatus = "not_applicable"                 // 休市
	PointStatusNotPredicted  PointStatus = "not_predicted"                  // calibration gate 或近期無資料
	PointStatusPredicted     PointStatus = "predicted_conditional_on_trade" // 有價格
)

// calendar_status / market_status 的字面值（forecasting.py:261-262/291-292）。
const (
	CalendarStatusVerifiedClosed = "verified_closed"
	CalendarStatusVerifiedOpen   = "verified_open"
	CalendarStatusUnverified     = "unverified"

	MarketStatusClosed  = "closed"
	MarketStatusOpen    = "open"
	MarketStatusUnknown = "unknown"
)

// TargetSemanticsPriceGivenTrade 是每一列都必須攜帶的固定值
// （forecasting.py:260 等四處硬編）。語意是「假設當天有成交，價格會是多少」——
// 不是「當天會不會有成交」。設計書 §4 line 304 把它列為必填而非選填，理由是
// 任何把它省略的 UI 都會讓賣家把預測讀成「保證會有這個成交價」。
const TargetSemanticsPriceGivenTrade = "price_given_trade"

// SeriesPoint 是一條 series 在一個 horizon 上的完整推論結果。
// 欄位對應上游 PriceForecast.to_dict()（forecasting.py:45-70）需要的子集，
// 加上 P4 的外層映射（Task 8）與品質閘門（Task 7）所需的血緣欄位。
type SeriesPoint struct {
	SeriesID     string
	MarketID     string
	CropID       string
	CategoryCode *string

	HorizonDays int
	TargetDate  Date

	CalendarStatus string
	MarketStatus   string
	PointStatus    PointStatus
	MissingReason  MissingReason
	ModelKind      string

	P10 *float64
	P50 *float64
	P90 *float64

	MaxFeatureAvailableAt *time.Time

	// 血緣與閘門輸入
	ModelVersion     string
	DatasetDigest    string
	SampleCount      int
	CalibrationCount int
	TargetSemantics  string
}

// SevenDaysInput 是一次推論的全部輸入。
type SevenDaysInput struct {
	Artifact ModelArtifact

	// HistoryBySeries 是每條 series 已解過 revision 的歷史觀測
	// （一個 (series_id, trade_date) 恰一筆）。
	//
	// 🔴 這裡不重跑 latest_points_as_of。上游在 forecasting.py:196-205 對已經由
	// SQL 解過的點再跑一次純 Python 解析器，而且兩者判準不同：SQL 版按
	// revision_no 排（storage/revisions.py:1048-1075 的 LEAD(knowledge_from)
	// OVER (PARTITION BY series_id, trade_date ORDER BY revision_no)），Python
	// 版按 available_at 排（features.py:257-290 — PricePoint 上根本沒有
	// revision_no 欄位）。我方統一在 SQL 做（見 db/queries/forecast.sql 的
	// SeriesObservationsAsOf），這裡的輸入契約就是「已解析完成」，與 P3 的
	// BuildFeatureVectorFromDailyPoints 既有假設一致。
	HistoryBySeries map[string]map[Date]PricePoint

	Origin   time.Time
	Closures *Closures
	Verified VerifiedCalendar
}

// SevenDays 移植 forecast_seven_days（forecasting.py:160-384）。
//
// 回傳固定 len(Artifact.Series) * 7 列，順序是 (series_id 升冪, horizon 1..7)。
// 任何分支都不減列——「這個點沒有價格」是用狀態欄位表達的，不是用缺列表達。
func SevenDays(in SevenDaysInput) ([]SeriesPoint, error) {
	calendarTimezone := "UTC"
	if v, ok := in.Artifact.Metadata["calendar_timezone"].(string); ok && v != "" {
		calendarTimezone = v
	}
	originDate, cutoff, err := OriginDateAndCutoff(in.Origin, calendarTimezone)
	if err != nil {
		return nil, err
	}
	if cutoff.Before(in.Artifact.TrainedAt) {
		return nil, fmt.Errorf("forecast: forecast origin %s cannot be earlier than artifact trained_at %s",
			cutoff.Format(time.RFC3339), in.Artifact.TrainedAt.Format(time.RFC3339))
	}
	if in.Closures == nil {
		in.Closures = NewClosures()
	}

	seriesIDs := slices.Sorted(maps.Keys(in.Artifact.Series))
	out := make([]SeriesPoint, 0, len(seriesIDs)*7)

	for _, seriesID := range seriesIDs {
		series := in.Artifact.Series[seriesID]
		history := in.HistoryBySeries[seriesID]

		for horizon := 1; horizon <= 7; horizon++ {
			targetDate := originDate.AddDays(horizon)
			horizonModel, ok := series.Horizon(horizon)
			if !ok {
				// 上游靠 SeriesModel.__post_init__（training.py:258-260）保證
				// horizons 恰為 1..7；Go 端讀 JSON 沒有那個建構期檢查。
				return nil, fmt.Errorf("forecast: series %s is missing horizon %d (artifact requires all of 1..7)", seriesID, horizon)
			}

			point := SeriesPoint{
				SeriesID: seriesID, MarketID: series.MarketID, CropID: series.CropID,
				CategoryCode: series.CategoryCode,
				HorizonDays:  horizon, TargetDate: targetDate,
				ModelKind:        horizonModel.Kind,
				ModelVersion:     in.Artifact.ModelVersion,
				DatasetDigest:    in.Artifact.DatasetDigest,
				SampleCount:      horizonModel.SampleCount,
				CalibrationCount: horizonModel.CalibrationCount,
				TargetSemantics:  TargetSemanticsPriceGivenTrade,
			}

			calendarVerified := in.Verified.IsVerified(series.MarketID, series.CategoryCode, targetDate.Year)

			closed := in.Closures.IsClosed(series.MarketID, series.CategoryCode, targetDate)
			if closed {
				point.CalendarStatus = CalendarStatusVerifiedClosed
				point.MarketStatus = MarketStatusClosed
			} else {
				point.CalendarStatus = CalendarStatusVerifiedOpen
				point.MarketStatus = MarketStatusOpen
				if !calendarVerified {
					point.CalendarStatus = CalendarStatusUnverified
					point.MarketStatus = MarketStatusUnknown
				}
			}

			closedPoint := func(status PointStatus, reason MissingReason) (PointStatus, MissingReason) {
				if closed {
					return PointStatusNotApplicable, MissingReasonMarketClosed
				}
				return status, reason
			}

			// 分支 B：calibration gate（forecasting.py:276-304）。
			if reason := CalibrationGateReason(horizonModel, in.Artifact.Config.MinCalibrationSamples); reason != "" {
				point.PointStatus, point.MissingReason = closedPoint(PointStatusNotPredicted, reason)
				out = append(out, point)
				continue
			}

			featureVector, err := BuildFeatureVectorFromDailyPoints(
				history, in.Origin, horizon, in.Artifact.Config.FeatureConfig, calendarTimezone)
			if err != nil {
				return nil, fmt.Errorf("forecast: series %s horizon %d: %w", seriesID, horizon, err)
			}
			point.MaxFeatureAvailableAt = featureVector.MaxFeatureAvailableAt

			lastPriceMissing, ok := featureVector.Value("last_price_missing")
			if !ok {
				return nil, fmt.Errorf("forecast: feature vector for series %s horizon %d has no last_price_missing", seriesID, horizon)
			}

			// 分支 C：近期完全沒有成交（forecasting.py:306-339）。
			if lastPriceMissing == 1 {
				point.PointStatus, point.MissingReason = closedPoint(PointStatusNotPredicted, MissingReasonInsufficientRecentData)
				point.ModelKind = ModelKindNoRecentData
				out = append(out, point)
				continue
			}

			// 分支 D：出價（forecasting.py:340-383）。
			var center float64
			if horizonModel.Kind == ModelKindDirectRidge {
				if horizonModel.Ridge == nil {
					// CalibrationGateReason 已擋掉這個組合，這裡是 defense in depth。
					return nil, fmt.Errorf("forecast: series %s horizon %d has kind=direct_ridge but no ridge parameters", seriesID, horizon)
				}
				center, err = horizonModel.Ridge.Predict(featureVector.Values)
				if err != nil {
					return nil, fmt.Errorf("forecast: series %s horizon %d: %w", seriesID, horizon, err)
				}
			} else {
				fallback := horizonModel.FallbackPrice
				centerPtr, err := SeasonalNaiveFromDailyPoints(
					history, in.Origin, targetDate, &fallback,
					horizonModel.SeasonalLagDays,
					in.Artifact.Config.FeatureConfig.MaxDaysSinceLastTrade,
					calendarTimezone)
				if err != nil {
					return nil, fmt.Errorf("forecast: series %s horizon %d: %w", seriesID, horizon, err)
				}
				center = horizonModel.FallbackPrice
				if centerPtr != nil {
					center = *centerPtr
				}
				// 上游在這裡會把 model_kind 覆寫成
				// "seasonal_naive_runtime_fallback"（forecasting.py:355），條件是
				// 「訓練期 kind 是 direct_ridge 但走了 naive 分支」。那個條件在
				// 上游**不可達**：分支 C 已經在 last_price_missing==1 時 continue，
				// 所以到這裡 kind==direct_ridge 必定走上面的 ridge 分支。不移植
				// 這個覆寫，因為它是死碼；保留這段註解讓未來讀者知道少的是什麼。
			}

			p10, p50, p90, err := OrderedNonNegative(center, horizonModel.ResidualQuantiles)
			if err != nil {
				// 上游在這裡 raise，會讓一條壞 series 殺掉整批數千條
				// （forecasting.py:114-115）。服務端一批就是全站，所以降級成
				// 該點的 missing_reason，由 Task 7 的閘門決定對外狀態。
				point.PointStatus, point.MissingReason = closedPoint(PointStatusNotPredicted, MissingReasonNonFinitePrice)
				out = append(out, point)
				continue
			}
			point.PointStatus = PointStatusPredicted
			point.MissingReason = ""
			if !calendarVerified {
				point.MissingReason = MissingReasonCalendarUnverified
			}
			point.PointStatus, point.MissingReason = closedPoint(point.PointStatus, point.MissingReason)
			point.P10, point.P50, point.P90 = &p10, &p50, &p90
			out = append(out, point)
		}
	}
	return out, nil
}
