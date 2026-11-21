-- Modify "agent_runs" table
ALTER TABLE "procurement"."agent_runs" ADD CONSTRAINT "agent_runs_quota_check" CHECK ((run_type = 'TURN'::text) = (quota_period_start IS NOT NULL));
