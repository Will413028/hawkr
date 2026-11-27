package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/go-pkg/timeutil"
	"github.com/Will413028/hawkr/invoice-service/internal/client/amego"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/eventwire"
	"go.uber.org/zap"
)

// ClaimForSubmit 在**單一事務**內做兩件必須同時成立的事：
//
//	① operation READY／RETRY_SCHEDULED／SENDING → SENDING（J07，CAS，租約推到 now+120s）
//	② document PENDING／ISSUE_FAILED → ISSUING（M1 的對應邊）
//
// 為什麼綁在一起：M1 那條邊的觸發者就是 ①，拆開之後只要 ② 失敗，document 會停在 PENDING
// 而 operation 已經在 SENDING —— 賣家看到「待開立」而系統正在送，兩邊都不會報錯。
//
// 回 false 代表沒搶到（別的副本先拿到，或租約未到期）。呼叫端跳過這一列，**不得**改走
// 別的路徑重試：另開一條旁路等於製造第二個 SENDING 持有者（§8.4.1 I10）。
//
// 🔴 leaseUntil 逐筆在 claim 當下現算（不是在批次迴圈外算一次）：算一次會讓不變式從
// 「單筆最壞耗時 < 租約」退化成「批次總耗時 < 租約」，隨 batch-size 一起破。
func (u *Issuance) ClaimForSubmit(ctx context.Context, operationID int64) (bool, error) {
	now := u.now()
	return u.ClaimForSubmitAt(ctx, operationID, now, now.Add(u.cfg.IssueLease))
}

// ClaimForSubmitAt 是 ClaimForSubmit 的顯式租約版本，給 pkg/outbox.Worker 用。
//
// 存在的理由只有一個：`Repository[T].Claim` 的租約基準由 **worker 逐筆現算**
// （`pkg/outbox/worker.go` 的 processRow：`time.Now().Add(cfg.ClaimLease)`），
// 而 CAS 上界用的是該批次的 tickNow。兩個值必須從 worker 傳進來，不能在這裡各算一次
// ——各算一次的話，「租約基準逐筆取」這條不變式就會有兩份實作，而其中一份沒有測試守著。
//
// now 同時是 `started_at` 的 COALESCE 值與 CAS 的 `claimable_before` 上界。
func (u *Issuance) ClaimForSubmitAt(ctx context.Context, operationID int64, now, leaseUntil time.Time) (bool, error) {
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
		if _, err := u.docs.MarkIssuing(ctx, op.DocumentID, now); err != nil {
			return err
		}
		if err := u.appendHistory(ctx, subjectOperation, op.ID, op.Status,
			codemeta.InvoiceOperationStatusSending.Value, "", "", now); err != nil {
			return err
		}
		claimed = true
		return nil
	})
	return claimed, err
}

// Submit 送出一次開票。呼叫端（INV.op worker）必須已經 ClaimForSubmit 成功。
//
// 回 error 代表「這一輪沒有得到結論、且列上的狀態已經寫好」——呼叫端不需要再做補償。
// 本函式**不自己重試**：重試一律在 worker 層（照 PayUni client 慣例，§4.2.2）。
func (u *Issuance) Submit(ctx context.Context, operationID int64) error {
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

	if doc.ProviderInvoiceNo != nil && *doc.ProviderInvoiceNo != "" {
		return u.settleIssued(ctx, doc, op, &amego.IssueResult{
			InvoiceNumber: *doc.ProviderInvoiceNo,
			RandomNumber:  derefString(doc.ProviderRandomCode),
		}, u.now())
	}

	if doc.FulfillmentMode == codemeta.InvoiceFulfillmentModePlatformReceipt.Value {
		return u.settleReceipt(ctx, doc, op, u.now())
	}

	if doc.FulfillmentMode != codemeta.InvoiceFulfillmentModeAmegoAuto.Value {
		return u.settlePermanent(ctx, doc, op, "", "", ErrUnexpectedIssueOperation.Error(), u.now())
	}

	lines, err := u.docs.ListLines(ctx, doc.ID)
	if err != nil {
		return err
	}

	var req amego.IssueRequest
	if err := u.docs.WithDecryptedRecipient(ctx, doc.ID, func(s domain.RecipientSecrets) error {
		var berr error
		req, berr = buildIssueRequest(doc, lines, s)
		return berr
	}); err != nil {
		return err
	}

	account, err := u.accounts.GetByIssuer(ctx, doc.IssuerID, providerAmego)
	if err != nil {
		return err
	}

	var result *amego.IssueResult
	var sendErr error
	auditCtx := amego.WithAuditContext(ctx, op.ID, int(op.AttemptCount)+1)
	var carrierRejected bool
	if err := u.accounts.WithDecryptedCredential(ctx, account.ID, func(providerTaxID, appKey string) error {
		cred := amego.Credential{IssuerID: doc.IssuerID, Invoice: providerTaxID, AppKey: appKey}
		if mobileBarcodeRejected(auditCtx, u.provider, cred, req) {
			carrierRejected = true
			return nil
		}
		result, sendErr = u.provider.IssueInvoice(auditCtx, cred, req)
		return nil
	}); err != nil {
		return err
	}

	now := u.now()
	if carrierRejected {
		return u.settleAwaitingManual(ctx, doc, op,
			codemeta.InvoiceManualReasonCarrierInvalid.Value, now)
	}
	if sendErr == nil {
		return u.settleIssued(ctx, doc, op, result, now)
	}
	return u.settleFailure(ctx, doc, op, sendErr, now)
}

