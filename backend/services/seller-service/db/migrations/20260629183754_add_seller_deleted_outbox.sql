-- Create "seller_deleted_outbox" table
CREATE TABLE "seller_deleted_outbox" (
  "id" bigint NOT NULL,
  "account_id" bigint NOT NULL,
  "status" character varying(20) NOT NULL DEFAULT 'pending',
  "attempt_count" integer NOT NULL DEFAULT 0,
  "next_attempt_at" timestamptz NOT NULL DEFAULT now(),
  "last_error" text NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "completed_at" timestamptz NULL,
  PRIMARY KEY ("id")
);
-- Create index "idx_seller_deleted_outbox_pending" to table: "seller_deleted_outbox"
CREATE INDEX "idx_seller_deleted_outbox_pending" ON "seller_deleted_outbox" ("status", "next_attempt_at") WHERE ((status)::text = 'pending'::text);
