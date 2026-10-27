package usecase

import (
	"github.com/Will413028/hawkr/go-pkg/codemeta"
)

// 本檔是「UI 視圖名 → DB 狀態集合」的展開表，也是本服務唯一的展開處。
//
// 🔴 **視圖名不是狀態值**（§6.1.11／§6.1.12／§6.1.14／§6.1.16）：`AWAITING_SELLER`、
// `NEEDS_MANUAL`、`NEW`、`ADM_REVIEW` 在 §5 與 §7 皆零命中。把它們直接寫進
// `WHERE status = ?` 查出來的恆為空集合，而空集合看起來就像「今天沒有這類案件」。
//
// 🔴 每個值都取自 CodeMeta 常數，不寫字面值（CLAUDE.md §8）：碼表改名時這裡會編譯失敗，
// 而字面值只會安靜地變成查不到。
//
// 🔴 展開表放在**呼叫端**而不是 SQL（db/queries 的區塊註解逐字）：SQL 一律收已展開的
// `text[]`。要改視圖歸屬就改本檔，不必動 query。

// ---------------------------------------------------------------------------
// 文件狀態集合
// ---------------------------------------------------------------------------

var (
	// abnormalDocumentStatuses 是端點 30 `abnormal_only=true` 的命中集合
	// （58A §S06 ⑥：開立失敗與待人工兩類）。
	//
	// 🔴 呼叫端必須在 AbnormalOnly=true 時同時給這個集合：SellerDocumentQuery 的註解
	// 逐字說明「集合為空 ⇒ 查不到任何列」，那是傳錯不是「沒有異常」。
	abnormalDocumentStatuses = []string{
		codemeta.InvoiceDocumentStatusIssueFailed.Value,
		codemeta.InvoiceDocumentStatusAwaitingManual.Value,
	}

	// pendingDocumentStatuses 是 S01 `pendingDocumentCount`（「N 筆處理中」）的口徑。
	//
	// 取 58A §S06 狀態 Chip 表的「處理中／待開立」那一群（`warning`），**不含**
	// `ISSUE_FAILED`／`AWAITING_MANUAL`：那兩個在同一張表裡是獨立的 `danger` 群，
	// 併進來會讓賣家看到「N 筆處理中」而其中幾筆其實是壞掉的、需要他去看 S06。
	pendingDocumentStatuses = []string{
		codemeta.InvoiceDocumentStatusPending.Value,
		codemeta.InvoiceDocumentStatusIssuing.Value,
		codemeta.InvoiceDocumentStatusVoiding.Value,
	}

	// manualDocumentStatuses 是「fail-closed 落點」那一個文件狀態。
	//
	// 它在端點 37／50 都以**聯集**的形式參與 job 的 NEEDS_MANUAL 判定：
	// AWAITING_MANUAL 屬於 document 不屬於 job（§6.1.14 逐字）。
	manualDocumentStatuses = []string{
		codemeta.InvoiceDocumentStatusAwaitingManual.Value,
	}

	// resolvedDocumentStatuses 是「這份憑證已經收束，沒有人要處理它了」的文件狀態。
	//
	// 🔴 它是 needs_manual 的**否決**條件，不是來源條件（#3014）：一筆
	// FAILED_PERMANENT 的 ISSUE operation，在它的 document 被作廢／被重開票取代之後
	// 已經沒有下一步——重開那一步就是處理。dev 實測 needs_manual=4 全部是這種票，
	// 而 A05 的每一筆都在要求管理員去看一件已經結束的事。
	//
	// ⚠️ **AWAITING_MANUAL 不在內**（它正是「需要人」那一格）；
	// ISSUE_FAILED 也不在內（它還等著 ADM 安全重試）。
	resolvedDocumentStatuses = []string{
		codemeta.InvoiceDocumentStatusVoided.Value,
		codemeta.InvoiceDocumentStatusSuperseded.Value,
		codemeta.InvoiceDocumentStatusCancelledBeforeIssue.Value,
		codemeta.InvoiceDocumentStatusNotApplicable.Value,
		codemeta.InvoiceDocumentStatusResolvedExternally.Value,
	}
)

