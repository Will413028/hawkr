package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// J24（對帳確認未開立 ＋ ADM 取消在途 ⇒ CANCELLED_BY_ADMIN）必須把兩個取消欄位
// **真的寫進 invoice_operations**。
//
// 🔴 在此之前它們在 decideRecheck 算出來、然後被丟掉：`domain.MarkRecheckResult`
// 沒有這兩個欄位 ⇒ applyDecision 組 struct 時整個消失。三道防線同時失效：
//
//	① 編譯：多算一個值不用會過（它掛在 RecheckDecision 上）
//	② 測試：`recheck_decide_test.go` 只測純函式 decide，寫回那段沒有覆蓋
//	③ DB：chk_invop_admin_cancel_needs_confirm 是三值邏輯的 no-op
//	   （兩欄皆 NULL ⇒ `FALSE OR FALSE OR NULL` = NULL ⇒ PostgreSQL 放行）
//
// dev 實證（op 7500463275386077184，2026-09-05）：J24 落 CANCELLED_BY_ADMIN 而
// cancel_confirmed_by／cancel_evidence_kind 兩欄皆 NULL ⇒ §6.4 的「人工出口四樣紀律」
// 少了證據那一樣，且沒有任何東西會出聲。

func TestRecheck_J24_PassesCancelEvidenceThrough(t *testing.T) {
	uc, ops, _ := rcFixture(t, true)

	// 從**決策函式**取 decision，不是自己手捏一個：手捏的話這支測試驗的是我對
	// J24 的印象，不是 decideRecheck 真的算出什麼（§0.3 手寫 fixture 的同一個病）。
	op := rcOp(codemeta.InvoiceOperationStatusProviderStatusRecheck.Value)
	requested := adminNow.Add(-time.Hour)
	op.CancelRequestedAt = &requested
	d := decideRecheck(op, RecheckProbe{NotIssued: true, WithinProviderRetention: true}, adminNow)

	if d.NextStatus != codemeta.InvoiceOperationStatusCancelledByAdmin.Value {
		t.Fatalf("前提不成立：這組輸入應走 J24，實得 %q", d.NextStatus)
	}
	if d.CancelEvidenceKind == nil || d.CancelConfirmedAt == nil {
		t.Fatalf("J24 的兩個取消欄位必須同進同出，實得 kind=%v at=%v",
			d.CancelEvidenceKind, d.CancelConfirmedAt)
	}

	if err := uc.applyDecision(context.Background(), op, d); err != nil {
		t.Fatalf("applyDecision 失敗：%v", err)
	}
	if len(ops.resolved) != 1 {
		t.Fatalf("CAS 應被呼叫一次，實得 %d", len(ops.resolved))
	}
	got := ops.resolved[0]
	// 期望值硬寫字面值，不從 d 取——從被測物取等於沒有斷言。
	if got.CancelEvidenceKind == nil || *got.CancelEvidenceKind != "PROVIDER_QUERY_71" {
		t.Fatalf("cancel_evidence_kind 應為 PROVIDER_QUERY_71 且真的傳進 repository，實得 %v",
			got.CancelEvidenceKind)
	}
	if got.CancelConfirmedAt == nil || !got.CancelConfirmedAt.Equal(adminNow) {
		t.Fatalf("cancel_confirmed_at 應為決策當下的時間，實得 %v —— 少了它，"+
			"「ADM 取消在途」判別式永遠成立，這筆會一直看起來像還沒處理完",
			got.CancelConfirmedAt)
	}
}

// J23／J25 一律不帶那兩欄（傳 nil）。
//
// 🔴 這不只是「用不到」：SQL 那側是 COALESCE(narg, 原值) ⇒ nil 代表**不動它**。
// 若這裡誤傳了值，J23（取消請求被回絕）會在資料上留下一份「取消成立」的證據，
// 而那次取消其實沒有生效。
func TestRecheck_NonJ24_LeavesCancelEvidenceUntouched(t *testing.T) {
	cases := []struct {
		name  string
		probe RecheckProbe
		want  string
	}{
		{"J23 查到已開立", RecheckProbe{Found: true}, codemeta.InvoiceOperationStatusSucceeded.Value},
		{"J25 確定沒開且無人取消", RecheckProbe{NotIssued: true, WithinProviderRetention: true},
			codemeta.InvoiceOperationStatusRetryScheduled.Value},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			uc, ops, _ := rcFixture(t, true)
			op := rcOp(codemeta.InvoiceOperationStatusProviderStatusRecheck.Value)
			d := decideRecheck(op, c.probe, adminNow)
			if d.NextStatus != c.want {
				t.Fatalf("前提不成立：應走 %s，實得 %q", c.want, d.NextStatus)
			}
			if err := uc.applyDecision(context.Background(), op, d); err != nil {
				t.Fatalf("applyDecision 失敗：%v", err)
			}
			got := ops.resolved[0]
			if got.CancelEvidenceKind != nil || got.CancelConfirmedAt != nil {
				t.Fatalf("%s 不得帶取消欄位（SQL 是 COALESCE，帶值＝寫入一份不存在的取消證據），"+
					"實得 kind=%v at=%v", c.name, got.CancelEvidenceKind, got.CancelConfirmedAt)
			}
		})
	}
}

// 保險：MarkRecheckResult 上真的有這兩個欄位（型別層的存在性）。
// 拿掉欄位會讓上面兩支編譯失敗，這一行只是把「為什麼會編譯失敗」寫清楚。
var _ = domain.MarkRecheckResult{CancelEvidenceKind: nil, CancelConfirmedAt: nil}
