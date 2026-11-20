# A6-3 crm 模組：實作設計 v1（草稿，待開工 design-review）

上層：計畫 `../2026-11-19-a6-review-im-crm.md` 的「A6-3」、不變量表、CD1–CD10、A6-D2、A6-D8。路徑相對於 `backend/`；`CR=services/crm-service`、`CRi=CR/internal`、`M=internal`。標記：【量】＝打開檔案或跑過指令；【推】＝讀程式推論。核對時的 HEAD：dev `0243d91d`。

## 0. 前提題：這一步的程式之後會不會被重寫、搬走或刪除？

會，一部分會。CRM 的「消費統計」投影（ledger＋`total_spent`／`order_count`）只因為訂單還在 Java 才存在：計畫「延後」寫明 ordering 搬遷時改讀 ordering 的 `pub_*`、這個投影退場（計畫「延後」第 2 條、Q7【量】）。所以：

- **投影與 legacy consumer**（§3）是過渡程式：只做到正確、冪等、可測，不做讀時聚合、重算工具、回填（舊資料不搬，計畫「前提」）。
- **分類（一般／VIP）與標籤**（§4）是長期資料，ordering 搬遷後仍在 crm，照長期標準設計。
- **coupon legacy client**（§6）若保留，coupon 搬遷時換成 in-process；本檔建議根本不建（W3）。

因此本檔把「投影」的設計壓在最小：一張每單淨額表取代 advisory lock 與 `SUM` 推導（§3.3），其餘沿用舊語意。

## 0.1 範圍

- `internal/modules/crm`：schema（4 張表改形＋1 張每單淨額表）、根套件 API、分類與標籤寫入、客戶列表與詳情讀取（keyset）、`VIPStatuses`。
- `M/legacy/crmconsumer`：`order.completed`、`aftersales.refund.completed` 的 legacy consumer（RabbitMQ → River job → crm 模組），**只在測試中啟用**（不變量「並行期單一生效者」）。
- CD1–CD10 的處置與回歸測試、mutation。
- `CARRYOVER.md`、模組 `AGENTS.md`（七節）、`internal/arch/modules.go` 的 crm 列。

## 0.2 不在範圍

- Connect `SellerCrmService` 與 handler（A6-5）；CRM 的 `lookup`、`meta`、`orders`、`coupons` 改由擁有者提供（A6-5）。
- 任何 client 與 UI（黑名單分頁拿掉在 A6-6，要先取得 Will 同意，計畫 A6-6「UI」條）。
- gateway 路由與 `openapi.yaml`（A6-6e）。
- 在 `cmd/hawkr` 接上 consumer（A6-6e）；`GetBuyerVipStatuses` 的切換時機見 W1（本檔認為與不變量衝突）。
- 刪 `services/crm-service`、`api/proto/crm`（A6-7）。

## 0.3 決策總表（只寫落點）

| 來源 | 決定 | 本檔落點 |
|---|---|---|
| A6-D2 (i) | 刪 `blocklist` 值與 CRM 分頁，封鎖只在店家封鎖名單 | §1 `customer_type` CHECK 只有 `normal`／`vip`；§7；crm 不加 seller 依賴（§8.3） |
| A6-D8 | CRM 只顯示暱稱與訂單統計 | §1 不存 `nickname`／`phone`；§5 讀取不回電話、Email、地址；identity 依賴移除（§8.3）；優惠券數見 W3 |
| Q7／預設 | 以 legacy consumer 保留 ledger 與投影；ordering 搬遷時退場 | §3 |
| Q11／預設 | 客戶在首筆完成訂單時建立 | §3.4：退款不建客戶列 |
| Q9／預設 | keyset，不回總數除非畫面需要 | §5 |
| 不變量「並行期單一生效者」 | consumer 只在測試啟用，A6-6e 才接 | §3.6、§8.4、W1 |
| A6-D9（模式） | 刪帳由模組訂閱 buyer、seller 的刪除事件 | §9，W2 |
| row-identity ADR | 自有實體 UUIDv7；關聯表用自然鍵 | §1 |
| 單體規則 3、4、9 | `db.InTx` 在 use case；同 tx 訂閱只做原子變更；trim 只用 `trimtext` | §3、§4、§9 |

## 0.4 查證過的現況【量】

