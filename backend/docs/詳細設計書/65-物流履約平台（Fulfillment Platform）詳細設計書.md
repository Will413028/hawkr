# 65　物流履約平台（Fulfillment Platform）詳細設計書

> **服務定位**：本書是物流履約領域「共用骨架」的現行規格，範圍只含下列四塊，各塊的權威實作都在 Java 服務：
>
> | 塊 | 服務 | 內容 |
> |---|---|---|
> | 訂單政策快照與收貨確認軸 | order-service | §1、§2 |
> | provider adapter 與動作可用性 | shipping-service | §3、§4 |
> | 責任判定案件（maker-checker）與事件 | payment-service、order-service | §5、§6 |
> | 帳本與 provider 帳單列 | payment-service | §7 |
>
> 配送單狀態機、PayUni 宅配與超商流程在 [06](06-物流微服務%EF%BC%88Shipping%20Service%EF%BC%89詳細設計書.md)；
> 賣家物流應收、收款、true-up 與分錄表在 [70](70-物流財務結算%EF%BC%88Logistics%20Settlement%20%26%20Receivable%EF%BC%89詳細設計書.md)。
> 事件契約與 broker 資源以 [36](36-MQ%20與事件總覽%EF%BC%88MQ%20%26%20Event%20Catalog%EF%BC%89.md) 與 `infra/mq/topology/topology.yaml` 為準。
> 碼值本體在 `../代碼管理（CodeMeta）/`。

| 配套文件 | 位置 |
|---|---|
| 外部設計 | `../基本設計書/65-物流履約平台（Fulfillment Platform）基本設計書.md` |
| 母系統 | `06-物流微服務（Shipping Service）詳細設計書.md` |
| 物流財務 | `70-物流財務結算（Logistics Settlement & Receivable）詳細設計書.md` |
| 發票鏈 | `58-電子發票服務（Invoice Service）詳細設計書.md` |

| 對應程式碼 | 位置 |
|---|---|
| 訂單政策快照 | `services/java/order-service/.../order/checkout/OrderPolicyResolver.java`、`OrderPolicySnapshot.java`、`db/migration/V14__add_order_policy_and_receipt_snapshot.sql`、`V16__drop_consignment_agreement_version.sql` |
| provider adapter | `services/java/shipping-service/.../shipping/provider/`（`FulfillmentProviderAdapter`、`FulfillmentProviderRegistry`、`FulfillmentContext`、`capability/*`、`payuni/*`） |
| 動作可用性 | `services/java/shipping-service/.../shipping/provider/action/`（`FulfillmentActionPolicy`、`AllowedActionResolver`、`ActionAvailability`） |
| 配送單擴充欄 | `services/java/shipping-service/src/main/resources/db/migration/V11__extend_shipments_for_fulfillment_platform.sql` |
| 責任判定 | `services/java/payment-service/.../payment/logistics/`（`application/ResponsibilityCaseService`、`domain/ResponsibilityCaseRules`、`adapter/grpc/FulfillmentResponsibilityGrpcService`、`mq/`）、`db/migration/V9`、`V12`～`V15` |
| 訂單終態驅動 | `services/java/order-service/.../order/mq/ResponsibilityFinalizedConsumer.java` |
| Admin 端點 | `gateway/internal/router/routes_logistics.go` |

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
| `fulfillment_mode` | 配送方式為 `3005` → `SELLER_MANAGED`；配送方式在平台履約白名單（`3001`、`3002`）→ `PLATFORM_LOGISTICS`；其餘（含 null、未知字串）→ 拋 `FULFILLMENT_COMMERCE_MODEL_UNKNOWN`（9504） |

- `PLATFORM_CONSIGNMENT`、`PLATFORM_AGENT_MERCHANT`、`PLATFORM_TO_BUYER`、`PROVIDER_COLLECTED`、`EXTERNAL_PREPAID` 是值域內的值，結帳與 RFQ 建單都不會寫入。啟用任一值的改動點是 `OrderPolicyResolver`，不是呼叫端。
- 平台履約白名單是具名集合（deny-by-default）：新增配送方式必須回到 `OrderPolicyResolver` 歸類，不得以「非 3005 即平台」推導。
- 快照在結帳 preview 階段決定，confirm 時隨訂單凍結；付款用的 MerID 與快照欄必須來自同一次推導。

### 1.3 付款 MerID：`CheckoutService.resolvePaymentMerId`

付款 MerID 由快照的 `collection_model` 決定，不讀 buyer request 的任何布林：

