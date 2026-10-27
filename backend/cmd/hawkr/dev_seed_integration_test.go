//go:build integration

package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// setSellerEnv 設定組裝 seller 的子命令（openSeller）必填的依賴：三個服務位址（惰性連線，子命令不呼叫它們）與 KYC 金鑰。
func setSellerEnv(t *testing.T) {
	t.Helper()
	t.Setenv("MEDIA_SERVICE_ADDR", "127.0.0.1:1")
	t.Setenv("ORDER_SERVICE_ADDR", "127.0.0.1:1")
	t.Setenv("PRODUCT_SERVICE_ADDR", "127.0.0.1:1")
	t.Setenv("KYC_ENCRYPTION_KEYS", "test1:dGVzdC1reWMtZW5jcnlwdGlvbi1rZXktMDAwMDAwMDE=")
	t.Setenv("KYC_HASH_KEYS", "test1:dGVzdC1reWMtaGFzaC1obWFjLWtleS0wMDAwMDAwMDE=")
}

// devSeedEnv 把 dev-seed 與 reannounce-exposures 需要的環境設成本機開發的樣子（與 Makefile 的 MONOLITH_ENV 同一組必填設定；
// 金鑰用整合測試自己的一組），回傳 migrate 過的資料庫。
func devSeedEnv(t *testing.T) *db.DB {
	t.Helper()
	ctx := context.Background()
	url := pg.NewURL(t)
	t.Setenv("DATABASE_URL", url)
	setSellerEnv(t)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "dev-auth.pem")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DEV_AUTH_KEY_FILE", path)
	if err := run(ctx, []string{"migrate"}); err != nil {
		t.Fatalf("migrate：%v", err)
	}
	d, err := db.Open(ctx, db.Config{URL: url, MaxConns: 4})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(d.Close)
	return d
}

