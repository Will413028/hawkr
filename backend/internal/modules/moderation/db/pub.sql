
CREATE SCHEMA moderation;

CREATE TABLE moderation.pub_review_item (
    kind         TEXT        NOT NULL,
    item_id      TEXT        NOT NULL,
    case_id      UUID        NOT NULL,
    start_at     TIMESTAMPTZ NOT NULL,
    due_at       TIMESTAMPTZ NOT NULL
);

CREATE FUNCTION moderation.pub_review_items(at TIMESTAMPTZ) RETURNS SETOF moderation.pub_review_item
LANGUAGE sql STABLE
AS $$ SELECT * FROM moderation.pub_review_item $$;
