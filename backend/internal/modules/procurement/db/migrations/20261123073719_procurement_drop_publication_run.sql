-- Modify "agent_runs" table
ALTER TABLE "procurement"."agent_runs" DROP CONSTRAINT "agent_runs_type_check", ADD CONSTRAINT "agent_runs_type_check" CHECK (run_type = ANY (ARRAY['TURN'::text, 'CANDIDATE_SEARCH'::text, 'CLARIFICATION_RESUME'::text])), DROP COLUMN "publication_id";
