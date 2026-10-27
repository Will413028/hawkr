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

// 本服務內部使用、值域由 DB CHECK 正面列舉而非 CodeMeta 管理的字面值（附錄 C.4）。
// 集中在此處而不是散落在呼叫點：值域的 SSOT 是 §5 的 DDL，改 DDL 時 grep 得到全部用點。
const (
	sourceTypeOrder   = "ORDER"  // chk_invdoc_source
	triggerTypeAuto   = "AUTO"   // chk_invdoc_trigger
	providerAmego     = "AMEGO"  // invoice_documents.provider
	actorTypeSystem   = "SYSTEM" // invoice_status_history.actor_type（C.4 判定不納管）
	subjectDocument   = "DOCUMENT"
	subjectOperation  = "OPERATION"
	outboxAggregateD  = "DOCUMENT" // invoice_outbox_events.aggregate_type（C.4 不納管）
	receiptAuthNoneV1 = ""         // 收據授權條款版本；本波不採集
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

	// receipts／receiptStore 只有 PLATFORM_RECEIPT 那條線會用到（#3026）。
	// 兩者都是**必填**：DI 那一層在字型讀不起來時就 fail-closed（pod 起不來），
	// 所以取到 nil 只可能發生在測試裡忘了給 —— settleReceipt 會就地報錯而不是靜默跳過。
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
		// nil 守衛放在建構子而不是每個呼叫點：少一處就是一個 nil deref，
		// 而那類 panic 只在「剛好走到那條分支」時才炸（2026-09-01 R08 實例）。
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

// ---------------------------------------------------------------------------
// Prepare：order.paid 消費之後的建立階段（§7.1.2 步驟 5–9）
// ---------------------------------------------------------------------------

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

	// ── 作廢重開（R04→R10）才會設的兩欄；一般開票留零值 ──────────────────
	//
	// 為什麼是**參數化既有的 buildPlan** 而不是另寫一套 document builder：
	// 建票要決定的東西（issuer 解析、稅務檔、fulfillment 判定、AWAITING_MANUAL、
	// 字軌預檢）在重開時**一項都沒有變**，複製一份出來就是兩份會各自漂移的實作。
	// 這裡只有兩件事真的不同：這是第幾張實體發票，以及它是誰的替代品。

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

	// 稅務資格與 provider 帳戶「查不到」不是錯誤，是決策樹的一個輸入值。
	// 🔴 deny-by-default：查不到 ⇒ 空字串 ⇒ ResolveFulfillmentMode 的 default 分支
	// ⇒ LEGACY_MANUAL + TAX_PROFILE_NOT_APPROVED。把它當錯誤往上拋會讓 consumer 重試三次
	// 後進 DLQ，而真相是「這位賣家還沒送審」——那不需要重試。
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

	// 明細行先組出來：混合稅別這一格要靠它判定，而它同時是後面落庫的內容。
	// ErrZeroAmount 是正常分支（§5A.10 第 1 列）不是失敗 —— 決策樹的 Z 節點承接它。
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

	// I-47：載具歸屬驗證失敗是 fail-closed 的第四格。它放在 §5A.11 斷言**之前**，
	// 理由是它與金額／稅額無關——用 A15 的 BUYER_INFO_CONFLICT 去蓋它，ADM 工作台上
	// 會看到一個指向錯方向的理由碼，而真相是「有人拿別人的常用發票資料對這張訂單試」。
	//
	// 與其餘 fail-closed 一樣只作用在真的會送出去的那條路徑：其餘葉節點本來就不開票。
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

	// §5A.11 送出前不變式斷言（fail-closed）。只對真的會送出去的那條路徑跑：
	// 其餘葉節點不會產生 AMEGO request，拿 AMEGO 專屬的斷言（A7 B2C 稅額、A14 DetailVat…）
	// 去擋它們只會把「賣家還沒串接」誤報成「金額算錯」。
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

// issuancePlan 是「決策已定、還沒落庫」的中間值。分出來的理由是 Prepare 的交易區塊必須
// 只剩寫入：任何一次 gRPC／解密／算式留在交易裡，都會把交易持有時間綁在外部依賴上。
type issuancePlan struct {
	doc         domain.NewDocument
	lines       []domain.NewDocumentLine
	makeOp      bool
	op          domain.NewOperation
	opNextState string
	opNextAt    time.Time
	// manualReason 非空 ⇒ 建成 PENDING 之後在**同一交易內**接 MarkAwaitingManual。
	// M1 只有兩條建立邊（→PENDING／→NOT_APPLICABLE），`[*]` → AWAITING_MANUAL 不存在。
	manualReason  string
	correlationID string
	now           time.Time

	// bumpIntakeProfileID 非 0 ⇒ 這位賣家在待審窗口內，persist 要在**同一交易內**
	// 累加接單額度（§5.1.2 口徑：已付款訂單，只在建立 invoice_documents 的同交易內累加）。
	// 零值代表不累加——把判斷留在 buildPlan 而不是 persist，是因為 persist 是純寫入區塊，
	// 讓它去查賣家狀態會在交易內多一次讀，而那個值 buildPlan 手上就有。
	bumpIntakeProfileID int64
	bumpIntakeAmount    int64

	// issuer 只給 P7（invoice.operation.review_required）用（#3055 的橫向稽查）：
	// 事件 payload 的 store_id 一律走 eventwire.StoreIDOf(issuer)，而 persist 是交易內的
	// 純寫入區塊，不該為了取它多一次讀 —— buildPlan 手上本來就有。
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

	// statutory_deadline_at 由 chk_invdoc_deadline_presence 綁 document_kind 而**與 status 無關**：
	// 連 NOT_APPLICABLE 的 B2C 文件都必須有期限，RECEIPT 則必須沒有。
	var deadline *time.Time
	if d, ok := domain.StatutoryDeadline(invoiceDate, decision.DocumentKind); ok {
		deadline = &d
	}

	header := domain.Header{TaxType: domain.TaxCategoryTaxable}
	if draft != nil {
		header = draft.Header
	}

	// chk_invdoc_exclusive：統編／載具／捐贈碼三選一，AMEGO 不擋、只能我方擋。
	// 這裡做的是**落庫正規化**（統編優先、其次載具），不是判斷：判斷已經由 A15 在
	// Validate 內做完並落成 BUYER_INFO_CONFLICT。兩者分開，才不會把「輸入互斥」
	// 悄悄變成「我幫你挑一個」。
	carrierType, carrierID1, carrierID2, donation := normalizeBuyerIdentity(
		decision.DocumentKind, draft, in.DonationCode)

	receiptNo := ""
	if decision.DocumentKind == codemeta.InvoiceDocumentKindReceipt.Value {
		// chk_invdoc_receipt 是雙向等式 ⇒ 號碼必須與 document_kind 同一句寫。
		// 取號在交易外：SEQUENCE 的 nextval 不受交易回滾影響，跳號本來就可接受。
		receiptNo = "" // 由 persist 在交易內取號（見該處註解）
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
			// 建立邊只有 PENDING 與 NOT_APPLICABLE 兩條；AWAITING_MANUAL 走 manualReason。
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
			// 🔴 **收據沒有稅期別**（#3026 收尾，2026-09-03）。期別碼是「這張發票落在
			// 哪一個財政部申報期」，而收據不申報、不上傳、沒有作廢期限（設計書 §7.2）。
			//
			// 這不是潔癖：它唯一一次被讀到就產生了錯答案 —— `canRequestCorrection` 拿
			// `WithinVoidWindow(tax_period_code)` 判定，收據帶著 11505 ⇒ 判定成立 ⇒
			// 買家看到一顆「申請更正」按鈕，而那條路整條是發票語意、按下去會在 AMEGO
			// 那一段失敗。守衛已補（兩道），但那是擋 instance；**這一行才是拿掉 class**。
			//
			// 空字串經 `snp` 落 NULL；`chk_invdoc_period` 允許 NULL。
			TaxPeriodCode: taxPeriodFor(decision.DocumentKind, invoiceDate),
			// Phase 1 的 issue_not_before ＝ 收款時點（即刻可開），不是未來時點。
			IssueNotBefore: in.PaidAt,
			// paid_at 與 issue_not_before 在這條路徑上同值，但語意不同（見
			// domain.Document.PaidAt 的紅線）⇒ 分兩欄寫，不共用一欄。
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

	// operation 的建立與否，逐葉節點：
	//   NOT_APPLICABLE  → 建，並在同一交易內推進 J04（讓「這一單為什麼沒開」在 M2 上也留痕）
	//   AWAITING_MANUAL → **不建**（§7.1.3 的 AM 節點明文「operation 不建」）。ADM 給出判定後
	//                     document 回 PENDING，那時才建 —— 建了再靠 HELD 壓住需要一條
	//                     `CREATED → HELD` 的邊，而 M2 上沒有那條邊（J08 的起點是 READY／
	//                     RETRY_SCHEDULED），硬寫會 rows=0 靜默失敗。
	//   PENDING         → 建 ＋ J02（issue_not_before <= now，Phase 1 恆成立）
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

	// 待審接單額度：只有在窗口內才累加。
	//
	// 🔴 **不得無條件累加**：這兩個計數器沒有任何遞減或重置路徑（遞減需要有人在窗口
	// 滑出時掃描，那是《後端服務責任邊界規範》禁止的 reconciliation job）⇒ 無條件累加
	// 會讓一個歷史悠久的賣家哪天進入待審時被累積總數瞬間擋死，症狀是結帳回 9326。
	//
	// 值域逐字對齊 intakeLimitOf（讀那兩個欄位的唯一地方）：兩處認的是同一組狀態，
	// 不一致的話會出現「累加了但永遠不檢查」或「檢查了但從不累加」——後者正是
	// 2026-09-01 發現的實況（BumpSellerTaxProfilePendingOrderCounters 零呼叫端）。
	if intakeLimitOf(profile).Applicable {
		plan.bumpIntakeProfileID = profile.ID
		plan.bumpIntakeAmount = plan.doc.TotalAmount
	}

	if plan.makeOp {
		// operation_type 的推導（REISSUE vs ISSUE）、operation_key 的參數順序與
		// issue_attempt_seq 的下界都在 newIssueOperation 一處，三個呼叫端共用
		// （見 issue_operation.go 的檔頭：那三件事錯了都不會編譯錯）。
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

// ensureIssuer 取得（必要時建立）這位賣家的 invoice_issuers 列。
//
// 🔴 **這是 dev 上 4 筆 ORDER_PAID 永遠卡在 RECEIVED 的根因。** 原本這裡是裸的 GetByRef，
// 查不到就把錯誤往上拋 ⇒ Prepare 失敗 ⇒ consumer 回 error ⇒ pkg/mq 重投 ⇒ 被 inbox 去重
// 擋掉 return nil ⇒ 該列永遠停在 RECEIVED、發票與留檔文件都不存在、沒有任何告警。
//
// 前提為什麼是假的：checkout.go 的同一格 guard 註解寫著「Prepare 那裡 issuer 是必然存在的，
// 因為 order.paid 之前賣家已經開過店」——但 invoice_issuers 的**唯一**建列點是
// TaxProfile.ensureProfile（賣家第一次存稅務資格草稿，§6.1.7），不是開店。從未進過發票
// 設定流程的賣家照樣可以有已付款訂單。dev 實測：卡住的 3 個 store_id 都是真實賣家。
//
// ⚠️ 不能只把 checkout.go 的 guard 抄過來（那裡查不到就回零值 domain.Issuer{}）：
// buildPlan 直接用 `IssuerID: issuer.ID`，零值 ID=0 會撞 fk_invdoc_issuer ⇒ 把「靜默卡住」
// 換成 FK violation，一樣開不出留檔。這裡必須真的建列。
//
// 建出來的 issuer 沒有稅務資格 ⇒ profile 查無 ⇒ SellerReviewStatus 為空 ⇒
// ResolveFulfillmentMode 走 default 分支 ⇒ LEGACY_MANUAL ＋ TAX_PROFILE_NOT_APPROVED
// ＋ document NOT_APPLICABLE ＋ operation NOT_APPLICABLE。**那才是這些訂單的正確處置**：
// 賣家沒送審本來就不該開票，但「不開票」也要有留檔、有法定期限欄、說得出理由。
//
// 🔴 呼叫點在 u.tx.WithTx **之外**（persist 的交易區塊刻意只剩寫入）。persist 之後失敗會
// 留下一列孤兒 issuer —— 無害且刻意接受：它是身分列、冪等、之後 ensureProfile 會重用同一列，
// display_name 由 UpdateDisplayName 在法定名稱出現時覆蓋。
//
// 建列邏輯逐字沿用 TaxProfile.ensureProfile（同樣的佔位 display_name、同樣的
// inserted=false 就回查）：兩處若分歧，同一個 store_id 會在兩條路徑下拿到不同的顯示名稱。
func (u *Issuance) ensureIssuer(ctx context.Context, storeID int64, now time.Time) (domain.Issuer, error) {
	issuer, err := u.issuers.GetByRef(ctx, codemeta.InvoiceIssuerTypeSeller.Value, storeID)
	switch {
	case err == nil:
		return issuer, nil
	// ⚠️ 「查無此列」回的是 InvoiceProviderAccountNotFound（9330）而不是 issuer 專屬碼
	// ——那是 seller_repo.go 的既有實作（notFound(err, bizcode.InvoiceProviderAccountNotFound)），
	// 不是這裡挑的。碼名沒有 issuer 語意，所以只能靠它，不能靠「碼名看起來對不對」。
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
		// 併發的另一個請求（或 TaxProfile.ensureProfile）搶先建了 ⇒ 讀它建的那一列。
		// ON CONFLICT DO NOTHING 的 rows=0 是正常結果不是錯誤。
		return u.issuers.GetByRef(ctx, codemeta.InvoiceIssuerTypeSeller.Value, storeID)
	}
	u.log.Info("invoice.issuance.issuer_created_on_demand",
		zap.Int64("store_id", storeID), zap.Int64("issuer_id", created.ID),
		zap.String("reason", "賣家從未進過發票稅務設定流程；本單走 LEGACY_MANUAL 留檔"))
	return created, nil
}

// persist 是 Prepare 的交易區塊：**只有寫入**，沒有任何外部呼叫。
func (u *Issuance) persist(ctx context.Context, plan issuancePlan) (PrepareResult, error) {
	if plan.doc.DocumentKind == codemeta.InvoiceDocumentKindReceipt.Value {
		// SEQUENCE 的 nextval 不受交易回滾影響（跳號可接受、不可重複，§5.3.3），
		// 放在交易內只是為了少一次往返。
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
		// 冪等命中：uq_invdoc_order_identity 已有這一列。呼叫端直接 ACK。
		// 這裡不補 SELECT —— 呼叫端手上有冪等鍵，要既有列自己去讀（GetBySourceRef）。
		return PrepareResult{AlreadyExisted: true}, nil
	}

	// 🔴 累加待審接單額度的位置就在這裡，不在下面任何一個 return 之前。
	//
	// persist 有**三個**成功出口（!makeOp／!opInserted／正常路徑），接在其中任何一個
	// 都會漏掉另外兩個或重複計數。這一行的位置保證「每張**新建**的 document 恰好一次」：
	// 上面那個 AlreadyExisted 早退已經把冪等重入濾掉了。
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

	// fail-closed：PENDING → AWAITING_MANUAL，**同一交易內**。
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
		// P7：賣家要知道有一筆開票作業卡在人工出口（#3055 的橫向稽查）。
		// 沒有這一則，fail-closed 的落點對賣家是完全無聲的 —— 他只會發現發票一直沒開。
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
		// 同一把 operation_key 已存在 ⇒ 冪等命中，讀既有列。
		// （另一種 rows=0 的意義「序列閘門被佔用」只發生在調整類 operation，ISSUE 不進那個
		//   partial unique 的述詞 ⇒ 這裡只可能是前者。分辨方式仍是讀 key 而不是猜。）
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

// bumpIntakeIfPending 累加待審接單額度（§5.1.2：**只在建立 invoice_documents 的
// 同交易內累加**，所以它在 persist 裡，不在 Prepare 的外層）。
//
// 🔴 在此之前 BumpSellerTaxProfilePendingOrderCounters 的非生成呼叫端是 **0**
// （2026-09-01 實查）——SQL、repo 方法與欄位註解都在，而沒有任何執行者。
// 後果是待審接單雙上限（§6.1.2 C5、業務碼 9326）**結構上永遠不會觸發**：
// 兩個計數器恆為 0，intakeLimitOf 的 Blocked 恆 false。
// 那是「機制存在於 SQL 與註解裡但沒有執行者」的同一個 class。
//
// ⚠️ 這一格只讓計數**動起來**。它讓 9326 變成可觸發的，但**結帳前置驗證仍未接線**
// （§6.2.4 order ③，見 #2900）——沒有接線就沒有人讀那個判定。兩件事都要做完，
// 這裡只完成前一半，不要因為計數會動了就以為額度已經在守。
func (u *Issuance) bumpIntakeIfPending(ctx context.Context, plan issuancePlan) error {
	if plan.bumpIntakeProfileID == 0 {
		return nil
	}
	return u.profiles.BumpPendingOrderCounters(ctx,
		plan.bumpIntakeProfileID, plan.bumpIntakeAmount, plan.now)
}

// issueAttemptSeqOf 把零值折成 1。
//
// 🔴 折在**一個地方**而不是讓每個呼叫端自己填 1：chk_invdoc_versions 要求
// issue_attempt_seq >= 1，漏填的症狀是建列當下 23514，而那個錯誤訊息不會告訴你
// 是哪一欄。同源教訓見 2026-08-31 的 MaxAttempts（同一個 CHECK 家族）。
func issueAttemptSeqOf(in OrderPaidInput) int32 {
	if in.IssueAttemptSeq < 1 {
		return 1
	}
	return in.IssueAttemptSeq
}

// reviewRequiredEvent 是 P7（`invoice.operation.review_required`，58 §6.3.9 定版 13 列第 8 列）。
//
// 收件人是**賣家**（模板的 target_user_field 是 notify_seller_account_id），而那個欄位由
// outbox dispatcher 在派送時用 payload 的 store_id 解析 ⇒ store_id 填錯的症狀是通知靜默
// 不送（notify_seller_account_id 恆為 "0"，下游 skip），全程零錯誤零 DLQ。
//
// review_reason_code 是 CodeMeta InvoiceManualReason 的**碼**不是人話：模板刻意不把它放進
// 文案，只放進 data 讓前端用 codeLabel 查表（migration 檔頭逐字寫了這個理由）。
type reviewRequiredEvent struct {
	NotificationRefID string            `json:"notification_ref_id"`
	InvoiceDocumentID string            `json:"invoice_document_id"`
	OrderNo           string            `json:"order_no"`
	BuyerID           eventwire.BuyerID `json:"buyer_id"`
	StoreID           eventwire.StoreID `json:"store_id"`
	ReviewReasonCode  string            `json:"review_reason_code"`
	OccurredAt        string            `json:"occurred_at"`
}

// emitReviewRequired 在 document 落 AWAITING_MANUAL 的**同一個交易內**寫 P7。
//
// 冪等鍵用 `INV:{invoice_document_id}`：M1 上以 AWAITING_MANUAL 為終點的邊全部來自
// PENDING／ISSUING／ISSUE_FAILED，而一份文件走完就停在那裡等人 ⇒ 同一份不會發第二則。
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
