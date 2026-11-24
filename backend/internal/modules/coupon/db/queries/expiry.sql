-- 到期提醒：單列游標與落在提醒窗的 AVAILABLE 券。

-- name: EnsureExpiryCursor :exec
INSERT INTO coupon.expiry_reminder_cursor (end_at, voucher_id) VALUES (sqlc.arg(end_at), sqlc.arg(voucher_id))
ON CONFLICT (singleton) DO NOTHING;

-- name: LockExpiryCursor :one
SELECT end_at, voucher_id FROM coupon.expiry_reminder_cursor WHERE singleton FOR UPDATE;

-- name: AdvanceExpiryCursor :exec
UPDATE coupon.expiry_reminder_cursor SET end_at = sqlc.arg(end_at), voucher_id = sqlc.arg(voucher_id) WHERE singleton;

-- name: ListExpiringVouchers :many
-- 只有 AVAILABLE 的券：LOCKED 已有訂單、REDEEMED 與 VOIDED 已結束。到期從活動的 end_at 推導；(end_at, id) 是穩定的鍵集順序。
SELECT v.id, v.campaign_id, v.buyer_id, c.title, c.end_at
FROM coupon.coupon_vouchers v
JOIN coupon.coupon_campaigns c ON c.id = v.campaign_id
WHERE v.status = 'AVAILABLE'
  AND c.end_at > sqlc.arg(now)::timestamptz
  AND c.end_at <= sqlc.arg(horizon)::timestamptz
  AND (c.end_at, v.id) > (sqlc.arg(after_end_at)::timestamptz, sqlc.arg(after_id)::uuid)
ORDER BY c.end_at, v.id
LIMIT sqlc.arg(batch_size)::int;
