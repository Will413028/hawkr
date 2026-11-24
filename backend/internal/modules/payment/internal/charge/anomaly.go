package charge

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
)

// failedResolved 是人工結案成失敗的原因碼（failure_reason）。
const failedResolved = "admin_resolved"

// evidenceMax 與 provider 外部 ID 的欄位上限相同（CHECK 的 255）。
const evidenceMax = 255

// ResolveAnomaly 是後台對 ANOMALY 的搬錢物件的人工結案：管理員核對 provider 之後，把它結案成 AnomalySucceeded（provider 確實執行了，evidence 是 provider 物件 ID，必填）
// 或 AnomalyFailed（確認 provider 沒有執行，evidence 選填）。領域變更與一筆 adminaudit 在同一個 transaction，沒有紀錄的結案不成立（Connect 端點在後台入口接上）。
//   - 付款：成功是把有效嘗試連上 evidence 這個 PaymentIntent、暫記 AWAITING_PAYMENT 並排同步 job（真正的狀態由重讀推導）；失敗進 CREATE_FAILED（買家可以再付，換新鍵）。
//   - 轉帳：成功記 evidence 為外部轉帳 ID 並進 SUCCEEDED，累計已撤回額與等待中的撤回預留都以 provider 核實的轉帳為準（金額減 provider 上已撤回的額度，不用列上的數字，
//     預留依建立順序重新分配、沒有額度的釋出，之後讓可開始的撤回開始）；失敗進 FAILED（讓出一張訂單應收一筆有效轉帳的位置、釋出等待中的撤回預留）。
//   - 退款（ANOMALY 是退款呼叫的異常）：成功表示 provider 確實建立了退款，evidence 是退款 ID，暫記 REFUNDING 並排退款同步 job（真正的狀態由重讀推導，接受後才讓等待中的撤回開始）；失敗進 FAILED（不佔額度、釋出等待中的撤回預留）。
//   - 補差額轉帳（AnomalyTopup，id 是退款 ID，補差額狀態是 ANOMALY）：成功表示 provider 確實補了差額，evidence 是轉帳 ID，進 DONE；失敗確認沒有補，進 RELEASED（人工另行處理）。
//   - 撤回（AnomalyReversal，id 是撤回列 ID，撤回列狀態是 ANOMALY）：成功表示這次撤回確實成立，evidence 是撤回 ID，同一個 transaction 累加目標（原轉帳或補差額轉帳）的累計撤回額、撤回列進 REVERSED、重算退款的彙總（退款已 FAILED 時發 MoneyAnomaly）；失敗釋出這一列（RELEASED）並重算彙總。
//
// 結案成成功之前，先在 transaction 外向 provider 查回證據（依列上的冪等鍵／嘗試 ID 查，不信管理員貼的 ID；退款與撤回分別用 FindRefund 與 FindReversal）：查到的物件 ID、金額必須與證據和列相符，不符回 FailedPrecondition、
// 列不動；transaction 內鎖列之後再確認列上的鍵沒變。付款的證據在目前設定的 provider 查得到才行（列屬於已無法查回的 provider 時不能結案成成功）。
// 不在 ANOMALY 的物件回 FailedPrecondition，找不到回 NotFound。鎖順序：付款列、訂單應收列、轉帳列、退款列（依 ID 升序）、撤回列。呼叫方不能在 transaction 內呼叫。
func (s *Service) ResolveAnomaly(ctx context.Context, actor adminaudit.Actor, kind model.AnomalyKind, id uuid.UUID, outcome model.AnomalyOutcome, reason model.AnomalyResolutionReason, note, evidence string) error {
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
	note, err := validateResolution(outcome, reason, note)
	if err != nil {
		return err
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
		var anomalyReason string
		var err error
		switch kind {
		case model.AnomalyPayment:
			anomalyReason, err = s.resolvePayment(ctx, q, id, outcome, evidence, proof)
		case model.AnomalyTransfer:
			anomalyReason, err = s.resolveTransfer(ctx, q, id, outcome, evidence, proof)
		case model.AnomalyRefund:
			anomalyReason, err = s.resolveRefund(ctx, q, id, outcome, evidence, proof)
		case model.AnomalyReversal:
			anomalyReason, err = s.resolveReversal(ctx, q, id, outcome, evidence, proof)
		case model.AnomalyTopup:
			anomalyReason, err = s.resolveTopup(ctx, q, id, outcome, evidence, proof)
		default:
			return apperr.InvalidInput("不認得的物件種類")
		}
		if err != nil {
			return err
		}
		detail := map[string]any{"outcome": string(outcome), "anomaly_reason": anomalyReason, "resolution_reason": string(reason), "note": note}
		if evidence != "" {
			detail["evidence_external_id"] = evidence
		}
		return s.d.Audit.Record(ctx, adminaudit.Entry{Actor: actor, TargetType: "payment_" + string(kind), TargetID: id.String(), Detail: detail})
	})
}

