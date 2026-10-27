-- Set comment to column: "user_id" on table: "cms_comment"
COMMENT ON COLUMN "cms_comment"."user_id" IS '刻意保留泛型命名（不改 buyer_id）：賣家身份操作時可能持有未 rekey 的 raw account_id';
-- Set comment to column: "user_id" on table: "cms_comment_like"
COMMENT ON COLUMN "cms_comment_like"."user_id" IS '刻意保留泛型命名（不改 buyer_id）：賣家身份操作時可能持有未 rekey 的 raw account_id';
-- Set comment to column: "user_id" on table: "cms_post_like"
COMMENT ON COLUMN "cms_post_like"."user_id" IS '刻意保留泛型命名（不改 buyer_id）：賣家身份操作時可能持有未 rekey 的 raw account_id';
-- Set comment to column: "user_id" on table: "cms_seller_forum"
COMMENT ON COLUMN "cms_seller_forum"."user_id" IS '刻意保留泛型命名（不改 buyer_id）：賣家身份操作時可能持有未 rekey 的 raw account_id';
-- Set comment to column: "reporter_id" on table: "reports"
COMMENT ON COLUMN "reports"."reporter_id" IS '刻意保留泛型命名（不改 buyer_id）：賣家身份操作時可能持有未 rekey 的 raw account_id';
-- Set comment to column: "blocker_id" on table: "user_blocks"
COMMENT ON COLUMN "user_blocks"."blocker_id" IS '刻意保留泛型命名（不改 buyer_id）：賣家身份操作時可能持有未 rekey 的 raw account_id';
