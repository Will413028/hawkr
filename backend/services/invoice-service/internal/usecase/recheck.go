package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/invoice-service/internal/client/amego"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/domain/fsm"
	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// INV.rc —— 結果未明對帳 ＋ 上傳狀態覆查（§4.3.3 ⑥⑦、§7.3 J15–J31、§7.5.3）
//
// 兩個候選集合、兩組租約，同一支 worker：
//   ① invoice_operations 的「結果未明」狀態集合 —— 租約 60s（下界 20.3s）
//   ② invoice_documents.upload_recheck_next_at —— 租約 120s（下界 10.3s）
//
// 🔴 **不走 pkg/outbox.Worker**（§4.3.3 line 825 的判準）：那個契約適用「推進到結論
// 就結束、次數有上界」的排程；INV.rc 是「週期性重查同一列直到某個業務條件成立」，
// 同一列會被處理多次、次數無上界 ⇒ 自寫 claim 迴圈。panic recovery 不受影響——
// 它出自 pkg/outbox.Scheduler 的逐 tick recover，不是 Worker 給的。
//
// 🔴 前置 ③「一次 claim 只做一次外部呼叫」是 60s 租約**唯一**的成立理由：
// 一次 claim 內翻 N 頁的最壞耗時是 N × 20s，N ≥ 4 就超過租約 ⇒ 別的副本合法搶走
// 仍在飛的那一列 ⇒ 重複外呼。本檔每一輪對每一列只打一次 invoice_query。
//
// 🔴 為什麼這支非有不可：沒有它，「結果未明」的 operation **永不收斂**，而
// L4 法定上傳時限（B2C 開立日 +2 日 23:59:59、末日不因假日延長）照樣在倒數。
// ---------------------------------------------------------------------------

// recheckOpLease 是候選集合①的租約（§7.5.4，下界 20.3s ＝ max(query 10s, list 一頁 20s) ＋ DB 0.3s）。
const recheckOpLease = 60 * time.Second

// recheckUploadLease 是候選集合②的租約（§7.5.4，下界 10.3s ＝ invoice_status 10s ＋ DB 0.3s）。
const recheckUploadLease = 120 * time.Second

// recheckBatch 是單輪各撈幾筆（§7.5.3 FetchBatch = 50）。
const recheckBatch = 50

// providerRetentionWindow 是 AMEGO 查詢窗口（§8.1.5 K2 ＝ 180 天）。
//
// 🔴 J25 的條件含這一項：超過它時 AMEGO 的 71 **不可信**——對「開過的發票」也回
// 「查無」⇒ 判成「確定沒開」而補送 ⇒ 同一筆訂單開出第二張發票。
const providerRetentionWindow = 180 * 24 * time.Hour

// derefOr 把可空欄位取值，nil 時回空字串。
//
// Provider 與 SourceRef 在 schema 上可空，但走到 INV.rc 的候選集合時它們必然有值
// （operation 存在 ⇒ 文件已 Prepare 過）。回空字串而不是 panic：讓 AMEGO 那側回
// 明確錯誤，好過在 worker 內炸掉整輪。
func derefOr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// RecheckQuerier 是候選集合①需要的出站面。
type RecheckQuerier interface {
	QueryInvoiceByOrderID(ctx context.Context, c amego.Credential, orderID string) (*amego.InvoiceDetail, error)
}

// RecheckOperationStore 是候選集合①需要的 repository 面。
type RecheckOperationStore interface {
	FetchPendingRechecks(ctx context.Context, now time.Time, limit int32) ([]domain.Operation, error)
	ClaimRecheck(ctx context.Context, id int64, now, leaseUntil time.Time) (bool, error)
	MarkRecheckResolved(ctx context.Context, in domain.MarkRecheckResult) (bool, error)
	MarkRecheckUnresolved(ctx context.Context, in domain.MarkRecheckResult) (bool, error)
	UpdateRecheckAttempt(ctx context.Context, in domain.UpdateRecheckAttempt) (bool, error)
}

// RecheckDocumentStore 是兩組候選都要用的 document 面。
type RecheckDocumentStore interface {
	GetByID(ctx context.Context, id int64) (domain.Document, error)
	FetchDueUploadRechecks(ctx context.Context, now time.Time, limit int32) ([]domain.Document, error)
	ClaimUploadRecheck(ctx context.Context, id int64, now, leaseUntil time.Time) (bool, error)
	RecordUploadRecheckResult(ctx context.Context, id int64, providerUploadStatus *int16, now time.Time, nextAt *time.Time) error
}

