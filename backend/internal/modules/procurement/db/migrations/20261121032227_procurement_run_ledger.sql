-- Drop index "agent_run_artifacts_run" from table: "agent_run_artifacts"
DROP INDEX "procurement"."agent_run_artifacts_run";
-- Modify "agent_run_artifacts" table
ALTER TABLE "procurement"."agent_run_artifacts" DROP CONSTRAINT "agent_run_artifacts_pkey", DROP COLUMN "artifact_id", ADD COLUMN "updated_at" timestamptz NOT NULL, ADD PRIMARY KEY ("run_id", "artifact_type");
-- Modify "agent_runs" table
ALTER TABLE "procurement"."agent_runs" DROP CONSTRAINT "agent_runs_publication_id_fkey", ADD CONSTRAINT "agent_runs_publication_id_fkey" FOREIGN KEY ("publication_id") REFERENCES "procurement"."rfq_publications" ("id") ON UPDATE NO ACTION ON DELETE SET NULL;
-- Create index "agent_runs_event_expiry" to table: "agent_runs"
CREATE INDEX "agent_runs_event_expiry" ON "procurement"."agent_runs" ("completed_at") WHERE ((completed_at IS NOT NULL) AND (min_available_seq <= last_event_seq));
