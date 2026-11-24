package charge

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// failedResolved 是人工結案成失敗的原因碼（failure_reason）。
const failedResolved = "admin_resolved"

// evidenceMax 與 provider 外部 ID 的欄位上限相同（CHECK 的 255）。
const evidenceMax = 255

// ResolveAnomaly 是後台對 ANOMALY 的搬錢物件的人工結案：管理員核對 provider 之後，把它結案成 AnomalySucceeded（provider 確實執行了，evidence 是 provider 物件 ID，必填）
// 或 AnomalyFailed（確認 provider 沒有執行，evidence 選填）。領域變更與一筆 adminaudit 在同一個 transaction，沒有紀錄的結案不成立（Connect 端點在後台入口接上）。
//   - 付款：成功是把有效嘗試連上 evidence 這個 PaymentIntent、暫記 AWAITING_PAYMENT 並排同步 job（真正的狀態由重讀推導）；失敗進 CREATE_FAILED（買家可以再付，換新鍵）。
//   - 轉帳：成功記 evidence 為外部轉帳 ID 並進 SUCCEEDED，累計已撤回額與等待中的撤回預留都以 provider 核實的轉帳為準（金額減 provider 上已撤回的額度，不用列上的數字，
//     預留依建立順序重新分配、沒有額度的釋出，之後讓可開始的撤回開始）；失敗進 FAILED（讓出一筆付款一筆有效轉帳的位置、釋出等待中的撤回預留）。
//   - 退款（ANOMALY 是退款呼叫的異常）：成功表示 provider 確實建立了退款，evidence 是退款 ID，暫記 REFUNDING 並排退款同步 job（真正的狀態由重讀推導，接受後才讓等待中的撤回開始）；失敗進 FAILED（不佔額度、釋出等待中的撤回預留）。
//   - 補差額轉帳（AnomalyTopup，id 是退款 ID，補差額狀態是 ANOMALY）：成功表示 provider 確實補了差額，evidence 是轉帳 ID，進 DONE；失敗確認沒有補，進 RELEASED（人工另行處理）。
//   - 撤回（AnomalyReversal，id 是退款 ID，撤回狀態是 ANOMALY）：成功表示撤回確實成立，evidence 是撤回 ID，同一個 transaction 累加轉帳的 reversed_amount、進 REVERSED（退款已 FAILED 時發 MoneyAnomaly）；失敗釋出預留（RELEASED）。
//
// 結案成成功之前，先在 transaction 外向 provider 查回證據（依列上的冪等鍵／嘗試 ID 查，不信管理員貼的 ID；退款與撤回分別用 FindRefund 與 FindReversal）：查到的物件 ID、金額必須與證據和列相符，不符回 FailedPrecondition、
// 列不動；transaction 內鎖列之後再確認列上的鍵沒變。付款的證據在目前設定的 provider 查得到才行（列屬於已無法查回的 provider 時不能結案成成功）。
// 不在 ANOMALY 的物件回 FailedPrecondition，找不到回 NotFound。鎖順序：付款列、轉帳列、退款列。呼叫方不能在 transaction 內呼叫。
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
	if _, ok := db.TxFrom(ctx); ok {
		return errInTransaction
	}
	var proof evidenceProof
	if outcome == model.AnomalySucceeded {
		var err error
		if proof, err = s.verifyEvidence(ctx, kind, id, evidence); err != nil {
			return err
		}
	}
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		var reason string
		var err error
		switch kind {
		case model.AnomalyPayment:
			reason, err = s.resolvePayment(ctx, q, id, outcome, evidence, proof)
		case model.AnomalyTransfer:
			reason, err = s.resolveTransfer(ctx, q, id, outcome, evidence, proof)
		case model.AnomalyRefund:
			reason, err = s.resolveRefund(ctx, q, id, outcome, evidence, proof)
		case model.AnomalyReversal:
			reason, err = s.resolveReversal(ctx, q, id, outcome, evidence, proof)
		case model.AnomalyTopup:
			reason, err = s.resolveTopup(ctx, q, id, outcome, evidence, proof)
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

var (
	errEvidenceMismatch = apperr.FailedPrecondition("provider 上查到的物件與證據或這筆紀錄對不起來，不能結案成成功")
	errEvidenceChanged  = apperr.FailedPrecondition("核對 provider 之後這筆紀錄又變動了，請重新核對")
)

// evidenceProof 是核對過證據時用的列上識別：key 是查回用的冪等鍵（嘗試 ID），parent 是退款核對時付款的外部 ID、撤回核對時轉帳的外部 ID；transaction 內鎖列後要確認沒變。
// 轉帳核對另外帶 provider 核實的轉帳金額 amount 與累計已撤回額 reversed。
type evidenceProof struct {
	key      uuid.UUID
	parent   string
	amount   int64
	reversed int64
}

// verifyEvidence 在 transaction 外向 provider 查回管理員給的證據：以列上的嘗試 ID 查，找到的物件 ID 與金額都要和證據、列相符。
func (s *Service) verifyEvidence(ctx context.Context, kind model.AnomalyKind, id uuid.UUID, evidence string) (evidenceProof, error) {
	q := store.New(s.d.DB.Conn(ctx))
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	switch kind {
	case model.AnomalyPayment:
		row, err := q.PaymentByID(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return evidenceProof{}, apperr.NotFound("找不到這筆付款")
		}
		if err != nil {
			return evidenceProof{}, err
		}
		if row.Status != string(model.PaymentAnomaly) || row.ExternalPaymentID != nil {
			return evidenceProof{}, errNotAnomaly
		}
		if row.Provider != s.d.Provider.Name() {
			return evidenceProof{}, apperr.FailedPrecondition("這筆付款屬於已無法查回的 provider，不能結案成成功")
		}
		snap, found, err := s.d.Provider.FindPayment(callCtx, row.CreateIdempotencyKey.String())
		if err != nil {
			return evidenceProof{}, fmt.Errorf("charge: 核對付款 %s 的證據失敗：%w", id, err)
		}
		if !found || snap.ExternalID != evidence || snap.Amount != row.Amount {
			return evidenceProof{}, errEvidenceMismatch
		}
		return evidenceProof{key: row.CreateIdempotencyKey}, nil
	case model.AnomalyTransfer:
		t, err := q.TransferByID(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return evidenceProof{}, apperr.NotFound("找不到這筆轉帳")
		}
		if err != nil {
			return evidenceProof{}, err
		}
		if t.Status != string(model.TransferAnomaly) {
			return evidenceProof{}, errNotAnomaly
		}
		pay, err := q.PaymentByID(ctx, t.PaymentID)
		if err != nil {
			return evidenceProof{}, err
		}
		snap, found, err := s.d.Provider.FindTransfer(callCtx, pay.OrderNo, t.IdempotencyKey.String())
		if err != nil {
			return evidenceProof{}, fmt.Errorf("charge: 核對轉帳 %s 的證據失敗：%w", id, err)
		}
		if !found || snap.ExternalID != evidence || snap.Amount != t.Amount {
			return evidenceProof{}, errEvidenceMismatch
		}
		return evidenceProof{key: t.IdempotencyKey}, nil
	case model.AnomalyRefund:
		r, err := q.RefundByID(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return evidenceProof{}, apperr.NotFound("找不到這筆退款")
		}
		if err != nil {
			return evidenceProof{}, err
		}
		if r.Status != string(model.RefundAnomaly) {
			return evidenceProof{}, errNotAnomaly
		}
		pay, err := q.PaymentByID(ctx, r.PaymentID)
		if err != nil {
			return evidenceProof{}, err
		}
		if pay.ExternalPaymentID == nil || pay.Provider != s.d.Provider.Name() {
			return evidenceProof{}, apperr.FailedPrecondition("付款沒有可查回的 provider 付款，不能結案成成功")
		}
		snap, found, err := s.d.Provider.FindRefund(callCtx, *pay.ExternalPaymentID, r.IdempotencyKey.String())
		if err != nil {
			return evidenceProof{}, fmt.Errorf("charge: 核對退款 %s 的證據失敗：%w", id, err)
		}
		if !found || snap.ExternalID != evidence || snap.Amount != r.Amount {
			return evidenceProof{}, errEvidenceMismatch
		}
		return evidenceProof{key: r.IdempotencyKey, parent: *pay.ExternalPaymentID}, nil
	case model.AnomalyReversal:
		r, err := q.RefundByID(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return evidenceProof{}, apperr.NotFound("找不到這筆退款")
		}
		if err != nil {
			return evidenceProof{}, err
		}
		if r.ReversalStatus != string(model.ReversalAnomaly) {
			return evidenceProof{}, errNotAnomaly
		}
		tr, err := q.SucceededTransferByPayment(ctx, r.PaymentID)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && tr.ExternalTransferID == nil) {
			return evidenceProof{}, apperr.FailedPrecondition("付款沒有已成立的轉帳，不能記撤回")
		}
		if err != nil {
			return evidenceProof{}, err
		}
		snap, found, err := s.d.Provider.FindReversal(callCtx, *tr.ExternalTransferID, r.ReversalIdempotencyKey.String())
		if err != nil {
			return evidenceProof{}, fmt.Errorf("charge: 核對退款 %s 的撤回證據失敗：%w", id, err)
		}
		if !found || snap.ExternalID != evidence || snap.Amount != r.ReversalAmount {
			return evidenceProof{}, errEvidenceMismatch
		}
		return evidenceProof{key: r.ReversalIdempotencyKey, parent: *tr.ExternalTransferID}, nil
	case model.AnomalyTopup:
		r, err := q.RefundByID(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return evidenceProof{}, apperr.NotFound("找不到這筆退款")
		}
		if err != nil {
			return evidenceProof{}, err
		}
		if r.TopupStatus != string(model.TopupAnomaly) {
			return evidenceProof{}, errNotAnomaly
		}
		pay, err := q.PaymentByID(ctx, r.PaymentID)
		if err != nil {
			return evidenceProof{}, err
		}
		snap, found, err := s.d.Provider.FindTransfer(callCtx, pay.OrderNo, r.TopupIdempotencyKey.String())
		if err != nil {
			return evidenceProof{}, fmt.Errorf("charge: 核對退款 %s 的補差額證據失敗：%w", id, err)
		}
		if !found || snap.ExternalID != evidence || snap.Amount != r.Amount {
			return evidenceProof{}, errEvidenceMismatch
		}
		return evidenceProof{key: r.TopupIdempotencyKey}, nil
	}
	return evidenceProof{}, apperr.InvalidInput("不認得的物件種類")
}