// RecheckVoidConfirmer 是作廢軸的確認面。
//
// 🔴 為什麼 INV.rc 要分派而不是讓 VOID operation 走原本那條路：probeOperation 打的是
// invoice_query type=order 並以「查得到發票號碼」當成「開立成功」。作廢單走進去
// 會查到那張**仍然存在的**發票 ⇒ 判成 Resolved ⇒ 把 document 寫回 ISSUED，
// 等於把剛送出去的作廢從帳上抹掉，而且完全不報錯。
type RecheckVoidConfirmer interface {
	ConfirmVoid(ctx context.Context, op domain.Operation) error
}

// RecheckAllowanceConfirmer 是折讓軸的確認面。
//
// 🔴 與作廢同一個理由、不同的後果：折讓 operation 走 probeOperation 會查到那張
// **仍然存在的原發票**、判成 Resolved ⇒ operation 被標成完成，而折讓到底開沒開
// 沒有任何人知道，adjustment 永遠停在 WAITING_CONFIRMATION。折讓只能拿我方自產的
// allowance_number 打 allowance_query，見 Allowance.ConfirmAllowance。
type RecheckAllowanceConfirmer interface {
	ConfirmAllowance(ctx context.Context, op domain.Operation) error
}

// RecheckCredentialSource 供解密憑證。
type RecheckCredentialSource interface {
	GetByIssuer(ctx context.Context, issuerID int64, provider string) (domain.ProviderAccount, error)
	WithDecryptedCredential(ctx context.Context, id int64, fn func(providerTaxID, appKey string) error) error
}

// Recheck 是 INV.rc 的 usecase。
type Recheck struct {
	voids      RecheckVoidConfirmer
	allowances RecheckAllowanceConfirmer
	ops        RecheckOperationStore
	docs       RecheckDocumentStore
	accounts   RecheckCredentialSource
	amego      RecheckQuerier
	// audit 是 I7 的寫入面（#3109，2026-09-04 補）。在那之前 INV.rc 的六條轉移
	// （J23／J24／J25／J26／J17／J31）**一列 history 都不寫**，而 A10 稽核與
	// AuditTrailDrawer 讀的就是那張表 ⇒ 使用者看到的軌跡是斷的。
	audit domain.AuditRepository
	log   *zap.Logger
	now   func() time.Time
}

// NewRecheck 建 INV.rc usecase。
func NewRecheck(
	ops RecheckOperationStore, docs RecheckDocumentStore,
	accounts RecheckCredentialSource, querier RecheckQuerier,
	voids RecheckVoidConfirmer, allowances RecheckAllowanceConfirmer,
	audit domain.AuditRepository, log *zap.Logger, now func() time.Time,
) *Recheck {
	return &Recheck{voids: voids, allowances: allowances, ops: ops, docs: docs, accounts: accounts,
		amego: querier, audit: audit, log: orNop(log), now: now}
}

// RunOnce 跑一輪：先對帳①、再覆查②。
//
// 兩組獨立，①失敗不擋②——它們是不同的候選集合、不同的租約、不同的業務後果。
func (u *Recheck) RunOnce(ctx context.Context) error {
	var firstErr error
	if err := u.reconcileOperations(ctx); err != nil {
		firstErr = err
		u.log.Error("invoice.rc.operations_failed", zap.Error(err))
	}
	if err := u.recheckUploads(ctx); err != nil {
		if firstErr == nil {
			firstErr = err
		}
		u.log.Error("invoice.rc.uploads_failed", zap.Error(err))
	}
	return firstErr
}

