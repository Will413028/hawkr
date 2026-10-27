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

// 本 package 共用一顆 postgres 容器，每支測試拿到一個從已 migrate 的 template
// 複製出來的獨立 database——機制見 pkg/testutil/dbtest 的套件註解。
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

// issuerRefSeq 供 nextIssuerRefID 使用。
var issuerRefSeq atomic.Int64

// nextIssuerRefID 產生一個**保證為正**且在本 process 內唯一的 issuer_ref_id。
//
// 🔴 不要退回 `time.Now().UnixNano()%100000` 的寫法。`chk_invoice_issuers_ref` 要求
// SELLER 的 ref_id **嚴格大於 0**，而那個式子會回 0 —— 在 macOS 上 `UnixNano()` 的
// 實際精度不到奈秒（末幾位常為 0），實測 10000 次取樣有 99 次（0.99%）為 0。
// 一輪整合測試約 18 次呼叫 ⇒ 每 6 次左右就會紅一次，錯誤是 23514 而訊息看起來
// 像業務邏輯錯，非常難追。2026-08-30 追到，證據見本註解。
func nextIssuerRefID() int64 {
	return 9_000_000 + issuerRefSeq.Add(1)
}

// seedIssuer 建一列 invoice_issuers（provider account 的 FK 對象）並回它的 id。
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

// ---------------------------------------------------------------------------
// ① Create 的 rows=0 路徑：uq_ipa_fingerprint 擋跨 issuer 重複綁定（AMEGO S17）
// ---------------------------------------------------------------------------

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

	// 同統編＋同 AppKey 由另一個 issuer 綁 ⇒ uq_ipa_fingerprint 擋下 ⇒ rows=0 ⇒ 業務碼。
	// 🔴 這條路徑是「先查再寫」永遠測不出來的：它在併發下擋不住，而這裡擋住的是 DB。
	err := repo.Create(ctx, newAccount(issuerB+1, issuerB, taxID, appKey))
	if err == nil {
		t.Fatal("同統編＋同 AppKey 被第二個 issuer 綁走了（uq_ipa_fingerprint 沒生效）")
	}
	// 🔴 斷言**精確的業務碼**，不只是「有錯」。
	//
	// 9337（這組憑證已被別人綁走）與 9331（憑證驗證失敗，請確認統編與 AppKey）的
	// 處置完全相反：前者要賣家去確認是不是重複設定，後者要他去改金鑰。回錯碼會把
	// 賣家引導去改一把其實正確的金鑰，而畫面上看起來只是「驗證失敗」。
	// 只斷言 err != nil 的話，這兩個碼互換不會有任何東西轉紅。
	var de *pkgerrors.DomainError
	if !errors.As(err, &de) {
		t.Fatalf("應回 DomainError，實際 %T：%v", err, err)
	}
	if int32(de.Code) != bizcode.InvoiceProviderCredentialTaken {
		t.Fatalf("業務碼不符：got %d, want %d（InvoiceProviderCredentialTaken）",
			de.Code, bizcode.InvoiceProviderCredentialTaken)
	}

	// 同 AppKey ＋ **不同統編** ⇒ fingerprint 不同 ⇒ 應成功。
	// 這一條證明 fingerprint 真的把統編算進去了（DB 層，不只是 Go 層的字串比對）。
	if err := repo.Create(ctx, newAccount(issuerC+1, issuerC, "87654321", appKey)); err != nil {
		t.Fatalf("不同統編＋同 AppKey 應可綁定：%v", err)
	}

	// 讀回來不得帶出任何憑證材料。
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

	// 明文只能經 callback 取得，且必須與寫進去的那一把逐字相同。
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