| `collection_model` | MerID |
|---|---|
| `SELLER_MERCHANT` | 該賣家的 `payuni_mer_id` |
| `PLATFORM_AGENT_MERCHANT` | 平台代理商戶 MerID；未設定 → `PLATFORM_COLLECTION_NOT_CONFIGURED`（9505） |
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

## 3. provider adapter（shipping-service）

### 3.1 硬規則

1. adapter 只做翻譯，不做業務判斷；「這個狀態要不要推進訂單」是狀態機的事。
2. 未知的 provider 原生值一律 fail-closed，落點是 `InboundCapability`。
3. adapter 不持有憑證，只拿到已解析好的 client。
4. PayUni 的 `LgsType`／`ShipType`／`GoodsType` 等原生 enum 不得越過 adapter 介面；capability 回正規化值加 `providerRawStatus` 字串。

### 3.2 `FulfillmentProviderAdapter` 與 capability

`FulfillmentProviderAdapter` 以 `supportedMethod()` 宣告服務哪一個配送方式，並以 `FulfillmentContext` 取得 capability；每個 getter 回 `Optional`：`Optional.empty()` 表示該 provider 不具備此能力，是可查詢的狀態，不是例外。

| getter | capability 介面 | 能力 |
|---|---|---|
| `create(ctx)` | `CreateCapability` | `createFromOrder(orderNo, orderInfo, onboardingSetting)`、`requiresOnboarding()` |
| `label(ctx)` | `LabelCapability` | `requestLabel`、`requestLabelBatch`、`batchConstraints()`、`usesThreeSegmentCode()`、標籤識別回查 |
| `dispatch(ctx)` | `DispatchCapability` | 叫件／派車 |
| `cancel(ctx)` | `CancelCapability` | `cancel(shipment, reason)` 回 `CancelOutcome(accepted, terminal, providerMessage)` |
| `track(ctx)` | `TrackCapability` | `query(shipment)` 回 `TrackResult(normalizedStatus, providerRawStatus, providerMessage, providerStatusAt)` |
| `returns(ctx)` | `ReturnCapability` | 逆物流建單 |
| `pod(ctx)` | `PodCapability` | 送達證明 |
| `onboard(ctx)` | `OnboardCapability` | provider 側建店／送審 |
| `inbound(ctx)` | `InboundCapability` | provider 回調的解析與正規化；`InboundPayload` 攜帶 raw bytes |
| `storeLocator(ctx)` | `StoreLocatorCapability` | 門市地圖（超商軸） |
| `cost(ctx)` | `CostCapability` | 這條 leg 已發生的物流成本 |

- 以上 getter 之外，adapter 另有 `requiresSellerConfirmation()`（預設 `false`）與 `pendingMigration()`。
- `cost(ctx)` 對不透過 API 提供成本的 provider 回 `Optional.empty()`，不得回 0：0 是合法金額，會讓賣家應收變成 0 且無訊號。
- `FulfillmentContext(storeId, method, shipment)`：`shipment` 可為 null（建店、買家選門市時尚無配送單）；capability 解析依 context，回傳值不得快取成 per-method 單例。`isReturnLeg()` 為 `shipment.isReturn`。
- adapter 不涵蓋：批次編排、provider 帳單匯入、賣家自送的服務範圍。

### 3.3 `FulfillmentProviderRegistry`

- 啟動時以 `supportedMethod().value()` 建索引；`supportedMethod()` 回 null 或兩個 adapter 宣告同一個配送方式 → `IllegalStateException`，啟動失敗。
- `require(method)`：查無 adapter → `FULFILLMENT_PROVIDER_NOT_ENABLED`（9503）。查無 adapter 是漏接線，與「provider 不支援某能力」（`Optional.empty()`）是兩件事。
- `find(method)` 只在缺席是合法情境時使用（報表投影、動作可用性計算）。沒有「找不到就回預設 adapter」的多載。
- 啟動 log 列出實際註冊的 `配送方式=adapter 類別簡名`。

### 3.4 PayUni adapter

| 配送方式 | adapter | 備註 |
|---|---|---|
| `3001` 黑貓宅配 | `PayUniHomeAdapter` | `label` 對已取號的單拋 `SHIPMENT_ALREADY_LABELED`（5003），標籤不可重印；沒有 `cancel` capability |
| `3002` 7-ELEVEN 店到店 | `PayUniCvsAdapter` | 標籤是三段碼，可冪等重取（`usesThreeSegmentCode() = true`）；沒有 `cancel` capability |

