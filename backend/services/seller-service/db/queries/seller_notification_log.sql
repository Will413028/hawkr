-- B-169 Seller Notification Log queries
-- audit 追蹤：admin 主動觸發（assist_investigation / temporary_takedown_self_check）
-- 與系統自動觸發（full_suspension_notice / violation_created / violation_revoked / appeal_decided）
-- 兩條路徑共用此 log。

-- name: CreateSellerNotificationLog :one
INSERT INTO seller_notification_log (
  id,
  source_report_id,
  source_report_table,
  template_key,
  seller_id,
  sent_by_admin_id,
  message_payload,
  sent_at
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, NOW()
) RETURNING *;

-- name: ListNotificationsBySourceReport :many
-- admin UI detail 頁顯示：「此 report 已發送過哪些通知」
SELECT * FROM seller_notification_log
WHERE source_report_table = $1
  AND source_report_id    = $2
ORDER BY sent_at DESC;

-- name: ListNotificationsBySeller :many
-- 賣家自助 / admin 看某 seller 收到的所有通知
SELECT * FROM seller_notification_log
WHERE seller_id = $1
ORDER BY sent_at DESC
LIMIT $2 OFFSET $3;
