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

// ---------------------------------------------------------------------------
// INV.op —— 開票送件排程器（§4.3.3 第 1 組、§7.1.2 步驟 11–14）
//
// 走 `pkg/outbox.Repository[T]` 的五方法契約（§4.3.3 line 825 與該節的 query 表逐字
// 指派：①~⑤ 給 INV.op）。**不是**自寫 claim 迴圈——那是 INV.rc 與 INV.hc 的裁決，
// 理由是「週期性重查同一列、次數無上界」，而 INV.op 是「推進到結論就結束」。
//
// 讓這條路徑安全的是三道守衛，缺一都會靜默出錯：
//   - ② ClaimInvoiceOperation 的候選述詞含 'SENDING' ⇒ 崩潰的租約回得來
//   - ③ MarkInvoiceOperationDone 帶 `AND status = 'SENDING'` ⇒ ProcessFunc 已經寫定的
//     終態（FAILED_PERMANENT / WAITING_CONFIRMATION…）不會被 Worker 的 MarkDone 蓋成
//     SUCCEEDED。沒有它，「永久失敗」會變成「成功」而且不報錯、不留痕
//   - ⑤ UpdateInvoiceOperationAttempt 寫回 RETRY_SCHEDULED ⇒ 真正的 transient 失敗
//     （DB 斷線之類）帶著退避回到候選集合
// ---------------------------------------------------------------------------

// operationRow 把 domain.Operation 套進 pkg/outbox.Row。
type operationRow struct{ op domain.Operation }

func (r operationRow) OutboxID() int64         { return r.op.ID }
func (r operationRow) OutboxAttemptCount() int { return int(r.op.AttemptCount) }

// issueSubmitter 是 INV.op 開票軸需要的 usecase 面。用窄介面讓 worker 的測試不必架整個 usecase。
type issueSubmitter interface {
	ClaimForSubmitAt(ctx context.Context, operationID int64, now, leaseUntil time.Time) (bool, error)
	Submit(ctx context.Context, operationID int64) error
}

// voidSubmitter 是 INV.op **作廢軸**的 usecase 面。
//
// 兩個軸共用同一條 lane（設計書 §7 表列：READY／RETRY_SCHEDULED／SENDING 這一組
// 述詞打 f0401／f0501／g0401／g0501，租約 120s），所以是同一支 worker、同一組候選、
// 同一份租約策略——**但送出的東西與落點完全不同**，因此分成兩個介面而不是在
// Submit 裡加一個 if。
//
// 🔴 少了這個分派，VOID operation 會被拿去走開票路徑，症狀是
// `document ... could not enter ISSUED`（2026-08-31 dev 實測）。
type voidSubmitter interface {
	ClaimForVoidAt(ctx context.Context, operationID int64, now, leaseUntil time.Time) (bool, error)
	SubmitVoid(ctx context.Context, operationID int64) error
}

// allowanceSubmitter 是 INV.op **折讓軸**的 usecase 面（#3073）。
//
// 第三條軸，理由與 voidSubmitter 逐字相同：同一條 lane、同一組候選、同一份租約策略，
// 但送出的東西（g0401，金額未稅）與落點（invoice_adjustments 的 M7）完全不同。
//
// 🔴 少了這個分派，ALLOWANCE operation 會被拿去走**開票**路徑 —— 症狀是
// `document ... could not enter ISSUED`，而那張發票早就開好了。
type allowanceSubmitter interface {
	ClaimForAllowanceAt(ctx context.Context, operationID int64, now, leaseUntil time.Time) (bool, error)
	SubmitAllowance(ctx context.Context, operationID int64) error
}

// isVoidOperation 是本檔的分派述詞之一。
//
// 錨在 CodeMeta 的值而不是字面字串：operation_type 的值域由 CodeMeta 生成，
// 字面字串會在碼表改名時靜默失配（而失配的後果是作廢單被拿去開票）。
func isVoidOperation(op domain.Operation) bool {
	return op.OperationType == codemeta.InvoiceOperationTypeVoid.Value
}

// isAllowanceOperation 同理，錨在 CodeMeta 的值而不是字面字串。
func isAllowanceOperation(op domain.Operation) bool {
	return op.OperationType == codemeta.InvoiceOperationTypeAllowance.Value
}

