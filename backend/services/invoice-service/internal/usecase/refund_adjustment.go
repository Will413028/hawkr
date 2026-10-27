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

// ---------------------------------------------------------------------------
// G01 / G02 / G03 —— `order.refund.finalized` 落到作廢或折讓（58 §7.2.2）
//
// 這是折讓軸唯一的入口：沒有它，`invoice_adjustments` 上永遠不會出現任何一列，
// 而它後面那整條執行軸（G08–G15、AMEGO g0401、P4 通知）都只是不會被呼叫的程式碼。
//
// 🔴 決策樹的 fail-closed 方向是**折讓**，不是作廢（§7.2.2 D 節點逐字：「折讓在任何
// 情況下都合法，作廢不是」）。所以每一個判斷不出來的節點都往折讓倒，而不是往作廢倒。
// 反過來寫的話，錯誤的後果是「把一張已申報的發票作廢」——那是稅務違規，不可回收。
// ---------------------------------------------------------------------------

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

// refundPlanKind 是決策樹的葉子。
type refundPlanKind int

const (
	// refundPlanAllowance 建 ALLOWANCE adjustment ＋ lines ＋ operation。
	refundPlanAllowance refundPlanKind = iota
	// refundPlanVoid 建 VOID adjustment ＋ operation（無 lines：作廢沒有明細）。
	refundPlanVoid
	// refundPlanRetryLater 是 G02／G03 的實際落點：**退避重試**，不建列。
	//
	// 🔴 這裡與設計書 §7.2.2 不同，理由是 schema 不支援設計書要的東西：
	// G02／G03 要求「建立 adjustment 列但凍結（status=BLOCKED_*）」，而
	// `chk_invadj_allowance_shape` 要求 ALLOWANCE **一律帶齊** allowance_number／
	// allowance_date_taipei ＋ 三個金額欄。BLOCKED_UNKNOWN_DOCUMENT 的觸發條件正是
	// 「原發票還沒開出來」⇒ 沒有 provider_invoice_no、明細的剩餘基數也還不確定
	// ⇒ 那六個欄位算不出來 ⇒ **那一列建不出來**。兩份權威互相矛盾，schema 贏
	// （它是 DB 層強制的，而設計書不是）。
	//
	// 那 BLOCKED 列的價值是什麼？「ADM 看得到有一筆待處理的調整」——而 ADM 對
	// invoice_adjustments **今天零可見性**（api/proto 與 internal/grpc 都零命中）。
	// 所以它今天是一個建了沒人看、還得自己配一套喚醒機制（G04／G05／G06／G07）的
	// 中間狀態，而那套機制的每一條邊都是新的失效面。
	//
	// 退避重試的代價：INV.dl 的 3 次嘗試（60s／2min／4min）之後停泊 6 小時。
	// 開票通常在幾分鐘內完成 ⇒ 多數情況第 2、3 次就成功；最壞情況延後 6 小時，
	// 而折讓的法定期限是月結，撐得住。
	//
	// ⚠️ 設計書另有一句「把該 ISSUE operation 的 next_attempt_at 提前到 now」催開票。
	// **那一句對註 A 的四個狀態沒有效果**：WAITING_CONFIRMATION／
	// PROVIDER_STATUS_RECHECK／FAILED_UNKNOWN／CANCEL_BLOCKED_UNKNOWN 都不在
	// INV.op 的候選集合裡（它們在等 INV.rc 查詢確認），提前 next_attempt_at
	// 只會讓一個撈不到的述詞早一點被評估。所以不做。
	refundPlanRetryLater
	// refundPlanSkip 是「不需要任何發票調整」，走 MarkInboxEventSkipped。
	refundPlanSkip
)

// refundPlan 是決策樹的輸出。
//
// 🔴 SKIP 的 Disposition 必須是 CodeMeta 的值（`invoice_inbox_events.disposition`
// 是 varchar(40)，而且 ADM 側日後要用它分類）。Reason 是給人看的細節，落在
// disposition_reason。兩者分開是因為前者要能 GROUP BY，後者不行。
type refundPlan struct {
	Kind   refundPlanKind
	Reason string
	// Disposition 只在 Kind == refundPlanSkip 時有值。
	Disposition string
}