// reconcileOperations 是候選集合①。
func (u *Recheck) reconcileOperations(ctx context.Context) error {
	due, err := u.ops.FetchPendingRechecks(ctx, u.now(), recheckBatch)
	if err != nil {
		return err
	}
	var claimed, resolved, stillWaiting, terminal int
	for _, op := range due {
		// 🔴 租約基準逐筆在 claim 當下取（《多副本排程搶佔規範》）：在迴圈外算一次
		// 會讓不變式從「單筆最壞耗時 < 租約」退化成「整批總耗時 < 租約」。
		at := u.now()
		ok, err := u.ops.ClaimRecheck(ctx, op.ID, at, at.Add(recheckOpLease))
		if err != nil {
			u.log.Error("invoice.rc.claim_failed", zap.Int64("operation_id", op.ID), zap.Error(err))
			continue
		}
		if !ok {
			continue // 別的副本先到，或述詞已不成立。不是錯誤。
		}
		claimed++
		// CAS 成功之後才留痕（op.Status 此時才確定是真實的 from，見 recordClaimTransition）。
		if err := u.recordClaimTransition(ctx, op.ID, op.Status, at); err != nil {
			u.log.Error("invoice.rc.claim_history_failed",
				zap.Int64("operation_id", op.ID), zap.String("from", op.Status), zap.Error(err))
		}
		// 🔴 **claim 已經把 status 蓋成 PROVIDER_STATUS_RECHECK，記憶體裡這一份必須跟上。**
		//
		// 不同步的後果不是「少一列 history」而是**整條 INV.rc 軸的 I7 完全不成立**：
		// 下游的 applyDecision 拿 op.Status 當 fsm.CheckOperation 的 from，而那是
		// FetchPendingRechecks 撈出來的**claim 之前**的值（WAITING_CONFIRMATION／
		// FAILED_UNKNOWN／CANCEL_REQUESTED／CANCEL_BLOCKED_UNKNOWN 之一）。
		// 真實流程是兩步（X --claim--> PROVIDER_STATUS_RECHECK --decide--> Y），
		// 合併成 (X, Y) 之後多半不在 M2 表上 ⇒ CheckOperation 回 error ⇒
		//   ① 一列 history 都不寫
		//   ② applyDecision 回 error、迴圈 continue ⇒ resolved／stillWaiting／terminal 三個計數少計
		// dev 實證（2026-09-05 全表查詢）：`reason_code='INV_RECHECK'` **0 列**、
		// `to_status='PROVIDER_STATUS_RECHECK'` **0 列**，而同一張表有 37 種其他 reason_code
		// （對照組，證明查詢有效）；pod log 逐字 `invoice.rc.illegal_transition ... not in the M2 table (I7)`
		// 緊接著 `invoice.rc.apply_failed`。
		//
		// ⚠️ 這一行要放在 recordClaimTransition **之後**：那一支要的 from 正是舊值。
		op.Status = codemeta.InvoiceOperationStatusProviderStatusRecheck.Value
		// 🔴 作廢單走**另一條**確認路徑。共用下面那條會把作廢從帳上抹掉，見
		// RecheckVoidConfirmer 的說明。分派錨在 CodeMeta 的值而不是字面字串。
		if op.OperationType == codemeta.InvoiceOperationTypeVoid.Value {
			if err := u.voids.ConfirmVoid(ctx, op); err != nil {
				u.log.Error("invoice.rc.confirm_void_failed",
					zap.Int64("operation_id", op.ID), zap.Error(err))
			}
			stillWaiting++
			continue
		}
		// 🔴 折讓單同理，而且後果更隱蔽：查到的是原發票 ⇒ 判成 Resolved ⇒
		// operation 收尾，但 adjustment 永遠留在 WAITING_CONFIRMATION，沒有第二個
		// 執行者會再碰它。見 RecheckAllowanceConfirmer。
		if op.OperationType == codemeta.InvoiceOperationTypeAllowance.Value {
			if err := u.allowances.ConfirmAllowance(ctx, op); err != nil {
				u.log.Error("invoice.rc.confirm_allowance_failed",
					zap.Int64("operation_id", op.ID), zap.Error(err))
			}
			stillWaiting++
			continue
		}
		probe, err := u.probeOperation(ctx, op)
		if err != nil {
			// 連查都查不成（憑證解不開、文件讀不到）⇒ 當成「仍不確定」走 J31/J17，
			// 不要當成「確定沒開」——後者會補送而重複開票。
			u.log.Warn("invoice.rc.probe_failed", zap.Int64("operation_id", op.ID), zap.Error(err))
		}
		d := decideRecheck(op, probe, u.now())
		if err := u.applyDecision(ctx, op, d); err != nil {
			u.log.Error("invoice.rc.apply_failed", zap.Int64("operation_id", op.ID), zap.Error(err))
			continue
		}
		switch d.Kind {
		case RecheckResolved:
			resolved++
		case RecheckStillWaiting:
			stillWaiting++
		case RecheckUnresolvedTerminal:
			terminal++
		}
	}
	u.log.Info("invoice.rc.operations_scanned",
		zap.Int("due", len(due)), zap.Int("claimed", claimed),
		zap.Int("resolved", resolved), zap.Int("still_waiting", stillWaiting), zap.Int("terminal", terminal))
	return nil
}

