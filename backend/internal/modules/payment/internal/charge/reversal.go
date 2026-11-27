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

const (
	reversalDeducted = "deducted_from_transfer"
	reversalTransferFailed = "transfer_failed"
	reversalNothingLeft = "no_reversible_amount"
	reversalExceedsRemaining = "reversal_exceeds_remaining"
)

// EnqueueReversal 排入一筆退款的轉帳撤回 job（payment.transfer_reversal）：只在退款的 reversal_status 已經是 REVERSING 的同一個 transaction 內呼叫（job 以退款 ID 為唯一鍵，逐列收斂這筆退款進行中的撤回列）。
func (s *Service) EnqueueReversal(ctx context.Context, refundID uuid.UUID) error {
	return enqueueReversal(ctx, s.d.Jobs, refundID)
}

type reversalTarget struct {
	refundID   *uuid.UUID
	externalID string
	free       int64
}

func reversalTargets(ctx context.Context, q *store.Queries, tr store.PaymentTransfer) ([]reversalTarget, error) {
	open, err := q.OpenReversalTotalOnTransfer(ctx, tr.PaymentOrderID)
	if err != nil {
		return nil, err
	}
	targets := []reversalTarget{{externalID: *tr.ExternalTransferID, free: tr.Amount - tr.ReversedAmount - open}}
	tops, err := q.ReversibleTopups(ctx, tr.PaymentOrderID)
	if err != nil {
		return nil, err
	}
	for _, t := range tops {
		if t.ExternalID == nil {
			continue
		}
		id := t.ID
		targets = append(targets, reversalTarget{refundID: &id, externalID: *t.ExternalID, free: t.Free})
	}
	return targets, nil
}

