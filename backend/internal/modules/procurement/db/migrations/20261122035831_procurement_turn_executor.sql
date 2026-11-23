-- Modify "agent_run_inputs" table
ALTER TABLE "procurement"."agent_run_inputs" ADD CONSTRAINT "agent_run_inputs_locale_check" CHECK (locale = ANY (ARRAY['ZH_TW'::text, 'EN'::text])), ADD COLUMN "locale" text NOT NULL;
-- Modify "agent_runs" table
ALTER TABLE "procurement"."agent_runs" DROP CONSTRAINT "agent_runs_failure_check", ADD CONSTRAINT "agent_runs_failure_check" CHECK ((failure_reason IS NULL) OR (failure_reason = ANY (ARRAY['WORKER_LOST'::text, 'SEARCH_FAILED'::text, 'TURN_PIPELINE_ERROR'::text, 'MODEL_UNAVAILABLE'::text])));
-- Modify "request_revisions" table
ALTER TABLE "procurement"."request_revisions" ADD CONSTRAINT "request_revisions_resolution_category_check" CHECK (((item_resolution_category_id IS NOT NULL) = (NOT (item_resolution_kind IS DISTINCT FROM 'CATEGORY_CONFIRMED'::text))) AND ((item_resolution_category_id IS NULL) OR (item_resolution_category_id > 0))), ADD COLUMN "item_resolution_category_id" bigint NULL;
