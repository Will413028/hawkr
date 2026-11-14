//go:build integration

package seller_test

import (
	"context"
	"slices"
	"strconv"
	"testing"
	"time"

	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"uuid"
)

// 命中的定義涵蓋受處分帳號目前的雜湊（Stripe 計畫 D46、D47、步驟 8e）：措施在 at 生效而且帶身分效果時，身分來源是快照加上被處分帳號
// 目前的 account_identity_hashes；處分頁的關聯帳號、待辦 SHARED_BANK_ACCOUNT 與申請送出、核准的比對讀同一份定義。

// hitAccounts 是措施在 at 的命中帳號（不含被處分帳號本身），直接讀 SQL 函式 seller.measure_identity_hits。
func (f *fixture) hitAccounts(t *testing.T, measureID uuid.UUID, exceptAccount int64, at time.Time) []int64 {
	t.Helper()
	ctx := context.Background()
	rows, err := f.db.Conn(ctx).Query(ctx,
		"SELECT DISTINCT account_id FROM seller.measure_identity_hits($1::timestamptz) WHERE measure_id = $2 AND account_id <> $3 ORDER BY 1", at, measureID, exceptAccount)
	must(t, err)
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		must(t, rows.Scan(&id))
		out = append(out, id)
	}
	must(t, rows.Err())
	return out
}

// linkedAccountIDs 是處分頁的關聯帳號（模組 API），排序後回傳。
func (f *fixture) linkedAccountIDs(t *testing.T, measureID uuid.UUID) []int64 {
	t.Helper()
	var out []int64
	for _, l := range f.measureOf(t, measureID).LinkedAccounts {
		out = append(out, l.AccountID)
	}
	slices.Sort(out)
	return out
}

// firstFingerprintAfter 是店家在 after 之後最早寫入的 BANK_FINGERPRINT 時間。
func (f *fixture) firstFingerprintAfter(t *testing.T, storeID int64, after time.Time) time.Time {
	t.Helper()
	var at time.Time
	must(t, f.db.Conn(context.Background()).QueryRow(context.Background(),
		"SELECT min(created_at) FROM seller.account_identity_hashes WHERE source_store_id = $1 AND field = 'BANK_FINGERPRINT' AND created_at > $2", storeID, after).Scan(&at))
	return at
}

// copyTaxHashes 讓 toStore 所屬的帳號「新增」fromStore 的統編 hash（寫入時間是現在）：寫入端被停權擋下時，以 SQL 直接給出 writeHashes 的結果，只驗讀取端。
func (f *fixture) copyTaxHashes(t *testing.T, fromStore, toStore int64) {
	t.Helper()
	ctx := context.Background()
	tag, err := f.db.Conn(ctx).Exec(ctx, `INSERT INTO seller.account_identity_hashes (source_store_id, field, key_id, hash, account_id, created_at)
		SELECT $2, h.field, h.key_id, h.hash, s.account_id, $3 FROM seller.account_identity_hashes h, seller.stores s
		WHERE h.source_store_id = $1 AND h.field = 'TAX_ID' AND s.store_id = $2`, fromStore, toStore, f.now)
	must(t, err)
	if tag.RowsAffected() == 0 {
		t.Fatal("前提：來源店家要有統編 hash")
	}
}

// identityBlocked 是新帳號以這個統編送出申請會不會被身分比對擋下。
func (f *fixture) identityBlocked(t *testing.T, taxID string) bool {
	t.Helper()
	accountID, _ := f.open(t)
	d := f.savedDraft(t, companyDraft(accountID, taxID))
	_, err := f.module.SubmitKYC(context.Background(), SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: d.Version})
	if err != nil && apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("送出失敗的原因不是身分比對：%v", err)
	}
	return err != nil
}

