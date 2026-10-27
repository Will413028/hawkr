-- 終止合作（TERMINATION，條款第 21 條）：措施加開始時間（回填 created_at）、事由與來源措施；依據加 CONTRACT；撤銷原因加 SOURCE_REVOKED；
-- 版本的作者可以是系統；關店可逆（關店紀錄由 ADMIN 改記造成關店的措施、歷史表 store_closures、清理時才釋放 logo 與封面）。
-- 轉換前先核對：既有的 closed_by = 'ADMIN' 列（管理員關店）沒有對應的終止措施可指（理由與作者不能憑空補成措施），整個 migration 失敗並列出 store_id，
-- 請先人工處理這些列（或重建本機資料庫）後重跑（先例：20261012214337 的 suspensions.source = 'ADMIN'）。
DO $$
DECLARE
  admin_closed text;
BEGIN
  SELECT string_agg(s.store_id::text, ', ' ORDER BY s.store_id) INTO admin_closed
  FROM "seller"."stores" s WHERE s.closed_by = 'ADMIN';
  IF admin_closed IS NOT NULL THEN
    RAISE EXCEPTION 'stores 有管理員關店的列（closed_by = ADMIN），無法轉成終止合作的關店紀錄，請先人工處理這些列後重跑：%', admin_closed;
  END IF;
