-- Create "procurement_answer_patches" table
CREATE TABLE "procurement_answer_patches" (
  "id" bigint NOT NULL,
  "request_id" bigint NOT NULL,
  "buyer_id" bigint NOT NULL,
  "base_revision_id" bigint NULL,
  "result_revision_id" bigint NOT NULL,
  "source" text NOT NULL,
  "payload_json" jsonb NOT NULL DEFAULT '{}',
  "client_request_id" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id")
);
-- Create index "idx_procurement_answer_patches_request" to table: "procurement_answer_patches"
CREATE INDEX "idx_procurement_answer_patches_request" ON "procurement_answer_patches" ("request_id", "created_at" DESC);
-- Create index "uq_procurement_answer_patches_idempotency" to table: "procurement_answer_patches"
CREATE UNIQUE INDEX "uq_procurement_answer_patches_idempotency" ON "procurement_answer_patches" ("buyer_id", "client_request_id");
-- Create "procurement_conversations" table
CREATE TABLE "procurement_conversations" (
  "id" bigint NOT NULL,
  "buyer_id" bigint NOT NULL,
  "request_id" bigint NOT NULL,
  "adk_session_id" text NOT NULL DEFAULT '',
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id")
);
-- Create index "idx_procurement_conversations_buyer" to table: "procurement_conversations"
CREATE INDEX "idx_procurement_conversations_buyer" ON "procurement_conversations" ("buyer_id", "created_at" DESC);
-- Create index "uq_procurement_conversations_request" to table: "procurement_conversations"
CREATE UNIQUE INDEX "uq_procurement_conversations_request" ON "procurement_conversations" ("request_id");
-- Create "procurement_request_revisions" table
CREATE TABLE "procurement_request_revisions" (
  "id" bigint NOT NULL,
  "request_id" bigint NOT NULL,
  "revision_no" integer NOT NULL,
  "parent_revision_id" bigint NULL,
  "branch_id" bigint NOT NULL,
  "slots_json" jsonb NOT NULL DEFAULT '{}',
  "slots_hash" text NOT NULL,
  "status" text NOT NULL DEFAULT 'ACTIVE',
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id")
);
-- Create index "idx_procurement_request_revisions_branch" to table: "procurement_request_revisions"
CREATE INDEX "idx_procurement_request_revisions_branch" ON "procurement_request_revisions" ("request_id", "branch_id", "revision_no" DESC);
-- Create index "uq_procurement_request_revisions_active" to table: "procurement_request_revisions"
CREATE UNIQUE INDEX "uq_procurement_request_revisions_active" ON "procurement_request_revisions" ("request_id") WHERE (status = 'ACTIVE'::text);
-- Create index "uq_procurement_request_revisions_no" to table: "procurement_request_revisions"
CREATE UNIQUE INDEX "uq_procurement_request_revisions_no" ON "procurement_request_revisions" ("request_id", "revision_no");
-- Create "procurement_requests" table
CREATE TABLE "procurement_requests" (
  "id" bigint NOT NULL,
  "buyer_id" bigint NOT NULL,
  "status" text NOT NULL DEFAULT 'CLARIFYING',
  "active_revision_id" bigint NULL,
  "version" integer NOT NULL DEFAULT 1,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id")
);
-- Create index "idx_procurement_requests_buyer" to table: "procurement_requests"
CREATE INDEX "idx_procurement_requests_buyer" ON "procurement_requests" ("buyer_id", "created_at" DESC);
