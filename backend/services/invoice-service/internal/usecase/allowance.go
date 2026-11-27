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
	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/go-pkg/timeutil"
	"github.com/Will413028/hawkr/invoice-service/internal/client/amego"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/eventwire"
	"go.uber.org/zap"
)

// Allowance 是折讓軸。
//
// 它與 Voiding 分開而不是加一個 if：兩者送的 payload、落的狀態、要維護的反正規化欄位
// 完全不同（折讓要累加 allowance_total_amount 與逐行 allowanced_amount，作廢不用）。
type Allowance struct {
	tx          database.TxManager
	docs        domain.DocumentRepository
	ops         domain.OperationRepository
	adjustments domain.AdjustmentRepository
	accounts    domain.ProviderAccountRepository
	issuers     domain.IssuerRepository
	audit       domain.AuditRepository
	outbox      domain.OutboxRepository
	provider    ProviderClient

	now         func() time.Time
	cfg         IssuanceConfig
	b2cSplitTax bool
	log         *zap.Logger
}

// NewAllowance 組出折讓軸。cfg 與 Issuance／Voiding 共用同一份 —— 三者搶的是
// **同一張表的同一組候選**，租約長度與退避必須一致。
func NewAllowance(
	tx database.TxManager,
	docs domain.DocumentRepository,
	ops domain.OperationRepository,
	adjustments domain.AdjustmentRepository,
	accounts domain.ProviderAccountRepository,
	issuers domain.IssuerRepository,
	audit domain.AuditRepository,
	outbox domain.OutboxRepository,
	provider ProviderClient,
	cfg IssuanceConfig,
	b2cSplitTax bool,
	log *zap.Logger,
) *Allowance {
	return &Allowance{
		tx: tx, docs: docs, ops: ops, adjustments: adjustments,
		accounts: accounts, issuers: issuers, audit: audit, outbox: outbox, provider: provider,
		now:         func() time.Time { return time.Now().UTC() },
		cfg:         cfg,
		b2cSplitTax: b2cSplitTax,
		log:         orNop(log),
	}
}

// WithClock 讓測試注入時鐘。
func (u *Allowance) WithClock(now func() time.Time) *Allowance {
	if now != nil {
		u.now = now
	}
	return u
}

// ClaimForAllowanceAt 是 worker 的租約閘門（形狀與 ClaimForVoidAt 相同）。
//
// 🔴 `ops.Claim` 的簽章是 `(ctx, id, now, leaseUntil)`，兩個 time.Time **不可對調**：
// `now` 落在 `started_at`／`updated_at` 與候選述詞的 `next_attempt_at <= claimable_before`，
// `leaseUntil` 落在 `next_attempt_at`（＝租約本身）。對調的後果是
// `next_attempt_at` 被寫成**當下** ⇒ 租約長度變 0 ⇒ 這一列在副作用還在飛的時候就已經
// 回到候選集合，而 `MarkInvoiceAdjustmentSending` 的 allowed-from 刻意含 `SENDING` 自己
// ⇒ adjustment 那一層攔不住第二個認領者 ⇒ 同一筆折讓打第二次 AMEGO。
// 兩個參數同型別，編譯器不會出聲；dev 上的指紋是 `started_at − updated_at ＝ +ClaimLease`
// （2026-09-05 實查 ALLOWANCE 5 列全中，而 VOID 15 列全無）。
func (u *Allowance) ClaimForAllowanceAt(
	ctx context.Context, operationID int64, now, leaseUntil time.Time,
) (bool, error) {
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

		adj, err := u.adjustmentOf(ctx, op)
		if err != nil {
			return err
		}
		ok, err = u.adjustments.MarkSending(ctx, adj.ID, now)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("invoice: adjustment %d could not enter SENDING (rows=0, status=%s)",
				adj.ID, adj.Status)
		}
		if err := appendStatusHistory(ctx, u.audit, subjectAdjustment, adj.ID,
			adj.Status, codemeta.InvoiceAdjustmentStatusSending.Value, "", "", now); err != nil {
			return err
		}
		claimed = true
		return nil
	})
	return claimed, err
}

