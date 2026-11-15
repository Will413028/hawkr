-- Modify "comments" table
ALTER TABLE "social"."comments" ADD CONSTRAINT "comments_version_check" CHECK (version >= 1), ADD COLUMN "version" bigint NOT NULL DEFAULT 1;
-- Set comment to column: "version" on table: "comments"
COMMENT ON COLUMN "social"."comments"."version" IS '樂觀鎖的版本，對外是 etag；內文修改與刪除各加一';
-- Modify "forum_entries" table
ALTER TABLE "social"."forum_entries" ADD CONSTRAINT "forum_entries_version_check" CHECK (version >= 1), ADD COLUMN "version" bigint NOT NULL DEFAULT 1;
-- Set comment to column: "version" on table: "forum_entries"
COMMENT ON COLUMN "social"."forum_entries"."version" IS '樂觀鎖的版本，對外是 etag；內文修改與刪除各加一';
