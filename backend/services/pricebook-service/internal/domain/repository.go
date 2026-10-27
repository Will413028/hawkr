package domain

import (
	"context"
	"time"
)

// Observation 是一筆已解過 revision 的日行情觀測。
//
// AvgPrice / VolumeKg 是 *float64：nil 代表 NULL（休市／無成交／缺值），
// 與 0（合法價格）在型別上無法互相冒充。從 NUMERIC 到 float64 的轉換只在
// repository 出口做一次——特徵數學與 Python 的契約就是 float64
// （見 internal/repository/postgres/numeric.go 對「不要經過 float64」的說明：
// 那條規則保護的是**儲存**往返，不是計算）。
type Observation struct {
	MarketCode   string
	CropCode     string
	CategoryCode *string
	MarketActive bool
	CropActive   bool
	TradeDate    time.Time
	AvgPrice     *float64
	VolumeKg     *float64
	AvailableAt  time.Time
	RevisionNo   int32
}

// RestDay 是某市場某營業類別的一個休市日。
type RestDay struct {
	MarketCode string
	MarketType string
	RestDate   time.Time
}

// RestDayCoverage 是「我方對某 (市場, 營業類別, 年) 有休市日曆」的事實。
type RestDayCoverage struct {
	MarketCode string
	MarketType string
	Year       int
}

// CropRef 是大類底下的一個子品項。
type CropRef struct {
	CropCode     string
	Name         string
	VariantName  *string
	CategoryCode *string
	Active       bool
}

// CropMarketVolume 是大類加權用的成交量。
type CropMarketVolume struct {
	MarketCode string
	CropCode   string
	Volume     float64
}

// MarketVolume 是單一品項（非大類）路徑用的成交量——選定的 crop_code 已經
// 是查詢條件本身，不像 CropMarketVolume 還要帶著 CropCode 供呼叫端依品項
// 拆分。issue #2248 spatial-surface 的 statistics.p50/minimum_p50/
// maximum_p50 跨市場加權用（2026-08-08 加）。
type MarketVolume struct {
	MarketCode string
	Volume     float64
}

// ObservedPrice 是一筆原始觀測價格（給 GetObservedPrice 用，含全部四個價格
// 欄位）。與 Observation 的差別只是多了 UpperPrice/MiddlePrice/LowerPrice——
// P4 的推論從不需要這三欄，刻意不擴充 Observation 本身（零 blast radius 對
// 已高度測試過的推論路徑）。
//
// Source/SourceDigest/RevisionNo/AvailableAt（issue #2249 Slice A dataMeta
// 前置，2026-08-07 加）：price_observation 表原本就有這四欄，這裡補上讓
// usecase 層能算 dataMeta.source/datasetDigest（依 Rule 3 跨列政策）與
// freshnessStatus（用 MAX(AvailableAt) 餵 forecast.FreshnessConfig.
// Classify，同 SeriesForecast/GroupForecast 既有算法，不新開一條）。
// CategoryCode（issue #2249 Slice B trend-summary 前置，2026-08-07 加）：
// crop.category_code，供 period-completeness 判斷用（見 domain/forecast/trend.go
// ExpectedTradingDay）——同一批新增理由同上方 Source/SourceDigest 等欄位：
// 這支查詢是唯一能取得「實際回傳列」真身分的地方。
type ObservedPrice struct {
	MarketCode   string
	CropCode     string
	CategoryCode *string
	TradeDate    time.Time
	AvgPrice     *float64
	UpperPrice   *float64
	MiddlePrice  *float64
	LowerPrice   *float64
	VolumeKg     *float64
	Source       string
	SourceDigest string
	RevisionNo   int32
	AvailableAt  time.Time
}

// CropGroupMatch 是「大類名稱 → crop_group 列」的一筆命中。可能不唯一——見
// CropGroupsByName 的說明。
type CropGroupMatch struct {
	GroupCode    string
	Name         string
	CategoryCode *string
}

// MarketRef is a market's display name, keyed by its code.
type MarketRef struct {
	MarketCode string
	Name       string
}

// CatalogSearchRow is one hit from ForecastRepository.GroupCropOptionsByRecentVolume — either
// a crop_group row (MatchKind=="group", CropCode/CropName/VariantName all
// empty) or a crop row (MatchKind=="crop", all fields set). This is a raw DB
// hit, before SearchCropCatalog's usecase layer merges in seed-table alias
// matches, has_forecast, and per-market options.
type CatalogSearchRow struct {
	MatchKind   string
	GroupCode   string
	GroupName   string
	CropCode    string
	CropName    string
	VariantName string
}