| 事實 | 證據 |
|---|---|
| 舊 schema 4 張表；`customer_type` 無 CHECK，註解 `normal, vip, blocklist`；`nickname`、`phone` 是快取欄 | `CR/db/schema/schema.sql:4-16` |
| `crm_tags` 無 `UNIQUE(store_id, name)`，只有 `store_id` 索引 | `schema.sql:24-32` |
| `crm_customer_tags.tag_id` FK 只指 `crm_tags(id)`，不保證同店；主鍵 `BIGSERIAL` | `schema.sql:35-42` |
| ledger PK＝`event_id VARCHAR(64)`，`event_type` 無 CHECK | `schema.sql:51-60` |
| 改分類 `UPDATE` 不檢查列數、不驗值 | `CR/db/queries/crm_customers.sql:39-42`；`CRi/usecase/crm.go:199-205` |
| 列表 `ORDER BY` 依 sort 欄後只有 `created_at DESC`，`LIMIT/OFFSET` | `crm_customers.sql:15-21` |
| 退款 upsert 無下限、無列時以 `order_count=0` 建客戶 | `crm_customers.sql:101-106` |
| `AddCustomerTags`：`GetTagByName` 任何錯誤都當不存在而新建；單筆失敗 `continue`／只記 warn，最後 `return nil` | `CRi/usecase/crm.go:256-276` |
| `GetCustomer` 取電話、Email、預設地址、優惠券數，失敗都只記 warn | `crm.go:55-113` |
| 列表讀取後以 `SafeGo` 非同步回寫暱稱快取 | `crm.go:181-189` |
| `RecordOrder`：tx 外查 buyer、account 取暱稱電話；tx 內 advisory lock→若已有全額退款，金額記 0→ledger `ON CONFLICT DO NOTHING`→upsert 客戶 | `crm.go:282-341`；`crm_customer_ledger.sql:12-13` |
| `RecordRefund`：**tx 外**呼叫 `GetOrderByNo` 取 store、buyer（CD10）；部分退款 `-refund_amount`；全額退款 `-SUM(order_no 的 ledger)`；`count_delta=0` | `crm.go:389-446` |
| 舊註解自承殘餘風險：部分退款經重試排到全額退款之後，會再扣一次、永久低估 | `crm.go:378-384` |
| `order.completed`：壞 JSON `return nil`（丟棄）；ID 解析失敗、缺 `order_no`／`event_id` 回一般錯誤（重試到上限）；ID 為 0 `return nil` | `CRi/worker/consumer.go:30-65` |
| `aftersales.refund.completed`：壞 JSON、缺欄位、未知 outcome 都 `mq.Permanent`；`order_not_found` permanent | `consumer.go:124-170` |
| **退款的冪等鍵不是 `event_id`**：`refundDedupKey` 用 `external_ref_id`，缺時 `"AS_REFUND_"+case_no` | `consumer.go:89-116` |
| Java 退款事件每次建構都產生新 UUID 作 `event_id`，且 `RefundSyncWorker` 每小時重發已完成退款 | `services/java/aftersales-service/.../mq/payload/RefundCompletedPayload.java:59`；`.../scheduler/RefundSyncWorker.java:56-73` |
| Java `order.completed` 的 `event_id` 在 outbox 重試間保持不變 | `services/java/order-service/.../mq/OrderOutboxPublisher.java:155-160`（「保留 core 的 eventId（跨重試冪等鍵穩定）」） |
| `buyer.deleted` 只用來抹 `nickname`／`phone` | `consumer.go:184-202`；`crm_customers.sql:108-131` |
| crm-service 沒有 `seller.deleted` 消費者（只綁 3 個事件） | `domain-inventory.md` M8；`consumer.go` 只有 3 個 handler |
| 單體 `legacy/crmservice` 只有 `VIPStatuses`，把 `customer_type == "vip"` 映成 bool | `M/legacy/crmservice/crmservice.go:29-54` |
| 唯一呼叫端：buyer 的會員卡列表 | `M/httpapi/buyer/membership.go:90`（`h.ports.CRM.VIPStatuses`）；介面 `M/httpapi/buyer/ports.go:64-65`；組裝 `cmd/hawkr/main.go:194,290` |
| 依賴表 `crm → ordering, coupon, buyer, identity`；`ordering`、`coupon` 模組尚不存在 | `M/arch/modules.go:30`；`ls internal/modules` |
| 單體沒有 coupon 的 legacy 生成碼（`internal/legacy/gen` 無 coupon） | `ls internal/legacy/gen` |
| `orderservice.OrderParties` 已存在（呼叫 `GetOrderForAfterSales`，訂單不存在回 `ErrOrderNotFound`，其他 fail-closed） | `M/legacy/orderservice/parties.go:14-23` |
| legacy consumer 前例：RabbitMQ handler 只驗證並 `Enqueue` River job（`river:"unique"` 的 `event_id`），permanent 錯誤以型別判斷；queue 名稱 `hawkr.monolith.*` | `M/legacy/buyerconsumer/consumer.go:43-86,196-199`；`rabbitmq.go:12-15` |
| seller 的店家封鎖 API：`BlockBuyer`、`UnblockBuyer`、`ListBlockedBuyers`、`StoreBlocksBuyer`、`StoresBlockingBuyer`、`ClearBlocksAgainstBuyer` | `M/modules/seller/blocks.go:26-53` |
| buyer 沒有暱稱搜尋 API，也沒有 profile 變更事件；有 `BatchDisplay`（刪除者回 Deleted 不回個資） | `M/modules/buyer/buyer.go:233-240`；`events.go` 只有 Favorite／Follow／View／Restock／`BuyerDeleted`／`BuyerDeletionRequested` |
| `lockorder.Modules` 已列 `crm` | `M/platform/lockorder/lockorder.go:50` |

## 1. 資料模型（`crm/db/schema.sql`，Atlas）