// ---------------------------------------------------------------------------
// 端點 34（S09 賣家申訴列表）：三個視圖互斥且窮盡
// ---------------------------------------------------------------------------

// sellerComplaintViews 是 §6.1.11 對照表的逐列實作。
//
// 三個視圖互斥且窮盡（7 個 DB 值各落在且只落在一個視圖）——**這一點與端點 58 相反**，
// 那裡的 OVERDUE 是刻意的橫切視圖。兩處不可互相套用。
var sellerComplaintViews = map[string][]string{
	// 兩者對賣家的手勢完全相同：該你回了。OVERDUE 做成第四個分頁會讓逾期件從
	// 「待回覆」消失，而那正是最該被看到的那幾件。
	"AWAITING_SELLER": {
		codemeta.InvoiceComplaintStatusOpen.Value,
		codemeta.InvoiceComplaintStatusOverdue.Value,
	},
	// ESCALATED_TO_ADM 對賣家而言就是「已回覆、等平台」。
	"SELLER_RESPONDED": {
		codemeta.InvoiceComplaintStatusSellerResponded.Value,
		codemeta.InvoiceComplaintStatusEscalatedToAdm.Value,
	},
	// 三個**已結案**狀態，不是「三個終態」：RESOLVED 有重開邊（端點 65）⇒
	// 案件可以從本分頁離開，S09 的分頁計數不是單調遞增的。
	"CLOSED": {
		codemeta.InvoiceComplaintStatusResolved.Value,
		codemeta.InvoiceComplaintStatusViolationConfirmed.Value,
		codemeta.InvoiceComplaintStatusWithdrawn.Value,
	},
}

// openComplaintStatuses 是「未結案」的申訴集合（＝7 值扣掉三個已結案狀態，§6.1.11a）。
// 端點 16 與 37 的 `open_complaints` KPI、端點 38 的 riskFlags 都讀它。
var openComplaintStatuses = []string{
	codemeta.InvoiceComplaintStatusOpen.Value,
	codemeta.InvoiceComplaintStatusOverdue.Value,
	codemeta.InvoiceComplaintStatusSellerResponded.Value,
	codemeta.InvoiceComplaintStatusEscalatedToAdm.Value,
}

// overdueComplaintStatuses 是「掃描已寫入」的逾期集合。
//
// 🔴 它只是 `overdue_complaints` 口徑的**一半**：另一半是「未結案且 sla_due_at 已過」，
// 由 SQL 以 OR 補上（OVERDUE 由 INV.dl 掃描寫入，掃描到期之間有一段窗口）。
// 只用這個集合會讓賣家／ADM 在最該行動的那幾小時看到 0。
var overdueComplaintStatuses = []string{
	codemeta.InvoiceComplaintStatusOverdue.Value,
}

// ---------------------------------------------------------------------------
// 端點 58（A07 ADM 申訴列表）：五個 tab 不互斥
// ---------------------------------------------------------------------------

// adminComplaintTab 是一個 tab 的展開結果。
//
// Statuses 與 DueOverdue 兩段在 SQL 裡是 **OR**（§6.1.16）：寫成 AND 會讓 OVERDUE
// 分頁只剩「掃描已寫入」的那一部分。
type adminComplaintTab struct {
	Statuses          []string
	IncludeDueOverdue bool
	DueOverdue        []string
}

