-- moderation 的發布讀取介面 moderation.pub_review_item／pub_review_items(at) 與內部的 review_moderation_case(at)。Atlas 社群版略過函式與複合型別，這支 migration 是手寫的（單體規則 6）；
-- 定義與 objects.sql 相同（TestSchemaFileMatchesMigrations），註解見 objects.sql。

CREATE TYPE moderation.pub_review_item AS (
    kind TEXT,
    item_id TEXT,
    case_id UUID,
    target_kind TEXT,
    report_count BIGINT,
    start_at TIMESTAMPTZ,
    due_at TIMESTAMPTZ
);

CREATE FUNCTION moderation.review_moderation_case(at TIMESTAMPTZ) RETURNS SETOF moderation.pub_review_item
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT 'MODERATION_CASE'::text, c.id::text, c.id, c.target_kind,
           (SELECT count(*) FROM moderation.reports r WHERE r.case_id = c.id),
           c.opened_at + INTERVAL '72 hours' - INTERVAL '24 hours', c.opened_at + INTERVAL '72 hours'
    FROM moderation.cases c
    WHERE c.decision IS NULL;
END;

CREATE FUNCTION moderation.pub_review_items(at TIMESTAMPTZ) RETURNS SETOF moderation.pub_review_item
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT * FROM moderation.review_moderation_case(at);
END;