func mobileBarcodeRejected(ctx context.Context, p ProviderClient, cred amego.Credential, req amego.IssueRequest) bool {
	if req.CarrierType != domain.AmegoCarrierMobileBarcode || req.CarrierID1 == "" {
		return false
	}
	err := p.VerifyMobileBarcode(ctx, cred, req.CarrierID1)
	if err == nil {
		return false
	}
	var ae *amego.APIError
	if !errors.As(err, &ae) {
		return false
	}
	return ae.Code == amegoCodeBarcodeMalformed || ae.Code == amegoCodeBarcodeNotFound
}

const (
	amegoCodeBarcodeMalformed = "9000112"
	amegoCodeBarcodeNotFound  = "9000113"
)

// SweepOverdueStatutoryDeadlines 把**已逾法定上傳期限**的文件標成 AWAITING_MANUAL ＋
// manual_reason_code=DEADLINE_MISSED（設計書 §5.2.1）。
//
// 🔴 這支在 2026-09-09 之前**不存在**，而 §5.2.1 逐字寫著「過期者由同一支掃描標
// AWAITING_MANUAL ＋ manual_reason_code='DEADLINE_MISSED'」。實查：
// `FetchApproachingStatutoryDeadlines` 零 usecase／worker 呼叫端、`DEADLINE_MISSED`
// 零生產者（business-alerts.yml 的 InvoiceStatutoryDeadlineOverdue 註解 2026-09-05
// 已記載，並警告「照那句話等掃描把它清掉的人會一直等下去」）。
//
// ⚠️ **這不是把 critical 降級**：逾期文件轉進 AWAITING_MANUAL 之後由 M32
// （invoice_documents_awaiting_manual）接手，而 `InvoiceAwaitingManualDeadlineAtRisk`
// 的述詞含 `bucket=~"overdue|..."` 且 severity 同樣是 **critical**。換的是桶不是等級，
// 而新桶的語意（「這件事在等人處理」）才是逾期文件的真實狀態。
//
// 🔴 只標 PENDING 與 ISSUE_FAILED：
//   - ISSUING 代表**正在送 AMEGO**，逾期了也該等它跑完（MarkAwaitingManual 的
//     allowed-from 是 ('PENDING','ISSUE_FAILED') ⇒ 天然擋住，這裡的 continue 只是
//     省一次無謂的 UPDATE 與一列誤導的 history）
//   - AWAITING_MANUAL 已經在目標狀態（本查詢的述詞含它，是為了「逼近告警」那個用途）
//
// **不動 operation**：它可能已是終態、也可能還在候選集合裡，而 document 轉
// AWAITING_MANUAL 是分類不是中止。動它會與 INV.op 的 lease 搶同一列。
func (u *Issuance) SweepOverdueStatutoryDeadlines(ctx context.Context, now time.Time) (int, error) {
	rows, err := u.docs.FetchApproachingStatutoryDeadlines(ctx, now, statutorySweepBatch)
	if err != nil {
		return 0, err
	}
	swept := 0
	for _, d := range rows {
		if d.Status != codemeta.InvoiceDocumentStatusPending.Value &&
			d.Status != codemeta.InvoiceDocumentStatusIssueFailed.Value {
			continue
		}
		ok, merr := u.docs.MarkAwaitingManual(ctx, d.ID,
			codemeta.InvoiceManualReasonDeadlineMissed.Value, now)
		if merr != nil {
			return swept, merr
		}
		if !ok {
			continue
		}
		if err := u.appendHistory(ctx, subjectDocument, d.ID, d.Status,
			codemeta.InvoiceDocumentStatusAwaitingManual.Value,
			codemeta.InvoiceManualReasonDeadlineMissed.Value, "", now); err != nil {
			return swept, err
		}
		swept++
	}
	return swept, nil
}

