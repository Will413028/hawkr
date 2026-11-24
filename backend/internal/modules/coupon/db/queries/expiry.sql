-- 到期提醒：落在提醒窗、領券時距到期不少於提前量、尚未提醒的 AVAILABLE 券。

-- name: ClaimExpiringVouchers :many
-- 只有 AVAILABLE 的券：LOCKED 已有訂單、REDEEMED 與 VOIDED 已結束；LOCKED 之後釋放回 AVAILABLE 的券下一輪照樣算。
-- 到期從活動的 end_at 推導；「領券時距到期不少於提前量」排除領到時已進窗的券。FOR UPDATE SKIP LOCKED：
-- 同時有別的 job 或鎖定（Lock）在處理的券這一輪略過，下一輪再算。以 v.id 的鍵集分批；only_id 非 NULL 時只處理那一張（失敗隔離重試用）。
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
