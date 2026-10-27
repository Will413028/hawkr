//go:build integration

package seller_test

import (
	"context"
	"testing"
	"time"

	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
)

// 關店後的兩段清理（ADR 2026-10-19-seller-kyc-payuni-consolidated 的 2026-10-20 修訂）：
// 第一段（退場）在關店後 StoreRetireAfter（30 天）：說明、獎項、網站、logo 與封面清掉，發 StoreRetired（relay 轉成 seller.deleted，下游刪除），
// 店名、聯絡、地址、KYC 與 PayUni 留著；第二段在關店後 StoreRetention（184 天）清註冊、身分與連線資料。兩段的邊界都是「嚴格晚於」。
// mutation：RetireStores 多清 store_name 或 contact_*（或 StoreRetireAfter 改成 4416 小時）→ 第一段的斷言失敗；
// 第二段的 hours 改傳 StoreRetireAfterHours → 31 天就清個資，斷言失敗。
func TestRetirementAtThirtyDaysAndPersonalDataPurgeAtOneHundredEightyFourDays(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.exposable(t)
	// 公開內容直接寫進 fixture（PayUni 已收下店名與介紹，UpdateProfile 會鎖住它們）。
	_, err := f.db.Conn(ctx).Exec(ctx, `UPDATE seller.stores SET description='自家種的蔬果', prize='優良農民', website_url='https://farm.example.invalid' WHERE store_id=$1`, s.StoreID)
	must(t, err)
	retired := record[StoreRetired](f)
	_, err = f.module.CloseCurrent(ctx, accountID)
	must(t, err)
	closedAt := f.now

	f.now = closedAt.Add(StoreRetireAfter)
	if _, err := f.module.PurgePII(ctx); err != nil || f.retired(t, s.StoreID) || len(retired.all()) != 0 {
		t.Fatalf("剛好 30 天不退場：err=%v retired=%v", err, f.retired(t, s.StoreID))
	}
	f.now = closedAt.Add(StoreRetireAfter + time.Second)
	n, err := f.module.PurgePII(ctx)
	must(t, err)
	if n != 0 || !f.retired(t, s.StoreID) || f.purged(t, s.StoreID) {
		t.Fatalf("30 天後一秒退場、不清個資：n=%d retired=%v purged=%v", n, f.retired(t, s.StoreID), f.purged(t, s.StoreID))
	}
	row := f.row(t, "stores", "store_id", s.StoreID)
	for _, c := range []string{"description", "prize", "website_url", "logo_key", "logo_asset_id", "cover_key", "cover_asset_id"} {
		if row[c] != nil {
			t.Errorf("退場要清掉公開內容 %s：%v", c, row[c])
		}
	}
	for _, c := range []string{"store_name", "contact_phone", "contact_name", "contact_mobile", "support_email", "town_code", "address_detail"} {
		if row[c] == nil {
			t.Errorf("退場不清註冊資料 %s", c)
		}
	}
	if f.row(t, "kyc_applications", "store_id", s.StoreID)["real_name"] == nil {
		t.Error("退場不清 KYC")
	}
	if events := retired.all(); len(events) != 1 || events[0].StoreID != s.StoreID || !events[0].OccurredAt.Equal(f.now) {
		t.Fatalf("退場發一次 StoreRetired：%+v", events)
	}

	f.now = closedAt.Add(StoreRetention)
	if n, err := f.module.PurgePII(ctx); err != nil || n != 0 || f.purged(t, s.StoreID) {
		t.Fatalf("剛好 184 天不清個資：n=%d err=%v", n, err)
	}
	f.now = closedAt.Add(StoreRetention + time.Second)
	if n, err := f.module.PurgePII(ctx); err != nil || n != 1 || !f.purged(t, s.StoreID) {
		t.Fatalf("184 天後一秒清個資：n=%d err=%v", n, err)
	}
	row = f.row(t, "stores", "store_id", s.StoreID)
	for _, c := range []string{"store_name", "contact_phone", "contact_name", "contact_mobile", "support_email", "town_code", "address_detail"} {
		if row[c] != nil {
			t.Errorf("第二段要清掉 %s", c)
		}
	}
	if f.row(t, "kyc_applications", "store_id", s.StoreID)["real_name"] != nil {
		t.Error("第二段要清 KYC")
	}
	if len(retired.all()) != 1 {
		t.Fatalf("StoreRetired 只在退場發一次：%d", len(retired.all()))
	}
}

