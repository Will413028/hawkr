-- Add new schema named "procurement"
CREATE SCHEMA "procurement";
-- Create "app_states" table
CREATE TABLE "procurement"."app_states" (
  "app_name" text NOT NULL,
  "state" jsonb NOT NULL DEFAULT '{}',
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("app_name"),
  CONSTRAINT "app_states_state_check" CHECK (jsonb_typeof(state) = 'object'::text)
);
-- Create "requests" table
CREATE TABLE "procurement"."requests" (
  "id" uuid NOT NULL,
  "buyer_id" bigint NULL,
  "status" text NOT NULL DEFAULT 'CLARIFYING',
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "requests_buyer_check" CHECK (buyer_id > 0),
  CONSTRAINT "requests_status_check" CHECK (status = ANY (ARRAY['CLARIFYING'::text, 'REQUIREMENT_READY'::text, 'MATCHING'::text]))
);
-- Create index "requests_buyer" to table: "requests"
CREATE INDEX "requests_buyer" ON "procurement"."requests" ("buyer_id", "created_at" DESC) WHERE (buyer_id IS NOT NULL);
-- Create "usage" table
CREATE TABLE "procurement"."usage" (
  "buyer_id" bigint NOT NULL,
  "period_start" timestamptz NOT NULL,
  "used" integer NOT NULL DEFAULT 0,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("buyer_id", "period_start"),
  CONSTRAINT "usage_buyer_check" CHECK (buyer_id > 0),
  CONSTRAINT "usage_used_check" CHECK (used >= 0)
);
-- Create "user_states" table
CREATE TABLE "procurement"."user_states" (
  "app_name" text NOT NULL,
  "user_id" text NOT NULL,
  "state" jsonb NOT NULL DEFAULT '{}',
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("app_name", "user_id"),
  CONSTRAINT "user_states_state_check" CHECK (jsonb_typeof(state) = 'object'::text)
);
-- Create "request_revisions" table
CREATE TABLE "procurement"."request_revisions" (
  "id" uuid NOT NULL,
  "request_id" uuid NOT NULL,
  "revision_no" integer NOT NULL,
  "parent_revision_id" uuid NULL,
  "status" text NOT NULL DEFAULT 'ACTIVE',
  "item" text NULL,
  "item_resolution_kind" text NULL,
  "item_resolution_label" text NULL,
  "quantity" integer NULL,
  "unit" text NULL,
  "packaging" text NULL,
  "budget_per_unit" bigint NULL,
  "delivery_city" text NULL,
  "delivery_date" date NULL,
  "exact_match_required" boolean NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "request_revisions_parent_revision_id_fkey" FOREIGN KEY ("parent_revision_id") REFERENCES "procurement"."request_revisions" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "request_revisions_request_id_fkey" FOREIGN KEY ("request_id") REFERENCES "procurement"."requests" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "request_revisions_budget_check" CHECK (budget_per_unit >= 0),
  CONSTRAINT "request_revisions_delivery_city_check" CHECK (((char_length(delivery_city) >= 1) AND (char_length(delivery_city) <= 100)) AND (delivery_city !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text)),
  CONSTRAINT "request_revisions_item_check" CHECK (((char_length(item) >= 1) AND (char_length(item) <= 200)) AND (item !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text)),
  CONSTRAINT "request_revisions_no_check" CHECK (revision_no >= 1),
  CONSTRAINT "request_revisions_packaging_check" CHECK (((char_length(packaging) >= 1) AND (char_length(packaging) <= 200)) AND (packaging !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text)),
  CONSTRAINT "request_revisions_quantity_check" CHECK (quantity > 0),
  CONSTRAINT "request_revisions_resolution_check" CHECK (((item_resolution_kind IS NULL) = (item_resolution_label IS NULL)) AND ((item_resolution_kind IS NULL) OR (item_resolution_kind = ANY (ARRAY['CATEGORY_CONFIRMED'::text, 'BUYER_DESCRIBED'::text])))),
  CONSTRAINT "request_revisions_resolution_label_check" CHECK (((char_length(item_resolution_label) >= 1) AND (char_length(item_resolution_label) <= 200)) AND (item_resolution_label !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text)),
  CONSTRAINT "request_revisions_status_check" CHECK (status = ANY (ARRAY['ACTIVE'::text, 'SUPERSEDED'::text])),
  CONSTRAINT "request_revisions_unit_check" CHECK (((char_length(unit) >= 1) AND (char_length(unit) <= 40)) AND (unit !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text))
);
-- Create index "request_revisions_active" to table: "request_revisions"
CREATE UNIQUE INDEX "request_revisions_active" ON "procurement"."request_revisions" ("request_id") WHERE (status = 'ACTIVE'::text);
-- Create index "request_revisions_no" to table: "request_revisions"
CREATE UNIQUE INDEX "request_revisions_no" ON "procurement"."request_revisions" ("request_id", "revision_no");
-- Create "candidate_sets" table
CREATE TABLE "procurement"."candidate_sets" (
  "id" uuid NOT NULL,
  "request_id" uuid NOT NULL,
  "request_revision_id" uuid NOT NULL,
  "buyer_id" bigint NOT NULL,
  "client_request_id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "candidate_sets_request_id_fkey" FOREIGN KEY ("request_id") REFERENCES "procurement"."requests" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "candidate_sets_request_revision_id_fkey" FOREIGN KEY ("request_revision_id") REFERENCES "procurement"."request_revisions" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "candidate_sets_buyer_check" CHECK (buyer_id > 0)
);
-- Create index "candidate_sets_idempotency" to table: "candidate_sets"
CREATE UNIQUE INDEX "candidate_sets_idempotency" ON "procurement"."candidate_sets" ("request_revision_id", "client_request_id");
-- Create index "candidate_sets_request" to table: "candidate_sets"
CREATE INDEX "candidate_sets_request" ON "procurement"."candidate_sets" ("request_id", "created_at" DESC);
-- Create "conversations" table
CREATE TABLE "procurement"."conversations" (
  "id" uuid NOT NULL,
  "buyer_id" bigint NOT NULL,
  "request_id" uuid NOT NULL,
  "title" text NOT NULL DEFAULT '',
  "title_source" text NOT NULL DEFAULT 'AUTO',
  "last_message_preview" text NOT NULL DEFAULT '',
  "deleted_at" timestamptz NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "conversations_request_id_fkey" FOREIGN KEY ("request_id") REFERENCES "procurement"."requests" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "conversations_buyer_check" CHECK (buyer_id > 0),
  CONSTRAINT "conversations_preview_check" CHECK ((char_length(last_message_preview) <= 200) AND (last_message_preview !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text)),
  CONSTRAINT "conversations_title_check" CHECK ((char_length(title) <= 200) AND (title !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text)),
  CONSTRAINT "conversations_title_source_check" CHECK (title_source = ANY (ARRAY['AUTO'::text, 'USER'::text]))
);
-- Create index "conversations_buyer_active" to table: "conversations"
CREATE INDEX "conversations_buyer_active" ON "procurement"."conversations" ("buyer_id", "updated_at" DESC, "id" DESC) WHERE (deleted_at IS NULL);
-- Create index "conversations_request" to table: "conversations"
CREATE UNIQUE INDEX "conversations_request" ON "procurement"."conversations" ("request_id");
-- Create "price_previews" table
CREATE TABLE "procurement"."price_previews" (
  "id" uuid NOT NULL,
  "buyer_id" bigint NULL,
  "request_id" uuid NOT NULL,
  "request_revision_id" uuid NOT NULL,
  "range_lower_amount" bigint NOT NULL,
  "range_upper_amount" bigint NOT NULL,
  "basis_quantity" integer NOT NULL,
  "basis_unit" text NOT NULL,
  "total_lower_amount" bigint NULL,
  "total_upper_amount" bigint NULL,
  "total_quantity" integer NULL,
  "total_unit" text NULL,
  "client_request_id" uuid NOT NULL,
  "expires_at" timestamptz NOT NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "price_previews_request_id_fkey" FOREIGN KEY ("request_id") REFERENCES "procurement"."requests" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "price_previews_request_revision_id_fkey" FOREIGN KEY ("request_revision_id") REFERENCES "procurement"."request_revisions" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "price_previews_basis_check" CHECK ((basis_quantity > 0) AND (char_length(basis_unit) >= 1)),
  CONSTRAINT "price_previews_basis_unit_check" CHECK ((char_length(basis_unit) <= 40) AND (basis_unit !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text)),
  CONSTRAINT "price_previews_buyer_check" CHECK (buyer_id > 0),
  CONSTRAINT "price_previews_range_check" CHECK ((range_lower_amount >= 0) AND (range_upper_amount >= range_lower_amount)),
  CONSTRAINT "price_previews_total_check" CHECK (((total_lower_amount IS NULL) = (total_upper_amount IS NULL)) AND ((total_lower_amount IS NULL) = (total_quantity IS NULL)) AND ((total_lower_amount IS NULL) = (total_unit IS NULL))),
  CONSTRAINT "price_previews_total_unit_check" CHECK ((char_length(total_unit) <= 40) AND (total_unit !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text))
);
-- Create index "price_previews_idempotency" to table: "price_previews"
CREATE UNIQUE INDEX "price_previews_idempotency" ON "procurement"."price_previews" ("buyer_id", "client_request_id");
-- Create index "price_previews_request" to table: "price_previews"
CREATE INDEX "price_previews_request" ON "procurement"."price_previews" ("request_id");
-- Create "confirmed_rfq_snapshots" table
CREATE TABLE "procurement"."confirmed_rfq_snapshots" (
  "id" uuid NOT NULL,
  "buyer_id" bigint NULL,
  "request_id" uuid NOT NULL,
  "request_revision_id" uuid NOT NULL,
  "price_preview_id" uuid NOT NULL,
  "buyer_note" text NOT NULL DEFAULT '',
  "client_request_id" uuid NOT NULL,
  "expires_at" timestamptz NOT NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "confirmed_rfq_snapshots_price_preview_id_fkey" FOREIGN KEY ("price_preview_id") REFERENCES "procurement"."price_previews" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "confirmed_rfq_snapshots_request_id_fkey" FOREIGN KEY ("request_id") REFERENCES "procurement"."requests" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "confirmed_rfq_snapshots_request_revision_id_fkey" FOREIGN KEY ("request_revision_id") REFERENCES "procurement"."request_revisions" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "confirmed_rfq_snapshots_buyer_check" CHECK (buyer_id > 0),
  CONSTRAINT "confirmed_rfq_snapshots_note_check" CHECK ((char_length(buyer_note) <= 500) AND (buyer_note !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text))
);
-- Create index "confirmed_rfq_snapshots_idempotency" to table: "confirmed_rfq_snapshots"
CREATE UNIQUE INDEX "confirmed_rfq_snapshots_idempotency" ON "procurement"."confirmed_rfq_snapshots" ("buyer_id", "client_request_id");
-- Create index "confirmed_rfq_snapshots_request" to table: "confirmed_rfq_snapshots"
CREATE INDEX "confirmed_rfq_snapshots_request" ON "procurement"."confirmed_rfq_snapshots" ("request_id", "created_at" DESC);
-- Create "rfq_publication_approvals" table
CREATE TABLE "procurement"."rfq_publication_approvals" (
  "id" uuid NOT NULL,
  "version" integer NOT NULL DEFAULT 1,
  "buyer_id" bigint NULL,
  "request_id" uuid NOT NULL,
  "request_revision_id" uuid NOT NULL,
  "confirmed_rfq_snapshot_id" uuid NOT NULL,
  "response_deadline" date NULL,
  "audience_mode" text NOT NULL,
  "audience_policy_id" text NULL,
  "audience_policy_version" integer NULL,
  "state" text NOT NULL,
  "schema_version" text NOT NULL,
  "client_request_id" uuid NOT NULL,
  "generated_at" timestamptz NOT NULL,
  "expires_at" timestamptz NOT NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "rfq_publication_approvals_confirmed_rfq_snapshot_id_fkey" FOREIGN KEY ("confirmed_rfq_snapshot_id") REFERENCES "procurement"."confirmed_rfq_snapshots" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "rfq_publication_approvals_request_id_fkey" FOREIGN KEY ("request_id") REFERENCES "procurement"."requests" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "rfq_publication_approvals_request_revision_id_fkey" FOREIGN KEY ("request_revision_id") REFERENCES "procurement"."request_revisions" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "rfq_publication_approvals_buyer_check" CHECK (buyer_id > 0),
  CONSTRAINT "rfq_publication_approvals_mode_check" CHECK (audience_mode = ANY (ARRAY['WISH_POOL'::text, 'DIRECT_RECIPIENTS'::text])),
  CONSTRAINT "rfq_publication_approvals_policy_check" CHECK ((audience_mode = 'WISH_POOL'::text) = ((audience_policy_id IS NOT NULL) AND (audience_policy_version IS NOT NULL))),
  CONSTRAINT "rfq_publication_approvals_schema_check" CHECK (char_length(schema_version) >= 1),
  CONSTRAINT "rfq_publication_approvals_state_check" CHECK (state = ANY (ARRAY['PENDING'::text, 'EXECUTING'::text, 'SUCCEEDED'::text])),
  CONSTRAINT "rfq_publication_approvals_version_check" CHECK (version >= 1)
);
-- Create index "rfq_publication_approvals_idempotency" to table: "rfq_publication_approvals"
CREATE UNIQUE INDEX "rfq_publication_approvals_idempotency" ON "procurement"."rfq_publication_approvals" ("buyer_id", "client_request_id");
-- Create index "rfq_publication_approvals_request" to table: "rfq_publication_approvals"
CREATE INDEX "rfq_publication_approvals_request" ON "procurement"."rfq_publication_approvals" ("request_id", "created_at" DESC);
-- Create "rfq_publications" table
CREATE TABLE "procurement"."rfq_publications" (
  "id" uuid NOT NULL,
  "version" integer NOT NULL DEFAULT 1,
  "buyer_id" bigint NULL,
  "request_id" uuid NOT NULL,
  "request_revision_id" uuid NOT NULL,
  "approval_id" uuid NOT NULL,
  "state" text NOT NULL DEFAULT 'PUBLISHED',
  "schema_version" text NOT NULL,
  "client_request_id" uuid NOT NULL,
  "published_at" timestamptz NOT NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "rfq_publications_approval_id_fkey" FOREIGN KEY ("approval_id") REFERENCES "procurement"."rfq_publication_approvals" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "rfq_publications_request_id_fkey" FOREIGN KEY ("request_id") REFERENCES "procurement"."requests" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "rfq_publications_request_revision_id_fkey" FOREIGN KEY ("request_revision_id") REFERENCES "procurement"."request_revisions" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "rfq_publications_buyer_check" CHECK (buyer_id > 0),
  CONSTRAINT "rfq_publications_schema_check" CHECK (char_length(schema_version) >= 1),
  CONSTRAINT "rfq_publications_state_check" CHECK (state = 'PUBLISHED'::text),
  CONSTRAINT "rfq_publications_version_check" CHECK (version >= 1)
);
-- Create index "rfq_publications_approval" to table: "rfq_publications"
CREATE UNIQUE INDEX "rfq_publications_approval" ON "procurement"."rfq_publications" ("approval_id");
-- Create index "rfq_publications_idempotency" to table: "rfq_publications"
CREATE UNIQUE INDEX "rfq_publications_idempotency" ON "procurement"."rfq_publications" ("buyer_id", "client_request_id");
-- Create index "rfq_publications_request" to table: "rfq_publications"
CREATE INDEX "rfq_publications_request" ON "procurement"."rfq_publications" ("request_id", "created_at" DESC);
-- Create "agent_runs" table
CREATE TABLE "procurement"."agent_runs" (
  "id" uuid NOT NULL,
  "buyer_id" bigint NOT NULL,
  "request_id" uuid NOT NULL,
  "conversation_id" uuid NULL,
  "parent_run_id" uuid NULL,
  "candidate_set_id" uuid NULL,
  "publication_id" uuid NULL,
  "run_type" text NOT NULL,
  "status" text NOT NULL,
  "failure_reason" text NULL,
  "resumed_into_run_id" uuid NULL,
  "result_revision_id" uuid NULL,
  "last_event_seq" bigint NOT NULL DEFAULT 0,
  "min_available_seq" bigint NOT NULL DEFAULT 1,
  "client_request_id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL,
  "started_at" timestamptz NULL,
  "completed_at" timestamptz NULL,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "agent_runs_candidate_set_id_fkey" FOREIGN KEY ("candidate_set_id") REFERENCES "procurement"."candidate_sets" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "agent_runs_conversation_id_fkey" FOREIGN KEY ("conversation_id") REFERENCES "procurement"."conversations" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "agent_runs_parent_run_id_fkey" FOREIGN KEY ("parent_run_id") REFERENCES "procurement"."agent_runs" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "agent_runs_publication_id_fkey" FOREIGN KEY ("publication_id") REFERENCES "procurement"."rfq_publications" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "agent_runs_request_id_fkey" FOREIGN KEY ("request_id") REFERENCES "procurement"."requests" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "agent_runs_result_revision_id_fkey" FOREIGN KEY ("result_revision_id") REFERENCES "procurement"."request_revisions" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "agent_runs_buyer_check" CHECK (buyer_id > 0),
  CONSTRAINT "agent_runs_failure_check" CHECK ((failure_reason IS NULL) OR (failure_reason = ANY (ARRAY['WORKER_LOST'::text, 'SEARCH_FAILED'::text, 'TURN_PIPELINE_ERROR'::text, 'EXPIRED'::text]))),
  CONSTRAINT "agent_runs_seq_check" CHECK ((last_event_seq >= 0) AND (min_available_seq >= 1)),
  CONSTRAINT "agent_runs_status_check" CHECK (status = ANY (ARRAY['QUEUED'::text, 'RUNNING'::text, 'WAITING_FOR_INPUT'::text, 'CANCEL_REQUESTED'::text, 'SUCCEEDED'::text, 'FAILED'::text, 'CANCELLED'::text, 'RESUMED'::text, 'EXPIRED'::text])),
  CONSTRAINT "agent_runs_type_check" CHECK (run_type = ANY (ARRAY['TURN'::text, 'CANDIDATE_SEARCH'::text, 'CLARIFICATION_RESUME'::text, 'PUBLICATION'::text]))
);
-- Create index "agent_runs_idempotency" to table: "agent_runs"
CREATE UNIQUE INDEX "agent_runs_idempotency" ON "procurement"."agent_runs" ("buyer_id", "client_request_id");
-- Create index "agent_runs_request" to table: "agent_runs"
CREATE INDEX "agent_runs_request" ON "procurement"."agent_runs" ("request_id", "created_at" DESC);
-- Create index "agent_runs_unfinished" to table: "agent_runs"
CREATE INDEX "agent_runs_unfinished" ON "procurement"."agent_runs" ("status", "created_at") WHERE (status = ANY (ARRAY['QUEUED'::text, 'RUNNING'::text]));
-- Create "agent_run_artifacts" table
CREATE TABLE "procurement"."agent_run_artifacts" (
  "artifact_id" uuid NOT NULL,
  "version" integer NOT NULL,
  "run_id" uuid NOT NULL,
  "artifact_type" text NOT NULL,
  "status" text NOT NULL,
  "data" jsonb NOT NULL,
  "checksum" text NOT NULL,
  "as_of" timestamptz NOT NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("artifact_id", "version"),
  CONSTRAINT "agent_run_artifacts_run_id_fkey" FOREIGN KEY ("run_id") REFERENCES "procurement"."agent_runs" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "agent_run_artifacts_data_check" CHECK (jsonb_typeof(data) = 'object'::text),
  CONSTRAINT "agent_run_artifacts_version_check" CHECK (version >= 1)
);
-- Create index "agent_run_artifacts_run" to table: "agent_run_artifacts"
CREATE INDEX "agent_run_artifacts_run" ON "procurement"."agent_run_artifacts" ("run_id", "artifact_type");
-- Create "agent_run_events" table
CREATE TABLE "procurement"."agent_run_events" (
  "run_id" uuid NOT NULL,
  "seq" bigint NOT NULL,
  "event_type" text NOT NULL,
  "payload" jsonb NOT NULL DEFAULT '{}',
  "occurred_at" timestamptz NOT NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("run_id", "seq"),
  CONSTRAINT "agent_run_events_run_id_fkey" FOREIGN KEY ("run_id") REFERENCES "procurement"."agent_runs" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "agent_run_events_payload_check" CHECK (jsonb_typeof(payload) = 'object'::text),
  CONSTRAINT "agent_run_events_seq_check" CHECK (seq >= 1),
  CONSTRAINT "agent_run_events_type_check" CHECK (char_length(event_type) >= 1)
);
-- Create "agent_run_inputs" table
CREATE TABLE "procurement"."agent_run_inputs" (
  "run_id" uuid NOT NULL,
  "buyer_message" text NOT NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("run_id"),
  CONSTRAINT "agent_run_inputs_run_id_fkey" FOREIGN KEY ("run_id") REFERENCES "procurement"."agent_runs" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "agent_run_inputs_message_check" CHECK (((char_length(buyer_message) >= 1) AND (char_length(buyer_message) <= 2000)) AND (buyer_message !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text))
);
-- Create "agent_run_interrupts" table
CREATE TABLE "procurement"."agent_run_interrupts" (
  "id" uuid NOT NULL,
  "run_id" uuid NOT NULL,
  "interrupt_kind" text NOT NULL,
  "status" text NOT NULL,
  "payload" jsonb NOT NULL DEFAULT '{}',
  "facts_as_of" timestamptz NULL,
  "resolved_by_child_run_id" uuid NULL,
  "created_at" timestamptz NOT NULL,
  "resolved_at" timestamptz NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "agent_run_interrupts_resolved_by_child_run_id_fkey" FOREIGN KEY ("resolved_by_child_run_id") REFERENCES "procurement"."agent_runs" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "agent_run_interrupts_run_id_fkey" FOREIGN KEY ("run_id") REFERENCES "procurement"."agent_runs" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "agent_run_interrupts_kind_check" CHECK (interrupt_kind = 'CLARIFICATION'::text),
  CONSTRAINT "agent_run_interrupts_payload_check" CHECK (jsonb_typeof(payload) = 'object'::text),
  CONSTRAINT "agent_run_interrupts_status_check" CHECK (status = ANY (ARRAY['PENDING'::text, 'RESOLVED'::text]))
);
-- Create index "agent_run_interrupts_run" to table: "agent_run_interrupts"
CREATE INDEX "agent_run_interrupts_run" ON "procurement"."agent_run_interrupts" ("run_id");
-- Create "seller_responses" table
CREATE TABLE "procurement"."seller_responses" (
  "id" uuid NOT NULL,
  "publication_id" uuid NOT NULL,
  "request_id" uuid NOT NULL,
  "store_id" bigint NOT NULL,
  "latest_version" integer NOT NULL DEFAULT 0,
  "withdrawn_version" integer NULL,
  "withdrawn_at" timestamptz NULL,
  "write_closed_at" timestamptz NULL,
  "write_closed_reason" text NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "seller_responses_publication_id_fkey" FOREIGN KEY ("publication_id") REFERENCES "procurement"."rfq_publications" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "seller_responses_request_id_fkey" FOREIGN KEY ("request_id") REFERENCES "procurement"."requests" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "seller_responses_latest_check" CHECK (latest_version >= 0),
  CONSTRAINT "seller_responses_store_check" CHECK (store_id > 0),
  CONSTRAINT "seller_responses_withdrawn_pair" CHECK ((withdrawn_version IS NULL) = (withdrawn_at IS NULL)),
  CONSTRAINT "seller_responses_write_closed_pair" CHECK ((write_closed_at IS NULL) = (write_closed_reason IS NULL)),
  CONSTRAINT "seller_responses_write_closed_reason" CHECK ((write_closed_reason IS NULL) OR (write_closed_reason = 'DECLINED'::text))
);
-- Create index "seller_responses_publication_store" to table: "seller_responses"
CREATE UNIQUE INDEX "seller_responses_publication_store" ON "procurement"."seller_responses" ("publication_id", "store_id");
-- Create index "seller_responses_request_submitted" to table: "seller_responses"
CREATE INDEX "seller_responses_request_submitted" ON "procurement"."seller_responses" ("request_id", "created_at" DESC) WHERE (latest_version > 0);
-- Create "buyer_quote_acceptances" table
CREATE TABLE "procurement"."buyer_quote_acceptances" (
  "id" uuid NOT NULL,
  "buyer_id" bigint NULL,
  "request_id" uuid NOT NULL,
  "publication_id" uuid NOT NULL,
  "seller_response_id" uuid NOT NULL,
  "seller_response_version" integer NOT NULL,
  "seller_response_payload_hash" text NOT NULL,
  "confirmed_rfq_payload_hash" text NOT NULL,
  "store_id" bigint NOT NULL,
  "currency" text NOT NULL,
  "goods_amount" bigint NOT NULL,
  "shipping_amount" bigint NOT NULL,
  "tax_amount" bigint NOT NULL,
  "accepted_deal_price" bigint NOT NULL,
  "state" text NOT NULL DEFAULT 'ACTIVE',
  "order_provisioning_state" text NOT NULL DEFAULT 'CREATING_ORDER',
  "order_failure_reason" text NULL,
  "order_no" text NULL,
  "payment_deadline" timestamptz NULL,
  "client_request_id" uuid NOT NULL,
  "accepted_at" timestamptz NOT NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "buyer_quote_acceptances_publication_id_fkey" FOREIGN KEY ("publication_id") REFERENCES "procurement"."rfq_publications" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "buyer_quote_acceptances_request_id_fkey" FOREIGN KEY ("request_id") REFERENCES "procurement"."requests" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "buyer_quote_acceptances_seller_response_id_fkey" FOREIGN KEY ("seller_response_id") REFERENCES "procurement"."seller_responses" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "buyer_quote_acceptances_amounts_check" CHECK ((goods_amount >= 0) AND (shipping_amount >= 0) AND (tax_amount >= 0)),
  CONSTRAINT "buyer_quote_acceptances_buyer_check" CHECK (buyer_id > 0),
  CONSTRAINT "buyer_quote_acceptances_failed_check" CHECK ((state = 'ORDER_FAILED'::text) = (order_provisioning_state = 'ORDER_FAILED'::text)),
  CONSTRAINT "buyer_quote_acceptances_order_no_check" CHECK ((order_provisioning_state = 'ORDER_CREATED'::text) = (order_no IS NOT NULL)),
  CONSTRAINT "buyer_quote_acceptances_price_check" CHECK (accepted_deal_price = ((goods_amount + shipping_amount) + tax_amount)),
  CONSTRAINT "buyer_quote_acceptances_provisioning_check" CHECK (order_provisioning_state = ANY (ARRAY['CREATING_ORDER'::text, 'ORDER_CREATED'::text, 'ORDER_FAILED'::text])),
  CONSTRAINT "buyer_quote_acceptances_state_check" CHECK (state = ANY (ARRAY['ACTIVE'::text, 'ORDER_FAILED'::text])),
  CONSTRAINT "buyer_quote_acceptances_store_check" CHECK (store_id > 0),
  CONSTRAINT "buyer_quote_acceptances_version_check" CHECK (seller_response_version >= 1)
);
-- Create index "buyer_quote_acceptances_idempotency" to table: "buyer_quote_acceptances"
CREATE UNIQUE INDEX "buyer_quote_acceptances_idempotency" ON "procurement"."buyer_quote_acceptances" ("buyer_id", "client_request_id");
-- Create index "buyer_quote_acceptances_live" to table: "buyer_quote_acceptances"
CREATE UNIQUE INDEX "buyer_quote_acceptances_live" ON "procurement"."buyer_quote_acceptances" ("request_id") WHERE (state = 'ACTIVE'::text);
-- Create index "buyer_quote_acceptances_order_no" to table: "buyer_quote_acceptances"
CREATE UNIQUE INDEX "buyer_quote_acceptances_order_no" ON "procurement"."buyer_quote_acceptances" ("order_no") WHERE (order_no IS NOT NULL);
-- Create index "buyer_quote_acceptances_response" to table: "buyer_quote_acceptances"
CREATE INDEX "buyer_quote_acceptances_response" ON "procurement"."buyer_quote_acceptances" ("seller_response_id", "seller_response_version");
-- Create "buyer_shortlist_entries" table
CREATE TABLE "procurement"."buyer_shortlist_entries" (
  "id" uuid NOT NULL,
  "request_id" uuid NOT NULL,
  "seller_response_id" uuid NOT NULL,
  "seller_response_version" integer NOT NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "buyer_shortlist_entries_request_id_fkey" FOREIGN KEY ("request_id") REFERENCES "procurement"."requests" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "buyer_shortlist_entries_seller_response_id_fkey" FOREIGN KEY ("seller_response_id") REFERENCES "procurement"."seller_responses" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "buyer_shortlist_entries_version_check" CHECK (seller_response_version >= 1)
);
-- Create index "buyer_shortlist_entries_response" to table: "buyer_shortlist_entries"
CREATE UNIQUE INDEX "buyer_shortlist_entries_response" ON "procurement"."buyer_shortlist_entries" ("request_id", "seller_response_id");
-- Create "candidates" table
CREATE TABLE "procurement"."candidates" (
  "id" uuid NOT NULL,
  "candidate_set_id" uuid NOT NULL,
  "store_id" bigint NOT NULL,
  "source_id" text NOT NULL,
  "score" integer NOT NULL DEFAULT 0,
  "rank" integer NOT NULL DEFAULT 0,
  "matched_facts" jsonb NOT NULL DEFAULT '[]',
  "unverifiable_facts" jsonb NOT NULL DEFAULT '[]',
  "facts_as_of" timestamptz NOT NULL,
  "selected" boolean NOT NULL DEFAULT false,
  "selected_at" timestamptz NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "candidates_candidate_set_id_fkey" FOREIGN KEY ("candidate_set_id") REFERENCES "procurement"."candidate_sets" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "candidates_matched_check" CHECK (jsonb_typeof(matched_facts) = 'array'::text),
  CONSTRAINT "candidates_selected_check" CHECK (selected = (selected_at IS NOT NULL)),
  CONSTRAINT "candidates_source_check" CHECK (char_length(source_id) >= 1),
  CONSTRAINT "candidates_store_check" CHECK (store_id > 0),
  CONSTRAINT "candidates_unverifiable_check" CHECK (jsonb_typeof(unverifiable_facts) = 'array'::text)
);
-- Create index "candidates_set_rank" to table: "candidates"
CREATE INDEX "candidates_set_rank" ON "procurement"."candidates" ("candidate_set_id", "rank");
-- Create index "candidates_set_store" to table: "candidates"
CREATE UNIQUE INDEX "candidates_set_store" ON "procurement"."candidates" ("candidate_set_id", "store_id");
-- Create "sessions" table
CREATE TABLE "procurement"."sessions" (
  "app_name" text NOT NULL,
  "user_id" text NOT NULL,
  "id" text NOT NULL,
  "state" jsonb NOT NULL DEFAULT '{}',
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("app_name", "user_id", "id"),
  CONSTRAINT "sessions_key_check" CHECK ((app_name <> ''::text) AND (user_id <> ''::text) AND (id <> ''::text)),
  CONSTRAINT "sessions_state_check" CHECK (jsonb_typeof(state) = 'object'::text)
);
-- Create "events" table
CREATE TABLE "procurement"."events" (
  "app_name" text NOT NULL,
  "user_id" text NOT NULL,
  "session_id" text NOT NULL,
  "id" text NOT NULL,
  "timestamp" timestamptz NOT NULL,
  "seq" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "event" jsonb NOT NULL,
  PRIMARY KEY ("app_name", "user_id", "session_id", "id"),
  CONSTRAINT "events_app_name_user_id_session_id_fkey" FOREIGN KEY ("app_name", "user_id", "session_id") REFERENCES "procurement"."sessions" ("app_name", "user_id", "id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "events_event_check" CHECK (jsonb_typeof(event) = 'object'::text),
  CONSTRAINT "events_id_check" CHECK (id <> ''::text)
);
-- Create index "events_session_time" to table: "events"
CREATE INDEX "events_session_time" ON "procurement"."events" ("app_name", "user_id", "session_id", "timestamp", "seq");
-- Create "rfq_publication_approval_recipients" table
CREATE TABLE "procurement"."rfq_publication_approval_recipients" (
  "approval_id" uuid NOT NULL,
  "position" integer NOT NULL,
  "store_id" bigint NOT NULL,
  PRIMARY KEY ("approval_id", "position"),
  CONSTRAINT "rfq_publication_approval_recipients_approval_id_fkey" FOREIGN KEY ("approval_id") REFERENCES "procurement"."rfq_publication_approvals" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "rfq_publication_approval_recipients_position_check" CHECK ("position" >= 0),
  CONSTRAINT "rfq_publication_approval_recipients_store_check" CHECK (store_id > 0)
);
-- Create index "rfq_publication_approval_recipients_store" to table: "rfq_publication_approval_recipients"
CREATE UNIQUE INDEX "rfq_publication_approval_recipients_store" ON "procurement"."rfq_publication_approval_recipients" ("approval_id", "store_id");
-- Create "rfq_publication_invites" table
CREATE TABLE "procurement"."rfq_publication_invites" (
  "id" uuid NOT NULL,
  "publication_id" uuid NOT NULL,
  "position" integer NOT NULL,
  "store_id" bigint NOT NULL,
  "delivery_state" text NOT NULL DEFAULT 'QUEUED',
  "delivery_state_at" timestamptz NOT NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "rfq_publication_invites_publication_id_fkey" FOREIGN KEY ("publication_id") REFERENCES "procurement"."rfq_publications" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "rfq_publication_invites_position_check" CHECK ("position" >= 0),
  CONSTRAINT "rfq_publication_invites_state_check" CHECK (delivery_state = ANY (ARRAY['QUEUED'::text, 'SENT'::text, 'FAILED'::text])),
  CONSTRAINT "rfq_publication_invites_store_check" CHECK (store_id > 0)
);
-- Create index "rfq_publication_invites_position" to table: "rfq_publication_invites"
CREATE UNIQUE INDEX "rfq_publication_invites_position" ON "procurement"."rfq_publication_invites" ("publication_id", "position");
-- Create index "rfq_publication_invites_store" to table: "rfq_publication_invites"
CREATE UNIQUE INDEX "rfq_publication_invites_store" ON "procurement"."rfq_publication_invites" ("publication_id", "store_id");
-- Create "seller_response_drafts" table
CREATE TABLE "procurement"."seller_response_drafts" (
  "seller_response_id" uuid NOT NULL,
  "payload" jsonb NOT NULL,
  "version" integer NOT NULL DEFAULT 1,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("seller_response_id"),
  CONSTRAINT "seller_response_drafts_seller_response_id_fkey" FOREIGN KEY ("seller_response_id") REFERENCES "procurement"."seller_responses" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "seller_response_drafts_payload_check" CHECK (jsonb_typeof(payload) = 'object'::text),
  CONSTRAINT "seller_response_drafts_version_check" CHECK (version >= 1)
);
-- Create "seller_response_versions" table
CREATE TABLE "procurement"."seller_response_versions" (
  "id" uuid NOT NULL,
  "seller_response_id" uuid NOT NULL,
  "version" integer NOT NULL,
  "response_type" text NOT NULL,
  "offered_quality_grade" text NOT NULL,
  "offered_quality_inspection_notes" text NOT NULL,
  "unit_basis_quantity" numeric(14,3) NOT NULL,
  "unit_basis_unit" text NOT NULL,
  "offered_quantity" numeric(14,3) NOT NULL,
  "offered_quantity_unit" text NOT NULL,
  "conversion_basis" text NOT NULL,
  "currency" text NOT NULL,
  "goods_amount" bigint NOT NULL,
  "shipping_amount" bigint NOT NULL,
  "tax_amount" bigint NOT NULL,
  "tax_inclusion_basis" text NOT NULL,
  "total_amount" bigint NOT NULL,
  "lead_time_days" integer NULL,
  "promised_delivery_at" timestamptz NULL,
  "valid_until" timestamptz NOT NULL,
  "terms_note" text NOT NULL DEFAULT '',
  "deviation_disclosure" text NOT NULL DEFAULT '',
  "outside_buyer_requested_range" boolean NOT NULL,
  "publication_version" integer NOT NULL,
  "client_request_id" uuid NOT NULL,
  "submitted_at" timestamptz NOT NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "seller_response_versions_seller_response_id_fkey" FOREIGN KEY ("seller_response_id") REFERENCES "procurement"."seller_responses" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "seller_response_versions_amounts_check" CHECK ((goods_amount >= 0) AND (shipping_amount >= 0) AND (tax_amount >= 0)),
  CONSTRAINT "seller_response_versions_conversion_basis_check" CHECK ((char_length(conversion_basis) <= 500) AND (conversion_basis !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text)),
  CONSTRAINT "seller_response_versions_deviation_check" CHECK ((char_length(deviation_disclosure) <= 1000) AND (deviation_disclosure !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text)),
  CONSTRAINT "seller_response_versions_notes_check" CHECK ((char_length(offered_quality_inspection_notes) <= 1000) AND (offered_quality_inspection_notes !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text)),
  CONSTRAINT "seller_response_versions_offered_unit_check" CHECK (((char_length(offered_quantity_unit) >= 1) AND (char_length(offered_quantity_unit) <= 40)) AND (offered_quantity_unit !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text)),
  CONSTRAINT "seller_response_versions_publication_version_check" CHECK (publication_version >= 1),
  CONSTRAINT "seller_response_versions_quality_grade_check" CHECK (((char_length(offered_quality_grade) >= 1) AND (char_length(offered_quality_grade) <= 100)) AND (offered_quality_grade !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text)),
  CONSTRAINT "seller_response_versions_quantities_check" CHECK ((unit_basis_quantity > (0)::numeric) AND (offered_quantity > (0)::numeric)),
  CONSTRAINT "seller_response_versions_tax_basis_check" CHECK (tax_inclusion_basis = ANY (ARRAY['TAX_INCLUDED'::text, 'TAX_EXCLUDED'::text])),
  CONSTRAINT "seller_response_versions_terms_check" CHECK ((char_length(terms_note) <= 1000) AND (terms_note !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text)),
  CONSTRAINT "seller_response_versions_total_check" CHECK (total_amount = ((goods_amount + shipping_amount) + tax_amount)),
  CONSTRAINT "seller_response_versions_type_check" CHECK (response_type = ANY (ARRAY['MANUAL_QUOTE'::text, 'ACCEPT_NON_PRICE_TERMS_WITH_QUOTE'::text])),
  CONSTRAINT "seller_response_versions_unit_basis_unit_check" CHECK (((char_length(unit_basis_unit) >= 1) AND (char_length(unit_basis_unit) <= 40)) AND (unit_basis_unit !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text)),
  CONSTRAINT "seller_response_versions_version_check" CHECK (version >= 1)
);
-- Create index "seller_response_versions_idempotency" to table: "seller_response_versions"
CREATE UNIQUE INDEX "seller_response_versions_idempotency" ON "procurement"."seller_response_versions" ("seller_response_id", "client_request_id");
-- Create index "seller_response_versions_version" to table: "seller_response_versions"
CREATE UNIQUE INDEX "seller_response_versions_version" ON "procurement"."seller_response_versions" ("seller_response_id", "version");
-- Create "turns" table
CREATE TABLE "procurement"."turns" (
  "id" uuid NOT NULL,
  "request_id" uuid NOT NULL,
  "conversation_id" uuid NOT NULL,
  "buyer_id" bigint NOT NULL,
  "result_revision_id" uuid NOT NULL,
  "source" text NOT NULL,
  "buyer_message" text NOT NULL DEFAULT '',
  "assistant_reply" text NOT NULL DEFAULT '',
  "steps" jsonb NOT NULL DEFAULT '[]',
  "recorded_this_turn" jsonb NOT NULL DEFAULT '[]',
  "client_request_id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "turns_conversation_id_fkey" FOREIGN KEY ("conversation_id") REFERENCES "procurement"."conversations" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "turns_request_id_fkey" FOREIGN KEY ("request_id") REFERENCES "procurement"."requests" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "turns_result_revision_id_fkey" FOREIGN KEY ("result_revision_id") REFERENCES "procurement"."request_revisions" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "turns_assistant_reply_check" CHECK ((char_length(assistant_reply) <= 4000) AND (assistant_reply !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text)),
  CONSTRAINT "turns_buyer_check" CHECK (buyer_id > 0),
  CONSTRAINT "turns_buyer_message_check" CHECK ((char_length(buyer_message) <= 2000) AND (buyer_message !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text)),
  CONSTRAINT "turns_recorded_check" CHECK (jsonb_typeof(recorded_this_turn) = 'array'::text),
  CONSTRAINT "turns_source_check" CHECK (source = ANY (ARRAY['LLM_EXTRACT'::text, 'ANSWER'::text, 'REWIND'::text])),
  CONSTRAINT "turns_steps_check" CHECK (jsonb_typeof(steps) = 'array'::text)
);
-- Create index "turns_idempotency" to table: "turns"
CREATE UNIQUE INDEX "turns_idempotency" ON "procurement"."turns" ("buyer_id", "client_request_id");
-- Create index "turns_request" to table: "turns"
CREATE INDEX "turns_request" ON "procurement"."turns" ("request_id", "created_at" DESC);