// refundFacts 是決策樹的全部輸入。
//
// 刻意做成「事實 struct ＋ 純函式」而不是把 DB 查詢混在樹裡：這棵樹有 9 個節點、
// 每個節點錯了都是稅務後果，而混著 I/O 的樹只能靠整合測試驗——那會讓大多數分支
// 永遠沒有測試走過。
type refundFacts struct {
	Doc                    domain.Document
	RefundAmount           int64
	CumulativeRefundAmount int64

	// IssueResultUnknown 是註 A 的述詞（開票結果未確認）。
	IssueResultUnknown bool

	// CorrectionInFlight 是序列閘門 I5 的 **correction 那一半**（非終態的 VOID／REISSUE）。
	CorrectionInFlight bool

	// AdjustmentInFlight 是同一道閘門的**另一半**（非終態的 ALLOWANCE／VOID_ALLOWANCE）。
	//
	// 🔴 兩半分成兩個欄位而不是一個，是為了讓 inbox 的 disposition_reason 分得出
	// 「被更正擋住」與「被前一筆折讓擋住」——兩者對 ADM 的下一步不同（前者等更正走完、
	// 後者等前一筆折讓終態或去 A06 處理它）。
	AdjustmentInFlight bool

	Now time.Time
}

// planRefundAdjustment 是 §7.2.2 的決策樹，純函式。
//
// 節點順序**不可調換**：B（已有折讓）必須在 C（金額）之前，否則「部分退款後又全額退款」
// 會走到 VOID，而 AMEGO 對已折讓的發票一律拒絕作廢（C-6）。
func planRefundAdjustment(f refundFacts) refundPlan {
	doc := f.Doc

	// ── 前置 I-46：只服務 AMEGO_AUTO 的 B2C／B2B ──────────────────────────
	// 其餘三種 mode 的調整不由本服務執行（§5.1.7）：SELLER_SELF_ISSUE 通知賣家自理、
	// PLATFORM_RECEIPT／LEGACY_MANUAL 只重開或註記收據。硬建 adjustment 的話，
	// 執行軸會拿一張根本不在 AMEGO 上的憑證去打 g0401。
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

	// ── 序列閘門 I5：correction 佔用中 ⇒ 退避重試 ───────────────────────
	// 在註 A 之前判：correction 在途時 document 也常常處於「結果未確認」，
	// 兩者同時成立時真正的阻擋者是 correction（它會換掉生效中的那張發票）。
	if f.CorrectionInFlight {
		return refundPlan{Kind: refundPlanRetryLater, Reason: "CORRECTION_IN_FLIGHT"}
	}

	// ── 註 A：開票結果未確認 ⇒ 退避重試 ─────────────────────────────────
	if f.IssueResultUnknown {
		return refundPlan{Kind: refundPlanRetryLater, Reason: "ISSUE_RESULT_UNKNOWN"}
	}

	switch doc.Status {
	case codemeta.InvoiceDocumentStatusIssued.Value:
		// 往下走可調整性判定。
	case codemeta.InvoiceDocumentStatusPending.Value,
		codemeta.InvoiceDocumentStatusIssuing.Value,
		codemeta.InvoiceDocumentStatusIssueFailed.Value:
		// 還沒開出來 ⇒ 退避重試，等它進 ISSUED 或終態。
		//
		// ⚠️ 設計書 §7.2.2 入向流程對這一格另有「全額 ⇒ 取消開票／有餘額 ⇒ 重建
		// snapshot」的寫法。本實作**不重建 snapshot**：那要改 snapshot_version 並重算
		// lines，是 Issuance 的職責。等發票開出來之後走折讓，稅務結果是「先開後折」
		// ——合法、且對買家可見（會收到 P4）。差別只在憑證張數，不在金額。
		return refundPlan{Kind: refundPlanRetryLater, Reason: "DOCUMENT_NOT_ISSUED_YET"}
	default:
		// VOIDED／SUPERSEDED／CANCELLED_BEFORE_ISSUE／NOT_APPLICABLE／
		// RESOLVED_EXTERNALLY：沒有可調整的發票。**退款本身不受影響**。
		return refundPlan{Kind: refundPlanSkip,
			Disposition: codemeta.InvoiceInboxDispositionNoAdjustableDocument.Value,
			Reason:      "document.status=" + doc.Status + "，沒有可調整的發票；退款本身不受影響"}
	}

	// ── 序列閘門 I5 的另一半：ALLOWANCE／VOID_ALLOWANCE 佔用中 ⇒ 退避重試 ──
	//
	// 🔴 位置刻意在 status switch **之後**（不與 CorrectionInFlight 併排）：
	// 那一格已經把 SUPERSEDED／VOIDED／NOT_APPLICABLE 判成 SKIP，而那些 document 上
	// 佔著閘門的 operation 不該把「本來就不需要調整」的退款變成無限退避
	// （dev 實查 2026-09-05：8 張佔閘門的 document 有 7 張是 SUPERSEDED／VOIDED）。
	//
	// 🔴 沒有這一格的後果不是「多跑幾個節點」而是**建列必然撞 DB**：
	// `uq_invop_open_adjustment_per_document` 的 operation_type 述詞是四種
	// （VOID／REISSUE／ALLOWANCE／VOID_ALLOWANCE），而 CorrectionInFlight 只涵蓋前兩種
	// ⇒ 第二筆退款會一路走到 createOperation 才被擋，那時折讓單號已經取走。
	// 在這裡擋掉，取號與建列都不會發生。createOperation 的回讀是同一件事的**兜底**
	// （兩個副本同時通過本格的 race），不是替代品。
	if f.AdjustmentInFlight {
		return refundPlan{Kind: refundPlanRetryLater, Reason: "ADJUSTMENT_IN_FLIGHT"}
	}

	// ── B：已有生效折讓 ⇒ 一律補折讓（AMEGO 3050141）────────────────────
	// 🔴 直接讀 doc 上的反正規化欄位，**不做成 facts 的獨立欄位**：同一件事的兩份
	// 表示必然漂移，而漂移的方向是「呼叫端算對了、測試算錯了」或反過來——兩者都
	// 讓這個節點失去保護。它是 I4 的單句 CAS 在維護的欄位，本身就是權威。
	if f.Doc.AllowanceTotalAmount > 0 {
		return refundPlan{Kind: refundPlanAllowance, Reason: "HAS_EXISTING_ALLOWANCE"}
	}

	// ── C：累計退款 == 發票金額？否 ⇒ 部分折讓 ──────────────────────────
	// 🔴 左手邊取 max(cumulative, 已折讓 + 本次)：cumulative 由上游算，而上游每開一張
	// 新售後單都拿 orders.total_actual_amount 當可退上限、不扣既退（§5A.9.1 事實 2）
	// ⇒ 它可能落後於我方帳上的事實。取大的那個，方向是「更容易判成全額」——而全額
	// 之後還有 D／E／F 三道閘門，判錯只是多走幾個節點；判小則直接漏掉 VOID 路徑。
	cumulative := f.CumulativeRefundAmount
	if own := doc.AllowanceTotalAmount + f.RefundAmount; own > cumulative {
		cumulative = own
	}
	if cumulative != doc.TotalAmount {
		return refundPlan{Kind: refundPlanAllowance, Reason: "PARTIAL_REFUND"}
	}

	// ── D：ReportedStatus 三態 ────────────────────────────────────────────
	// 只有 NOT_REPORTED 往作廢走；REPORTED 與 UNKNOWN 一律折讓（fail-closed）。
	if doc.TaxPeriodCode == nil || *doc.TaxPeriodCode == "" {
		return refundPlan{Kind: refundPlanAllowance, Reason: "NO_TAX_PERIOD_CODE"}
	}
	reported, err := domain.ReportedStatus(*doc.TaxPeriodCode, f.Now)
	if err != nil || reported != domain.ReportedStatusNotReported {
		return refundPlan{Kind: refundPlanAllowance, Reason: "REPORTED_STATUS_" + reported}
	}

	// ── E：AMEGO 可作廢期限內？（關帳鎖定＝期別結束後單月 5 日 23:59:59）──
	//
	// ⚠️ **在現行 ReportedStatus 算式下，D 嚴格強於 E ⇒ 這一格今天走不到 false。**
	// NOT_REPORTED 的條件是 `now < 期末`，而關帳鎖定在**次期**首月 5 日
	// ⇒ now < 期末 < 關帳日，永遠在窗口內。留著它不是冗餘：
	//   ① 設計書 §7.2.2 把兩者列為獨立節點，拿掉會讓 code 與設計書對不上
	//   ② ReportedStatus 的三態邊界若日後調整（例如把 UNKNOWN 收窄），
	//      這一格立刻變成有效的第二道閘門
	// 對應的測試 TestPlanRefundAdjustment_VoidLockIsShadowedByReportedStatus
	// 驗的正是「它今天走不到」這件事本身——那比一支恆綠的假測試誠實。
	// 走既有的 WithinVoidWindow（＝ProviderCloseDeadlineTaipei），不要另算一次：
	// 關帳日的算式各寫一份必然漂，而漂掉的那一份沒有任何東西會發現。
	inWindow, err := domain.WithinVoidWindow(*doc.TaxPeriodCode, f.Now)
	if err != nil || !inWindow {
		return refundPlan{Kind: refundPlanAllowance, Reason: "PAST_VOID_LOCK"}
	}

	// ── F：invoice_status = 99（AMEGO 已上傳完成）？──────────────────────
	// 🔴 未完成時設計書的處置是「等 5 分鐘重跑整棵樹」，但 `invoice_adjustments`
	// 上**沒有 next_attempt_at 欄位**（schema 實查），那個等待狀態今天沒有落點，
	// 而且它需要一個重跑決策樹的執行者。本實作直接走折讓：
	//   - 方向與 D 節點的 UNKNOWN 分支一致（fail-closed 往折讓倒）
	//   - 後果是「該作廢的開了折讓」，金額與買家收到的通知都正確，差別只在會計慣例
	//   - 反方向（造一個等待狀態卻沒有執行者）會讓這筆退款無聲卡住，那才是壞的
	if doc.ProviderUploadStatus == nil || *doc.ProviderUploadStatus != amegoUploadCompleted {
		return refundPlan{Kind: refundPlanAllowance, Reason: "UPLOAD_NOT_COMPLETED"}
	}

	return refundPlan{Kind: refundPlanVoid, Reason: "FULL_REFUND_VOIDABLE"}
}

