package charge

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/store"
)

// EnqueueReversal 排入一筆退款的轉帳撤回 job（payment.transfer_reversal）：3c 的退款 use case 在寫下 REVERSING 退款列的同一個 transaction 內呼叫（job 以退款 ID 為唯一鍵）。
// 呼叫方負責 REVERSING 的列、reversal_amount 與它不超過轉帳剩餘可撤額度。
func (s *Service) EnqueueReversal(ctx context.Context, refundID uuid.UUID) error {
	return enqueueReversal(ctx, s.d.Jobs, refundID)
}

// Reverse 是撤回 job（payment.transfer_reversal）的內容：退款還是 REVERSING 時，呼叫 provider 之前先記 reversal_called_at（獨立提交），
// 帶列上的 reversal_idempotency_key 撤回 reversal_amount，成功後在一個 transaction 內依序鎖付款列、轉帳列、退款列，累加轉帳的 reversed_amount，
// 並把退款記上 reversal_external_id、進 REVERSED（之後的退款呼叫是 3c）。賣家餘額不足不是錯誤（spikes.md #9），這裡沒有那個分支。
// 已不是 REVERSING 的退款什麼都不做（同一把鍵的重送不會重複累加）。provider 永久拒絕進終局：退款 FAILED（reversal_refused，不佔額度）；
// 呼叫過、沒有撤回 ID 超過 UnresolvedDeadline 同樣進 FAILED（unresolved_reversal，鍵不再重送；provider 那邊可能已撤回，需後台核對）。暫時性失敗回錯誤由 River 重試，
// final 為 true 時只記警示 log 並回 nil，留給週期掃描。退款對不到已成立的轉帳、或 reversal_amount 超過轉帳剩餘額度、或 provider 回的金額與列不符回 ErrAnomaly（不呼叫或不寫入）。
func (s *Service) Reverse(ctx context.Context, refundID uuid.UUID, final bool) error {
	err := s.reverse(ctx, refundID)
	if err != nil && final && !errors.Is(err, ErrPermanent) {
		s.d.Logger.WarnContext(ctx, "payment：轉帳撤回用完重試，留給週期掃描", "refund_id", refundID.String(), "err", err)
		return nil
	}
	return err
}

func (s *Service) reverse(ctx context.Context, refundID uuid.UUID) error {
	if err := s.requireProvider(); err != nil {
		return err
	}
	q := store.New(s.d.DB.Conn(ctx))
	ref, err := q.RefundByID(ctx, refundID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if ref.Status != string(model.RefundReversing) {
		return nil
	}
	tr, err := q.SucceededTransferByPayment(ctx, ref.PaymentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%w：退款 %s 要撤回，付款卻沒有已成立的轉帳", ErrAnomaly, refundID)
	}
	if err != nil {
		return err
	}
	if tr.ExternalTransferID == nil || ref.ReversalAmount > tr.Amount-tr.ReversedAmount {
		return fmt.Errorf("%w：退款 %s 的撤回額度 %d 超過轉帳的剩餘可撤額度", ErrAnomaly, refundID, ref.ReversalAmount)
	}
	now := s.d.Now().UTC()
	if ref.ReversalCalledAt != nil && now.Sub(*ref.ReversalCalledAt) > UnresolvedDeadline {
		s.d.Logger.ErrorContext(ctx, "payment：撤回呼叫過、結果不明超過期限，退款進終局", "refund_id", refundID.String(), "transfer_id", tr.ID.String())
		return s.failReversal(ctx, refundID, failedReversalUnresolved)
	}
	called, err := s.markReversalCalled(ctx, refundID, now)
	if err != nil || called == nil {
		return err
	}
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	snap, err := s.d.Provider.ReverseTransfer(callCtx, ReversalInput{
		ExternalTransferID: *tr.ExternalTransferID, Amount: called.ReversalAmount, IdempotencyKey: called.ReversalIdempotencyKey.String(),
	})
	if errors.Is(err, ErrPermanent) {
		s.d.Logger.WarnContext(ctx, "payment：provider 永久拒絕撤回轉帳", "refund_id", refundID.String(), "transfer_id", tr.ID.String(), "err", err)
		return s.failReversal(ctx, refundID, failedRefused)
	}
	if err != nil {
		return err
	}
	if snap.ExternalID == "" || snap.Amount != called.ReversalAmount {
		s.d.Logger.ErrorContext(ctx, "payment：provider 回的撤回與列不一致，不寫入", "refund_id", refundID.String(), "reversal_external_id", snap.ExternalID)
		return fmt.Errorf("%w：撤回的外部 ID 或金額與列不同", ErrAnomaly)
	}
	wctx, cancelWrite := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancelWrite()
	return s.d.DB.InTx(wctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		if _, err := q.LockPayment(ctx, ref.PaymentID); err != nil {
			return err
		}
		if _, err := q.LockTransfer(ctx, tr.ID); err != nil {
			return err
		}
		if _, err := q.LockRefund(ctx, refundID); err != nil {
			return err
		}
		applied, err := q.ApplyRefundReversal(ctx, store.ApplyRefundReversalParams{ID: refundID, ReversalExternalID: &snap.ExternalID, Now: s.d.Now().UTC()})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		_, err = q.AddTransferReversal(ctx, store.AddTransferReversalParams{ID: tr.ID, Amount: applied.ReversalAmount, Now: s.d.Now().UTC()})
		return err
	})
}

// markReversalCalled 在獨立的 transaction 記下「撤回呼叫過」的時刻（呼叫 provider 之前提交）；退款已不是 REVERSING 時回 nil。
func (s *Service) markReversalCalled(ctx context.Context, refundID uuid.UUID, now time.Time) (*store.PaymentRefund, error) {
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancel()
	var row *store.PaymentRefund
	err := s.d.DB.InTx(wctx, func(ctx context.Context) error {
		r, err := store.New(s.d.DB.Conn(ctx)).MarkRefundReversalCalled(ctx, store.MarkRefundReversalCalledParams{ID: refundID, Now: now})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		row = &r
		return err
	})
	return row, err
}

// failReversal 讓 REVERSING 的退款進終局 FAILED；退款已不是 REVERSING 時什麼都不做。
func (s *Service) failReversal(ctx context.Context, refundID uuid.UUID, reason string) error {
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancel()
	return s.d.DB.InTx(wctx, func(ctx context.Context) error {
		_, err := store.New(s.d.DB.Conn(ctx)).FailRefundReversal(ctx, store.FailRefundReversalParams{ID: refundID, FailureReason: &reason, Now: s.d.Now().UTC()})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	})
}
