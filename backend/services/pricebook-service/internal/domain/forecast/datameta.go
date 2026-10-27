package forecast

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"time"
)

// taipeiFixedOffset 是台北固定 UTC+8 時區——台灣現行不施行日光節約時間，固定
// offset 不需要 tzdata 查表（比照 date.go 的 cachedLoadLocation 快取動機，但
// 這裡連查表都省了）。刻意不重用 calendarTimezoneOf（usecase 層讀 artifact
// metadata 的那份）：sourceDataAsOf 描述的是「MOA 來源交易日的日終」，跟模型
// artifact 宣告的 calendar_timezone（休市日曆比對用）是兩個不同的關注點——
// 即使兩者現在剛好都是 Asia/Taipei，語意上不應該共用同一個讀法。
var taipeiFixedOffset = time.FixedZone("Asia/Taipei", 8*60*60)

// SourceDataAsOfUTC 把「來源資料截止」的日期換算成該台北交易日日終
// （23:59:59 台北時間）的 UTC 時刻。回傳值固定是 UTC time.Time——wire format
// 上一律序列化成 "...Z"，不是 issue 範例裡的 "+08:00"（docs/開發規範/Java 微服務
// 開發規範.md §2：時間一律 UTC，API 進出 YYYY-MM-DDTHH:mm:ssZ；issue 範例只是
// 人類可讀示意，不能照抄成實際 wire format）。
func SourceDataAsOfUTC(d Date) time.Time {
	return time.Date(d.Year, time.Month(d.Month), d.Day, 23, 59, 59, 0, taipeiFixedOffset).UTC()
}

// UsabilityStatus 是 issue #2249 Slice A `dataMeta.usabilityStatus` 的三態值。
// 沿用既有 PredictionStatus/FreshnessLabel 一樣的「外層小寫字面值」慣例
// （見 status.go 檔頭註解）——這組值同樣預計隨 Plan C 進 CodeMeta。
type UsabilityStatus string

const (
	UsabilityUsable   UsabilityStatus = "usable"
	UsabilityDegraded UsabilityStatus = "degraded"
	UsabilityUnusable UsabilityStatus = "unusable"
)

// QualityReasonCode 是大類聚合路徑 `dataMeta.qualityReasonCodes` 的原因碼。
// 與 MissingReason 不同：這組值全新（無既有消費者、無 grandfather 需求），依
// docs/代碼管理（CodeMeta）/業務狀態碼.md 的一般慣例採 UPPER_SNAKE_CASE
// value——對照 PredictionStatus/FreshnessLabel 因既有前端契約被迫沿用小寫值
// 是唯二的例外，這組不是例外。
type QualityReasonCode string

const (
	QualityReasonPartialVariantCoverage    QualityReasonCode = "PARTIAL_VARIANT_COVERAGE"
	QualityReasonVolumeWeightFallback      QualityReasonCode = "VOLUME_WEIGHT_FALLBACK"
	QualityReasonInsufficientMarketSupport QualityReasonCode = "INSUFFICIENT_MARKET_SUPPORT"
)

// GroupUsability 實作 ADR
// 私人封存的 ADR 2026-08-07-pricebook-group-readiness-gate-resolution
// 的 D2/D3：純從既有 GroupPoint 欄位（Status/AggregationMethod/
// MarketSupportCount）推導大類聚合路徑的 usabilityStatus/qualityReasonCodes，
// 不建任何新的品質閘門機制。
//
// 三個 quality reason code 各自獨立判斷（不是 if/elif chain）：一個回應可能
// 同時帶多個 code（例如同時 partial 又 equal-weight-fallback）。這些條件的
// 判定與最終 usability 分類（unusable 優先於 usable 優先於 degraded）彼此獨立
// ——即使最終判定是 unusable（例如 freshness 覆寫後的 Status=unavailable），
// AggregationMethod/MarketSupportCount 若仍呈現 degraded 訊號，對應 code 依然
// 回傳：這是除錯用的輔助資訊，不是使用者要不要信任這筆資料的唯一依據
// （unusable 本身已經夠清楚地說「不要用」）。
//
// 判定優先序（見 ADR Decision 段）：
//  1. unusable：Status 為 insufficient_data/market_closed/unavailable，或
//     MarketSupportCount == 0。
//  2. usable：Status == ready 且非 equal-weight-fallback 且
//     MarketSupportCount >= minMarketSupport。
//  3. 其餘（partial、fallback、或 0 < MarketSupportCount < minMarketSupport）
//     → degraded。
//
// minMarketSupport 呼叫端傳入 config.GroupMinMarketSupportCount（ADR D3 的
// 暫定值 2，待正式確認）。
func GroupUsability(p GroupPoint, minMarketSupport int) (UsabilityStatus, []QualityReasonCode) {
	partialCoverage := p.Status == PredictionStatusPartial
	equalWeightFallback := p.AggregationMethod == AggregationMethodEqualWeightFallback
	insufficientMarketSupport := p.MarketSupportCount > 0 && p.MarketSupportCount < minMarketSupport

	var reasons []QualityReasonCode
	if partialCoverage {
		reasons = append(reasons, QualityReasonPartialVariantCoverage)
	}
	if equalWeightFallback {
		reasons = append(reasons, QualityReasonVolumeWeightFallback)
	}
	if insufficientMarketSupport {
		reasons = append(reasons, QualityReasonInsufficientMarketSupport)
	}

	switch {
	case p.Status == PredictionStatusInsufficientData || p.Status == PredictionStatusMarketClosed || p.Status == PredictionStatusUnavailable || p.MarketSupportCount == 0:
		return UsabilityUnusable, reasons
	case p.Status == PredictionStatusReady && !equalWeightFallback && p.MarketSupportCount >= minMarketSupport:
		return UsabilityUsable, reasons
	default:
		return UsabilityDegraded, reasons
	}
}

