-- Modify "banners" table: add gradient card mode fields
ALTER TABLE "banners" ADD COLUMN "banner_mode" varchar(20) NOT NULL DEFAULT 'image';
ALTER TABLE "banners" ADD COLUMN "subtitle" varchar(255) NOT NULL DEFAULT '';
ALTER TABLE "banners" ADD COLUMN "button_text" varchar(100) NOT NULL DEFAULT '';
ALTER TABLE "banners" ADD COLUMN "gradient_start" varchar(10) NOT NULL DEFAULT '';
ALTER TABLE "banners" ADD COLUMN "gradient_end" varchar(10) NOT NULL DEFAULT '';
ALTER TABLE "banners" ADD COLUMN "action" jsonb NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE "banners" ADD COLUMN "popup" jsonb NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE "banners" ADD COLUMN "images" jsonb NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE "banners" ALTER COLUMN "image_url" SET DEFAULT '';
