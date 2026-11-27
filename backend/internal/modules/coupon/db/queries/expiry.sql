
-- name: ClaimExpiringVouchers :many
SELECT v.id, v.campaign_id, v.buyer_id, c.title, c.end_at
FROM coupon.coupon_vouchers v
JOIN coupon.coupon_campaigns c ON c.id = v.campaign_id
WHERE v.status = 'AVAILABLE'
  AND NOT EXISTS (SELECT 1 FROM coupon.voucher_expiry_reminders r WHERE r.voucher_id = v.id)
  AND c.end_at > sqlc.arg(now)::timestamptz
  AND c.end_at <= sqlc.arg(now)::timestamptz + make_interval(secs => sqlc.arg(lead_seconds)::float8)
  AND v.created_at <= c.end_at - make_interval(secs => sqlc.arg(lead_seconds)::float8)
  AND v.id > sqlc.arg(after_id)::uuid
  AND (sqlc.narg(only_id)::uuid IS NULL OR v.id = sqlc.narg(only_id)::uuid)
ORDER BY v.id
LIMIT sqlc.arg(batch_size)::int
FOR UPDATE OF v SKIP LOCKED;

-- name: InsertExpiryReminder :exec
INSERT INTO coupon.voucher_expiry_reminders (voucher_id, reminded_at) VALUES (sqlc.arg(voucher_id), sqlc.arg(reminded_at));
