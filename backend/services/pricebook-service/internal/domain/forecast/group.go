package forecast

import (
	"fmt"
	"math"
	"sort"
)

// 大類聚合方法標籤（設計書 §8 line 392）。上游只有一個寫死 42 天的字串且在
// 權重全 0 時靜默退化；§8 要求把方法回給 UI 說明數字來源，所以退化必須有標籤。
const (
	AggregationMethodVolumeWeightedTwoStage = "volume_weighted_two_stage"
	AggregationMethodEqualWeightFallback    = "equal_weight_fallback"
)

// DefaultGroupVolumeWindowDays 是計算加權用成交量的回看天數（設計書 §8 line 386-387）。
// 上游用 42（api/catalog.py:1013），設計書寫 28。照設計書，並做成 config
// （GROUP_VOLUME_WINDOW_DAYS）讓值可調而不必動 code。
const DefaultGroupVolumeWindowDays = 28

// VariantMarketKey 是 (子品項, 市場) 的複合鍵。
type VariantMarketKey struct {
	CropID   string
	MarketID string
}

// VariantPoint 是一個子品項在一個 (市場, horizon) 上、已經過外層狀態映射的點。
type VariantPoint struct {
	MarketID      string
	CropID        string
	HorizonDays   int
	TargetDate    Date
	Status        PredictionStatus
	MissingReason MissingReason
	P10           *float64
	P50           *float64
	P90           *float64
}

// GroupVolumes 是加權用的成交量。兩層各一份，對應兩個 stage。
type GroupVolumes struct {
	// ByVariantMarket 是 stage 1 的權重：某子品項在某市場的近 N 日成交量。
	ByVariantMarket map[VariantMarketKey]float64
	// ByMarket 是 stage 2 的權重：某市場在本大類所有子品項上的近 N 日總成交量。
	ByMarket map[string]float64
}

// GroupPoint 是大類在一個 horizon 上的聚合結果。
type GroupPoint struct {
	HorizonDays   int
	TargetDate    Date
	Status        PredictionStatus
	MissingReason MissingReason

	AggregationMethod     string
	EffectiveVariantCount int
	TotalVariantCount     int
	MarketSupportCount    int

	P10 *float64
	P50 *float64
	P90 *float64
}

// AggregateGroup 把子品項的點聚合成大類的七日點（設計書 §8 line 382-395）。
//
// 兩階段：
//  1. 市場內：同一市場的各子品項，以「該子品項在該市場」的近 N 日成交量為權重。
//  2. 跨市場：各市場的大類價格，以「該市場」的近 N 日總成交量為權重。
//
// 只有 Status == ready 且三個分位數齊全的子品項參與；被擋掉的**不以 0 計入**
// （以 0 計入會把大類價格拉向 0，那是比缺值更糟的錯誤）。
// P10／P50／P90 各自獨立加權，不從 P50 推導（§8 line 395）——同一組參與者、
// 同一組權重，所以單調性在加權後保持。
//
// totalVariantCount 是本大類「應有」的子品項數（呼叫端從 crop 表數），用來判
// partial：effective 介於 1 和 total-1 之間時回 partial。
func AggregateGroup(points []VariantPoint, vols GroupVolumes, totalVariantCount int) ([]GroupPoint, error) {
	if err := validateWeights(vols); err != nil {
		return nil, err
	}
	byHorizon := map[int][]VariantPoint{}
	dateByHorizon := map[int]Date{}
	for _, p := range points {
		if existing, ok := dateByHorizon[p.HorizonDays]; ok && existing != p.TargetDate {
			return nil, fmt.Errorf("forecast: horizon %d has conflicting target dates %s and %s", p.HorizonDays, existing, p.TargetDate)
		}
		dateByHorizon[p.HorizonDays] = p.TargetDate
		byHorizon[p.HorizonDays] = append(byHorizon[p.HorizonDays], p)
	}
	// 與 EvaluateHorizonSet（Task 7）同一條不變量，但型別不同所以各自實作：
	// merge 多個子品項的列是「七日缺日或重複」唯一真正可達的路徑
	// （SevenDays 的 for horizon := 1; horizon <= 7 結構上不可能缺或重複）。
	seenDate := map[Date]bool{}
	for horizon := 1; horizon <= 7; horizon++ {
		if len(byHorizon[horizon]) == 0 {
			return nil, fmt.Errorf("forecast: group aggregation input is missing horizon %d (all of 1..7 required)", horizon)
		}
		d := dateByHorizon[horizon]
		if seenDate[d] {
			return nil, fmt.Errorf("forecast: group aggregation input has duplicate target date %s", d)
		}
		seenDate[d] = true
	}
	if len(byHorizon) != 7 {
		return nil, fmt.Errorf("forecast: group aggregation input has %d horizons, want exactly 7", len(byHorizon))
	}

	out := make([]GroupPoint, 0, 7)
	for horizon := 1; horizon <= 7; horizon++ {
		out = append(out, aggregateOneHorizon(horizon, dateByHorizon[horizon], byHorizon[horizon], vols, totalVariantCount))
	}
	return out, nil
}

