//go:build integration

// 本檔跑真的 SQL。它存在的理由是：本 package 幾乎每一條規則的失效形狀都**不是編譯錯**
// ——ON CONFLICT DO NOTHING 的 rows=0、CAS 的 rows=0、partial unique 的述詞邊界、
// 「claim 不得動事實時間戳」、「交易感知」，靜態測試一條都驗不到。
//
// 跑法：`make test-integration`（需要本機 Docker daemon）。容器由
// pkg/testutil/dbtest 起一顆 postgres:16-alpine、重放 db/migrations，每支測試拿到
// 一個自己的 database。機制與 im-service / notification-service 的
// dbtest_test.go 相同。
//
// 🔴 刻意不用「環境變數沒設就 t.Skip」：那會讓 CI 的 job 永久綠而測試從未執行過
// （2026-08-30 實測 12 PASS / 6 SKIP，skip 掉的正是只有真 DB 驗得到的那些）。
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/go-pkg/crypto"
	"github.com/Will413028/hawkr/go-pkg/database"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/go-pkg/testutil/dbtest"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

var pg = dbtest.New(dbtest.Config{
	TemplateDB:    "invoice_test",
	MigrationsDir: filepath.Join("..", "..", "..", "db", "migrations"),
})

func TestMain(m *testing.M) {
	dbtest.RunTests(m, pg)
}

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	db, _ := pg.Clone(ctx, t)
	pool, err := pgxpool.New(ctx, db.DSN)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func testKeyRing(t *testing.T) *KeyRing {
	t.Helper()
	cur, err := crypto.New("integration-master-key")
	if err != nil {
		t.Fatalf("crypto.New: %v", err)
	}
	return &KeyRing{Current: cur, CurrentVersion: 1, LookupHMACKey: "integration-lookup-key"}
}

var issuerRefSeq atomic.Int64

func nextIssuerRefID() int64 {
	return 9_000_000 + issuerRefSeq.Add(1)
}

func seedIssuer(ctx context.Context, t *testing.T, pool *pgxpool.Pool, refID int64) int64 {
	t.Helper()
	id := time.Now().UnixNano() + refID
	_, err := pool.Exec(ctx,
		`INSERT INTO invoice_issuers (id, issuer_type, issuer_ref_id, display_name, status)
		 VALUES ($1, 'SELLER', $2, $3, 'active')`,
		id, refID, fmt.Sprintf("test-seller-%d", refID))
	if err != nil {
		t.Fatalf("seed issuer: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM invoice_provider_accounts WHERE issuer_id = $1`, id)
		_, _ = pool.Exec(context.Background(), `DELETE FROM invoice_issuers WHERE id = $1`, id)
	})
	return id
}

func TestIntegration_ProviderAccount_FingerprintUniquenessIsEnforcedByDB(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repo := NewProviderAccountRepository(pool, testKeyRing(t))

	issuerA := seedIssuer(ctx, t, pool, 9001)
	issuerB := seedIssuer(ctx, t, pool, 9002)
	issuerC := seedIssuer(ctx, t, pool, 9003)

	const taxID = "12345678"
	appKey := fmt.Sprintf("AK%d", time.Now().UnixNano())

	if err := repo.Create(ctx, newAccount(issuerA+1, issuerA, taxID, appKey)); err != nil {
		t.Fatalf("第一次綁定應成功：%v", err)
	}

	err := repo.Create(ctx, newAccount(issuerB+1, issuerB, taxID, appKey))
	if err == nil {
		t.Fatal("同統編＋同 AppKey 被第二個 issuer 綁走了（uq_ipa_fingerprint 沒生效）")
	}
	var de *pkgerrors.DomainError
	if !errors.As(err, &de) {
		t.Fatalf("應回 DomainError，實際 %T：%v", err, err)
	}
	if int32(de.Code) != bizcode.InvoiceProviderCredentialTaken {
		t.Fatalf("業務碼不符：got %d, want %d（InvoiceProviderCredentialTaken）",
			de.Code, bizcode.InvoiceProviderCredentialTaken)
	}

	if err := repo.Create(ctx, newAccount(issuerC+1, issuerC, "87654321", appKey)); err != nil {
		t.Fatalf("不同統編＋同 AppKey 應可綁定：%v", err)
	}

	got, err := repo.GetByIssuer(ctx, issuerA, "AMEGO")
	if err != nil {
		t.Fatalf("GetByIssuer: %v", err)
	}
	if got.CredentialLast4 != credentialLast4(appKey) {
		t.Fatalf("last4 = %q, want %q", got.CredentialLast4, credentialLast4(appKey))
	}
	if got.HasPendingCredential {
		t.Fatal("剛建立的帳戶不該處於輪替中")
	}

	var seen string
	if err := repo.WithDecryptedCredential(ctx, issuerA+1, func(taxid, key string) error {
		seen = key
		if taxid != taxID {
			t.Errorf("callback 拿到的統編 = %q, want %q", taxid, taxID)
		}
		return nil
	}); err != nil {
		t.Fatalf("WithDecryptedCredential: %v", err)
	}
	if seen != appKey {
		t.Fatalf("解出來的 AppKey 不對：%q", seen)
	}
}

