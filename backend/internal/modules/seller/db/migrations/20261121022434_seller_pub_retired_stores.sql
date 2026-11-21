-- 發布的讀取介面 pub_retired_stores()：已退場（第一段清理完成、StoreRetired 已發）的店家，不含只是 CLOSED 但可恢復營業的店；給 im、crm 的讀取過濾。Atlas 社群版略過
-- 型別與函式，這支 migration 是手寫的（`atlas migrate new`，單體規則 6）；定義與 objects.sql 相同（TestSchemaFileMatchesMigrations）。
CREATE TYPE seller.pub_retired_store AS (store_id BIGINT);

CREATE FUNCTION seller.pub_retired_stores() RETURNS SETOF seller.pub_retired_store
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT s.store_id
    FROM seller.stores s
    WHERE s.retired_at IS NOT NULL;
END;
