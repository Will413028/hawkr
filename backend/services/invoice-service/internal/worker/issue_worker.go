package worker

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/outbox"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"go.uber.org/zap"
)

type operationRow struct{ op domain.Operation }

func (r operationRow) OutboxID() int64         { return r.op.ID }
func (r operationRow) OutboxAttemptCount() int { return int(r.op.AttemptCount) }

type issueSubmitter interface {
	ClaimForSubmitAt(ctx context.Context, operationID int64, now, leaseUntil time.Time) (bool, error)
	Submit(ctx context.Context, operationID int64) error
}

type voidSubmitter interface {
	ClaimForVoidAt(ctx context.Context, operationID int64, now, leaseUntil time.Time) (bool, error)
	SubmitVoid(ctx context.Context, operationID int64) error
}

type allowanceSubmitter interface {
	ClaimForAllowanceAt(ctx context.Context, operationID int64, now, leaseUntil time.Time) (bool, error)
	SubmitAllowance(ctx context.Context, operationID int64) error
}

func isVoidOperation(op domain.Operation) bool {
	return op.OperationType == codemeta.InvoiceOperationTypeVoid.Value
}

func isAllowanceOperation(op domain.Operation) bool {
	return op.OperationType == codemeta.InvoiceOperationTypeAllowance.Value
}

var issuePathOperationTypes = map[string]struct{}{
	codemeta.InvoiceOperationTypeIssue.Value:   {},
	codemeta.InvoiceOperationTypeReissue.Value: {},
}

// ErrUnroutableOperation 是「這個 operation_type 沒有任何執行者」。
//
// fail-closed：寧可讓這一筆重試到 ADM 工作台，也不要讓它靜靜走錯路徑。
var ErrUnroutableOperation = errors.New("invoice: operation_type has no executor on the INV.op lane")

type issueRepo struct {
	ops       domain.OperationRepository
	uc        issueSubmitter
	void      voidSubmitter
	allowance allowanceSubmitter
	log       *zap.Logger
}

var _ outbox.Repository[operationRow] = (*issueRepo)(nil)

// FetchPendingDue 先跑兩支前置步驟（⑪⑫），再撈候選。
//
// 兩支都是集合式、無外部副作用的單句 UPDATE ⇒ 不需要 claim（§5.2.1／§7.5.4 註 3、註 4）。
// 🔴 少了 PromoteDueTriggers，J05（AWAITING_TRIGGER → READY）**沒有任何元件會執行**
// ——不是編譯錯、不是測試紅，是那批列永遠不動。
//
// 前置步驟失敗不阻斷本輪：它們是把更多列推進候選集合，失敗只代表這一輪少撈幾列，
// 下一輪（10 秒後）會再試一次。真正不能失敗的是 FetchDue 本身。
func (r *issueRepo) FetchPendingDue(ctx context.Context, now time.Time, limit int) ([]operationRow, error) {
	if n, err := r.ops.ReleaseExpiredHolds(ctx, now); err != nil {
		r.log.Error("invoice.op.release_holds_failed", zap.Error(err))
	} else if n > 0 {
		r.log.Info("invoice.op.released_holds", zap.Int64("rows", n))
	}
	if n, err := r.ops.PromoteDueTriggers(ctx, now); err != nil {
		r.log.Error("invoice.op.promote_triggers_failed", zap.Error(err))
	} else if n > 0 {
		r.log.Info("invoice.op.promoted_triggers", zap.Int64("rows", n))
	}

	ops, err := r.ops.FetchDue(ctx, now, int32(limit)) //nolint:gosec // limit 來自 config，值域由 Validate 守
	if err != nil {
		return nil, err
	}
	rows := make([]operationRow, 0, len(ops))
	for _, op := range ops {
		rows = append(rows, operationRow{op: op})
	}
	return rows, nil
}

// Claim 是搶佔閘門。
//
// 它同時做兩件必須一起成立的事（operation CAS ＋ document → ISSUING），所以委派給
// usecase 的 ClaimForSubmitAt，而不是直接呼叫 ops.Claim：拆開之後只要 document 那半
// 失敗，賣家會看到「待開立」而系統正在送，兩邊都不會報錯。
//
// 🔴 leaseUntil 由 worker **逐筆**現算後傳進來（不是在批次迴圈外算一次）：算一次會讓
// 不變式從「單筆最壞耗時 < 租約」退化成「批次總耗時 < 租約」，隨 batch-size 一起破。
func (r *issueRepo) Claim(ctx context.Context, id int64, leaseUntil, claimableBefore time.Time) (int64, error) {
	op, err := r.ops.GetByID(ctx, id)
	if err != nil {
		return 0, err
	}
	var ok bool
	switch {
	case isVoidOperation(op):
		ok, err = r.void.ClaimForVoidAt(ctx, id, claimableBefore, leaseUntil)
	case isAllowanceOperation(op):
		ok, err = r.allowance.ClaimForAllowanceAt(ctx, id, claimableBefore, leaseUntil)
	default:
		if _, routable := issuePathOperationTypes[op.OperationType]; !routable {
			r.log.Error("invoice.op.unroutable_operation_type",
				zap.Int64("operation_id", op.ID),
				zap.String("operation_type", op.OperationType))
			return 0, fmt.Errorf("%w: %s (operation %d)", ErrUnroutableOperation, op.OperationType, op.ID)
		}
		ok, err = r.uc.ClaimForSubmitAt(ctx, id, claimableBefore, leaseUntil)
	}
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, nil
	}
	return 1, nil
}

