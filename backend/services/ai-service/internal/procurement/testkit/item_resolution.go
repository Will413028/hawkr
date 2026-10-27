package testkit

import "github.com/Will413028/hawkr/ai-service/internal/procurement/domain"

// SelfPricedResolution 是買家自填單價的品項解析 fixture。price 直接給指標，讓防禦性邊界
// 測試建構得出 nil／負數／超界這些 domain.validateItemResolution 本來會擋掉、但要證明下游
// 也有獨立防線的狀態。
func SelfPricedResolution(price *int) *domain.ItemResolution {
	return &domain.ItemResolution{
		Kind: domain.ItemResolvedSelfPriced, PricePerKg: price,
		Provenance: domain.ItemResolutionByBuyerChoice,
	}
}
