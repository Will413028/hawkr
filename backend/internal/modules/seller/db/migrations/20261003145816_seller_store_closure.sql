-- Modify "stores" table: columns first, so the backfill can run before the constraints
ALTER TABLE "seller"."stores" ADD COLUMN "closed_by" text NULL, ADD COLUMN "closed_by_admin_id" bigint NULL, ADD COLUMN "close_reason" text NULL;
-- 關店的紀錄一律有值：既有的已關閉 stint 沒有記錄是誰關的（上線前只有本機與測試資料），回填成賣家自己，再設 CHECK
UPDATE "seller"."stores" SET "closed_by" = 'ACCOUNT_OWNER' WHERE "status" = 'CLOSED';
-- Add constraints to "stores" table
ALTER TABLE "seller"."stores" ADD CONSTRAINT "stores_close_reason_check" CHECK ((close_reason = btrim(close_reason)) AND ((char_length(close_reason) >= 1) AND (char_length(close_reason) <= 1000))), ADD CONSTRAINT "stores_closed_by_check" CHECK (closed_by = ANY (ARRAY['ACCOUNT_OWNER'::text, 'ADMIN'::text, 'ACCOUNT_DELETION'::text])), ADD CONSTRAINT "stores_closure_check" CHECK (((status = 'CLOSED'::text) = (closed_by IS NOT NULL)) AND (((closed_by IS DISTINCT FROM 'ADMIN'::text) AND (closed_by_admin_id IS NULL) AND (close_reason IS NULL)) OR ((closed_by = 'ADMIN'::text) AND (closed_by_admin_id IS NOT NULL) AND (close_reason IS NOT NULL))));
-- Create index "stores_admin_list" to table: "stores"
CREATE INDEX "stores_admin_list" ON "seller"."stores" ("created_at" DESC, "store_id" DESC);
