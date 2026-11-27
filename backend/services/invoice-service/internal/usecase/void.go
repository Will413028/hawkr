package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/go-pkg/timeutil"
	"github.com/Will413028/hawkr/invoice-service/internal/client/amego"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/eventwire"
	"go.uber.org/zap"
)


// Voiding 是作廢軸的 usecase。
type Voiding struct {
	tx          database.TxManager
	docs        domain.DocumentRepository
	ops         domain.OperationRepository
	corrections domain.CorrectionRequestRepository
	accounts    domain.ProviderAccountRepository
	audit       domain.AuditRepository
	provider    ProviderClient
	outbox  domain.OutboxRepository
	issuers domain.IssuerRepository
	adjustments domain.AdjustmentRepository

	now func() time.Time
	cfg IssuanceConfig
	log *zap.Logger
}

// NewVoiding 組出作廢軸。cfg 與 Issuance 共用同一份——租約長度與退避必須一致，
// 因為兩者搶的是**同一張表的同一組候選**。
func NewVoiding(
	tx database.TxManager,
	docs domain.DocumentRepository,
	ops domain.OperationRepository,
	corrections domain.CorrectionRequestRepository,
	accounts domain.ProviderAccountRepository,
	audit domain.AuditRepository,
	provider ProviderClient,
	outbox domain.OutboxRepository,
	issuers domain.IssuerRepository,
	adjustments domain.AdjustmentRepository,
	cfg IssuanceConfig,
	log *zap.Logger,
) *Voiding {
	return &Voiding{
		tx: tx, docs: docs, ops: ops, corrections: corrections,
		accounts: accounts, audit: audit, provider: provider,
		outbox: outbox, issuers: issuers, adjustments: adjustments,
		now: func() time.Time { return time.Now().UTC() },
		cfg: cfg,
		log: orNop(log),
	}
}

// WithClock 讓測試注入時鐘。
func (u *Voiding) WithClock(now func() time.Time) *Voiding {
	if now != nil {
		u.now = now
	}
	return u
}

func orNop(l *zap.Logger) *zap.Logger {
	if l == nil {
		return zap.NewNop()
	}
	return l
}


// ClaimForVoidAt 在**單一事務**內做兩件必須同時成立的事：
//
//	① operation READY／RETRY_SCHEDULED／SENDING → SENDING（CAS，租約推到 now+120s）
//	② document ISSUED → VOIDING（M1 的對應邊，觸發者逐字就是 ①）
//
// 與 ClaimForSubmitAt 是**鏡像**關係，差別只在 ② 的目標狀態。拆開之後只要 ② 失敗，
// document 會停在 ISSUED 而 operation 已在 SENDING —— 買家看到「發票有效」而系統
// 正在作廢它，兩邊都不報錯。
//
// 🔴 ② 回 false **不當失敗**：租約到期被重新 claim 時 document 已經在 VOIDING
// （候選述詞含 SENDING，否則崩潰的租約永遠回不來）。真正的守衛是 ① 的 CAS。
//
// 🔴 leaseUntil 由 worker 逐筆現算後傳進來，不在這裡算——算一次會讓不變式從
// 「單筆最壞耗時 < 租約」退化成「批次總耗時 < 租約」，隨 batch-size 一起破。
func (u *Voiding) ClaimForVoidAt(ctx context.Context, operationID int64, now, leaseUntil time.Time) (bool, error) {
	claimed := false
	err := u.tx.WithTx(ctx, func(ctx context.Context) error {
		op, err := u.ops.GetByID(ctx, operationID)
		if err != nil {
			return err
		}
		ok, err := u.ops.Claim(ctx, operationID, now, leaseUntil)
		if err != nil || !ok {
			return err
		}
		if _, err := u.docs.MarkVoiding(ctx, op.DocumentID, now); err != nil {
			return err
		}
		if err := appendStatusHistory(ctx, u.audit, subjectOperation, op.ID, op.Status,
			codemeta.InvoiceOperationStatusSending.Value, "", "", now); err != nil {
			return err
		}
		if err := u.advanceAdjustment(ctx, op, adjustmentSending, "", now); err != nil {
			return err
		}
		claimed = true
		return nil
	})
	return claimed, err
}


