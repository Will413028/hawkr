-- Vendor portal invite EXTERNAL lane golden-fixture marker (#2539 E1, Task 6).
--
-- procurement.rfq_publication_invite.external_created is deliberately NOT
-- dispatched by the DB-driven template engine: the recipient is an off-platform
-- vendor (seller_prospects), not a 'buyer'|'seller' user_id, so the normal
-- target_user_field -> user_id -> notifications_inbox/push_notifications path
-- cannot apply. internal/worker/vendor_invite.go's withVendorInvite intercepts
-- this routing key ahead of the template dispatch and sends email via pkg/mail
-- instead; this row is never read by that path.
--
-- It exists solely so TestTemplateContract's reverse check (golden entry
-- "procurement.rfq_publication_invite.external_created.vendor" must have a
-- matching notification_templates.event_type row, full table including
-- disabled) does not treat this event as an orphaned/retired golden entry.
-- target_user_field is deliberately '' (empty): assertIdentitySpace treats an
-- empty target_user_field as "broadcast template：無 1:1 收件人語意，沒有身分欄位
-- 可驗" and returns before checking target_user_type's domain — the same escape
-- hatch 20260830100003_notification_announcement_identity_migrated_marker.sql
-- uses. enabled=false keeps this row out of TemplateCache.LoadFromDB (which
-- only loads enabled=true rows) and out of any "live templates" admin listing.
-- target_user_type='vendor' is not a real identity space (13 號 §12.1 only
-- defines seller/buyer); it is inert here precisely because target_user_field
-- is empty, and is chosen for readability, matching this feature's own "vendor"
-- naming (see the golden entry's ".vendor" suffix).
INSERT INTO "notification_templates" (
  "exchange", "routing_key",
  "event_type", "target_user_type", "target_user_field",
  "category", "title_template", "body_template", "body_fallback",
  "data_type", "data_fields", "schema_url", "reference_id_template",
  "enabled", "push_mode", "identity_migrated"
) VALUES
(
  'procurement.events', 'procurement.rfq_publication_invite.external_created',
  'procurement.rfq_publication_invite.external_created.vendor', 'vendor', '',
  'order_update',
  '(inert — sent by internal/worker/vendor_invite.go via pkg/mail, not this template)',
  '(inert — sent by internal/worker/vendor_invite.go via pkg/mail, not this template)',
  NULL,
  'PROCUREMENT_RFQ_PUBLICATION_INVITE_EXTERNAL',
  NULL,
  NULL, NULL,
  false, 'visible', true
)
ON CONFLICT ON CONSTRAINT uni_notification_templates_event_user_type DO NOTHING;
