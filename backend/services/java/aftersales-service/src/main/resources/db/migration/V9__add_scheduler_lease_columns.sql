-- issue #2294：多副本排程搶佔（Scheduler Claim）—— aftersales 側新增三個「租約欄」
--   L5 refund_tasks.resync_at
--   L6 aftersale_cases.return_stuck_alerted_at
--   L7 aftersale_cases.label_retry_at
--
-- 設計書：09 §8.5「多副本排程搶佔語意」；規範：docs/開發規範/多副本排程搶佔規範.md
--
-- 三欄都是 **租約欄（lease）**，不是「已做過旗標」：
--   值 = 目前持有者的租約到期時間；NULL 或已過期 = 可被重新搶佔。
--   因此 NULL 的正確語意就是「從未被搶佔、立即可搶」——這正是新欄位的預期初始狀態，
--   **不需要也不可以回填**（回填成 NOW() 反而會讓既有列全部沉默一個租約週期）。
--   ⚠️ 對照 10 檔 §11.2 的教訓：那條講的是「能由既有欄位推導的欄位必須同批回填」；
--      租約欄無法也不該從既有欄位推導，故此處刻意不做 UPDATE。
--
-- 為什麼不挪用既有欄位（兩支各有一個看起來很像的欄位，都不能用）：
--   refund_tasks.updated_at        → 由 MetaObjectHandler 統一維護，任何寫入都會動它；
--                                     且它本身就是 RefundSyncWorker 的候選判定欄。
--   aftersale_cases.updated_at     → 同上，是 ReturnStuckAlertScheduler 的候選判定欄。

-- ==================== L5：RefundSyncWorker（補發遺漏 MQ 通知，每小時） ====================
-- 租約長度 = 3 小時（排程週期 1h × 3）
ALTER TABLE refund_tasks
    ADD COLUMN resync_at TIMESTAMPTZ;

COMMENT ON COLUMN refund_tasks.resync_at IS
    'issue #2294 L5：RefundSyncWorker 補發事件的租約到期時間（非「已補發旗標」）。NULL 或 <= NOW() 表示可搶佔。';

-- 候選述詞：status = '20'(DONE) AND updated_at < NOW()-1h AND (resync_at IS NULL OR resync_at <= NOW())
CREATE INDEX idx_refund_tasks_resync ON refund_tasks (updated_at, resync_at)
    WHERE status = '20';

-- ==================== L6：ReturnStuckAlertScheduler（退貨卡死告警，每日） ====================
-- 租約長度 = hawkr.shipping.anomaly.return-stuck-days（預設 14 天，與告警冷卻同值）
ALTER TABLE aftersale_cases
    ADD COLUMN return_stuck_alerted_at TIMESTAMPTZ;

COMMENT ON COLUMN aftersale_cases.return_stuck_alerted_at IS
    'issue #2294 L6：RETURN_STUCK 卡死告警的租約到期時間（非「已告警旗標」——卡死沒解除就該一直吵，租約到期會再發）。NULL 或 <= NOW() 表示可搶佔。';

-- 候選述詞：status = '25'(RETURN_SHIPPING) AND updated_at < NOW()-Nd
--           AND (return_stuck_alerted_at IS NULL OR return_stuck_alerted_at <= NOW())
CREATE INDEX idx_aftersale_cases_return_stuck ON aftersale_cases (updated_at, return_stuck_alerted_at)
    WHERE status = '25';

-- ==================== L7：ReturnLabelRetryWorker（7-11 退貨代碼自動重試，每 2 分） ====================
-- 租約長度 = 6 分鐘（排程週期 2 分 × 3）
--
-- 修復前的搶佔閘門是 label_retry_count 的 0→1 CAS（AfterSaleCaseMapper.markLabelRetried），
-- 候選述詞是 label_retry_count = 0 ⇒ **CAS 成功即永久離開候選集合、無時間欄可讓它回來**
-- ＝《多副本排程搶佔規範》§二 第 3 步定義的永久旗標。而副作用（shipping
-- RetryReturnPrintLabel gRPC）在其後，且例外被 catch 吞掉 ⇒ 不必等 pod 崩潰，
-- **一次 gRPC 失敗就永久用掉這個案件唯一一次自動重試**。
--
-- 為什麼要新欄位而不是挪用既有欄位：
--   label_retry_count → 它是**業務預算**（「自動只重試一次」），不是租約；
--                        把它當閘門正是上面那個缺陷。修復後它降為「已確認觸發過幾次」，
--                        只在 gRPC 確認成功之後才寫。
--   created_at        → 候選述詞用它算逾時（created_at < NOW()-5min），不可寫。
--   updated_at        → MetaObjectHandler 統一維護，任何寫入都會動它（同 L5/L6 的理由）。
--
-- 🚨 本欄是 **三態**，不是「有/沒有租約」兩態。三態互斥且完整，兩個消費述詞各取其一：
--   NULL     = 從未嘗試過（新案件，或 ops resetLabelRetryCount 重置）→ 可搶佔、**不算耗盡**
--   過去時間 = 嘗試過但目前無人持有                                   → 可搶佔、**算耗盡**
--   未來時間 = 正在嘗試中（租約有效）                                 → 不可搶佔
-- 因此 shipping 觸發失敗時的釋放（releaseLabelRetryLease）寫的是**一個過去的時間**，
-- 而不是「還原成 claim 之前的舊值」——對從未成功觸發過的案件那個舊值就是 NULL，
-- 會把耗盡掃描述詞 (label_retry_count = 0 AND label_retry_at IS NOT NULL) 賴以判定的證據抹掉，
-- 使 LABEL_RETRY_EXHAUSTED 告警只在租約被持有的那幾秒可見，而取樣間隔是 10 分鐘 ⇒ 實質失效。
-- ⇒ **本欄一旦被 claim 過就不得再回到 NULL，唯一例外是 ops 的 resetLabelRetryCount。**
ALTER TABLE aftersale_cases
    ADD COLUMN label_retry_at TIMESTAMPTZ;

COMMENT ON COLUMN aftersale_cases.label_retry_at IS
    'issue #2294 L7：7-11 退貨代碼自動重試的租約到期時間（非「已重試旗標」——旗標是 label_retry_count，而它只在 gRPC 確認成功後才寫）。三態：NULL=從未嘗試過（不算耗盡）、過去時間=嘗試過但無人持有（可搶佔且算耗盡）、未來時間=嘗試中。一旦 claim 過即不得回到 NULL（唯一例外＝ops reset）。';

-- 候選述詞不另建索引：既有的 idx_aftersale_cases_label_retry
--（V1，(created_at, label_retry_count) WHERE status IN ('20','25') AND
--  return_shipping_method = '3002' AND partner_id IS NULL）已經把候選集合壓到極小，
-- label_retry_at 只是其上的後置過濾，再加一支索引買不到東西、卻要付每次寫入的維護成本。
