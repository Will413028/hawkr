package project

import (
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/crm/internal/model"
)

type ev struct {
	kind   string
	amount int64
}

func replay(events []ev) (state, int64, int64) {
	var s state
	var total int64
	var minTotal int64
	for _, e := range events {
		var st step
		switch e.kind {
		case "order":
			st = onOrderCompleted(s, e.amount)
		case "partial":
			st = onRefund(s, model.RefundPartial, e.amount)
		case "full":
			st = onRefund(s, model.RefundFull, 0)
		}
		s = st.Next
		total += st.Delta
		minTotal = min(minTotal, total)
	}
	return s, total, minTotal
}

// 事件順序表格（design §11.1）：total_spent 等於這單貢獻 max(net,0)，每一步都 >= 0。
func TestOrderEventSequences(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		events    []ev
		wantTotal int64
		wantNet   int64
	}{
		{"訂單→部分→全額", []ev{{"order", 100}, {"partial", 30}, {"full", 0}}, 0, 0},
		{"全額→訂單", []ev{{"full", 0}, {"order", 100}}, 0, 0},
		{"部分→訂單", []ev{{"partial", 30}, {"order", 100}}, 70, 70},
		{"訂單→全額→部分（重試排後）", []ev{{"order", 100}, {"full", 0}, {"partial", 30}}, 0, -30},
		{"訂單→部分", []ev{{"order", 100}, {"partial", 30}}, 70, 70},
		{"部分→全額→訂單", []ev{{"partial", 30}, {"full", 0}, {"order", 100}}, 0, 0},
		{"部分退款超過訂單金額", []ev{{"order", 100}, {"partial", 150}}, 0, -50},
		{"只有退款沒有訂單", []ev{{"partial", 30}, {"full", 0}}, 0, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s, total, minTotal := replay(tc.events)
			if total != tc.wantTotal || s.Net != tc.wantNet {
				t.Fatalf("total_spent=%d net=%d，預期 %d、%d", total, s.Net, tc.wantTotal, tc.wantNet)
			}
			if minTotal < 0 {
				t.Fatalf("total_spent 中途跌到 %d（CHECK >= 0 會讓這個 transaction 失敗）", minTotal)
			}
			if total != contribution(s) {
				t.Fatalf("累計 %d 不等於這單的貢獻 %d（不變量：total_spent 恆等於各單 max(net,0) 之和）", total, contribution(s))
			}
		})
	}
}

// 同一單號的第二個 order.completed（換了 event_id）不重複累加。
func TestSecondOrderCompletedForSameOrderIsIgnored(t *testing.T) {
	t.Parallel()
	first := onOrderCompleted(state{}, 100)
	second := onOrderCompleted(first.Next, 100)
	if second.Changed || second.Delta != 0 || second.Next != first.Next {
		t.Fatalf("第二個 order.completed 改變了狀態：%+v", second)
	}
}

// 未完成的訂單不計入：只有退款先到時 contribution 是 0，客戶的 total_spent 不動。
func TestIncompleteOrderContributesNothing(t *testing.T) {
	t.Parallel()
	if got := contribution(state{Completed: false, Net: 500}); got != 0 {
		t.Fatalf("contribution = %d", got)
	}
	if st := onRefund(state{}, model.RefundPartial, 30); st.Delta != 0 {
		t.Fatalf("退款先到的 delta = %d", st.Delta)
	}
}
