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

// 向賣家收回的預留釋出原因碼（refunds.reversal_reason，上限 200 字）。
const (
	// reversalDeducted 退款已從轉帳金額扣掉（轉帳在送出前重新算定金額），不必撤回。
	reversalDeducted = "deducted_from_transfer"
	// reversalTransferFailed 轉帳終局失敗，錢本來就在平台手上。
	reversalTransferFailed = "transfer_failed"
	// reversalNothingLeft 轉帳人工結案後，provider 核實的轉帳沒有剩餘可撤額度給這筆退款。
	reversalNothingLeft = "no_reversible_amount"
)

// EnqueueReversal 排入一筆退款的轉帳撤回 job（payment.transfer_reversal）：只在退款的 reversal_status 已經是 REVERSING 的同一個 transaction 內呼叫（job 以退款 ID 為唯一鍵）。
func (s *Service) EnqueueReversal(ctx context.Context, refundID uuid.UUID) error {
	return enqueueReversal(ctx, s.d.Jobs, refundID)
}

// startReversals 讓付款底下「預留在 WAITING、provider 已接受退款（REFUNDING 或 SUCCEEDED）」的退款開始撤回（REVERSING）並排撤回 job。
// 呼叫方必須已鎖付款列與（成立的）轉帳列、並確認轉帳是 SUCCEEDED：撤回的兩個前置事件（退款被接受、轉帳成立）誰後到，誰在自己的 transaction 內呼叫它。
func (s *Service) startReversals(ctx context.Context, q *store.Queries, paymentID uuid.UUID) error {
	rows, err := q.StartableReversals(ctx, paymentID)
	if err != nil {
		return err
	}
	for _, r := range rows {
		started, err := q.StartRefundReversal(ctx, store.StartRefundReversalParams{ID: r.ID, Now: s.d.Now().UTC()})
		if err != nil {
			return err
		}
		if err := enqueueReversal(ctx, s.d.Jobs, started.ID); err != nil {
			return err
		}
	}
	return nil
}

// releaseWaitingReversals 釋出付款底下全部 WAITING 的預留（轉帳金額重新算定、或轉帳終局失敗）。呼叫方必須已鎖付款列與轉帳列。
func (s *Service) releaseWaitingReversals(ctx context.Context, q *store.Queries, paymentID uuid.UUID, reason string) error {
	rows, err := q.WaitingReversals(ctx, paymentID)
	if err != nil {
		return err
	}
	for _, r := range rows {
		if _, err := q.ReleaseRefundReversal(ctx, store.ReleaseRefundReversalParams{ID: r.ID, Reason: &reason, FromStatuses: []string{string(model.ReversalWaiting)}, Now: s.d.Now().UTC()}); err != nil {
			return err
		}
	}
	return nil
}

// Reverse 是撤回 job（payment.transfer_reversal）的內容：退款列的 reversal_status 是 REVERSING 時依結果三分收斂（ADR D68，同 Transfer）。
//   - 呼叫前先記 reversal_called_at（這把鍵的第一次呼叫，獨立提交），帶列上的 reversal_idempotency_key 撤回 reversal_amount，成功後在一個 transaction 內
//     依序鎖付款列、轉帳列、退款列，累加轉帳的 reversed_amount，並把退款記上 reversal_external_id、進 REVERSED。賣家餘額不足不是錯誤，這裡沒有那個分支。
//     撤回落地時退款已經 FAILED（接受之後才失敗）：錢留在平台、發 MoneyAnomaly。
//   - 終局拒絕（ErrPermanent）釋出預留（RELEASED，退款維持原狀、錢由平台吸收）並發 MoneyAnomaly；可重試拒絕換新的撤回冪等鍵並回錯誤由 River 重試，不計期限。
//   - 結果不明同鍵重送到 ResendWindow，之後以 FindReversal 查回：找到就收下，查無滿 NotFoundAfter 才釋出預留並發 MoneyAnomaly，查回連續失敗進 ANOMALY。
//   - 轉帳不是 SUCCEEDED、reversal_amount 超過轉帳剩餘額度、provider 回的金額或外部 ID 與列不符進 ANOMALY（預留持續佔住、補掃不再重試、發 MoneyAnomaly；job 完成）。
//   - provider 已撤回、列的 reversal_status 卻已不是 REVERSING：記 error log 並發 MoneyAnomaly，不靜默（同一個撤回 ID 的重複寫入除外）。
//
// reversal_status 不是 REVERSING 的退款什麼都不做。final 為 true 時暫時性失敗只記警示 log 並回 nil，留給週期掃描。呼叫方不能在 transaction 內呼叫。
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
	if ref.ReversalStatus != string(model.ReversalReversing) {
		return nil
	}
	tr, err := q.SucceededTransferByPayment(ctx, ref.PaymentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return s.anomalyReversal(ctx, ref, "no_succeeded_transfer", "")
	}
	if err != nil {
		return err
	}
	if tr.ExternalTransferID == nil {
		return s.anomalyReversal(ctx, ref, "transfer_without_external_id", "")
	}
	if ref.ReversalAmount > tr.Amount-tr.ReversedAmount {
		return s.anomalyReversal(ctx, ref, "reversal_exceeds_remaining", "")
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
		return s.releaseReversal(ctx, refundID, ref.PaymentID, failedRefused)
	case callRetryable:
		if !firstCall(ref.ReversalCalledAt, called.ReversalCalledAt, now) {
			s.d.Logger.WarnContext(ctx, "payment：重送撤回被拒絕，但這把鍵之前的呼叫結果不明，不換鍵", "refund_id", refundID.String(), "transfer_id", tr.ID.String(), "err", err)
			return err
		}
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
	s.d.Logger.WarnContext(ctx, "payment：查回證明 provider 沒有這次撤回，釋出預留", "refund_id", ref.ID.String(), "transfer_id", tr.ID.String())
	return s.releaseReversal(ctx, ref.ID, ref.PaymentID, failedNotFound)
}

