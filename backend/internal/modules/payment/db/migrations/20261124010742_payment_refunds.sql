-- Modify "refunds" table
ALTER TABLE "payment"."refunds" DROP CONSTRAINT "refunds_external_state_check", ADD CONSTRAINT "refunds_external_state_check" CHECK ((external_refund_id IS NULL) OR (status = ANY (ARRAY['REFUNDING'::text, 'SUCCEEDED'::text, 'FAILED'::text, 'ANOMALY'::text]))), ADD CONSTRAINT "refunds_refund_lookup_failures_check" CHECK (refund_lookup_failures >= 0), ADD COLUMN "refund_lookup_failures" integer NOT NULL DEFAULT 0;
-- Create index "refunds_open" to table: "refunds"
CREATE INDEX "refunds_open" ON "payment"."refunds" ("updated_at", "id") WHERE (status = ANY (ARRAY['PENDING'::text, 'REVERSED'::text, 'REFUNDING'::text]));
