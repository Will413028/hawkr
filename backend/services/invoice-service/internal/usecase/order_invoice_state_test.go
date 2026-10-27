package usecase

import (
	"strings"
	"testing"
)

// TestOrderInvoiceStateNoticesCoverEveryState 守住「新增了一個 state 卻忘了文案」。
//
// 🔴 查表落空回的是**空字串**，不是錯誤：買家會看到一個沒有說明文字的橫幅，
// 而後端一切正常、log 乾淨。這正是 §3.10 的形狀（看不到就放行），所以要有東西
// 替它出聲。#2904 新增第 9 個值時，這一格是最容易漏的。
func TestOrderInvoiceStateNoticesCoverEveryState(t *testing.T) {
	// 值域寫在這裡而不是從 map 反推：從 orderInvoiceStateNotices 的鍵反推的話，
	// 「新增常數但沒加文案」正好是它看不見的那一格——那會讓這支測試恆綠。
	states := []string{
		orderInvoiceStateAwaitingPayment,
		orderInvoiceStateProcessing,
		orderInvoiceStateIssued,
		orderInvoiceStateVoided,
		orderInvoiceStateAllowanceApplied,
		orderInvoiceStateFailed,
		orderInvoiceStateSellerWillIssue,
		orderInvoiceStateSelfIssued,
		orderInvoiceStateOrderCancelled,
	}
	for _, st := range states {
		if orderInvoiceStateNotices[st] == "" {
			t.Errorf("state %q 沒有對應的 stateNotice —— 買家會看到一個沒有說明的橫幅", st)
		}
	}
	if len(orderInvoiceStateNotices) != len(states) {
		t.Errorf("notices 有 %d 條、本測試列了 %d 個 state：\n"+
			"多出來的那條永遠不會被讀到；少列的那個代表這份清單自己過期了",
			len(orderInvoiceStateNotices), len(states))
	}
}

// TestOrderInvoiceStatesAreNotPrefixesOfDocumentStatuses 釘住 §6.1.3「兩套字串不可
// 互相代入」的**可機械檢查的那一半**。
//
// ⚠️ 兩套值域刻意**不是**互斥的：`ISSUED`／`VOIDED` 在兩邊都存在，而那是設計上的
// 恆等映射（documentStatusToBuyerState 有 ISSUED→ISSUED、VOIDED→VOIDED）。所以
// 「值不得重複」是錯的斷言——本測試曾經那樣寫，實跑當場被這兩個值推翻。
//
// 真正抓得到的是**前綴**：一個 buyer state 若以某個 document status 為前綴而語意不同，
// 任何 HasPrefix／startsWith／`switch` 的 fallthrough 都會把兩者混在一起。#2904 因此
// 沒有把新值命名為 `NOT_APPLICABLE_CANCELLED`（`NOT_APPLICABLE` 是 document status，
// 映到 SELLER_WILL_ISSUE），改用 `ORDER_CANCELLED`。
func TestOrderInvoiceStatesAreNotPrefixesOfDocumentStatuses(t *testing.T) {
	if len(orderInvoiceStateNotices) == 0 || len(documentStatusToBuyerState) == 0 {
		t.Fatal("兩份值域之一是空的 —— 這支測試會恆綠")
	}
	checked := 0
	for state := range orderInvoiceStateNotices {
		for docStatus := range documentStatusToBuyerState {
			checked++
			if state == docStatus {
				continue // 恆等映射，見上方註解
			}
			if strings.HasPrefix(state, docStatus) || strings.HasPrefix(docStatus, state) {
				t.Errorf("buyer state %q 與 InvoiceDocumentStatus %q 互為前綴 —— "+
					"任何 HasPrefix 判斷都會把兩者混在一起", state, docStatus)
			}
		}
	}
	if checked == 0 {
		t.Fatal("一組都沒比到")
	}
}

// TestOrderCancelledStateIsPrefixFree 是上一支的鑑別力驗證：餵一個「長得像正例的
// 反例」，比對邏輯必須抓得到。
func TestOrderCancelledStateIsPrefixFree(t *testing.T) {
	const rejected = "NOT_APPLICABLE_CANCELLED" // #2904 曾考慮、被否決的命名
	hit := false
	for docStatus := range documentStatusToBuyerState {
		if rejected != docStatus && strings.HasPrefix(rejected, docStatus) {
			hit = true
		}
	}
	if !hit {
		t.Fatal("反例沒有被抓到：NOT_APPLICABLE_CANCELLED 以 document status " +
			"NOT_APPLICABLE 為前綴，前綴檢查竟然看不到它")
	}
	// 而實際採用的那個名字必須是乾淨的。
	for docStatus := range documentStatusToBuyerState {
		if orderInvoiceStateOrderCancelled != docStatus &&
			(strings.HasPrefix(orderInvoiceStateOrderCancelled, docStatus) ||
				strings.HasPrefix(docStatus, orderInvoiceStateOrderCancelled)) {
			t.Errorf("採用的 %q 與 %q 互為前綴", orderInvoiceStateOrderCancelled, docStatus)
		}
	}
}