// adminComplaintTabs 是 §6.1.16 對照表的逐列實作。
//
// 五個 tab 的筆數加總 ≠ 總筆數（OVERDUE 與前三者交集）⇒ A07 的 UI 不得顯示「合計」，
// 也不得用「總數 − 其他四頁」倒算任何一頁。
var adminComplaintTabs = map[string]adminComplaintTab{
	"AWAITING_SELLER": {Statuses: []string{
		codemeta.InvoiceComplaintStatusOpen.Value,
		codemeta.InvoiceComplaintStatusOverdue.Value,
	}},
	"SELLER_RESPONDED": {Statuses: []string{
		codemeta.InvoiceComplaintStatusSellerResponded.Value,
	}},
	"ADM_REVIEW": {Statuses: []string{
		codemeta.InvoiceComplaintStatusEscalatedToAdm.Value,
	}},
	// 橫切視圖：三類案件會從單純 `status='OVERDUE'` 底下消失（掃描還沒跑到／賣家
	// 逾期後才回覆／已升級到 ADM），而它們正是 ADM 最該看到的。
	"OVERDUE": {
		Statuses:          []string{codemeta.InvoiceComplaintStatusOverdue.Value},
		IncludeDueOverdue: true,
		DueOverdue: []string{
			codemeta.InvoiceComplaintStatusOpen.Value,
			codemeta.InvoiceComplaintStatusSellerResponded.Value,
			codemeta.InvoiceComplaintStatusEscalatedToAdm.Value,
		},
	},
	"CLOSED": {Statuses: []string{
		codemeta.InvoiceComplaintStatusResolved.Value,
		codemeta.InvoiceComplaintStatusViolationConfirmed.Value,
		codemeta.InvoiceComplaintStatusWithdrawn.Value,
	}},
}

// violationComplaintStatus 是違規計數的唯一計入狀態（只計 DECIDE_VIOLATION 落地的那個）。
var violationComplaintStatus = codemeta.InvoiceComplaintStatusViolationConfirmed.Value

// ---------------------------------------------------------------------------
// 端點 50（A05 工作與異常佇列）：六個分頁是 20 個 job 狀態的分割
// ---------------------------------------------------------------------------

// adminJobTabs 是 §6.1.14 對照表的逐列實作。
//
// 🔴 六個集合的**聯集 ＝ 20 值、兩兩交集 ＝ 空**（分割計數 4＋2＋3＋7＋2＋2＝20）。
// 少一個值 ⇒ 那個狀態的 job 在六個分頁裡都看不到（不是報錯，是消失）；多算一個 ⇒
// 同一筆出現在兩頁、ADM 處理兩次。兩種失敗都不會有任何測試轉紅，
// 除非那條驗算被寫成測試 —— 它是 TestAdminJobTabs_PartitionsAllOperationStatuses。
var adminJobTabs = map[string][]string{
	"PENDING": {
		codemeta.InvoiceOperationStatusCreated.Value,
		codemeta.InvoiceOperationStatusAwaitingTrigger.Value,
		codemeta.InvoiceOperationStatusReady.Value,
		codemeta.InvoiceOperationStatusHeld.Value,
	},
	"RETRYING": {
		codemeta.InvoiceOperationStatusRetryScheduled.Value,
		codemeta.InvoiceOperationStatusSending.Value,
	},
	// SELLER_ACTION_REQUIRED 在這裡不在 NEEDS_MANUAL：要動手的是**賣家**
	// （去財政部申請丁種字軌），配到字軌後系統會自己接回自動線（J33）。
	"NEEDS_ATTENTION": {
		codemeta.InvoiceOperationStatusFailedPermanent.Value,
		codemeta.InvoiceOperationStatusFailedRetryExhausted.Value,
		codemeta.InvoiceOperationStatusSellerActionRequired.Value,
	},
	// WAITING_CONFIRMATION 不是在重試而是「送出去了但不知道結果」；
	// PROVIDER_STATUS_RECHECK／CANCEL_REQUESTED／CANCELLED_BY_ADMIN 都還沒有結論。
	"NEEDS_MANUAL": {
		codemeta.InvoiceOperationStatusWaitingConfirmation.Value,
		codemeta.InvoiceOperationStatusProviderStatusRecheck.Value,
		codemeta.InvoiceOperationStatusFailedUnknown.Value,
		codemeta.InvoiceOperationStatusCancelRequested.Value,
		codemeta.InvoiceOperationStatusCancelBlockedUnknown.Value,
		codemeta.InvoiceOperationStatusCancelledByAdmin.Value,
		codemeta.InvoiceOperationStatusManualProcessAllowed.Value,
	},
	"COMPLETED": {
		codemeta.InvoiceOperationStatusSucceeded.Value,
		codemeta.InvoiceOperationStatusResolvedExternally.Value,
	},
	"CANCELLED": {
		codemeta.InvoiceOperationStatusCancelledBeforeIssue.Value,
		codemeta.InvoiceOperationStatusNotApplicable.Value,
	},
}

