# 顧客管理微服務 (CRM Service) 詳細設計書

## 0. 文件元資訊 (Metadata)

| 項目 | 值 |
|---|---|
| 對應程式碼 | `services/crm-service/internal/{usecase,grpc,repository,worker}/`、`services/crm-service/db/schema/schema.sql`、`gateway/internal/handler/crm.go` |
| 相關 doc | [44-身份模型](./44-買賣家身分分離（Buyer%20%26%20Seller%20Identity%20Separation）詳細設計書.md)（`store_id`／`buyer_id` 語意）、36 號《MQ 與事件總覽》（本服務消費的兩個事件） |

---

## 1. 概述

CRM Service 負責管理「賣家 → 買家」之間的 CRM 關係，採**多租戶**設計：每個賣家（StoreID）的客戶資料、標籤、行銷任務彼此完全隔離。

| 項目 | 說明 |
|------|------|
| 語言 | Go |
| gRPC Port | 10030 |
| Management / health transport | gRPC `grpc.health.v1.Health` only（aggregate `""` + named `hawkr.crm.CrmService`） |
| Database | PostgreSQL（`hawkr_crm`） |
| 資料庫存取 | sqlc 自動生成（type-safe） |
| DI | Uber fx |
| Observability | OpenTelemetry trace + metrics，OTLP push to Collector（不暴露 `/metrics`） |

**服務位置**：`backend-payuni/services/crm-service/`

---

## 2. 多租戶模型

```
賣家 A (store_id = 1001)
  ├── 客戶 X (buyer_id = 2001)  ← 狀態：正常
  ├── 客戶 Y (buyer_id = 2002)  ← 狀態：VIP
  └── 標籤：[老客戶, 高消費]

賣家 B (store_id = 1002)
  ├── 客戶 X (buyer_id = 2001)  ← 狀態：黑名單（與賣家A互不影響）
  └── 標籤：[新客戶]
```

- `store_id` = 賣家的 store 代理鍵（Gateway 由 JWT account_id 解析賣家身份後取得，不在 URL 中出現）
- `buyer_id` = 買家的 `buyers.buyer_id`（per-stint PK，非帳號 ID，永不等於 account_id）
- 所有查詢必須同時帶 `store_id`，確保資料隔離

---

## 3. 資料庫設計

**Database**：`hawkr_crm`

### 3.1 `crm_customers`（賣家對買家的 CRM 關係）

| 欄位 | 類型 | 說明 |
|------|------|------|
| `id` | BIGSERIAL PK | |
| `store_id` | BIGINT NOT NULL | 賣家 store_id（多租戶 key） |
| `buyer_id` | BIGINT NOT NULL | 買家 buyer_id（buyers.buyer_id，per-stint PK，非帳號 ID） |
| `customer_type` | VARCHAR(20) DEFAULT 'normal' | `normal` / `vip` / `blocklist` |
| `total_spent` | BIGINT DEFAULT 0 | 累計消費金額（台幣，整數元） |
| `order_count` | INT DEFAULT 0 | 累計訂單數 |
| `nickname` | VARCHAR(100) DEFAULT '' | 快取自單體 buyer 模組（搜尋用） |
| `phone` | VARCHAR(20) DEFAULT '' | 快取自單體 buyer 模組（搜尋用） |
| `created_at` | TIMESTAMPTZ DEFAULT NOW() | |
| `updated_at` | TIMESTAMPTZ DEFAULT NOW() | |

**約束與索引**：
```sql
UNIQUE (store_id, buyer_id)
INDEX idx_crm_customers_store_id (store_id)
INDEX idx_crm_customers_customer_type (store_id, customer_type)
```

> `CreateCustomer` 使用 `ON CONFLICT (store_id, buyer_id) DO NOTHING`，首次下單時由 Order Service 呼叫自動建立記錄，冪等安全。

### 3.1.1 `crm_customer_ledger`（客戶消費增量帳）

`crm_customers.total_spent` / `order_count` 不是直接被覆寫的欄位，而是這張 event-sourced 增量帳套用出來的結果。

