package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/database"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/go-pkg/timeutil"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/eventwire"
	"go.uber.org/zap"
)

// 本檔是 A05／A06 發票工作佇列的**寫入面**（§6.1.14 端點 52 安全重試／53 受控取消／
// 56 代錄人工處理結果）。讀取面（端點 50／51）在 query_read.go。
//
// 五條貫穿全檔的約束：
//
//	① **allowed-from 只有一份，住在 SQL 的 WHERE**（db/queries/invoice_operations.sql）。
//	   本層先讀一次 operation 是為了回**可分辨的碼**（9340／9341／9342／9343／9349），
//	   不是正確性防線——兩個分頁同時送出時應用層的 if 會各自讀到舊值。真正的守衛是
//	   那句 CAS，而 **rows=0 一律回明確業務碼，絕不當成功往下走**。
//	② **一次動作 ＝ 一個交易**：operation CAS ＋（可能的）document CAS ＋ 狀態歷程 ＋
//	   ADM 稽核 ＋（端點 56 的）outbox 必須同進同退。少了交易會出現「狀態轉了但沒有人
//	   知道是誰轉的」，而 invoice_admin_audit_logs 是 5 年保存義務（§9.5）。
//	③ **稽核的 before/after 只放狀態相關欄位**。那張表保存 5 年，把統編／姓名／email
//	   複製進去等於憑空造一個五年期的 PII 倉庫，而稽核要回答的只是「誰把它從什麼狀態
//	   改成什麼狀態」。
//	④ **operation 與 document 是兩列、各自遞增 version**。ADM 帶進來的 expectedVersion
//	   是 §6.1 端點 51 給的 **job** 版本（AdminInvoiceJobDetail.job.version），不是
//	   document 的；document 的 CAS 用同一交易內剛讀到的值。
//	⑤ **`Idempotency-Key` 落進稽核列的 request_id**。uq_invadmaudit_client_request
//	   （actor_account_id, action, request_id）讓同一位 ADM 對同一動作重送同一把鍵
//	   只留一列稽核——那是本服務目前唯一真正生效的請求級冪等點。

const (
	// adminReasonMaxRunes 是 ADM 理由欄的長度上限，與稅務側同值（58A 的 maxLength=500）。
	//
	// 在這裡擋而不是讓 DB 擋：reason_detail 是 text 欄，DB 只擋空字串（chk_invadmaudit_reason）
	// 不擋長度 ⇒ 少了本檢查，前端的 maxLength 是唯一防線，而那是 client-side 的。
	adminReasonMaxRunes = 500

	// adminNoteMaxRunes 是端點 56 備註欄的上限（同上理由）。
	adminNoteMaxRunes = 500

	// invoice_status_history.reason_code 的工作佇列軸。它們不是 CodeMeta 碼值——
	// 該欄是自由的 varchar(48)，而 CodeMeta 沒有「歷程原因」這張碼表。
	jobReasonAdminRetry          = "ADMIN_JOB_RETRY"
	jobReasonAdminCancel         = "ADMIN_JOB_CANCEL"
	jobReasonAdminFundingDecided = "ADMIN_FUNDING_DECIDED"
	jobReasonResolvedExternally  = "SELLER_SELF_ISSUE_REPORTED"
	jobReasonAdminDocResume      = "ADMIN_DOC_RESUME"

	// invoice_admin_audit_logs.action。前綴與既有的 INVOICE_* 一致。
	jobAuditActionRetry        = "INVOICE_JOB_RETRY"
	jobAuditActionCancel       = "INVOICE_JOB_CANCEL"
	jobAuditActionManualResult = "INVOICE_JOB_MANUAL_RESULT"
	// 端點 73（人工出口 → 自動線，以 document 為鍵）。與 jobAuditActionRetry 分開：
	// 那一支的 resource 是 JOB，這一支按下去時**還沒有 job**，resource 只能是 DOCUMENT。
	// 混用同一個 action 會讓稽核查詢分不出「重試既有作業」與「補建缺席的作業」。
	docAuditActionResume = "INVOICE_DOCUMENT_RESUME"

	// invoice_admin_audit_logs.resource_type ＝ proto AdminAuditLog.target_kind 的 JOB。
	jobResourceType = "JOB"
	// docResourceType 是同一張表的 DOCUMENT 軸（端點 57 用）。
	docResourceType = "DOCUMENT"

	// invoice_status_history.actor_type 的 ADM 值（chk_invhist_actor 五值之一）。
	jobActorAdmin = "ADMIN"

	// outbox aggregate_type 的文件軸。
	outboxAggregateDocument = "DOCUMENT"

	// 端點 52 的 fundingDecision 值域（§6.1.14 的裁決表，兩值不多不少）。
	//
	// ⚠️ **不是 CodeMeta 碼值**：`funded_by` 的三值（SELLER／PLATFORM／UNDETERMINED）是
	// coupon 側的事實，而這裡是 ADM 對「這筆平台出資折扣怎麼算」的裁決，值域只有兩個。
	// 兩者名字相近、語意不同，不得互相代入。
	fundingDecisionSellerFunded   = "SELLER_FUNDED"
	fundingDecisionPlatformFunded = "PLATFORM_FUNDED"
)

// externalInvoiceNoPattern 是賣家自行開立的發票號碼格式（§6.1 端點 56／62 逐字）。
//
// 🔴 DB 端另有 chk_invdoc_external_no_shape 守同一件事。兩道都要：DB 那道回的是 23514
// （對 ADM 是一句 500），本層這道才回得出「號碼格式不正確」。
var externalInvoiceNoPattern = regexp.MustCompile(`^[A-Z]{2}[0-9]{8}$`)

// 型別邊界錯誤（契約違規 ⇒ 呼叫端映成 InvalidArgument／400，不借業務碼）。
//
// 借一個近似的業務碼會讓前端把「你的字太長」顯示成別的意思；而沒有任何業務碼描述
// 「此案件需先裁決折扣資金來源」以外的這些格式問題。
var (
	ErrJobReasonRequired           = errors.New("invoice: reason is required")
	ErrJobReasonTooLong            = errors.New("invoice: reason exceeds 500 characters")
	ErrJobNoteTooLong              = errors.New("invoice: note exceeds 500 characters")
	ErrJobCancelReasonCodeInvalid  = errors.New("invoice: reason_code must be ADM_MANUAL for admin-initiated cancellation")
	ErrJobCancelDetailRequired     = errors.New("invoice: reason_detail is required")
	ErrJobFundingDecisionInvalid   = errors.New("invoice: funding_decision must be SELLER_FUNDED or PLATFORM_FUNDED")
	ErrJobFundingDecisionMissing   = errors.New("此案件需先裁決折扣資金來源（funding_decision）")
	ErrExternalInvoiceNoInvalid    = errors.New("invoice: invoice_no must match ^[A-Z]{2}[0-9]{8}$")
	ErrExternalInvoiceDateInvalid  = errors.New("invoice: invoice_date must be YYYY-MM-DD")
	ErrExternalInvoiceDateTooEarly = errors.New("invoice: invoice_date must not be earlier than the payment date")
	ErrExternalTotalAmountInvalid  = errors.New("invoice: total_amount must be a non-negative integer")
)

// AdminJob 是 A05／A06 的寫入 usecase。
//
// 它與 Query 分開的理由同 Complaint／TaxProfile：只有它需要 TxManager 與 outbox。
// 把寫入掛進 Query，「唯讀查詢面」那句話就變成假的。
// UnlockEvidenceFinalizer 是 A4 取消證據的換發面（#3108）。
//
// 介面由消費端宣告：usecase 不 import client/media（那會形成
// usecase → client → mq → usecase 的循環，見 client/di.go 的紅字）。
type UnlockEvidenceFinalizer interface {
	FinalizeUnlockEvidence(ctx context.Context, stagedToken string, operationID int64) (string, error)
	// ReadUnlockEvidence 讀回證據檔（#3108 第 4 件）。
	//
	// 🔴 覆核者看得到檔案本身，雙人確認才不是儀式——J28 的註解逐字：
	// 「沒有它，第二個人是在對一份看不見證據的請求按同意」。
	ReadUnlockEvidence(ctx context.Context, objectKey string, operationID int64) ([]byte, string, string, error)
}