// adminJobNeedsManualTab 是唯一會聯集 document.status 的分頁名。
const adminJobNeedsManualTab = "NEEDS_MANUAL"

// openJobStatuses 是「還沒了結」的 job 集合（端點 37 的 priorityJobs 候選 ＋
// `pending_jobs` KPI 的母體）。
//
// 它是 PENDING ∪ RETRYING ∪ NEEDS_ATTENTION ∪ NEEDS_MANUAL —— 由上面那張分割表推導
// 而不是另打一份清單：另打一份就會在有人改分頁歸屬時安靜地漂移。
func openJobStatuses() []string {
	out := make([]string, 0, 16)
	for _, tab := range []string{"PENDING", "RETRYING", "NEEDS_ATTENTION", adminJobNeedsManualTab} {
		out = append(out, adminJobTabs[tab]...)
	}
	return out
}

// pendingJobStatuses 是 A01 `pending_jobs` KPI 的口徑（§6.1.11a 逐字：
// A05 `PENDING` ＋ `RETRYING` 兩個分頁的 job 數）。
func pendingJobStatuses() []string {
	out := make([]string, 0, 6)
	out = append(out, adminJobTabs["PENDING"]...)
	out = append(out, adminJobTabs["RETRYING"]...)
	return out
}

// needsManualJobStatuses 是 A01 `needs_manual` KPI 的 job 那一半
// （另一半是 document 的 AWAITING_MANUAL，由 SQL 以 OR 聯集）。
func needsManualJobStatuses() []string { return adminJobTabs[adminJobNeedsManualTab] }

// ---------------------------------------------------------------------------
// 端點 50 的 upload_status：與 tab 正交，不是第七個分頁
// ---------------------------------------------------------------------------

// uploadStatusProjection 是「發票開出來了，但財政部端沒收到」那一類的唯一入口。
//
// DB 存 AMEGO 原始整數（`invoice_status`：1／2／3／31／32／91／99，另實測會回未列舉的 -1），
// 對外只給四個投影值。
//
// 🔴 PENDING 必須同時涵蓋 NULL（還沒查過）與在途碼 ⇒ IncludeNull 是獨立旗標，
// 把它併進整數陣列表達不出來（NULL ≠ 任何整數）。
type uploadStatusProjection struct {
	Codes       []int16
	IncludeNull bool
}

// uploadStatusProjections 的整數值域來自 db/schema/schema.sql 的
// `chk_invdoc_upload_recheck`（在途 ＝ 1,2,3,31,32）與 §7.5 的落點表
// （99 ＝ 財政部已收訖、91 ＝ 錯誤、-1 ＝ NOT_FOUND）。
//
// ⚠️ `-1` 是 AMEGO 實測回的 false negative，**絕不可讀成「這張票沒開成功」**
// （照信會重複開立）——它在這裡只是一個可篩選的投影值。
var uploadStatusProjections = map[string]uploadStatusProjection{
	"PENDING":   {Codes: []int16{1, 2, 3, 31, 32}, IncludeNull: true},
	"UPLOADED":  {Codes: []int16{99}},
	"FAILED":    {Codes: []int16{91}},
	"NOT_FOUND": {Codes: []int16{-1}},
}

// ---------------------------------------------------------------------------
// 端點 38（A02 稅務審核佇列）：六個 filter 不互斥
// ---------------------------------------------------------------------------

// adminTaxReviewFilter 是一個 filter 的展開結果。
//
// 三個條件在 SQL 裡是 **AND**（§6.1.12）：NEAR_DEADLINE 與 EXTENDED 的定義都是
// 「前四狀態之一 AND 額外條件」的交集視圖。做成互斥會讓案件從原分頁消失。
type adminTaxReviewFilter struct {
	Statuses     []string
	NearDeadline bool
	Extended     bool
}

// taxReviewQueueStatuses 是 A02 佇列全集（＝A01 `pending_tax_reviews` KPI 的口徑）。
var taxReviewQueueStatuses = []string{
	codemeta.SellerTaxReviewStatusSubmitted.Value,
	codemeta.SellerTaxReviewStatusUnderReview.Value,
	codemeta.SellerTaxReviewStatusSupplementRequired.Value,
	codemeta.SellerTaxReviewStatusSelfIssuePending.Value,
}

