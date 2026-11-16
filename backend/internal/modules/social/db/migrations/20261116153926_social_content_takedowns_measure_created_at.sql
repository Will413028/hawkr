-- Rename column "created_at" to "measure_created_at" of "content_takedowns" table：欄位現在是 seller 措施的建立時間（原本是寫入複本的時間）。
-- 保留既有的列（改名而不是 DROP 再 ADD）；對帳補寫過的列帶的是對帳時間，下一次改版前的舊列沒有更精確的來源。
ALTER TABLE "social"."content_takedowns" RENAME COLUMN "created_at" TO "measure_created_at";
-- PostgreSQL 18 的 NOT NULL 是有名字的約束，改欄位名不會跟著改；名字要和 schema.sql 新建出來的一致（TestSchemaFileMatchesMigrations 比對約束名）。
ALTER TABLE "social"."content_takedowns" RENAME CONSTRAINT "content_takedowns_created_at_not_null" TO "content_takedowns_measure_created_at_not_null";
