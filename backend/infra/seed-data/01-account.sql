-- ============================================================================
-- Hawkr 測試資料：帳號與買家（單體的 identity、buyer schema，資料庫 hawkr）
-- ============================================================================
-- 測試帳號 ID 約定：
--   賣家帳號: 9, 4；店家（seller 模組，由 backend 的 make dev-seed（infra/seed-test-data.sh）在本檔之後 upsert 成 fixture，不碰種子以外的店家；本檔重建的帳號區間若擁有種子以外的店家，腳本在本檔之前就中止，見 make dev-seed-check）: store_id 1009, 1004，store_code 9, 4（merId HAWK9, HAWK4）
--   買家: 2001~2099
--   還沒開店的買家: 3001~3099（沒有 SELLER 角色、沒有店）
--   buyer_id: 50004、50009、52001~52099、53001~53099（見下方規則）
--
-- 建帳就是買家（ADR 2026-09-30-account-created-provisions-buyer）：帳號活著 ⇔ 買家活著，所以每個 seed 帳號（含只當
-- 賣家的 9、4）在同一個 transaction 內都有一個 buyer。買家不是 identity 的角色（W5），account_roles 只有 SELLER。
-- buyer_id 與 account_id 是不同的命名空間（設計書 44），seed 刻意讓兩者不同，互換的缺陷才藏不住：
--   buyer_id = account_id + 50000（9 → 50009、4 → 50004、2001 → 52001、2002 → 52002、3001 → 53001）。
-- 05-coupon.sql 在另一個資料庫（hawkr_coupon），查不到 buyer schema，所以直接寫這裡的 buyer_id，兩邊同一條規則。
--
-- 前置：單體的 migration 已套用（backend 的 make migrate）。帳號與買家的主鍵是 IDENTITY，固定的測試 ID 以
-- OVERRIDING SYSTEM VALUE 寫入，最後把序號推到這些範圍之後，新註冊的帳號與買家不會撞到它們。
-- 本機登入不走外部 provider，也不建假的登入身分（account_identities）：dev 使用者以帳號 ID 識別，
-- 以 make dev-token args='-account 2001' 之類的方式為帳號建 dev session。
-- 後台本機用 make dev-token args='-admin -user dev_ops'；Access 管理員以 JWT sub 對應 access_subject。
-- 已經在本機登入過（由 dev token 建出帳號）的 DB 不保證與這裡一致：改用這份 seed 前先重建本機 DB。
-- ============================================================================

BEGIN;

-- 清除舊測試資料（先刪參照 buyers、accounts 的列）
CREATE TEMP TABLE seed_accounts ON COMMIT DROP AS
SELECT id FROM identity.accounts WHERE id IN (9, 4) OR id BETWEEN 2001 AND 2099 OR id BETWEEN 3001 AND 3099;
CREATE TEMP TABLE seed_buyers ON COMMIT DROP AS
SELECT buyer_id FROM buyer.buyers WHERE account_id IN (SELECT id FROM seed_accounts)
   OR buyer_id IN (50009, 50004) OR buyer_id BETWEEN 52001 AND 52099 OR buyer_id BETWEEN 53001 AND 53099;
DELETE FROM buyer.shipping_addresses WHERE buyer_id IN (SELECT buyer_id FROM seed_buyers);
DELETE FROM buyer.favorites WHERE buyer_id IN (SELECT buyer_id FROM seed_buyers);
DELETE FROM buyer.seller_subscriptions WHERE buyer_id IN (SELECT buyer_id FROM seed_buyers);
DELETE FROM buyer.visit_history WHERE buyer_id IN (SELECT buyer_id FROM seed_buyers);
DELETE FROM buyer.restock_subscriptions WHERE buyer_id IN (SELECT buyer_id FROM seed_buyers);
DELETE FROM buyer.memberships WHERE buyer_id IN (SELECT buyer_id FROM seed_buyers);
DELETE FROM buyer.buyers WHERE buyer_id IN (SELECT buyer_id FROM seed_buyers);
DELETE FROM identity.account_roles WHERE account_id IN (SELECT id FROM seed_accounts);
DELETE FROM identity.sessions WHERE account_id IN (SELECT id FROM seed_accounts);
DELETE FROM identity.login_flows WHERE link_account_id IN (SELECT id FROM seed_accounts);
DELETE FROM identity.account_identities WHERE account_id IN (SELECT id FROM seed_accounts);
DELETE FROM identity.accounts WHERE id IN (SELECT id FROM seed_accounts);

