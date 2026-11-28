package charge

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

const (
	requestKeyMax  = 128
	failedProvider = "provider_failed"
)

var errUnknownRefundState = errors.New("charge: provider 回了不認得的退款狀態")

func reversalFor(amount, available int64) int64 {
	if available <= 0 {
		return 0
	}
	return min(amount, available)
}

func refundTarget(state RefundState) (model.RefundStatus, bool) {
	switch state {
	case RefundPending:
		return model.RefundRefunding, true
	case RefundSucceeded:
		return model.RefundSucceeded, true
	case RefundFailed:
		return model.RefundFailed, true
	case RefundUnknown:
		return "", false
	}
	return "", false
}

func refundSettled(cur store.PaymentRefund, externalID string, target model.RefundStatus) bool {
	if cur.ExternalRefundID == nil || *cur.ExternalRefundID != externalID {
		return false
	}
	if cur.Status == string(target) {
		return true
	}
	terminal := cur.Status == string(model.RefundSucceeded) || cur.Status == string(model.RefundFailed)
	return terminal && target == model.RefundRefunding
}

func providerFailureReason(code string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return failedProvider
	}
	r := failedProvider + ":" + code
	if utf8.RuneCountInString(r) > 200 {
		r = string([]rune(r)[:200])
	}
	return r
}

// CreateRefund 是建立退款的入口（D4；售後與訂單一方的 CreateRefund 呼叫，自動退款帶 Auto）：在一個 transaction 內鎖付款列（LockPaymentByOrderNo，FOR UPDATE）、
// 以 (付款, request_key) 找既有的退款（同一把鍵回同一筆，內容不同回 FailedPrecondition）、確認付款已入帳、加總非失敗的退款（進行中與 ANOMALY 都算）守累計上限，
// 寫 PENDING 的退款列並排退款 job；provider 在 transaction 外由 job 呼叫（規則 5）。退款先退買家，轉帳的任何狀態都可以退款（D11、ADR D69），向賣家收回依付款的轉帳決定：
//   - 沒有有效轉帳（或轉帳都已 FAILED），或轉帳還沒承諾金額（PENDING 且 called_at 為空）：不必收回（reversal_status NONE），轉帳第一次送出時的金額已扣掉這筆退款。
//   - 轉帳金額已承諾（PENDING 已呼叫、SUCCEEDED 或 ANOMALY）：預留 reversal_amount = min(退款額, 轉帳金額 − 已撤回 + 成功的補差額轉帳的（退款額 − 已撤回） − 其他預留)；大於 0 寫 WAITING，等 provider 接受退款且轉帳 SUCCEEDED 才開始收回（依目標分成一或多次 provider 撤回，見 startReversals）；
//     是 0（剩餘額度已被預留或撤光，款項在平台手上）寫 NONE。
//
// 付款不存在回 NotFound；付款未入帳、超過累計上限回 FailedPrecondition；金額不是正數或鍵不合回 InvalidInput。呼叫方不能在 transaction 內呼叫。
func (s *Service) CreateRefund(ctx context.Context, in model.RefundRequest) (model.Refund, error) {
	if err := s.requireProvider(); err != nil {
		return model.Refund{}, err
	}
	if _, ok := db.TxFrom(ctx); ok {
		return model.Refund{}, errInTransaction
	}
	orderNo, key := strings.TrimSpace(in.OrderNo), strings.TrimSpace(in.RequestKey)
	if n := utf8.RuneCountInString(orderNo); n < 1 || n > 64 {
		return model.Refund{}, apperr.InvalidInput("訂單編號長度不合")
	}
	if n := utf8.RuneCountInString(key); n < 1 || n > requestKeyMax {
		return model.Refund{}, apperr.InvalidInput("退款冪等鍵長度不合")
	}
	if in.Amount <= 0 {
		return model.Refund{}, apperr.InvalidInput("退款金額必須大於零")
	}
	var out store.PaymentRefund
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		pay, err := q.LockPaymentByOrderNo(ctx, orderNo)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("找不到這筆付款")
		}
		if err != nil {
			return err
		}
		order, err := q.LockPaymentOrderByOrderNo(ctx, orderNo)
		if err != nil {
			return err
		}
		existing, err := q.RefundByRequestKey(ctx, store.RefundByRequestKeyParams{PaymentOrderID: order.ID, RequestKey: key})
		if err == nil {
			if existing.Amount != in.Amount || existing.Auto != in.Auto {
				return apperr.FailedPrecondition("這把退款冪等鍵已用於內容不同的退款")
			}
			out = existing
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if pay.Status != string(model.PaymentSucceeded) {
			return apperr.FailedPrecondition("付款尚未入帳，不能退款")
		}
		taken, err := q.NonFailedRefundTotal(ctx, order.ID)
		if err != nil {
			return err
		}
		if in.Amount > order.Amount-taken {
			return apperr.FailedPrecondition("退款總額超過這張訂單的應收金額")
		}
		if !s.refundable(pay) {
			return apperr.FailedPrecondition("付款目前不能退款（沒有外部付款或屬於別的付款服務）")
		}
		reversal, err := s.reservableReversal(ctx, q, order, in.Amount)
		if err != nil {
			return err
		}
		reversalStatus := model.ReversalNone
		if reversal > 0 {
			reversalStatus = model.ReversalWaiting
		}
		row, err := q.InsertRefund(ctx, store.InsertRefundParams{
			ID: s.d.NewID(), PaymentOrderID: order.ID, RequestKey: key, Amount: in.Amount, Auto: in.Auto, ReversalStatus: string(reversalStatus), ReversalAmount: reversal, Now: s.d.Now().UTC(),
		})
		if err != nil {
			return err
		}
		out = row
		return enqueueRefund(ctx, s.d.Jobs, row.ID)
	})
	if err != nil {
		return model.Refund{}, err
	}
	view := toRefund(out)
	view.OrderNo = orderNo
	return view, nil
}

