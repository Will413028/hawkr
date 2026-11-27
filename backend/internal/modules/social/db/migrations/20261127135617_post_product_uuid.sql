-- Modify "posts" table
ALTER TABLE "social"."posts" DROP CONSTRAINT "posts_product_check", ALTER COLUMN "product_id" TYPE uuid USING NULL::uuid, ADD CONSTRAINT "posts_product_check" CHECK (product_id <> '00000000-0000-0000-0000-000000000000'::uuid);