func (s *Service) startReversals(ctx context.Context, q *store.Queries, paymentID uuid.UUID) error {
	rows, err := q.StartableReversals(ctx, paymentID)
	if err != nil || len(rows) == 0 {
		return err
	}
	tr, err := q.SucceededTransferByOrder(ctx, paymentID)
	if err != nil {
		return err
	}
	if tr.ExternalTransferID == nil {
		return fmt.Errorf("charge: 成立的轉帳 %s 沒有外部 ID", tr.ID)
	}
	targets, err := reversalTargets(ctx, q, tr)
	if err != nil {
		return err
	}
	now := s.d.Now().UTC()
	for _, r := range rows {
		type leg struct {
			seq    int32
			target reversalTarget
			amount int64
		}
		var legs []leg
		left := r.ReversalAmount
		for i := range targets {
			take := min(left, max(targets[i].free, 0))
			if take == 0 {
				continue
			}
			targets[i].free -= take
			left -= take
			legs = append(legs, leg{seq: int32(i), target: targets[i], amount: take})
		}
		if left > 0 {
			if err := s.publishOrderAnomaly(ctx, model.AnomalyRefund, r.ID, r.PaymentOrderID, reversalExceedsRemaining); err != nil {
				return err
			}
			if len(legs) == 0 {
				reason := reversalNothingLeft
				if _, err := q.ReleaseRefundReversal(ctx, store.ReleaseRefundReversalParams{ID: r.ID, Reason: &reason, Now: now}); err != nil {
					return err
				}
				continue
			}
			if _, err := q.SetRefundReversalAmount(ctx, store.SetRefundReversalAmountParams{ID: r.ID, ReversalAmount: r.ReversalAmount - left, Now: now}); err != nil {
				return err
			}
		}
		if _, err := q.StartRefundReversal(ctx, store.StartRefundReversalParams{ID: r.ID, Now: now}); err != nil {
			return err
		}
		for _, l := range legs {
			if _, err := q.InsertRefundReversal(ctx, store.InsertRefundReversalParams{
				ID: s.d.NewID(), RefundID: r.ID, PaymentOrderID: r.PaymentOrderID, Seq: l.seq, TargetRefundID: l.target.refundID, TargetExternalID: l.target.externalID, Amount: l.amount, Now: now,
			}); err != nil {
				return err
			}
		}
		if err := enqueueReversal(ctx, s.d.Jobs, r.ID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) releaseWaitingReversals(ctx context.Context, q *store.Queries, paymentID uuid.UUID, reason string) error {
	rows, err := q.WaitingReversals(ctx, paymentID)
	if err != nil {
		return err
	}
	for _, r := range rows {
		if _, err := q.ReleaseRefundReversal(ctx, store.ReleaseRefundReversalParams{ID: r.ID, Reason: &reason, Now: s.d.Now().UTC()}); err != nil {
			return err
		}
	}
	return nil
}

// Reverse 是撤回 job（payment.transfer_reversal）的內容：逐列收斂這筆退款進行中（REVERSING）的撤回列，每一列各自依結果三分（ADR D68，同 Transfer），一列失敗不擋其他列，錯誤合併後回給 River 重試。
//   - 呼叫前先記撤回列的 called_at（這把鍵的第一次呼叫，獨立提交），帶列上的 idempotency_key 向目標轉帳（原轉帳或補差額轉帳）撤回 amount，成功後在一個 transaction 內
//     依序鎖付款列、轉帳列、退款列（依 ID 升序）、撤回列，累加目標的累計撤回額，把撤回列記上 external_id、進 REVERSED，並重算退款列的彙總。賣家餘額不足不是錯誤，這裡沒有那個分支。
//     撤回落地時退款已經 FAILED（接受之後才失敗）：錢留在平台、發 MoneyAnomaly。
//   - 終局拒絕（ErrPermanent）釋出這一列（RELEASED，退款維持原狀、錢由平台吸收）並發 MoneyAnomaly；可重試拒絕換新的撤回冪等鍵並回錯誤由 River 重試，不計期限。
//   - 結果不明同鍵重送到 ResendWindow，之後以 FindReversal 查回：找到就收下，查無滿 NotFoundAfter 才釋出這一列並發 MoneyAnomaly，查回連續失敗進 ANOMALY。
//   - 目標不成立（轉帳不是 SUCCEEDED、補差額轉帳不是 DONE、外部 ID 變了）、amount 超過目標剩餘額度、provider 回的金額或外部 ID 與列不符進 ANOMALY（佔住目標額度、補掃不再重試、發 MoneyAnomaly；job 完成）。
//   - provider 已撤回、列的狀態卻已不是 REVERSING：記 error log 並發 MoneyAnomaly，不靜默（同一個撤回 ID 的重複寫入除外）。
//
// 沒有 REVERSING 的撤回列的退款什麼都不做。final 為 true 時暫時性失敗只記警示 log 並回 nil，留給週期掃描。呼叫方不能在 transaction 內呼叫。
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
	segs, err := store.New(s.d.DB.Conn(ctx)).ReversingReversalsByRefund(ctx, refundID)
	if err != nil {
		return err
	}
	var errs []error
	for _, seg := range segs {
		if err := s.reverseSegment(ctx, seg); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) == 1 {
		return errs[0]
	}
	return errors.Join(errs...)
}

type segmentTarget struct {
	transferID uuid.UUID
	remaining  int64
}

func (s *Service) checkSegmentTarget(ctx context.Context, q *store.Queries, seg store.PaymentRefundReversal) (segmentTarget, string, error) {
	if seg.TargetRefundID == nil {
		tr, err := q.SucceededTransferByOrder(ctx, seg.PaymentOrderID)
		if errors.Is(err, pgx.ErrNoRows) {
			return segmentTarget{}, "no_succeeded_transfer", nil
		}
		if err != nil {
			return segmentTarget{}, "", err
		}
		if tr.ExternalTransferID == nil {
			return segmentTarget{}, "transfer_without_external_id", nil
		}
		if *tr.ExternalTransferID != seg.TargetExternalID {
			return segmentTarget{}, "transfer_changed", nil
		}
		return segmentTarget{transferID: tr.ID, remaining: tr.Amount - tr.ReversedAmount}, "", nil
	}
	t, err := q.RefundByID(ctx, *seg.TargetRefundID)
	if err != nil {
		return segmentTarget{}, "", err
	}
	if t.TopupStatus != string(model.TopupDone) || t.TopupExternalID == nil || *t.TopupExternalID != seg.TargetExternalID {
		return segmentTarget{}, "topup_not_done", nil
	}
	return segmentTarget{remaining: t.Amount - t.TopupReversedAmount}, "", nil
}

func (s *Service) reverseSegment(ctx context.Context, seg store.PaymentRefundReversal) error {
	q := store.New(s.d.DB.Conn(ctx))
	target, reason, err := s.checkSegmentTarget(ctx, q, seg)
	if err != nil {
		return err
	}
	if reason != "" {
		return s.anomalyReversal(ctx, seg, reason, "")
	}
	if seg.Amount > target.remaining {
		return s.anomalyReversal(ctx, seg, reversalExceedsRemaining, "")
	}
	now := s.d.Now().UTC()
	if !inResendWindow(seg.CalledAt, now) {
		return s.lookupReversal(ctx, seg, target, now)
	}
	called, err := s.markReversalCalled(ctx, seg.ID, now)
	if err != nil || called == nil {
		return err
	}
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	snap, err := s.d.Provider.ReverseTransfer(callCtx, ReversalInput{
		ExternalTransferID: seg.TargetExternalID, Amount: called.Amount, IdempotencyKey: called.IdempotencyKey.String(),
	})
	switch s.classify(ctx, "reverse_transfer", err) {
	case callRefused:
		s.d.Logger.WarnContext(ctx, "payment：provider 永久拒絕撤回轉帳", "reversal_id", seg.ID.String(), "refund_id", seg.RefundID.String(), "err", err)
		return s.releaseReversal(ctx, seg, failedRefused)
	case callRetryable:
		if !firstCall(seg.CalledAt, called.CalledAt, now) {
			s.d.Logger.WarnContext(ctx, "payment：重送撤回被拒絕，但這把鍵之前的呼叫結果不明，不換鍵", "reversal_id", seg.ID.String(), "refund_id", seg.RefundID.String(), "err", err)
			return err
		}
		s.d.Logger.WarnContext(ctx, "payment：provider 拒絕撤回轉帳、沒有執行，換新冪等鍵重試", "reversal_id", seg.ID.String(), "refund_id", seg.RefundID.String(), "err", err)
		if rerr := s.rotateReversalKey(ctx, seg.ID, called.IdempotencyKey, now); rerr != nil {
			return errors.Join(err, rerr)
		}
		return err
	case callUnknown:
		return err
	case callSucceeded:
	}
	return s.acceptReversal(ctx, *called, target, snap)
}

func (s *Service) lookupReversal(ctx context.Context, seg store.PaymentRefundReversal, target segmentTarget, now time.Time) error {
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	snap, found, err := s.d.Provider.FindReversal(callCtx, seg.TargetExternalID, seg.IdempotencyKey.String())
	if err != nil {
		return s.reversalLookupFailed(ctx, seg, err)
	}
	if err := s.clearReversalLookupFailures(ctx, seg); err != nil {
		return err
	}
	if found {
		return s.acceptReversal(ctx, seg, target, snap)
	}
	if now.Sub(*seg.CalledAt) < NotFoundAfter {
		return fmt.Errorf("%w：撤回 %s", errNotFoundYet, seg.ID)
	}
	s.d.Logger.WarnContext(ctx, "payment：查回證明 provider 沒有這次撤回，釋出", "reversal_id", seg.ID.String(), "refund_id", seg.RefundID.String())
	return s.releaseReversal(ctx, seg, failedNotFound)
}

func lockReversalRows(ctx context.Context, q *store.Queries, seg store.PaymentRefundReversal, transferID uuid.UUID) error {
	if _, _, err := lockOrder(ctx, q, seg.PaymentOrderID); err != nil {
		return err
	}
	if seg.TargetRefundID == nil {
		if _, err := q.LockTransfer(ctx, transferID); err != nil {
			return err
		}
	}
	ids := []uuid.UUID{seg.RefundID}
	if seg.TargetRefundID != nil {
		ids = append(ids, *seg.TargetRefundID)
	}
	if _, err := q.LockRefundsByID(ctx, ids); err != nil {
		return err
	}
	_, err := q.LockRefundReversal(ctx, seg.ID)
	return err
}

func addTargetReversal(ctx context.Context, q *store.Queries, seg store.PaymentRefundReversal, transferID uuid.UUID, now time.Time) error {
	if seg.TargetRefundID == nil {
		_, err := q.AddTransferReversal(ctx, store.AddTransferReversalParams{ID: transferID, Amount: seg.Amount, Now: now})
		return err
	}
	_, err := q.AddTopupReversal(ctx, store.AddTopupReversalParams{ID: *seg.TargetRefundID, Amount: seg.Amount, Now: now})
	return err
}

func (s *Service) acceptReversal(ctx context.Context, seg store.PaymentRefundReversal, target segmentTarget, snap ReversalSnapshot) error {
	if snap.ExternalID == "" || snap.Amount != seg.Amount {
		return s.anomalyReversal(ctx, seg, "reply_mismatch", snap.ExternalID)
	}
	wctx, cancelWrite := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancelWrite()
	orphan := false
	err := s.d.DB.InTx(wctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		if err := lockReversalRows(ctx, q, seg, target.transferID); err != nil {
			return err
		}
		now := s.d.Now().UTC()
		if _, err := q.ApplyRefundReversal(ctx, store.ApplyRefundReversalParams{ID: seg.ID, ExternalID: &snap.ExternalID, Now: now}); err != nil {
			if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			cur, rerr := q.RefundReversalByID(ctx, seg.ID)
			if rerr != nil {
				return rerr
			}
			if cur.Status == string(model.ReversalReversed) && cur.ExternalID != nil && *cur.ExternalID == snap.ExternalID {
				return nil
			}
			orphan = true
			return s.publishOrderAnomaly(ctx, model.AnomalyReversal, seg.ID, seg.PaymentOrderID, fmt.Sprintf("reversed_but_reversal_%s:%s", cur.Status, snap.ExternalID))
		}
		if err := addTargetReversal(ctx, q, seg, target.transferID, now); err != nil {
			return err
		}
		return s.settleReversal(ctx, q, seg, snap.ExternalID, now)
	})
	if err == nil && orphan {
		s.d.Logger.ErrorContext(ctx, "payment：provider 已撤回轉帳，撤回列的狀態卻已不在 REVERSING，待人工核對",
			"reversal_id", seg.ID.String(), "refund_id", seg.RefundID.String(), "payment_order_id", seg.PaymentOrderID.String(), "reversal_external_id", snap.ExternalID)
	}
	return err
}

