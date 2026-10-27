# 存量已取消訂單回填 `invoice_cancelled_orders`（issue #2904）

> 對應 [58-電子發票服務（Invoice Service）詳細設計書](../詳細設計書/58-電子發票服務（Invoice%20Service）詳細設計書.md) §5.1.9a／§6.1.3。
> **何時跑**：`invoice-service` 的 `order.cancelled` consumer（C3）**部署完成之後**，越快越好（理由見 §3）。
> **兩個 DB**：來源 `hawkr_order`（`orders`）、目標 `hawkr_invoice`（`invoice_cancelled_orders`）。同一台 PostgreSQL、不同 database ⇒ 無法 JOIN，走 §4 的匯出／匯入。
> **DB 連線**：bastion ＋ 1Password 取憑證（同 [food-safety runbook](./food-safety-incident-runbook.md) §2 慣例）；dev 與 prod 各跑一輪，**dev 先跑完整一輪再上 prod**。

---

## 0. 為什麼需要這一份

端點 8（B04 訂單發票區）在「這張訂單沒有任何 `invoice_documents` 列」時，原本一律回
`state=AWAITING_PAYMENT` ＋「付款完成後會立即開立發票」。那句話對**已取消**的訂單永遠不會
成真——它不會再被付款——而買家在同一個畫面上方看到的正是紅色的「已取消」時間軸。

invoice-service 的讀路徑看不到訂單狀態，所以 #2904 讓 `order.cancelled` 把這個事實送進本地的
`invoice_cancelled_orders`，讀路徑在「零 document」時查它，命中就改回
`ORDER_CANCELLED` ＋ 對應文案（`internal/usecase/query_read.go` 的 `GetOrderInvoice`）。

🔴 **但 consumer 只吃得到「它上線之後」發生的取消**。在它上線之前就已經取消的訂單，
`invoice_cancelled_orders` 裡沒有那一列 ⇒ **那些訂單的發票區仍然顯示原本那句錯話**。
症狀與修復前一模一樣：零錯誤、零 log、畫面完全正常，只是那句話是假的。

⇒ 存量資料需要一次性回填，而它是**維運操作不是程式**：依
[後端服務責任邊界規範](../開發規範/後端服務責任邊界規範.md)（CLAUDE.md §22），
一次性歷史資料清整走 migration SQL 或經審核的維運 SQL，**不得**在 application code 裡建
backfill CLI／reconciliation job／drift scanner。本表沒有 status 欄、沒有排程欄、沒有任何
worker 掃它，回填完就結束，沒有需要長期維護的東西。

**為什麼不放進 Atlas migration**：資料在**另一個 database**，migration 只跑在 invoice 自己的
連線上，拿不到 `orders`。硬要做只剩 `dblink`／`postgres_fdw`，那是為了一次性操作而永久
擴大 invoice DB 的信任面，代價遠大於一次人工 SQL。

---

## 1. 跨庫做法的裁決：匯出／匯入，**不是**讓 order-service 重發 `order.cancelled`

| 方案 | 判定 | 理由 |
|---|---|---|
| 讓 order-service 重發 `order.cancelled` | 🔴 **否決** | `order.cancelled` **不只有發票在聽**。`infra/mq/topology/topology.yaml` 對同一個 routing key 綁了兩條 queue：`invoice.order.cancelled` 與 `product.order.cancelled`，後者是 product-service 的**庫存歸還**。重放會用新的 `event_id` 打進去 ⇒ 對方的冪等鍵是新的 ⇒ **幾個月前的訂單被再歸還一次庫存**。而要「只發給發票」就得為此新增一條旁路 routing key ＋ 一支只在這次用的發送程式，那正是 §22 禁止的 backfill 基礎設施。 |
| order DB 唯讀匯出 → invoice DB 匯入 | ✅ **採用** | 只碰 invoice 自己的一張 append-only 表，不動 MQ、不驚動任何其他 consumer；`ON CONFLICT DO NOTHING` 讓它**可重跑**；每一步都看得到筆數。 |

---

## 2. 欄位對照（回填要寫成 consumer 會寫的樣子）

| `invoice_cancelled_orders` | 來源 | 說明 |
|---|---|---|
| `order_no` | `orders.order_no` | PK |
| `buyer_id` | `orders.buyer_id` | ⚠️ 這一欄在 V8 之前叫 `buyer_account_id`（`V8__account_id_to_buyer_id.sql` 改名），**值本來就是 buyer 業務鍵 surrogate**，不是登入 account_id——與 `OrderCancelledPayload.buyerId`（producer 寫 `order.getBuyerId()`）同源。目標表有 `CHECK (buyer_id > 0)`。 |
| `cancel_reason` | `orders.cancel_reason` | 可為 NULL（買家自願取消未付款訂單）。⚠️ **來源 VARCHAR(200)、目標 VARCHAR(32)**，跑之前先做 §3 的長度檢查。 |
| `occurred_at` | `COALESCE(orders.cancelled_at, orders.updated_at)` | ⚠️ **近似值**。`cancelled_at` 是 `V11__add_order_cancelled_at.sql` 才加的，更早取消的列是 NULL。可接受的理由：這一欄**沒有任何讀者**——讀路徑 `IsOrderCancelledForBuyer` 只回 EXISTS，不看時間；它存在是為了事後查案。回填時要知道自己寫的是近似值，不要拿它當稽核依據。 |
| `created_at` | `now()` | 回填當下 |