const statutorySweepBatch int32 = 200

func (u *Issuance) settleFailure(ctx context.Context, doc domain.Document, op domain.Operation, sendErr error, now time.Time) error {
	var apiErr *amego.APIError
	if !errors.As(sendErr, &apiErr) {
		return u.settleUnknown(ctx, doc, op, "", sendErr.Error(), now)
	}

	switch apiErr.Class {
	case amego.ClassRetryable:
		return u.settleRetry(ctx, doc, op, apiErr, now)

	case amego.ClassReconcile:
		return u.settleUnknown(ctx, doc, op, apiErr.Code, apiErr.Error(), now)

	case amego.ClassManual:
		if apiErr.SellerActionRequired {
			return u.settleSellerAction(ctx, doc, op, apiErr, now)
		}
		return u.settlePermanent(ctx, doc, op, apiErr.Code, apiErr.Msg, apiErr.Error(), now)

	case amego.ClassPermanent:
		if apiErr.SellerActionRequired {
			return u.settleSellerAction(ctx, doc, op, apiErr, now)
		}
		return u.settlePermanent(ctx, doc, op, apiErr.Code, apiErr.Msg, apiErr.Error(), now)

	default:
		return u.settleUnknown(ctx, doc, op, apiErr.Code, apiErr.Error(), now)
	}
}

func (u *Issuance) settleIssued(ctx context.Context, doc domain.Document, op domain.Operation, res *amego.IssueResult, now time.Time) error {
	var issuedAt *time.Time
	if res.InvoiceTime > 0 {
		t := time.Unix(res.InvoiceTime, 0).UTC()
		issuedAt = &t
	}

	invoiceDate := doc.InvoiceDateTaipei
	taxPeriod := derefString(doc.TaxPeriodCode)

	var pdfNext *time.Time
	if doc.CarrierType == nil || *doc.CarrierType == "" {
		t := now
		pdfNext = &t
	}
	uploadNext := now

	issuer, err := u.issuers.GetByID(ctx, doc.IssuerID)
	if err != nil {
		return err
	}
	payload, err := u.issuedEventPayload(issuer, doc, res, issuedAt, now)
	if err != nil {
		return err
	}

	return u.tx.WithTx(ctx, func(ctx context.Context) error {
		if _, err := u.outbox.Insert(ctx, domain.NewOutboxEvent{
			AggregateType: outboxAggregateD,
			AggregateID:   doc.ID,
			EventType:     string(issuedEventType(doc.DocumentKind)),
			Exchange:      pkgmq.ExchangeInvoice,
			RoutingKey:    issuedRoutingKey(doc.DocumentKind),
			Payload:       payload,
			NextAttemptAt: now,
			Now:           now,
		}); err != nil {
			return err
		}

		ok, err := u.docs.MarkIssued(ctx, domain.MarkDocumentIssued{
			ID:                  doc.ID,
			Provider:            providerAmego,
			ProviderInvoiceNo:   res.InvoiceNumber,
			ProviderRandomCode:  res.RandomNumber,
			ProviderIssuedAt:    issuedAt,
			InvoiceDateTaipei:   invoiceDate,
			TaxPeriodCode:       taxPeriod,
			PdfFetchNextAt:      pdfNext,
			UploadRecheckNextAt: &uploadNext,
			Now:                 now,
		})
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("invoice: document %d could not enter ISSUED (rows=0, status=%s)", doc.ID, doc.Status)
		}
		if _, err := u.ops.MarkDone(ctx, op.ID, now); err != nil {
			return err
		}
		if err := u.appendHistory(ctx, subjectDocument, doc.ID,
			codemeta.InvoiceDocumentStatusIssuing.Value,
			codemeta.InvoiceDocumentStatusIssued.Value, "", "", now); err != nil {
			return err
		}
		if err := u.appendHistory(ctx, subjectOperation, op.ID,
			codemeta.InvoiceOperationStatusSending.Value,
			codemeta.InvoiceOperationStatusSucceeded.Value, "", "", now); err != nil {
			return err
		}
		return u.completeReissueIfAny(ctx, issuer, doc, res, now)
	})
}