type AdminJob struct {
	tx      database.TxManager
	ops     domain.OperationRepository
	docs    domain.DocumentRepository
	issuers domain.IssuerRepository
	audit   domain.AuditRepository
	outbox  domain.OutboxRepository
	// unlocks 是 A4 人工出口雙人解鎖（§6.1.14 端點 54／55）的存取面。它掛在 AdminJob
	// 而不是獨立 usecase，因為端點 55 的第二段就是 job 的 J29 轉移——兩者必須同一交易。
	unlocks domain.ManualUnlockRepository
	// unlockEvidence 把 ADM 上傳的證據檔（staged token）換成 objectKey（#3108）。
	// **可以是 nil**：證據檔是選填的，wiring 缺它時端點 54 只是不接受 token，
	// 其餘路徑照常——不讓一個選填功能變成整條 A4 的硬依賴。
	unlockEvidence UnlockEvidenceFinalizer
	// adjustments 是 G16／G19 的連動面（#3073）：ADM 對一筆 VOID／ALLOWANCE operation
	// 按端點 52／53 時，它服務的 adjustment 必須在**同一事務內**跟著動。
	// 少了 G16，重試會必然再失敗——operation 回 READY 而 adjustment 還在
	// FAILED_PERMANENT ⇒ 下一輪 claim 的 MarkSending（allowed-from = PENDING）rows=0。
	adjustments domain.AdjustmentRepository
	// corrections 是 R09 的連動面（#3089 A 組第 1 項）：ADM 對一筆 REISSUE operation
	// 按端點 52 時，它服務的付款後修改請求必須在**同一事務內**跟著回
	// VOID_REISSUE_IN_PROGRESS。
	//
	// 🔴 少了這一句，correction 會**永遠**停在 REISSUE_FAILED —— 而且是無聲的：
	// 重開票會照樣送出、照樣可能成功，只有那一列不動、reissued_document_id 永遠 NULL。
	// dev 上曾有 3 列因此卡了 1–2 天，而 invoice_correction_requests 在 monitoring/
	// 全域零命中 ⇒ 沒有任何告警看得到它。
	corrections domain.CorrectionRequestRepository
	// issueCfg 只用一格：補建 ISSUE operation 時的 max_attempts 初值（端點 73）。
	// 沿用開票主鏈的同一份設定而不是另開一個 ADM 專屬值——兩份設定必然漂移，
	// 而 worker 讀的是**列上的值**，漂移的後果是同一條線上兩種忍耐度。
	issueCfg IssuanceConfig
	// newID 產生 unlock_id（TSID，對外暴露 ⇒ §7 要求 TSID 而不是 bigserial）。
	newID func() int64

	now func() time.Time
	log *zap.Logger
}

// NewAdminJob 組出 A05／A06 的寫入 usecase。
func NewAdminJob(
	tx database.TxManager,
	ops domain.OperationRepository,
	docs domain.DocumentRepository,
	issuers domain.IssuerRepository,
	audit domain.AuditRepository,
	outbox domain.OutboxRepository,
	unlocks domain.ManualUnlockRepository,
	adjustments domain.AdjustmentRepository,
	corrections domain.CorrectionRequestRepository,
	unlockEvidence UnlockEvidenceFinalizer,
	issueCfg IssuanceConfig,
	newID func() int64,
	log *zap.Logger,
) *AdminJob {
	return &AdminJob{
		tx: tx, ops: ops, docs: docs, issuers: issuers, audit: audit, outbox: outbox,
		unlocks: unlocks, adjustments: adjustments, unlockEvidence: unlockEvidence,
		corrections: corrections, issueCfg: issueCfg, newID: newID,
		now: func() time.Time { return time.Now().UTC() },
		log: orNop(log),
	}
}

// WithClock 注入時間來源（測試用）。
func (u *AdminJob) WithClock(now func() time.Time) *AdminJob {
	if now != nil {
		u.now = now
	}
	return u
}

// JobMutationResult 是三支端點共通的回應（proto InvoiceJobMutationData）。
//
// Blocked=true 只有端點 53 會設：那是 **HTTP 200 ＋ 9342 ＋ preserveData**，
// 不是錯誤（§6.1.14）。前端一律讀 code 判斷是否被回絕，不看 status。
type JobMutationResult struct {
	JobID      int64
	Status     string
	Version    int64
	AuditLogID int64

	Blocked             bool
	LastProviderCheckAt *time.Time
	ProviderRequestID   string
	NextStep            string
}

// ---------------------------------------------------------------------------
// 端點 52：安全重試
// ---------------------------------------------------------------------------

// RetryJobInput 是端點 52 的輸入。
type RetryJobInput struct {
	ActorAccountID  int64
	JobID           int64
	Reason          string
	ExpectedVersion int64
	IdempotencyKey  string
	// FundingDecision 只在「document 為 AWAITING_MANUAL ＋ manual_reason_code =
	// PLATFORM_FUNDED_DISCOUNT」時必填；其餘情況帶了也會被值域驗證，但沒有可套用的狀態。
	FundingDecision string
}

// Retry 執行 ADM 的安全重試（端點 52）。
//
// 🔴 **可重試的來源狀態 ＝ 兩支 CAS 述詞的聯集**，逐字對應 §7.3.2 的三條 ADM 邊：
//
//	J18 FAILED_PERMANENT       → READY（AdminRetryAfterFix，attempt_count 不歸零）
//	J19 FAILED_RETRY_EXHAUSTED → READY（AdminResetAttempts，attempt_count 歸零）
//	J34 SELLER_ACTION_REQUIRED → READY（同上）
//
// WAITING_CONFIRMATION／FAILED_UNKNOWN 一律不可重試——那正是「結果不明」的定義，
// 重送可能開出第二張發票（§6.1.14）。
//
// 🔴 **document 若停在 AWAITING_MANUAL，必須在同一交易內回 PENDING**。少了這一步，
// operation 會被推成 READY 而 document 還在 AWAITING_MANUAL ⇒ worker 撈到它、
// MarkIssuing 的 allowed-from（PENDING／ISSUE_FAILED）不成立 ⇒ rows=0 靜默失敗，
// 那一筆從此卡在「READY 但永遠送不出去」而沒有任何告警。
func (u *AdminJob) Retry(ctx context.Context, in RetryJobInput) (JobMutationResult, error) {
	if err := requireAdminReason(in.Reason); err != nil {
		return JobMutationResult{}, err
	}
	if err := validateFundingDecision(in.FundingDecision); err != nil {
		return JobMutationResult{}, err
	}

	var out JobMutationResult
	err := u.tx.WithTx(ctx, func(ctx context.Context) error {
		op, err := u.ops.GetByID(ctx, in.JobID)
		if err != nil {
			return err // repository 已把查無映成 9340
		}
		if op.Version != in.ExpectedVersion {
			return jobBiz(bizcode.InvoiceJobVersionConflict)
		}
		doc, err := u.docs.GetByID(ctx, op.DocumentID)
		if err != nil {
			return err
		}
		now := u.now()

		// ① 折扣資金來源裁決（與重試同一交易、同一次稽核；拆成兩支端點會多一個 TOCTOU 窗口）。
		//
		// PLATFORM_FUNDED 一律拒絕重試，**不看 document 目前是不是 AWAITING_MANUAL**：
		// 那是 ADM 對這筆訂單性質的明確宣告，平台補貼的金額性質與金流回沖 Phase 1 都不存在，
		// 自動線算不出正確的銷售額。正確出口寫在 message 裡（端點 53 取消 ＋ 端點 57 手動開票）。
		if in.FundingDecision == fundingDecisionPlatformFunded {
			return pkgerrors.NewBusiness(bizcode.InvoiceJobNotRetryable,
				"本案屬平台出資折扣，無法自動開票；請改以「受控取消」結束本作業後，再用「手動開立發票」補開")
		}
		if needsFundingDecision(doc) && in.FundingDecision != fundingDecisionSellerFunded {
			return ErrJobFundingDecisionMissing
		}

		// ② document 回自動線（M1 的 AWAITING_MANUAL → PENDING，trigger 逐字是
		//    「ADM 給出判定（funding_decision 等），回自動線」）。
		if doc.Status == codemeta.InvoiceDocumentStatusAwaitingManual.Value {
			ok, err := u.docs.ResumeFromManual(ctx, doc.ID, doc.Version, now)
			if err != nil {
				return err
			}
			if !ok {
				// document 在讀與寫之間被別人改走。對 ADM 而言就是「這個決策現在不適用」。
				return jobBiz(bizcode.InvoiceJobVersionConflict)
			}
			if err := u.appendHistory(ctx, subjectDocument, doc.ID,
				doc.Status, codemeta.InvoiceDocumentStatusPending.Value,
				jobReasonAdminFundingDecided, in.Reason, in.ActorAccountID, now); err != nil {
				return err
			}
		}

		// ③ operation 的重試邊。
		applied, err := u.applyRetry(ctx, op, now)
		if err != nil {
			return err
		}
		if !applied {
			return jobBiz(bizcode.InvoiceJobVersionConflict)
		}

		nextStatus := codemeta.InvoiceOperationStatusReady.Value
		if err := u.appendHistory(ctx, subjectOperation, op.ID, op.Status, nextStatus,
			jobReasonAdminRetry, in.Reason, in.ActorAccountID, now); err != nil {
			return err
		}
		// G16：這筆 job 若服務一個 adjustment，它也要回到可執行狀態。
		// 少了這一句，重試會**必然再失敗**（見 advanceAdjustmentForAdmin 的說明）。
		if err := u.advanceAdjustmentForAdmin(ctx, op, true, "", in.ActorAccountID, now); err != nil {
			return err
		}
		// R09：這筆 job 若是某個付款後修改請求的重開票，那一列也要跟著回
		// VOID_REISSUE_IN_PROGRESS。少了它，重開票成功之後 correction 仍停在
		// REISSUE_FAILED（#3089 A 組第 1 項）。
		if err := u.advanceCorrectionForAdmin(ctx, doc, in.ActorAccountID, now); err != nil {
			return err
		}

		auditID, err := u.appendAudit(ctx, adminAuditInput{
			ActorAccountID: in.ActorAccountID,
			Action:         jobAuditActionRetry,
			ResourceType:   jobResourceType,
			ResourceID:     op.ID,
			Before:         jobAuditState(op.Status, map[string]string{"documentStatus": doc.Status}),
			After: jobAuditState(nextStatus, map[string]string{
				"documentStatus":  resumedDocumentStatus(doc),
				"fundingDecision": in.FundingDecision,
			}),
			ReasonCode:     jobReasonAdminRetry,
			ReasonDetail:   in.Reason,
			IdempotencyKey: in.IdempotencyKey,
			Now:            now,
		})
		if err != nil {
			return err
		}

		// 重讀而不是就地 +1：version 的新值由 SQL 算出，Go 端拼一份等於維護第二套算式。
		updated, err := u.ops.GetByID(ctx, op.ID)
		if err != nil {
			return err
		}
		out = JobMutationResult{
			JobID: updated.ID, Status: updated.Status, Version: updated.Version, AuditLogID: auditID,
		}
		return nil
	})
	if err != nil {
		return JobMutationResult{}, err
	}
	return out, nil
}

