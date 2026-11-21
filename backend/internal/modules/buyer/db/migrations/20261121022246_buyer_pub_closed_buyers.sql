-- 發布的讀取介面 pub_closed_buyers()：此刻已關閉（CLOSED）的買家，不含刪除中；給 im、crm 的讀取過濾。Atlas 社群版略過型別與函式，這支 migration 是手寫的
-- （`atlas migrate new`，單體規則 6）；定義與 objects.sql 相同（TestSchemaFileMatchesMigrations）。
CREATE TYPE buyer.pub_closed_buyer AS (buyer_id BIGINT);

CREATE FUNCTION buyer.pub_closed_buyers() RETURNS SETOF buyer.pub_closed_buyer
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT b.buyer_id
    FROM buyer.buyers b
    WHERE b.status = 'CLOSED';
END;