// adminTaxReviewFilters 是 §6.1.12 對照表的逐列實作。
//
// 六個 filter 的筆數加總 ≠ 總筆數 ⇒ A02 的 UI 不得顯示「合計」，
// 也不得用「總數 − 其他五頁」倒算任何一頁。
var adminTaxReviewFilters = map[string]adminTaxReviewFilter{
	"NEW": {Statuses: []string{codemeta.SellerTaxReviewStatusSubmitted.Value}},
	"SUPPLEMENT_REQUIRED": {Statuses: []string{
		codemeta.SellerTaxReviewStatusSupplementRequired.Value,
	}},
	// 兩者都是待 ADM 決策，A02 的手勢不需要分開。
	"UNDER_REVIEW": {Statuses: []string{
		codemeta.SellerTaxReviewStatusUnderReview.Value,
		codemeta.SellerTaxReviewStatusSelfIssuePending.Value,
	}},
	"NEAR_DEADLINE": {Statuses: taxReviewQueueStatuses, NearDeadline: true},
	"EXTENDED":      {Statuses: taxReviewQueueStatuses, Extended: true},
	"EXPIRED":       {Statuses: []string{codemeta.SellerTaxReviewStatusExpired.Value}},
}

// ---------------------------------------------------------------------------
// 端點 37／42：provider 帳號
// ---------------------------------------------------------------------------

// providerErrorStatuses 是 A01 `provider_errors` KPI 的口徑（§6.1.11a）。
//
// ⚠️ `CONFIG_ERROR` 是**投影值不落 DB**（chk_ipa_status 是該 enum 的真子集，§6.1.13
// 三態表）⇒ 它出現在這裡只是為了讓口徑文案與需求一致；SQL 對它永遠查不到列，
// 而那不是 bug：真正的 CONFIG_ERROR 是「有列但 status 為未知值」，
// 只有列表端點的三態投影看得出來。
var providerErrorStatuses = []string{
	codemeta.InvoiceProviderAccountStatusInvalidCredential.Value,
	codemeta.InvoiceProviderAccountStatusProviderRejected.Value,
	codemeta.InvoiceProviderAccountStatusDegraded.Value,
	codemeta.InvoiceProviderAccountStatusConfigError.Value,
}

// projectProviderStatus 把「查無列／有列」折成 §6.1.13 的三態。
//
// 🔴 需求書寫「狀態空值視為配置錯誤」，照字面實作會讓上線當天每一個既有賣家都被標成
// 配置錯誤 —— 查無列是 NOT_STARTED（尚未開始設定，＝上線當天的全體），
// 只有「有列但 status 是空字串／值域外」才是 CONFIG_ERROR。
func projectProviderStatus(hasAccount bool, status string) string {
	if !hasAccount {
		return codemeta.InvoiceProviderAccountStatusNotStarted.Value
	}
	if _, ok := codemeta.InvoiceProviderAccountStatusFromValue(status); !ok {
		return codemeta.InvoiceProviderAccountStatusConfigError.Value
	}
	return status
}

// ---------------------------------------------------------------------------
// 端點 8：買家可見的 state 投影（8 值）
// ---------------------------------------------------------------------------

