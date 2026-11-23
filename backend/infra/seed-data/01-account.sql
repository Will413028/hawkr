-- ============================================================================
-- Hawkr 測試資料：帳號與買家（單體的 identity、buyer schema，資料庫 hawkr）
-- ============================================================================
-- 測試帳號 ID 約定：
--   賣家帳號: 9, 4；店家（seller 模組，由 backend 的 make dev-seed（infra/seed-test-data.sh）在本檔之後 upsert 成 fixture，不碰種子以外的店家；所有種子帳號（9、4、2001、2002、3001）若擁有種子以外的未關閉店家，腳本在本檔之前就中止，見 make dev-seed-check）: store_id 1009, 1004，store_code 9, 4（merId HAWK9, HAWK4）
--   買家: 2001~2099（保留區間，fixture 只寫 2001、2002）
--   還沒開店的買家: 3001~3099（保留區間，fixture 只寫 3001；沒有 SELLER 角色、沒有店）
--   buyer_id: 50004、50009、52001~52099、53001~53099（見下方規則）
--
-- 建帳就是買家（ADR 2026-09-30-account-created-provisions-buyer）：帳號活著 ⇔ 買家活著，所以每個 seed 帳號（含只當
-- 賣家的 9、4）在同一個 transaction 內都有一個 buyer。買家不是 identity 的角色（W5），account_roles 只有 SELLER。
-- buyer_id 與 account_id 是不同的命名空間（設計書 44），seed 刻意讓兩者不同，互換的缺陷才藏不住：
--   buyer_id = account_id + 50000（9 → 50009、4 → 50004、2001 → 52001、2002 → 52002、3001 → 53001）。
-- 05-coupon.sql 在另一個資料庫（hawkr_coupon），查不到 buyer schema，所以直接寫這裡的 buyer_id，兩邊同一條規則。
--
-- 前置：單體的 migration 已套用（backend 的 make migrate）。帳號與買家的主鍵是 IDENTITY，固定的測試 ID 以
-- OVERRIDING SYSTEM VALUE 寫入，最後把序號推到這些範圍之後（只增不減），新註冊的帳號與買家不會撞到它們。
--
-- 本檔是冪等的 upsert，不刪任何資料。開發種子的規則：冪等 upsert、只寫固定 ID 的種子實體、版本與序號只增不減、不刪非種子的資料、
-- 有下游消費者的生命週期狀態不悄悄還原（遇到就中止並說明處理方式）：
--   - 只寫固定 ID 的 fixture（帳號 9、4、2001、2002、3001、它們的買家與收件地址）；其他帳號、買家與使用者建立的列原封不動，
--     也不碰登入身分、session、收藏、追蹤、瀏覽、補貨訂閱、會員與頭像；
--   - 帳號的 email、買家的個資與收件地址收斂成 fixture：買家的版本只在欄位真的有變時加一；
--     收件地址有固定的 address_id（buyer_id * 100 + 序號），只以 address_id 認定是種子的列，沒有種子以外的地址被刪；
--   - 帳號與買家的個資變更本來就不發事件（登入更新 email、買家個資沒有事件），所以這裡也不發；
--   - 有下游消費者的生命週期狀態不悄悄還原：種子帳號停權、刪除、等待刪除，種子買家不是 ACTIVE，帳號與買家的 ID 對不上 fixture，
--     種子的賣家角色指向別的店家，種子買家已有種子以外的預設地址，或補回種子地址會讓買家的地址數超過上限時，整個 transaction 中止，沒有寫入任何資料；
--     處理方式是重建本機 DB（或改用 make dev-token 不帶 -account 建立的新帳號）。
-- 本機登入不走外部 provider，也不建假的登入身分（account_identities）：dev 使用者以帳號 ID 識別，
-- 以 make dev-token args='-account 2001' 之類的方式為帳號建 dev session。
-- 後台本機用 make dev-token args='-admin -user dev_ops'；Access 管理員以 JWT sub 對應 access_subject。
-- ============================================================================

BEGIN;

-- 買家保留區間的上界（53001~53099 的 53099）；address_id = buyer_id * 100 + 序號（1..99）。
CREATE TEMP TABLE seed_reserved (max_buyer_id BIGINT NOT NULL) ON COMMIT DROP;
INSERT INTO seed_reserved VALUES (53099);

