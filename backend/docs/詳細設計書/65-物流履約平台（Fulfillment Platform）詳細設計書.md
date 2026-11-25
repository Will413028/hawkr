# 65　物流履約平台（Fulfillment Platform）詳細設計書

> **現況（B3，ADR `docs/adr/2026-11-15-fulfillment-seller-managed-shipping.md` 與其 2026-11-24 修訂）**：出貨只做賣家自寄。物流商的 provider adapter、配送單擴充、責任判定案件、帳本與物流商帳單已隨 Java shipping 與物流財務退場，本書只留 B8 之前仍存在於 Java order-service 的兩塊（§1 訂單政策快照、§2 收貨確認軸）；兩者在 B8 重新設計 ordering 時一併回答「沿用或重做」，本書不預先設計。
>
> | 塊 | 歸屬 | 內容 |
> |---|---|---|
> | 運費設定與報價 | seller 模組（B3） | 每個賣家一份：固定運費（TWD）、免運門檻（可空）、處理天數（1–7，預設 3 個工作日）；報價是純函式，cart 預覽與 ordering 結帳共用，結帳把結果快照進訂單；規格見 [21](21-運費計算模組%EF%BC%88Shipping%20Fee%20Module%EF%BC%89詳細設計書.md) |
> | 出貨記錄與含出貨的訂單狀態機 | ordering（B8） | 出貨記錄 0..N 筆（作品集先開放整單一筆）、逾期取消、自動完成、確認收貨、「未收到」回報；細節在 B8 計畫決定 |
> | 退貨物流 | aftersales（B9） | 買家自寄並回填追蹤號碼 |
> | §1、§2 | order-service（Java，到 B8） | 本書下列兩節 |
>
> 事件契約與 broker 資源以 [36](36-MQ%20與事件總覽%EF%BC%88MQ%20%26%20Event%20Catalog%EF%BC%89.md) 為準。碼值本體在 `../代碼管理（CodeMeta）/`。

| 配套文件 | 位置 |
|---|---|
| 外部設計 | `../基本設計書/65-物流履約平台（Fulfillment Platform）基本設計書.md` |
| 運費設定與報價 | `21-運費計算模組（Shipping Fee Module）詳細設計書.md` |
| 發票鏈 | `58-電子發票服務（Invoice Service）詳細設計書.md` |

| 對應程式碼 | 位置 |
|---|---|
| 訂單政策快照 | `services/java/order-service/.../order/checkout/OrderPolicyResolver.java`、`OrderPolicySnapshot.java`、`db/migration/V14__add_order_policy_and_receipt_snapshot.sql`、`V16__drop_consignment_agreement_version.sql` |

---

## 1. 訂單政策快照（order-service）

### 1.1 欄位

`orders` 在建單時寫入四個政策欄與一個版本欄，付款後不再變更，後續修改賣家設定不回溯既有訂單。

| 欄位 | 型別 | 值域（DB CHECK） | 語意 |
|---|---|---|---|
| `commerce_model` | VARCHAR(24) NOT NULL DEFAULT `SELLER_DIRECT` | `SELLER_DIRECT`／`PLATFORM_CONSIGNMENT` | 這筆錢誰收、誰開票 |
| `collection_model` | VARCHAR(32) NOT NULL DEFAULT `SELLER_MERCHANT` | `SELLER_MERCHANT`／`PLATFORM_AGENT_MERCHANT`／`PROVIDER_COLLECTED`／`EXTERNAL_PREPAID` | 款項實際經過誰的手，決定付款用的 PayUni MerID |
| `fulfillment_mode` | VARCHAR(24) NOT NULL DEFAULT `SELLER_MANAGED` | `SELLER_MANAGED`／`PLATFORM_LOGISTICS` | 誰負責把貨送到 |
| `invoice_model` | VARCHAR(24) NOT NULL DEFAULT `SELLER_TO_BUYER` | `SELLER_TO_BUYER`／`PLATFORM_TO_BUYER` | 買家發票由誰開 |
| `policy_version` | VARCHAR(32) NOT NULL DEFAULT `v0-legacy` | 無 | 政策版本；結帳寫入 `v1-foundation` |
| `seller_borne_discount_amount` | BIGINT NOT NULL DEFAULT 0 | `>= 0` | 賣家負擔的商品折扣（元） |

配送方式不另存政策欄：`orders.shipping_method` 即第五個維度。

組合約束（DB CHECK）：

| CHECK | 規則 |
|---|---|
| `chk_orders_policy_matrix` | `(commerce_model = PLATFORM_CONSIGNMENT) = (collection_model = PLATFORM_AGENT_MERCHANT)` |
| `chk_orders_invoice_collection` | `(invoice_model = PLATFORM_TO_BUYER) = (collection_model = PLATFORM_AGENT_MERCHANT)` |
| `chk_orders_fulfillment_mode_consistent` | `shipping_method <> '3005'` 或 `fulfillment_mode = SELLER_MANAGED` |

### 1.2 推導點：`OrderPolicyResolver`

快照由伺服器推導，呼叫端（含買家 request）不得指定任何政策欄。`OrderPolicyResolver.resolve(shippingMethod)` 是結帳路徑唯一的決定點：

| 欄位 | 結帳推導結果 |
|---|---|
| `commerce_model` | 恆為 `SELLER_DIRECT` |
| `collection_model` | 恆為 `SELLER_MERCHANT` |
| `invoice_model` | 恆為 `SELLER_TO_BUYER` |
| `policy_version` | `v1-foundation` |
| `fulfillment_mode` | 平台履約白名單（`3001`、`3002`、`3003`）與 `PLATFORM_LOGISTICS` 的推導隨 B3 刪除；每筆訂單都是賣家自寄（`SELLER_MANAGED`），對外契約不帶配送方式（B3 R2）。政策快照本身是否保留，B8 重新設計 ordering 時決定 |