`AbstractPayUniAdapter` 提供兩者共用的 PayUni 呼叫與審計。PayUni 沒有 cancel 端點，兩者的取消都是只寫我方狀態的領域動作。

建單失敗時，adapter 落一列 `status = CREATE_FAILED(990)` 的 shell 配送單（帶 provider 狀態與訊息），再把原例外丟出。

---

## 4. 配送單擴充與動作可用性（shipping-service）

### 4.1 `shipments` 與 `shipment_status_logs` 擴充欄（`V11`）

| 欄位 | 語意 |
|---|---|
| `shipments.status_entered_at` TIMESTAMPTZ NOT NULL DEFAULT NOW() | 進入當前 `status` 的時間；aging 指標的唯一錨，不得用 `updated_at` |
| `shipments.provider_status` VARCHAR(32) | provider 原生狀態值；`shipment_status_logs.provider_status` 同寬 |
| `shipment_status_logs.subject_type` VARCHAR(24) NOT NULL DEFAULT `SHIPMENT` | CHECK `chk_ssl_subject`：`SHIPMENT`／`SHIPMENT_BATCH`；由 transition 函式填，呼叫端不得傳 |
| `shipment_status_logs.actor_type` VARCHAR(16) NOT NULL DEFAULT `SYSTEM` | CHECK `chk_ssl_actor`：`SYSTEM`／`ADMIN`／`SELLER`／`BUYER`／`PROVIDER` |
| `shipment_status_logs.actor_account_id` BIGINT | 操作者的 `account_id`；`SYSTEM`／`PROVIDER` 為 NULL |
| `shipment_status_logs.correlation_id` VARCHAR(64) | 跨服務鏈路追蹤，選填 |

- 任何寫 `shipments.status` 的 SQL 必須在同一句推進 `status_entered_at`：`status_entered_at = CASE WHEN status IS DISTINCT FROM <新值> THEN NOW() ELSE status_entered_at END`；終態 `11`、`41` 不再推進。
- `shipment_status_logs` 索引 `idx_status_logs_subject (subject_type, shipment_id, created_at)`。
- 只有 `subject_type = SHIPMENT` 會被寫入。

### 4.2 狀態映射

`ShipmentStatusMapper` 持有 PayUni 原生碼到正規化 `ShippingStatus` 的顯式映射表（20 個兩位數碼，恆等映射但必須存在），與狀態轉移的 allowed-from 判定。三位數狀態是平台自訂值，PayUni 不會回傳，不進映射表。provider 回傳映射表以外的值 → `SHIPMENT_PROVIDER_STATUS_UNKNOWN`（9526）。狀態轉移規則見 06。

### 4.3 動作可用性：兩層模型

對外 `allowedActions[]` 由兩層串接計算，順序固定：

1. **狀態閘門**：這顆動作在當前狀態能不能做。每個動作以「實際會把配送單推到哪個狀態，那條邊合不合法」判定，不另列第二份狀態白名單。不通過 → `enabled = false` 加狀態類原因碼。
2. **capability 閘門**：只在 `FulfillmentActionPolicy.requiresProviderCall(action, method)` 為 true 時才問 adapter；capability 為 `Optional.empty()` → `enabled = false`、原因碼 `FULFILLMENT_PROVIDER_NOT_ENABLED`（9503）。`requiresProviderCall = false` 的動作完全不受 capability 影響。

`ActionAvailability(action, enabled, disabledReasonCode, actorAccountTypes)` 的建構約束：`action` 非空；`actorAccountTypes` 非空；`enabled = false` 必須帶 `disabledReasonCode`。

`FulfillmentAction` 值域見 CodeMeta。`AllowedActionResolver` 計算的動作與順序固定為：`dispatch`、`handover`、`print_label`、`track`、`view_only`。

| 動作 | 狀態閘門 | 其他條件 |
|---|---|---|
| `dispatch` | 目標狀態 `TCAT_CALLED(210)` 的轉移合法；否則 `SHIPMENT_STATE_TRANSITION_REJECTED`（9520） | `requiresProviderCall` 為 true 時，標籤須已取號（`label_print_status = 4103`），否則 `SHIPMENT_LABEL_REQUIRED`（5011） |
| `handover` | 目標狀態 `SELLER_CONFIRMED(921)` 的轉移合法；否則 `SHIPMENT_HANDOVER_NOT_ALLOWED`（9527） | 標籤須已取號；動作在該配送方式不適用 → `SHIPMENT_PROVIDER_MISMATCH`（9528） |
| `print_label` | 配送狀態非終態，否則 `SHIPMENT_STATE_TRANSITION_REJECTED`（9520） | 列印狀態為處理中（4102）→ `SHIPMENT_PROVIDER_CALL_IN_PROGRESS`（5047）；無 provider 單號 → 9520；已取號（4103）且 adapter 不支援重印 → `SHIPMENT_ALREADY_LABELED`（5003） |
| `track`、`view_only` | 永遠可用 | 無 |

