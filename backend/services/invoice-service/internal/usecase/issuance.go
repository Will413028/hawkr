// Package usecase 是 Layer 4：全部業務判斷。
//
// 不直接打 AMEGO 的 HTTP，一律經 ProviderClient 介面（方便注入 fake）；不自己組 SQL、
// 不自己 Begin —— 交易一律 pkg/database.TxManager.WithTx，repository 由 ctx 感知交易。
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
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/eventwire"
	"go.uber.org/zap"
)

const (
	sourceTypeOrder   = "ORDER"
	triggerTypeAuto   = "AUTO"
	providerAmego     = "AMEGO"
	actorTypeSystem   = "SYSTEM"
	subjectDocument   = "DOCUMENT"
	subjectOperation  = "OPERATION"
	outboxAggregateD  = "DOCUMENT"
	receiptAuthNoneV1 = ""
)

// ErrUnexpectedIssueOperation ＝ 一筆 ISSUE operation 的 document 不是 AMEGO_AUTO。
//
// 🔴 **這是防線不是正常路徑**：決策樹（domain.ResolveFulfillmentMode）對每一個
// 非 AMEGO_AUTO 的分支都落 NOT_APPLICABLE，而 NOT_APPLICABLE 不進開票候選集合
// ⇒ 正常情況下沒有東西會走到這裡。
//
// 它曾經是正常路徑：#3022 之前 PLATFORM_RECEIPT（免用統一發票賣家）落 PENDING，
// 於是每一筆這種訂單都會走到這一格，被當成 PERMANENT ⇒ document 落 ISSUE_FAILED。
// 而 §5 L11 逐字寫著 PLATFORM_RECEIPT「不可混入同一條發票狀態機」。
// 決策樹修好之後，這裡只剩下「修好之前建的舊列」與「有人繞過決策樹」兩種來源。
//
// 仍當 PERMANENT 處置而不是無限重試：一個永遠不會自己好起來的狀態，留在候選集合裡
// 只會每 10 秒燒一次 tick 而且不會有任何人看到。
var ErrUnexpectedIssueOperation = errors.New("invoice: ISSUE operation on a document whose fulfillment_mode is not AMEGO_AUTO")

// IssuanceConfig 是開票主鏈需要的排程參數（值來自 config.ScheduleConfig，§0.2）。
type IssuanceConfig struct {
	// MaxAttempts 是新建 operation 的 max_attempts 初值。
	// 🔴 worker 讀的是**列上的值**不是本欄；本欄只在建列當下寫進去一次。
	MaxAttempts int32
	BaseBackoff time.Duration
	MaxBackoff  time.Duration
	// IssueLease 是 INV.op 的租約長度（120s）。下界 ＝ 該次 claim 內所有 deadline 之和。
	IssueLease time.Duration
	// RecheckBase 是 WAITING_CONFIRMATION 進入重查迴圈的第一段退避（30s）。
	RecheckBase time.Duration
}

// Issuance 承載 S-1 自動開票的全部業務判斷（§7.1.2 步驟 5–14）。
type Issuance struct {
	tx          database.TxManager
	issuers     domain.IssuerRepository
	profiles    domain.SellerTaxProfileRepository
	accounts    domain.ProviderAccountRepository
	docs        domain.DocumentRepository
	ops         domain.OperationRepository
	corrections domain.CorrectionRequestRepository
	outbox      domain.OutboxRepository
	audit       domain.AuditRepository
	provider    ProviderClient

	receipts     ReceiptRenderer
	receiptStore ReceiptUploader

	newID func() int64
	now   func() time.Time
	cfg   IssuanceConfig
	log   *zap.Logger
}

