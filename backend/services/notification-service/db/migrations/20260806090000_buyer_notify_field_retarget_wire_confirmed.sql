-- Buyer notification write/read identity-space mismatch fix, PR 1 of 2.
-- These 21 templates' publishers already emit `buyer_id` on the wire
-- (confirmed by reading each publisher's payload record directly —
-- see docs/superpowers/plans/2026-08-06-buyer-notification-identity-mismatch-fix.md
-- "Per-Template Audit"). Retargeting closes the write/read id-space mismatch:
-- gateway's inbox query and device-token registration both resolve and key
-- on buyer_id (gateway/internal/buyerid/resolver.go CurrentBuyerID), while
-- this write path previously kept these 21 on notify_buyer_account_id
-- (the resolved login account_id) — silently misrouting both inbox writes
-- and FCM push for every buyer whose buyer_id != account_id.
--
-- The remaining 2 aftersales.refund.* templates are deliberately NOT
-- included here. Their publisher's payload gains a `buyer_id` field in
-- this same PR (see RefundCompletedPayload.java and
-- AwaitingBuyerRefundPagePayload.java), but retargeting these 2 templates
-- is gated on that publisher change being confirmed LIVE IN DEV first —
-- not merely present in source. Retargeting in the same deploy as (or
-- before) the publisher change risks a race where these templates read
-- `buyer_id` before any deployed instance is actually emitting it,
-- silently dropping both notification types entirely. See PR 2's
-- migration, services/notification-service/db/migrations/
-- <timestamp>_aftersales_refund_notify_field_retarget.sql — not yet
-- created as of this commit; see this plan's Task 6.
UPDATE "notification_templates"
SET target_user_field = 'buyer_id',
    updated_at        = CURRENT_TIMESTAMP
WHERE target_user_type = 'buyer'
  AND target_user_field = 'notify_buyer_account_id'
  AND event_type IN (
    'order.paid',
    'order.completed.buyer',
    'order.cancelled.buyer',
    'order.cancelled.seller_suspended',
    'shipment.delivered',
    'shipment.status.changed',
    'coupon.expiring',
    'aftersales.appeal.submitted.buyer',
    'aftersales.case.applied.buyer',
    'aftersales.case.auto_completed_return_pending_timeout.buyer',
    'aftersales.case.auto_escalated.buyer',
    'aftersales.case.auto_rejected.buyer',
    'aftersales.case.cancelled_by_buyer.buyer',
    'aftersales.case.coupon_compensated.buyer',
    'aftersales.case.refund_approved.buyer',
    'aftersales.case.rejected.buyer',
    'aftersales.case.return_approved.buyer',
    'aftersales.case.return_failed.buyer',
    'aftersales.case.return_pending_24h.buyer',
    'aftersales.case.return_received.buyer',
    'aftersales.platform_decision.made.buyer'
  );