// ErrVoidMissingInvoiceNumber 是「要作廢的 document 沒有發票號碼」。
//
// 這在結構上不該發生（M1 只讓 ISSUED 進 VOIDING，而 ISSUED 必有號碼），所以它是
// 我方資料錯 ⇒ 永久失敗進 ADM 工作台，不重試。
var ErrVoidMissingInvoiceNumber = errors.New("invoice: document has no provider invoice number, cannot void")

// SubmitVoid 送出一次 f0501。呼叫端（INV.op worker）必須已經 ClaimForVoidAt 成功。
//
// 回 error 只代表「這一輪連狀態都沒寫成」（DB 斷線之類）——所有業務落點都回 nil，
// 狀態已經寫定。這與 Issuance.Submit 的契約逐字相同，因為兩者共用同一個 worker。
func (u *Voiding) SubmitVoid(ctx context.Context, operationID int64) error {
	op, err := u.ops.GetByID(ctx, operationID)
	if err != nil {
		return err
	}
	if op.Status != codemeta.InvoiceOperationStatusSending.Value {
		return fmt.Errorf("invoice: operation %d is %s, expected SENDING (claim first)", op.ID, op.Status)
	}

	doc, err := u.docs.GetByID(ctx, op.DocumentID)
	if err != nil {
		return err
	}

	if doc.Status == codemeta.InvoiceDocumentStatusVoided.Value {
		return u.settleVoidAccepted(ctx, doc, op, u.now())
	}

	if doc.ProviderInvoiceNo == nil || *doc.ProviderInvoiceNo == "" {
		return u.settleVoidPermanent(ctx, doc, op, "", "", ErrVoidMissingInvoiceNumber.Error(), u.now())
	}

	account, err := u.accounts.GetByIssuer(ctx, doc.IssuerID, providerAmego)
	if err != nil {
		return err
	}

	req := amego.VoidRequest{
		CancelInvoiceNumber: *doc.ProviderInvoiceNo,
		CancelReason:        u.cancelReasonFor(ctx, op),
	}

	var sendErr error
	auditCtx := amego.WithAuditContext(ctx, op.ID, int(op.AttemptCount)+1)
	if err := u.accounts.WithDecryptedCredential(ctx, account.ID, func(providerTaxID, appKey string) error {
		_, sendErr = u.provider.VoidInvoice(auditCtx,
			amego.Credential{IssuerID: doc.IssuerID, Invoice: providerTaxID, AppKey: appKey}, req)
		return nil
	}); err != nil {
		return err
	}

	now := u.now()
	if sendErr == nil {
		return u.settleVoidAccepted(ctx, doc, op, now)
	}
	return u.settleVoidFailure(ctx, doc, op, sendErr, now)
}

const (
	voidReasonCorrection = "買方資料更正"
	voidReasonOrderRefund = "訂單退款"
)

func (u *Voiding) cancelReasonFor(ctx context.Context, op domain.Operation) string {
	if op.AdjustmentID == nil {
		return voidReasonCorrection
	}
	adj, err := u.adjustments.GetByID(ctx, *op.AdjustmentID)
	if err != nil {
		u.log.Warn("invoice.void.adjustment_lookup_failed",
			zap.Int64("operation_id", op.ID), zap.Int64p("adjustment_id", op.AdjustmentID), zap.Error(err))
		return voidReasonCorrection
	}
	if adj.TriggerKind == codemeta.InvoiceAdjustmentTriggerOrderRefund.Value {
		return voidReasonOrderRefund
	}
	return voidReasonCorrection
}


