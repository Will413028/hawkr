-- Create "restock_purge_queue" table
CREATE TABLE "catalog"."restock_purge_queue" (
  "sku_id" uuid NOT NULL,
  "due_at" timestamptz NOT NULL,
  "requested_at" timestamptz NOT NULL,
  "claimed_until" timestamptz NULL,
  "attempts" integer NOT NULL DEFAULT 0,
  "last_error" text NULL,
  "failed_at" timestamptz NULL,
  PRIMARY KEY ("sku_id"),
  CONSTRAINT "restock_purge_queue_attempts_check" CHECK (attempts >= 0)
);
-- Create index "restock_purge_queue_due_idx" to table: "restock_purge_queue"
CREATE INDEX "restock_purge_queue_due_idx" ON "catalog"."restock_purge_queue" ("due_at") WHERE (failed_at IS NULL);
-- Create "restock_queue" table
CREATE TABLE "catalog"."restock_queue" (
  "sku_id" uuid NOT NULL,
  "due_at" timestamptz NOT NULL,
  "requested_at" timestamptz NOT NULL,
  "claimed_until" timestamptz NULL,
  "attempts" integer NOT NULL DEFAULT 0,
  "last_error" text NULL,
  "failed_at" timestamptz NULL,
  PRIMARY KEY ("sku_id"),
  CONSTRAINT "restock_queue_attempts_check" CHECK (attempts >= 0)
);
-- Create index "restock_queue_due_idx" to table: "restock_queue"
CREATE INDEX "restock_queue_due_idx" ON "catalog"."restock_queue" ("due_at") WHERE (failed_at IS NULL);
