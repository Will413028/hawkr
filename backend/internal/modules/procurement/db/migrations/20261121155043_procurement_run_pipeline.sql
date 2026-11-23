-- Modify "agent_run_interrupts" table
ALTER TABLE "procurement"."agent_run_interrupts" ADD CONSTRAINT "agent_run_interrupts_resolved_check" CHECK (((status = 'RESOLVED'::text) = ((resolved_by_child_run_id IS NOT NULL) AND (resolved_at IS NOT NULL))) AND ((status = 'PENDING'::text) = ((resolved_by_child_run_id IS NULL) AND (resolved_at IS NULL))));
-- Create index "agent_run_interrupts_child" to table: "agent_run_interrupts"
CREATE UNIQUE INDEX "agent_run_interrupts_child" ON "procurement"."agent_run_interrupts" ("resolved_by_child_run_id") WHERE (resolved_by_child_run_id IS NOT NULL);
-- Create index "agent_run_interrupts_pending" to table: "agent_run_interrupts"
CREATE UNIQUE INDEX "agent_run_interrupts_pending" ON "procurement"."agent_run_interrupts" ("run_id") WHERE (status = 'PENDING'::text);
-- Modify "agent_runs" table
ALTER TABLE "procurement"."agent_runs" ALTER COLUMN "quota_period_start" DROP NOT NULL;
-- Create index "agent_runs_buyer_active" to table: "agent_runs"
CREATE INDEX "agent_runs_buyer_active" ON "procurement"."agent_runs" ("buyer_id") WHERE (status = ANY (ARRAY['QUEUED'::text, 'RUNNING'::text, 'CANCEL_REQUESTED'::text]));
-- Create index "agent_runs_buyer_created" to table: "agent_runs"
CREATE INDEX "agent_runs_buyer_created" ON "procurement"."agent_runs" ("buyer_id", "created_at" DESC);
-- Drop "agent_run_artifacts" table
DROP TABLE "procurement"."agent_run_artifacts";