func newAccount(id, issuerID int64, taxID, appKey string) domain.NewProviderAccount {
	return domain.NewProviderAccount{
		ID:                id,
		IssuerID:          issuerID,
		Provider:          "AMEGO",
		ProviderTaxID:     taxID,
		AppKey:            appKey,
		Status:            "CREDENTIAL_SUBMITTED",
		HealthCheckNextAt: time.Now().UTC(),
	}
}

func TestIntegration_ProviderAccount_RotationKeepsCurrentCredentialUsable(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repo := NewProviderAccountRepository(pool, testKeyRing(t))

	issuer := seedIssuer(ctx, t, pool, 9101)
	id := issuer + 1
	const taxID = "11111111"
	oldKey := fmt.Sprintf("OLD%d", time.Now().UnixNano())
	newKey := fmt.Sprintf("NEW%d", time.Now().UnixNano())

	if err := repo.Create(ctx, newAccount(id, issuer, taxID, oldKey)); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE invoice_provider_accounts SET status='ACTIVE', verified_at=now() WHERE id=$1`, id); err != nil {
		t.Fatalf("seed ACTIVE: %v", err)
	}
	before, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}

	same, err := repo.MatchesCurrentCredential(ctx, id, taxID, oldKey)
	if err != nil {
		t.Fatalf("MatchesCurrentCredential: %v", err)
	}
	if !same {
		t.Fatal("同一把 key 必須被判為相同")
	}
	if diff, _ := repo.MatchesCurrentCredential(ctx, id, taxID, newKey); diff {
		t.Fatal("不同的 key 被判成相同：賣家會以為換過了，實際上外洩的那一把還在用")
	}

	if err := repo.StartCredentialRotation(ctx, domainStartRotation(id, taxID, newKey, before.Version)); err != nil {
		t.Fatalf("StartCredentialRotation: %v", err)
	}

	mid, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if mid.Status != "ROTATION_PENDING" {
		t.Fatalf("status = %q, want ROTATION_PENDING", mid.Status)
	}
	if !mid.HasPendingCredential {
		t.Fatal("HasPendingCredential 應為 true")
	}
	if mid.PendingLast4 == nil || *mid.PendingLast4 != credentialLast4(newKey) {
		t.Fatalf("pending_last4 不對：%v", mid.PendingLast4)
	}

	var during string
	if err := repo.WithDecryptedCredential(ctx, id, func(_, key string) error {
		during = key
		return nil
	}); err != nil {
		t.Fatalf("WithDecryptedCredential: %v", err)
	}
	if during != oldKey {
		t.Fatalf("輪替期間解出來的應是舊 key，實得 %q", during)
	}

	if err := repo.StartCredentialRotation(ctx, domainStartRotation(id, taxID, newKey, before.Version)); err == nil {
		t.Fatal("過期的 expectedVersion 必須被擋下")
	}

	if err := repo.PromoteCredential(ctx, domainPromote(id, taxID, newKey, "ACTIVE", mid.Version)); err != nil {
		t.Fatalf("PromoteCredential: %v", err)
	}
	after, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if after.HasPendingCredential {
		t.Fatal("扶正後 pending 三欄必須清空")
	}
	var final string
	if err := repo.WithDecryptedCredential(ctx, id, func(_, key string) error {
		final = key
		return nil
	}); err != nil {
		t.Fatalf("WithDecryptedCredential: %v", err)
	}
	if final != newKey {
		t.Fatalf("扶正後應解出新 key，實得 %q", final)
	}
}

func TestIntegration_ProviderAccount_ClaimDoesNotTouchLastHealthCheckedAt(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repo := NewProviderAccountRepository(pool, testKeyRing(t))

	issuer := seedIssuer(ctx, t, pool, 9201)
	id := issuer + 1
	if err := repo.Create(ctx, newAccount(id, issuer, "22222222", fmt.Sprintf("HC%d", time.Now().UnixNano()))); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE invoice_provider_accounts SET status='ACTIVE', verified_at=now(), health_check_next_at=now() - interval '1 hour' WHERE id=$1`, id); err != nil {
		t.Fatalf("seed active: %v", err)
	}

	now := time.Now().UTC()
	due, err := repo.FetchDueHealthChecks(ctx, now, now.Add(-24*time.Hour), 10)
	if err != nil {
		t.Fatalf("FetchDueHealthChecks: %v", err)
	}
	found := false
	for _, a := range due {
		if a.ID == id {
			found = true
		}
	}
	if !found {
		t.Fatal("到期的 ACTIVE 帳戶不在候選集合裡")
	}

	ok, err := repo.ClaimHealthCheck(ctx, id, now, now.Add(120*time.Second), now.Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("ClaimHealthCheck: %v", err)
	}
	if !ok {
		t.Fatal("第一次 claim 應成功")
	}
	if ok, err := repo.ClaimHealthCheck(ctx, id, now, now.Add(120*time.Second), now.Add(-24*time.Hour)); err != nil || ok {
		t.Fatalf("第二次 claim 應失敗（ok=%v err=%v）", ok, err)
	}

	var lastChecked *time.Time
	if err := pool.QueryRow(ctx, `SELECT last_health_checked_at FROM invoice_provider_accounts WHERE id=$1`, id).Scan(&lastChecked); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if lastChecked != nil {
		t.Fatalf("claim 動到了 last_health_checked_at（%v）", *lastChecked)
	}

	checkedAt := time.Now().UTC().Truncate(time.Millisecond)
	if err := repo.RecordHealthCheckResult(ctx, domainHealthResult(id, checkedAt)); err != nil {
		t.Fatalf("RecordHealthCheckResult: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT last_health_checked_at FROM invoice_provider_accounts WHERE id=$1`, id).Scan(&lastChecked); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if lastChecked == nil {
		t.Fatal("RecordHealthCheckResult 之後 last_health_checked_at 仍為 NULL")
	}
}