```sql
CREATE TABLE crm.customers (
  store_id      bigint      NOT NULL,
  buyer_id      bigint      NOT NULL,
  customer_type text        NOT NULL DEFAULT 'normal' CHECK (customer_type IN ('normal','vip')),
  total_spent   bigint      NOT NULL DEFAULT 0 CHECK (total_spent >= 0),
  order_count   integer     NOT NULL DEFAULT 0 CHECK (order_count >= 0),
  first_order_at timestamptz NOT NULL,
  updated_at    timestamptz NOT NULL,
  PRIMARY KEY (store_id, buyer_id)
);
CREATE INDEX ON crm.customers (store_id, total_spent DESC, buyer_id);
CREATE INDEX ON crm.customers (store_id, order_count DESC, buyer_id);
CREATE INDEX ON crm.customers (store_id, first_order_at DESC, buyer_id);
CREATE INDEX ON crm.customers (buyer_id);              -- 必要條件：W2 採 (b)（BuyerDeleted 依 buyer_id 刪跨店列）；VIPStatuses 用主鍵點查即可，不靠它

CREATE TABLE crm.tags (
  id         uuid        PRIMARY KEY,                 -- UUIDv7
  store_id   bigint      NOT NULL,
  name       text        NOT NULL CHECK (<trimtext.Check>) CHECK (char_length(name) BETWEEN 1 AND 50),
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  UNIQUE (store_id, name),                            -- CD3
  UNIQUE (store_id, id)                               -- 複合 FK 的目標（CD4）
);

CREATE TABLE crm.customer_tags (
  store_id   bigint      NOT NULL,
  buyer_id   bigint      NOT NULL,
  tag_id     uuid        NOT NULL,
  created_at timestamptz NOT NULL,
  PRIMARY KEY (store_id, buyer_id, tag_id),
  FOREIGN KEY (store_id, tag_id) REFERENCES crm.tags (store_id, id) ON DELETE CASCADE,          -- CD4
  FOREIGN KEY (store_id, buyer_id) REFERENCES crm.customers (store_id, buyer_id) ON DELETE CASCADE
);
CREATE INDEX ON crm.customer_tags (store_id, tag_id, buyer_id);

-- 投影（過渡，ordering 搬遷時退場）
CREATE TABLE crm.order_nets (
  order_no     text        PRIMARY KEY,
  store_id     bigint      NOT NULL,
  buyer_id     bigint      NOT NULL,
  completed    boolean     NOT NULL DEFAULT false,    -- 收過 order.completed
  net_amount   bigint      NOT NULL,                  -- 可暫時為負（退款先到）
  updated_at   timestamptz NOT NULL
);
CREATE INDEX ON crm.order_nets (store_id, buyer_id);

CREATE TABLE crm.ledger (
  id          uuid        PRIMARY KEY DEFAULT uuidv7(),  -- row-identity ADR D1「大量、只追加的紀錄」
  dedup_key   text        NOT NULL UNIQUE CHECK (char_length(dedup_key) BETWEEN 1 AND 128),  -- 冪等靠這個 UNIQUE
  order_no    text        NOT NULL REFERENCES crm.order_nets (order_no),
  kind        text        NOT NULL CHECK (kind IN ('order_completed','refund_full','refund_partial'))
);

-- W2 採 (b) 時才建：刪帳買家與退場店家的複本，照 im.inactive_parties（im/db/schema.sql:88）、review.inactive_authors（review/db/schema.sql:91）
CREATE TABLE crm.inactive_parties (
  kind     text        NOT NULL CHECK (kind IN ('BUYER','STORE')),
  party_id bigint      NOT NULL CHECK (party_id > 0),
  since    timestamptz NOT NULL,
  PRIMARY KEY (kind, party_id)
);
```

### 1.1 和舊 schema 的差異與理由

| 差異 | 理由 |
|---|---|
| `customers` 去掉 TSID `id`，主鍵改 `(store_id, buyer_id)` | 舊 `UNIQUE(store_id,buyer_id)` 已是自然鍵，`id` 對外不需要（client 都以 `buyerId` 定位，`client-inventory.md` §2.3）【量】；row-identity ADR：關係列用自然鍵【推，依 ADR 分類】 |
| 去 `nickname`、`phone` | A6-D8、M9：不存個資副本，刪帳後不留在別人的表；讀時經 buyer 補暱稱（§5） |
| `customer_type` CHECK 只有 `normal`、`vip` | CD1／A6-D2 (i)、CD2 |
| `total_spent >= 0`、`order_count >= 0` CHECK | CD7：下限語意由 §3.3 的計算結構保證，CHECK 是第二道 |
| `first_order_at` 取代 `created_at` | 列表預設排序的語意鍵（舊以 `created_at DESC` 排）；`(…, buyer_id)` 次序鍵（CD9） |
| `tags.id` UUIDv7；`UNIQUE(store_id,name)`；`UNIQUE(store_id,id)` | ADR 自有實體 UUIDv7；CD3；CD4 的複合 FK 目標 |
| `customer_tags` 去 `BIGSERIAL`，主鍵＝三欄自然鍵；FK 改 `(store_id, tag_id)`；加 FK 到 `customers` | CD4；只有客戶才能貼標（舊 gateway 以「有訂單」把關，`G/handler/seller_crm.go:20-50`【量】，新規則由 DB 保證） |
| 新增 `order_nets`；ledger 去 `store_id`、`buyer_id`、`count_delta`、`occurred_at` | §3.3：每單淨額列取代 advisory lock＋`SUM`；店與買家只存一次 |
| ledger 主鍵 `id uuid DEFAULT uuidv7()`，冪等改 `UNIQUE(dedup_key)`；去 `amount_delta`、`received_at` | ADR `docs/adr/2026-09-25-row-identity-by-data-kind.md:47`（D1「大量、只追加的紀錄 → `uuid DEFAULT uuidv7()`」）：ledger 是只追加的紀錄，不是值的集合；冪等只需要 UNIQUE。§3.3 只讀 `dedup_key`（去重）與 `kind`（是否全額退款過），金額已在 `order_nets.net_amount`，兩欄無讀取者故刪 |
| 新增 `inactive_parties`（W2 採 (b) 時） | 刪帳後重放舊事件不重建客戶，照 im、review 先例（W2） |
| ledger `event_type` 加 CHECK、改名 `kind` | 值域進 CHECK（AGENTS 契約規則） |

## 2. 分類值域與型別

Go 型別 `crm.CustomerType`（`Normal`、`VIP`），與 CHECK 同一份值域；邊界（A6-5 的 proto enum）未知值回 `InvalidArgument`。`blocklist` 不存在於任何層（CD1）。

## 3. legacy consumer 與投影（過渡）

### 3.1 流程（照 `buyerconsumer` 前例）

RabbitMQ handler 只做解碼與驗證，然後 `jobs.Enqueue` 一個 River job（`dedup_key` 為 `river:"unique"`），ack；worker 呼叫 crm 模組的 `RecordOrderCompleted`／`RecordRefund`。理由：重試交給 River（有上限、可觀測），MQ handler 不碰 DB 與 Java（CD10 的外部呼叫在 worker、tx 外）。

queue：`hawkr.monolith.crm.order.completed`（exchange `hawkr.order`）、`hawkr.monolith.crm.refund.completed`（exchange `aftersales.events`）。【推】自有 queue 名稱代表單體不會和舊 crm-service「搶」同一個 queue，而是各收一份；並行期的真正問題是**兩邊各自生效**（舊服務仍是分類與標籤的寫入者），見 §3.6、W1。

### 3.2 冪等鍵（與計畫文字不同，見 §10 第 1 項）