// applyRetry 依來源狀態選一條 ADM 重試邊，並回 CAS 的 rows>0。
//
// 🔴 不在允許集合內時回 9341 而不是靜默成功：ADM 按下去什麼事都不會發生，
// 而 rows=0 是**沒有訊號**的（§7.3.0 規則 4）。
// advanceCorrectionForAdmin 是 R09 的連動（REISSUE_FAILED → VOID_REISSUE_IN_PROGRESS）。
//
// 連結路徑是 op.DocumentID → doc.OriginalDocumentID → correction：invoice_operations
// **沒有** correction 外鍵（REISSUE operation 建立時 AdjustmentID 留空），而 doc 在
// Retry 裡已經讀好了，不必多一次查詢。判準與 markReissueFailedIfAny 同一個：
// doc.OriginalDocumentID == nil ⇒ 這不是重開票，什麼都不做。
//
// 🔴 rows=0 **不是錯誤**：絕大多數重試的 document 上根本沒有停在 REISSUE_FAILED 的
// 請求（連 OriginalDocumentID 都是 nil）。回 error 會讓一個合法的 ADM 操作
// 因為別的表沒有對應列而失敗。
//
// ⚠️ CAS 用 **correction 自己的 version**（同一交易內現讀），不是 ADM 帶進來的
// operation version：兩者是不同的樂觀鎖，混用會讓一方的版本前進抹掉另一方的保護
// （與 advanceAdjustmentForAdmin 同一個理由）。
func (u *AdminJob) advanceCorrectionForAdmin(
	ctx context.Context, doc domain.Document, actorAccountID int64, now time.Time,
) error {
	if u.corrections == nil || doc.OriginalDocumentID == nil {
		return nil
	}
	corr, err := u.corrections.GetReissueFailedByDocument(ctx, *doc.OriginalDocumentID)
	if err != nil {
		if isBizCode(err, bizcode.InvoiceCorrectionNotAllowed) {
			return nil // 這張原始憑證上沒有停在 REISSUE_FAILED 的請求
		}
		return err
	}
	ok, err := u.corrections.RetryReissue(ctx, corr.ID, corr.Version, now)
	if err != nil {
		return err
	}
	if !ok {
		u.log.Info("invoice.admin_job.correction_not_advanced",
			zap.Int64("correction_id", corr.ID),
			zap.String("status", corr.Status))
		return nil
	}
	return u.appendHistory(ctx, subjectCorrectionRequest, corr.ID,
		corr.Status, codemeta.InvoiceCorrectionStatusVoidReissueInProgress.Value,
		jobReasonAdminRetry, "", actorAccountID, now)
}

// adminRetryKind 是「ADM 按下重試時該走哪一支 CAS」。
type adminRetryKind int

const (
	adminRetryNone adminRetryKind = iota
	// adminRetryAfterFix ＝ J18（AdminRetryInvoiceOperationAfterFix）。
	adminRetryAfterFix
	// adminRetryResetAttempts ＝ J19 ＋ J34（AdminResetInvoiceOperationAttempts）。
	adminRetryResetAttempts
)

// adminRetryKindOf 是「這件作業可不可以重試」的**唯一**判準。
//
// 🔴 **讀端（CanRetryOperation → proto 的 can_retry）與寫端（applyRetry）必須走同一支。**
// #3089 第 8 項就是它們分家的後果：讀端只認 FAILED_PERMANENT、寫端接受三個 ⇒
// dev 實測 FAILED_RETRY_EXHAUSTED／SELLER_ACTION_REQUIRED 下端點 52 實打回 200
// 且 status→READY，而 ADM 畫面上那顆「安全重試」鍵**根本不渲染**（前端是條件渲染
// 不是 disabled）⇒ 功能在，使用者按不到。
//
// 那次分家還有第二層成因：讀端的註解自稱「SSOT 是**兩支** CAS 述詞」，
// 而實際上有**三支**——它漏數了 AdminResetInvoiceOperationAttempts。
// **一份寫在註解裡的清單不是 SSOT**，所以這裡把它變成一支函式。
//
// allowed-from 的真正權威是 db/queries/invoice_operations.sql 的兩支 CAS 述詞：
//   - AdminRetryInvoiceOperationAfterFix     ← status = 'FAILED_PERMANENT'
//   - AdminResetInvoiceOperationAttempts     ← status IN (FAILED_RETRY_EXHAUSTED,
//     SELLER_ACTION_REQUIRED)
//
// 放寬這裡而 SQL 沒放寬，按鈕就會亮在一個 CAS 必定 rows=0 的狀態上——
// 而 rows=0 是**靜默**的（§7.3.0 規則 4），ADM 按下去什麼都不會發生也不會有錯誤訊息。
func adminRetryKindOf(status string) adminRetryKind {
	switch status {
	case codemeta.InvoiceOperationStatusFailedPermanent.Value:
		return adminRetryAfterFix
	case codemeta.InvoiceOperationStatusFailedRetryExhausted.Value,
		codemeta.InvoiceOperationStatusSellerActionRequired.Value:
		return adminRetryResetAttempts
	}
	return adminRetryNone
}

func (u *AdminJob) applyRetry(ctx context.Context, op domain.Operation, now time.Time) (bool, error) {
	switch adminRetryKindOf(op.Status) {
	case adminRetryAfterFix:
		// J18。request_snapshot_hash 原值帶回：Phase 1 沒有任何路徑會為既有 document
		// 重新推導 IssueDraft（唯一產生點是 Issuance.buildPlan）⇒ 這裡改不出新雜湊。
		// 若在此強制「新雜湊 ≠ 舊雜湊」，FAILED_PERMANENT 會變成永遠不可重試。
		// 缺口登記在交付回報。
		return u.ops.AdminRetryAfterFix(ctx, op.ID, op.Version, op.RequestSnapshotHash, now)
	case adminRetryResetAttempts:
		// J19 ＋ J34：資料沒問題（是重試預算用完／外部前置沒到位）⇒ attempt_count 歸零。
		return u.ops.AdminResetAttempts(ctx, op.ID, op.Version, now)
	}
	return false, jobBiz(bizcode.InvoiceJobNotRetryable)
}

// advanceAdjustmentForAdmin 是 G16／G19 的連動（§7.3.7 逐字「同一事務內連動」）。
//
// 🔴 rows=0 **不是錯誤**：兩句的 allowed-from 都比 operation 那邊窄
// （G16 只認 FAILED_PERMANENT、G19 不認 SENDING），而 ADM 的意圖是對 operation 操作
// ——adjustment 不在那一格時，正確結果就是「不動它」。回 error 會讓一個合法的
// ADM 操作因為側寫狀態不符而失敗。
//
// ⚠️ CAS 用 **adjustment 自己的 version**（同一交易內現讀），不是 ADM 帶進來的
// operation version：兩者是不同的樂觀鎖，混用會讓一方的版本前進抹掉另一方的保護。
func (u *AdminJob) advanceAdjustmentForAdmin(
	ctx context.Context, op domain.Operation, retry bool, cancelReasonCode string,
	actorAccountID int64, now time.Time,
) error {
	if op.AdjustmentID == nil || u.adjustments == nil {
		return nil
	}
	adj, err := u.adjustments.GetByID(ctx, *op.AdjustmentID)
	if err != nil {
		return err
	}
	var ok bool
	var to string
	if retry {
		ok, err = u.adjustments.AdminRetry(ctx, adj.ID, adj.Version, now)
		to = codemeta.InvoiceAdjustmentStatusPending.Value
	} else {
		ok, err = u.adjustments.AdminCancel(ctx, adj.ID, adj.Version, cancelReasonCode, now)
		to = codemeta.InvoiceAdjustmentStatusCancelled.Value
	}
	if err != nil {
		return err
	}
	if !ok {
		u.log.Info("invoice.admin_job.adjustment_not_advanced",
			zap.Int64("adjustment_id", adj.ID), zap.Int64("operation_id", op.ID),
			zap.String("adjustment_status", adj.Status), zap.String("want", to))
		return nil
	}
	// 🔴 reason_code 必須跟著 retry 分流，且 actor 要帶進來。
	// 原本這兩格寫死 jobReasonAdminCancel 與 0，dev 實測（第 3 輪 ALW-06）的結果是：
	// ADM 按「重試」之後，operation 那列正確記成
	// `FAILED_PERMANENT → READY / ADMIN_JOB_RETRY / actor=2600000022`，
	// 而同一交易的 adjustment 那列記成
	// `FAILED_PERMANENT → PENDING / ADMIN_JOB_CANCEL / actor=0`
	// —— 同一個動作在稽核軌跡上被記成兩件不同的事，而且操作者不見了。
	//
	// actor 寫 0 比留白更糟：那個欄位是 *int64，0 會實際落庫成 0 而不是 NULL，
	// 讀的人分不出「系統做的」與「某個 id 為 0 的人做的」。#3048 修的正是
	// actor_account_id 為空，而這條路徑是同一輪新增的 code、漏了同一格。
	reason := jobReasonAdminRetry
	if !retry {
		reason = jobReasonAdminCancel
	}
	return u.appendHistory(ctx, subjectAdjustment, adj.ID, adj.Status, to,
		reason, cancelReasonCode, actorAccountID, now)
}

