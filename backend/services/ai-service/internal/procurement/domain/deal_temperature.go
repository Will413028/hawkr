package domain

import "math"

// 成交溫度計 v1 的值域。三者皆已在 CodeMeta 登記（業務類型碼.md），這裡不重新
// 定義字面值以外的東西；綁到 codemeta 生成常數的工作在 Task 3 完成後由 usecase
// 層做，domain 層只負責計算，刻意不 import codemeta 以維持純函數性質。
const (
	DealTemperatureStatusAvailable = "AVAILABLE"
	DealTemperatureStatusUnknown   = "UNKNOWN"

	DealTemperatureReasonRangeOutsideReference = "RANGE_OUTSIDE_REFERENCE"
	DealTemperatureReasonQuantilesNotMonotonic = "QUANTILES_NOT_MONOTONIC"
	DealTemperatureReasonBuyerRangeNotSet      = "BUYER_RANGE_NOT_SET"

	DealTemperatureBandCold = "COLD"
	DealTemperatureBandWarm = "WARM"
	DealTemperatureBandHot  = "HOT"
)

// DealTemperatureCoverage 是成交溫度計 v1 的兩個機率與色帶。
//
// 兩個數字都是關於「市場價格」的機率，不是關於賣家會不會回應的機率——後者需要
// 歷史 outcome 才估得出來，屬 outcomeDefinitionVersion 2（見設計書 §1.1）。
type DealTemperatureCoverage struct {
	// WithinRangeBps：市場價落在買家希望區間內的機率。
	//
	// ⚠️ 買家畫面**不顯示**這個數字，色帶也不由它決定（BANDS_V2 起）。它對買家
	// 是**反指標**：出價 70–90、行情 52–74 時它只有 12%，但那是買家最有利的
	// 情況——行情整段低於他的上限。留在 wire 上是因為它會成為
	// outcomeDefinitionVersion = 2 的 feature，不作廢。
	WithinRangeBps int64
	// AtOrBelowUpperBps：市場價不超過買家希望上限的機率。等價於「買家上限落在
	// 行情分布的第幾百分位」，是買家畫面上唯一顯示的數字。
	AtOrBelowUpperBps int64
	// Band 由 AtOrBelowUpperBps 決定。買家要的是「行情會不會超過我願意付的
	// 上限」，而那個量對上限是單調的（上限拉高 → 數字上升）；WithinRangeBps
	// 不是（區間放寬會讓它上升，但出價遠高於行情反而讓它下降）。
	Band string
}

// referenceCDF 回傳分段線性 CDF 在 x 的值。三個分位點分別對應 0.10／0.50／0.90。
//
// [refLower, refUpper] 之外的分布形狀未知，一律截斷而不外插（設計書 §2.3）：
// 外插要先發明尾部形狀，而發明出來的數字沒有任何東西支撐。截斷在兩端都使結果
// 低估，方向保守。
func referenceCDF(refLower, refMid, refUpper, x float64) float64 {
	switch {
	case x <= refLower:
		return 0.10
	case x < refMid:
		return 0.10 + (x-refLower)/(refMid-refLower)*0.40
	case x < refUpper:
		return 0.50 + (x-refMid)/(refUpper-refMid)*0.40
	default:
		return 0.90
	}
}

// toBps 把機率取整到整數百分點再轉 bps。三個分位點撐不起小數點後的精度。
func toBps(p float64) int64 {
	return int64(math.Round(p*100)) * 100
}

// bandFor 由寬鬆界（＝買家上限所在的百分位）決定色帶。
//
// 門檻是百分位的自然四分位切法，不是 BANDS_V1 的 20/50。沿用舊門檻會讓色帶
// 失去鑑別力：寬鬆界的值域是 [0.10, 0.90]，而且只要買家上限 >= 市場中位就
// 必然 >= 0.50——那會讓「熱」涵蓋一半以上的情形。
//
// 三個門檻與 BANDS_V1 一樣**無實證基礎**，是第一版判斷；`bandVersion` 的存在
// 就是為了讓它之後能改而不動計算層。
func bandFor(atOrBelowBps int64) string {
	switch {
	case atOrBelowBps < 2500:
		return DealTemperatureBandCold
	case atOrBelowBps < 7500:
		return DealTemperatureBandWarm
	default:
		return DealTemperatureBandHot
	}
}

// ComputeDealTemperatureCoverage 算出成交溫度計 v1 的兩個機率。
//
// refLower／refMid／refUpper 必須是**編譯後**（domain.CompileReferencePrice）的
// 金額，與 buyerLower／buyerUpper 同一個 canonical basis。傳 raw 的 per-KG 分位數
// 會在 GRAM 品項上差 1000 倍。
//
// 呼叫端不保證 buyerLower <= buyerUpper；若區間顛倒（buyerLower > buyerUpper）
// 會回傳 DealTemperatureReasonBuyerRangeNotSet。
//
// 第二個回傳值是 UNKNOWN 的 reason code，空字串代表 AVAILABLE。
func ComputeDealTemperatureCoverage(
	refLower, refMid, refUpper float64,
	buyerLower, buyerUpper int64,
) (DealTemperatureCoverage, string) {
	if !(refLower < refMid && refMid < refUpper) {
		return DealTemperatureCoverage{}, DealTemperatureReasonQuantilesNotMonotonic
	}
	lo, hi := float64(buyerLower), float64(buyerUpper)
	// 守衛：呼叫端不保證 lower<=upper，PRICEBOOK_GUIDED 路徑不走 validateBuyerRequestedRange
	if lo > hi {
		return DealTemperatureCoverage{}, DealTemperatureReasonBuyerRangeNotSet
	}
	// 嚴格不等式：單點相接時涵蓋率為 0，視為完全在外（設計書 §2.3）。
	if !(hi > refLower && lo < refUpper) {
		return DealTemperatureCoverage{}, DealTemperatureReasonRangeOutsideReference
	}
	atOrBelow := referenceCDF(refLower, refMid, refUpper, hi)
	within := atOrBelow - referenceCDF(refLower, refMid, refUpper, lo)
	atOrBelowBps := toBps(atOrBelow)
	return DealTemperatureCoverage{
		WithinRangeBps:    toBps(within),
		AtOrBelowUpperBps: atOrBelowBps,
		Band:              bandFor(atOrBelowBps),
	}, ""
}

func ComputeDealTemperatureFromPoint(
	refMid float64,
	buyerLower, buyerUpper int64,
) (DealTemperatureCoverage, string) {
	lo, hi := float64(buyerLower), float64(buyerUpper)
	if lo > hi {
		return DealTemperatureCoverage{}, DealTemperatureReasonBuyerRangeNotSet
	}
	var within, atOrBelow int64
	if refMid <= hi {
		atOrBelow = 10000
	}
	if refMid >= lo && refMid <= hi {
		within = 10000
	}
	return DealTemperatureCoverage{
		WithinRangeBps:    within,
		AtOrBelowUpperBps: atOrBelow,
		Band:              bandFor(atOrBelow),
	}, ""
}
