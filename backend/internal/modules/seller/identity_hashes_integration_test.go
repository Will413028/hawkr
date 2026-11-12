//go:build integration

package seller_test

import (
	"context"
	"encoding/base64"
	"testing"

	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/seal"
)

// rotatedMAC 是輪替之後的 HMAC 金鑰環：新的金鑰在前、舊的留著（舊的就是 sealtest.MAC 的那一把）。
func rotatedMAC(t *testing.T) *seal.MAC {
	t.Helper()
	b64 := func(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }
	mac, err := seal.ParseMAC("rot2:" + b64("rotated-kyc-hash-hmac-key-0000002") + "," + "test1:" + b64("test-kyc-hash-hmac-key-000000001"))
	must(t, err)
	return mac
}

func (f *fixture) identityHashRows(t *testing.T, field string) map[string]string {
	t.Helper()
	ctx := context.Background()
	rows, err := f.db.Conn(ctx).Query(ctx, `SELECT key_id, encode(hash, 'hex') FROM seller.account_identity_hashes WHERE field = $1`, field)
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

func hex(b []byte) string {
	const digits = "0123456789abcdef"
	out := make([]byte, 0, len(b)*2)
	for _, c := range b {
		out = append(out, digits[c>>4], digits[c&0xf])
	}
	return string(out)
}

// 統編的比對 hash（account_identity_hashes 的 TAX_ID）金鑰環每一把金鑰各一列，而且跟著草稿的編輯：輪替之後編輯統編，每把金鑰各一列、都是新值的 hash；
// 清空之後沒有列。
// mutation：syncHashes 不刪舊列 → 編輯後舊 hash 仍在；SaveDraft 不呼叫 syncHashes → 沒有列；寫入時只用第一把金鑰（Sum 取代 SumAll）→ 列數不是 2。
func TestIdentityHashesFollowTheApplication(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _ := f.open(t)
	draft := f.savedDraft(t, companyDraft(accountID, taxIDOne))
	if got := f.identityHashRows(t, "TAX_ID"); len(got) != 1 || got["test1"] == "" {
		t.Fatalf("輪替前只有舊金鑰的列：%v", got)
	}
	mac := rotatedMAC(t)
	f.module = f.newModuleWithKeys(t, f.jobs, mac)
	edited, err := f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: draft.Version, TaxID: ptr(taxIDTwo)})
	must(t, err)
	want := map[string]string{}
	for _, h := range mac.SumAll(model.IdentityInput(model.IdentityTaxID, taxIDTwo)) {
		want[h.KeyID] = hex(h.Sum)
	}
	got := f.identityHashRows(t, "TAX_ID")
	if len(got) != 2 || got["rot2"] != want["rot2"] || got["test1"] != want["test1"] {
		t.Fatalf("編輯後每把金鑰各一列、都是新值的 hash：%v want %v", got, want)
	}
	_, err = f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: edited.Version, TaxID: ptr("")})
	must(t, err)
	if got := f.identityHashRows(t, "TAX_ID"); len(got) != 0 {
		t.Fatalf("清空欄位之後沒有列：%v", got)
	}
}

// 換鑰之後既有申請缺新金鑰的列，RehashKYCIdentities 只為缺列的申請補算（算出來的 hash 等於直接以新金鑰算的，統編取自申請的明文欄位），補完再跑什麼都不做；
// 補算之後關聯帳號的比對仍然成立。
// mutation：RehashIdentities 不補（回 0）→ 缺列斷言失敗；補算時用別的輸入 → hash 比對失敗；每次都重算所有申請 → 第二次跑的數量不是 0。
func TestRehashFillsTheKeysAddedByARotation(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, sA, _ := f.submittedCompany(t, taxIDOne)
	accountB, _, _ := f.submittedCompany(t, taxIDOne)
	mac := rotatedMAC(t)
	f.module = f.newModuleWithKeys(t, f.jobs, mac)
	if n := f.count(t, "SELECT count(*) FROM seller.account_identity_hashes WHERE key_id = 'rot2'"); n != 0 {
		t.Fatalf("輪替後既有申請還沒有新金鑰的列：%d", n)
	}
	n, err := f.module.RehashKYCIdentities(ctx)
	must(t, err)
	if n != 2 {
		t.Fatalf("補算 2 筆申請：%d", n)
	}
	want := ""
	for _, h := range mac.SumAll(model.IdentityInput(model.IdentityTaxID, taxIDOne)) {
		if h.KeyID == "rot2" {
			want = hex(h.Sum)
		}
	}
	if got := f.identityHashRows(t, "TAX_ID"); got["rot2"] != want {
		t.Fatalf("補算的 hash 要等於以新金鑰直接算的：%s want %s", got["rot2"], want)
	}
	if total := f.count(t, "SELECT count(*) FROM seller.account_identity_hashes WHERE field = 'TAX_ID'"); total != 4 {
		t.Fatalf("兩個 stint × 兩把金鑰 = 4 列：%d", total)
	}
	if again, err := f.module.RehashKYCIdentities(ctx); err != nil || again != 0 {
		t.Fatalf("補完之後再跑什麼都不做：%d %v", again, err)
	}
	source := f.terminate(t, sA.StoreID, GroundFraud)
	linked := f.measureOf(t, source.ID).LinkedAccounts
	if len(linked) != 1 || linked[0].AccountID != accountB || linked[0].Matched != LinkedUBN {
		t.Fatalf("補算之後仍列出同統編的帳號：%+v", linked)
	}
}
