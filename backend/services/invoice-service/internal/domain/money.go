package domain

// Untax 把含稅金額換成未稅金額，等價於官方算式的 Round(gross / 1.05)。
//
// 推導：gross / 1.05 == gross * 20 / 21。整數四捨五入 ⇒ (gross*20 + 21/2) / 21，
// 取 21/2 的上整數 10（21 為奇數，+10 與 +10.5 在整數除法下等價）⇒ (gross*20 + 10) / 21。
//
// 捨入模式（HALF_UP vs HALF_EVEN）在這條式子上沒有鑑別力：20·G/21 的小數部分恆不為 0.5
// （需 42 | (40G+21)，而 40G 必為偶數、40G+21 必為奇數 ⇒ 永不被偶數 42 整除）。
// ⇒ 不要「找 .5 邊界值去測」——那個樣本不存在，測出來的「兩種模式一致」是假訊號。
//
// gross 必須 >= 0。負數（DISCOUNT 行）不會單獨呼叫本函式：折扣已在稅別桶內被吸收，
// 進到這裡的一律是桶層加總後的非負值。
func Untax(gross int64) int64 {
	if gross <= 0 {
		return 0
	}
	return (gross*20 + 10) / 21
}

// TaxOf 回傳含稅金額中的營業稅額，等價於 gross - Round(gross/1.05)。
func TaxOf(gross int64) int64 { return gross - Untax(gross) }
