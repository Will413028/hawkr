-- atlas:txmode none
-- City backfill/constraint are additive; the public lookup index is built concurrently
-- so existing seller writes are not blocked during deployment.

-- Canonicalize the four 台/臺 aliases before enforcing the 22-city contract.
UPDATE "sellers"
SET "city" = CASE btrim("city")
  WHEN '臺北市' THEN '台北市'
  WHEN '臺中市' THEN '台中市'
  WHEN '臺南市' THEN '台南市'
  WHEN '臺東縣' THEN '台東縣'
  ELSE btrim("city")
END
WHERE "city" IS NOT NULL;

-- Modify "sellers" table
ALTER TABLE "sellers" ADD CONSTRAINT "chk_sellers_city" CHECK ((city IS NULL) OR ((city)::text = ''::text) OR ((city)::text = ANY ((ARRAY['基隆市'::character varying, '台北市'::character varying, '新北市'::character varying, '桃園市'::character varying, '新竹市'::character varying, '新竹縣'::character varying, '苗栗縣'::character varying, '台中市'::character varying, '彰化縣'::character varying, '南投縣'::character varying, '雲林縣'::character varying, '嘉義市'::character varying, '嘉義縣'::character varying, '台南市'::character varying, '高雄市'::character varying, '屏東縣'::character varying, '宜蘭縣'::character varying, '花蓮縣'::character varying, '台東縣'::character varying, '澎湖縣'::character varying, '金門縣'::character varying, '連江縣'::character varying])::text[])));
-- Create index "idx_sellers_public_city" to table: "sellers"
CREATE INDEX CONCURRENTLY "idx_sellers_public_city" ON "sellers" ("city", "store_id") WHERE (is_current AND (deleted_at IS NULL) AND ((status)::text = 'active'::text));
