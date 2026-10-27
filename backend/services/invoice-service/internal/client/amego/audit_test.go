package amego

import (
	"context"
	"testing"
)

// TestAuditFromContext_DefaultsToFirstAttempt 守 attempt_seq 的下限。
//
// 🔴 這一支守的是「不會回 0」，而 0 的後果**不在本 package**：DB 的
// chk_invpaudit_attempt 是 >= 1，而 sink 會把 INSERT 失敗吞掉只記一行 log
// ⇒ 回 0 的那天，整張 invoice_provider_audit_logs 會安靜地保持空的
// （2026-09-01 實際發生過，上線至今 0 列）。
//
// ⚠️ 它擋不住「別的欄位違反別的約束」——那要 integration test
// （provider_audit_integration_test.go）才驗得到。兩支是不同的問題，不可互相取代。
func TestAuditFromContext_DefaultsToFirstAttempt(t *testing.T) {
	for _, tc := range []struct {
		name    string
		ctx     context.Context
		wantOp  int64
		wantSeq int
	}{
		{"沒有 audit context", context.Background(), 0, 1},
		{"呼叫端傳 0（0-based 算錯）", WithAuditContext(context.Background(), 7, 0), 7, 1},
		{"呼叫端傳負數", WithAuditContext(context.Background(), 7, -3), 7, 1},
		{"呼叫端傳合法值就原樣沿用", WithAuditContext(context.Background(), 7, 4), 7, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			op, seq := auditFromContext(tc.ctx)
			if op != tc.wantOp || seq != tc.wantSeq {
				t.Fatalf("operation_id/attempt_seq = %d/%d，期望 %d/%d", op, seq, tc.wantOp, tc.wantSeq)
			}
			if seq < 1 {
				t.Fatalf("attempt_seq 永遠不得小於 1（DB chk_invpaudit_attempt），實得 %d", seq)
			}
		})
	}
}
