-- Modify "styles" table
ALTER TABLE "imageai"."styles" ADD CONSTRAINT "styles_default_enabled_check" CHECK ((NOT is_default) OR enabled);
-- Modify "usage" table
ALTER TABLE "imageai"."usage" DROP CONSTRAINT "usage_pkey", ADD PRIMARY KEY ("store_id");
