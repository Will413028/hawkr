-- Drop index "agent_run_interrupts_pending" from table: "agent_run_interrupts"
DROP INDEX "procurement"."agent_run_interrupts_pending";
-- Modify "agent_run_interrupts" table
ALTER TABLE "procurement"."agent_run_interrupts" DROP CONSTRAINT "agent_run_interrupts_resolved_check", DROP CONSTRAINT "agent_run_interrupts_status_check", ADD CONSTRAINT "agent_run_interrupts_status_check" CHECK (status = ANY (ARRAY['PENDING'::text, 'RESOLVED'::text, 'CANCELLED'::text])), ADD CONSTRAINT "agent_run_interrupts_closed_check" CHECK ((status = 'PENDING'::text) = (closed_at IS NULL)), DROP COLUMN "resolved_by_child_run_id", DROP COLUMN "resolved_at", ADD COLUMN "request_id" uuid NOT NULL, ADD COLUMN "closed_at" timestamptz NULL, ADD CONSTRAINT "agent_run_interrupts_request_id_fkey" FOREIGN KEY ("request_id") REFERENCES "procurement"."requests" ("id") ON UPDATE NO ACTION ON DELETE CASCADE;
-- Create index "agent_run_interrupts_pending" to table: "agent_run_interrupts"
CREATE UNIQUE INDEX "agent_run_interrupts_pending" ON "procurement"."agent_run_interrupts" ("request_id") WHERE (status = 'PENDING'::text);
-- Modify "agent_runs" table
ALTER TABLE "procurement"."agent_runs" DROP CONSTRAINT "agent_runs_status_check", ADD CONSTRAINT "agent_runs_status_check" CHECK (status = ANY (ARRAY['QUEUED'::text, 'RUNNING'::text, 'WAITING_FOR_INPUT'::text, 'CANCEL_REQUESTED'::text, 'SUCCEEDED'::text, 'FAILED'::text, 'CANCELLED'::text])), ADD CONSTRAINT "agent_runs_interrupt_check" CHECK ((run_type = 'CLARIFICATION_RESUME'::text) = (interrupt_id IS NOT NULL)), DROP COLUMN "parent_run_id", DROP COLUMN "resumed_into_run_id", ADD COLUMN "interrupt_id" uuid NULL, ADD CONSTRAINT "agent_runs_interrupt_fkey" FOREIGN KEY ("interrupt_id") REFERENCES "procurement"."agent_run_interrupts" ("id") ON UPDATE NO ACTION ON DELETE CASCADE;
-- Create index "agent_runs_interrupt" to table: "agent_runs"
CREATE INDEX "agent_runs_interrupt" ON "procurement"."agent_runs" ("interrupt_id") WHERE (interrupt_id IS NOT NULL);
-- Drop "app_states" table
DROP TABLE "procurement"."app_states";
-- Drop "user_states" table
DROP TABLE "procurement"."user_states";
-- Drop "events" table
DROP TABLE "procurement"."events";
-- Drop "sessions" table
DROP TABLE "procurement"."sessions";