// DailyVariantPrice is one series' actual observed price on one calendar
// date — the observed-history analog of VariantPoint, which carries a
// forecast horizon's P10/P50/P90 triple instead of a single realized price.
type DailyVariantPrice struct {
	MarketID  string
	CropID    string
	TradeDate Date
	Price     float64
	// Volume（issue #2249 Slice B trend-summary 前置，2026-08-07 加；型別
	// 2026-08-08 review 修正為 *float64——原本的 float64 把「該列 volume_kg
	// 是 NULL（真正未知）」與「該列有量資料、但確實是 0」兩件不同的事塌縮成
	// 同一個 0，任何呼叫端都無法分辨，違反本 feature「never fabricate 0 for
	// unknown」的核心規則）：nil 代表該列 volume_kg 是 SQL NULL（未知，不可
	// 當 0 計入加總）；non-nil 0.0 代表該列確實有價格資料、量真的是 0（合法、
	// 只是罕見）。aggregateOneDate 只加總非 nil 的列，且僅當該日至少有一列
	// 非 nil 才回傳 non-nil 的 GroupObservedPoint.Volume——這是本 struct 唯一
	// 的建構點在 usecase 層（trend_summary.go groupPath），直接傳
	// r.VolumeKg（*float64），不做任何 nil→0 轉換。
	Volume *float64
}

// GroupObservedPoint is a group's two-stage-weighted actual price on one
// calendar date — the observed-history analog of GroupPoint.
type GroupObservedPoint struct {
	TradeDate Date
	Value     *float64
	// Volume（同上）：該日所有「貢獻列中 Volume 非 nil 者」的加總。nil 有兩
	// 種成因，刻意不再細分（呼叫端不需要、也無法從聚合後的單一數字反推是哪
	// 一種）：(a) 當日零參與者（鏡射 Value 的 nil 語意），或 (b) 當日有參與
	// 者、有 Value，但每一列的 Volume 都是 NULL（真正未知）。non-nil 但為
	// 0.0 代表當日確實有至少一列非 nil 的 Volume，加總後為 0（合法、只是
	// 罕見）——這與「一筆都不知道」是不同的事實，不可合併成同一個 0。
	Volume *float64
}

// AggregateGroupObservations applies the identical two-stage volume
// weighting AggregateGroup uses for forecasts (stage 1: within each market,
// weight the crops present that day by their ByVariantMarket volume; stage
// 2: across markets, weight each market's stage-1 result by its ByMarket
// total) to a group's raw daily observed prices — producing one aggregated
// point per calendar date, instead of AggregateGroup's fixed 7 horizons.
//
// Deliberately does NOT filter by artifact.Series / HasSeries the way
// GroupForecast's relevantSeries filtering does: "what actually traded" is
// not the same question as "what the model can forecast" — a crop with no
// trained series still contributes its real observed price here.
func AggregateGroupObservations(points []DailyVariantPrice, vols GroupVolumes) []GroupObservedPoint {
	byDate := map[Date][]DailyVariantPrice{}
	for _, p := range points {
		byDate[p.TradeDate] = append(byDate[p.TradeDate], p)
	}
	dates := make([]Date, 0, len(byDate))
	for d := range byDate {
		dates = append(dates, d)
	}
	sort.Slice(dates, func(i, j int) bool { return dates[i].Before(dates[j]) })

	out := make([]GroupObservedPoint, 0, len(dates))
	for _, d := range dates {
		out = append(out, aggregateOneDate(d, byDate[d], vols))
	}
	return out
}

