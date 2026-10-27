package domain

import (
	"strings"

	"github.com/Will413028/hawkr/ai-service/internal/usecase"
)

// PriceAvailability 描述這批候選能不能拿到可信的市場參考價，取代原本
// 「查無資料就回 error 整筆擋掉」的設計（issue 見設計書背景段）。
type PriceAvailability string

const (
	// PriceAvailabilityMarket：ready 且新鮮的市場觀測資料。
	PriceAvailabilityMarket PriceAvailability = "MARKET"
	// PriceAvailabilityMarketStale：ready 但 FreshnessStatus 不是 fresh
	// （pricebook-service 認為資料偏舊），或本次改動加入的 fallback（crop
	// 查無資料、改用 group 命中）——仍然是可用的數字，只是精度打了折扣，
	// 前端顯示「行情資料較舊，僅供參考」。
	PriceAvailabilityMarketStale PriceAvailability = "MARKET_STALE"
	// PriceAvailabilityUnavailable：完全查無資料，或資料本身不可用（見
	// isUsablePricePerKg）。金額欄位全部為 nil，報價單仍然照常建立。
	PriceAvailabilityUnavailable PriceAvailability = "UNAVAILABLE"
	// PriceAvailabilityBuyerEntered：這批的 base_price_per_kg 來自買家在「D 其他」
	// 逃生艙自填的單價（repository.Request.CropResolutionSelfPricedPricePerKg），
	// 不是 Pricebook 市場行情查詢——這類 request 的 ResolvedCropCode／
	// ResolvedGroupCode 恰好都是 nil（品項本來就不支援解析，設計書 §6），完全不會
	// 呼叫 matchRegionPriceWithFallback。
	PriceAvailabilityBuyerEntered PriceAvailability = "BUYER_ENTERED"
)

// RegionPriceResult 是 MatchRegionPrice 的完整輸出：PricePerKg 只在
// Availability 是 MARKET 或 MARKET_STALE 時非 nil。
type RegionPriceResult struct {
	PricePerKg   *float64
	Availability PriceAvailability
}

// MatchRegionPrice 從 GetRegionalComparison 的縣市清單找出對應買家
// delivery_city 的 p50，作為 base_price_per_kg 的來源。用 NormalizeCityName
// 消除「臺/台」雙寫差異後比對。查無資料或資料狀態不是 ready 時回
// PriceAvailabilityUnavailable，不再是 error——呼叫端（priceQuote）決定
// 要不要視為可以繼續建立報價單（V1：永遠繼續，見設計書）。
func MatchRegionPrice(regions []usecase.RegionalComparisonRegion, deliveryCity string) RegionPriceResult {
	target := NormalizeCityName(deliveryCity)
	for _, r := range regions {
		if NormalizeCityName(r.RegionName) != target {
			continue
		}
		if r.Status != "ready" || r.P50 == nil {
			return RegionPriceResult{Availability: PriceAvailabilityUnavailable}
		}
		avail := PriceAvailabilityMarket
		if r.FreshnessStatus != "fresh" {
			avail = PriceAvailabilityMarketStale
		}
		return RegionPriceResult{PricePerKg: r.P50, Availability: avail}
	}
	return RegionPriceResult{Availability: PriceAvailabilityUnavailable}
}

// NormalizeCityName 消除縣市名的「臺/台」雙寫差異，供任何要比對 Pricebook
// 區域名與買家 delivery_city 的呼叫端共用（Task 18／Fix 1：
// pricing/price_preview.go 的區域比對迴圈曾經各自用裸 == 比對，對台北/台中/
// 台南/台東四市（Pricebook 回「台」開頭、買家 canonicalized 值可能是「臺」
// 開頭）永遠比不中——這裡是唯一的正規化實作，不為單一呼叫端另開一份平行邏輯，
// 見 CLAUDE.md §15）。從套件私有的 normalizeCityName 改為匯出，行為完全不變。
func NormalizeCityName(s string) string {
	return strings.ReplaceAll(s, "臺", "台")
}

// ConvertToUnitPrice 回傳市場公斤價的整數元副本，供 CalculateInvitePrices 當
// unitPrice 使用。AllowedUnits 只剩重量單位後，買家的需求單位本身就是公斤／台斤／
// 公克，市場公斤價可以直接當單價用，不再需要依 lane 或候選商品的 SKU 淨重換算成
// 單件價（舊版 BOX/CASE/PIECE/SET 計數單位路徑已隨 AllowedUnits 一起移除）。
//
// basePricePerKg 是 nil 時（查無市場行情，PriceAvailability==UNAVAILABLE）直接回
// nil——沒有行情就沒有任何單價可以算，nil 一路傳遞到 CalculateInvitePrices，不得
// 中途冒出一個看起來正常的數字。回傳值特意用新指標而非原樣傳回 basePricePerKg：
// 呼叫端在候選迴圈裡每個候選都會拿到同一顆 unitPrice，若共用同一個指標，日後任何
// 一段程式碼經由某個候選的 unitPrice 原地改值，會悄悄污染同批其他候選的價格。
func ConvertToUnitPrice(basePricePerKg *int64) *int64 {
	if basePricePerKg == nil {
		return nil
	}
	v := *basePricePerKg
	return &v
}
