-- Drop constraint: "banners_action_check" from table: "banners"
ALTER TABLE "content"."banners" DROP CONSTRAINT "banners_action_check";
UPDATE "content"."banners" SET "action_type" = upper("action_type");
-- Create "banners_action_check" constraint
ALTER TABLE "content"."banners" ADD CONSTRAINT "banners_action_check" CHECK (((action_type = 'POPUP'::text) AND (action_target = ''::text)) OR ((action_type = 'URL'::text) AND (action_target ~ '^https?://'::text)) OR ((action_type = ANY (ARRAY['PRODUCT'::text, 'POST'::text])) AND (action_target <> ''::text)));
-- Set comment to column: "action_type" on table: "banners"
COMMENT ON COLUMN "content"."banners"."action_type" IS '點擊行為（設計書 25）：POPUP、URL、PRODUCT、POST；沒有預設值';
-- Set comment to column: "action_target" on table: "banners"
COMMENT ON COLUMN "content"."banners"."action_target" IS '點擊行為的值：URL 的網址、PRODUCT 與 POST 的 ID；POPUP 是空字串';
