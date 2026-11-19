-- pub_content_takedown 去掉 measure_created_at：一則內容至多一個生效措施，social 的複本不再讀它。
-- 複合型別與函式 Atlas 社群版不處理，這支 migration 是手寫的（單體規則 6）；定義與 objects.sql 相同。
DROP FUNCTION seller.pub_content_takedowns();
DROP TYPE seller.pub_content_takedown;

CREATE TYPE seller.pub_content_takedown AS (measure_id UUID, content_kind TEXT, content_id UUID);

CREATE FUNCTION seller.pub_content_takedowns() RETURNS SETOF seller.pub_content_takedown
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT c.measure_id, c.content_kind, c.content_id
    FROM seller.measures m
    JOIN seller.measure_contents c ON c.measure_id = m.id
    WHERE m.status = 'ENFORCED' AND m.kind = 'CONTENT_TAKEDOWN';
END;
