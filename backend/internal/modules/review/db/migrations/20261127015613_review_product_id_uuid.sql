-- Delete reviews: numeric product ids have no mapping to catalog UUIDs and nothing is live.
DELETE FROM "review"."reviews";
-- Modify "reviews" table
ALTER TABLE "review"."reviews" DROP CONSTRAINT "reviews_product_check";
ALTER TABLE "review"."reviews" ALTER COLUMN "product_id" TYPE uuid USING NULL;
ALTER TABLE "review"."reviews" ADD CONSTRAINT "reviews_product_check" CHECK (product_id <> '00000000-0000-0000-0000-000000000000'::uuid);