func (u *Issuance) completeReissueIfAny(
	ctx context.Context, issuer domain.Issuer, doc domain.Document,
	res *amego.IssueResult, now time.Time,
) error {
	if doc.OriginalDocumentID == nil {
		return nil
	}
	originalID := *doc.OriginalDocumentID
	original, err := u.docs.GetByID(ctx, originalID)
	if err != nil {
		return err
	}
	superseded, err := u.docs.MarkSuperseded(ctx, originalID, doc.ID, now)
	if err != nil {
		return err
	}
	if !superseded {
		return fmt.Errorf(
			"invoice: original document %d could not enter SUPERSEDED (rows=0, snapshot status=%s)",
			originalID, original.Status)
	}
	if err := u.appendHistory(ctx, subjectDocument, originalID,
		original.Status,
		codemeta.InvoiceDocumentStatusSuperseded.Value, "", "", now); err != nil {
		return err
	}

	reissuedPayload, err := u.reissuedEventPayload(issuer, doc, original, res, now)
	if err != nil {
		return err
	}
	if _, err := u.outbox.Insert(ctx, domain.NewOutboxEvent{
		AggregateType: outboxAggregateD,
		AggregateID:   doc.ID,
		EventType:     string(pkgmq.EventTypeInvoiceReissued),
		Exchange:      pkgmq.ExchangeInvoice,
		RoutingKey:    pkgmq.RoutingKeyInvoiceReissued,
		Payload:       reissuedPayload,
		NextAttemptAt: now,
		Now:           now,
	}); err != nil {
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
	done, err := u.corrections.Complete(ctx, corr.ID, doc.ID, now)
	if err != nil {
		return err
	}
	if !done {
		return fmt.Errorf("invoice: correction %d could not enter COMPLETED (rows=0, status=%s)", corr.ID, corr.Status)
	}
	return u.appendHistory(ctx, subjectCorrectionRequest, corr.ID,
		corr.Status, codemeta.InvoiceCorrectionStatusCompleted.Value, "", "", now)
}

func (u *Issuance) settleRetry(ctx context.Context, doc domain.Document, op domain.Operation, apiErr *amego.APIError, now time.Time) error {
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
			if _, err := u.docs.MarkPendingRetry(ctx, doc.ID, now); err != nil {
				return err
			}
			return appendStatusHistoryDetail(ctx, u.audit, subjectDocument, doc.ID,
				codemeta.InvoiceDocumentStatusIssuing.Value,
				codemeta.InvoiceDocumentStatusPending.Value,
				codemeta.InvoiceManualReasonProviderRetryableError.Value,
				lastErr, "", now)
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
		return u.failDocument(ctx, doc, op, failureClassRetryExhausted,
			codemeta.InvoiceManualReasonRetryBudgetExhausted.Value, apiErr.Code, lastErr, now)
	})
}

func (u *Issuance) settlePermanent(ctx context.Context, doc domain.Document, op domain.Operation, providerCode, providerMessage, lastError string, now time.Time) error {
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
		return u.failDocument(ctx, doc, op, failureClassPermanent,
			codemeta.InvoiceManualReasonProviderRejectedPermanent.Value, providerCode, lastError, now)
	})
}

