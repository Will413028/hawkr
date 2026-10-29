SELECT p.id AS "id",
     p.store_id AS "store_id",
     p.body AS "body",
     p.image_key AS "image_key",
     p.image_asset_id AS "image_asset_id",
     p.product_id AS "product_id",
     p.status AS "status",
     p.published_at AS "published_at",
     p.version AS "version",
     p.created_at AS "created_at",
     p.updated_at AS "updated_at",
     p.deleted_at AS "deleted_at",
     s.post_id AS "post_id",
     s.like_count AS "like_count",
     s.comment_count AS "comment_count",
     s.view_count AS "view_count",
     s.share_count AS "share_count"
FROM social.post_hashtags AS h
     INNER JOIN social.posts AS p ON (p.id = h.post_id)
     INNER JOIN social.post_stats AS s ON (s.post_id = p.id)
WHERE (((((((p.status = 'PUBLISHED') AND (p.deleted_at IS NULL)) AND (NOT (EXISTS (
           SELECT td.post_id AS "td.post_id"
           FROM social.content_takedowns AS td
           WHERE td.post_id = p.id
      )))) AND (EXISTS (SELECT 1 FROM seller.pub_exposable_stores($1::timestamp with time zone) AS pub WHERE pub.store_id = p.store_id))) AND (NOT (EXISTS (SELECT 1 FROM buyer.pub_blocked_stores() AS pub WHERE pub.blocker_buyer_id = $2::bigint AND ((store_id = p.store_id)))))) AND (h.name = $3::text)) AND (h.published_at IS NOT NULL)) AND (ROW(h.published_at, h.post_id) < ROW($4::timestamp with time zone, $5::uuid))
ORDER BY h.published_at DESC, h.post_id DESC
LIMIT $6;

-- args:
-- $1 = 2026-10-23 12:00:00 +0000 UTC
-- $2 = 42
-- $3 = organic
-- $4 = 2026-10-23 12:00:00 +0000 UTC
-- $5 = 018f0000-0000-7000-8000-000000000001
-- $6 = 51
