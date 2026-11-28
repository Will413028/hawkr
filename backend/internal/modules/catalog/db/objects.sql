CREATE FUNCTION catalog.buyer_viewable_products(at TIMESTAMPTZ) RETURNS SETOF catalog.products
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT p.*
    FROM catalog.products p
    WHERE p.status IN ('ACTIVE', 'ARCHIVED') AND p.deleted_at IS NULL
      AND p.store_id IN (SELECT e.store_id FROM seller.pub_exposable_stores(at) e)
      AND NOT EXISTS (SELECT 1 FROM seller.pub_content_takedowns() t WHERE t.content_kind = 'PRODUCT' AND t.content_id = p.id);
END;

CREATE FUNCTION catalog.buyer_visible_products(at TIMESTAMPTZ) RETURNS SETOF catalog.products
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT v.*
    FROM catalog.buyer_viewable_products(at) v
    WHERE v.status = 'ACTIVE';
END;
