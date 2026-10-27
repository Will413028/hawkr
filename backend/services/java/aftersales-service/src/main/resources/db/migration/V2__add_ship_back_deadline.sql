-- V2__add_ship_back_deadline.sql
-- 對應 09 設計書 §3.9.1 / §5.3.8（2026-06-15 待寄回逾時政策修訂）
-- 政策變更：30 天起算 created_at → 7 天起算 ship_back_deadline_at（進入 status=20 時 stamp）；
-- 逾時動作從「只寫 audit log」→ 自動關案（status 20→33），退款資格解除。

-- 1. 新增欄位
ALTER TABLE aftersale_cases
    ADD COLUMN IF NOT EXISTS ship_back_deadline_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS warned_ship_back_24h_at TIMESTAMPTZ;

-- 2. 既有 status='20' 案件 backfill：上線日 + 7 天寬限，避免溯及既往導致 V2 跑完 cron 立即關案
UPDATE aftersale_cases
   SET ship_back_deadline_at = NOW() + INTERVAL '7 days'
 WHERE status = '20' AND ship_back_deadline_at IS NULL;

-- 3. 待寄回逾時自動關案掃描 partial index（取代既有 idx_aftersale_cases_return_timeout 用 created_at 起算）
DROP INDEX IF EXISTS idx_aftersale_cases_return_timeout;
CREATE INDEX idx_aftersale_cases_return_timeout ON aftersale_cases (ship_back_deadline_at)
    WHERE status = '20' AND ship_back_deadline_at IS NOT NULL;

-- 4. 待寄回 24h 預警掃描 partial index
CREATE INDEX IF NOT EXISTS idx_aftersale_cases_warn_ship_back_24h ON aftersale_cases (ship_back_deadline_at)
    WHERE status = '20' AND warned_ship_back_24h_at IS NULL;
