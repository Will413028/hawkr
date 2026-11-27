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
	topupTransferFailed = "transfer_failed"
	topupCountedInNewAmount = "counted_in_new_amount"
)

var errTopupRekeyed = errors.New("charge: 補差額上一把鍵查無，已換新鍵")

func (s *Service) startTopups(ctx context.Context, q *store.Queries, paymentID uuid.UUID) error {
	rows, err := q.WaitingTopups(ctx, paymentID)
	if err != nil {
		return err
	}
	for _, r := range rows {
		started, err := q.StartRefundTopup(ctx, store.StartRefundTopupParams{ID: r.ID, Now: s.d.Now().UTC()})
		if err != nil {
			return err
		}
		if err := enqueueTopup(ctx, s.d.Jobs, started.ID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) settleTopupsWithoutTransfer(ctx context.Context, q *store.Queries, paymentID uuid.UUID) error {
	if err := s.releaseWaitingTopups(ctx, q, paymentID, topupTransferFailed); err != nil {
		return err
	}
	return q.ClearRefundsDeducted(ctx, paymentID)
}

func (s *Service) releaseWaitingTopups(ctx context.Context, q *store.Queries, paymentID uuid.UUID, reason string) error {
	rows, err := q.WaitingTopups(ctx, paymentID)
	if err != nil {
		return err
	}
	for _, r := range rows {
		if _, err := q.ReleaseRefundTopup(ctx, store.ReleaseRefundTopupParams{ID: r.ID, Reason: &reason, FromStatuses: []string{string(model.TopupWaiting)}, Now: s.d.Now().UTC()}); err != nil {
			return err
		}
	}
	return nil
}

// TopUp 是補差額轉帳 job（payment.transfer_topup）的內容（D11 補充）：已算進轉帳金額的退款失敗之後，賣家少收了那筆錢，補一筆轉帳給賣家，不留在平台。
// 退款列的 topup_status 是 SENDING 時依結果三分收斂（ADR D68，同 Transfer）：
//   - 呼叫前先記 topup_called_at（這把鍵的第一次呼叫，獨立提交），帶列上的 topup_idempotency_key 把退款額轉給賣家（同付款的 charge、transfer_group 與轉帳的收款帳戶），
//     成功後在一個 transaction 內鎖付款列、退款列，進 DONE 並記 topup_external_id。補差額轉帳不是轉帳列，不動轉帳的 amount 與唯一索引。
//   - 終局拒絕（ErrPermanent）進 RELEASED（賣家少收，發 MoneyAnomaly 交後台）；可重試拒絕（平台餘額不足）只在這把鍵的第一次呼叫被拒絕才換新鍵並回錯誤由 River 重試，不計期限。
//   - 結果不明回錯誤，同鍵重送到 ResendWindow，之後以 FindTransfer 依嘗試 ID 查回：找到就收下；查無滿 NotFoundAfter 就是證明上一次不存在，換新鍵重送（錢不能留在平台）；
//     查回連續失敗進 ANOMALY。
//   - 轉帳不是 SUCCEEDED、付款沒有 charge、provider 回的外部 ID 或金額與列不符進 ANOMALY（補掃不再重試、發 MoneyAnomaly；job 完成）。
//   - provider 已轉帳、列的 topup_status 卻已不是 SENDING：記 error log 並發 MoneyAnomaly，不靜默（同一個轉帳 ID 的重複寫入除外）。
//
// topup_status 不是 SENDING 的退款什麼都不做。final 為 true 時暫時性失敗只記警示 log 並回 nil，留給週期掃描。呼叫方不能在 transaction 內呼叫。
func (s *Service) TopUp(ctx context.Context, refundID uuid.UUID, final bool) error {
	if _, ok := db.TxFrom(ctx); ok {
		return errInTransaction
	}
	err := s.topUp(ctx, refundID)
	if err != nil && final && !errors.Is(err, ErrPermanent) {
		s.d.Logger.WarnContext(ctx, "payment：補差額轉帳用完重試，留給週期掃描", "refund_id", refundID.String(), "err", err)
		return nil
	}
	return err
}

func (s *Service) topUp(ctx context.Context, refundID uuid.UUID) error {
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
	if ref.TopupStatus != string(model.TopupSending) {
		return nil
	}
	tr, err := q.SucceededTransferByOrder(ctx, ref.PaymentOrderID)
	if errors.Is(err, pgx.ErrNoRows) {
		return s.anomalyTopup(ctx, ref, "no_succeeded_transfer", "")
	}
	if err != nil {
		return err
	}
	if tr.ExternalTransferID == nil {
		return s.anomalyTopup(ctx, ref, "transfer_without_external_id", "")
	}
	pay, order, err := paymentOfOrder(ctx, q, ref.PaymentOrderID)
	if err != nil {
		return err
	}
	if pay.ExternalChargeID == nil {
		return s.anomalyTopup(ctx, ref, "payment_without_charge", "")
	}
	now := s.d.Now().UTC()
	if !inResendWindow(ref.TopupCalledAt, now) {
		return s.lookupTopup(ctx, ref, tr, pay, order, now)
	}
	called, err := s.markTopupCalled(ctx, refundID, now)
	if err != nil || called == nil {
		return err
	}
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	snap, err := s.d.Provider.CreateTransfer(callCtx, TransferInput{
		Amount: called.Amount, SourceChargeID: *pay.ExternalChargeID, DestinationAccountID: tr.DestinationAccountID,
		TransferGroup: TransferGroupOf(pay.ID), IdempotencyKey: called.TopupIdempotencyKey.String(),
	})
	switch s.classify(ctx, "create_topup_transfer", err) {
	case callRefused:
		s.d.Logger.WarnContext(ctx, "payment：provider 永久拒絕補差額轉帳", "refund_id", refundID.String(), "order_no", order.OrderNo, "err", err)
		return s.releaseTopup(ctx, refundID, ref.PaymentOrderID, failedRefused)
	case callRetryable:
		if !firstCall(ref.TopupCalledAt, called.TopupCalledAt, now) {
			s.d.Logger.WarnContext(ctx, "payment：重送補差額轉帳被拒絕，但這把鍵之前的呼叫結果不明，不換鍵", "refund_id", refundID.String(), "order_no", order.OrderNo, "err", err)
			return err
		}
		s.d.Logger.WarnContext(ctx, "payment：provider 拒絕補差額轉帳、沒有執行，換新冪等鍵重試", "refund_id", refundID.String(), "order_no", order.OrderNo, "err", err)
		if rerr := s.rotateTopupKey(ctx, refundID, called.TopupIdempotencyKey, now); rerr != nil {
			return errors.Join(err, rerr)
		}
		return err
	case callUnknown:
		return err
	case callSucceeded:
	}
	return s.acceptTopup(ctx, *called, tr, snap)
}

func (s *Service) lookupTopup(ctx context.Context, ref store.PaymentRefund, tr store.PaymentTransfer, pay store.PaymentPayment, order store.PaymentPaymentOrder, now time.Time) error {
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	snap, found, err := s.d.Provider.FindTransfer(callCtx, TransferGroupOf(pay.ID), ref.TopupIdempotencyKey.String(), lookupLowerBound(ref.TopupCalledAt, ref.CreatedAt))
	if err != nil {
		return s.topupLookupFailed(ctx, ref, err)
	}
	if err := s.clearTopupLookupFailures(ctx, ref); err != nil {
		return err
	}
	if found {
		return s.acceptTopup(ctx, ref, tr, snap)
	}
	if now.Sub(*ref.TopupCalledAt) < NotFoundAfter {
		return fmt.Errorf("%w：退款 %s 的補差額", errNotFoundYet, ref.ID)
	}
	s.d.Logger.WarnContext(ctx, "payment：查回證明 provider 沒有這次補差額轉帳，換新冪等鍵重送", "refund_id", ref.ID.String(), "order_no", order.OrderNo)
	if err := s.rotateTopupKey(ctx, ref.ID, ref.TopupIdempotencyKey, now); err != nil {
		return err
	}
	return fmt.Errorf("%w：退款 %s", errTopupRekeyed, ref.ID)
}

func (s *Service) acceptTopup(ctx context.Context, ref store.PaymentRefund, tr store.PaymentTransfer, snap TransferSnapshot) error {
	if snap.ExternalID == "" || snap.Amount != ref.Amount || (tr.ExternalTransferID != nil && snap.ExternalID == *tr.ExternalTransferID) {
		return s.anomalyTopup(ctx, ref, "reply_mismatch", snap.ExternalID)
	}
	wctx, cancelWrite := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancelWrite()
	orphan := false
	err := s.d.DB.InTx(wctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		if _, _, err := lockOrder(ctx, q, ref.PaymentOrderID); err != nil {
			return err
		}
		if _, err := q.LockRefund(ctx, ref.ID); err != nil {
			return err
		}
		_, err := q.ApplyRefundTopup(ctx, store.ApplyRefundTopupParams{ID: ref.ID, TopupExternalID: &snap.ExternalID, Now: s.d.Now().UTC()})
		if err == nil {
			return nil
		}
		if errors.Is(err, pgx.ErrNoRows) {
			cur, rerr := q.RefundByID(ctx, ref.ID)
			if rerr != nil {
				return rerr
			}
			if cur.TopupStatus == string(model.TopupDone) && cur.TopupExternalID != nil && *cur.TopupExternalID == snap.ExternalID {
				return nil
			}
			orphan = true
			return s.publishOrderAnomaly(ctx, model.AnomalyTopup, ref.ID, ref.PaymentOrderID, fmt.Sprintf("topped_up_but_topup_%s:%s", cur.TopupStatus, snap.ExternalID))
		}
		return err
	})
	if err == nil && orphan {
		s.d.Logger.ErrorContext(ctx, "payment：provider 已補差額轉帳，列的補差額狀態卻已不在 SENDING，待人工核對",
			"refund_id", ref.ID.String(), "payment_order_id", ref.PaymentOrderID.String(), "topup_external_id", snap.ExternalID)
	}
	return err
}

func (s *Service) markTopupCalled(ctx context.Context, refundID uuid.UUID, now time.Time) (*store.PaymentRefund, error) {
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancel()
	var row *store.PaymentRefund
	err := s.d.DB.InTx(wctx, func(ctx context.Context) error {
		r, err := store.New(s.d.DB.Conn(ctx)).MarkRefundTopupCalled(ctx, store.MarkRefundTopupCalledParams{ID: refundID, Now: now})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		row = &r
		return err
	})
	return row, err
}

func (s *Service) rotateTopupKey(ctx context.Context, refundID, oldKey uuid.UUID, now time.Time) error {
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancel()
	return s.d.DB.InTx(wctx, func(ctx context.Context) error {
		_, err := store.New(s.d.DB.Conn(ctx)).RotateRefundTopupKey(ctx, store.RotateRefundTopupKeyParams{ID: refundID, OldKey: oldKey, Now: now})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	})
}

func (s *Service) topupLookupFailed(ctx context.Context, ref store.PaymentRefund, cause error) error {
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancel()
	var n int32
	err := s.d.DB.InTx(wctx, func(ctx context.Context) error {
		var err error
		n, err = store.New(s.d.DB.Conn(ctx)).BumpRefundTopupLookupFailures(ctx, store.BumpRefundTopupLookupFailuresParams{ID: ref.ID, Now: s.d.Now().UTC()})
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
		return s.anomalyTopup(ctx, ref, "lookup_failed", "")
	}
	return fmt.Errorf("charge: 退款 %s 的補差額查回失敗：%w", ref.ID, cause)
}

func (s *Service) clearTopupLookupFailures(ctx context.Context, ref store.PaymentRefund) error {
	if ref.TopupLookupFailures == 0 {
		return nil
	}
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancel()
	return s.d.DB.InTx(wctx, func(ctx context.Context) error {
		return store.New(s.d.DB.Conn(ctx)).ClearRefundTopupLookupFailures(ctx, ref.ID)
	})
}

func (s *Service) anomalyTopup(ctx context.Context, ref store.PaymentRefund, reason, detail string) error {
	s.d.Logger.ErrorContext(ctx, "payment：退款的補差額轉帳進 ANOMALY，待人工核對 provider", "refund_id", ref.ID.String(), "payment_order_id", ref.PaymentOrderID.String(), "reason", reason, "external_id", detail)
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancel()
	return s.d.DB.InTx(wctx, func(ctx context.Context) error {
		r, err := store.New(s.d.DB.Conn(ctx)).AnomalyRefundTopup(ctx, store.AnomalyRefundTopupParams{ID: ref.ID, Reason: &reason, Now: s.d.Now().UTC()})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		return s.publishOrderAnomaly(ctx, model.AnomalyTopup, r.ID, r.PaymentOrderID, withDetail(reason, detail))
	})
}

func (s *Service) releaseTopup(ctx context.Context, refundID, paymentOrderID uuid.UUID, reason string) error {
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancel()
	return s.d.DB.InTx(wctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		if _, _, err := lockOrder(ctx, q, paymentOrderID); err != nil {
			return err
		}
		if _, err := q.LockRefund(ctx, refundID); err != nil {
			return err
		}
		r, err := q.ReleaseRefundTopup(ctx, store.ReleaseRefundTopupParams{ID: refundID, Reason: &reason, FromStatuses: []string{string(model.TopupSending)}, Now: s.d.Now().UTC()})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		return s.publishOrderAnomaly(ctx, model.AnomalyTopup, r.ID, r.PaymentOrderID, "topup_released:"+reason)
	})
}
