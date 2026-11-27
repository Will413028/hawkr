//go:build integration

package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)


func seedManualJob(ctx context.Context, t *testing.T, pool *pgxpool.Pool,
	docStatus, opStatus string) (docID, opID int64) {
	t.Helper()
	issuerID := seedIssuerFor(ctx, t, pool, nextID()%1_000_000_000)
	docID = nextID()
	mustExec(ctx, t, pool,
		`INSERT INTO invoice_documents (
		   id, issuer_id, issuer_name_snapshot, source_type, source_ref, trigger_type,
		   recipient_type, recipient_ref_id, recipient_name, recipient_tax_id,
		   document_kind, fulfillment_mode, status,
		   snapshot_version, issue_attempt_seq,
		   sales_amount, tax_amount, total_amount, tax_type, tax_rate_bps,
		   issue_not_before, statutory_deadline_at)
		 VALUES ($1, $2, 'seed-issuer', 'ORDER', $3, 'AUTO',
		         'BUYER', 2623900520, '消費者', '0000000000',
		         'B2C', 'AMEGO_AUTO', $4,
		         1, 1,
		         100, 0, 100, 1, 500,
		         now() - interval '1 day', now() + interval '2 days')`,
		docID, issuerID, "O"+time.Now().Format("20060102150405")+itoa(docID%1000), docStatus)

	opID = nextID()
	var cancelEvidence any
	if opStatus == "CANCELLED_BY_ADMIN" {
		cancelEvidence = "PROVIDER_QUERY_71"
	}
	mustExec(ctx, t, pool,
		`INSERT INTO invoice_operations (
		   id, document_id, operation_type, operation_key, provider_request_no,
		   request_snapshot_hash, status, max_attempts, next_attempt_at, cancel_evidence_kind)
		 VALUES ($1, $2, 'ISSUE', $3, $4, 'hash-v1', $5, 5, now(), $6)`,
		opID, docID, "issue:"+itoa(opID)+":1", "REQ-"+itoa(opID), opStatus, cancelEvidence)
	return docID, opID
}

func seedComplaintOn(ctx context.Context, t *testing.T, pool *pgxpool.Pool) (*ComplaintRepository, int64, int64, string) {
	t.Helper()
	issuerID := seedIssuerFor(ctx, t, pool, nextID()%1_000_000_000)
	orderNo := "O" + itoa(nextID()%1_000_000_000_000)
	docID := seedOrderDocument(ctx, t, pool, issuerID, iBuyerA, orderNo, "ISSUED", 1, time.Now().UTC())
	return NewComplaintRepository(pool), issuerID, docID, orderNo
}

func mustScan(ctx context.Context, t *testing.T, pool *pgxpool.Pool, sql string, args []any, dest ...any) {
	t.Helper()
	if err := pool.QueryRow(ctx, sql, args...).Scan(dest...); err != nil {
		t.Fatalf("query %s: %v", sql, err)
	}
}

func itoa(v int64) string {
	const digits = "0123456789"
	if v == 0 {
		return "0"
	}
	var b []byte
	for v > 0 {
		b = append([]byte{digits[v%10]}, b...)
		v /= 10
	}
	return string(b)
}

// 🔴 這支守的是本波新增的那句 `AND version = @expected_version`。
// 少了它，端點 56／62 的 expectedVersion 就只是應用層的一句 if（純 TOCTOU）。
func TestIntegration_AdminWrite_ResolvedExternallyRequiresVersion(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repo := NewOperationRepository(pool)
	_, opID := seedManualJob(ctx, t, pool, "ISSUE_FAILED", "MANUAL_PROCESS_ALLOWED")

	ok, err := repo.MarkResolvedExternally(ctx, opID, 99, time.Now().UTC())
	if err != nil {
		t.Fatalf("MarkResolvedExternally: %v", err)
	}
	if ok {
		t.Fatal("version 不符時必須 rows=0——否則樂觀鎖是假的")
	}
	var status string
	mustScan(ctx, t, pool, `SELECT status FROM invoice_operations WHERE id = $1`, []any{opID}, &status)
	if status != "MANUAL_PROCESS_ALLOWED" {
		t.Fatalf("守衛不成立卻改了狀態：%q", status)
	}

	ok, err = repo.MarkResolvedExternally(ctx, opID, 0, time.Now().UTC())
	if err != nil {
		t.Fatalf("MarkResolvedExternally: %v", err)
	}
	if !ok {
		t.Fatal("版本正確時應該轉移成功")
	}
}

