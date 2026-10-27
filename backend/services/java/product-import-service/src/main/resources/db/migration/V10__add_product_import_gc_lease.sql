-- issue #2294 L9：多副本排程搶佔（Scheduler Claim）—— product-import 側 tierA 的租約欄
--
-- 設計書：40 §8.5；規範：docs/開發規範/多副本排程搶佔規範.md
--
-- 本服務的 GC 有兩條需要搶佔的路徑，只有其中一條需要新欄位：
--
--   ① runCallbackTimeoutGc（每 5 分，asset DOWNLOADING 逾時 → FAILED ＋ 可能發 job.parsed MQ）
--      候選述詞 = status='9302'(DOWNLOADING) AND updated_at < now − callbackTimeoutMinutes
--      ⇒ **候選述詞自己就用了一個時間欄**，把 updated_at 推到 claim 當下即可讓該列退出候選
--        callbackTimeoutMinutes（＝有效租約），崩潰後自動回來。**零 schema 變更**。
--      ⚠️ 此處推進 updated_at 不違反 L5 對 updated_at 的禁令：L5 是拿它當「事件已補發」的記號
--        （語意錯配），這裡它就是候選述詞本身的語意（「多久沒動過」）。
--
--   ② tierAExpireJobsBody（每日 02:00，job 過期 → EXPIRED ＋ item 批次 REMOVED ＋ 發 expire_job MQ）
--      候選述詞 = expires_at < now AND status NOT IN (終態)
--      ⇒ 唯一會把該列推出候選集合的是 **status 轉終態**，那是業務狀態轉換本身
--        （《多副本排程搶佔規範》通則 0：狀態欄當閘門 = 永久旗標，claim 後崩潰就再也回不來，
--         expire_job 事件永久遺失且無告警）。
--      ⇒ expires_at **不可**挪用（它是業務時戳「這個匯入任務何時失效」，推它等於延長任務壽命）。
--      ⇒ 必須另加一個純租約欄，就是本 migration。
--
-- 租約欄語意：值 = 目前持有者的租約到期時間；NULL 或已過期 = 可搶佔。
-- NULL ＝「從未被搶佔、立即可搶」＝新欄位的預期初始狀態，**不需要也不可以回填**。

ALTER TABLE product_import_job
    ADD COLUMN gc_lease_until TIMESTAMPTZ;

COMMENT ON COLUMN product_import_job.gc_lease_until IS
    'issue #2294 L9：GC tierA（過期任務）的租約到期時間（非「已處理旗標」）。NULL 或 <= NOW() 表示可搶佔。';

-- 候選述詞：expires_at < #{now} AND status NOT IN ('9106','9107','9108','9103')
--           AND (gc_lease_until IS NULL OR gc_lease_until <= #{now})
DROP INDEX IF EXISTS idx_pij_expires_at_for_gc;
CREATE INDEX idx_pij_expires_at_for_gc ON product_import_job (expires_at, gc_lease_until)
    WHERE status NOT IN ('9106','9107','9108','9103');