// NewIssuance 組出 usecase。now 為 nil 時用 time.Now（測試注入 clock，不用 sleep）。
func NewIssuance(
	tx database.TxManager,
	issuers domain.IssuerRepository,
	profiles domain.SellerTaxProfileRepository,
	accounts domain.ProviderAccountRepository,
	docs domain.DocumentRepository,
	ops domain.OperationRepository,
	corrections domain.CorrectionRequestRepository,
	outbox domain.OutboxRepository,
	audit domain.AuditRepository,
	provider ProviderClient,
	receipts ReceiptRenderer,
	receiptStore ReceiptUploader,
	newID func() int64,
	cfg IssuanceConfig,
	log *zap.Logger,
) *Issuance {
	return &Issuance{
		tx: tx, issuers: issuers, profiles: profiles, accounts: accounts,
		docs: docs, ops: ops, corrections: corrections, outbox: outbox, audit: audit, provider: provider,
		receipts: receipts, receiptStore: receiptStore,
		newID: newID, now: func() time.Time { return time.Now().UTC() }, cfg: cfg,
		log: orNop(log),
	}
}

// WithClock 注入時間來源（測試用）。時間敏感邏輯一律注入 clock，不以 sleep 建立不穩定測試。
func (u *Issuance) WithClock(now func() time.Time) *Issuance {
	if now != nil {
		u.now = now
	}
	return u
}


// OrderPaidInput 是 Prepare 的全部輸入。
//
// 它刻意是**值**而不是「幾個 client 介面」：三份事實（ORD 快照／PAY 付款事實／本地稅務
// 與 provider 狀態）的取得是 consumer 的責任（步驟 5），Prepare 只做判斷與落庫。
// 這讓決策樹的 15 個葉節點全部可以用純值測試，不必架 gRPC mock。
type OrderPaidInput struct {
	OrderNo string
	// BuyerID 是買家業務鍵（buyers.buyer_id），不是 account_id。
	BuyerID int64
	// StoreID 是賣家業務鍵（sellers.store_id）＝ invoice_issuers.issuer_ref_id。
	StoreID int64
	// SnapshotVersion 的唯一合法來源是 order-service 的回傳值（I-50）：
	// 本服務沒有、也不得新增任何遞增它的路徑。
	SnapshotVersion int32
	// PaidAt ＝ COALESCE(payment.provider_paid_at, payment.pay_time)，UTC（§5.3.4）。
	// provider_paid_at 可為 NULL（ATM webhook 通常無時間欄位）⇒ 呼叫端必須先 COALESCE。
	PaidAt time.Time

	Snapshot domain.OrderSnapshot

	// 買家發票偏好（明文，方向往內）。
	BuyerTaxID   string
	BuyerTitle   string
	CarrierType  codemeta.InvoiceCarrierType
	CarrierID1   string
	CarrierID2   string
	DonationCode string
	NotifyEmail  string

	// fail-closed 三格中程式推不出來的兩格（§7.1.3）。
	// 第三格（混合稅別）由本層自己從明細行推導。
	HasPlatformFundedDiscount bool
	IsConsolidatedOrder       bool

	// PreferenceOwnershipMismatch ＝ ORD adapter 解析 buyer_invoice_preference_id 時
	// 歸屬驗證失敗（I-47）。它是第四個 fail-closed 訊號，語意與上面兩格相同：
	// **拒絕開票並轉 AWAITING_MANUAL**，不是 fallback 成無載具（後者把 IDOR 靜默吞掉）。
	PreferenceOwnershipMismatch bool

	CorrelationID string


	// IssueAttemptSeq 是實體軸（§5：同一 snapshot_version 下第幾張實體發票，
	// 作廢重開 +1）。**零值視為 1**，讓一般開票路徑不必知道這一欄存在。
	IssueAttemptSeq int32
	// OriginalDocumentID 非 nil ⇒ 這是重開票，指回被作廢的那一張。
	// settleIssued 的 R10 以它為判準（見 completeReissueIfAny）。
	OriginalDocumentID *int64
}

