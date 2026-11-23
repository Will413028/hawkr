package charge

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"testing"
	"time"
)

func deadline(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)
	return ctx
}

func input(key string) CreateInput {
	return CreateInput{OrderNo: "O1", StoreID: 1, Amount: 100, TransferGroup: "O1", IdempotencyKey: key, Description: "d"}
}

// 同一把冪等鍵回同一個 PaymentIntent；同一把鍵送了不同內容是永久失敗（Stripe 的 idempotency_error）。
func TestFakeReplaysTheSameIdempotencyKeyAndRejectsADifferentBody(t *testing.T) {
	t.Parallel()
	f, ctx := NewFake(), deadline(t)
	first, err := f.CreatePayment(ctx, input("k1"))
	if err != nil {
		t.Fatal(err)
	}
	again, err := f.CreatePayment(ctx, input("k1"))
	if err != nil || again.ExternalID != first.ExternalID || f.Intents() != 1 {
		t.Fatalf("重送=%+v err=%v，PaymentIntent %d 個", again, err, f.Intents())
	}
	different := input("k1")
	different.Amount = 200
	if _, err := f.CreatePayment(ctx, different); !errors.Is(err, ErrPermanent) {
		t.Fatalf("同一把鍵不同金額：%v", err)
	}
	if got := f.CreateKeys(); !slices.Equal(got, []string{"k1", "k1", "k1"}) {
		t.Fatalf("記下的冪等鍵=%v", got)
	}
}

// 每個呼叫都要有 deadline；沒有的記在 Violations。
func TestFakeRecordsACallWithoutADeadline(t *testing.T) {
	t.Parallel()
	f := NewFake()
	if _, err := f.CreatePayment(context.Background(), input("k")); err != nil {
		t.Fatal(err)
	}
	if _, err := f.GetPayment(context.Background(), "fake_pi_1"); err != nil {
		t.Fatal(err)
	}
	if got := f.Violations(); len(got) != 2 {
		t.Fatalf("違規紀錄=%v", got)
	}
}

// 事件的 body 可以帶「宣稱的狀態」，ParseEvent 只取物件 ID，不把宣稱交出去。
func TestFakeParseEventIgnoresTheClaimedState(t *testing.T) {
	t.Parallel()
	f := NewFake()
	h := http.Header{}
	h.Set(FakeEventHeader, "evt_1")
	ev, err := f.ParseEvent(h, []byte("fake_pi_7|succeeded"))
	if err != nil || ev != (Event{ID: "evt_1", Kind: EventPayment, ObjectID: "fake_pi_7"}) {
		t.Fatalf("事件=%+v err=%v", ev, err)
	}
	if _, err := f.ParseEvent(http.Header{}, []byte("fake_pi_7")); !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("沒有事件 ID：%v", err)
	}
	h.Set(FakeIgnoredEventHeader, "1")
	if _, err := f.ParseEvent(h, []byte("fake_pi_7")); !errors.Is(err, ErrIgnoredEvent) {
		t.Fatalf("不處理的事件：%v", err)
	}
}

// 撤回超過轉帳的剩餘額度是永久失敗，部分撤回累計；退款超過付款金額是永久失敗（3b、3c 的測試靠 Fake 驅動）。
func TestFakeTransferReversalsAccumulateAndCannotExceedTheTransfer(t *testing.T) {
	t.Parallel()
	f, ctx := NewFake(), deadline(t)
	tr, err := f.CreateTransfer(ctx, TransferInput{Amount: 100, SourceChargeID: "ch", DestinationAccountID: "acct", TransferGroup: "O1", IdempotencyKey: "t1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.ReverseTransfer(ctx, ReversalInput{ExternalTransferID: tr.ExternalID, Amount: 60, IdempotencyKey: "r1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.ReverseTransfer(ctx, ReversalInput{ExternalTransferID: tr.ExternalID, Amount: 60, IdempotencyKey: "r2"}); !errors.Is(err, ErrPermanent) {
		t.Fatalf("超過剩餘額度：%v", err)
	}
	if _, err := f.ReverseTransfer(ctx, ReversalInput{ExternalTransferID: tr.ExternalID, Amount: 40, IdempotencyKey: "r3"}); err != nil {
		t.Fatal(err)
	}
	snap, err := f.CreatePayment(ctx, input("k"))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Settle(snap.ExternalID, StateSucceeded, time.Unix(1, 0)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.CreateRefund(ctx, RefundInput{ExternalPaymentID: snap.ExternalID, Amount: 101, IdempotencyKey: "x"}); !errors.Is(err, ErrPermanent) {
		t.Fatalf("退款超過付款：%v", err)
	}
	if got, err := f.CreateRefund(ctx, RefundInput{ExternalPaymentID: snap.ExternalID, Amount: 30, IdempotencyKey: "x"}); err != nil || got.State != RefundSucceeded {
		t.Fatalf("退款=%+v err=%v", got, err)
	}
	if v := f.Violations(); len(v) != 0 {
		t.Fatalf("違規=%v", v)
	}
}