| 欄位 | 類型 | 說明 |
|------|------|------|
| `event_id` | VARCHAR(64) PK | **冪等鍵**：來源 MQ 事件的 event_id，`ON CONFLICT DO NOTHING` ⇒ 同一事件重投不會重複計算 |
| `store_id` | BIGINT NOT NULL | 賣家 store_id |
| `buyer_id` | BIGINT NOT NULL | 買家 buyer_id |
| `event_type` | VARCHAR(30) NOT NULL | `order_completed` \| `refund_completed_full` \| `refund_completed_partial` |
| `order_no` | VARCHAR(64) NOT NULL | 訂單編號（netting 的分組鍵） |
| `amount_delta` | BIGINT NOT NULL | 對 `total_spent` 的增量（退款為負） |
| `count_delta` | INT NOT NULL | 對 `order_count` 的增量 |
| `occurred_at` | TIMESTAMPTZ DEFAULT NOW() | |

**索引**：`idx_crm_customer_ledger_customer (store_id, buyer_id, occurred_at)`、`idx_crm_customer_ledger_order_no (order_no)`

**不變式**：
- 每筆事件恰好落一列，冪等鍵是 `event_id`；帳是 append-only，**不 update、不刪**。
- 同一 `order_no` 的所有事件以 `pg_advisory_xact_lock(hashtext(order_no))` 序列化——兩個 MQ subscription 各自是獨立 goroutine，沒有這把鎖就會有 lost update。鎖必須是該 transaction 的**第一個**語句。
- `order_count` 不因退款遞減：已退款的訂單仍留在賣家的購買紀錄清單裡。
- **全額退款不信任來源事件的退款金額**，而是把該 `order_no` 在本帳的歷史總和取負（`amount_delta = -SUM(amount_delta) WHERE order_no = $1`），在同一 transaction 內讀取。理由：全額退款的主流路徑上 `order.completed` 結構性地永遠不會到達，netting 到任何「來源回報的固定金額」都會錯。

⚠️ **已知殘留風險**：本鎖提供互斥，不提供**順序**。若 `PARTIAL_REFUND` 因 MQ retry 被重投到佇列尾端、落在 `FULL_REFUND` 已經歸零之後，它仍會套用自己的負增量，使 `total_spent` 永久偏低。這需要事件本身已經亂序再加上一次暫時性重試才會發生，屬已接受的殘留風險。

---

### 3.2 `crm_tags`（賣家自定義標籤）

| 欄位 | 類型 | 說明 |
|------|------|------|
| `id` | BIGSERIAL PK | |
| `store_id` | BIGINT NOT NULL | 賣家 store_id |
| `name` | VARCHAR(50) NOT NULL | 標籤名稱 |
| `created_at` | TIMESTAMPTZ DEFAULT NOW() | |
| `updated_at` | TIMESTAMPTZ DEFAULT NOW() | |

**索引**：
```sql
INDEX idx_crm_tags_store_id (store_id)
```

> 標籤只有名稱，沒有顏色／描述／排序欄位。

---

### 3.3 `crm_customer_tags`（客戶 ↔ 標籤 多對多關聯）

| 欄位 | 類型 | 說明 |
|------|------|------|
| `id` | BIGSERIAL PK | |
| `store_id` | BIGINT NOT NULL | 賣家 store_id |
| `buyer_id` | BIGINT NOT NULL | 買家 buyer_id（buyers.buyer_id，per-stint PK，非帳號 ID） |
| `tag_id` | BIGINT NOT NULL REFERENCES crm_tags(id) ON DELETE CASCADE | |
| `created_at` | TIMESTAMPTZ DEFAULT NOW() | |

**約束與索引**：
```sql
UNIQUE (store_id, buyer_id, tag_id)
INDEX idx_crm_customer_tags_customer (store_id, buyer_id)
INDEX idx_crm_customer_tags_tag (tag_id)
```

> 標籤刪除時，`ON DELETE CASCADE` 自動清除所有關聯紀錄。
> `AddCustomerTag` 使用 `ON CONFLICT DO NOTHING`，重複新增冪等。

---

---

## 4. API 設計

### 4.1 HTTP REST API（via Gateway）

所有路由在 `/api/v1/app/sellers/me/` namespace 下，需要 JWT 認證且具備賣家權限（account_type 2 或 3）。

#### 客戶管理

