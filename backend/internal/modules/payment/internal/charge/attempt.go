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

var errIncompleteSuccess = errors.New("charge: 入帳的快照還沒有 charge 或入帳時間")

func checkSucceededSnapshot(snap Snapshot) error {
	if snap.State == StateSucceeded && (snap.ChargeID == "" || snap.PaidAt.IsZero()) {
		return errIncompleteSuccess
	}
	return nil
}

func attemptState(s State) string {
	switch s {
	case StateAwaitingPayment:
		return string(model.PaymentAwaitingPayment)
	case StateProcessing:
		return string(model.PaymentProcessing)
	case StateSucceeded:
		return string(model.PaymentSucceeded)
	case StateCanceled:
		return string(model.PaymentCanceled)
	case StateUnknown:
	}
	return ""
}

func attemptClosed(state *string) bool {
	return state != nil && (*state == string(model.PaymentSucceeded) || *state == string(model.PaymentCanceled))
}

type lateOutcome int

const (
	lateRecord lateOutcome = iota
	lateCancel
	lateAdopt
	lateExcess
)

func lateOutcomeOf(row model.PaymentStatus, snap State) lateOutcome {
	switch {
	case snap == StateSucceeded && row == model.PaymentSucceeded:
		return lateExcess
	case snap == StateSucceeded:
		return lateAdopt
	case row == model.PaymentSucceeded && (snap == StateAwaitingPayment || snap == StateProcessing):
		return lateCancel
	}
	return lateRecord
}

func newPaymentParams(id uuid.UUID, buyerID int64, orders []Order, provider string, now time.Time) store.InsertPaymentParams {
	return store.InsertPaymentParams{ID: id, BuyerID: buyerID, Amount: totalOf(orders), Provider: provider, Description: paymentDescription(orders[0]), Now: now}
}

func totalOf(orders []Order) int64 {
	var total int64
	for _, o := range orders {
		total += o.Amount
	}
	return total
}

func (s *Service) applyLate(ctx context.Context, q *store.Queries, locked store.PaymentPayment, att store.PaymentPaymentAttempt, snap Snapshot, startedAt time.Time) (applyResult, error) {
	switch lateOutcomeOf(model.PaymentStatus(locked.Status), snap.State) {
	case lateCancel:
		if err := enqueueCancel(ctx, s.d.Jobs, att.ID); err != nil {
			return applyResult{}, err
		}
	case lateExcess:
		if err := q.MarkAttemptExcessPaid(ctx, store.MarkAttemptExcessPaidParams{ID: att.ID, Now: s.d.Now().UTC()}); err != nil {
			return applyResult{}, err
		}
		s.d.Logger.ErrorContext(ctx, "payment：同一張訂單的兩個 PaymentIntent 都入帳（重複入帳），不自動退款，待訂單一方或後台處理",
			"payment_id", locked.ID.String(), "paid_external_payment_id", derefString(locked.ExternalPaymentID), "excess_external_payment_id", snap.ExternalID)
	case lateAdopt:
		if _, keep, err := Next(model.PaymentStatus(locked.Status), snap); err != nil || keep {
			if err != nil {
				return applyResult{row: locked}, err
			}
			s.d.Logger.WarnContext(ctx, "payment：舊 PaymentIntent 的入帳與付款列的狀態衝突，不採用", "payment_id", locked.ID.String(), "status", locked.Status)
			return applyResult{row: locked, judged: true}, nil
		}
		prev, err := q.SupersedeEffectiveAttempt(ctx, store.SupersedeEffectiveAttemptParams{PaymentID: locked.ID, Now: ptr(s.d.Now().UTC())})
		hadPrev := err == nil
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return applyResult{}, err
		}
		revived, err := q.ReviveAttempt(ctx, att.ID)
		if err != nil {
			return applyResult{}, err
		}
		res, err := s.writeRow(ctx, q, locked, revived, snap, startedAt)
		if err != nil {
			return res, err
		}
		if hadPrev && prev.ExternalPaymentID != nil && !attemptClosed(prev.State) {
			if err := enqueueCancel(ctx, s.d.Jobs, prev.ID); err != nil {
				return applyResult{}, err
			}
		}
		return res, nil
	case lateRecord:
	}
	return applyResult{row: locked, judged: true}, nil
}

func ptr[T any](v T) *T { return &v }

// CancelAttempt 是取消舊 PaymentIntent 的 job（payment.charge_cancel）的內容：只取消已被取代、還沒結案的嘗試（有效的那個不是這裡取消的對象），
// 在 transaction 外呼叫 provider 取消，結果快照照一般路徑寫入（取消與入帳競態時 provider 回入帳，列記成重複入帳）。
func (s *Service) CancelAttempt(ctx context.Context, attemptID uuid.UUID) error {
	if err := s.requireProvider(); err != nil {
		return err
	}
	if _, ok := db.TxFrom(ctx); ok {
		return errInTransaction
	}
	p := s.d.Provider
	att, err := store.New(s.d.DB.Conn(ctx)).AttemptByIDOnly(ctx, attemptID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if att.Provider != p.Name() || att.ExternalPaymentID == nil || att.SupersededAt == nil || attemptClosed(att.State) {
		return nil
	}
	startedAt := s.d.Now().UTC()
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	snap, err := p.CancelPayment(callCtx, *att.ExternalPaymentID)
	if err != nil {
		return err
	}
	wctx, cancelWrite := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancelWrite()
	return s.d.DB.InTx(wctx, func(ctx context.Context) error {
		_, err := s.applySnapshot(ctx, att.PaymentID, att.ID, p.Name(), snap, startedAt)
		return err
	})
}

// ExcessPaid 列出重複入帳的嘗試（後台查詢用）；afterID 是上一頁最後一筆的嘗試 ID。
func (s *Service) ExcessPaid(ctx context.Context, afterID uuid.UUID, limit int32) ([]model.ExcessPaid, error) {
	rows, err := store.New(s.d.DB.Conn(ctx)).ExcessPaidAttempts(ctx, store.ExcessPaidAttemptsParams{AfterID: afterID, Batch: limit})
	if err != nil {
		return nil, err
	}
	out := make([]model.ExcessPaid, 0, len(rows))
	for _, r := range rows {
		if r.ExcessPaidAt == nil || r.ExternalPaymentID == nil {
			return nil, fmt.Errorf("charge: 重複入帳的嘗試 %s 缺少外部 ID 或時刻", r.ID)
		}
		out = append(out, model.ExcessPaid{AttemptID: r.ID, PaymentID: r.PaymentID, Provider: r.Provider, ExternalPaymentID: *r.ExternalPaymentID, ExcessPaidAt: *r.ExcessPaidAt})
	}
	return out, nil
}
