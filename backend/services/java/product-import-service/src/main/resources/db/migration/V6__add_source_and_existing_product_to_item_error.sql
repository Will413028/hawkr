-- 40 號設計書 §3.5 + §4.3a wipe scope：
-- 1) product_import_item_error 加 source 欄位區分來源（L2_VALIDATOR / VALIDATE_PRODUCT_DRAFT / HTML_PARSE / CERT_VERIFY / SHIPPING_DOWNGRADE）
-- 2) 加 existing_product_* 三欄位（DUPLICATE_NAME_VS_EXISTING 時填值，供前端 DuplicateNameWarning 渲染）

ALTER TABLE product_import_item_error
    ADD COLUMN source VARCHAR(32) NOT NULL DEFAULT 'L2_VALIDATOR',
    ADD COLUMN existing_product_id BIGINT,
    ADD COLUMN existing_product_name VARCHAR(255),
    ADD COLUMN existing_product_status_label VARCHAR(50);

ALTER TABLE product_import_item_error
    ADD CONSTRAINT chk_piie_source CHECK (
        source IN ('L2_VALIDATOR','VALIDATE_PRODUCT_DRAFT','HTML_PARSE','CERT_VERIFY','SHIPPING_DOWNGRADE')
    );

CREATE INDEX idx_piie_source ON product_import_item_error (import_item_id, source);
