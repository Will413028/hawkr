//go:build !integration

package usecase

import (
	"context"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// 本檔守 §6.3.2.3 的 clamp：
//
//	「折讓累計一律 min(cumulative, invoiced_total)，並遞增 metric
//	  invoice_refund_exceeds_invoiced_total 觸發告警，**但不得因此擋住折讓**」
//
// 🔴 這一段在 2026-09-05 之前**完全不存在**：超額直接回 error ⇒ inbox 標 FAILED
// ⇒ 訊息進 DLQ ⇒ 買家一張折讓單都拿不到，而 ADM 對 inbox 零可見性。
// 三支測試各守一格，缺任何一格那個回歸都不會轉紅：
//
//	① 超額 ⇒ **有折讓單**、金額被折到餘額（不是回 error）
//	② 餘額為 0 ⇒ SKIPPED ＋ disposition（不是 error，也不是建一張 0 元折讓單）
//	③ 未超額 ⇒ 一個字都不動（clamp 不得誤傷正常路徑）

// allowancedDoc 回一張「已折讓 700、還剩 300」的發票。
func allowancedDoc(alreadyAllowanced int64) domain.Document {
	d := voidableDoc()
	d.AllowanceTotalAmount = alreadyAllowanced
	return d
}

// remainingLines 讓明細層的剩餘基數與文件層一致（AllowanceableBase 是**剩餘**額）。
func remainingLines(total int64) []domain.DocumentLine {
	return []domain.DocumentLine{
		{ID: 8001, LineSeq: 1, LineType: "PRODUCT", Description: "有機高麗菜",
			Quantity: 2, UnitPrice: 300, NetAmount: 600, AllowanceableBase: total, TaxCategory: 1},
	}
}

// TestApply_OverCapRefundIsClampedNotRejected 是 ① —— 本輪修的那個回歸。
//
// 變異驗證（2026-09-05 實跑）：把 createAllowance 裡的 clamp 段拿掉，
// 本測試轉紅（Apply 回 error、零 adjustment）；還原後轉綠。
func TestApply_OverCapRefundIsClampedNotRejected(t *testing.T) {
	h := newRefundHarness(t)
	h.docs.lines = remainingLines(300)
	doc := allowancedDoc(700) // total 1000、已折 700 ⇒ 只剩 300

	in := refundInput(800) // 上游沒扣既退，送來 800
	in.CumulativeRefundAmount = 1500

	skip, err := h.uc.Apply(context.Background(), in, doc)
	if err != nil {
		t.Fatalf("超額退款被擋下了：%v\n"+
			"§6.3.2.3 逐字：「不得因此擋住折讓」。回 error ⇒ inbox FAILED ⇒ 訊息進 DLQ "+
			"⇒ 買家一張折讓單都拿不到，而 ADM 對 inbox 零可見性。", err)
	}
	if skip != nil {
		t.Fatalf("回了 SKIP（%s）—— 還有 300 元餘額可以折，不該跳過", skip.Disposition)
	}
	if len(h.adjs.inserted) != 1 {
		t.Fatalf("建了 %d 筆 adjustment，want 1", len(h.adjs.inserted))
	}
	got := h.adjs.inserted[0]
	if got.GrossAmount == nil || *got.GrossAmount != 300 {
		t.Errorf("gross = %v，want 300（＝clamp 到剩餘額）", got.GrossAmount)
	}
	// 這一格擋「clamp 成功但仍用原始金額建列」：那會在落帳時撞
	// chk_invdoc_allowance_cap（累計 700+800 > 1000），而那句 CAS 的 rows=0
	// 曾經被呼叫端丟棄 ⇒ 靜默不落帳。
	if got.NetAmountExTax != nil && got.TaxAmount != nil &&
		*got.NetAmountExTax+*got.TaxAmount != 300 {
		t.Errorf("net+tax = %d，want 300", *got.NetAmountExTax+*got.TaxAmount)
	}
	if len(h.ops.created) != 1 {
		t.Errorf("建了 %d 筆 operation，want 1 —— 沒有它折讓永遠不會被送出", len(h.ops.created))
	}
}

// TestApply_CapReachedSkipsWithDisposition 是 ②。
//
// 餘額為 0 時回 error 會讓這則事件**無限重試**（金額永遠不會變小），
// 而那是「刻意不做」不是「現在還做不了」——兩者在 inbox 上是不同的狀態。
func TestApply_CapReachedSkipsWithDisposition(t *testing.T) {
	h := newRefundHarness(t)
	h.docs.lines = remainingLines(0)
	doc := allowancedDoc(1000) // 已經折滿

	in := refundInput(200)
	in.CumulativeRefundAmount = 1200

	skip, err := h.uc.Apply(context.Background(), in, doc)
	if err != nil {
		t.Fatalf("餘額為 0 時回了 error：%v —— 那會讓這則事件無限重試", err)
	}
	if skip == nil {
		t.Fatal("回了 nil skip —— 呼叫端會標 PROCESSED，事後查「為什麼沒有折讓單」只能靠猜")
	}
	if skip.Disposition != codemeta.InvoiceInboxDispositionAllowanceCapReached.Value {
		t.Errorf("disposition = %q，want %q",
			skip.Disposition, codemeta.InvoiceInboxDispositionAllowanceCapReached.Value)
	}
	if !strings.Contains(skip.Reason, "退款本身不受影響") {
		t.Errorf("reason 沒有說明退款不受影響：%q", skip.Reason)
	}
	if len(h.adjs.inserted) != 0 {
		t.Errorf("建了 %d 筆 adjustment —— 0 元折讓單送出必被 AMEGO 拒絕", len(h.adjs.inserted))
	}
}

// TestApply_WithinCapIsUntouched 是 ③ 的陰性對照。
//
// 沒有它的話，一個「永遠回 0」的 clamp 也會讓 ① 與 ② 轉綠，而正常退款全部壞掉。
func TestApply_WithinCapIsUntouched(t *testing.T) {
	h := newRefundHarness(t)
	skip, err := h.uc.Apply(context.Background(), refundInput(300), issuedDoc())
	if err != nil || skip != nil {
		t.Fatalf("正常退款被 clamp 影響了：err=%v skip=%v", err, skip)
	}
	if len(h.adjs.inserted) != 1 || h.adjs.inserted[0].GrossAmount == nil ||
		*h.adjs.inserted[0].GrossAmount != 300 {
		t.Fatalf("正常退款的金額被動過：%+v", h.adjs.inserted)
	}
}

// TestClampTakesTheSmallerOfTwoCeilings 釘住「取兩個上限的最小值」。
//
// 文件層與明細層是**兩個獨立維護的累計欄**；只 clamp 其中一邊，另一邊的守衛
// （I3 或 chk_invdoc_allowance_cap）仍會擋下來，就退回原本那條買家拿不到折讓單的路。
func TestClampTakesTheSmallerOfTwoCeilings(t *testing.T) {
	cases := []struct {
		name          string
		allowanced    int64
		lineBase      int64
		requested     int64
		wantEffective int64
	}{
		{"文件層較緊", 900, 500, 400, 100},
		{"明細層較緊", 500, 120, 400, 120},
		{"都夠 ⇒ 不動", 0, 1000, 400, 400},
		{"文件層已滿", 1000, 300, 400, 0},
		{"反正規化欄漂到負數 ⇒ 折到 0", 1200, 300, 400, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, req := clampAllowanceToRemaining(allowancedDoc(c.allowanced), remainingLines(c.lineBase), c.requested)
			if got != c.wantEffective {
				t.Errorf("effective = %d, want %d", got, c.wantEffective)
			}
			if req != c.requested {
				t.Errorf("requested 回傳被改動：%d, want %d", req, c.requested)
			}
		})
	}
}
