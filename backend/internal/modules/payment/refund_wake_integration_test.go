//go:build integration

package payment_test

import (
	"context"
	"strings"
	"testing"

	. "github.com/Will413028/hawkr/backend/internal/modules/payment"
)

// 退款涵蓋整筆付款、轉帳在等它終局：退款進終局（SUCCEEDED、FAILED，含各條寫入路徑與人工結案）的同一個 transaction 排入喚醒轉帳的 job，
// 賣家不必等 snooze 或週期掃描；等待時 Transfer 回 errRefundsInFlight（final 也不吞），log 指出轉帳與擋住它的退款；人工結案不發 RefundChanged（spec §5）。
func TestARefundReachingATerminalStateWakesTheTransferWaitingOnIt(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		finish   func(t *testing.T, f *chargeFixture, r PaymentRefund)
		prepare  func(f *chargeFixture)
		paid     bool
		previous string
	}{
		{
			name:     "provider refuses the refund call",
			prepare:  func(f *chargeFixture) { f.fake.FailRefund(errRefundDeclined) },
			finish:   func(t *testing.T, f *chargeFixture, r PaymentRefund) { must(t, f.runRefund(r.ID, false)) },
			paid:     true,
			previous: "PENDING",
		},
		{
			name:    "provider reports the accepted refund failed",
			prepare: func(f *chargeFixture) { f.fake.NextRefundState(ChargeRefundPending) },
			finish: func(t *testing.T, f *chargeFixture, r PaymentRefund) {
				must(t, f.runRefund(r.ID, false))
				must(t, f.fake.SetRefundState(f.refundText(t, "external_refund_id", r.ID), ChargeRefundFailed, "declined"))
				f.syncRefund(t, context.Background(), r.ID)
			},
			paid:     true,
			previous: "REFUNDING",
		},
		{
			name:    "provider reports the accepted refund succeeded",
			prepare: func(f *chargeFixture) { f.fake.NextRefundState(ChargeRefundPending) },
			finish: func(t *testing.T, f *chargeFixture, r PaymentRefund) {
				must(t, f.runRefund(r.ID, false))
				must(t, f.fake.SetRefundState(f.refundText(t, "external_refund_id", r.ID), ChargeRefundSucceeded, ""))
				f.syncRefund(t, context.Background(), r.ID)
			},
			paid:     false,
			previous: "REFUNDING",
		},
		{
			name:    "an anomaly resolved as failed",
			prepare: func(f *chargeFixture) {},
			finish: func(t *testing.T, f *chargeFixture, r PaymentRefund) {
				f.exec(t, `UPDATE payment.refunds SET status = 'ANOMALY', failure_reason = 'x' WHERE id = $1`, r.ID)
				must(t, f.module.ResolveAnomaly(context.Background(), f.admin(), AnomalyRefund, r.ID, AnomalyFailed, ResolutionProviderNotExecuted, "核對 provider 之後確認沒有執行", ""))
			},
			paid:     true,
			previous: "ANOMALY",
		},
	}
	for i, c := range cases {
		order := "RW-" + string(rune('1'+i))
		t.Run(c.name, func(t *testing.T) {
			f := setupCharge(t)
			f.paidPayment(t, order, 100)
			f.enableAccount(t, 7, "acct_seller_7", true)
			tr := f.requestTransfer(t, order)
			c.prepare(f)
			r := f.createRefund(t, order, "all", 100, false)
			if err := f.runTransfer(tr.ID, true); err == nil || f.transferText(t, "status", tr.ID) != "PENDING" || f.fake.TransferCalls() != 0 {
				t.Fatalf("退款進行中，轉帳該等且 final 也不吞：%v，%s", err, f.transferText(t, "status", tr.ID))
			}
			if logs := f.logs.String(); !strings.Contains(logs, "轉帳等進行中的退款終局") || !strings.Contains(logs, tr.ID.String()) || !strings.Contains(logs, r.ID.String()) {
				t.Fatalf("log 沒有指出轉帳與擋住它的退款：%s", logs)
			}
			f.clearJobs()
			c.finish(t, f, r)
			if got := f.refundText(t, "status", r.ID); got != "FAILED" && got != "SUCCEEDED" {
				t.Fatalf("前置：退款沒有進終局：%s", got)
			}
			if ids, wakes := f.jobIDs(t, kindTransferCreate, "transfer_id"), f.jobIDs(t, kindTransferCreate, "wake_refund_id"); len(ids) != 1 || ids[0] != tr.ID || len(wakes) != 1 || wakes[0] != r.ID {
				t.Fatalf("退款終局時沒有為等待的轉帳排喚醒 job：%v %v", ids, wakes)
			}
			f.jobs.mu.Lock()
			for _, out := range f.jobs.out {
				if out {
					f.jobs.mu.Unlock()
					t.Fatal("喚醒 job 不是在退款終局的 transaction 內排入的")
				}
			}
			f.jobs.mu.Unlock()
			must(t, f.runTransfer(tr.ID, false))
			if c.paid {
				if got := f.fake.TransferAmounts(); len(got) != 1 || got[0] != 100 || f.transferText(t, "status", tr.ID) != "SUCCEEDED" {
					t.Fatalf("賣家該收到 100：%v，%s", got, f.transferText(t, "status", tr.ID))
				}
			} else if f.fake.TransferCalls() != 0 || f.transferText(t, "status", tr.ID) != "FAILED" || f.transferText(t, "failure_reason", tr.ID) != "fully_refunded" {
				t.Fatalf("退款全部成功：轉帳該以 fully_refunded 終結：%s %s", f.transferText(t, "status", tr.ID), f.transferText(t, "failure_reason", tr.ID))
			}
			if c.previous == "ANOMALY" {
				if len(f.refundChanges) != 0 {
					t.Fatalf("人工結案不發 RefundChanged（§5）：%+v", f.refundChanges)
				}
			} else if len(f.refundChanges) != 1 || f.refundChanges[0].RefundID != r.ID || string(f.refundChanges[0].Previous) != c.previous || !f.refundEventInTx[0] {
				t.Fatalf("退款終局該發一筆 RefundChanged（%s → 終局、在 transaction 內）：%+v", c.previous, f.refundChanges)
			}
			f.noViolations(t)
		})
	}
}

// 轉帳已承諾金額（SUCCEEDED）時，退款進終局不為它排喚醒 job：沒有東西在等。
// mutation：wakeTransfer 拿掉「轉帳還是 PENDING 且 called_at 為空」的判斷 → 為已成立的轉帳排了 job，失敗。
func TestARefundReachingATerminalStateDoesNotWakeATransferThatAlreadyCommittedItsAmount(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	_, tr := f.transferred(t, "RW-9")
	r := f.createRefund(t, "RW-9", "k", 30, false)
	f.fake.FailRefund(errRefundDeclined)
	f.clearJobs()
	must(t, f.runRefund(r.ID, false))
	if f.refundText(t, "status", r.ID) != "FAILED" {
		t.Fatalf("前置：%s", f.refundText(t, "status", r.ID))
	}
	if ids := f.jobIDs(t, kindTransferCreate, "transfer_id"); len(ids) != 0 {
		t.Fatalf("轉帳 %s 已成立，不該被喚醒：%v", tr.ID, ids)
	}
	f.noViolations(t)
}
