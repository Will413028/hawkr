package charge

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/store"
)

// 撤回額度：退款額與轉帳剩餘可撤額度（已扣預留）取小，沒有剩餘就是 0（款項在平台手上，直接退款）。
// mutation：reversalFor 不取小、永遠回 amount → 剩餘額度小於退款額的列失敗。
func TestReversalForTakesTheSmallerOfTheRefundAndWhatIsLeftToReverse(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name              string
		amount, available int64
		want              int64
	}{
		{"all of it is still on the transfer", 30, 100, 30},
		{"only part is left", 30, 20, 20},
		{"exactly what is left", 30, 30, 30},
		{"nothing left", 30, 0, 0},
		{"over-reserved is not negative", 30, -5, 0},
	} {
		if got := reversalFor(c.amount, c.available); got != c.want {
			t.Errorf("%s：%d，預期 %d", c.name, got, c.want)
		}
	}
}

// provider 的退款狀態對應成本地狀態：處理中是 REFUNDING，成功與失敗是終局，不認得的沒有對應。
func TestRefundTargetMapsEveryKnownProviderStateAndRejectsTheRest(t *testing.T) {
	t.Parallel()
	for state, want := range map[RefundState]model.RefundStatus{
		RefundPending: model.RefundRefunding, RefundSucceeded: model.RefundSucceeded, RefundFailed: model.RefundFailed,
	} {
		if got, ok := refundTarget(state); !ok || got != want {
			t.Errorf("%s：%s %v，預期 %s", state, got, ok, want)
		}
	}
	for _, state := range []RefundState{RefundUnknown, "", "canceled"} {
		if got, ok := refundTarget(state); ok || got != "" {
			t.Errorf("%q 不該有對應：%q", state, got)
		}
	}
}

func refundRow(status, external string) store.PaymentRefund {
	r := store.PaymentRefund{Status: status}
	if external != "" {
		r.ExternalRefundID = &external
	}
	return r
}

// 寫不進去的快照是否已被列涵蓋：同一個 provider 退款而且狀態相同，或列已終局而快照只是還在處理中的舊讀取；其餘（終局衝突、別的狀態、別的退款 ID）是孤兒。
// mutation：refundSettled 對「列已終局、快照是處理中」回 false → 過期的讀取被當孤兒，第三列失敗。
// mutation：refundSettled 不比對外部 ID → 別的退款 ID 被當成已涵蓋，最後一列失敗。
func TestRefundSettledOnlyCoversTheSameProviderRefundInTheSameOrAnEarlierState(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name   string
		cur    store.PaymentRefund
		ext    string
		target model.RefundStatus
		want   bool
	}{
		{"same refund already succeeded", refundRow("SUCCEEDED", "re_1"), "re_1", model.RefundSucceeded, true},
		{"same refund still refunding", refundRow("REFUNDING", "re_1"), "re_1", model.RefundRefunding, true},
		{"stale pending read after success", refundRow("SUCCEEDED", "re_1"), "re_1", model.RefundRefunding, true},
		{"stale pending read after failure", refundRow("FAILED", "re_1"), "re_1", model.RefundRefunding, true},
		{"success against a failed row conflicts", refundRow("FAILED", "re_1"), "re_1", model.RefundSucceeded, false},
		{"failure against a succeeded row conflicts", refundRow("SUCCEEDED", "re_1"), "re_1", model.RefundFailed, false},
		{"anomaly row is not covered", refundRow("ANOMALY", "re_1"), "re_1", model.RefundSucceeded, false},
		{"row without an external id is not covered", refundRow("FAILED", ""), "re_1", model.RefundSucceeded, false},
		{"another provider refund is not covered", refundRow("SUCCEEDED", "re_2"), "re_1", model.RefundSucceeded, false},
	} {
		if got := refundSettled(c.cur, c.ext, c.target); got != c.want {
			t.Errorf("%s：%v，預期 %v", c.name, got, c.want)
		}
	}
}

// ANOMALY 的兩個階段：要撤回而撤回還沒成立是撤回階段，其餘是退款階段（人工結案走不同的證據與去向）。
// mutation：inReversalStage 永遠回 true → 退款階段的列被當撤回階段，需要 FindRefund 證據的結案失敗。
func TestInReversalStageSeparatesTheTwoAnomalyStages(t *testing.T) {
	t.Parallel()
	ext := "trr_1"
	for _, c := range []struct {
		name string
		row  store.PaymentRefund
		want bool
	}{
		{"reversal wanted and not yet done", store.PaymentRefund{ReversalAmount: 30}, true},
		{"reversal done", store.PaymentRefund{ReversalAmount: 30, ReversalExternalID: &ext}, false},
		{"no reversal needed", store.PaymentRefund{}, false},
	} {
		if got := inReversalStage(c.row); got != c.want {
			t.Errorf("%s：%v，預期 %v", c.name, got, c.want)
		}
	}
}

func TestProviderFailureReasonKeepsTheCodeWithinTheColumnLimit(t *testing.T) {
	t.Parallel()
	if got := providerFailureReason(""); got != failedProvider {
		t.Errorf("沒有原因碼：%q", got)
	}
	if got := providerFailureReason(" lost_or_stolen_card "); got != failedProvider+":lost_or_stolen_card" {
		t.Errorf("原因碼：%q", got)
	}
	if got := providerFailureReason(strings.Repeat("字", 500)); len([]rune(got)) != 200 {
		t.Errorf("長度 %d", len([]rune(got)))
	}
}

