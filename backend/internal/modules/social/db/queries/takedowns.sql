-- 內容下架的複本（schema.sql 的 content_takedowns）：訂閱者在 seller 建立與撤銷內容下架措施的同一個 transaction 寫入，每日對帳雙向核對 seller.pub_content_takedowns()。

-- name: InsertTakedowns :execrows
-- 冪等：(措施, 內容) 已經有的列略過。內容不存在時外鍵失敗，措施的建立跟著回滾。
INSERT INTO social.content_takedowns (measure_id, post_id, comment_id, forum_entry_id, created_at)
SELECT sqlc.arg(measure_id)::uuid, p, NULL::uuid, NULL::uuid, sqlc.arg(created_at)::timestamptz FROM unnest(sqlc.arg(post_ids)::uuid[]) AS p
UNION ALL
SELECT sqlc.arg(measure_id)::uuid, NULL::uuid, c, NULL::uuid, sqlc.arg(created_at)::timestamptz FROM unnest(sqlc.arg(comment_ids)::uuid[]) AS c
UNION ALL
SELECT sqlc.arg(measure_id)::uuid, NULL::uuid, NULL::uuid, f, sqlc.arg(created_at)::timestamptz FROM unnest(sqlc.arg(forum_entry_ids)::uuid[]) AS f
ON CONFLICT DO NOTHING;

-- name: InvalidTakedownContents :many
-- 內容下架的歸屬核對（建立措施的同一個 transaction，留言與討論區留言的列已經鎖住）：回傳不存在、已刪除、或不是該店家自己寫的內容。貼文的作者是它所屬的店家。
SELECT 'POST'::text AS content_kind, p::uuid AS content_id FROM unnest(sqlc.arg(post_ids)::uuid[]) AS p
WHERE NOT EXISTS (SELECT 1 FROM social.posts AS x WHERE x.id = p AND x.store_id = sqlc.arg(store_id)::bigint AND x.deleted_at IS NULL)
UNION ALL
SELECT 'COMMENT'::text, c::uuid FROM unnest(sqlc.arg(comment_ids)::uuid[]) AS c
WHERE NOT EXISTS (SELECT 1 FROM social.comments AS x WHERE x.id = c AND x.actor_kind = 'STORE' AND x.actor_id = sqlc.arg(store_id)::bigint AND x.deleted_at IS NULL)
UNION ALL
SELECT 'FORUM_ENTRY'::text, f::uuid FROM unnest(sqlc.arg(forum_entry_ids)::uuid[]) AS f
WHERE NOT EXISTS (SELECT 1 FROM social.forum_entries AS x WHERE x.id = f AND x.actor_kind = 'STORE' AND x.actor_id = sqlc.arg(store_id)::bigint AND x.deleted_at IS NULL)
ORDER BY 1, 2;

-- name: InsertActiveTakedowns :execrows
-- 對帳補漏：只寫此刻 seller.pub_content_takedowns() 仍生效的列（在寫入的語句裡再看一次，讀到「缺」的清單之後措施才被撤銷時不寫回）。冪等：已有的列略過。
INSERT INTO social.content_takedowns (measure_id, post_id, comment_id, forum_entry_id, created_at)
SELECT t.measure_id,
       CASE WHEN t.content_kind = 'POST' THEN t.content_id END,
       CASE WHEN t.content_kind = 'COMMENT' THEN t.content_id END,
       CASE WHEN t.content_kind = 'FORUM_ENTRY' THEN t.content_id END,
       sqlc.arg(created_at)::timestamptz
FROM seller.pub_content_takedowns() AS t
WHERE t.measure_id = sqlc.arg(measure_id)::uuid
  AND ((t.content_kind = 'POST' AND t.content_id = ANY(sqlc.arg(post_ids)::uuid[]))
    OR (t.content_kind = 'COMMENT' AND t.content_id = ANY(sqlc.arg(comment_ids)::uuid[]))
    OR (t.content_kind = 'FORUM_ENTRY' AND t.content_id = ANY(sqlc.arg(forum_entry_ids)::uuid[])))
ON CONFLICT DO NOTHING;

