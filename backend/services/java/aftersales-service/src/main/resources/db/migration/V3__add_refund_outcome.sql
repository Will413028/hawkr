-- V3__add_refund_outcome.sql
-- 對應 09 設計書 §refund.completed event 契約修訂（issue #1023）
--
-- 背景：refund_tasks 缺 outcome 欄位 → aftersales.refund.completed event
-- caller 全傳 null → order-service 無法區分 FULL vs PARTIAL 退款。
--
-- 設計：refund_outcome NULL allowed（cancel_id path 不需要），
-- 由 CHECK constraint 保證 case_no path 必須有值。

-- 1. 新增欄位
ALTER TABLE refund_tasks
    ADD COLUMN IF NOT EXISTS refund_outcome VARCHAR(32);

-- 2. CHECK constraint：case_no path 必有 outcome；cancel_id path 可 NULL
--    NOT VALID：只對新 insert/update 生效，既有 row 不立即驗證；
--    既有 refund_tasks（case_no 但 refund_outcome IS NULL）由 dev SQL 一次性回填，
--    回填後可手動 `ALTER TABLE refund_tasks VALIDATE CONSTRAINT ...`。
ALTER TABLE refund_tasks
    ADD CONSTRAINT chk_refund_tasks_outcome_for_case
    CHECK ((case_no IS NULL) OR (refund_outcome IS NOT NULL)) NOT VALID;