- 配送方式無法解析時，`resolve` 只回 `view_only`（不回空清單、不拋例外）；`require(action, shipment)` 對同一情境拋 `FULFILLMENT_PROVIDER_NOT_ENABLED`。
- `AllowedActionResolver.require(action, shipment)` 是執行面的守衛，與 `resolve` 共用同一份判定；`enabled = false` 時以 `disabledReasonCode` 拋 `ShippingBusinessException`。
- 計算的動作清單與 gateway 的 `actionRoute` 是兩份必須對齊的清單；多算的動作會被 gateway fail-closed 丟棄。
- 未列於 switch 的動作一律 `SHIPMENT_STATE_TRANSITION_REJECTED`（fail-closed）。

### 4.4 `FulfillmentActionPolicy`

| 表 | 規則 |
|---|---|
| `requiresProviderCall` | 預設為 true 的動作：`dispatch`、`cancel`、`print_label`、`reprint_label`、`track`、`requery`、`arrange_return`、`view_pod`、`update`、`resubmit`；其餘動作（`handover`、案件處置等）為 false。`OVERRIDES` 表可逐 (動作, 配送方式) 覆寫，只列與預設不同的格子 |
| `NOT_APPLICABLE` | 動作在該配送方式不存在，預設「適用」，只列不適用的格子；`handover@3001`（黑貓司機到府取件，沒有賣家確認出貨這一步） |
| `ACTORS` | 每個動作可由哪些 `AccountType` 執行；`dispatch`、`handover`、`print_label`、`reprint_label`、`arrange_return`、`update`、`resubmit`、`upload_pod`、`view_pod` 為賣家；`track`、`view_only` 為買家與賣家；未列出的動作歸 `ADMIN`（fail-closed） |
| `EXPLICIT_ALLOWED_FROM` | 逐 (動作, 配送方式) 的顯式 allowed-from；有宣告就是完整的狀態閘門，不再疊加其他狀態條件 |

---

## 5. 責任判定案件（payment-service）

配送失敗但成因證據不足時，由 payment-service 開一張責任判定案件，經 maker-checker 兩人程序定案後，以事件驅動訂單終態與賣家應收。

### 5.1 表

#### `fulfillment_exception_cases`（`V9`）

| 欄位 | 語意 |
|---|---|
| `id` BIGINT PK | TSID |
| `case_no` VARCHAR(32) UNIQUE | `EX` + `yyyyMMdd` + 8 位序號（`exception_case_no_seq`） |
| `source_service` | CHECK：`shipping`／`order`／`payment`；表達例外來源，不是表的擁有者 |
| `category` VARCHAR(48) | `FulfillmentExceptionCategory`：`RESPONSIBILITY_UNRESOLVED`、`LOGISTICS_STATEMENT_UNMATCHED`、`LOGISTICS_PROVIDER_COST_DISPUTE` |
| `severity` | CHECK：`P0`／`P1`／`P2` |
| `subject_type`、`subject_ref` | `subject_type` CHECK：`ORDER`／`SHIPMENT`／`SHIPMENT_BATCH`／`CONFIG` |
| `store_id` BIGINT | 賣家業務鍵 |
| `status` | CHECK：`OPEN`／`IN_PROGRESS`／`RESOLVED`／`DISMISSED` |
| `status_entered_at` | aging 指標的錨 |
| `owner_account_id`、`owner_assigned_at` | 承辦人 |
| `dedup_key` VARCHAR(200) | `{category}:{subject_type}:{subject_ref}` |
| `detail` JSONB（object）、`proposed_action_codes` JSONB（array） | 案件細節與建議動作 |
| `resolution_code`、`resolution_note`、`resolved_by_account_id`、`resolved_at` | 結案資訊 |
| `occurred_at` | 來源端發生時間，不與 `created_at` 混用 |
| `order_no` VARCHAR(32)、`shipment_id` BIGINT、`shipping_method` VARCHAR(8) | 事件脈絡，建案時凍結；責任判定案件的 `order_no` 與 `shipping_method` 不可為 NULL（`chk_exc_responsibility_context`）；索引 `idx_exc_order` |

