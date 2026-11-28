package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/timeutil"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"go.uber.org/zap"
)

// RefundAdjustmentInput 是套用一則退款所需的全部輸入。
//
// 刻意不吃 mq 的型別：usecase 不得依賴 mq（方向相反，會循環），轉換由 mq 側的
// adapter 做。欄位語意逐字對齊 `internal/mq/ports.go` 的 `RefundFinalized`。
type RefundAdjustmentInput struct {
	OrderNo   string
	RefundRef string

	// RefundAmount 是**這一則**退款的金額（元，整數）。
	RefundAmount int64

	// CumulativeRefundAmount 是 order-service 在同一交易內以
	// `sumAmountByType(order_no, REFUND_ORIGINAL)` 算出的累計退款。
	// 🔴 它是 VOID vs ALLOWANCE 的判準（§6.3.5.4 步驟 3），不是參考值。
	CumulativeRefundAmount int64

	OrderTotalActualAmount int64
	OccurredAt             time.Time
}

type refundPlanKind int

const (
	refundPlanAllowance refundPlanKind = iota
	refundPlanVoid
	refundPlanRetryLater
	refundPlanSkip
)

type refundPlan struct {
	Kind        refundPlanKind
	Reason      string
	Disposition string
}

type refundFacts struct {
	Doc                    domain.Document
	RefundAmount           int64
	CumulativeRefundAmount int64

	IssueResultUnknown bool

	CorrectionInFlight bool

	AdjustmentInFlight bool

	Now time.Time
}

func planRefundAdjustment(f refundFacts) refundPlan {
	doc := f.Doc

	if doc.FulfillmentMode != codemeta.InvoiceFulfillmentModeAmegoAuto.Value {
		return refundPlan{Kind: refundPlanSkip,
			Disposition: codemeta.InvoiceInboxDispositionNotAmegoAuto.Value,
			Reason:      "fulfillment_mode=" + doc.FulfillmentMode + "，調整不由本服務執行"}
	}
	if doc.DocumentKind == codemeta.InvoiceDocumentKindReceipt.Value {
		return refundPlan{Kind: refundPlanSkip,
			Disposition: codemeta.InvoiceInboxDispositionNotAmegoAuto.Value,
			Reason:      "document_kind=RECEIPT，只重開或註記收據，不走發票調整"}
	}

	if f.CorrectionInFlight {
		return refundPlan{Kind: refundPlanRetryLater, Reason: "CORRECTION_IN_FLIGHT"}
	}

	if f.IssueResultUnknown {
		return refundPlan{Kind: refundPlanRetryLater, Reason: "ISSUE_RESULT_UNKNOWN"}
	}

	switch doc.Status {
	case codemeta.InvoiceDocumentStatusIssued.Value:
	case codemeta.InvoiceDocumentStatusPending.Value,
		codemeta.InvoiceDocumentStatusIssuing.Value,
		codemeta.InvoiceDocumentStatusIssueFailed.Value:
		return refundPlan{Kind: refundPlanRetryLater, Reason: "DOCUMENT_NOT_ISSUED_YET"}
	default:
		return refundPlan{Kind: refundPlanSkip,
			Disposition: codemeta.InvoiceInboxDispositionNoAdjustableDocument.Value,
			Reason:      "document.status=" + doc.Status + "，沒有可調整的發票；退款本身不受影響"}
	}

	if f.AdjustmentInFlight {
		return refundPlan{Kind: refundPlanRetryLater, Reason: "ADJUSTMENT_IN_FLIGHT"}
	}

	if f.Doc.AllowanceTotalAmount > 0 {
		return refundPlan{Kind: refundPlanAllowance, Reason: "HAS_EXISTING_ALLOWANCE"}
	}

	cumulative := f.CumulativeRefundAmount
	if own := doc.AllowanceTotalAmount + f.RefundAmount; own > cumulative {
		cumulative = own
	}
	if cumulative != doc.TotalAmount {
		return refundPlan{Kind: refundPlanAllowance, Reason: "PARTIAL_REFUND"}
	}

	if doc.TaxPeriodCode == nil || *doc.TaxPeriodCode == "" {
		return refundPlan{Kind: refundPlanAllowance, Reason: "NO_TAX_PERIOD_CODE"}
	}
	reported, err := domain.ReportedStatus(*doc.TaxPeriodCode, f.Now)
	if err != nil || reported != domain.ReportedStatusNotReported {
		return refundPlan{Kind: refundPlanAllowance, Reason: "REPORTED_STATUS_" + reported}
	}

	inWindow, err := domain.WithinVoidWindow(*doc.TaxPeriodCode, f.Now)
	if err != nil || !inWindow {
		return refundPlan{Kind: refundPlanAllowance, Reason: "PAST_VOID_LOCK"}
	}

	if doc.ProviderUploadStatus == nil || *doc.ProviderUploadStatus != amegoUploadCompleted {
		return refundPlan{Kind: refundPlanAllowance, Reason: "UPLOAD_NOT_COMPLETED"}
	}

	return refundPlan{Kind: refundPlanVoid, Reason: "FULL_REFUND_VOIDABLE"}
}

