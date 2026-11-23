package charge

import (
	"errors"
	"fmt"
	"testing"

	"github.com/riverqueue/river"
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
