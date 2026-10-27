package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/Will413028/hawkr/pricebook-service/internal/config"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/forecast"
)

// Forecast 把 repository 讀來的觀測與休市日曆餵進 domain 的推論編排，
// 並套上外層狀態映射與品質閘門。
//
// 刻意不在建構時載入 model.json：P4 沒有任何 RPC 會吐預測（Plan C 才有），
// artifact 路徑一律由呼叫端（CLI）傳入。這樣可以完全避開「production
// model.json 154MB 怎麼進 image / 要不要放物件儲存」這個決策——它的前置條件
// （從上游訓練機取得那份檔案）還沒完成，現在決定會是憑空猜。
type Forecast struct {
	repo   domain.ForecastRepository
	cfg    *config.Config
	logger *zap.Logger
}

func NewForecast(repo domain.ForecastRepository, cfg *config.Config, logger *zap.Logger) *Forecast {
	return &Forecast{repo: repo, cfg: cfg, logger: logger}
}

// SeriesForecastInput / SeriesForecastOutput 是單一 (market, crop) 的輸入輸出。
type SeriesForecastInput struct {
	Artifact   forecast.ModelArtifact
	MarketCode string
	CropCode   string
	Origin     time.Time
}

type ForecastPoint struct {
	HorizonDays   int
	TargetDate    forecast.Date
	Status        forecast.PredictionStatus
	MissingReason forecast.MissingReason
	ModelKind     string
	P10, P50, P90 *float64
}

type ObservedPoint struct {
	TradeDate    forecast.Date
	MarketStatus string // open | closed | no_trade | missing（設計書 §4 ObservedPoint）
	AvgPrice     *float64
	VolumeKg     *float64
	RevisionNo   int32
}

type SeriesForecastOutput struct {
	SeriesID        string
	MarketCode      string
	CropCode        string
	CategoryCode    *string
	Observed        []ObservedPoint
	Forecast        []ForecastPoint
	Freshness       forecast.FreshnessLabel
	DataThroughDate *forecast.Date
	ModelVersion    string
	DatasetDigest   string
	TargetSemantics string
	WeatherFeature  string // 固定 "available_not_used"（設計書 §4 line 288）
	// MaxFeatureAvailableAt（issue #2249 Slice A dataMeta 前置，2026-08-07
	// 加）：本次拿去餵 freshness 分級的同一個 maxAvailableAt——「知識時間
	// 截止」，不是另開一條邏輯重算。nil 代表完全沒有可用觀測（同 Freshness
	// 計算時 latestAvailableAt==nil 的情形）。
	MaxFeatureAvailableAt *time.Time
}

// GroupForecastInput / GroupForecastOutput 是一個大類（跨市場、跨子品項）的
// 輸入輸出。輸出直接重用 forecast.GroupPoint——它已經帶齊 CLI 與未來 proto
// 需要的所有欄位（aggregation_method/effective_variant_count/
// market_support_count），沒有理由在 usecase 層再包一層同構的型別。
type GroupForecastInput struct {
	Artifact  forecast.ModelArtifact
	GroupCode string
	CropCodes []string
	Origin    time.Time
}

type GroupForecastOutput struct {
	GroupCode       string
	Forecast        []forecast.GroupPoint
	Freshness       forecast.FreshnessLabel
	DataThroughDate *forecast.Date
	ModelVersion    string
	DatasetDigest   string
	// MaxFeatureAvailableAt：同 SeriesForecastOutput 的同名欄位，理由相同。
	MaxFeatureAvailableAt *time.Time
}

// calendarTimezoneOf 鏡射 forecast.SevenDays 內部同名邏輯
// （sevendays.go:102-105）。usecase 必須在呼叫 SevenDays 之前就知道
// calendar_timezone，才能算出 lookback 窗要查哪個 originDate 的歷史——這是
// 唯一必須複製這段邏輯的理由，兩處改動要同步。
func calendarTimezoneOf(artifact forecast.ModelArtifact) string {
	if v, ok := artifact.Metadata["calendar_timezone"].(string); ok && v != "" {
		return v
	}
	return "UTC"
}

// freshnessConfigFrom 把 config.InferenceConfig 的 *_WITHIN_HOURS 換算成
// forecast.FreshnessConfig。
func freshnessConfigFrom(c config.InferenceConfig) forecast.FreshnessConfig {
	return forecast.FreshnessConfig{
		FreshWithin: time.Duration(c.FreshWithinHours) * time.Hour,
		StaleWithin: time.Duration(c.StaleWithinHours) * time.Hour,
	}
}

// buildVerifiedCalendar 把 repo.RestDayCoverage 的結果轉成
// forecast.VerifiedCalendar。
//
// 🔴 陷阱防禦（三個累積 review 陷阱之一）：forecast.VerifiedCalendar 的零值是
// enforced=false，IsVerified 會一律回 true（fail-open）。這個函式是本檔唯一
// 建構 VerifiedCalendar 的地方，一律呼叫 forecast.NewVerifiedCalendar（即使
// keys 為空），絕不讓呼叫端不小心用到零值。coverage 為空時記 Warn——那代表
// 「這個市場完全沒有休市日曆覆蓋」，所有點會被判定 calendar_unverified /
// insufficient_data，這是預期但必須可觀察的行為，不能靜默發生。
func buildVerifiedCalendar(logger *zap.Logger, source string, coverage []domain.RestDayCoverage) forecast.VerifiedCalendar {
	keys := appendVerifiedCalendarKeys(logger, nil, source, coverage)
	return forecast.NewVerifiedCalendar(keys)
}

