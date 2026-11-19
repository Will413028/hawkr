-- Drop index "content_takedowns_comment" from table: "content_takedowns"
DROP INDEX "social"."content_takedowns_comment";
-- Drop index "content_takedowns_forum_entry" from table: "content_takedowns"
DROP INDEX "social"."content_takedowns_forum_entry";
-- Drop index "content_takedowns_post" from table: "content_takedowns"
DROP INDEX "social"."content_takedowns_post";
-- Create index "content_takedowns_comment" to table: "content_takedowns"
CREATE UNIQUE INDEX "content_takedowns_comment" ON "social"."content_takedowns" ("comment_id") WHERE (comment_id IS NOT NULL);
-- Create index "content_takedowns_forum_entry" to table: "content_takedowns"
CREATE UNIQUE INDEX "content_takedowns_forum_entry" ON "social"."content_takedowns" ("forum_entry_id") WHERE (forum_entry_id IS NOT NULL);
-- Create index "content_takedowns_post" to table: "content_takedowns"
CREATE UNIQUE INDEX "content_takedowns_post" ON "social"."content_takedowns" ("post_id") WHERE (post_id IS NOT NULL);