// probeOperation 對單一 operation 打**一次** invoice_query（前置 ③）。
func (u *Recheck) probeOperation(ctx context.Context, op domain.Operation) (RecheckProbe, error) {
	var out RecheckProbe
	doc, err := u.docs.GetByID(ctx, op.DocumentID)
	if err != nil {
		return out, err
	}
	// 🔴 發票日期在 180 天內才讓 71 可信（J25）。用 invoice_date_taipei 而不是
	// provider_issued_at：後者在「從未成功開立」的情況下是 NULL，而那正是本迴圈
	// 最常見的處境。
	if doc.InvoiceDateTaipei != nil {
		out.WithinProviderRetention = u.now().Sub(*doc.InvoiceDateTaipei) <= providerRetentionWindow
	}

	acct, err := u.accounts.GetByIssuer(ctx, doc.IssuerID, derefOr(doc.Provider))
	if err != nil {
		return out, err
	}
	var queryErr error
	var detail *amego.InvoiceDetail
	if err := u.accounts.WithDecryptedCredential(ctx, acct.ID, func(providerTaxID, appKey string) error {
		detail, queryErr = u.amego.QueryInvoiceByOrderID(ctx,
			amego.Credential{IssuerID: doc.IssuerID, Invoice: providerTaxID, AppKey: appKey},
			derefOr(doc.SourceRef))
		return nil
	}); err != nil {
		return out, err
	}

	if queryErr == nil && detail != nil {
		out.Found = true
		out.InvoiceNumber = detail.InvoiceNumber
		return out, nil
	}
	var apiErr *amego.APIError
	if errors.As(queryErr, &apiErr) {
		code := apiErr.Code
		msg := apiErr.Msg
		out.ProviderCode = &code
		out.ProviderMessage = &msg
		// NotFound ＝ code 71（「180 天內確定沒有」）。是正常業務結果不是錯誤。
		out.NotIssued = apiErr.NotFound
	}
	return out, queryErr
}

// applyDecision 依 Kind 選 repository 方法寫回，**並留痕**（I7）。
//
// 🔴 三個分支原本都只寫 status 不寫 history（#3109）。後果不是「少了幾列」而是
// **軌跡自相矛盾**：dev 上實測到 `→ CANCEL_REQUESTED` 的下一列直接是
// `CANCELLED_BY_ADMIN →`，中間 J27＋J24 兩步整段消失，照它讀這筆 job 是
// 憑空變成已取消的。
//
// ⚠️ 留痕在 CAS **成功之後**才做，且 rows=0 時不寫：CAS 失敗代表狀態不是我以為的
// 那個，此時 op.Status 是快照值，拿它當 from 會寫出一條沒發生過的邊。
func (u *Recheck) applyDecision(ctx context.Context, op domain.Operation, d RecheckDecision) error {
	now := u.now()
	var ok bool
	var err error
	switch d.Kind {
	case RecheckResolved:
		completed := now
		ok, err = u.ops.MarkRecheckResolved(ctx, domain.MarkRecheckResult{
			ID: op.ID, NextStatus: d.NextStatus, NextAttemptAt: d.NextAttemptAt,
			CompletedAt: &completed,
			// J24 專屬的兩欄。在此之前它們在 decideRecheck 算出來、然後被丟掉
			// （domain.MarkRecheckResult 沒有對應欄位）⇒ §6.4 的「證據」那一樣缺席，
			// 而擋它的 CHECK 因為三值邏輯是 no-op ⇒ 完全無聲。
			CancelEvidenceKind: d.CancelEvidenceKind,
			CancelConfirmedAt:  d.CancelConfirmedAt,
			Now:                now,
		})
	case RecheckUnresolvedTerminal:
		ok, err = u.ops.MarkRecheckUnresolved(ctx, domain.MarkRecheckResult{
			ID: op.ID, NextStatus: d.NextStatus, NextAttemptAt: d.NextAttemptAt, Now: now,
		})
	default:
		ok, err = u.ops.UpdateRecheckAttempt(ctx, domain.UpdateRecheckAttempt{
			ID: op.ID, NextStatus: d.NextStatus, NextAttemptAt: d.NextAttemptAt,
			RecheckListPage: op.RecheckListPage, Now: now,
		})
	}
	if err != nil || !ok {
		return err
	}
	return u.recordTransition(ctx, op.ID, op.Status, d.NextStatus, now)
}