// appendVerifiedCalendarKeys 把一個市場的 rest-day coverage 轉成
// VerifiedCalendarKey 附加進 keys 並回傳。coverage 為空時記 Warn，未知
// market_type 略過並記 Warn。SeriesForecast（經 buildVerifiedCalendar，單一
// 市場）與 GroupForecast（迴圈跨多市場累積進同一份 keys）共用，避免這段映射
// 邏輯在兩處分岔（P4 review followup）。extraFields 讓呼叫端補上自己才有的
// 上下文（例如 GroupForecast 補 group_code）——coverage 為空這件事必須可觀察，
// 少了呼叫端context 會讓多個呼叫者共用同一個 market_code 時無法從 log 分辨
// 是誰觸發的。
func appendVerifiedCalendarKeys(logger *zap.Logger, keys []forecast.VerifiedCalendarKey, source string, coverage []domain.RestDayCoverage, extraFields ...zap.Field) []forecast.VerifiedCalendarKey {
	if len(coverage) == 0 {
		fields := append([]zap.Field{zap.String("source", source)}, extraFields...)
		logger.Warn("usecase: no rest-day calendar coverage; all points will be calendar_unverified until ingest runs", fields...)
	}
	for _, c := range coverage {
		category, ok := forecast.CategoryCodeForMarketType(c.MarketType)
		if !ok {
			logger.Warn("usecase: unknown market_type in rest-day coverage, skipping",
				zap.String("market_code", c.MarketCode), zap.String("market_type", c.MarketType))
			continue
		}
		keys = append(keys, forecast.VerifiedCalendarKey{
			MarketID: "moa:" + c.MarketCode, CategoryCode: category, Year: c.Year,
		})
	}
	return keys
}

// addRestDaysToClosures 把 repo.RestDays 的結果加進 closures。無法把
// market_type 換算成 category code 的列一律記 Warn 並跳過——上游改了代碼時
// 要看得見，不能靜默當成沒休市。
func addRestDaysToClosures(logger *zap.Logger, closures *forecast.Closures, restDays []domain.RestDay) {
	for _, rd := range restDays {
		category, ok := forecast.CategoryCodeForMarketType(rd.MarketType)
		if !ok {
			logger.Warn("usecase: unknown market_type in market_rest_day, skipping",
				zap.String("market_code", rd.MarketCode), zap.String("market_type", rd.MarketType))
			continue
		}
		closures.AddMarketCategory("moa:"+rd.MarketCode, category, forecast.DateFromTime(rd.RestDate))
	}
}

// evaluatePointStatus 對單一 SeriesPoint 套用品質閘門與外層狀態映射，
// invalid_calibration_lineage 是 artifact 損壞而非資料不足——對外沒有更誠實的
// 值可用，但必須有人看到，所以另外記 Warn（brief Step 2 第 7 點）。
// SeriesForecast（經 applyGatesAndOuterStatus）與 GroupForecast 共用同一份判定，
// 避免品質閘門邏輯在兩處分岔（P4 review followup）。
func (u *Forecast) evaluatePointStatus(p forecast.SeriesPoint) (forecast.PredictionStatus, forecast.MissingReason) {
	gateCfg := forecast.GateConfig{MinRidgeSampleCount: u.cfg.Inference.MinRidgeSampleCount}
	verdict := forecast.EvaluatePointGates(p, gateCfg)
	status, reason := forecast.OuterStatus(p, verdict)
	if reason == forecast.MissingReasonInvalidCalibrationLineage {
		u.logger.Warn("usecase: invalid calibration lineage in artifact",
			zap.String("series_id", p.SeriesID), zap.Int("horizon_days", p.HorizonDays))
	}
	return status, reason
}

// applyGatesAndOuterStatus 對一批 SeriesPoint 逐點套用 evaluatePointStatus。
func (u *Forecast) applyGatesAndOuterStatus(points []forecast.SeriesPoint) []ForecastPoint {
	out := make([]ForecastPoint, 0, len(points))
	for _, p := range points {
		status, reason := u.evaluatePointStatus(p)
		out = append(out, ForecastPoint{
			HorizonDays: p.HorizonDays, TargetDate: p.TargetDate,
			Status: status, MissingReason: reason, ModelKind: p.ModelKind,
			P10: p.P10, P50: p.P50, P90: p.P90,
		})
	}
	return out
}

// insufficientForecastPoints 在 EvaluateHorizonSet 沒過（七日缺日或重複）時，
// 整條 series 一律回 insufficient_data——原始 points 的結構本身已經不可信，
// 不逐點嘗試搶救。
func insufficientForecastPoints(originDate forecast.Date, reason forecast.MissingReason) []ForecastPoint {
	out := make([]ForecastPoint, 0, 7)
	for h := 1; h <= 7; h++ {
		out = append(out, ForecastPoint{
			HorizonDays: h, TargetDate: originDate.AddDays(h),
			Status: forecast.PredictionStatusInsufficientData, MissingReason: reason,
		})
	}
	return out
}