func fakePaid(t *testing.T, f *Fake, amount int64) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	snap, err := f.CreatePayment(ctx, CreateInput{PaymentID: uuid.NewV7(), OrderNo: "O", StoreID: 1, Amount: amount, Description: "d", TransferGroup: "O", IdempotencyKey: uuid.NewV7().String()})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Settle(snap.ExternalID, StateSucceeded, time.Now()); err != nil {
		t.Fatal(err)
	}
	return snap.ExternalID
}

// fake 能驅動退款的全部結果：處理中、失敗、回應遺失、可重試拒絕、查回（找到、查無、查回失敗）與累計上限。
func TestTheFakeDrivesEveryRefundOutcomeAndLookup(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	f := NewFake()
	pi := fakePaid(t, f, 100)
	f.NextRefundState(RefundPending)
	snap, err := f.CreateRefund(ctx, RefundInput{ExternalPaymentID: pi, Amount: 60, IdempotencyKey: "k1"})
	if err != nil || snap.State != RefundPending {
		t.Fatalf("處理中：%+v %v", snap, err)
	}
	if err := f.SetRefundState(snap.ExternalID, RefundFailed, "declined"); err != nil {
		t.Fatal(err)
	}
	if got, _ := f.GetRefund(ctx, snap.ExternalID); got.State != RefundFailed || got.FailureReason != "declined" {
		t.Fatalf("失敗：%+v", got)
	}
	f.NextRefundState(RefundSucceeded)
	if _, err := f.CreateRefund(ctx, RefundInput{ExternalPaymentID: pi, Amount: 70, IdempotencyKey: "k3"}); err != nil {
		t.Fatalf("k1 失敗後額度釋出：%v", err)
	}
	if _, err := f.CreateRefund(ctx, RefundInput{ExternalPaymentID: pi, Amount: 31, IdempotencyKey: "k4"}); !errors.Is(err, ErrPermanent) {
		t.Fatalf("累計超過付款：%v", err)
	}
	f.FailRefundAfterCreate(errors.New("回應逾時"))
	if _, err := f.CreateRefund(ctx, RefundInput{ExternalPaymentID: pi, Amount: 30, IdempotencyKey: "k5"}); err == nil {
		t.Fatal("回應遺失沒有回錯誤")
	}
	if found, ok, err := f.FindRefund(ctx, pi, "k5"); err != nil || !ok || found.Amount != 30 {
		t.Fatalf("回應遺失之後查回：%+v %v %v", found, ok, err)
	}
	if _, ok, _ := f.FindRefund(ctx, pi, "never"); ok {
		t.Fatal("沒建立過的鍵被找到")
	}
	f.FailLookup(errors.New("查回逾時"))
	if _, _, err := f.FindRefund(ctx, pi, "k5"); err == nil {
		t.Fatal("注入的查回失敗沒有回錯誤")
	}
	f.FailRefund(fmt.Errorf("%w：429", ErrRetryableRefusal))
	if _, err := f.CreateRefund(ctx, RefundInput{ExternalPaymentID: pi, Amount: 1, IdempotencyKey: "k6"}); !errors.Is(err, ErrRetryableRefusal) {
		t.Fatalf("可重試拒絕：%v", err)
	}
	if got := f.RefundKeys(); len(got) != 5 || f.RefundCalls() != 5 || f.Refunds() != 3 {
		t.Fatalf("呼叫 %v 筆數 %d", got, f.Refunds())
	}
	if v := f.Violations(); len(v) != 0 {
		t.Fatalf("Violations=%v", v)
	}
}

func TestTheFakeParsesRefundEventsAsRefundKind(t *testing.T) {
	t.Parallel()
	f := NewFake()
	h := map[string][]string{FakeEventHeader: {"evt_1"}}
	ev, err := f.ParseEvent(h, []byte("fake_re_3|succeeded"))
	if err != nil || ev.Kind != EventRefund || ev.ObjectID != "fake_re_3" {
		t.Fatalf("%+v %v", ev, err)
	}
	ev, err = f.ParseEvent(h, []byte("fake_pi_3"))
	if err != nil || ev.Kind != EventPayment {
		t.Fatalf("%+v %v", ev, err)
	}
}

// fake 的累計上限比較不溢位：已有一筆 40 的退款後再退 MaxInt64 是永久拒絕。
// mutation：fake-refund-cap-overflow（改回 taken+in.Amount > pi.snap.Amount）→ 回繞成負數而被接受，失敗。
func TestTheFakeRefusesAHugeRefundAfterAnExistingOneInsteadOfOverflowing(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	f := NewFake()
	pi := fakePaid(t, f, 100)
	if _, err := f.CreateRefund(ctx, RefundInput{ExternalPaymentID: pi, Amount: 40, IdempotencyKey: "a"}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.CreateRefund(ctx, RefundInput{ExternalPaymentID: pi, Amount: math.MaxInt64, IdempotencyKey: "b"}); !errors.Is(err, ErrPermanent) {
		t.Fatalf("巨額退款：%v", err)
	}
}
