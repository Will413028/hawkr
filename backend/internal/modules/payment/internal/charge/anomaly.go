package charge

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// failedResolved 是人工結案成失敗的原因碼（failure_reason）。
const failedResolved = "admin_resolved"

// evidenceMax 與 provider 外部 ID 的欄位上限相同（CHECK 的 255）。
const evidenceMax = 255

// ResolveAnomaly 是後台對 ANOMALY 的搬錢物件的人工結案：管理員核對 provider 之後，把它結案成 AnomalySucceeded（provider 確實執行了，evidence 是 provider 物件 ID，必填）
// 或 AnomalyFailed（確認 provider 沒有執行，evidence 選填）。領域變更與一筆 adminaudit 在同一個 transaction，沒有紀錄的結案不成立（Connect 端點在後台入口接上）。
//   - 付款：成功是把有效嘗試連上 evidence 這個 PaymentIntent、暫記 AWAITING_PAYMENT 並排同步 job（真正的狀態由重讀推導）；失敗進 CREATE_FAILED（買家可以再付，換新鍵）。
//   - 轉帳：成功記 evidence 為外部轉帳 ID 並進 SUCCEEDED；失敗進 FAILED（讓出一筆付款一筆有效轉帳的位置）。
//   - 退款（撤回階段的 ANOMALY）：成功表示撤回確實成立，evidence 是撤回 ID，同一個 transaction 累加轉帳的 reversed_amount 並進 REVERSED；失敗進 FAILED（不佔額度）。
//
// 不在 ANOMALY 的物件回 FailedPrecondition，找不到回 NotFound。鎖順序：付款列、轉帳列、退款列。
func (s *Service) ResolveAnomaly(ctx context.Context, actor adminaudit.Actor, kind model.AnomalyKind, id uuid.UUID, outcome model.AnomalyOutcome, evidence string) error {
	if err := s.requireProvider(); err != nil {
		return err
	}
	evidence = strings.TrimSpace(evidence)
	if len(evidence) > evidenceMax {
		return apperr.InvalidInput("provider 物件 ID 過長")
	}
	if outcome != model.AnomalySucceeded && outcome != model.AnomalyFailed {
		return apperr.InvalidInput("結案結果不合")
	}
	if outcome == model.AnomalySucceeded && evidence == "" {
		return apperr.InvalidInput("結案成成功需要 provider 物件 ID 作為證據")
	}
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		var reason string
		var err error
		switch kind {
		case model.AnomalyPayment:
			reason, err = s.resolvePayment(ctx, q, id, outcome, evidence)
		case model.AnomalyTransfer:
			reason, err = s.resolveTransfer(ctx, q, id, outcome, evidence)
		case model.AnomalyRefund:
			reason, err = s.resolveRefund(ctx, q, id, outcome, evidence)
		default:
			return apperr.InvalidInput("不認得的物件種類")
		}
		if err != nil {
			return err
		}
		detail := map[string]any{"outcome": string(outcome), "anomaly_reason": reason}
		if evidence != "" {
			detail["evidence_external_id"] = evidence
		}
		return s.d.Audit.Record(ctx, adminaudit.Entry{Actor: actor, TargetType: "payment_" + string(kind), TargetID: id.String(), Detail: detail})
	})
}

var errNotAnomaly = apperr.FailedPrecondition("這筆不在 ANOMALY，不能人工結案")

// conflictOrErr 把 provider 外部 ID 的唯一鍵衝突轉成 AlreadyExists（證據指到別的列）。
func conflictOrErr(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return apperr.AlreadyExists("這個 provider 物件 ID 已對應到別的紀錄")
	}
	return err
}

