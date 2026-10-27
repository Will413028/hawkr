SELECT ((EXISTS (SELECT 1 FROM seller.pub_exposable_stores($1::timestamp with time zone) AS pub WHERE pub.store_id = $2::bigint)) AND (NOT (EXISTS (SELECT 1 FROM buyer.pub_blocked_stores() AS pub WHERE pub.blocker_buyer_id = $3::bigint AND ((store_id = $4::bigint)))))) AS "value";

-- args:
-- $1 = 2026-10-23 12:00:00 +0000 UTC
-- $2 = 7
-- $3 = 42
-- $4 = 7