// SeriesForecast 對單一 (market, crop) 跑七日推論，回傳對外用的預測與觀測窗。
func (u *Forecast) SeriesForecast(ctx context.Context, in SeriesForecastInput) (*SeriesForecastOutput, error) {
	// 🔴 陷阱防禦：Origin 零值經 pgxutil.TimestamptzParam/DateParam 會變成 SQL NULL，
	// `available_at <= NULL` 恆不成立，查詢會靜默回 0 列、err=nil。在這裡
	// 顯式擋掉，讓呼叫端拿到明確 error 而不是一份看起來合法、其實全空的結果。
	if in.Origin.IsZero() {
		return nil, fmt.Errorf("usecase: origin must not be zero (a zero time.Time silently becomes SQL NULL and every as-of query returns 0 rows with err=nil)")
	}
	if in.MarketCode == "" || in.CropCode == "" {
		return nil, fmt.Errorf("usecase: market code and crop code must not be empty")
	}

	seriesID := "moa:" + in.MarketCode + ":" + in.CropCode
	marketID := "moa:" + in.MarketCode
	cropID := "moa:" + in.CropCode

	series, ok := in.Artifact.Series[seriesID]
	if !ok {
		return nil, fmt.Errorf("usecase: artifact has no series %q (model.json does not cover this market/crop combination)", seriesID)
	}

	calendarTimezone := calendarTimezoneOf(in.Artifact)
	originDate, cutoff, err := forecast.OriginDateAndCutoff(in.Origin, calendarTimezone)
	if err != nil {
		return nil, fmt.Errorf("usecase: %w", err)
	}

	// Step 1：lookback 窗（鏡射 forecasting.py:186-196）。
	maxLookback := in.Artifact.Config.FeatureConfig.MaxLookbackDays()
	// review Finding 5（2026-07-30）：ObservedWindowDays 是可調的 env（見
	// config.InferenceConfig），SeriesObservations 的查詢窗卻固定被 maxLookback
	// 卡住。運維把 OBSERVED_WINDOW_DAYS 調得比模型的 max_days_since_last_trade
	// 還大時，超出 maxLookback 的那幾天觀測根本沒被查回來，Observed 窗會把它們
	// 靜默回 missing——看起來像「那段時間市場沒資料」，其實是設定超出了模型
	// 支援的範圍。這不是硬錯誤（合法設定，只是超出模型 lookback），只需要留下
	// 可觀察的 Warn，不需要擋掉。
	if u.cfg.Inference.ObservedWindowDays > maxLookback {
		u.logger.Warn("usecase: OBSERVED_WINDOW_DAYS exceeds the model's max_days_since_last_trade; observations beyond the lookback window will read back as missing",
			zap.Int("observed_window_days", u.cfg.Inference.ObservedWindowDays), zap.Int("max_lookback_days", maxLookback))
	}
	from := originDate.AddDays(-maxLookback).ToTime()
	to := originDate.ToTime()

	// Step 2：觀測。
	observations, err := u.repo.SeriesObservations(ctx, in.MarketCode, in.CropCode, from, to, cutoff)
	if err != nil {
		return nil, fmt.Errorf("usecase: SeriesObservations(%s, %s): %w", in.MarketCode, in.CropCode, err)
	}
	obsByDate := make(map[forecast.Date]domain.Observation, len(observations))
	for _, obs := range observations {
		obsByDate[forecast.DateFromTime(obs.TradeDate)] = obs
	}

	history := make(map[forecast.Date]forecast.PricePoint, len(obsByDate))
	var maxAvailableAt *time.Time
	for d, obs := range obsByDate {
		isActive := obs.MarketActive && obs.CropActive
		pt, err := forecast.NewPricePoint(forecast.PricePointInput{
			SeriesID: seriesID, MarketID: marketID, CropID: cropID,
			TradeDate: d, AveragePrice: obs.AvgPrice, Volume: obs.VolumeKg,
			AvailableAt: obs.AvailableAt, CategoryCode: obs.CategoryCode, IsActive: &isActive,
		})
		if err != nil {
			return nil, fmt.Errorf("usecase: building price point for %s %s on %s: %w", in.MarketCode, in.CropCode, d, err)
		}
		history[d] = pt
		if maxAvailableAt == nil || obs.AvailableAt.After(*maxAvailableAt) {
			avail := obs.AvailableAt
			maxAvailableAt = &avail
		}
	}

	// Step 3：休市日曆。
	//
	// 🔴 review Finding 2（2026-07-30）：查詢窗原本只有 [originDate, originDate+7]
	// （只夠涵蓋 D+1..D+7 的推論窗），但 Step 9 的 Observed 窗回看的是過去
	// ObservedWindowDays 天——30 天裡只有 originDate 當天落在這個查詢窗內，
	// 其餘 29 天即使真的休市，也會因為 closures 裡完全沒有那個日期的 key 而
	// 被 IsClosed 判 false，最終在 Observed 誤標成 missing。這是設計書 §9
	// line 442「休市要標灰階、不是斷線」在觀測窗這一側被塌縮的形狀，方向與
	// 陷阱 2 相反：這裡是「有日曆、那天休市」被謊報成「沒資料」。
	//
	// 修法：把 from 往前延伸到涵蓋 Observed 窗（originDate 往回
	// ObservedWindowDays-1 天），to 維持 originDate+7 不變，兩份查詢共用同一個
	// closures。安全性：Closures 只被兩處消費——SevenDays 只對 D+1..D+7 的
	// targetDate 呼叫 IsClosed，Observed 迴圈只對過去日期呼叫；多出來的過去日
	// key 不可能改變預測輸出（BuildFeatureVectorFromDailyPoints 完全不吃
	// Closures，簽名見 features.go 的 BuildFeatureVectorFromDailyPoints）。
	closures := forecast.NewClosures()
	restDaysFrom := originDate.AddDays(-(u.cfg.Inference.ObservedWindowDays - 1))
	restDays, err := u.repo.RestDays(ctx, in.MarketCode, restDaysFrom.ToTime(), originDate.AddDays(7).ToTime())
	if err != nil {
		return nil, fmt.Errorf("usecase: RestDays(%s): %w", in.MarketCode, err)
	}
	addRestDaysToClosures(u.logger, closures, restDays)

	// Step 4：驗證日曆覆蓋。
	coverage, err := u.repo.RestDayCoverage(ctx, in.MarketCode)
	if err != nil {
		return nil, fmt.Errorf("usecase: RestDayCoverage(%s): %w", in.MarketCode, err)
	}
	verified := buildVerifiedCalendar(u.logger, in.MarketCode, coverage)

	// Step 5：七日推論。只餵這一條 series 的 artifact 子集，避免 SevenDays
	// 連帶算出 artifact 裡其他不相干 series 的點（demo artifact 有兩條 series，
	// 若整份塞進去，輸出會混進不是本次請求的 series）。
	filteredArtifact := in.Artifact
	filteredArtifact.Series = map[string]forecast.SeriesModel{seriesID: series}

	points, err := forecast.SevenDays(forecast.SevenDaysInput{
		Artifact:        filteredArtifact,
		HistoryBySeries: map[string]map[forecast.Date]forecast.PricePoint{seriesID: history},
		Origin:          in.Origin,
		Closures:        closures,
		Verified:        verified,
	})
	if err != nil {
		return nil, fmt.Errorf("usecase: SevenDays(%s): %w", seriesID, err)
	}

	// Step 6：七日缺日或重複 → 整條 series 回 insufficient_data
	// （設計書 §8 line 427；對單一 series 的 filteredArtifact 這條分支結構上
	// 不可達，但作為 defense in depth 保留——理由同 forecast.EvaluateHorizonSet
	// 的檔頭註解）。
	var forecastPoints []ForecastPoint
	if v := forecast.EvaluateHorizonSet(points); !v.Passed {
		forecastPoints = insufficientForecastPoints(originDate, v.Reason)
	} else {
		// Step 7：逐點品質閘門 + 外層狀態映射。
		forecastPoints = u.applyGatesAndOuterStatus(points)
	}

	// Step 8：新鮮度分級。只描述「這份數字有多舊」，不改寫逐點狀態——2026-09-14
	// 決定（ADR docs/adr/2026-09-14-pricebook-freshness-describes-age-not-usability.md）：
	// 「有多舊」與「能不能用」是兩件事，前者由本欄位表達，後者留給消費端判斷。
	// 移除前這裡會在 >72h 時把整條 series 的 7 個點覆寫成 unavailable，而值一路
	// 算完仍留在 payload 裡，只是被下游 usablePredictionStatus 濾掉。
	//
	// 🔴 "now" 用 in.Origin，不是 time.Now()：本方法從 lookback 窗到 cutoff
	// 全部是相對 Origin 的 as-of 查詢，新鮮度若改用真實牆鐘時間，會是唯一一個
	// 不跟著 Origin 時間旅行的欄位——用 --origin 指過去某個時間點重放時，
	// freshness 會被「現在幾點」污染，變得不可預測也不可測試（Forecast struct
	// 刻意沒有注入時鐘，這是本方法不依賴真實牆鐘時間的證據之一）。正式服務
	// （Plan C）沒有 --origin，Origin 恆等於呼叫當下，兩種取法沒有差異。
	freshnessCfg := freshnessConfigFrom(u.cfg.Inference)
	freshness := freshnessCfg.Classify(in.Origin, maxAvailableAt)

	// Step 9：Observed 窗。
	//
	// 🔴 review Finding 3（2026-07-30）：原本 RevisionNo/AvgPrice/VolumeKg 的
	// 賦值散落在 switch 的個別分支裡，導致 no_trade 分支丟了 VolumeKg（即使
	// volume_kg 非 NULL）、closed 分支連 RevisionNo 都沒填。schema 上
	// avg_price 與 volume_kg 是各自獨立可為 NULL 的欄位，設計書 §4 的
	// ObservedPoint 也是兩個獨立欄位——「這天休市」或「這天有成交量但價格
	// 修訂中」不該連帶把本來有的欄位變不見。修法：先查一次 obsByDate，只要
	// 有列就把該列真實有的欄位都帶上（缺值仍然是 nil，不是 0），market_status
	// 的判定完全獨立、不影響欄位賦值。
	observed := make([]ObservedPoint, 0, u.cfg.Inference.ObservedWindowDays)
	for offset := u.cfg.Inference.ObservedWindowDays - 1; offset >= 0; offset-- {
		d := originDate.AddDays(-offset)
		obs, hasObs := obsByDate[d]
		op := ObservedPoint{TradeDate: d}
		if hasObs {
			op.RevisionNo = obs.RevisionNo
			op.AvgPrice = obs.AvgPrice
			op.VolumeKg = obs.VolumeKg
		}
		switch {
		case closures.IsClosed(marketID, series.CategoryCode, d):
			op.MarketStatus = "closed"
		case !hasObs:
			op.MarketStatus = "missing"
		case obs.AvgPrice == nil:
			op.MarketStatus = "no_trade"
		default:
			op.MarketStatus = "open"
		}
		observed = append(observed, op)
	}

	var dataThroughDate *forecast.Date
	for d := range obsByDate {
		if dataThroughDate == nil || d.After(*dataThroughDate) {
			dd := d
			dataThroughDate = &dd
		}
	}

	return &SeriesForecastOutput{
		SeriesID: seriesID, MarketCode: in.MarketCode, CropCode: in.CropCode, CategoryCode: series.CategoryCode,
		Observed: observed, Forecast: forecastPoints, Freshness: freshness, DataThroughDate: dataThroughDate,
		ModelVersion: in.Artifact.ModelVersion, DatasetDigest: in.Artifact.DatasetDigest,
		TargetSemantics: forecast.TargetSemanticsPriceGivenTrade, WeatherFeature: "available_not_used",
		MaxFeatureAvailableAt: maxAvailableAt,
	}, nil
}

