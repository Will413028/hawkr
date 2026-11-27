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


const (
	adminReasonMaxRunes = 500

	adminNoteMaxRunes = 500

	jobReasonAdminRetry          = "ADMIN_JOB_RETRY"
	jobReasonAdminCancel         = "ADMIN_JOB_CANCEL"
	jobReasonAdminFundingDecided = "ADMIN_FUNDING_DECIDED"
	jobReasonResolvedExternally  = "SELLER_SELF_ISSUE_REPORTED"
	jobReasonAdminDocResume      = "ADMIN_DOC_RESUME"

	jobAuditActionRetry        = "INVOICE_JOB_RETRY"
	jobAuditActionCancel       = "INVOICE_JOB_CANCEL"
	jobAuditActionManualResult = "INVOICE_JOB_MANUAL_RESULT"
	docAuditActionResume = "INVOICE_DOCUMENT_RESUME"

	jobResourceType = "JOB"
	docResourceType = "DOCUMENT"

	jobActorAdmin = "ADMIN"

	outboxAggregateDocument = "DOCUMENT"

	fundingDecisionSellerFunded   = "SELLER_FUNDED"
	fundingDecisionPlatformFunded = "PLATFORM_FUNDED"
)

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
	unlocks domain.ManualUnlockRepository
	unlockEvidence UnlockEvidenceFinalizer
	adjustments domain.AdjustmentRepository
	corrections domain.CorrectionRequestRepository
	issueCfg IssuanceConfig
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
			return err
		}
		if op.Version != in.ExpectedVersion {
			return jobBiz(bizcode.InvoiceJobVersionConflict)
		}
		doc, err := u.docs.GetByID(ctx, op.DocumentID)
		if err != nil {
			return err
		}
		now := u.now()

		if in.FundingDecision == fundingDecisionPlatformFunded {
			return pkgerrors.NewBusiness(bizcode.InvoiceJobNotRetryable,
				"本案屬平台出資折扣，無法自動開票；請改以「受控取消」結束本作業後，再用「手動開立發票」補開")
		}
		if needsFundingDecision(doc) && in.FundingDecision != fundingDecisionSellerFunded {
			return ErrJobFundingDecisionMissing
		}

		if doc.Status == codemeta.InvoiceDocumentStatusAwaitingManual.Value {
			ok, err := u.docs.ResumeFromManual(ctx, doc.ID, doc.Version, now)
			if err != nil {
				return err
			}
			if !ok {
				return jobBiz(bizcode.InvoiceJobVersionConflict)
			}
			if err := u.appendHistory(ctx, subjectDocument, doc.ID,
				doc.Status, codemeta.InvoiceDocumentStatusPending.Value,
				jobReasonAdminFundingDecided, in.Reason, in.ActorAccountID, now); err != nil {
				return err
			}
		}

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
		if err := u.advanceAdjustmentForAdmin(ctx, op, true, "", in.ActorAccountID, now); err != nil {
			return err
		}
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

func (u *AdminJob) advanceCorrectionForAdmin(
	ctx context.Context, doc domain.Document, actorAccountID int64, now time.Time,
) error {
	if u.corrections == nil || doc.OriginalDocumentID == nil {
		return nil
	}
	corr, err := u.corrections.GetReissueFailedByDocument(ctx, *doc.OriginalDocumentID)
	if err != nil {
		if isBizCode(err, bizcode.InvoiceCorrectionNotAllowed) {
			return nil
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

type adminRetryKind int

const (
	adminRetryNone adminRetryKind = iota
	adminRetryAfterFix
	adminRetryResetAttempts
)

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
		return u.ops.AdminRetryAfterFix(ctx, op.ID, op.Version, op.RequestSnapshotHash, now)
	case adminRetryResetAttempts:
		return u.ops.AdminResetAttempts(ctx, op.ID, op.Version, now)
	}
	return false, jobBiz(bizcode.InvoiceJobNotRetryable)
}

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
	reason := jobReasonAdminRetry
	if !retry {
		reason = jobReasonAdminCancel
	}
	return u.appendHistory(ctx, subjectAdjustment, adj.ID, adj.Status, to,
		reason, cancelReasonCode, actorAccountID, now)
}

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

