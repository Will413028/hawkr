-- 發布的讀取介面 pub_blocked_buyers()、pub_blocked_stores()：個人封鎖的現況，給 social 的讀取查詢以 EXISTS 檢查。Atlas 社群版略過型別與函式，
-- 這支 migration 是手寫的（`atlas migrate new`，單體規則 6）；定義與 objects.sql 相同（TestSchemaFileMatchesMigrations）。
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