INSERT INTO identity.accounts (id, email) OVERRIDING SYSTEM VALUE VALUES
  (9,    'seller1@hawkr.test'),
  (4,    'seller2@hawkr.test'),
  (2001, 'buyer1@hawkr.test'),
  (2002, 'buyer2@hawkr.test'),
  (3001, 'both1@hawkr.test');

INSERT INTO identity.admins (access_subject, email) VALUES ('dev_ops', 'dev_ops@dev.localhost')
ON CONFLICT (access_subject) DO NOTHING;

-- 本機的 dev 管理員是 OWNER（ADR 2026-10-04-admin-roles）；授予留下與 CLI 相同的事件，重跑 seed 不重複。
WITH granted AS (
  INSERT INTO identity.admin_roles (admin_id, role)
  SELECT id, 'OWNER' FROM identity.admins WHERE access_subject = 'dev_ops'
  ON CONFLICT (admin_id, role) DO NOTHING
  RETURNING admin_id, role
)
INSERT INTO identity.admin_role_events (admin_id, role, action, reason, source, db_user)
SELECT admin_id, role, 'GRANT', '本機 seed 的 dev 管理員', 'CLI', session_user FROM granted;

INSERT INTO buyer.buyers (buyer_id, account_id, display_name, phone, gender, city, created_at, updated_at) OVERRIDING SYSTEM VALUE VALUES
  (50009, 9,    '翠綠有機農場', NULL,         NULL,     NULL,     now(), now()),
  (50004, 4,    '澎湖鮮味水產', NULL,         NULL,     NULL,     now(), now()),
  (52001, 2001, '王小明',       '0933111222', 'MALE',   '臺北市', now(), now()),
  (52002, 2002, '林美玲',       '0933333444', 'FEMALE', '新北市', now(), now()),
  (53001, 3001, '陳大偉',       '0933555666', 'MALE',   '臺中市', now(), now());

-- 賣家角色的來源是店家（store_id，#3189 的柵欄）：9、4 的店由 dev-seed 以同樣的 store_id 寫入（cmd/hawkr/dev_seed.go，
-- store_id 與 account_id 刻意不同值，互換的缺陷才藏不住）。3001 是還沒開店的買家：SELLER 角色與店家永遠成對，開店時 seller 模組在同一個
-- transaction 建店並授予角色，所以沒有店就沒有角色（有角色沒有店是壞掉的不變量，入口回 500）。
INSERT INTO identity.account_roles (account_id, role, source_id) VALUES
  (9, 'SELLER', 1009),
  (4, 'SELLER', 1004);

-- 收件地址（每個買家最多一個預設）
INSERT INTO buyer.shipping_addresses (
  buyer_id, receiver_name, receiver_phone, town_code, address_detail, is_default, created_at, updated_at
) VALUES
  (52001, '王小明', '0933111222', '63000020', '松仁路100號12樓',      true,  now(), now()),
  (52001, '王小明', '0933111222', '63000040', '南京東路三段50號8樓', false, now(), now()),
  (52002, '林美玲', '0933333444', '65000010', '文化路一段200號5樓',  true,  now(), now()),
  (53001, '陳大偉', '0933555666', '66000060', '台灣大道四段300號',   true,  now(), now());

SELECT setval(pg_get_serial_sequence('identity.accounts', 'id'), GREATEST((SELECT max(id) FROM identity.accounts), 9999));
SELECT setval(pg_get_serial_sequence('buyer.buyers', 'buyer_id'), GREATEST((SELECT max(buyer_id) FROM buyer.buyers), 9999));

COMMIT;