func (s *Service) refundable(pay store.PaymentPayment) bool {
	return pay.ExternalPaymentID != nil && pay.Status == string(model.PaymentSucceeded) && pay.Provider == s.d.Provider.Name()
}

func (s *Service) reservableReversal(ctx context.Context, q *store.Queries, order store.PaymentPaymentOrder, amount int64) (int64, error) {
	active, err := q.ActiveTransferByOrder(ctx, order.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	tr, err := q.LockTransfer(ctx, active.ID)
	if err != nil {
		return 0, err
	}
	switch model.TransferStatus(tr.Status) {
	case model.TransferFailed:
		return 0, nil
	case model.TransferPending:
		if tr.CalledAt == nil {
			return 0, nil
		}
	case model.TransferSucceeded, model.TransferAnomaly:
	}
	reserved, err := q.PendingReversalTotal(ctx, order.ID)
	if err != nil {
		return 0, err
	}
	topups, err := q.DoneTopupCapacity(ctx, order.ID)
	if err != nil {
		return 0, err
	}
	return reversalFor(amount, tr.Amount-tr.ReversedAmount+topups-reserved), nil
}

func toRefund(r store.PaymentRefund) model.Refund {
	out := model.Refund{
		ID: r.ID, PaymentOrderID: r.PaymentOrderID, RequestKey: r.RequestKey, Amount: r.Amount, Auto: r.Auto,
		Status: model.RefundStatus(r.Status), ReversalAmount: r.ReversalAmount, ReversalStatus: model.ReversalStatus(r.ReversalStatus), TopupStatus: model.TopupStatus(r.TopupStatus), CreatedAt: r.CreatedAt,
	}
	if r.FailureReason != nil {
		out.FailureReason = *r.FailureReason
	}
	return out
}

// Refund 是退款 job（payment.refund_create）的內容：退款是 PENDING 時依結果三分收斂（ADR D68，同 Transfer）。
//   - 呼叫前先記 refund_called_at（這把鍵的第一次呼叫，獨立提交），帶列上的冪等鍵與付款的外部 ID 呼叫 provider，成功依快照進 REFUNDING、SUCCEEDED 或 FAILED；provider 一接受（REFUNDING 或 SUCCEEDED）就讓等待中的撤回開始（轉帳已成立時排撤回 job）。
//   - 終局拒絕（ErrPermanent）進 FAILED（不佔額度，job 完成）；可重試拒絕（只在這把鍵的第一次呼叫被拒絕）換新鍵並回錯誤由 River 重試，不計期限。
//   - 結果不明回錯誤，同鍵重送到 ResendWindow，之後只以 FindRefund 查回：找到就收下，查無滿 NotFoundAfter 才進 FAILED，未滿回錯誤稍後再查；查回連續失敗進 ANOMALY。
//   - 付款沒有外部 ID、不是已入帳或屬於別的 provider、provider 回的外部 ID 或金額與列不符進 ANOMALY（佔額度、補掃不再重試、發 MoneyAnomaly；job 完成）。
//
// 已不是 PENDING 的退款什麼都不做。final 為 true 時暫時性失敗只記警示 log 並回 nil，留給週期掃描。呼叫方不能在 transaction 內呼叫。
func (s *Service) Refund(ctx context.Context, refundID uuid.UUID, final bool) error {
	if _, ok := db.TxFrom(ctx); ok {
		return errInTransaction
	}
	err := s.refund(ctx, refundID)
	if err != nil && final && !errors.Is(err, ErrPermanent) {
		s.d.Logger.WarnContext(ctx, "payment：退款用完重試，留給週期掃描", "refund_id", refundID.String(), "err", err)
		return nil
	}
	return err
}

func (s *Service) refund(ctx context.Context, refundID uuid.UUID) error {
	if err := s.requireProvider(); err != nil {
		return err
	}
	q := store.New(s.d.DB.Conn(ctx))
	row, err := q.RefundByID(ctx, refundID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if row.Status != string(model.RefundPending) {
		return nil
	}
	pay, order, err := paymentOfOrder(ctx, q, row.PaymentOrderID)
	if err != nil {
		return err
	}
	if !s.refundable(pay) {
		return s.anomalyRefund(ctx, row, "payment_not_refundable", "")
	}
	now := s.d.Now().UTC()
	if !inResendWindow(row.RefundCalledAt, now) {
		return s.lookupRefund(ctx, row, pay, order, now)
	}
	called, err := s.markRefundCalled(ctx, refundID, now)
	if err != nil || called == nil {
		return err
	}
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	snap, err := s.d.Provider.CreateRefund(callCtx, RefundInput{ExternalPaymentID: *pay.ExternalPaymentID, Amount: called.Amount, IdempotencyKey: called.IdempotencyKey.String()})
	switch s.classify(ctx, "create_refund", err) {
	case callRefused:
		s.d.Logger.WarnContext(ctx, "payment：provider 永久拒絕建立退款", "refund_id", refundID.String(), "order_no", order.OrderNo, "err", err)
		return s.failRefund(ctx, refundID, failedRefused)
	case callRetryable:
		if !firstCall(row.RefundCalledAt, called.RefundCalledAt, now) {
			s.d.Logger.WarnContext(ctx, "payment：重送退款被拒絕，但這把鍵之前的呼叫結果不明，不換鍵", "refund_id", refundID.String(), "order_no", order.OrderNo, "err", err)
			return err
		}
		s.d.Logger.WarnContext(ctx, "payment：provider 拒絕建立退款、沒有執行，換新冪等鍵重試", "refund_id", refundID.String(), "order_no", order.OrderNo, "err", err)
		if rerr := s.rotateRefundKey(ctx, refundID, called.IdempotencyKey, now); rerr != nil {
			return errors.Join(err, rerr)
		}
		return err
	case callUnknown:
		return err
	case callSucceeded:
	}
	return s.acceptRefund(ctx, *called, snap)
}

func (s *Service) lookupRefund(ctx context.Context, row store.PaymentRefund, pay store.PaymentPayment, order store.PaymentPaymentOrder, now time.Time) error {
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	snap, found, err := s.d.Provider.FindRefund(callCtx, *pay.ExternalPaymentID, row.IdempotencyKey.String(), lookupLowerBound(row.RefundCalledAt, row.CreatedAt))
	if err != nil {
		return s.refundLookupFailed(ctx, row, err)
	}
	if err := s.clearRefundLookupFailures(ctx, row); err != nil {
		return err
	}
	if found {
		return s.acceptRefund(ctx, row, snap)
	}
	if now.Sub(*row.RefundCalledAt) < NotFoundAfter {
		return fmt.Errorf("%w：退款 %s", errNotFoundYet, row.ID)
	}
	s.d.Logger.WarnContext(ctx, "payment：查回證明 provider 沒有這筆退款，進 FAILED", "refund_id", row.ID.String(), "order_no", order.OrderNo)
	return s.failRefund(ctx, row.ID, failedNotFound)
}

func (s *Service) acceptRefund(ctx context.Context, row store.PaymentRefund, snap RefundSnapshot) error {
	if snap.ExternalID == "" || snap.Amount != row.Amount || (row.ExternalRefundID != nil && *row.ExternalRefundID != snap.ExternalID) {
		return s.anomalyRefund(ctx, row, "reply_mismatch", snap.ExternalID)
	}
	target, ok := refundTarget(snap.State)
	if !ok {
		return fmt.Errorf("%w：退款 %s", errUnknownRefundState, row.ID)
	}
	var reason *string
	if target == model.RefundFailed {
		r := providerFailureReason(snap.FailureReason)
		reason = &r
	}
	wctx, cancelWrite := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancelWrite()
	orphan := false
	err := s.d.DB.InTx(wctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		if _, _, err := lockOrder(ctx, q, row.PaymentOrderID); err != nil {
			return err
		}
		transferSucceeded, err := s.lockActiveTransfer(ctx, q, row.PaymentOrderID)
		if err != nil {
			return err
		}
		locked, err := q.LockRefund(ctx, row.ID)
		if err != nil {
			return err
		}
		written, err := q.ApplyRefundSnapshot(ctx, store.ApplyRefundSnapshotParams{
			ID: row.ID, Status: string(target), ExternalRefundID: &snap.ExternalID, FailureReason: reason, Now: s.d.Now().UTC(),
		})
		if errors.Is(err, pgx.ErrNoRows) {
			if refundSettled(locked, snap.ExternalID, target) {
				return nil
			}
			orphan = true
			return s.publishOrderAnomaly(ctx, model.AnomalyRefund, row.ID, row.PaymentOrderID, withDetail("refunded_but_refund_"+locked.Status, snap.ExternalID))
		}
		if err != nil {
			return err
		}
		if target != model.RefundFailed {
			if transferSucceeded {
				if err := s.startReversals(ctx, q, row.PaymentOrderID); err != nil {
					return err
				}
			}
			if target == model.RefundRefunding {
				return nil
			}
			if err := s.wakeTransfer(ctx, q, row.ID, row.PaymentOrderID); err != nil {
				return err
			}
			return s.publishRefundResult(ctx, q, written, model.RefundStatus(locked.Status))
		}
		if err := s.afterRefundFailed(ctx, written); err != nil {
			return err
		}
		if transferSucceeded {
			if err := s.startTopups(ctx, q, row.PaymentOrderID); err != nil {
				return err
			}
		}
		if err := s.wakeTransfer(ctx, q, row.ID, row.PaymentOrderID); err != nil {
			return err
		}
		return s.publishRefundResult(ctx, q, written, model.RefundStatus(locked.Status))
	})
	if err == nil && orphan {
		s.d.Logger.ErrorContext(ctx, "payment：provider 已建立退款，退款列卻不在可接收的狀態，待人工核對",
			"refund_id", row.ID.String(), "payment_order_id", row.PaymentOrderID.String(), "refund_external_id", snap.ExternalID)
	}
	return err
}

func (s *Service) lockActiveTransfer(ctx context.Context, q *store.Queries, paymentOrderID uuid.UUID) (succeeded bool, err error) {
	active, err := q.ActiveTransferByOrder(ctx, paymentOrderID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	tr, err := q.LockTransfer(ctx, active.ID)
	if err != nil {
		return false, err
	}
	return tr.Status == string(model.TransferSucceeded), nil
}

func (s *Service) afterRefundFailed(ctx context.Context, r store.PaymentRefund) error {
	if r.ReversalRecovered > 0 {
		return s.publishOrderAnomaly(ctx, model.AnomalyRefund, r.ID, r.PaymentOrderID, "refund_failed_after_reversal")
	}
	return nil
}

func (s *Service) wakeTransfer(ctx context.Context, q *store.Queries, refundID, paymentOrderID uuid.UUID) error {
	t, err := q.ActiveTransferByOrder(ctx, paymentOrderID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if t.Status != string(model.TransferPending) || t.CalledAt != nil {
		return nil
	}
	return enqueueTransferWake(ctx, s.d.Jobs, t.ID, refundID)
}

func (s *Service) publishRefundResult(ctx context.Context, q *store.Queries, r store.PaymentRefund, previous model.RefundStatus) error {
	pay, order, err := paymentOfOrder(ctx, q, r.PaymentOrderID)
	if err != nil {
		return err
	}
	if err := events.Publish(ctx, s.d.Events, model.RefundChanged{
		OccurredAt: r.UpdatedAt, RefundID: r.ID, PaymentID: pay.ID, OrderNo: order.OrderNo, StoreID: order.StoreID, BuyerID: pay.BuyerID, Amount: r.Amount,
		Auto: r.Auto, RequestKey: r.RequestKey, Status: model.RefundStatus(r.Status), Previous: previous, FailureReason: derefString(r.FailureReason),
	}); err != nil {
		return err
	}
	if r.Auto && r.Status == string(model.RefundSucceeded) {
		return events.Publish(ctx, s.d.Events, model.AutoRefundCompleted{
			OccurredAt: r.UpdatedAt, RefundID: r.ID, PaymentID: pay.ID, OrderNo: order.OrderNo, StoreID: order.StoreID, BuyerID: pay.BuyerID, Amount: r.Amount,
		})
	}
	return nil
}

// SyncRefund 是退款同步 job（payment.refund_sync）的內容：REFUNDING 的退款在 transaction 外重讀 provider 的退款（不信事件內容），在 transaction 內依狀態進 SUCCEEDED 或 FAILED
// （還在處理就維持 REFUNDING），並把讀取開始前收到的事件標為已處理。讀不到這筆退款（永久失敗）或與列不符進 ANOMALY。已不是 REFUNDING 的退款只標記事件。
// 暫時性失敗與不認得的狀態回錯誤由 River 重試，事件保持未處理（掃描會重排）。付款屬於別的 provider 的退款不讀也不標記（那個 provider 的退款 ID 與目前的無關，同付款同步丟棄別的 provider 快照）。
// again 為 true 表示讀取期間又收到事件：呼叫方稍後再跑一輪（同付款同步）。呼叫方不能在 transaction 內呼叫。
func (s *Service) SyncRefund(ctx context.Context, refundID uuid.UUID) (again bool, err error) {
	if s.d.Provider == nil {
		return false, nil
	}
	if _, ok := db.TxFrom(ctx); ok {
		return false, errInTransaction
	}
	q := store.New(s.d.DB.Conn(ctx))
	row, err := q.RefundByID(ctx, refundID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if row.ExternalRefundID == nil {
		return false, nil
	}
	pay, _, err := paymentOfOrder(ctx, q, row.PaymentOrderID)
	if err != nil {
		return false, err
	}
	if pay.Provider != s.d.Provider.Name() {
		return false, nil
	}
	external, p := *row.ExternalRefundID, s.d.Provider
	startedAt := s.d.Now().UTC()
	if row.Status == string(model.RefundRefunding) {
		callCtx, cancel := context.WithTimeout(ctx, callTimeout)
		snap, err := p.GetRefund(callCtx, external)
		cancel()
		switch {
		case errors.Is(err, ErrPermanent):
			if aerr := s.anomalyRefund(ctx, row, "refund_missing_at_provider", external); aerr != nil {
				return false, aerr
			}
		case err != nil:
			return false, err
		default:
			if err := s.acceptRefund(ctx, row, snap); err != nil {
				return false, err
			}
		}
	}
	if err := s.markEventsProcessed(ctx, p.Name(), external, startedAt); err != nil {
		return false, err
	}
	return s.d.Inbox.RoutePending(ctx, p.Name(), external)
}

func (s *Service) markRefundCalled(ctx context.Context, refundID uuid.UUID, now time.Time) (*store.PaymentRefund, error) {
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancel()
	var row *store.PaymentRefund
	err := s.d.DB.InTx(wctx, func(ctx context.Context) error {
		r, err := store.New(s.d.DB.Conn(ctx)).MarkRefundCalled(ctx, store.MarkRefundCalledParams{ID: refundID, Now: now})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		row = &r
		return err
	})
	return row, err
}

func (s *Service) rotateRefundKey(ctx context.Context, refundID, oldKey uuid.UUID, now time.Time) error {
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancel()
	return s.d.DB.InTx(wctx, func(ctx context.Context) error {
		_, err := store.New(s.d.DB.Conn(ctx)).RotateRefundKey(ctx, store.RotateRefundKeyParams{ID: refundID, OldKey: oldKey, Now: now})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	})
}

func (s *Service) refundLookupFailed(ctx context.Context, row store.PaymentRefund, cause error) error {
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancel()
	var n int32
	err := s.d.DB.InTx(wctx, func(ctx context.Context) error {
		var err error
		n, err = store.New(s.d.DB.Conn(ctx)).BumpRefundLookupFailures(ctx, store.BumpRefundLookupFailuresParams{ID: row.ID, Now: s.d.Now().UTC()})
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
		return s.anomalyRefund(ctx, row, "lookup_failed", "")
	}
	return fmt.Errorf("charge: 退款 %s 查回失敗：%w", row.ID, cause)
}

func (s *Service) clearRefundLookupFailures(ctx context.Context, row store.PaymentRefund) error {
	if row.RefundLookupFailures == 0 {
		return nil
	}
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancel()
	return s.d.DB.InTx(wctx, func(ctx context.Context) error {
		return store.New(s.d.DB.Conn(ctx)).ClearRefundLookupFailures(ctx, row.ID)
	})
}

func (s *Service) anomalyRefund(ctx context.Context, row store.PaymentRefund, reason, detail string) error {
	s.d.Logger.ErrorContext(ctx, "payment：退款進 ANOMALY，待人工核對 provider", "refund_id", row.ID.String(), "payment_order_id", row.PaymentOrderID.String(), "reason", reason, "external_id", detail)
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancel()
	return s.d.DB.InTx(wctx, func(ctx context.Context) error {
		r, err := store.New(s.d.DB.Conn(ctx)).AnomalyRefund(ctx, store.AnomalyRefundParams{ID: row.ID, Reason: &reason, Now: s.d.Now().UTC()})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		return s.publishOrderAnomaly(ctx, model.AnomalyRefund, r.ID, r.PaymentOrderID, withDetail(reason, detail))
	})
}

func (s *Service) failRefund(ctx context.Context, refundID uuid.UUID, reason string) error {
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancel()
	return s.d.DB.InTx(wctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		prev, err := q.RefundByID(ctx, refundID)
		if err != nil {
			return err
		}
		if _, _, err := lockOrder(ctx, q, prev.PaymentOrderID); err != nil {
			return err
		}
		transferSucceeded, err := s.lockActiveTransfer(ctx, q, prev.PaymentOrderID)
		if err != nil {
			return err
		}
		if _, err := q.LockRefund(ctx, refundID); err != nil {
			return err
		}
		r, err := q.FailRefundCall(ctx, store.FailRefundCallParams{ID: refundID, FailureReason: &reason, Now: s.d.Now().UTC()})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := s.afterRefundFailed(ctx, r); err != nil {
			return err
		}
		if transferSucceeded {
			if err := s.startTopups(ctx, q, prev.PaymentOrderID); err != nil {
				return err
			}
		}
		if err := s.wakeTransfer(ctx, q, refundID, prev.PaymentOrderID); err != nil {
			return err
		}
		return s.publishRefundResult(ctx, q, r, model.RefundStatus(prev.Status))
	})
}
