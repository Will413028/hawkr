-- 讚表沒有身分種類，既有的讚無從歸屬：清空，兩個計數歸零。
TRUNCATE TABLE "cms_comment_like", "cms_post_like";
UPDATE "cms_post" SET "like_count" = 0 WHERE "like_count" <> 0;
UPDATE "cms_comment" SET "like_count" = 0 WHERE "like_count" <> 0;
-- 店家身分只用在回應自己內容的動作：刪除店家身分的留言（連同它底下的回覆）在別家的貼文、別家的討論區。
WITH bad AS (
  DELETE FROM "cms_comment" c
  WHERE c."role" = 2
    AND NOT EXISTS (SELECT 1 FROM "cms_post" p WHERE p."id" = c."post_id" AND p."seller_id" = c."user_id")
  RETURNING c."id"
)
DELETE FROM "cms_comment" WHERE "parent_id" IN (SELECT "id" FROM bad);
WITH bad AS (
  DELETE FROM "cms_seller_forum"
  WHERE "role" = 2 AND "user_id" <> "seller_id"
  RETURNING "id"
)
DELETE FROM "cms_seller_forum" WHERE "parent_id" IN (SELECT "id" FROM bad);
-- 對帳留言數與回覆數（形狀同 ReconcilePostCommentCounts、ReconcileCommentReplyCounts）。
UPDATE "cms_post" p SET "comment_count" = sub.cnt
FROM (
  SELECT p2."id" AS post_id, COUNT(c."id")::int AS cnt
  FROM "cms_post" p2
  LEFT JOIN "cms_comment" c ON c."post_id" = p2."id" AND c."parent_id" = 0 AND c."deleted_at" IS NULL
  WHERE p2."deleted_at" IS NULL
  GROUP BY p2."id"
) sub
WHERE p."id" = sub.post_id AND p."comment_count" <> sub.cnt;
UPDATE "cms_comment" c SET "reply_count" = sub.cnt
FROM (
  SELECT parent."id" AS parent_id, COUNT(r."id")::int AS cnt
  FROM "cms_comment" parent
  LEFT JOIN "cms_comment" r ON r."parent_id" = parent."id" AND r."deleted_at" IS NULL
  WHERE parent."parent_id" = 0 AND parent."deleted_at" IS NULL
  GROUP BY parent."id"
) sub
WHERE c."id" = sub.parent_id AND c."reply_count" <> sub.cnt;
UPDATE "cms_seller_forum" f SET "reply_count" = sub.cnt
FROM (
  SELECT parent."id" AS parent_id, COUNT(r."id")::int AS cnt
  FROM "cms_seller_forum" parent
  LEFT JOIN "cms_seller_forum" r ON r."parent_id" = parent."id" AND r."deleted_at" IS NULL
  WHERE parent."parent_id" = 0 AND parent."deleted_at" IS NULL
  GROUP BY parent."id"
) sub
WHERE f."id" = sub.parent_id AND f."reply_count" <> sub.cnt;
-- Modify "cms_comment" table
ALTER TABLE "cms_comment" ALTER COLUMN "role" DROP DEFAULT, ADD CONSTRAINT "chk_cms_comment_role" CHECK (role = ANY (ARRAY[1, 2]));
-- Modify "cms_seller_forum" table
ALTER TABLE "cms_seller_forum" ALTER COLUMN "role" DROP DEFAULT, ADD CONSTRAINT "chk_cms_seller_forum_role" CHECK (role = ANY (ARRAY[1, 2]));
-- Set comment to column: "user_id" on table: "cms_comment"
COMMENT ON COLUMN "cms_comment"."user_id" IS '留言者的業務鍵，命名空間由 role 決定：role=1 是 buyer_id、role=2 是 store_id（只會是該貼文所屬店家自己），不是 account_id。';
-- Set comment to column: "role" on table: "cms_comment"
COMMENT ON COLUMN "cms_comment"."role" IS '留言者的身分種類（actor_kind）：1=buyer、2=store；與 user_id 合成互動者 (kind, id)。身分由伺服器依貼文歸屬推導，見 ADR 2026-10-17-social-actor-identity。';
-- Modify "cms_comment_like" table
ALTER TABLE "cms_comment_like" DROP CONSTRAINT "cms_comment_like_comment_id_user_id_key", ADD CONSTRAINT "chk_cms_comment_like_actor_kind" CHECK (actor_kind = ANY (ARRAY[1, 2])), ADD COLUMN "actor_kind" smallint NOT NULL, ADD CONSTRAINT "cms_comment_like_comment_id_actor_kind_user_id_key" UNIQUE ("comment_id", "actor_kind", "user_id");
-- Set comment to column: "user_id" on table: "cms_comment_like"
COMMENT ON COLUMN "cms_comment_like"."user_id" IS '按讚者的業務鍵，命名空間由 actor_kind 決定：1 是 buyer_id、2 是 store_id，不是 account_id。';
-- Set comment to column: "actor_kind" on table: "cms_comment_like"
COMMENT ON COLUMN "cms_comment_like"."actor_kind" IS '按讚者的身分種類：1=buyer、2=store（只會是該留言所在貼文所屬的店家自己）。';
-- Modify "cms_post_like" table
ALTER TABLE "cms_post_like" DROP CONSTRAINT "cms_post_like_post_id_user_id_key", ADD CONSTRAINT "chk_cms_post_like_actor_kind" CHECK (actor_kind = 1), ADD COLUMN "actor_kind" smallint NOT NULL, ADD CONSTRAINT "cms_post_like_post_id_actor_kind_user_id_key" UNIQUE ("post_id", "actor_kind", "user_id");
-- Set comment to column: "user_id" on table: "cms_post_like"
COMMENT ON COLUMN "cms_post_like"."user_id" IS '按讚者的業務鍵，命名空間由 actor_kind 決定（恆為 buyer_id），不是 account_id。';
-- Set comment to column: "actor_kind" on table: "cms_post_like"
COMMENT ON COLUMN "cms_post_like"."actor_kind" IS '按讚者的身分種類：貼文讚永遠是本人身分，所以寫入的值恆為 1（buyer）；欄位與唯一鍵照 cms_comment_like 的形狀，讓兩張讚表同一種讀寫。';
-- Set comment to column: "user_id" on table: "cms_seller_forum"
COMMENT ON COLUMN "cms_seller_forum"."user_id" IS '留言者的業務鍵，命名空間由 role 決定：role=1 是 buyer_id、role=2 是 store_id（只會是該討論區所屬店家自己），不是 account_id。';
-- Set comment to column: "role" on table: "cms_seller_forum"
COMMENT ON COLUMN "cms_seller_forum"."role" IS '留言者的身分種類（actor_kind）：1=buyer、2=store；與 user_id 合成互動者 (kind, id)，決定名稱解析、擁有權比對與刪除處理器的範圍。';
