-- issue #1176 PR-2: 刪除從未投遞的 broken refund notification template。
-- 三者 target_user_field 對應的 account id 根本不在 payment payload → extractUserID 找不到 → 0 delivery。
-- 售後退款買家/賣家通知已由 aftersales.refund.completed.* 提供；rescue 買家版正常保留；
-- refund.succeeded / refund.failed event 本身保留給 order/aftersales 業務 consumer（非通知）。

DELETE FROM notification_templates
WHERE (event_type = 'refund.succeeded' AND target_user_type = 'buyer')
   OR (event_type = 'refund.succeeded.seller' AND target_user_type = 'seller')
   OR (event_type = 'payment.rescue.refund_completed.seller' AND target_user_type = 'seller');
