-- Drop index "refunds_reversing" from table: "refunds"
DROP INDEX "payment"."refunds_reversing";
-- Modify "refunds" table
ALTER TABLE "payment"."refunds" DROP CONSTRAINT "refunds_lookup_failures_check", DROP CONSTRAINT "refunds_reversal_called_check", DROP CONSTRAINT "refunds_reversal_external_id_check", DROP CONSTRAINT "refunds_reversed_check", DROP CONSTRAINT "refunds_reversing_check", DROP CONSTRAINT "refunds_external_state_check", ADD CONSTRAINT "refunds_external_state_check" CHECK ((external_refund_id IS NULL) OR (status = ANY (ARRAY['REFUNDING'::text, 'SUCCEEDED'::text, 'FAILED'::text, 'ANOMALY'::text]))), DROP CONSTRAINT "refunds_status_check", ADD CONSTRAINT "refunds_status_check" CHECK (status = ANY (ARRAY['PENDING'::text, 'REFUNDING'::text, 'SUCCEEDED'::text, 'FAILED'::text, 'ANOMALY'::text])), ADD CONSTRAINT "refunds_failed_releases_waiting_check" CHECK ((status <> 'FAILED'::text) OR (reversal_status <> 'WAITING'::text)), ADD CONSTRAINT "refunds_refund_lookup_failures_check" CHECK (refund_lookup_failures >= 0), ADD CONSTRAINT "refunds_reversal_after_acceptance_check" CHECK ((reversal_status <> ALL (ARRAY['REVERSING'::text, 'REVERSED'::text])) OR (external_refund_id IS NOT NULL)), ADD CONSTRAINT "refunds_reversal_none_check" CHECK ((reversal_status = 'NONE'::text) = (reversal_amount = 0)), ADD CONSTRAINT "refunds_reversal_not_started_check" CHECK ((reversal_status <> ALL (ARRAY['NONE'::text, 'WAITING'::text])) OR (reversal_recovered = 0)), ADD CONSTRAINT "refunds_reversal_reason_check" CHECK (((reversal_status = ANY (ARRAY['RELEASED'::text, 'ANOMALY'::text])) = (reversal_reason IS NOT NULL)) AND ((reversal_reason IS NULL) OR ((char_length(reversal_reason) >= 1) AND (char_length(reversal_reason) <= 200)))), ADD CONSTRAINT "refunds_reversal_recovered_check" CHECK ((reversal_recovered >= 0) AND (reversal_recovered <= reversal_amount)), ADD CONSTRAINT "refunds_reversal_reversed_check" CHECK ((reversal_status <> 'REVERSED'::text) OR (reversal_recovered = reversal_amount)), ADD CONSTRAINT "refunds_reversal_status_check" CHECK (reversal_status = ANY (ARRAY['NONE'::text, 'WAITING'::text, 'REVERSING'::text, 'REVERSED'::text, 'RELEASED'::text, 'ANOMALY'::text])), ADD CONSTRAINT "refunds_topup_called_check" CHECK ((topup_external_id IS NULL) OR (topup_called_at IS NOT NULL)), ADD CONSTRAINT "refunds_topup_external_check" CHECK (((topup_status = 'DONE'::text) = (topup_external_id IS NOT NULL)) AND ((topup_external_id IS NULL) OR ((char_length(topup_external_id) >= 1) AND (char_length(topup_external_id) <= 255)))), ADD CONSTRAINT "refunds_topup_lookup_failures_check" CHECK (topup_lookup_failures >= 0), ADD CONSTRAINT "refunds_topup_only_failed_check" CHECK ((topup_status = 'NONE'::text) OR (status = 'FAILED'::text)), ADD CONSTRAINT "refunds_topup_reason_check" CHECK (((topup_status = ANY (ARRAY['RELEASED'::text, 'ANOMALY'::text])) = (topup_reason IS NOT NULL)) AND ((topup_reason IS NULL) OR ((char_length(topup_reason) >= 1) AND (char_length(topup_reason) <= 200)))), ADD CONSTRAINT "refunds_topup_reversed_check" CHECK ((topup_reversed_amount >= 0) AND (topup_reversed_amount <= amount)), ADD CONSTRAINT "refunds_topup_reversed_only_done_check" CHECK ((topup_status = 'DONE'::text) OR (topup_reversed_amount = 0)), ADD CONSTRAINT "refunds_topup_status_check" CHECK (topup_status = ANY (ARRAY['NONE'::text, 'WAITING'::text, 'SENDING'::text, 'DONE'::text, 'RELEASED'::text, 'ANOMALY'::text])), DROP COLUMN "reversal_idempotency_key", DROP COLUMN "reversal_called_at", DROP COLUMN "reversal_external_id", DROP COLUMN "reversal_lookup_failures", ADD COLUMN "reversal_status" text NOT NULL DEFAULT 'NONE', ADD COLUMN "reversal_recovered" bigint NOT NULL DEFAULT 0, ADD COLUMN "reversal_reason" text NULL, ADD COLUMN "transfer_deducted" boolean NOT NULL DEFAULT false, ADD COLUMN "topup_status" text NOT NULL DEFAULT 'NONE', ADD COLUMN "topup_reason" text NULL, ADD COLUMN "topup_idempotency_key" uuid NOT NULL DEFAULT gen_random_uuid(), ADD COLUMN "topup_called_at" timestamptz NULL, ADD COLUMN "topup_external_id" text NULL, ADD COLUMN "topup_lookup_failures" integer NOT NULL DEFAULT 0, ADD COLUMN "topup_reversed_amount" bigint NOT NULL DEFAULT 0, ADD COLUMN "refund_lookup_failures" integer NOT NULL DEFAULT 0;
-- Create index "refunds_open" to table: "refunds"
CREATE INDEX "refunds_open" ON "payment"."refunds" ("updated_at", "id") WHERE (status = ANY (ARRAY['PENDING'::text, 'REFUNDING'::text]));
-- Create index "refunds_topup_external" to table: "refunds"
CREATE UNIQUE INDEX "refunds_topup_external" ON "payment"."refunds" ("topup_external_id") WHERE (topup_external_id IS NOT NULL);
-- Create index "refunds_topup_sending" to table: "refunds"
CREATE INDEX "refunds_topup_sending" ON "payment"."refunds" ("updated_at", "id") WHERE (topup_status = 'SENDING'::text);
-- Create "refund_reversals" table
CREATE TABLE "payment"."refund_reversals" (
  "id" uuid NOT NULL,
  "refund_id" uuid NOT NULL,
  "payment_id" uuid NOT NULL,
  "seq" integer NOT NULL,
  "target_refund_id" uuid NULL,
  "target_external_id" text NOT NULL,
  "amount" bigint NOT NULL,
  "status" text NOT NULL DEFAULT 'REVERSING',
  "reason" text NULL,
  "idempotency_key" uuid NOT NULL DEFAULT gen_random_uuid(),
  "called_at" timestamptz NULL,
  "external_id" text NULL,
  "lookup_failures" integer NOT NULL DEFAULT 0,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "refund_reversals_payment_id_fkey" FOREIGN KEY ("payment_id") REFERENCES "payment"."payments" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "refund_reversals_refund_id_fkey" FOREIGN KEY ("refund_id") REFERENCES "payment"."refunds" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "refund_reversals_target_refund_id_fkey" FOREIGN KEY ("target_refund_id") REFERENCES "payment"."refunds" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "refund_reversals_amount_check" CHECK (amount > 0),
  CONSTRAINT "refund_reversals_called_check" CHECK ((external_id IS NULL) OR (called_at IS NOT NULL)),
  CONSTRAINT "refund_reversals_external_check" CHECK (((status = 'REVERSED'::text) = (external_id IS NOT NULL)) AND ((external_id IS NULL) OR ((char_length(external_id) >= 1) AND (char_length(external_id) <= 255)))),
  CONSTRAINT "refund_reversals_lookup_failures_check" CHECK (lookup_failures >= 0),
  CONSTRAINT "refund_reversals_reason_check" CHECK (((status = ANY (ARRAY['RELEASED'::text, 'ANOMALY'::text])) = (reason IS NOT NULL)) AND ((reason IS NULL) OR ((char_length(reason) >= 1) AND (char_length(reason) <= 200)))),
  CONSTRAINT "refund_reversals_seq_check" CHECK (seq >= 0),
  CONSTRAINT "refund_reversals_status_check" CHECK (status = ANY (ARRAY['REVERSING'::text, 'REVERSED'::text, 'RELEASED'::text, 'ANOMALY'::text])),
  CONSTRAINT "refund_reversals_target_check" CHECK ((seq = 0) = (target_refund_id IS NULL)),
  CONSTRAINT "refund_reversals_target_external_check" CHECK ((char_length(target_external_id) >= 1) AND (char_length(target_external_id) <= 255))
);
-- Create index "refund_reversals_external" to table: "refund_reversals"
CREATE UNIQUE INDEX "refund_reversals_external" ON "payment"."refund_reversals" ("external_id") WHERE (external_id IS NOT NULL);
-- Create index "refund_reversals_open" to table: "refund_reversals"
CREATE INDEX "refund_reversals_open" ON "payment"."refund_reversals" ("payment_id", "target_refund_id") WHERE (status = ANY (ARRAY['REVERSING'::text, 'ANOMALY'::text]));
-- Create index "refund_reversals_reversing" to table: "refund_reversals"
CREATE INDEX "refund_reversals_reversing" ON "payment"."refund_reversals" ("updated_at", "id") WHERE (status = 'REVERSING'::text);
-- Create index "refund_reversals_seq" to table: "refund_reversals"
CREATE UNIQUE INDEX "refund_reversals_seq" ON "payment"."refund_reversals" ("refund_id", "seq");
