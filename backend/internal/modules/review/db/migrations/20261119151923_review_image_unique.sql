-- Modify "review_images" table
ALTER TABLE "review"."review_images" ADD CONSTRAINT "review_images_asset_key" UNIQUE ("review_id", "asset_id");
