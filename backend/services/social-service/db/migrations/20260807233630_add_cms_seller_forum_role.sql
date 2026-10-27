-- Modify "cms_seller_forum" table
ALTER TABLE "cms_seller_forum" ADD COLUMN "role" smallint NOT NULL DEFAULT 1;
-- Set comment to column: "role" on table: "cms_seller_forum"
COMMENT ON COLUMN "cms_seller_forum"."role" IS '留言者角色，決定 user_id 落在 buyer_id 還是 store_id 命名空間——enrichForumUserInfo／resolveForumIdentity 依此分流查 buyer-service 或 seller-service，不得只查其中一邊（2026-08-08，鏡射 cms_comment.role 的既有修法，見 669bc471f）';
