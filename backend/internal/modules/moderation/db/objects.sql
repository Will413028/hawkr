
CREATE TYPE moderation.pub_review_item AS (
    kind TEXT,
    item_id TEXT,
    case_id UUID,
    start_at TIMESTAMPTZ,
    due_at TIMESTAMPTZ
);

CREATE FUNCTION moderation.review_moderation_case(at TIMESTAMPTZ) RETURNS SETOF moderation.pub_review_item
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT 'MODERATION_CASE'::text, c.id::text, c.id,
           c.opened_at + INTERVAL '72 hours' - INTERVAL '24 hours', c.opened_at + INTERVAL '72 hours'
    FROM moderation.cases c
    WHERE c.closed_at IS NULL;
END;

CREATE FUNCTION moderation.pub_review_items(at TIMESTAMPTZ) RETURNS SETOF moderation.pub_review_item
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT * FROM moderation.review_moderation_case(at);
END;