| 事件 | 鍵 | 依據 |
|---|---|---|
| `order.completed` | `event_id` | Java outbox 重試保持同一 `event_id`【量，§0.4】 |
| `aftersales.refund.completed` | `external_ref_id`，缺時 `"AS_REFUND_"+case_no`；兩者都缺 → permanent | `event_id` 每次重發都是新 UUID【量】；照用 `event_id` 會讓每小時的重發重複扣款。沿用舊 `refundDedupKey`（`consumer.go:104-116`） |

ledger 的 `dedup_key` 有命名空間前綴（`order:`、`refund:`），兩種事件的鍵不會互撞【推，舊碼沒有前綴，兩者值域在實務上不重疊但未保證】。長度上限 128（舊為 64，前綴加長；超過 → permanent）。

### 3.3 投影計算（CD7）

每單一列 `order_nets`，`customers.total_spent` 定義為該客戶所有 `completed` 訂單的 `GREATEST(net_amount, 0)` 之和。一個事件的 transaction：

1. `INSERT … ON CONFLICT (order_no) DO UPDATE … RETURNING`（或 `SELECT … FOR UPDATE`）取得並鎖住該單的列——取代舊的 `pg_advisory_xact_lock(hashtext(order_no))`，同單事件序列化，且鎖是一般列鎖，lockorder 檢查器看得到。
2. `INSERT INTO ledger … ON CONFLICT (dedup_key) DO NOTHING`；0 列 → 重放，結束（不改任何東西）。
3. 依事件更新 `net_amount`：
   - `order_completed`：若單上已有全額退款 → 金額 0（沿用舊語意，`crm.go:310-317`）；否則 `net += amount`；`completed = true`。
   - `refund_partial`：`net -= refund_amount`。
   - `refund_full`：`net = 0`（等同舊的 `-SUM`，但直接寫結果）。
4. 在步驟 1 已鎖住的 `order_nets` 列上算增量：`delta = (new_completed ? GREATEST(new_net,0) : 0) - (old_completed ? GREATEST(old_net,0) : 0)`（只算 completed 的單；即 `GREATEST(new,0) - GREATEST(old,0)`），若客戶列存在（或本事件是 `order_completed` 而需建立）則 `UPDATE customers SET total_spent = total_spent + delta`；`order_completed` 首次另 `order_count += 1`。不用 `SUM` 子查詢重算：READ COMMITTED 下同一客戶的兩張不同訂單並發時，後到者等完 `customers` 列鎖後 EvalPlanQual 只重讀目標列，`SET` 裡的子查詢仍用語句開始時的快照，會漏算對方剛 commit 的 `order_nets`（lost update）【推，依 PG READ COMMITTED 的 UPDATE 語意】；增量 `total_spent + delta` 在 EPQ 下讀到最新列值，`customers` 的列級 UPDATE 讓同一客戶的事件序列化。
5. W2 採 (b) 時：在投影 transaction 內、步驟 1 之前，依序持 (BUYER, buyer_id)、(STORE, store_id) 的 advisory lock（`pg_advisory_xact_lock`，與訂閱者的 `Deactivate` 用同一把），再查 `inactive_parties`；已 inactive → no-op（ack，不寫 ledger、`order_nets`、`customers`）。照 im 的 `Send`／`Deactivate` 串行（`im/internal/purge/purge.go:59,68`【量】）：沒有這把鎖時，投影讀到「未 inactive」後、commit 前，訂閱者 commit 並由刪除 job 先跑完，投影 commit 會重建客戶。刪帳後 ledger 被刪、dedup 不再擋重放，此時的冪等由 `inactive_parties` 承擔。另照 im 加保底掃描：已在 `inactive_parties` 但仍有列的身分補排刪除 job。

結果：

- `total_spent` 結構上 ≥ 0（每單貢獻 ≥ 0）【推】，CHECK 是第二道防線。
- 舊碼自承的「部分退款排在全額退款後再扣一次」（`crm.go:378-384`）也被吸收：全額後 `net=0`，再扣變負，貢獻仍 0【推】。
- 退款先到、訂單後到：`net` 暫為負、`completed=false` 不計；訂單到後 `net = amount - refund`，正確。
- 代價：每事件 O(1)（只動一張單與一位客戶的列）【推】。
- 不變量：`total_spent` 恆等於該客戶 completed 訂單的 `SUM(GREATEST(net,0))`，因為每次只把該單貢獻的變化量加上去；§11.1 以並發測試與事後 SUM 對照驗證。

`order_count` 的語意：完成訂單數，退款不扣（沿用舊語意 `crm.go:343-345`，舊註解說退款訂單仍留在購買紀錄）；不另立待決題，因為 CHECK `>= 0` 已滿足 CD7 的「下限寫進 CHECK」。

### 3.4 客戶列的建立（Q11）

只有 `order_completed` 建 `customers` 列（`INSERT … ON CONFLICT DO UPDATE`）。舊碼的退款會以 `order_count=0` 建客戶（`crm_customers.sql:101-106`）【量】，造成「0 筆訂單的客戶」；新設計退款只動 `order_nets`，客戶不存在時不建，之後訂單到時重算已含退款。

### 3.5 錯誤分類（CD6）

| 情況 | 分類 | 舊行為 |
|---|---|---|
| JSON 解碼失敗、必要欄位缺漏或格式不合（ID 非正整數、金額非整數、未知 `refund_outcome`、鍵過長） | permanent（進 DLQ 並記 log），兩個事件一致 | `order.completed` 壞 JSON 丟棄、缺欄位重試；ID 為 0 丟棄（`consumer.go:30-65`） |
| `OrderParties` 回 `ErrOrderNotFound` | permanent（River `JobCancel`） | 同（`consumer.go:124-129`） |
| `OrderParties` 其他錯誤、DB 錯誤 | 重試（River 退避，有上限） | 同 |
| 金額為負 | permanent | 舊不驗 |