func TestIntegration_ProviderAccount_CredentialTimestampMovesOnlyWithCredential(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repo := NewProviderAccountRepository(pool, testKeyRing(t))

	issuer := seedIssuer(ctx, t, pool, 9301)
	id := issuer + 1
	if err := repo.Create(ctx, newAccount(id, issuer, "33333333", fmt.Sprintf("CT%dOLD1", time.Now().UnixNano()))); err != nil {
		t.Fatalf("Create: %v", err)
	}

	read := func() (credUpdated *time.Time, updated time.Time) {
		t.Helper()
		if err := pool.QueryRow(ctx,
			`SELECT credential_updated_at, updated_at FROM invoice_provider_accounts WHERE id=$1`,
			id).Scan(&credUpdated, &updated); err != nil {
			t.Fatalf("scan: %v", err)
		}
		return
	}

	afterCreate, updatedAfterCreate := read()
	if afterCreate == nil {
		t.Fatal("建列時沒寫 credential_updated_at ⇒ 新設定的賣家會看到「—」")
	}

	if _, err := pool.Exec(ctx,
		`UPDATE invoice_provider_accounts SET status='ACTIVE', verified_at=now() WHERE id=$1`, id); err != nil {
		t.Fatalf("seed active: %v", err)
	}
	checkedAt := time.Now().UTC().Add(time.Second).Truncate(time.Millisecond)
	if err := repo.RecordHealthCheckResult(ctx, domainHealthResult(id, checkedAt)); err != nil {
		t.Fatalf("RecordHealthCheckResult: %v", err)
	}
	afterHealth, updatedAfterHealth := read()
	if !updatedAfterHealth.After(updatedAfterCreate) {
		t.Fatal("健康檢查沒有推動 updated_at ⇒ 這個測試的前提（輪詢會碰 updated_at）不成立，" +
			"下面那條斷言就沒有鑑別力")
	}
	if afterHealth == nil || !afterHealth.Equal(*afterCreate) {
		t.Fatalf("健康檢查動到了 credential_updated_at：%v → %v", afterCreate, afterHealth)
	}

	var beforeLast4 string
	if err := pool.QueryRow(ctx,
		`SELECT credential_last4 FROM invoice_provider_accounts WHERE id=$1`, id).Scan(&beforeLast4); err != nil {
		t.Fatalf("scan last4: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE invoice_provider_accounts SET status='INVALID_CREDENTIAL' WHERE id=$1`, id); err != nil {
		t.Fatalf("seed invalid: %v", err)
	}
	var version int64
	if err := pool.QueryRow(ctx,
		`SELECT version FROM invoice_provider_accounts WHERE id=$1`, id).Scan(&version); err != nil {
		t.Fatalf("scan version: %v", err)
	}
	newKey := fmt.Sprintf("CTNEW%dNEW2", time.Now().UnixNano())
	ok, _, err := repo.ResubmitCredential(ctx, domain.ResubmitProviderCredential{
		ID: id, ProviderTaxID: "33333333", AppKey: newKey,
		HealthCheckNextAt: time.Now().UTC(), ExpectedVersion: version,
	})
	if err != nil || !ok {
		t.Fatalf("ResubmitCredential: ok=%v err=%v", ok, err)
	}
	afterResubmit, _ := read()
	if afterResubmit == nil || !afterResubmit.After(*afterHealth) {
		t.Fatalf("換了憑證，credential_updated_at 卻沒前進：%v → %v", afterHealth, afterResubmit)
	}
	var afterLast4 string
	if err := pool.QueryRow(ctx,
		`SELECT credential_last4 FROM invoice_provider_accounts WHERE id=$1`, id).Scan(&afterLast4); err != nil {
		t.Fatalf("scan last4: %v", err)
	}
	if afterLast4 == beforeLast4 {
		t.Fatal("last4 沒變 ⇒ 憑證其實沒換，上面那條斷言證明不了什麼")
	}
}

func TestIntegration_Outbox_HonoursCallerTransaction(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repo := NewOutboxRepository(pool)
	tm := database.NewTxManager(pool)

	routingKey := fmt.Sprintf("test.rollback.%d", time.Now().UnixNano())
	sentinel := fmt.Errorf("deliberate rollback")

	err := tm.WithTx(ctx, func(ctx context.Context) error {
		if _, err := repo.Insert(ctx, newOutbox(routingKey)); err != nil {
			return err
		}
		return sentinel
	})
	if err == nil {
		t.Fatal("WithTx 應把 sentinel 傳出來")
	}

	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM invoice_outbox_events WHERE routing_key=$1`, routingKey).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("rollback 之後仍有 %d 列：repository 沒有跟隨呼叫端的交易", n)
	}

	if err := tm.WithTx(ctx, func(ctx context.Context) error {
		_, err := repo.Insert(ctx, newOutbox(routingKey))
		return err
	}); err != nil {
		t.Fatalf("WithTx: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM invoice_outbox_events WHERE routing_key=$1`, routingKey).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("commit 之後應有 1 列，實得 %d", n)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM invoice_outbox_events WHERE routing_key=$1`, routingKey)
	})
}

func TestIntegration_Inbox_DedupesOnBothUniqueKeysWithoutAbortingTx(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repo := NewInboxRepository(pool)
	tm := database.NewTxManager(pool)

	stamp := time.Now().UnixNano()
	eventID := fmt.Sprintf("evt-%d", stamp)
	dedupKey := fmt.Sprintf("dedup-%d", stamp)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM invoice_inbox_events WHERE business_dedup_key=$1`, dedupKey)
	})

	in := newInbox(eventID, dedupKey)
	id, inserted, err := repo.Insert(ctx, in)
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if !inserted {
		t.Fatal("第一次應寫入成功")
	}
	if id <= 0 {
		t.Fatalf("RETURNING id 應回真實 id，got %d", id)
	}

	if dupID, inserted, err := repo.Insert(ctx, in); err != nil || inserted || dupID != 0 {
		t.Fatalf("重複 event_id 應靜默跳過且不回 id（id=%d inserted=%v err=%v）", dupID, inserted, err)
	}

	other := in
	other.EventID = fmt.Sprintf("evt-%d-b", stamp)
	err = tm.WithTx(ctx, func(ctx context.Context) error {
		otherID, inserted, err := repo.Insert(ctx, other)
		if err != nil {
			return fmt.Errorf("insert: %w", err)
		}
		if inserted || otherID != 0 {
			return fmt.Errorf("同業務鍵應被擋下（id=%d inserted=%v）", otherID, inserted)
		}
		n, err := repo.CountStaleDeferred(ctx, time.Now().UTC())
		if err != nil {
			return fmt.Errorf("交易在唯一鍵衝突後被 abort 了：%w", err)
		}
		_ = n
		return nil
	})
	if err != nil {
		t.Fatalf("%v", err)
	}
}