// PrepareResult 回報這一次判定的落點。呼叫端（MQ consumer）據此決定 ACK 與 log。
type PrepareResult struct {
	DocumentID          int64
	OperationID         int64 // 0 ＝ 本次沒有建立 operation
	DocumentStatus      string
	OperationStatus     string // "" ＝ 沒有 operation
	FulfillmentMode     string
	DocumentKind        string
	NotApplicableReason string
	ManualReasonCode    string
	// AlreadyExisted ＝ 冪等命中（重複消費 order.paid）。呼叫端直接 ACK，不重做業務。
	AlreadyExisted bool
}

// Prepare 跑完 §7.1.3 的決策樹並在**單一事務**內建立 document ＋ lines ＋ operation。
//
// 五個必須一起成立的性質，任一個拆到交易外都會壞：
//   - fail-closed 三格與 §5A.11 的斷言結果必須與 document 同一交易落 manual_reason_code
//     （不是先建成 PENDING 再靠掃描器改過去）
//   - operation 的 statutory_deadline_at 是 document 同名欄的副本（I-52）
//   - operation 的 CREATED → READY/AWAITING_TRIGGER/NOT_APPLICABLE 是「建列的同一事務內判定」
//   - 明細行與表頭金額必須一起可見，否則 INV.op 可能撈到一張沒有明細的發票去送
//   - 履約模式在消費當下解析一次、之後永不重算（註 4 的 automation_cutover_at race）
//
// **建立不發任何事件**：§6.3.3.1 的 12 條 routing key 沒有 invoice.document.created，
// 買家端的「處理中」由 B04 讀 gRPC 呈現（§7.1.2 步驟 10）。
func (u *Issuance) Prepare(ctx context.Context, in OrderPaidInput) (PrepareResult, error) {
	now := u.now()

	issuer, err := u.ensureIssuer(ctx, in.StoreID, now)
	if err != nil {
		return PrepareResult{}, err
	}

	profile, err := u.profiles.GetByIssuer(ctx, issuer.ID)
	if err != nil && !isBizCode(err, bizcode.SellerTaxProfileNotFound) {
		return PrepareResult{}, err
	}
	account, err := u.accounts.GetByIssuer(ctx, issuer.ID, providerAmego)
	if err != nil && !isBizCode(err, bizcode.InvoiceProviderAccountNotFound) {
		return PrepareResult{}, err
	}

	invoiceDate := domain.InvoiceDateTaipei(in.PaidAt)

	resolver, err := taxResolverFor(profile)
	if err != nil {
		return PrepareResult{}, err
	}

	buyerHasTaxID := domain.IsBusinessTaxID(in.BuyerTaxID)
	buyerIdentifier := domain.B2CBuyerIdentifier
	if buyerHasTaxID {
		buyerIdentifier = in.BuyerTaxID
	}

	draft, buildErr := domain.NewIssueDraft(in.Snapshot, resolver, buyerIdentifier)
	if buildErr != nil && !errors.Is(buildErr, domain.ErrZeroAmount) {
		return PrepareResult{}, fmt.Errorf("invoice: build issue draft (order_no=%s): %w", in.OrderNo, buildErr)
	}
	if draft != nil {
		amegoCarrier, ok := domain.CarrierTypeToAmego(in.CarrierType)
		if !ok && in.CarrierType.Code != "" {
			return PrepareResult{}, fmt.Errorf("invoice: unknown carrier type %q (order_no=%s)", in.CarrierType.Code, in.OrderNo)
		}
		draft.CarrierType = amegoCarrier
		draft.CarrierValue = in.CarrierID1
		draft.DonationCode = in.DonationCode
	}

	mixedTax := false
	if draft != nil {
		mixedTax = len(domain.TaxCategorySet(draft.Items)) > 1
	}

	decision := domain.ResolveFulfillmentMode(domain.FulfillmentInput{
		TotalActualAmount:         in.Snapshot.TotalActualAmount,
		SellerReviewStatus:        profile.ReviewStatus,
		ProviderStatus:            account.Status,
		AutomationCutoverAt:       account.AutomationCutoverAt,
		InvoiceDate:               in.PaidAt,
		HasPlatformFundedDiscount: in.HasPlatformFundedDiscount,
		IsConsolidatedOrder:       in.IsConsolidatedOrder,
		HasMixedTaxCategory:       mixedTax,
		BuyerHasTaxID:             buyerHasTaxID,
		SellerReceiptAuthorized:   profile.ReceiptAuthorizedAt != nil,
	})

	if in.PreferenceOwnershipMismatch &&
		decision.FulfillmentMode == codemeta.InvoiceFulfillmentModeAmegoAuto.Value &&
		decision.DocumentStatus == codemeta.InvoiceDocumentStatusPending.Value {
		decision.DocumentStatus = codemeta.InvoiceDocumentStatusAwaitingManual.Value
		decision.ManualReasonCode = codemeta.InvoiceManualReasonPreferenceOwnershipMismatch.Value
		u.logf(ctx).Warn("invoice.prepare.preference_ownership_mismatch",
			zap.String("order_no", in.OrderNo),
			zap.Int64("buyer_id", in.BuyerID),
			zap.String("manual_reason_code", decision.ManualReasonCode))
	}

	if decision.FulfillmentMode == codemeta.InvoiceFulfillmentModeAmegoAuto.Value &&
		decision.DocumentStatus == codemeta.InvoiceDocumentStatusPending.Value {
		if verr := draft.Validate(); verr != nil {
			var ie *domain.InvariantError
			if !errors.As(verr, &ie) {
				return PrepareResult{}, verr
			}
			decision.DocumentStatus = codemeta.InvoiceDocumentStatusAwaitingManual.Value
			decision.ManualReasonCode = string(ie.Code)
			u.logf(ctx).Warn("invoice.prepare.invariant_failed",
				zap.String("order_no", in.OrderNo),
				zap.String("manual_reason_code", string(ie.Code)),
				zap.String("detail", ie.Detail))
		}
	}

	plan, err := u.buildPlan(in, issuer, profile, account, decision, draft, invoiceDate, now)
	if err != nil {
		return PrepareResult{}, err
	}

	var out PrepareResult
	err = u.tx.WithTx(ctx, func(ctx context.Context) error {
		var terr error
		out, terr = u.persist(ctx, plan)
		return terr
	})
	return out, err
}

