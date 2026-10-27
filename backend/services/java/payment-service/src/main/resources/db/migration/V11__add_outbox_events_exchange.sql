-- V11：outbox_events 多 exchange 化（explicit persisted exchange）。
--
-- contract SSOT ＝ 36 號《MQ 與事件總覽》§3.3.1「Outbox persisted exchange」；
-- payment-service 端的逐項實作 impact 清單見 65 號 §6A.1.2 同名段落。兩份不得各自增列契約語意。
--
-- 模型：每一列自己帶著「要投遞到哪個 exchange」。刻意**不**做 routing-key prefix 猜 exchange 的
-- hidden mapping——那會讓「這則事件會去哪」變成一條藏在 code 裡的規則，新增一條 routing key 的人
-- 不會知道自己要同時去改那張對照表。
--   既有 payment 事件（payment.success / payment.failed / payment.conflict / payment.expired /
--   payment.rescue.*）⇒ 'payment.events'
--   未來 fulfillment responsibility 事件 ⇒ 'hawkr.shipping'（本 migration 不啟用其寫入路徑）
--
-- 🔴 為什麼需要 DEFAULT（expand-contract 的 expand 階段）：
-- 本檔在 Spring Boot 啟動當下套用，而 surge 滾動更新（maxSurge:1, maxUnavailable:0）視窗內
-- 仍有舊 pod 在跑。舊版 code 的 OutboxEventEntity 沒有 exchange 欄位，其 INSERT 不會帶這一欄
-- ——沒有 DEFAULT 的 NOT NULL 會讓舊 pod 的每一筆 outbox 寫入被 DB 拒絕，而 outbox 寫入與業務
-- 狀態在同一個交易內 ⇒ 付款事件寫不進去 = 整筆業務交易 rollback = 對外 500，直到該 pod 被汰換。
-- 有了 DEFAULT，舊 pod 的寫入自動落 'payment.events'，而那正是舊 code 唯一會發的 exchange
-- （舊 publisher 硬寫 MqConstants.EXCHANGE_PAYMENT）⇒ DEFAULT 值語意正確，不是將就。
--
-- ⚠️ 對照 coupon V6 的教訓（該檔檔頭）：那次之所以不能把約束放進 expand 階段，是因為
-- 「舊 pod 的寫入會違反新約束」（它完全不知道新欄位存在，寫進去就是 NULL）。本檔不成立該條件
-- ——DEFAULT 讓舊 pod 的寫入恆滿足 NOT NULL 與 CHECK ⇒ 兩者可以同批進。
--
-- 🔴 DEFAULT ≠ publisher 的 fallback（36 §3.3.1 明令 fail-closed，不得 fallback）：
-- DB DEFAULT 只作用於「INSERT 時未提及該欄」的情況，攔不到「明確寫入 null／空字串」；
-- 而 publisher 端對 null／空白 exchange 一律不投遞並標 FAILED。兩者是不同層、不可互相取代：
-- 沒有 DEFAULT 滾動更新會掛，沒有 publisher 守衛則 corrupt／legacy／手寫 SQL 造出的壞列會被靜默發錯。
--
-- 既有列回填：PostgreSQL 11+ 的 ADD COLUMN ... NOT NULL DEFAULT 為 metadata-only（不重寫表），
-- 既有列即時取得 DEFAULT 值 ⇒ 不需要獨立的回填 UPDATE。dev 實查 155 列、prod 實查 0 列。

ALTER TABLE outbox_events
    ADD COLUMN exchange VARCHAR(64) NOT NULL DEFAULT 'payment.events';

-- DB 層不變式：NOT NULL 擋 null，本 CHECK 擋空字串與**任何** whitespace-only 值。
-- （命名沿用同表既有的 chk_payment_outbox_status 前綴慣例。）
-- ⚠️ 刻意不用 btrim(exchange) <> ''：btrim 預設只剝空格，tab／換行組成的值會通過
--    （本輪實測 E'\t\n' 確實穿過了 btrim 版本）。`~ '\S'` 要求「至少一個非空白字元」，
--    涵蓋 space／tab／CR／LF／FF／VT 全部。
ALTER TABLE outbox_events
    ADD CONSTRAINT chk_payment_outbox_exchange CHECK (exchange ~ '\S');

COMMENT ON COLUMN outbox_events.exchange IS
    'RabbitMQ exchange to publish this row to (explicit persisted exchange; publisher reads this column and never infers it from routing_key). payment events = payment.events; fulfillment responsibility events = hawkr.shipping.';
