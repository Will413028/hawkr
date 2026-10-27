-- Modify "payuni_registrations" table
ALTER TABLE "seller"."payuni_registrations" ADD COLUMN "bind_failed_at" timestamptz NULL;
-- 既有的 FAILED 列以 updated_at 回填（上線前沒有正式資料，回填只為讓下面的 CHECK 成立）。
UPDATE "seller"."payuni_registrations" SET "bind_failed_at" = "updated_at" WHERE "bind_status" = 'FAILED';
ALTER TABLE "seller"."payuni_registrations" ADD CONSTRAINT "payuni_registrations_bind_failed_check" CHECK ((bind_status = 'FAILED'::text) = (bind_failed_at IS NOT NULL));
