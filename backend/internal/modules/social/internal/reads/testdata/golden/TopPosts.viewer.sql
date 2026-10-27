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
FROM (
          SELECT p0.id AS "id"
          FROM social.posts AS p0
          WHERE (((p0.status = 'PUBLISHED') AND (p0.deleted_at IS NULL)) AND (EXISTS (SELECT 1 FROM seller.pub_exposable_stores($1::timestamp with time zone) AS pub WHERE pub.store_id = p0.store_id))) AND (NOT (EXISTS (SELECT 1 FROM buyer.pub_blocked_stores() AS pub WHERE pub.blocker_buyer_id = $2::bigint AND ((store_id = p0.store_id)))))
          ORDER BY p0.published_at DESC, p0.id DESC
          LIMIT $3
     ) AS latest
     INNER JOIN social.posts AS p ON (p.id = latest.id)
     INNER JOIN social.post_stats AS s ON (s.post_id = p.id)
WHERE (((p.status = 'PUBLISHED') AND (p.deleted_at IS NULL)) AND (EXISTS (SELECT 1 FROM seller.pub_exposable_stores($4::timestamp with time zone) AS pub WHERE pub.store_id = p.store_id))) AND (NOT (EXISTS (SELECT 1 FROM buyer.pub_blocked_stores() AS pub WHERE pub.blocker_buyer_id = $5::bigint AND ((store_id = p.store_id)))))
ORDER BY s.view_count DESC, p.published_at DESC, p.id DESC
LIMIT $6;

-- args:
-- $1 = 2026-10-23 12:00:00 +0000 UTC
-- $2 = 42
-- $3 = 30
-- $4 = 2026-10-23 12:00:00 +0000 UTC
-- $5 = 42
-- $6 = 5
