package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)


func TestRecheck_J24_PassesCancelEvidenceThrough(t *testing.T) {
	uc, ops, _ := rcFixture(t, true)

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

var _ = domain.MarkRecheckResult{CancelEvidenceKind: nil, CancelConfirmedAt: nil}
