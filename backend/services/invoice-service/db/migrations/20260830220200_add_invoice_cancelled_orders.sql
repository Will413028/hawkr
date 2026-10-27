-- Create "invoice_cancelled_orders" table
CREATE TABLE "invoice_cancelled_orders" (
  "order_no" character varying(64) NOT NULL,
  "buyer_id" bigint NOT NULL,
  "cancel_reason" character varying(32) NULL,
  "occurred_at" timestamptz NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("order_no"),
  CONSTRAINT "chk_invcanord_buyer" CHECK (buyer_id > 0)
);
-- Create index "idx_invcanord_order_buyer" to table: "invoice_cancelled_orders"
CREATE INDEX "idx_invcanord_order_buyer" ON "invoice_cancelled_orders" ("order_no", "buyer_id");
