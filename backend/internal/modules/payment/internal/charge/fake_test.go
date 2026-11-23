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

// 查回依嘗試 ID（建立時的冪等鍵）比對：建立過就找到，沒建立過是查無（不是錯誤），FailLookup 注入的是查回本身的失敗。
func TestFakeLookupsFindByAttemptIDAndReportNotFoundWithoutAnError(t *testing.T) {
	t.Parallel()
	f, ctx := NewFake(), deadline(t)
	if _, found, err := f.FindPayment(ctx, "k1"); found || err != nil {
		t.Fatalf("沒建立過：found=%v err=%v", found, err)
	}
	created, err := f.CreatePayment(ctx, input("k1"))
	if err != nil {
		t.Fatal(err)
	}
	snap, found, err := f.FindPayment(ctx, "k1")
	if !found || err != nil || snap.ExternalID != created.ExternalID {
		t.Fatalf("建立過：%+v found=%v err=%v", snap, found, err)
	}
	tr, err := f.CreateTransfer(ctx, TransferInput{Amount: 10, SourceChargeID: "ch", DestinationAccountID: "acct", TransferGroup: "O1", IdempotencyKey: "t1"})
	if err != nil {
		t.Fatal(err)
	}
	if got, found, err := f.FindTransfer(ctx, "O1", "t1"); !found || err != nil || got.ExternalID != tr.ExternalID {
		t.Fatalf("轉帳查回：%+v found=%v err=%v", got, found, err)
	}
	if _, found, _ := f.FindTransfer(ctx, "OTHER", "t1"); found {
		t.Fatal("transfer_group 不同卻找到")
	}
	if _, found, _ := f.FindTransfer(ctx, "O1", "t2"); found {
		t.Fatal("嘗試 ID 不同卻找到")
	}
	rev, err := f.ReverseTransfer(ctx, ReversalInput{ExternalTransferID: tr.ExternalID, Amount: 4, IdempotencyKey: "r1"})
	if err != nil {
		t.Fatal(err)
	}
	if got, found, err := f.FindReversal(ctx, tr.ExternalID, "r1"); !found || err != nil || got.ExternalID != rev.ExternalID {
		t.Fatalf("撤回查回：%+v found=%v err=%v", got, found, err)
	}
	if _, found, _ := f.FindReversal(ctx, "other_tr", "r1"); found {
		t.Fatal("轉帳不同卻找到撤回")
	}
	boom := errors.New("查回逾時")
	f.FailLookup(boom)
	if _, found, err := f.FindPayment(ctx, "k1"); found || !errors.Is(err, boom) {
		t.Fatalf("注入的查回失敗：found=%v err=%v", found, err)
	}
	if _, found, err := f.FindPayment(ctx, "k1"); !found || err != nil {
		t.Fatalf("失敗只注入一次：found=%v err=%v", found, err)
	}
	if f.LookupCalls() != 9 {
		t.Fatalf("查回次數=%d", f.LookupCalls())
	}
}

// 四種建立結果都能驅動：可重試拒絕不建立物件；回應遺失已建立物件，同一把鍵重送取回它。
func TestFakeDrivesRetryableRefusalAndLostResponses(t *testing.T) {
	t.Parallel()
	f, ctx := NewFake(), deadline(t)
	f.FailTransfer(ErrRetryableRefusal)
	in := TransferInput{Amount: 10, SourceChargeID: "ch", DestinationAccountID: "acct", TransferGroup: "O1", IdempotencyKey: "t1"}
	if _, err := f.CreateTransfer(ctx, in); !errors.Is(err, ErrRetryableRefusal) || f.Transfers() != 0 {
		t.Fatalf("可重試拒絕：%v，轉帳 %d 筆", err, f.Transfers())
	}
	f.FailTransferAfterCreate(errors.New("回應逾時"))
	if _, err := f.CreateTransfer(ctx, in); err == nil || f.Transfers() != 1 {
		t.Fatalf("回應遺失：%v，轉帳 %d 筆", err, f.Transfers())
	}
	if again, err := f.CreateTransfer(ctx, in); err != nil || again.ExternalID != f.TransferIDs()[0] || f.Transfers() != 1 {
		t.Fatalf("同鍵重送：%+v %v", again, err)
	}
}
