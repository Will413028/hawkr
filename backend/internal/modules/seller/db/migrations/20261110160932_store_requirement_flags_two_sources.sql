-- 開通閘門的兩個來源各自成一欄（Stripe 計畫 D31 Q3，步驟 7e1）：store_requirement_flags 另外輸出 kyc_approved（平台審核已核准）與 transfers_active
-- （seller 自存的轉帳能力副本），onboarding_complete 定義成兩者的 AND（語意不變：先前是同一個 EXISTS 對兩張表的 join，店家一列對一列，等價）。
-- 開通任務清單（GetMyOnboarding）與能力推導（model.CapabilitiesOf）都讀這兩欄，所以任務與閘門同源；SQL 端的 pub_exposable_stores 照舊只讀
-- onboarding_complete。新欄接在最後（CREATE OR REPLACE VIEW 只能在尾端加欄）。手寫的 migration（單體規則 6）；定義與 objects.sql 相同
-- （TestSchemaFileMatchesMigrations）。
CREATE OR REPLACE VIEW seller.store_requirement_flags AS
SELECT s.store_id,
       s.account_id,
       s.status <> 'CLOSED' AS active,
       s.status <> 'PENDING_DELETION' AS not_pending_deletion,
       (c.kyc_approved AND c.transfers_active) AS onboarding_complete,
       (s.store_name IS NOT NULL)::boolean AS has_store_name,
       (s.logo_key IS NOT NULL)::boolean AS has_logo,
       (s.town_code IS NOT NULL)::boolean AS has_town_code,
       c.kyc_approved,
       c.transfers_active
FROM seller.stores s
CROSS JOIN LATERAL (
    SELECT EXISTS (
               SELECT 1 FROM seller.kyc_applications a WHERE a.store_id = s.store_id AND a.status = 'APPROVED'
           ) AS kyc_approved,
           EXISTS (
               SELECT 1 FROM seller.payout_capabilities p WHERE p.store_id = s.store_id AND p.transfers_active
           ) AS transfers_active
) c;