func TestIntegration_Inbox_ReturnedIDCanAdvanceRowToProcessed(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repo := NewInboxRepository(pool)

	stamp := time.Now().UnixNano()
	eventID := fmt.Sprintf("evt-proc-%d", stamp)
	dedupKey := fmt.Sprintf("dedup-proc-%d", stamp)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM invoice_inbox_events WHERE business_dedup_key=$1`, dedupKey)
	})

	id, inserted, err := repo.Insert(ctx, newInbox(eventID, dedupKey))
	if err != nil || !inserted {
		t.Fatalf("Insert: id=%d inserted=%v err=%v", id, inserted, err)
	}

	var gotEventID, gotStatus string
	if err := pool.QueryRow(ctx,
		`SELECT event_id, status FROM invoice_inbox_events WHERE id=$1`, id,
	).Scan(&gotEventID, &gotStatus); err != nil {
		t.Fatalf("回傳的 id 查無此列: %v", err)
	}
	if gotEventID != eventID {
		t.Fatalf("id=%d 指向的是別列（event_id=%s，want %s）", id, gotEventID, eventID)
	}
	if gotStatus != "RECEIVED" {
		t.Fatalf("初始 status 應為 RECEIVED，got %s", gotStatus)
	}

	ok, err := repo.MarkProcessed(ctx, id, time.Now().UTC())
	if err != nil {
		t.Fatalf("MarkProcessed: %v", err)
	}
	if !ok {
		t.Fatal("MarkProcessed 回 rows=0 —— allowed-from 不含 RECEIVED？")
	}

	if err := pool.QueryRow(ctx,
		`SELECT status FROM invoice_inbox_events WHERE id=$1`, id,
	).Scan(&gotStatus); err != nil {
		t.Fatalf("覆核: %v", err)
	}
	if gotStatus != "PROCESSED" {
		t.Fatalf("status 應為 PROCESSED，got %s", gotStatus)
	}

	due, err := repo.FetchDue(ctx, time.Now().UTC().Add(time.Hour), 100)
	if err != nil {
		t.Fatalf("FetchDue: %v", err)
	}
	for _, e := range due {
		if e.ID == id {
			t.Fatal("PROCESSED 的列仍出現在兜底候選集合裡")
		}
	}
}

func TestIntegration_BusinessDay_NeverPushedReturnsNil(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repo := NewBusinessDayRepository(pool)

	if _, err := pool.Exec(ctx, `TRUNCATE invoice_business_day_mirror, invoice_business_day_coverage`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	_, got, err := repo.Snapshot(ctx, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("沒有涵蓋區間不該回錯誤：%v", err)
	}
	if got != nil {
		t.Fatalf("從未推送應回 nil，實得 %+v", *got)
	}
}

func TestIntegration_BusinessDay_CoverageRoundTripAndOverwrite(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repo := NewBusinessDayRepository(pool)
	if _, err := pool.Exec(ctx, `TRUNCATE invoice_business_day_mirror, invoice_business_day_coverage`); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	day := func(m time.Month, d int) time.Time { return time.Date(2026, m, d, 0, 0, 0, 0, time.UTC) }
	synced := time.Date(2026, 11, 22, 4, 0, 0, 0, time.UTC)
	if err := repo.SetCoverage(ctx, day(10, 23), day(12, 31), synced); err != nil {
		t.Fatalf("SetCoverage: %v", err)
	}
	if err := repo.Insert(ctx, day(12, 25), synced); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	_, got, err := repo.Snapshot(ctx, day(1, 1), day(12, 31))
	if err != nil || got == nil {
		t.Fatalf("Coverage: %v %v", got, err)
	}
	if got.From.Format("2006-01-02") != "2026-10-23" || got.To.Format("2006-01-02") != "2026-12-31" || !got.SyncedAt.Equal(synced) {
		t.Fatalf("涵蓋區間沒有原樣存取：%+v", *got)
	}

	if err := repo.SetCoverage(ctx, day(11, 1), day(11, 30), synced.Add(time.Hour)); err != nil {
		t.Fatalf("SetCoverage 覆蓋: %v", err)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM invoice_business_day_coverage`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("涵蓋區間應恆為單列，n=%d err=%v", n, err)
	}
	_, got, _ = repo.Snapshot(ctx, day(1, 1), day(12, 31))
	if got.From.Format("2006-01-02") != "2026-11-01" || got.To.Format("2006-01-02") != "2026-11-30" {
		t.Fatalf("覆蓋後的區間不對：%+v", *got)
	}

	if err := repo.SetCoverage(ctx, day(12, 1), day(11, 1), synced); err == nil {
		t.Fatal("covered_from > covered_to 必須被 CHECK 擋下")
	}
}