約束：`chk_exc_resolved`（`RESOLVED`／`DISMISSED` 必須帶 `resolved_at`、`resolved_by_account_id`、`resolution_code`）、`chk_exc_owner`（`OPEN` 時無承辦人，其餘狀態必須有承辦人與指派時間）、`uq_exc_open_dedup`（`dedup_key` 在 `OPEN`／`IN_PROGRESS` 時唯一）。索引：`idx_exc_queue`（`severity, created_at`，條件為未結案）、`idx_exc_subject`、`idx_exc_aging`。

責任判定案件另有約束：同一 `(subject_type, subject_ref)` 終其一生只有一張（`uq_exc_responsibility_subject`，條件為 category 是 `RESPONSIBILITY_UNRESOLVED`）；不得 `DISMISSED`（`chk_exc_responsibility_no_dismiss`）；`RESOLVED` 時 `resolution_code` 必須是四個成因之一（`chk_exc_responsibility_resolved_cause`）。`order_no` 在建案後無更新路徑。

#### `fulfillment_case_resolutions`（`V9`）

| 欄位 | 語意 |
|---|---|
| `id` BIGINT PK | TSID，即對外的 `resolution_id` |
| `case_id` BIGINT FK | 指向 `fulfillment_exception_cases`，`ON DELETE RESTRICT` |
| `resolution_type` | CHECK：`CLAIM_SETTLE`／`CLAIM_DISMISS`／`SHIPMENT_CANCEL`／`SHIPMENT_CANCEL_CONFIRM`／`SHIPMENT_CANCEL_AND_REFUND`／`ANOMALY_RESUME`／`RESPONSIBILITY_ATTRIBUTE` |
| `case_category` VARCHAR(48) NOT NULL | 與 `case_id` 組成複合 FK `(case_id, case_category)` 指向案件的 `(id, category)`；`chk_fcr_responsibility_type_binding`：`case_category = RESPONSIBILITY_UNRESOLVED` 當且僅當 `resolution_type = RESPONSIBILITY_ATTRIBUTE` |
| `proposed_responsibility_cause` VARCHAR(48) | maker 主張的成因；`chk_fcr_proposed_cause_binding`：非 NULL 當且僅當 `resolution_type = RESPONSIBILITY_ATTRIBUTE`；`chk_fcr_proposed_cause_domain`：限四個成因 |
| `reason` TEXT NOT NULL | 非空 |
| `impact_preview` JSONB（object）、`impact_computed_at` | 影響預覽（退款金額）與其計算時間 |
| `requires_review` BOOLEAN | `= (resolution_type <> 'ANOMALY_RESUME')` |
| `status` | CHECK：`AWAITING_REVIEW`／`APPROVED`／`REJECTED` |
| `proposed_by_account_id`、`proposed_at` | maker |
| `reviewed_by_account_id`、`review_comment`、`reviewed_at` | checker |

約束：`chk_fcr_two_person`（覆核者不得為提案者）、`chk_fcr_decided_pair`（`APPROVED`／`REJECTED` 與 `reviewed_at` 同有同無）、`chk_fcr_reject_comment`（駁回必須有意見）、`uq_fcr_open_per_case`（每案件至多一筆 `AWAITING_REVIEW`）。

### 5.2 責任成因與案件規則（`ResponsibilityCaseRules`）

`ResponsibilityCause`（結案成因）四個值：`SELLER_CAUSED`、`BUYER_CAUSED`、`PLATFORM_CAUSED`（含系統原因）、`PROVIDER_CAUSED`；沒有 `SYSTEM_CAUSED`。證據不足時案件留在 `IN_PROGRESS`，不落任何成因。