// 買家可見投影的 9 個值。它們**不納管 CodeMeta**（§C.4）且與 InvoiceDocumentStatus
// 的字串值域刻意分離 —— 兩套字串不可互相代入（§6.1.3）。
//
// ⚠️ `SELLER_WILL_ISSUE` 不是 `SELF_ISSUE_PENDING`：後者是 SellerTaxReviewStatus
// 的值，共用同一字串會讓前端的 codeLabel 查錯表。
//
// ⚠️ `ORDER_CANCELLED`（#2904）刻意**不叫** `NOT_APPLICABLE_CANCELLED`：後者以
// `NOT_APPLICABLE` 開頭，而 `NOT_APPLICABLE` 是 InvoiceDocumentStatus 的值（映到
// SELLER_WILL_ISSUE）。兩套字串本來就規定不可互相代入，而讓其中一個成為另一個的
// 前綴，等於在未來每一次 `strings.HasPrefix` / `startsWith` 上放一顆地雷。
const (
	orderInvoiceStateAwaitingPayment  = "AWAITING_PAYMENT"
	orderInvoiceStateProcessing       = "PROCESSING"
	orderInvoiceStateIssued           = "ISSUED"
	orderInvoiceStateVoided           = "VOIDED"
	orderInvoiceStateAllowanceApplied = "ALLOWANCE_APPLIED"
	orderInvoiceStateFailed           = "FAILED"
	orderInvoiceStateSellerWillIssue  = "SELLER_WILL_ISSUE"
	orderInvoiceStateSelfIssued       = "SELLER_SELF_ISSUED"
	// orderInvoiceStateOrderCancelled 是第 9 個值（#2904）。它是**唯一一個判準不在
	// invoice-service 自己資料上的 state**：來源是 order.cancelled 落下的
	// invoice_cancelled_orders 一列。
	orderInvoiceStateOrderCancelled = "ORDER_CANCELLED"
)

// orderInvoiceStateNotices 是 §6.1.3 對照表第三欄的逐字文案。
//
// ⚠️ `SELLER_SELF_ISSUED` 的文案**不得寫成「平台已開立」**：責任歸屬（誰是開票人）
// 在爭議時是法律問題。
var orderInvoiceStateNotices = map[string]string{
	orderInvoiceStateAwaitingPayment:  "付款完成後會立即開立發票",
	orderInvoiceStateProcessing:       "系統正在處理，完成後會寄送 Email",
	orderInvoiceStateIssued:           "發票已開立，已寄送至您的電子信箱",
	orderInvoiceStateVoided:           "原發票已作廢",
	orderInvoiceStateAllowanceApplied: "已開立折讓單",
	orderInvoiceStateFailed:           "發票開立遇到問題，平台處理中",
	orderInvoiceStateSellerWillIssue:  "本筆由賣家自行開立發票",
	orderInvoiceStateSelfIssued:       "賣家已自行開立發票，如未收到請聯繫賣家",
	// ⚠️ 這句話刻意**不提退款**：訂單取消與退款是兩條不同的流程（未付款的取消
	// 根本沒有錢要退），在發票區承諾退款會變成一個本服務管不到的對外承諾。
	orderInvoiceStateOrderCancelled: "訂單已取消，不會開立發票",
}

// receiptStateNotices 是**收據線**（PLATFORM_RECEIPT／document_kind=RECEIPT）的文案覆蓋。
//
// 🔴 為什麼非有不可：上面那張表是依 `state` 索引的，而 state 不帶 document_kind
// ⇒ 收據會拿到「發票已開立，已寄送至您的電子信箱」。#3026 的 dev 實測就是這樣
// （訂單 O481452453：拿到的是收據 R000000000003，畫面說發票已開立）。
//
// 這是 §5 L11 要防的那件事，只是方向相反：#3022 是「依法不該開發票的賣家被說成
// 開立失敗」，這裡是「拿到收據的買家被說成拿到發票」。**兩者都是畫面對使用者
// 做出與事實不符的告知**，而收據與發票在稅務上不能互相冒充。
//
// ⚠️ 只覆蓋收據**真的會走到**的 state。走不到的（VOIDED 之外的發票專屬狀態）
// 不列——列了會產生「看起來想過、其實從未執行」的死文案。
// 收據線的狀態路徑見 §7.1.2 M1：PENDING → ISSUING → ISSUED → VOIDED/SUPERSEDED。
var receiptStateNotices = map[string]string{
	orderInvoiceStateAwaitingPayment: "付款完成後會立即產製收據",
	orderInvoiceStateProcessing:      "系統正在產製收據，完成後可在此下載",
	orderInvoiceStateIssued:          "收據已產製，可在此下載",
	orderInvoiceStateVoided:          "原收據已作廢",
	orderInvoiceStateFailed:          "收據產製遇到問題，平台處理中",
	orderInvoiceStateOrderCancelled:  "訂單已取消，不會產製收據",
}

