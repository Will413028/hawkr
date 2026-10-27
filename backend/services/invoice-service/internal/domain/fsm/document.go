// Package fsm 承載 invoice-service 的狀態轉移表。
//
// 每一條轉移都是 CAS，述詞形如 WHERE id=? AND status IN (<allowed_from>) AND version=?。
// ⇒ allowed-from 漏一個起點 ＝ rows=0 靜默失敗：不拋例外、不進 DLQ、不告警，該筆永遠留在
// 原狀態。本 package 的 From 欄就是 allowed-from 的 SSOT，由 §7.3 的轉換表逐列抄來。
//
// 一律正面列舉狀態值，禁止 NOT IN／<> 這類否定述詞（I-28）：值域日後長胖時，否定述詞會把
// 新狀態靜默納入候選集合。
package fsm

import "github.com/Will413028/hawkr/go-pkg/codemeta"

// DocumentTransition 是 M1（invoice_documents.status）的一列轉移。
//
// M1 在設計書中不編號 ⇒ 本表以 (From, To) 為自然鍵，該鍵在 M1 上唯一（M2 不唯一，故用 J 碼）。
// From 為空表示由 [*] 建立。
type DocumentTransition struct {
	From    []string
	To      string
	Trigger string
}

// 為可讀性取的區域別名；值全部來自 CodeMeta 生成常數，禁止字串字面值。
var (
	docPending              = codemeta.InvoiceDocumentStatusPending.Value
	docIssuing              = codemeta.InvoiceDocumentStatusIssuing.Value
	docIssued               = codemeta.InvoiceDocumentStatusIssued.Value
	docIssueFailed          = codemeta.InvoiceDocumentStatusIssueFailed.Value
	docAwaitingManual       = codemeta.InvoiceDocumentStatusAwaitingManual.Value
	docVoiding              = codemeta.InvoiceDocumentStatusVoiding.Value
	docVoided               = codemeta.InvoiceDocumentStatusVoided.Value
	docSuperseded           = codemeta.InvoiceDocumentStatusSuperseded.Value
	docCancelledBeforeIssue = codemeta.InvoiceDocumentStatusCancelledBeforeIssue.Value
	docNotApplicable        = codemeta.InvoiceDocumentStatusNotApplicable.Value
	docResolvedExternally   = codemeta.InvoiceDocumentStatusResolvedExternally.Value
)

// DocumentTransitions 是 §7.3.1 的 17 列轉移，順序與設計書的表格逐列對應。
var DocumentTransitions = []DocumentTransition{
	{From: nil, To: docPending, Trigger: "order.paid 消費 / ADM 手動建單（AMEGO_AUTO 或 PLATFORM_RECEIPT）"},
	{From: nil, To: docNotApplicable, Trigger: "決策樹判定不由平台開立，或 ZERO_AMOUNT"},
	{From: []string{docPending}, To: docIssuing, Trigger: "ISSUE operation READY→SENDING"},
	{From: []string{docPending}, To: docCancelledBeforeIssue, Trigger: "累計退款 = 實付總額"},
	{From: []string{docIssuing}, To: docIssued, Trigger: "f0401 code=0 或 invoice_query 回填"},
	// ⚠️ SELLER_ACTION_REQUIRED 不叫 FAILED_*，「任一 FAILED_*」這種寫法會漏掉它——
	// 漏掉的後果是 document 永遠停在 ISSUING，賣家看到「開立中」而系統已停手在等他配字軌。
	{From: []string{docIssuing}, To: docIssueFailed, Trigger: "operation 進入 FAILED_PERMANENT / FAILED_RETRY_EXHAUSTED / FAILED_UNKNOWN / SELLER_ACTION_REQUIRED"},
	{From: []string{docIssuing}, To: docPending, Trigger: "operation 退回 RETRY_SCHEDULED"},
	{From: []string{docIssueFailed}, To: docIssuing, Trigger: "ADM 重試 / worker 重試（operation 已回 READY）"},
	{From: []string{docPending, docIssueFailed}, To: docAwaitingManual, Trigger: "fail-closed 偵測命中，或 ADM 判定需人工（必填 manual_reason_code）"},
	{From: []string{docAwaitingManual}, To: docPending, Trigger: "ADM 給出判定（funding_decision 等），回自動線"},
	{From: []string{docAwaitingManual}, To: docResolvedExternally, Trigger: "賣家回報自行開立號碼"},
	{From: []string{docAwaitingManual}, To: docCancelledBeforeIssue, Trigger: "ADM 雙人確認本單不需開立"},
	{From: []string{docIssueFailed}, To: docResolvedExternally, Trigger: "賣家回報自行開立號碼（operation 在 MANUAL_PROCESS_ALLOWED）"},
	{From: []string{docIssued}, To: docVoiding, Trigger: "VOID operation READY→SENDING"},
	{From: []string{docVoiding}, To: docVoided, Trigger: "invoice_query 確認 invoice_type = C0501"},
	{From: []string{docVoiding}, To: docIssued, Trigger: "f0501 回 3050121/3050123/3050126/3050141"},
	{From: []string{docVoided, docIssued}, To: docSuperseded, Trigger: "重開文件已 ISSUED"},
}

// DocumentTerminalStates 是 M1 的終態。
//
// ISSUED 刻意不在其中：它是靜止態而非封閉終態——隨時可以再走 VOIDING、SUPERSEDED，或被折讓
// （折讓不改原件 status，只改 allowance_total_amount）。任何「document 已到終態就可以停止追蹤
// ／可以清理」的實作假設都是錯的：法定保存 5 年，且第 5 年仍可能有折讓。
var DocumentTerminalStates = []string{
	docCancelledBeforeIssue,
	docNotApplicable,
	docResolvedExternally,
	docSuperseded,
	docVoided,
}

// DocumentAllowedFrom 回傳「要轉進 to 這個狀態」時 CAS 述詞該正面列舉的來源狀態集合。
// 第二個回傳值為 false 表示沒有任何轉移以 to 為目標。
func DocumentAllowedFrom(to string) ([]string, bool) {
	seen := map[string]struct{}{}
	out := []string{}
	found := false
	for _, t := range DocumentTransitions {
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

// CanTransitionDocument 回報 from → to 是不是 M1 上的一條合法邊。
func CanTransitionDocument(from, to string) bool {
	for _, t := range DocumentTransitions {
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

// IsDocumentTerminal 回報該狀態是否為 M1 終態。
func IsDocumentTerminal(status string) bool {
	for _, s := range DocumentTerminalStates {
		if s == status {
			return true
		}
	}
	return false
}