// GroupForecast 對一個大類（跨市場、跨子品項）跑七日推論並做兩階段成交量加權。
func (u *Forecast) GroupForecast(ctx context.Context, in GroupForecastInput) (*GroupForecastOutput, error) {
	// 🔴 陷阱防禦：同 SeriesForecast，Origin 零值會讓每一條 as-of 查詢都靜默
	// 回 0 列。
	if in.Origin.IsZero() {
		return nil, fmt.Errorf("usecase: origin must not be zero (a zero time.Time silently becomes SQL NULL and every as-of query returns 0 rows with err=nil)")
	}
	if in.GroupCode == "" {
		return nil, fmt.Errorf("usecase: group code must not be empty")
	}

	calendarTimezone := calendarTimezoneOf(in.Artifact)
	originDate, cutoff, err := forecast.OriginDateAndCutoff(in.Origin, calendarTimezone)
	if err != nil {
		return nil, fmt.Errorf("usecase: %w", err)
	}

	crops, err := u.repo.CropsInGroup(ctx, in.GroupCode)
	if err != nil {
		return nil, fmt.Errorf("usecase: CropsInGroup(%s): %w", in.GroupCode, err)
	}
	if len(crops) == 0 {
		return nil, fmt.Errorf("usecase: group %q has no crops (mapping seed table missing this group, or the group code is wrong)", in.GroupCode)
	}
	// review Finding 4（2026-07-30）：CropsInGroup 的 SQL 不過濾 active（見
	// db/queries/forecast.sql），domain.CropRef.Active 這個欄位查出來了但原本
	// 全程沒被讀。後果：大類裡只要有一個已退場、沒有近期成交的子品項被計進
	// totalVariantCount，EffectiveVariantCount < TotalVariantCount 就會恆成立
	// ——該大類永遠回 partial，永遠不會是 ready（§8 定義的 ready 在大類模式下
	// 實質不可達，而且沒有任何訊號說明為什麼）。
	//
	// 修法：只保留 Active 的子品項——不只是 totalVariantCount 這個分母，
	// groupCropIDs（決定哪些 series 會被餵進 SevenDays）也一併過濾，否則
	// EffectiveVariantCount 可能因為一個 inactive 子品項恰好還有預測結果而
	// 超過用 Active 算出來的 TotalVariantCount，兩個數字就不是同一個子品項
	// 集合、自相矛盾。
	activeCrops := make([]domain.CropRef, 0, len(crops))
	for _, c := range crops {
		if c.Active {
			activeCrops = append(activeCrops, c)
		}
	}
	activeCrops = narrowCropRefs(activeCrops, in.CropCodes)
	if len(activeCrops) == 0 {
		return nil, fmt.Errorf("usecase: group %q has no active crops", in.GroupCode)
	}
	groupCropIDs := make(map[string]bool, len(activeCrops))
	for _, c := range activeCrops {
		groupCropIDs["moa:"+c.CropCode] = true
	}

	// 只保留 artifact 裡屬於本大類的 series——artifact 可能覆蓋不只一個大類，
	// 未過濾會讓聚合摻進不相干子品項的點。
	relevantSeries := map[string]forecast.SeriesModel{}
	for seriesID, series := range in.Artifact.Series {
		if groupCropIDs[series.CropID] {
			relevantSeries[seriesID] = series
		}
	}
	if len(relevantSeries) == 0 {
		return nil, fmt.Errorf("usecase: artifact has no series for any crop in group %q", in.GroupCode)
	}

	// D1（issue #2249 Slice A dataMeta 前置決策，見 ADR
	// 私人封存的 ADR 2026-08-07-pricebook-group-readiness-gate-resolution）：
	// totalVariantCount 只算「DB-active 且上游模型/artifact 也有 series 覆蓋」
	// 的子品項，不是全部 DB-active 子品項（那是舊定義，見本檔上方 review
	// Finding 4 的註解）。理由：usabilityStatus 即將是賣家看得到的真實訊號，
	// 舊定義下 ready 幾乎不可達（模型覆蓋率天生就低於 DB active 子品項數），
	// 會讓 degraded/partial 恆真、失去訊號意義。用 relevantSeries 裡出現過的
	// distinct CropID 集合當分子來源——這正是「有模型覆蓋」的定義，且與
	// SevenDays 實際會產生預測點的子品項集合一致（不會有 TotalVariantCount
	// 大於 SevenDays 實際能覆蓋的子品項數這種自相矛盾）。
	coveredCropIDs := make(map[string]bool, len(relevantSeries))
	for _, series := range relevantSeries {
		coveredCropIDs[series.CropID] = true
	}
	totalVariantCount := len(coveredCropIDs)

	maxLookback := in.Artifact.Config.FeatureConfig.MaxLookbackDays()
	from := originDate.AddDays(-maxLookback).ToTime()
	to := originDate.ToTime()

	observations, err := u.repo.GroupObservations(ctx, in.GroupCode, "", from, to, cutoff)
	if err != nil {
		return nil, fmt.Errorf("usecase: GroupObservations(%s): %w", in.GroupCode, err)
	}

	// 🔴 陷阱防禦（review Finding 1 的呼叫端一半）：marketCode=="" 時同一個
	// (crop, trade_date) 會有多個市場各一列，這裡按 (market, crop) 分組，
	// 不能假設一個 crop 一天一筆——併掉市場鍵會讓大類價格取到「另一個不相干
	// 市場剛好多修訂一次」的結果。
	type seriesKey struct{ marketCode, cropCode string }
	obsBySeries := map[seriesKey]map[forecast.Date]domain.Observation{}
	var maxAvailableAt *time.Time
	for _, obs := range observations {
		k := seriesKey{marketCode: obs.MarketCode, cropCode: obs.CropCode}
		if obsBySeries[k] == nil {
			obsBySeries[k] = map[forecast.Date]domain.Observation{}
		}
		obsBySeries[k][forecast.DateFromTime(obs.TradeDate)] = obs
		if maxAvailableAt == nil || obs.AvailableAt.After(*maxAvailableAt) {
			avail := obs.AvailableAt
			maxAvailableAt = &avail
		}
	}

	// 休市日曆與驗證日曆：對 relevantSeries 涉及的每個市場各查一次，累積進
	// 同一份 Closures / VerifiedCalendar（兩者的鍵都含 marketID，可以安全共用）。
	marketCodes := map[string]bool{}
	for _, series := range relevantSeries {
		marketCodes[strings.TrimPrefix(series.MarketID, "moa:")] = true
	}

	closures := forecast.NewClosures()
	var verifiedKeys []forecast.VerifiedCalendarKey
	for marketCode := range marketCodes {
		restDays, err := u.repo.RestDays(ctx, marketCode, originDate.ToTime(), originDate.AddDays(7).ToTime())
		if err != nil {
			return nil, fmt.Errorf("usecase: RestDays(%s): %w", marketCode, err)
		}
		addRestDaysToClosures(u.logger, closures, restDays)

		coverage, err := u.repo.RestDayCoverage(ctx, marketCode)
		if err != nil {
			return nil, fmt.Errorf("usecase: RestDayCoverage(%s): %w", marketCode, err)
		}
		verifiedKeys = appendVerifiedCalendarKeys(u.logger, verifiedKeys, marketCode, coverage, zap.String("group_code", in.GroupCode))
	}
	// 🔴 陷阱防禦：即使 verifiedKeys 是 nil（所有市場都缺覆蓋，前面已個別
	// warn 過），仍一律走 NewVerifiedCalendar 顯式建構——不落回零值
	// VerifiedCalendar{}（enforced=false 會讓每個點都變成 fail-open 的
	// verified_open）。
	verified := forecast.NewVerifiedCalendar(verifiedKeys)

	historyBySeries := make(map[string]map[forecast.Date]forecast.PricePoint, len(relevantSeries))
	for seriesID, series := range relevantSeries {
		marketCode := strings.TrimPrefix(series.MarketID, "moa:")
		cropCode := strings.TrimPrefix(series.CropID, "moa:")
		obsForSeries := obsBySeries[seriesKey{marketCode: marketCode, cropCode: cropCode}]
		history := make(map[forecast.Date]forecast.PricePoint, len(obsForSeries))
		for d, obs := range obsForSeries {
			isActive := obs.MarketActive && obs.CropActive
			pt, err := forecast.NewPricePoint(forecast.PricePointInput{
				SeriesID: seriesID, MarketID: series.MarketID, CropID: series.CropID,
				TradeDate: d, AveragePrice: obs.AvgPrice, Volume: obs.VolumeKg,
				AvailableAt: obs.AvailableAt, CategoryCode: obs.CategoryCode, IsActive: &isActive,
			})
			if err != nil {
				return nil, fmt.Errorf("usecase: building price point for %s on %s: %w", seriesID, d, err)
			}
			history[d] = pt
		}
		historyBySeries[seriesID] = history
	}

	filteredArtifact := in.Artifact
	filteredArtifact.Series = relevantSeries

	points, err := forecast.SevenDays(forecast.SevenDaysInput{
		Artifact: filteredArtifact, HistoryBySeries: historyBySeries, Origin: in.Origin,
		Closures: closures, Verified: verified,
	})
	if err != nil {
		return nil, fmt.Errorf("usecase: SevenDays for group %s: %w", in.GroupCode, err)
	}

	variantPoints := make([]forecast.VariantPoint, 0, len(points))
	for _, p := range points {
		status, reason := u.evaluatePointStatus(p)
		variantPoints = append(variantPoints, forecast.VariantPoint{
			MarketID: p.MarketID, CropID: p.CropID, HorizonDays: p.HorizonDays, TargetDate: p.TargetDate,
			Status: status, MissingReason: reason, P10: p.P10, P50: p.P50, P90: p.P90,
		})
	}

	vols, err := u.groupVolumes(ctx, in.GroupCode, originDate, cutoff)
	if err != nil {
		return nil, err
	}

	groupPoints, err := forecast.AggregateGroup(variantPoints, vols, totalVariantCount)
	if err != nil {
		return nil, fmt.Errorf("usecase: AggregateGroup(%s): %w", in.GroupCode, err)
	}

	// 同 SeriesForecast Step 8：只分級、不覆寫逐點狀態，且用 in.Origin 而非
	// time.Now()（理由見 SeriesForecast 內的完整說明）。
	freshnessCfg := freshnessConfigFrom(u.cfg.Inference)
	freshness := freshnessCfg.Classify(in.Origin, maxAvailableAt)

	var dataThroughDate *forecast.Date
	for _, dates := range obsBySeries {
		for d := range dates {
			if dataThroughDate == nil || d.After(*dataThroughDate) {
				dd := d
				dataThroughDate = &dd
			}
		}
	}

	return &GroupForecastOutput{
		GroupCode: in.GroupCode, Forecast: groupPoints, Freshness: freshness, DataThroughDate: dataThroughDate,
		ModelVersion: in.Artifact.ModelVersion, DatasetDigest: in.Artifact.DatasetDigest,
		MaxFeatureAvailableAt: maxAvailableAt,
	}, nil
}

