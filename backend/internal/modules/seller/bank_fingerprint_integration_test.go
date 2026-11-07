//go:build integration

package seller_test

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/payment"
	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

const (
	fpShared = "fp_shared_bank_0001"
	fpOther  = "fp_other_bank_0002"
)

// approvedWithFingerprints 核准一家新店（身分證號與銀行帳號都不同於其他店）並讓外部帳戶回這些 fingerprint，執行同步一輪。
func (f *fixture) approvedWithFingerprints(t *testing.T, idCard, bank string, fingerprints ...string) (int64, Store) {
	t.Helper()
	accountID, s, sub := f.submitted(t, idCard, bank)
	must(t, approve(f, sub))
	snap := enabledSnapshot
	snap.BankFingerprints = fingerprints
	f.setPayment(t, s.StoreID, snap)
	return accountID, s
}

func (f *fixture) fingerprintRows(t *testing.T, storeID int64) int64 {
	t.Helper()
	return f.count(t, "SELECT count(*) FROM seller.account_identity_hashes WHERE source_store_id = $1 AND field = 'BANK_FINGERPRINT'", storeID)
}

// 兩個不同帳號的店綁同一個 provider fingerprint：同步把它寫進 account_identity_hashes，處分其中一個之後另一個出現在關聯帳號、標成銀行帳號、
// 只列出：不能建立關聯終止（清單 false、CreateMeasure 拒絕），也不自動建立措施。fingerprint 不同的店不命中。
// mutation：Sync 不呼叫 RecordBankFingerprints → 清單沒有帳號 O；LinkedAccountHits 拿掉 account_identity_hashes 那一支 → 同；
// SnapshotMeasureIdentities 不把 BANK_FINGERPRINT 對成 BANK_ACCOUNT → 同（kind 變 ID_CARD 對不上）；CountsAsGround 對銀行帳號回 true → can_create 變 true。
func TestSharedProviderBankFingerprintIsListedButNeverTerminates(t *testing.T) {
	t.Parallel()
	f := setup(t)
	accountS, sS := f.approvedWithFingerprints(t, idA, bankAcct, fpShared)
	accountO, sO := f.approvedWithFingerprints(t, idB, "999888777666", fpShared)
	accountN, _ := f.approvedWithFingerprints(t, idC, "012345678903", fpOther)
	source := f.terminate(t, sS.StoreID, GroundFraud)
	byAccount := map[int64]LinkedAccount{}
	for _, l := range f.measureOf(t, source.ID).LinkedAccounts {
		byAccount[l.AccountID] = l
	}
	if len(byAccount) != 1 {
		t.Fatalf("只有共用 fingerprint 的 O 命中（N 的 fingerprint 不同）：%+v", byAccount)
	}
	if o := byAccount[accountO]; o.Matched != LinkedBankAccount || o.CanCreateLinkedTermination || o.TerminationEffect != nil || o.StoreID != sO.StoreID {
		t.Fatalf("fingerprint 命中只列出：%+v", o)
	}
	src := source.ID
	if _, err := f.createTermination(sO.StoreID, GroundLinkedAccount, func(in *CreateMeasureInput) { in.Termination.SourceMeasureID = &src }); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("只命中 fingerprint 不能建立關聯終止：%v", err)
	}
	for _, a := range []int64{accountO, accountN} {
		if n := f.count(t, "SELECT count(*) FROM seller.measures WHERE account_id = $1", a); n != 0 {
			t.Fatalf("命中不自動建立任何措施：帳號 %d 有 %d 筆", a, n)
		}
	}
	_ = accountS
}

