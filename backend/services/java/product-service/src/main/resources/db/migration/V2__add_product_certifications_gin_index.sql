-- 02 §5.3.4 商品認證篩選（certification_types）
-- certifications JSONB 的 @> containment 查詢加速；對應 idx_product_certifications_gin
CREATE INDEX IF NOT EXISTS idx_product_certifications_gin
    ON product USING gin (certifications jsonb_path_ops);
