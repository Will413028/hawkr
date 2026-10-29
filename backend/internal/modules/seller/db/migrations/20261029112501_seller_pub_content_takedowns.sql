-- 發布的讀取介面 seller.pub_content_takedown／pub_content_takedowns()：此刻生效中的內容下架措施的對象。Atlas 社群版略過函式與複合型別，
-- 這支 migration 是手寫的（`atlas migrate new`，單體規則 6）；定義與 objects.sql 相同（TestSchemaFileMatchesMigrations），註解見 objects.sql。
CREATE TYPE seller.pub_content_takedown AS (measure_id UUID, content_kind TEXT, content_id UUID);

CREATE FUNCTION seller.pub_content_takedowns() RETURNS SETOF seller.pub_content_takedown
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT c.measure_id, c.content_kind, c.content_id
    FROM seller.measures m
    JOIN seller.measure_contents c ON c.measure_id = m.id
    WHERE m.status = 'ENFORCED' AND m.kind = 'CONTENT_TAKEDOWN';
END;
