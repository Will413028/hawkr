package usecase

import (
	"time"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/forecast"
)

// 常數值（issue #2249 Slice A `dataMeta`）：所有回應共用，本服務目前只有一個
// 上游來源、一種契約 schema 版本、一種計價幣別/單位——沒有任何一個是
// per-request 才決定的，所以直接常數，不做成可注入設定（那會是為沒有發生過
// 的情境加不必要的抽象層）。
const (
	dataMetaSource        = "MOA_WHOLESALE_DAILY"
	dataMetaSchemaVersion = "hawkr.pricebook/v1"
	dataMetaCurrency      = "TWD"
	dataMetaUnit          = "kg"
)

// DataMeta 是 issue #2249 Slice A 附加 `dataMeta` 物件的 usecase 層 DTO
// （go-native 型別；gRPC/proto 映射在 internal/grpc/handler.go 做，同本檔案
// 其餘 usecase→proto 映射的既有分層）。
//
// 缺值一律用 nil/空 slice，不用假的 0/空字串/"now" 冒充——這是 issue 本文
// 明講的規則，也是 CLAUDE.local.md「缺值使用 null/省略，永不補 0」慣例的
// 延伸。
type DataMeta struct {
	// SnapshotID 與 DatasetDigest 目前是同一個值（見 newDataMetaCommon 的
	// 說明）：本服務尚未有獨立於 dataset digest 之外的「snapshot」版本概念，
	// 沿用既有的內容定址雜湊比發明一個新的、未經測試的第二重識別軸更安全。
	SnapshotID            *string
	Source                string
	SourceDataAsOf        *time.Time
	MaxFeatureAvailableAt *time.Time
	GeneratedAt           time.Time
	FreshnessStatus       forecast.FreshnessLabel
	UsabilityStatus       forecast.UsabilityStatus
	QualityReasonCodes    []forecast.QualityReasonCode
	ModelVersion          *string
	DatasetDigest         *string
	SchemaVersion         string
	Currency              string
	Unit                  string
}

// newDataMetaCommon 組出所有路徑共用的欄位（source/schemaVersion/currency/
// unit/generatedAt 這五個常數/衍生值，以及 snapshotID←datasetDigest 的既定
// 映射），呼叫端只需要補上 freshness/usability/quality reason
// codes/modelVersion/sourceDataAsOf/maxFeatureAvailableAt/datasetDigest 這些
// 隨路徑而異的部分。
func newDataMetaCommon(now time.Time, datasetDigest *string) DataMeta {
	return DataMeta{
		SnapshotID:    datasetDigest,
		Source:        dataMetaSource,
		GeneratedAt:   now,
		SchemaVersion: dataMetaSchemaVersion,
		Currency:      dataMetaCurrency,
		Unit:          dataMetaUnit,
		DatasetDigest: datasetDigest,
	}
}

// observedDataMeta 是 GetObservedPrice 兩條路徑（crop 直查、group 聚合）共用
// 的 dataMeta 建構：freshnessStatus 重用既有 forecast.FreshnessConfig.
// Classify（不新開一條新鮮度邏輯），usabilityStatus 從同一個
// freshnessStatus 對映（見 forecast.ObservedUsabilityFromFreshness 的doc
// comment：觀測資料沒有 PredictionStatus/GroupPoint 概念，重新接一條
// forecast 管線才能算「真的」usability 訊號，正是 ADR 明講要避免的新機制）。
// qualityReasonCodes 在 observed 路徑固定回空 slice。modelVersion 固定
// nil——issue 規則明講「Observed 沒有 model 時可省略 modelVersion，不可填入
// forecast model」。
//
// sourceDataAsOfDate 是回傳列中最新的 trade_date（nil 代表沒有任何列）；
// maxAvailableAt 是回傳列中最大的 available_at（nil 語意同 Freshness.
// Classify 的 latestAvailableAt==nil：完全沒有可用觀測）；datasetDigest 已
// 依 Rule 3（cross-row policy）算好，呼叫端傳入。
func observedDataMeta(now time.Time, freshnessCfg forecast.FreshnessConfig, maxAvailableAt *time.Time, sourceDataAsOfDate *forecast.Date, datasetDigest *string) DataMeta {
	meta := newDataMetaCommon(now, datasetDigest)
	meta.MaxFeatureAvailableAt = maxAvailableAt
	if sourceDataAsOfDate != nil {
		t := forecast.SourceDataAsOfUTC(*sourceDataAsOfDate)
		meta.SourceDataAsOf = &t
	}
	meta.FreshnessStatus = freshnessCfg.Classify(now, maxAvailableAt)
	meta.UsabilityStatus = forecast.ObservedUsabilityFromFreshness(meta.FreshnessStatus)
	meta.QualityReasonCodes = nil
	meta.ModelVersion = nil
	return meta
}