**回填範圍刻意比 consumer 廣**：producer 的 `OrderService.insertCancelledOutbox` 第一件事是
「查不到成功的扣減紀錄就 `return`（不落 outbox）」⇒ `order.cancelled` 的真實語意是
「**有庫存要歸還的**訂單取消了」。那個守衛的存在理由是庫存，與發票無關——「這張訂單已取消」
對買家而言不論有沒有扣過庫存都是真的。回填直接取 `status = '80'` 的全集，是**比 live 路徑更完整**
而不是不一致。

---

## 3. 部署順序：**先部署 consumer，再跑回填**

與 [seller-notification-identity-store-id-backfill](./seller-notification-identity-store-id-backfill.md)
相反，理由也不同，不要照抄那一份的結論：

- 那一份是 `UPDATE` 既有列、目標鍵可能被新寫入佔走 ⇒ 撞鍵會讓整批 rollback ⇒ 必須 SQL-first。
- **本份是 `INSERT ... ON CONFLICT DO NOTHING`**：與 live consumer 寫同一列時靜默跳過，
  結構上不可能撞鍵 ⇒ 沒有 SQL-first 的理由。

而 consumer-first 有一個 SQL-first 沒有的好處：**匯出快照之後才發生的取消由 live 路徑自動接手**，
中間沒有缺口。倒過來做的話，「跑完 SQL 到 consumer 上線」這段窗口內的取消訂單誰都不會寫，
要再跑第二輪才補得回來。

⇒ 順序固定：`invoice-service` 部署完成並確認 `invoice.order.cancelled` 的 `consumers ≥ 1`
（`rabbitmqctl list_queues name consumers`，見 CLAUDE.md §10.7）→ 才跑下面的回填。

---

## 4. 執行

### 4.1 前置檢查（`hawkr_order`，唯讀）

```sql
SET TIME ZONE 'UTC';

-- (a) 母體量級
SELECT count(*) AS cancelled_orders FROM orders WHERE status = '80';

-- (b) 🔴 buyer_id 必須全部 > 0（目標表 CHECK）。非 0 即停，先查那幾筆為什麼。
SELECT count(*) AS bad_buyer_id FROM orders WHERE status = '80' AND (buyer_id IS NULL OR buyer_id <= 0);

-- (c) 🔴 cancel_reason 長度：來源 VARCHAR(200)、目標 VARCHAR(32)。非 0 即停。
--     **不要自作主張截斷**——那一欄含 SELLER_SUSPENDED 這類內部代碼，截斷後看起來仍像
--     一個合法代碼，是最糟的一種壞法。真的出現時另案決定（改欄寬 or 該筆寫 NULL）。
SELECT count(*) AS overlong_reason FROM orders
WHERE status = '80' AND length(cancel_reason) > 32;

-- (d) occurred_at 會有多少筆落在近似值上（cancelled_at 為 NULL ⇒ 退用 updated_at）
SELECT count(*) FILTER (WHERE cancelled_at IS NULL) AS approx_occurred_at,
       count(*)                                     AS total
FROM orders WHERE status = '80';
```

(b)、(c) 任一不為 0 ⇒ **停下來**，不要往下走。

### 4.2 匯出（`hawkr_order`，唯讀）

用 `\copy`（psql 客戶端指令，不需要伺服器檔案權限、不需要任何 extension）。
筆數可能上千，「用 SQL 產生 SQL」逐句貼在這個規模不好審；CSV 一樣人工看得到、還能 `wc -l`。

```
\set ON_ERROR_STOP on
SET TIME ZONE 'UTC';
\copy (SELECT order_no, buyer_id, cancel_reason, COALESCE(cancelled_at, updated_at) AS occurred_at FROM orders WHERE status = '80' AND buyer_id > 0 ORDER BY order_no) TO 'cancelled-orders-YYYYMMDD.csv' WITH (FORMAT csv, HEADER)
```

匯出後對一次帳：`wc -l cancelled-orders-YYYYMMDD.csv` 應等於 §4.1 (a) 的筆數 ＋ 1（header）。
不相等就是 (b) 排掉了東西，回頭看。

### 4.3 匯入（`hawkr_invoice`）