// amegoUploadCompleted 是 AMEGO invoice_status 的「上傳完成」值（§4.5.10）。
const amegoUploadCompleted int16 = 99

// ---------------------------------------------------------------------------
// 執行面
// ---------------------------------------------------------------------------

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
		// consumer 已經 fail-loud 過一次，這裡是第二道：用空字串當 refund_no 會讓
		// uq_invadj_allowance_refund 的 partial 述詞不成立（它要求 NOT NULL），
		// 冪等保護整個失效 ⇒ 每次重投開一張新折讓單。
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
		// 不需要任何發票調整。**退款本身不受影響**——錢已經退了，只是沒有憑證要動。
		// 回 RefundSkip 而不是 nil：呼叫端要標 SKIPPED ＋ disposition，不是 PROCESSED。
		return &RefundSkip{Disposition: plan.Disposition, Reason: plan.Reason}, nil
	case refundPlanVoid:
		return nil, u.createVoid(ctx, in, doc, now)
	case refundPlanAllowance:
		return u.createAllowance(ctx, in, doc, now, codemeta.InvoiceAdjustmentStatusPending.Value)
	case refundPlanRetryLater:
		// 回 error ⇒ inbox 標 FAILED ＋ 退避重試（INV.dl 的 3 次 ＋ 6 小時停泊）。
		// **不是 SKIPPED**：那個狀態說「刻意不做」，而這裡是「現在還做不了」。
		return nil, fmt.Errorf(
			"invoice: refund %s for document %d not adjustable yet (%s); will retry",
			in.RefundRef, doc.ID, plan.Reason)
	default:
		return nil, fmt.Errorf("invoice: unknown refund plan %d", plan.Kind)
	}
}