-- name: TakedownsOfMeasure :many
SELECT post_id, comment_id, forum_entry_id FROM social.content_takedowns
WHERE measure_id = sqlc.arg(measure_id);

-- name: DeleteTakedownsOfMeasure :execrows
DELETE FROM social.content_takedowns WHERE measure_id = sqlc.arg(measure_id);

-- name: DeleteTakedownContents :execrows
-- 對帳刪除多出來的列：一個措施底下的指定內容。
DELETE FROM social.content_takedowns
WHERE measure_id = sqlc.arg(measure_id)
  AND (post_id = ANY(sqlc.arg(post_ids)::uuid[])
    OR comment_id = ANY(sqlc.arg(comment_ids)::uuid[])
    OR forum_entry_id = ANY(sqlc.arg(forum_entry_ids)::uuid[]));

-- name: LockCommentsByID :many
-- 依 ID 由小到大鎖住內容列（FOR NO KEY UPDATE 在排序之後依序取鎖），之後才改複本與求值。
SELECT id FROM social.comments WHERE id = ANY(sqlc.arg(ids)::uuid[]) ORDER BY id FOR NO KEY UPDATE;

-- name: LockForumEntriesByID :many
SELECT id FROM social.forum_entries WHERE id = ANY(sqlc.arg(ids)::uuid[]) ORDER BY id FOR NO KEY UPDATE;

-- name: TakedownPostStores :many
-- 受影響的貼文與它們的店家：直接下架的貼文，加上被下架的留言所屬的貼文（留言的快取在貼文頁）。
SELECT p.id, p.store_id FROM social.posts AS p WHERE p.id = ANY(sqlc.arg(post_ids)::uuid[])
UNION
SELECT p.id, p.store_id FROM social.comments AS c JOIN social.posts AS p ON p.id = c.post_id WHERE c.id = ANY(sqlc.arg(comment_ids)::uuid[])
ORDER BY 1;

-- name: TakedownForumStores :many
SELECT DISTINCT store_id FROM social.forum_entries WHERE id = ANY(sqlc.arg(ids)::uuid[]) ORDER BY store_id;

-- name: MissingTakedowns :many
-- seller 發布的「此刻生效中的內容下架」裡，複本還沒有、而且內容在 social 存在的（漂移）：每日對帳補上。內容不存在的略過（插入會違反外鍵）。
SELECT t.measure_id, t.content_kind, t.content_id FROM seller.pub_content_takedowns() AS t
WHERE NOT EXISTS (
    SELECT 1 FROM social.content_takedowns AS c
    WHERE c.measure_id = t.measure_id
      AND ((t.content_kind = 'POST' AND c.post_id = t.content_id)
        OR (t.content_kind = 'COMMENT' AND c.comment_id = t.content_id)
        OR (t.content_kind = 'FORUM_ENTRY' AND c.forum_entry_id = t.content_id)))
  AND ((t.content_kind = 'POST' AND EXISTS (SELECT 1 FROM social.posts AS p WHERE p.id = t.content_id))
    OR (t.content_kind = 'COMMENT' AND EXISTS (SELECT 1 FROM social.comments AS m WHERE m.id = t.content_id))
    OR (t.content_kind = 'FORUM_ENTRY' AND EXISTS (SELECT 1 FROM social.forum_entries AS f WHERE f.id = t.content_id)))
ORDER BY t.measure_id, t.content_kind, t.content_id;

-- name: ExtraTakedowns :many
-- 複本有、seller 此刻沒有生效中的內容下架（撤銷時訂閱者漏了）：每日對帳刪除。
SELECT c.measure_id, c.post_id, c.comment_id, c.forum_entry_id FROM social.content_takedowns AS c
WHERE NOT EXISTS (
    SELECT 1 FROM seller.pub_content_takedowns() AS t
    WHERE t.measure_id = c.measure_id
      AND ((t.content_kind = 'POST' AND c.post_id = t.content_id)
        OR (t.content_kind = 'COMMENT' AND c.comment_id = t.content_id)
        OR (t.content_kind = 'FORUM_ENTRY' AND c.forum_entry_id = t.content_id)))
ORDER BY c.measure_id, c.created_at;
