-- Modify "comments" table
ALTER TABLE "social"."comments" ADD CONSTRAINT "comments_request_check" CHECK ((request_id IS NULL) = (request_hash IS NULL)), ADD COLUMN "request_id" uuid NULL, ADD COLUMN "request_hash" bytea NULL, ADD CONSTRAINT "comments_request_key" UNIQUE ("actor_kind", "actor_id", "request_id");
-- Set comment to column: "request_id" on table: "comments"
COMMENT ON COLUMN "social"."comments"."request_id" IS '建立時 client 帶的 request_id（AIP-155，選填）；範圍是 (互動者, request_id)，同一個互動者重送回這一列';
-- Set comment to column: "request_hash" on table: "comments"
COMMENT ON COLUMN "social"."comments"."request_hash" IS '建立輸入（貼文、父留言、被回覆的留言、內文）的 SHA-256；同一個 request_id 重送時比對內容是否相同，與 request_id 同時有或同時沒有';
-- Modify "forum_entries" table
ALTER TABLE "social"."forum_entries" ADD CONSTRAINT "forum_entries_request_check" CHECK ((request_id IS NULL) = (request_hash IS NULL)), ADD COLUMN "request_id" uuid NULL, ADD COLUMN "request_hash" bytea NULL, ADD CONSTRAINT "forum_entries_request_key" UNIQUE ("actor_kind", "actor_id", "request_id");
-- Set comment to column: "request_id" on table: "forum_entries"
COMMENT ON COLUMN "social"."forum_entries"."request_id" IS '建立時 client 帶的 request_id（AIP-155，選填）；範圍是 (互動者, request_id)，同一個互動者重送回這一列';
-- Set comment to column: "request_hash" on table: "forum_entries"
COMMENT ON COLUMN "social"."forum_entries"."request_hash" IS '建立輸入（店家、父項、被回覆的項目、內文）的 SHA-256；同一個 request_id 重送時比對內容是否相同，與 request_id 同時有或同時沒有';
