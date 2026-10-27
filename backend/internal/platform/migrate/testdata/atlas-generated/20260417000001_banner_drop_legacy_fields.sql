-- Drop legacy banner columns after migration to preview/popup image URLs.
-- Retained: button_text (CTA label), canvas_json (admin Canva editor), action (CTA routing).

ALTER TABLE "banners" DROP COLUMN IF EXISTS "link_url";
ALTER TABLE "banners" DROP COLUMN IF EXISTS "banner_mode";
ALTER TABLE "banners" DROP COLUMN IF EXISTS "subtitle";
ALTER TABLE "banners" DROP COLUMN IF EXISTS "gradient_start";
ALTER TABLE "banners" DROP COLUMN IF EXISTS "gradient_end";
ALTER TABLE "banners" DROP COLUMN IF EXISTS "popup";