| Method | Path | 說明 |
|--------|------|------|
| `GET` | `/api/v1/app/sellers/me/customers` | 取得客戶列表（分頁、篩選、排序） |
| `GET` | `/api/v1/app/sellers/me/customers/:buyerId` | 取得單一客戶詳情（含標籤） |
| `PATCH` | `/api/v1/app/sellers/me/customers/:buyerId` | 更新客戶分類（VIP / 黑名單） |
| `GET` | `/api/v1/app/sellers/me/customers/:buyerId/meta` | 查詢買家基本資訊（不限 CRM 客戶，透過單體 buyer 模組） |

> URL 路徑參數統一使用 `:buyerId`，不使用 `:customerId`。收到的值就是 `buyers.buyer_id`（per-stint PK，永不等於 account_id），Gateway 不做任何解析，直接透傳給 crm-service（見 §4.2「客戶識別碼是 buyerId」）。

**GET /customers Query Parameters**：

| 參數 | 類型 | 說明 |
|------|------|------|
| `page` | int | 頁碼，預設 1 |
| `page_size` | int | 每頁筆數，預設 20 |
| `customer_type` | string | 空=全部、`normal` / `vip` / `blocklist` |
| `keyword` | string | 搜尋關鍵字（ILIKE 模糊搜尋） |
| `tag_id` | int | 依標籤篩選，0=不篩選 |
| `sort_by` | string | `total_spent` / `order_count` / `created_at` |
| `sort_order` | string | `asc` / `desc`，預設 `desc` |

**PATCH /customers/:buyerId Request Body**：
```json
{
  "customer_type": "blocklist"
}
```

#### 標籤管理

| Method | Path | 說明 |
|--------|------|------|
| `GET` | `/api/v1/app/sellers/me/tags` | 列出賣家所有標籤 |
| `POST` | `/api/v1/app/sellers/me/tags` | 建立標籤（單筆） |
| `PATCH` | `/api/v1/app/sellers/me/tags/:tagId` | 更新標籤名稱 |
| `DELETE` | `/api/v1/app/sellers/me/tags/:tagId` | 刪除標籤（自動解除所有客戶關聯） |

**POST /tags Request Body**：
```json
{ "name": "高消費" }
```

**PATCH /tags/:tagId Request Body**：
```json
{ "name": "高消費客戶" }
```

#### 客戶標籤

| Method | Path | 說明 |
|--------|------|------|
| `GET` | `/api/v1/app/sellers/me/customers/:buyerId/tags` | 取得客戶的所有標籤 |
| `POST` | `/api/v1/app/sellers/me/customers/:buyerId/tags` | 批次將標籤貼到客戶（by 名稱，find-or-create） |
| `DELETE` | `/api/v1/app/sellers/me/customers/:buyerId/tags/:tagId` | 移除客戶的某個標籤 |

**POST /customers/:buyerId/tags Request Body**：
```json
{
  "tag_names": ["VIP", "高消費", "新客"]
}
```

> 傳標籤**名稱**而非 ID：後端自動查找該賣家是否已有同名標籤，有則直接使用，無則建立後再貼到客戶。此設計讓前端快速標籤（hardcode 名稱列表）不需要預先取得 ID 即可使用。

---

### 4.2 gRPC Interface（內部服務呼叫）

**Proto**：`backend-payuni/api/proto/crm/crm.proto`
**Package**：`hawkr.crm`
**Go Package**：`github.com/Will413028/hawkr/go-pkg/proto/crm`

```protobuf
service CrmService {
    // ── 客戶管理 ──────────────────────────────────
    rpc GetCustomerList(GetCustomerListRequest)       returns (CustomerListResponse);
    rpc GetCustomerStats(GetCustomerStatsRequest)     returns (CustomerStatsResponse);
    rpc UpdateCustomerStatus(UpdateCustomerStatusRequest) returns (CrmResponse);

    // ── 標籤管理 ──────────────────────────────────
    rpc CreateTag(CreateTagRequest)   returns (TagResponse);
    rpc ListTags(ListTagsRequest)     returns (TagListResponse);
    rpc UpdateTag(UpdateTagRequest)   returns (CrmResponse);
    rpc DeleteTag(DeleteTagRequest)   returns (CrmResponse);

    // ── 客戶標籤 ──────────────────────────────────
    rpc AddCustomerTags(AddCustomerTagsRequest)       returns (CrmResponse);  // tag_names: []string，find-or-create
    rpc RemoveCustomerTag(RemoveCustomerTagRequest)   returns (CrmResponse);
    rpc GetCustomerTags(GetCustomerTagsRequest)       returns (TagListResponse);
}
```

