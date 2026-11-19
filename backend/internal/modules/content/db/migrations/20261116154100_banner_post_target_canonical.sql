-- 既有 POST 目標：去掉空白並轉小寫成標準形式；不是 UUID 的（舊的 TSID 格式）指不到任何貼文，直接刪除；再加上新的約束。
DELETE FROM "content"."banners" WHERE "action_type" = 'POST' AND lower(btrim("action_target")) !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$';
UPDATE "content"."banners" SET "action_target" = lower(btrim("action_target")) WHERE "action_type" = 'POST';
-- Modify "banners" table
ALTER TABLE "content"."banners" DROP CONSTRAINT "banners_action_check", ADD CONSTRAINT "banners_action_check" CHECK (((action_type = 'POPUP'::text) AND (action_target = ''::text)) OR ((action_type = 'URL'::text) AND (action_target ~ '^https?://'::text)) OR ((action_type = 'PRODUCT'::text) AND (action_target <> ''::text)) OR ((action_type = 'POST'::text) AND (action_target ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'::text)));
