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
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

// createResult 是一次建立嘗試之後的列；clientSecret 只在這次拿到 PaymentIntent 而且列處於 AWAITING_PAYMENT 時有值（請求內建立回給買家用，不存）。
type createResult struct {
	row          store.PaymentPayment
	clientSecret string
}

// Create 是補做 job（payment.charge_create）的內容，也是請求內建立的本體：列還在建立中時依結果三分收斂（ADR D68）。
//   - 呼叫 provider 之前先記 create_called_at（這把鍵的第一次呼叫，獨立提交），帶列上的冪等鍵建立 PaymentIntent（請求內與 job 用同一把鍵，所以同時或先後建立都得到同一個
//     PaymentIntent），成功在新的 transaction 寫外部 ID 與狀態。已不在建立中的列（別的呼叫已建好）什麼都不做。
//   - provider 終局拒絕（ErrPermanent）進終局 CREATE_FAILED，不回錯誤（job 完成）；可重試拒絕（ErrRetryableRefusal）換新冪等鍵（舊嘗試被取代、新嘗試另寫一列）並回錯誤由 River 重試，不計期限。
//   - 結果不明回錯誤，補做 job 與掃描以同一把鍵重送到 ResendWindow；之後以 FindPayment 依嘗試 ID 查回：找到就收下，查無滿 NotFoundAfter 才進 CREATE_FAILED，
//     查回連續失敗 LookupFailureLimit 次進 ANOMALY。
//   - provider 回的快照與列對不起來（金額不符、沒有外部 ID）進 ANOMALY（佔住這張訂單、補掃不再重試、發 MoneyAnomaly；job 完成）。
//
// final 為 true 時暫時性失敗只記警示 log 並回 nil，留給週期掃描。呼叫方不能在 transaction 內呼叫。
func (s *Service) Create(ctx context.Context, paymentID uuid.UUID, final bool) error {
	if _, ok := db.TxFrom(ctx); ok {
		return errInTransaction
	}
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
		// 設定換了 provider：只有舊 provider 的呼叫從沒送出（沒有東西可能已建在它那邊）才改成目前的（新嘗試、換新鍵、重新起算）。
		// 呼叫過的列：同一時間只設定一個 provider，舊的已無法查回，不能證明沒有建出 PaymentIntent，所以進 ANOMALY 由人工核對，不依時間換鍵（D68）。
		rebound, err := s.rebind(ctx, paymentID, p.Name(), now)
		if err != nil {
			return createResult{}, err
		}
		if rebound.Status != string(model.PaymentCreating) {
			return createResult{row: rebound}, nil
		}
		if rebound.Provider != p.Name() {
			return s.anomalyPayment(ctx, paymentID, "provider_changed_after_call", rebound.Provider)
		}
		row = rebound
	}
	if !inResendWindow(row.CreateCalledAt, now) {
		return s.lookupCreate(ctx, row, now, timeout)
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
	switch classifyCall(err) {
	case callRefused:
		s.d.Logger.WarnContext(ctx, "payment：provider 永久拒絕建立 PaymentIntent", "payment_id", paymentID.String(), "order_no", called.OrderNo, "err", err)
		return s.failCreate(ctx, paymentID, failedRefused)
	case callRetryable:
		if !firstCall(row.CreateCalledAt, called.CreateCalledAt, now) {
			s.d.Logger.WarnContext(ctx, "payment：重送建立 PaymentIntent 被拒絕，但這把鍵之前的呼叫結果不明，不換鍵", "payment_id", paymentID.String(), "order_no", called.OrderNo, "err", err)
			return createResult{}, err
		}
		s.d.Logger.WarnContext(ctx, "payment：provider 拒絕建立 PaymentIntent、沒有執行，換新冪等鍵重試", "payment_id", paymentID.String(), "order_no", called.OrderNo, "err", err)
		if rerr := s.rotateCreateKey(ctx, paymentID, called.CreateIdempotencyKey, now); rerr != nil {
			return createResult{}, errors.Join(err, rerr)
		}
		return createResult{}, err
	case callUnknown:
		return createResult{}, err
	case callSucceeded:
	}
	return s.acceptCreated(ctx, paymentID, attempt.ID, p.Name(), snap, startedAt)
}

// acceptCreated 把建立的結果（呼叫的回應或查回找到的）寫進列。寫結果的 ctx 在呼叫之後才建：呼叫佔用的時間不算進寫入的上限，job 的 ctx 逾時被取消後仍要能寫入。
// 快照與列對不起來（ErrAnomaly）讓列進 ANOMALY；入帳缺 charge 或不認得的狀態是暫時性的，回錯誤由 job 重試。
func (s *Service) acceptCreated(ctx context.Context, paymentID, attemptID uuid.UUID, provider string, snap Snapshot, startedAt time.Time) (createResult, error) {
	wctx, cancelWrite := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancelWrite()
	var out createResult
	err := s.d.DB.InTx(wctx, func(ctx context.Context) error {
		res, err := s.applySnapshot(ctx, paymentID, attemptID, provider, snap, startedAt)
		out.row = res.row
		if res.written && res.row.Status == string(model.PaymentAwaitingPayment) {
			out.clientSecret = snap.ClientSecret
		}
		return err
	})
	if errors.Is(err, ErrAnomaly) {
		return s.anomalyPayment(ctx, paymentID, "reply_mismatch", snap.ExternalID)
	}
	return out, err
}

