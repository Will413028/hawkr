-- Seller notification identity-migration marker, batch 2 of N (issue #2707).
--
-- 語意同 20260830100002_seller_identity_migrated_batch1.sql：`identity_migrated`
-- 標記「這張 seller 側 template 的 target_user_field 現在指向的是已解析的當前
-- store_id，不是登入 account_id」——跟 20260830110000_seller_target_field_store_id_
-- rekey_batch2.sql 是同一件事的兩面，兩份 migration 綁定同一組 36 支 event_type。
--
-- 36 支清單逐字取自 20260830110000 的五個 WHERE ... event_type IN (...) 子句
-- （Part A/B/C/D/E 合計 25+1+6+1+3=36 條，不是改寫或摘要）：
UPDATE "notification_templates"
SET identity_migrated = true,
    updated_at         = CURRENT_TIMESTAMP
WHERE target_user_type = 'seller'
  AND event_type IN (
    'aftersales.appeal.submitted.seller',
    'aftersales.case.applied.seller',
    'aftersales.case.auto_completed_confirm_receipt_timeout.seller',
    'aftersales.case.auto_completed_return_pending_timeout.seller',
    'aftersales.case.auto_completed_store_pickup_timeout.seller',
    'aftersales.case.auto_escalated.seller',
    'aftersales.case.cancelled_by_buyer.seller',
    'aftersales.case.cancelled_by_confirm_receipt.seller',
    'aftersales.case.coupon_compensated.seller',
    'aftersales.case.refund_approved.seller',
    'aftersales.case.rejected.seller',
    'aftersales.case.return_approved.seller',
    'aftersales.case.return_at_store.seller',
    'aftersales.case.return_at_store_24h.seller',
    'aftersales.case.return_delivered.seller',
    'aftersales.case.return_delivered_24h.seller',
    'aftersales.case.return_failed.seller',
    'aftersales.case.return_in_transit.seller',
    'aftersales.case.return_received.seller',
    'aftersales.platform_decision.made.seller',
    'aftersales.refund.completed.seller',
    'product.favorited',
    'shipment.anomaly.detected.seller',
    'shipment.created',
    'shipment.delivered.seller',
    'chat.message.sent',
    'follow.added',
    'merchant.verification.approved',
    'merchant.verification.forwarded',
    'merchant.verification.pending_info',
    'merchant.verification.rejected',
    'merchant.verification.under_review',
    'payout.completed',
    'product.viewed',
    'post.hidden_by_platform.seller',
    'post.liked'
  );
