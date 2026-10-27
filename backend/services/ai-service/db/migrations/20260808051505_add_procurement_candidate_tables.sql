-- Create "procurement_candidate_selections" table
CREATE TABLE "procurement_candidate_selections" (
  "id" bigint NOT NULL,
  "request_id" bigint NOT NULL,
  "request_revision_id" bigint NOT NULL,
  "buyer_id" bigint NOT NULL,
  "candidate_set_id" bigint NOT NULL,
  "lane" text NOT NULL,
  "candidate_id" bigint NOT NULL,
  "selected" boolean NOT NULL DEFAULT true,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id")
);
-- Create index "idx_procurement_candidate_selections_request" to table: "procurement_candidate_selections"
CREATE INDEX "idx_procurement_candidate_selections_request" ON "procurement_candidate_selections" ("request_id", "created_at" DESC);
-- Create index "uq_procurement_candidate_selections" to table: "procurement_candidate_selections"
CREATE UNIQUE INDEX "uq_procurement_candidate_selections" ON "procurement_candidate_selections" ("request_revision_id", "lane", "candidate_id");
-- Create "procurement_candidate_sets" table
CREATE TABLE "procurement_candidate_sets" (
  "id" bigint NOT NULL,
  "request_id" bigint NOT NULL,
  "request_revision_id" bigint NOT NULL,
  "buyer_id" bigint NOT NULL,
  "policy_version" integer NOT NULL DEFAULT 1,
  "status" text NOT NULL DEFAULT 'QUEUED',
  "platform_lane_status" text NOT NULL DEFAULT 'PENDING',
  "external_lane_status" text NOT NULL DEFAULT 'PENDING',
  "error_code" text NOT NULL DEFAULT '',
  "lease_owner" text NOT NULL DEFAULT '',
  "lease_expires_at" timestamptz NULL,
  "fencing_token" bigint NOT NULL DEFAULT 0,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id")
);
-- Create index "idx_procurement_candidate_sets_lease" to table: "procurement_candidate_sets"
CREATE INDEX "idx_procurement_candidate_sets_lease" ON "procurement_candidate_sets" ("status", "lease_expires_at") WHERE (status = ANY (ARRAY['QUEUED'::text, 'RUNNING'::text]));
-- Create index "idx_procurement_candidate_sets_request" to table: "procurement_candidate_sets"
CREATE INDEX "idx_procurement_candidate_sets_request" ON "procurement_candidate_sets" ("request_id", "created_at" DESC);
-- Create index "uq_procurement_candidate_sets_idempotency" to table: "procurement_candidate_sets"
CREATE UNIQUE INDEX "uq_procurement_candidate_sets_idempotency" ON "procurement_candidate_sets" ("request_revision_id", "policy_version");
-- Create "procurement_candidates" table
CREATE TABLE "procurement_candidates" (
  "id" bigint NOT NULL,
  "candidate_set_id" bigint NOT NULL,
  "lane" text NOT NULL,
  "source_id" text NOT NULL,
  "display_name" text NOT NULL,
  "score" integer NOT NULL DEFAULT 0,
  "matched_facts_json" jsonb NOT NULL DEFAULT '[]',
  "unverifiable_facts_json" jsonb NOT NULL DEFAULT '[]',
  "facts_as_of" timestamptz NOT NULL,
  "rank" integer NOT NULL DEFAULT 0,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id")
);
-- Create index "idx_procurement_candidates_set" to table: "procurement_candidates"
CREATE INDEX "idx_procurement_candidates_set" ON "procurement_candidates" ("candidate_set_id", "lane", "rank");