const amegoUploadCompleted int16 = 99

// RefundAdjustmentAdjustments 是本 usecase 需要的 adjustment 面。
type RefundAdjustmentAdjustments interface {
	GetByRefund(ctx context.Context, documentID int64, refundNo string) (domain.Adjustment, bool, error)
	Insert(ctx context.Context, in domain.NewAdjustment) (domain.Adjustment, bool, error)
	InsertLines(ctx context.Context, adjustmentID int64, lines []domain.NewAdjustmentLine, now time.Time) error
	NextAllowanceNumber(ctx context.Context) (string, error)
}

// RefundAdjustmentDocuments 是本 usecase 需要的 document 面。
type RefundAdjustmentDocuments interface {
	ListLines(ctx context.Context, documentID int64) ([]domain.DocumentLine, error)
}

// RefundAdjustmentOperations 是本 usecase 需要的 operation 面。
//
// ListByDocument 同時回答註 A（開票結果未確認）與 I5（correction 佔用）：一張 document
// 上的 operation 是個位數，兩個述詞都在記憶體判比兩句 SQL 便宜，而且判準看得見。
type RefundAdjustmentOperations interface {
	ListByDocument(ctx context.Context, documentID int64) ([]domain.Operation, error)
	Create(ctx context.Context, in domain.NewOperation) (domain.Operation, bool, error)

	// GetByKey 是 `Create` 回 inserted=false 之後的**分辨手段**（I-22）。
	//
	// 🔴 沒有它就只能猜，而猜錯的那一半是靜默資料遺失：
	// `InsertInvoiceOperation` 的 `ON CONFLICT DO NOTHING` 不指名 target，
	// rows=0 有兩種意義——同一把 key 重送（冪等）vs
	// `uq_invop_open_adjustment_per_document`（I5 序列閘門）被別的調整佔用。
	// 前者 return nil 正確；後者 return nil 會留下一張**沒有執行者**的折讓單
	// （adjustment 是 PENDING、operation 一列都沒有、inbox 標 PROCESSED）。
	GetByKey(ctx context.Context, operationKey string) (domain.Operation, error)

	// Promote 把剛建好的 operation 從 CREATED 推到 READY。
	//
	// 🔴 沒有它，那筆 operation **沒有任何執行者**：INV.op 的候選述詞是
	// status IN ('READY','RETRY_SCHEDULED','SENDING')，CREATED 不在裡面。
	Promote(ctx context.Context, id int64, nextStatus string, nextAttemptAt, now time.Time) (bool, error)
}

// RefundAdjustment 是 G01／G02／G03。
type RefundAdjustment struct {
	tx          database.TxManager
	adjustments RefundAdjustmentAdjustments
	docs        RefundAdjustmentDocuments
	ops         RefundAdjustmentOperations
	audit       domain.AuditRepository
	newID       func() int64
	cfg         IssuanceConfig
	splitTax    bool
	log         *zap.Logger
	now         func() time.Time
}

