-- V15：EXTERNAL lane RFQ 成交建單的冪等鍵（64 號 §6.4.7）
--
-- 目的：一筆 buyer acceptance 至多一張訂單。建單命令由 ai-service 的 outbox worker
--      投遞，重試必然發生；沒有這把鍵，同一筆成交會建出第二張 PENDING_PAYMENT 訂單。
--
-- 🔴 為什麼不複用 checkout_id：它非空就會觸發 CheckoutService 的購物車鎖
--    （OrderService.createOrder 的第一段），RFQ 沒有購物車，會去鎖一個不存在的批次。
--    而 gRPC 那條建單入口本來就沒有任何冪等——findByCheckoutId 只存在於
--    createOrderFromCheckout。
--
-- expand-only：欄位可為 NULL，既有列與 rolling 期間的舊 pod 完全不受影響
--（舊 pod 的 INSERT 不帶這一欄 ⇒ NULL ⇒ 偏索引不收，不會互相干擾）。
--
-- ⚠️ 本欄只對 EXTERNAL lane 的 RFQ 訂單有值。PLATFORM lane 的 RFQ 成交走現行
--    createOrder（有真 SKU 與真庫存，賣方就是那個平台賣家），不經過這條路徑。

ALTER TABLE orders ADD COLUMN IF NOT EXISTS acceptance_id BIGINT NULL;

COMMENT ON COLUMN orders.acceptance_id IS
    'EXTERNAL lane RFQ 成交的 buyer acceptance id（ai-service 的 procurement_buyer_quote_acceptances.id）。非 RFQ 訂單為 NULL。64 號 §6.4.7';

-- 偏索引而非整表 UNIQUE：既有與未來的非 RFQ 訂單全部是 NULL，Postgres 的 UNIQUE
-- 雖然允許多個 NULL，但偏索引讓意圖寫在 schema 上——這把鍵只約束 RFQ 訂單。
CREATE UNIQUE INDEX IF NOT EXISTS uk_orders_acceptance_id
    ON orders (acceptance_id)
    WHERE acceptance_id IS NOT NULL;
