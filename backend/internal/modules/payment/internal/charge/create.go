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
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

// createResult 是一次建立嘗試之後的列；clientSecret 只在這次拿到 PaymentIntent 而且列處於 AWAITING_PAYMENT 時有值（請求內建立回給買家用，不存）。
type createResult struct {
	row          store.PaymentPayment
	clientSecret string
}

// Create 是補做 job（payment.charge_create）的內容，也是請求內建立的本體：列還在建立中時，呼叫 provider 之前先記 create_called_at（獨立提交），
// 帶列上的冪等鍵建立 PaymentIntent（請求內與 job 用同一把鍵，所以同時或先後建立都得到同一個 PaymentIntent），成功在新的 transaction 寫外部 ID 與狀態。
// 已不在建立中的列（別的呼叫已建好）什麼都不做。呼叫過、沒有外部 ID 超過 UnresolvedDeadline 的列進終局，不再重送。provider 永久拒絕進終局 CREATE_FAILED，
// 不回錯誤（job 完成）；暫時性失敗回錯誤由 River 重試，final 為 true 時只記警示 log 並回 nil，留給週期掃描。
func (s *Service) Create(ctx context.Context, paymentID uuid.UUID, final bool) error {
	_, err := s.create(ctx, paymentID, callTimeout)
	if err != nil && final {
		s.d.Logger.WarnContext(ctx, "payment：補做建立 PaymentIntent 用完重試，留給週期掃描", "payment_id", paymentID.String(), "err", err)
		return nil
	}
	return err
}

func (s *Service) create(ctx context.Context, paymentID uuid.UUID, timeout time.Duration) (createResult, error) {
	if err := s.requireProvider(); err != nil {
		return createResult{}, err
	}
	p := s.d.Provider
	row, err := store.New(s.d.DB.Conn(ctx)).PaymentByID(ctx, paymentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return createResult{}, nil
	}
	if err != nil {
		return createResult{}, err
	}
	if row.Status != string(model.PaymentCreating) {
		return createResult{row: row}, nil
	}
	now := s.d.Now().UTC()
	if row.Provider != p.Name() {
		// 設定換了 provider：舊 provider 的呼叫從沒送出、或已超過期限的列才改成目前的（新嘗試、換新鍵、重新起算）；呼叫過又還在期限內的列不動，等舊 provider 的結果。
		rebound, err := s.rebind(ctx, paymentID, p.Name(), now)
		if err != nil {
			return createResult{}, err
		}
		if rebound.Status != string(model.PaymentCreating) || rebound.Provider != p.Name() {
			return createResult{row: rebound}, nil
		}
		row = rebound
	}
	if row.CreateCalledAt != nil && now.Sub(*row.CreateCalledAt) > UnresolvedDeadline {
		return s.failCreate(ctx, paymentID, failedUnresolved)
	}
	called, attempt, err := s.markCalled(ctx, paymentID, now)
	if err != nil {
		return createResult{}, err
	}
	if called.Status != string(model.PaymentCreating) {
		return createResult{row: called}, nil
	}
	startedAt := s.d.Now().UTC()
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	snap, err := p.CreatePayment(callCtx, CreateInput{
		PaymentID: called.ID, OrderNo: called.OrderNo, StoreID: called.StoreID, Amount: called.Amount, Description: called.Description,
		TransferGroup: called.OrderNo, IdempotencyKey: called.CreateIdempotencyKey.String(),
	})
	if errors.Is(err, ErrPermanent) {
		s.d.Logger.WarnContext(ctx, "payment：provider 永久拒絕建立 PaymentIntent", "payment_id", paymentID.String(), "order_no", called.OrderNo, "err", err)
		return s.failCreate(ctx, paymentID, failedRefused)
	}
	if err != nil {
		return createResult{}, err
	}
	// 寫結果的 ctx 在呼叫之後才建：呼叫佔用的時間不算進寫入的上限，job 的 ctx 逾時被取消後仍要能寫入。
	wctx, cancelWrite := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancelWrite()
	var out createResult
	err = s.d.DB.InTx(wctx, func(ctx context.Context) error {
		res, err := s.applySnapshot(ctx, paymentID, attempt.ID, p.Name(), snap, startedAt)
		out.row = res.row
		if res.written && res.row.Status == string(model.PaymentAwaitingPayment) {
			out.clientSecret = snap.ClientSecret
		}
		return err
	})
	return out, err
}

