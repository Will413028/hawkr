
-- name: UpsertMembership :execrows
INSERT INTO coupon.store_buyer_membership (store_id, buyer_id, created_at)
VALUES (sqlc.arg(store_id), sqlc.arg(buyer_id), sqlc.arg(now))
ON CONFLICT (store_id, buyer_id) DO NOTHING;

-- name: HasMembership :one
SELECT EXISTS (SELECT 1 FROM coupon.store_buyer_membership WHERE store_id = sqlc.arg(store_id) AND buyer_id = sqlc.arg(buyer_id));

-- name: DeleteMembershipsByBuyer :execrows
DELETE FROM coupon.store_buyer_membership WHERE buyer_id = sqlc.arg(buyer_id);

-- name: DeleteTargetAccountsByBuyer :execrows
DELETE FROM coupon.coupon_target_accounts WHERE buyer_id = sqlc.arg(buyer_id);

-- name: VoidAvailableVouchersByBuyer :many
UPDATE coupon.coupon_vouchers SET status = 'VOIDED', voided_at = sqlc.arg(now), updated_at = sqlc.arg(now)
WHERE buyer_id = sqlc.arg(buyer_id) AND status = 'AVAILABLE'
RETURNING id, campaign_id;