// needsFundingDecision 回報這件 job 是不是「平台出資折扣待裁決」那一格。
func needsFundingDecision(doc domain.Document) bool {
	return NeedsFundingDecision(doc.Status, derefString(doc.ManualReasonCode))
}

// NeedsFundingDecision 是同一個述詞的字串版，給只拿得到那兩欄的讀取視圖用
// （`domain.OperationView` 帶 DocumentStatus 與 DocumentManualReasonCode）。
//
// 🔴 **兩個入口一份定義**：寫入端（端點 52 重試前的閘門）與讀取端
// （A05 的 needs_funding_decision 旗標）必須用同一個述詞。分成兩份的後果是
// 清單說「不需要裁決」而重試時被擋，或反過來——兩者都沒有錯誤訊息，
// 只是按鈕點了沒反應（#3031）。
func NeedsFundingDecision(documentStatus, manualReasonCode string) bool {
	return documentStatus == codemeta.InvoiceDocumentStatusAwaitingManual.Value &&
		manualReasonCode == codemeta.InvoiceManualReasonPlatformFundedDiscount.Value
}

// resumedDocumentStatus 回本次重試之後 document 應有的狀態（稽核 after_state 用）。
func resumedDocumentStatus(doc domain.Document) string {
	if doc.Status == codemeta.InvoiceDocumentStatusAwaitingManual.Value {
		return codemeta.InvoiceDocumentStatusPending.Value
	}
	return doc.Status
}

// ---------------------------------------------------------------------------
// 端點 73：人工出口 → 自動線（以 **document** 為鍵）
// ---------------------------------------------------------------------------

// ErrDocumentResumeNotAllowed 是「這份文件現在不能送回自動線」的可分辨錯誤。
//
// 借 9341（不可安全重試）而不新增碼值：對 ADM 而言兩者是同一件事（「這個動作現在
// 不適用」），而 message 說得出差別。新增一個碼值要動 CodeMeta ＋ 四份前端拷貝，
// 收益只是讓兩句話有不同的數字。
var (
	// ErrDocumentNotAwaitingManual ＝ 文件不在人工出口。
	ErrDocumentNotAwaitingManual = "這份文件目前不在「待人工處理」狀態，無法送回自動線"
	// ErrDocumentAlreadyHasJob ＝ 文件已經有開票作業 ⇒ 正確入口是端點 52。
	ErrDocumentAlreadyHasJob = "這份文件已經有開票作業，請改從「發票工作」頁面執行安全重試"
)

// ResumeDocumentInput 是端點 73 的輸入。
type ResumeDocumentInput struct {
	ActorAccountID int64
	DocumentID     int64
	Reason         string
	// ExpectedVersion 是 **document** 自己的版本，不是 job 的（這條路徑上還沒有 job）。
	ExpectedVersion int64
	IdempotencyKey  string
	// FundingDecision 的必填條件與端點 52 同一個述詞（NeedsFundingDecision）。
	FundingDecision string
}

// CanResumeDocumentFromManual 是「ADM 可不可以把這份文件送回自動線」的**唯一**判準。
//
// 🔴 讀端（端點 48 的 can_resume_from_manual）與寫端（本檔的 ResumeDocumentFromManual）
// 必須走同一支。#3031 與 #3089 第 8 項兩次都是讀寫端分家的後果：按鈕亮在一個 CAS 必定
// rows=0 的狀態上，而 rows=0 是**靜默**的——ADM 按下去什麼都不會發生也沒有錯誤訊息。
//
// 兩個條件缺一不可：
//   - status = AWAITING_MANUAL —— ResumeInvoiceDocumentFromManual 的 allowed-from
//   - 零 operation —— 有 operation 的那些走端點 52（那條路徑另有 job 版本的樂觀鎖與
//     G16／R09 的連動；從這裡再建一列只會撞 uq_invop_operation_key 或多出一件孤兒作業）
func CanResumeDocumentFromManual(documentStatus string, operationCount int) bool {
	return documentStatus == codemeta.InvoiceDocumentStatusAwaitingManual.Value && operationCount == 0
}