// allowed-from 只有 MANUAL_PROCESS_ALLOWED（J30）。
func TestIntegration_AdminWrite_ResolvedExternallyStateGuard(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repo := NewOperationRepository(pool)
	for _, from := range []string{"FAILED_PERMANENT", "CANCELLED_BY_ADMIN", "READY"} {
		t.Run(from, func(t *testing.T) {
			_, opID := seedManualJob(ctx, t, pool, "ISSUE_FAILED", from)
			ok, err := repo.MarkResolvedExternally(ctx, opID, 0, time.Now().UTC())
			if err != nil {
				t.Fatalf("MarkResolvedExternally: %v", err)
			}
			if ok {
				t.Fatalf("%s 不是 J30 的來源狀態，必須 rows=0", from)
			}
		})
	}
}

// 🔴 chk_invdoc_external_no 是**三條**雙向等式：只寫號碼、另兩欄留空會當場 23514。
// 這一條在 fake 上永遠測不到——fake 沒有 CHECK。
func TestIntegration_AdminWrite_DocumentResolvedExternallyWritesAllThreeColumns(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repo := NewDocumentRepository(pool, testKeyRing(t))
	docID, _ := seedManualJob(ctx, t, pool, "ISSUE_FAILED", "MANUAL_PROCESS_ALLOWED")

	now := time.Now().UTC()
	ok, err := repo.MarkResolvedExternally(ctx, domain.MarkDocumentResolvedExternally{
		ID:                  docID,
		ExternalInvoiceNo:   "AB12345678",
		ExternalInvoiceDate: now.AddDate(0, 0, -1),
		ExternalReportedAt:  now,
		Now:                 now,
	})
	if err != nil {
		t.Fatalf("MarkResolvedExternally: %v", err)
	}
	if !ok {
		t.Fatal("ISSUE_FAILED 是合法來源，應該轉移成功")
	}

	var status, extNo string
	var extDate, reportedAt *time.Time
	var providerNo *string
	mustScan(ctx, t, pool,
		`SELECT status, external_invoice_no, external_invoice_date, external_reported_at, provider_invoice_no
		   FROM invoice_documents WHERE id = $1`, []any{docID},
		&status, &extNo, &extDate, &reportedAt, &providerNo)

	if status != "RESOLVED_EXTERNALLY" || extNo != "AB12345678" || extDate == nil || reportedAt == nil {
		t.Fatalf("三個 external_* 欄必須與狀態同進同出：status=%q no=%q date=%v at=%v",
			status, extNo, extDate, reportedAt)
	}
	if providerNo != nil {
		t.Fatalf("provider_invoice_no 必須保持 NULL，實得 %q", *providerNo)
	}
}

// K08／K09 進 RESOLVED／VIOLATION_CONFIRMED 時 resolved_at 必須同一句寫
// （chk_invcomp_resolved_pair 是雙向等式）。這一條同樣只有真 DB 驗得到。
func TestIntegration_AdminWrite_DecideEscalatedWritesResolvedAt(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repo, issuerID, docID, orderNo := seedComplaintOn(ctx, t, pool)

	now := time.Now().UTC()
	c, inserted, err := repo.Create(ctx, newComplaintInput(nextID(), issuerID, docID, iBuyerA, orderNo, now))
	if err != nil || !inserted {
		t.Fatalf("建案失敗：%v inserted=%v", err, inserted)
	}
	mustExec(ctx, t, pool,
		`UPDATE invoice_complaints SET status = 'ESCALATED_TO_ADM' WHERE id = $1`, c.ID)

	ok, err := repo.DecideEscalated(ctx, domain.DecideComplaint{
		ID:                 c.ID,
		ExpectedVersion:    c.Version,
		NextStatus:         "VIOLATION_CONFIRMED",
		ResolutionCode:     "VIOLATION",
		ResolutionNote:     "賣家未開立",
		DecidedByAccountID: 42,
		Now:                now,
	})
	if err != nil {
		t.Fatalf("DecideEscalated: %v", err)
	}
	if !ok {
		t.Fatal("ESCALATED_TO_ADM 是合法來源，應該轉移成功")
	}

	var status string
	var resolvedAt *time.Time
	var decidedBy *int64
	mustScan(ctx, t, pool,
		`SELECT status, resolved_at, decided_by_account_id FROM invoice_complaints WHERE id = $1`,
		[]any{c.ID}, &status, &resolvedAt, &decidedBy)
	if status != "VIOLATION_CONFIRMED" || resolvedAt == nil || decidedBy == nil {
		t.Fatalf("VIOLATION_CONFIRMED 必須同時帶 resolved_at 與 decided_by：%q %v %v",
			status, resolvedAt, decidedBy)
	}

	count, err := repo.CountConfirmedViolationOrders(ctx, issuerID, now.AddDate(0, 0, -90))
	if err != nil {
		t.Fatalf("CountConfirmedViolationOrders: %v", err)
	}
	if count != 1 {
		t.Fatalf("90 天窗口內應計 1 件，實得 %d", count)
	}
}