handler 回傳的 permanent 錯誤用單一型別（照 `buyerconsumer.isPermanent`），兩個 handler 共用一個分類函式。

### 3.6 註冊與並行期

`crmconsumer.Consumer.Run` 與 worker 寫好、以 integration 測試啟用；**不在 `cmd/hawkr` 呼叫**，A6-6e 刪 CRM 路由的同一變更才接（計畫不變量）。River worker 的註冊同樣延到 A6-6e【推：註冊 worker 無害，但沒有排入者時無意義；放在同一處接線較不易漏】。

## 4. 寫入路徑

所有寫入帶 `store_id`（呼叫端由 seller session 決定，A6-5），use case 開 `db.InTx`。

| 方法 | 行為 | CD |
|---|---|---|
| `SetCustomerType(store, buyer, type)` | 值域驗證 → `UPDATE … WHERE store_id AND buyer_id`，0 列回 NotFound | CD2 |
| `CreateTag(store, name)` | `trimtext.Trim`、1–50 rune；`INSERT`，撞 `UNIQUE(store_id,name)` 回 AlreadyExists | CD3 |
| `RenameTag(store, tagID, name)` | 0 列 NotFound；撞名 AlreadyExists | CD3 |
| `DeleteTag(store, tagID)` | 0 列 NotFound；關聯由 FK cascade | — |
| `AddCustomerTags(store, buyer, names)` | 名稱去重、驗證；單一 tx：`INSERT INTO tags … ON CONFLICT (store_id,name) DO NOTHING`，`SELECT id … WHERE name = ANY`，`INSERT INTO customer_tags … ON CONFLICT DO NOTHING`；客戶不存在 → FK 失敗轉 NotFound；任何錯誤整批回滾回錯 | CD3、CD5 |
| `RemoveCustomerTag(store, buyer, tagID)` | 冪等刪除（0 列不是錯，與「移除後的狀態」一致）【推，舊亦不報錯】 | — |

上限：單次 `names` ≤ 20、每店標籤數上限見 W4（舊無上限【量，schema 無】）。

## 5. 讀取

| 方法 | 回傳 | 備註 |
|---|---|---|
| `ListCustomers(store, filter{type, tagID}, sort{total_spent\|order_count\|first_order_at}, page)` | `buyer_id`、`customer_type`、`total_spent`、`order_count`、`first_order_at`、標籤 | keyset `(sort_key, buyer_id)`，`platform/pagination` token；不回總數（Q9）；`DESC`／`ASC` 都用同一索引方向（PG 可反向掃）。CD9 |
| `GetCustomer(store, buyer)` | 同上＋標籤 | 不回電話、Email、地址（CD8／A6-D8）；優惠券數見 W3 |
| `ListTags(store)` | 全部（受上限約束） | |
| `VIPStatuses(buyer, stores)` | `map[store]bool` | 取代 `legacy/crmservice.VIPStatuses`；會被 buyer 的 httpapi 呼叫，不用 `InReadSnapshot`（規則 3） |

暱稱與頭像不在 crm：A6-5 的 handler 以 `buyer.BatchDisplay` 組裝（httpapi composition，規則 2；刪帳者顯示 Deleted）。舊的 keyword 搜尋（暱稱、電話 `ILIKE`，`crm_customers.sql:10`）沒有替代資料來源，見 W5。`SearchCustomersByTags`（多標籤交集）沒有 client 呼叫者【量，`client-inventory.md` §1「沒有呼叫者的 gateway 路由」】，不搬。

## 6. coupon legacy client

舊用途只有一個：`GetCustomer` 的 `CouponCount`（`crm.go:106-113`）【量】。A6-D8 的「只顯示暱稱與訂單統計」字面上排除它，且計畫 A6-5 已寫 `coupons` 改由擁有者的方法提供。建 client 還要先生成 coupon 的 legacy 程式碼（目前沒有）。建議不建、從計畫 A6-3 範圍移除，依賴表 crm 去 `coupon`（W3）。

## 7. 黑名單刪除（CD1、A6-D2 (i)）的影響面

模組內：值域沒有 `blocklist`；不加 seller 依賴（(i) 不在 CRM 顯示封鎖）。

受影響、但改在 A6-6（本步不動）【量，`client-inventory.md` §2.3、§6.2】：

- website：`lib/seller/repo.ts:276,330-332`、`CustomersClient.tsx:37-62`（分頁與徽章）、`CustomerDetailPanel.tsx:29-31`（「加入黑名單」）、`lib/content/data.ts:427-432`、`seller-guide-data.ts:482,494`（說明文字）。
- seller-app：`customer_management_screen.dart:313`（黑名單分頁）、`customer_detail_screen.dart`、`customer_model.dart:31`、`customer_repository.dart` 等 6 處。
- 替代入口已存在：website `blocked-buyers/BlockedBuyersPage.tsx`、seller-app `block_list_screen.dart`（A7）。

舊 crm-service 在 A6-6e 前仍接受 `blocklist`，不改舊服務（計畫「不做」）。

## 8. 根套件 API、錯誤、依賴與註冊

### 8.1 根套件（`crm/crm.go`）

```go
type Deps struct{ DB *db.DB; Now func() time.Time; NewID func() (uuid.UUID, error); Logger *slog.Logger }
func New(Deps) *Module
func Migrations() fs.FS

// 讀
func (m *Module) ListCustomers(ctx, CustomerListInput) (CustomerPage, error)
func (m *Module) GetCustomer(ctx, storeID, buyerID int64) (Customer, error)
func (m *Module) ListTags(ctx, storeID int64) ([]Tag, error)
func (m *Module) VIPStatuses(ctx, buyerID int64, storeIDs []int64) (map[int64]bool, error)
// 寫
func (m *Module) SetCustomerType(ctx, storeID, buyerID int64, t CustomerType) error
func (m *Module) CreateTag / RenameTag / DeleteTag / AddCustomerTags / RemoveCustomerTag
// 投影（只給 legacy/crmconsumer）
func (m *Module) RecordOrderCompleted(ctx, OrderCompleted) (recorded bool, err error)
func (m *Module) RecordRefund(ctx, Refund) (recorded bool, err error)   // Refund 含 OrderParties 解析後的 store、buyer
```