// ResumeDocumentFromManual 把一份「停在人工出口而且沒有任何 operation」的文件送回自動線。
//
// 🔴 **這是那些文件唯一的入口。** 自動線落 AWAITING_MANUAL 時 operation 根本不建
// （§7.1.3 的 AM 節點明文，見 issuance.buildPlan 的 switch），而 M1 的三條出邊
// （端點 52 ResumeFromManual／53 CancelBeforeIssue／56 MarkResolvedExternally）
// 的呼叫端**全部以 operation id 為鍵** ⇒ 在本 RPC 之前，那些文件沒有任何角色推得動：
// ADM 的 A05 工作台是 job-first（INNER JOIN invoice_operations）根本看不到它們、
// 賣家的 62R 同樣 JOIN operation、賣家補簽收據授權也不回頭處理既有文件、
// 沒有任何掃描器碰它們（FetchApproachingStatutoryDeadlines 零呼叫端）。
// dev 實證：同賣家同金額，有 operation 的走完全程 ISSUED，零 operation 的 version 恆 1。
//
// 兩件事必須在**同一交易**內完成，少一件就退化成另一種卡死：
//
//	① M1：AWAITING_MANUAL → PENDING（帶 document 自己的 expectedVersion CAS）
//	② 補建那件缺席的 ISSUE／REISSUE operation 並推進 READY（J02）
//
// 只做 ① ⇒ document 回 PENDING 而沒有任何 worker 撈得到它（worker 的候選集合是
// invoice_operations，不是 invoice_documents）⇒ 從「卡在 AWAITING_MANUAL」變成
// 「卡在 PENDING」，一樣沒有告警。
// 只做 ② ⇒ operation READY 而 document 還在 AWAITING_MANUAL ⇒ MarkIssuing 的
// allowed-from（PENDING／ISSUE_FAILED）不成立 ⇒ rows=0 靜默失敗（與端點 52 的紅字同源）。
//
// ⚠️ **不在建立當下就建 operation**（那是被評估過並否決的修法）：建成 HELD 會被
// ReleaseExpiredInvoiceOperationHolds 自動放回 READY、建成 AWAITING_TRIGGER 會被
// PromoteDueInvoiceOperationTriggers 自動 promote ⇒ 兩者都讓 fail-closed 失效、
// AMEGO 照樣被打；而 M2 上沒有 CREATED → HELD 這條邊（J08 的起點是 READY／
// RETRY_SCHEDULED），硬寫會 rows=0 靜默失敗。**缺的從來不是「建列」，是「事後的入口」。**
func (u *AdminJob) ResumeDocumentFromManual(ctx context.Context, in ResumeDocumentInput) (JobMutationResult, error) {
	if err := requireAdminReason(in.Reason); err != nil {
		return JobMutationResult{}, err
	}
	if err := validateFundingDecision(in.FundingDecision); err != nil {
		return JobMutationResult{}, err
	}

	var out JobMutationResult
	err := u.tx.WithTx(ctx, func(ctx context.Context) error {
		doc, err := u.docs.GetByID(ctx, in.DocumentID)
		if err != nil {
			return err // repository 已把查無映成 9301
		}
		if doc.Version != in.ExpectedVersion {
			return jobBiz(bizcode.InvoiceJobVersionConflict)
		}
		existing, err := u.ops.ListByDocument(ctx, doc.ID)
		if err != nil {
			return err
		}
		if !CanResumeDocumentFromManual(doc.Status, len(existing)) {
			msg := ErrDocumentNotAwaitingManual
			if len(existing) > 0 {
				msg = ErrDocumentAlreadyHasJob
			}
			return pkgerrors.NewBusiness(bizcode.InvoiceJobNotRetryable, msg)
		}
		now := u.now()

		// ① 折扣資金來源裁決。判準與端點 52 共用 needsFundingDecision——同一份 document
		// 從兩個入口進來不該得到不同的答案。
		if in.FundingDecision == fundingDecisionPlatformFunded {
			return pkgerrors.NewBusiness(bizcode.InvoiceJobNotRetryable,
				"本案屬平台出資折扣，無法自動開票；請改用「手動開立發票」補開")
		}
		if needsFundingDecision(doc) && in.FundingDecision != fundingDecisionSellerFunded {
			return ErrJobFundingDecisionMissing
		}

		// ② M1：AWAITING_MANUAL → PENDING。
		ok, err := u.docs.ResumeFromManual(ctx, doc.ID, doc.Version, now)
		if err != nil {
			return err
		}
		if !ok {
			// 讀與寫之間被別人改走。對 ADM 而言就是「這個決策現在不適用」。
			return jobBiz(bizcode.InvoiceJobVersionConflict)
		}
		if err := u.appendHistory(ctx, subjectDocument, doc.ID,
			doc.Status, codemeta.InvoiceDocumentStatusPending.Value,
			jobReasonAdminDocResume, in.Reason, in.ActorAccountID, now); err != nil {
			return err
		}

		// ③ 補建缺席的 operation 並推進 J02。
		//
		// request_snapshot_hash 落 "resume:<documentID>"：這條路徑上沒有 IssueDraft 可
		// 雜湊（draft 只在 issuance.buildPlan 產生，Phase 1 沒有任何路徑為既有 document
		// 重新推導它）。該欄不進任何述詞、不參與 CAS，只被 J18 讀來比對「有沒有人改過
		// 快照」——落一個誠實的識別字，比落一個假的雜湊好。與端點 57 的 "manual:<id>" 同形。
		opID, opStatus, _, err := createAndPromoteIssueOperation(ctx, u.ops, issueOperationSpec{
			OperationID:         u.newID(),
			DocumentID:          doc.ID,
			OriginalDocumentID:  doc.OriginalDocumentID,
			IssueAttemptSeq:     doc.IssueAttemptSeq,
			ProviderRequestNo:   derefString(doc.ProviderRequestNo),
			RequestSnapshotHash: "resume:" + strconv.FormatInt(doc.ID, 10),
			MaxAttempts:         u.issueCfg.MaxAttempts,
			NextAttemptAt:       now,
			StatutoryDeadlineAt: doc.StatutoryDeadlineAt,
			Now:                 now,
		}, func(ctx context.Context, operationID int64, from, to string) error {
			return u.appendHistory(ctx, subjectOperation, operationID, from, to,
				jobReasonAdminDocResume, in.Reason, in.ActorAccountID, now)
		})
		if err != nil {
			return err
		}

		auditID, err := u.appendAudit(ctx, adminAuditInput{
			ActorAccountID: in.ActorAccountID,
			Action:         docAuditActionResume,
			ResourceType:   docResourceType,
			ResourceID:     doc.ID,
			Before:         jobAuditState(doc.Status, map[string]string{"operationCount": "0"}),
			After: jobAuditState(codemeta.InvoiceDocumentStatusPending.Value, map[string]string{
				"jobId":           strconv.FormatInt(opID, 10),
				"jobStatus":       opStatus,
				"fundingDecision": in.FundingDecision,
			}),
			ReasonCode:     jobReasonAdminDocResume,
			ReasonDetail:   in.Reason,
			IdempotencyKey: in.IdempotencyKey,
			Now:            now,
		})
		if err != nil {
			return err
		}

		// 重讀而不是就地拼：version 的新值由 SQL 算出。
		created, err := u.ops.GetByID(ctx, opID)
		if err != nil {
			return err
		}
		out = JobMutationResult{
			JobID: created.ID, Status: created.Status, Version: created.Version, AuditLogID: auditID,
		}
		return nil
	})
	if err != nil {
		return JobMutationResult{}, err
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// 端點 53：受控取消
// ---------------------------------------------------------------------------

// CancelJobInput 是端點 53 的輸入。
type CancelJobInput struct {
	ActorAccountID int64
	JobID          int64
	// ReasonCode 的值域是 CodeMeta InvoiceCancelReason；ADM 從 A06 手動取消時**一律**
	// 送 ADM_MANUAL（空字串視同它），細分類寫進 ReasonDetail。其餘三值由系統路徑寫入
	// 同一欄，端點收下來會讓 A05 的「依取消原因分類統計」出現假資料。
	ReasonCode                   string
	ReasonDetail                 string
	AllowManualAfterCancellation bool
	ExpectedVersion              int64
	IdempotencyKey               string
}

// cancelUnknownStatuses 是「provider 端結果尚未確認」而必須回絕取消的來源狀態。
//
// §6.1.14 明列 WAITING_CONFIRMATION／FAILED_UNKNOWN 兩個。另外三個一併納入，理由是
// **它們描述的是同一件事實**：
//
//	SENDING                 —— 請求正在飛，結果連「有沒有送到」都還不知道
//	PROVIDER_STATUS_RECHECK —— INV.rc 正握著租約在查，結論還沒出來
//	CANCEL_BLOCKED_UNKNOWN  —— 上一次取消就是被「結果不明」擋下來的
//
// 少了它們，這三個狀態會落到 CAS（它們本來就不在 J21 的 allowed-from）而 rows=0，
// 回出一句「已被更新，請重新整理」——ADM 重新整理一百次也不會變，而真正的下一步是等對帳。
var cancelUnknownStatuses = []string{
	codemeta.InvoiceOperationStatusWaitingConfirmation.Value,
	codemeta.InvoiceOperationStatusFailedUnknown.Value,
	codemeta.InvoiceOperationStatusSending.Value,
	codemeta.InvoiceOperationStatusProviderStatusRecheck.Value,
	codemeta.InvoiceOperationStatusCancelBlockedUnknown.Value,
}

// Cancel 執行 ADM 的受控取消（端點 53 ＝ J21）。
//
// 🔴 回絕取消（9342）**不是錯誤**：HTTP 200 ＋ preserveData，data 帶最後查證時間、
// providerRequestId 與人類可讀的下一步。ADM 前端一律讀 code 判斷是否被回絕。
func (u *AdminJob) Cancel(ctx context.Context, in CancelJobInput) (JobMutationResult, error) {
	reasonCode, err := normalizeAdminCancelReason(in.ReasonCode)
	if err != nil {
		return JobMutationResult{}, err
	}
	if err := requireAdminCancelDetail(in.ReasonDetail); err != nil {
		return JobMutationResult{}, err
	}

	var out JobMutationResult
	err = u.tx.WithTx(ctx, func(ctx context.Context) error {
		op, err := u.ops.GetByID(ctx, in.JobID)
		if err != nil {
			return err
		}
		if op.Version != in.ExpectedVersion {
			return jobBiz(bizcode.InvoiceJobVersionConflict)
		}
		now := u.now()

		if containsString(cancelUnknownStatuses, op.Status) {
			// 9342：不寫任何東西、不留稽核（沒有發生任何狀態變更）。
			out = JobMutationResult{
				JobID: op.ID, Status: op.Status, Version: op.Version,
				Blocked:             true,
				LastProviderCheckAt: op.UnknownSince,
				ProviderRequestID:   op.ProviderRequestNo,
				NextStep:            "系統正在向服務商查證這筆作業是否已開立；查證完成前無法取消。若需人工介入，請走人工出口解鎖流程。",
			}
			// 留一行 INFO：ADM 按了取消卻被回絕，是「這一筆卡在對帳」的第一手訊號。
			// 它在 A06 上只是一句提示，而在 log 上才數得出「有多少筆卡在這裡」。
			u.logger().Info("invoice.admin.job.cancel.blocked",
				zap.Int64("job_id", op.ID), zap.String("status", op.Status))
			return nil
		}

		ok, err := u.ops.RequestCancel(ctx, domain.RequestOperationCancel{
			ID:              op.ID,
			ExpectedVersion: op.Version,
			ActorAccountID:  in.ActorAccountID,
			ReasonCode:      reasonCode,
			ReasonDetail:    in.ReasonDetail,
			Now:             now,
		})
		if err != nil {
			return err
		}
		if !ok {
			// allowed-from 不符或版本被搶先。兩者對 ADM 的下一步相同：重新整理再看。
			return jobBiz(bizcode.InvoiceJobVersionConflict)
		}

		nextStatus := codemeta.InvoiceOperationStatusCancelRequested.Value
		if err := u.appendHistory(ctx, subjectOperation, op.ID, op.Status, nextStatus,
			jobReasonAdminCancel, reasonCode+": "+in.ReasonDetail, in.ActorAccountID, now); err != nil {
			return err
		}
		// G19：還沒送出去的 adjustment 一起收掉，唯一鍵同時釋放
		// （uq_invadj_* 都排除 CANCELLED ⇒ 同一筆退款之後可以重新建列）。
		// 已在 SENDING 的**不動**：租約持有中代表可能正在打 AMEGO，
		// 那時取消會讓「我方帳上已取消、對方端已生效」。述詞在 SQL，rows=0 是正確結果。
		if err := u.advanceAdjustmentForAdmin(ctx, op, false, reasonCode, in.ActorAccountID, now); err != nil {
			return err
		}

		auditID, err := u.appendAudit(ctx, adminAuditInput{
			ActorAccountID: in.ActorAccountID,
			Action:         jobAuditActionCancel,
			ResourceType:   jobResourceType,
			ResourceID:     op.ID,
			Before:         jobAuditState(op.Status, nil),
			After: jobAuditState(nextStatus, map[string]string{
				"cancelReasonCode": reasonCode,
				// allowManualAfterCancellation 沒有 DB 欄可落（人工解鎖走端點 54／55 的
				// 獨立流程）。留在稽核而不是丟掉：那是 ADM 明確表達過的意圖。
				"allowManualAfterCancellation": strconv.FormatBool(in.AllowManualAfterCancellation),
			}),
			ReasonCode:     reasonCode,
			ReasonDetail:   in.ReasonDetail,
			IdempotencyKey: in.IdempotencyKey,
			Now:            now,
		})
		if err != nil {
			return err
		}

		updated, err := u.ops.GetByID(ctx, op.ID)
		if err != nil {
			return err
		}
		out = JobMutationResult{
			JobID: updated.ID, Status: updated.Status, Version: updated.Version, AuditLogID: auditID,
		}
		return nil
	})
	if err != nil {
		return JobMutationResult{}, err
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// 端點 56：代錄賣家自行開立的結果
// ---------------------------------------------------------------------------

// ReportManualResultInput 是端點 56 的輸入。
//
// 它與端點 62（賣家自報）是**同一件事的兩個入口**：守衛、目標狀態、寫入欄位完全相同，
// 只差 actor 與稽核欄 ⇒ 兩者共用 resolveExternally 這一段 domain 邏輯（§6.1.14 逐字）。
type ReportManualResultInput struct {
	ActorAccountID  int64
	JobID           int64
	InvoiceNo       string
	InvoiceDate     string // YYYY-MM-DD（UTC+8 曆日）
	TotalAmount     string // 元
	Note            string
	ExpectedVersion int64
	IdempotencyKey  string
}

// ReportManualResult 代錄賣家自行開立的發票號碼（端點 56 ＝ J30）。
//
// 🔴 三條不可協商的落點（§6.1.14）：
//
//	job.status      → RESOLVED_EXTERNALLY（**不是 SUCCEEDED**：provider 從頭到尾沒開出東西）
//	document.status → RESOLVED_EXTERNALLY，號碼寫進 external_invoice_no
//	provider_invoice_no **必須留空** —— 平台不冒充開立人；責任歸屬在爭議時是法律問題
func (u *AdminJob) ReportManualResult(ctx context.Context, in ReportManualResultInput) (JobMutationResult, error) {
	invoiceNo := in.InvoiceNo
	if !externalInvoiceNoPattern.MatchString(invoiceNo) {
		return JobMutationResult{}, ErrExternalInvoiceNoInvalid
	}
	invoiceDate, err := time.Parse(time.DateOnly, in.InvoiceDate)
	if err != nil {
		return JobMutationResult{}, ErrExternalInvoiceDateInvalid
	}
	totalAmount, err := parseAdminAmount(in.TotalAmount)
	if err != nil {
		return JobMutationResult{}, err
	}
	if utf8.RuneCountInString(in.Note) > adminNoteMaxRunes {
		return JobMutationResult{}, ErrJobNoteTooLong
	}

	var out JobMutationResult
	err = u.tx.WithTx(ctx, func(ctx context.Context) error {
		op, err := u.ops.GetByID(ctx, in.JobID)
		if err != nil {
			return err
		}
		if op.Version != in.ExpectedVersion {
			return jobBiz(bizcode.InvoiceJobVersionConflict)
		}
		// 🔴 只有**開立類**的工作能用「回報自行開立結果」收尾（#3112，2026-09-04）。
		//
		// 端點 56 的整套語意是「賣家自己去開了一張發票，號碼是這個」——它把
		// external_invoice_no／date 寫進 **document** 並把 document 推向
		// RESOLVED_EXTERNALLY。而 VOID／ALLOWANCE／VOID_ALLOWANCE／QUERY 沒有
		// 「新開出來的發票號」這回事，它們的 document 也早在自己的終態
		// （VOIDED／SUPERSEDED），不在 MarkInvoiceDocumentResolvedExternally 的
		// allowed-from（ISSUE_FAILED／AWAITING_MANUAL）裡。
		//
		// ⚠️ **在 J28 接上之前這條分支走不到**：CANCELLED_BY_ADMIN 過去只能從開立類的
		// 失敗來。J28（#3089 A 組第 2 項）讓 VOID 也走得到 MANUAL_PROCESS_ALLOWED，
		// dev 上實測就炸出 500——errExternalDocumentStateDrift 把它當成「故障」，
		// 而它其實是「這個類型不能走這條路」，是說得清楚的業務結果。
		if !isIssuanceOperation(op.OperationType) {
			return pkgerrors.NewBusiness(bizcode.InvoiceExternalIssuanceNotAllowed,
				"這件工作不是開立類（作廢／折讓／查詢），沒有「自行開立的發票號碼」可回報")
		}
		doc, err := u.docs.GetByID(ctx, op.DocumentID)
		if err != nil {
			return err
		}
		// 9349 的語意逐字就是這一格：「本作業尚未開放自行開立回報」。
		//
		// 兩條合法來源（2026-09-05，DEC-20260905-221900 選 A 之後是兩條，不是一條）：
		//   ① MANUAL_PROCESS_ALLOWED —— J30，ADM 人工出口解鎖的產物
		//   ② NOT_APPLICABLE ＋ fulfillment_mode ∈ {SELLER_SELF_ISSUE, LEGACY_MANUAL}
		//      —— 稅務資格尚未核准的賣家，平台從一開始就不開票，他自行開了票要回報號碼
		//
		// 🔴 ② 的 mode 條件不可省：NOT_APPLICABLE 也涵蓋 AMEGO_AUTO 底下的零元訂單與
		// 切點前訂單，那些不該開放回報。**SQL 那兩句也各自帶了同一個條件**——這裡先擋
		// 是為了回一個可分辨的業務碼（走到 SQL 才被擋只會拿到 rows=0 的泛用漂移錯誤）。
		if !canReportExternalIssuance(op, doc) {
			return jobBiz(bizcode.InvoiceExternalIssuanceNotAllowed)
		}
		// 發票日不得早於付款日（§6.1 端點 62 的 400 清單）。issue_not_before ＝ 收款時點，
		// 兩者的曆日都以 UTC+8 比較——拿 UTC 日期比會讓台北時間 08:00 之前的付款被誤判。
		if invoiceDate.Before(domain.InvoiceDateTaipei(doc.IssueNotBefore)) {
			return ErrExternalInvoiceDateTooEarly
		}

		now := u.now()
		if err := u.resolveExternally(ctx, op, doc, externalIssuance{
			InvoiceNo:      invoiceNo,
			InvoiceDate:    invoiceDate,
			TotalAmount:    totalAmount,
			Note:           in.Note,
			ActorType:      jobActorAdmin,
			ActorAccountID: in.ActorAccountID,
			Now:            now,
		}); err != nil {
			return err
		}

		auditID, err := u.appendAudit(ctx, adminAuditInput{
			ActorAccountID: in.ActorAccountID,
			Action:         jobAuditActionManualResult,
			ResourceType:   jobResourceType,
			ResourceID:     op.ID,
			Before:         jobAuditState(op.Status, map[string]string{"documentStatus": doc.Status}),
			After: jobAuditState(codemeta.InvoiceOperationStatusResolvedExternally.Value, map[string]string{
				"documentStatus": codemeta.InvoiceDocumentStatusResolvedExternally.Value,
				// 號碼與日期是「賣家開了哪一張票」的識別資訊，不是買受人 PII ⇒ 可入稽核。
				// 金額另記：invoice_documents 沒有承載外部發票金額的欄位（缺口登記在交付回報），
				// 這裡是它唯一的落點。
				"externalInvoiceNo":   invoiceNo,
				"externalInvoiceDate": invoiceDate.Format(time.DateOnly),
				"externalTotalAmount": strconv.FormatInt(totalAmount, 10),
			}),
			ReasonCode:     jobReasonResolvedExternally,
			ReasonDetail:   firstNonEmpty(in.Note, "ADM 代錄賣家自行開立結果"),
			IdempotencyKey: in.IdempotencyKey,
			Now:            now,
		})
		if err != nil {
			return err
		}

		updated, err := u.ops.GetByID(ctx, op.ID)
		if err != nil {
			return err
		}
		out = JobMutationResult{
			JobID: updated.ID, Status: updated.Status, Version: updated.Version, AuditLogID: auditID,
		}
		return nil
	})
	if err != nil {
		return JobMutationResult{}, err
	}
	return out, nil
}

// externalIssuance 是 resolveExternally 的輸入。
type externalIssuance struct {
	InvoiceNo   string
	InvoiceDate time.Time
	TotalAmount int64
	Note        string

	ActorType      string // ADMIN（端點 56）／SELLER（端點 62）
	ActorAccountID int64
	Now            time.Time
}

// resolveExternally 是端點 56 與端點 62 **共用**的那一段 domain 邏輯。
//
// 🔴 五件事必須同一交易（§6.1.14 逐字；⑤ 於 2026-09-08 補）：
//
//	① invoice_operations CAS（MANUAL_PROCESS_ALLOWED → RESOLVED_EXTERNALLY，帶 expectedVersion）
//	② invoice_documents.status = RESOLVED_EXTERNALLY **＋ 三個 external_* 欄一起寫**
//	③ invoice_status_history 各一列（帶 actor）
//	④ outbox invoice.resolved_externally（P12）
//	⑤ 這張憑證**本身是一張重開票**時的鏈路收尾（settleReissueChainForExternal）
//
// 只轉 operation 不轉 document，賣家 S08 會同時看到「已由賣家開立」與「處理中」。
func (u *AdminJob) resolveExternally(
	ctx context.Context, op domain.Operation, doc domain.Document, in externalIssuance,
) error {
	ok, err := u.ops.MarkResolvedExternally(ctx, op.ID, op.Version, in.Now)
	if err != nil {
		return err
	}
	if !ok {
		return jobBiz(bizcode.InvoiceJobVersionConflict)
	}

	ok, err = u.docs.MarkResolvedExternally(ctx, domain.MarkDocumentResolvedExternally{
		ID:                  doc.ID,
		ExternalInvoiceNo:   in.InvoiceNo,
		ExternalInvoiceDate: in.InvoiceDate,
		ExternalReportedAt:  in.Now,
		Now:                 in.Now,
	})
	if err != nil {
		return err
	}
	if !ok {
		// job 在 MANUAL_PROCESS_ALLOWED 而 document 不在 ISSUE_FAILED／AWAITING_MANUAL
		// ⇒ 兩列已經不自洽。**回錯誤讓整筆 rollback**：放行會留下一個「job 說已解決、
		// document 說還在失敗」的組合，而那是沒有任何人會發現的。
		return errExternalDocumentStateDrift
	}

	// 🔴 **兩條軸都要走 appendHistoryAs**：`appendHistory` 會寫死 `jobActorAdmin`，
	// 而 resolveExternally 有兩個呼叫端（端點 56 ADM／端點 62 賣家）。
	// 2026-09-07 dev 實測（op `7501682158306590721`，以賣家身分打端點 62）：
	// DOCUMENT 軸記 `SELLER`、OPERATION 軸記 `ADMIN` —— **相鄰兩行，一行有 actor 意識、
	// 一行沒有**，而 `appendHistoryAs` 的註解自己就寫著「端點 62 走 SELLER」。
	// 發票留痕要保存 5 年供稅務稽核，把賣家自行開立回報記成 ADM 做的是實質的稽核缺陷。
	if err := u.appendHistoryAs(ctx, subjectOperation, op.ID, op.Status,
		codemeta.InvoiceOperationStatusResolvedExternally.Value,
		jobReasonResolvedExternally, externalHistoryDetail(in),
		in.ActorType, in.ActorAccountID, in.Now); err != nil {
		return err
	}
	if err := u.appendHistoryAs(ctx, subjectDocument, doc.ID, doc.Status,
		codemeta.InvoiceDocumentStatusResolvedExternally.Value,
		jobReasonResolvedExternally, externalHistoryDetail(in),
		in.ActorType, in.ActorAccountID, in.Now); err != nil {
		return err
	}

	if err := u.settleReissueChainForExternal(ctx, doc, in); err != nil {
		return err
	}

	return u.emitResolvedExternally(ctx, doc, in)
}

// settleReissueChainForExternal 是「這張外部開立的憑證**本身是一張重開票**」時的鏈路收尾。
//
// 🔴 它與 Issuance.completeReissueIfAny 是**同一條不變式的兩個執行點**：一張重開票走到
// 終態時，原票必須離開 VOIDED（→ SUPERSEDED），而它身上的 correction 必須離開
// REISSUE_FAILED（→ COMPLETED）。前者接的是「重開票真的開出來了（ISSUED）」，本支接的是
// 「改由賣家在外部開立（RESOLVED_EXTERNALLY）」。**改一支就要看另一支。**
//
// 沒有本支之前的實際後果（2026-09-08 dev 實測，兩筆）：
//   - 原票停在 VOIDED ⇒ 觸發 I8 InvoiceVoidWithoutReissue（它的述詞是 status='VOIDED'
//     且無 ISSUED 後繼，而外部開立的後繼永遠不是 ISSUED ⇒ **永久紅**）
//   - correction 停在 REISSUE_FAILED、reissued_document_id 永遠 NULL ⇒ 觸發
//     InvoiceCorrectionReissueFailed（§7.3.6 自稱「最嚴重的單一狀態」，SLA 15 分鐘）
//     一個缺口同時點亮兩條 critical，而兩條的處置文件都指向「去查另一張表」。
//
// ⚠️ **刻意不發 P5 invoice.reissued**：同一交易的 ④ 已經發了 invoice.resolved_externally，
// 它的 payload 帶著 external_invoice_no ⇒ 買家已經拿得到新號碼。補發 P5 會讓同一件事
// 對買家響兩次，而那是使用者看得見的變化、屬於通知文案的產品決定，不由本次修復裁決。
//
// 🔴 rows=0 一律回 error 整筆 rollback，與 completeReissueIfAny 同一個理由：原票與新票
// 都不是 SUPERSEDED 的組合，在稅務上就是重複開立。
func (u *AdminJob) settleReissueChainForExternal(
	ctx context.Context, doc domain.Document, in externalIssuance,
) error {
	if doc.OriginalDocumentID == nil {
		return nil // 這不是重開票（一般的開票失敗轉外部開立），沒有鏈路要收
	}
	originalID := *doc.OriginalDocumentID

	// 讀在 MarkSuperseded **之前**：history 的 from 要的是真正的來源狀態，而
	// MarkSuperseded 的 allowed-from 是 ('VOIDED','ISSUED') 兩個值（後者是作廢確認
	// 遲到的補正路徑）⇒ 硬寫 VOIDED 會在稽核軌跡上留下一條沒有發生過的邊。
	original, err := u.docs.GetByID(ctx, originalID)
	if err != nil {
		return err
	}
	superseded, err := u.docs.MarkSuperseded(ctx, originalID, doc.ID, in.Now)
	if err != nil {
		return err
	}
	if !superseded {
		return fmt.Errorf(
			"invoice: original document %d could not enter SUPERSEDED (rows=0, snapshot status=%s)",
			originalID, original.Status)
	}
	if err := u.appendHistoryAs(ctx, subjectDocument, originalID,
		original.Status, codemeta.InvoiceDocumentStatusSuperseded.Value,
		jobReasonResolvedExternally, externalHistoryDetail(in),
		in.ActorType, in.ActorAccountID, in.Now); err != nil {
		return err
	}

	// correction 兩道查詢缺一不可，理由與 completeReissueIfAny 逐字相同：
	// GetOpenByDocument 的述詞只有 REQUESTED／VOID_REISSUE_IN_PROGRESS，停在
	// REISSUE_FAILED 的查不到——而後者正是本支最常見的來源（重開票失敗過，ADM 才改走
	// 外部開立）。Complete 的 allowed-from 兩個值都含，兩道各自對應其中一個。
	corr, err := u.corrections.GetOpenByDocument(ctx, originalID)
	if err != nil {
		if !isBizCode(err, bizcode.InvoiceCorrectionNotAllowed) {
			return err
		}
		corr, err = u.corrections.GetReissueFailedByDocument(ctx, originalID)
		if err != nil {
			if isBizCode(err, bizcode.InvoiceCorrectionNotAllowed) {
				return nil // 這張原始憑證上真的沒有待收尾的修改請求（非更正來源的重開）
			}
			return err
		}
	}
	done, err := u.corrections.Complete(ctx, corr.ID, doc.ID, in.Now)
	if err != nil {
		return err
	}
	if !done {
		return fmt.Errorf("invoice: correction %d could not enter COMPLETED (rows=0, status=%s)",
			corr.ID, corr.Status)
	}
	return u.appendHistoryAs(ctx, subjectCorrectionRequest, corr.ID,
		corr.Status, codemeta.InvoiceCorrectionStatusCompleted.Value,
		jobReasonResolvedExternally, externalHistoryDetail(in),
		in.ActorType, in.ActorAccountID, in.Now)
}

// externalHistoryDetail 把號碼／日期／金額／備註折成歷程的 reason_detail。
//
// 金額寫在這裡是刻意的：invoice_documents 沒有承載「賣家自己那張票開了多少錢」的欄位，
// 而那個數字是事後對帳唯一的線索。歷程表是 append-only、保存期同稽核。
func externalHistoryDetail(in externalIssuance) string {
	detail := in.InvoiceNo + " / " + in.InvoiceDate.Format(time.DateOnly) +
		" / " + strconv.FormatInt(in.TotalAmount, 10)
	if in.Note != "" {
		detail += " / " + truncateRunes(in.Note, adminNoteMaxRunes)
	}
	return detail
}

// resolvedExternallyEvent 是 P12 的 payload。
//
// 🔴 wire 欄名是 `external_invoice_no` 不是 `invoice_no`：兩者對應 §5 兩個**永不互相
// fallback** 的欄位。共用一個 wire 名的後果不是欄位錯，是**平台冒充開立人**——下游只要
// 照 P1 的習慣讀 invoice_no，就會把賣家自行開立的號碼當成平台開的。
//
// 🔴 payload 邊界（36 號）：不放載具識別碼、買受人統編、email、電話，也不放 random_code
// （invoice_no + random_code 即可在財政部平台查得全部內容，那是**憑證**不是資料）。
// P12 另外**不帶 actor**：對買家而言 ADM 代錄與賣家自報是同一個事實，actor 留在
// invoice_status_history 供稽核。
type resolvedExternallyEvent struct {
	NotificationRefID   string            `json:"notification_ref_id"`
	InvoiceDocumentID   string            `json:"invoice_document_id"`
	OrderNo             string            `json:"order_no"`
	BuyerID             eventwire.BuyerID `json:"buyer_id"`
	StoreID             eventwire.StoreID `json:"store_id"`
	ExternalInvoiceNo   string            `json:"external_invoice_no"`
	ExternalInvoiceDate string            `json:"external_invoice_date"`
	ReportedAt          string            `json:"reported_at"`
	OccurredAt          string            `json:"occurred_at"`
}

// emitResolvedExternally 把 P12 寫進 outbox（與狀態轉移同交易）。
func (u *AdminJob) emitResolvedExternally(ctx context.Context, doc domain.Document, in externalIssuance) error {
	// store_id 走 invoice_issuers.issuer_ref_id，**不是 issuer_id**：後者是 invoice_issuers
	// 的 PK，把它當 store_id 送出去，下游拿去查賣家會查不到（或更糟，查到別人）。
	issuer, err := u.issuers.GetByID(ctx, doc.IssuerID)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(resolvedExternallyEvent{
		NotificationRefID:   "INVEXT:" + strconv.FormatInt(doc.ID, 10),
		InvoiceDocumentID:   strconv.FormatInt(doc.ID, 10),
		OrderNo:             derefString(doc.SourceRef),
		BuyerID:             eventwire.BuyerIDOfDocument(doc),
		StoreID:             eventwire.StoreIDOf(issuer),
		ExternalInvoiceNo:   in.InvoiceNo,
		ExternalInvoiceDate: in.InvoiceDate.Format(time.DateOnly),
		ReportedAt:          timeutil.FormatISO8601(in.Now),
		OccurredAt:          timeutil.FormatISO8601(in.Now),
	})
	if err != nil {
		return err
	}
	_, err = u.outbox.Insert(ctx, domain.NewOutboxEvent{
		AggregateType: outboxAggregateDocument,
		AggregateID:   doc.ID,
		EventType:     string(pkgmq.EventTypeInvoiceResolvedExternally),
		Exchange:      pkgmq.ExchangeInvoice,
		RoutingKey:    pkgmq.RoutingKeyInvoiceResolvedExternally,
		Payload:       payload,
		NextAttemptAt: in.Now,
		Now:           in.Now,
	})
	return err
}

// errExternalDocumentStateDrift 是「job 已在 MANUAL_PROCESS_ALLOWED，document 卻不在
// ISSUE_FAILED／AWAITING_MANUAL」。它是故障不是業務結果 ⇒ 刻意不是 DomainError，
// 走 codes.Internal。
// isIssuanceOperation 回報這個 operation_type 會不會產生「一張新的發票號碼」。
//
// 只有這兩類的 document 走得到 RESOLVED_EXTERNALLY（#3112）。刻意用**正面列舉**：
// 值域日後長胖時漏列的後果是回 9349 而不是 500，方向安全；否定列舉會讓新型別
// 靜默落進「可以回報」那一邊。
func isIssuanceOperation(opType string) bool {
	return opType == codemeta.InvoiceOperationTypeIssue.Value ||
		opType == codemeta.InvoiceOperationTypeReissue.Value
}

// ⚠️ errExternalDocumentStateDrift 現在只剩**真正的** drift（#3112 之後）：
// 非開立類已在上面被 9349 擋掉，走到這裡代表「開立類的 job 進了
// MANUAL_PROCESS_ALLOWED，而它的 document 既不在 ISSUE_FAILED 也不在
// AWAITING_MANUAL」——那確實是不該發生的狀態組合。
var errExternalDocumentStateDrift = errors.New(
	"invoice: operation is MANUAL_PROCESS_ALLOWED but its document rejected RESOLVED_EXTERNALLY")

// ---------------------------------------------------------------------------
// 共用小工具
// ---------------------------------------------------------------------------

// adminAuditInput 是 appendAudit 的輸入。
type adminAuditInput struct {
	ActorAccountID int64
	Action         string
	ResourceType   string
	ResourceID     int64
	Before         json.RawMessage
	After          json.RawMessage
	ReasonCode     string
	ReasonDetail   string
	IdempotencyKey string
	Now            time.Time
	// ApprovalActorAccountID 是雙人確認時的**另一位**。只有端點 55 會填。
	// chk_invadmaudit_two_person 保證它 ≠ ActorAccountID —— 填成同一人會當場 23514，
	// 那正是「雙人確認被降級成一個人按兩次」在資料層的形狀。
	ApprovalActorAccountID *int64
}

// appendAudit 寫一列 ADM 稽核。
//
// 🔴 request_id 帶的是 Idempotency-Key：uq_invadmaudit_client_request 讓同一位 ADM
// 對同一動作重送同一把鍵只留一列。空字串要傳 NULL（該索引是 partial，WHERE
// request_id IS NOT NULL）——傳空字串會讓所有沒帶鍵的請求互相衝突成一列。
func (u *AdminJob) appendAudit(ctx context.Context, in adminAuditInput) (int64, error) {
	return u.audit.AppendAdminAuditLog(ctx, domain.NewAdminAuditLogEntry{
		ActorAccountID:         in.ActorAccountID,
		Action:                 in.Action,
		ResourceType:           in.ResourceType,
		ResourceID:             in.ResourceID,
		BeforeState:            in.Before,
		AfterState:             in.After,
		ReasonCode:             in.ReasonCode,
		ReasonDetail:           in.ReasonDetail,
		ApprovalActorAccountID: in.ApprovalActorAccountID,
		RequestID:              nilIfEmpty(in.IdempotencyKey),
		Now:                    in.Now,
	})
}

// appendHistory 記一次狀態轉移（actor 固定 ADMIN）。
func (u *AdminJob) appendHistory(
	ctx context.Context, subjectType string, subjectID int64,
	from, to, reasonCode, reasonDetail string, actorAccountID int64, now time.Time,
) error {
	return u.appendHistoryAs(ctx, subjectType, subjectID, from, to, reasonCode, reasonDetail,
		jobActorAdmin, actorAccountID, now)
}

// appendHistoryAs 是 actor_type 可指定的版本（端點 62 走 SELLER）。
func (u *AdminJob) appendHistoryAs(
	ctx context.Context, subjectType string, subjectID int64,
	from, to, reasonCode, reasonDetail, actorType string, actorAccountID int64, now time.Time,
) error {
	return u.audit.AppendStatusHistory(ctx, domain.NewStatusHistoryEntry{
		SubjectType:    subjectType,
		SubjectID:      subjectID,
		FromStatus:     nilIfEmpty(from),
		ToStatus:       to,
		ReasonCode:     nilIfEmpty(reasonCode),
		ReasonDetail:   nilIfEmpty(reasonDetail),
		ActorType:      actorType,
		ActorAccountID: &actorAccountID,
		Now:            now,
	})
}

// jobAuditState 產生稽核列的 before／after。
//
// 🔴 只放狀態相關欄位（§6.1.14 與稅務側同一條規則）：before_state／after_state 是 jsonb，
// 塞整列 document 會把統編、抬頭、email 複製進一張**保存 5 年**的表（§9.5）——那是憑空
// 造一個五年期的 PII 倉庫，而稽核要回答的問題只是「誰把它從什麼狀態改成什麼狀態」。
func jobAuditState(status string, extra map[string]string) json.RawMessage {
	m := map[string]string{"status": status}
	for k, v := range extra {
		if v == "" {
			continue
		}
		m[k] = v
	}
	b, _ := json.Marshal(m)
	return b
}

func requireAdminReason(reason string) error {
	if isBlank(reason) {
		return ErrJobReasonRequired
	}
	if utf8.RuneCountInString(reason) > adminReasonMaxRunes {
		return ErrJobReasonTooLong
	}
	return nil
}

func requireAdminCancelDetail(detail string) error {
	if isBlank(detail) {
		return ErrJobCancelDetailRequired
	}
	if utf8.RuneCountInString(detail) > adminReasonMaxRunes {
		return ErrJobReasonTooLong
	}
	return nil
}

// normalizeAdminCancelReason 把端點 53 的 reasonCode 收斂成 ADM_MANUAL。
//
// 🔴 只收 ADM_MANUAL（空字串視同它），其餘三個 InvoiceCancelReason 值一律 400：
// 它們是**系統路徑**的取值（全額退款前取消、ADM 停用連線、無可調整發票）。從 A06 手動
// 取消卻寫進其中之一，A05 的「依取消原因分類統計」就會出現查不出來源的資料，而錯誤分類
// 看起來與正確分類完全一樣。
func normalizeAdminCancelReason(code string) (string, error) {
	if code == "" || code == codemeta.InvoiceCancelReasonAdmManual.Value {
		return codemeta.InvoiceCancelReasonAdmManual.Value, nil
	}
	return "", ErrJobCancelReasonCodeInvalid
}

// validateFundingDecision 驗值域（空字串合法：多數 job 沒有這一格要裁決）。
func validateFundingDecision(v string) error {
	switch v {
	case "", fundingDecisionSellerFunded, fundingDecisionPlatformFunded:
		return nil
	}
	return ErrJobFundingDecisionInvalid
}

// parseAdminAmount 解析元整數金額。
//
// 🔴 空字串不視同 0：端點 56 的 totalAmount 是必填，把缺席折成 0 會讓「賣家開了多少錢」
// 這個事後對帳唯一的線索靜默變成 0（金額禁浮點、禁猜，§4）。
func parseAdminAmount(s string) (int64, error) {
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil || v < 0 {
		return 0, ErrExternalTotalAmountInvalid
	}
	return v, nil
}

// isBlank 回報字串去掉前後空白後是不是空的。
//
// 用 TrimSpace 而不是 s == ""：一格空白在 DB 的 chk_invadmaudit_reason（length >= 1）
// 底下是合法的，那條 CHECK 擋不住「填了一個空白就算填了理由」。
func isBlank(s string) bool { return strings.TrimSpace(s) == "" }

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func jobBiz(code int32) error {
	return pkgerrors.NewBusiness(code, bizcode.Message(code))
}

func (u *AdminJob) logger() *zap.Logger {
	if u.log == nil {
		return zap.NewNop()
	}
	return u.log
}

// canReportExternalIssuance 判這件 operation 現在能不能回報自行開立的號碼。
//
// 判準與 MarkInvoiceOperationResolvedExternally／MarkInvoiceDocumentResolvedExternally
// 的 allowed-from **逐字對齊**（#3166 / DEC-20260905-221900 選 A）。三份要一起改：
// 這一份多出來會回 200 然後在 SQL 撞 rows=0（漂移錯誤），少了會用 9349 擋掉合法回報。
func canReportExternalIssuance(op domain.Operation, doc domain.Document) bool {
	switch op.Status {
	case codemeta.InvoiceOperationStatusManualProcessAllowed.Value:
		return true
	case codemeta.InvoiceOperationStatusNotApplicable.Value:
		// 只有「平台從一開始就不開票」的兩個 mode。AMEGO_AUTO 的 NOT_APPLICABLE
		// （零元訂單／切點前）不在此列。
		return doc.FulfillmentMode == codemeta.InvoiceFulfillmentModeSellerSelfIssue.Value ||
			doc.FulfillmentMode == codemeta.InvoiceFulfillmentModeLegacyManual.Value
	default:
		return false
	}
}