func domainStartRotation(id int64, taxID, appKey string, version int64) domain.StartCredentialRotation {
	return domain.StartCredentialRotation{ID: id, ProviderTaxID: taxID, AppKey: appKey, ExpectedVersion: version}
}

func domainPromote(id int64, taxID, appKey, nextStatus string, version int64) domain.PromoteCredential {
	return domain.PromoteCredential{ID: id, ProviderTaxID: taxID, AppKey: appKey, NextStatus: nextStatus, ExpectedVersion: version}
}

func domainHealthResult(id int64, checkedAt time.Time) domain.RecordHealthCheckResult {
	return domain.RecordHealthCheckResult{
		ID:          id,
		CheckedAt:   checkedAt,
		NextCheckAt: checkedAt.Add(15 * time.Minute),
		NextStatus:  "ACTIVE",
	}
}

func newOutbox(routingKey string) domain.NewOutboxEvent {
	now := time.Now().UTC()
	return domain.NewOutboxEvent{
		AggregateType: "DOCUMENT",
		AggregateID:   1,
		EventType:     "invoice.issued",
		Exchange:      "hawkr.invoice",
		RoutingKey:    routingKey,
		Payload:       json.RawMessage(`{"k":"v"}`),
		NextAttemptAt: now,
		Now:           now,
	}
}

