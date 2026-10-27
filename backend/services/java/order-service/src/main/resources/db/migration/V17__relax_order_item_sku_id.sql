-- non-catalog line（Wave A / expand）：RFQ 成交的訂單明細沒有平台 SKU
--
-- ⚠️ sku_name **維持 NOT NULL** —— 品名必須真實，這是 non-catalog line 的核心不變式
--    （doc 64 §6.4.9「⛔ sku_name 禁止 sentinel」；那條禁令只約束品名，
--    不約束 sku_id 指向誰，兩者是不同的事）。
--
-- uk_order_items_order_sku (order_no, sku_id) 與 idx_order_items_sku_id **都不動**：
-- Postgres 視 NULL 相異，而 CreateRfqOrderRequest 的 proto 註解自陳 order_items
-- 恆為一列且 quantity = 1 ⇒ RFQ 訂單不可能自撞，catalog 訂單的約束原樣成立。
--
-- 🔴 本波只讓讀取端容忍，**不產生任何 NULL**（expand-contract 的 expand）。寫入端照舊；
--    NULL 要到 Wave B 的 orderprovisioning worker 上線才會出現。
-- ⛔ 因此 Wave B 不得在 aftersales 的兩項放寬（after_sale_case_items.sku_id DROP NOT NULL
--    ＋ AfterSaleCaseService 的 optional parse）上線前部署，否則 RFQ 訂單開不了售後單。

ALTER TABLE order_items ALTER COLUMN sku_id DROP NOT NULL;
