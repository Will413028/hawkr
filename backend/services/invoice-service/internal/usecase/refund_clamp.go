package usecase

import "github.com/Will413028/hawkr/invoice-service/internal/domain"

// clampAllowanceToRemaining 把一筆退款折成「這張發票還能折的金額」。
//
// 回傳 (effective, requested)：`effective < requested` 就代表 clamp 發生了，
// 呼叫端必須同時遞增 M16 並補那則 Error log（§8.2.3 M16 明訂兩者缺一不可）。
//
// 🔴 **為什麼取兩個上限的最小值，而不是只看設計書寫的那一個**：
//   - 文件層 `total_amount - allowance_total_amount` 是法定不變式，由
//     `chk_invdoc_allowance_cap` 與 `AddInvoiceDocumentAllowanceTotal` 的 CAS 守；
//   - 明細層 Σ 剩餘可折讓基數是 `BuildAllowanceDraft` 的 I3 守衛（`chk_invline_allowanced_cap`）。
//
// 兩者在帳目一致時相等，但它們是**兩個獨立維護的累計欄**（`allowance_total_amount`
// 與逐行 `allowanced_amount`），任何一邊漂掉，只 clamp 另一邊就會在更下游炸開：
// 只看文件層 ⇒ 明細層的 I3 仍然回錯、退回原本那條「買家拿不到折讓單」的路；
// 只看明細層 ⇒ CAS 的 rows=0（那一句的呼叫端曾經丟棄它，見 allowance.go）。
// 取小的那個，兩道守衛都不會被觸發。
//
// requested <= 0 由呼叫端更早的路徑排除（`BuildAllowanceDraft` 對非正數回錯），
// 這裡不另行判斷——多一個分支只會多一條沒有測試走過的路。
func clampAllowanceToRemaining(doc domain.Document, lines []domain.DocumentLine, requested int64) (effective, req int64) {
	effective = requested

	if docRemaining := doc.TotalAmount - doc.AllowanceTotalAmount; docRemaining < effective {
		effective = docRemaining
	}

	_, base := domain.AllowanceBaseOf(allowanceSourceLines(lines))
	var lineRemaining int64
	for _, b := range base {
		lineRemaining += b
	}
	if lineRemaining < effective {
		effective = lineRemaining
	}

	if effective < 0 {
		// 反正規化欄漂到超過 total_amount 時 docRemaining 會是負數。折到 0 就好：
		// 負數往下傳會在 BuildAllowanceDraft 變成一個看不懂的錯誤訊息。
		effective = 0
	}
	return effective, requested
}