func voidRejectReasonOf(providerCode string) string {
	switch providerCode {
	case "3050123":
		return codemeta.InvoiceCorrectionRejectReasonProviderStateConflict.Value
	case "3050126":
		return codemeta.InvoiceCorrectionRejectReasonPastVoidDeadline.Value
	case "3050141":
		return codemeta.InvoiceCorrectionRejectReasonAllowanceExists.Value
	default:
		return ""
	}
}

func (u *Voiding) settleVoidAccepted(ctx context.Context, doc domain.Document, op domain.Operation, now time.Time) error {
	return u.tx.WithTx(ctx, func(ctx context.Context) error {
		ok, err := u.ops.MarkWaitingConfirmation(ctx, domain.MarkOperationUnknown{
			ID:            op.ID,
			NextAttemptAt: now.Add(u.cfg.RecheckBase),
			Now:           now,
		})
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("invoice: operation %d could not enter WAITING_CONFIRMATION (rows=0)", op.ID)
		}
		if err := appendStatusHistory(ctx, u.audit, subjectOperation, op.ID,
			codemeta.InvoiceOperationStatusSending.Value,
			codemeta.InvoiceOperationStatusWaitingConfirmation.Value, "", "", now); err != nil {
			return err
		}
		return u.advanceAdjustment(ctx, op, adjustmentWaitingConfirmation, "", now)
	})
}

func (u *Voiding) settleVoidRejected(
	ctx context.Context, doc domain.Document, op domain.Operation,
	providerCode, providerMessage, reasonCode, lastError string, now time.Time,
) error {
	return u.tx.WithTx(ctx, func(ctx context.Context) error {
		ok, err := u.ops.MarkFailedWithProviderResult(ctx, domain.MarkOperationProviderResult{
			// 🔴 ProviderMessage 是**原文**（APIError.Msg），與 LastError 不是同一件事
			// （#3082 收尾，2026-09-04 dev 實測發現）：LastError 是我方組的診斷字串
			// （`amego: /json/f0401: code=… class=… kind=…`），provider_message 要的是
			// 對方原樣回的那句話——§8.2.1 稱它「落庫供稅務舉證」。
			//
			// dev 實證：重試一筆 REISSUE 後 provider_code=3040122 有落庫、
			// provider_message 仍是 NULL ⇒ 本票的宣稱只修好了一半。
			ID:              op.ID,
			ProviderCode:    providerCode,
			ProviderMessage: providerMessage,
			LastError:       lastError,
			Now:             now,
		})
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("invoice: operation %d could not enter FAILED_PERMANENT (rows=0)", op.ID)
		}
		if err := appendStatusHistoryDetail(ctx, u.audit, subjectOperation, op.ID,
			codemeta.InvoiceOperationStatusSending.Value,
			codemeta.InvoiceOperationStatusFailedPermanent.Value,
			codemeta.InvoiceManualReasonProviderRejectedPermanent.Value,
			lastError, "", now); err != nil {
			return err
		}

		restored, err := u.docs.RestoreToIssued(ctx, doc.ID, now)
		if err != nil {
			return err
		}
		if !restored {
			return fmt.Errorf("invoice: document %d could not return to ISSUED from VOIDING (rows=0, status=%s)",
				doc.ID, doc.Status)
		}
		if err := appendStatusHistoryDetail(ctx, u.audit, subjectDocument, doc.ID,
			codemeta.InvoiceDocumentStatusVoiding.Value,
			codemeta.InvoiceDocumentStatusIssued.Value,
			codemeta.InvoiceManualReasonProviderRejectedPermanent.Value,
			lastError, "", now); err != nil {
			return err
		}

		if err := u.cancelPendingReissue(ctx, doc, providerCode, now); err != nil {
			return err
		}

		corr, err := u.corrections.GetOpenByDocument(ctx, doc.ID)
		if err != nil {
			if isBizCode(err, bizcode.InvoiceCorrectionNotAllowed) {
				u.log.Info("invoice.void.rejected_without_correction",
					zap.Int64("operation_id", op.ID), zap.Int64("document_id", doc.ID),
					zap.String("provider_code", providerCode))
				return u.advanceAdjustment(ctx, op, adjustmentFailedPermanent, reasonCode, now)
			}
			return err
		}
		detail := lastError
		rejected, err := u.corrections.Reject(ctx, domain.RejectCorrection{
			ID:           corr.ID,
			ReasonCode:   reasonCode,
			ReasonDetail: &detail,
			Now:          now,
		})
		if err != nil {
			return err
		}
		if !rejected {
			return fmt.Errorf("invoice: correction %d could not enter REJECTED (rows=0, status=%s)",
				corr.ID, corr.Status)
		}
		if err := appendStatusHistory(ctx, u.audit, subjectCorrectionRequest, corr.ID,
			corr.Status, codemeta.InvoiceCorrectionStatusRejected.Value, reasonCode, "", now); err != nil {
			return err
		}
		return u.advanceAdjustment(ctx, op, adjustmentFailedPermanent, reasonCode, now)
	})
}

