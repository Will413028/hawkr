-- issue #2294：多副本排程搶佔（Scheduler Claim）—— shipping 側新增兩個「租約欄」
--
-- 設計書：06 §5.19.3.B / §5.19.3.C / §8.5；規範：docs/開發規範/多副本排程搶佔規範.md
--
-- 兩欄都是 **租約欄（lease）**，不是「已做過旗標」：
--   值 = 目前持有者的租約到期時間；NULL 或已過期 = 可被重新搶佔。
--   NULL 的正確語意就是「從未被搶佔、立即可搶」——這正是新欄位的預期初始狀態，
--   **不需要也不可以回填**（回填成 NOW() 會讓既有列全部沉默一個租約週期）。
--   ⚠️ 對照 10 檔 §11.2 的教訓：那條講的是「能由既有欄位推導的欄位必須同批回填」；
--      租約欄無法也不該從既有欄位推導，故此處刻意不做 UPDATE。
--
-- 為什麼不挪用既有欄位（兩支各有一個看起來很像的欄位，都不能用）：
--   shipments.label_print_requested_at → 業務時戳（「何時送印」）。推它等於竄改業務事實，
--                                        而且它同時是 requestedBefore grace 與 maxAgeCutoff
--                                        兩個判準的基準，推它會把該列的候選壽命無限延長。
--   shipments.tcat_call_requested_at   → 同上（黑貓叫件時間）。
--   （L1 用既有的 processing_deadline、L2 用既有的 last_reconciled_at，兩者本身就是排程判準欄，
--     故不在本 migration 內——它們零 schema 變更。）

-- ==================== L3：ShipmentLabelReconcileScheduler（列印狀態機補收，每 10 分） ====================
-- 租約長度 = 10 分鐘（＝一個 tick）；候選窗口 = label.max-age-days（預設 7 天）⇒ 窗口內約 1000 次機會
ALTER TABLE shipments
    ADD COLUMN label_reconcile_next_at TIMESTAMPTZ;

COMMENT ON COLUMN shipments.label_reconcile_next_at IS
    'issue #2294 L3：ShipmentLabelReconcileScheduler 的租約到期時間（非「已補收旗標」）。NULL 或 <= NOW() 表示可搶佔。';

-- 候選述詞：label_print_status IN ('4102','4104') AND ship_trade_no IS NOT NULL
--           AND label_print_requested_at < #{requestedBefore} AND >= #{maxAgeCutoff}
--           AND (label_reconcile_next_at IS NULL OR label_reconcile_next_at <= NOW())
CREATE INDEX idx_shipments_label_reconcile_lease
    ON shipments (label_print_requested_at, label_reconcile_next_at)
    WHERE label_print_status IN ('4102', '4104');

-- ==================== L4：TcatCallStuckAlertScheduler（黑貓叫件卡死告警，每 10 分） ====================
-- 🚨 租約長度 = 1 小時，**不是 24 小時**：候選窗口寬度 = alert.tcat-stuck.max-age-hours（預設 24h），
--    租約與窗口等寬 ⇒ 首次告警蓋章後，租約到期時該列早已掉出窗口 ⇒ 永遠只告警一次 ⇒
--    退化成 at-most-once，而 publishAnomalyDetected 在迴圈內無 try/catch，一拋例外就完全無聲。
--    1 小時 = tick（10 分）× 6（足以止噪）且 ≪ 24h 窗口（卡死期間會響約 23 次，符合「持續有聲音」）。
ALTER TABLE shipments
    ADD COLUMN tcat_stuck_alerted_at TIMESTAMPTZ;

COMMENT ON COLUMN shipments.tcat_stuck_alerted_at IS
    'issue #2294 L4：黑貓叫件卡死告警的冷卻租約到期時間（非「已告警旗標」——卡死沒解除就該一直吵，租約到期會再發）。NULL 或 <= NOW() 表示可搶佔。';

-- 候選述詞：tcat_call_status = '4202' AND tcat_call_requested_at < #{requestedBefore} AND >= #{maxAgeCutoff}
--           AND (tcat_stuck_alerted_at IS NULL OR tcat_stuck_alerted_at <= NOW())
CREATE INDEX idx_shipments_tcat_stuck_lease
    ON shipments (tcat_call_requested_at, tcat_stuck_alerted_at)
    WHERE tcat_call_status = '4202';
