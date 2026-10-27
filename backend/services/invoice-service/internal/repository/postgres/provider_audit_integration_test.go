//go:build integration

// 本檔跑真的 SQL，守的是 provider 稽核落庫這條路徑——它在單元測試裡**完全驗不到**，
// 而且失效時是全靜默的。
//
// 2026-09-01 的實際事故：`amego.auditFromContext` 在沒有 audit context 時回
// attempt_seq=0，而 `chk_invpaudit_attempt` 要求 >= 1 ⇒ 每一次 AMEGO 往返的稽核
// INSERT 都被 DB 拒絕；`repoAuditSink.record` 又（正確地）把錯誤吞掉只記一行 log
// ⇒ **invoice_provider_audit_logs 從上線到被發現為止一列都沒有**，而沒有任何
// 測試、gate 或告警會紅。dev 實測：該表 0 列，同 DB 的 invoice_status_history 89 列、
// invoice_admin_audit_logs 3 列（對照組，證明查得到）。
//
// 單元測試為什麼看不到：sink 的測試用 fake repo，fake 沒有 CHECK 約束 ⇒ 對這一類的
// 鑑別力是零（同 `.claude/rules/03-delegation-templates.md` §0.3：手寫 fixture 等於
// 自己跟自己對）。只有真的打到 Postgres 才驗得到。
package postgres

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// newProviderAuditEntry 組一筆「除了 attempt_seq 以外都合法」的稽核列。
func newProviderAuditEntry(issuerID int64, attemptSeq int32) domain.NewProviderAuditLogEntry {
	return domain.NewProviderAuditLogEntry{
		IssuerID:      issuerID,
		Provider:      "AMEGO",
		Endpoint:      "/json/ban_query",
		AttemptSeq:    attemptSeq,
		RequestMasked: json.RawMessage(`{"data":"[]","time":"1","sign":"****"}`),
		RawBodyHash:   "0000000000000000000000000000000000000000000000000000000000000000",
		Now:           time.Now().UTC(),
	}
}

// ① 正向：attempt_seq >= 1 真的落得了庫。
//
// 這一支若不存在，「稽核有沒有在寫」就沒有任何自動化的答案——事故當時正是如此。
func TestIntegration_ProviderAudit_AppendLandsARow(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repo := NewAuditRepository(pool)
	issuer := seedIssuer(ctx, t, pool, nextIssuerRefID())
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM invoice_provider_audit_logs WHERE issuer_id = $1`, issuer)
	})

	if err := repo.AppendProviderAuditLog(ctx, newProviderAuditEntry(issuer, 1)); err != nil {
		t.Fatalf("attempt_seq=1 應該落得了庫，實得: %v", err)
	}

	var n int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM invoice_provider_audit_logs WHERE issuer_id = $1`, issuer).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("應有 1 列稽核，實得 %d —— 這正是事故的形狀：呼叫沒報錯但表是空的", n)
	}
}

// ② 反向：attempt_seq=0 真的會被 DB 擋下。
//
// 🔴 這一支是本檔的鑑別力來源。少了它，①「改成 1 就會過」只證明 1 可以，
// 證明不了「0 不行」——而事故的成因正是 0 被送進來。
func TestIntegration_ProviderAudit_ZeroAttemptSeqIsRejectedByDB(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repo := NewAuditRepository(pool)
	issuer := seedIssuer(ctx, t, pool, nextIssuerRefID())

	err := repo.AppendProviderAuditLog(ctx, newProviderAuditEntry(issuer, 0))
	if err == nil {
		t.Fatal("attempt_seq=0 必須被 chk_invpaudit_attempt 擋下；沒擋下代表約束被拿掉了")
	}

	var n int
	if qerr := pool.QueryRow(ctx,
		`SELECT count(*) FROM invoice_provider_audit_logs WHERE issuer_id = $1`, issuer).Scan(&n); qerr != nil {
		t.Fatalf("count: %v", qerr)
	}
	if n != 0 {
		t.Fatalf("被拒的那筆不該留下任何列，實得 %d", n)
	}
}
