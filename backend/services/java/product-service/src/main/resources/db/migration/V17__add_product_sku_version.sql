-- SKU 層樂觀鎖版本欄（#2545）。
--
-- 為什麼 product 有 version 還要再加一個：product.version 的反正規化寫入路徑
-- （reaggregateStockFromSku 等四條，56 §5.4 F41 第 11 列）**刻意不 bump**，
-- 否則代行編輯在高流量商品上幾乎必然 409。代價是 product.version 對「表單開著
-- 期間發生的扣庫存」完全失明，而賣家／代行的 PUT 會把 client 送來的 stockQty
-- 逐 SKU 原樣寫回 ⇒ 只改個商品名稱存檔就把中間賣掉的庫存復活，且沒有任何訊號。
--
-- SKU 自己的 version 讓兩個目標同時成立：product.version 仍然不動（不製造 409
-- 風暴），而 SKU 層攔得到 lost update。
--
-- 回填判定（DB Migration 相容性規範（expand-contract）§三 逐條走過，結論同 V16）：
-- 該教訓針對「新欄位可由既有欄位推導」——那時 ADD COLUMN ... DEFAULT 'x' 會把每
-- 一列填成同一個錯值。version 不由任何既有欄位推導，樂觀鎖從 0 起算，
-- NOT NULL DEFAULT 0 對既有列填 0 **即為正確初值**（PostgreSQL 的 ADD COLUMN
-- 會把 DEFAULT 套用到全部既有列，不會留下 NULL），因此不需要追加 UPDATE。
ALTER TABLE product_sku
    ADD COLUMN version BIGINT NOT NULL DEFAULT 0;

-- 恆真的不變式交給 DB 守（expand-contract 規範 §三）。
ALTER TABLE product_sku
    ADD CONSTRAINT chk_product_sku_version_nonneg CHECK (version >= 0);

COMMENT ON COLUMN product_sku.version IS
    'SKU 樂觀鎖版本（設計書 56 §5.4 / #2545）。每次成功寫入 +1（含扣庫存／歸還庫存）；'
    '賣家與代行寫回 SKU 時以每筆 SKU 的 expectedVersion 比對，不符即 409。';