// conflictOrErr 把 provider 外部 ID 的唯一鍵衝突轉成 AlreadyExists（證據指到別的列）。
func conflictOrErr(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return apperr.AlreadyExists("這個 provider 物件 ID 已對應到別的紀錄")
	}
	return err
}

func (s *Service) resolvePayment(ctx context.Context, q *store.Queries, id uuid.UUID, outcome model.AnomalyOutcome, evidence string, proof evidenceProof) (string, error) {
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
	if row.CreateIdempotencyKey != proof.key {
		return "", errEvidenceChanged
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

func (s *Service) resolveTransfer(ctx context.Context, q *store.Queries, id uuid.UUID, outcome model.AnomalyOutcome, evidence string, proof evidenceProof) (string, error) {
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
		if _, err = q.ResolveTransferFailed(ctx, store.ResolveTransferFailedParams{ID: id, FailureReason: &failed, Now: now}); err != nil {
			return reason, err
		}
		if err := s.releaseWaitingReversals(ctx, q, t.PaymentID, reversalTransferFailed); err != nil {
			return reason, err
		}
		return reason, s.settleTopupsWithoutTransfer(ctx, q, t.PaymentID)
	}
	if locked.IdempotencyKey != proof.key {
		return "", errEvidenceChanged
	}
	resolved, err := q.ResolveTransferSucceeded(ctx, store.ResolveTransferSucceededParams{ID: id, ExternalTransferID: &evidence, VerifiedReversed: proof.reversed, Now: now})
	if err != nil {
		return reason, conflictOrErr(err)
	}
	if err := s.reassignWaitingReversals(ctx, q, resolved, proof); err != nil {
		return reason, err
	}
	return reason, s.startTopups(ctx, q, resolved.PaymentID)
}

// reassignWaitingReversals 在轉帳人工結案成成功之後，以 provider 核實的轉帳為準重新分配等待中的撤回預留：剩餘可撤額度 = 核實的金額 − provider 上已撤回的額度 − 其他進行中的預留，
// 依退款建立順序給每筆 min(退款額, 剩餘)，分到 0 的釋出（RELEASED，no_reversible_amount）；分配完讓 provider 已接受退款的撤回開始。呼叫方已鎖付款列與轉帳列。
func (s *Service) reassignWaitingReversals(ctx context.Context, q *store.Queries, tr store.PaymentTransfer, proof evidenceProof) error {
	waiting, err := q.WaitingReversals(ctx, tr.PaymentID)
	if err != nil {
		return err
	}
	if len(waiting) == 0 {
		return nil
	}
	reserved, err := q.PendingReversalTotal(ctx, tr.PaymentID)
	if err != nil {
		return err
	}
	var waitingTotal int64
	for _, r := range waiting {
		waitingTotal += r.ReversalAmount
	}
	remaining := proof.amount - proof.reversed - (reserved - waitingTotal)
	now := s.d.Now().UTC()
	for _, r := range waiting {
		amount := reversalFor(r.Amount, remaining)
		remaining -= amount
		if amount == 0 {
			nothing := reversalNothingLeft
			if _, err := q.ReleaseRefundReversal(ctx, store.ReleaseRefundReversalParams{ID: r.ID, Reason: &nothing, FromStatuses: []string{string(model.ReversalWaiting)}, Now: now}); err != nil {
				return err
			}
			continue
		}
		if amount != r.ReversalAmount {
			if _, err := q.SetRefundReversalAmount(ctx, store.SetRefundReversalAmountParams{ID: r.ID, ReversalAmount: amount, Now: now}); err != nil {
				return err
			}
		}
	}
	return s.startReversals(ctx, q, tr.PaymentID)
}

// resolveRefund 結案退款呼叫的異常（見 ResolveAnomaly）：鎖順序付款列、轉帳列、退款列。失敗時等待中的撤回預留由寫 FAILED 的同一個 statement 釋出。
func (s *Service) resolveRefund(ctx context.Context, q *store.Queries, id uuid.UUID, outcome model.AnomalyOutcome, evidence string, proof evidenceProof) (string, error) {
	r, err := q.RefundByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", apperr.NotFound("找不到這筆退款")
	}
	if err != nil {
		return "", err
	}
	pay, err := q.LockPayment(ctx, r.PaymentID)
	if err != nil {
		return "", err
	}
	transferSucceeded, err := s.lockActiveTransfer(ctx, q, r.PaymentID)
	if err != nil {
		return "", err
	}
	locked, err := q.LockRefund(ctx, id)
	if err != nil {
		return "", err
	}
	if locked.Status != string(model.RefundAnomaly) {
		return "", errNotAnomaly
	}
	reason := derefString(locked.FailureReason)
	now := s.d.Now().UTC()
	if outcome == model.AnomalyFailed {
		failed := failedResolved
		if _, err = q.ResolveRefundCallFailed(ctx, store.ResolveRefundCallFailedParams{ID: id, FailureReason: &failed, Now: now}); err != nil {
			return reason, err
		}
		if transferSucceeded {
			return reason, s.startTopups(ctx, q, r.PaymentID)
		}
		return reason, nil
	}
	if locked.IdempotencyKey != proof.key || proof.parent == "" || pay.ExternalPaymentID == nil || *pay.ExternalPaymentID != proof.parent {
		return "", errEvidenceChanged
	}
	if _, err := q.ResolveRefundRefunding(ctx, store.ResolveRefundRefundingParams{ID: id, ExternalRefundID: &evidence, Now: now}); err != nil {
		return "", conflictOrErr(err)
	}
	return reason, enqueueRefundSync(ctx, s.d.Jobs, id)
}