// recordTransition 是 INV.rc 這一側的 I7 寫入點。
//
// reason_code 走 `INV_RECHECK`：這一軸的轉移全部由對帳結果驅動，沒有 actor，
// 而 §5.1.12 的 actor_type 對它是 `SYSTEM`。**不填 J 碼**——J 碼是設計書的編號、
// 不是 CodeMeta 值域，把它寫進 reason_code 會讓那一欄同時承載兩種值域。
func (u *Recheck) recordTransition(ctx context.Context, id int64, from, to string, now time.Time) error {
	kind, err := fsm.CheckOperation(from, to)
	if err != nil {
		// fsm 不認得這條邊 ⇒ 轉換表漏登記，或有人繞過它加了邊。兩者都要當場炸掉，
		// 否則 history 會累積出一份與設計書不一致的「事實」。
		u.log.Error("invoice.rc.illegal_transition",
			zap.Int64("operation_id", id), zap.String("from", from), zap.String("to", to), zap.Error(err))
		return err
	}
	if kind == fsm.TransitionSelfLoop {
		// 留在原地等下一輪（J31／自轉移）。最短 30 秒一輪，逐輪寫會把真正的
		// 狀態變化淹掉——那件事由 next_attempt_at 與 unknown_since 表達。
		return nil
	}
	if u.audit == nil {
		return nil
	}
	return appendStatusHistory(ctx, u.audit, subjectOperation, id, from, to, recheckReasonCode, "", now)
}

// recheckReasonCode 是 INV.rc 軸的 invoice_status_history.reason_code。
const recheckReasonCode = "INV_RECHECK"

// recheckClaimReasonCode 是「進入重查迴圈」那一步的 reason_code。
//
// 與 recheckReasonCode 分開是為了讓 A10 稽核分得出兩件事：**誰把它撿起來**
// （claim）與 **對帳的結論是什麼**（applyDecision）。同一個碼會讓兩者在畫面上
// 長得一樣，而它們對 ADM 的意義完全不同。
const recheckClaimReasonCode = "INV_RECHECK_CLAIMED"

// recordClaimTransition 補上 **claim 那一步**的 I7 留痕（J20／J22／J27）。
//
// 🔴 為什麼原本沒有、而它是真的缺口：`ClaimRecheck` 是一句會改 status 的 CAS
// （M2 上的 J15／J20／J22／J27，四條都在轉換表裡），但 `recordTransition` 只在
// `applyDecision` 被呼叫 ⇒ 只記**離開**重查迴圈那一步。dev 實測（2026-09-04 16:21，
// operation 7500300000000000002）留下的軌跡是：
//
//	FAILED_PERMANENT → CANCEL_REQUESTED        (ADMIN)
//	PROVIDER_STATUS_RECHECK → SUCCEEDED        (SYSTEM)   ← 上一列的 to 對不上這一列的 from
//
// 那正是本包 transition.go 檔頭引為病徵的形狀——「中間兩步整段消失，照它讀這筆 job
// 是憑空變成已取消的」。#3109 補了出邊那一半，入邊這一半還在。
//
// 🔴 **J15（WAITING_CONFIRMATION → PROVIDER_STATUS_RECHECK）刻意不記**，理由與
// `fsm.TransitionSelfLoop` 逐字相同：它與 J31（回頭那一步）構成 30 秒一輪的輪詢對，
// 一筆卡住的 operation 24 小時內會產生上千列，把真正的狀態變化淹掉。
// 「這一輪也沒有結果」是 next_attempt_at 與 unknown_since 在表達的事，不是歷程。
// 另外三條的退避是小時到天級（J20 ＋6h／+24h／+7d、J27 退避週期、J22 一次性的
// 強制最終查詢），逐條留痕的量可忽略，而它們正好是 ADM 會去讀軌跡的那幾種案件。
//
// ⚠️ 呼叫端必須在 **CAS 成功之後**才呼叫（rows>0）：CAS 失敗時 `op.Status` 是快照值
// 不是真實值，此時留痕會寫進一條沒有發生過的轉移（`transition.go` 的同一條紅字）。
func (u *Recheck) recordClaimTransition(ctx context.Context, id int64, from string, now time.Time) error {
	if from == codemeta.InvoiceOperationStatusWaitingConfirmation.Value {
		return nil // J15：輪詢對的一半，見上方紅字。
	}
	to := codemeta.InvoiceOperationStatusProviderStatusRecheck.Value
	kind, err := fsm.CheckOperation(from, to)
	if err != nil {
		u.log.Error("invoice.rc.illegal_claim_transition",
			zap.Int64("operation_id", id), zap.String("from", from), zap.Error(err))
		return err
	}
	if kind == fsm.TransitionSelfLoop {
		return nil // J16：租約到期後被重新 Claim，from == to。
	}
	if u.audit == nil {
		return nil
	}
	return appendStatusHistory(ctx, u.audit, subjectOperation, id, from, to,
		recheckClaimReasonCode, "", now)
}

