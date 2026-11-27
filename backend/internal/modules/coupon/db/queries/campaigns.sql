
-- name: InsertCampaign :execrows
INSERT INTO coupon.coupon_campaigns (
    id, title, discount_type, discount_amount, discount_percent, max_discount_amount, threshold_amount,
    target_type, store_id, issuer_type, issuer_admin_id, issuer_store_id, start_at, end_at,
    total_limit, status, source_type, source_id, created_at, updated_at)
VALUES (
    sqlc.arg(id), sqlc.arg(title), sqlc.arg(discount_type), sqlc.narg(discount_amount), sqlc.narg(discount_percent), sqlc.narg(max_discount_amount), sqlc.arg(threshold_amount),
    sqlc.arg(target_type), sqlc.narg(store_id), sqlc.arg(issuer_type), sqlc.narg(issuer_admin_id), sqlc.narg(issuer_store_id), sqlc.arg(start_at), sqlc.arg(end_at),
    sqlc.narg(total_limit), sqlc.arg(status), sqlc.narg(source_type), sqlc.narg(source_id), sqlc.arg(now), sqlc.arg(now))
ON CONFLICT (source_type, source_id) DO NOTHING;

-- name: GetCampaign :one
SELECT * FROM coupon.coupon_campaigns WHERE id = sqlc.arg(id);

-- name: GetCampaignForUpdate :one
SELECT * FROM coupon.coupon_campaigns WHERE id = sqlc.arg(id) FOR UPDATE;

-- name: GetCampaignForShare :one
SELECT * FROM coupon.coupon_campaigns WHERE id = sqlc.arg(id) FOR SHARE;

-- name: GetCampaignBySource :one
SELECT * FROM coupon.coupon_campaigns WHERE source_type = sqlc.arg(source_type) AND source_id = sqlc.arg(source_id);

-- name: IncrementIssued :execrows
UPDATE coupon.coupon_campaigns
SET issued_count = issued_count + sqlc.arg(count)::int, updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'ACTIVE'
  AND (total_limit IS NULL OR issued_count + sqlc.arg(count)::int <= total_limit);

-- name: PublishCampaign :execrows
UPDATE coupon.coupon_campaigns SET status = 'ACTIVE', updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'DRAFT' AND end_at > sqlc.arg(now);

-- name: EndCampaign :execrows
UPDATE coupon.coupon_campaigns SET status = 'ENDED', ended_at = sqlc.arg(now), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'ACTIVE';

-- name: CountActiveSellerCampaigns :one
SELECT count(*) FROM coupon.coupon_campaigns
WHERE issuer_store_id = sqlc.arg(store_id) AND status = 'ACTIVE' AND end_at >= sqlc.arg(now) AND target_type <> 'COMPENSATION';

-- name: InsertQuotaIfAbsent :exec
INSERT INTO coupon.seller_coupon_quotas (store_id, updated_at) VALUES (sqlc.arg(store_id), sqlc.arg(now))
ON CONFLICT (store_id) DO NOTHING;

-- name: LockQuota :one
SELECT max_campaign_count FROM coupon.seller_coupon_quotas WHERE store_id = sqlc.arg(store_id) FOR UPDATE;

-- name: UpsertQuota :exec
INSERT INTO coupon.seller_coupon_quotas (store_id, max_campaign_count, updated_at)
VALUES (sqlc.arg(store_id), sqlc.arg(max_campaign_count), sqlc.arg(now))
ON CONFLICT (store_id) DO UPDATE SET max_campaign_count = EXCLUDED.max_campaign_count, updated_at = EXCLUDED.updated_at;

-- name: InsertTargetAccounts :execrows
INSERT INTO coupon.coupon_target_accounts (campaign_id, buyer_id, created_at)
SELECT sqlc.arg(campaign_id), b, sqlc.arg(now) FROM unnest(sqlc.arg(buyer_ids)::bigint[]) AS b
ON CONFLICT (campaign_id, buyer_id) DO NOTHING;

-- name: HasTargetAccount :one
SELECT EXISTS (
    SELECT 1 FROM coupon.coupon_target_accounts WHERE campaign_id = sqlc.arg(campaign_id) AND buyer_id = sqlc.arg(buyer_id));


-- name: ListSellerCampaigns :many
SELECT * FROM coupon.coupon_campaigns
WHERE issuer_store_id = sqlc.arg(store_id) AND target_type <> 'COMPENSATION'
  AND (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status)::text)
  AND (sqlc.narg(after_created_at)::timestamptz IS NULL OR (created_at, id) < (sqlc.narg(after_created_at)::timestamptz, sqlc.narg(after_id)::uuid))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(max_rows);

-- name: ListAdminCampaigns :many
SELECT * FROM coupon.coupon_campaigns
WHERE target_type <> 'COMPENSATION'
  AND (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status)::text)
  AND (sqlc.narg(issuer_type)::text IS NULL OR issuer_type = sqlc.narg(issuer_type)::text)
  AND (sqlc.narg(issuer_store_id)::bigint IS NULL OR issuer_store_id = sqlc.narg(issuer_store_id)::bigint)
  AND (sqlc.narg(after_created_at)::timestamptz IS NULL OR (created_at, id) < (sqlc.narg(after_created_at)::timestamptz, sqlc.narg(after_id)::uuid))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(max_rows);

-- name: ListClaimCandidates :many
SELECT c.* FROM coupon.coupon_campaigns c
WHERE c.status = 'ACTIVE' AND c.target_type <> 'COMPENSATION'
  AND c.start_at <= sqlc.arg(now)::timestamptz AND c.end_at >= sqlc.arg(now)::timestamptz
  AND (c.total_limit IS NULL OR c.issued_count < c.total_limit)
  AND (sqlc.narg(store_id)::bigint IS NULL OR c.store_id IS NULL OR c.store_id = sqlc.narg(store_id)::bigint)
  AND (c.target_type <> 'DESIGNATED' OR EXISTS (SELECT 1 FROM coupon.coupon_target_accounts t WHERE t.campaign_id = c.id AND t.buyer_id = sqlc.arg(buyer_id)))
  AND NOT EXISTS (SELECT 1 FROM coupon.coupon_vouchers v WHERE v.campaign_id = c.id AND v.buyer_id = sqlc.arg(buyer_id))
  AND (sqlc.narg(after_created_at)::timestamptz IS NULL OR (c.created_at, c.id) < (sqlc.narg(after_created_at)::timestamptz, sqlc.narg(after_id)::uuid))
ORDER BY c.created_at DESC, c.id DESC
LIMIT sqlc.arg(max_rows);

-- name: ListCampaignsByIDs :many
SELECT * FROM coupon.coupon_campaigns WHERE id = ANY(sqlc.arg(ids)::uuid[]);

-- name: ListTargetBuyers :many
SELECT buyer_id FROM coupon.coupon_target_accounts WHERE campaign_id = sqlc.arg(campaign_id) ORDER BY buyer_id;

-- name: GetSellerQuota :one
SELECT max_campaign_count FROM coupon.seller_coupon_quotas WHERE store_id = sqlc.arg(store_id);
