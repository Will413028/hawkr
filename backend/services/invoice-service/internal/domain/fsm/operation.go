package fsm

import "github.com/Will413028/hawkr/go-pkg/codemeta"

// OperationTransition 是 M2（invoice_operations.status）的一列轉移。
//
// Code 是設計書 §7.3.2 的轉移代號（J01–J34），編號一經定版即凍結、不因日後增刪而重編。
// (From, To) 在 M2 上不唯一（例：J33／J34 都是 SELLER_ACTION_REQUIRED → READY），
// 所以識別一律用 Code。
type OperationTransition struct {
	Code    string
	From    []string
	To      string
	Trigger string
}

var (
	opCreated               = codemeta.InvoiceOperationStatusCreated.Value
	opAwaitingTrigger       = codemeta.InvoiceOperationStatusAwaitingTrigger.Value
	opReady                 = codemeta.InvoiceOperationStatusReady.Value
	opHeld                  = codemeta.InvoiceOperationStatusHeld.Value
	opSending               = codemeta.InvoiceOperationStatusSending.Value
	opRetryScheduled        = codemeta.InvoiceOperationStatusRetryScheduled.Value
	opWaitingConfirmation   = codemeta.InvoiceOperationStatusWaitingConfirmation.Value
	opProviderStatusRecheck = codemeta.InvoiceOperationStatusProviderStatusRecheck.Value
	opSucceeded             = codemeta.InvoiceOperationStatusSucceeded.Value
	opFailedPermanent       = codemeta.InvoiceOperationStatusFailedPermanent.Value
	opFailedRetryExhausted  = codemeta.InvoiceOperationStatusFailedRetryExhausted.Value
	opFailedUnknown         = codemeta.InvoiceOperationStatusFailedUnknown.Value
	opSellerActionRequired  = codemeta.InvoiceOperationStatusSellerActionRequired.Value
	opCancelRequested       = codemeta.InvoiceOperationStatusCancelRequested.Value
	opCancelBlockedUnknown  = codemeta.InvoiceOperationStatusCancelBlockedUnknown.Value
	opCancelledByAdmin      = codemeta.InvoiceOperationStatusCancelledByAdmin.Value
	opManualProcessAllowed  = codemeta.InvoiceOperationStatusManualProcessAllowed.Value
	opResolvedExternally    = codemeta.InvoiceOperationStatusResolvedExternally.Value
	opNotApplicable         = codemeta.InvoiceOperationStatusNotApplicable.Value
	opCancelledBeforeIssue  = codemeta.InvoiceOperationStatusCancelledBeforeIssue.Value
)

