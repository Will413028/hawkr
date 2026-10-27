-- Modify "recipes" table
ALTER TABLE "recipes" ADD COLUMN "view_count" bigint NOT NULL DEFAULT 0;
-- Create index "idx_recipes_category" to table: "recipes"
CREATE INDEX "idx_recipes_category" ON "recipes" ("category");
-- Create index "idx_recipes_created_at" to table: "recipes"
CREATE INDEX "idx_recipes_created_at" ON "recipes" ("created_at" DESC);
-- Create index "idx_recipes_view_count" to table: "recipes"
CREATE INDEX "idx_recipes_view_count" ON "recipes" ("view_count" DESC);
