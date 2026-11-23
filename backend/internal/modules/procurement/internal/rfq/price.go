package rfq

import "math"

// Basis 回傳單價的計價基準：GRAM 以每 1000 個計價，其餘每 1 個。
func Basis(unit string) (quantity int32, basisUnit string) {
	if unit == "GRAM" {
		return 1000, unit
	}
	return 1, unit
}

// ValidRange 回傳單價區間是否合法：下界與上界是正整數、下界不大於上界。
func ValidRange(lower, upper int64) bool { return lower > 0 && upper > 0 && lower <= upper }

// Total 回傳單價區間乘以數量、除以計價基準的總額區間（四捨五入到元）；溢位時 ok 為 false。
func Total(lower, upper int64, quantity, basis int32) (totalLower, totalUpper int64, ok bool) {
	totalLower, ok = scale(lower, int64(quantity), int64(basis))
	if !ok {
		return 0, 0, false
	}
	totalUpper, ok = scale(upper, int64(quantity), int64(basis))
	return totalLower, totalUpper, ok
}

func scale(unitAmount, quantity, basis int64) (int64, bool) {
	if unitAmount > math.MaxInt64/quantity {
		return 0, false
	}
	product := unitAmount * quantity
	if product > math.MaxInt64-basis/2 {
		return 0, false
	}
	return (product + basis/2) / basis, true
}