// groupVolumes builds the two-stage weighting inputs shared by GroupForecast
// (forward prediction) and GroupObservedSeries (historical aggregation) —
// same RecentVolume window, same (market, crop) → volume conversion. Must
// stay the single implementation of this conversion; do not inline a second
// copy.
func (u *Forecast) groupVolumes(ctx context.Context, groupCode string, originDate forecast.Date, cutoff time.Time) (forecast.GroupVolumes, error) {
	volFrom := originDate.AddDays(-u.cfg.Inference.GroupVolumeWindowDays).ToTime()
	volTo := originDate.ToTime()
	recentVolume, err := u.repo.RecentVolume(ctx, groupCode, volFrom, volTo, cutoff)
	if err != nil {
		return forecast.GroupVolumes{}, fmt.Errorf("usecase: RecentVolume(%s): %w", groupCode, err)
	}
	vols := forecast.GroupVolumes{
		ByVariantMarket: make(map[forecast.VariantMarketKey]float64, len(recentVolume)),
		ByMarket:        make(map[string]float64),
	}
	for _, v := range recentVolume {
		marketID := "moa:" + v.MarketCode
		cropID := "moa:" + v.CropCode
		vols.ByVariantMarket[forecast.VariantMarketKey{CropID: cropID, MarketID: marketID}] = v.Volume
		vols.ByMarket[marketID] += v.Volume
	}
	return vols, nil
}