func (u *Issuance) settleSellerAction(ctx context.Context, doc domain.Document, op domain.Operation, apiErr *amego.APIError, now time.Time) error {
	return u.tx.WithTx(ctx, func(ctx context.Context) error {
		ok, err := u.ops.MarkSellerActionRequired(ctx, domain.MarkOperationProviderResult{
			ID:              op.ID,
			ProviderCode:    apiErr.Code,
			ProviderMessage: apiErr.Msg,
			LastError:       apiErr.Error(),
			Now:             now,
		})
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("invoice: operation %d could not enter SELLER_ACTION_REQUIRED (rows=0)", op.ID)
		}
		if err := appendStatusHistoryDetail(ctx, u.audit, subjectOperation, op.ID,
			codemeta.InvoiceOperationStatusSending.Value,
			codemeta.InvoiceOperationStatusSellerActionRequired.Value,
			codemeta.InvoiceManualReasonSellerTrackInsufficient.Value,
			apiErr.Error(), "", now); err != nil {
			return err
		}
		return u.failDocument(ctx, doc, op, failureClassPermanent,
			codemeta.InvoiceManualReasonSellerTrackInsufficient.Value, apiErr.Code, apiErr.Error(), now)
	})
}

func (u *Issuance) settleUnknown(ctx context.Context, doc domain.Document, op domain.Operation, providerCode, lastError string, now time.Time) error {
	_ = doc
	return u.tx.WithTx(ctx, func(ctx context.Context) error {
		ok, err := u.ops.MarkWaitingConfirmation(ctx, domain.MarkOperationUnknown{
			ID:            op.ID,
			NextAttemptAt: now.Add(u.cfg.RecheckBase),
			ProviderCode:  providerCode,
			LastError:     lastError,
			Now:           now,
		})
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("invoice: operation %d could not enter WAITING_CONFIRMATION (rows=0)", op.ID)
		}
		return appendStatusHistoryDetail(ctx, u.audit, subjectOperation, op.ID,
			codemeta.InvoiceOperationStatusSending.Value,
			codemeta.InvoiceOperationStatusWaitingConfirmation.Value,
			codemeta.InvoiceManualReasonProviderResultUnknown.Value,
			lastError, "", now)
	})
}

const (
	failureClassPermanent      = "PERMANENT"
	failureClassRetryExhausted = "RETRYABLE"
)

func (u *Issuance) failDocument(ctx context.Context, doc domain.Document, op domain.Operation, failureClass, reasonCode, providerCode, detail string, now time.Time) error {
	ok, err := u.docs.MarkIssueFailed(ctx, doc.ID, reasonCode, now)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("invoice: document %d could not enter ISSUE_FAILED (rows=0, status=%s)", doc.ID, doc.Status)
	}
	if err := appendStatusHistoryDetail(ctx, u.audit, subjectDocument, doc.ID,
		codemeta.InvoiceDocumentStatusIssuing.Value,
		codemeta.InvoiceDocumentStatusIssueFailed.Value, reasonCode, detail, "", now); err != nil {
		return err
	}
	issuer, err := u.issuers.GetByID(ctx, doc.IssuerID)
	if err != nil {
		return err
	}
	payload, err := u.failedEventPayload(issuer, doc, op, failureClass, reasonCode, providerCode, now)
	if err != nil {
		return err
	}
	if _, err = u.outbox.Insert(ctx, domain.NewOutboxEvent{
		AggregateType: outboxAggregateD,
		AggregateID:   doc.ID,
		EventType:     string(pkgmq.EventTypeInvoiceIssueFailed),
		Exchange:      pkgmq.ExchangeInvoice,
		RoutingKey:    pkgmq.RoutingKeyInvoiceIssueFailed,
		Payload:       payload,
		NextAttemptAt: now,
		Now:           now,
	}); err != nil {
		return err
	}
	return u.markReissueFailedIfAny(ctx, doc, now)
}

func (u *Issuance) markReissueFailedIfAny(ctx context.Context, doc domain.Document, now time.Time) error {
	if doc.OriginalDocumentID == nil {
		return nil
	}
	corr, err := u.corrections.GetOpenByDocument(ctx, *doc.OriginalDocumentID)
	if err != nil {
		if isBizCode(err, bizcode.InvoiceCorrectionNotAllowed) {
			return nil
		}
		return err
	}
	ok, err := u.corrections.MarkReissueFailed(ctx, corr.ID, now)
	if err != nil {
		return err
	}
	if !ok {
		u.log.Info("invoice.reissue.mark_failed_skipped",
			zap.Int64("correction_id", corr.ID), zap.String("status", corr.Status))
		return nil
	}
	u.log.Error("invoice.reissue.failed",
		zap.Int64("correction_id", corr.ID),
		zap.Int64("original_document_id", *doc.OriginalDocumentID),
		zap.Int64("reissue_document_id", doc.ID))
	return appendStatusHistory(ctx, u.audit, subjectCorrectionRequest, corr.ID,
		corr.Status, codemeta.InvoiceCorrectionStatusReissueFailed.Value, "", "", now)
}