type issuancePlan struct {
	doc         domain.NewDocument
	lines       []domain.NewDocumentLine
	makeOp      bool
	op          domain.NewOperation
	opNextState string
	opNextAt    time.Time
	manualReason  string
	correlationID string
	now           time.Time

	bumpIntakeProfileID int64
	bumpIntakeAmount    int64

	issuer domain.Issuer
}

func (u *Issuance) buildPlan(
	in OrderPaidInput,
	issuer domain.Issuer,
	profile domain.SellerTaxProfile,
	account domain.ProviderAccount,
	decision domain.FulfillmentDecision,
	draft *domain.IssueDraft,
	invoiceDate time.Time,
	now time.Time,
) (issuancePlan, error) {
	documentID := u.newID()

	var deadline *time.Time
	if d, ok := domain.StatutoryDeadline(invoiceDate, decision.DocumentKind); ok {
		deadline = &d
	}

	header := domain.Header{TaxType: domain.TaxCategoryTaxable}
	if draft != nil {
		header = draft.Header
	}

	carrierType, carrierID1, carrierID2, donation := normalizeBuyerIdentity(
		decision.DocumentKind, draft, in.DonationCode)

	receiptNo := ""
	if decision.DocumentKind == codemeta.InvoiceDocumentKindReceipt.Value {
		receiptNo = ""
	}

	provider := ""
	if decision.FulfillmentMode == codemeta.InvoiceFulfillmentModeAmegoAuto.Value {
		provider = providerAmego
	}

	snapshotJSON, err := marshalSourceSnapshot(in, decision, profile, account)
	if err != nil {
		return issuancePlan{}, err
	}

	buyerID := in.BuyerID
	plan := issuancePlan{
		issuer:        issuer,
		now:           now,
		correlationID: in.CorrelationID,
		doc: domain.NewDocument{
			ID:                  documentID,
			IssuerID:            issuer.ID,
			IssuerNameSnapshot:  issuerNameSnapshot(issuer, profile),
			IssuerTaxIDSnapshot: derefString(profile.TaxID),
			SourceType:          sourceTypeOrder,
			SourceRef:           in.OrderNo,
			TriggerType:         triggerTypeAuto,
			RecipientType:       codemeta.InvoiceRecipientTypeBuyer.Value,
			RecipientRefID:      &buyerID,
			RecipientName:       recipientName(decision.DocumentKind, in.BuyerTitle),
			RecipientTaxID:      recipientTaxID(decision.DocumentKind, in.BuyerTaxID),
			RecipientEmail:      in.NotifyEmail,
			CarrierType:         carrierType,
			CarrierID1:          carrierID1,
			CarrierID2:          carrierID2,
			DonationCode:        donation,
			DocumentKind:        decision.DocumentKind,
			FulfillmentMode:     decision.FulfillmentMode,
			Status:                      creationStatus(decision),
			NotApplicableReason:         decision.NotApplicableReason,
			SnapshotVersion:             in.SnapshotVersion,
			IssueAttemptSeq:             issueAttemptSeqOf(in),
			OriginalDocumentID:          in.OriginalDocumentID,
			ProviderAccountVersion:      providerAccountVersion(account),
			SalesAmount:                 header.SalesAmount,
			FreeTaxSalesAmount:          header.FreeTaxSalesAmount,
			ZeroTaxSalesAmount:          header.ZeroTaxSalesAmount,
			TaxAmount:                   header.TaxAmount,
			TotalAmount:                 header.TotalAmount,
			TaxType:                     int16(headerTaxType(header)),
			TaxRateBps:                  domain.TaxRateBps,
			Provider:                    provider,
			ProviderRequestNo:           domain.ProviderRequestNoForOrder(in.OrderNo, in.SnapshotVersion, issueAttemptSeqOf(in)),
			ReceiptNo:                   receiptNo,
			ReceiptAuthorizationVersion: receiptAuthNoneV1,
			InvoiceDateTaipei:           &invoiceDate,
			TaxPeriodCode: taxPeriodFor(decision.DocumentKind, invoiceDate),
			IssueNotBefore: in.PaidAt,
			PaidAt:              &in.PaidAt,
			StatutoryDeadlineAt: deadline,
			SourceSnapshot:      snapshotJSON,
			Now:                 now,
		},
		manualReason: decision.ManualReasonCode,
	}

	if draft != nil {
		plan.lines = toNewLines(draft.Lines)
	}

	switch plan.doc.Status {
	case codemeta.InvoiceDocumentStatusNotApplicable.Value:
		plan.makeOp = true
		plan.opNextState = codemeta.InvoiceOperationStatusNotApplicable.Value
		plan.opNextAt = now
	case codemeta.InvoiceDocumentStatusPending.Value:
		if plan.manualReason != "" {
			plan.makeOp = false
			break
		}
		plan.makeOp = true
		plan.opNextState = codemeta.InvoiceOperationStatusReady.Value
		plan.opNextAt = now
	}

	if intakeLimitOf(profile).Applicable {
		plan.bumpIntakeProfileID = profile.ID
		plan.bumpIntakeAmount = plan.doc.TotalAmount
	}

	if plan.makeOp {
		op, err := newIssueOperation(issueOperationSpec{
			OperationID:         u.newID(),
			DocumentID:          documentID,
			OriginalDocumentID:  in.OriginalDocumentID,
			IssueAttemptSeq:     issueAttemptSeqOf(in),
			ProviderRequestNo:   plan.doc.ProviderRequestNo,
			RequestSnapshotHash: domain.RequestSnapshotHash(draft),
			MaxAttempts:         u.cfg.MaxAttempts,
			NextAttemptAt:       plan.opNextAt,
			StatutoryDeadlineAt: deadline,
			Now:                 now,
		})
		if err != nil {
			return issuancePlan{}, err
		}
		plan.op = op
	}
	return plan, nil
}

