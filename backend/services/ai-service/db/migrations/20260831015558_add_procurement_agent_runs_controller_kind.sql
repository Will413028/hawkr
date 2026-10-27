-- Modify "procurement_agent_runs" table
ALTER TABLE "procurement_agent_runs" ADD CONSTRAINT "chk_procurement_agent_runs_controller_kind" CHECK (controller_kind = ANY (ARRAY['DURABLE'::text, 'GRAPH'::text])), ADD COLUMN "controller_kind" text NOT NULL DEFAULT 'DURABLE';