func (u *Voiding) settleVoidPermanent(
	ctx context.Context, doc domain.Document, op domain.Operation,
	providerCode, providerMessage, lastError string, now time.Time,
) error {
	return u.tx.WithTx(ctx, func(ctx context.Context) error {
		ok, err := u.ops.MarkFailedWithProviderResult(ctx, domain.MarkOperationProviderResult{
			// 🔴 ProviderMessage 是**原文**（APIError.Msg），與 LastError 不是同一件事
			// （#3082 收尾，2026-09-04 dev 實測發現）：LastError 是我方組的診斷字串
			// （`amego: /json/f0401: code=… class=… kind=…`），provider_message 要的是
			// 對方原樣回的那句話——§8.2.1 稱它「落庫供稅務舉證」。
			//
			// dev 實證：重試一筆 REISSUE 後 provider_code=3040122 有落庫、
			// provider_message 仍是 NULL ⇒ 本票的宣稱只修好了一半。
			ID:              op.ID,
			ProviderCode:    providerCode,
			ProviderMessage: providerMessage,
			LastError:       lastError,
			Now:             now,
		})
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("invoice: operation %d could not enter FAILED_PERMANENT (rows=0)", op.ID)
		}
		if err := appendStatusHistoryDetail(ctx, u.audit, subjectOperation, op.ID,
			codemeta.InvoiceOperationStatusSending.Value,
			codemeta.InvoiceOperationStatusFailedPermanent.Value,
			codemeta.InvoiceManualReasonProviderRejectedPermanent.Value,
			lastError, "", now); err != nil {
			return err
		}
		if doc.Status == codemeta.InvoiceDocumentStatusVoiding.Value {
			if _, err := u.docs.RestoreToIssued(ctx, doc.ID, now); err != nil {
				return err
			}
		}
		reason := codemeta.InvoiceManualReasonProviderRejectedPermanent.Value
		if providerCode == "" {
			reason = codemeta.InvoiceManualReasonAllowanceNoSourceInvoice.Value
		}
		return u.advanceAdjustment(ctx, op, adjustmentFailedPermanent, reason, now)
	})
}

func (u *Voiding) settleVoidFailure(
	ctx context.Context, doc domain.Document, op domain.Operation, sendErr error, now time.Time,
) error {
	var apiErr *amego.APIError
	if !errors.As(sendErr, &apiErr) {
		return u.settleVoidAccepted(ctx, doc, op, now)
	}

	switch apiErr.Class {
	case amego.ClassRetryable:
		return u.settleVoidRetry(ctx, doc, op, apiErr, now)

	case amego.ClassReconcile:
		return u.settleVoidAccepted(ctx, doc, op, now)

	case amego.ClassManual, amego.ClassPermanent:
		if reason := voidRejectReasonOf(apiErr.Code); reason != "" {
			return u.settleVoidRejected(ctx, doc, op, apiErr.Code, apiErr.Msg, reason, apiErr.Error(), now)
		}
		return u.settleVoidPermanent(ctx, doc, op, apiErr.Code, apiErr.Msg, apiErr.Error(), now)

	default:
		return u.settleVoidAccepted(ctx, doc, op, now)
	}
}