func newInbox(eventID, dedupKey string) domain.NewInboxEvent {
	now := time.Now().UTC()
	return domain.NewInboxEvent{
		EventID:          eventID,
		EventType:        "order.paid",
		RoutingKey:       "order.paid",
		Exchange:         "hawkr.order",
		BusinessDedupKey: dedupKey,
		Payload:          json.RawMessage(`{"orderNo":"O1"}`),
		Status:           "RECEIVED",
		NextAttemptAt:    now,
		OccurredAt:       now,
		Now:              now,
	}
}

func TestIntegration_BusinessDay_SnapshotSingleQuery(t *testing.T) {
	pool := testPool(t)
	repo := NewBusinessDayRepository(pool)
	tm := database.NewTxManager(pool)
	synced := time.Date(2026, 11, 22, 0, 0, 0, 0, time.UTC)
	day := func(m time.Month, d int) time.Time { return time.Date(2026, m, d, 0, 0, 0, 0, time.UTC) }
	fmtDays := func(ds []time.Time) []string {
		out := make([]string, 0, len(ds))
		for _, d := range ds {
			out = append(out, d.Format("2006-01-02"))
		}
		return out
	}

	modes := map[string]func(t *testing.T, fn func(ctx context.Context)){
		"outside_tx": func(t *testing.T, fn func(ctx context.Context)) { fn(context.Background()) },
		"inside_tx": func(t *testing.T, fn func(ctx context.Context)) {
			if err := tm.WithTx(context.Background(), func(ctx context.Context) error { fn(ctx); return nil }); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, run := range modes {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			if _, err := pool.Exec(ctx, `TRUNCATE invoice_business_day_mirror, invoice_business_day_coverage`); err != nil {
				t.Fatal(err)
			}
			if err := repo.Insert(ctx, day(11, 24), synced); err != nil {
				t.Fatal(err)
			}
			run(t, func(ctx context.Context) {
				h, cov, err := repo.Snapshot(ctx, day(11, 1), day(11, 30))
				if err != nil || cov != nil || len(h) != 1 {
					t.Fatalf("無涵蓋列：holidays=%v cov=%v err=%v", h, cov, err)
				}
			})

			if err := repo.Insert(ctx, day(10, 1), synced); err != nil {
				t.Fatal(err)
			}
			if err := repo.Insert(ctx, day(11, 1), synced); err != nil {
				t.Fatal(err)
			}
			if err := repo.Insert(ctx, day(11, 30), synced); err != nil {
				t.Fatal(err)
			}
			if err := repo.Insert(ctx, day(12, 25), synced); err != nil {
				t.Fatal(err)
			}
			if err := repo.SetCoverage(ctx, day(10, 23), day(12, 31), synced); err != nil {
				t.Fatal(err)
			}
			run(t, func(ctx context.Context) {
				h, cov, err := repo.Snapshot(ctx, day(11, 1), day(11, 30))
				if err != nil || cov == nil {
					t.Fatalf("Snapshot = %v %v %v", h, cov, err)
				}
				if got := fmtDays(h); len(got) != 3 || got[0] != "2026-11-01" || got[1] != "2026-11-24" || got[2] != "2026-11-30" {
					t.Fatalf("區間內假日（含兩端、依日期排序）不對：%v", got)
				}
				if !cov.From.Equal(day(10, 23)) || !cov.To.Equal(day(12, 31)) || !cov.SyncedAt.Equal(synced) {
					t.Fatalf("涵蓋區間不對：%+v", *cov)
				}
			})

			run(t, func(ctx context.Context) {
				h, cov, err := repo.Snapshot(ctx, day(11, 2), day(11, 3))
				if err != nil || len(h) != 0 || cov == nil {
					t.Fatalf("空假日：holidays=%v cov=%v err=%v", h, cov, err)
				}
			})
		})
	}
}
