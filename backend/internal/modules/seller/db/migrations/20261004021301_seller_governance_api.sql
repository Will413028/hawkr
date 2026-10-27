-- Create "evidence_accesses" table
CREATE TABLE "seller"."evidence_accesses" (
  "access_id" uuid NOT NULL,
  "report_id" uuid NULL,
  "violation_id" uuid NULL,
  "position" integer NOT NULL,
  "admin_id" bigint NOT NULL,
  "accessed_at" timestamptz NOT NULL,
  PRIMARY KEY ("access_id"),
  CONSTRAINT "evidence_accesses_appeal_fkey" FOREIGN KEY ("violation_id", "position") REFERENCES "seller"."appeal_evidence" ("violation_id", "position") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "evidence_accesses_report_fkey" FOREIGN KEY ("report_id", "position") REFERENCES "seller"."report_evidence" ("report_id", "position") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "evidence_accesses_admin_check" CHECK (admin_id > 0),
  CONSTRAINT "evidence_accesses_owner_check" CHECK (num_nonnulls(report_id, violation_id) = 1)
);
-- Create index "evidence_accesses_report" to table: "evidence_accesses"
CREATE INDEX "evidence_accesses_report" ON "seller"."evidence_accesses" ("report_id", "accessed_at" DESC) WHERE (report_id IS NOT NULL);
-- Create index "evidence_accesses_violation" to table: "evidence_accesses"
CREATE INDEX "evidence_accesses_violation" ON "seller"."evidence_accesses" ("violation_id", "accessed_at" DESC) WHERE (violation_id IS NOT NULL);
-- Create "violation_changes" table
CREATE TABLE "seller"."violation_changes" (
  "change_id" uuid NOT NULL,
  "violation_id" uuid NOT NULL,
  "version" bigint NOT NULL,
  "admin_id" bigint NOT NULL,
  "changed_at" timestamptz NOT NULL,
  "description_before" text NULL,
  "statement_before" text NULL,
  "ends_at_before" timestamptz NULL,
  "ends_at_after" timestamptz NULL,
  PRIMARY KEY ("change_id"),
  CONSTRAINT "violation_changes_version_key" UNIQUE ("violation_id", "version"),
  CONSTRAINT "violation_changes_violation_id_fkey" FOREIGN KEY ("violation_id") REFERENCES "seller"."violations" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "violation_changes_admin_check" CHECK (admin_id > 0),
  CONSTRAINT "violation_changes_ends_at_check" CHECK ((ends_at_before IS NULL) = (ends_at_after IS NULL)),
  CONSTRAINT "violation_changes_nonempty_check" CHECK (num_nonnulls(description_before, statement_before, ends_at_after) > 0),
  CONSTRAINT "violation_changes_version_check" CHECK (version >= 2)
);