// StateNoticeFor 依文件種類選文案。**這是唯一的取用入口**——直接查
// orderInvoiceStateNotices 就是本次 bug 的形狀。
//
// documentKind 為空字串（零 document 的訂單，還沒決定要開什麼）⇒ 走發票那份：
// 那時買家看到的是「付款完成後會立即開立發票」，而絕大多數訂單確實如此；
// 免用發票賣家的訂單在付款前就已知道會走收據線嗎？**不知道**——決策樹在
// order.paid 才跑。這一格是誠實的預設值，不是遺漏。
func StateNoticeFor(documentKind, state string) string {
	if documentKind == codemeta.InvoiceDocumentKindReceipt.Value {
		if s, ok := receiptStateNotices[state]; ok {
			return s
		}
	}
	return orderInvoiceStateNotices[state]
}

// ReceiptShippingFeeNotice 是 ShippingFeeNotice 的收據版本（#3026）。
//
// 原句逐字提到兩次「發票」，掛在收據上就是把收據講成發票。句型與資訊量保持一致：
// 要說的是同一件事（運費已在總額內、另列的物流明細不是第二份憑證）。
const ReceiptShippingFeeNotice = "物流費用已包含在本張收據的總額中，另列的物流明細不是另一張收據。"

// ShippingFeeNoticeFor 依文件種類選運費說明。
func ShippingFeeNoticeFor(documentKind string) string {
	if documentKind == codemeta.InvoiceDocumentKindReceipt.Value {
		return ReceiptShippingFeeNotice
	}
	return ShippingFeeNotice
}

// ShippingFeeNotice 是 B04／B05 底部 info banner 的定版字串（58A §B04 畫面表，#3031）。
//
// 🔴 **逐字，不得改寫**：58A 兩處都寫「逐字顯示後端 shippingFeeNotice」，B05 那頁還註明
// 「本頁沒有該欄位時用同一句常數」⇒ 前端有一份硬編碼的同句話，兩邊一旦漂移，
// 同一個買家在兩個頁面會看到兩種說法。
const ShippingFeeNotice = "物流費用已包含在本張發票的銷售額中，另列的物流明細不是另一張發票。"

// CorrectionStatusNotices 是 B06 進度卡（端點 11／12）每個 status 的逐字文案（#3031）。
//
// 🔴 **「前端不解析 statusNotice」與「statusNotice 沒有內容」是兩件事**，程式碼原本的
// 註解把它們混成一件（寫著「這一欄沒有被規定的內容」）：58A §B06 逐字寫著
// 「`statusNotice`→成功 toast 的 body（逐字）」——前端**顯示**它，只是不**解析**它去
// 決定行為（那是 rejectReasonCode 的職責）。恆為空字串的結果是那個 toast 沒有內文。
//
// 文案規約沿用 orderInvoiceStateNotices：說買家下一步該預期什麼，不承諾本服務管不到的事
// （例如退款）。
//
// ⚠️ REJECTED 只給一句中性的引導：真正的原因由 rejectReasonCode 決定文案，
// 在這裡再寫一次會變成第二份真相，而兩份必然漂移。
var CorrectionStatusNotices = map[string]string{
	codemeta.InvoiceCorrectionStatusRequested.Value:             "已收到您的修改需求，正在確認可否修改",
	codemeta.InvoiceCorrectionStatusRejected.Value:              "這次修改無法進行，請見下方原因",
	codemeta.InvoiceCorrectionStatusUpdatedBeforeIssue.Value:    "發票尚未開立，已直接更新為新的資料",
	codemeta.InvoiceCorrectionStatusVoidReissueInProgress.Value: "原發票作廢與重新開立處理中，完成後會寄送 Email",
	codemeta.InvoiceCorrectionStatusCompleted.Value:             "已重新開立發票，已寄送至您的電子信箱",
	// ⚠️ 這一句必須說出「原發票已作廢」：買家手上此刻一張憑證都沒有（§7 R09），
	// 沿用「處理中」的通用文案等於把最嚴重的狀態說成一般等待。
	codemeta.InvoiceCorrectionStatusReissueFailed.Value: "原發票已作廢、新發票尚未開立，平台正在處理",
}