// NewRefundAdjustment 建 G01 usecase。
func NewRefundAdjustment(
	tx database.TxManager,
	adjustments RefundAdjustmentAdjustments,
	docs RefundAdjustmentDocuments,
	ops RefundAdjustmentOperations,
	audit domain.AuditRepository,
	newID func() int64,
	cfg IssuanceConfig,
	splitTax bool,
	log *zap.Logger,
	now func() time.Time,
) *RefundAdjustment {
	return &RefundAdjustment{
		tx: tx, adjustments: adjustments, docs: docs, ops: ops, audit: audit,
		newID: newID, cfg: cfg, splitTax: splitTax, log: orNop(log), now: now,
	}
}

// RefundSkip 是「這則退款不需要任何發票調整」的處置。
//
// 🔴 它與「處理完成」是**兩件不同的事**，不可合併成 return nil：
// PROCESSED 說「這則事件的業務動作做完了」，SKIPPED 說「刻意不做，理由是 X」。
// 合併之後，事後查「為什麼這筆退款沒有折讓單」只能靠猜——而那正是
// invoice_inbox_events 這張表存在的理由（§6.3.5.4 步驟 3 逐字：
// 「判定為不需要任何發票調整 ⇒ MarkInboxEventSkipped(id, disposition, reason, now)，
// **disposition 不得為空**」）。
type RefundSkip struct {
	// Disposition 是 CodeMeta InvoiceInboxDisposition 的值（可 GROUP BY）。
	Disposition string
	// Reason 是給人看的細節，落 disposition_reason。
	Reason string
}

// Apply 套用一則退款。
//
// 回傳值三態：
//   - (nil, nil)        建了調整（或冪等命中）⇒ 呼叫端標 PROCESSED
//   - (*RefundSkip, nil) 確定不需要任何調整 ⇒ 呼叫端標 SKIPPED ＋ disposition
//   - (nil, err)        這一輪沒做成 ⇒ 呼叫端標 FAILED
//
// 🔴 冪等走 `GetByRefund`（uq_invadj_allowance_refund 的反查），**不靠 INSERT 撞唯一鍵**：
// PostgreSQL 上撞唯一鍵會讓整個交易 aborted（25P02）。Insert 那一句另有
// `ON CONFLICT DO NOTHING`（不指名 target）當第二道防線，兩者都要。
func (u *RefundAdjustment) Apply(
	ctx context.Context, in RefundAdjustmentInput, doc domain.Document,
) (*RefundSkip, error) {
	if in.RefundRef == "" {
		return nil, fmt.Errorf("invoice: refund adjustment requires refund_ref (order_no=%s)", in.OrderNo)
	}
	if existing, found, err := u.adjustments.GetByRefund(ctx, doc.ID, in.RefundRef); err != nil {
		return nil, err
	} else if found {
		u.log.Info("invoice.refund_adjustment.already_applied",
			zap.String("refund_ref", in.RefundRef), zap.Int64("adjustment_id", existing.ID),
			zap.String("status", existing.Status))
		return nil, nil
	}

	opsOnDoc, err := u.ops.ListByDocument(ctx, doc.ID)
	if err != nil {
		return nil, err
	}
	now := u.now()
	plan := planRefundAdjustment(refundFacts{
		Doc:                    doc,
		RefundAmount:           in.RefundAmount,
		CumulativeRefundAmount: in.CumulativeRefundAmount,
		IssueResultUnknown:     issueResultUnknown(doc, opsOnDoc),
		CorrectionInFlight:     correctionInFlight(opsOnDoc),
		AdjustmentInFlight:     adjustmentInFlight(opsOnDoc),
		Now:                    now,
	})

	u.log.Info("invoice.refund_adjustment.planned",
		zap.String("refund_ref", in.RefundRef), zap.Int64("document_id", doc.ID),
		zap.String("plan", plan.Reason), zap.Int("kind", int(plan.Kind)),
		zap.Int64("refund_amount", in.RefundAmount),
		zap.Int64("cumulative", in.CumulativeRefundAmount))

	switch plan.Kind {
	case refundPlanSkip:
		return &RefundSkip{Disposition: plan.Disposition, Reason: plan.Reason}, nil
	case refundPlanVoid:
		return nil, u.createVoid(ctx, in, doc, now)
	case refundPlanAllowance:
		return u.createAllowance(ctx, in, doc, now, codemeta.InvoiceAdjustmentStatusPending.Value)
	case refundPlanRetryLater:
		return nil, fmt.Errorf(
			"invoice: refund %s for document %d not adjustable yet (%s); will retry",
			in.RefundRef, doc.ID, plan.Reason)
	default:
		return nil, fmt.Errorf("invoice: unknown refund plan %d", plan.Kind)
	}
}