func (s *Service) settleReversal(ctx context.Context, q *store.Queries, seg store.PaymentRefundReversal, detail string, now time.Time) error {
	if err := q.SettleRefundReversal(ctx, store.SettleRefundReversalParams{ID: seg.RefundID, Now: now}); err != nil {
		return err
	}
	cur, err := q.RefundByID(ctx, seg.RefundID)
	if err != nil {
		return err
	}
	if cur.Status == string(model.RefundFailed) {
		return s.publishOrderAnomaly(ctx, model.AnomalyRefund, seg.RefundID, seg.PaymentOrderID, withDetail("refund_failed_after_reversal", detail))
	}
	return nil
}

func (s *Service) markReversalCalled(ctx context.Context, id uuid.UUID, now time.Time) (*store.PaymentRefundReversal, error) {
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancel()
	var row *store.PaymentRefundReversal
	err := s.d.DB.InTx(wctx, func(ctx context.Context) error {
		r, err := store.New(s.d.DB.Conn(ctx)).MarkRefundReversalCalled(ctx, store.MarkRefundReversalCalledParams{ID: id, Now: now})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		row = &r
		return err
	})
	return row, err
}

func (s *Service) rotateReversalKey(ctx context.Context, id, oldKey uuid.UUID, now time.Time) error {
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancel()
	return s.d.DB.InTx(wctx, func(ctx context.Context) error {
		_, err := store.New(s.d.DB.Conn(ctx)).RotateReversalKey(ctx, store.RotateReversalKeyParams{ID: id, OldKey: oldKey, Now: now})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	})
}