func (u *Voiding) settleVoidRetry(
	ctx context.Context, doc domain.Document, op domain.Operation, apiErr *amego.APIError, now time.Time,
) error {
	newAttempt := op.AttemptCount + 1
	exhausted := newAttempt >= op.MaxAttempts
	delay := backoffFor(int(newAttempt), u.cfg.BaseBackoff, u.cfg.MaxBackoff)
	lastErr := apiErr.Error()

	return u.tx.WithTx(ctx, func(ctx context.Context) error {
		if err := u.ops.UpdateAttempt(ctx, op.ID, newAttempt, now.Add(delay), &lastErr, now); err != nil {
			return err
		}
		if err := appendStatusHistoryDetail(ctx, u.audit, subjectOperation, op.ID,
			codemeta.InvoiceOperationStatusSending.Value,
			codemeta.InvoiceOperationStatusRetryScheduled.Value,
			codemeta.InvoiceManualReasonProviderRetryableError.Value,
			lastErr, "", now); err != nil {
			return err
		}
		if !exhausted {
			return nil
		}

		ok, err := u.ops.MarkRetryExhausted(ctx, op.ID, lastErr, now)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("invoice: operation %d could not enter FAILED_RETRY_EXHAUSTED (rows=0)", op.ID)
		}
		if err := appendStatusHistoryDetail(ctx, u.audit, subjectOperation, op.ID,
			codemeta.InvoiceOperationStatusRetryScheduled.Value,
			codemeta.InvoiceOperationStatusFailedRetryExhausted.Value,
			codemeta.InvoiceManualReasonRetryBudgetExhausted.Value,
			lastErr, "", now); err != nil {
			return err
		}
		if doc.Status == codemeta.InvoiceDocumentStatusVoiding.Value ||
			doc.Status == codemeta.InvoiceDocumentStatusIssued.Value {
			if _, err := u.docs.RestoreToIssued(ctx, doc.ID, now); err != nil {
				return err
			}
		}
		return nil
	})
}

func (u *Voiding) cancelPendingReissue(
	ctx context.Context, orig domain.Document, providerCode string, now time.Time,
) error {
	if orig.SourceRef == nil || *orig.SourceRef == "" {
		return nil
	}
	requestNo := domain.ProviderRequestNoForOrder(*orig.SourceRef, orig.SnapshotVersion, orig.IssueAttemptSeq+1)
	reissue, err := u.docs.GetByProviderRequestNo(ctx, requestNo)
	if err != nil {
		if isBizCode(err, bizcode.InvoiceDocumentNotFound) {
			return nil
		}
		return err
	}
	if reissue.Status != codemeta.InvoiceDocumentStatusPending.Value {
		return nil
	}

	ops, err := u.ops.ListByDocument(ctx, reissue.ID)
	if err != nil {
		return err
	}
	for _, op := range ops {
		if op.OperationType != codemeta.InvoiceOperationTypeReissue.Value {
			continue
		}
		if _, err := u.ops.CancelBeforeIssue(ctx, op.ID,
			codemeta.InvoiceCancelReasonVoidRejected.Value, "作廢被 provider 拒絕（"+providerCode+"）", now); err != nil {
			return err
		}
	}
	if _, err := u.docs.CancelBeforeIssue(ctx, reissue.ID,
		codemeta.InvoiceCancelReasonVoidRejected.Value, "作廢被 provider 拒絕（"+providerCode+"）", now); err != nil {
		return err
	}
	u.log.Info("invoice.void.reissue_cancelled",
		zap.Int64("original_document_id", orig.ID), zap.Int64("reissue_document_id", reissue.ID),
		zap.String("provider_code", providerCode))
	return nil
}