// MarkDone 走 ③（帶 `AND status='SENDING'` 守衛）。rows=0 是**合法**的，見本檔頭。
func (r *issueRepo) MarkDone(ctx context.Context, id int64, now time.Time) error {
	_, err := r.ops.MarkDone(ctx, id, now)
	return err
}

// MarkFailed 走 ④。同樣守狀態，不覆蓋 ProcessFunc 已經寫定的終態。
func (r *issueRepo) MarkFailed(ctx context.Context, id int64, lastErr string, now time.Time) error {
	_, err := r.ops.MarkFailed(ctx, id, lastErr, now)
	return err
}

// UpdateAttempt 走 ⑤。
func (r *issueRepo) UpdateAttempt(ctx context.Context, id int64, attemptCount int, nextAttemptAt time.Time, lastError string) error {
	var le *string
	if lastError != "" {
		le = &lastError
	}
	return r.ops.UpdateAttempt(ctx, id, int32(attemptCount), nextAttemptAt, le, time.Now().UTC()) //nolint:gosec // attemptCount 上界是 max_attempts
}

// IssueWorker 是 INV.op 的整支：Worker（claim ＋ process ＋ 記帳）＋ 結果指標。
type IssueWorker struct {
	w         *outbox.Worker[operationRow]
	ops       domain.OperationRepository
	docs      domain.DocumentRepository
	uc        issueSubmitter
	void      voidSubmitter
	allowance allowanceSubmitter
	metrics   *Metrics
	log       *zap.Logger
}

// NewIssueWorker 組出 INV.op。
func NewIssueWorker(
	ops domain.OperationRepository,
	docs domain.DocumentRepository,
	uc issueSubmitter,
	void voidSubmitter,
	allowance allowanceSubmitter,
	cfg outbox.Config,
	metrics *Metrics,
	obs *outbox.Metrics,
	log *zap.Logger,
) *IssueWorker {
	l := orNopLogger(log)
	iw := &IssueWorker{ops: ops, docs: docs, uc: uc, void: void, allowance: allowance, metrics: metrics, log: l}
	repo := &issueRepo{ops: ops, uc: uc, void: void, allowance: allowance, log: l}
	iw.w = outbox.New[operationRow](repo, iw.process, cfg, outbox.Hooks[operationRow]{
		OnAttemptFailed: func(_ context.Context, row operationRow, cause string, err error) {
			obs.AttemptFailedTotal(cause).Inc()
			l.Error("invoice.op.attempt_failed",
				zap.Int64("operation_id", row.op.ID), zap.String("cause", cause), zap.Error(err))
		},
		OnTerminalFailed: func(_ context.Context, row operationRow, attempt int, lastErr string, markErr error) {
			if markErr == nil {
				obs.FailedTotal().Inc()
			}
			l.Error("invoice.op.terminal_failed",
				zap.Int64("operation_id", row.op.ID), zap.Int("attempt", attempt),
				zap.String("last_error", lastErr), zap.Error(markErr))
		},
	})
	return iw
}

// RunOnce 跑一輪。由 Scheduler 每 INVOICE_ISSUE_POLL_INTERVAL_SECONDS 秒呼叫一次。
func (w *IssueWorker) RunOnce(ctx context.Context) error { return w.w.RunOnce(ctx) }

func (w *IssueWorker) process(ctx context.Context, row operationRow) (any, error) {
	send := w.uc.Submit
	switch {
	case isVoidOperation(row.op):
		send = w.void.SubmitVoid
	case isAllowanceOperation(row.op):
		send = w.allowance.SubmitAllowance
	}
	if err := send(ctx, row.op.ID); err != nil {
		return nil, err
	}
	w.observeOutcome(ctx, row.op.ID)
	return nil, nil
}

func (w *IssueWorker) observeOutcome(ctx context.Context, operationID int64) {
	op, err := w.ops.GetByID(ctx, operationID)
	if err != nil {
		w.log.Error("invoice.op.observe_reload_operation_failed",
			zap.Int64("operation_id", operationID), zap.Error(err))
		return
	}
	result, terminal := issueResultOf(op.Status)
	if !terminal {
		return
	}

	doc, err := w.docs.GetByID(ctx, op.DocumentID)
	if err != nil {
		w.log.Error("invoice.op.observe_reload_document_failed",
			zap.Int64("operation_id", operationID), zap.Int64("document_id", op.DocumentID), zap.Error(err))
		return
	}
	w.metrics.ObserveIssueResult(result, doc.FulfillmentMode)

	if result == "succeeded" && doc.ProviderIssuedAt != nil {
		w.metrics.ObserveIssueE2E(doc.DocumentKind, doc.ProviderIssuedAt.Sub(doc.IssueNotBefore).Seconds())
	}
}

func issueResultOf(status string) (string, bool) {
	switch status {
	case codemeta.InvoiceOperationStatusSucceeded.Value:
		return "succeeded", true
	case codemeta.InvoiceOperationStatusFailedPermanent.Value:
		return "permanent_failed", true
	case codemeta.InvoiceOperationStatusFailedRetryExhausted.Value:
		return "retry_exhausted", true
	case codemeta.InvoiceOperationStatusFailedUnknown.Value:
		return "unknown_terminal", true
	case codemeta.InvoiceOperationStatusCancelledByAdmin.Value,
		codemeta.InvoiceOperationStatusCancelledBeforeIssue.Value:
		return "cancelled", true
	case codemeta.InvoiceOperationStatusNotApplicable.Value:
		return "not_applicable", true
	case codemeta.InvoiceOperationStatusResolvedExternally.Value:
		return "resolved_externally", true
	default:
		return "", false
	}
}

func orNopLogger(l *zap.Logger) *zap.Logger {
	if l == nil {
		return zap.NewNop()
	}
	return l
}