**時間欄位**：gRPC 中時間傳遞為 Unix timestamp（int64 秒），0 代表 null。

**統一 Response**：
```protobuf
message CrmResponse {
    int32 code = 1;    // 0 = 成功
    string message = 2;
}
```

**CrmCustomer**：
```protobuf
message CrmCustomer {
    string id = 1;
    string store_id = 2;            // 賣家 store_id
    string buyer_id = 3;            // 買家 buyers.buyer_id（per-stint PK，非帳號 ID，永不等於 account_id）
    string customer_type = 4;       // "normal" / "vip" / "blocklist"
    string total_spent = 6;
    string order_count = 7;
    string nickname = 11;           // 即時從單體 buyer 模組 BatchGetBuyerMeta enrichment
    string avatar_key = 12;         // 即時從單體 buyer 模組 BatchGetBuyerMeta enrichment（objectKey，非 URL）
    repeated Tag tags = 13;
    string coupon_count = 17;       // 從 coupon-service enrichment
}
```

> **Enrichment 機制**：CRM 本地 DB 僅快取 nickname/phone 供搜尋用。`ListCustomers` 時批量呼叫單體 buyer 模組的 `BatchGetBuyerMeta` 取得最新的 nickname + 頭像 objectKey。Avatar 不存在 CRM DB 中，每次查詢即時取得。
>
> **頭像欄位契約（49 §6.1／§6.6 S9）**：crm-service 只透傳 objectKey（`avatar_key`），**URL 一律由 Gateway 在對外回應層組**，顧客列表與詳情都以 `avatarKey` + `avatarUrl` 成對輸出（`http` 開頭的外部頭像由 `mediaurl.BuildURL` passthrough）。兩支端點的 wire shape 由 gateway golden test（`seller_customer_list_success` / `seller_customer_detail_success`）鎖定。
>
> **對外欄位一律 camelCase**：顧客列表、詳情、`lookup`、`{buyerId}/meta`、`search-by-tags` 五支的回應欄位名統一 camelCase。標籤相關端點（`/customers/{id}/tags`、商家標籤 CRUD）的 key 由 gateway 的 `sellerTagResp` mapping struct 決定，**不是** proto passthrough，同樣是 camelCase。
>
> ⚠️ **客戶識別碼是 `buyerId`**——`id` 是 crm row 主鍵、`storeId` 是賣家自己的 store surrogate，前端拿這兩個當客戶 ID 都會指向別的對象且無任何錯誤訊號。`buyerId` 的值是 `buyers.buyer_id`（per-stint PK，永不等於 `account_id`），可直接回頭打 `/customers/{buyerId}` 系列端點。
>
> `{buyerId}/meta` 的識別碼欄位對外叫 `buyerId`（**不是 `storeId`**）。⚠️ 它的值取自單體 buyer 模組 `BatchGetBuyerMetaByBuyerIDs` 的 `BuyerMeta.account_id`——**那個 proto 欄位裝的其實是 buyer_id**（沿用單體 seller 模組 `GetSellerBatch` 慣例）。proto 的錯名屬 buyer 的 `BatchGetBuyerMetaByBuyerIDs`，Gateway 負責不讓它滲進對外契約：對外只能是 `buyerId`。
>
> ⚠️ **前端一律只讀 `buyerId`，不得補 fallback chain**。多層 `??` 在 wire contract 上不是韌性，是把 breaking change 從 fail-fast 降級成 silent-wrong：讀不到就該壞，不該悄悄拿到別人的 ID。

---

## 5. 程式碼架構