// ConfirmVoid 對一筆 WAITING_CONFIRMATION 的 VOID operation 打**一次** invoice_query，
// 確認 invoice_type 是否已經變成 C0501。
//
// 🔴 為什麼作廢需要「第二段」：f0501 回 code:0 只代表 AMEGO **受理**了請求，
// invoice_query 的 invoice_type 仍是 C0401，作廢只排進了 wait[] 陣列
// （設計書 §5.1.5、光貿 API 參考 §2.3.4 實測）。「動作成功」與「憑證已變成作廢」
// 之間必然有時間差，用同一個欄位表達那段時間只能靠猜。
//
// 🔴 為什麼它在 INV.rc 這條 lane 而不是另開一支 worker：WAITING_CONFIRMATION 已經在
// FetchPendingInvoiceRechecks 的候選述詞裡，兩支掃描器掃同一個集合就是搶同一批列。
//
// 確認成功時同一事務內兩件事：
//
//	① document VOIDING → VOIDED
//	② R04 建好的 REISSUE operation CREATED → READY（**推進它的唯一執行者就是這裡**）
//
// ② 少了就是本服務反覆出現的那個 class：建了一列而沒有東西會推它。症狀是原票已作廢、
// 新票永遠不開、correction 永遠停在 VOID_REISSUE_IN_PROGRESS，而且完全不報錯。
func (u *Voiding) ConfirmVoid(ctx context.Context, op domain.Operation) error {
	doc, err := u.docs.GetByID(ctx, op.DocumentID)
	if err != nil {
		return err
	}
	if doc.Status == codemeta.InvoiceDocumentStatusVoided.Value {
		now := u.now()
		return u.tx.WithTx(ctx, func(ctx context.Context) error {
			if err := u.promoteReissue(ctx, doc, now); err != nil {
				return err
			}
			return finishOperationSucceeded(ctx, u.ops, u.audit, op, now)
		})
	}
	if doc.ProviderInvoiceNo == nil || *doc.ProviderInvoiceNo == "" {
		return fmt.Errorf("invoice: document %d has no invoice number, cannot confirm void", doc.ID)
	}

	account, err := u.accounts.GetByIssuer(ctx, doc.IssuerID, providerAmego)
	if err != nil {
		return err
	}
	var detail *amego.InvoiceDetail
	var queryErr error
	confirmAuditCtx := amego.WithAuditContext(ctx, op.ID, int(op.AttemptCount)+1)
	if err := u.accounts.WithDecryptedCredential(ctx, account.ID, func(providerTaxID, appKey string) error {
		detail, queryErr = u.provider.QueryInvoiceByNumber(confirmAuditCtx,
			amego.Credential{IssuerID: doc.IssuerID, Invoice: providerTaxID, AppKey: appKey},
			*doc.ProviderInvoiceNo)
		return nil
	}); err != nil {
		return err
	}

	now := u.now()
	if queryErr != nil || detail == nil {
		u.log.Warn("invoice.void.confirm_query_failed",
			zap.Int64("operation_id", op.ID), zap.Error(queryErr))
		return u.scheduleVoidRecheck(ctx, op, now)
	}
	if detail.InvoiceType != voidedInvoiceType {
		return u.scheduleVoidRecheck(ctx, op, now)
	}

	return u.tx.WithTx(ctx, func(ctx context.Context) error {
		ok, err := u.docs.MarkVoided(ctx, doc.ID, now)
		if err != nil {
			return err
		}
		alreadyApplied := !ok
		if alreadyApplied &&
			doc.Status != codemeta.InvoiceDocumentStatusVoided.Value &&
			doc.Status != codemeta.InvoiceDocumentStatusSuperseded.Value {
			return fmt.Errorf("invoice: document %d could not enter VOIDED (rows=0, status=%s)", doc.ID, doc.Status)
		}
		if alreadyApplied {
			u.log.Warn("invoice.void.document_already_past_voiding",
				zap.Int64("operation_id", op.ID), zap.Int64("document_id", doc.ID),
				zap.String("document_status", doc.Status))
		} else if err := appendStatusHistory(ctx, u.audit, subjectDocument, doc.ID,
			codemeta.InvoiceDocumentStatusVoiding.Value,
			codemeta.InvoiceDocumentStatusVoided.Value, "", "", now); err != nil {
			return err
		}
		if err := finishOperationSucceeded(ctx, u.ops, u.audit, op, now); err != nil {
			return err
		}
		if !alreadyApplied {
			if err := u.emitVoided(ctx, doc, now); err != nil {
				return err
			}
		}
		if err := u.advanceAdjustment(ctx, op, adjustmentCompleted, "", now); err != nil {
			return err
		}
		return u.promoteReissue(ctx, doc, now)
	})
}

