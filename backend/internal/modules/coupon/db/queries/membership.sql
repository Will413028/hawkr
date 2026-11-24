-- MEMBER 資格的消費紀錄投影與買家刪帳的清除。

-- name: UpsertMembership :execrows
-- 重放冪等：已存在就不動，回傳 0 列。
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
-- 刪帳：只作廢該買家 AVAILABLE 的券（同 Java 的 voidAllUnusedByOwner）；LOCKED 由持鎖訂單的 Release 或 Redeem 處理，已核銷與已作廢的留作紀錄。
UPDATE coupon.coupon_vouchers SET status = 'VOIDED', voided_at = sqlc.arg(now), updated_at = sqlc.arg(now)
WHERE buyer_id = sqlc.arg(buyer_id) AND status = 'AVAILABLE'
RETURNING id, campaign_id;