// rebind 在獨立的 transaction 把建立中、沒有外部 ID 的列改成目前的 provider（舊嘗試記為被取代、新嘗試用新的冪等鍵）；條件不成立（列已不是那樣，或舊 provider 呼叫過又還在期限內）時回目前的列。
func (s *Service) rebind(ctx context.Context, paymentID uuid.UUID, provider string, now time.Time) (store.PaymentPayment, error) {
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancel()
	var row store.PaymentPayment
	err := s.d.DB.InTx(wctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		r, err := q.RebindCreatingPayment(ctx, store.RebindCreatingPaymentParams{ID: paymentID, Provider: provider, Now: now, StaleBefore: now.Add(-UnresolvedDeadline)})
		if errors.Is(err, pgx.ErrNoRows) {
			r, err = q.PaymentByID(ctx, paymentID)
			row = r
			return err
		}
		if err != nil {
			return err
		}
		row = r
		if _, err := q.SupersedeEffectiveAttempt(ctx, store.SupersedeEffectiveAttemptParams{PaymentID: paymentID, Now: &now}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		return q.EnsureEffectiveAttempt(ctx, store.EnsureEffectiveAttemptParams{ID: s.d.NewID(), PaymentID: paymentID, Now: now})
	})
	return row, err
}

// markCalled 在獨立的 transaction 記下「呼叫過」的時刻（呼叫 provider 之前提交），付款列與有效嘗試各記一份，回傳列與這次呼叫對應的嘗試。
// 列已不在建立中（別的呼叫已寫入結果）時回目前的列，嘗試是零值。
func (s *Service) markCalled(ctx context.Context, paymentID uuid.UUID, now time.Time) (store.PaymentPayment, store.PaymentPaymentAttempt, error) {
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancel()
	var row store.PaymentPayment
	var attempt store.PaymentPaymentAttempt
	err := s.d.DB.InTx(wctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		r, err := q.MarkPaymentCreateCalled(ctx, store.MarkPaymentCreateCalledParams{ID: paymentID, Now: now})
		if errors.Is(err, pgx.ErrNoRows) {
			// 列已不在建立中（別的呼叫已寫入結果）：回目前的列。
			row, err = q.PaymentByID(ctx, paymentID)
			return err
		}
		if err != nil {
			return err
		}
		row = r
		attempt, err = q.MarkAttemptCalled(ctx, store.MarkAttemptCalledParams{PaymentID: paymentID, IdempotencyKey: r.CreateIdempotencyKey, Now: now})
		if err != nil {
			return fmt.Errorf("charge: 付款 %s 建立中卻沒有對應的有效嘗試：%w", paymentID, err)
		}
		return nil
	})
	return row, attempt, err
}

// failCreate 讓還在建立中、沒有外部 ID 的列進終局 CREATE_FAILED；列已不是那樣（別的呼叫已建好）時回目前的列。
func (s *Service) failCreate(ctx context.Context, paymentID uuid.UUID, reason string) (createResult, error) {
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancel()
	var row store.PaymentPayment
	err := s.d.DB.InTx(wctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		r, err := q.FailPaymentCreate(ctx, store.FailPaymentCreateParams{ID: paymentID, FailureReason: &reason, Now: s.d.Now().UTC()})
		if errors.Is(err, pgx.ErrNoRows) {
			r, err = q.PaymentByID(ctx, paymentID)
		}
		row = r
		return err
	})
	return createResult{row: row}, err
}

// applyResult 是 applySnapshot 的結果：written 為 true 表示快照寫進了付款列；judged 為 true 表示這份快照已經被判斷過、不需要再讀一次
// （寫進了列或嘗試列，或與列的終局狀態衝突而保留列）。兩者都不是表示讀取過期（較新的讀取已寫入）或沒有可對的嘗試。
type applyResult struct {
	row     store.PaymentPayment
	written bool
	judged  bool
}