// K10 必須同時重置 sla_due_at 並清 overdue_at：兩者都不會拋例外，這正是它們危險的地方。
func TestIntegration_AdminWrite_ReturnToSellerResetsSla(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repo, issuerID, docID, orderNo := seedComplaintOn(ctx, t, pool)

	now := time.Now().UTC()
	c, _, err := repo.Create(ctx, newComplaintInput(nextID(), issuerID, docID, iBuyerA, orderNo, now))
	if err != nil {
		t.Fatalf("建案失敗：%v", err)
	}
	mustExec(ctx, t, pool,
		`UPDATE invoice_complaints
		    SET status = 'ESCALATED_TO_ADM', overdue_at = now(), sla_due_at = now() - interval '5 days'
		  WHERE id = $1`, c.ID)

	newSla := now.Add(72 * time.Hour)
	ok, err := repo.ReturnToSeller(ctx, c.ID, c.Version, newSla, now)
	if err != nil {
		t.Fatalf("ReturnToSeller: %v", err)
	}
	if !ok {
		t.Fatal("ESCALATED_TO_ADM 是合法來源，應該轉移成功")
	}

	var status string
	var slaDueAt time.Time
	var overdueAt *time.Time
	mustScan(ctx, t, pool,
		`SELECT status, sla_due_at, overdue_at FROM invoice_complaints WHERE id = $1`,
		[]any{c.ID}, &status, &slaDueAt, &overdueAt)
	if status != "OPEN" {
		t.Fatalf("K10 應回 OPEN，實得 %q", status)
	}
	if slaDueAt.Before(now) {
		t.Fatal("sla_due_at 沒有重置 ⇒ 這一列一進 OPEN 就會被再標一次 OVERDUE")
	}
	if overdueAt != nil {
		t.Fatal("overdue_at 沒有清空 ⇒ 下一輪升級的計時起點會沿用舊值")
	}
}

// 🔴 本支守的是「auditLogId 必須是真的列 id」。
//
// 原本 InsertInvoiceAdminAuditLog 是 :execrows，回的是受影響列數 ⇒ 每一次都回 1。
// 那比不回更糟：**1 長得像一個合法的 id**，ADM 拿它去 A10 查稽核列只會查不到，
// 而且看不出是為什麼。改成 :one RETURNING id 之後型別仍是 int64 ⇒ **編譯完全不會錯**，
// 所以這個不變式只有執行期測得到。
//
// 兩個斷言缺一不可：
//
//	① 回傳值必須等於 DB 裡真的那一列的 id（否則就只是換了個常數）
//	② 兩次不同的插入必須回不同的 id（否則「恆為 1」照樣通過 ①）
func TestIntegration_AdminAuditLog_ReturnsRealRowID(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repo := NewAuditRepository(pool)

	mk := func(reason string) int64 {
		id, err := repo.AppendAdminAuditLog(ctx, domain.NewAdminAuditLogEntry{
			ActorAccountID: 900001,
			Action:         "INVOICE_JOB_RETRY",
			ResourceType:   "INVOICE_OPERATION",
			ResourceID:     7788,
			ReasonCode:     "TEST",
			ReasonDetail:   reason,
			Now:            time.Now().UTC(),
		})
		if err != nil {
			t.Fatalf("AppendAdminAuditLog: %v", err)
		}
		return id
	}

	first := mk("first")
	second := mk("second")

	if first == second {
		t.Fatalf("兩次插入回了同一個 id（%d）—— 那是 rows 不是 id", first)
	}

	var reason string
	if err := pool.QueryRow(ctx,
		`SELECT reason_detail FROM invoice_admin_audit_logs WHERE id = $1`, first,
	).Scan(&reason); err != nil {
		t.Fatalf("回傳的 id 在 DB 裡查不到：%v", err)
	}
	if reason != "first" {
		t.Fatalf("回傳的 id 指向錯的列：reason_detail=%q", reason)
	}
}