END
$$;
-- Modify "measure_revisions" table
ALTER TABLE "seller"."measure_revisions" ADD CONSTRAINT "measure_revisions_author_check" CHECK ((system_cause IS NULL) = (admin_id IS NOT NULL)), ADD CONSTRAINT "measure_revisions_system_cause_check" CHECK (system_cause = 'STORE_CLOSED'::text), ALTER COLUMN "admin_id" DROP NOT NULL, ADD COLUMN "system_cause" text NULL;
-- Drop index "measures_active_open" from table: "measures"
DROP INDEX "seller"."measures_active_open";
-- Drop index "measures_active_timed" from table: "measures"
DROP INDEX "seller"."measures_active_timed";
-- Drop index "measures_enforced_suspension" from table: "measures"
DROP INDEX "seller"."measures_enforced_suspension";
-- Add columns to "measures" table：starts_at 先允許空值，回填成 created_at（既有的措施都是建立即生效）後再設 NOT NULL。
ALTER TABLE "seller"."measures" ADD COLUMN "starts_at" timestamptz NULL, ADD COLUMN "termination_ground" text NULL, ADD COLUMN "source_measure_id" uuid NULL;
UPDATE "seller"."measures" SET "starts_at" = "created_at";
-- Modify "measures" table
ALTER TABLE "seller"."measures" ALTER COLUMN "starts_at" SET NOT NULL, DROP CONSTRAINT "measures_basis_value_check", ADD CONSTRAINT "measures_basis_value_check" CHECK (basis = ANY (ARRAY['VIOLATION'::text, 'RISK'::text, 'CONTRACT'::text])), DROP CONSTRAINT "measures_kind_check", ADD CONSTRAINT "measures_kind_check" CHECK (kind = ANY (ARRAY['WARNING'::text, 'SUSPENSION'::text, 'TERMINATION'::text])), DROP CONSTRAINT "measures_revoked_reason_check", ADD CONSTRAINT "measures_revoked_reason_check" CHECK (revoked_reason = ANY (ARRAY['APPEAL_ACCEPTED'::text, 'ADMIN_CORRECTION'::text, 'EXTERNAL_DECISION'::text, 'OTHER'::text, 'FINDING_REVOKED'::text, 'SOURCE_REVOKED'::text])), ADD CONSTRAINT "measures_breach_check" CHECK ((termination_ground <> 'MATERIAL_BREACH'::text) OR (basis = 'VIOLATION'::text)), ADD CONSTRAINT "measures_contract_check" CHECK ((basis <> 'CONTRACT'::text) OR (kind = 'TERMINATION'::text)), ADD CONSTRAINT "measures_ends_check" CHECK ((ends_at IS NULL) OR (ends_at > created_at)), ADD CONSTRAINT "measures_ground_basis_check" CHECK ((termination_ground <> ALL (ARRAY['NOTICE'::text, 'LINKED_ACCOUNT'::text])) OR (basis = 'CONTRACT'::text)), ADD CONSTRAINT "measures_source_check" CHECK ((source_measure_id IS NOT NULL) = (NOT (termination_ground IS DISTINCT FROM 'LINKED_ACCOUNT'::text))), ADD CONSTRAINT "measures_starts_check" CHECK ((starts_at >= created_at) AND ((kind = 'TERMINATION'::text) OR (starts_at = created_at)) AND ((termination_ground IS DISTINCT FROM 'NOTICE'::text) OR (starts_at = (created_at + '720:00:00'::interval))) AND ((termination_ground IS NULL) OR (termination_ground = 'NOTICE'::text) OR (starts_at = created_at))), ADD CONSTRAINT "measures_termination_check" CHECK ((kind = 'TERMINATION'::text) = (termination_ground IS NOT NULL)), ADD CONSTRAINT "measures_termination_ground_check" CHECK (termination_ground = ANY (ARRAY['NOTICE'::text, 'MATERIAL_BREACH'::text, 'CEASED_BUSINESS'::text, 'DISSOLUTION'::text, 'BANKRUPTCY'::text, 'LICENSE_REVOKED'::text, 'FOOD_SAFETY_INCIDENT'::text, 'FRAUD'::text, 'PLATFORM_SECURITY'::text, 'OTHER_SERIOUS'::text, 'LINKED_ACCOUNT'::text])), ADD CONSTRAINT "measures_source_measure_id_fkey" FOREIGN KEY ("source_measure_id") REFERENCES "seller"."measures" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Create index "measures_active_open" to table: "measures"
CREATE INDEX "measures_active_open" ON "seller"."measures" ("starts_at") WHERE ((status = 'ENFORCED'::text) AND (kind = ANY (ARRAY['SUSPENSION'::text, 'TERMINATION'::text])) AND (ends_at IS NULL));
-- Create index "measures_active_timed" to table: "measures"
CREATE INDEX "measures_active_timed" ON "seller"."measures" ("ends_at", "starts_at") WHERE ((status = 'ENFORCED'::text) AND (kind = ANY (ARRAY['SUSPENSION'::text, 'TERMINATION'::text])) AND (ends_at IS NOT NULL));
-- Create index "measures_enforced_suspension" to table: "measures"
CREATE INDEX "measures_enforced_suspension" ON "seller"."measures" ("account_id", "starts_at", "ends_at") WHERE ((status = 'ENFORCED'::text) AND (kind = ANY (ARRAY['SUSPENSION'::text, 'TERMINATION'::text])));
-- Create index "measures_linked_source" to table: "measures"
CREATE UNIQUE INDEX "measures_linked_source" ON "seller"."measures" ("source_measure_id", "account_id") WHERE ((source_measure_id IS NOT NULL) AND (status = 'ENFORCED'::text));
-- Create index "measures_source" to table: "measures"
CREATE INDEX "measures_source" ON "seller"."measures" ("source_measure_id") WHERE (source_measure_id IS NOT NULL);
-- Add column to "stores" table：既有的關店紀錄沒有 ADMIN 列（上面已核對），欄位直接換成 closed_by_measure_id。
-- Modify "stores" table
ALTER TABLE "seller"."stores" DROP CONSTRAINT "stores_close_reason_check", DROP CONSTRAINT "stores_closed_by_check", ADD CONSTRAINT "stores_closed_by_check" CHECK (closed_by = ANY (ARRAY['ACCOUNT_OWNER'::text, 'TERMINATION'::text, 'ACCOUNT_DELETION'::text])), DROP CONSTRAINT "stores_closed_images_check", ADD CONSTRAINT "stores_closed_images_check" CHECK ((pii_purged_at IS NULL) OR ((logo_key IS NULL) AND (cover_key IS NULL))), DROP CONSTRAINT "stores_closure_check", ADD CONSTRAINT "stores_closure_check" CHECK (((status = 'CLOSED'::text) = (closed_by IS NOT NULL)) AND ((NOT (closed_by IS DISTINCT FROM 'TERMINATION'::text)) = (closed_by_measure_id IS NOT NULL))), DROP CONSTRAINT "stores_pii_purged_check", ADD CONSTRAINT "stores_pii_purged_check" CHECK (((pii_purged_at IS NULL) AND (store_name IS NOT NULL)) OR ((pii_purged_at IS NOT NULL) AND (status = 'CLOSED'::text) AND (store_name IS NULL) AND (description IS NULL) AND (contact_phone IS NULL) AND (contact_name IS NULL) AND (contact_mobile IS NULL) AND (support_email IS NULL) AND (town_code IS NULL) AND (address_detail IS NULL) AND (prize IS NULL) AND (website_url IS NULL))), DROP COLUMN "closed_by_admin_id", DROP COLUMN "close_reason", ADD COLUMN "closed_by_measure_id" uuid NULL, ADD CONSTRAINT "stores_closed_by_measure_fkey" FOREIGN KEY ("closed_by_measure_id") REFERENCES "seller"."measures" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Create index "stores_closed_by_measure" to table: "stores"
CREATE INDEX "stores_closed_by_measure" ON "seller"."stores" ("closed_by_measure_id") WHERE (closed_by_measure_id IS NOT NULL);
-- Create index "stores_retire_pending" to table: "stores"
CREATE INDEX "stores_retire_pending" ON "seller"."stores" ("deleted_at", "store_id") WHERE ((status = 'CLOSED'::text) AND (pii_purged_at IS NULL));
-- Create "store_closures" table
CREATE TABLE "seller"."store_closures" (
  "store_id" bigint NOT NULL,
  "closed_at" timestamptz NOT NULL,
  "closed_by" text NOT NULL,
  "closed_by_measure_id" uuid NULL,
  "reopened_at" timestamptz NOT NULL,
  PRIMARY KEY ("store_id", "closed_at"),
  CONSTRAINT "store_closures_measure_fkey" FOREIGN KEY ("closed_by_measure_id") REFERENCES "seller"."measures" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "store_closures_store_id_fkey" FOREIGN KEY ("store_id") REFERENCES "seller"."stores" ("store_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "store_closures_closed_by_check" CHECK (closed_by = ANY (ARRAY['ACCOUNT_OWNER'::text, 'TERMINATION'::text, 'ACCOUNT_DELETION'::text])),
  CONSTRAINT "store_closures_measure_check" CHECK ((closed_by = 'TERMINATION'::text) = (closed_by_measure_id IS NOT NULL)),
  CONSTRAINT "store_closures_reopened_check" CHECK (reopened_at >= closed_at)
);
