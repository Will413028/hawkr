-- Create "crm_customer_ledger" table
CREATE TABLE "crm_customer_ledger" (
  "event_id" character varying(64) NOT NULL,
  "store_id" bigint NOT NULL,
  "customer_account_id" bigint NOT NULL,
  "event_type" character varying(20) NOT NULL,
  "order_no" character varying(64) NOT NULL,
  "amount_delta" bigint NOT NULL,
  "count_delta" integer NOT NULL,
  "occurred_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("event_id")
);
-- Create index "idx_crm_customer_ledger_customer" to table: "crm_customer_ledger"
CREATE INDEX "idx_crm_customer_ledger_customer" ON "crm_customer_ledger" ("store_id", "customer_account_id", "occurred_at");
-- Drop "crm_processed_orders" table
DROP TABLE "crm_processed_orders";
