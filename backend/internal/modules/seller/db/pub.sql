
CREATE SCHEMA seller;

CREATE TABLE seller.pub_exposable_store (
    store_id BIGINT NOT NULL
);

CREATE FUNCTION seller.pub_exposable_stores(at TIMESTAMPTZ) RETURNS SETOF seller.pub_exposable_store
LANGUAGE sql STABLE
AS $$ SELECT * FROM seller.pub_exposable_store $$;

CREATE TABLE seller.pub_review_item (
    kind TEXT NOT NULL,
    item_id TEXT NOT NULL,
    report_id UUID,
    case_id UUID,
    inquiry_id UUID,
    authority_order_id UUID,
    target_position SMALLINT,
    account_id BIGINT,
    measure_id UUID,
    request_id UUID,
    kyc_application_id UUID,
    store_id BIGINT,
    store_name TEXT,
    start_at TIMESTAMPTZ NOT NULL,
    due_at TIMESTAMPTZ NOT NULL
);

CREATE FUNCTION seller.pub_review_items(at TIMESTAMPTZ) RETURNS SETOF seller.pub_review_item
LANGUAGE sql STABLE
AS $$ SELECT * FROM seller.pub_review_item $$;

CREATE TABLE seller.pub_content_takedown (
    measure_id UUID NOT NULL,
    content_kind TEXT NOT NULL,
    content_id UUID NOT NULL
);

CREATE FUNCTION seller.pub_content_takedowns() RETURNS SETOF seller.pub_content_takedown
LANGUAGE sql STABLE
AS $$ SELECT * FROM seller.pub_content_takedown $$;

CREATE FUNCTION seller.pub_lock_content_takedowns(p_measure_id UUID) RETURNS SETOF seller.pub_content_takedown
LANGUAGE sql VOLATILE
AS $$ SELECT * FROM seller.pub_content_takedown $$;

CREATE TABLE seller.pub_retired_store (
    store_id BIGINT NOT NULL
);

CREATE FUNCTION seller.pub_retired_stores() RETURNS SETOF seller.pub_retired_store
LANGUAGE sql STABLE
AS $$ SELECT * FROM seller.pub_retired_store $$;