// SubmitAllowance 送一張折讓單（G08 → G09／G10／G11／G12）。
//
// 契約與 SubmitVoid 逐字相同：回 error 只代表「這一輪連狀態都沒寫成」（DB 斷線之類），
// 所有業務落點都回 nil —— 狀態已經寫定。兩者共用同一個 worker，契約不一致會讓
// worker 的 attempt 簿記打在錯的前提上。
func (u *Allowance) SubmitAllowance(ctx context.Context, operationID int64) error {
	op, err := u.ops.GetByID(ctx, operationID)
	if err != nil {
		return err
	}
	if op.Status != codemeta.InvoiceOperationStatusSending.Value {
		return fmt.Errorf("invoice: operation %d is %s, expected SENDING (claim first)", op.ID, op.Status)
	}

	adj, err := u.adjustmentOf(ctx, op)
	if err != nil {
		return err
	}
	if adj.Status == codemeta.InvoiceAdjustmentStatusCompleted.Value {
		return u.finishOperation(ctx, op, u.now())
	}

	doc, err := u.docs.GetByID(ctx, adj.DocumentID)
	if err != nil {
		return err
	}
	if doc.ProviderInvoiceNo == nil || *doc.ProviderInvoiceNo == "" {
		return u.settleAllowancePermanent(ctx, adj, op,
			codemeta.InvoiceManualReasonAllowanceNoSourceInvoice.Value, "", "",
			"原發票沒有 provider_invoice_no，折讓無所依附", u.now())
	}

	account, err := u.accounts.GetByIssuer(ctx, doc.IssuerID, providerAmego)
	if err != nil {
		return err
	}

	lines, err := u.adjustments.ListLines(ctx, adj.ID)
	if err != nil {
		return err
	}
	if len(lines) == 0 {
		return u.settleAllowancePermanent(ctx, adj, op,
			codemeta.InvoiceManualReasonLineCountInvalid.Value, "", "",
			"折讓明細為空", u.now())
	}

	req := allowanceRequestOf(adj, doc, lines)

	var sendErr error
	auditCtx := amego.WithAuditContext(ctx, op.ID, int(op.AttemptCount)+1)
	if err := u.accounts.WithDecryptedCredential(ctx, account.ID, func(providerTaxID, appKey string) error {
		_, sendErr = u.provider.IssueAllowance(auditCtx,
			amego.Credential{IssuerID: doc.IssuerID, Invoice: providerTaxID, AppKey: appKey}, req)
		return nil
	}); err != nil {
		return err
	}

	now := u.now()
	if sendErr == nil {
		return u.settleAllowanceCompleted(ctx, adj, doc, op, now)
	}
	return u.settleAllowanceFailure(ctx, adj, doc, op, sendErr, now)
}

func allowanceRequestOf(
	adj domain.Adjustment, doc domain.Document, lines []domain.AdjustmentLine,
) amego.AllowanceRequest {
	items := make([]amego.AllowanceProductItem, 0, len(lines))
	for _, l := range lines {
		date := ""
		if l.OriginalInvoiceDate != nil {
			date = l.OriginalInvoiceDate.Format(amegoDateLayout)
		}
		items = append(items, amego.AllowanceProductItem{
			OriginalInvoiceNumber: l.OriginalInvoiceNo,
			OriginalInvoiceDate:   date,
			OriginalDescription:   l.OriginalDescription,
			Quantity:              strconv.FormatInt(int64(l.Quantity), 10),
			UnitPrice:             strconv.FormatInt(l.UnitPriceExTax, 10),
			Amount:                strconv.FormatInt(l.NetAmountExTax, 10),
			Tax:                   strconv.FormatInt(l.TaxAmount, 10),
			TaxType:               strconv.FormatInt(int64(l.TaxCategory), 10),
		})
	}
	allowanceDate := ""
	if adj.AllowanceDateTaipei != nil {
		allowanceDate = adj.AllowanceDateTaipei.Format(amegoDateLayout)
	}
	return amego.AllowanceRequest{
		AllowanceNumber: derefString(adj.AllowanceNumber),
		AllowanceDate:   allowanceDate,
		AllowanceType:   allowanceTypeBuyerIssued,
		BuyerIdentifier: doc.RecipientTaxID,
		BuyerName:       doc.RecipientName,
		ProductItem:     items,
		TaxAmount:       strconv.FormatInt(derefInt64(adj.TaxAmount), 10),
		TotalAmount:     strconv.FormatInt(derefInt64(adj.NetAmountExTax), 10),
	}
}