CREATE TEMP TABLE seed_accounts (id BIGINT PRIMARY KEY, email TEXT NOT NULL, seller_store_id BIGINT) ON COMMIT DROP;
INSERT INTO seed_accounts (id, email, seller_store_id) VALUES
  (9,    'seller1@hawkr.test', 1009),
  (4,    'seller2@hawkr.test', 1004),
  (2001, 'buyer1@hawkr.test',  NULL),
  (2002, 'buyer2@hawkr.test',  NULL),
  (3001, 'both1@hawkr.test',   NULL);

CREATE TEMP TABLE seed_buyers (
  buyer_id BIGINT PRIMARY KEY, account_id BIGINT NOT NULL UNIQUE, display_name TEXT NOT NULL,
  phone TEXT, gender TEXT, city TEXT
) ON COMMIT DROP;
INSERT INTO seed_buyers (buyer_id, account_id, display_name, phone, gender, city) VALUES
  (50009, 9,    '翠綠有機農場', NULL,         NULL,     NULL),
  (50004, 4,    '澎湖鮮味水產', NULL,         NULL,     NULL),
  (52001, 2001, '王小明',       '0933111222', 'MALE',   '臺北市'),
  (52002, 2002, '林美玲',       '0933333444', 'FEMALE', '新北市'),
  (53001, 3001, '陳大偉',       '0933555666', 'MALE',   '臺中市');

-- 收件地址（每個買家最多一個預設）
CREATE TEMP TABLE seed_addresses (
  address_id BIGINT PRIMARY KEY, buyer_id BIGINT NOT NULL, receiver_name TEXT NOT NULL, receiver_phone TEXT NOT NULL,
  town_code TEXT NOT NULL, address_detail TEXT NOT NULL, is_default BOOLEAN NOT NULL
) ON COMMIT DROP;
INSERT INTO seed_addresses (address_id, buyer_id, receiver_name, receiver_phone, town_code, address_detail, is_default) VALUES
  (5200101, 52001, '王小明', '0933111222', '63000020', '松仁路100號12樓',      true),
  (5200102, 52001, '王小明', '0933111222', '63000040', '南京東路三段50號8樓', false),
  (5200201, 52002, '林美玲', '0933333444', '65000010', '文化路一段200號5樓',  true),
  (5300101, 53001, '陳大偉', '0933555666', '66000060', '台灣大道四段300號',   true);

-- 與 accountdeletion 同一個鎖順序（帳號列先於買家列）；鎖到 transaction 結束，檢查與寫入之間不被改動。
SELECT 1 FROM identity.accounts WHERE id IN (SELECT id FROM seed_accounts) ORDER BY id FOR UPDATE;
SELECT 1 FROM buyer.buyers WHERE buyer_id IN (SELECT buyer_id FROM seed_buyers) OR account_id IN (SELECT id FROM seed_accounts) ORDER BY buyer_id FOR UPDATE;

DO $$
DECLARE
  bad TEXT;
  max_addresses CONSTANT INT := 20;