// seriesForecastDataMeta 是 GetForecastCurve 非大類（單一 crop/series）路徑
// 的 dataMeta 建構。usabilityStatus 從既有 PredictionStatus（回應頂層
// status，也就是 horizon_days==1 那個點的 status）對映——不引入
// qualityReasonCodes 概念，這個階段固定回空 slice（同
// forecast.SeriesUsability 的 doc comment）。modelVersion 只有在頂層
// model_version 非空字串時才填（Rule：頂層 model_version 必須等於
// dataMeta.modelVersion；頂層在 target 完全 unavailable 時是空字串，對映到
// dataMeta.modelVersion=nil 而不是空字串，兩者在「沒有模型版本可言」這件事
// 上其實是同一個事實的兩種表達方式，不是不一致）。
func seriesForecastDataMeta(now time.Time, status forecast.PredictionStatus, modelVersion string, out *SeriesForecastOutput) DataMeta {
	var datasetDigest *string
	if out != nil && out.DatasetDigest != "" {
		d := "sha256:" + out.DatasetDigest
		datasetDigest = &d
	}
	meta := newDataMetaCommon(now, datasetDigest)
	if modelVersion != "" {
		meta.ModelVersion = &modelVersion
	}
	meta.QualityReasonCodes = nil
	meta.UsabilityStatus = forecast.SeriesUsability(status)
	if out == nil {
		meta.FreshnessStatus = forecast.FreshnessUnavailable
		return meta
	}
	meta.FreshnessStatus = out.Freshness
	meta.MaxFeatureAvailableAt = out.MaxFeatureAvailableAt
	if out.DataThroughDate != nil {
		t := forecast.SourceDataAsOfUTC(*out.DataThroughDate)
		meta.SourceDataAsOf = &t
	}
	return meta
}

// groupForecastDataMeta 是 GetForecastCurve/GetObservedPrice(group-aggregated
// history=true) 兩者的大類聚合路徑共用的 dataMeta 建構——usabilityStatus/
// qualityReasonCodes 純從 horizon_days==1 的 GroupPoint 算（ADR D2/D3，見
// forecast.GroupUsability），不建新機制。
func groupForecastDataMeta(now time.Time, horizon1 forecast.GroupPoint, minMarketSupport int, modelVersion string, out *GroupForecastOutput) DataMeta {
	var datasetDigest *string
	if out != nil && out.DatasetDigest != "" {
		d := "sha256:" + out.DatasetDigest
		datasetDigest = &d
	}
	meta := newDataMetaCommon(now, datasetDigest)
	if modelVersion != "" {
		meta.ModelVersion = &modelVersion
	}
	usability, reasons := forecast.GroupUsability(horizon1, minMarketSupport)
	meta.UsabilityStatus = usability
	meta.QualityReasonCodes = reasons
	if out == nil {
		meta.FreshnessStatus = forecast.FreshnessUnavailable
		return meta
	}
	meta.FreshnessStatus = out.Freshness
	meta.MaxFeatureAvailableAt = out.MaxFeatureAvailableAt
	if out.DataThroughDate != nil {
		t := forecast.SourceDataAsOfUTC(*out.DataThroughDate)
		meta.SourceDataAsOf = &t
	}
	return meta
}

// unavailableDataMeta 是「完全沒有東西可以描述」的 dataMeta——crop_name 根本
// 沒解析到、或呼叫端遇到 caller-contract 錯誤前置就短路的情形。
func unavailableDataMeta(now time.Time) DataMeta {
	meta := newDataMetaCommon(now, nil)
	meta.FreshnessStatus = forecast.FreshnessUnavailable
	meta.UsabilityStatus = forecast.UsabilityUnusable
	return meta
}

// observedRowsSourceDigests / observedRowsMaxAvailableAt /
// observedRowsLatestTradeDate 是三個從「實際要回傳的列」抽欄位的小 helper——
// 刻意都吃同一個 []domain.ObservedPrice，確保 dataMeta 描述的「同一個
// snapshot」就是 payload 本身回傳的那些列，不是另外查一次（Snapshot 一致性
// 先決條件）。