// resolveTopup 結案補差額轉帳的異常（見 ResolveAnomaly）：鎖順序付款列、退款列。成功＝provider 確實補了差額（evidence 是轉帳 ID，進 DONE）；失敗＝確認沒有補，不補了（RELEASED，人工另行處理）。
func (s *Service) resolveTopup(ctx context.Context, q *store.Queries, id uuid.UUID, outcome model.AnomalyOutcome, evidence string, proof evidenceProof) (string, error) {
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
	locked, err := q.LockRefund(ctx, id)
	if err != nil {
		return "", err
	}
	if locked.TopupStatus != string(model.TopupAnomaly) {
		return "", errNotAnomaly
	}
	reason := derefString(locked.TopupReason)
	now := s.d.Now().UTC()
	if outcome == model.AnomalyFailed {
		failed := failedResolved
		_, err = q.ResolveRefundTopupReleased(ctx, store.ResolveRefundTopupReleasedParams{ID: id, Reason: &failed, Now: now})
		return reason, err
	}
	if locked.TopupIdempotencyKey != proof.key {
		return "", errEvidenceChanged
	}
	if _, err := q.ResolveRefundTopupDone(ctx, store.ResolveRefundTopupDoneParams{ID: id, TopupExternalID: &evidence, Now: now}); err != nil {
		return "", conflictOrErr(err)
	}
	return reason, nil
}

