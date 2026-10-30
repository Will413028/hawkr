//go:build integration

package seller_test

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/seal"
)

// 防詐保存（Stripe 計畫 D3）：account_identity_hashes 與 kyc_identity_hashes 同一個 transaction 寫入，但不隨 184 天的個資清理刪除；
// 之後才建立的處分，快照取得到已清理的舊 stint 的身分，同一個身分仍然不能在別的帳號通過認證。
// mutation：SnapshotMeasureIdentities 改回從 kyc_identity_hashes 取 → 快照列數是 0、新帳號通過認證；writeHashes 不寫新表 → 清理前就沒有列；
// PurgeKYCIdentityHashes 連新表一起刪 → 清理後沒有列。
func TestIdentityHashesOutliveThePIIPurgeAndFeedLaterSnapshots(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	account, s := f.loadedStore(t, false, idA, bankAcct)
	if n := f.count(t, "SELECT count(*) FROM seller.account_identity_hashes WHERE source_store_id = $1 AND account_id = $2", s.StoreID, account); n != 2 {
		t.Fatalf("與 kyc_identity_hashes 同時寫入：身分證號與銀行帳號各一把金鑰共 2 列：%d", n)
	}
	_, err := f.module.CloseCurrent(ctx, account)
	must(t, err)
	f.now = f.now.Add(StoreRetention + time.Second)
	if n, err := f.module.PurgePII(ctx); err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.kyc_identity_hashes"); n != 0 {
		t.Fatalf("申請的比對 hash 隨個資清理刪除：%d", n)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.account_identity_hashes WHERE source_store_id = $1", s.StoreID); n != 2 {
		t.Fatalf("防詐保存的 hash 不隨個資清理刪除：%d", n)
	}
	// 清理之後才建立的處分：快照含舊 stint 的身分。
	m := f.terminate(t, s.StoreID, GroundFraud)
	if n := f.count(t, "SELECT count(*) FROM seller.measure_identities WHERE measure_id = $1", m.ID); n != 2 {
		t.Fatalf("快照要含已清理的舊 stint 的身分與銀行帳號：%d", n)
	}
	other, _ := f.open(t)
	d := f.savedDraft(t, kycDraft(other, idA, "444555666777"))
	if _, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: other, ExpectedVersion: d.Version}); err == nil {
		t.Fatal("同一個身分證號不該通過認證")
	}
}

// 來源 stint 關店滿 5 年才刪：差一小時不刪，滿了刪；營業中的 stint 不論多久都不刪；被主管機關來函的保存擋下的不刪。
// mutation：清理 job 不刪（拿掉第三階段）→ 滿 5 年仍有列；時間條件少一年或寫成 StoreRetention → 差一小時就被刪；不看關店（只看建立時間）→ 營業中的 stint 被刪。
func TestIdentityHashesExpireFiveYearsAfterTheStintCloses(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	account, closed := f.loadedStore(t, false, idA, bankAcct)
	f.submitted(t, idB, "999888777666")
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
	if rows() != 2 {
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

// 編輯草稿時同步修剪：改掉身分證號之後舊值的防詐列消失、新值的列齊全；沒動的欄位不受影響。
// mutation：syncHashes 不呼叫 PruneAccountIdentityHashes → 舊值的列還在；Prune 的 NOT EXISTS 拿掉 → 同 stint 其他申請持有的值也被刪。
func TestAccountIdentityHashesFollowDraftEdits(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _ := f.open(t)
	draft := f.savedDraft(t, kycDraft(accountID, idA, bankAcct))
	idRows := func() int64 {
		return f.count(t, "SELECT count(*) FROM seller.account_identity_hashes WHERE account_id = $1 AND field = 'ID_CARD_NO'", accountID)
	}
	if idRows() != 1 {
		t.Fatalf("寫入時各欄位各一列：%d", idRows())
	}
	edited, err := f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: draft.Version, IDCardNo: ptr(idB)})
	must(t, err)
	if idRows() != 1 || f.count(t, "SELECT count(*) FROM seller.account_identity_hashes WHERE account_id = $1", accountID) != 2 {
		t.Fatalf("舊值的列要被修剪，只剩新值的一列：%d", idRows())
	}
	_, err = f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: edited.Version, IDCardNo: ptr("")})
	must(t, err)
	if idRows() != 0 {
		t.Fatalf("清空欄位之後沒有列：%d", idRows())
	}
}

