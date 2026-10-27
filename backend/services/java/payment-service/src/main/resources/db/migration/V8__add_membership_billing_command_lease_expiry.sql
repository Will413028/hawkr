-- V8：PROCESSING provider call 的 lease expiry 與 crash recovery。
ALTER TABLE billing_commands
    ADD COLUMN lease_expires_at TIMESTAMPTZ;

-- V5–V7 可能已存在未完成 command；依最後一次 DB touch 建立第一個可驗證的 lease。
UPDATE billing_commands
SET lease_expires_at = COALESCE(updated_at, created_at, NOW()) + INTERVAL '5 minutes'
WHERE status = 'PROCESSING' AND lease_expires_at IS NULL;

COMMENT ON COLUMN billing_commands.lease_expires_at IS
    'Provider-call lease expiry; stale PROCESSING may be atomically reclaimed with the same command UUID/idempotency key';
