-- Create "procurement_agent_run_artifacts" table
CREATE TABLE "procurement_agent_run_artifacts" (
  "artifact_id" bigint NOT NULL,
  "run_id" bigint NOT NULL,
  "artifact_type" text NOT NULL,
  "version" integer NOT NULL,
  "status" text NOT NULL,
  "public_data_json" jsonb NOT NULL,
  "checksum" text NOT NULL,
  "domain_ref_type" text NOT NULL DEFAULT '',
  "domain_ref_id" text NOT NULL DEFAULT '',
  "as_of" timestamptz NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("artifact_id", "version")
);
-- Create index "idx_procurement_agent_run_artifacts_run" to table: "procurement_agent_run_artifacts"
CREATE INDEX "idx_procurement_agent_run_artifacts_run" ON "procurement_agent_run_artifacts" ("run_id", "artifact_type");
-- Create "procurement_agent_run_commands" table
CREATE TABLE "procurement_agent_run_commands" (
  "id" bigint NOT NULL,
  "buyer_id" bigint NOT NULL,
  "command_kind" text NOT NULL,
  "client_request_id" text NOT NULL,
  "payload_hash" text NOT NULL,
  "run_id" bigint NOT NULL,
  "status" text NOT NULL,
  "result_json" jsonb NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "completed_at" timestamptz NULL,
  PRIMARY KEY ("id")
);
-- Create index "uq_procurement_agent_run_commands" to table: "procurement_agent_run_commands"
CREATE UNIQUE INDEX "uq_procurement_agent_run_commands" ON "procurement_agent_run_commands" ("buyer_id", "client_request_id");
-- Create "procurement_agent_run_events" table
CREATE TABLE "procurement_agent_run_events" (
  "id" bigint NOT NULL,
  "run_id" bigint NOT NULL,
  "seq" bigint NOT NULL,
  "event_type" text NOT NULL,
  "schema_version" text NOT NULL,
  "public_payload_json" jsonb NOT NULL DEFAULT '{}',
  "occurred_at" timestamptz NOT NULL,
  "trace_id" text NOT NULL DEFAULT '',
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id")
);
-- Create index "uq_procurement_agent_run_events" to table: "procurement_agent_run_events"
CREATE UNIQUE INDEX "uq_procurement_agent_run_events" ON "procurement_agent_run_events" ("run_id", "seq");
-- Create "procurement_agent_run_inputs" table
CREATE TABLE "procurement_agent_run_inputs" (
  "run_id" bigint NOT NULL,
  "message_id" bigint NULL,
  "input_ref" jsonb NOT NULL,
  "input_schema_version" text NOT NULL,
  "retention_class" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("run_id")
);
-- Create "procurement_agent_run_interrupts" table
CREATE TABLE "procurement_agent_run_interrupts" (
  "id" text NOT NULL,
  "run_id" bigint NOT NULL,
  "interrupt_kind" text NOT NULL,
  "response_schema_version" text NOT NULL,
  "status" text NOT NULL,
  "expires_at" timestamptz NULL,
  "resolved_by_child_run_id" bigint NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "resolved_at" timestamptz NULL,
  PRIMARY KEY ("id")
);
-- Create index "idx_procurement_agent_run_interrupts_run" to table: "procurement_agent_run_interrupts"
CREATE INDEX "idx_procurement_agent_run_interrupts_run" ON "procurement_agent_run_interrupts" ("run_id");
-- Create "procurement_agent_runs" table
CREATE TABLE "procurement_agent_runs" (
  "id" bigint NOT NULL,
  "buyer_id" bigint NOT NULL,
  "conversation_id" bigint NULL,
  "parent_run_id" bigint NULL,
  "run_type" text NOT NULL,
  "domain_ref_type" text NOT NULL DEFAULT '',
  "domain_ref_id" text NOT NULL DEFAULT '',
  "status" text NOT NULL,
  "failure_reason" text NOT NULL DEFAULT '',
  "resumed_into_run_id" bigint NULL,
  "version" bigint NOT NULL DEFAULT 1,
  "last_event_seq" bigint NOT NULL DEFAULT 0,
  "min_available_seq" bigint NOT NULL DEFAULT 1,
  "result_revision_id" bigint NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "started_at" timestamptz NULL,
  "completed_at" timestamptz NULL,
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "chk_procurement_agent_runs_status" CHECK (status = ANY (ARRAY['QUEUED'::text, 'RUNNING'::text, 'WAITING_FOR_INPUT'::text, 'CANCEL_REQUESTED'::text, 'SUCCEEDED'::text, 'FAILED'::text, 'CANCELLED'::text, 'RESUMED'::text, 'EXPIRED'::text])),
  CONSTRAINT "chk_procurement_agent_runs_type" CHECK (run_type = ANY (ARRAY['TURN'::text, 'CANDIDATE_SEARCH'::text, 'CLARIFICATION_RESUME'::text, 'QUOTE_APPROVAL'::text]))
);
-- Create index "idx_procurement_agent_runs_buyer" to table: "procurement_agent_runs"
CREATE INDEX "idx_procurement_agent_runs_buyer" ON "procurement_agent_runs" ("buyer_id", "created_at" DESC);
-- Create index "idx_procurement_agent_runs_conversation" to table: "procurement_agent_runs"
CREATE INDEX "idx_procurement_agent_runs_conversation" ON "procurement_agent_runs" ("conversation_id", "created_at" DESC) WHERE (conversation_id IS NOT NULL);
-- Create index "idx_procurement_agent_runs_parent" to table: "procurement_agent_runs"
CREATE INDEX "idx_procurement_agent_runs_parent" ON "procurement_agent_runs" ("parent_run_id") WHERE (parent_run_id IS NOT NULL);