func aggregateOneDate(date Date, points []DailyVariantPrice, vols GroupVolumes) GroupObservedPoint {
	// Stage 1: within each market, weight the crops present that day by
	// their (crop, market) volume — mirrors aggregateOneHorizon's byMarket
	// grouping exactly, just on a single Price instead of a P10/P50/P90
	// triple.
	byMarket := map[string][]DailyVariantPrice{}
	for _, p := range points {
		byMarket[p.MarketID] = append(byMarket[p.MarketID], p)
	}
	marketIDs := make([]string, 0, len(byMarket))
	for marketID := range byMarket {
		marketIDs = append(marketIDs, marketID)
	}
	sort.Strings(marketIDs) // deterministic order — floating sums are order-sensitive

	stageOneValues := make([]float64, 0, len(marketIDs))
	stageOneWeights := make([]float64, 0, len(marketIDs))
	for _, marketID := range marketIDs {
		participants := byMarket[marketID]
		values := make([]float64, len(participants))
		weights := make([]float64, len(participants))
		for i, p := range participants {
			values[i] = p.Price
			weights[i] = vols.ByVariantMarket[VariantMarketKey{CropID: p.CropID, MarketID: p.MarketID}]
		}
		avg, _ := WeightedAverage(values, weights)
		stageOneValues = append(stageOneValues, avg)
		stageOneWeights = append(stageOneWeights, vols.ByMarket[marketID])
	}

	// Stage 2: across markets.
	value, _ := WeightedAverage(stageOneValues, stageOneWeights)
	if len(stageOneValues) == 0 {
		return GroupObservedPoint{TradeDate: date, Value: nil, Volume: nil}
	}
	// Sum only the rows whose Volume is non-nil (a real number, possibly
	// 0.0) — a nil Volume means that row's volume_kg is genuinely unknown
	// (SQL NULL) and must not be counted as a 0 contribution. If NONE of the
	// day's contributing rows had a known Volume, the aggregate Volume is
	// nil too, not a fabricated &0.0 (see GroupObservedPoint.Volume's own
	// doc comment).
	totalVolume := 0.0
	haveVolume := false
	for _, p := range points {
		if p.Volume == nil {
			continue
		}
		totalVolume += *p.Volume
		haveVolume = true
	}
	var volume *float64
	if haveVolume {
		volume = &totalVolume
	}
	return GroupObservedPoint{TradeDate: date, Value: &value, Volume: volume}
}

func validateWeights(vols GroupVolumes) error {
	for k, v := range vols.ByVariantMarket {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return fmt.Errorf("forecast: non-finite volume weight for crop %s in market %s", k.CropID, k.MarketID)
		}
	}
	for marketID, v := range vols.ByMarket {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return fmt.Errorf("forecast: non-finite volume weight for market %s", marketID)
		}
	}
	return nil
}

