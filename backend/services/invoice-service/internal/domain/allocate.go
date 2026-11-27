package domain

import (
	"errors"
	"sort"
)

// MaxAmount 是業務上限，與結帳側 OrderAmountLimit.ORDER_MAX_AMOUNT（order-service）對齊。
// 由 A20 檢查，不是 AllocateLargestRemainder 的護欄。
const MaxAmount int64 = 199_999

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
		if sum >= maxSafeOperand {
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
		copy(out, weights)
		return out, nil
	}

	rem := make([]int64, len(weights))
	var allocated int64
	for i, w := range weights {
		num := total * w
		out[i] = num / sum
		rem[i] = num - out[i]*sum
		allocated += out[i]
	}

	remaining := int(total - allocated)
	if remaining > 0 {
		order := make([]int, len(weights))
		for i := range order {
			order[i] = i
		}
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
