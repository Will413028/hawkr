package domain

import (
	"errors"
	"sort"
)

// MaxAmount 是業務上限，與結帳側 PaymentMethodLimitRules.ORDER_MAX_AMOUNT 對齊。
// 由 A20 檢查，不是 AllocateLargestRemainder 的護欄。
const MaxAmount int64 = 199_999

// maxSafeOperand 是溢位護欄，與業務上限無關。
//
// total < 2^31 且 Σweights < 2^31 ⇒ total × weights[i] < 2^62 < 2^63−1 ⇒ 永不溢位。
//
// 刻意不用 MaxAmount 當護欄：Σweights 是商品小計（orders.total_original_amount），而結帳只對
// total_actual_amount 設上限、沒有對小計設上限 ⇒「小計 300,000 − 折扣 150,000 ＝ 實付 150,000」
// 是完全合法的訂單，但它的 Σweights 超過 MaxAmount。拿業務常數當護欄會把這種訂單誤擋，
// 錯誤訊息還指向「金額超過上限」這個假原因。
const maxSafeOperand int64 = 1 << 31

var (
	// ErrAllocateNegative 表示權重或總額為負。
	ErrAllocateNegative = errors.New("invoice: allocate: negative input")
	// ErrAllocateOperandTooLarge 表示總額或權重合計 >= 2^31。
	ErrAllocateOperandTooLarge = errors.New("invoice: allocate: operand exceeds overflow guard")
	// ErrAllocateTooLarge 表示總額大於權重合計；呼叫端必須先夾住。
	ErrAllocateTooLarge = errors.New("invoice: allocate: total exceeds sum of weights")
)

// AllocateLargestRemainder 用最大餘數法把 total 依 weights 的比例分成整數份。
//
// 前置條件：total >= 0；∀i weights[i] >= 0；total <= Σweights。
// 後置條件：Σ out == total；∀i 0 <= out[i] <= weights[i]；同輸入必得同輸出（決定性）。
//
// 全程 int64，禁止改用 float64：既有的 CheckoutService.allocateShareDiscount 就是因為
// Math.round((double)...) ＋ Math.max(0, ...) 而在特定金額組合下讓加總不等於折扣總額
// （實測反例：小計 [750,156,198,10]、discount 26 ⇒ Σshare = 27 != 26）。
func AllocateLargestRemainder(weights []int64, total int64) ([]int64, error) {
	// 護欄三個檢查，順序即實作順序。
	if total < 0 {
		return nil, ErrAllocateNegative
	}
	for _, w := range weights {
		if w < 0 {
			return nil, ErrAllocateNegative
		}
	}
	if total >= maxSafeOperand {
		return nil, ErrAllocateOperandTooLarge
	}
	var sum int64
	for _, w := range weights {
		sum += w
		if sum >= maxSafeOperand { // 累加時逐步檢查，不等全部加完
			return nil, ErrAllocateOperandTooLarge
		}
	}
	if total > sum {
		return nil, ErrAllocateTooLarge
	}

	out := make([]int64, len(weights))
	if total == 0 || sum == 0 {
		return out, nil
	}
	if total == sum {
		copy(out, weights) // 全額吃滿，跳過除法
		return out, nil
	}

	rem := make([]int64, len(weights))
	var allocated int64
	for i, w := range weights {
		num := total * w // 上界由 maxSafeOperand 證明
		out[i] = num / sum
		rem[i] = num - out[i]*sum
		allocated += out[i]
	}

	// R = 待分配的剩餘單位，0 <= R < n。
	remaining := int(total - allocated)
	if remaining > 0 {
		order := make([]int, len(weights))
		for i := range order {
			order[i] = i
		}
		// 排序鍵三段皆為必要：
		//   rem DESC     —— 最大餘數法本體
		//   weights DESC —— 餘數相同時讓金額大的先拿（否則結果依賴 slice/map 迭代順序，
		//                   同一筆訂單重跑兩次會得到不同的「申報」金額）
		//   index ASC    —— 前兩者都相同時的最終決勝，保證同輸入必得同輸出
		sort.SliceStable(order, func(a, b int) bool {
			ia, ib := order[a], order[b]
			if rem[ia] != rem[ib] {
				return rem[ia] > rem[ib]
			}
			if weights[ia] != weights[ib] {
				return weights[ia] > weights[ib]
			}
			return ia < ib
		})
		for k := 0; k < remaining; k++ {
			out[order[k]]++
		}
	}
	return out, nil
}
