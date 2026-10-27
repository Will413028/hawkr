-- =====================================================
-- V2: shipments 加 expected_delivery_date
-- 用途：§5.21 DELIVERY_OVERDUE 配送逾期偵測使用
-- 寫入點：
--   宅配：CallTcat 成功時，由 processing_deadline + 3 工作日換算
--   超商：Webhook ShipStatus=22 到店時 + 7 天
-- 歷史 row：保持 NULL，DELIVERY_OVERDUE 偵測不會誤觸發
-- 非破壞性：純 ADD COLUMN
-- =====================================================

ALTER TABLE shipments
    ADD COLUMN IF NOT EXISTS expected_delivery_date DATE;

-- 部分索引：僅含有期限且非終態的正向物流單，§5.21 DELIVERY_OVERDUE 掃描用
CREATE INDEX IF NOT EXISTS idx_shipments_expected_delivery
    ON shipments(expected_delivery_date)
    WHERE expected_delivery_date IS NOT NULL
      AND is_return = FALSE
      AND status NOT IN ('11', '41');

COMMENT ON COLUMN shipments.expected_delivery_date IS
    '預計送達日期（DATE，UTC 之 date 部分）。§5.21 DELIVERY_OVERDUE 偵測來源；NULL 表無法估算，不參與逾期偵測。';