func (u *Issuance) ensureIssuer(ctx context.Context, storeID int64, now time.Time) (domain.Issuer, error) {
	issuer, err := u.issuers.GetByRef(ctx, codemeta.InvoiceIssuerTypeSeller.Value, storeID)
	switch {
	case err == nil:
		return issuer, nil
	case !isBizCode(err, bizcode.InvoiceProviderAccountNotFound):
		return domain.Issuer{}, err
	}

	created, inserted, cerr := u.issuers.Create(ctx, domain.NewIssuer{
		ID:          u.newID(),
		IssuerType:  codemeta.InvoiceIssuerTypeSeller.Value,
		IssuerRefID: storeID,
		DisplayName: taxIssuerPlaceholderPrefix + strconv.FormatInt(storeID, 10),
		Now:         now,
	})
	if cerr != nil {
		return domain.Issuer{}, cerr
	}
	if !inserted {
		return u.issuers.GetByRef(ctx, codemeta.InvoiceIssuerTypeSeller.Value, storeID)
	}
	u.log.Info("invoice.issuance.issuer_created_on_demand",
		zap.Int64("store_id", storeID), zap.Int64("issuer_id", created.ID),
		zap.String("reason", "賣家從未進過發票稅務設定流程；本單走 LEGACY_MANUAL 留檔"))
	return created, nil
}

