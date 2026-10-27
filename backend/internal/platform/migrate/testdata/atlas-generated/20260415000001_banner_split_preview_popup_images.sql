-- Banner 改為雙圖設計：小圖（首頁）+ 大圖（點擊彈窗）
ALTER TABLE "banners" DROP COLUMN "image_url";
ALTER TABLE "banners" DROP COLUMN "images";
ALTER TABLE "banners" ADD COLUMN "preview_image_url" TEXT NOT NULL DEFAULT '';
ALTER TABLE "banners" ADD COLUMN "popup_image_url" TEXT NOT NULL DEFAULT '';
