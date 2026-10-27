-- =====================================================
-- V6: shipment_anomaly_acknowledgments 擴 actor 維度（issue #1327）
-- 目的：原表 seller_id NOT NULL + UNIQUE(shipment_id, anomaly_type) 只容得下賣家單一 actor；
--       admin 監控（§29）需對同一 (shipment, anomaly_type) 獨立標記，賣家 ack 與 admin ack 互不覆蓋。
-- 手法：pre-launch 單檔破壞性 rename（同 V4__rename_shipping_seller_account_id_to_store_id 先例；
--       資料量極小、無真實流量，rolling 窗口風險可接受）。新增 actor_type 維度並將
--       seller_id 誠實改名為 actor_id（SELLER 列值原樣保留）。
-- §22 邊界：仍為純使用者操作記錄表，不引入補償/狀態機；admin 標記不改寫 shipment/order 業務狀態。
-- 設計書：06 §5.21.4
-- =====================================================

-- 1. expand：新增 actor_type，既有列 backfill 'SELLER'
ALTER TABLE shipment_anomaly_acknowledgments ADD COLUMN actor_type VARCHAR(16);
UPDATE shipment_anomaly_acknowledgments SET actor_type = 'SELLER' WHERE actor_type IS NULL;
ALTER TABLE shipment_anomaly_acknowledgments ALTER COLUMN actor_type SET NOT NULL;
ALTER TABLE shipment_anomaly_acknowledgments
    ADD CONSTRAINT chk_anomaly_actor_type CHECK (actor_type IN ('SELLER', 'ADMIN'));

-- 2. rename：seller_id → actor_id（SELLER 列的值原樣保留）
ALTER TABLE shipment_anomaly_acknowledgments RENAME COLUMN seller_id TO actor_id;

-- 3. 換 UNIQUE、移除死索引
--    所有讀路徑（upsert selectOne / dashboard JOIN）皆走 UNIQUE (shipment_id, anomaly_type, actor_type)；
--    無 actor 清單查詢路徑，故只 DROP 舊索引、不另建 actor 索引（避免純寫放大）。
ALTER TABLE shipment_anomaly_acknowledgments DROP CONSTRAINT uk_shipment_anomaly_ack;
ALTER TABLE shipment_anomaly_acknowledgments
    ADD CONSTRAINT uk_shipment_anomaly_ack UNIQUE (shipment_id, anomaly_type, actor_type);
DROP INDEX IF EXISTS idx_anomaly_ack_seller_acted;

-- 4. 註解刷新（誠實命名：欄位承載 SELLER 與 ADMIN 兩類 actor）
COMMENT ON COLUMN shipment_anomaly_acknowledgments.actor_type IS
    'actor 類別：SELLER（賣家 store surrogate）/ ADMIN（admin account_id, x-account-id）';
COMMENT ON COLUMN shipment_anomaly_acknowledgments.actor_id IS
    'actor 識別：SELLER=賣家 store surrogate（原 seller_id 語意）；ADMIN=admin account_id';
COMMENT ON TABLE shipment_anomaly_acknowledgments IS
    '§5.21 配送異常處置記錄。UNIQUE(shipment_id, anomaly_type, actor_type)：同 (shipment, anomaly_type) 每個 actor_type 各一筆，後續變更 action 走 UPDATE。';
