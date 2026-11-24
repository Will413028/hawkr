//go:build integration

package coupon_test

import (
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/coupon"
)

// 取代 Java 的 IssuerIdentityMigrationIT：schema 的 CHECK 與 UNIQUE 直接在 DB 層守住（BD31）。
// 欄位順序：id、折抵 amount、門檻、issuer_type、issuer_admin_id、issuer_store_id、store_id。
const insertCampaignSQL = `
INSERT INTO coupon.coupon_campaigns (id, title, discount_type, discount_amount, threshold_amount, target_type, store_id,
    issuer_type, issuer_admin_id, issuer_store_id, start_at, end_at, status, created_at, updated_at)
VALUES ($1, 'c', 'CASH', $2, $3, 'ALL', $7, $4, $5, $6, now(), now() + interval '1 day', 'ACTIVE', now(), now())`

func TestIssuerIdentityIsExclusive(t *testing.T) {
	t.Parallel()
	f := setup(t)
	id := func(n int) string { return "00000000-0000-7000-8000-00000000010" + string(rune('0'+n)) }
	if err := f.exec(insertCampaignSQL, id(1), 100, 0, "ADMIN", 1, nil, nil); err != nil {
		t.Fatalf("admin 發行（只有 issuer_admin_id）要通過：%v", err)
	}
	if err := f.exec(insertCampaignSQL, id(2), 100, 0, "SELLER", nil, storeA, storeA); err != nil {
		t.Fatalf("賣家發行（只有 issuer_store_id，店家範圍是自己的店）要通過：%v", err)
	}
	for name, args := range map[string][]any{
		"admin 同時帶 store": {id(3), 100, 0, "ADMIN", 1, storeA, nil},
		"賣家同時帶 admin":     {id(4), 100, 0, "SELLER", 1, storeA, storeA},
		"admin 沒有 id":     {id(5), 100, 0, "ADMIN", nil, nil, nil},
		"賣家沒有 store":      {id(6), 100, 0, "SELLER", nil, nil, nil},
		"未知的發行者":          {id(7), 100, 0, "SYSTEM", 1, nil, nil},
	} {
		if err := f.exec(insertCampaignSQL, args...); err == nil {
			t.Errorf("%s：要被 campaigns_issuer_check 擋下", name)
		}
	}
	wantConstraint(t, f.exec(insertCampaignSQL, id(8), 100, 0, "SELLER", nil, storeA, storeB), "campaigns_seller_scope_check")
}

func TestSchemaChecksAndUniqueKeys(t *testing.T) {
	t.Parallel()
	f := setup(t)
	c := f.campaign(func(in *coupon.CreateCampaignInput) { in.TotalLimit = 1 })
	v := f.claim(c, buyer1)

	wantConstraint(t, f.exec("UPDATE coupon.coupon_campaigns SET total_limit = 0, issued_count = 0 WHERE id = $1", c.ID), "campaigns_total_limit_check")
	wantConstraint(t, f.exec("UPDATE coupon.coupon_campaigns SET discount_amount = NULL WHERE id = $1", c.ID), "campaigns_discount_shape_check")
	wantConstraint(t, f.exec("UPDATE coupon.coupon_campaigns SET issued_count = 2 WHERE id = $1", c.ID), "campaigns_issued_count_check")
	wantConstraint(t, f.exec("UPDATE coupon.coupon_campaigns SET status = 'EXPIRED' WHERE id = $1", c.ID), "campaigns_status_check")
	wantConstraint(t, f.exec("UPDATE coupon.coupon_campaigns SET status = 'ENDED' WHERE id = $1", c.ID), "campaigns_ended_check")
	wantConstraint(t, f.exec("UPDATE coupon.coupon_campaigns SET end_at = start_at - interval '1 second' WHERE id = $1", c.ID), "campaigns_window_check")
	wantConstraint(t, f.exec("UPDATE coupon.coupon_campaigns SET target_type = 'MEMBER' WHERE id = $1", c.ID), "campaigns_store_scope_check")
	wantConstraint(t, f.exec("UPDATE coupon.coupon_campaigns SET title = ' x' WHERE id = $1", c.ID), "campaigns_title_check")

	// 券的狀態與欄位互斥：LOCKED 要有 order_no 與 locked_at，AVAILABLE 不能有。
	wantConstraint(t, f.exec("UPDATE coupon.coupon_vouchers SET status = 'LOCKED' WHERE id = $1", v.ID), "vouchers_state_check")
	wantConstraint(t, f.exec("UPDATE coupon.coupon_vouchers SET order_no = 'O1' WHERE id = $1", v.ID), "vouchers_state_check")
	wantConstraint(t, f.exec("UPDATE coupon.coupon_vouchers SET origin = 'BUY' WHERE id = $1", v.ID), "vouchers_origin_check")
	wantConstraint(t, f.exec("UPDATE coupon.coupon_vouchers SET status = 'VOIDED' WHERE id = $1", v.ID), "vouchers_state_check")

	// 每位買家在同一個活動只有一張；一筆訂單只持有一張；補償來源只能用一次。
	const insertVoucher = `INSERT INTO coupon.coupon_vouchers (id, campaign_id, buyer_id, origin, status, order_no, locked_at, created_at, updated_at)
		VALUES ($1, $2, $3, 'CLAIM', $4, $5, $6, now(), now())`
	wantConstraint(t, f.exec(insertVoucher, "00000000-0000-7000-8000-000000000201", c.ID, buyer1, "AVAILABLE", nil, nil), "vouchers_campaign_buyer_key")
	c2 := f.campaign()
	if err := f.exec(insertVoucher, "00000000-0000-7000-8000-000000000202", c2.ID, buyer1, "LOCKED", "OX", f.now()); err != nil {
		t.Fatal(err)
	}
	wantConstraint(t, f.exec(insertVoucher, "00000000-0000-7000-8000-000000000203", c2.ID, buyer2, "LOCKED", "OX", f.now()), "vouchers_order_no_key")
	const insertComp = `INSERT INTO coupon.coupon_vouchers (id, campaign_id, buyer_id, origin, source_type, source_id, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'COMPENSATION', 'AFTERSALES_CASE', 'AS-9', 'AVAILABLE', now(), now())`
	if err := f.exec(insertComp, "00000000-0000-7000-8000-000000000204", c2.ID, buyer3); err != nil {
		t.Fatal(err)
	}
	wantConstraint(t, f.exec(insertComp, "00000000-0000-7000-8000-000000000205", c.ID, buyer3), "vouchers_source_key")
	wantConstraint(t, f.exec("UPDATE coupon.coupon_vouchers SET source_id = NULL WHERE id = $1", "00000000-0000-7000-8000-000000000204"), "vouchers_source_check")

	// 沒有跨 schema 的 FK、稽核紀錄不擋刪除；稽核理由頭尾不得有空白。
	wantConstraint(t, f.exec(`INSERT INTO coupon.coupon_redemption_logs (campaign_id, action, reason, created_at) VALUES ($1, 'VOID', ' x', now())`, c.ID), "redemption_logs_reason_check")
	wantConstraint(t, f.exec(`INSERT INTO coupon.coupon_redemption_logs (campaign_id, action, created_at) VALUES ($1, 'REFUND', now())`, c.ID), "redemption_logs_action_check")
}