BEGIN
  SELECT string_agg(a.id::text, ', ' ORDER BY a.id) INTO bad FROM identity.accounts a JOIN seed_accounts s ON s.id = a.id
  WHERE a.suspended_at IS NOT NULL OR a.deleted_at IS NOT NULL OR a.deletion_pending_at IS NOT NULL;
  IF bad IS NOT NULL THEN
    RAISE EXCEPTION '01-account.sql 中止，沒有寫入任何資料：種子帳號 % 已停權、已刪除或等待刪除；它們有下游消費者（事件、刪帳流程），種子不悄悄還原。請重建本機資料庫', bad;
  END IF;

  SELECT string_agg(b.buyer_id::text, ', ' ORDER BY b.buyer_id) INTO bad FROM buyer.buyers b
  WHERE (b.buyer_id IN (SELECT buyer_id FROM seed_buyers) OR b.account_id IN (SELECT id FROM seed_accounts))
    AND (b.status <> 'ACTIVE' OR b.deleted_at IS NOT NULL OR b.pii_cleared_at IS NOT NULL);
  IF bad IS NOT NULL THEN
    RAISE EXCEPTION '01-account.sql 中止，沒有寫入任何資料：種子買家 % 不是 ACTIVE（等待刪除或已關閉）；種子不悄悄還原。請重建本機資料庫', bad;
  END IF;

  SELECT string_agg(b.buyer_id || '→帳號' || b.account_id, ', ' ORDER BY b.buyer_id) INTO bad FROM buyer.buyers b
  WHERE (b.account_id IN (SELECT id FROM seed_accounts) OR b.buyer_id IN (SELECT buyer_id FROM seed_buyers))
    AND NOT EXISTS (SELECT 1 FROM seed_buyers s WHERE s.buyer_id = b.buyer_id AND s.account_id = b.account_id);
  IF bad IS NOT NULL THEN
    RAISE EXCEPTION '01-account.sql 中止，沒有寫入任何資料：買家 % 與 fixture 的 buyer_id = account_id + 50000 對不上（例如 dev token 建出的帳號）。請重建本機資料庫', bad;
  END IF;

  SELECT string_agg(r.account_id::text, ', ' ORDER BY r.account_id) INTO bad FROM identity.account_roles r JOIN seed_accounts s ON s.id = r.account_id
  WHERE r.role = 'SELLER' AND (s.seller_store_id IS NULL
    OR EXISTS (SELECT 1 FROM seller.stores st WHERE st.account_id = r.account_id AND st.status <> 'CLOSED' AND st.store_id <> s.seller_store_id));
  IF bad IS NOT NULL THEN
    RAISE EXCEPTION '01-account.sql 中止，沒有寫入任何資料：種子帳號 % 有 SELLER 角色，但種子沒有替它們開店，或目前的店不是種子的店家（例如 3001 這類不該有店的帳號已開店）。請重建本機資料庫', bad;
  END IF;

  SELECT string_agg(d.address_id || '（買家' || d.buyer_id || '）', ', ' ORDER BY d.address_id) INTO bad FROM buyer.shipping_addresses d
  JOIN seed_addresses a ON a.address_id = d.address_id WHERE d.buyer_id <> a.buyer_id;
  IF bad IS NOT NULL THEN
    RAISE EXCEPTION '01-account.sql 中止，沒有寫入任何資料：種子地址的固定 address_id 已被別的買家的地址佔用：%。請重建本機資料庫', bad;
  END IF;

  SELECT string_agg(DISTINCT d.buyer_id::text, ', ') INTO bad FROM buyer.shipping_addresses d
  WHERE d.is_default AND d.buyer_id IN (SELECT buyer_id FROM seed_buyers)
    AND NOT EXISTS (SELECT 1 FROM seed_addresses a WHERE a.address_id = d.address_id);
  IF bad IS NOT NULL THEN
    RAISE EXCEPTION '01-account.sql 中止，沒有寫入任何資料：種子買家 % 有不在 fixture 內的預設地址（例如舊版種子以自動編號寫入的地址，或使用者建立的地址）；種子不改它們，也不搬移。請重建本機資料庫', bad;
  END IF;

  -- 上限是買家模組的 MaxAddresses（backend/internal/modules/buyer/internal/model/model.go）；dev_seed_test.go 檢查這個常數與它一致。
  SELECT string_agg(c.buyer_id || '（補回後 ' || c.total || ' 筆）', ', ' ORDER BY c.buyer_id) INTO bad FROM (
    SELECT b.buyer_id,
           (SELECT count(*) FROM buyer.shipping_addresses d WHERE d.buyer_id = b.buyer_id)
         + (SELECT count(*) FROM seed_addresses a WHERE a.buyer_id = b.buyer_id
              AND NOT EXISTS (SELECT 1 FROM buyer.shipping_addresses d WHERE d.address_id = a.address_id)) AS total
    FROM seed_buyers b) c
  WHERE c.total > max_addresses;
  IF bad IS NOT NULL THEN
    RAISE EXCEPTION '01-account.sql 中止，沒有寫入任何資料：補回被刪的種子地址會讓種子買家 % 的地址數超過上限 % 筆。請刪除該買家一些使用者建立的地址，或重建本機資料庫', bad, max_addresses;
  END IF;
END $$;

INSERT INTO identity.accounts (id, email) OVERRIDING SYSTEM VALUE
SELECT id, email FROM seed_accounts
ON CONFLICT (id) DO UPDATE SET email = EXCLUDED.email, updated_at = now()
WHERE identity.accounts.email IS DISTINCT FROM EXCLUDED.email;

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

