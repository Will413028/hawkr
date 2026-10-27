-- =====================================================
-- V3: 建立 shipment_anomaly_acknowledgments 表
-- 用途：§5.21 配送異常賣家通知 — 賣家對單一 (shipment, anomaly_type) 的處置動作
-- 性質：純使用者操作記錄表，不存 anomaly 狀態本身（異常存在性由 shipments 既有欄位推導）
-- §22 邊界：本表記錄使用者互動，與資料一致性無關；shipment.status 恢復後 dashboard
--          JOIN miss 自然不顯示，無需 reconciliation job 清表
-- =====================================================

CREATE TABLE IF NOT EXISTS shipment_anomaly_acknowledgments (
    id              BIGSERIAL PRIMARY KEY,
    shipment_id     BIGINT NOT NULL REFERENCES shipments(id),
    order_no        VARCHAR(64) NOT NULL,
    anomaly_type    VARCHAR(40) NOT NULL,   -- ShipmentAnomalyType.value
    seller_id       BIGINT NOT NULL,
    action          VARCHAR(16) NOT NULL,   -- ShipmentAnomalyAckAction.value: ACKNOWLEDGED / IGNORED
    acted_at        TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uk_shipment_anomaly_ack UNIQUE (shipment_id, anomaly_type),
    CONSTRAINT chk_anomaly_action CHECK (action IN ('ACKNOWLEDGED', 'IGNORED'))
);

CREATE INDEX IF NOT EXISTS idx_anomaly_ack_seller_acted
    ON shipment_anomaly_acknowledgments (seller_id, acted_at DESC);

CREATE INDEX IF NOT EXISTS idx_anomaly_ack_order_no
    ON shipment_anomaly_acknowledgments (order_no);

-- updated_at 自動更新
CREATE TRIGGER trg_shipment_anomaly_ack_updated_at
    BEFORE UPDATE ON shipment_anomaly_acknowledgments
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE shipment_anomaly_acknowledgments IS
    '§5.21 配送異常賣家處置記錄。UNIQUE(shipment_id, anomaly_type)：同一 anomaly_type 一筆，後續變更 action 走 UPDATE。';
COMMENT ON COLUMN shipment_anomaly_acknowledgments.anomaly_type IS
    'ShipmentAnomalyType.value（如 TCAT_CALL_FAILED）。寫入前由 service 層用 CodeMetaValidator.validate 驗證。';
COMMENT ON COLUMN shipment_anomaly_acknowledgments.action IS
    'ShipmentAnomalyAckAction.value: ACKNOWLEDGED（已處理）/ IGNORED（已忽略）';