func resumedDocumentStatus(doc domain.Document) string {
	if doc.Status == codemeta.InvoiceDocumentStatusAwaitingManual.Value {
		return codemeta.InvoiceDocumentStatusPending.Value
	}
	return doc.Status
}


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
			return err
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

		if in.FundingDecision == fundingDecisionPlatformFunded {
			return pkgerrors.NewBusiness(bizcode.InvoiceJobNotRetryable,
				"本案屬平台出資折扣，無法自動開票；請改用「手動開立發票」補開")
		}
		if needsFundingDecision(doc) && in.FundingDecision != fundingDecisionSellerFunded {
			return ErrJobFundingDecisionMissing
		}

		ok, err := u.docs.ResumeFromManual(ctx, doc.ID, doc.Version, now)
		if err != nil {
			return err
		}
		if !ok {
			return jobBiz(bizcode.InvoiceJobVersionConflict)
		}
		if err := u.appendHistory(ctx, subjectDocument, doc.ID,
			doc.Status, codemeta.InvoiceDocumentStatusPending.Value,
			jobReasonAdminDocResume, in.Reason, in.ActorAccountID, now); err != nil {
			return err
		}

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
			out = JobMutationResult{
				JobID: op.ID, Status: op.Status, Version: op.Version,
				Blocked:             true,
				LastProviderCheckAt: op.UnknownSince,
				ProviderRequestID:   op.ProviderRequestNo,
				NextStep:            "系統正在向服務商查證這筆作業是否已開立；查證完成前無法取消。若需人工介入，請走人工出口解鎖流程。",
			}
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
			return jobBiz(bizcode.InvoiceJobVersionConflict)
		}

		nextStatus := codemeta.InvoiceOperationStatusCancelRequested.Value
		if err := u.appendHistory(ctx, subjectOperation, op.ID, op.Status, nextStatus,
			jobReasonAdminCancel, reasonCode+": "+in.ReasonDetail, in.ActorAccountID, now); err != nil {
			return err
		}
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
		if !isIssuanceOperation(op.OperationType) {
			return pkgerrors.NewBusiness(bizcode.InvoiceExternalIssuanceNotAllowed,
				"這件工作不是開立類（作廢／折讓／查詢），沒有「自行開立的發票號碼」可回報")
		}
		doc, err := u.docs.GetByID(ctx, op.DocumentID)
		if err != nil {
			return err
		}
		if !canReportExternalIssuance(op, doc) {
			return jobBiz(bizcode.InvoiceExternalIssuanceNotAllowed)
		}
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

type externalIssuance struct {
	InvoiceNo   string
	InvoiceDate time.Time
	TotalAmount int64
	Note        string

	ActorType      string
	ActorAccountID int64
	Now            time.Time
}

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
		return errExternalDocumentStateDrift
	}

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

func (u *AdminJob) settleReissueChainForExternal(
	ctx context.Context, doc domain.Document, in externalIssuance,
) error {
	if doc.OriginalDocumentID == nil {
		return nil
	}
	originalID := *doc.OriginalDocumentID

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

	corr, err := u.corrections.GetOpenByDocument(ctx, originalID)
	if err != nil {
		if !isBizCode(err, bizcode.InvoiceCorrectionNotAllowed) {
			return err
		}
		corr, err = u.corrections.GetReissueFailedByDocument(ctx, originalID)
		if err != nil {
			if isBizCode(err, bizcode.InvoiceCorrectionNotAllowed) {
				return nil
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

func externalHistoryDetail(in externalIssuance) string {
	detail := in.InvoiceNo + " / " + in.InvoiceDate.Format(time.DateOnly) +
		" / " + strconv.FormatInt(in.TotalAmount, 10)
	if in.Note != "" {
		detail += " / " + truncateRunes(in.Note, adminNoteMaxRunes)
	}
	return detail
}

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

func (u *AdminJob) emitResolvedExternally(ctx context.Context, doc domain.Document, in externalIssuance) error {
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

func isIssuanceOperation(opType string) bool {
	return opType == codemeta.InvoiceOperationTypeIssue.Value ||
		opType == codemeta.InvoiceOperationTypeReissue.Value
}

var errExternalDocumentStateDrift = errors.New(
	"invoice: operation is MANUAL_PROCESS_ALLOWED but its document rejected RESOLVED_EXTERNALLY")


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
	ApprovalActorAccountID *int64
}

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

func (u *AdminJob) appendHistory(
	ctx context.Context, subjectType string, subjectID int64,
	from, to, reasonCode, reasonDetail string, actorAccountID int64, now time.Time,
) error {
	return u.appendHistoryAs(ctx, subjectType, subjectID, from, to, reasonCode, reasonDetail,
		jobActorAdmin, actorAccountID, now)
}

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

func normalizeAdminCancelReason(code string) (string, error) {
	if code == "" || code == codemeta.InvoiceCancelReasonAdmManual.Value {
		return codemeta.InvoiceCancelReasonAdmManual.Value, nil
	}
	return "", ErrJobCancelReasonCodeInvalid
}

func validateFundingDecision(v string) error {
	switch v {
	case "", fundingDecisionSellerFunded, fundingDecisionPlatformFunded:
		return nil
	}
	return ErrJobFundingDecisionInvalid
}

func parseAdminAmount(s string) (int64, error) {
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil || v < 0 {
		return 0, ErrExternalTotalAmountInvalid
	}
	return v, nil
}

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

func canReportExternalIssuance(op domain.Operation, doc domain.Document) bool {
	switch op.Status {
	case codemeta.InvoiceOperationStatusManualProcessAllowed.Value:
		return true
	case codemeta.InvoiceOperationStatusNotApplicable.Value:
		return doc.FulfillmentMode == codemeta.InvoiceFulfillmentModeSellerSelfIssue.Value ||
			doc.FulfillmentMode == codemeta.InvoiceFulfillmentModeLegacyManual.Value
	default:
		return false
	}
}