INSERT INTO buyer.buyers (buyer_id, account_id, display_name, phone, gender, city, created_at, updated_at) OVERRIDING SYSTEM VALUE
SELECT buyer_id, account_id, display_name, phone, gender, city, now(), now() FROM seed_buyers
ON CONFLICT (buyer_id) DO UPDATE
SET display_name = EXCLUDED.display_name, phone = EXCLUDED.phone, gender = EXCLUDED.gender, city = EXCLUDED.city,
    version = buyer.buyers.version + 1, updated_at = now()
WHERE (buyer.buyers.display_name, buyer.buyers.phone, buyer.buyers.gender, buyer.buyers.city)
      IS DISTINCT FROM (EXCLUDED.display_name, EXCLUDED.phone, EXCLUDED.gender, EXCLUDED.city);

-- 9、4 的店由 dev-seed 寫入（cmd/hawkr/dev_seed.go，
-- store_id 與 account_id 刻意不同值，互換的缺陷才藏不住）。3001 是還沒開店的買家：SELLER 角色與店家永遠成對，開店時 seller 模組在同一個
-- transaction 建店並授予角色，所以沒有店就沒有角色（有角色沒有店是壞掉的不變量，入口回 500）。
INSERT INTO identity.account_roles (account_id, role)
SELECT id, 'SELLER' FROM seed_accounts WHERE seller_store_id IS NOT NULL
ON CONFLICT (account_id, role) DO NOTHING;

-- 先把 fixture 裡不是預設的地址降級，再 upsert，預設地址的 partial UNIQUE 才不會在中途衝突。
UPDATE buyer.shipping_addresses d
SET is_default = false, version = d.version + 1, updated_at = now()
FROM seed_addresses a
WHERE a.address_id = d.address_id AND d.is_default AND NOT a.is_default;

INSERT INTO buyer.shipping_addresses (address_id, buyer_id, receiver_name, receiver_phone, town_code, address_detail, is_default, created_at, updated_at) OVERRIDING SYSTEM VALUE
SELECT address_id, buyer_id, receiver_name, receiver_phone, town_code, address_detail, is_default, now(), now() FROM seed_addresses
ON CONFLICT (address_id) DO UPDATE
SET receiver_name = EXCLUDED.receiver_name, receiver_phone = EXCLUDED.receiver_phone, town_code = EXCLUDED.town_code,
    address_detail = EXCLUDED.address_detail, is_default = EXCLUDED.is_default, version = buyer.shipping_addresses.version + 1, updated_at = now()
WHERE (buyer.shipping_addresses.receiver_name, buyer.shipping_addresses.receiver_phone, buyer.shipping_addresses.town_code,
       buyer.shipping_addresses.address_detail, buyer.shipping_addresses.is_default)
      IS DISTINCT FROM (EXCLUDED.receiver_name, EXCLUDED.receiver_phone, EXCLUDED.town_code, EXCLUDED.address_detail, EXCLUDED.is_default);

-- 序號只往前：不低於目前的 last_value（從未呼叫過是 NULL，以 0 計），也不低於表內最大的 ID 與 9999；
-- 收件地址的序號另外墊高到整個保留區間之後（買家保留區間的上界 * 100 + 99），自動產生的 ID 不會落進種子 ID 空間。
SELECT setval(seq, GREATEST(COALESCE(pg_sequence_last_value(seq::regclass), 0),
  COALESCE((SELECT max(id) FROM identity.accounts), 0), 9999))
FROM (SELECT pg_get_serial_sequence('identity.accounts', 'id') AS seq) q;
SELECT setval(seq, GREATEST(COALESCE(pg_sequence_last_value(seq::regclass), 0),
  COALESCE((SELECT max(buyer_id) FROM buyer.buyers), 0), 9999))
FROM (SELECT pg_get_serial_sequence('buyer.buyers', 'buyer_id') AS seq) q;
SELECT setval(seq, GREATEST(COALESCE(pg_sequence_last_value(seq::regclass), 0),
  COALESCE((SELECT max(address_id) FROM buyer.shipping_addresses), 0), 9999,
  (SELECT max_buyer_id * 100 + 99 FROM seed_reserved)))
FROM (SELECT pg_get_serial_sequence('buyer.shipping_addresses', 'address_id') AS seq) q;

COMMIT;