// SeriesUsability 是「非大類聚合」路徑（單一 crop/series）的
// usabilityStatus 推導：純從既有 PredictionStatus 對映，不引入任何新概念
// （這個階段沒有 qualityReasonCodes 概念可用，呼叫端一律回空 slice）。
//
// ready/partial → usable；stale → degraded；其餘（insufficient_data/
// market_closed/unavailable/not_predicted，以及任何未來新增的未知值）→
// unusable，保守處理——同 OuterStatus 未知內層狀態時的「預設值必須保守」
// 慣例（status.go）：預設 usable 會讓一個沒人想過的狀態直接告訴賣家這筆資料
// 能信。
func SeriesUsability(status PredictionStatus) UsabilityStatus {
	switch status {
	case PredictionStatusReady, PredictionStatusPartial:
		return UsabilityUsable
	case PredictionStatusStale:
		return UsabilityDegraded
	default:
		return UsabilityUnusable
	}
}

// ObservedUsabilityFromFreshness 是 GetObservedPrice 路徑（crop 與 group 皆
// 適用）的 usabilityStatus 推導。
//
// 觀測資料沒有 PredictionStatus 概念（那是預測特有的狀態機），也沒有
// GroupPoint 等價物——大類聚合觀測（GroupObservedSeries/
// AggregateGroupObservations）刻意不跑 forecast/SevenDays 管線（見
// group.go AggregateGroupObservations 的doc comment：不用 model 覆蓋率過濾，
// 才能涵蓋模型不 cover 的作物）。若為了算 usabilityStatus 額外呼叫
// GroupForecast，等於為觀測路徑重新接一條完整的預測管線，正是 ADR D2/D3
// 明講「不建新 gate 機制」要避免的事，而且會讓 dataMeta 描述的東西
// 跟payload 的 snapshot 脫鉤（觀測列與當下的模型/artifact 快照是兩件事）。
//
// 因此改用觀測資料唯一真正擁有的品質信號——FreshnessLabel（本來就要為
// freshnessStatus 算一次，見 usecase 層的呼叫點）——做直接對映：
// fresh → usable；stale → degraded；unavailable（無資料或資料過舊）→
// unusable。qualityReasonCodes 在這條路徑上固定回空 slice，理由同
// SeriesUsability：没有可信賴的大類品質訊號可用，不無中生有。
func ObservedUsabilityFromFreshness(f FreshnessLabel) UsabilityStatus {
	switch f {
	case FreshnessFresh:
		return UsabilityUsable
	case FreshnessStale:
		return UsabilityDegraded
	default: // FreshnessUnavailable，或任何未來新增的未知值——保守處理
		return UsabilityUnusable
	}
}

// datasetDigestSeparator 是 Rule 3（cross-row datasetDigest policy）的固定
// join 分隔符。用不會出現在 64 位小寫 hex 摘要裡的字元，避免不同排列組合的
// 摘要集合意外撞出同一個 joined 字串（雖然 hex 字元集本來就不含 "|"，加上
// 這個分隔符只是讓這件事顯式、不依賴「剛好沒有欄位含這個字元」的巧合）。
const datasetDigestSeparator = "|"

// DatasetDigestFromSourceDigests 實作 issue #2249 Slice A 的
// datasetDigest 跨列政策：取實際回傳列的 distinct source_digest 集合，
// 依字典序排序、以固定分隔符 join、sha256、加上既有 "sha256:" 前綴慣例
// （與 forecast.ModelArtifact.DatasetDigest 的既有格式一致，isSHA256Hex
// 檢查的即是加上前綴之前那 64 位小寫 hex 的部分）。
//
// 空輸入或全空字串 → nil（沒有列可以描述，不可省略成假摘要）。
// 恰一個 distinct digest → 直接回那個 digest 本身（不重新 hash一次）——
// 這是最常見的單一 crop/market/date 查詢情形，也是 Rule 3 明講「應收斂回
// 原始摘要」的要求：composite hash 只在真的跨多個 distinct digest 時才
// 有意義，單一輸入時 hash 自己只會製造一個無法從既有列反查、也無法與
// ingest 端 source_digest 直接比對的新值。
func DatasetDigestFromSourceDigests(digests []string) *string {
	seen := make(map[string]bool, len(digests))
	var distinct []string
	for _, d := range digests {
		if d == "" || seen[d] {
			continue
		}
		seen[d] = true
		distinct = append(distinct, d)
	}
	if len(distinct) == 0 {
		return nil
	}
	sort.Strings(distinct)

	if len(distinct) == 1 {
		out := "sha256:" + distinct[0]
		return &out
	}

	sum := sha256.Sum256([]byte(strings.Join(distinct, datasetDigestSeparator)))
	out := "sha256:" + hex.EncodeToString(sum[:])
	return &out
}