// 寫入的形狀：每個 fingerprint 每把 HMAC 金鑰一列；重複同步不增加列；fingerprint 之後從快照消失，列仍保留（已掛上並經 provider 驗證的事實，只由過期刪除）；
// 新的 fingerprint 另外加列；資料庫（兩張表）裡沒有 fingerprint 明文。
// 金鑰環是兩把金鑰（輪替之後），列數斷言為 2 把 × fingerprint 數。
// mutation：只用第一把金鑰寫（Sum 取代 SumAll）→ 列數 N 而不是 2N，斷言失敗；INSERT 拿掉 ON CONFLICT → 第二輪同步失敗或重複；
// Sync 在 fingerprint 消失時修剪 → 保留的那一步失敗；寫入明文 → 明文檢查失敗。
func TestBankFingerprintRowsAreIdempotentKeptAndNeverPlaintext(t *testing.T) {
	t.Parallel()
	f := setup(t)
	enc, mac := rotatedKeys(t)
	f.module = f.newModuleWithKeys(t, f.payuni, f.jobs, enc, mac)
	_, s := f.approvedWithFingerprints(t, idA, bankAcct, fpShared, fpOther)
	keys := f.count(t, "SELECT count(DISTINCT key_id) FROM seller.account_identity_hashes WHERE source_store_id = $1 AND field = 'ID_CARD_NO'", s.StoreID)
	if keys != 2 {
		t.Fatalf("前提：金鑰環兩把金鑰：%d", keys)
	}
	if n := f.fingerprintRows(t, s.StoreID); n != keys*2 {
		t.Fatalf("2 個 fingerprint × %d 把金鑰：%d", keys, n)
	}
	f.setPayment(t, s.StoreID, payment.Snapshot{Transfers: payment.CapabilityActive, PayoutsActive: true, BankFingerprints: []string{fpShared, fpOther}})
	if n := f.fingerprintRows(t, s.StoreID); n != keys*2 {
		t.Fatalf("重複同步不增加列：%d", n)
	}
	f.setPayment(t, s.StoreID, enabledSnapshot)
	if n := f.fingerprintRows(t, s.StoreID); n != keys*2 {
		t.Fatalf("fingerprint 從快照消失，列仍保留：%d", n)
	}
	f.setPayment(t, s.StoreID, payment.Snapshot{Transfers: payment.CapabilityActive, PayoutsActive: true, BankFingerprints: []string{"fp_third_bank_0003"}})
	if n := f.fingerprintRows(t, s.StoreID); n != keys*3 {
		t.Fatalf("新的 fingerprint 另外加列：%d", n)
	}
	for _, table := range []string{"account_identity_hashes", "kyc_identity_hashes"} {
		for _, fp := range []string{fpShared, fpOther, "fp_third_bank_0003"} {
			if n := f.count(t, "SELECT count(*) FROM seller."+table+" r WHERE r::text LIKE '%' || $1 || '%'", fp); n != 0 {
				t.Fatalf("%s 有 fingerprint 明文：%d", table, n)
			}
		}
	}
}

// 措施的快照含 fingerprint 的 hash（kind BANK_ACCOUNT），快照裡沒有明文。
// mutation：SnapshotMeasureIdentities 不複製 BANK_FINGERPRINT → 快照列數少；對成 ID_CARD → kind 檢查失敗。
func TestMeasureSnapshotCarriesTheBankFingerprintAsBankAccount(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, s := f.approvedWithFingerprints(t, idA, bankAcct, fpShared)
	m := f.terminate(t, s.StoreID, GroundFraud)
	if n := f.count(t, "SELECT count(*) FROM seller.measure_identities WHERE measure_id = $1 AND kind = 'BANK_ACCOUNT'", m.ID); n < 2 {
		t.Fatalf("快照要有 KYC 銀行帳號與 fingerprint 各一列（BANK_ACCOUNT）：%d", n)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.measure_identities r WHERE r::text LIKE '%' || $1 || '%'", fpShared); n != 0 {
		t.Fatalf("快照有 fingerprint 明文：%d", n)
	}
}