// documentStatusToBuyerState 是 §6.1.3 對照表的逐列實作（不含 ALLOWANCE_APPLIED —
// 那一格的判準不在 status 欄上，見 projectOrderInvoiceState）。
var documentStatusToBuyerState = map[string]string{
	codemeta.InvoiceDocumentStatusPending.Value:              orderInvoiceStateProcessing,
	codemeta.InvoiceDocumentStatusIssuing.Value:              orderInvoiceStateProcessing,
	codemeta.InvoiceDocumentStatusVoiding.Value:              orderInvoiceStateProcessing,
	codemeta.InvoiceDocumentStatusAwaitingManual.Value:       orderInvoiceStateProcessing,
	codemeta.InvoiceDocumentStatusIssued.Value:               orderInvoiceStateIssued,
	codemeta.InvoiceDocumentStatusVoided.Value:               orderInvoiceStateVoided,
	codemeta.InvoiceDocumentStatusSuperseded.Value:           orderInvoiceStateVoided,
	codemeta.InvoiceDocumentStatusIssueFailed.Value:          orderInvoiceStateFailed,
	codemeta.InvoiceDocumentStatusNotApplicable.Value:        orderInvoiceStateSellerWillIssue,
	codemeta.InvoiceDocumentStatusResolvedExternally.Value:   orderInvoiceStateSelfIssued,
	codemeta.InvoiceDocumentStatusCancelledBeforeIssue.Value: orderInvoiceStateVoided,
}

// ---------------------------------------------------------------------------
// 端點 9／48／69：timeline 的 5 值投影
// ---------------------------------------------------------------------------

// timelineEventOf 把一列 invoice_status_history 折成 InvoiceDocumentTimelineEvent。
//
// 🔴 值域是 CodeMeta `InvoiceDocumentTimelineEvent`（5 值），**不是**
// `InvoiceEventType`（那是 MQ 事件型別、前端不可見、兩者交集為空），
// 也**不是** to_status 原值：直接把 to_status 塞進 event 會讓前端拿到
// `PENDING`／`ISSUING` 這種時間軸上不存在的值，而 codeLabel 查不到就顯示原始碼。
//
// 回 ok=false ⇒ 這一列不進時間軸（時間軸沒有「處理中」與「失敗」的值，
// 那些由 state ＋ stateNotice 承擔）。
func timelineEventOf(subjectType, toStatus string) (string, bool) {
	switch {
	case subjectType == subjectDocument && toStatus == codemeta.InvoiceDocumentStatusIssued.Value:
		return codemeta.InvoiceDocumentTimelineEventIssued.Value, true
	case subjectType == subjectDocument && toStatus == codemeta.InvoiceDocumentStatusVoided.Value:
		return codemeta.InvoiceDocumentTimelineEventVoided.Value, true
	// REISSUED 落在**舊**文件的時間軸上（新文件的鏈頭由 original_document_id 指回）。
	case subjectType == subjectDocument && toStatus == codemeta.InvoiceDocumentStatusSuperseded.Value:
		return codemeta.InvoiceDocumentTimelineEventReissued.Value, true
	case subjectType == subjectCorrectionRequest &&
		toStatus == codemeta.InvoiceCorrectionStatusUpdatedBeforeIssue.Value:
		return codemeta.InvoiceDocumentTimelineEventUpdated.Value, true
	default:
		return "", false
	}
}

// subjectCorrectionRequest 是 invoice_status_history.subject_type 的第三個值。
// （subjectDocument／subjectOperation 定義在 issuance.go，本檔只補這一個。）
const subjectCorrectionRequest = "CORRECTION_REQUEST"

// adjustmentTypeAllowance 與 adjustmentStatusCompleted 是「已折讓」的判準。
//
// 🔴 ALLOWANCE_APPLIED 的判準**不在 status 欄**（折讓不改原件 status，只累加
// allowance_total_amount）⇒ 只投影 subject_type='DOCUMENT' 的實作會讓它永遠不出現，
// 而且不報錯。
const (
	adjustmentTypeAllowance   = "ALLOWANCE"
	adjustmentStatusCompleted = "COMPLETED"
)