// acceptReversal 驗證 provider 回的撤回（呼叫的回應或查回找到的）與列相符後，在一個 transaction 內累加轉帳的 reversed_amount 並把撤回推進到 REVERSED；不符進 ANOMALY。
// 退款在撤回落地時已經 FAILED（provider 接受之後才失敗）：撤回照樣成立、錢留在平台，同 transaction 發 MoneyAnomaly 交後台決定。
func (s *Service) acceptReversal(ctx context.Context, ref store.PaymentRefund, tr store.PaymentTransfer, snap ReversalSnapshot) error {
	if snap.ExternalID == "" || snap.Amount != ref.ReversalAmount {
		return s.anomalyReversal(ctx, ref, "reply_mismatch", snap.ExternalID)
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
			if cur.ReversalStatus == string(model.ReversalReversed) && cur.ReversalExternalID != nil && *cur.ReversalExternalID == snap.ExternalID {
				return nil
			}
			orphan = true
			return s.publishAnomaly(ctx, model.AnomalyReversal, ref.ID, ref.PaymentID, fmt.Sprintf("reversed_but_reversal_%s:%s", cur.ReversalStatus, snap.ExternalID))
		}
		if err != nil {
			return err
		}
		if _, err = q.AddTransferReversal(ctx, store.AddTransferReversalParams{ID: tr.ID, Amount: applied.ReversalAmount, Now: s.d.Now().UTC()}); err != nil {
			return err
		}
		if applied.Status == string(model.RefundFailed) {
			return s.publishAnomaly(ctx, model.AnomalyRefund, ref.ID, ref.PaymentID, withDetail("refund_failed_after_reversal", snap.ExternalID))
		}
		return nil
	})
	if err == nil && orphan {
		s.d.Logger.ErrorContext(ctx, "payment：provider 已撤回轉帳，列的撤回狀態卻已不在 REVERSING，待人工核對",
			"refund_id", ref.ID.String(), "payment_id", ref.PaymentID.String(), "transfer_id", tr.ID.String(), "reversal_external_id", snap.ExternalID)
	}
	return err
}

// markReversalCalled 在獨立的 transaction 記下「撤回呼叫過」的時刻（呼叫 provider 之前提交）；撤回已不是 REVERSING 時回 nil。
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

// rotateReversalKey 在 provider 已知拒絕之後換新的撤回冪等鍵；已被別的呼叫換過或撤回已不是 REVERSING 時什麼都不做。
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
		return s.anomalyReversal(ctx, ref, "lookup_failed", "")
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

// anomalyReversal 讓 REVERSING 的撤回進 ANOMALY 並在同一個 transaction 發 MoneyAnomaly；撤回已不是 REVERSING 時什麼都不做。
// 列上只存原因碼 reason（reversal_reason，上限 200 字）；provider 的外部 ID（detail）只進 log 與事件。
func (s *Service) anomalyReversal(ctx context.Context, ref store.PaymentRefund, reason, detail string) error {
	s.d.Logger.ErrorContext(ctx, "payment：退款的撤回進 ANOMALY，待人工核對 provider", "refund_id", ref.ID.String(), "payment_id", ref.PaymentID.String(), "reason", reason, "external_id", detail)
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
		return s.publishAnomaly(ctx, model.AnomalyReversal, r.ID, r.PaymentID, withDetail(reason, detail))
	})
}

// releaseReversal 是撤回的終局失敗（provider 永久拒絕，或查回證明上一次不存在又過了最短等待）：REVERSING 的預留釋出、退款維持原狀，
// 錢留在平台由平台吸收，同一個 transaction 發 MoneyAnomaly；撤回已不是 REVERSING 時什麼都不做。
func (s *Service) releaseReversal(ctx context.Context, refundID, paymentID uuid.UUID, reason string) error {
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancel()
	return s.d.DB.InTx(wctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		if _, err := q.LockPayment(ctx, paymentID); err != nil {
			return err
		}
		if _, err := q.LockRefund(ctx, refundID); err != nil {
			return err
		}
		r, err := q.ReleaseRefundReversal(ctx, store.ReleaseRefundReversalParams{ID: refundID, Reason: &reason, FromStatuses: []string{string(model.ReversalReversing)}, Now: s.d.Now().UTC()})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		return s.publishAnomaly(ctx, model.AnomalyReversal, r.ID, r.PaymentID, "reversal_released:"+reason)
	})
}
