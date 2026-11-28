CREATE INDEX products_public_price_asc_idx ON catalog.products ((-min_price) DESC, id DESC) WHERE deleted_at IS NULL AND status = 'ACTIVE';
CREATE INDEX products_public_price_desc_idx ON catalog.products (max_price DESC, id DESC) WHERE deleted_at IS NULL AND status = 'ACTIVE';
CREATE INDEX products_public_rating_idx ON catalog.products ((round(rating_average * 1000)::bigint) DESC, id DESC) WHERE deleted_at IS NULL AND status = 'ACTIVE';
