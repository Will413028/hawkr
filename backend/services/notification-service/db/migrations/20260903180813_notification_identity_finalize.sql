-- Final recipient identity cutover.
--
-- All seller templates now target the store surrogate. The two procurement
-- quote-invite rows are prepared for the future producer but remain disabled:
-- procurement.events topology and the ai-service publisher are not implemented
-- in this release, so this migration must not enable them.
--
-- Buyer cleanup is limited to the two producer-backed rows whose payloads
-- already carry buyer_id. Existing compatibility wire keys remain in the
-- payloads for one rolling-deployment window.
--
-- Historical notifications are deliberately not rewritten. Retired buyer and
-- seller stints are handled by the existing lifecycle soft-delete consumers and
-- 90-day cleanup job; the final query no longer exposes an account fallback.

UPDATE "notification_templates"
SET "target_user_field" = 'store_id',
    "identity_migrated" = true,
    "updated_at" = CURRENT_TIMESTAMP
WHERE "target_user_type" = 'seller'
  AND "event_type" IN (
    'procurement.quote_invite.created.seller',
    'procurement.quote_invite.lost.seller'
  );

UPDATE "notification_templates"
SET "target_user_field" = 'buyer_id',
    "updated_at" = CURRENT_TIMESTAMP
WHERE "target_user_type" = 'buyer'
  AND "event_type" = 'payment.rescue.refund_completed';

UPDATE "notification_templates"
SET "target_user_field" = 'buyer_id',
    "updated_at" = CURRENT_TIMESTAMP
WHERE "target_user_type" = 'buyer'
  AND "event_type" = 'product.restock.target';

ALTER TABLE "notification_templates"
  ADD CONSTRAINT "chk_notification_templates_seller_identity_migrated"
  CHECK (((target_user_type)::text <> 'seller'::text) OR (identity_migrated = true));