func (u *Issuance) persist(ctx context.Context, plan issuancePlan) (PrepareResult, error) {
	if plan.doc.DocumentKind == codemeta.InvoiceDocumentKindReceipt.Value {
		no, err := u.docs.NextReceiptNo(ctx)
		if err != nil {
			return PrepareResult{}, err
		}
		plan.doc.ReceiptNo = no
	}

	doc, inserted, err := u.docs.Create(ctx, plan.doc)
	if err != nil {
		return PrepareResult{}, err
	}
	if !inserted {
		return PrepareResult{AlreadyExisted: true}, nil
	}

	if err := u.bumpIntakeIfPending(ctx, plan); err != nil {
		return PrepareResult{}, err
	}

	if err := u.docs.InsertLines(ctx, doc.ID, plan.lines, plan.now); err != nil {
		return PrepareResult{}, err
	}

	if err := u.appendHistory(ctx, subjectDocument, doc.ID, "", doc.Status,
		firstNonEmpty(plan.doc.NotApplicableReason, plan.manualReason), plan.correlationID, plan.now); err != nil {
		return PrepareResult{}, err
	}

	out := PrepareResult{
		DocumentID:          doc.ID,
		DocumentStatus:      doc.Status,
		FulfillmentMode:     doc.FulfillmentMode,
		DocumentKind:        doc.DocumentKind,
		NotApplicableReason: derefString(doc.NotApplicableReason),
	}

	if plan.manualReason != "" {
		ok, err := u.docs.MarkAwaitingManual(ctx, doc.ID, plan.manualReason, plan.now)
		if err != nil {
			return PrepareResult{}, err
		}
		if !ok {
			return PrepareResult{}, fmt.Errorf("invoice: document %d could not enter AWAITING_MANUAL (rows=0)", doc.ID)
		}
		if err := u.appendHistory(ctx, subjectDocument, doc.ID,
			codemeta.InvoiceDocumentStatusPending.Value,
			codemeta.InvoiceDocumentStatusAwaitingManual.Value,
			plan.manualReason, plan.correlationID, plan.now); err != nil {
			return PrepareResult{}, err
		}
		out.DocumentStatus = codemeta.InvoiceDocumentStatusAwaitingManual.Value
		out.ManualReasonCode = plan.manualReason
		if err := u.emitReviewRequired(ctx, plan.issuer, doc, plan.manualReason, plan.now); err != nil {
			return PrepareResult{}, err
		}
	}

	if !plan.makeOp {
		return out, nil
	}

	op, opInserted, err := u.ops.Create(ctx, plan.op)
	if err != nil {
		return PrepareResult{}, err
	}
	if !opInserted {
		op, err = u.ops.GetByKey(ctx, plan.op.OperationKey)
		if err != nil {
			return PrepareResult{}, err
		}
		out.OperationID = op.ID
		out.OperationStatus = op.Status
		return out, nil
	}

	promoted, err := u.ops.Promote(ctx, op.ID, plan.opNextState, plan.opNextAt, plan.now)
	if err != nil {
		return PrepareResult{}, err
	}
	if !promoted {
		return PrepareResult{}, fmt.Errorf("invoice: operation %d could not leave CREATED (rows=0)", op.ID)
	}
	if err := u.appendHistory(ctx, subjectOperation, op.ID,
		codemeta.InvoiceOperationStatusCreated.Value, plan.opNextState,
		plan.doc.NotApplicableReason, plan.correlationID, plan.now); err != nil {
		return PrepareResult{}, err
	}

	out.OperationID = op.ID
	out.OperationStatus = plan.opNextState
	return out, nil
}

