
CREATE SCHEMA buyer;

CREATE TABLE buyer.pub_inactive_buyer (
    buyer_id BIGINT NOT NULL
);

CREATE FUNCTION buyer.pub_inactive_buyers() RETURNS SETOF buyer.pub_inactive_buyer
LANGUAGE sql STABLE
AS $$ SELECT * FROM buyer.pub_inactive_buyer $$;

CREATE TABLE buyer.pub_closed_buyer (
    buyer_id BIGINT NOT NULL
);

CREATE FUNCTION buyer.pub_closed_buyers() RETURNS SETOF buyer.pub_closed_buyer
LANGUAGE sql STABLE
AS $$ SELECT * FROM buyer.pub_closed_buyer $$;

CREATE TABLE buyer.pub_blocked_buyer (
    blocker_buyer_id BIGINT NOT NULL,
    blocked_buyer_id BIGINT NOT NULL
);

CREATE FUNCTION buyer.pub_blocked_buyers() RETURNS SETOF buyer.pub_blocked_buyer
LANGUAGE sql STABLE
AS $$ SELECT * FROM buyer.pub_blocked_buyer $$;

CREATE TABLE buyer.pub_blocked_store (
    blocker_buyer_id BIGINT NOT NULL,
    store_id         BIGINT NOT NULL
);

CREATE FUNCTION buyer.pub_blocked_stores() RETURNS SETOF buyer.pub_blocked_store
LANGUAGE sql STABLE
AS $$ SELECT * FROM buyer.pub_blocked_store $$;
