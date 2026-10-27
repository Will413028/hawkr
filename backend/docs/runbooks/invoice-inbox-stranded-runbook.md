# invoice_inbox_events 擱淺列處置（告警 `InvoiceInboxStranded` / 指標 M30）

> 對應 [58-電子發票詳細設計書](../詳細設計書/58-電子發票服務（Invoice%20Service）詳細設計書.md) §7.2.5（inbox 的兩條推進路徑）與 §7.2.2（作廢／折讓 usecase）。
> **這份 runbook 不是「跑一支腳本把數字歸零」**。M30 是 gauge，它說的是「這些列沒有任何自動執行者」——處置一律要先分辨**為什麼**沒有，四類的正解完全不同。
> 🔴 **禁止**用 reconciliation job／backfill CLI／drift scanner 處理本表（CLAUDE.md §22、後端服務責任邊界規範）。歷史積壓走一次性維運 SQL，新增的靠把 §7.2.2 做完來排空。

## 0. 這條告警在說什麼

```
invoice_inbox_stranded_rows{event_type, status}
  = count(invoice_inbox_events
          WHERE status IN ('RECEIVED','DEFERRED','FAILED')
            AND received_at < now() - interval '7 days')
```

述詞刻意**不列舉「誰有掃描器」**——那份名單隨 code 而變，而 YAML 裡不會有人記得改。判準是與執行者無關的「非終態 ＋ 超過 7 天沒進終態」。

⚠️ **它不會因為「還在重試」而亮**：正常重試的列 `next_attempt_at` 會持續推進、幾分鐘內就進終態。撐過 7 天只有兩種可能——沒有執行者，或執行者每次都失敗。

## 1. 先分類（一句 SQL）

```sql
SELECT event_type,
       status,
       count(*)                          AS rows,
       min(received_at)                  AS oldest,
       count(*) FILTER (WHERE last_error IS NOT NULL) AS with_error
  FROM invoice_inbox_events
 WHERE status IN ('RECEIVED','DEFERRED','FAILED')
   AND received_at < now() - interval '7 days'
 GROUP BY 1,2
 ORDER BY 1,2;
```

| # | `event_type` | 判準 | 正解 |
|---|---|---|---|
| A | `ORDER_PAID` | INV.dl **應該**在掃它 | 是執行者壞了，往 §2 |
| B | `ORDER_REFUND_FINALIZED`，且對應訂單的 `ORDER_PAID` **曾經**進過本服務 inbox | 有執行者了（#3073），出現在這裡代表它反覆失敗 | 讀 `last_error` 找業務原因，往 §3 |
| C | `ORDER_REFUND_FINALIZED`，且對應訂單的 `ORDER_PAID` **從未**進過本服務 inbox | **永遠不可認領**（cutover 積壓） | 一次性維運 SQL 標 SKIPPED，往 §4 |
| D | `ORDER_CANCELLED` | 🔴 **這一格結構上不該出現**：該 handler 全程單交易（58 §6.3.2.4），失敗時 inbox 列與業務寫入一起 rollback ⇒ 它只會是 `PROCESSED` 或根本不存在 | **不要重放**。先確認 `OrderCancelledConsumer.Handle` 還包在 `tx.WithTx` 裡（`TestIntegration_OrderCancelledLeavesNoNonTerminalInboxRow` 守著它）；不變式若完好，就是有人手動插了列，查 `created_at` 與稽核 |
| E | 其他 `event_type` | 新增的訂閱沒有執行者 | 回去補該 event_type 的推進路徑；不要改本告警的門檻 |

🔴 **B 與 C 的差別不是年齡也不是數量，是「反向認領有沒有機會發生」**。反向認領（`OrderPaidConsumer.reclaimDeferred`）只在該訂單的 `ORDER_PAID` **被成功處理的那一刻**跑一次；那個事件若在本服務上線之前就發完了，這一列的唯一出口從第一天就是關的。

分辨 B / C：