// 受處分帳號在處分之後新同步的 fingerprint，與別的帳號 O 相同：處分頁的關聯帳號列出 O，待辦恰一筆。兩種先後都成立：
// O 的 hash 比 S 新的 hash 早（命中時間是 S 新 hash 的寫入時間），或晚（命中時間是 O 的寫入時間）。新 hash 同步之前沒有命中。
// mutation：measure_identity_sources 拿掉目前 hash 那一半 → 同步之後 O 不在關聯帳號、待辦為 0；命中時間改成只取 f.created_at → 「O 較早」的開始時間斷言失敗；
// 改成 least → 「O 較晚」的開始時間斷言失敗；review_shared_bank_account 的 min 拿掉 → 重複列使筆數不符。
func TestFingerprintSyncedByTheSanctionedAccountAfterTheSanctionIsAHit(t *testing.T) {
	t.Parallel()
	for name, oFirst := range map[string]bool{"O 的 hash 較早": true, "O 的 hash 較晚": false} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := setup(t)
			_, sS := f.approvedWithFingerprints(t, fpOther)
			var accountO int64
			var sO Store
			if oFirst {
				f.tick(time.Hour)
				accountO, sO = f.approvedWithFingerprints(t, fpShared)
			}
			f.tick(time.Hour)
			m := f.terminate(t, sS.StoreID, GroundFraud)
			f.tick(time.Hour)
			if got := f.hitAccounts(t, m.ID, m.AccountID, f.now); len(got) != 0 {
				t.Fatalf("新 fingerprint 同步之前沒有命中：%v", got)
			}
			snap := enabledSnapshot
			snap.BankFingerprints = []string{fpOther, fpShared}
			f.setPayment(t, sS.StoreID, snap)
			newAt := f.firstFingerprintAfter(t, sS.StoreID, m.CreatedAt)
			f.tick(time.Hour)
			if !oFirst {
				accountO, sO = f.approvedWithFingerprints(t, fpShared)
				f.tick(time.Hour)
			}
			want := newAt
			if !oFirst {
				want = f.firstFingerprintAt(t, sO.StoreID)
			}
			linked := f.measureOf(t, m.ID).LinkedAccounts
			if len(linked) != 1 || linked[0].AccountID != accountO || linked[0].Matched != LinkedBankAccount || linked[0].CanCreateLinkedTermination {
				t.Fatalf("關聯帳號要列出 O（銀行帳號、只列出）：%+v", linked)
			}
			items := f.sharedBankItems(t)
			if len(items) != 1 {
				t.Fatalf("待辦恰一筆：%+v", items)
			}
			got := items[0]
			if got.ItemID != m.ID.String()+"/"+strconv.FormatInt(accountO, 10) || got.Start == nil || !got.Start.Equal(want) || got.Due == nil || !got.Due.Equal(want.Add(168*time.Hour)) {
				t.Fatalf("待辦列：%+v 預期開始 %s", got, want)
			}
		})
	}
}

// 受處分帳號在處分之後新增的統編 hash：別的帳號 O（已送出）與 R（草稿）有同一個統編，兩者都列為關聯帳號；R 送出與 O 核准都被擋，擋的集合與列出的相同；
// 處分撤銷之後不再擋（證明前面是身分比對擋的）。
// mutation：measure_identity_sources 拿掉目前 hash 那一半 → 關聯帳號為空、R 的送出通過；MeasureIdentityMatches 改回只讀 measure_identities → R 送出通過、O 核准通過；
// 兩者各自拿掉都會讓「擋的集合與列出的相同」失敗。
func TestTaxIDGainedByTheSanctionedAccountAfterTheSanctionListsAndBlocksTheSameAccounts(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountS, storeS := f.open(t)
	f.savedDraft(t, companyDraft(accountS, taxIDOne))
	accountO, storeO, subO := f.submittedCompany(t, taxIDTwo)
	accountR, _ := f.open(t)
	f.savedDraft(t, companyDraft(accountR, taxIDTwo))
	f.tick(time.Hour)
	m := f.suspend(t, storeS.StoreID)
	f.tick(time.Hour)
	if got := f.hitAccounts(t, m.ID, m.AccountID, f.now); len(got) != 0 {
		t.Fatalf("前提：S 只有統編一，沒有命中：%v", got)
	}
	f.copyTaxHashes(t, storeO.StoreID, storeS.StoreID)
	f.tick(time.Hour)
	listed := f.linkedAccountIDs(t, m.ID)
	wantSet := []int64{accountO, accountR}
	slices.Sort(wantSet)
	if !slices.Equal(listed, wantSet) {
		t.Fatalf("關聯帳號 %v，預期 O 與 R %v", listed, wantSet)
	}
	for _, l := range f.measureOf(t, m.ID).LinkedAccounts {
		if l.Matched != LinkedUBN {
			t.Fatalf("統編命中要標成統編：%+v", l)
		}
	}
	if err := approve(f, subO); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("O 核准要被擋：%v", err)
	}
	if got, err := f.module.AdminKYCApplication(ctx, subO.ID); err != nil || got.Status != KYCStatusSubmitted {
		t.Fatalf("O 的申請仍是已送出：%v %v", got.Status, err)
	}
	if !f.identityBlocked(t, taxIDTwo) {
		t.Fatal("新增的統編 hash 要擋下別的帳號送出")
	}
	f.revoke(t, m)
	if f.identityBlocked(t, taxIDTwo) {
		t.Fatal("處分撤銷之後不再擋")
	}
}