// 銀行帳號相同的帳號出現在關聯帳號，標成銀行帳號，只列出：不能建立關聯終止（清單 false、CreateMeasure 拒絕），也不會自動建立任何措施；
// 同時命中身分證號的帳號照舊可以建立。
// mutation：LinkedAccountHits 拿掉銀行帳號的條件 → 清單沒有帳號 O；CountsAsGround 固定 true → O 的 can_create 變 true、CreateMeasure 成功；
// 命中銀行帳號時自動建立終止 → O 的帳號有措施。
func TestBankAccountHitsAreListedButNeverTerminate(t *testing.T) {
	t.Parallel()
	f := setup(t)
	accountS, sS, _ := f.submitted(t, idA, bankAcct)
	accountO, sO, _ := f.submitted(t, idB, bankAcct)
	accountP, sP, _ := f.submitted(t, idA, "999888777666")
	source := f.terminate(t, sS.StoreID, GroundFraud)
	byAccount := map[int64]LinkedAccount{}
	for _, l := range f.measureOf(t, source.ID).LinkedAccounts {
		byAccount[l.AccountID] = l
	}
	if len(byAccount) != 2 {
		t.Fatalf("O（銀行帳號）與 P（身分證號）：%+v", byAccount)
	}
	if o := byAccount[accountO]; o.Matched != LinkedBankAccount || o.CanCreateLinkedTermination || o.TerminationEffect != nil || o.StoreID != sO.StoreID {
		t.Fatalf("銀行帳號命中只列出：%+v", o)
	}
	if p := byAccount[accountP]; p.Matched != LinkedNationalID || !p.CanCreateLinkedTermination {
		t.Fatalf("身分證號命中照舊可建立：%+v", p)
	}
	src := source.ID
	if _, err := f.createTermination(sO.StoreID, GroundLinkedAccount, func(in *CreateMeasureInput) { in.Termination.SourceMeasureID = &src }); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("只命中銀行帳號不能建立關聯終止：%v", err)
	}
	if _, err := f.createTermination(sP.StoreID, GroundLinkedAccount, func(in *CreateMeasureInput) { in.Termination.SourceMeasureID = &src }); err != nil {
		t.Fatalf("命中身分證號可以建立：%v", err)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.measures WHERE account_id = $1", accountO); n != 0 {
		t.Fatalf("銀行帳號命中不自動建立任何措施：%d", n)
	}
	_ = accountS
}

// 啟動檢查涵蓋防詐保存的金鑰：個資清理之後申請、快照都不參照金鑰，只剩 account_identity_hashes 參照 HASH:test1；金鑰被拿掉時要失敗。
// mutation：ReferencedKeyIDs 拿掉 account_identity_hashes 那一行 → 金鑰被拿掉也通過。
func TestStartupKeyCheckCoversTheAccountIdentityHashes(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	account, _ := f.loadedStore(t, false, idA, bankAcct)
	_, err := f.module.CloseCurrent(ctx, account)
	must(t, err)
	f.now = f.now.Add(StoreRetention + time.Second)
	if n, err := f.module.PurgePII(ctx); err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.kyc_identity_hashes") + f.count(t, "SELECT count(*) FROM seller.measure_identities"); n != 0 {
		t.Fatalf("前提：只剩防詐保存參照金鑰：%d", n)
	}
	if err := f.module.CheckKYCKeys(ctx); err != nil {
		t.Fatalf("同一組金鑰：%v", err)
	}
	enc, _ := rotatedKeys(t) // 加密金鑰舊的還在：失敗只可能來自 HASH
	newMAC, err := seal.ParseMAC("rot2:" + base64.StdEncoding.EncodeToString([]byte("rotated-kyc-hash-hmac-key-0000002")))
	must(t, err)
	err = f.newModuleWithKeys(t, f.payuni, f.jobs, enc, newMAC).CheckKYCKeys(ctx)
	if err == nil || !strings.Contains(err.Error(), "HASH:test1") {
		t.Fatalf("防詐保存參照的舊 HMAC 金鑰被拿掉時要失敗：%v", err)
	}
}
