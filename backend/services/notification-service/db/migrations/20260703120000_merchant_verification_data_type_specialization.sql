-- 2026-07-03：merchant.verification.* 四個 seller 模板的 data_type 一直共用通用值
-- 'VERIFICATION_UPDATE'，導致 seller app（router.dart / manager.dart）預留的逐一事件
-- case（CERTIFICATION_APPROVED 等、MERCHANT.VERIFICATION.*）從未被觸發，四種認證推播
-- （核准/駁回/需補件/轉送 PayUni）點擊後完全無反應。比照 Issue 3 ORDER_* 系列前例
-- （initial_templates.sql:348-356）將 data_type 精細化為逐一事件的具體值。
UPDATE "notification_templates" SET "data_type" = 'MERCHANT.VERIFICATION.APPROVED'
  WHERE "event_type" = 'merchant.verification.approved' AND "target_user_type" = 'seller';
UPDATE "notification_templates" SET "data_type" = 'MERCHANT.VERIFICATION.REJECTED'
  WHERE "event_type" = 'merchant.verification.rejected' AND "target_user_type" = 'seller';
UPDATE "notification_templates" SET "data_type" = 'MERCHANT.VERIFICATION.PENDING_INFO'
  WHERE "event_type" = 'merchant.verification.pending_info' AND "target_user_type" = 'seller';
UPDATE "notification_templates" SET "data_type" = 'MERCHANT.VERIFICATION.FORWARDED'
  WHERE "event_type" = 'merchant.verification.forwarded' AND "target_user_type" = 'seller';
