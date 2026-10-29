-- Modify "authority_orders" table
ALTER TABLE "seller"."authority_orders" ADD CONSTRAINT "authority_orders_revoke_reason_text_check" CHECK (revoke_reason !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text);
-- Modify "cases" table
ALTER TABLE "seller"."cases" ADD CONSTRAINT "cases_decision_note_text_check" CHECK (decision_note !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text);
-- Modify "measure_requests" table
ALTER TABLE "seller"."measure_requests" DROP CONSTRAINT "measure_requests_reason_check", ADD CONSTRAINT "measure_requests_reason_check" CHECK (((char_length(reason) >= 1) AND (char_length(reason) <= 2000)) AND (reason !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text)), ADD CONSTRAINT "measure_requests_decision_reason_text_check" CHECK (decision_reason !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text);
-- Modify "measure_revisions" table
ALTER TABLE "seller"."measure_revisions" DROP CONSTRAINT "measure_revisions_statement_check", ADD CONSTRAINT "measure_revisions_statement_check" CHECK (((char_length(statement_to_seller) >= 1) AND (char_length(statement_to_seller) <= 2000)) AND (statement_to_seller !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text));
-- Create "finding_moderation_reports" table
CREATE TABLE "seller"."finding_moderation_reports" (
  "violation_id" uuid NOT NULL,
  "report_id" uuid NOT NULL,
  PRIMARY KEY ("violation_id", "report_id"),
  CONSTRAINT "finding_moderation_reports_violation_id_fkey" FOREIGN KEY ("violation_id") REFERENCES "seller"."violations" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "finding_moderation_reports_report" to table: "finding_moderation_reports"
CREATE INDEX "finding_moderation_reports_report" ON "seller"."finding_moderation_reports" ("report_id");
-- Modify "measures" table
ALTER TABLE "seller"."measures" DROP CONSTRAINT "measures_kind_check", ADD CONSTRAINT "measures_kind_check" CHECK (kind = ANY (ARRAY['WARNING'::text, 'SUSPENSION'::text, 'TERMINATION'::text, 'CONTENT_TAKEDOWN'::text])), DROP CONSTRAINT "measures_statement_check", ADD CONSTRAINT "measures_statement_check" CHECK (((char_length(statement_to_seller) >= 1) AND (char_length(statement_to_seller) <= 2000)) AND (statement_to_seller !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text)), ADD CONSTRAINT "measures_takedown_check" CHECK ((kind <> 'CONTENT_TAKEDOWN'::text) OR ((basis = 'VIOLATION'::text) AND (starts_at = created_at) AND (ends_at IS NULL)));
-- Create index "measures_active_takedown" to table: "measures"
CREATE INDEX "measures_active_takedown" ON "seller"."measures" ("id") WHERE ((status = 'ENFORCED'::text) AND (kind = 'CONTENT_TAKEDOWN'::text));
-- Create "measure_contents" table
CREATE TABLE "seller"."measure_contents" (
  "measure_id" uuid NOT NULL,
  "content_kind" text NOT NULL,
  "content_id" uuid NOT NULL,
  PRIMARY KEY ("measure_id", "content_kind", "content_id"),
  CONSTRAINT "measure_contents_measure_id_fkey" FOREIGN KEY ("measure_id") REFERENCES "seller"."measures" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "measure_contents_content_kind_check" CHECK (content_kind = ANY (ARRAY['POST'::text, 'COMMENT'::text, 'FORUM_ENTRY'::text]))
);
