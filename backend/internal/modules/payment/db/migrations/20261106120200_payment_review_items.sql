-- payment 發布的後台待辦（db/objects.sql；手寫，單體規則 6）。
CREATE TYPE payment.pub_review_item AS (
    kind TEXT,
    item_id TEXT,
    store_id BIGINT,
    start_at TIMESTAMPTZ,
    due_at TIMESTAMPTZ
);

CREATE FUNCTION payment.pub_review_items(at TIMESTAMPTZ) RETURNS SETOF payment.pub_review_item
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT 'PAYMENT_ACCOUNT_CREATE_FAILED'::text, a.store_id::text, a.store_id, a.create_failed_at, a.create_failed_at + INTERVAL '24 hours'
    FROM payment.accounts a
    WHERE a.create_failed_at IS NOT NULL;
END;