// ForecastRepository 是推論所需的全部讀取。
//
// 所有帶 cutoff 的方法都是 point-in-time 查詢：只看 available_at <= cutoff 的
// 列，並在同一 (market, crop, trade_date) 取最大 revision_no。
type ForecastRepository interface {
	// DailyObservedPrices batches active crops into one NUMERIC aggregate per date; marketCode empty aggregates all active markets.
	DailyObservedPrices(ctx context.Context, cropCodes []string, marketCode string, from, to, cutoff time.Time) ([]DailyObservedPrice, error)
	SeriesObservations(ctx context.Context, marketCode, cropCode string, from, to time.Time, cutoff time.Time) ([]Observation, error)
	// GroupObservations 的 marketCode 傳空字串代表不過濾市場。
	GroupObservations(ctx context.Context, groupCode, marketCode string, from, to time.Time, cutoff time.Time) ([]Observation, error)
	RestDays(ctx context.Context, marketCode string, from, to time.Time) ([]RestDay, error)
	RestDayCoverage(ctx context.Context, marketCode string) ([]RestDayCoverage, error)
	CropsInGroup(ctx context.Context, groupCode string) ([]CropRef, error)
	// CropByCode looks up a single crop's catalog data by its crop_code,
	// independent of whether any price_observation exists for it — a bare
	// crop-table lookup, unlike ObservedPricesByCrop. TrendSummary's crop
	// path (issue #2249 Slice B) needs this: ExpectedTradingDay's
	// market_closed-vs-insufficient_data distinction is keyed by
	// categoryCode, and a genuinely closed market produces ZERO
	// price_observation rows for its crops (NewForecastRepository's own doc
	// comment: price_observation carries no closed-market sentinel rows,
	// closure info lives entirely in market_rest_day) — deriving
	// categoryCode only from observed rows would make market_closed
	// permanently unreachable whenever a period has no data at all,
	// collapsing exactly the distinction this method exists to preserve.
	// Returns CropRef{} zero value with no error if the code doesn't exist,
	// same "resolution miss is a business outcome" convention as
	// MarketByCode. CropRef.Active is always false (the underlying query
	// does not select crop.active) — this method is for category_code only;
	// callers needing Active semantics use CropsInGroup instead.
	CropByCode(ctx context.Context, cropCode string) (CropRef, error)
	RecentVolume(ctx context.Context, groupCode string, from, to time.Time, cutoff time.Time) ([]CropMarketVolume, error)
	// RecentVolumeForCrop is RecentVolume's single-crop counterpart — same
	// window/cutoff/latest-revision-per-day semantics (db/queries/
	// forecast.sql's RecentVolumeByCrop mirrors RecentVolumeByCropMarket
	// exactly, minus the crop_group JOIN), for callers that already have one
	// crop_code and have no group to weight sub-variants within (issue #2248
	// spatial-surface's crop-path cross-market statistics, 2026-08-08 加).
	RecentVolumeForCrop(ctx context.Context, cropCode string, from, to time.Time, cutoff time.Time) ([]MarketVolume, error)
	// ObservedPricesByCrop / ObservedPricesByGroup: raw observed-price
	// lookups for GetObservedPrice. from/to bound which trade_dates to
	// search; cutoff bounds which revisions are visible "as of" that
	// instant — same point-in-time semantics as SeriesObservations/
	// GroupObservations's own cutoff parameter. cutoff must NOT be derived
	// from `to`: when the caller pins an exact trade_date, `to` collapses to
	// that (possibly past) date, but available_at (ingest/backfill time) can
	// legitimately lag trade_date by more than a day — see
	// db/queries/observation.sql's ObservedPriceByCrop comment (review
	// Finding I1, 2026-07-31). The usecase layer decides the [from,to]
	// window and reduces to "latest per (market,crop)" if the request didn't
	// pin an exact trade_date.
	ObservedPricesByCrop(ctx context.Context, marketCode, cropCode string, from, to, cutoff time.Time) ([]ObservedPrice, error)
	ObservedPricesByGroup(ctx context.Context, groupCode, marketCode string, from, to, cutoff time.Time) ([]ObservedPrice, error)
	// CropGroupsByName resolves a crop_group name to its (possibly
	// multiple, if ambiguous — see FindCropGroupByName's own comment in
	// forecast.sql) matching rows.
	CropGroupsByName(ctx context.Context, name string) ([]CropGroupMatch, error)
	// MarketByCode looks up a market's display name by code. Returns
	// domain.MarketRef{} zero value with no error if the code doesn't
	// exist — callers treat "unknown market code" as "no display name
	// available", not an error (mirrors the rest of this interface's
	// "resolution miss is a business outcome" philosophy).
	MarketByCode(ctx context.Context, marketCode string) (MarketRef, error)
	// MarketsByCode is the batched form of MarketByCode: one round trip for
	// a whole set of codes, for callers that label many markets at once
	// (SearchCropCatalog labels every market of every crop hit — doing that
	// one code at a time is a per-keystroke DB fan-out; review Finding I3).
	// Codes with no market row are simply absent from the result — the same
	// "resolution miss is a business outcome" stance as the singular method
	// (which returns a zero value), so callers keep their own fallback
	// (display the code itself) instead of getting a fake empty-name row.
	// Order is not guaranteed; callers key by MarketCode.
	MarketsByCode(ctx context.Context, marketCodes []string) ([]MarketRef, error)
	// GroupCropOptionsByRecentVolume returns one crop_group's active crops
	// (MatchKind always "crop"), ordered by how many price_observation rows
	// they have in the trailing windowDays, most first, with crop_code as a
	// stable tiebreak. Crops with no recent rows are kept and sort last —
	// the ordering is a popularity proxy, not an availability filter.
	//
	// An unknown or inactive groupCode returns an empty slice, not an error:
	// the same "a resolution miss is a business outcome" stance the rest of
	// this interface takes.
	GroupCropOptionsByRecentVolume(ctx context.Context, groupCode string, windowDays, limit int32) ([]CatalogSearchRow, error)
}
