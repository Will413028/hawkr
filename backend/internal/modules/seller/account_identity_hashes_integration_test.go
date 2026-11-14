//go:build integration

package seller_test

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/seal"
)

// 防詐保存（Stripe 計畫 D3、D42）：統編的 hash 寫進 account_identity_hashes，不隨 184 天的個資清理刪除；
// 之後才建立的處分，快照取得到已清理的舊 stint 的統編，同一個統編仍然不能在別的帳號通過認證。
// mutation：SnapshotMeasureIdentities 改成從申請的明文統編取 → 清理後快照列數是 0、新帳號通過認證；writeHashes 不寫保存表 → 清理前就沒有列；
// 清理連保存表一起刪 → 清理後沒有列。
func TestIdentityHashesOutliveThePIIPurgeAndFeedLaterSnapshots(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	account, s := f.loadedStore(t, true)
	if n := f.count(t, "SELECT count(*) FROM seller.account_identity_hashes WHERE source_store_id = $1 AND account_id = $2 AND field = 'TAX_ID'", s.StoreID, account); n != 1 {
		t.Fatalf("統編一把金鑰一列：%d", n)
	}
	_, err := f.module.CloseCurrent(ctx, account)
	must(t, err)
	f.now = f.now.Add(StoreRetention + time.Second)
	if n, err := f.module.PurgePII(ctx); err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.account_identity_hashes WHERE source_store_id = $1", s.StoreID); n != 1 {
		t.Fatalf("防詐保存的 hash 不隨個資清理刪除：%d", n)
	}
	// 清理之後才建立的處分：快照含舊 stint 的統編。
	m := f.terminate(t, s.StoreID, GroundFraud)
	if n := f.count(t, "SELECT count(*) FROM seller.measure_identities WHERE measure_id = $1 AND kind = 'TAX_ID'", m.ID); n != 1 {
		t.Fatalf("快照要含已清理的舊 stint 的統編：%d", n)
	}
	other, _ := f.open(t)
	d := f.savedDraft(t, companyDraft(other, taxIDOne))
	if _, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: other, ExpectedVersion: d.Version}); err == nil {
		t.Fatal("同一個統編不該通過認證")
	}
}

// 來源 stint 關店滿 5 年才刪：差一小時不刪，滿了刪；營業中的 stint 不論多久都不刪；被主管機關來函的保存擋下的不刪。
// mutation：清理 job 不刪（拿掉第三階段）→ 滿 5 年仍有列；時間條件少一年或寫成 StoreRetention → 差一小時就被刪；不看關店（只看建立時間）→ 營業中的 stint 被刪。
func TestIdentityHashesExpireFiveYearsAfterTheStintCloses(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	account, closed := f.loadedStore(t, true)
	f.submittedCompany(t, taxIDTwo)
	openStore := func() int64 {
		return f.count(t, "SELECT count(*) FROM seller.account_identity_hashes a JOIN seller.stores s ON s.store_id = a.source_store_id WHERE s.status = 'ACTIVE'")
	}
	if openStore() == 0 {
		t.Fatal("營業中的店也要有防詐保存的列")
	}
	_, err := f.module.CloseCurrent(ctx, account)
	must(t, err)
	closedAt := f.now
	rows := func() int64 {
		return f.count(t, "SELECT count(*) FROM seller.account_identity_hashes WHERE source_store_id = $1", closed.StoreID)
	}
	f.now = closedAt.Add(StoreRetention + time.Second)
	if _, err := f.module.PurgePII(ctx); err != nil {
		t.Fatal(err)
	}
	f.now = closedAt.Add(IdentityHashRetention - time.Hour)
	if _, err := f.module.PurgePII(ctx); err != nil {
		t.Fatal(err)
	}
	if rows() != 1 {
		t.Fatalf("關店滿 5 年之前不刪：%d", rows())
	}
	f.now = closedAt.Add(IdentityHashRetention + time.Hour)
	if _, err := f.module.PurgePII(ctx); err != nil {
		t.Fatal(err)
	}
	if rows() != 0 {
		t.Fatalf("關店滿 5 年要刪：%d", rows())
	}
	if openStore() == 0 {
		t.Fatal("營業中的 stint 的列不能被刪")
	}
}

// 編輯草稿時同步修剪：改掉統編之後舊值的防詐列消失、新值的列齊全；清空之後沒有列。
// mutation：syncHashes 不呼叫 PruneAccountIdentityHashes → 舊值的列還在；Prune 的 keep 條件拿掉 → 同 stint 其他申請持有的值也被刪。
func TestAccountIdentityHashesFollowDraftEdits(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _ := f.open(t)
	draft := f.savedDraft(t, companyDraft(accountID, taxIDOne))
	rows := func() int64 {
		return f.count(t, "SELECT count(*) FROM seller.account_identity_hashes WHERE account_id = $1 AND field = 'TAX_ID'", accountID)
	}
	if rows() != 1 {
		t.Fatalf("寫入時一列：%d", rows())
	}
	before := f.identityHashRows(t, "TAX_ID")
	edited, err := f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: draft.Version, TaxID: ptr(taxIDTwo)})
	must(t, err)
	after := f.identityHashRows(t, "TAX_ID")
	if rows() != 1 || len(after) != 1 || after["test1"] == before["test1"] {
		t.Fatalf("舊值的列要被修剪，只剩新值的一列：before=%v after=%v", before, after)
	}
	_, err = f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: edited.Version, TaxID: ptr("")})
	must(t, err)
	if rows() != 0 {
		t.Fatalf("清空欄位之後沒有列：%d", rows())
	}
}

// 啟動檢查涵蓋防詐保存的金鑰：個資清理之後申請、快照都不參照金鑰，只剩 account_identity_hashes 參照 HASH:test1；金鑰被拿掉時要失敗。
// mutation：ReferencedKeyIDs 拿掉 account_identity_hashes 那一行 → 金鑰被拿掉也通過。
func TestStartupKeyCheckCoversTheAccountIdentityHashes(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	account, _ := f.loadedStore(t, true)
	_, err := f.module.CloseCurrent(ctx, account)
	must(t, err)
	f.now = f.now.Add(StoreRetention + time.Second)
	if n, err := f.module.PurgePII(ctx); err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.measure_identities"); n != 0 {
		t.Fatalf("前提：只剩防詐保存參照金鑰：%d", n)
	}
	if err := f.module.CheckKYCKeys(ctx); err != nil {
		t.Fatalf("同一組金鑰：%v", err)
	}
	newMAC, err := seal.ParseMAC("rot2:" + base64.StdEncoding.EncodeToString([]byte("rotated-kyc-hash-hmac-key-0000002")))
	must(t, err)
	err = f.newModuleWithKeys(t, f.jobs, newMAC).CheckKYCKeys(ctx)
	if err == nil || !strings.Contains(err.Error(), "HASH:test1") {
		t.Fatalf("防詐保存參照的舊 HMAC 金鑰被拿掉時要失敗：%v", err)
	}
}
