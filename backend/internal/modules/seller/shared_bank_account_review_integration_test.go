//go:build integration

package seller_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"uuid"

	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// 共用銀行帳號提醒（Stripe 計畫 D45、步驟 8d）：待辦 SHARED_BANK_ACCOUNT 由 seller.review_shared_bank_account(at) 推導，成員是處分生效中、
// 處分建立之後才同步進來的同一個 fingerprint 命中的別的帳號；只存「已檢視」的處置（measure_linked_reviews）。

const sharedBankKind = "SHARED_BANK_ACCOUNT"

// sharedBankItems 回傳 f.now 這一刻 pub_review_items 裡的 SHARED_BANK_ACCOUNT 列。
func (f *fixture) sharedBankItems(t *testing.T) []reviewRow {
	t.Helper()
	var out []reviewRow
	for _, r := range f.pubReviewItems(t, f.now) {
		if r.Kind == sharedBankKind {
			out = append(out, r)
		}
	}
	return out
}

// firstFingerprintAt 是一個店家最早的 BANK_FINGERPRINT 寫入時間（待辦開始時間的預期值；列的 created_at 是同步當下的時鐘）。
func (f *fixture) firstFingerprintAt(t *testing.T, storeID int64) time.Time {
	t.Helper()
	var at time.Time
	must(t, f.db.Conn(context.Background()).QueryRow(context.Background(),
		"SELECT min(created_at) FROM seller.account_identity_hashes WHERE source_store_id = $1 AND field = 'BANK_FINGERPRINT'", storeID).Scan(&at))
	return at
}

// sanctionedThenShared 建立被處分的帳號 S（fingerprint 先於處分就已同步）、處分 M（終止合作），隔一小時之後帳號 O 才同步同一個 fingerprint。
func (f *fixture) sanctionedThenShared(t *testing.T) (m Measure, accountO int64, sO Store) {
	t.Helper()
	_, sS := f.approvedWithFingerprints(t, fpShared)
	f.tick(time.Hour)
	m = f.terminate(t, sS.StoreID, GroundFraud)
	f.tick(time.Hour)
	accountO, sO = f.approvedWithFingerprints(t, fpShared)
	return m, accountO, sO
}

// 處分建立之後才同步的同一個 fingerprint 產生一筆：種類、對象的鍵、店家、開始（那筆 fingerprint 的寫入時間）與期限（168 小時）。
// 重複同步不增加；金鑰環兩把金鑰（每個 fingerprint 兩列）仍是一筆；不同的 fingerprint 不產生；列的 item_id 同種類內唯一。
// mutation：拿掉 hash 比對（view 的 f.hash = mi.hash）→ 不同 fingerprint 的帳號 N 也出現，筆數不符；
// 把 GROUP BY 的 account_id 拿掉或不分組 → 兩把金鑰出現兩列；開始時間改 max → 開始時間斷言失敗。
func TestSharedBankAccountIsQueuedForAFingerprintSyncedAfterTheSanction(t *testing.T) {
	t.Parallel()
	f := setup(t)
	f.module = f.newModuleWithKeys(t, f.jobs, rotatedMAC(t))
	m, accountO, sO := f.sanctionedThenShared(t)
	f.approvedWithFingerprints(t, fpOther)
	if n := f.fingerprintRows(t, sO.StoreID); n != 2 {
		t.Fatalf("前提：一個 fingerprint 兩把金鑰兩列：%d", n)
	}
	items := f.sharedBankItems(t)
	if len(items) != 1 {
		t.Fatalf("只有共用 fingerprint 的 O 產生一筆，N 的 fingerprint 不同：%+v", items)
	}
	got, start := items[0], f.firstFingerprintAt(t, sO.StoreID)
	if got.ItemID != m.ID.String()+"/"+strconv.FormatInt(accountO, 10) || got.MeasureID == nil || *got.MeasureID != m.ID || got.AccountID == nil || *got.AccountID != accountO ||
		got.StoreID == nil || *got.StoreID != sO.StoreID || got.StoreName == nil || got.Start == nil || !got.Start.Equal(start) || got.Due == nil || !got.Due.Equal(start.Add(168*time.Hour)) {
		t.Fatalf("列：%+v 預期店家 %d 開始 %s", got, sO.StoreID, start)
	}
	// 重複同步（同一個 fingerprint 再同步一輪）不增加。
	f.tick(time.Hour)
	snap := enabledSnapshot
	snap.BankFingerprints = []string{fpShared}
	f.setPayment(t, sO.StoreID, snap)
	if again := f.sharedBankItems(t); len(again) != 1 || again[0].ItemID != got.ItemID || !again[0].Start.Equal(start) {
		t.Fatalf("重複同步：%+v", again)
	}
}

