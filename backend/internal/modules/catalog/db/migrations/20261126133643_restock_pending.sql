-- Create "restock_pending" table
CREATE TABLE "catalog"."restock_pending" (
  "job_key" text NOT NULL,
  "job_id" bigint NOT NULL,
  PRIMARY KEY ("job_key"),
  CONSTRAINT "restock_pending_key_check" CHECK ((char_length(job_key) >= 1) AND (char_length(job_key) <= 100))
);