// resolveReversal 結案撤回的異常（見 ResolveAnomaly）：鎖順序付款列、轉帳列（成功才鎖）、退款列。
func (s *Service) resolveReversal(ctx context.Context, q *store.Queries, id uuid.UUID, outcome model.AnomalyOutcome, evidence string, proof evidenceProof) (string, error) {
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
	if locked.ReversalStatus != string(model.ReversalAnomaly) {
		return "", errNotAnomaly
	}
	reason := derefString(locked.ReversalReason)
	now := s.d.Now().UTC()
	if outcome == model.AnomalyFailed {
		failed := failedResolved
		_, err = q.ResolveRefundReversalReleased(ctx, store.ResolveRefundReversalReleasedParams{ID: id, Reason: &failed, Now: now})
		return reason, err
	}
	if locked.ReversalIdempotencyKey != proof.key || tr.ExternalTransferID == nil || *tr.ExternalTransferID != proof.parent {
		return "", errEvidenceChanged
	}
	if locked.ReversalAmount > tr.Amount-tr.ReversedAmount {
		return "", apperr.FailedPrecondition("撤回額度超過轉帳的剩餘可撤額度")
	}
	done, err := q.ResolveRefundReversed(ctx, store.ResolveRefundReversedParams{ID: id, ReversalExternalID: &evidence, Now: now})
	if err != nil {
		return "", conflictOrErr(err)
	}
	if _, err = q.AddTransferReversal(ctx, store.AddTransferReversalParams{ID: tr.ID, Amount: locked.ReversalAmount, Now: now}); err != nil {
		return "", err
	}
	return reason, s.afterRefundFailed(ctx, done)
}