// lookupCreate 是過了 ResendWindow 的建立：不再呼叫 provider，只依嘗試 ID 查回。
func (s *Service) lookupCreate(ctx context.Context, row store.PaymentPayment, now time.Time, timeout time.Duration) (createResult, error) {
	q := store.New(s.d.DB.Conn(ctx))
	attempt, err := q.EffectiveAttempt(ctx, row.ID)
	if err != nil {
		return createResult{}, fmt.Errorf("charge: 付款 %s 建立中卻沒有有效的嘗試：%w", row.ID, err)
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	startedAt := s.d.Now().UTC()
	snap, found, err := s.d.Provider.FindPayment(callCtx, row.CreateIdempotencyKey.String())
	if err != nil {
		return s.createLookupFailed(ctx, row, err)
	}
	if row.CreateLookupFailures > 0 {
		if err := s.inShortTx(ctx, func(ctx context.Context) error {
			return store.New(s.d.DB.Conn(ctx)).ClearCreateLookupFailures(ctx, row.ID)
		}); err != nil {
			return createResult{}, err
		}
	}
	if found {
		return s.acceptCreated(ctx, row.ID, attempt.ID, s.d.Provider.Name(), snap, startedAt)
	}
	if now.Sub(*row.CreateCalledAt) < NotFoundAfter {
		return createResult{}, fmt.Errorf("%w：付款 %s", errNotFoundYet, row.ID)
	}
	s.d.Logger.WarnContext(ctx, "payment：查回證明 provider 沒有這個 PaymentIntent，進 CREATE_FAILED", "payment_id", row.ID.String(), "order_no", row.OrderNo)
	return s.failCreate(ctx, row.ID, failedNotFound)
}

// inShortTx 在獨立的 transaction 內做一次小寫入（呼叫之後 ctx 可能已逾時，所以用自己的上限）。
func (s *Service) inShortTx(ctx context.Context, fn func(context.Context) error) error {
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancel()
	return s.d.DB.InTx(wctx, fn)
}

// createLookupFailed 記一次查回失敗；連續失敗到上限進 ANOMALY，否則回錯誤由 job 重試。
func (s *Service) createLookupFailed(ctx context.Context, row store.PaymentPayment, cause error) (createResult, error) {
	var n int32
	err := s.inShortTx(ctx, func(ctx context.Context) error {
		var err error
		n, err = store.New(s.d.DB.Conn(ctx)).BumpCreateLookupFailures(ctx, store.BumpCreateLookupFailuresParams{ID: row.ID, Now: s.d.Now().UTC()})
		if errors.Is(err, pgx.ErrNoRows) {
			n = 0
			return nil
		}
		return err
	})
	if err != nil {
		return createResult{}, errors.Join(cause, err)
	}
	if n >= LookupFailureLimit {
		return s.anomalyPayment(ctx, row.ID, "lookup_failed", "")
	}
	return createResult{}, fmt.Errorf("charge: 付款 %s 查回失敗：%w", row.ID, cause)
}

// rotateCreateKey 在 provider 已知拒絕之後換新冪等鍵：付款列換鍵、有效嘗試記被取代、新嘗試另寫一列；已被別的呼叫換過或列已不在建立中時什麼都不做。
func (s *Service) rotateCreateKey(ctx context.Context, paymentID, oldKey uuid.UUID, now time.Time) error {
	return s.inShortTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		if _, err := q.RotateCreateKey(ctx, store.RotateCreateKeyParams{ID: paymentID, OldKey: oldKey, Now: now}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
		if _, err := q.SupersedeEffectiveAttempt(ctx, store.SupersedeEffectiveAttemptParams{PaymentID: paymentID, Now: &now}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		return q.EnsureEffectiveAttempt(ctx, store.EnsureEffectiveAttemptParams{ID: s.d.NewID(), PaymentID: paymentID, Now: now})
	})
}

// anomalyPayment 讓建立中、沒有外部 ID 的列進 ANOMALY 並在同一個 transaction 發 MoneyAnomaly；列已不是那樣（別的呼叫已建好）時回目前的列。
// 列上只存原因碼 reason；provider 的外部 ID（detail）只進 log 與事件（failure_reason 上限 200 字）。
func (s *Service) anomalyPayment(ctx context.Context, paymentID uuid.UUID, reason, detail string) (createResult, error) {
	s.d.Logger.ErrorContext(ctx, "payment：建立付款進 ANOMALY，待人工核對 provider", "payment_id", paymentID.String(), "reason", reason, "external_id", detail)
	var row store.PaymentPayment
	err := s.inShortTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		r, err := q.AnomalyPayment(ctx, store.AnomalyPaymentParams{ID: paymentID, Reason: &reason, Now: s.d.Now().UTC()})
		if errors.Is(err, pgx.ErrNoRows) {
			row, err = q.PaymentByID(ctx, paymentID)
			return err
		}
		if err != nil {
			return err
		}
		row = r
		return s.publishAnomaly(ctx, model.AnomalyPayment, r.ID, r.ID, withDetail(reason, detail))
	})
	return createResult{row: row}, err
}

// rebind 在獨立的 transaction 把建立中、沒有外部 ID、從沒呼叫過的列改成目前的 provider（舊嘗試記為被取代、新嘗試用新的冪等鍵）；條件不成立（列已不是那樣，或已呼叫過）時回目前的列。
func (s *Service) rebind(ctx context.Context, paymentID uuid.UUID, provider string, now time.Time) (store.PaymentPayment, error) {
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancel()
	var row store.PaymentPayment
	err := s.d.DB.InTx(wctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		r, err := q.RebindCreatingPayment(ctx, store.RebindCreatingPaymentParams{ID: paymentID, Provider: provider, Now: now})
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
		if locked.Status == string(model.PaymentAnomaly) {
			if err := s.publishAnomaly(ctx, model.AnomalyPayment, locked.ID, locked.ID, withDetail("snapshot_on_anomaly_"+string(snap.State), snap.ExternalID)); err != nil {
				return applyResult{}, err
			}
		}
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
