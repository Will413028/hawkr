-- buyer 發布的讀取介面（pub_*）的形狀，給依賴表允許的上層模組的 sqlc 讀（它們的 sqlc.yaml 列 `../../buyer/db/pub.sql`）。
-- 這個檔只宣告形狀，不是定義：定義在 objects.sql 與手寫的 migration，契約測試（cmd/hawkr 的 TestPubContractsMatchMigrations）
-- 比對這裡的每個物件與 migration 套用後的真實物件（欄位、型別、參數與回傳，雙向）。只能有 `CREATE SCHEMA buyer` 與 `buyer.pub_*`
-- （internal/arch 的測試）。
--
-- 函式宣告成 RETURNS SETOF <pub 表>：sqlc 不型別化 RETURNS TABLE 與複合型別，宣告成表才會檢查與生成欄位。真實的 pub_inactive_buyer
-- 是複合型別；buyer_id 來自 buyers 的主鍵，一定有值，所以這裡宣告 NOT NULL。函式的 body 只是讓 sqlc 能解析，不是規則。
-- 消費端的 query 只能呼叫函式，不得直接參照 buyer.pub_inactive_buyer（真實物件不是表，執行期才失敗；internal/arch 的測試擋）。

CREATE SCHEMA buyer;

CREATE TABLE buyer.pub_inactive_buyer (
    buyer_id BIGINT NOT NULL
);

-- 此刻不是營運中的買家：正在刪除（PENDING_DELETION）與已關閉（CLOSED）。買家的狀態不隨時間變化，所以沒有時間參數。
-- 用法：上層模組在買家刪帳的同 transaction 訂閱者把這個事實複製進自己的表（social 的 inactive_authors），並用這個函式在每日對帳
-- 核對複本是否漏了：`SELECT b.buyer_id FROM buyer.pub_inactive_buyers() b WHERE NOT EXISTS (… 自己的複本 …)`；不給 trigger 或旗標的求值呼叫。
CREATE FUNCTION buyer.pub_inactive_buyers() RETURNS SETOF buyer.pub_inactive_buyer
LANGUAGE sql STABLE
AS $$ SELECT * FROM buyer.pub_inactive_buyer $$;

CREATE TABLE buyer.pub_blocked_buyer (
    blocker_buyer_id BIGINT NOT NULL,
    blocked_buyer_id BIGINT NOT NULL
);

-- 買家封鎖買家的現況：blocker_buyer_id 封鎖了 blocked_buyer_id。強一致，沒有時間參數。發起者或對象刪帳時列就消失。
-- 用法：social 的查詢以 EXISTS 檢查兩位買家之間任一方向的封鎖，例如 `EXISTS (SELECT 1 FROM buyer.pub_blocked_buyers() b WHERE
-- b.blocker_buyer_id = $1 AND b.blocked_buyer_id = $2)`。
CREATE FUNCTION buyer.pub_blocked_buyers() RETURNS SETOF buyer.pub_blocked_buyer
LANGUAGE sql STABLE
AS $$ SELECT * FROM buyer.pub_blocked_buyer $$;

CREATE TABLE buyer.pub_blocked_store (
    blocker_buyer_id BIGINT NOT NULL,
    store_id         BIGINT NOT NULL
);

-- 買家封鎖店的現況：blocker_buyer_id 封鎖了店 store_id。店和買家是兩個號碼空間，所以是獨立的函式。強一致，沒有時間參數。發起者刪帳時列就消失；
-- 店退場後列保留。用法：`EXISTS (SELECT 1 FROM buyer.pub_blocked_stores() b WHERE b.blocker_buyer_id = $1 AND b.store_id = $2)`。
CREATE FUNCTION buyer.pub_blocked_stores() RETURNS SETOF buyer.pub_blocked_store
LANGUAGE sql STABLE
AS $$ SELECT * FROM buyer.pub_blocked_store $$;
