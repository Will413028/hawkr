-- Create "product_view_events" table
CREATE TABLE "seller"."product_view_events" (
  "event_id" uuid NOT NULL,
  "product_id" bigint NOT NULL,
  "occurred_at" timestamptz NOT NULL,
  "store_id" bigint NULL,
  "processed_at" timestamptz NULL,
  "outcome" text NULL,
  PRIMARY KEY ("event_id"),
  CONSTRAINT "product_view_events_outcome_check" CHECK (outcome = ANY (ARRAY['COUNTED'::text, 'UNRESOLVED'::text])),
  CONSTRAINT "product_view_events_processed_check" CHECK (((processed_at IS NULL) AND (outcome IS NULL) AND (store_id IS NULL)) OR ((processed_at IS NOT NULL) AND (outcome = 'COUNTED'::text) AND (store_id IS NOT NULL) AND (store_id > 0)) OR ((processed_at IS NOT NULL) AND (outcome = 'UNRESOLVED'::text) AND (store_id IS NULL))),
  CONSTRAINT "product_view_events_product_check" CHECK (product_id > 0)
);
-- Create index "product_view_events_pending" to table: "product_view_events"
CREATE INDEX "product_view_events_pending" ON "seller"."product_view_events" ("occurred_at", "event_id") WHERE (processed_at IS NULL);
-- Create index "product_view_events_processed" to table: "product_view_events"
CREATE INDEX "product_view_events_processed" ON "seller"."product_view_events" ("occurred_at") WHERE (processed_at IS NOT NULL);