// validateResolution 檢查人工結案的原因代碼與備註：原因必須是已知的值，而且與結果相容（PROVIDER_EXECUTED_NOT_RECORDED 只搭配成功、PROVIDER_NOT_EXECUTED 只搭配失敗）；
// 備註去頭尾空白後 1 到 ResolutionNoteMaxRunes 個字元。回傳整理過的備註。
func validateResolution(outcome model.AnomalyOutcome, reason model.AnomalyResolutionReason, note string) (string, error) {
	if !slices.Contains(model.AnomalyResolutionReasonValues(), reason) {
		return "", apperr.InvalidInputOn([]string{"Reason"}, "結案原因必填")
	}
	if (reason == model.ResolutionProviderExecutedNotRecorded && outcome != model.AnomalySucceeded) ||
		(reason == model.ResolutionProviderNotExecuted && outcome != model.AnomalyFailed) {
		return "", apperr.InvalidInputOn([]string{"Reason"}, "結案原因與結果不相容")
	}
	note = trimtext.Trim(note)
	if n := utf8.RuneCountInString(note); n < 1 || n > model.ResolutionNoteMaxRunes {
		return "", apperr.InvalidInputOn([]string{"Note"}, "結案備註必填，且不得超過 255 個字元")
	}
	return note, nil
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
		snap, found, err := s.d.Provider.FindPayment(callCtx, row.CreateIdempotencyKey.String(), lookupLowerBound(row.CreateCalledAt, row.CreatedAt))
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
		pay, _, err := paymentOfOrder(ctx, q, t.PaymentOrderID)
		if err != nil {
			return evidenceProof{}, err
		}
		snap, found, err := s.d.Provider.FindTransfer(callCtx, TransferGroupOf(pay.ID), t.IdempotencyKey.String(), lookupLowerBound(t.CalledAt, t.CreatedAt))
		if err != nil {
			return evidenceProof{}, fmt.Errorf("charge: 核對轉帳 %s 的證據失敗：%w", id, err)
		}
		if !found || snap.ExternalID != evidence || snap.Amount != t.Amount {
			return evidenceProof{}, errEvidenceMismatch
		}
		return evidenceProof{key: t.IdempotencyKey, amount: snap.Amount, reversed: snap.ReversedAmount}, nil
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
		pay, _, err := paymentOfOrder(ctx, q, r.PaymentOrderID)
		if err != nil {
			return evidenceProof{}, err
		}
		if pay.ExternalPaymentID == nil || pay.Provider != s.d.Provider.Name() {
			return evidenceProof{}, apperr.FailedPrecondition("付款沒有可查回的 provider 付款，不能結案成成功")
		}
		snap, found, err := s.d.Provider.FindRefund(callCtx, *pay.ExternalPaymentID, r.IdempotencyKey.String(), lookupLowerBound(r.RefundCalledAt, r.CreatedAt))
		if err != nil {
			return evidenceProof{}, fmt.Errorf("charge: 核對退款 %s 的證據失敗：%w", id, err)
		}
		if !found || snap.ExternalID != evidence || snap.Amount != r.Amount {
			return evidenceProof{}, errEvidenceMismatch
		}
		return evidenceProof{key: r.IdempotencyKey, parent: *pay.ExternalPaymentID}, nil
	case model.AnomalyReversal:
		seg, err := q.RefundReversalByID(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return evidenceProof{}, apperr.NotFound("找不到這筆撤回")
		}
		if err != nil {
			return evidenceProof{}, err
		}
		if seg.Status != string(model.ReversalAnomaly) {
			return evidenceProof{}, errNotAnomaly
		}
		snap, found, err := s.d.Provider.FindReversal(callCtx, seg.TargetExternalID, seg.IdempotencyKey.String())
		if err != nil {
			return evidenceProof{}, fmt.Errorf("charge: 核對撤回 %s 的證據失敗：%w", id, err)
		}
		if !found || snap.ExternalID != evidence || snap.Amount != seg.Amount {
			return evidenceProof{}, errEvidenceMismatch
		}
		return evidenceProof{key: seg.IdempotencyKey, parent: seg.TargetExternalID}, nil
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
		pay, _, err := paymentOfOrder(ctx, q, r.PaymentOrderID)
		if err != nil {
			return evidenceProof{}, err
		}
		snap, found, err := s.d.Provider.FindTransfer(callCtx, TransferGroupOf(pay.ID), r.TopupIdempotencyKey.String(), lookupLowerBound(r.TopupCalledAt, r.CreatedAt))
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
	if _, _, err := lockOrder(ctx, q, t.PaymentOrderID); err != nil {
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
		if err := s.releaseWaitingReversals(ctx, q, t.PaymentOrderID, reversalTransferFailed); err != nil {
			return reason, err
		}
		return reason, s.settleTopupsWithoutTransfer(ctx, q, t.PaymentOrderID)
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
	return reason, s.startTopups(ctx, q, resolved.PaymentOrderID)
}

// reassignWaitingReversals 在轉帳人工結案成成功之後，以 provider 核實的轉帳為準重新分配等待中的撤回預留：剩餘可撤額度 = 核實的金額 − provider 上已撤回的額度 − 其他進行中的預留，
// 依退款建立順序給每筆 min(退款額, 剩餘)，分到 0 的釋出（RELEASED，no_reversible_amount）；分配完讓 provider 已接受退款的撤回開始。呼叫方已鎖付款列與轉帳列。
func (s *Service) reassignWaitingReversals(ctx context.Context, q *store.Queries, tr store.PaymentTransfer, proof evidenceProof) error {
	waiting, err := q.WaitingReversals(ctx, tr.PaymentOrderID)
	if err != nil {
		return err
	}
	if len(waiting) == 0 {
		return nil
	}
	reserved, err := q.PendingReversalTotal(ctx, tr.PaymentOrderID)
	if err != nil {
		return err
	}
	var waitingTotal int64
	for _, r := range waiting {
		waitingTotal += r.ReversalAmount
	}
	topups, err := q.DoneTopupCapacity(ctx, tr.PaymentOrderID)
	if err != nil {
		return err
	}
	remaining := proof.amount - proof.reversed + topups - (reserved - waitingTotal)
	now := s.d.Now().UTC()
	for _, r := range waiting {
		amount := reversalFor(r.Amount, remaining)
		remaining -= amount
		if amount == 0 {
			nothing := reversalNothingLeft
			if _, err := q.ReleaseRefundReversal(ctx, store.ReleaseRefundReversalParams{ID: r.ID, Reason: &nothing, Now: now}); err != nil {
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
	return s.startReversals(ctx, q, tr.PaymentOrderID)
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
	pay, _, err := lockOrder(ctx, q, r.PaymentOrderID)
	if err != nil {
		return "", err
	}
	transferSucceeded, err := s.lockActiveTransfer(ctx, q, r.PaymentOrderID)
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
		written, err := q.ResolveRefundCallFailed(ctx, store.ResolveRefundCallFailedParams{ID: id, FailureReason: &failed, Now: now})
		if err != nil {
			return reason, err
		}
		if err := s.afterRefundFailed(ctx, written); err != nil {
			return reason, err
		}
		if transferSucceeded {
			if err := s.startTopups(ctx, q, r.PaymentOrderID); err != nil {
				return reason, err
			}
		}
		if err := s.wakeTransfer(ctx, q, id, r.PaymentOrderID); err != nil {
			return reason, err
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
	if _, _, err := lockOrder(ctx, q, r.PaymentOrderID); err != nil {
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

// resolveReversal 結案撤回列的異常（見 ResolveAnomaly）：id 是撤回列 ID。鎖順序：付款列、轉帳列（目標是原轉帳時）、退款列（依 ID 升序，含補差額轉帳的擁有者）、撤回列。
// 成功時累加目標的累計撤回額（不超過目標剩餘可撤額）並重算退款的彙總；失敗釋出這一列並重算彙總。
func (s *Service) resolveReversal(ctx context.Context, q *store.Queries, id uuid.UUID, outcome model.AnomalyOutcome, evidence string, proof evidenceProof) (string, error) {
	seg, err := q.RefundReversalByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", apperr.NotFound("找不到這筆撤回")
	}
	if err != nil {
		return "", err
	}
	var transferID uuid.UUID
	if seg.TargetRefundID == nil {
		found, err := q.ActiveTransferByOrder(ctx, seg.PaymentOrderID)
		if errors.Is(err, pgx.ErrNoRows) {
			return "", apperr.FailedPrecondition("付款沒有轉帳，不能結案撤回")
		}
		if err != nil {
			return "", err
		}
		transferID = found.ID
	}
	if err := lockReversalRows(ctx, q, seg, transferID); err != nil {
		return "", err
	}
	locked, err := q.RefundReversalByID(ctx, id)
	if err != nil {
		return "", err
	}
	if locked.Status != string(model.ReversalAnomaly) {
		return "", errNotAnomaly
	}
	reason := derefString(locked.Reason)
	now := s.d.Now().UTC()
	if outcome == model.AnomalyFailed {
		failed := failedResolved
		if _, err = q.ResolveReversalReleased(ctx, store.ResolveReversalReleasedParams{ID: id, Reason: &failed, Now: now}); err != nil {
			return reason, err
		}
		return reason, q.SettleRefundReversal(ctx, store.SettleRefundReversalParams{ID: seg.RefundID, Now: now})
	}
	if locked.IdempotencyKey != proof.key || locked.TargetExternalID != proof.parent {
		return "", errEvidenceChanged
	}
	remaining, err := s.targetRemaining(ctx, q, locked)
	if err != nil {
		return "", err
	}
	if locked.Amount > remaining {
		return "", apperr.FailedPrecondition("撤回額度超過目標轉帳的剩餘可撤額度")
	}
	if _, err := q.ResolveReversalDone(ctx, store.ResolveReversalDoneParams{ID: id, ExternalID: &evidence, Now: now}); err != nil {
		return "", conflictOrErr(err)
	}
	if err := addTargetReversal(ctx, q, locked, transferID, now); err != nil {
		return "", err
	}
	return reason, s.settleReversal(ctx, q, locked, evidence, now)
}

// targetRemaining 讀撤回列目標（已鎖）的剩餘可撤額度：原轉帳是 amount - reversed_amount，補差額轉帳是退款額 - topup_reversed_amount（且必須是 DONE）。
func (s *Service) targetRemaining(ctx context.Context, q *store.Queries, seg store.PaymentRefundReversal) (int64, error) {
	if seg.TargetRefundID == nil {
		tr, err := q.SucceededTransferByOrder(ctx, seg.PaymentOrderID)
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, apperr.FailedPrecondition("付款沒有已成立的轉帳，不能記撤回")
		}
		if err != nil {
			return 0, err
		}
		if tr.ExternalTransferID == nil || *tr.ExternalTransferID != seg.TargetExternalID {
			return 0, errEvidenceChanged
		}
		return tr.Amount - tr.ReversedAmount, nil
	}
	t, err := q.RefundByID(ctx, *seg.TargetRefundID)
	if err != nil {
		return 0, err
	}
	if t.TopupStatus != string(model.TopupDone) || t.TopupExternalID == nil || *t.TopupExternalID != seg.TargetExternalID {
		return 0, apperr.FailedPrecondition("補差額轉帳不是成功的，不能記撤回")
	}
	return t.Amount - t.TopupReversedAmount, nil
}