func (s *Service) reversalLookupFailed(ctx context.Context, seg store.PaymentRefundReversal, cause error) error {
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancel()
	var n int32
	err := s.d.DB.InTx(wctx, func(ctx context.Context) error {
		var err error
		n, err = store.New(s.d.DB.Conn(ctx)).BumpReversalLookupFailures(ctx, store.BumpReversalLookupFailuresParams{ID: seg.ID, Now: s.d.Now().UTC()})
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
		return s.anomalyReversal(ctx, seg, "lookup_failed", "")
	}
	return fmt.Errorf("charge: 撤回 %s 的查回失敗：%w", seg.ID, cause)
}

func (s *Service) clearReversalLookupFailures(ctx context.Context, seg store.PaymentRefundReversal) error {
	if seg.LookupFailures == 0 {
		return nil
	}
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancel()
	return s.d.DB.InTx(wctx, func(ctx context.Context) error {
		return store.New(s.d.DB.Conn(ctx)).ClearReversalLookupFailures(ctx, seg.ID)
	})
}

func (s *Service) anomalyReversal(ctx context.Context, seg store.PaymentRefundReversal, reason, detail string) error {
	s.d.Logger.ErrorContext(ctx, "payment：退款的撤回進 ANOMALY，待人工核對 provider", "reversal_id", seg.ID.String(), "refund_id", seg.RefundID.String(), "payment_order_id", seg.PaymentOrderID.String(), "reason", reason, "external_id", detail)
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancel()
	return s.d.DB.InTx(wctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		if _, _, err := lockOrder(ctx, q, seg.PaymentOrderID); err != nil {
			return err
		}
		if _, err := q.LockRefund(ctx, seg.RefundID); err != nil {
			return err
		}
		now := s.d.Now().UTC()
		r, err := q.AnomalyReversal(ctx, store.AnomalyReversalParams{ID: seg.ID, Reason: &reason, Now: now})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := q.SettleRefundReversal(ctx, store.SettleRefundReversalParams{ID: seg.RefundID, Now: now}); err != nil {
			return err
		}
		return s.publishOrderAnomaly(ctx, model.AnomalyReversal, r.ID, r.PaymentOrderID, withDetail(reason, detail))
	})
}

func (s *Service) releaseReversal(ctx context.Context, seg store.PaymentRefundReversal, reason string) error {
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancel()
	return s.d.DB.InTx(wctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		if _, _, err := lockOrder(ctx, q, seg.PaymentOrderID); err != nil {
			return err
		}
		if _, err := q.LockRefund(ctx, seg.RefundID); err != nil {
			return err
		}
		now := s.d.Now().UTC()
		r, err := q.ReleaseReversal(ctx, store.ReleaseReversalParams{ID: seg.ID, Reason: &reason, Now: now})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := q.SettleRefundReversal(ctx, store.SettleRefundReversalParams{ID: seg.RefundID, Now: now}); err != nil {
			return err
		}
		return s.publishOrderAnomaly(ctx, model.AnomalyReversal, r.ID, r.PaymentOrderID, "reversal_released:"+reason)
	})
}
