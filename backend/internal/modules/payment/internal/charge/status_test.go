package charge

import (
	"errors"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/model"
)

// 狀態只由快照推導，除了入帳（終局）之外直接跟著快照走；入帳一律勝出，之後的快照不倒退它。
// mutation：Next 讓 SUCCEEDED 之後的快照也能改狀態 → 下面入帳不被覆蓋的列失敗。
func TestNextFollowsTheDecisionTable(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		cur  model.PaymentStatus
		in   State
		want model.PaymentStatus
		keep bool
	}{
		{"creating to awaiting", model.PaymentCreating, StateAwaitingPayment, model.PaymentAwaitingPayment, false},
		{"awaiting to processing", model.PaymentAwaitingPayment, StateProcessing, model.PaymentProcessing, false},
		{"processing back to awaiting after a failed attempt", model.PaymentProcessing, StateAwaitingPayment, model.PaymentAwaitingPayment, false},
		{"awaiting to succeeded", model.PaymentAwaitingPayment, StateSucceeded, model.PaymentSucceeded, false},
		{"awaiting to canceled", model.PaymentAwaitingPayment, StateCanceled, model.PaymentCanceled, false},
		{"canceled then paid: the provider's success wins", model.PaymentCanceled, StateSucceeded, model.PaymentSucceeded, false},
		{"succeeded stays on a duplicate", model.PaymentSucceeded, StateSucceeded, model.PaymentSucceeded, false},
		{"succeeded is not overwritten by awaiting", model.PaymentSucceeded, StateAwaitingPayment, model.PaymentSucceeded, true},
		{"succeeded is not overwritten by canceled", model.PaymentSucceeded, StateCanceled, model.PaymentSucceeded, true},
		{"succeeded is not overwritten by processing", model.PaymentSucceeded, StateProcessing, model.PaymentSucceeded, true},
		{"the creation terminal state keeps", model.PaymentCreateFailed, StateAwaitingPayment, model.PaymentCreateFailed, true},
		{"an anomaly keeps until a person resolves it", model.PaymentAnomaly, StateSucceeded, model.PaymentAnomaly, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, keep, err := Next(c.cur, Snapshot{State: c.in})
			if err != nil || got != c.want || keep != c.keep {
				t.Fatalf("Next(%s, %s) = %s keep=%v err=%v，預期 %s keep=%v", c.cur, c.in, got, keep, err, c.want, c.keep)
			}
		})
	}
}

// 不認得的狀態與空值不是有效值：不推導出下一個狀態，回錯誤（呼叫方不寫入）。
func TestNextRejectsAStateThatIsNotAValue(t *testing.T) {
	t.Parallel()
	for _, s := range []State{StateUnknown, "", "requires_capture"} {
		for _, cur := range model.PaymentStatusValues() {
			got, keep, err := Next(cur, Snapshot{State: s})
			if err == nil || got != cur || keep {
				t.Fatalf("Next(%s, %q) = %s keep=%v err=%v，預期錯誤且狀態不變", cur, s, got, keep, err)
			}
		}
	}
	if State("").Valid() || StateUnknown.Valid() || RefundUnknown.Valid() || RefundState("").Valid() {
		t.Fatal("未知與空值被當成有效值")
	}
}

func TestAnAnomalyIsPermanent(t *testing.T) {
	t.Parallel()
	if !errors.Is(ErrAnomaly, ErrPermanent) {
		t.Fatal("快照與列不一致必須是永久失敗，同步 job 才會取消而不是重試")
	}
}