func (u *Issuance) bumpIntakeIfPending(ctx context.Context, plan issuancePlan) error {
	if plan.bumpIntakeProfileID == 0 {
		return nil
	}
	return u.profiles.BumpPendingOrderCounters(ctx,
		plan.bumpIntakeProfileID, plan.bumpIntakeAmount, plan.now)
}

func issueAttemptSeqOf(in OrderPaidInput) int32 {
	if in.IssueAttemptSeq < 1 {
		return 1
	}
	return in.IssueAttemptSeq
}

type reviewRequiredEvent struct {
	NotificationRefID string            `json:"notification_ref_id"`
	InvoiceDocumentID string            `json:"invoice_document_id"`
	OrderNo           string            `json:"order_no"`
	BuyerID           eventwire.BuyerID `json:"buyer_id"`
	StoreID           eventwire.StoreID `json:"store_id"`
	ReviewReasonCode  string            `json:"review_reason_code"`
	OccurredAt        string            `json:"occurred_at"`
}

func (u *Issuance) emitReviewRequired(
	ctx context.Context, issuer domain.Issuer, doc domain.Document, reasonCode string, now time.Time,
) error {
	payload, err := json.Marshal(reviewRequiredEvent{
		NotificationRefID: "INV:" + strconv.FormatInt(doc.ID, 10),
		InvoiceDocumentID: strconv.FormatInt(doc.ID, 10),
		OrderNo:           derefString(doc.SourceRef),
		BuyerID:           eventwire.BuyerIDOfDocument(doc),
		StoreID:           eventwire.StoreIDOf(issuer),
		ReviewReasonCode:  reasonCode,
		OccurredAt:        timeutil.FormatISO8601(now),
	})
	if err != nil {
		return err
	}
	_, err = u.outbox.Insert(ctx, domain.NewOutboxEvent{
		AggregateType: outboxAggregateD,
		AggregateID:   doc.ID,
		EventType:     string(pkgmq.EventTypeInvoiceOperationReviewRequired),
		Exchange:      pkgmq.ExchangeInvoice,
		RoutingKey:    pkgmq.RoutingKeyInvoiceOperationReviewRequired,
		Payload:       payload,
		NextAttemptAt: now,
		Now:           now,
	})
	return err
}