const (
	amegoDateLayout          = "20060102"
	allowanceTypeBuyerIssued = "2"
)

func (u *Allowance) settleAllowanceCompleted(
	ctx context.Context, adj domain.Adjustment, doc domain.Document, op domain.Operation, now time.Time,
) error {
	issuer, err := u.issuers.GetByID(ctx, doc.IssuerID)
	if err != nil {
		return err
	}
	payload, err := u.allowanceIssuedPayload(issuer, doc, adj, now)
	if err != nil {
		return err
	}

	return u.tx.WithTx(ctx, func(ctx context.Context) error {
		if _, err := u.outbox.Insert(ctx, domain.NewOutboxEvent{
			AggregateType: outboxAggregateAdjustment,
			AggregateID:   adj.ID,
			EventType:     string(pkgmq.EventTypeInvoiceAllowanceIssued),
			Exchange:      pkgmq.ExchangeInvoice,
			RoutingKey:    pkgmq.RoutingKeyInvoiceAllowanceIssued,
			Payload:       payload,
			NextAttemptAt: now,
			Now:           now,
		}); err != nil {
			return err
		}

		ok, err := u.adjustments.MarkCompleted(ctx, domain.MarkAdjustmentCompleted{ID: adj.ID, Now: now})
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("invoice: adjustment %d could not enter COMPLETED (rows=0, status=%s)",
				adj.ID, adj.Status)
		}

		ok, err = u.docs.AddAllowanceTotal(ctx, doc.ID, derefInt64(adj.GrossAmount), now)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf(
				"invoice: document %d allowance cap exceeded (adjustment %d, delta %d, already %d, total %d)",
				doc.ID, adj.ID, derefInt64(adj.GrossAmount), doc.AllowanceTotalAmount, doc.TotalAmount)
		}

		lines, err := u.adjustments.ListLines(ctx, adj.ID)
		if err != nil {
			return err
		}
		for _, l := range lines {
			if l.SourceLineID == nil {
				continue
			}
			ok, err := u.docs.AddLineAllowancedAmount(ctx, *l.SourceLineID,
				l.NetAmountExTax+l.TaxAmount, now)
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("invoice: line %d allowanced cap exceeded (adjustment %d)", *l.SourceLineID, adj.ID)
			}
		}

		if err := appendStatusHistory(ctx, u.audit, subjectAdjustment, adj.ID,
			adj.Status, codemeta.InvoiceAdjustmentStatusCompleted.Value, "", "", now); err != nil {
			return err
		}
		return u.finishOperationTx(ctx, op, now)
	})
}

func (u *Allowance) settleAllowanceFailure(
	ctx context.Context, adj domain.Adjustment, doc domain.Document,
	op domain.Operation, sendErr error, now time.Time,
) error {
	var apiErr *amego.APIError
	if !errors.As(sendErr, &apiErr) {
		return u.settleAllowanceUnknown(ctx, adj, op, now)
	}
	switch apiErr.Class {
	case amego.ClassRetryable:
		return u.settleAllowanceRetry(ctx, adj, op, apiErr, now)
	case amego.ClassReconcile:
		return u.settleAllowanceUnknown(ctx, adj, op, now)
	case amego.ClassManual, amego.ClassPermanent:
		return u.settleAllowancePermanent(ctx, adj, op,
			codemeta.InvoiceManualReasonProviderRejectedPermanent.Value,
			apiErr.Code, apiErr.Msg, apiErr.Error(), now)
	default:
		return u.settleAllowanceUnknown(ctx, adj, op, now)
	}
}