`storeIDs` 上限照 `buyer.MaxBatchSize` 同級常數；空集合回空 map（沿用 `crmservice.go:30-32`）。

### 8.2 錯誤

`apperr` 分類：NotFound（客戶、標籤）、AlreadyExists（標籤名）、InvalidArgument（值域、長度、批量）、Unavailable（DB）。reason 常數 `CRM_CUSTOMER_NOT_FOUND`、`CRM_TAG_NOT_FOUND`、`CRM_TAG_NAME_TAKEN`、`CRM_TAG_LIMIT`（A6-5 加進 proto `ErrorReason`）。

### 8.3 依賴表

| 改動 | 理由 |
|---|---|
| 去 `identity` | A6-D8：不再讀電話、Email |
| 去 `coupon` | W3 採建議時 |
| 去 `buyer`？ | crm 自己不呼叫 buyer（暱稱在 httpapi 組裝；刪帳訂閱若採 W2 (b) 則需要 buyer 事件型別 → **保留 `buyer`**） |
| 保留 `ordering` | 表內名義依賴；實際走 `legacy/orderservice` 在 `legacy/crmconsumer`，模組本身不 import【推】。ordering 搬遷時投影換讀 `pub_*` |
| W2 採 (b) 時加 `seller` | 理由：刪帳訂閱 `seller.StoreRetired`（`seller/events.go:31`，A6-D9 模式），不是封鎖；W2 不採 (b) 則不加（封鎖不需要，A6-D2 (i)） |

`lockorder`：crm 已在 `Modules`；W2 (b) 的訂閱者在 buyer、seller 事件 tx 內只持該身分的 advisory lock 並插入 `inactive_parties`、排 job；`customers`、`order_nets`、ledger 由刪除 job 在自己的 tx 刪（先 ledger 後 `order_nets`，或 FK 加 `ON DELETE CASCADE`，實作擇一並寫測試），順序 buyer → crm、seller → crm 皆合法；crm 不回頭鎖 buyer 或 seller 的列。

### 8.4 註冊

- `cmd/hawkr/modules.go` 加 `{Name: "crm", Dir: crm.Migrations()}`；`cmd/hawkr/crm.go` 的 `newCRM` 建構模組（serve 與 worker）。
- `trimmedColumns` 登記 `crm.tags.name`。
- consumer 與 worker：A6-6e。
- `VIPStatuses` 的接線：W1。

## 9. 刪帳與退場

舊服務：`buyer.deleted` 只抹 `nickname`／`phone`，客戶列、標籤、ledger 都保留；店家退場沒有處理（§0.4）【量】。新 schema 沒有個資欄，剩下的是「店家對某買家的分類與標籤」與消費統計，以 `buyer_id`（假名鍵）存放。W2 決定是否訂閱。

## 10. 本檔提議的計畫修改（已套用，A6-D18）

1. A6-3 範圍「ledger 冪等鍵取自事件 ID」改為「`order.completed` 取 `event_id`；退款取 `external_ref_id`（缺時 `case_no`），因 Java 每次重發退款事件換新 `event_id`」（§3.2，證據【量】）。
2. A6-3 範圍的「coupon legacy client」依 W3 移除；依賴表 crm 去 `identity`（A6-D8 的直接結果）、W3 採建議時去 `coupon`。
3. A6-3 範圍的「`GetBuyerVipStatuses` 改 in-process；刪 `internal/legacy/crmservice`」依 W1 移到 A6-6e（與 A6-D11 同理）。
4. CD7 處置補一句：下限由「每單淨額取 `GREATEST(…,0)` 加總」結構保證，順帶吸收舊碼自承的「部分退款排在全額後」殘餘風險。

## 11. 測試與 mutation

### 11.1 測試

- unit：值域、名稱 trim／長度、consumer 解碼與分類表格（兩事件 × 每種壞輸入，期望同一分類）、`refundDedupKey`。
- integration（CI）：
  - **重放冪等**：同一 `order.completed` 送兩次、同一退款以不同 `event_id`、同 `external_ref_id` 送兩次 → `total_spent`、`order_count`、ledger 列數不變。
  - 事件順序表格：{訂單→部分→全額, 全額→訂單, 部分→訂單, 訂單→全額→部分（重試排後）} → 期望 `total_spent`，皆 ≥ 0。
  - 並發：同單的訂單與全額退款並行 N 次 → 結果與序列化一致。
  - 並發：同一客戶、不同訂單的 `order.completed`／退款並行 N 次 → `total_spent` 等於事後 `SUM(GREATEST(net_amount,0)) WHERE completed` 與期望值。
  - W2 採 (b) 時：買家刪帳（或店家退場）後重放該人的 `order.completed`、退款 → 不重建 `customers`、`order_nets`、ledger 列。
  - `AddCustomerTags` 並發同名 → 一個標籤；含一個超長名 → 整批無寫入。
  - `customer_tags` 以他店 tag 插入 → FK 失敗。
  - keyset：同 `total_spent` 多列跨頁無重複無遺漏。
  - `VIPStatuses`：混合有無客戶列的店。
  - 退款先到不建客戶列。

### 11.2 mutation（在複本跑；改回舊寫法，測試要失敗）