// ComputeGroupVolumes (issue #2249 Slice B trend-summary, 2026-08-07 加)：
// groupVolumes 的 exported 包裝——trend_summary.go 需要同一份「近 N 日成交量」
// 空間權重（給 AggregateGroupObservations 的 spatial 聚合用），跟
// GroupObservedSeries/GroupForecast 共用同一條實作，不重寫第二份。
func (u *Forecast) ComputeGroupVolumes(ctx context.Context, groupCode string, originDate forecast.Date, cutoff time.Time) (forecast.GroupVolumes, error) {
	return u.groupVolumes(ctx, groupCode, originDate, cutoff)
}

// cropVolumes is groupVolumes' single-crop counterpart — same window/cutoff
// semantics via RecentVolumeForCrop, keyed by bare market_code (unlike
// GroupVolumes.ByMarket's "moa:"-prefixed keys — that prefix convention
// belongs to forecast.VariantMarketKey's artifact-series lookup, which this
// caller never needs, since a single crop has no sub-variants to blend
// within a market).
func (u *Forecast) cropVolumes(ctx context.Context, cropCode string, originDate forecast.Date, cutoff time.Time) (map[string]float64, error) {
	volFrom := originDate.AddDays(-u.cfg.Inference.GroupVolumeWindowDays).ToTime()
	volTo := originDate.ToTime()
	recentVolume, err := u.repo.RecentVolumeForCrop(ctx, cropCode, volFrom, volTo, cutoff)
	if err != nil {
		return nil, fmt.Errorf("usecase: RecentVolumeForCrop(%s): %w", cropCode, err)
	}
	out := make(map[string]float64, len(recentVolume))
	for _, v := range recentVolume {
		out[v.MarketCode] = v.Volume
	}
	return out, nil
}

