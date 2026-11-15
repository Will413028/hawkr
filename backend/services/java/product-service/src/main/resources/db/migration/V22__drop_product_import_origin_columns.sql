DROP INDEX IF EXISTS idx_product_import_origin;
ALTER TABLE product
    DROP COLUMN IF EXISTS import_job_id,
    DROP COLUMN IF EXISTS import_item_id;