- `PLATFORM_CONSIGNMENT`、`PLATFORM_AGENT_MERCHANT`、`PLATFORM_TO_BUYER`、`PROVIDER_COLLECTED`、`EXTERNAL_PREPAID` 是值域內的值，結帳與 RFQ 建單都不會寫入。啟用任一值的改動點是 `OrderPolicyResolver`，不是呼叫端。
- 快照在結帳 preview 階段決定，confirm 時隨訂單凍結；收款模式守門與快照欄必須來自同一次推導。

### 1.3 收款模式守門：`CheckoutService.requireSupportedCollectionModel`

結帳是否放行由快照的 `collection_model` 決定，不讀 buyer request 的任何布林；結帳不再決策或回傳商店代號：

| `collection_model` | 結果 |
|---|---|
| `SELLER_MERCHANT` | 放行 |
| `PLATFORM_AGENT_MERCHANT` | `PLATFORM_COLLECTION_NOT_CONFIGURED`（9505） |
| null、空白、`PROVIDER_COLLECTED`、`EXTERNAL_PREPAID`、其他字串 | `FULFILLMENT_COMMERCE_MODEL_UNKNOWN`（9504） |

判定窮舉，沒有 fall-through；失敗發生在結帳當下，不是建單之後。

### 1.4 RFQ（採購助理）訂單

`OrderService.createRfqOrder` 不呼叫 `OrderPolicyResolver`，直接寫入固定政策：

| 欄位 | 值 |
|---|---|
| `shipping_method` | `3005`（站外業者自行出貨，平台不安排物流） |
| `commerce_model`／`collection_model`／`invoice_model` | `SELLER_DIRECT`／`SELLER_MERCHANT`／`SELLER_TO_BUYER` |
| `fulfillment_mode` | `SELLER_MANAGED` |
| `policy_version` | `v1-foundation` |
| `shipping_fee_payer` | `BUYER`（運費已含在賣家報價的 `totalAmount`） |

- `lane` 不是 `PLATFORM` 或 `EXTERNAL` → `FULFILLMENT_COMMERCE_MODEL_UNKNOWN`（9504）。兩條 lane 的政策欄相同，差別只在賣方 `store_id`。
- 賣家沒有 PayUni MerID → `SELLER_NOT_ONBOARDED`。
- `acceptance_id`（RFQ 成交的 buyer acceptance id）有部分唯一索引 `uk_orders_acceptance_id`；撞鍵時回查既有訂單並視為冪等命中。

---

## 2. 收貨確認軸（E 軸，order-service）

收貨確認是獨立於 `orders.status` 的平行軸，`confirmed_at`／`confirm_type` 是它的附屬事實欄。

### 2.1 欄位與 CHECK

| 欄位 | 語意 |
|---|---|
| `receipt_status` VARCHAR(24) NULL | NULL 表示這條軸尚未開始，不是一個狀態值 |
| `receipt_status_entered_at` TIMESTAMPTZ | 進入當前 `receipt_status` 的時間 |
| `receipt_review_due_at` TIMESTAMPTZ | 人工裁決期限 |

| CHECK | 規則 |
|---|---|
| `chk_orders_receipt_status` | 值域 `PENDING`／`BUYER_CONFIRMED`／`AUTO_CONFIRMED`／`REVIEW_PENDING`／`ADMIN_CONFIRMED`／`ADMIN_REJECTED`／`DISPUTED` 或 NULL |
| `chk_orders_receipt_axis_present` | `delivered_at IS NULL` 或 `receipt_status IS NOT NULL` |
| `chk_orders_receipt_entered_pair` | `receipt_status` 與 `receipt_status_entered_at` 同為 NULL 或同為非 NULL |

索引：`idx_orders_receipt_review_due`（`receipt_review_due_at`，條件 `receipt_review_due_at IS NOT NULL AND receipt_status = 'REVIEW_PENDING'`）。

### 2.2 寫入者

| 時機 | 寫入 |
|---|---|
| 訂單進入已送達（`OrderRepository.updateOrderDelivered`） | 與 `delivered_at` 同一句 `receipt_status = COALESCE(receipt_status, PENDING)`、`receipt_status_entered_at = COALESCE(…, NOW())`；COALESCE 保證重複投遞不會把已前進的軸打回 `PENDING` |
| 買家手動確認（`confirm_type = MANUAL`） | `BUYER_CONFIRMED` |
| 逾期自動確認（`confirm_type = AUTO`） | `AUTO_CONFIRMED` |

- 新增 `confirm_type` 時必須同批決定它對應的 `ReceiptStatus`，`OrderRepository.receiptStatusFor` 對未知值拋例外。
- `REVIEW_PENDING`、`ADMIN_CONFIRMED`、`ADMIN_REJECTED`、`DISPUTED` 是值域內的值，目前沒有寫入者。
- 值域的權威是 CodeMeta `ReceiptStatus`，SQL 內不寫字面值。

---

## 3–8. 已退場（B3）

原 §3 provider adapter、§4 配送單擴充與動作可用性、§5 責任判定案件、§6 事件、§7 帳本與 provider 帳單列、§8 錯誤碼，隨 Java shipping 與物流財務刪除（ADR `docs/adr/2026-11-15-fulfillment-seller-managed-shipping.md`）：

- 出貨只有賣家自寄，沒有向物流商建單、取標籤與貨態 webhook，也就沒有 provider adapter 與動作可用性模型；物流 port 與模擬 adapter 延後到決定上線並簽約時。
- 責任判定案件與帳本隨 B2 的物流財務刪除（`5883f499e`）退場；平台不收也不負擔運費。
- 舊內容見 git 歷史。
