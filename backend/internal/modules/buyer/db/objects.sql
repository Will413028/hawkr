-- buyer 的 view、函式與型別（單體規則 6：Atlas 只讀 schema.sql，這些以手寫的 migration 建立；schema.sql 加本檔與 migration 套用後的結果
-- 相同，TestSchemaFileMatchesMigrations）。

-- 發布的讀取介面（pub_*，形狀宣告在 db/pub.sql，契約測試比對兩者）：此刻不是營運中的買家。申請刪除起就算（買家刪帳申請不能撤回，
-- accountdeletion.Request 之後只有完成）。消費端是 social 每日對帳的 Go 查詢：核對 social 自己的 inactive_authors 複本有沒有漏（複本由
-- BuyerDeletionRequested、BuyerDeleted 的同 transaction 訂閱者寫，內容的可見性與計數看複本，不 join 這個函式）。
CREATE TYPE buyer.pub_inactive_buyer AS (buyer_id BIGINT);

CREATE FUNCTION buyer.pub_inactive_buyers() RETURNS SETOF buyer.pub_inactive_buyer
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT b.buyer_id
    FROM buyer.buyers b
    WHERE b.status <> 'ACTIVE';
END;

-- 發布的讀取介面 pub_blocked_buyers() 與 pub_blocked_stores()（形狀宣告在 db/pub.sql，契約測試比對兩者）：個人封鎖的現況，一列一個封鎖，
-- 買家封鎖買家與買家封鎖店各一個。消費端是 social 的讀取查詢（EXISTS 檢查「觀看者封鎖了店 S」「觀看者與買家 B 之間任一方向有封鎖」），
-- 強一致、不建投影。簡單的 SQL 函式，planner 內聯後走各自的主鍵與反向索引。
CREATE TYPE buyer.pub_blocked_buyer AS (blocker_buyer_id BIGINT, blocked_buyer_id BIGINT);

CREATE FUNCTION buyer.pub_blocked_buyers() RETURNS SETOF buyer.pub_blocked_buyer
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT b.blocker_buyer_id, b.blocked_buyer_id
    FROM buyer.blocked_buyers b;
END;

CREATE TYPE buyer.pub_blocked_store AS (blocker_buyer_id BIGINT, store_id BIGINT);

CREATE FUNCTION buyer.pub_blocked_stores() RETURNS SETOF buyer.pub_blocked_store
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT b.blocker_buyer_id, b.store_id
    FROM buyer.blocked_stores b;
END;
