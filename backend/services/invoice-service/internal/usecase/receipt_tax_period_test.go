package usecase

import (
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// 🔴 收據沒有稅期別（#3026 收尾）。
//
// 期別碼的語意是「這張發票落在哪一個財政部申報期」。收據不申報、不上傳、
// 沒有作廢期限（設計書 §7.2：收據只重開／註記）⇒ 它沒有期別可言。
//
// 這不是潔癖：它唯一一次被讀到就產生了錯答案 —— `canRequestCorrection` 拿
// `WithinVoidWindow(tax_period_code)` 判定，dev 實測收據帶著 `11505` ⇒ 判定成立
// ⇒ 買家看到一顆按下去會在 AMEGO 那段失敗的「申請更正」按鈕。
// 守衛擋的是 instance，這一格守的是 class。
func TestTaxPeriodFor_EmptyForReceipt(t *testing.T) {
	d := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	if got := taxPeriodFor(codemeta.InvoiceDocumentKindReceipt.Value, d); got != "" {
		t.Errorf("收據拿到稅期別 %q —— 它不申報也沒有作廢期限，不該有期別", got)
	}
}

// 🔴 陰性對照：**發票線不得被一起清掉**。這是這次改動最容易造成的附帶損害
// ——期別碼是發票作廢窗口判定的唯一輸入，清掉它會讓所有發票都失去更正入口
// （`canRequestCorrection` 的 `doc.TaxPeriodCode == nil` 分支直接回 false）。
func TestTaxPeriodFor_UnchangedForInvoices(t *testing.T) {
	d := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	want := domain.TaxPeriodCode(d)
	if want == "" {
		t.Fatal("測試前提壞了：domain.TaxPeriodCode 對正常日期回空字串")
	}
	for _, kind := range []string{
		codemeta.InvoiceDocumentKindB2c.Value,
		codemeta.InvoiceDocumentKindB2b.Value,
	} {
		if got := taxPeriodFor(kind, d); got != want {
			t.Errorf("kind=%s 的期別碼 = %q, want %q", kind, got, want)
		}
	}
}
