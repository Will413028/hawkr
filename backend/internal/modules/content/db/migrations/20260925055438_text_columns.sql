-- ALTER TYPE 不會改寫預設值的型別（仍是 ''::character varying），手動加上 SET DEFAULT，與 schema.sql 一致。
-- Modify "app_version_gates" table
ALTER TABLE "content"."app_version_gates" ALTER COLUMN "app_type" TYPE text, ALTER COLUMN "platform" TYPE text, ALTER COLUMN "minimum_version" TYPE text, ALTER COLUMN "notice_title" TYPE text, ALTER COLUMN "notice_title" SET DEFAULT '';
-- Modify "banners" table
ALTER TABLE "content"."banners" ALTER COLUMN "title" TYPE text, ALTER COLUMN "button_text" TYPE text, ALTER COLUMN "button_text" SET DEFAULT '';
-- Modify "recipe_ingredients" table
ALTER TABLE "content"."recipe_ingredients" ALTER COLUMN "name" TYPE text, ALTER COLUMN "amount" TYPE text, ALTER COLUMN "amount" SET DEFAULT '';
-- Modify "recipes" table
ALTER TABLE "content"."recipes" ALTER COLUMN "title" TYPE text, ALTER COLUMN "category" TYPE text, ALTER COLUMN "category" SET DEFAULT '';
-- Modify "site_contents" table
ALTER TABLE "content"."site_contents" ALTER COLUMN "content_key" TYPE text, ALTER COLUMN "label" TYPE text, ALTER COLUMN "label" SET DEFAULT '';
