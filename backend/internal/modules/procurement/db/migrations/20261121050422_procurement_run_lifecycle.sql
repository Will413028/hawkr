-- Drop index "agent_run_artifacts_run" from table: "agent_run_artifacts"
DROP INDEX "procurement"."agent_run_artifacts_run";
-- Modify "agent_run_artifacts" table
ALTER TABLE "procurement"."agent_run_artifacts" DROP CONSTRAINT "agent_run_artifacts_pkey", DROP COLUMN "artifact_id", ADD COLUMN "updated_at" timestamptz NOT NULL, ADD PRIMARY KEY ("run_id", "artifact_type");
-- Drop index "agent_runs_unfinished" from table: "agent_runs"
DROP INDEX "procurement"."agent_runs_unfinished";
-- Modify "agent_runs" table
ALTER TABLE "procurement"."agent_runs" DROP CONSTRAINT "agent_runs_publication_id_fkey", DROP CONSTRAINT "agent_runs_failure_check", ADD CONSTRAINT "agent_runs_failure_check" CHECK ((failure_reason IS NULL) OR (failure_reason = ANY (ARRAY['WORKER_LOST'::text, 'SEARCH_FAILED'::text, 'TURN_PIPELINE_ERROR'::text]))), DROP CONSTRAINT "agent_runs_status_check", ADD CONSTRAINT "agent_runs_status_check" CHECK (status = ANY (ARRAY['QUEUED'::text, 'RUNNING'::text, 'WAITING_FOR_INPUT'::text, 'CANCEL_REQUESTED'::text, 'SUCCEEDED'::text, 'FAILED'::text, 'CANCELLED'::text, 'RESUMED'::text])), ADD COLUMN "job_id" bigint NULL, ADD COLUMN "quota_period_start" timestamptz NOT NULL, ADD CONSTRAINT "agent_runs_publication_id_fkey" FOREIGN KEY ("publication_id") REFERENCES "procurement"."rfq_publications" ("id") ON UPDATE NO ACTION ON DELETE SET NULL;
-- Create index "agent_runs_unfinished" to table: "agent_runs"
CREATE INDEX "agent_runs_unfinished" ON "procurement"."agent_runs" ("id") WHERE (status = ANY (ARRAY['QUEUED'::text, 'RUNNING'::text, 'CANCEL_REQUESTED'::text]));
-- Create index "agent_runs_event_expiry" to table: "agent_runs"
CREATE INDEX "agent_runs_event_expiry" ON "procurement"."agent_runs" ("completed_at") WHERE ((completed_at IS NOT NULL) AND (min_available_seq <= last_event_seq));
