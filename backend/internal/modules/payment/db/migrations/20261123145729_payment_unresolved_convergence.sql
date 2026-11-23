-- Modify "payments" table
ALTER TABLE "payment"."payments" DROP CONSTRAINT "payments_failure_check", ADD CONSTRAINT "payments_failure_check" CHECK ((status = ANY (ARRAY['CREATE_FAILED'::text, 'ANOMALY'::text])) = (failure_reason IS NOT NULL)), DROP CONSTRAINT "payments_status_check", ADD CONSTRAINT "payments_status_check" CHECK (status = ANY (ARRAY['CREATING'::text, 'AWAITING_PAYMENT'::text, 'PROCESSING'::text, 'SUCCEEDED'::text, 'CANCELED'::text, 'CREATE_FAILED'::text, 'ANOMALY'::text])), ADD CONSTRAINT "payments_lookup_failures_check" CHECK (create_lookup_failures >= 0), ADD COLUMN "create_lookup_failures" integer NOT NULL DEFAULT 0;
-- Modify "refunds" table
ALTER TABLE "payment"."refunds" DROP CONSTRAINT "refunds_failure_check", ADD CONSTRAINT "refunds_failure_check" CHECK ((status = ANY (ARRAY['FAILED'::text, 'ANOMALY'::text])) = (failure_reason IS NOT NULL)), DROP CONSTRAINT "refunds_status_check", ADD CONSTRAINT "refunds_status_check" CHECK (status = ANY (ARRAY['PENDING'::text, 'REVERSING'::text, 'REVERSED'::text, 'REFUNDING'::text, 'SUCCEEDED'::text, 'FAILED'::text, 'ANOMALY'::text])), ADD CONSTRAINT "refunds_lookup_failures_check" CHECK (reversal_lookup_failures >= 0), ADD COLUMN "reversal_lookup_failures" integer NOT NULL DEFAULT 0;
-- Create index "refunds_reversing" to table: "refunds"
CREATE INDEX "refunds_reversing" ON "payment"."refunds" ("updated_at", "id") WHERE (status = 'REVERSING'::text);
-- Drop index "transfers_active_payment" from table: "transfers"
DROP INDEX "payment"."transfers_active_payment";
-- Modify "transfers" table
ALTER TABLE "payment"."transfers" DROP CONSTRAINT "transfers_failure_check", ADD CONSTRAINT "transfers_failure_check" CHECK ((status = ANY (ARRAY['FAILED'::text, 'ANOMALY'::text])) = (failure_reason IS NOT NULL)), DROP CONSTRAINT "transfers_status_check", ADD CONSTRAINT "transfers_status_check" CHECK (status = ANY (ARRAY['PENDING'::text, 'SUCCEEDED'::text, 'FAILED'::text, 'ANOMALY'::text])), ADD CONSTRAINT "transfers_lookup_failures_check" CHECK (lookup_failures >= 0), ADD COLUMN "lookup_failures" integer NOT NULL DEFAULT 0;
-- Create index "transfers_active_payment" to table: "transfers"
CREATE UNIQUE INDEX "transfers_active_payment" ON "payment"."transfers" ("payment_id") WHERE (status = ANY (ARRAY['PENDING'::text, 'SUCCEEDED'::text, 'ANOMALY'::text]));
-- Create index "transfers_pending" to table: "transfers"
CREATE INDEX "transfers_pending" ON "payment"."transfers" ("updated_at", "id") WHERE (status = 'PENDING'::text);
