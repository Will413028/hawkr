-- Rename image_url columns -> *_key（保留既有值；顯示 URL 改由 Gateway 組，DB 只存 objectKey）
-- banners: preview_image_url -> preview_image_key, popup_image_url -> popup_image_key
ALTER TABLE "banners" RENAME COLUMN "preview_image_url" TO "preview_image_key";
ALTER TABLE "banners" RENAME COLUMN "popup_image_url" TO "popup_image_key";
-- recipes: image_url -> image_key
ALTER TABLE "recipes" RENAME COLUMN "image_url" TO "image_key";
