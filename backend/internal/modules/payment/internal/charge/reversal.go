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
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// EnqueueReversal 排入一筆退款的轉帳撤回 job（payment.transfer_reversal）：退款 use case 在寫下 REVERSING 退款列的同一個 transaction 內呼叫（job 以退款 ID 為唯一鍵）。
// 呼叫方負責 REVERSING 的列、reversal_amount 與它不超過轉帳剩餘可撤額度。
func (s *Service) EnqueueReversal(ctx context.Context, refundID uuid.UUID) error {
	return enqueueReversal(ctx, s.d.Jobs, refundID)
}

// Reverse 是撤回 job（payment.transfer_reversal）的內容：退款還是 REVERSING 時依結果三分收斂（ADR D68，同 Transfer）。
//   - 呼叫前先記 reversal_called_at（這把鍵的第一次呼叫，獨立提交），帶列上的 reversal_idempotency_key 撤回 reversal_amount，成功後在一個 transaction 內
//     依序鎖付款列、轉帳列、退款列，累加轉帳的 reversed_amount，並把退款記上 reversal_external_id、進 REVERSED。賣家餘額不足不是錯誤，這裡沒有那個分支。
//   - 終局拒絕（ErrPermanent）讓退款進 FAILED（provider_refused，不佔額度）；可重試拒絕換新的撤回冪等鍵並回錯誤由 River 重試，不計期限。
//   - 結果不明同鍵重送到 ResendWindow，之後以 FindReversal 查回：找到就收下，查無滿 NotFoundAfter 才進 FAILED（provider_not_found），查回連續失敗進 ANOMALY。
//   - 退款對不到已成立的轉帳、reversal_amount 超過轉帳剩餘額度、provider 回的金額或外部 ID 與列不符進 ANOMALY（仍佔額度、補掃不再重試、發 MoneyAnomaly；job 完成）。
//   - provider 已撤回、退款卻已不在 REVERSING：記 error log 並發 MoneyAnomaly，不靜默（同一個撤回 ID 的重複寫入除外）。
//
// 已不是 REVERSING 的退款什麼都不做。final 為 true 時暫時性失敗只記警示 log 並回 nil，留給週期掃描。呼叫方不能在 transaction 內呼叫。
func (s *Service) Reverse(ctx context.Context, refundID uuid.UUID, final bool) error {
	if _, ok := db.TxFrom(ctx); ok {
		return errInTransaction
	}
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
		return s.anomalyReversal(ctx, ref, "no_succeeded_transfer")
	}
	if err != nil {
		return err
	}
	if tr.ExternalTransferID == nil || ref.ReversalAmount > tr.Amount-tr.ReversedAmount {
		return s.anomalyReversal(ctx, ref, "reversal_exceeds_remaining")
	}
	now := s.d.Now().UTC()
	if !inResendWindow(ref.ReversalCalledAt, now) {
		return s.lookupReversal(ctx, ref, tr, now)
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
	switch classifyCall(err) {
	case callRefused:
		s.d.Logger.WarnContext(ctx, "payment：provider 永久拒絕撤回轉帳", "refund_id", refundID.String(), "transfer_id", tr.ID.String(), "err", err)
		return s.failReversal(ctx, refundID, failedRefused)
	case callRetryable:
		s.d.Logger.WarnContext(ctx, "payment：provider 拒絕撤回轉帳、沒有執行，換新冪等鍵重試", "refund_id", refundID.String(), "transfer_id", tr.ID.String(), "err", err)
		if rerr := s.rotateReversalKey(ctx, refundID, called.ReversalIdempotencyKey, now); rerr != nil {
			return errors.Join(err, rerr)
		}
		return err
	case callUnknown:
		return err
	case callSucceeded:
	}
	return s.acceptReversal(ctx, *called, tr, snap)
}

// lookupReversal 是過了 ResendWindow 的撤回：不再呼叫 provider，只依嘗試 ID 查回。
func (s *Service) lookupReversal(ctx context.Context, ref store.PaymentRefund, tr store.PaymentTransfer, now time.Time) error {
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	snap, found, err := s.d.Provider.FindReversal(callCtx, *tr.ExternalTransferID, ref.ReversalIdempotencyKey.String())
	if err != nil {
		return s.reversalLookupFailed(ctx, ref, err)
	}
	if err := s.clearReversalLookupFailures(ctx, ref); err != nil {
		return err
	}
	if found {
		return s.acceptReversal(ctx, ref, tr, snap)
	}
	if now.Sub(*ref.ReversalCalledAt) < NotFoundAfter {
		return fmt.Errorf("%w：退款 %s 的撤回", errNotFoundYet, ref.ID)
	}
	s.d.Logger.WarnContext(ctx, "payment：查回證明 provider 沒有這次撤回，退款進 FAILED", "refund_id", ref.ID.String(), "transfer_id", tr.ID.String())
	return s.failReversal(ctx, ref.ID, failedNotFound)
}