```
\set ON_ERROR_STOP on
SET TIME ZONE 'UTC';
BEGIN;

CREATE TEMP TABLE t_cancelled (
  order_no     varchar(64)  NOT NULL,
  buyer_id     bigint       NOT NULL,
  cancel_reason varchar(200),          -- 先照來源寬度收，長度已在 §4.1 (c) 擋過
  occurred_at  timestamptz  NOT NULL
) ON COMMIT DROP;

\copy t_cancelled FROM 'cancelled-orders-YYYYMMDD.csv' WITH (FORMAT csv, HEADER)

-- 先看讀進來幾筆，與 §4.2 對得起來才往下。
SELECT count(*) AS staged FROM t_cancelled;

INSERT INTO invoice_cancelled_orders (order_no, buyer_id, cancel_reason, occurred_at, created_at)
SELECT order_no, buyer_id, cancel_reason, occurred_at, now()
FROM t_cancelled
ON CONFLICT DO NOTHING;   -- 刻意不指名 conflict target，理由同 invoice_cancelled_orders.sql

-- 這裡先跑 §5 的對帳查詢，全部通過才 COMMIT。
-- ROLLBACK;
COMMIT;
```

⚠️ `ON CONFLICT DO NOTHING` **不指名 conflict target**，與 `InsertCancelledOrder` 一致：
指名之後若這張表日後多一個唯一約束，撞到另一個會拋 23505 ⇒ 整個交易 aborted（25P02，
見 CLAUDE.md §11.1），之後每一句 SQL 都被拒。

---

## 5. 對帳（COMMIT 之前，在同一個交易內跑）

```sql
-- (1) 🔴 來源全集必須都在目標裡（漏一筆 = 那張訂單的買家仍看到錯話）
SELECT count(*) AS missing
FROM t_cancelled t
LEFT JOIN invoice_cancelled_orders c USING (order_no)
WHERE c.order_no IS NULL;
-- 期望 0

-- (2) 🔴 buyer_id 必須一致。這一條同時是「讀路徑的授權述詞對不對得上」的實測——
--     讀路徑是 WHERE order_no = $1 AND buyer_id = $2，$2 來自 gateway 的**現任** surrogate。
SELECT count(*) AS buyer_mismatch
FROM t_cancelled t
JOIN invoice_cancelled_orders c USING (order_no)
WHERE c.buyer_id <> t.buyer_id;
-- 期望 0；非 0 表示 live consumer 與 orders 對同一張訂單的 buyer_id 有分歧，停下來查

-- (3) 目標表總筆數 ≥ 來源筆數（多出來的是 live consumer 在匯出之後寫的，正常）
SELECT (SELECT count(*) FROM invoice_cancelled_orders) AS target_rows,
       (SELECT count(*) FROM t_cancelled)              AS source_rows;
```

(1)(2) 任一非 0 ⇒ `ROLLBACK;`，不要 COMMIT。

**功能面收尾驗證**（COMMIT 之後，不能省——上面三條只證明資料進去了，沒證明畫面變了）：
挑一張回填到的已取消訂單，用**該買家的帳號**打端點 8
（`GET /app/orders/{order_no}/invoice`），斷言 `data.state == "ORDER_CANCELLED"`
且文案不再是「付款完成後會立即開立發票」。挑一張**未取消且未付款**的訂單做陰性對照，
它必須仍然是 `AWAITING_PAYMENT`。

---

## 6. 收尾與已知邊界

- CSV 是**含買家 id 與訂單號的營運資料**：跑完即刪，不要留在 bastion 或本機（同 CLAUDE.md 紅線 15）。
- 本回填**只跑一次**。跑完之後新的取消一律由 C3 consumer 承接；如果日後又發現缺列，
  那是 consumer 出問題，要去查 `invoice.order.cancelled` 的 `consumers` 數與 DLQ，
  **不是**再跑一次這份 runbook。
- ⚠️ **`invoice_cancelled_orders.buyer_id` 寫入後永不更新**，而讀路徑的述詞是
  `buyer_id = $2`（gateway `buyerid` resolver 解出來的**當前** surrogate）。買家 re-key
  （CLAUDE.md §6 的 W2／W3，進行中）之後，舊列的 `buyer_id` 會對不上新的 surrogate ⇒
  該訂單靜默退回錯誤文案。這是**既有的系統性風險、非本次回填引入**，回填只是讓受影響的
  列變多。追蹤見 §7。

## 7. 未決

- 買家 re-key 後 `invoice_cancelled_orders.buyer_id` 失準（上一節最後一項）——
  屬 buyer_id re-key W2／W3 的收斂範圍，需與該波一起決定是「跟著 re-key 一起搬」還是
  「讀路徑改查 order_no ＋ 由 order-service 驗歸屬」。本 runbook 不處理。