// uploadTerminalStatuses 是「不再需要覆查」的 provider_upload_status（§5.2.3 第 3 組）。
//
// 🔴 候選集合用**正面枚舉**「還在飛」的值（NULL／1／2／3／31／32），本集合是它的
// 補集裡確定終態的那幾個：99 完成、91 與 -1 是失敗終態。
// 用 `IS DISTINCT FROM 99` 當述詞會讓 91／-1 的列**永久滯留**在候選集合裡。
var uploadTerminalStatuses = map[int16]bool{99: true, 91: true, -1: true}

// uploadRecheckMaxAttempts 是覆查次數上限（db/queries/invoice_documents.sql 的
// ClaimUploadRecheck 逐字：「次數用盡（>= 48）的處置是 upload_recheck_next_at = NULL
// ＋ 進 ADM 佇列，辨識鍵＝次數 ≥ 48 且租約欄為 NULL」）。
//
// ⚠️ 那句話寫在 SQL 註解裡，而**沒有任何程式碼實作它**（#3014，2026-09-02 修）：
// uploadNextAt 唯一的出邊是「拿得到 status 且為終態」，於是 status 永遠拿不到的那一列
// 就永遠留在候選集合裡。dev 實測 attempts 累到 29 仍在排程，而對 AMEGO 的呼叫有 92%
// 是這個迴圈、真正的開票只佔 0.4%。
const uploadRecheckMaxAttempts = 48

// uploadNextAt 算下一次覆查時點；回 nil ＝ 離開候選集合。
//
// 🔴 退避基準取**時間戳**（發票開立時間）而非 upload_recheck_attempts：
// 與 §4.3.3 前置 ① 同一條紀律——計數器在「claim 成功但副本崩潰」之後會失真，
// 時間戳不會。而且 RecordUploadRecheckResult 的 SQL **不遞增** attempts，
// 拿它當基準會讓退避永遠停在第一階。
//
// ⚠️ **上限用 attempts 而退避用時間戳，兩者刻意不同源。** 這不是不一致：
// 退避問「多久查一次」——用計數器會在崩潰後失真而查得過密；
// 上限問「查過幾次了」——用時間戳會讓一列停擺的文件光靠時間流逝就被放棄。
// attempts 由 ClaimUploadRecheck 就地遞增（claim 成功才 +1），失真的方向是**少算**，
// 對「上限」而言那是安全的一側。
//
// probeErr 非 nil 且為 PERMANENT ⇒ 立刻停止：那類錯誤重試 48 次也不會變
// （dev 實測 code=71「查無資料」連續 29 次，class 每一次都是 PERMANENT）。
// 停下來不是放棄，是**把它交給人**：next_at = NULL 之後這一列不再打 AMEGO，
// 而「已開立卻查不到上傳狀態」本來就需要有人看一眼。
func uploadNextAt(doc domain.Document, status *int16, probeErr error, now time.Time) *time.Time {
	if status != nil && uploadTerminalStatuses[*status] {
		return nil // 已終態：不再排下一次。
	}
	if isPermanentProbeError(probeErr) {
		return nil
	}
	// 🔴 attempts 已由本輪的 ClaimUploadRecheck 遞增，而 doc 是 claim 之前讀到的
	// ⇒ 這裡要用 `+1` 才是「含本輪」的次數。少了它，實際會多跑一輪。
	if int(doc.UploadRecheckAttempts)+1 >= uploadRecheckMaxAttempts {
		return nil
	}
	base := now
	if doc.ProviderIssuedAt != nil {
		base = *doc.ProviderIssuedAt
	}
	next := now.Add(recheckBackoff(now.Sub(base)))
	return &next
}

