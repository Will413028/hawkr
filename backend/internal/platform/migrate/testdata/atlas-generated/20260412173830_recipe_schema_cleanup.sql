-- Modify "recipe_ingredients" table
ALTER TABLE "recipe_ingredients" ADD COLUMN "sort_order" integer NOT NULL DEFAULT 0;
-- Modify "recipes" table
ALTER TABLE "recipes" DROP COLUMN "source_url", ADD COLUMN "category" character varying(50) NOT NULL DEFAULT '', ADD COLUMN "cooking_time_minutes" integer NULL, ADD COLUMN "servings" integer NULL, ADD COLUMN "updated_at" timestamptz NOT NULL DEFAULT now();
-- Drop "ingredient_product_mappings" table
DROP TABLE "ingredient_product_mappings";