// issueResultUnknown 是 §7.2.2 註 A 的述詞，**兩個條件都要**。
//
// 逐字：`d.status IN ('ISSUING','ISSUE_FAILED')` AND ISSUE operation 的 status ∈
// {WAITING_CONFIRMATION, PROVIDER_STATUS_RECHECK, FAILED_UNKNOWN, CANCEL_BLOCKED_UNKNOWN}。
// 只判 document 那一半會把「還在正常重試中」也凍結起來；只判 operation 那一半會把
// 已經開好的發票也凍結。
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

// holdsSequenceGate 是 `uq_invop_open_adjustment_per_document`（I5 序列閘門）的
// **狀態述詞**，逐字對齊那個 partial 索引的 `status <> ALL (...)` 那一半。
//
// 🔴 述詞是**否定**的（NOT IN 排除清單）而不是肯定列舉：肯定列舉在新增一個中間狀態
// 時會靜默漏掉，而漏掉的後果是兩件事同時對同一張發票動手。
//
// ⚠️ 排除清單只有五個，**FAILED_PERMANENT 不在裡面**——一筆「AMEGO 永久拒絕、等 ADM
// 處理」的調整會持續佔著閘門好幾天，那不是邊界情況而是本閘門最常見的觸發原因。
//
// 這份清單是索引與應用層閘門的**唯一**來源：兩個 in-flight 述詞都呼叫它，各自只加
// operation_type 的過濾。各寫一份必然漂，而漂掉的那一份沒有任何東西會發現。
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

