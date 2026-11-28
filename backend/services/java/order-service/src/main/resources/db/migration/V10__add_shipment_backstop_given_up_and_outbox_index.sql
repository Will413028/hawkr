-- issue #2294：多副本排程搶佔（Scheduler Claim）—— order 側
--
-- 設計書：03 §8.10 / §8.11；規範：docs/開發規範/多副本排程搶佔規範.md
--
-- 本檔兩件事，都與 order-service 的排程正確性有關：
--   1. H4：ShipmentCreationBackstopScheduler 的 give-up 分支新增**觀測欄**
--   2. H1-DEAD：order_outbox 的派送部分索引改成正面枚舉（與新的候選述詞對齊）

-- ==================== 1. H4：give-up 觀測欄 ====================
-- ⚠️ 這一欄**不是**「已放棄旗標」，也**不進候選述詞**。
--    give-up 分支的搶佔走的是既有的 shipment_create_next_retry_at 租約
--    （claimShipmentBackstopGiveUp，租約 = hawkr.order.shipment-backstop-backoff-minutes，
--     預設 30 分，與重試分支同值）。本欄只記「第一次放棄的時間」（COALESCE 只寫一次），
--    用途是給 sql_exporter 一條告警：
--        SELECT count(*) FROM orders
--         WHERE shipment_backstop_given_up_at IS NOT NULL AND status = '10';
--    （status '10' = OrderStatus.PAID；已放棄卻仍停在 PAID = 取消退款沒生效，需要人看）
--
-- ⚠️ 10 檔 §11.2 教訓的適用判定：那條講的是「能由既有欄位推導的欄位必須同批回填」。
--    本欄無法也不該從既有欄位推導（既有資料裡沒有「第一次放棄的時間」這個事實），
--    NULL 的語意就是「從未放棄」——正是新欄位的預期初始狀態，故**刻意不做 UPDATE 回填**。
ALTER TABLE orders ADD COLUMN IF NOT EXISTS shipment_backstop_given_up_at TIMESTAMPTZ;

COMMENT ON COLUMN orders.shipment_backstop_given_up_at IS
    'issue #2294 H4：建單兜底「第一次放棄（送取消+退款）」的時間戳，純觀測欄（不進候選述詞）。搶佔用的是 shipment_create_next_retry_at 租約。';

-- 候選 SQL（OrderRepository.findPaidOrdersWithoutShipmentPastGrace）一個字都不用改，
-- 故 idx_orders_shipment_backstop（V3:36-38）完全不受影響，不需要擴充。

-- ==================== 2. H1-DEAD：order_outbox 派送索引改正面枚舉 ====================
-- 舊述詞 status <> '8222'（SENT）對 '8224'(DEAD) 恆真 ⇒ DEAD 列永久滯留候選集合、
-- 每 5 秒被撈回一次，累積到批次上限（50）即整條 outbox starvation（單 pod 也壞）。
-- Java 側候選 SQL 已改為正面枚舉 status IN ('8221','8223')；索引跟著改，
-- 讓部分索引不再收納 SENT/DEAD 兩種終態。
--
-- 值為 OutboxEventStatus 的 CodeMeta value：
--   '8221'=PENDING / '8222'=SENT / '8223'=FAILED / '8224'=DEAD
-- （SSOT：docs/代碼管理（CodeMeta）/內部技術碼.md；SQL 只能寫 value，不能寫 enum 名）
DROP INDEX IF EXISTS idx_order_outbox_dispatch;

CREATE INDEX IF NOT EXISTS idx_order_outbox_dispatch
    ON order_outbox(status, next_retry_at)
    WHERE status IN ('8221', '8223');
