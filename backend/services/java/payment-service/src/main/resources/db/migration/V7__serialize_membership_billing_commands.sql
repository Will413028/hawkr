-- V7：provider side effect 期間的 subscription-scoped command lease。
-- PROCESSING／UNKNOWN 都代表仍可能有 provider side effect 或待 reconciliation；
-- 同一 subscription 在這段期間不得再建立另一個 lifecycle command。
CREATE UNIQUE INDEX uq_billing_commands_active_subscription
    ON billing_commands(result_subscription_id)
    WHERE result_subscription_id IS NOT NULL
      AND status IN ('PROCESSING', 'UNKNOWN');

COMMENT ON INDEX uq_billing_commands_active_subscription IS
    'Persisted lease: at most one in-flight or unknown provider lifecycle command per subscription';