// 資料庫的保底：退場與個資清理的欄位約束、第二段一定在第一段之後、已退場的店家沒有圖片。
func TestRetirementAndPurgeSchemaConstraints(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.exposable(t)
	_, err := f.module.CloseCurrent(ctx, accountID)
	must(t, err)
	for name, c := range map[string]struct{ constraint, sql string }{
		"退場沒有清說明":  {"stores_retired_check", "UPDATE seller.stores SET retired_at=now(), logo_key=NULL, logo_asset_id=NULL, cover_key=NULL, cover_asset_id=NULL, prize=NULL, website_url=NULL, description='x' WHERE store_id=$1"},
		"退場沒有釋放圖片": {"stores_closed_images_check", "UPDATE seller.stores SET retired_at=now(), description=NULL, prize=NULL, website_url=NULL WHERE store_id=$1"},
		"沒有退場就清個資": {"stores_pii_purged_check", "UPDATE seller.stores SET pii_purged_at=now(), store_name=NULL, contact_phone=NULL, contact_name=NULL, contact_mobile=NULL, support_email=NULL, town_code=NULL, address_detail=NULL WHERE store_id=$1"},
	} {
		f.wantViolates(t, name, c.constraint, c.sql, s.StoreID)
	}
}

// 保存條件擋兩段清理（StoreHolds 是唯一定義）：
//   - 主管機關來函的保存：擋第一段（指名的店家關店 31 天仍不退場）與第二段（已退場的店家到 184 天仍不清個資）；保存期限一過、或來函撤銷登記，下一輪照常清。
//   - 申覆的 legal hold：擋第二段（已退場的店家，帳號上之後建立的終止合作有待處理的申覆）；裁定之後下一輪照常清。
//
// mutation：retireBatch 或 purgeBatch 拿掉 StoreHolds → 對應的斷言失敗；StoreHolds 的保存條件不看 revoked_at → 撤銷登記之後仍不清，斷言失敗。
func TestHoldsBlockEachStageAndReleaseWhenTheyEnd(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	heldAccount, held := f.closedStore(t)
	appealAccount, appealed := f.closedStore(t)
	closedAt := f.now
	f.now = closedAt.Add(time.Hour)
	order := f.holdOrder(t, heldAccount) // 保存到收文後 184 天：晚於兩段的時間

	f.now = closedAt.Add(StoreRetireAfter + time.Second)
	_, err := f.module.PurgePII(ctx)
	must(t, err)
	if f.retired(t, held.StoreID) || !f.retired(t, appealed.StoreID) {
		t.Fatalf("來函的保存擋第一段：held=%v appealed=%v", f.retired(t, held.StoreID), f.retired(t, appealed.StoreID))
	}
	// 已退場的店家，帳號上之後才建立的終止合作（21.3，建在已關閉的最近 stint）有待處理的申覆。
	m := f.terminate(t, appealed.StoreID, GroundOtherSerious)
	f.appeal(t, appealAccount, f.measureOf(t, m.ID), "異議")

	f.now = closedAt.Add(StoreRetention + time.Second)
	if n, err := f.module.PurgePII(ctx); err != nil || n != 0 || f.purged(t, appealed.StoreID) || f.purged(t, held.StoreID) {
		t.Fatalf("申覆的 legal hold 擋第二段、來函的保存擋兩段：n=%d err=%v", n, err)
	}
	f.decideAppeal(t, m.ID, RequestRejected, "維持")
	if n, err := f.module.PurgePII(ctx); err != nil || n != 1 || !f.purged(t, appealed.StoreID) {
		t.Fatalf("裁定之後下一輪清理：n=%d err=%v", n, err)
	}
	// 撤銷登記（登記錯誤）解除保存：同一輪先退場，再清個資。
	cur := f.orderOf(t, order.ID)
	_, err = f.module.RevokeAuthorityOrder(ctx, RevokeAuthorityOrderInput{ID: order.ID, AdminID: authorityRegistrar, ExpectedVersion: cur.Version, Reason: "帳號登記錯了"})
	must(t, err)
	if n, err := f.module.PurgePII(ctx); err != nil || n != 1 || !f.retired(t, held.StoreID) || !f.purged(t, held.StoreID) {
		t.Fatalf("撤銷登記之後保存解除：n=%d err=%v", n, err)
	}
}

// 已退場的店家不能恢復營業（下游已刪除）：因終止合作而關閉的店，退場（申覆期限之後）之前撤銷終止會恢復；退場之後撤銷只撤銷紀錄。
// 恢復與否看的是 retired_at（退場有沒有跑），不是時鐘。
// mutation：closing.ReopenPlan 不看 RetiredAt（或 ReopenStore 不帶 retired_at IS NULL）→ 已退場的店被恢復，斷言失敗。
func TestARetiredStoreIsNeverReopened(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s, m := f.terminated(t, GroundCeasedBusiness)
	deadline := m.StartsAt.Add(4416 * time.Hour) // 申覆期限（AppealWindow）
	f.now = deadline.Add(time.Second)
	_, err := f.module.PurgePII(ctx)
	must(t, err)
	if !f.retired(t, s.StoreID) {
		t.Fatal("申覆期限之後要退場")
	}
	f.revoke(t, m)
	if got := f.storeRow(t, s.StoreID); got.Status != Closed {
		t.Fatalf("已退場的店撤銷終止之後仍是關閉：%+v", got.Status)
	}
}
