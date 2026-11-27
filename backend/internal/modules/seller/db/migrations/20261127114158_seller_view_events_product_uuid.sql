-- Clear raw view events: their integer product IDs cannot be carried into the uuid column (nothing is live; the daily rollup is kept)
DELETE FROM "seller"."product_view_events";
-- Modify "product_view_events" table
ALTER TABLE "seller"."product_view_events" DROP COLUMN "product_id", ADD COLUMN "product_id" uuid NOT NULL, ADD CONSTRAINT "product_view_events_product_check" CHECK (product_id <> '00000000-0000-0000-0000-000000000000'::uuid);