// fingerprint 的列與其他防詐保存的列一樣，來源 stint 關店滿 5 年才刪。
// mutation：過期刪除只刪申請的欄位（WHERE field <> 'BANK_FINGERPRINT'）→ 滿 5 年仍有列；關店之前就刪 → 營業中的檢查失敗。
func TestBankFingerprintRowsExpireWithTheStint(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	account, s := f.approvedWithFingerprints(t, idA, bankAcct, fpShared)
	_, err := f.module.CloseCurrent(ctx, account)
	must(t, err)
	closedAt := f.now
	f.now = closedAt.Add(StoreRetention + time.Second)
	if _, err := f.module.PurgePII(ctx); err != nil {
		t.Fatal(err)
	}
	f.now = closedAt.Add(IdentityHashRetention - time.Hour)
	if _, err := f.module.PurgePII(ctx); err != nil {
		t.Fatal(err)
	}
	if f.fingerprintRows(t, s.StoreID) == 0 {
		t.Fatal("關店滿 5 年之前不刪")
	}
	f.now = closedAt.Add(IdentityHashRetention + time.Hour)
	if _, err := f.module.PurgePII(ctx); err != nil {
		t.Fatal(err)
	}
	if n := f.fingerprintRows(t, s.StoreID); n != 0 {
		t.Fatalf("關店滿 5 年要刪：%d", n)
	}
}

// 銀行帳號 fingerprint 的關聯帳號反查不掃整張表：LinkedAccountHits 的 fingerprint 那一支以 account_identity_hashes 的 (hash, key_id) 索引與措施快照等值連接，
// 在上萬家店的 fingerprint 列（保存 5 年的表）之上仍走索引。
// mutation：拿掉 account_identity_hashes_hash 索引，或把連接條件改成 encode(f.hash, 'hex') = encode(mi.hash, 'hex') → 計畫出現 Seq Scan on account_identity_hashes，斷言失敗。
func TestLinkedAccountLookupUsesTheFingerprintHashIndex(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, sS := f.approvedWithFingerprints(t, idA, bankAcct, fpShared)
	accountO, _ := f.approvedWithFingerprints(t, idB, "999888777666", fpShared)
	source := f.terminate(t, sS.StoreID, GroundFraud)
	must(t, f.db.InTx(ctx, func(ctx context.Context) error {
		c := f.db.Conn(ctx)
		for _, sql := range []string{
			`INSERT INTO seller.stores (store_code, account_id, store_name, created_at, updated_at)
				SELECT 2800000000 + g, 500000 + g, 'x', now(), now() FROM generate_series(1, 20000) g`,
			`INSERT INTO seller.account_identity_hashes (source_store_id, field, key_id, hash, account_id, created_at)
				SELECT s.store_id, 'BANK_FINGERPRINT', k, decode(md5(s.store_id::text || k) || md5(k || s.store_id::text), 'hex'), s.account_id, now()
				FROM seller.stores s, (VALUES ('test1'), ('rot2')) v(k) WHERE s.store_code > 2800000000`,
		} {
			if _, err := c.Exec(ctx, sql); err != nil {
				return err
			}
		}
		return nil
	}))
	for _, table := range []string{"stores", "account_identity_hashes", "measure_identities"} {
		_, err := f.db.Conn(ctx).Exec(ctx, "ANALYZE seller."+table)
		must(t, err)
	}
	rec := &recorder{inner: f.db.Conn(ctx)}
	hits, err := store.New(rec).LinkedAccountHits(ctx, store.LinkedAccountHitsParams{MeasureID: source.ID, ExceptAccountID: sS.AccountID})
	must(t, err)
	if len(hits) != 1 || hits[0].AccountID != accountO || hits[0].Field != "BANK_FINGERPRINT" {
		t.Fatalf("hits=%+v", hits)
	}
	plan := f.explain(t, rec)
	t.Logf("LinkedAccountHits：\n%s", plan)
	if !regexp.MustCompile(`Index (Only )?Scan using account_identity_hashes_hash on account_identity_hashes`).MatchString(plan) {
		t.Errorf("要以 account_identity_hashes_hash 索引查找：\n%s", plan)
	}
	if strings.Contains(plan, "Seq Scan on account_identity_hashes") {
		t.Errorf("不得掃描整張表：\n%s", plan)
	}
}
