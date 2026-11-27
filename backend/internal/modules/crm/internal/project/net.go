package project

import "github.com/Will413028/hawkr/backend/internal/modules/crm/internal/model"

type state struct {
	Completed    bool
	FullRefunded bool
	Net          int64
}

func contribution(s state) int64 {
	if !s.Completed {
		return 0
	}
	return max(s.Net, 0)
}

type step struct {
	Next state
	Delta int64
	Changed bool
}

func stepTo(old, next state) step {
	return step{Next: next, Delta: contribution(next) - contribution(old), Changed: next != old}
}

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
