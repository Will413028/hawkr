-- Add new schema named "moderation"
CREATE SCHEMA "moderation";
-- Create "cases" table
CREATE TABLE "moderation"."cases" (
  "id" uuid NOT NULL,
  "target_kind" text NOT NULL,
  "content_id" uuid NULL,
  "buyer_id" bigint NULL,
  "version" bigint NOT NULL DEFAULT 1,
  "opened_at" timestamptz NOT NULL,
  "decision" text NULL,
  "decision_category" text NULL,
  "decision_action" text NULL,
  "decided_by_admin_id" bigint NULL,
  "closed_at" timestamptz NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "cases_action_target_check" CHECK ((decision_action IS NULL) OR ((decision_action = 'HIDE'::text) AND (target_kind = ANY (ARRAY['COMMENT'::text, 'FORUM_ENTRY'::text]))) OR ((decision_action = ANY (ARRAY['WARN'::text, 'BAN'::text])) AND (target_kind = ANY (ARRAY['COMMENT'::text, 'FORUM_ENTRY'::text, 'BUYER'::text]))) OR ((decision_action = 'TAKEDOWN'::text) AND (target_kind = ANY (ARRAY['POST'::text, 'COMMENT'::text, 'FORUM_ENTRY'::text])))),
  CONSTRAINT "cases_closed_together_check" CHECK (((decision IS NULL) AND (decided_by_admin_id IS NULL) AND (closed_at IS NULL)) OR ((decision IS NOT NULL) AND (decided_by_admin_id IS NOT NULL) AND (closed_at IS NOT NULL) AND (closed_at >= opened_at))),
  CONSTRAINT "cases_decided_by_check" CHECK (decided_by_admin_id > 0),
  CONSTRAINT "cases_decision_action_check" CHECK (decision_action = ANY (ARRAY['HIDE'::text, 'WARN'::text, 'BAN'::text, 'TAKEDOWN'::text])),
  CONSTRAINT "cases_decision_category_check" CHECK (decision_category = ANY (ARRAY['FOOD_SAFETY'::text, 'PROHIBITED_GOODS'::text, 'UNLICENSED_SALE'::text, 'MISLEADING_INFO'::text, 'FAKE_CERTIFICATION'::text, 'COUNTERFEIT'::text, 'IP_INFRINGEMENT'::text, 'FRAUD'::text, 'OFF_PLATFORM_TRADE'::text, 'PRIVACY'::text, 'IMPERSONATION'::text, 'PLATFORM_ABUSE'::text, 'SPAM'::text, 'HARASSMENT'::text, 'HATE'::text, 'SEXUAL_CONTENT'::text])),
  CONSTRAINT "cases_decision_check" CHECK (decision = ANY (ARRAY['VIOLATION'::text, 'NO_VIOLATION'::text])),
  CONSTRAINT "cases_decision_shape_check" CHECK ((decision IS NULL) OR ((decision = 'VIOLATION'::text) AND (decision_category IS NOT NULL) AND (decision_action IS NOT NULL)) OR ((decision = 'NO_VIOLATION'::text) AND (decision_category IS NULL) AND (decision_action IS NULL))),
  CONSTRAINT "cases_target_check" CHECK (((target_kind = 'BUYER'::text) AND (buyer_id > 0) AND (content_id IS NULL)) OR ((target_kind <> 'BUYER'::text) AND (content_id IS NOT NULL) AND (buyer_id IS NULL))),
  CONSTRAINT "cases_target_kind_check" CHECK (target_kind = ANY (ARRAY['POST'::text, 'COMMENT'::text, 'FORUM_ENTRY'::text, 'BUYER'::text])),
  CONSTRAINT "cases_version_check" CHECK (version >= 1)
);
-- Create index "cases_closed_by_time" to table: "cases"
CREATE INDEX "cases_closed_by_time" ON "moderation"."cases" ("closed_at") WHERE (closed_at IS NOT NULL);
-- Create index "cases_one_open_buyer" to table: "cases"
CREATE UNIQUE INDEX "cases_one_open_buyer" ON "moderation"."cases" ("buyer_id") WHERE ((closed_at IS NULL) AND (buyer_id IS NOT NULL));
-- Create index "cases_one_open_content" to table: "cases"
CREATE UNIQUE INDEX "cases_one_open_content" ON "moderation"."cases" ("target_kind", "content_id") WHERE ((closed_at IS NULL) AND (content_id IS NOT NULL));
-- Create index "cases_open_by_time" to table: "cases"
CREATE INDEX "cases_open_by_time" ON "moderation"."cases" ("opened_at", "id") WHERE (closed_at IS NULL);
-- Create "media_ref_sync" table
CREATE TABLE "moderation"."media_ref_sync" (
  "ref_type" text NOT NULL,
  "ref_id" text NOT NULL,
  "seq" bigint NOT NULL DEFAULT 1,
  "synced_seq" bigint NOT NULL DEFAULT 0,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("ref_type", "ref_id")
);
-- Create index "media_ref_sync_unsynced" to table: "media_ref_sync"
CREATE INDEX "media_ref_sync_unsynced" ON "moderation"."media_ref_sync" ("updated_at") WHERE (synced_seq < seq);
-- Create "reporter_daily_counts" table
CREATE TABLE "moderation"."reporter_daily_counts" (
  "reporter_buyer_id" bigint NOT NULL,
  "day" date NOT NULL,
  "submitted" integer NOT NULL,
  PRIMARY KEY ("reporter_buyer_id"),
  CONSTRAINT "reporter_daily_counts_reporter_check" CHECK (reporter_buyer_id > 0),
  CONSTRAINT "reporter_daily_counts_submitted_check" CHECK ((submitted >= 1) AND (submitted <= 20))
);
-- Create "retired_assets" table
CREATE TABLE "moderation"."retired_assets" (
  "ref_type" text NOT NULL,
  "ref_id" text NOT NULL,
  "asset_id" text NOT NULL,
  "retired_at" timestamptz NOT NULL,
  "unlinked_at" timestamptz NULL,
  PRIMARY KEY ("ref_type", "ref_id", "asset_id")
);
-- Create index "retired_assets_retired_at" to table: "retired_assets"
CREATE INDEX "retired_assets_retired_at" ON "moderation"."retired_assets" ("retired_at");
-- Create "case_snapshots" table
CREATE TABLE "moderation"."case_snapshots" (
  "id" uuid NOT NULL,
  "case_id" uuid NOT NULL,
  "content_sha256" bytea NOT NULL,
  "content" text NOT NULL,
  "asset_ids" text[] NOT NULL DEFAULT '{}',
  "captured_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "case_snapshots_case_content_key" UNIQUE ("case_id", "content_sha256"),
  CONSTRAINT "case_snapshots_case_id_fkey" FOREIGN KEY ("case_id") REFERENCES "moderation"."cases" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "case_snapshots_content_check" CHECK (char_length(content) >= 2),
  CONSTRAINT "case_snapshots_sha256_check" CHECK (octet_length(content_sha256) = 32)
);
-- Create "reports" table
CREATE TABLE "moderation"."reports" (
  "id" uuid NOT NULL,
  "case_id" uuid NOT NULL,
  "reporter_buyer_id" bigint NOT NULL,
  "category" text NOT NULL,
  "description" text NULL,
  "snapshot_id" uuid NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "reports_case_reporter_key" UNIQUE ("case_id", "reporter_buyer_id"),
  CONSTRAINT "reports_case_id_fkey" FOREIGN KEY ("case_id") REFERENCES "moderation"."cases" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "reports_snapshot_id_fkey" FOREIGN KEY ("snapshot_id") REFERENCES "moderation"."case_snapshots" ("id") ON UPDATE NO ACTION ON DELETE SET NULL,
  CONSTRAINT "reports_category_check" CHECK (category = ANY (ARRAY['SPAM'::text, 'HARASSMENT'::text, 'HATE'::text, 'SEXUAL_CONTENT'::text, 'MISLEADING_INFO'::text, 'FOOD_SAFETY'::text, 'FRAUD'::text, 'OTHER'::text])),
  CONSTRAINT "reports_description_check" CHECK ((description IS NULL) OR ((char_length(description) >= 1) AND (char_length(description) <= 500) AND (description !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text))),
  CONSTRAINT "reports_reporter_check" CHECK (reporter_buyer_id > 0)
);
-- Create index "reports_case_time" to table: "reports"
CREATE INDEX "reports_case_time" ON "moderation"."reports" ("case_id", "created_at", "id");
-- Create index "reports_snapshot" to table: "reports"
CREATE INDEX "reports_snapshot" ON "moderation"."reports" ("snapshot_id") WHERE (snapshot_id IS NOT NULL);