// applySnapshot 在呼叫方的 transaction 內鎖付款列、驗證快照、把狀態記進這次讀的那一個嘗試（paymentID 底下的 attemptID），再依嘗試是不是有效的決定付款列：
//   - 有效的嘗試：推導狀態並寫付款列，狀態與寫入之前不同時發布 PaymentChanged（同一個狀態的重複讀取不重發）；
//   - 被取代的（舊的）嘗試：只更新嘗試列，付款列不動（舊 ID 不會寫回剛重設的列）；舊 PaymentIntent 晚到成功的處置見 applyLate。
//
// provider 是回這份快照的 provider：與嘗試的 provider 不同就丟棄（別的設定寫的嘗試）。快照與列對不起來（外部 ID 換了、金額不符、狀態不是有效值）回 ErrAnomaly 或不認得狀態的錯誤，不寫入；
// 入帳的快照缺 charge 或入帳時間回 errIncompleteSuccess（暫時性）；衝突（列已入帳、快照不是入帳）不寫入並記警示。
// 只在這次讀取的開始時間不早於嘗試列已存的 fetched_at 時寫入：較早開始的讀取被擋下時 written 與 judged 都是 false。
func (s *Service) applySnapshot(ctx context.Context, paymentID, attemptID uuid.UUID, provider string, snap Snapshot, startedAt time.Time) (applyResult, error) {
	q := store.New(s.d.DB.Conn(ctx))
	locked, err := q.LockPayment(ctx, paymentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return applyResult{}, nil
	}
	if err != nil {
		return applyResult{}, err
	}
	att, err := q.AttemptByID(ctx, store.AttemptByIDParams{ID: attemptID, PaymentID: paymentID})
	if errors.Is(err, pgx.ErrNoRows) {
		return applyResult{row: locked}, nil
	}
	if err != nil {
		return applyResult{}, err
	}
	if att.Provider != provider {
		s.d.Logger.WarnContext(ctx, "payment：快照來自另一個 provider 設定，丟棄", "payment_id", paymentID.String(), "attempt_provider", att.Provider, "snapshot_provider", provider)
		return applyResult{row: locked}, nil
	}
	if snap.ExternalID == "" || (att.ExternalPaymentID != nil && *att.ExternalPaymentID != snap.ExternalID) {
		return applyResult{row: locked}, fmt.Errorf("%w：外部 ID %q 與列上的不同", ErrAnomaly, snap.ExternalID)
	}
	if snap.Amount != locked.Amount {
		return applyResult{row: locked}, fmt.Errorf("%w：provider 的金額 %d 與列上的 %d 不同", ErrAnomaly, snap.Amount, locked.Amount)
	}
	if !snap.State.Valid() {
		return applyResult{row: locked}, errUnknownState
	}
	if err := checkSucceededSnapshot(snap); err != nil {
		return applyResult{row: locked}, err
	}
	stored, err := q.ApplyAttemptSnapshot(ctx, store.ApplyAttemptSnapshotParams{ID: att.ID, ExternalPaymentID: snap.ExternalID, State: attemptState(snap.State), FetchedAt: &startedAt})
	if errors.Is(err, pgx.ErrNoRows) {
		return applyResult{row: locked}, nil
	}
	if err != nil {
		return applyResult{}, err
	}
	if stored.SupersededAt != nil {
		return s.applyLate(ctx, q, locked, stored, snap, startedAt)
	}
	return s.writeRow(ctx, q, locked, stored, snap, startedAt)
}

// writeRow 把有效嘗試的快照寫進付款列：由列目前的狀態與快照推導下一個狀態，狀態改變才發布 PaymentChanged。
func (s *Service) writeRow(ctx context.Context, q *store.Queries, locked store.PaymentPayment, att store.PaymentPaymentAttempt, snap Snapshot, startedAt time.Time) (applyResult, error) {
	next, keep, err := Next(model.PaymentStatus(locked.Status), snap)
	if err != nil {
		return applyResult{row: locked}, err
	}
	if keep {
		s.d.Logger.WarnContext(ctx, "payment：provider 的快照與付款列的終局狀態衝突，不覆蓋", "payment_id", locked.ID.String(), "order_no", locked.OrderNo, "status", locked.Status, "snapshot_state", string(snap.State))
		return applyResult{row: locked, judged: true}, nil
	}
	params := store.ApplyPaymentIntentSnapshotParams{
		ID: locked.ID, Status: string(next), Provider: att.Provider, ExternalPaymentID: &snap.ExternalID, CreateIdempotencyKey: att.IdempotencyKey, CreateCalledAt: att.CalledAt,
		FetchedAt: &startedAt, Now: s.d.Now().UTC(),
	}
	if next == model.PaymentSucceeded {
		paidAt := snap.PaidAt.UTC()
		params.ExternalChargeID, params.PaidAt = &snap.ChargeID, &paidAt
	} else if snap.LastError != "" {
		params.LastError = &snap.LastError
	}
	written, err := q.ApplyPaymentIntentSnapshot(ctx, params)
	if err != nil {
		return applyResult{}, err
	}
	if next != model.PaymentStatus(locked.Status) {
		err = events.Publish(ctx, s.d.Events, model.PaymentChanged{
			OccurredAt: written.UpdatedAt, PaymentID: written.ID, OrderNo: written.OrderNo, StoreID: written.StoreID, BuyerID: written.BuyerID, Amount: written.Amount,
			Provider: written.Provider, Status: next, Previous: model.PaymentStatus(locked.Status), PaidAt: written.PaidAt, LastError: derefString(written.LastError),
		})
		if err != nil {
			return applyResult{}, err
		}
	}
	return applyResult{row: written, written: true, judged: true}, nil
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
