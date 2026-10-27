-- Buyer notification write/read identity-space mismatch fix, PR 2 of 2.
-- Gated on aftersales-service's publisher expand (buyer_id now emitted on
-- the wire for both event types) being live in dev — confirmed via
-- live-wire-verify before this migration was written, see
-- docs/superpowers/plans/2026-08-06-buyer-notification-identity-mismatch-fix.md
-- Task 5.
UPDATE "notification_templates"
SET target_user_field = 'buyer_id',
    updated_at        = CURRENT_TIMESTAMP
WHERE target_user_type = 'buyer'
  AND target_user_field = 'notify_buyer_account_id'
  AND event_type IN (
    'aftersales.refund.awaiting_buyer_page.buyer',
    'aftersales.refund.completed.buyer'
  );