// 快照有、受處分帳號目前的 hash 已刪（來源 stint 關店滿 5 年由清理 job 刪）的仍命中：保存期過後身分仍比得到。
// mutation：measure_identity_sources 拿掉快照那一半 → 清理後關聯帳號為空；快照的 INSERT 不複製 → 同。
func TestSnapshotStillHitsAfterTheSanctionedAccountsHashesExpire(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountS, sS := f.approvedWithFingerprints(t, fpShared)
	accountO, _ := f.approvedWithFingerprints(t, fpShared)
	m := f.suspend(t, sS.StoreID)
	if got := f.linkedAccountIDs(t, m.ID); !slices.Equal(got, []int64{accountO}) {
		t.Fatalf("前提：處分時 O 已命中：%v", got)
	}
	_, err := f.module.CloseCurrent(ctx, accountS)
	must(t, err)
	closedAt := f.now
	f.now = closedAt.Add(StoreRetention + time.Second)
	if _, err := f.module.PurgePII(ctx); err != nil {
		t.Fatal(err)
	}
	f.now = closedAt.Add(IdentityHashRetention + time.Hour)
	if _, err := f.module.PurgePII(ctx); err != nil {
		t.Fatal(err)
	}
	if n := f.fingerprintRows(t, sS.StoreID); n != 0 {
		t.Fatalf("前提：S 的 hash 已過期刪除：%d", n)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.measure_identities WHERE measure_id = $1", m.ID); n == 0 {
		t.Fatal("前提：快照還在")
	}
	if got := f.hitAccounts(t, m.ID, m.AccountID, f.now); !slices.Equal(got, []int64{accountO}) {
		t.Fatalf("快照仍命中 O：%v", got)
	}
	if got := f.linkedAccountIDs(t, m.ID); !slices.Equal(got, []int64{accountO}) {
		t.Fatalf("關聯帳號仍列出 O：%v", got)
	}
}

// 預告終止（生效中）不帶身分效果：該帳號舊的與新的 hash 都不命中，不擋別的帳號送出，也沒有快照。
// mutation：identity_effect_measures 拿掉 NOTICE 的排除 → 目前 hash 那一半命中，關聯帳號與待辦出現、新帳號被擋；快照也會寫入，快照列數斷言失敗。
func TestEffectiveNoticeTerminationCarriesNoIdentityEffect(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, storeS, subS := f.submittedCompany(t, taxIDOne)
	must(t, approve(f, subS))
	snap := enabledSnapshot
	snap.BankFingerprints = []string{fpShared}
	f.setPayment(t, storeS.StoreID, snap)
	notice := f.terminate(t, storeS.StoreID, GroundNotice)
	f.now = f.now.Add(721 * time.Hour)
	f.orders.open = true
	f.effect(t, notice)
	if !f.effectiveAt(t, notice.AccountID, f.now) {
		t.Fatal("前提：預告終止已生效")
	}
	if n := f.count(t, "SELECT count(*) FROM seller.measure_identities WHERE measure_id = $1", notice.ID); n != 0 {
		t.Fatalf("預告終止沒有快照：%d", n)
	}
	accountR, storeR := f.open(t)
	f.savedDraft(t, companyDraft(accountR, taxIDTwo))
	f.tick(time.Hour)
	snap.BankFingerprints = []string{fpShared, fpOther}
	f.setPayment(t, storeS.StoreID, snap)
	f.copyTaxHashes(t, storeR.StoreID, storeS.StoreID)
	f.tick(time.Hour)
	f.approvedWithFingerprints(t, fpShared, fpOther)
	if got := f.hitAccounts(t, notice.ID, notice.AccountID, f.now); len(got) != 0 {
		t.Fatalf("預告終止不命中：%v", got)
	}
	if got := f.linkedAccountIDs(t, notice.ID); len(got) != 0 {
		t.Fatalf("關聯帳號為空：%v", got)
	}
	if items := f.sharedBankItems(t); len(items) != 0 {
		t.Fatalf("沒有待辦：%+v", items)
	}
	if f.identityBlocked(t, taxIDOne) || f.identityBlocked(t, taxIDTwo) {
		t.Fatal("預告終止不擋別的帳號送出（舊的與新的統編）")
	}
}

