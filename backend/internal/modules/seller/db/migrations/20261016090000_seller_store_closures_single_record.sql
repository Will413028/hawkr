-- 關店的紀錄只留 store_closures 一處：stores 的 deleted_at、closed_by、closed_by_measure_id 併進去，目前這一次的關店是 reopened_at 為 NULL 的那一列。
-- 既有已關閉的 stint 各回填一列目前的紀錄（closed_at = 原本的 deleted_at），已恢復的歷史列照舊。
-- Modify "store_closures" table
ALTER TABLE "seller"."store_closures" DROP CONSTRAINT "store_closures_reopened_check", ALTER COLUMN "reopened_at" DROP NOT NULL, ADD CONSTRAINT "store_closures_reopened_check" CHECK ((reopened_at IS NULL) OR (reopened_at >= closed_at));
INSERT INTO "seller"."store_closures" ("store_id", "closed_at", "closed_by", "closed_by_measure_id", "reopened_at")
SELECT "store_id", "deleted_at", "closed_by", "closed_by_measure_id", NULL FROM "seller"."stores" WHERE "status" = 'CLOSED';
-- Modify "store_closures" table
ALTER TABLE "seller"."store_closures" ADD COLUMN "open_closed_at" timestamptz NULL GENERATED ALWAYS AS (CASE WHEN reopened_at IS NULL THEN closed_at END) STORED, ADD CONSTRAINT "store_closures_open_key" UNIQUE ("store_id", "open_closed_at");
-- Modify "stores" table
ALTER TABLE "seller"."stores" ADD COLUMN "current_closed_at" timestamptz NULL;
UPDATE "seller"."stores" s SET "current_closed_at" = c."closed_at" FROM "seller"."store_closures" c WHERE c."store_id" = s."store_id" AND c."reopened_at" IS NULL;
-- Create index "store_closures_current" to table: "store_closures"
CREATE UNIQUE INDEX "store_closures_current" ON "seller"."store_closures" ("store_id") WHERE (reopened_at IS NULL);
-- Create index "store_closures_measure" to table: "store_closures"
CREATE INDEX "store_closures_measure" ON "seller"."store_closures" ("closed_by_measure_id") WHERE (closed_by_measure_id IS NOT NULL);
-- Drop index "stores_closed_by_measure" from table: "stores"
DROP INDEX "seller"."stores_closed_by_measure";
-- Drop index "stores_retire_pending" from table: "stores"
DROP INDEX "seller"."stores_retire_pending";
-- Modify "stores" table
ALTER TABLE "seller"."stores" DROP CONSTRAINT "stores_closure_check", DROP CONSTRAINT "stores_closed_by_check", DROP CONSTRAINT "stores_deleted_check", DROP CONSTRAINT "stores_closed_by_measure_fkey", DROP COLUMN "deleted_at", DROP COLUMN "closed_by", DROP COLUMN "closed_by_measure_id";
-- Create index "stores_retire_pending" to table: "stores"
CREATE INDEX "stores_retire_pending" ON "seller"."stores" ("store_id") WHERE ((status = 'CLOSED'::text) AND (pii_purged_at IS NULL));
-- Modify "stores" table
ALTER TABLE "seller"."stores" ADD CONSTRAINT "stores_current_closure_key" UNIQUE ("store_id", "current_closed_at"), ADD CONSTRAINT "stores_current_closure_check" CHECK ((status = 'CLOSED'::text) = (current_closed_at IS NOT NULL));
-- Modify "store_closures" table
ALTER TABLE "seller"."store_closures" ADD CONSTRAINT "store_closures_current_fkey" FOREIGN KEY ("store_id", "open_closed_at") REFERENCES "seller"."stores" ("store_id", "current_closed_at") ON UPDATE NO ACTION ON DELETE NO ACTION DEFERRABLE INITIALLY DEFERRED;
-- Modify "stores" table
ALTER TABLE "seller"."stores" ADD CONSTRAINT "stores_current_closure_fkey" FOREIGN KEY ("store_id", "current_closed_at") REFERENCES "seller"."store_closures" ("store_id", "open_closed_at") ON UPDATE NO ACTION ON DELETE NO ACTION DEFERRABLE INITIALLY DEFERRED;
