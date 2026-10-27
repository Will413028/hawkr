-- 發布的讀取介面 pub_inactive_buyers()：此刻不是營運中的買家（刪除中與已關閉），給 social 每日對帳核對自己的 inactive_authors 複本有沒有漏。Atlas 社群版略過
-- 型別與函式，這支 migration 是手寫的（`atlas migrate new`，單體規則 6）；定義與 objects.sql 相同（TestSchemaFileMatchesMigrations）。
CREATE TYPE buyer.pub_inactive_buyer AS (buyer_id BIGINT);

CREATE FUNCTION buyer.pub_inactive_buyers() RETURNS SETOF buyer.pub_inactive_buyer
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT b.buyer_id
    FROM buyer.buyers b
    WHERE b.status <> 'ACTIVE';
END;
