-- Remove PRODUCT banners whose target is not a UUID
DELETE FROM "content"."banners" WHERE "action_type" = 'PRODUCT' AND "action_target" !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$';
-- Modify "banners" table
ALTER TABLE "content"."banners" DROP CONSTRAINT "banners_action_check", ADD CONSTRAINT "banners_action_check" CHECK (((action_type = 'POPUP'::text) AND (action_target = ''::text)) OR ((action_type = 'URL'::text) AND (action_target ~ '^https?://'::text)) OR ((action_type = 'PRODUCT'::text) AND (action_target ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'::text)) OR ((action_type = 'POST'::text) AND (action_target ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'::text)));