// issuePathOperationTypes 是**明列**可以走開票路徑的 operation_type。
//
// 🔴 這裡刻意用白名單而不是 default fallthrough。原本的 `default:` 會把
// **任何**不是 VOID／ALLOWANCE 的型別送去 ClaimForSubmitAt ——
// 而 chk_invop_type 的值域有六個，另外兩個（QUERY／VOID_ALLOWANCE）
// 今天沒有生產者，所以這個洞是靜默的：哪天有人補上生產者，那一類 operation
// 會被拿去**開票**，症狀是 `document ... could not enter ISSUING`
// （正是 voidSubmitter／allowanceSubmitter 檔頭警告的同一個形狀）。
//
// ⚠️ VOID_ALLOWANCE（AMEGO g0501，作廢一張已開立的折讓單）**整條軸今天不存在**：
// 契約層齊備（CodeMeta 值、chk_invop_type、I5 述詞、operation_key 前綴、退避參數），
// 但沒有生產者、沒有分派、沒有 usecase，`amego.Client.VoidAllowance` 零呼叫端。
// 那是 58 號 ADR-32／D17 的刻意範圍決定（賣家主動申請作廢／折讓入口移出 Phase 1），
// 不是漏做。要補的時候：先補 M7 的 COMPLETED → CANCELLED 邊（今天只有 G18 的
// FAILED_UNKNOWN → CANCELLED），再補這裡的分派。
var issuePathOperationTypes = map[string]struct{}{
	codemeta.InvoiceOperationTypeIssue.Value:   {},
	codemeta.InvoiceOperationTypeReissue.Value: {},
}

// ErrUnroutableOperation 是「這個 operation_type 沒有任何執行者」。
//
// fail-closed：寧可讓這一筆重試到 ADM 工作台，也不要讓它靜靜走錯路徑。
var ErrUnroutableOperation = errors.New("invoice: operation_type has no executor on the INV.op lane")

// issueRepo 是 pkg/outbox.Repository[operationRow] 的實作。
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
	// 分派要在 claim 這一層做，不能只在 process 做：claim 的第二半是
	// document 的狀態轉移（開票 → ISSUING、作廢 → VOIDING），兩者是**不同的邊**。
	// 只在 process 分派的話，作廢單會先把 document 推進 ISSUING，而那條邊在 M1
	// 根本不存在（ISSUED ⇏ ISSUING）⇒ rows=0 被當成合法的重入而靜默放行。
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

// process 是 ProcessFunc：送一次件。
//
// Submit 在**所有**業務落點都回 nil（成功、永久失敗、重試排程、結果未明…）——它已經
// 把狀態寫定了。回 error 只代表「這一輪連狀態都沒寫成」（DB 斷線之類），那時才該由
// Worker 記一次 attempt 並退避。
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

// observeOutcome 記 M1（invoice_issue_total）與 M22（invoice_issue_e2e_duration_seconds）。
//
// 為什麼是「Submit 之後再讀一次」而不是讓 Submit 回傳落點：Submit 的六個 settle* 分支
// 各自在自己的交易內寫終態，回傳一個「我落在哪裡」的列舉等於把同一件事寫兩份，而兩份
// 一定會漂。讀回來的是 DB 上的事實。
//
// 「一輩子一次」由 claim 保證：進到這裡的列在 Submit 之前必然是 SENDING（claim 的 CAS
// 就是把它寫成 SENDING 的那一句），所以任何終態都恰好被觀測一次。
//
// 指標失敗絕不阻斷業務 ⇒ 全程只記 log。
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
		// 🔴 終點是 provider_issued_at（AMEGO 回的 invoice_time），**不是我方 now**：
		// 時鐘偏移會算出負延遲，而時鐘偏移正是本服務的已知風險（§8.3.1 code 15）。
		//
		// ⚠️ 起點：§8.2.3 M22 要的是 `COALESCE(provider_paid_at, pay_time)`，而
		// invoice_documents 上**沒有**那一欄。Phase 1 用 issue_not_before 是等值的
		// ——`buildPlan` 逐字寫著 `IssueNotBefore: in.PaidAt`（§7.1.2 步驟 9：Phase 1
		// 的 issue_not_before ＝ invoice_date ＝ 即刻）。
		// 🔴 **哪一天 issue_not_before 被加上延遲（步驟 9 的 Phase 2），這個等式就不成立，
		// 而症狀是 SLO-2 的 p99 整體偏小、沒有任何東西會轉紅。** 那時要做的是在
		// invoice_documents 上補一個 paid_at 欄（或從 source_snapshot 取），不是在這裡調偏移。
		w.metrics.ObserveIssueE2E(doc.DocumentKind, doc.ProviderIssuedAt.Sub(doc.IssueNotBefore).Seconds())
	}
}

// issueResultOf 把 M2 的終態映射到 M1 的 7 個 result 值（§8.2.3 M1）。
//
// terminal=false 代表這一列還在自動推進迴圈內（RETRY_SCHEDULED／WAITING_CONFIRMATION／
// SELLER_ACTION_REQUIRED…）⇒ 不計數。SELLER_ACTION_REQUIRED 刻意不算終態：賣家去財政部
// 配完字軌之後它還會回到迴圈，算進去會讓同一個 operation 被計兩次。
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
