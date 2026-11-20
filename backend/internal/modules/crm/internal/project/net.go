package project

import "github.com/Will413028/hawkr/backend/internal/modules/crm/internal/model"

// state 是一張訂單在投影裡的狀態（order_nets 的列）：是否收過 order.completed、是否收過全額退款，與淨額（可暫時為負：退款先到）。
type state struct {
	Completed    bool
	FullRefunded bool
	Net          int64
}

// contribution 是這張單計入 customers.total_spent 的金額：只算 completed 的單，下限 0。
// total_spent 結構上 >= 0（每單貢獻 >= 0），也吸收「部分退款排在全額退款之後再扣一次」的重試順序（全額後淨額 0，再扣變負，貢獻仍 0）。
func contribution(s state) int64 {
	if !s.Completed {
		return 0
	}
	return max(s.Net, 0)
}

// step 是一個事件對一張單的影響。
type step struct {
	Next state
	// Delta 是這張單貢獻的變化量，加到客戶的 total_spent。
	Delta int64
	// Changed 為 false 表示事件沒有改變任何東西（例如同單號的第二個 order.completed）。
	Changed bool
}

func stepTo(old, next state) step {
	return step{Next: next, Delta: contribution(next) - contribution(old), Changed: next != old}
}

// onOrderCompleted：單上已有全額退款時，訂單金額記 0（全額退款已把淨額歸零）；否則淨額加上訂單金額。
// 單上已 completed 時不再計入：同一單號的第二個 order.completed（換了 event_id）不重複累加。
func onOrderCompleted(old state, amount int64) step {
	if old.Completed {
		return step{Next: old}
	}
	next := state{Completed: true, FullRefunded: old.FullRefunded, Net: old.Net + amount}
	if old.FullRefunded {
		next.Net = 0
	}
	return stepTo(old, next)
}

// onRefund：部分退款扣掉退款金額；全額退款不信上游的金額，淨額直接歸零（等同扣掉自己記錄的全部淨額）。
func onRefund(old state, outcome model.RefundOutcome, amount int64) step {
	next := old
	if outcome == model.RefundFull {
		next.FullRefunded = true
		next.Net = 0
	} else {
		next.Net -= amount
	}
	return stepTo(old, next)
}