func issueResultUnknown(doc domain.Document, ops []domain.Operation) bool {
	if doc.Status != codemeta.InvoiceDocumentStatusIssuing.Value &&
		doc.Status != codemeta.InvoiceDocumentStatusIssueFailed.Value {
		return false
	}
	for _, op := range ops {
		if op.OperationType != codemeta.InvoiceOperationTypeIssue.Value {
			continue
		}
		switch op.Status {
		case codemeta.InvoiceOperationStatusWaitingConfirmation.Value,
			codemeta.InvoiceOperationStatusProviderStatusRecheck.Value,
			codemeta.InvoiceOperationStatusFailedUnknown.Value,
			codemeta.InvoiceOperationStatusCancelBlockedUnknown.Value:
			return true
		}
	}
	return false
}

func holdsSequenceGate(op domain.Operation) bool {
	switch op.Status {
	case codemeta.InvoiceOperationStatusSucceeded.Value,
		codemeta.InvoiceOperationStatusNotApplicable.Value,
		codemeta.InvoiceOperationStatusCancelledBeforeIssue.Value,
		codemeta.InvoiceOperationStatusResolvedExternally.Value,
		codemeta.InvoiceOperationStatusCancelledByAdmin.Value:
		return false
	default:
		return true
	}
}

func correctionInFlight(ops []domain.Operation) bool {
	for _, op := range ops {
		if op.OperationType != codemeta.InvoiceOperationTypeVoid.Value &&
			op.OperationType != codemeta.InvoiceOperationTypeReissue.Value {
			continue
		}
		if holdsSequenceGate(op) {
			return true
		}
	}
	return false
}

func adjustmentInFlight(ops []domain.Operation) bool {
	for _, op := range ops {
		if op.OperationType != codemeta.InvoiceOperationTypeAllowance.Value &&
			op.OperationType != codemeta.InvoiceOperationTypeVoidAllowance.Value {
			continue
		}
		if holdsSequenceGate(op) {
			return true
		}
	}
	return false
}