func (s *Service) resolvePayment(ctx context.Context, q *store.Queries, id uuid.UUID, outcome model.AnomalyOutcome, evidence string) (string, error) {
	row, err := q.LockPayment(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", apperr.NotFound("找不到這筆付款")
	}
	if err != nil {
		return "", err
	}
	if row.Status != string(model.PaymentAnomaly) {
		return "", errNotAnomaly
	}
	reason := derefString(row.FailureReason)
	now := s.d.Now().UTC()
	if outcome == model.AnomalyFailed {
		failed := failedResolved
		_, err = q.ResolvePaymentFailed(ctx, store.ResolvePaymentFailedParams{ID: id, FailureReason: &failed, Now: now})
		return reason, err
	}
	if row.ExternalPaymentID != nil {
		return "", errNotAnomaly
	}
	if _, err := q.LinkEffectiveAttempt(ctx, store.LinkEffectiveAttemptParams{PaymentID: id, ExternalPaymentID: &evidence, Now: now}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", errNotAnomaly
		}
		return "", conflictOrErr(err)
	}
	if _, err := q.ResolvePaymentLinked(ctx, store.ResolvePaymentLinkedParams{ID: id, ExternalPaymentID: &evidence, Now: now}); err != nil {
		return "", conflictOrErr(err)
	}
	return reason, enqueueSync(ctx, s.d.Jobs, id)
}

func (s *Service) resolveTransfer(ctx context.Context, q *store.Queries, id uuid.UUID, outcome model.AnomalyOutcome, evidence string) (string, error) {
	t, err := q.TransferByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", apperr.NotFound("找不到這筆轉帳")
	}
	if err != nil {
		return "", err
	}
	if _, err := q.LockPayment(ctx, t.PaymentID); err != nil {
		return "", err
	}
	locked, err := q.LockTransfer(ctx, id)
	if err != nil {
		return "", err
	}
	if locked.Status != string(model.TransferAnomaly) {
		return "", errNotAnomaly
	}
	reason := derefString(locked.FailureReason)
	now := s.d.Now().UTC()
	if outcome == model.AnomalyFailed {
		failed := failedResolved
		_, err = q.ResolveTransferFailed(ctx, store.ResolveTransferFailedParams{ID: id, FailureReason: &failed, Now: now})
		return reason, err
	}
	_, err = q.ResolveTransferSucceeded(ctx, store.ResolveTransferSucceededParams{ID: id, ExternalTransferID: &evidence, Now: now})
	return reason, conflictOrErr(err)
}

func (s *Service) resolveRefund(ctx context.Context, q *store.Queries, id uuid.UUID, outcome model.AnomalyOutcome, evidence string) (string, error) {
	r, err := q.RefundByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", apperr.NotFound("找不到這筆退款")
	}
	if err != nil {
		return "", err
	}
	if _, err := q.LockPayment(ctx, r.PaymentID); err != nil {
		return "", err
	}
	var tr store.PaymentTransfer
	if outcome == model.AnomalySucceeded {
		found, err := q.SucceededTransferByPayment(ctx, r.PaymentID)
		if errors.Is(err, pgx.ErrNoRows) {
			return "", apperr.FailedPrecondition("付款沒有已成立的轉帳，不能記撤回")
		}
		if err != nil {
			return "", err
		}
		if tr, err = q.LockTransfer(ctx, found.ID); err != nil {
			return "", err
		}
	}
	locked, err := q.LockRefund(ctx, id)
	if err != nil {
		return "", err
	}
	if locked.Status != string(model.RefundAnomaly) || locked.RefundCalledAt != nil || locked.ExternalRefundID != nil {
		return "", errNotAnomaly
	}
	reason := derefString(locked.FailureReason)
	now := s.d.Now().UTC()
	if outcome == model.AnomalyFailed {
		failed := failedResolved
		_, err = q.ResolveRefundFailed(ctx, store.ResolveRefundFailedParams{ID: id, FailureReason: &failed, Now: now})
		return reason, err
	}
	if locked.ReversalAmount > tr.Amount-tr.ReversedAmount {
		return "", apperr.FailedPrecondition("撤回額度超過轉帳的剩餘可撤額度")
	}
	if _, err := q.ResolveRefundReversed(ctx, store.ResolveRefundReversedParams{ID: id, ReversalExternalID: &evidence, Now: now}); err != nil {
		return "", err
	}
	_, err = q.AddTransferReversal(ctx, store.AddTransferReversalParams{ID: tr.ID, Amount: locked.ReversalAmount, Now: now})
	return reason, err
}
