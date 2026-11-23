-- Modify "agent_runs" table
ALTER TABLE "procurement"."agent_runs" DROP CONSTRAINT "agent_runs_failure_check", ADD CONSTRAINT "agent_runs_failure_check" CHECK ((failure_reason IS NULL) OR (failure_reason = ANY (ARRAY['WORKER_LOST'::text, 'SEARCH_FAILED'::text, 'TURN_PIPELINE_ERROR'::text, 'MODEL_UNAVAILABLE'::text, 'ITEM_CATEGORY_UNAVAILABLE'::text])));
-- Modify "candidate_sets" table
ALTER TABLE "procurement"."candidate_sets" DROP COLUMN "client_request_id";
-- Modify "candidates" table
ALTER TABLE "procurement"."candidates" DROP CONSTRAINT "candidates_matched_check", DROP CONSTRAINT "candidates_unverifiable_check", DROP COLUMN "score", DROP COLUMN "matched_facts", DROP COLUMN "unverifiable_facts", DROP COLUMN "facts_as_of";