func (u *RefundAdjustment) createAllowance(
	ctx context.Context, in RefundAdjustmentInput, doc domain.Document, now time.Time, status string,
) (*RefundSkip, error) {
	if doc.ProviderInvoiceNo == nil || *doc.ProviderInvoiceNo == "" {
		return nil, fmt.Errorf(
			"invoice: document %d is %s but has no provider_invoice_no, cannot build allowance lines",
			doc.ID, doc.Status)
	}

	docLines, err := u.docs.ListLines(ctx, doc.ID)
	if err != nil {
		return nil, err
	}

	effective, clampedFrom := clampAllowanceToRemaining(doc, docLines, in.RefundAmount)
	if effective < clampedFrom {
		observeRefundExceedsInvoiced()
		u.log.Error("invoice.allowance.refund_exceeds_invoiced",
			zap.String("order_no", in.OrderNo),
			zap.String("refund_ref", in.RefundRef),
			zap.Int64("document_id", doc.ID),
			zap.Int64("issuer_id", doc.IssuerID),
			zap.Int64("refund_amount", in.RefundAmount),
			zap.Int64("cumulative_refund_amount", in.CumulativeRefundAmount),
			zap.Int64("invoiced_total", doc.TotalAmount),
			zap.Int64("already_allowanced", doc.AllowanceTotalAmount),
			zap.Int64("clamped_to", effective))
	}
	if effective <= 0 {
		return &RefundSkip{
			Disposition: codemeta.InvoiceInboxDispositionAllowanceCapReached.Value,
			Reason: fmt.Sprintf(
				"累計折讓已達發票金額（invoiced_total=%d、已折讓=%d），本筆退款 %d 無可折讓餘額；退款本身不受影響",
				doc.TotalAmount, doc.AllowanceTotalAmount, in.RefundAmount),
		}, nil
	}

	draft, err := domain.BuildAllowanceDraft(
		allowanceSourceLines(docLines), effective, domain.AllowanceContext{
			DocumentTotalAmount:    doc.TotalAmount,
			DocumentTaxAmount:      doc.TaxAmount,
			ExistingAllowanceTotal: doc.AllowanceTotalAmount,
			ProviderInvoiceNo:      *doc.ProviderInvoiceNo,
			InvoiceDateTaipeiYmd:   invoiceDateYmd(doc),
			B2CSplitTax:            u.splitTax,
		})
	if err != nil {
		return nil, fmt.Errorf("invoice: build allowance draft for document %d: %w", doc.ID, err)
	}
	if err := draft.Validate(); err != nil {
		return nil, fmt.Errorf("invoice: allowance draft invariant for document %d: %w", doc.ID, err)
	}

	allowanceNo, err := u.adjustments.NextAllowanceNumber(ctx)
	if err != nil {
		return nil, err
	}

	adjID := u.newID()
	taipeiDate := now.In(timeutil.LocationTaipei)
	refundNo := in.RefundRef
	netEx, tax, gross := draft.TotalAmount, draft.TaxAmount, draft.TotalAmount+draft.TaxAmount

	return nil, u.tx.WithTx(ctx, func(ctx context.Context) error {
		adj, inserted, err := u.adjustments.Insert(ctx, domain.NewAdjustment{
			ID:                  adjID,
			DocumentID:          doc.ID,
			AdjustmentType:      codemeta.InvoiceAdjustmentTypeAllowance.Value,
			Status:              status,
			TriggerKind:         codemeta.InvoiceAdjustmentTriggerOrderRefund.Value,
			RefundNo:            &refundNo,
			AllowanceNumber:     &allowanceNo,
			AllowanceDateTaipei: &taipeiDate,
			NetAmountExTax:      &netEx,
			TaxAmount:           &tax,
			GrossAmount:         &gross,
			Now:                 now,
		})
		if err != nil {
			return err
		}
		if !inserted {
			u.log.Info("invoice.refund_adjustment.insert_conflict",
				zap.String("refund_ref", in.RefundRef), zap.Int64("document_id", doc.ID))
			return nil
		}

		if err := u.adjustments.InsertLines(ctx, adj.ID,
			adjustmentLinesOf(draft, docLines, doc), now); err != nil {
			return err
		}
		if err := appendStatusHistory(ctx, u.audit, subjectAdjustment, adj.ID,
			"", status, "", "", now); err != nil {
			return err
		}
		return u.createOperation(ctx, doc.ID, adj.ID,
			codemeta.InvoiceOperationTypeAllowance.Value, now)
	})
}

func (u *RefundAdjustment) createVoid(
	ctx context.Context, in RefundAdjustmentInput, doc domain.Document, now time.Time,
) error {
	adjID := u.newID()
	refundNo := in.RefundRef
	reasonCode := codemeta.InvoiceAdjustmentTriggerOrderRefund.Value
	detail := fmt.Sprintf("訂單退款全額作廢（refund_ref=%s）", in.RefundRef)

	return u.tx.WithTx(ctx, func(ctx context.Context) error {
		adj, inserted, err := u.adjustments.Insert(ctx, domain.NewAdjustment{
			ID:               adjID,
			DocumentID:       doc.ID,
			AdjustmentType:   codemeta.InvoiceAdjustmentTypeVoid.Value,
			Status:           codemeta.InvoiceAdjustmentStatusPending.Value,
			TriggerKind:      codemeta.InvoiceAdjustmentTriggerOrderRefund.Value,
			RefundNo:         &refundNo,
			VoidReasonCode:   &reasonCode,
			VoidReasonDetail: &detail,
			Now:              now,
		})
		if err != nil {
			return err
		}
		if !inserted {
			u.log.Info("invoice.refund_adjustment.void_insert_conflict",
				zap.String("refund_ref", in.RefundRef), zap.Int64("document_id", doc.ID))
			return nil
		}
		if err := appendStatusHistoryDetail(ctx, u.audit, subjectAdjustment, adj.ID,
			"", codemeta.InvoiceAdjustmentStatusPending.Value, reasonCode, detail, "", now); err != nil {
			return err
		}
		return u.createOperation(ctx, doc.ID, adj.ID,
			codemeta.InvoiceOperationTypeVoid.Value, now)
	})
}

