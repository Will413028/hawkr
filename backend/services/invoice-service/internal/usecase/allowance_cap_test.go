//go:build !integration

package usecase

import (
	"context"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/invoice-service/internal/client/amego"
)

// TestAllowance_CompleteRollsBackWhenAllowanceCapExceeded 守 I4 那一句 CAS 的
// **rows=0 契約**：呼叫端必須整筆 rollback，不可當成功。
//
// 🔴 這條路徑在 2026-09-05 之前**沒有任何測試走得到**：呼叫端寫的是
// `if _, err := u.docs.AddAllowanceTotal(...)`（丟掉 bool），而測試替身
// `alwDocs.AddAllowanceTotal` 恆回 true。兩個「各自看起來沒問題」的決定加起來，
// 等於那份逐字寫在三個檔案裡的契約完全沒有人守。
//
// 失效時的症狀（本測試要防的）：adjustment 落 COMPLETED、折讓事件照發，而
// `allowance_total_amount` 停在舊值 ⇒ 下一筆退款的上限判斷用錯的基數，
// 零錯誤、零告警。
//
// 變異驗證（2026-09-05 實跑）：把 `if !ok { return ... }` 拿掉 ⇒ 本測試轉紅；
// 把 fake 改回 `return true, nil` ⇒ 本測試也轉紅（**兩個修法缺一都不成立**）。
func TestAllowance_CompleteRollsBackWhenAllowanceCapExceeded(t *testing.T) {
	h := newAllowanceHarness(t)
	ctx := context.Background()

	// 讓落帳必然超過上限：已折 900、這一筆 300 ⇒ 1200 > total 1000。
	h.docs.doc.AllowanceTotalAmount = 900
	h.adjs.adj.Status = codemeta.InvoiceAdjustmentStatusWaitingConfirmation.Value

	snapshot := h.ops.op
	snapshot.Status = codemeta.InvoiceOperationStatusWaitingConfirmation.Value
	h.ops.op.Status = codemeta.InvoiceOperationStatusProviderStatusRecheck.Value
	h.prov.allowanceQueryFn = func(no string) (*amego.AllowanceDetail, error) {
		return &amego.AllowanceDetail{AllowanceNumber: no}, nil
	}

	err := h.uc.ConfirmAllowance(ctx, snapshot)
	if err == nil {
		t.Fatal("落帳超過發票金額卻回 nil —— 交易會 commit，" +
			"allowance_total_amount 停在舊值而 adjustment 已 COMPLETED，零錯誤零告警")
	}
	if !strings.Contains(err.Error(), "allowance cap exceeded") {
		t.Errorf("錯誤訊息認不出是上限問題：%v", err)
	}
	if len(h.docs.allowanceAdd) != 0 {
		t.Errorf("超額的累加被記進去了：%v", h.docs.allowanceAdd)
	}
}
