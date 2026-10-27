-- issue #2806：PAYUni 補件通知 outbox 化 + 去重鍵重新設計（backend-payuni
-- docs/superpowers/specs/2026-08-28-payuni-verification-outbox-notifications-design.md §6）。
-- 五種商家認證結果的 reference_id_template 一律改成 per-transition 值（納入
-- event_id = merchant_verification_outbox row 自己的 TSID），修正同一 merchant
-- 同一 outcome 第二次發生時被 unique index 永久吞掉的落差（18 號設計書 §5.7.1
-- 早已記載此修法，此前只有 forwarded 套用過）。
UPDATE "notification_templates" SET
  "reference_id_template" = 'merchant_verification_#{merchant_id}_#{event_id}',
  "updated_at" = CURRENT_TIMESTAMP
WHERE "event_type" = 'merchant.verification.approved' AND "target_user_type" = 'seller';

UPDATE "notification_templates" SET
  "reference_id_template" = 'merchant_verification_#{merchant_id}_#{event_id}',
  "updated_at" = CURRENT_TIMESTAMP
WHERE "event_type" = 'merchant.verification.rejected' AND "target_user_type" = 'seller';

UPDATE "notification_templates" SET
  "reference_id_template" = 'merchant_verification_#{merchant_id}_#{event_id}',
  "updated_at" = CURRENT_TIMESTAMP
WHERE "event_type" = 'merchant.verification.pending_info' AND "target_user_type" = 'seller';

UPDATE "notification_templates" SET
  "reference_id_template" = 'merchant_verification_forwarded_#{merchant_id}_#{event_id}',
  "updated_at" = CURRENT_TIMESTAMP
WHERE "event_type" = 'merchant.verification.forwarded' AND "target_user_type" = 'seller';

-- 新事件：補件已收到、PayUni 重新審核中（seller-service payuni.go 的
-- outcomeUnderReview，此前完全不發通知）。
INSERT INTO "notification_templates" (
  "exchange", "routing_key",
  "event_type", "target_user_type", "target_user_field",
  "category", "title_template", "body_template", "body_fallback",
  "data_type", "data_fields", "schema_url", "reference_id_template",
  "enabled", "push_mode"
) VALUES (
  'hawkr.merchant', 'merchant.verification.under_review',
  'merchant.verification.under_review', 'seller', 'merchant_id',
  'system',
  '補件已收到',
  '您的實名認證補件已收到，PayUni 正重新審核中，請耐心等候。',
  NULL,
  'MERCHANT.VERIFICATION.UNDER_REVIEW',
  '{"refType": {"const": "MERCHANT_VERIFICATION"}, "refId": {"field": "merchant_id"}}'::jsonb,
  NULL, 'merchant_verification_under_review_#{merchant_id}_#{event_id}',
  true, 'visible'
)
ON CONFLICT ON CONSTRAINT uni_notification_templates_event_user_type DO NOTHING;