| CD | 改法 | 應失敗的測試 |
|---|---|---|
| CD1 | CHECK 加回 `'blocklist'` 並讓 Go 值域接受（`CRi/domain/customer_types.go:8-10`） | 值域拒絕 `blocklist` |
| CD2 | `SetCustomerType` 不檢查列數（`crm_customers.sql:39-42` 的 `:exec`） | 非客戶回 NotFound |
| CD2 | 拿掉 CHECK 與 Go 值域驗證（`crm.go:199-205`） | 任意字串被拒 |
| CD3 | 拿掉 `UNIQUE(store_id,name)`，`AddCustomerTags` 改回先查再建、錯誤當不存在（`crm.go:257-265`） | 並發同名只有一個標籤 |
| CD4 | FK 改回只指 `tags(id)`（`schema.sql:39`） | 他店 tag 插入失敗 |
| CD5 | 單筆失敗 `continue`、回 nil（`crm.go:261-276`） | 整批無寫入且回錯 |
| CD6 | `order.completed` 壞 JSON 回 nil（`consumer.go:30-33`） | 分類表格：壞 JSON → permanent |
| CD7 | `total_spent` 改回增量加減、拿掉 `GREATEST`（`crm_customers.sql:101-106`） | 「訂單→全額→部分」案例 ≥ 0、等於 0 |
| CD7 | 退款路徑改回會建客戶列（`crm_customers.sql:102-103`） | 退款先到不建客戶列 |
| CD7 | 步驟 4 改回以 `SUM` 子查詢重算 `total_spent` | 同一客戶不同訂單並發 |
| CD7 | 步驟 4 拿掉 `delta`（只更新 `order_nets`，不加到 `customers`） | 事件順序表格、同客戶不同訂單並發 |
| W2 (b) | 拿掉 `inactive_parties` 檢查 | 刪帳後重放不重建 |
| W2 (b) | 投影不持身分 advisory lock（只查不鎖） | 投影與 `Deactivate` 交錯時不重建（以兩條連線控制交錯的 integration） |
| CD8 | 回應結構加回 phone（`crm.go:77-95`） | 【推】改以「`Customer` 型別沒有聯絡欄位」的編譯期保證，mutation 改為 A6-5 handler 回應欄位的 golden 測試 |
| CD9 | 排序拿掉 `buyer_id` 次序鍵（`crm_customers.sql:15-20`） | keyset 同值跨頁 |
| 冪等 | 退款鍵改用 `event_id` | 不同 `event_id` 同 `external_ref_id` 重放 |
| CD10 | 接受，不做 mutation | — |

## 12. 沿用盤點（寫進 `CARRYOVER.md`）

| 機制 | 從零設計會怎麼做 | 決定 |
|---|---|---|
| ledger＋投影（M7） | 讀時聚合 ordering 的資料 | 沿用（資料在 Java，Q7），ordering 搬遷時退場 |
| advisory lock 序列化同單 | 每單一列的列鎖 | 改（§3.3） |
| 全額退款以 ledger 自身歷史淨額（不信 Java 的 `refund_total_amount`，`crm.go:348-365`） | 同：以自己記錄的淨額為準 | 沿用（`net=0`） |
| 全額退款先到時後到訂單記 0 | 同 | 沿用 |
| 退款 dedup 鍵 `external_ref_id`／`case_no` | 上游給穩定事件 ID | 沿用（Java 不改，計畫「不做」），ordering／aftersales 搬遷時換單體事件 |
| MQ 直接處理 | consumer 只排 River job | 改（buyerconsumer 前例）；正確性（冪等）來自 ledger 的 `UNIQUE(dedup_key)`，River 的 `UniqueOpts` 只在 job 還保留時有效，只是減少重複排程的最佳化，不能據此拿掉 ledger |
| tx 外呼叫 order 取店與買家（CD10） | 事件自帶或 in-process | 接受，移到 River worker |
| 暱稱電話快取＋非同步回寫（M9） | 讀時補 | 刪 |
| TSID `id`、`BIGSERIAL` | 自然鍵／UUIDv7 | 改 |
| page/size＋COUNT（M18） | keyset | 改 |
| `customer_type` 字串無 CHECK | enum＋CHECK | 改 |
| `SearchCustomersByTags`、`GetCustomerStats` 等無呼叫者 RPC | 不做 | 不搬（A6-5 對照） |
| `buyer.deleted` 抹 PII | 沒有 PII 就不需要 | 見 W2 |

## 13. 待決題

### W1 `VIPStatuses` 改 in-process 的時機——已決（Will 2026-11-20，A6-D18）：(a)

衝突：計畫 A6-3 要「`GetBuyerVipStatuses` 改 in-process、刪 `legacy/crmservice`」，但不變量「並行期單一生效者」讓分類寫入（gateway → 舊 crm-service）留到 A6-6e；consumer 也到 A6-6e 才接。A6-3 就切換的話，crm 模組的 `customers` 是空表，買家會員卡的 VIP 全部顯示為否（`membership.go:90`）——與 A6-D11 延後評分統計切換是同一個情況。

- (a) 延到 A6-6e：A6-3 只建 `crm.VIPStatuses`（含測試），A6-6e 刪路由、接 consumer 的同一變更把 `httpbuyer.Ports.CRM` 換成模組、刪 `legacy/crmservice`。代價：legacy client 多活幾步。
- (b) 照計畫在 A6-3 換：VIP 顯示從 A6-3 到 A6-6e 全錯（系統未上線，只影響本機與 e2e 的觀察）。
- **建議 (a)**：與 A6-D11 一致，不違反不變量；計畫修改見 §10 第 3 項。

### W2 刪帳時 crm 資料怎麼處理——已決（Will 2026-11-20，A6-D18）：(b)

事實：新表無個資欄；剩 `buyer_id` 為鍵的分類、標籤（店家對此人的主觀標註）與消費統計。舊服務保留它們【量】。A6-D9 的模式是 review、im 訂閱 buyer、seller 刪除事件。

