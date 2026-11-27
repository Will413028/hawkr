-- Cancel pending clarification interrupts whose stored payload carries non-UUID category IDs, and their asking runs
WITH stale AS (
    UPDATE "procurement"."agent_run_interrupts"
    SET "status" = 'CANCELLED', "closed_at" = now()
    WHERE "status" = 'PENDING'
      AND (
        jsonb_path_exists("payload", '$.rootId ? (!(@ like_regex "^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$"))')
        OR jsonb_path_exists("payload", '$.candidates[*].categoryId ? (!(@ like_regex "^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$"))')
      )
    RETURNING "run_id"
)
UPDATE "procurement"."agent_runs"
SET "status" = 'CANCELLED', "completed_at" = now(), "updated_at" = now()
WHERE "id" IN (SELECT "run_id" FROM stale) AND "status" = 'WAITING_FOR_INPUT';
-- Reset confirmed item resolutions: the old integer category IDs cannot be carried into the uuid column
ALTER TABLE "procurement"."request_revisions" DROP CONSTRAINT "request_revisions_resolution_category_check";
UPDATE "procurement"."request_revisions"
SET "item_resolution_kind" = NULL, "item_resolution_label" = NULL, "item_resolution_category_id" = NULL
WHERE "item_resolution_kind" = 'CATEGORY_CONFIRMED';
-- Modify "request_revisions" table
ALTER TABLE "procurement"."request_revisions" DROP COLUMN "item_resolution_category_id", ADD COLUMN "item_resolution_category_id" uuid NULL, ADD CONSTRAINT "request_revisions_resolution_category_check" CHECK (((item_resolution_category_id IS NOT NULL) = (NOT (item_resolution_kind IS DISTINCT FROM 'CATEGORY_CONFIRMED'::text))) AND ((item_resolution_category_id IS NULL) OR (item_resolution_category_id <> '00000000-0000-0000-0000-000000000000'::uuid)));