func buildIssueRequest(doc domain.Document, lines []domain.DocumentLine, secrets domain.RecipientSecrets) (amego.IssueRequest, error) {
	items := make([]amego.IssueProductItem, 0, len(lines))
	for _, l := range lines {
		if l.NetAmount == 0 {
			continue
		}
		taxType, err := domain.TaxCategoryToAmego(domain.TaxCategory(l.TaxCategory))
		if err != nil {
			return amego.IssueRequest{}, err
		}
		items = append(items, amego.IssueProductItem{
			Description: l.Description,
			Quantity:    strconv.FormatInt(int64(l.Quantity), 10),
			UnitPrice:   strconv.FormatInt(l.UnitPrice, 10),
			Amount:      strconv.FormatInt(l.NetAmount, 10),
			Remark:      derefString(l.Remark),
			TaxType:     taxType,
		})
	}

	return amego.IssueRequest{
		OrderID:           derefString(doc.ProviderRequestNo),
		BuyerIdentifier:   doc.RecipientTaxID,
		BuyerName:         doc.RecipientName,
		BuyerEmailAddress: secrets.Email,
		CarrierType:       derefString(doc.CarrierType),
		CarrierID1:        secrets.CarrierID1,
		CarrierID2:        secrets.CarrierID2,
		NPOBAN:            derefString(doc.DonationCode),
		ProductItem:       items,

		SalesAmount:        strconv.FormatInt(doc.SalesAmount, 10),
		FreeTaxSalesAmount: strconv.FormatInt(doc.FreeTaxSalesAmount, 10),
		ZeroTaxSalesAmount: strconv.FormatInt(doc.ZeroTaxSalesAmount, 10),
		TaxAmount:          strconv.FormatInt(doc.TaxAmount, 10),
		TotalAmount:        strconv.FormatInt(doc.TotalAmount, 10),
		TaxType:            strconv.FormatInt(int64(doc.TaxType), 10),
		TaxRate:            domain.TaxRateLiteral,
		DetailVat:          strconv.Itoa(domain.DetailVatInclusive),
		DetailAmountRound:  strconv.Itoa(domain.DetailAmountRoundOn),
	}, nil
}

type issuedEvent struct {
	NotificationRefID string            `json:"notification_ref_id"`
	InvoiceDocumentID string            `json:"invoice_document_id"`
	OrderNo           string            `json:"order_no"`
	BuyerID           eventwire.BuyerID `json:"buyer_id"`
	StoreID           eventwire.StoreID `json:"store_id"`
	DocumentKind      string            `json:"document_kind"`
	InvoiceNo         string            `json:"invoice_no"`
	InvoiceDate       string            `json:"invoice_date"`
	TotalAmount       string            `json:"total_amount"`
	HasCarrier        bool              `json:"has_carrier"`
	HasBuyerTaxID     bool              `json:"has_buyer_tax_id"`
	OccurredAt        string            `json:"occurred_at"`
}

type reissuedEvent struct {
	NotificationRefID string            `json:"notification_ref_id"`
	InvoiceDocumentID string            `json:"invoice_document_id"`
	OrderNo           string            `json:"order_no"`
	BuyerID           eventwire.BuyerID `json:"buyer_id"`
	StoreID           eventwire.StoreID `json:"store_id"`
	VoidedDocumentID  string            `json:"voided_document_id"`
	VoidedInvoiceNo   string            `json:"voided_invoice_no"`
	NewInvoiceNo      string            `json:"new_invoice_no"`
	OccurredAt        string            `json:"occurred_at"`
}

