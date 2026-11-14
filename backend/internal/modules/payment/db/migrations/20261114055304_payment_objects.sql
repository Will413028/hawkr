-- payment 的發布讀取介面 payment.pub_review_item／pub_review_items(at)。Atlas 社群版略過函式與複合型別，這支 migration 是手寫的（單體規則 6）；
-- 定義與 objects.sql 相同（TestSchemaFileMatchesMigrations），註解見 objects.sql。

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
    WHERE a.create_failed_at IS NOT NULL
    UNION ALL
    SELECT 'PAYMENT_ACCOUNT_PURGE_STUCK'::text, a.store_id::text, a.store_id, a.purge_blocked_at, a.purge_blocked_at + INTERVAL '24 hours'
    FROM payment.accounts a
    WHERE a.purge_blocked_at IS NOT NULL;
END;