| 規則 | 內容 |
|---|---|
| 案件識別 | `category = RESPONSIBILITY_UNRESOLVED`，`subject_type = SHIPMENT`，`dedup_key = RESPONSIBILITY_UNRESOLVED:SHIPMENT:{subject_ref}` |
| 唯一結案型別 | `RESPONSIBILITY_ATTRIBUTE`；`resolution_code` 必須是四個成因之一。責任判定案件不接受其他型別，其他 category 不接受 `RESPONSIBILITY_ATTRIBUTE` |
| 可操作狀態 | 案件 `OPEN` 或 `IN_PROGRESS` |
| 禁止 `DISMISSED` | 責任判定沒有「不處理」的結局；訂單停在 `SHIPPING_ABNORMAL(35)` 等待判定 |
| 提案 | 必須帶成因（四值之一）與非空 `reason`；影響預覽（退款金額）由伺服器依成因與 `orders.total_actual_amount` 計算並持久化；案件 `OPEN → IN_PROGRESS`，提案者成為承辦人 |
| 覆核 | 先 `SELECT … FOR UPDATE` 鎖同一張案件；覆核成因取自提案列的 `proposed_responsibility_cause`，不取自覆核者輸入；以當下的退款基準重算影響預覽，業務影響與持久化版本不一致 → `EXCEPTION_CASE_IMPACT_PREVIEW_STALE`（9575） |
| 駁回 | resolution → `REJECTED`，案件維持 `IN_PROGRESS`、承辦人保留，可再提新案 |
| 定案 | resolution `APPROVED` 與案件 `RESOLVED` 在同一交易；只有兩道 CAS 都 rows = 1 的交易才寫入 `fulfillment.responsibility.finalized` outbox |
| 副作用 | 本服務不推訂單狀態、不建撥款任務、不退款、不建應收；那些由 §6 事件的 consumer 在各自交易內處理 |
| 退款基準 | `fullRefundBasis`（`orders.total_actual_amount`）由交易外的 gRPC 取得，不接受呼叫端提供 |

`ResponsibilityCaseService` 提供四個方法：`openUnresolved`（建案，並寫 `fulfillment.responsibility.unresolved` outbox）、`propose`、`finalizeResponsibility`、`reject`。

### 5.3 Admin 端點

經 gateway `registerLogisticsFinanceRoutes`，掛在 admin group：

| 方法與路徑 | 作用 |
|---|---|
| `GET /api/v1/admin/logistics/exception_cases` | 案件列表；`awaiting_review_only=true` 只列有待覆核提案者 |
| `GET /api/v1/admin/logistics/exception_cases/{case_no}` | 案件詳情（含 resolutions） |
| `POST /api/v1/admin/logistics/exception_cases/{case_no}/resolutions` | maker 提案；必帶 `Idempotency-Key`（`^[A-Za-z0-9._:-]{8,128}$`） |
| `POST /api/v1/admin/logistics/exception_cases/{case_no}/resolutions/{resolution_id}/approvals` | checker 覆核（`decision` 為 `APPROVE` 或 `REJECT`）；必帶 `Idempotency-Key` |

後端由 `FulfillmentResponsibilityGrpcService` 承擔：

- 兩個寫入動作都要求 `AccountType.ADMIN`；操作者 `account_id` 取自認證 context，不接受 request 指定。
- 提案的 `resolution_type` 或案件 category 不是責任判定軸 → `EXCEPTION_CASE_STATE_INVALID`（9571）；`reason` 空白 → `INVALID_PARAM`。其餘 `resolution_type`（`CLAIM_*`、`SHIPMENT_CANCEL*`、`ANOMALY_RESUME`）沒有執行副作用的實作，一律拒絕。
- 提案回應的影響預覽取自持久化的那一列。
- 錯誤碼：`EXCEPTION_CASE_NOT_FOUND`（9570）、`EXCEPTION_CASE_STATE_INVALID`（9571）、`EXCEPTION_CASE_RESOLUTION_NOT_FOUND`（9572）、`EXCEPTION_CASE_REVIEW_SELF_APPROVAL_DENIED`（9573）、`EXCEPTION_CASE_RESOLUTION_ALREADY_REVIEWED`（9574）、`EXCEPTION_CASE_IMPACT_PREVIEW_STALE`（9575）、`FULFILLMENT_IDEMPOTENCY_PAYLOAD_MISMATCH`（9560）、`FULFILLMENT_OPERATION_IN_PROGRESS`（9561）。

---

## 6. 事件

事件走 exchange `hawkr.shipping`，producer 是 payment-service 的 `outbox_events`（exchange 為列上持久化的明示值）；payload 為 snake_case JSON（`FulfillmentResponsibilityFinalizedPayload`、`FulfillmentResponsibilityUnresolvedPayload`）。

| routing key | `FulfillmentEventType` | 說明 |
|---|---|---|
| `fulfillment.responsibility.unresolved` | `FULFILLMENT_RESPONSIBILITY_UNRESOLVED` | 配送失敗但成因證據不足，與建案同交易；不帶任何財務定案語意 |
| `fulfillment.responsibility.finalized` | `FULFILLMENT_RESPONSIBILITY_FINALIZED` | 與 resolution `APPROVED` 加案件 `RESOLVED` 同交易；`responsibility_cause` 只能是四個成因之一，`financially_final = true` |

