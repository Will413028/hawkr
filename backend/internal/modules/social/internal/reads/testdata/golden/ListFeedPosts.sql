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
FROM social.posts AS p
     INNER JOIN social.post_stats AS s ON (s.post_id = p.id)
WHERE (((p.status = 'PUBLISHED') AND (p.deleted_at IS NULL)) AND (EXISTS (SELECT 1 FROM seller.pub_exposable_stores($1::timestamp with time zone) AS pub WHERE pub.store_id = p.store_id))) AND (ROW(p.published_at, p.id) < ROW($2::timestamp with time zone, $3::uuid))
ORDER BY p.published_at DESC, p.id DESC
LIMIT $4;

-- args:
-- $1 = 2026-10-23 12:00:00 +0000 UTC
-- $2 = 2026-10-23 12:00:00 +0000 UTC
-- $3 = 018f0000-0000-7000-8000-000000000001
-- $4 = 51
