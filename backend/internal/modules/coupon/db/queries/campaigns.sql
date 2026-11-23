-- 活動：建立、讀取、發布、提前結束、發行計數與賣家配額。

-- name: InsertCampaign :execrows
-- 補償券以 (source_type, source_id) 對應來源事件：重送撞 UNIQUE 回 0 列（呼叫者改讀既有的）；其他活動的來源欄是 NULL，不會撞。
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
-- 領券、發券與結束活動的串行點：同一個活動的這三件事排隊，之後的檢查讀到的狀態不會再被改。
SELECT * FROM coupon.coupon_campaigns WHERE id = sqlc.arg(id) FOR UPDATE;

-- name: GetCampaignForShare :one
-- 鎖券與釋放讀活動時共享鎖：結束活動的 UPDATE 要等它們提交，結束之後才進來的看到 ENDED。
SELECT * FROM coupon.coupon_campaigns WHERE id = sqlc.arg(id) FOR SHARE;

-- name: GetCampaignBySource :one
SELECT * FROM coupon.coupon_campaigns WHERE source_type = sqlc.arg(source_type) AND source_id = sqlc.arg(source_id);

-- name: IncrementIssued :execrows
-- 發行計數的原子遞增：條件在 WHERE 裡，回 0 列表示活動不是 ACTIVE 或會超過總量（沒有「先讀、再判斷、再寫」）。
UPDATE coupon.coupon_campaigns
SET issued_count = issued_count + sqlc.arg(count)::int, updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'ACTIVE'
  AND (total_limit IS NULL OR issued_count + sqlc.arg(count)::int <= total_limit);

-- name: PublishCampaign :execrows
UPDATE coupon.coupon_campaigns SET status = 'ACTIVE', updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'DRAFT';

-- name: EndCampaign :execrows
UPDATE coupon.coupon_campaigns SET status = 'ENDED', ended_at = sqlc.arg(now), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'ACTIVE';

-- name: CountActiveSellerCampaigns :one
SELECT count(*) FROM coupon.coupon_campaigns
WHERE issuer_store_id = sqlc.arg(store_id) AND status = 'ACTIVE' AND target_type <> 'COMPENSATION';

-- name: InsertQuotaIfAbsent :exec
INSERT INTO coupon.seller_coupon_quotas (store_id, updated_at) VALUES (sqlc.arg(store_id), sqlc.arg(now))
ON CONFLICT (store_id) DO NOTHING;

-- name: LockQuota :one
-- 建立賣家活動時先鎖配額列：同一家店的配額檢查與插入串行（BD15）。
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
