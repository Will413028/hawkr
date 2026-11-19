-- Create index "posts_admin_list" to table: "posts"
CREATE INDEX "posts_admin_list" ON "social"."posts" ("created_at" DESC, "id" DESC) WHERE ((deleted_at IS NULL) AND (status <> 'DRAFT'::text));