```
services/crm-service/
├── cmd/crm-service/main.go        # 進入點
├── cmd/grpc-healthcheck/main.go   # container / local gRPC health probe
├── internal/
│   ├── app/app.go                 # fx.App 組裝
│   ├── config/
│   │   ├── config.go              # 環境變數讀取
│   │   └── di.go                  # Config / Logger / DB Pool
│   ├── domain/
│   │   ├── customer.go            # CrmCustomer, CrmTag, CrmCustomerTag entity + CrmRepository interface
│   │   ├── customer_types.go      # CustomerType enum (normal/vip/blocklist)
│   │   └── errors.go              # ErrCustomerNotFound, ErrTagNotFound, ...
│   ├── worker/                    # MQ consumer（order.completed / aftersales.refund.completed）
│   ├── usecase/
│   │   ├── crm.go                 # CrmUseCase（客戶、標籤、客戶標籤業務邏輯 + ledger 套用 + 單體 buyer 模組 enrichment）
│   │   ├── di.go
│   │   └── dto/
│   │       ├── customer_req.go    # ListCustomersReq, UpdateCustomerStatusReq
│   │       ├── customer_res.go    # Customer, PaginatedCustomers
│   │       ├── tag_req.go         # CreateTagReq, UpdateTagReq, ManageCustomerTagReq
│   │       └── tag_res.go         # Tag
│   ├── grpc/
│   │   ├── handler.go             # CrmServer（實作 proto CrmService）
│   │   ├── mapper.go              # DTO → proto message 轉換
│   │   └── di.go
│   ├── repository/
│   │   ├── di.go
│   │   └── postgres/
│   │       ├── crm_repository.go      # CrmRepository 實作
│   │       ├── mapper.go              # gen.* → domain.* 轉換
│   │       ├── di.go
│   │       └── gen/                   # sqlc 自動生成（禁止手動編輯）
│   └── infrastructure/
│       └── di.go                  # shared gRPC bootstrap + OTel providers
└── db/
    ├── schema/schema.sql          # 完整 DB Schema
    ├── queries/                # sqlc SQL 定義（檔案清單以目錄實況為準）
    └── sqlc.yaml
```

**依賴方向**：
```
grpc/handler → usecase → domain
repository/postgres → domain（實作 interface）
usecase 不可 import grpc、repository
```

---

## 6. 業務邏輯細節

### 6.1 客戶記錄的建立

CRM 客戶記錄**不是**由賣家手動建立，而是由 Order Service 在買家首次下單時透過 gRPC 呼叫 `CreateCustomer` 自動建立。

`CreateCustomer` 使用 `ON CONFLICT DO NOTHING`，重複呼叫安全。

### 6.2 客戶統計更新（MQ 驅動）

`crm_customers.total_spent` / `order_count` 由 CRM 自己消費 MQ 事件維護，**不是**由 Order Service 呼叫 RPC 推進。

| 訂閱 queue | 來源事件 | 動作 |
|---|---|---|
| `crm.order.completed` | 訂單完成 | append `order_completed` ledger 列（`count_delta=+1`），同 transaction 內 upsert 客戶並套用增量 |
| `crm.aftersales.refund.completed` | 售後退款完成 | append `refund_completed_full` 或 `refund_completed_partial` ledger 列（`amount_delta` 為負，`count_delta=0`），同 transaction 內套用增量 |

**每筆處理都在單一 transaction 內完成**，順序固定：`LockOrderForNetting(order_no)` → 讀既有 ledger → append 新列（`ON CONFLICT (event_id) DO NOTHING`）→ 套用增量到 `crm_customers`。

- **冪等鍵是事件的 `event_id`**：insert 回 0 rows 即代表該事件已處理過，直接結束、不重複套用增量。
- ⚠️ `aftersales.refund.completed` 的 `event_id` 在來源端**不保證穩定**，consumer 端的處理見 `internal/worker/consumer.go`；缺 `order_no` 或缺 `event_id` 的事件一律當錯誤退回（無法去重就不能處理）。
- 資料模型與 netting 規則見 §3.1.1。

### 6.3 標籤貼到客戶的 Find-or-Create 流程

`AddCustomerTags` 接受標籤**名稱**陣列（`tag_names`），usecase 依序對每個名稱執行：
1. `GetTagByName(storeID, name)` — 查找該賣家是否已有同名標籤
2. 找到 → 直接取得 tag ID
3. 找不到 → 呼叫 `CreateTag` 建立後取得 tag ID
4. 呼叫 `AddCustomerTag(storeID, buyerID, tagID)` — 冪等（ON CONFLICT DO NOTHING）

任一步驟失敗只 log、繼續下一個，不中斷整體操作。

### 6.3.1 前端取得 tag ID 的流程（新增後刪除）

`AddCustomerTags` 只回傳 `CrmResponse{code, message}`，**不回傳 tag ID**。前端需要 ID 才能呼叫 `RemoveCustomerTag`，做法如下：

```
AddCustomerTags（傳 tag_names）
    ↓
GetCustomerStats（重新 fetch 客戶詳情）
    ↓  回傳 CustomerStats.tags[]（含 id + name）
前端 tags.find(t => t.name === 剛剛輸入的名稱)
    ↓  得到 id
RemoveCustomerTag（傳 tag_id）
```