// ---------------------------------------------------------------------------
// ② 憑證輪替：pending_* 寫入後，現行憑證仍解得開
// ---------------------------------------------------------------------------

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
	// 🔴 P11 的 allowed-from ＝ ACTIVE 單值（2026-08-30 起寫進
	// StartProviderCredentialRotation 的 WHERE）：輪替的前提是「現在這一把已經在服役」，
	// 從 CREDENTIAL_SUBMITTED 也能輪替的話，一把還沒驗過的 key 可以直接覆蓋現役憑證。
	// chk_ipa_active_verified 要求 ACTIVE ⇒ verified_at 非空，所以兩欄一起寫。
	if _, err := pool.Exec(ctx,
		`UPDATE invoice_provider_accounts SET status='ACTIVE', verified_at=now() WHERE id=$1`, id); err != nil {
		t.Fatalf("seed ACTIVE: %v", err)
	}
	before, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}

	// U3：送同一把 key 進來時必須答「這與現行憑證相同」——呼叫端據此不寫任何欄位、不打 AMEGO。
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

	// 🔴 核心：ROTATION_PENDING 期間開票用的仍是**舊的、已驗證的**那一把。
	// 直接覆蓋的話，驗證失敗＝該賣家從那一刻起每一筆開票都失敗，而 UI 上 status 仍是 ACTIVE。
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

	// 樂觀鎖：拿過期的 version 再送一次必須被擋下（rows=0 不得當成功）。
	if err := repo.StartCredentialRotation(ctx, domainStartRotation(id, taxID, newKey, before.Version)); err == nil {
		t.Fatal("過期的 expectedVersion 必須被擋下")
	}

	// 扶正之後才換成新 key，且 pending 三欄同時清空（chk_ipa_rotation 是雙向等式）。
	// 目標狀態是 ACTIVE（P12）：UpdateProviderAccountCredential 的 allowed-from 已收斂成
	// ROTATION_PENDING 單值，而 P12／P13 的目標都是 ACTIVE。
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

// ---------------------------------------------------------------------------
// ③ claim 只推租約欄，不動事實時間戳（§4.6 W-02）
// ---------------------------------------------------------------------------

