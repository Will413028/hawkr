-- Seller invoice notification identity rekey (issue #3061).
--
-- The six producer-backed invoice seller templates already receive a raw
-- `store_id` in the event payload.  The value is the seller business key
-- (`issuer_ref_id`, equal to `sellers.store_id`), not the `invoice_issuers.id`
-- technical primary key.  `notify_seller_account_id` is retained in the wire
-- payload as the compatibility account key, but it is no longer the
-- notification template's seller recipient field.
--
-- Flip target_user_field and identity_migrated together.  The existing
-- notification read path will then match new seller rows by current store_id.
-- Historical notifications are append-only and are intentionally not
-- rewritten; old account-keyed rows remain subject to the established
-- migration policy.  `invoice.allowance.issued.buyer` is a separate buyer-side
-- feature and is outside this migration.
UPDATE "notification_templates"
SET "target_user_field" = 'store_id',
    "identity_migrated" = true,
    "updated_at" = CURRENT_TIMESTAMP
WHERE "target_user_type" = 'seller'
  AND "target_user_field" = 'notify_seller_account_id'
  AND "event_type" IN (
    'invoice.issued.seller',
    'invoice.issue_failed.seller',
    'invoice.operation.review_required.seller',
    'invoice.complaint.created.seller',
    'invoice.complaint.overdue.seller',
    'invoice.seller_tax_profile.status_changed.seller'
  );