func (u *Allowance) settleAllowanceRetry(
	ctx context.Context, adj domain.Adjustment, op domain.Operation, apiErr *amego.APIError, now time.Time,
) error {
	newAttempt := op.AttemptCount + 1
	exhausted := newAttempt >= op.MaxAttempts
	delay := backoffFor(int(newAttempt), u.cfg.BaseBackoff, u.cfg.MaxBackoff)
	lastErr := apiErr.Error()

	return u.tx.WithTx(ctx, func(ctx context.Context) error {
		if err := u.ops.UpdateAttempt(ctx, op.ID, newAttempt, now.Add(delay), &lastErr, now); err != nil {
			return err
		}
		ok, err := u.adjustments.Reschedule(ctx, adj.ID, now)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("invoice: adjustment %d could not go back to PENDING (rows=0, status=%s)",
				adj.ID, adj.Status)
		}
		if err := appendStatusHistoryDetail(ctx, u.audit, subjectAdjustment, adj.ID,
			adj.Status, codemeta.InvoiceAdjustmentStatusPending.Value, "", lastErr, "", now); err != nil {
			return err
		}
		if !exhausted {
			return u.rescheduleOperationTx(ctx, op, newAttempt, now.Add(delay), lastErr, now)
		}
		return u.exhaustOperationTx(ctx, op, lastErr, now)
	})
}

func (u *Allowance) settleAllowancePermanent(
	ctx context.Context, adj domain.Adjustment, op domain.Operation,
	reasonCode, providerCode, providerMessage, detail string, now time.Time,
) error {
	return u.tx.WithTx(ctx, func(ctx context.Context) error {
		ok, err := u.adjustments.MarkFailedPermanent(ctx, adj.ID, reasonCode, now)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("invoice: adjustment %d could not enter FAILED_PERMANENT (rows=0, status=%s)",
				adj.ID, adj.Status)
		}
		if err := appendStatusHistoryDetail(ctx, u.audit, subjectAdjustment, adj.ID,
			adj.Status, codemeta.InvoiceAdjustmentStatusFailedPermanent.Value, reasonCode, detail, "", now); err != nil {
			return err
		}
		return u.failOperationTx(ctx, op, reasonCode, providerCode, providerMessage, detail, now)
	})
}

func (u *Allowance) settleAllowanceUnknown(
	ctx context.Context, adj domain.Adjustment, op domain.Operation, now time.Time,
) error {
	return u.tx.WithTx(ctx, func(ctx context.Context) error {
		ok, err := u.adjustments.MarkWaitingConfirmation(ctx, adj.ID, now)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("invoice: adjustment %d could not enter WAITING_CONFIRMATION (rows=0, status=%s)",
				adj.ID, adj.Status)
		}
		if err := appendStatusHistory(ctx, u.audit, subjectAdjustment, adj.ID,
			adj.Status, codemeta.InvoiceAdjustmentStatusWaitingConfirmation.Value, "", "", now); err != nil {
			return err
		}
		return u.waitOperationTx(ctx, op, now)
	})
}

type allowanceIssuedEvent struct {
	NotificationRefID      string            `json:"notification_ref_id"`
	InvoiceDocumentID      string            `json:"invoice_document_id"`
	AdjustmentID           string            `json:"adjustment_id"`
	OrderNo                string            `json:"order_no"`
	BuyerID                eventwire.BuyerID `json:"buyer_id"`
	StoreID                eventwire.StoreID `json:"store_id"`
	AllowanceNumber        string            `json:"allowance_number"`
	AllowanceAmount        string            `json:"allowance_amount"`
	CumulativeAllowanceAmt string            `json:"cumulative_allowance_amount"`
	InvoiceNo              string            `json:"invoice_no"`
	OccurredAt             string            `json:"occurred_at"`
}

