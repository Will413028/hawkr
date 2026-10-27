-- non-catalog line 的 aftersales 半邊（Wave A / expand）
--
-- 售後建案會把 order_items 的明細原樣複製進 aftersale_case_items，而 RFQ 成交的
-- 訂單明細沒有平台 SKU（order-service V17 已放寬 order_items.sku_id）⇒ 這一欄
-- 同步放寬，否則 RFQ 訂單開不了售後單。
--
-- 🔴 表名是 `aftersale_case_items`（無中間底線）。doc 64 §6.4.9 影響面表與 :476 的
--    SQL 逐字寫的是 `after_sale_case_items`，**那是錯的**——照抄會 relation does not
--    exist。三方查證：V1__init_aftersales_tables.sql:126 的 CREATE TABLE、
--    AfterSaleCaseItemEntity 的 @TableName、dev DB 的 information_schema 都是無底線版。
--
-- ⚠️ sku_name 維持 NOT NULL —— 與 order_items 同一條不變式：品名必須真實。
-- 實查（用正確表名）本欄無 UNIQUE／INDEX／FK，後續 migration 對本表零改動
--    ⇒ 放寬不牽動任何其他約束
--    （與 order_items 需要論證 uk_order_items_order_sku 的情況不同）。
--
-- ⛔ 本項是 Wave B 的部署前置：Wave B 的 worker 一上線就開始產生 sku_id 為 NULL 的
--    RFQ 訂單，那之後本欄若仍是 NOT NULL，買家對 RFQ 訂單開售後單會直接失敗。

ALTER TABLE aftersale_case_items ALTER COLUMN sku_id DROP NOT NULL;
