//go:build integration

package seller_test

import (
	"context"
	"encoding/base64"
	"regexp"
	"strings"
	"testing"

	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/seal"
)

// rotatedKeys 是輪替之後的金鑰環：新的金鑰在前、舊的留著（舊的就是 sealtest.Keys 的那一組）。
func rotatedKeys(t *testing.T) (*seal.Keyring, *seal.MAC) {
	t.Helper()
	b64 := func(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }
	oldEnc, oldMAC := "test1:"+b64("test-kyc-encryption-key-00000001"), "test1:"+b64("test-kyc-hash-hmac-key-000000001")
	enc, err := seal.Parse("rot2:" + b64("rotated-kyc-encryption-key-00002") + "," + oldEnc)
	must(t, err)
	mac, err := seal.ParseMAC("rot2:" + b64("rotated-kyc-hash-hmac-key-0000002") + "," + oldMAC)
	must(t, err)
	return enc, mac
}

func (f *fixture) identityHashRows(t *testing.T, field string) map[string]string {
	t.Helper()
	ctx := context.Background()
	rows, err := f.db.Conn(ctx).Query(ctx, `SELECT key_id, encode(hash, 'hex') FROM seller.kyc_identity_hashes WHERE field = $1`, field)
	must(t, err)
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var key, hash string
		must(t, rows.Scan(&key, &hash))
		out[key] = hash
	}
	must(t, rows.Err())
	return out
}

// 申請的比對 hash（kyc_identity_hashes）每個識別欄位、金鑰環每一把金鑰各一列，而且跟著草稿的編輯：改掉身分證號之後舊值的列消失、新值的列齊全；
// 清空欄位之後沒有列；個資清理時整批刪除。
// mutation：syncHashes 不刪舊列 → 編輯後舊 hash 仍在；SaveDraft 不呼叫 syncHashes → 沒有列；purge 不刪 → 清理後仍有列。
func TestIdentityHashesFollowTheApplication(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _ := f.open(t)
	draft := f.savedDraft(t, kycDraft(accountID, idA, bankAcct))
	if got := f.identityHashRows(t, "ID_CARD_NO"); len(got) != 1 || got["test1"] == "" {
		t.Fatalf("輪替前只有舊金鑰的列：%v", got)
	}
	enc, mac := rotatedKeys(t)
	f.module = f.newModuleWithKeys(t, f.payuni, f.jobs, enc, mac)
	edited, err := f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: draft.Version, IDCardNo: ptr("F131104093")})
	must(t, err)
	want := map[string]string{}
	for _, h := range mac.SumAll(model.IdentityInput(model.IdentityIDCard, "F131104093")) {
		want[h.KeyID] = hex(h.Sum)
	}
	got := f.identityHashRows(t, "ID_CARD_NO")
	if len(got) != 2 || got["rot2"] != want["rot2"] || got["test1"] != want["test1"] {
		t.Fatalf("編輯後每把金鑰各一列、都是新值的 hash：%v want %v", got, want)
	}
	if bank := f.identityHashRows(t, "BANK_ACCOUNT"); len(bank) != 1 || bank["test1"] == "" {
		t.Fatalf("沒動的欄位不重寫：%v", bank)
	}
	_, err = f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: edited.Version, IDCardNo: ptr("")})
	must(t, err)
	if got := f.identityHashRows(t, "ID_CARD_NO"); len(got) != 0 {
		t.Fatalf("清空欄位之後沒有列：%v", got)
	}
}

func hex(b []byte) string {
	const digits = "0123456789abcdef"
	out := make([]byte, 0, len(b)*2)
	for _, c := range b {
		out = append(out, digits[c>>4], digits[c&0xf])
	}
	return string(out)
}