// gainedIdentities 建立被處分的帳號 S（sanction 建立處分）、處分之後 S 同步新的 fingerprint 並新增 R 的統編 hash；O 有同一個 fingerprint、R 有同一個統編。
// 回傳處分與命中的帳號。
func (f *fixture) gainedIdentities(t *testing.T, sanction func(storeID int64) Measure) (m Measure, accountO, accountR int64) {
	t.Helper()
	_, storeS, subS := f.submittedCompany(t, taxIDOne)
	must(t, approve(f, subS))
	f.setPayment(t, storeS.StoreID, enabledSnapshot)
	accountO, _ = f.approvedWithFingerprints(t, fpShared)
	accountR, storeR := f.open(t)
	f.savedDraft(t, companyDraft(accountR, taxIDTwo))
	f.tick(time.Hour)
	m = sanction(storeS.StoreID)
	f.tick(time.Hour)
	snap := enabledSnapshot
	snap.BankFingerprints = []string{fpShared}
	f.setPayment(t, storeS.StoreID, snap)
	f.copyTaxHashes(t, storeR.StoreID, storeS.StoreID)
	f.tick(time.Hour)
	return m, accountO, accountR
}

// 處分撤銷之後，目前 hash 那一半不再命中：命中為空、關聯帳號（措施已撤銷）為空、待辦消失、新帳號不被擋。
// mutation：目前 hash 那一半不經 effective_measures(at)（只看措施列）→ 撤銷之後仍命中、待辦仍在、新帳號仍被擋。
func TestCurrentHashesStopHittingWhenTheSanctionIsRevoked(t *testing.T) {
	t.Parallel()
	f := setup(t)
	m, accountO, accountR := f.gainedIdentities(t, func(storeID int64) Measure { return f.suspend(t, storeID) })
	want := []int64{accountO, accountR}
	slices.Sort(want)
	if got := f.hitAccounts(t, m.ID, m.AccountID, f.now); !slices.Equal(got, want) {
		t.Fatalf("前提：撤銷前命中 O 與 R：%v", got)
	}
	if items := f.sharedBankItems(t); len(items) != 1 {
		t.Fatalf("前提：撤銷前待辦一筆：%+v", items)
	}
	f.revoke(t, m)
	if got := f.hitAccounts(t, m.ID, m.AccountID, f.now); len(got) != 0 {
		t.Fatalf("撤銷之後不命中：%v", got)
	}
	if items := f.sharedBankItems(t); len(items) != 0 {
		t.Fatalf("撤銷之後待辦消失：%+v", items)
	}
	if f.identityBlocked(t, taxIDTwo) {
		t.Fatal("撤銷之後不擋")
	}
}

// 處分到期之後（job 還沒刪快照），兩半都不再命中：命中為空、待辦消失、新帳號不被擋，而快照列還在，證明不靠 job 清掉。
// mutation：快照那一半不經 effective_measures(at)（靠 PruneIdentities 刪）→ 到期後 S 原有 hash 的命中仍在；目前 hash 那一半不經 effective_measures(at) → 新 hash 的命中仍在。
func TestHitsStopAtExpiryWithoutWaitingForTheSnapshotCleanup(t *testing.T) {
	t.Parallel()
	f := setup(t)
	endsAt := f.now.Add(72 * time.Hour)
	m, accountO, accountR := f.gainedIdentities(t, func(storeID int64) Measure {
		return f.violate(t, storeID, SeverityHigh, MeasureSuspension, &endsAt)
	})
	want := []int64{accountO, accountR}
	slices.Sort(want)
	if got := f.hitAccounts(t, m.ID, m.AccountID, f.now); !slices.Equal(got, want) {
		t.Fatalf("前提：到期前命中 O 與 R：%v", got)
	}
	f.now = endsAt.Add(time.Minute)
	if n := f.count(t, "SELECT count(*) FROM seller.measure_identities WHERE measure_id = $1", m.ID); n == 0 {
		t.Fatal("前提：job 還沒跑，快照還在")
	}
	if got := f.hitAccounts(t, m.ID, m.AccountID, f.now); len(got) != 0 {
		t.Fatalf("到期之後不命中：%v", got)
	}
	if items := f.sharedBankItems(t); len(items) != 0 {
		t.Fatalf("到期之後待辦消失：%+v", items)
	}
	if f.identityBlocked(t, taxIDTwo) {
		t.Fatal("到期之後不擋")
	}
}
