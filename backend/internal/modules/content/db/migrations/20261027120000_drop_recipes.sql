-- 刪除食譜功能（ADR 2026-10-27-general-ecommerce-scope-cut）：三張食譜表，以及它們在 media 引用同步狀態裡的列（ref_type 7119）。
-- Drop "recipe_ingredients" table
DROP TABLE "content"."recipe_ingredients";
-- Drop "recipe_steps" table
DROP TABLE "content"."recipe_steps";
-- Drop "recipes" table
DROP TABLE "content"."recipes";
-- 食譜的 ref 不再有業務列；留著會讓保底掃描補排 job 後撞上「不認得的 media ref 類型」。
DELETE FROM "content"."media_ref_sync" WHERE "ref_type" = '7119';
DELETE FROM "content"."retired_assets" WHERE "ref_type" = '7119';