func (u *RefundAdjustment) createOperation(
	ctx context.Context, documentID, adjustmentID int64, opType string, now time.Time,
) error {
	key, ok := domain.OperationKeyFor(opType, documentID, adjustmentID, 0, 0)
	if !ok {
		return fmt.Errorf("invoice: no operation_key rule for %s", opType)
	}
	op, inserted, err := u.ops.Create(ctx, domain.NewOperation{
		ID:            u.newID(),
		DocumentID:    documentID,
		AdjustmentID:  adjustmentID,
		OperationType: opType,
		OperationKey:  key,
		MaxAttempts:   u.cfg.MaxAttempts,
		NextAttemptAt: now,
		Now:           now,
	})
	if err != nil {
		return err
	}
	if !inserted {
		existing, getErr := u.ops.GetByKey(ctx, key)
		if getErr == nil {
			u.log.Info("invoice.refund_adjustment.operation_exists",
				zap.Int64("document_id", documentID), zap.Int64("adjustment_id", adjustmentID),
				zap.String("operation_key", key), zap.Int64("operation_id", existing.ID),
				zap.String("operation_status", existing.Status))
			return nil
		}
		return fmt.Errorf(
			"invoice: operation_key %s not inserted and not found for document %d "+
				"(I5 sequence gate held by another adjustment, or key lookup failed: %v); will retry",
			key, documentID, getErr)
	}
	if err := appendStatusHistory(ctx, u.audit, subjectOperation, op.ID,
		"", op.Status, "", "", now); err != nil {
		return err
	}

	promoted, err := u.ops.Promote(ctx, op.ID, codemeta.InvoiceOperationStatusReady.Value, now, now)
	if err != nil {
		return err
	}
	if !promoted {
		return fmt.Errorf("invoice: operation %d could not be promoted to READY (rows=0)", op.ID)
	}
	return appendStatusHistory(ctx, u.audit, subjectOperation, op.ID,
		codemeta.InvoiceOperationStatusCreated.Value,
		codemeta.InvoiceOperationStatusReady.Value, "", "", now)
}

func allowanceSourceLines(in []domain.DocumentLine) []domain.Line {
	out := make([]domain.Line, 0, len(in))
	for _, l := range in {
		out = append(out, domain.Line{
			LineSeq:           l.LineSeq,
			LineType:          l.LineType,
			Description:       l.Description,
			TaxCategory:       domain.TaxCategory(l.TaxCategory),
			AllowanceableBase: l.AllowanceableBase,
			AllowancedAmount:  l.AllowancedAmount,
		})
	}
	return out
}

func adjustmentLinesOf(
	draft *domain.AllowanceDraft, docLines []domain.DocumentLine, doc domain.Document,
) []domain.NewAdjustmentLine {
	bySeq := make(map[int32]int64, len(docLines))
	for _, l := range docLines {
		bySeq[l.LineSeq] = l.ID
	}
	invoiceDate := time.Time{}
	if doc.InvoiceDateTaipei != nil {
		invoiceDate = *doc.InvoiceDateTaipei
	}
	out := make([]domain.NewAdjustmentLine, 0, len(draft.Lines))
	for i, l := range draft.Lines {
		var srcID *int64
		if id, ok := bySeq[l.SourceLineSeq]; ok {
			srcID = &id
		}
		out = append(out, domain.NewAdjustmentLine{
			LineSeq:             int32(i + 1),
			SourceLineID:        srcID,
			OriginalInvoiceNo:   l.OriginalInvoiceNumber,
			OriginalInvoiceDate: invoiceDate,
			OriginalDescription: l.OriginalDescription,
			Quantity:            l.Quantity,
			UnitPriceExTax:      l.UnitPrice,
			NetAmountExTax:      l.Amount,
			TaxAmount:           l.Tax,
			TaxCategory:         int16(l.TaxCategory),
		})
	}
	return out
}

func invoiceDateYmd(doc domain.Document) string {
	if doc.InvoiceDateTaipei == nil {
		return ""
	}
	return doc.InvoiceDateTaipei.In(timeutil.LocationTaipei).Format("20060102")
}
