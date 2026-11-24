package charge

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/riverqueue/river"
	"uuid"
)

// 轉帳與撤回的錯誤分類：只有 ErrPermanent 取消 job（不重試）；平台餘額不足與逾時是一般錯誤，job 照常重試。
func TestOnlyAPermanentTransferErrorCancelsTheJob(t *testing.T) {
	t.Parallel()
	insufficient := errors.New("insufficient available funds")
	var cancel *river.JobCancelError
	if err := jobError(insufficient); !errors.Is(err, insufficient) || errors.As(err, &cancel) {
		t.Fatalf("平台餘額不足被當成永久失敗：%v", err)
	}
	if err := jobError(fmt.Errorf("%w：被拒絕", ErrPermanent)); !errors.As(err, &cancel) {
		t.Fatalf("永久失敗沒有取消 job：%v", err)
	}
	if err := jobError(ErrAnomaly); !errors.As(err, &cancel) {
		t.Fatalf("不一致的快照沒有取消 job：%v", err)
	}
}

// Fake 能驅動轉帳與撤回的四種結果：成功、永久失敗、平台餘額不足（一般錯誤）、回應遺失（已建立但呼叫方拿不到結果，同一把鍵取回同一筆）。
func TestFakeDrivesTransferAndReversalOutcomes(t *testing.T) {
	t.Parallel()
	f, ctx := NewFake(), deadline(t)
	in := TransferInput{Amount: 100, SourceChargeID: "ch", DestinationAccountID: "acct", TransferGroup: "O1", IdempotencyKey: "t1"}
	f.FailTransfer(errors.New("insufficient available funds"), fmt.Errorf("%w：目的帳戶不能收款", ErrPermanent))
	if _, err := f.CreateTransfer(ctx, in); err == nil || errors.Is(err, ErrPermanent) {
		t.Fatalf("餘額不足：%v", err)
	}
	if _, err := f.CreateTransfer(ctx, in); !errors.Is(err, ErrPermanent) {
		t.Fatalf("永久失敗：%v", err)
	}
	f.FailTransferAfterCreate(errors.New("逾時"))
	if _, err := f.CreateTransfer(ctx, in); err == nil || f.Transfers() != 1 {
		t.Fatalf("回應遺失：%v，provider 有 %d 筆", err, f.Transfers())
	}
	got, err := f.CreateTransfer(ctx, in)
	if err != nil || got.ExternalID == "" || f.Transfers() != 1 || f.TransferCalls() != 4 {
		t.Fatalf("同一把鍵取回=%+v err=%v，provider %d 筆，%d 次呼叫", got, err, f.Transfers(), f.TransferCalls())
	}
	rin := ReversalInput{ExternalTransferID: got.ExternalID, Amount: 40, IdempotencyKey: "r1"}
	f.FailReversalAfterCreate(errors.New("逾時"))
	if _, err := f.ReverseTransfer(ctx, rin); err == nil {
		t.Fatal("撤回的回應遺失沒有回錯誤")
	}
	back, err := f.ReverseTransfer(ctx, rin)
	if err != nil || back.Amount != 40 || f.ReversalCalls() != 2 {
		t.Fatalf("撤回同一把鍵取回=%+v err=%v", back, err)
	}
	if keys := f.TransferKeys(); len(keys) != 4 || keys[3] != "t1" {
		t.Fatalf("轉帳冪等鍵=%v", keys)
	}
	if v := f.Violations(); len(v) != 0 {
		t.Fatalf("違規=%v", v)
	}
}

// 同一把冪等鍵送不同的金額是永久失敗（Stripe 比對參數）：轉帳的金額綁著冪等鍵，只有換新鍵才會重算。
// mutation：Fake.CreateTransfer 拿掉金額比對 → 同鍵不同額被當成重送成功，失敗。
func TestFakeRefusesTheSameTransferKeyWithAnotherAmount(t *testing.T) {
	t.Parallel()
	f, ctx := NewFake(), deadline(t)
	in := TransferInput{Amount: 100, SourceChargeID: "ch", DestinationAccountID: "acct", TransferGroup: "O1", IdempotencyKey: "t1"}
	if _, err := f.CreateTransfer(ctx, in); err != nil {
		t.Fatal(err)
	}
	in.Amount = 60
	if _, err := f.CreateTransfer(ctx, in); !errors.Is(err, ErrPermanent) {
		t.Fatalf("同鍵不同額：%v", err)
	}
	if got := f.TransferAmounts(); len(got) != 2 || got[0] != 100 || got[1] != 60 || f.Transfers() != 1 {
		t.Fatalf("金額=%v，轉帳 %d 筆", got, f.Transfers())
	}
}

// 轉帳等進行中的退款終局是預期的等待：job 以 transferWaitDelay snooze（不計嘗試、不是錯誤），其他錯誤照 jobError。
// mutation：transferJobResult 對 errRefundsInFlight 改回回傳錯誤 → 不是 snooze，失敗。
func TestAWaitingTransferSnoozesTheJobInsteadOfFailingIt(t *testing.T) {
	t.Parallel()
	waiting := fmt.Errorf("外層：%w", &refundsInFlightError{transferID: uuid.New(), refundIDs: []uuid.UUID{uuid.New()}})
	var snooze *river.JobSnoozeError
	if err := transferJobResult(waiting); !errors.As(err, &snooze) || snooze.Duration != transferWaitDelay {
		t.Fatalf("等進行中的退款沒有 snooze：%v", err)
	}
	if err := transferJobResult(nil); err != nil {
		t.Fatalf("成功：%v", err)
	}
	other := errors.New("insufficient available funds")
	if err := transferJobResult(other); !errors.Is(err, other) || errors.As(err, &snooze) {
		t.Fatalf("其他錯誤不該 snooze：%v", err)
	}
	var cancel *river.JobCancelError
	if err := transferJobResult(fmt.Errorf("%w：被拒絕", ErrPermanent)); !errors.As(err, &cancel) {
		t.Fatalf("永久失敗沒有取消 job：%v", err)
	}
}

// 喚醒轉帳的 job 以退款 ID 進唯一鍵：同一筆轉帳正在 snooze 的 job（WakeRefundID 為零值）不會把它合併掉，同一筆退款重複喚醒則合併。
// mutation：TransferArgs.WakeRefundID 拿掉 river:"unique" → 喚醒被 snooze 中的 job 合併而沒有效果，失敗。
func TestTheTransferWakeJobIsDistinctFromTheSnoozedOneByTheRefundID(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"TransferID", "WakeRefundID"} {
		field, ok := reflect.TypeOf(TransferArgs{}).FieldByName(name)
		if !ok || field.Tag.Get("river") != "unique" {
			t.Fatalf("%s 不在唯一鍵內：%v", name, field.Tag)
		}
	}
	if opts := (TransferArgs{}).InsertOpts(); !opts.UniqueOpts.ByArgs {
		t.Fatalf("轉帳 job 不依參數去重：%+v", opts.UniqueOpts)
	}
}
