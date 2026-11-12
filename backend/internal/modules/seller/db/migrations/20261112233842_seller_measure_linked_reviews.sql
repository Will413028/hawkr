-- Create "measure_linked_reviews" table
CREATE TABLE "seller"."measure_linked_reviews" (
  "measure_id" uuid NOT NULL,
  "account_id" bigint NOT NULL,
  "reviewed_at" timestamptz NOT NULL,
  "reviewed_by_admin_id" bigint NOT NULL,
  PRIMARY KEY ("measure_id", "account_id"),
  CONSTRAINT "measure_linked_reviews_measure_id_fkey" FOREIGN KEY ("measure_id") REFERENCES "seller"."measures" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "measure_linked_reviews_admin_check" CHECK (reviewed_by_admin_id > 0)
);