func (u *Allowance) allowanceIssuedPayload(
	issuer domain.Issuer, doc domain.Document, adj domain.Adjustment, now time.Time,
) (json.RawMessage, error) {
	gross := derefInt64(adj.GrossAmount)
	return json.Marshal(allowanceIssuedEvent{
		NotificationRefID:      "INV:" + strconv.FormatInt(adj.ID, 10),
		InvoiceDocumentID:      strconv.FormatInt(doc.ID, 10),
		AdjustmentID:           strconv.FormatInt(adj.ID, 10),
		OrderNo:                derefString(doc.SourceRef),
		BuyerID:                eventwire.BuyerIDOfDocument(doc),
		StoreID:                eventwire.StoreIDOf(issuer),
		AllowanceNumber:        derefString(adj.AllowanceNumber),
		AllowanceAmount:        strconv.FormatInt(gross, 10),
		CumulativeAllowanceAmt: strconv.FormatInt(doc.AllowanceTotalAmount+gross, 10),
		InvoiceNo:              derefString(doc.ProviderInvoiceNo),
		OccurredAt:             timeutil.FormatISO8601(now),
	})
}

func (u *Allowance) adjustmentOf(ctx context.Context, op domain.Operation) (domain.Adjustment, error) {
	if op.AdjustmentID == nil {
		return domain.Adjustment{}, fmt.Errorf(
			"invoice: operation %d has no adjustment_id (type=%s)", op.ID, op.OperationType)
	}
	return u.adjustments.GetByID(ctx, *op.AdjustmentID)
}

func (u *Allowance) finishOperation(ctx context.Context, op domain.Operation, now time.Time) error {
	return u.tx.WithTx(ctx, func(ctx context.Context) error {
		return u.finishOperationTx(ctx, op, now)
	})
}

func (u *Allowance) finishOperationTx(ctx context.Context, op domain.Operation, now time.Time) error {
	return finishOperationSucceeded(ctx, u.ops, u.audit, op, now)
}

func (u *Allowance) rescheduleOperationTx(
	ctx context.Context, op domain.Operation, attempt int32, nextAt time.Time, lastErr string, now time.Time,
) error {
	_ = attempt
	_ = nextAt
	return appendStatusHistoryDetail(ctx, u.audit, subjectOperation, op.ID,
		op.Status, codemeta.InvoiceOperationStatusRetryScheduled.Value, "", lastErr, "", now)
}

func (u *Allowance) exhaustOperationTx(
	ctx context.Context, op domain.Operation, lastErr string, now time.Time,
) error {
	if _, err := u.ops.MarkRetryExhausted(ctx, op.ID, lastErr, now); err != nil {
		return err
	}
	return appendStatusHistoryDetail(ctx, u.audit, subjectOperation, op.ID,
		op.Status, codemeta.InvoiceOperationStatusFailedRetryExhausted.Value, "", lastErr, "", now)
}

func (u *Allowance) failOperationTx(
	ctx context.Context, op domain.Operation, reasonCode, providerCode, providerMessage, detail string, now time.Time,
) error {
	if _, err := u.ops.MarkFailedWithProviderResult(ctx, domain.MarkOperationProviderResult{
		ID:              op.ID,
		ProviderCode:    providerCode,
		ProviderMessage: providerMessage,
		LastError:       detail,
		Now:             now,
	}); err != nil {
		return err
	}
	return appendStatusHistoryDetail(ctx, u.audit, subjectOperation, op.ID,
		op.Status, codemeta.InvoiceOperationStatusFailedPermanent.Value, reasonCode, detail, "", now)
}

func (u *Allowance) waitOperationTx(ctx context.Context, op domain.Operation, now time.Time) error {
	ok, err := u.ops.MarkWaitingConfirmation(ctx, domain.MarkOperationUnknown{
		ID:            op.ID,
		NextAttemptAt: now.Add(u.cfg.RecheckBase),
		LastError:     "allowance result unknown",
		Now:           now,
	})
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf(
			"invoice: operation %d could not enter WAITING_CONFIRMATION (rows=0, snapshot status=%s)",
			op.ID, op.Status)
	}
	return appendStatusHistory(ctx, u.audit, subjectOperation, op.ID,
		op.Status, codemeta.InvoiceOperationStatusWaitingConfirmation.Value, "", "", now)
}

