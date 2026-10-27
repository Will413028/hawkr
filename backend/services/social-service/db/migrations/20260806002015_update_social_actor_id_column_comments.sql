-- Set comment to column: "user_id" on table: "cms_comment"
COMMENT ON COLUMN "cms_comment"."user_id" IS '依當下 active role 存現任 buyer_id 或現任 store_id，皆非 raw account_id；僅查無現任 stint（ErrNoCurrentBuyer/ErrNoCurrentStore）時 fallback 用 account_id，見 2026-08-01 ADR（seller-content-moderation-identity-store-id-rekey）';
-- Set comment to column: "user_id" on table: "cms_comment_like"
COMMENT ON COLUMN "cms_comment_like"."user_id" IS '依當下 active role 存現任 buyer_id 或現任 store_id，皆非 raw account_id；僅查無現任 stint（ErrNoCurrentBuyer/ErrNoCurrentStore）時 fallback 用 account_id，見 2026-08-01 ADR（seller-content-moderation-identity-store-id-rekey）';
-- Set comment to column: "user_id" on table: "cms_post_like"
COMMENT ON COLUMN "cms_post_like"."user_id" IS '依當下 active role 存現任 buyer_id 或現任 store_id，皆非 raw account_id；僅查無現任 stint（ErrNoCurrentBuyer/ErrNoCurrentStore）時 fallback 用 account_id，見 2026-08-01 ADR（seller-content-moderation-identity-store-id-rekey）';
-- Set comment to column: "user_id" on table: "cms_seller_forum"
COMMENT ON COLUMN "cms_seller_forum"."user_id" IS '依當下 active role 存現任 buyer_id 或現任 store_id，皆非 raw account_id；僅查無現任 stint（ErrNoCurrentBuyer/ErrNoCurrentStore）時 fallback 用 account_id，見 2026-08-01 ADR（seller-content-moderation-identity-store-id-rekey）';
-- Set comment to column: "reporter_id" on table: "reports"
COMMENT ON COLUMN "reports"."reporter_id" IS '依當下 active role 存現任 buyer_id 或現任 store_id，皆非 raw account_id；僅查無現任 stint（ErrNoCurrentBuyer/ErrNoCurrentStore）時 fallback 用 account_id，見 2026-08-01 ADR（seller-content-moderation-identity-store-id-rekey）';
-- Set comment to column: "blocker_id" on table: "user_blocks"
COMMENT ON COLUMN "user_blocks"."blocker_id" IS '依當下 active role 存現任 buyer_id 或現任 store_id，皆非 raw account_id；僅查無現任 stint（ErrNoCurrentBuyer/ErrNoCurrentStore）時 fallback 用 account_id，見 2026-08-01 ADR（seller-content-moderation-identity-store-id-rekey）';