type voidedEvent struct {
	NotificationRefID string            `json:"notification_ref_id"`
	InvoiceDocumentID string            `json:"invoice_document_id"`
	OrderNo           string            `json:"order_no"`
	BuyerID           eventwire.BuyerID `json:"buyer_id"`
	StoreID           eventwire.StoreID `json:"store_id"`
	VoidedInvoiceNo   string            `json:"voided_invoice_no"`
	VoidReasonCode    string            `json:"void_reason_code"`
	OccurredAt        string            `json:"occurred_at"`
}

func (u *Voiding) emitVoided(ctx context.Context, doc domain.Document, now time.Time) error {
	issuer, err := u.issuers.GetByID(ctx, doc.IssuerID)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(voidedEvent{
		NotificationRefID: "INV:" + strconv.FormatInt(doc.ID, 10),
		InvoiceDocumentID: strconv.FormatInt(doc.ID, 10),
		OrderNo:           derefString(doc.SourceRef),
		BuyerID:           eventwire.BuyerIDOfDocument(doc),
		StoreID:           eventwire.StoreIDOf(issuer),
		VoidedInvoiceNo:   derefString(doc.ProviderInvoiceNo),
		VoidReasonCode:    derefString(doc.ManualReasonCode),
		OccurredAt:        timeutil.FormatISO8601(now),
	})
	if err != nil {
		return err
	}
	_, err = u.outbox.Insert(ctx, domain.NewOutboxEvent{
		AggregateType: outboxAggregateD,
		AggregateID:   doc.ID,
		EventType:     string(pkgmq.EventTypeInvoiceVoided),
		Exchange:      pkgmq.ExchangeInvoice,
		RoutingKey:    pkgmq.RoutingKeyInvoiceVoided,
		Payload:       payload,
		NextAttemptAt: now,
		Now:           now,
	})
	return err
}

const voidedInvoiceType = "C0501"

func (u *Voiding) promoteReissue(ctx context.Context, orig domain.Document, now time.Time) error {
	if orig.SourceRef == nil || *orig.SourceRef == "" {
		return nil
	}
	requestNo := domain.ProviderRequestNoForOrder(*orig.SourceRef, orig.SnapshotVersion, orig.IssueAttemptSeq+1)
	reissue, err := u.docs.GetByProviderRequestNo(ctx, requestNo)
	if err != nil {
		if isBizCode(err, bizcode.InvoiceDocumentNotFound) {
			return nil
		}
		return err
	}
	ops, err := u.ops.ListByDocument(ctx, reissue.ID)
	if err != nil {
		return err
	}
	for _, op := range ops {
		if op.OperationType != codemeta.InvoiceOperationTypeReissue.Value ||
			op.Status != codemeta.InvoiceOperationStatusCreated.Value {
			continue
		}
		promoted, err := u.ops.Promote(ctx, op.ID, codemeta.InvoiceOperationStatusReady.Value, now, now)
		if err != nil {
			return err
		}
		if !promoted {
			return fmt.Errorf("invoice: reissue operation %d could not be promoted to READY (rows=0)", op.ID)
		}
		u.log.Info("invoice.void.reissue_promoted",
			zap.Int64("original_document_id", orig.ID),
			zap.Int64("reissue_document_id", reissue.ID), zap.Int64("operation_id", op.ID))
	}
	return nil
}

