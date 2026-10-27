-- Create index "idx_procurement_agent_runs_result_revision" to table: "procurement_agent_runs"
CREATE INDEX "idx_procurement_agent_runs_result_revision" ON "procurement_agent_runs" ("result_revision_id") WHERE (result_revision_id IS NOT NULL);
