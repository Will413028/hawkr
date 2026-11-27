-- Create "rating_recompute_queue" table
CREATE TABLE "review"."rating_recompute_queue" (
  "product_id" uuid NOT NULL,
  "seq" bigint NOT NULL DEFAULT 1,
  "due_at" timestamptz NOT NULL,
  "claimed_until" timestamptz NULL,
  "attempts" integer NOT NULL DEFAULT 0,
  "last_error" text NULL,
  "failed_at" timestamptz NULL,
  PRIMARY KEY ("product_id"),
  CONSTRAINT "rating_recompute_queue_attempts_check" CHECK (attempts >= 0),
  CONSTRAINT "rating_recompute_queue_seq_check" CHECK (seq >= 1)
);
-- Create index "rating_recompute_queue_due" to table: "rating_recompute_queue"
CREATE INDEX "rating_recompute_queue_due" ON "review"."rating_recompute_queue" ("due_at") WHERE (failed_at IS NULL);
