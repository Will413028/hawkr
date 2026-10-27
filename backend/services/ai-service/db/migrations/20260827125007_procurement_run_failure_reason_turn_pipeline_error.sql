-- Modify "procurement_candidate_sets" table
ALTER TABLE "procurement_candidate_sets" DROP CONSTRAINT "chk_pcs_failure_reason", ADD CONSTRAINT "chk_pcs_failure_reason" CHECK ((failure_reason = ''::text) OR (failure_reason = ANY (ARRAY['WORKER_LOST'::text, 'LANE_ALL_FAILED'::text, 'TURN_PIPELINE_ERROR'::text])));