func adjustmentAlreadySettled(status string) bool {
	switch status {
	case codemeta.InvoiceAdjustmentStatusFailedUnknown.Value,
		codemeta.InvoiceAdjustmentStatusCompleted.Value,
		codemeta.InvoiceAdjustmentStatusCancelled.Value,
		codemeta.InvoiceAdjustmentStatusFailedPermanent.Value:
		return true
	default:
		return false
	}
}

const outboxAggregateAdjustment = "ADJUSTMENT"

// ConfirmAllowance 是折讓的「結果未明」收斂路徑，由 INV.rc 分派進來。
//
// 🔴 折讓單**不能**走 recheck 預設的 probeOperation：那條打的是 invoice_query
// type=order，會查到那張仍然存在的**原發票**並判成「開立成功」⇒ 把折讓 operation
// 標成完成，而折讓到底開沒開立沒有人知道。折讓只有一種問法——拿我方自產的
// allowance_number 打 allowance_query（AMEGO 開立折讓不回傳任何識別碼，
// 所以那個號碼是「送出成功但回應遺失」時唯一的救回路徑，見 InsertInvoiceAdjustment）。
// 同一形狀的坑在作廢軸已經踩過一次，見 RecheckVoidConfirmer 的說明。
func (u *Allowance) ConfirmAllowance(ctx context.Context, op domain.Operation) error {
	adj, err := u.adjustmentOf(ctx, op)
	if err != nil {
		return err
	}
	if adj.Status == codemeta.InvoiceAdjustmentStatusCompleted.Value {
		return u.finishOperation(ctx, op, u.now())
	}
	if adj.AllowanceNumber == nil || *adj.AllowanceNumber == "" {
		return u.settleAllowancePermanent(ctx, adj, op,
			codemeta.InvoiceManualReasonAllowanceNumberMissing.Value, "", "",
			"折讓單缺 allowance_number，無法查詢確認", u.now())
	}

	doc, err := u.docs.GetByID(ctx, adj.DocumentID)
	if err != nil {
		return err
	}
	account, err := u.accounts.GetByIssuer(ctx, doc.IssuerID, providerAmego)
	if err != nil {
		return err
	}

	var detail *amego.AllowanceDetail
	var queryErr error
	auditCtx := amego.WithAuditContext(ctx, op.ID, int(op.AttemptCount)+1)
	if err := u.accounts.WithDecryptedCredential(ctx, account.ID, func(providerTaxID, appKey string) error {
		detail, queryErr = u.provider.QueryAllowance(auditCtx,
			amego.Credential{IssuerID: doc.IssuerID, Invoice: providerTaxID, AppKey: appKey},
			*adj.AllowanceNumber)
		return nil
	}); err != nil {
		return err
	}

	now := u.now()
	if queryErr != nil || detail == nil || detail.AllowanceNumber == "" {
		u.log.Warn("invoice.allowance.confirm_query_failed",
			zap.Int64("operation_id", op.ID), zap.Int64("adjustment_id", adj.ID), zap.Error(queryErr))
		if op.CancelRequestedAt != nil && op.CancelConfirmedAt == nil {
			return u.settleAllowanceAdminCancel(ctx, adj, op, allowanceQueryNotFound(queryErr), now)
		}
		return u.scheduleAllowanceRecheck(ctx, adj, op, now)
	}

	return u.settleAllowanceCompleted(ctx, adj, doc, op, now)
}

func allowanceQueryNotFound(err error) bool {
	var apiErr *amego.APIError
	if errors.As(err, &apiErr) {
		return apiErr.NotFound
	}
	return false
}