type issueFailedEvent struct {
	NotificationRefID  string            `json:"notification_ref_id"`
	InvoiceDocumentID  string            `json:"invoice_document_id"`
	InvoiceOperationID string            `json:"invoice_operation_id"`
	OrderNo            string            `json:"order_no"`
	BuyerID            eventwire.BuyerID `json:"buyer_id"`
	StoreID            eventwire.StoreID `json:"store_id"`
	FailureClass       string            `json:"failure_class"`
	FailureReasonCode  string            `json:"failure_reason_code"`
	ProviderErrorCode  string            `json:"provider_error_code"`
	OccurredAt         string            `json:"occurred_at"`
}

func (u *Issuance) issuedEventPayload(issuer domain.Issuer, doc domain.Document, res *amego.IssueResult, issuedAt *time.Time, now time.Time) (json.RawMessage, error) {
	invoiceDate := ""
	if doc.InvoiceDateTaipei != nil {
		invoiceDate = doc.InvoiceDateTaipei.Format("2006-01-02")
	}
	_ = issuedAt
	return json.Marshal(issuedEvent{
		NotificationRefID: "INV:" + strconv.FormatInt(doc.ID, 10),
		InvoiceDocumentID: strconv.FormatInt(doc.ID, 10),
		OrderNo:           derefString(doc.SourceRef),
		BuyerID:           eventwire.BuyerIDOfDocument(doc),
		StoreID:           eventwire.StoreIDOf(issuer),
		DocumentKind:      doc.DocumentKind,
		InvoiceNo:         res.InvoiceNumber,
		InvoiceDate:       invoiceDate,
		TotalAmount:       strconv.FormatInt(doc.TotalAmount, 10),
		HasCarrier:        doc.CarrierType != nil && *doc.CarrierType != "",
		HasBuyerTaxID:     doc.RecipientTaxID != domain.B2CBuyerIdentifier,
		OccurredAt:        timeutil.FormatISO8601(now),
	})
}

func (u *Issuance) reissuedEventPayload(
	issuer domain.Issuer, doc, original domain.Document, res *amego.IssueResult, now time.Time,
) (json.RawMessage, error) {
	return json.Marshal(reissuedEvent{
		NotificationRefID: "INV:" + strconv.FormatInt(doc.ID, 10),
		InvoiceDocumentID: strconv.FormatInt(doc.ID, 10),
		OrderNo:           derefString(doc.SourceRef),
		BuyerID:           eventwire.BuyerIDOfDocument(doc),
		StoreID:           eventwire.StoreIDOf(issuer),
		VoidedDocumentID:  strconv.FormatInt(original.ID, 10),
		VoidedInvoiceNo:   derefString(original.ProviderInvoiceNo),
		NewInvoiceNo:      res.InvoiceNumber,
		OccurredAt:        timeutil.FormatISO8601(now),
	})
}

func (u *Issuance) failedEventPayload(issuer domain.Issuer, doc domain.Document, op domain.Operation, failureClass, reasonCode, providerCode string, now time.Time) (json.RawMessage, error) {
	return json.Marshal(issueFailedEvent{
		NotificationRefID:  fmt.Sprintf("INVOP:%d:%d", op.ID, op.AttemptCount),
		InvoiceDocumentID:  strconv.FormatInt(doc.ID, 10),
		InvoiceOperationID: strconv.FormatInt(op.ID, 10),
		OrderNo:            derefString(doc.SourceRef),
		BuyerID:            eventwire.BuyerIDOfDocument(doc),
		StoreID:            eventwire.StoreIDOf(issuer),
		FailureClass:       failureClass,
		FailureReasonCode:  reasonCode,
		ProviderErrorCode:  providerCode,
		OccurredAt:         timeutil.FormatISO8601(now),
	})
}

func issuedRoutingKey(documentKind string) string {
	if documentKind == codemeta.InvoiceDocumentKindReceipt.Value {
		return pkgmq.RoutingKeyInvoiceReceiptIssued
	}
	return pkgmq.RoutingKeyInvoiceIssued
}

func issuedEventType(documentKind string) pkgmq.EventType {
	if documentKind == codemeta.InvoiceDocumentKindReceipt.Value {
		return pkgmq.EventTypeInvoiceReceiptIssued
	}
	return pkgmq.EventTypeInvoiceIssued
}

func backoffFor(attempt int, base, max time.Duration) time.Duration {
	d := base
	for i := 0; i < attempt; i++ {
		d *= 2
		if d >= max {
			return max
		}
	}
	return d
}