func count(t *testing.T, d *db.DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := d.Pool().QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// 種子的店家全部在 seller 模組的可曝光集合裡（模組自己的規則判斷，不是 SQL 重寫一份），後台揭露回的是 fixture 的明文，資料庫參照的金鑰
// 都在執行中的金鑰環裡；全量重新公告之後每家店的公告是 version 1、可曝光。
func TestDevSeedWritesStoresThatTheModuleServesAsExposable(t *testing.T) {
	ctx := context.Background()
	d := devSeedEnv(t)
	var out bytes.Buffer
	if err := runDevSeed(ctx, nil, &out); err != nil {
		t.Fatalf("dev-seed：%v", err)
	}
	if err := runReannounceExposures(ctx, nil, &out); err != nil {
		t.Fatalf("reannounce-exposures：%v", err)
	}
	module, closeFn, err := openSellerCommand(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer closeFn()
	fixture := devSeedStores()
	ids := make([]int64, len(fixture))
	for i, s := range fixture {
		ids[i] = s.StoreID
	}
	got, err := module.ExposableStores(ctx, ids)
	if err != nil {
		t.Fatal(err)
	}
	exposable := map[int64]bool{}
	for _, e := range got {
		exposable[e.StoreID] = true
	}
	for _, s := range fixture {
		if !exposable[s.StoreID] {
			t.Errorf("種子帳號名下的店家 %d 不在可曝光的集合裡（%v）", s.StoreID, got)
		}
		var applicationID uuid.UUID
		if err := d.Pool().QueryRow(ctx, `SELECT application_id FROM seller.kyc_applications WHERE store_id = $1`, s.StoreID).Scan(&applicationID); err != nil {
			t.Fatal(err)
		}
		revealed, err := module.RevealKYCSensitive(ctx, seller.RevealKYCInput{AdminID: 1, ApplicationID: applicationID, Reason: "驗證種子的明文"})
		if err != nil {
			t.Fatalf("店 %d 的揭露：%v", s.StoreID, err)
		}
		if revealed.IDCardNo == nil || *revealed.IDCardNo != *s.KYC.IDCardNo || revealed.BankAccount == nil || *revealed.BankAccount != *s.KYC.BankAccount {
			t.Errorf("店 %d 的揭露 = %+v，want 身分證號 %s、銀行帳號 %s", s.StoreID, revealed, *s.KYC.IDCardNo, *s.KYC.BankAccount)
		}
		var version int64
		var announced bool
		if err := d.Pool().QueryRow(ctx, `SELECT version, exposable FROM seller.store_exposures WHERE store_id = $1`, s.StoreID).Scan(&version, &announced); err != nil {
			t.Fatal(err)
		}
		if version != 1 || !announced {
			t.Errorf("店 %d 的公告列 = (version %d, exposable %v)，want (1, true)", s.StoreID, version, announced)
		}
		var code string
		if err := d.Pool().QueryRow(ctx, `SELECT mer_id FROM seller.payuni_registrations WHERE store_id = $1`, s.StoreID).Scan(&code); err != nil {
			t.Fatal(err)
		}
		if want := "HAWK" + strconv.FormatInt(s.StoreCode, 10); code != want {
			t.Errorf("店 %d 的 mer_id = %s，want %s", s.StoreID, code, want)
		}
	}
	if err := module.CheckKYCKeys(ctx); err != nil {
		t.Fatalf("CheckKYCKeys：%v", err)
	}
	if n := count(t, d, `SELECT count(*) FROM seller.stores WHERE store_id = ANY($1)`, ids); n != len(fixture) {
		t.Errorf("seller.stores 有 %d 家種子店家，want %d", n, len(fixture))
	}
	if n := count(t, d, `SELECT count(*) FROM seller.kyc_applications WHERE id_card_no_enc IS NOT NULL AND position(convert_to('A123456789', 'UTF8') in id_card_no_enc) > 0`); n != 0 {
		t.Errorf("身分證號的明文出現在密文欄位（%d 列）", n)
	}
	var next int64
	if err := d.Pool().QueryRow(ctx, `SELECT nextval(pg_get_serial_sequence('seller.stores', 'store_id'))`).Scan(&next); err != nil {
		t.Fatal(err)
	}
	if next <= 9999 {
		t.Errorf("IDENTITY 序號 = %d，want 推到固定 ID 之後（> 9999）", next)
	}
}

// 種子店家都有入駐同意的紀錄（開店的 SELLER_TERMS 與已核准的 KYC 要求的兩份），版本是現行版本；重跑之後筆數相同，不重複。
func TestDevSeedWritesTheAgreementAcceptancesTheInvariantRequires(t *testing.T) {
	ctx := context.Background()
	d := devSeedEnv(t)
	for range 2 {
		if err := runDevSeed(ctx, nil, new(bytes.Buffer)); err != nil {
			t.Fatalf("dev-seed：%v", err)
		}
		for _, st := range devSeedStores() {
			if n := count(t, d, `SELECT count(*) FROM seller.agreement_acceptances WHERE store_id = $1`, st.StoreID); n != 3 {
				t.Fatalf("store %d 有 %d 筆同意紀錄，want 3", st.StoreID, n)
			}
			for _, k := range seller.AgreementKinds() {
				cur, _ := seller.CurrentAgreement(k)
				if n := count(t, d, `SELECT count(*) FROM seller.agreement_acceptances WHERE store_id = $1 AND kind = $2 AND version = $3 AND account_id = $4`, st.StoreID, string(k), cur.Version, st.AccountID); n != 1 {
					t.Fatalf("store %d 的 %s 紀錄 %d 筆，want 1", st.StoreID, k, n)
				}
			}
		}
	}
}

// 重跑 dev-seed 重建同一組店家，連同它們的瀏覽彙總一起清掉：留著會讓 product_views_daily 的 FK 擋住刪除，或在 FK 之前留下孤兒。
func TestDevSeedRerunLeavesNoOrphanedViewRows(t *testing.T) {
	ctx := context.Background()
	d := devSeedEnv(t)
	if err := runDevSeed(ctx, nil, new(bytes.Buffer)); err != nil {
		t.Fatalf("dev-seed：%v", err)
	}
	storeID := devSeedStores()[0].StoreID
	if _, err := d.Pool().Exec(ctx, `INSERT INTO seller.product_views_daily (store_id, day, views) VALUES ($1, CURRENT_DATE, 3)`, storeID); err != nil {
		t.Fatal(err)
	}
	if err := runDevSeed(ctx, nil, new(bytes.Buffer)); err != nil {
		t.Fatalf("第二次 dev-seed：%v", err)
	}
	if n := count(t, d, `SELECT count(*) FROM seller.product_views_daily v WHERE NOT EXISTS (SELECT 1 FROM seller.stores s WHERE s.store_id = v.store_id)`); n != 0 {
		t.Fatalf("有 %d 列瀏覽彙總的店家不存在", n)
	}
	if n := count(t, d, `SELECT count(*) FROM seller.stores`); n != len(devSeedStores()) {
		t.Fatalf("seller.stores 有 %d 列，want %d", n, len(devSeedStores()))
	}
}

// newAccount 以固定 ID 建立帳號（identity.accounts 的主鍵是 IDENTITY）。
func newAccount(t *testing.T, d *db.DB, id int64) {
	t.Helper()
	if _, err := d.Pool().Exec(context.Background(), `INSERT INTO identity.accounts (id, clerk_user_id) OVERRIDING SYSTEM VALUE VALUES ($1, $2)`, id, "dev_account_"+strconv.FormatInt(id, 10)); err != nil {
		t.Fatal(err)
	}
}

// openStore 經模組的開店路徑（Apply：建店、授予 SELLER 在同一個 transaction）替帳號開一家店。
func openStore(t *testing.T, accountID int64, name string) int64 {
	t.Helper()
	c, err := openSeller(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	st, err := c.Seller.Apply(context.Background(), seller.ApplyInput{AcceptedAgreements: seller.CurrentApplyAcceptances(), AccountID: accountID, StoreName: name, ContactPhone: "0912345678", TownCode: "63000050", AddressDetail: "測試路一號"})
	if err != nil {
		t.Fatal(err)
	}
	return st.StoreID
}

func exposureVersions(t *testing.T, d *db.DB) map[int64]int64 {
	t.Helper()
	rows, err := d.Pool().Query(context.Background(), `SELECT store_id, version FROM seller.store_exposures`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[int64]int64{}
	for rows.Next() {
		var id, v int64
		if err := rows.Scan(&id, &v); err != nil {
			t.Fatal(err)
		}
		out[id] = v
	}
	return out
}

// rowJSON 是一列的完整內容（比較「完全沒動」用）。
func rowJSON(t *testing.T, d *db.DB, query string, args ...any) string {
	t.Helper()
	var s string
	if err := d.Pool().QueryRow(context.Background(), query, args...).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

// 重跑 dev-seed 是 idempotent 的 upsert：種子店家留下已解除的停權、改過資料之後回到 fixture 而且可曝光，公告版本只增不減（全量重新公告之後嚴格變大，在途的
// 舊事件版本較低，Java 才會正確丟棄）；種子以外的店家（帳號在種子區間之外，經開店路徑取得 SELLER）、它的公告列、SELLER 角色與 media ref 列
// 原封不動；IDENTITY 序號不倒退。
func TestDevSeedRerunRestoresSeedStoresAndLeavesOtherStoresUntouched(t *testing.T) {
	ctx := context.Background()
	d := devSeedEnv(t)
	fixture := devSeedStores()
	ids := []int64{fixture[0].StoreID, fixture[1].StoreID}
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := d.Pool().Exec(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	if err := runDevSeed(ctx, nil, new(bytes.Buffer)); err != nil {
		t.Fatalf("dev-seed：%v", err)
	}
	newAccount(t, d, 50001)
	userStore := openStore(t, 50001, "使用者的店")
	// 種子店家被使用過：留下已撤銷的停權措施與已結束的期間、改了店名與地址（沒有生效中的生命週期狀態：那些種子會拒絕，見
	// TestDevSeedRefusesLifecycleStateItWouldHaveToRevert）、有使用者排進來的 media ref 列。
	insertRiskSuspension(t, d, ids[0], true)
	exec(`INSERT INTO seller.suspension_periods (store_id, started_at, ended_at) VALUES ($1, now() - interval '2 hours', now() - interval '1 hour')`, ids[0])
	exec(`UPDATE seller.stores SET store_name = '被改過', town_code = '63000050', prize = '獎', version = version + 1 WHERE store_id = $1`, ids[1])
	exec(`INSERT INTO seller.media_ref_sync (ref_type, ref_id, seq, synced_seq, updated_at) VALUES ('SELLER_LOGO', $1, 3, 3, now())`, strconv.FormatInt(ids[0], 10))
	exec(`INSERT INTO seller.retired_assets (ref_type, ref_id, asset_id, retired_at) VALUES ('SELLER_LOGO', $1, 'old-asset', now())`, strconv.FormatInt(ids[0], 10))
	if err := runReannounceExposures(ctx, nil, new(bytes.Buffer)); err != nil {
		t.Fatal(err)
	}
	before := exposureVersions(t, d)
	userBefore := rowJSON(t, d, `SELECT to_jsonb(s)::text FROM seller.stores s WHERE store_id = $1`, userStore)
	roleBefore := rowJSON(t, d, `SELECT to_jsonb(r)::text FROM identity.account_roles r WHERE account_id = 50001`)
	refBefore := rowJSON(t, d, `SELECT (SELECT to_jsonb(m)::text FROM seller.media_ref_sync m LIMIT 1) || (SELECT to_jsonb(r)::text FROM seller.retired_assets r LIMIT 1)`)
	var seqBefore int64
	if err := d.Pool().QueryRow(ctx, `SELECT last_value FROM pg_sequences WHERE schemaname = 'seller' AND sequencename = 'stores_store_id_seq'`).Scan(&seqBefore); err != nil {
		t.Fatal(err)
	}

	if err := runDevSeed(ctx, nil, new(bytes.Buffer)); err != nil {
		t.Fatalf("重跑 dev-seed：%v", err)
	}
	if err := runReannounceExposures(ctx, nil, new(bytes.Buffer)); err != nil {
		t.Fatal(err)
	}

	module, closeFn, err := openSellerCommand(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer closeFn()
	got, err := module.ExposableStores(ctx, ids)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Errorf("重跑之後可曝光的種子店家 = %v，want 兩家都可曝光", got)
	}
	for _, s := range fixture {
		var name, status string
		if err := d.Pool().QueryRow(ctx, `SELECT store_name, status FROM seller.stores WHERE store_id = $1`, s.StoreID).Scan(&name, &status); err != nil {
			t.Fatal(err)
		}
		if name != s.Name || status != "ACTIVE" {
			t.Errorf("店 %d = (%s, %s)，want fixture (%s, ACTIVE)", s.StoreID, name, status, s.Name)
		}
	}
	if n := count(t, d, `SELECT count(*) FROM seller.stores WHERE store_id = $1 AND (prize IS NOT NULL OR town_code <> '67000180')`, ids[0]) + count(t, d, `SELECT count(*) FROM seller.stores WHERE store_id = $1 AND (prize IS NOT NULL OR town_code <> '10016010')`, ids[1]); n != 0 {
		t.Errorf("種子店家的欄位沒有回到 fixture（%d 家）", n)
	}
	if n := count(t, d, `SELECT count(*) FROM seller.measures WHERE store_id = ANY($1)`, ids) + count(t, d, `SELECT count(*) FROM seller.suspension_periods WHERE store_id = ANY($1)`, ids); n != 0 {
		t.Errorf("種子店家的停權沒有清掉（%d 列）", n)
	}
	after := exposureVersions(t, d)
	for _, id := range ids {
		if after[id] <= before[id] {
			t.Errorf("店 %d 的公告版本 %d，want 嚴格大於重跑前的 %d", id, after[id], before[id])
		}
	}
	if got, want := after[userStore], before[userStore]+1; got != want {
		// 全量重新公告對每一家店加一，使用者的店只因為它而變。
		t.Errorf("使用者的店的公告版本 = %d，want %d（只有全量重新公告加一）", got, want)
	}
	if now := rowJSON(t, d, `SELECT to_jsonb(s)::text FROM seller.stores s WHERE store_id = $1`, userStore); now != userBefore {
		t.Errorf("使用者的店被改動：\n之前 %s\n之後 %s", userBefore, now)
	}
	if now := rowJSON(t, d, `SELECT to_jsonb(r)::text FROM identity.account_roles r WHERE account_id = 50001`); now != roleBefore {
		t.Errorf("使用者的 SELLER 角色被改動：%s → %s", roleBefore, now)
	}
	if now := rowJSON(t, d, `SELECT (SELECT to_jsonb(m)::text FROM seller.media_ref_sync m LIMIT 1) || (SELECT to_jsonb(r)::text FROM seller.retired_assets r LIMIT 1)`); now != refBefore {
		t.Errorf("media ref 列被改動：%s → %s", refBefore, now)
	}
	if n := count(t, d, `SELECT count(*) FROM seller.stores`); n != 3 {
		t.Errorf("seller.stores 有 %d 列，want 2 家種子加 1 家使用者的店", n)
	}
	var seqAfter, next int64
	if err := d.Pool().QueryRow(ctx, `SELECT last_value FROM pg_sequences WHERE schemaname = 'seller' AND sequencename = 'stores_store_id_seq'`).Scan(&seqAfter); err != nil {
		t.Fatal(err)
	}
	if seqAfter < seqBefore {
		t.Errorf("IDENTITY 序號倒退：%d → %d", seqBefore, seqAfter)
	}
	if err := d.Pool().QueryRow(ctx, `SELECT nextval(pg_get_serial_sequence('seller.stores', 'store_id'))`).Scan(&next); err != nil {
		t.Fatal(err)
	}
	if next <= userStore || next <= 9999 {
		t.Errorf("下一個 store_id = %d，會重用 %d 或落在固定 ID 之內", next, userStore)
	}
}

// 01-account.sql 重建的帳號（這裡是 3001~3099）擁有種子以外的店家時，前提檢查失敗並說明處理方式，dev-seed 本身也拒絕；兩者都不寫、不刪任何資料
// （不自動刪店：刪了店，帳號重建後的 SELLER 角色沒有對應的店，反過來不刪，重建帳號會讓店家失去角色）。
func TestDevSeedCheckRefusesRebuiltAccountsThatOwnOtherStores(t *testing.T) {
	ctx := context.Background()
	d := devSeedEnv(t)
	if err := runDevSeed(ctx, []string{"-check"}, new(bytes.Buffer)); err != nil {
		t.Fatalf("乾淨的資料庫上前提檢查應該成立：%v", err)
	}
	newAccount(t, d, 3002)
	other := openStore(t, 3002, "開店的買家")
	if err := runDevSeed(ctx, nil, new(bytes.Buffer)); err == nil {
		t.Fatal("區間內有種子以外的店家時 dev-seed 不該寫入")
	}
	err := runDevSeed(ctx, []string{"-check"}, new(bytes.Buffer))
	if err == nil {
		t.Fatal("前提檢查應該失敗")
	}
	for _, want := range []string{"店家 " + strconv.FormatInt(other, 10), "帳號 3002", "重建本機資料庫", "dev_<名稱>"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("錯誤訊息缺少 %q：%v", want, err)
		}
	}
	if n := count(t, d, `SELECT count(*) FROM seller.stores`); n != 1 {
		t.Errorf("seller.stores 有 %d 列，want 只剩使用者的 1 家（沒有寫入、沒有刪除）", n)
	}
	if n := count(t, d, `SELECT count(*) FROM identity.account_roles WHERE account_id = 3002 AND source_id = $1`, other); n != 1 {
		t.Errorf("SELLER 角色被動過")
	}
	// 佔用種子 store_code 的非種子店家同樣擋下（unique 衝突會讓 upsert 失敗在半途）。
	if _, err := d.Pool().Exec(ctx, `UPDATE seller.stores SET account_id = 60000, store_code = 9 WHERE store_id = $1`, other); err != nil {
		t.Fatal(err)
	}
	if err := runDevSeed(ctx, []string{"-check"}, new(bytes.Buffer)); err == nil || !strings.Contains(err.Error(), "store_code 9") {
		t.Fatalf("佔用種子 store_code 的店家應該被擋下：%v", err)
	}
}

func seedOK(t *testing.T) {
	t.Helper()
	if err := runDevSeed(context.Background(), nil, new(bytes.Buffer)); err != nil {
		t.Fatalf("dev-seed：%v", err)
	}
}

func mustExec(t *testing.T, d *db.DB, query string, args ...any) {
	t.Helper()
	if _, err := d.Pool().Exec(context.Background(), query, args...); err != nil {
		t.Fatalf("%s：%v", query, err)
	}
}

// insertRiskSuspension 寫一個 RISK 停權措施（沒有固定結束時間）；revoked 為 true 時是已撤銷的。
func insertRiskSuspension(t *testing.T, d *db.DB, storeID int64, revoked bool) {
	t.Helper()
	status := "ENFORCED"
	if revoked {
		status = "REVOKED"
	}
	mustExec(t, d, `WITH m AS (
			INSERT INTO seller.measures (id, account_id, store_id, kind, basis, risk_ground, review_by, statement_to_seller, current_revision, created_by_admin_id, created_at, updated_at, starts_at, status, revoked_reason, revoked_by_admin_id, revoked_at)
			SELECT gen_random_uuid(), account_id, store_id, 'SUSPENSION', 'RISK', 'FRAUD', now() + interval '7 days', 's', 1, 1, now() - interval '2 hours', now() - interval '2 hours', now() - interval '2 hours', $2::text,
				CASE WHEN $2::text = 'REVOKED' THEN 'OTHER' END, CASE WHEN $2::text = 'REVOKED' THEN 1::bigint END, CASE WHEN $2::text = 'REVOKED' THEN now() - interval '1 hour' END
			FROM seller.stores WHERE store_id = $1 RETURNING id)
		INSERT INTO seller.measure_revisions (measure_id, revision, statement_to_seller, admin_id, created_at)
		SELECT id, 1, 's', 1, now() - interval '2 hours' FROM m`, storeID, status)
}

// insertTermination 寫一個生效中的終止合作措施（契約依據，沒有結束時間）。
func insertTermination(t *testing.T, d *db.DB, storeID int64) {
	t.Helper()
	mustExec(t, d, `WITH m AS (
			INSERT INTO seller.measures (id, account_id, store_id, kind, basis, termination_ground, statement_to_seller, current_revision, created_by_admin_id, created_at, updated_at, starts_at)
			SELECT gen_random_uuid(), account_id, store_id, 'TERMINATION', 'CONTRACT', 'CEASED_BUSINESS', 's', 1, 1, now() - interval '2 hours', now() - interval '2 hours', now() - interval '2 hours'
			FROM seller.stores WHERE store_id = $1 RETURNING id)
		INSERT INTO seller.measure_revisions (measure_id, revision, statement_to_seller, admin_id, created_at)
		SELECT id, 1, 's', 1, now() - interval '2 hours' FROM m`, storeID)
}

// insertViolationMeasure 在一個 transaction 內寫一個依據認定的措施（認定與措施互相指向，約束可延遲到 commit）、它的版本與內部註記；
// withAppeal 時再加一筆待處理的申訴與它的一張佐證。
func insertViolationMeasure(t *testing.T, d *db.DB, storeID int64, id, kind string, withAppeal bool) {
	t.Helper()
	ctx := context.Background()
	tx, err := d.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := tx.Exec(ctx, query, args...); err != nil {
			t.Fatalf("%s：%v", query, err)
		}
	}
	exec(`SET CONSTRAINTS ALL DEFERRED`)
	exec(`INSERT INTO seller.violations (id, store_id, account_id, category, severity, origin_measure_id, created_by_admin_id, created_at, updated_at)
		SELECT $2::uuid, store_id, account_id, 'FRAUD', 'LOW', $2::uuid, 1, now(), now() FROM seller.stores WHERE store_id = $1`, storeID, id)
	exec(`INSERT INTO seller.measures (id, account_id, store_id, kind, basis, lead_finding_id, statement_to_seller, ends_at, current_revision, created_by_admin_id, created_at, updated_at, starts_at)
		SELECT $2::uuid, account_id, store_id, $3::text, 'VIOLATION', $2::uuid, 's', CASE WHEN $3::text = 'SUSPENSION' THEN now() + interval '1 day' END, 1, 1, now() - interval '1 hour', now() - interval '1 hour', now() - interval '1 hour' FROM seller.stores WHERE store_id = $1`, storeID, id, kind)
	exec(`INSERT INTO seller.measure_findings (measure_id, violation_id) VALUES ($1, $1)`, id)
	exec(`INSERT INTO seller.measure_revisions (measure_id, revision, statement_to_seller, ends_at, admin_id, created_at)
		VALUES ($1, 1, 's', CASE WHEN $2::text = 'SUSPENSION' THEN now() + interval '1 day' END, 1, now() - interval '1 hour')`, id, kind)
	exec(`INSERT INTO seller.governance_notes (note_id, measure_id, admin_id, body, created_at) VALUES (gen_random_uuid(), $1, 1, 'n', now())`, id)
	if withAppeal {
		exec(`INSERT INTO seller.measure_requests (id, measure_id, kind, state, reason, revision, submitted_at, basis)
			SELECT $1::uuid, $1::uuid, 'APPEAL', 'PENDING', 'r', 1, now(), basis FROM seller.measures WHERE id = $1`, id)
		exec(`INSERT INTO seller.measure_request_evidence (request_id, position, asset_id, object_key) VALUES ($1, 0, 'appeal-asset-1', 'secure/a1')`, id)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

// 種子不悄悄還原有下游消費者的生命週期狀態：停權與處分、到期後還沒結束的停權期間、等待刪除、關閉、進行中的刪帳申請，前提檢查與寫入都拒絕、不寫任何資料
// （店家列原封不動），錯誤訊息說明怎麼處理。mutation：拿掉 lifecycleStatesSQL 的停權那一段，「生效中的停權」這一列測試失敗。
func TestDevSeedRefusesLifecycleStateItWouldHaveToRevert(t *testing.T) {
	ctx := context.Background()
	ids := []int64{devSeedStores()[0].StoreID, devSeedStores()[1].StoreID}
	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, d *db.DB)
		want  []string
	}{
		{"風險停權措施", func(t *testing.T, d *db.DB) {
			insertRiskSuspension(t, d, ids[0], false)
		}, []string{"種子帳號 9", "生效中的停權措施（RISK）", "EndMeasure"}},
		{"依據認定的停權措施", func(t *testing.T, d *db.DB) {
			insertViolationMeasure(t, d, ids[1], "00000000-0000-7000-8000-000000000001", "SUSPENSION", false)
		}, []string{"種子帳號 4", "生效中的停權措施（VIOLATION）", "RevokeMeasure"}},
		{"生效中的終止合作", func(t *testing.T, d *db.DB) {
			insertTermination(t, d, ids[0])
		}, []string{"種子帳號 9", "生效中的終止合作措施", "RevokeMeasure"}},
		{"進行中的停權期間", func(t *testing.T, d *db.DB) {
			mustExec(t, d, `INSERT INTO seller.suspension_periods (store_id, started_at) VALUES ($1, now())`, ids[0])
		}, []string{"種子帳號名下的店家 1009", "進行中的停權期間"}},
		{"等待刪除", func(t *testing.T, d *db.DB) {
			mustExec(t, d, `UPDATE seller.stores SET status = 'PENDING_DELETION' WHERE store_id = $1`, ids[1])
		}, []string{"種子帳號名下的店家 1004", "店家狀態 PENDING_DELETION", "重建本機資料庫"}},
		{"已關閉", func(t *testing.T, d *db.DB) {
			closeStoreRow(t, d, ids[0])
		}, []string{"種子帳號名下的店家 1009", "店家狀態 CLOSED"}},
		{"進行中的刪帳申請", func(t *testing.T, d *db.DB) {
			mustExec(t, d, `INSERT INTO accountdeletion.deletion_requests (id, account_id, status, requested_at) VALUES (gen_random_uuid(), 4, 'PENDING_ORDERS', now())`)
		}, []string{"帳號 [4]", "刪帳申請"}},
		{"非種子的重建帳號有進行中的刪帳申請", func(t *testing.T, d *db.DB) {
			mustExec(t, d, `INSERT INTO accountdeletion.deletion_requests (id, account_id, status, requested_at) VALUES (gen_random_uuid(), 2001, 'PENDING_ORDERS', now())`)
		}, []string{"帳號 [2001]", "刪帳申請"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := devSeedEnv(t)
			seedOK(t)
			tc.setup(t, d)
			before := rowJSON(t, d, `SELECT jsonb_agg(to_jsonb(s) ORDER BY store_id)::text FROM seller.stores s`)
			for _, args := range [][]string{{"-check"}, nil} {
				err := runDevSeed(ctx, args, new(bytes.Buffer))
				if err == nil {
					t.Fatalf("dev-seed %v 應該拒絕", args)
				}
				for _, w := range tc.want {
					if !strings.Contains(err.Error(), w) {
						t.Errorf("dev-seed %v 的錯誤缺少 %q：%v", args, w, err)
					}
				}
			}
			if now := rowJSON(t, d, `SELECT jsonb_agg(to_jsonb(s) ORDER BY store_id)::text FROM seller.stores s`); now != before {
				t.Errorf("被拒絕之後店家列被改動：\n之前 %s\n之後 %s", before, now)
			}
		})
	}
}

// 處分是帳號層級的：種子帳號在別的（已關閉）任期上有生效中的停權與期間，種子也拒絕（否則會替被停權的帳號新開任期）。
// mutation：把 lifecycleStatesSQL 兩段的 `OR ... account_id = ANY($3)` 拿掉，兩個子測試失敗。
func TestDevSeedRefusesSanctionOnAnotherStintOfASeedAccount(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct{ name, insert, want string }{
		{"停權", "", "生效中的停權措施（RISK）"},
		{"停權期間", `INSERT INTO seller.suspension_periods (store_id, started_at) VALUES ($1, now())`, "進行中的停權期間"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := devSeedEnv(t)
			seedOK(t)
			newAccount(t, d, 3005)
			old := openStore(t, 3005, "舊任期")
			closeStoreRow(t, d, old)
			mustExec(t, d, `UPDATE seller.stores SET account_id = 9 WHERE store_id = $1`, old)
			if tc.insert == "" {
				insertRiskSuspension(t, d, old, false)
			} else {
				mustExec(t, d, tc.insert, old)
			}
			for _, args := range [][]string{{"-check"}, nil} {
				err := runDevSeed(ctx, args, new(bytes.Buffer))
				if err == nil {
					t.Fatalf("dev-seed %v 應該拒絕", args)
				}
				who := "種子帳號名下的店家 " + strconv.FormatInt(old, 10)
				if tc.insert == "" {
					who = "種子帳號 9"
				}
				for _, w := range []string{who, tc.want} {
					if !strings.Contains(err.Error(), w) {
						t.Errorf("dev-seed %v 的錯誤缺少 %q：%v", args, w, err)
					}
				}
			}
		})
	}
}

// closeStoreRow 把店家列直接設成已關閉（關閉的同一個 transaction 會移除 SELLER 角色，這裡一併刪）：測試的前置，不經 CloseCurrent（它要問訂單服務）。
func closeStoreRow(t *testing.T, d *db.DB, storeID int64) {
	t.Helper()
	mustExec(t, d, `WITH c AS (INSERT INTO seller.store_closures (store_id, closed_at, closed_by) VALUES ($1, '2026-01-01T00:00:00Z', 'ACCOUNT_OWNER') RETURNING closed_at)
		UPDATE seller.stores SET status = 'CLOSED', current_closed_at = (SELECT closed_at FROM c), logo_key = NULL, logo_asset_id = NULL, cover_key = NULL, cover_asset_id = NULL
		WHERE store_id = $1`, storeID)
	mustExec(t, d, `DELETE FROM identity.account_roles WHERE source_id = $1`, storeID)
}

// 01 重建的帳號擁有的「已關閉」非種子店家不擋前提檢查（角色已移除，01 動不到它），未關閉的仍擋；種子照常執行、已關閉的店家原封不動。
func TestDevSeedCheckAllowsAClosedStoreOfARebuiltAccount(t *testing.T) {
	ctx := context.Background()
	d := devSeedEnv(t)
	seedOK(t)
	newAccount(t, d, 3003)
	closed := openStore(t, 3003, "開過又關掉的店")
	closeStoreRow(t, d, closed)
	if err := runDevSeed(ctx, []string{"-check"}, new(bytes.Buffer)); err != nil {
		t.Fatalf("已關閉的非種子店家不該擋前提檢查：%v", err)
	}
	before := rowJSON(t, d, `SELECT to_jsonb(s)::text FROM seller.stores s WHERE store_id = $1`, closed)
	seedOK(t)
	if now := rowJSON(t, d, `SELECT to_jsonb(s)::text FROM seller.stores s WHERE store_id = $1`, closed); now != before {
		t.Errorf("已關閉的店家被改動：\n之前 %s\n之後 %s", before, now)
	}
	// 同一個帳號重新開了一家未關閉的店，就擋下。
	reopened := openStore(t, 3003, "重新入駐")
	err := runDevSeed(ctx, []string{"-check"}, new(bytes.Buffer))
	if err == nil || !strings.Contains(err.Error(), "店家 "+strconv.FormatInt(reopened, 10)) || strings.Contains(err.Error(), "店家 "+strconv.FormatInt(closed, 10)) {
		t.Fatalf("只該擋未關閉的 %d，不該列已關閉的 %d：%v", reopened, closed, err)
	}
}

func profileChangedEvents(t *testing.T, d *db.DB, storeID int64) int {
	t.Helper()
	return count(t, d, `SELECT count(*) FROM river.river_job WHERE kind = 'legacy.seller_store_profile_changed' AND args->>'store_id' = $1`, strconv.FormatInt(storeID, 10))
}

// 種子還原 profile 時發 StoreProfileChanged（與模組自己寫 profile 同一個事件，經 relay 變成給舊服務的 seller.profile.changed）；profile 沒變的重跑不發。
// mutation：不發事件，第二段的 +1 測試失敗。
func TestDevSeedPublishesTheProfileChangedEventWhenItRestoresTheProfile(t *testing.T) {
	d := devSeedEnv(t)
	seedOK(t)
	a, b := devSeedStores()[0].StoreID, devSeedStores()[1].StoreID
	if profileChangedEvents(t, d, a) == 0 {
		t.Fatalf("第一次寫入（新建的列）也要發事件；river_job 的 kind 或 args 欄位名變了？\n%s", rowJSON(t, d, `SELECT coalesce(jsonb_agg(DISTINCT kind)::text, '[]') FROM river.river_job`))
	}
	baseA, baseB := profileChangedEvents(t, d, a), profileChangedEvents(t, d, b)
	seedOK(t)
	if got := profileChangedEvents(t, d, a); got != baseA {
		t.Errorf("profile 沒變的重跑不該發事件：1009 的事件 %d → %d", baseA, got)
	}
	mustExec(t, d, `UPDATE seller.stores SET store_name = '被改過', prize = '獎' WHERE store_id = $1`, a)
	seedOK(t)
	if got := profileChangedEvents(t, d, a); got != baseA+1 {
		t.Errorf("還原 profile 要發一個 StoreProfileChanged：1009 的事件 %d → %d，want %d", baseA, got, baseA+1)
	}
	if got := profileChangedEvents(t, d, b); got != baseB {
		t.Errorf("沒被改的 1004 不該有事件：%d → %d", baseB, got)
	}
}

// 種子刪掉的 KYC 證件、通報與申訴的佐證圖片，先經模組的退場機制記成 retired_assets 並遞增 ref 的 seq（排同步 job），media 的引用才釋放；
// 業務列照樣被刪、種子不留孤兒。
func TestDevSeedRetiresTheAssetsOfTheEvidenceItDeletes(t *testing.T) {
	d := devSeedEnv(t)
	seedOK(t)
	id := devSeedStores()[0].StoreID
	app := rowJSON(t, d, `SELECT application_id::text FROM seller.kyc_applications WHERE store_id = $1`, id)
	mustExec(t, d, `INSERT INTO seller.kyc_documents (application_id, slot, position, asset_id, object_key) VALUES ($1, 'ID_CARD_FRONT', 0, 'kyc-asset-1', 'secure/k1')`, app)
	mustExec(t, d, `INSERT INTO seller.reports (id, store_id, category, description, reporter_buyer_id, created_at, updated_at)
		VALUES ('00000000-0000-7000-8000-0000000000a1', $1, 'OTHER', 'd', 7, now(), now())`, id)
	mustExec(t, d, `INSERT INTO seller.report_evidence (report_id, submission_seq, position, asset_id, object_key) VALUES ('00000000-0000-7000-8000-0000000000a1', 0, 0, 'report-asset-1', 'secure/r1')`)
	insertViolationMeasure(t, d, id, "00000000-0000-7000-8000-0000000000b1", "WARNING", true)
	seedOK(t)
	for _, tc := range []struct{ refType, refID, asset string }{
		{media.RefTypeCertification, app, "kyc-asset-1"},
		{media.RefTypeGovernanceEvidence, "00000000-0000-7000-8000-0000000000a1", "report-asset-1"},
		{media.RefTypeGovernanceEvidence, "00000000-0000-7000-8000-0000000000b1", "appeal-asset-1"},
	} {
		if n := count(t, d, `SELECT count(*) FROM seller.retired_assets WHERE ref_type = $1 AND ref_id = $2 AND asset_id = $3 AND unlinked_at IS NULL`, tc.refType, tc.refID, tc.asset); n != 1 {
			t.Errorf("%s/%s 的資產 %s 沒有記成退場", tc.refType, tc.refID, tc.asset)
		}
		if n := count(t, d, `SELECT count(*) FROM seller.media_ref_sync WHERE ref_type = $1 AND ref_id = $2 AND seq > synced_seq`, tc.refType, tc.refID); n != 1 {
			t.Errorf("%s/%s 的 ref 沒有排入同步（seq 沒有遞增）", tc.refType, tc.refID)
		}
	}
	if n := count(t, d, `SELECT (SELECT count(*) FROM seller.report_evidence) + (SELECT count(*) FROM seller.measure_request_evidence) + (SELECT count(*) FROM seller.reports) + (SELECT count(*) FROM seller.violations)
		+ (SELECT count(*) FROM seller.measures) + (SELECT count(*) FROM seller.measure_requests) + (SELECT count(*) FROM seller.measure_revisions) + (SELECT count(*) FROM seller.governance_notes) + (SELECT count(*) FROM seller.measure_findings) + (SELECT count(*) FROM seller.kyc_documents)`); n != 0 {
		t.Errorf("使用痕跡沒有刪乾淨：%d 列", n)
	}
	if n := count(t, d, `SELECT count(*) FROM river.river_job WHERE kind = 'seller.media_sync'`); n < 3 {
		t.Errorf("seller.media_sync job 有 %d 個，want 至少 3 個（每個 ref 一個）", n)
	}
}

// 已經退場的資產不能重新成為目前的 logo（current 與 retired 不相交）：店家沒有 logo 時補 fixture 的，fixture 的資產 ID 若已在 logo ref 的退場名單，換一個沒用過的。
func TestDevSeedLogoNeverReusesARetiredAsset(t *testing.T) {
	d := devSeedEnv(t)
	seedOK(t)
	st := devSeedStores()[0]
	mustExec(t, d, `UPDATE seller.stores SET logo_key = NULL, logo_asset_id = NULL WHERE store_id = $1`, st.StoreID)
	mustExec(t, d, `INSERT INTO seller.retired_assets (ref_type, ref_id, asset_id, retired_at) VALUES ($1, $2, $3, now())`, media.RefTypeSellerLogo, strconv.FormatInt(st.StoreID, 10), st.LogoAssetID)
	seedOK(t)
	got := rowJSON(t, d, `SELECT logo_asset_id FROM seller.stores WHERE store_id = $1`, st.StoreID)
	if got == st.LogoAssetID || got == "" {
		t.Fatalf("logo 資產 = %q，不該重用已退場的 %q", got, st.LogoAssetID)
	}
	if n := count(t, d, `SELECT count(*) FROM seller.stores s JOIN seller.retired_assets r ON r.ref_type = $1 AND r.ref_id = s.store_id::text AND r.asset_id = s.logo_asset_id`, media.RefTypeSellerLogo); n != 0 {
		t.Errorf("%d 家店的目前 logo 在退場名單裡", n)
	}
}

// PayUni 的 intent_seq 重跑不重來、只增不減：在途的綁定 job 以舊的 seq 記結果，新的註冊的 seq 一定比它大，job 會再跑一輪，不會剛好對上。
func TestDevSeedKeepsPayUniIntentSeqMonotonic(t *testing.T) {
	d := devSeedEnv(t)
	seedOK(t)
	seq := func(id int64) int64 {
		var n int64
		if err := d.Pool().QueryRow(context.Background(), `SELECT intent_seq FROM seller.payuni_registrations WHERE store_id = $1`, id).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	for _, st := range devSeedStores() {
		mustExec(t, d, `UPDATE seller.payuni_registrations SET intent_seq = 7 WHERE store_id = $1`, st.StoreID)
		prev := int64(7)
		for run := 0; run < 3; run++ {
			seedOK(t)
			got := seq(st.StoreID)
			if got <= prev {
				t.Fatalf("店 %d 第 %d 次重跑後 intent_seq = %d，want 大於 %d", st.StoreID, run+1, got, prev)
			}
			if synced := rowJSON(t, d, `SELECT synced_seq::text FROM seller.payuni_registrations WHERE store_id = $1`, st.StoreID); synced != strconv.FormatInt(got, 10) {
				t.Errorf("店 %d 的 synced_seq = %s，want 等於 intent_seq %d（種子的註冊已綁定）", st.StoreID, synced, got)
			}
			prev = got
		}
	}
}

// dev-seed 只在本機開發的環境執行：沒有 DEV_AUTH_KEY_FILE，或同時設了正式的 Clerk、Cloudflare Access，都不寫任何資料。
func TestDevSeedRefusesAnEnvironmentThatIsNotLocalDevelopment(t *testing.T) {
	ctx := context.Background()
	for name, setup := range map[string]func(t *testing.T){
		"沒有 DEV_AUTH_KEY_FILE": func(t *testing.T) { t.Setenv("DEV_AUTH_KEY_FILE", "") },
		"DEV_AUTH_KEY_FILE 不是金鑰": func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "not-a-key")
			if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("DEV_AUTH_KEY_FILE", path)
		},
		"正式的 Clerk":               func(t *testing.T) { t.Setenv("CLERK_SECRET_KEY", "sk_live_x") },
		"Cloudflare Access":       func(t *testing.T) { t.Setenv("CF_ACCESS_TEAM_DOMAIN", "example.cloudflareaccess.com") },
		"Cloudflare Access 的 AUD": func(t *testing.T) { t.Setenv("CF_ACCESS_ADMIN_AUD", "aud") },
	} {
		t.Run(name, func(t *testing.T) {
			d := devSeedEnv(t)
			setup(t)
			if err := runDevSeed(ctx, nil, new(bytes.Buffer)); err == nil {
				t.Fatal("非本機開發的環境不該能執行 dev-seed")
			}
			if n := count(t, d, `SELECT count(*) FROM seller.stores`); n != 0 {
				t.Fatalf("被拒絕之後仍寫了 %d 家店", n)
			}
		})
	}
}