```sql
SELECT r.id, r.aggregate_ref, r.received_at,
       EXISTS (SELECT 1 FROM invoice_inbox_events p
                WHERE p.event_type = 'ORDER_PAID'
                  AND p.aggregate_ref = r.aggregate_ref) AS paid_seen
  FROM invoice_inbox_events r
 WHERE r.event_type = 'ORDER_REFUND_FINALIZED'
   AND r.status IN ('DEFERRED','FAILED')
 ORDER BY paid_seen, r.received_at;
```

`paid_seen = false` ⇒ C 類。

## 2. A 類：`ORDER_PAID` 擱淺 ＝ INV.dl 沒在動

INV.dl（`internal/worker/deadline_worker.go`）是它唯一的執行者。照順序查，每一步都能單獨定案：

1. `next_attempt_at` 是不是一直在未來？⇒ 有人持有租約卻死了，等租約到期即可（不要手動改）。
2. pod log 有沒有 `invoice.dl.*`？完全沒有 ⇒ worker 沒啟動（最常見成因是 `Scheduler.Start` 傳錯 ctx：服務健康、`/metrics` 正常、一列都不會處理、log 也沒有錯誤）⇒ `kubectl rollout restart`；重啟後仍無輸出就是 code bug。
3. `last_error` 一直是同一句 ⇒ 那是業務錯誤不是暫時故障，修完再讓它自然重試。

**不要**手動把 `status` 改回 `RECEIVED`——那不是修復，只是把計數清掉。

## 3. B 類：有執行者了，出現在這裡代表它一直失敗

**#3073 起這一格的意義變了。** 在那之前它是「可認領但沒有執行者」，正解是等；現在
`newAdjustmentApplier` 回真的 applier、INV.dl 也在掃 `ORDER_REFUND_FINALIZED`
（走 `FetchDueActionableByType`，**排除 DEFERRED**）⇒ 一列 `RECEIVED`／`FAILED` 的退款
還在這裡，代表**它每一輪都失敗**。

處置是讀 `last_error` 找業務原因，不是重放：

```sql
SELECT id, aggregate_ref, status, attempt_count, last_error, received_at
  FROM invoice_inbox_events
 WHERE event_type = 'ORDER_REFUND_FINALIZED' AND status IN ('RECEIVED','FAILED')
 ORDER BY received_at;
```

常見的三種 `last_error`，修法完全不同：

| `last_error` 形狀 | 意思 | 修法 |
|---|---|---|
| `build allowance draft ... exceeds remaining base` | I3／A17 超額：累計折讓會超過發票金額 | **不是 bug**。上游每開一張新售後單都拿 `orders.total_actual_amount` 當可退上限、不扣既退（58 §5A.9.1 事實 2）。人工判定該筆退款對應哪張發票 |
| `document ... has no provider_invoice_no` | 原發票沒有號碼卻走到建列 | 該 document 的開票結果沒收斂。先讓 INV.rc 把它推到 `ISSUED` 或終態 |
| `lookup document ...` | 讀不到 document | 它在事件抵達當下存在過（否則會落 `DEFERRED`），現在讀不到是真的異常。查 document 是不是被刪了 |

⚠️ **`DEFERRED` 不在本節**：那一格的歸宿是反向認領（`OrderPaidConsumer.reclaimDeferred`），
而兜底掃描刻意不碰它——碰了就會在失敗時翻成 `FAILED`，
`ListDeferredInboxEventsByAggregateRef` 的述詞從此永久不成立，該筆退款再也不會被認領。
放了 7 天以上的 `DEFERRED` 一律是 C 類（往 §4），不是這一節。

⚠️ **「先補租約／CAS」這個曾經的硬性前置條件已被更強的東西取代**：
`uq_invadj_allowance_refund`／`uq_invadj_void_once` 兩個 partial unique 讓第二次 Insert
走 `ON CONFLICT DO NOTHING` ⇒ `RefundAdjustment.Apply` 讀到 `inserted=false` 就 return nil、
**不建 operation**。租約會過期（崩潰、時鐘漂移、GC 停頓），唯一鍵不會。

## 4. C 類：cutover 積壓，一次性維運 SQL