// 處分建立之前就有的 fingerprint 不進待辦（處分頁的關聯帳號已看得到），但關聯帳號仍列出它，而且不在待辦。
// mutation：拿掉 h.created_at > m.created_at → 這裡出現一筆。
func TestSharedBankAccountExistingBeforeTheSanctionIsNotQueued(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, sS := f.approvedWithFingerprints(t, fpShared)
	f.tick(time.Hour)
	accountO, _ := f.approvedWithFingerprints(t, fpShared)
	f.tick(time.Hour)
	m := f.terminate(t, sS.StoreID, GroundFraud)
	f.tick(time.Hour)
	if items := f.sharedBankItems(t); len(items) != 0 {
		t.Fatalf("處分建立前就有的 fingerprint：%+v", items)
	}
	linked := f.measureOf(t, m.ID).LinkedAccounts
	if len(linked) != 1 || linked[0].AccountID != accountO || linked[0].SharedBankAccountPending || linked[0].ReviewedAt != nil {
		t.Fatalf("關聯帳號仍列出，但不在待辦：%+v", linked)
	}
}

// 處分頁的關聯帳號列帶「在待辦」；標為已檢視之後離開待辦、帶已檢視時間；重複標記成功、不改原來的時間與管理員。
// 不是命中的帳號與不存在的措施回 NotFound（不寫入）；處分撤銷之後待辦消失。
// mutation：拿掉 NOT EXISTS measure_linked_reviews → 標記後仍在待辦；標記改成 ON CONFLICT DO UPDATE → 時間被改；
// 拿掉 BankFingerprintHitExists → 對沒命中的帳號也寫入而不是 NotFound。
func TestSharedBankAccountLeavesTheQueueOnlyWhenMarkedReviewedOrNoLongerApplicable(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	m, accountO, _ := f.sanctionedThenShared(t)
	accountN, _ := f.approvedWithFingerprints(t, fpOther)
	if items := f.sharedBankItems(t); len(items) != 1 {
		t.Fatalf("前提：待辦一筆：%+v", items)
	}
	if l := f.measureOf(t, m.ID).LinkedAccounts; len(l) != 1 || !l[0].SharedBankAccountPending || l[0].ReviewedAt != nil {
		t.Fatalf("關聯帳號列要帶在待辦：%+v", l)
	}
	for name, in := range map[string]MarkLinkedAccountReviewedInput{
		"帳號沒有命中":   {MeasureID: m.ID, AccountID: accountN, AdminID: 9},
		"措施不存在":    {MeasureID: uuid.NewV7(), AccountID: accountO, AdminID: 9},
		"被處分的帳號本身": {MeasureID: m.ID, AccountID: m.AccountID, AdminID: 9},
	} {
		if err := f.module.MarkLinkedAccountReviewed(ctx, in); apperr.KindOf(err) != apperr.KindNotFound {
			t.Errorf("%s：%v", name, err)
		}
	}
	if n := f.count(t, "SELECT count(*) FROM seller.measure_linked_reviews"); n != 0 {
		t.Fatalf("NotFound 不寫入：%d", n)
	}
	f.tick(time.Hour)
	first := f.now
	must(t, f.module.MarkLinkedAccountReviewed(ctx, MarkLinkedAccountReviewedInput{MeasureID: m.ID, AccountID: accountO, AdminID: 9}))
	if items := f.sharedBankItems(t); len(items) != 0 {
		t.Fatalf("標為已檢視後消失：%+v", items)
	}
	f.tick(time.Hour)
	must(t, f.module.MarkLinkedAccountReviewed(ctx, MarkLinkedAccountReviewedInput{MeasureID: m.ID, AccountID: accountO, AdminID: 10}))
	var at time.Time
	var admin int64
	must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT reviewed_at, reviewed_by_admin_id FROM seller.measure_linked_reviews WHERE measure_id = $1 AND account_id = $2", m.ID, accountO).Scan(&at, &admin))
	if !at.Equal(first) || admin != 9 {
		t.Fatalf("重複標記不改原來的時間與管理員：%s %d，預期 %s 9", at, admin, first)
	}
	l := f.measureOf(t, m.ID).LinkedAccounts
	if len(l) != 1 || l[0].SharedBankAccountPending || l[0].ReviewedAt == nil || !l[0].ReviewedAt.Equal(first) {
		t.Fatalf("已檢視的關聯帳號列：%+v", l)
	}
}

