-- 內容下架對象的帶鎖發布函式：social 對帳補漏寫複本之前鎖住措施列與對象列，序列化撤銷。
-- 函式 Atlas 社群版不處理，這支 migration 是手寫的（單體規則 6）；定義與 objects.sql 相同。
CREATE FUNCTION seller.pub_lock_content_takedowns(p_measure_id UUID) RETURNS SETOF seller.pub_content_takedown
LANGUAGE sql VOLATILE
BEGIN ATOMIC
    SELECT c.measure_id, c.content_kind, c.content_id
    FROM seller.measures m
    JOIN seller.measure_contents c ON c.measure_id = m.id
    WHERE m.id = p_measure_id AND m.status = 'ENFORCED' AND m.kind = 'CONTENT_TAKEDOWN'
    ORDER BY c.content_kind, c.content_id
    FOR SHARE OF m, c;
END;