// OperationTransitions 是 §7.3.2 的 J01–J34。
var OperationTransitions = []OperationTransition{
	{Code: "J01", From: nil, To: opCreated, Trigger: "建立 operation（operation_key 未存在）"},
	{Code: "J02", From: []string{opCreated}, To: opReady, Trigger: "同一事務內判定 issue_not_before <= now"},
	{Code: "J03", From: []string{opCreated}, To: opAwaitingTrigger, Trigger: "同上，issue_not_before > now"},
	{Code: "J04", From: []string{opCreated}, To: opNotApplicable, Trigger: "決策樹落在四個 reason 之一"},
	{Code: "J05", From: []string{opAwaitingTrigger}, To: opReady, Trigger: "issue_not_before 到期（INV.op RunOnce 前置 promote）"},
	// ⚠️ CREATED 是「作廢重開全鏈路」那次（6219d7a50）刻意加進 SQL 的：重開票的
	// REISSUE operation 會停在 CREATED，那時全額退款一樣要取消得掉。
	// 本表與該支 SQL 的註解**當時都沒跟上** —— 這是 2026-09-04 由
	// TestOperationFSMMatchesSQL 對帳出來的（#3089 §1.5 T3：一個沒有人讀的 SSOT
	// 必然漂移，而漂移了也不會有任何東西轉紅）。
	{Code: "J06", From: []string{opCreated, opAwaitingTrigger, opReady, opRetryScheduled, opHeld}, To: opCancelledBeforeIssue, Trigger: "累計退款 = 實付總額"},
	// J07 的候選述詞必須含 SENDING，否則崩潰的租約永遠回不來。
	{Code: "J07", From: []string{opReady, opRetryScheduled, opSending}, To: opSending, Trigger: "Claim CAS rows=1（next_attempt_at <= tickNow）"},
	{Code: "J08", From: []string{opReady, opRetryScheduled}, To: opHeld, Trigger: "買家送出付款後修改（held_until = now + 60s）"},
	{Code: "J09", From: []string{opHeld}, To: opReady, Trigger: "快照更新完成 或 held_until <= now"},
	{Code: "J10", From: []string{opSending}, To: opSucceeded, Trigger: "code = 0（或 3050122／4050132 視同成功）"},
	// ⚠️ RETRY_SCHEDULED 同上是對帳補回來的：MarkInvoiceOperationFailed 的
	// allowed-from 是 IN ('SENDING','RETRY_SCHEDULED')，因為 pkg/outbox 在
	// 重試預算用完時呼叫它，而那個時點的 status 正是 RETRY_SCHEDULED。
	{Code: "J11", From: []string{opSending, opRetryScheduled}, To: opFailedPermanent, Trigger: "錯誤碼分類 = PERMANENT，或重試預算用完（pkg/outbox）"},
	{Code: "J12", From: []string{opSending}, To: opRetryScheduled, Trigger: "錯誤碼分類 = RETRYABLE 且 attempt_count < MaxAttempts"},
	{Code: "J13", From: []string{opSending}, To: opWaitingConfirmation, Trigger: "逾時／連線中斷／JSON 解析失敗／分類 = UNKNOWN"},
	{Code: "J14", From: []string{opRetryScheduled}, To: opFailedRetryExhausted, Trigger: "attempt_count 達 MaxAttempts"},
	{Code: "J15", From: []string{opWaitingConfirmation}, To: opProviderStatusRecheck, Trigger: "INV.rc Claim CAS rows=1"},
	// J16 的候選述詞必須含 PROVIDER_STATUS_RECHECK，理由同 J07。
	{Code: "J16", From: []string{opProviderStatusRecheck}, To: opProviderStatusRecheck, Trigger: "租約到期後被重新 Claim（持有者崩潰）"},
	{Code: "J17", From: []string{opProviderStatusRecheck}, To: opFailedUnknown, Trigger: "重查仍不確定，now − unknown_since > 24h，且無 ADM 取消在途"},
	{Code: "J18", From: []string{opFailedPermanent}, To: opReady, Trigger: "ADM 修正資料後重試（必須帶新的 snapshot_version）"},
	{Code: "J19", From: []string{opFailedRetryExhausted}, To: opReady, Trigger: "ADM 重試（attempt_count = 0）"},
	{Code: "J20", From: []string{opFailedUnknown}, To: opProviderStatusRecheck, Trigger: "週期重查（唯一出邊，禁止直接重試）"},
	{Code: "J21", From: []string{opReady, opRetryScheduled, opFailedPermanent, opFailedRetryExhausted, opSellerActionRequired}, To: opCancelRequested, Trigger: "ADM 取消（必填 reasonCode／reasonDetail）"},
	{Code: "J22", From: []string{opCancelRequested, opFailedUnknown, opCancelBlockedUnknown}, To: opProviderStatusRecheck, Trigger: "強制最終查詢"},
	{Code: "J23", From: []string{opProviderStatusRecheck}, To: opSucceeded, Trigger: "invoice_query code=0 或 invoice_list 比對到"},
	{Code: "J24", From: []string{opProviderStatusRecheck}, To: opCancelledByAdmin, Trigger: "code=71 且有 ADM 取消在途"},
	{Code: "J25", From: []string{opProviderStatusRecheck}, To: opRetryScheduled, Trigger: "code=71 且無 ADM 取消在途（發票日期在 180 天內）"},
	{Code: "J26", From: []string{opProviderStatusRecheck}, To: opCancelBlockedUnknown, Trigger: "code=51 或再次逾時，且有 ADM 取消在途"},
	{Code: "J27", From: []string{opCancelBlockedUnknown}, To: opProviderStatusRecheck, Trigger: "退避週期到期"},
	{Code: "J28", From: []string{opCancelBlockedUnknown}, To: opCancelledByAdmin, Trigger: "ADM 雙人確認 ＋ 證據（兩個 actor 不同）"},
	{Code: "J29", From: []string{opCancelledByAdmin}, To: opManualProcessAllowed, Trigger: "第二人覆核解鎖（actor != 取消者）"},
	{Code: "J30", From: []string{opManualProcessAllowed}, To: opResolvedExternally, Trigger: "賣家回報號碼（provider_invoice_no 必須留空）"},
	{Code: "J31", From: []string{opProviderStatusRecheck}, To: opWaitingConfirmation, Trigger: "重查仍不確定，now − unknown_since <= 24h，且無 ADM 取消在途"},
	// J32 與 J11 分開列是刻意的：兩者分類同為 PERMANENT，但一個要通知賣家去財政部、
	// 一個要通知 ADM 改資料。
	{Code: "J32", From: []string{opSending}, To: opSellerActionRequired, Trigger: "PERMANENT 且屬「賣家法定前置未完成」子類（3040111 字軌不足）"},
	// J33 是 SELLER_ACTION_REQUIRED 不是永久旗標的唯一理由——刪掉它，卡住的票就再也回不來，
	// 而且無告警。集合式 UPDATE、無外部副作用、不需 claim。
	{Code: "J33", From: []string{opSellerActionRequired}, To: opReady, Trigger: "INV.trk 查到該 issuer_id 字軌餘量恢復"},
	{Code: "J34", From: []string{opSellerActionRequired}, To: opReady, Trigger: "ADM 手動重試（不要求新的 snapshot_version）"},
}