// 處分撤銷之後待辦消失（處分不在 effective_measures）；命中的帳號自己被處分時不列（它已在處分之下）。
// mutation：拿掉 effective_measures(at) 的 join → 撤銷後仍在；拿掉 NOT EXISTS (… x.account_id = h.account_id) → O 被停權後仍在。
func TestSharedBankAccountDisappearsWhenTheSanctionIsRevokedOrTheHitAccountIsSanctioned(t *testing.T) {
	t.Parallel()
	f := setup(t)
	m, _, sO := f.sanctionedThenShared(t)
	if items := f.sharedBankItems(t); len(items) != 1 {
		t.Fatalf("前提：待辦一筆：%+v", items)
	}
	own := f.suspend(t, sO.StoreID)
	if items := f.sharedBankItems(t); len(items) != 0 {
		t.Fatalf("命中的帳號自己被處分時不列：%+v", items)
	}
	f.revoke(t, own)
	if items := f.sharedBankItems(t); len(items) != 1 {
		t.Fatalf("它的處分撤銷後回到待辦（還沒檢視）：%+v", items)
	}
	f.revoke(t, m)
	if items := f.sharedBankItems(t); len(items) != 0 {
		t.Fatalf("處分撤銷後消失：%+v", items)
	}
}

// 命中的店家取帳號最近的 stint（store_id 最大的，含已關閉）：與處分頁關聯帳號的店家（CurrentStore，沒有時 LatestStoreOfAccount）是同一家。
// mutation：SQL 取最早的 stint（ORDER BY s.store_id）或用 fingerprint 的來源 stint → 兩邊不同家，斷言失敗。
func TestSharedBankAccountStoreIsTheSameAsTheLinkedAccountRow(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, sS := f.approvedWithFingerprints(t, fpShared)
	f.tick(time.Hour)
	m := f.terminate(t, sS.StoreID, GroundFraud)
	f.tick(time.Hour)
	accountO, firstStint := f.approvedWithFingerprints(t, fpShared)
	f.tick(time.Hour)
	_, err := f.module.CloseCurrent(ctx, accountO)
	must(t, err)
	second, err := f.module.Apply(ctx, validApply(accountO))
	must(t, err)
	f.setContact(t, second.StoreID)
	items := f.sharedBankItems(t)
	linked := f.measureOf(t, m.ID).LinkedAccounts
	if len(items) != 1 || len(linked) != 1 {
		t.Fatalf("items=%+v linked=%+v", items, linked)
	}
	if second.StoreID == firstStint.StoreID || items[0].StoreID == nil || *items[0].StoreID != second.StoreID || linked[0].StoreID != second.StoreID {
		t.Fatalf("待辦的店家 %s、關聯帳號的店家 %d，預期都是最近的 stint %d（不是 %d）", ptrStr(items[0].StoreID), linked[0].StoreID, second.StoreID, firstStint.StoreID)
	}
}