func aggregateOneHorizon(horizon int, targetDate Date, points []VariantPoint, vols GroupVolumes, totalVariantCount int) GroupPoint {
	result := GroupPoint{
		HorizonDays: horizon, TargetDate: targetDate,
		TotalVariantCount: totalVariantCount,
	}

	type marketAgg struct {
		p10, p50, p90 float64
		weight        float64
	}
	perMarket := map[string]*marketAgg{}
	effectiveCrops := map[string]bool{}
	fellBack := false

	// stage 1：市場內，以子品項成交量為權重
	byMarket := map[string][]VariantPoint{}
	for _, p := range points {
		if p.Status != PredictionStatusReady || p.P10 == nil || p.P50 == nil || p.P90 == nil {
			continue
		}
		byMarket[p.MarketID] = append(byMarket[p.MarketID], p)
		effectiveCrops[p.CropID] = true
	}
	for marketID, participants := range byMarket {
		weights := make([]float64, len(participants))
		for i, p := range participants {
			weights[i] = vols.ByVariantMarket[VariantMarketKey{CropID: p.CropID, MarketID: p.MarketID}]
		}
		p10, fb10 := WeightedAverage(collect(participants, func(p VariantPoint) float64 { return *p.P10 }), weights)
		p50, fb50 := WeightedAverage(collect(participants, func(p VariantPoint) float64 { return *p.P50 }), weights)
		p90, fb90 := WeightedAverage(collect(participants, func(p VariantPoint) float64 { return *p.P90 }), weights)
		fellBack = fellBack || fb10 || fb50 || fb90
		perMarket[marketID] = &marketAgg{p10: p10, p50: p50, p90: p90, weight: vols.ByMarket[marketID]}
	}

	result.EffectiveVariantCount = len(effectiveCrops)
	result.MarketSupportCount = len(perMarket)

	if result.EffectiveVariantCount == 0 {
		// §8 line 393 只寫「有效子品項數為 0 → insufficient_data」，但把「這個
		// 市場今天全休市」說成「資料不足」是錯的解釋。照上游的區分
		// （api/catalog.py:1092-1103 的 all_closed 分支）：全部子品項都是休市 →
		// 大類也是休市。
		allClosed := len(points) > 0
		for _, p := range points {
			if p.Status != PredictionStatusMarketClosed {
				allClosed = false
				break
			}
		}
		if allClosed {
			result.Status = PredictionStatusMarketClosed
			result.MissingReason = MissingReasonMarketClosed
		} else {
			result.Status = PredictionStatusInsufficientData
			result.MissingReason = firstMissingReason(points)
		}
		result.AggregationMethod = AggregationMethodVolumeWeightedTwoStage
		return result
	}

	// stage 2：跨市場，以市場總成交量為權重
	marketIDs := make([]string, 0, len(perMarket))
	for marketID := range perMarket {
		marketIDs = append(marketIDs, marketID)
	}
	sort.Strings(marketIDs) // 固定順序：浮點加總對順序敏感，map 迭代順序隨機會讓結果不可重現
	values10 := make([]float64, 0, len(marketIDs))
	values50 := make([]float64, 0, len(marketIDs))
	values90 := make([]float64, 0, len(marketIDs))
	weights := make([]float64, 0, len(marketIDs))
	for _, marketID := range marketIDs {
		agg := perMarket[marketID]
		values10 = append(values10, agg.p10)
		values50 = append(values50, agg.p50)
		values90 = append(values90, agg.p90)
		weights = append(weights, agg.weight)
	}
	p10, fb10 := WeightedAverage(values10, weights)
	p50, fb50 := WeightedAverage(values50, weights)
	p90, fb90 := WeightedAverage(values90, weights)
	fellBack = fellBack || fb10 || fb50 || fb90

	result.P10, result.P50, result.P90 = &p10, &p50, &p90
	result.AggregationMethod = AggregationMethodVolumeWeightedTwoStage
	if fellBack {
		result.AggregationMethod = AggregationMethodEqualWeightFallback
	}
	if result.EffectiveVariantCount < totalVariantCount {
		result.Status = PredictionStatusPartial
	} else {
		result.Status = PredictionStatusReady
	}
	return result
}

func collect(points []VariantPoint, pick func(VariantPoint) float64) []float64 {
	out := make([]float64, len(points))
	for i, p := range points {
		out[i] = pick(p)
	}
	return out
}

// WeightedAverage 回傳加權平均與「是否退成等權」。
//
// exported 的理由同 PredictionStatusSeverity：issue #2248 的 spatial-surface
// 也要把「同一個市場、大類底下多個子品項」折成一個價格，而那必須跟本檔案
// （GroupForecast / GroupObservedSeries）用同一條加權規則——服務不能有兩份
// 定義不同的「這個大類在市場 X 的價格」。
//
// 鏡射上游 _weighted_average（api/catalog.py:102-108）的數學（負權重當 0、
// 正權重總和為 0 時退成等權算術平均），但把退化這件事回報出來——上游是靜默的，
// 而 §8 line 392 要求標成 equal_weight_fallback。
func WeightedAverage(values, weights []float64) (float64, bool) {
	if len(values) == 0 {
		return 0, false
	}
	positiveWeight := 0.0
	for _, w := range weights {
		if w > 0 {
			positiveWeight += w
		}
	}
	if positiveWeight > 0 {
		sum := 0.0
		for i, v := range values {
			if weights[i] > 0 {
				sum += v * weights[i]
			}
		}
		return sum / positiveWeight, false
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	// 單一參與者時，weighted 與 equal-weight 平均數學上等價（sum/1 == v），不是
	// 真的退化成等權重——回報 fallback=false，避免單一市場缺權重就誤標整個
	// horizon 的 AggregationMethod（GroupPoint 只有一個 horizon-wide 欄位，見
	// aggregateOneHorizon 的 fellBack OR 聚合）。
	return sum / float64(len(values)), len(values) > 1
}

func firstMissingReason(points []VariantPoint) MissingReason {
	for _, p := range points {
		if p.MissingReason != "" {
			return p.MissingReason
		}
	}
	return MissingReasonInsufficientRecentData
}