// correctionInFlight 是序列閘門 I5 的 **correction 那一半**：該 document 有非終態的
// VOID／REISSUE operation。
//
// ⚠️ 它**不等於**索引本身：`uq_invop_open_adjustment_per_document` 的 operation_type
// 述詞是四種（VOID／REISSUE／ALLOWANCE／VOID_ALLOWANCE），這裡只涵蓋前兩種。
// 另外兩種由 `adjustmentInFlight` 涵蓋，兩者合起來才是那個索引。
// （這段註解原本自稱與索引「逐字一致」——狀態集合確實一致，operation_type 集合
// 只涵蓋 4 種裡的 2 種，而那個差額正是 P0「孤兒折讓單」的應用層成因。）
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

// adjustmentInFlight 是序列閘門 I5 的**另一半**：該 document 有非終態的
// ALLOWANCE／VOID_ALLOWANCE operation。
//
// 🔴 VOID_ALLOWANCE 必須列進來，即使今天沒有任何路徑建得出它：索引述詞含它，
// 而應用層閘門比索引窄的那一格，症狀是「建列時才撞、且撞出來是靜默的」。
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

// createAllowance 建 ALLOWANCE adjustment ＋ lines ＋ operation（G01）。
//
// status 恆為 PENDING：兩個 BLOCKED_* 已改走退避重試（見 refundPlanRetryLater 的說明）。
// 參數留著是因為 chk_invadj_status 的值域比這裡用到的多，寫死會讓「哪些狀態合法」
// 這件事從呼叫端消失。
func (u *RefundAdjustment) createAllowance(
	ctx context.Context, in RefundAdjustmentInput, doc domain.Document, now time.Time, status string,
) (*RefundSkip, error) {
	if doc.ProviderInvoiceNo == nil || *doc.ProviderInvoiceNo == "" {
		// 走到這裡代表決策樹放行了一張沒有號碼的 ISSUED 發票——那是不變式破了
		// （MarkIssued 與 provider_invoice_no 在同一句 SQL 內回填）。
		// 擋下來讓它留在 inbox 上看得見，不要建一張掛著空發票號碼的折讓單：
		// 那種單送出必被 AMEGO 拒絕，而那時已經來不及。
		return nil, fmt.Errorf(
			"invoice: document %d is %s but has no provider_invoice_no, cannot build allowance lines",
			doc.ID, doc.Status)
	}

	docLines, err := u.docs.ListLines(ctx, doc.ID)
	if err != nil {
		return nil, err
	}

	// ── §6.3.2.3 的 clamp（法定不變式：累計折讓 ≤ 已開立發票金額）─────────
	//
	// 🔴 上游的 `refundable_amount` **不扣既退**（每開一張新售後單都拿
	// orders.total_actual_amount 當可退上限，§5A.9.1 事實 2）⇒ 第二筆退款的金額
	// 可以大於這張發票還能折的餘額。設計書對這一格的處置逐字是：
	//   「折讓累計一律 min(cumulative, invoiced_total)，並遞增 metric
	//     invoice_refund_exceeds_invoiced_total 觸發告警，**但不得因此擋住折讓**」
	//
	// 這裡曾經沒有 clamp：超額直接回 error ⇒ inbox 標 FAILED ⇒ 訊息進 DLQ ⇒
	// **買家一張折讓單都拿不到**，而 ADM 對 inbox 零可見性、要靠維運 SQL 才處理得掉
	// （2026-09-05 第 6 輪稽核）。退款是既成事實，把能開的那部分開出來才是對的：
	// 買家拿到最大合法金額的憑證，差額由 Error log ＋ M16 交給人判。
	effective, clampedFrom := clampAllowanceToRemaining(doc, docLines, in.RefundAmount)
	if effective < clampedFrom {
		observeRefundExceedsInvoiced()
		// 🔴 這則 Error log **不是選配**：M16 無 label（label 基數紅線），
		// 「是哪一家、哪一張」只有這裡答得出來（§8.2.3 M16 逐字要求）。
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
		// 累計折讓已經等於發票金額 ⇒ 沒有東西可以開。**退款本身不受影響。**
		// 回 SKIPPED ＋ disposition 而不是 error：後者會讓這則事件無限重試
		// （金額永遠不會變小），而那是「刻意不做」不是「現在還做不了」。
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
		// 走到這裡代表 clamp 之後**仍然**建不出草稿 ⇒ 不是金額超額（那已經在上面
		// clamp 掉了），而是明細層的分攤或稅別出了問題。留在 inbox 上讓它看得見。
		return nil, fmt.Errorf("invoice: build allowance draft for document %d: %w", doc.ID, err)
	}
	if err := draft.Validate(); err != nil {
		return nil, fmt.Errorf("invoice: allowance draft invariant for document %d: %w", doc.ID, err)
	}

	// 🔴 取號在**送出之前**、且在建列的同一交易內（§5.3.3）：AMEGO 開立折讓成功後
	// 不回傳任何識別碼，這個號碼是「送出成功但回應遺失」時唯一的救回路徑。
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
			// ON CONFLICT DO NOTHING 命中：併發的第二個副本已經建過。
			// GetByRefund 那一關沒攔到代表兩者同時進來——不是錯誤。
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

// createVoid 建 VOID adjustment ＋ operation。作廢沒有明細（chk_invadj_allowance_shape
// 要求 VOID 的六個折讓欄全部為 NULL）。
func (u *RefundAdjustment) createVoid(
	ctx context.Context, in RefundAdjustmentInput, doc domain.Document, now time.Time,
) error {
	adjID := u.newID()
	refundNo := in.RefundRef
	// void_reason_code 這一欄沒有 CHECK 也沒有 CodeMeta 值域（DDL 註解：「AMEGO f0501
	// CancelReason 對應」）。存 trigger_kind 的值而不是自己造一個字串：送給 AMEGO 的
	// CancelReason 正是由它推導（Voiding.cancelReasonFor），兩處錨在同一個碼上。
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

// createOperation 建調整類 operation（G01 的後半）。
func (u *RefundAdjustment) createOperation(
	ctx context.Context, documentID, adjustmentID int64, opType string, now time.Time,
) error {
	key, ok := domain.OperationKeyFor(opType, documentID, adjustmentID, 0, 0)
	if !ok {
		// 值域外一律當錯誤，**不得** fallback 成任一前綴——撞到別人的鍵就是把兩件事
		// 當成同一件（OperationKeyFor 的逐字要求）。
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
		// 🔴 rows=0 有**兩種**意義，必須回讀 operation_key 分辨（`InsertInvoiceOperation`
		// 的檔頭逐字：「分辨方式是先讀 operation_key，不是猜」）：
		//
		//   ① 查得到 ⇒ 同一把 key 重送（重投／併發的第二個副本）⇒ 冪等，不是錯誤。
		//   ② 查不到 ⇒ 撞的是 `uq_invop_open_adjustment_per_document`（I5 序列閘門）：
		//      這張 document 上還有一筆非終態的調整類 operation 佔著。
		//
		// **② 當成冪等 return nil 是靜默資料遺失**：adjustment 建出來了（單號也取了）、
		// operation 一列都沒有、inbox 標 PROCESSED ⇒ 買家的第二筆退款永遠拿不到折讓單，
		// 而唯一的訊號是 `invoice_adjustment_stuck_rows` 的 24h 述詞 ＋ 告警 for:1h
		// ⇒ 最快 25 小時。dev 實測（2026-09-05，adjustment 7501688835508535296）：
		// 索引排除清單不含 FAILED_PERMANENT ⇒ 一筆「等 ADM 處理」的失敗折讓可以佔著
		// 閘門好幾天，期間該張發票的每一筆退款都會走進這條路。
		//
		// ②（以及反查本身失敗）一律回 error ⇒ 整筆 rollback（adjustment 與號碼都不落庫）
		// ⇒ inbox 標 FAILED ＋ 退避重試，等前一筆調整終態後自然成功。這與
		// `refundPlanRetryLater` 是同一個機制，不另造一套。
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

	// 🔴 **必須 promote 到 READY**：`Create` 落在 `CREATED`，而 INV.op 的候選述詞是
	// `status IN ('READY','RETRY_SCHEDULED','SENDING')` ——停在 CREATED 的 operation
	// 沒有任何執行者，而且完全無聲（adjustment 是 PENDING、operation 是 CREATED，
	// 兩者看起來都「剛建好、等著跑」）。
	//
	// dev 實測就是這樣抓到的：第一次 E2E 建出了正確的 adjustment ＋ 明細 ＋ operation，
	// 然後**什麼都沒發生**。
	//
	// 調整類沒有 issue_not_before 那種延後條件（那是開票的 J03），可調整性判定已經在
	// planRefundAdjustment 做完 ⇒ 一律 READY ＋ next_attempt_at = now，與
	// Correction.promote 那一段逐字同理。
	promoted, err := u.ops.Promote(ctx, op.ID, codemeta.InvoiceOperationStatusReady.Value, now, now)
	if err != nil {
		return err
	}
	if !promoted {
		// rows=0 ⇒ 那筆已不在 CREATED。整筆 rollback：放行會留下一筆沒有人會撿的調整，
		// 而那正是本段要防的事。
		return fmt.Errorf("invoice: operation %d could not be promoted to READY (rows=0)", op.ID)
	}
	return appendStatusHistory(ctx, u.audit, subjectOperation, op.ID,
		codemeta.InvoiceOperationStatusCreated.Value,
		codemeta.InvoiceOperationStatusReady.Value, "", "", now)
}

// allowanceSourceLines 把落庫的 DocumentLine 折成 BuildAllowanceDraft 吃的 domain.Line。
//
// 只取分攤算式真正吃到的六欄。**不要**整個 struct 硬轉：兩邊的欄位集合會各自演化，
// 硬轉會在其中一邊長胖時默默帶進不該進來的值（同 reissueHeaderLines 的理由）。
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

// adjustmentLinesOf 把草稿折成待寫入的明細列。
//
// 🔴 SourceLineID 走 line_seq → id 的查表，**不是**用切片位置對：草稿會跳過
// a_i == 0 的行（BuildAllowanceDraft 的 `if a == 0 { continue }`），兩邊的索引
// 從第一個被跳過的行開始就錯開，而錯開的後果是折讓額累加到別的品項上——
// 那會讓下一次折讓的剩餘基數算錯，AMEGO 4040173 到很後面才發現。
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

// invoiceDateYmd 回傳原發票的台灣曆日 Ymd（AMEGO 折讓明細的 OriginalInvoiceDate）。
func invoiceDateYmd(doc domain.Document) string {
	if doc.InvoiceDateTaipei == nil {
		return ""
	}
	return doc.InvoiceDateTaipei.In(timeutil.LocationTaipei).Format("20060102")
}
