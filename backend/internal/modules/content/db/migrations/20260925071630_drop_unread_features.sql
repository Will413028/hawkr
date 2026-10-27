-- Modify "recipes" table
ALTER TABLE "content"."recipes" DROP COLUMN "view_count";
-- Drop "site_contents" table
DROP TABLE "content"."site_contents";