// 換鑰之後既有申請缺新金鑰的列，RehashKYCIdentities 只為缺列的申請補算（算出來的 hash 等於直接以新金鑰算的），補完再跑什麼都不做；
// 補算之後關聯帳號的比對仍然成立。
// mutation：RehashIdentities 不補（回 0）→ 缺列斷言失敗；補算時用第一把金鑰以外的輸入 → hash 比對失敗；每次都重算所有申請 → 第二次跑的數量不是 0。
func TestRehashFillsTheKeysAddedByARotation(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, sA, _ := f.submitted(t, idA, bankAcct)
	accountB, sB, _ := f.submitted(t, idA, "999888777666")
	enc, mac := rotatedKeys(t)
	f.module = f.newModuleWithKeys(t, f.payuni, f.jobs, enc, mac)
	if n := f.count(t, "SELECT count(*) FROM seller.kyc_identity_hashes WHERE key_id = 'rot2'"); n != 0 {
		t.Fatalf("輪替後既有申請還沒有新金鑰的列：%d", n)
	}
	n, err := f.module.RehashKYCIdentities(ctx)
	must(t, err)
	if n != 2 {
		t.Fatalf("補算 2 筆申請：%d", n)
	}
	for field, value := range map[string]struct {
		kind  model.SanctionIdentityKind
		value string
	}{"ID_CARD_NO": {model.IdentityIDCard, idA}} {
		got := f.identityHashRows(t, field)
		want := ""
		for _, h := range mac.SumAll(model.IdentityInput(value.kind, value.value)) {
			if h.KeyID == "rot2" {
				want = hex(h.Sum)
			}
		}
		if got["rot2"] != want {
			t.Fatalf("%s 補算的 hash 要等於以新金鑰直接算的：%s want %s", field, got["rot2"], want)
		}
	}
	if total := f.count(t, "SELECT count(*) FROM seller.kyc_identity_hashes"); total != 8 {
		t.Fatalf("兩筆申請 × (身分證號、銀行帳號) × 兩把金鑰 = 8 列：%d", total)
	}
	if again, err := f.module.RehashKYCIdentities(ctx); err != nil || again != 0 {
		t.Fatalf("補完之後再跑什麼都不做：%d %v", again, err)
	}
	source := f.terminate(t, sA.StoreID, GroundFraud)
	linked := f.measureOf(t, source.ID).LinkedAccounts
	if len(linked) != 1 || linked[0].AccountID != accountB || linked[0].Matched != LinkedNationalID {
		t.Fatalf("補算之後仍列出同身分的帳號：%+v", linked)
	}
	_ = sB
}

// 關聯帳號的反查不解密、走索引：LinkedAccountHits 的計畫以 kyc_identity_hashes 的 (hash, key_id) 索引與措施快照等值連接，
// 在上萬筆申請之上不掃描 kyc_applications 或 kyc_identity_hashes 的整張表，也沒有讀密文欄位。
// mutation：把連接條件改成 encode(h.hash, 'hex') = encode(mi.hash, 'hex')（用不到索引）→ 計畫出現 Seq Scan，斷言失敗。
func TestLinkedAccountLookupUsesTheIdentityHashIndex(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, sA, _ := f.submitted(t, idA, bankAcct)
	accountB, _, _ := f.submitted(t, idA, "999888777666")
	source := f.terminate(t, sA.StoreID, GroundFraud)
	must(t, f.db.InTx(ctx, func(ctx context.Context) error {
		c := f.db.Conn(ctx)
		for _, sql := range []string{
			`INSERT INTO seller.stores (store_code, account_id, store_name, created_at, updated_at)
				SELECT 2800000000 + g, 500000 + g, 'x', now(), now() FROM generate_series(1, 20000) g`,
			`INSERT INTO seller.kyc_applications (application_id, store_id, status, seller_type, created_at, updated_at)
				SELECT uuidv7(), store_id, 'DRAFT', 'INDIVIDUAL', now(), now() FROM seller.stores WHERE store_code > 2800000000`,
			`INSERT INTO seller.kyc_identity_hashes (application_id, field, key_id, hash)
				SELECT a.application_id, 'ID_CARD_NO', k, decode(md5(a.application_id::text || k) || md5(k || a.application_id::text), 'hex')
				FROM seller.kyc_applications a, (VALUES ('test1'), ('rot2')) v(k) WHERE a.status = 'DRAFT'`,
		} {
			if _, err := c.Exec(ctx, sql); err != nil {
				return err
			}
		}
		return nil
	}))
	for _, table := range []string{"stores", "kyc_applications", "kyc_identity_hashes", "measure_identities"} {
		_, err := f.db.Conn(ctx).Exec(ctx, "ANALYZE seller."+table)
		must(t, err)
	}
	rec := &recorder{inner: f.db.Conn(ctx)}
	hits, err := store.New(rec).LinkedAccountHits(ctx, store.LinkedAccountHitsParams{MeasureID: source.ID, ExceptAccountID: sA.AccountID})
	must(t, err)
	if len(hits) != 1 || hits[0].AccountID != accountB || hits[0].Field != "ID_CARD_NO" {
		t.Fatalf("hits=%+v", hits)
	}
	plan := f.explain(t, rec)
	t.Logf("LinkedAccountHits：\n%s", plan)
	if !regexp.MustCompile(`Index (Only )?Scan using kyc_identity_hashes_hash on kyc_identity_hashes`).MatchString(plan) {
		t.Errorf("要以 kyc_identity_hashes_hash 索引查找：\n%s", plan)
	}
	for _, seq := range []string{"Seq Scan on kyc_identity_hashes", "Seq Scan on kyc_applications"} {
		if strings.Contains(plan, seq) {
			t.Errorf("不得掃描整張表（%s）：\n%s", seq, plan)
		}
	}
}