func (u *Voiding) scheduleVoidRecheck(ctx context.Context, op domain.Operation, now time.Time) error {
	elapsed := time.Duration(0)
	if op.UnknownSince != nil {
		elapsed = now.Sub(*op.UnknownSince)
	}
	if elapsed > recheckGiveUpAfter {
		lastErr := "invoice: void confirmation still unknown after give-up window"
		ok, err := u.ops.MarkRecheckUnresolved(ctx, domain.MarkRecheckResult{
			ID:            op.ID,
			NextStatus:    codemeta.InvoiceOperationStatusFailedUnknown.Value,
			NextAttemptAt: now.Add(recheckBackoff(elapsed)),
			LastError:     &lastErr,
			Now:           now,
		})
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("invoice: operation %d could not enter FAILED_UNKNOWN (rows=0)", op.ID)
		}
		u.log.Error("invoice.void.confirm_gave_up",
			zap.Int64("operation_id", op.ID), zap.Int64("document_id", op.DocumentID),
			zap.Duration("elapsed", elapsed))
		if err := appendStatusHistory(ctx, u.audit, subjectOperation, op.ID,
			op.Status, codemeta.InvoiceOperationStatusFailedUnknown.Value, "", "", now); err != nil {
			return err
		}
		return u.advanceAdjustment(ctx, op, adjustmentFailedUnknown, "", now)
	}
	ok, err := u.ops.MarkWaitingConfirmation(ctx, domain.MarkOperationUnknown{
		ID:            op.ID,
		NextAttemptAt: now.Add(recheckBackoff(elapsed)),
		Now:           now,
	})
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf(
			"invoice: operation %d could not re-enter WAITING_CONFIRMATION (rows=0, snapshot status=%s)",
			op.ID, op.Status)
	}
	return nil
}

type adjustmentStep int

const (
	adjustmentSending adjustmentStep = iota
	adjustmentCompleted
	adjustmentFailedPermanent
	adjustmentWaitingConfirmation
	adjustmentFailedUnknown
)

func (u *Voiding) advanceAdjustment(
	ctx context.Context, op domain.Operation, step adjustmentStep, reasonCode string, now time.Time,
) error {
	if op.AdjustmentID == nil {
		return nil
	}
	id := *op.AdjustmentID

	var (
		ok  bool
		err error
		to  string
	)
	switch step {
	case adjustmentSending:
		ok, err = u.adjustments.MarkSending(ctx, id, now)
		to = codemeta.InvoiceAdjustmentStatusSending.Value
	case adjustmentCompleted:
		ok, err = u.adjustments.MarkCompleted(ctx, domain.MarkAdjustmentCompleted{ID: id, Now: now})
		to = codemeta.InvoiceAdjustmentStatusCompleted.Value
	case adjustmentFailedPermanent:
		ok, err = u.adjustments.MarkFailedPermanent(ctx, id, reasonCode, now)
		to = codemeta.InvoiceAdjustmentStatusFailedPermanent.Value
	case adjustmentWaitingConfirmation:
		ok, err = u.adjustments.MarkWaitingConfirmation(ctx, id, now)
		to = codemeta.InvoiceAdjustmentStatusWaitingConfirmation.Value
	case adjustmentFailedUnknown:
		ok, err = u.adjustments.MarkFailedUnknown(ctx, id, now)
		to = codemeta.InvoiceAdjustmentStatusFailedUnknown.Value
	default:
		return fmt.Errorf("invoice: unknown adjustment step %d", step)
	}
	if err != nil {
		return err
	}
	if !ok {
		u.log.Info("invoice.void.adjustment_step_no_rows",
			zap.Int64("adjustment_id", id), zap.Int64("operation_id", op.ID),
			zap.String("want_status", to))
		return nil
	}
	return appendStatusHistory(ctx, u.audit, subjectAdjustment, id, "", to, reasonCode, "", now)
}