// OperationTerminalStates 是 M2 的四個終態。
// CANCEL_BLOCKED_UNKNOWN 與 MANUAL_PROCESS_ALLOWED 各有出邊（J27／J28、J30），不是終態。
var OperationTerminalStates = []string{
	opSucceeded,
	opNotApplicable,
	opCancelledBeforeIssue,
	opResolvedExternally,
}

// OperationTransitionByCode 依 J 碼取一條轉移。
func OperationTransitionByCode(code string) (OperationTransition, bool) {
	for _, t := range OperationTransitions {
		if t.Code == code {
			return t, true
		}
	}
	return OperationTransition{}, false
}

// OperationAllowedFrom 回傳「要轉進 to」時 CAS 述詞該正面列舉的來源狀態集合（跨所有 J 碼取聯集）。
//
// ⚠️ 針對單一條轉移做 CAS 時要用 OperationTransitionByCode(code).From，不是本函式：
// 本函式是聯集，用它會讓一條轉移接受它本來不該接受的起點。本函式的用途是靜態對帳
// （例：ClaimInvoiceRecheck 的 status IN (...) 必須逐字等於 OperationAllowedFrom(PROVIDER_STATUS_RECHECK)）。
func OperationAllowedFrom(to string) ([]string, bool) {
	seen := map[string]struct{}{}
	out := []string{}
	found := false
	for _, t := range OperationTransitions {
		if t.To != to {
			continue
		}
		found = true
		for _, f := range t.From {
			if _, dup := seen[f]; dup {
				continue
			}
			seen[f] = struct{}{}
			out = append(out, f)
		}
	}
	return out, found
}

// CanTransitionOperation 回報 from → to 是不是 M2 上的一條合法邊。
func CanTransitionOperation(from, to string) bool {
	for _, t := range OperationTransitions {
		if t.To != to {
			continue
		}
		for _, f := range t.From {
			if f == from {
				return true
			}
		}
	}
	return false
}

// IsOperationTerminal 回報該狀態是否為 M2 終態。
func IsOperationTerminal(status string) bool {
	for _, s := range OperationTerminalStates {
		if s == status {
			return true
		}
	}
	return false
}