`fulfillment.responsibility.finalized` 有兩條 queue，各自做不同的副作用，不合併：

| queue | consumer | 處置 |
|---|---|---|
| `order.fulfillment.responsibility_finalized`（DLX `hawkr.order.dlx`） | order-service `ResponsibilityFinalizedConsumer` | `BUYER_CAUSED`：單一交易內 `SHIPPING_ABNORMAL(35) → DELIVERY_FAILED_NO_REFUND(54)` CAS 並建 `grant_payment_tasks`；其餘三個成因不動訂單（走既有全額退款 closure，不得推進 54） |
| `payment.fulfillment.responsibility_finalized`（DLX `payment.events.dlx`） | payment-service `logistics.mq.ResponsibilityFinalizedConsumer` | 依成因決定該 leg 的賣家應收處置，規則見 70 |

order 側 consumer 的 fail-loud 規則：`order_no` 或 `responsibility_cause` 缺漏、成因不在四值內（含 `RESPONSIBILITY_UNRESOLVED`）、`financially_final` 不是布林 `true` → 進 DLQ，不得靜默 ACK。CAS `WHERE status = '35'` 影響 0 列時仍須驗證既有終態：`ALREADY_FINAL` → ACK；`INCONSISTENT` → DLQ。`54` 的唯一入邊是 `35 → 54`，唯一驅動者是本事件；order-service 沒有人工改狀態端點或補推排程。不得二次建立撥款任務。

---

## 7. 帳本與 provider 帳單列（payment-service，`V9`）

### 7.1 `ledger_transactions` 與 `ledger_entries`

雙分錄帳本，`ledger_transactions` 為 append-only（無 `updated_at`，零 UPDATE／DELETE）；更正一律開反向的 `MANUAL_ADJUSTMENT`。

`ledger_transactions`：

| 欄位 | 語意 |
|---|---|
| `txn_no` VARCHAR(32) UNIQUE | `LT` + `yyyyMMdd` + 8 位序號（`ledger_txn_no_seq`） |
| `txn_type` | CHECK 值域：`ORDER_ACCRUAL`、`REFUND_REVERSAL_GOODS`、`REFUND_REVERSAL_SHIPPING`、`PAYOUT_FEE`、`PLATFORM_SERVICE_FEE`、`HOLD_APPLY`、`HOLD_RELEASE`、`PAYOUT_SETTLE`、`MANUAL_ADJUSTMENT`、`LOGISTICS_COST_ACCRUAL`、`PROVIDER_SETTLEMENT`、`LOGISTICS_RECEIVABLE_ACCRUAL`、`LOGISTICS_COLLECTION_SETTLED`、`LOGISTICS_TRUE_UP`、`LOGISTICS_CREDIT_ISSUE`、`LOGISTICS_CREDIT_APPLY`、`COLLECTION_FEE_ACCRUAL` |
| `store_id` BIGINT（> 0） | 賣家業務鍵，不是 `account_id` |
| `order_no` | `MANUAL_ADJUSTMENT` 可為 NULL |
| `commerce_model` | CHECK：`PLATFORM_CONSIGNMENT`／`SELLER_DIRECT` |
| `settlement_period` CHAR(6) | `YYYYMM`（UTC 曆月） |
| `source_type` | CHECK：`ORDER`／`REFUND`／`PAYOUT`／`ADMIN` |
| `source_ref` VARCHAR(64) NOT NULL | 每筆分錄都指得回一張單據 |
| `idempotency_key` VARCHAR(160) UNIQUE | `{txn_type}:{source_type}:{source_ref}` |

`chk_ledgtxn_model_type`：`txn_type` 依 `commerce_model` 限縮——

| `commerce_model` | 允許的 `txn_type` |
|---|---|
| `SELLER_DIRECT` | `LOGISTICS_COST_ACCRUAL`、`PROVIDER_SETTLEMENT`、`LOGISTICS_RECEIVABLE_ACCRUAL`、`LOGISTICS_COLLECTION_SETTLED`、`LOGISTICS_TRUE_UP`、`LOGISTICS_CREDIT_ISSUE`、`LOGISTICS_CREDIT_APPLY`、`COLLECTION_FEE_ACCRUAL`、`MANUAL_ADJUSTMENT` |
| `PLATFORM_CONSIGNMENT` | `ORDER_ACCRUAL`、`REFUND_REVERSAL_GOODS`、`REFUND_REVERSAL_SHIPPING`、`PAYOUT_FEE`、`PLATFORM_SERVICE_FEE`、`HOLD_APPLY`、`HOLD_RELEASE`、`PAYOUT_SETTLE`、`MANUAL_ADJUSTMENT`、`LOGISTICS_COST_ACCRUAL`、`PROVIDER_SETTLEMENT` |