// acceptReversal 驗證 provider 回的撤回（呼叫的回應或查回找到的）與列相符後，在一個 transaction 內累加轉帳的 reversed_amount 並把退款推進到 REVERSED；不符進 ANOMALY。
func (s *Service) acceptReversal(ctx context.Context, ref store.PaymentRefund, tr store.PaymentTransfer, snap ReversalSnapshot) error {
	if snap.ExternalID == "" || snap.Amount != ref.ReversalAmount {
		return s.anomalyReversal(ctx, ref, "reply_mismatch:"+snap.ExternalID)
	}
	wctx, cancelWrite := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancelWrite()
	orphan := false
	err := s.d.DB.InTx(wctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		if _, err := q.LockPayment(ctx, ref.PaymentID); err != nil {
			return err
		}
		if _, err := q.LockTransfer(ctx, tr.ID); err != nil {
			return err
		}
		if _, err := q.LockRefund(ctx, ref.ID); err != nil {
			return err
		}
		applied, err := q.ApplyRefundReversal(ctx, store.ApplyRefundReversalParams{ID: ref.ID, ReversalExternalID: &snap.ExternalID, Now: s.d.Now().UTC()})
		if errors.Is(err, pgx.ErrNoRows) {
			cur, rerr := q.RefundByID(ctx, ref.ID)
			if rerr != nil {
				return rerr
			}
			if cur.Status == string(model.RefundReversed) && cur.ReversalExternalID != nil && *cur.ReversalExternalID == snap.ExternalID {
				return nil
			}
			orphan = true
			return s.publishAnomaly(ctx, model.AnomalyRefund, ref.ID, ref.PaymentID, fmt.Sprintf("reversed_but_refund_%s:%s", cur.Status, snap.ExternalID))
		}
		if err != nil {
			return err
		}
		_, err = q.AddTransferReversal(ctx, store.AddTransferReversalParams{ID: tr.ID, Amount: applied.ReversalAmount, Now: s.d.Now().UTC()})
		return err
	})
	if err == nil && orphan {
		s.d.Logger.ErrorContext(ctx, "payment：provider 已撤回轉帳，退款卻已不在 REVERSING，待人工核對",
			"refund_id", ref.ID.String(), "payment_id", ref.PaymentID.String(), "transfer_id", tr.ID.String(), "reversal_external_id", snap.ExternalID)
	}
	return err
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

// rotateReversalKey 在 provider 已知拒絕之後換新的撤回冪等鍵；已被別的呼叫換過或退款已不是 REVERSING 時什麼都不做。
func (s *Service) rotateReversalKey(ctx context.Context, refundID, oldKey uuid.UUID, now time.Time) error {
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancel()
	return s.d.DB.InTx(wctx, func(ctx context.Context) error {
		_, err := store.New(s.d.DB.Conn(ctx)).RotateRefundReversalKey(ctx, store.RotateRefundReversalKeyParams{ID: refundID, OldKey: oldKey, Now: now})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	})
}

// reversalLookupFailed 記一次查回失敗；連續失敗到上限進 ANOMALY（回 nil），否則回錯誤由 River 重試。
func (s *Service) reversalLookupFailed(ctx context.Context, ref store.PaymentRefund, cause error) error {
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancel()
	var n int32
	err := s.d.DB.InTx(wctx, func(ctx context.Context) error {
		var err error
		n, err = store.New(s.d.DB.Conn(ctx)).BumpRefundReversalLookupFailures(ctx, store.BumpRefundReversalLookupFailuresParams{ID: ref.ID, Now: s.d.Now().UTC()})
		if errors.Is(err, pgx.ErrNoRows) {
			n = 0
			return nil
		}
		return err
	})
	if err != nil {
		return errors.Join(cause, err)
	}
	if n >= LookupFailureLimit {
		return s.anomalyReversal(ctx, ref, "lookup_failed")
	}
	return fmt.Errorf("charge: 退款 %s 的撤回查回失敗：%w", ref.ID, cause)
}

func (s *Service) clearReversalLookupFailures(ctx context.Context, ref store.PaymentRefund) error {
	if ref.ReversalLookupFailures == 0 {
		return nil
	}
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancel()
	return s.d.DB.InTx(wctx, func(ctx context.Context) error {
		return store.New(s.d.DB.Conn(ctx)).ClearRefundReversalLookupFailures(ctx, ref.ID)
	})
}

// anomalyReversal 讓 REVERSING 的退款進 ANOMALY 並在同一個 transaction 發 MoneyAnomaly；退款已不是 REVERSING 時什麼都不做。
func (s *Service) anomalyReversal(ctx context.Context, ref store.PaymentRefund, reason string) error {
	s.d.Logger.ErrorContext(ctx, "payment：退款的撤回進 ANOMALY，待人工核對 provider", "refund_id", ref.ID.String(), "payment_id", ref.PaymentID.String(), "reason", reason)
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancel()
	return s.d.DB.InTx(wctx, func(ctx context.Context) error {
		r, err := store.New(s.d.DB.Conn(ctx)).AnomalyRefundReversal(ctx, store.AnomalyRefundReversalParams{ID: ref.ID, Reason: &reason, Now: s.d.Now().UTC()})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		return s.publishAnomaly(ctx, model.AnomalyRefund, r.ID, r.PaymentID, reason)
	})
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
