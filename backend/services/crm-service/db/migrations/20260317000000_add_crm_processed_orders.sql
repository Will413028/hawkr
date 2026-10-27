CREATE TABLE crm_processed_orders (
    id         BIGSERIAL PRIMARY KEY,
    order_no   VARCHAR(64) NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_crm_processed_orders_created_at ON crm_processed_orders (created_at);
