-- Modify "procurement_agent_runs" table
ALTER TABLE "procurement_agent_runs" DROP CONSTRAINT "chk_procurement_agent_runs_type", ADD CONSTRAINT "chk_procurement_agent_runs_type" CHECK (run_type = ANY (ARRAY['TURN'::text, 'CANDIDATE_SEARCH'::text, 'CLARIFICATION_RESUME'::text, 'QUOTE_APPROVAL'::text, 'PUBLICATION'::text]));