// isPermanentProbeError 判斷這次 probe 失敗是不是「重試不會改善」。
//
// 只認 amego.APIError 的 Class：非 AMEGO 來源的錯誤（憑證解密失敗、DB 讀不到帳戶）
// 一律當成暫時性而繼續排程——那類錯誤與「AMEGO 說這張票查無資料」是兩件事，
// 把它們一起停掉會讓一次金鑰輪替就永久終止全部覆查。
func isPermanentProbeError(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *amego.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	return apiErr.Class == amego.ClassPermanent
}

// recheckUploads 是候選集合②：已開立文件的上傳狀態輪詢。
//
// 🔴 候選述詞在 SQL 側用**正面枚舉**「還在飛」的 provider_upload_status
// （NULL／1／2／3／31／32）——`91` 與 `-1` 是終態、`99` 是完成，都不在候選集合內。
// 用 `IS DISTINCT FROM 99` 會讓終態的列永久滯留（§5.2.3 第 3 組）。
func (u *Recheck) recheckUploads(ctx context.Context) error {
	due, err := u.docs.FetchDueUploadRechecks(ctx, u.now(), recheckBatch)
	if err != nil {
		return err
	}
	var claimed, updated int
	for _, doc := range due {
		at := u.now()
		ok, err := u.docs.ClaimUploadRecheck(ctx, doc.ID, at, at.Add(recheckUploadLease))
		if err != nil {
			u.log.Error("invoice.rc.upload_claim_failed", zap.Int64("document_id", doc.ID), zap.Error(err))
			continue
		}
		if !ok {
			continue
		}
		claimed++
		status, probeErr := u.probeUploadStatus(ctx, doc)
		if probeErr != nil {
			// 查不到就不寫狀態；要不要再排下一次交給 uploadNextAt 判
			// （PERMANENT 立刻停、次數用盡也停，其餘退避後重來）。
			u.log.Warn("invoice.rc.upload_probe_failed",
				zap.Int64("document_id", doc.ID),
				zap.Int16("attempts", doc.UploadRecheckAttempts),
				zap.Bool("permanent", isPermanentProbeError(probeErr)),
				zap.Error(probeErr))
		}
		nextAt := uploadNextAt(doc, status, probeErr, u.now())
		if nextAt == nil && status == nil {
			// 停在「不知道上傳狀態」⇒ 這一列需要人看。Error 級是刻意的：
			// 它的意思是「一張已開立的發票，我方永遠不會知道它有沒有上傳到財政部」，
			// 而那是 5 年保存義務的一部分，不是一次可以忽略的查詢失敗。
			u.log.Error("invoice.rc.upload_recheck_stopped",
				zap.Int64("document_id", doc.ID),
				zap.Int16("attempts", doc.UploadRecheckAttempts),
				zap.Bool("permanent", isPermanentProbeError(probeErr)),
				zap.String("impact", "provider_upload_status 停留在 NULL，需 ADM 人工確認"),
				zap.Error(probeErr))
		}
		if err := u.docs.RecordUploadRecheckResult(ctx, doc.ID, status, u.now(), nextAt); err != nil {
			u.log.Error("invoice.rc.upload_record_failed", zap.Int64("document_id", doc.ID), zap.Error(err))
			continue
		}
		updated++
	}
	u.log.Info("invoice.rc.uploads_scanned",
		zap.Int("due", len(due)), zap.Int("claimed", claimed), zap.Int("updated", updated))
	return nil
}

// probeUploadStatus 打**一次** invoice_query 取回上傳狀態（前置 ③）。
func (u *Recheck) probeUploadStatus(ctx context.Context, doc domain.Document) (*int16, error) {
	acct, err := u.accounts.GetByIssuer(ctx, doc.IssuerID, derefOr(doc.Provider))
	if err != nil {
		return nil, err
	}
	var detail *amego.InvoiceDetail
	var queryErr error
	if err := u.accounts.WithDecryptedCredential(ctx, acct.ID, func(providerTaxID, appKey string) error {
		detail, queryErr = u.amego.QueryInvoiceByOrderID(ctx,
			amego.Credential{IssuerID: doc.IssuerID, Invoice: providerTaxID, AppKey: appKey},
			derefOr(doc.SourceRef))
		return nil
	}); err != nil {
		return nil, err
	}
	if queryErr != nil || detail == nil {
		return nil, queryErr
	}
	s := int16(detail.InvoiceStatus)
	return &s, nil
}
