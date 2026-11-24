-- Create "payment_orders" table
CREATE TABLE "payment"."payment_orders" (
  "id" uuid NOT NULL,
  "payment_id" uuid NOT NULL,
  "order_no" text NOT NULL,
  "store_id" bigint NOT NULL,
  "amount" bigint NOT NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "payment_orders_payment_id_fkey" FOREIGN KEY ("payment_id") REFERENCES "payment"."payments" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "payment_orders_amount_check" CHECK (amount > 0),
  CONSTRAINT "payment_orders_order_no_check" CHECK ((char_length(order_no) >= 1) AND (char_length(order_no) <= 64)),
  CONSTRAINT "payment_orders_store_id_check" CHECK (store_id > 0)
);
-- Create index "payment_orders_order" to table: "payment_orders"
CREATE UNIQUE INDEX "payment_orders_order" ON "payment"."payment_orders" ("order_no");
-- Create index "payment_orders_payment" to table: "payment_orders"
CREATE INDEX "payment_orders_payment" ON "payment"."payment_orders" ("payment_id");
-- Backfill one order receivable per existing payment (a payment used to belong to exactly one order)
INSERT INTO "payment"."payment_orders" ("id", "payment_id", "order_no", "store_id", "amount", "created_at")
SELECT gen_random_uuid(), "id", "order_no", "store_id", "amount", "created_at" FROM "payment"."payments";
-- Modify "payments" table
ALTER TABLE "payment"."payments" DROP CONSTRAINT "payments_order_no_check", DROP CONSTRAINT "payments_store_id_check", DROP COLUMN "order_no", DROP COLUMN "store_id";
-- Drop index "refund_reversals_open" from table: "refund_reversals"
DROP INDEX "payment"."refund_reversals_open";
-- Modify "refund_reversals" table
ALTER TABLE "payment"."refund_reversals" ADD COLUMN "payment_order_id" uuid;
UPDATE "payment"."refund_reversals" AS v SET "payment_order_id" = o."id" FROM "payment"."payment_orders" AS o WHERE o."payment_id" = v."payment_id";
ALTER TABLE "payment"."refund_reversals" DROP COLUMN "payment_id", ALTER COLUMN "payment_order_id" SET NOT NULL, ADD CONSTRAINT "refund_reversals_payment_order_id_fkey" FOREIGN KEY ("payment_order_id") REFERENCES "payment"."payment_orders" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Create index "refund_reversals_open" to table: "refund_reversals"
CREATE INDEX "refund_reversals_open" ON "payment"."refund_reversals" ("payment_order_id", "target_refund_id") WHERE (status = ANY (ARRAY['REVERSING'::text, 'ANOMALY'::text]));
-- Drop index "refunds_request" from table: "refunds"
DROP INDEX "payment"."refunds_request";
-- Modify "refunds" table
ALTER TABLE "payment"."refunds" ADD COLUMN "payment_order_id" uuid;
UPDATE "payment"."refunds" AS r SET "payment_order_id" = o."id" FROM "payment"."payment_orders" AS o WHERE o."payment_id" = r."payment_id";
ALTER TABLE "payment"."refunds" DROP COLUMN "payment_id", ALTER COLUMN "payment_order_id" SET NOT NULL, ADD CONSTRAINT "refunds_payment_order_id_fkey" FOREIGN KEY ("payment_order_id") REFERENCES "payment"."payment_orders" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Create index "refunds_request" to table: "refunds"
CREATE UNIQUE INDEX "refunds_request" ON "payment"."refunds" ("payment_order_id", "request_key");
-- Create index "refunds_order" to table: "refunds"
CREATE INDEX "refunds_order" ON "payment"."refunds" ("payment_order_id");
-- Modify "transfers" table
ALTER TABLE "payment"."transfers" ADD COLUMN "payment_order_id" uuid;
UPDATE "payment"."transfers" AS t SET "payment_order_id" = o."id" FROM "payment"."payment_orders" AS o WHERE o."payment_id" = t."payment_id";
ALTER TABLE "payment"."transfers" DROP COLUMN "payment_id", ALTER COLUMN "payment_order_id" SET NOT NULL, ADD CONSTRAINT "transfers_payment_order_id_fkey" FOREIGN KEY ("payment_order_id") REFERENCES "payment"."payment_orders" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Create index "transfers_active_order" to table: "transfers"
CREATE UNIQUE INDEX "transfers_active_order" ON "payment"."transfers" ("payment_order_id") WHERE (status = ANY (ARRAY['PENDING'::text, 'SUCCEEDED'::text, 'ANOMALY'::text]));
