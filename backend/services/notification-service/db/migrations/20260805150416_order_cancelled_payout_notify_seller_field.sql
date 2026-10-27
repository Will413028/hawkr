-- order.cancelled.merchant / payout.completed 的 target_user_field 由 seller_account_id
-- 改讀 notify_seller_account_id，對齊 order.completed.merchant 於 2026-07-18
-- （20260718011500_plan_c_phase2_order_completed_notify_field.sql）已完成的命名。
-- order-service 於同一批改動起「雙寫」兩個 key（舊 key 於後續 PR 移除）。
--
-- ⚠️ 部署順序不是無關緊要：本 migration 必須等 order-service 的雙寫（Task 4，同一個 PR）
-- 已 rollout 到「全部」pod 之後才可套用，不能反過來。若本 migration 先套用，rolling
-- update 期間尚未更新的舊 pod 仍只會 emit 舊 key seller_account_id，但 template 已經
-- 改讀新 key，notification-service/internal/worker/template_handler.go 的 extractUserID
-- 會找不到該欄位、記一筆 Warn log 後直接 return——通知被靜默丟棄，無 DLQ、無告警、無重試。
-- payout.completed 受影響更大：撥款 task 已標 DONE、通知例外被 GrantPaymentWorker 內部
-- catch 吞掉不會重拋，不會有任何重試機制補發，賣家可能整筆撥款通知都收不到。

UPDATE "notification_templates"
SET target_user_field = 'notify_seller_account_id'
WHERE event_type IN ('order.cancelled.merchant', 'payout.completed')
  AND target_user_type = 'seller';
