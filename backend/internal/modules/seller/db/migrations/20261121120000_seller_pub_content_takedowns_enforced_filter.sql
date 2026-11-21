-- pub_content_takedowns() 加 c.measure_status = 'ENFORCED'（與 m.status 同值，由複合外鍵同步）：讓 im、review 的逐列 EXISTS（content_kind、content_id 相等）走部分唯一索引
-- measure_contents_one_enforced。回傳的列不變。Atlas 社群版略過函式，這支 migration 是手寫的（單體規則 6）；定義與 objects.sql 相同。
CREATE OR REPLACE FUNCTION seller.pub_content_takedowns() RETURNS SETOF seller.pub_content_takedown
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT c.measure_id, c.content_kind, c.content_id
    FROM seller.measures m
    JOIN seller.measure_contents c ON c.measure_id = m.id
    WHERE m.status = 'ENFORCED' AND m.kind = 'CONTENT_TAKEDOWN' AND c.measure_status = 'ENFORCED';
END;
