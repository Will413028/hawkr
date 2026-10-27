package testkit

import "github.com/Will413028/hawkr/ai-service/internal/procurement/domain"

// 以下四個 helper 是品項解析結果 fixture，對應舊形狀 repository.Request 上的
// 欄位組合；解析結果現在住 domain.Slots.ItemResolution 裡
// （ADR 2026-09-04-crop-resolution-into-slots）。

// GroupResolution ＝ 舊的 repository.Request{ResolvedGroupCode: ...}。
func GroupResolution(code string) *domain.ItemResolution {
	return &domain.ItemResolution{
		Kind: domain.ItemResolvedGroup, Code: &code,
		Provenance: domain.ItemResolutionBySystem,
	}
}

// CropResolutionWithFallback ＝ 舊的 {ResolvedCropCode, ResolvedFallbackGroupCode}。
// 刻意不同時帶 group code：那兩個欄位在舊形狀互斥，同時有值是系統不會產生的狀態
// （新形狀連表達它的機會都沒有——Kind 只能是其中一個）。
func CropResolutionWithFallback(cropCode, fallbackGroupCode string) *domain.ItemResolution {
	return &domain.ItemResolution{
		Kind: domain.ItemResolvedCrop, Code: &cropCode,
		FallbackGroupCode: &fallbackGroupCode, Provenance: domain.ItemResolutionBySystem,
	}
}

// SelfPricedResolution ＝ 舊的 {CropResolutionSelfPriced: true, ...PricePerKg}。
// price 直接給指標，讓防禦性邊界測試建構得出 nil／負數／超界這些
// domain.validateItemResolution 本來會擋掉、但要證明下游也有獨立防線的狀態。
func SelfPricedResolution(price *int) *domain.ItemResolution {
	return &domain.ItemResolution{
		Kind: domain.ItemResolvedSelfPriced, PricePerKg: price,
		Provenance: domain.ItemResolutionByBuyerChoice,
	}
}

// CommodityResolution 是「品項已解析為量產商品（毛豬等）」的 fixture，供
// PRICEBOOK_COMMODITY 相關測試共用。
func CommodityResolution(code string) *domain.ItemResolution {
	return &domain.ItemResolution{
		Kind: domain.ItemResolvedCommodity, Code: &code, Label: "毛豬（規格豬）",
		Provenance: domain.ItemResolutionByBuyerChoice,
	}
}
