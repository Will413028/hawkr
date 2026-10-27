-- Rename review image columns url -> key（保留既有值；顯示 URL 改由 Gateway 組，DB 只存 objectKey，49 §5.3/§6.2）
ALTER TABLE "order_item_reviews" RENAME COLUMN "image_urls" TO "image_keys";
ALTER TABLE "order_item_reviews" RENAME COLUMN "item_image_url" TO "item_image_key";
