
CREATE SCHEMA payment;

CREATE TABLE payment.pub_review_item (
    kind     TEXT        NOT NULL,
    item_id  TEXT        NOT NULL,
    store_id BIGINT      NOT NULL,
    start_at TIMESTAMPTZ NOT NULL,
    due_at   TIMESTAMPTZ NOT NULL
);

CREATE FUNCTION payment.pub_review_items(at TIMESTAMPTZ) RETURNS SETOF payment.pub_review_item
LANGUAGE sql STABLE
AS $$ SELECT * FROM payment.pub_review_item $$;