// ComputeCropVolumes (issue #2248 spatial-surface 跨市場 statistics,
// 2026-08-08 加): cropVolumes 的 exported 包裝，同 ComputeGroupVolumes 的理由
// ——讓 usecase 邊界外的呼叫端（spatial_surface.go 的
// marketVolumesForStatistics）取得這份權重，不用重寫第二份查詢。
func (u *Forecast) ComputeCropVolumes(ctx context.Context, cropCode string, originDate forecast.Date, cutoff time.Time) (map[string]float64, error) {
	return u.cropVolumes(ctx, cropCode, originDate, cutoff)
}

// GroupObservedSeriesInput/Output are GroupObservedSeries' request/response.
type GroupObservedSeriesInput struct {
	GroupCode string
	// MarketCode narrows the aggregation to one market. Empty (the zero
	// value) means all markets — unchanged, existing behavior. A non-empty
	// value reuses AggregateGroupObservations completely unmodified: fed
	// only one market's raw rows, its cross-market weighting stage
	// naturally degenerates to that one market's own within-market blend
	// (design spec §1b) — no special-casing needed in the aggregation
	// algorithm itself.
	MarketCode string
	CropCodes  []string
	From, To   time.Time
	Now        time.Time // cutoff for point-in-time revision visibility; also doubles as the volume-window's origin date
}