「對應訂單的 `ORDER_PAID` 從未進過本服務 inbox」＝ 這筆退款發生在 invoice-service 上線之前，那張訂單在本服務裡沒有、也永遠不會有 `invoice_documents` 列 ⇒ **沒有可作廢／可折讓的對象**。它缺的不是處理，是「有人知道它在那裡並留下結論」。

`invoice_inbox_events` 已經有這個出口：`SKIPPED` ＋ `disposition`（`chk_invinbox_skip_reason` 保證 SKIPPED 一定說得出理由）。

**執行時機**：invoice-service 首次部署到該環境、且 `order.refund.finalized` 的消費穩定之後（積壓量不再成長）。dev 與 prod 各跑一輪。

**先看，再做**（乾跑）：

```sql
SELECT count(*)
  FROM invoice_inbox_events r
 WHERE r.event_type = 'ORDER_REFUND_FINALIZED'
   AND r.status IN ('DEFERRED','FAILED')
   AND NOT EXISTS (SELECT 1 FROM invoice_inbox_events p
                    WHERE p.event_type = 'ORDER_PAID'
                      AND p.aggregate_ref = r.aggregate_ref);
```

**執行**（單一 transaction，述詞與乾跑逐字相同）：

```sql
BEGIN;

UPDATE invoice_inbox_events r
   SET status             = 'SKIPPED',
       disposition        = 'IGNORED_PRE_CUTOVER',
       disposition_reason = 'order.paid for this order never reached invoice-service; no invoice document exists to void or credit',
       processed_at       = now()
 WHERE r.event_type = 'ORDER_REFUND_FINALIZED'
   AND r.status IN ('DEFERRED','FAILED')
   AND NOT EXISTS (SELECT 1 FROM invoice_inbox_events p
                    WHERE p.event_type = 'ORDER_PAID'
                      AND p.aggregate_ref = r.aggregate_ref);

-- 這一句必須回 0，否則述詞漏了東西，ROLLBACK 重來。
SELECT count(*) FROM invoice_inbox_events r
 WHERE r.event_type = 'ORDER_REFUND_FINALIZED'
   AND r.status IN ('DEFERRED','FAILED')
   AND NOT EXISTS (SELECT 1 FROM invoice_inbox_events p
                    WHERE p.event_type = 'ORDER_PAID'
                      AND p.aggregate_ref = r.aggregate_ref);

COMMIT;
```

⚠️ **`disposition` 的值域是 CodeMeta `InvoiceInboxDisposition`**（`docs/代碼管理（CodeMeta）/內部技術碼.md`），不要直接寫字面值進 DB。
本節用的 `IGNORED_PRE_CUTOVER` 就在那張表裡。
⚠️ 該區塊**在 #3073 之前並不存在**，而這句話當時就已經寫著「要先在 CodeMeta 登記」
—— 照它施工的人會找不到地方登記，然後直接寫字面值。訂規範時要同時把它指向的東西做出來。
⚠️ `chk_invinbox_skip_reason` **只保證「SKIPPED ⇒ disposition 非空」，沒有值域 CHECK**
⇒ 打錯字不會撞 `23514`，只會靜默產生一個沒有人認得的分類。

⚠️ **去重鍵一列都不刪**：`(event_type, business_dedup_key)` 是同一筆退款重投時的唯一防線，刪掉它會讓同一筆退款在 §7.2.2 落地後被再處理一次。本 SQL 只改 `status` 與 `disposition`。

## 5. 為什麼不加一支掃描器就好

本節寫於「applier 還不存在」的年代，那時 `ORDER_REFUND_FINALIZED` 的擱淺列確實無事可做，
所以正解是可觀測性。**#3073 之後，`RECEIVED`／`FAILED` 已經改由 INV.dl 掃**（走
`FetchDueActionableByType`）。

**仍然沒有掃描器的只剩 `DEFERRED`**，而理由與原本一樣、只是更精確了：
它的出口是反向認領，不是掃描。C 類（`ORDER_PAID` 從未進過本服務）的那一批
**唯一的出口從第一天就是關的** ⇒ 加掃描器對它們零幫助，只會把它們翻成 `FAILED`
並順手打壞 B 類的反向認領。所以對 `DEFERRED` 而言，正解仍是可觀測性 ＋ §4 的一次性 SQL。
