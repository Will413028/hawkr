-- 將 banners.id 從 BIGSERIAL (auto-increment) 改為 BIGINT (application-generated TSID)
ALTER TABLE banners ALTER COLUMN id DROP DEFAULT;
DROP SEQUENCE IF EXISTS banners_id_seq;