func TestIntegration_ProviderAccount_ClaimDoesNotTouchLastHealthCheckedAt(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repo := NewProviderAccountRepository(pool, testKeyRing(t))

	issuer := seedIssuer(ctx, t, pool, 9201)
	id := issuer + 1
	if err := repo.Create(ctx, newAccount(id, issuer, "22222222", fmt.Sprintf("HC%d", time.Now().UnixNano()))); err != nil {
		t.Fatalf("Create: %v", err)
	}
	// 推進到 ACTIVE（chk_ipa_active_verified 要求 verified_at 非空）並讓租約到期。
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
	// 第二次必須失敗：租約已被推到未來，述詞 health_check_next_at <= now 不再成立。
	// 這正是「同一時刻只有一個副本在打 AMEGO」的全部保證。
	if ok, err := repo.ClaimHealthCheck(ctx, id, now, now.Add(120*time.Second), now.Add(-24*time.Hour)); err != nil || ok {
		t.Fatalf("第二次 claim 應失敗（ok=%v err=%v）", ok, err)
	}

	// 🔴 claim 不得寫 last_health_checked_at：它是事實時間戳，被推到未來就變成謊言
	// ——憑證看起來剛檢查過、實際上從未檢查，而它同時是那條告警的來源。
	var lastChecked *time.Time
	if err := pool.QueryRow(ctx, `SELECT last_health_checked_at FROM invoice_provider_accounts WHERE id=$1`, id).Scan(&lastChecked); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if lastChecked != nil {
		t.Fatalf("claim 動到了 last_health_checked_at（%v）", *lastChecked)
	}

	// 真的打過之後才由 RecordHealthCheckResult 寫它。
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

// ---------------------------------------------------------------------------
// ③-b 憑證時間戳只有憑證動得了它（#3051）
//
// S03 的「AppKey 更新於」原本下發整列的 updated_at，而健康檢查輪詢每次都改寫它
// （dev 實查同一列 version 已被推到 225、updated_at == last_health_checked_at，
// 而憑證從 2026-08-29 起沒變過）⇒ 賣家唯一能察覺「AppKey 被換過嗎」的訊號永遠是雜訊。
// ---------------------------------------------------------------------------

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

	// ① 陰性：健康檢查跑一輪 ⇒ updated_at 必須前進，credential_updated_at 必須不動。
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

	// ② 陽性對照：真的換一把 key ⇒ 必須前進，且 last4 跟著變。
	//    只驗①分不清「修好了」與「這個欄位被寫成常數」。
	var beforeLast4 string
	if err := pool.QueryRow(ctx,
		`SELECT credential_last4 FROM invoice_provider_accounts WHERE id=$1`, id).Scan(&beforeLast4); err != nil {
		t.Fatalf("scan last4: %v", err)
	}
	// ResubmitCredential 的 allowed-from 不含 ACTIVE（那條路走輪替），先退回可重填的狀態。
	if _, err := pool.Exec(ctx,
		`UPDATE invoice_provider_accounts SET status='INVALID_CREDENTIAL' WHERE id=$1`, id); err != nil {
		t.Fatalf("seed invalid: %v", err)
	}
	var version int64
	if err := pool.QueryRow(ctx,
		`SELECT version FROM invoice_provider_accounts WHERE id=$1`, id).Scan(&version); err != nil {
		t.Fatalf("scan version: %v", err)
	}
	// 固定後綴保證 last4 不同：只靠 UnixNano 的話，時鐘精度只到微秒的平台（macOS）
	// 末三碼恆為 000，新舊 key 的 last4 約 1/10 機率相同，下方的陽性對照會誤報。
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

// ---------------------------------------------------------------------------
// ④ 交易感知：repo 不自己 Begin/Commit，rollback 必須把它寫的列一起帶走
// ---------------------------------------------------------------------------

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
	// 🔴 這條測試的鑑別力：把 baseRepo 改成持有一個 *gen.Queries 欄位（＝永遠拿 pool），
	// 這裡就會變成 1 —— 而業務層「同一交易內寫齊」的保證會靜默消失。
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

// ---------------------------------------------------------------------------
// ⑤ inbox 去重：兩個唯一鍵都要靜默跳過，而且交易不能被 abort
// ---------------------------------------------------------------------------

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

	// 同 event_id ⇒ uq_invinbox_event_id 擋下，靜默跳過。
	if dupID, inserted, err := repo.Insert(ctx, in); err != nil || inserted || dupID != 0 {
		t.Fatalf("重複 event_id 應靜默跳過且不回 id（id=%d inserted=%v err=%v）", dupID, inserted, err)
	}

	// 換一個 event_id 但同業務鍵 ⇒ uq_invinbox_business_key 擋下。
	// 🔴 而且必須在**同一個交易內**還能繼續下一句 SQL：ON CONFLICT DO NOTHING 若指名了
	// conflict target，這裡會拋 23505 ⇒ 整個交易 aborted（25P02）⇒ 後面每一句都被拒。
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
		// 交易還活著才做得到這一句。
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

// ---------------------------------------------------------------------------
// ⑤b Insert 回的 id 真的能把該列推進 PROCESSED
//
// 這一條守的是一個**只會靜默發生**的缺陷：Insert 原本是 :execrows 只回筆數，而所有
// Mark* 都以 id 為鍵 ⇒ consumer 拿不到 id、那一列永遠停在 RECEIVED ＋
// next_attempt_at=now，也就是永久留在 idx_invinbox_due 的兜底候選集合裡。
// 業務完全正確（Prepare 冪等），沒有任何測試會紅，只是 INV.dl 每一輪白撈一次。
// ---------------------------------------------------------------------------

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

	// 拿回來的 id 必須指向剛寫入的那一列，不是隨便一個數字。
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

	// 陰性：推進之後不得還留在兜底候選集合裡（FetchDue 的述詞不含 PROCESSED）。
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

// ---------------------------------------------------------------------------
// ⑥ 行事曆鏡像為空時回 nil 而不是錯誤
// ---------------------------------------------------------------------------

func TestIntegration_BusinessDay_EmptyMirrorReturnsNil(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repo := NewBusinessDayRepository(pool)

	// 自己造出「空鏡像」這個前置狀態，而不是「非空就 skip」——後者在 migration
	// 哪天加了 seed 之後會靜默不執行，而測試報告仍然是綠的。
	if _, err := pool.Exec(ctx, `TRUNCATE invoice_business_day_mirror`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	got, err := repo.SyncedAt(ctx)
	if err != nil {
		t.Fatalf("空表不該回錯誤：%v", err)
	}
	if got != nil {
		t.Fatalf("空表應回 nil，實得 %v", *got)
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
