-- effective_measures(at) 改成單一 SELECT（A′(i) 審查修正）：UNION ALL 的兩段讓外層的帳號關聯條件推不進去，pub_exposable_stores 的每個
-- 探測都重掃全平台生效中的措施；單一 SELECT 內聯後帳號條件成為 measures_enforced_suspension 的 Index Cond。定義（生效中的集合）不變，
-- 回傳型別不變，所以 CREATE OR REPLACE 即可，pub_exposable_stores 不必重建。Atlas 社群版略過函式，這支是手寫的（`atlas migrate new`，
-- 單體規則 6）；定義與 schema.sql 相同（TestSchemaFileMatchesMigrations），註解見 schema.sql。
CREATE OR REPLACE FUNCTION seller.effective_measures(at TIMESTAMPTZ) RETURNS SETOF seller.effective_measure
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT m.id, m.account_id
    FROM seller.measures m
    WHERE m.status = 'ENFORCED' AND m.kind IN ('SUSPENSION', 'TERMINATION') AND m.starts_at <= at AND (m.ends_at IS NULL OR m.ends_at > at);
END;