type GroupObservedSeriesOutput struct {
	Points []forecast.GroupObservedPoint
	// MaxAvailableAt / SourceDigests（issue #2249 Slice A dataMeta 前置，
	// 2026-08-07 加）：Points 裡的 GroupObservedPoint 是聚合後的合成點，
	// 本身沒有 available_at/source_digest（一個聚合點橫跨多列，沒有單一
	// 列的身分——同 domain.ObservedPrice 聚合列 market_code/crop_code
	// 留空的道理）。這兩個欄位改由此處另外帶出來：取自實際貢獻進聚合結果
	// 的 raw rows（同一份 active-crop + AvgPrice!=nil 過濾條件，被過濾掉的
	// 列不算），讓呼叫端能算 freshnessStatus（MAX available_at）與
	// datasetDigest（Rule 3 cross-row policy），同時仍滿足 snapshot 一致性
	// ——這些欄位描述的正是「實際餵進這個 payload 的資料」，不是另外查一次。
	MaxAvailableAt *time.Time
	SourceDigests  []string
}

// GroupObservedSeries returns a crop group's two-stage-weighted historical
// observed price — one point per calendar date in [From, To] — using the
// exact same market-then-crop volume weighting GroupForecast uses for
// predictions (via the shared groupVolumes helper), so the actual-price
// line and the P50 forecast line a chart draws are methodologically
// consistent at the boundary where one ends and the other begins.
//
// Unlike GroupForecast, this does not touch forecast.ModelArtifact / run
// SevenDays — it aggregates raw observed_price rows directly, so it works
// even for crops the model doesn't cover (see AggregateGroupObservations's
// own doc comment for why that's deliberate).
func (u *Forecast) GroupObservedSeries(ctx context.Context, in GroupObservedSeriesInput) (*GroupObservedSeriesOutput, error) {
	if in.Now.IsZero() {
		return nil, fmt.Errorf("usecase: now must not be zero (a zero time.Time silently becomes SQL NULL and every as-of query returns 0 rows with err=nil)")
	}
	if in.GroupCode == "" {
		return nil, fmt.Errorf("usecase: group code must not be empty")
	}

	crops, err := u.repo.CropsInGroup(ctx, in.GroupCode)
	if err != nil {
		return nil, fmt.Errorf("usecase: CropsInGroup(%s): %w", in.GroupCode, err)
	}
	activeCropCodes := make(map[string]bool, len(crops))
	for _, c := range crops {
		if c.Active {
			activeCropCodes[c.CropCode] = true
		}
	}
	narrowToCropCodes(activeCropCodes, in.CropCodes)

	raw, err := u.repo.ObservedPricesByGroup(ctx, in.GroupCode, in.MarketCode, in.From, in.To, in.Now)
	if err != nil {
		return nil, fmt.Errorf("usecase: ObservedPricesByGroup(%s): %w", in.GroupCode, err)
	}

	points := make([]forecast.DailyVariantPrice, 0, len(raw))
	var maxAvailableAt *time.Time
	var sourceDigests []string
	for _, o := range raw {
		if !activeCropCodes[o.CropCode] || o.AvgPrice == nil {
			continue
		}
		points = append(points, forecast.DailyVariantPrice{
			MarketID: "moa:" + o.MarketCode, CropID: "moa:" + o.CropCode,
			TradeDate: forecast.DateFromTime(o.TradeDate), Price: *o.AvgPrice,
		})
		if !o.AvailableAt.IsZero() && (maxAvailableAt == nil || o.AvailableAt.After(*maxAvailableAt)) {
			at := o.AvailableAt
			maxAvailableAt = &at
		}
		if o.SourceDigest != "" {
			sourceDigests = append(sourceDigests, o.SourceDigest)
		}
	}

	originDate := forecast.DateFromTime(in.Now)
	vols, err := u.groupVolumes(ctx, in.GroupCode, originDate, in.Now)
	if err != nil {
		return nil, err
	}

	return &GroupObservedSeriesOutput{
		Points:         forecast.AggregateGroupObservations(points, vols),
		MaxAvailableAt: maxAvailableAt, SourceDigests: sourceDigests,
	}, nil
}

func narrowToCropCodes(active map[string]bool, subset []string) {
	if len(subset) == 0 {
		return
	}
	keep := make(map[string]bool, len(subset))
	for _, code := range subset {
		if active[code] {
			keep[code] = true
		}
	}
	for code := range active {
		if !keep[code] {
			delete(active, code)
		}
	}
}

func narrowCropRefs(crops []domain.CropRef, subset []string) []domain.CropRef {
	if len(subset) == 0 {
		return crops
	}
	keep := make(map[string]bool, len(subset))
	for _, code := range subset {
		keep[code] = true
	}
	out := make([]domain.CropRef, 0, len(subset))
	for _, c := range crops {
		if keep[c.CropCode] {
			out = append(out, c)
		}
	}
	return out
}
