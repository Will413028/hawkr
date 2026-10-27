-- Create "seller_deletion_audit" table
CREATE TABLE "seller_deletion_audit" (
  "id" bigint NOT NULL,
  "account_id" bigint NOT NULL,
  "operator_id" bigint NOT NULL,
  "active_order_count" integer NOT NULL,
  "reason" character varying(255) NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id")
);
-- Create index "idx_sda_account" to table: "seller_deletion_audit"
CREATE INDEX "idx_sda_account" ON "seller_deletion_audit" ("account_id", "created_at" DESC);
