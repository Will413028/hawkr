-- Modify "app_version_gates" table
ALTER TABLE "content"."app_version_gates" ALTER COLUMN "store_url" SET NOT NULL, ALTER COLUMN "store_url" SET DEFAULT '';
-- Modify "banners" table
ALTER TABLE "content"."banners" ADD CONSTRAINT "banners_action_check" CHECK (((action_type = 'popup'::text) AND (action_target = ''::text)) OR ((action_type = 'url'::text) AND (action_target ~ '^https?://'::text)) OR ((action_type = ANY (ARRAY['product'::text, 'post'::text])) AND (action_target <> ''::text))), DROP COLUMN "action", ADD COLUMN "action_type" text NOT NULL, ADD COLUMN "action_target" text NOT NULL DEFAULT '';
-- Set comment to column: "action_type" on table: "banners"
COMMENT ON COLUMN "content"."banners"."action_type" IS '點擊行為（設計書 25）：popup、url、product、post；沒有預設值';
-- Set comment to column: "action_target" on table: "banners"
COMMENT ON COLUMN "content"."banners"."action_target" IS '點擊行為的值：url 的網址、product 與 post 的 ID；popup 是空字串';