func (u *Allowance) settleAllowanceAdminCancel(
	ctx context.Context, adj domain.Adjustment, op domain.Operation, notIssued bool, now time.Time,
) error {
	if adj.Status != codemeta.InvoiceAdjustmentStatusCancelled.Value {
		u.log.Warn("invoice.allowance.admin_cancel_adjustment_not_settled",
			zap.Int64("operation_id", op.ID), zap.Int64("adjustment_id", adj.ID),
			zap.String("adjustment_status", adj.Status))
	}

	if notIssued {
		ev := codemeta.InvoiceCancelEvidenceKindProviderQuery71.Value
		completed := now
		confirmed := now
		next := codemeta.InvoiceOperationStatusCancelledByAdmin.Value
		ok, err := u.ops.MarkRecheckResolved(ctx, domain.MarkRecheckResult{
			ID:                 op.ID,
			NextStatus:         next,
			NextAttemptAt:      now,
			CompletedAt:        &completed,
			CancelEvidenceKind: &ev,
			CancelConfirmedAt:  &confirmed,
			Now:                now,
		})
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf(
				"invoice: operation %d could not enter CANCELLED_BY_ADMIN (rows=0, snapshot status=%s)",
				op.ID, op.Status)
		}
		u.log.Info("invoice.allowance.admin_cancel_confirmed",
			zap.Int64("operation_id", op.ID), zap.Int64("adjustment_id", adj.ID))
		return appendStatusHistory(ctx, u.audit, subjectOperation, op.ID,
			op.Status, next, recheckReasonCode, "", now)
	}

	elapsed := time.Duration(0)
	if op.UnknownSince != nil {
		elapsed = now.Sub(*op.UnknownSince)
	}
	next := codemeta.InvoiceOperationStatusCancelBlockedUnknown.Value
	lastErr := "allowance cancellation blocked: provider result still unknown"
	ok, err := u.ops.MarkRecheckUnresolved(ctx, domain.MarkRecheckResult{
		ID:            op.ID,
		NextStatus:    next,
		NextAttemptAt: now.Add(recheckBackoff(elapsed)),
		LastError:     &lastErr,
		Now:           now,
	})
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf(
			"invoice: operation %d could not enter CANCEL_BLOCKED_UNKNOWN (rows=0, snapshot status=%s)",
			op.ID, op.Status)
	}
	return appendStatusHistoryDetail(ctx, u.audit, subjectOperation, op.ID,
		op.Status, next, recheckReasonCode, lastErr, "", now)
}

func (u *Allowance) scheduleAllowanceRecheck(
	ctx context.Context, adj domain.Adjustment, op domain.Operation, now time.Time,
) error {
	elapsed := time.Duration(0)
	if op.UnknownSince != nil {
		elapsed = now.Sub(*op.UnknownSince)
	}
	if elapsed <= recheckGiveUpAfter {
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

	lastErr := "invoice: allowance confirmation still unknown after give-up window"
	return u.tx.WithTx(ctx, func(ctx context.Context) error {
		ok, err := u.adjustments.MarkFailedUnknown(ctx, adj.ID, now)
		if err != nil {
			return err
		}
		switch {
		case ok:
			if err := appendStatusHistoryDetail(ctx, u.audit, subjectAdjustment, adj.ID,
				adj.Status, codemeta.InvoiceAdjustmentStatusFailedUnknown.Value, "", lastErr, "", now); err != nil {
				return err
			}
		case adjustmentAlreadySettled(adj.Status):
			u.log.Info("invoice.allowance.give_up_adjustment_already_settled",
				zap.Int64("operation_id", op.ID), zap.Int64("adjustment_id", adj.ID),
				zap.String("adjustment_status", adj.Status))
		default:
			return fmt.Errorf("invoice: adjustment %d could not enter FAILED_UNKNOWN (rows=0, status=%s)",
				adj.ID, adj.Status)
		}
		ok, err = u.ops.MarkRecheckUnresolved(ctx, domain.MarkRecheckResult{
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
		u.log.Error("invoice.allowance.confirm_gave_up",
			zap.Int64("operation_id", op.ID), zap.Int64("adjustment_id", adj.ID),
			zap.Duration("elapsed", elapsed))
		return appendStatusHistoryDetail(ctx, u.audit, subjectOperation, op.ID,
			op.Status, codemeta.InvoiceOperationStatusFailedUnknown.Value, "", lastErr, "", now)
	})
}