- `GetCustomerStats` 回傳的 `tags` 已包含完整 tag 資訊（id、name），不需另外呼叫 `GetCustomerTags`。
- 由於同一賣家帳號下 tag name 唯一，用 name 配對不會有歧義。
- 前端在渲染 tag 列表時即可帶著 id，後續刪除直接使用，**不需每次刪除前重新 fetch**。

### 6.4 客戶分類語意

| customer_type | 說明 | 影響 |
|---------------|------|------|
| `normal` | 正常 | 無限制 |
| `vip` | VIP | 顯示用，無系統強制行為 |
| `blocklist` | 黑名單 | 標記用，**目前無系統強制行為**（與 `vip` 同級）——結帳路徑不查本欄位 |

---

## 7. 環境變數

| 變數 | 預設值 | 說明 |
|------|--------|------|
| `ENV` | `development` | 環境名稱（production 需設定 OTLP metrics endpoint） |
| `DB_HOST` | `localhost` | PostgreSQL host |
| `DB_PORT` | `5432` | PostgreSQL port |
| `DB_USER` | `postgres` | PostgreSQL user |
| `DB_PASSWORD` | `postgres` | PostgreSQL password |
| `DB_NAME` | `hawkr_crm` | Database 名稱 |
| `GRPC_PORT` | `10030` | gRPC 監聽 port |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | - | OTLP Collector endpoint；production 必填（或使用 signal-specific endpoint） |
| `OTEL_EXPORTER_OTLP_METRICS_ENDPOINT` | - | OTLP metrics endpoint；優先於共用 OTLP endpoint |

---

## 8. gRPC Server 配置

CRM 使用 `pkg/server.NewGRPCServer` 建立唯一 gRPC listener（10030），並由 shared bootstrap 統一處理：

**攔截器鏈（Unary）**：
1. `CorrelationInterceptor` — 傳遞 correlation / trace context
2. `LoggingInterceptor` — 記錄 request / response
3. `InternalAuthServerInterceptor` — 服務間授權
4. `RecoveryInterceptor` — panic recovery
5. `ErrorInterceptor` — 錯誤碼統一轉換

另由 shared gRPC stats handler 與 OTel providers 匯出 trace / metrics 至 Collector。

**gRPC Reflection**：啟用，供 grpcurl / Postman 探索。

**gRPC health contract**：

- aggregate service `""`：listener bind 成功後為 `SERVING`，供 startup / liveness probe。
- `hawkr.crm.CrmService`：FX readiness hook 完成後為 `SERVING`，供 readiness probe。
- shutdown 開始前兩者轉為 `NOT_SERVING`，再由 shared server graceful drain。
- 未宣告的 service name 維持標準 `NOT_FOUND`。

CRM 不提供 service-local HTTP `/health`、`/metrics` 或 `/debug/pprof/*`；對外 HTTP API 仍由 Gateway 轉譯並呼叫 CRM 的 gRPC RPC。

---

## 9. 錯誤處理

**Domain 錯誤**（`internal/domain/errors.go`）：

| 錯誤 | 說明 |
|------|------|
| `ErrCustomerNotFound` | 客戶不存在 |
| `ErrTagNotFound` | 標籤不存在 |
| `ErrTagAlreadyExists` | 標籤重複建立 |

**Repository → Domain 轉換**：`pgx.ErrNoRows` 轉為對應的 `Err*NotFound`。

**gRPC Handler 回應**：所有錯誤統一回傳 `codes.Internal`，並由 `ErrorInterceptor` 做最終處理。

---

## 10. 備註

- **Enrichment**：`nickname` / `avatar_key` 由單體 buyer 模組的 `BatchGetBuyerMeta` 即時取得（是 objectKey，不是 URL），`coupon_count` 由 coupon-service 取得。CRM 本地 DB 的 nickname／phone 只是搜尋用快取。
- **客戶統計更新**：由 CRM 自己消費 MQ 事件維護（見 §6.2），Order Service 不呼叫 CRM 推進統計。
- **`/customers/:buyerId/meta`**：直接查買家基本資訊，**不限該買家是不是這個賣家的 CRM 客戶**（用於優惠券指定買家等場景）。