func observedRowsSourceDigests(rows []domain.ObservedPrice) []string {
	digests := make([]string, 0, len(rows))
	for _, r := range rows {
		digests = append(digests, r.SourceDigest)
	}
	return digests
}

func observedRowsMaxAvailableAt(rows []domain.ObservedPrice) *time.Time {
	var latest *time.Time
	for _, r := range rows {
		if r.AvailableAt.IsZero() {
			continue
		}
		if latest == nil || r.AvailableAt.After(*latest) {
			at := r.AvailableAt
			latest = &at
		}
	}
	return latest
}

func observedRowsLatestTradeDate(rows []domain.ObservedPrice) *forecast.Date {
	var latest *forecast.Date
	for _, r := range rows {
		d := forecast.DateFromTime(r.TradeDate)
		if latest == nil || d.After(*latest) {
			dd := d
			latest = &dd
		}
	}
	return latest
}

// trendSummaryDataMeta builds GetTrendSummary's response-level dataMeta.
// modelVersion is always nil — trend-summary is built entirely from
// observed history, never a forecast model (same rule observedDataMeta
// already applies; do not fill it just because the field exists on the
// shared struct).
//
// Rollup rule: usabilityStatus/freshnessStatus reflect the worst status
// among periodStatuses (a response covering N periods is only as
// trustworthy as its least-trustworthy period) — reuses the same
// unusable > degraded > usable priority GroupUsability already applies
// within a single point, folded across periods instead of across markets;
// no new priority scheme invented. Empty periodStatuses (zero periods in
// the response) is unusable, mirroring unavailableDataMeta.
func trendSummaryDataMeta(now time.Time, freshnessCfg forecast.FreshnessConfig, periodStatuses []forecast.PredictionStatus, maxAvailableAt *time.Time, latestTradeDate *forecast.Date, datasetDigest *string) DataMeta {
	meta := newDataMetaCommon(now, datasetDigest)
	meta.MaxFeatureAvailableAt = maxAvailableAt
	if latestTradeDate != nil {
		t := forecast.SourceDataAsOfUTC(*latestTradeDate)
		meta.SourceDataAsOf = &t
	}
	meta.FreshnessStatus = freshnessCfg.Classify(now, maxAvailableAt)
	meta.ModelVersion = nil
	meta.QualityReasonCodes = nil

	if len(periodStatuses) == 0 {
		meta.UsabilityStatus = forecast.UsabilityUnusable
		return meta
	}
	meta.UsabilityStatus = worstUsability(periodStatuses, meta.FreshnessStatus)
	return meta
}

// worstUsability maps each period's PredictionStatus to the same
// GroupUsability-style single-point classification (ready → usable, partial
// → degraded, everything else → unusable — matching GroupUsability's
// default-branch treatment of partial, not SeriesUsability's, which treats
// partial as usable), then takes the worst across all periods (unusable >
// degraded > usable) — the priority order GroupUsability's own doc comment
// already establishes for one point, applied here across many.
func worstUsability(periodStatuses []forecast.PredictionStatus, freshness forecast.FreshnessLabel) forecast.UsabilityStatus {
	worst := forecast.UsabilityUsable
	downgrade := func(u forecast.UsabilityStatus) {
		switch {
		case u == forecast.UsabilityUnusable:
			worst = forecast.UsabilityUnusable
		case u == forecast.UsabilityDegraded && worst != forecast.UsabilityUnusable:
			worst = forecast.UsabilityDegraded
		}
	}
	for _, s := range periodStatuses {
		switch s {
		case forecast.PredictionStatusReady:
			downgrade(forecast.UsabilityUsable)
		case forecast.PredictionStatusPartial:
			downgrade(forecast.UsabilityDegraded)
		default: // insufficient_data, market_closed, or any future status
			downgrade(forecast.UsabilityUnusable)
		}
	}
	// A stale/unavailable overall freshness downgrades usability the same
	// way ObservedUsabilityFromFreshness already does for the non-group
	// observed path — trend-summary's per-period statuses only capture
	// data-completeness, not staleness, so freshness must still be folded
	// in separately.
	switch freshness {
	case forecast.FreshnessStale:
		if worst == forecast.UsabilityUsable {
			worst = forecast.UsabilityDegraded
		}
	case forecast.FreshnessUnavailable:
		worst = forecast.UsabilityUnusable
	}
	return worst
}
