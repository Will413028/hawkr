-- Drop index "agent_runs_unfinished" from table: "agent_runs"
DROP INDEX "procurement"."agent_runs_unfinished";
-- Modify "agent_runs" table
ALTER TABLE "procurement"."agent_runs" ADD COLUMN "job_id" bigint NULL, ADD COLUMN "quota_period_start" timestamptz NOT NULL;
-- Create index "agent_runs_unfinished" to table: "agent_runs"
CREATE INDEX "agent_runs_unfinished" ON "procurement"."agent_runs" ("id") WHERE (status = ANY (ARRAY['QUEUED'::text, 'RUNNING'::text, 'CANCEL_REQUESTED'::text]));
