SELECT (EXISTS (SELECT 1 FROM seller.pub_exposable_stores($1::timestamp with time zone) AS pub WHERE pub.store_id = $2::bigint)) AS "value";

-- args:
-- $1 = 2026-10-23 12:00:00 +0000 UTC
-- $2 = 7
