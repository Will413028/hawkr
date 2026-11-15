-- Create index "comments_store_replies" to table: "comments"
CREATE INDEX "comments_store_replies" ON "social"."comments" ("parent_id") WHERE (counted AND (actor_kind = 'STORE'::text));
-- Modify "posts" table
ALTER TABLE "social"."posts" ADD CONSTRAINT "posts_request_check" CHECK ((request_id IS NULL) = (request_hash IS NULL)), ADD COLUMN "request_id" uuid NULL, ADD COLUMN "request_hash" bytea NULL, ADD CONSTRAINT "posts_request_key" UNIQUE ("store_id", "request_id");
-- Set comment to column: "request_id" on table: "posts"
COMMENT ON COLUMN "social"."posts"."request_id" IS '建立時 client 帶的 request_id（AIP-155，選填）；範圍是 (店家, request_id)，同一家店重送回這一列；貼文刪除後它仍占著，重送回原本那一列（刪除的貼文讀不到，回 not_found）';
-- Set comment to column: "request_hash" on table: "posts"
COMMENT ON COLUMN "social"."posts"."request_hash" IS '建立輸入（內文、圖片 token、商品、標籤、是否發布）的 SHA-256；同一個 request_id 重送時比對內容是否相同，與 request_id 同時有或同時沒有';
