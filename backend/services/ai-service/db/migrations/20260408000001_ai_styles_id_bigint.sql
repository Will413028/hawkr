-- 移除 ai_styles.id 的 bigserial sequence，改為純 bigint（由 TSID 生成）
ALTER TABLE ai_styles ALTER COLUMN id DROP DEFAULT;
DROP SEQUENCE IF EXISTS ai_styles_id_seq;
