package domain

import (
	"fmt"
	"math"
)

const PriceComponentGoodsOnly = "GOODS_ONLY"

// weightConversionFactor 回傳重量單位（KG/CATTY/GRAM）的 kg 換算係數。
// 呼叫者應確保 unit 是 domain.AllowedUnits 允許的值；如果 unit 不是已知的
// weight-based unit，會回傳 error，而非靜默回傳預設值。這確保了如果
// AllowedUnits 未來又擴張卻忘記新增對應係數，程式會立即失敗並要求補上，
// 而非靜默做錯的計算。
func weightConversionFactor(unit string) (float64, error) {
	switch unit {
	case "KG":
		return 1.0, nil
	case "CATTY":
		return 0.6, nil
	case "GRAM":
		// 1000 GRAM 為基準，等同 1 KG，避免每公克價格取整為 0
		return 1.0, nil
	}
	return 0, fmt.Errorf("unit %q has no configured weight conversion factor", unit)
}

// CompileReferencePrice 把 Pricebook 的 TWD/kg 市場參考價依 canonical unit
// basis 換算成對應金額，不含任何 margin/markup（V.7.1：referencePriceRange
// 只是行情參考，不是成交價或賣家報價；MarkupRateByLane 與本函式無關）。
//
// domain.AllowedUnits 只允許重量單位（KG/CATTY/GRAM，見 slots.go）：BOX/
// CASE/PIECE/SET 等計數單位沒有天然公斤換算，Pricebook 對它們永遠算不出
// 參考價，因此不屬於本函式要處理的範圍。unit 不是已知的重量單位時明確
// 報錯（weightConversionFactor 的守門），而非靜默漂移。
//
// 回傳值在這裡就 math.Round 成整數元（Task 18／Fix 2：CLAUDE.md §4「金額以
// 新台幣「元」的整數表示，禁止浮點運算」。既有規則是「唯一的浮點入口是
// pricebook 的 p50 市場行情，取得後就四捨五入成整數元一次，之後不再有小數
// 重新進入這條路徑」——本函式就是 price-preview 這條路徑對應的
// 「唯一浮點入口」：sourcePriceTwdPerKg 是 Pricebook 的原始浮點市場價，
// factor 對 CATTY 是 0.6，兩者相乘會產生 IEEE754 無法精確表示的殘留值
// （例如 100.1*0.6 = 60.059999999999995），修前這個殘留值會一路傳到
// usecase.formatAmount／gRPC handler 的 formatPriceAmount，用
// strconv.FormatFloat(f, 'f', -1, 64) 原樣印成字串——math.Round 在這裡（回傳
// 前、唯一的浮點入口）就地取整一次，之後呼叫端只做字串化，不再有第二次
// 四捨五入、也不再有小數重新進入這條路徑。回傳型別維持 float64（不是
// int64）：取整後的值仍是「整數值的 float64」，經
// strconv.FormatFloat(f, 'f', -1, 64) 一樣印成乾淨的整數字串（例如
// 36 而非 36.0），改回 int64 會牽動 ReferencePriceRange 既有欄位型別與呼叫端，
// 這裡選最小、風險最低的做法。
func CompileReferencePrice(sourcePriceTwdPerKg float64, unit string, priceComponent string) (float64, error) {
	if priceComponent != PriceComponentGoodsOnly {
		return 0, fmt.Errorf("unsupported priceComponent %q", priceComponent)
	}
	factor, err := weightConversionFactor(unit)
	if err != nil {
		return 0, err
	}
	return math.Round(sourcePriceTwdPerKg * factor), nil
}