- (a) 不訂閱：資料保留，暱稱由 buyer 讀時回 Deleted。最少程式；店家對已刪帳者的標註永久保留。
- (b) 訂閱 `buyer.BuyerDeleted`：同 tx 插入 `crm.inactive_parties`（BUYER）並排 job 刪該買家在各店的 `customers`（cascade 標籤）與 `order_nets`、ledger；訂閱 seller 的 `StoreRetired` 同理（STORE）刪該店全部列。「刪後重放舊事件會重建」由 `inactive_parties` 結構上關閉：§3.3 步驟 5 遇 inactive 的買家或店家直接 no-op，照 `im.inactive_parties`（`im/db/schema.sql:88`）、`review.inactive_authors`（`review/db/schema.sql:91`）先例【量，`rg -n inactive internal/modules/{review,im}/db/schema.sql`】。不靠「刪帳時無進行中訂單」（【推】，且退款可能在完成很久後才到、`RefundSyncWorker` 每小時重發）。代價：多一張只插入的表與每事件一次點查；依賴表加 `seller`（§8.3）。
- (c) (b) 但只刪分類與標籤，統計保留（假名、非個資）。
- **建議 (b)**：標籤是對可識別個人的主觀註記，刪帳應清除；與 review、im 一致；店家退場時資料無擁有者用途。投影本身過渡，刪除不影響 ordering 搬遷。

### W3 優惠券數與 coupon legacy client——已決（Will 2026-11-20，A6-D18）：(a)

- (a) 不建 client，CRM 詳情不回優惠券數；需要時 A6-5 由 coupon 擁有者的方法提供（計畫 A6-5 已列 `coupons`）。依賴表去 `coupon`。
- (b) 照計畫建 coupon legacy client（含生成 legacy coupon 程式碼），詳情回優惠券數。
- **建議 (a)**：A6-D8 只留暱稱與訂單統計；少一個過渡 client。

### W4 標籤數上限——已決（Will 2026-11-20，A6-D18）：(a)

舊無上限【量】。(a) 每店 200、每客戶 20；(b) 不設。**建議 (a)**：`ListTags` 不分頁、`AddCustomerTags` 單 tx，需要有界；數字為【推】的合理值，等 Will 改。

### W5 客戶列表的關鍵字搜尋——已決（Will 2026-11-20，A6-D18）：(a)

舊以 crm 內的暱稱、電話快取 `ILIKE`（`crm_customers.sql:10`）；A6-D8／M9 刪了快取，buyer 沒有搜尋 API 與 profile 變更事件【量】。website、seller-app 都送 keyword【量，`client-inventory.md` §2.3】。

- (a) 拿掉關鍵字搜尋（UI 改動，A6-6 要 Will 同意）；賣家以分類、標籤、排序找人。
- (b) buyer 加「依暱稱前綴查 buyer_id」API，httpapi 先查 buyer 再以 `buyer_id = ANY` 篩 crm（結果集大小要有上限）。
- (c) crm 存暱稱複本，buyer 加 profile 變更事件同 tx 更新（單體規則 4 允許的複本）。刪帳仍要抹除。
- **建議 (a)**：最少機制、無個資複本；若 Will 認為搜尋必要，選 (b)。

## 14. 開工 design-review 的處理

兩輪：第 1 輪審全文，第 2 輪只審第 1 輪修正的段落（ledger 1–7 皆未退步）。

| # | finding | 嚴重度 | 處理 |
|---|---|---|---|
| 1 | §3.3 步驟 4 以 `SUM` 子查詢重算 `total_spent`，同客戶不同訂單並發時 lost update | should | fixed：§3.3 步驟 4 改在鎖住的 `order_nets` 列上算 `delta`、`total_spent + delta`；§11.1 加同客戶不同訂單並發測試；§11.2 加兩條 CD7 mutation（改回 SUM、拿掉 delta） |
| 2 | ledger 主鍵＝`dedup_key`，未依 row-identity ADR 歸類；`amount_delta`、`received_at` 無讀取者 | should | fixed：§1 ledger 改 `id uuid DEFAULT uuidv7()`＋`UNIQUE(dedup_key)`，刪兩欄；§1.1 加一列引 ADR D1（`row-identity-by-data-kind.md:47`） |
| 3 | W2 (b) 刪後重放會重建客戶，缺 tombstone | should | fixed：§1 加 `crm.inactive_parties`；§3.3 步驟 5 no-op；W2 (b) 改寫（仍未決，建議 (b)）；§11.1、§11.2 加測試與 mutation |
| 4 | §8.3「不加 seller」與 W2 (b) 訂閱 `StoreRetired` 矛盾 | should | fixed：§8.3 改「W2 採 (b) 時加 seller」，lockorder 補 seller → crm |
| 5 | `customers(buyer_id)` 索引的必要條件寫錯 | nit | fixed：§1 註解改為必要條件＝W2 採 (b) |
| 6 | River unique 與 ledger 兩層去重的分工沒寫 | nit | fixed：§12 MQ 列註明正確性來自 ledger UNIQUE，River unique 只是最佳化 |
| 7 | CD8 mutation 延到 A6-5 但 A6-5 驗收沒寫 | nit | fixed：計畫 A6-5 加一行 CD8 驗收（回應欄位測試＋加回欄位的 mutation） |
| 8 | W1–W5 待決 | nit | rejected（不改）：各題已附建議與取捨，交 Will 決定 |

| 9 | 第 2 輪：W2 (b) 的 inactive 檢查在 tx 外、無鎖，投影與 `Deactivate`、刪除 job 交錯時仍會重建 | should | fixed：§3.3 步驟 5 改照 im 先例持身分 advisory lock 後查、加保底掃描；§11.2 加 mutation |
| 10 | 第 2 輪：§8.3 說訂閱者鎖 `customers` 等三表，與 W2 (b) 只插 inactive、job 刪除不一致 | nit | fixed：§8.3 改寫 |
| 11 | 第 2 輪：ledger→`order_nets` FK 無 cascade、刪後冪等的承擔者沒寫在 §3.3 | nit | fixed：§8.3 寫刪除順序或 cascade（實作擇一＋測試）；§3.3 步驟 5 寫明 |

findings 11 (blocking 0)，fixed 10、recorded 0、raised 0、rejected 1
