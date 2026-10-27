-- Modify "app_version_gates" table
ALTER TABLE "content"."app_version_gates" ADD COLUMN "version" bigint NOT NULL DEFAULT 1;
-- Set comment to column: "version" on table: "app_version_gates"
COMMENT ON COLUMN "content"."app_version_gates"."version" IS '樂觀鎖的版本，對外是 etag。沒有刪除，所以不必區分同一組 key 的不同建立（ADR D4）';
-- Modify "banners" table
ALTER TABLE "content"."banners" ADD COLUMN "version" bigint NOT NULL DEFAULT 1;
-- Set comment to column: "version" on table: "banners"
COMMENT ON COLUMN "content"."banners"."version" IS '樂觀鎖的版本，對外是 etag；每次修改加一（ADR 2026-09-25-optimistic-concurrency-etag）';
-- Modify "recipes" table
ALTER TABLE "content"."recipes" ADD COLUMN "version" bigint NOT NULL DEFAULT 1;
-- Set comment to column: "version" on table: "recipes"
COMMENT ON COLUMN "content"."recipes"."version" IS '樂觀鎖的版本，對外是 etag；食材與步驟只經由食譜寫入，換掉它們也讓這裡加一';
