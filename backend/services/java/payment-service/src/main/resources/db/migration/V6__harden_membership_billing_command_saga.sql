-- V6：補強 billing command saga 的 response snapshot、unknown outcome 與重試資料。
-- V5 已可部署環境不可改寫；所有新欄位以 additive migration 方式加入。

ALTER TABLE billing_commands
    ADD COLUMN error_message TEXT,
    ADD COLUMN result_provider_subscription_id VARCHAR(128),
    ADD COLUMN result_redirect_url TEXT,
    ADD COLUMN result_mer_trade_no VARCHAR(128),
    ADD COLUMN result_status VARCHAR(32),
    ADD COLUMN action_reason TEXT;

ALTER TABLE billing_commands
    DROP CONSTRAINT chk_billing_commands_status,
    ADD CONSTRAINT chk_billing_commands_status
        CHECK (status IN ('PROCESSING', 'SUCCEEDED', 'FAILED', 'UNKNOWN'));

COMMENT ON COLUMN billing_commands.error_message IS
    'Immutable caller-safe error message snapshot for idempotent replay';
COMMENT ON COLUMN billing_commands.result_status IS
    'Immutable provider/lifecycle status snapshot returned by the original command';
COMMENT ON COLUMN billing_commands.action_reason IS
    'Original lifecycle action reason, retained so an UNKNOWN command can be reconciled safely';
