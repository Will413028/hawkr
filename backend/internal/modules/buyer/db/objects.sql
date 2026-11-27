
CREATE TYPE buyer.pub_inactive_buyer AS (buyer_id BIGINT);

CREATE FUNCTION buyer.pub_inactive_buyers() RETURNS SETOF buyer.pub_inactive_buyer
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT b.buyer_id
    FROM buyer.buyers b
    WHERE b.status <> 'ACTIVE';
END;

CREATE TYPE buyer.pub_closed_buyer AS (buyer_id BIGINT);

CREATE FUNCTION buyer.pub_closed_buyers() RETURNS SETOF buyer.pub_closed_buyer
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT b.buyer_id
    FROM buyer.buyers b
    WHERE b.status = 'CLOSED';
END;

CREATE TYPE buyer.pub_blocked_buyer AS (blocker_buyer_id BIGINT, blocked_buyer_id BIGINT);

CREATE FUNCTION buyer.pub_blocked_buyers() RETURNS SETOF buyer.pub_blocked_buyer
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT b.blocker_buyer_id, b.blocked_buyer_id
    FROM buyer.blocked_buyers b;
END;

CREATE TYPE buyer.pub_blocked_store AS (blocker_buyer_id BIGINT, store_id BIGINT);

CREATE FUNCTION buyer.pub_blocked_stores() RETURNS SETOF buyer.pub_blocked_store
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT b.blocker_buyer_id, b.store_id
    FROM buyer.blocked_stores b;
END;
