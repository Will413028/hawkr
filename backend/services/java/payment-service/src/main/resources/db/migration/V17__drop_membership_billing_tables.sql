-- 會員計費沒有消費者也不再做；V5～V8 建立的四張表隨 payment-service 的會員計費程式一併移除。
-- 子表先於被參照的表（membership_charge_attempts → membership_provider_subscriptions，
-- billing_commands.result_subscription_id → membership_provider_subscriptions，
-- membership_provider_subscriptions.provider_account_ref → billing_provider_accounts）。
DROP TABLE IF EXISTS membership_charge_attempts;
DROP TABLE IF EXISTS billing_commands;
DROP TABLE IF EXISTS membership_provider_subscriptions;
DROP TABLE IF EXISTS billing_provider_accounts;
