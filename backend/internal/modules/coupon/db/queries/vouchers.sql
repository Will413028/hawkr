-- 券：建立、鎖定、核銷、釋放、作廢與稽核紀錄。

-- name: InsertVoucher :execrows
INSERT INTO coupon.coupon_vouchers (id, campaign_id, buyer_id, origin, source_type, source_id, status, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(campaign_id), sqlc.arg(buyer_id), sqlc.arg(origin), sqlc.narg(source_type), sqlc.narg(source_id), 'AVAILABLE', sqlc.arg(now), sqlc.arg(now))
ON CONFLICT (campaign_id, buyer_id) DO NOTHING;

-- name: ListHeldBuyers :many
SELECT buyer_id FROM coupon.coupon_vouchers WHERE campaign_id = sqlc.arg(campaign_id) AND buyer_id = ANY(sqlc.arg(buyer_ids)::bigint[]);

-- name: GetVoucher :one
SELECT * FROM coupon.coupon_vouchers WHERE id = sqlc.arg(id);

-- name: GetVoucherByCampaignBuyer :one
SELECT * FROM coupon.coupon_vouchers WHERE campaign_id = sqlc.arg(campaign_id) AND buyer_id = sqlc.arg(buyer_id);

-- name: GetVoucherByOrderNo :one
SELECT * FROM coupon.coupon_vouchers WHERE order_no = sqlc.arg(order_no);

-- name: LockVoucher :one
-- 一張券同時只被一筆訂單鎖：只有 AVAILABLE 的券能被鎖，回 ErrNoRows 表示沒有符合的券（不存在、不是本人的，或已被鎖定、核銷、作廢）。
UPDATE coupon.coupon_vouchers
SET status = 'LOCKED', order_no = sqlc.arg(order_no), locked_at = sqlc.arg(now), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND buyer_id = sqlc.arg(buyer_id) AND status = 'AVAILABLE'
RETURNING id, campaign_id, buyer_id;

-- name: RedeemVoucher :one
UPDATE coupon.coupon_vouchers
SET status = 'REDEEMED', redeemed_at = sqlc.arg(now), updated_at = sqlc.arg(now)
WHERE order_no = sqlc.arg(order_no) AND status = 'LOCKED'
RETURNING id, campaign_id, buyer_id;

-- name: ReleaseVoucher :one
-- 釋放持有這張券的訂單（鎖定或已核銷，後者是賣家取消已付款訂單）。活動已結束時券不回到可用，改為作廢（D5：結束後 AVAILABLE 都作廢）。
UPDATE coupon.coupon_vouchers
SET status = sqlc.arg(to_status)::text, order_no = NULL, locked_at = NULL, redeemed_at = NULL,
    voided_at = sqlc.narg(voided_at), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND order_no = sqlc.arg(order_no) AND status IN ('LOCKED', 'REDEEMED')
RETURNING id, campaign_id, buyer_id;

-- name: VoidAvailableVouchers :execrows
-- 結束活動：只作廢 AVAILABLE；LOCKED 仍可核銷或釋放（D5）。
UPDATE coupon.coupon_vouchers SET status = 'VOIDED', voided_at = sqlc.arg(now), updated_at = sqlc.arg(now)
WHERE campaign_id = sqlc.arg(campaign_id) AND status = 'AVAILABLE';

-- name: VoidVoucher :one
-- admin 作廢單張券：AVAILABLE 或 LOCKED（Release 漏送而卡住的券，D3）；已核銷的不能作廢。
UPDATE coupon.coupon_vouchers
SET status = 'VOIDED', order_no = NULL, locked_at = NULL, voided_at = sqlc.arg(now), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status IN ('AVAILABLE', 'LOCKED')
RETURNING id, campaign_id, buyer_id;

-- name: InsertLog :exec
INSERT INTO coupon.coupon_redemption_logs (campaign_id, voucher_id, buyer_id, action, order_no, operator_admin_id, operator_store_id, reason, voided_count, created_at)
VALUES (sqlc.arg(campaign_id), sqlc.narg(voucher_id), sqlc.narg(buyer_id), sqlc.arg(action), sqlc.narg(order_no), sqlc.narg(operator_admin_id), sqlc.narg(operator_store_id), sqlc.narg(reason), sqlc.narg(voided_count), sqlc.arg(now));

-- name: GetVoucherBySource :one
SELECT * FROM coupon.coupon_vouchers WHERE source_type = sqlc.arg(source_type) AND source_id = sqlc.arg(source_id);