`ledger_entries`：

| 欄位 | 語意 |
|---|---|
| `txn_id` BIGINT FK | `ON DELETE RESTRICT`；`(txn_id, line_seq)` 唯一，`line_seq >= 1` |
| `account_code` | CodeMeta `LedgerAccountCode` |
| `owner_type`、`owner_ref_id` | CHECK：`PLATFORM` 時 `owner_ref_id = 0`；`SELLER`、`PROVIDER` 時 `> 0`（`SELLER` 是 `store_id`，`PROVIDER` 是配送方式碼值） |
| `direction` | CHECK：`DEBIT`／`CREDIT` |
| `amount` BIGINT | 元，整數，恆 `> 0` |
| `memo` VARCHAR(200) | 說明 |

借貸平衡由 `LedgerTxn` 在寫入前驗證（`LedgerValidationException`）。各 `txn_type` 的分錄對照見 70 §3.8。

### 7.2 `provider_settlement_lines`

provider 月結帳單的逐列匯入（`ProviderStatementImporter`，匯入者是 ADM）。

| 欄位 | 語意 |
|---|---|
| `provider` VARCHAR(8) | 配送方式碼值，與 ledger `owner_ref_id` 同口徑 |
| `statement_ref`、`statement_period`（`YYYY-MM`） | 帳單識別與期間 |
| `provider_shipment_ref` | provider 側單號；彙總列可為 NULL |
| `order_no` | 對應訂單 |
| `fee_type` | `ProviderFeeType`：`DELIVERY`、`CANCELLATION`、`WAITING`、`RETURN`、`ADJUSTMENT` |
| `amount` BIGINT | 元，恆 `> 0` |
| `recon_status` | CHECK：`UNMATCHED`／`MATCHED`／`DISPUTED`／`WRITTEN_OFF` |
| `ledger_txn_no` | `chk_provsettle_ledger`：`recon_status = MATCHED` 當且僅當 `ledger_txn_no IS NOT NULL` |
| `exception_case_id` | FK 指向 `fulfillment_exception_cases` |
| `imported_by` | ADM 的 `account_id` |

唯一鍵 `uq_provsettle_line (provider, statement_ref, provider_shipment_ref, fee_type)`。帳單列對不到應收時，開 `LOGISTICS_STATEMENT_UNMATCHED` 案件；`PROVIDER_CAUSED` 的超額成本開 `LOGISTICS_PROVIDER_COST_DISPUTE` 案件，不得自動向賣家收。對帳流程見 70 §6。

---

## 8. 錯誤碼

| 碼 | 名稱 | 觸發 |
|---|---|---|
| 9503 | `FULFILLMENT_PROVIDER_NOT_ENABLED` | registry 查無 adapter；動作的 capability 缺席 |
| 9504 | `FULFILLMENT_COMMERCE_MODEL_UNKNOWN` | 政策推導遇到 null、未知配送方式、未知 `collection_model` 或未知 RFQ lane |
| 9505 | `PLATFORM_COLLECTION_NOT_CONFIGURED` | `collection_model = PLATFORM_AGENT_MERCHANT` 但平台代理商戶 MerID 未設定 |
| 9520 | `SHIPMENT_STATE_TRANSITION_REJECTED` | 當前配送狀態不允許此操作 |
| 9526 | `SHIPMENT_PROVIDER_STATUS_UNKNOWN` | provider 回報映射表以外的狀態 |
| 9527 | `SHIPMENT_HANDOVER_NOT_ALLOWED` | 不可回報交運 |
| 9528 | `SHIPMENT_PROVIDER_MISMATCH` | 動作不適用於該配送方式 |
| 9560 | `FULFILLMENT_IDEMPOTENCY_PAYLOAD_MISMATCH` | 相同 `Idempotency-Key` 帶不同內容 |
| 9561 | `FULFILLMENT_OPERATION_IN_PROGRESS` | 相同操作處理中 |
| 9570～9575 | `EXCEPTION_CASE_*` | 見 §5.3 |

碼值本體以 CodeMeta 為準。