func seedIssuedDocument(ctx context.Context, t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()
	docID, _ := seedManualJob(ctx, t, pool, "ISSUE_FAILED", "SUCCEEDED")
	mustExec(ctx, t, pool,
		`UPDATE invoice_documents
		    SET status = 'ISSUED', provider_invoice_no = 'ZA10068421', provider = 'AMEGO'
		  WHERE id = $1`, docID)
	return docID
}


func TestIntegration_NotificationResend_TenMinuteWindow(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repo := NewDocumentRepository(pool, testKeyRing(t))
	docID := seedIssuedDocument(ctx, t, pool)
	now := time.Now().UTC()
	const window = 10 * time.Minute

	seq, claimed, err := repo.ClaimNotificationResend(ctx, docID, now.Add(-window), now)
	if err != nil || !claimed || seq != 1 {
		t.Fatalf("第一次應放行且 seq=1：seq=%d claimed=%v err=%v", seq, claimed, err)
	}

	if _, claimed, err = repo.ClaimNotificationResend(ctx, docID, now.Add(-window), now); err != nil || claimed {
		t.Fatalf("窗口內必須擋下（那是 9352 唯一的防線）：claimed=%v err=%v", claimed, err)
	}

	at9 := now.Add(9 * time.Minute)
	if _, claimed, err = repo.ClaimNotificationResend(ctx, docID, at9.Add(-window), at9); err != nil || claimed {
		t.Fatalf("9 分鐘仍在窗口內：claimed=%v err=%v", claimed, err)
	}

	at10 := now.Add(window)
	seq, claimed, err = repo.ClaimNotificationResend(ctx, docID, at10.Add(-window), at10)
	if err != nil || !claimed || seq != 2 {
		t.Fatalf("滿 10 分鐘應放行且 seq=2：seq=%d claimed=%v err=%v", seq, claimed, err)
	}
}

// 🔴 重寄**不得遞增 version**：它不是狀態轉移，沒有人拿它做樂觀鎖。跟著 +1 的話，
// ADM 開著 A06 頁面時只要有人按了重寄，他的重試／取消就會被回 9343 而畫面上什麼都沒變。
func TestIntegration_NotificationResend_DoesNotBumpVersion(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repo := NewDocumentRepository(pool, testKeyRing(t))
	docID := seedIssuedDocument(ctx, t, pool)
	now := time.Now().UTC()

	before, err := repo.GetByID(ctx, docID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if _, claimed, err := repo.ClaimNotificationResend(ctx, docID, now.Add(-10*time.Minute), now); err != nil || !claimed {
		t.Fatalf("claim 應成功：claimed=%v err=%v", claimed, err)
	}
	after, err := repo.GetByID(ctx, docID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if after.Version != before.Version {
		t.Fatalf("重寄不得動 version：before=%d after=%d", before.Version, after.Version)
	}
	if after.NotificationResendCount != 1 || after.NotificationResentAt == nil {
		t.Fatalf("兩欄必須同時寫入，實得 count=%d at=%v", after.NotificationResendCount, after.NotificationResentAt)
	}
}

// chk_invdoc_resend_pair：時點與次數同進同出（雙向）。
//
// 只有次數沒有時點 ⇒ 窗口述詞永遠放行（等於沒有限流）；
// 只有時點沒有次數 ⇒ notification_ref_id 的去重種子恆為 0，第二封信會被
// notification-service 依 reference_id 靜默吞掉，而按下去的人看到的是成功。
func TestIntegration_NotificationResend_PairCheckIsBidirectional(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	docID := seedIssuedDocument(ctx, t, pool)

	if _, err := pool.Exec(ctx,
		`UPDATE invoice_documents SET notification_resend_count = 1 WHERE id = $1`, docID); err == nil {
		t.Fatal("只有次數沒有時點必須被擋下（窗口述詞會永遠放行）")
	}
	if _, err := pool.Exec(ctx,
		`UPDATE invoice_documents SET notification_resent_at = now() WHERE id = $1`, docID); err == nil {
		t.Fatal("只有時點沒有次數必須被擋下（ref id 的去重種子恆為 0）")
	}
	if _, err := pool.Exec(ctx,
		`UPDATE invoice_documents SET notification_resent_at = now(), notification_resend_count = 1
		  WHERE id = $1`, docID); err != nil {
		t.Fatalf("兩欄一起寫應該可以：%v", err)
	}
}
