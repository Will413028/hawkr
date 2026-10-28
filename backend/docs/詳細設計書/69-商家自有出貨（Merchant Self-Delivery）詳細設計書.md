# 69　商家自有出貨（Merchant Self-Delivery）詳細設計書

> 共用規約（identity 鍵／金額口徑／時間口徑／命名／API envelope／錯誤碼號段／CodeMeta 流程／
> MQ 兩套 SSOT／排程器判準／新服務登記／媒體 objectKey）**只在 65 §3.9 定義一次，本書只引用**：
> §一 identity／§二 金額／§三 時間／§四 命名／§五 API／§六 錯誤碼／§七 CodeMeta／§八 MQ／
> §九 排程器／§十一 媒體（POD 影像）。
> （**不引用 §十 新服務**——本設計落在既有 `shipping-service`，見 §4.3）

---

## 0. 關鍵參數速查

> 🔴 **本節的每一個數字都要能回答「它的上下界從哪來」**。沒有推導的數字不放進本表。

### 0.1 三種成熟度（L0 Manual／L1 Webhook／L2 API）的能力矩陣與撥款證據要求

| 能力 | L0 Manual | L1 Webhook | L2 API |
|---|:---:|:---:|:---:|
| 賣家在我方 App／Web 手動推進貨態 | ✅ 唯一途徑 | ✅（降級時的退路） | ✅（同左） |
| 我方 outbound 通知商家（`order.ready_for_fulfillment`） | 🚫 無端點 | ✅ | ✅ |
| 商家 inbound 回報事件（驗簽＋去重＋順序合併） | 🚫 | ✅ | ✅ |
| 我方主動查詢商家（失聯回查） | 🚫 | 🚫 **對方沒有查詢端點** | ✅ |
| 動態報價（結帳熱路徑 quote） | 🚫 | 🚫 | ⚠️ **契約做進去、預設關閉**（§1.3） |
| 取消需商家 ACK | 🚫（同步生效） | ✅ | ✅ |
| POD（照片／簽收人／PIN） | ✅ 賣家上傳 | ✅ 商家帶 metadata | ✅ 同左 |

🔴 **撥款證據要求：三級完全相同——賣家（商家）單方回報一律不足以通過收貨 Gate**（§7.4）。
成熟度只影響「誰按下那個按鈕」與「多快知道」，**不影響證據的信任等級**。
理由見 §3.6 的第一條裁決：L2 的「carrier event」在本書的架構裡仍然是**商家自己轉述的**，
我方與承運商之間沒有任何直接通道 ⇒ 它與 L0 的「賣家自稱」在信任鏈上同源。

⚠️ **L1 沒有回查端點，這一格不是留白**：它決定 §7.6 的失聯處置在 L1 上只有一條路
（降級 L0 ＋ 建案件），不能寫成「回查後收斂」。

### 0.2 Deadline 與租約

| 參數 | 值 | 單位 | 出處／推導 | 上下界 |
|---|---|---|---|---|
| outbound 單次呼叫 deadline | `10`（connect 3 ＋ read 7） | 秒 | — | 🔴 **必須有值**：無 deadline ⇒ 單筆最壞耗時無上界 ⇒ 派送器的租約下界無從建立（65 §3.9 九） |
| outbound 派送器租約 | `60` | 秒 | ＝ 單次 deadline × 6 | 下界 > 10s（單筆最壞耗時）；上界 < 候選窗口（放棄門檻 24h） |
| outbound 放棄門檻 | `6` 次或 `24` 小時（先到者） | — | 退避階梯總和 ≈ 2h45m ⇒ 24h 內必然用完 6 次 | 觸發後 job 進 degraded（§7.5） |
| 商家接單 SLA | profile `accept_sla_minutes`，預設 `30` | 分 | 商家可在 profile 內下修，**不得上修超過 `120`** | 上界 120 分＝ `orders.processing_deadline` 的既有語意衝突點（§7.7） |
| 失聯回查首次觸發 | `max(2 × accept_sla, 30 分)` 自最後一則商家事件起 | 分 | — | 🔴 下界必須 > accept SLA，否則正常等待期就會被判失聯 |
| 失聯回查間隔 | `30` | 分 | 與 Uber 側的 5–10 分刻意不同：商家自送沒有外送員逐分變化的貨態 | 上界 < 回查窗口 24h |
| 失聯回查窗口 | `24` | 小時 | 逾時 ⇒ 建 case ＋ 降級 L0 | 🔴 **本設計自有推導，不對齊 65 的 `992` 逾時**（那是另一條軸的參數，跟著它變會讓本窗口被動漂移）：下界 > accept SLA 上限、上界 ＝ 賣家可容忍的人工介入延遲 |
| POD 上傳窗口 | `24` | 小時（自送達回報起） | 軟窗口：逾時只記 counter ＋ 通知賣家，**不改狀態** | 🔴 必須 < D+7，否則 ADM 裁決時證據還沒到 |
| secret rotation 重疊窗口 | 預設 `7`，最長 `30` | 天 | 商家換 key 需要人工作業窗口 | 下界 > 一個商家發版週期；上界＝舊 key 的曝險時間 |
| 商家 HMAC 時鐘容忍 | `±300` | 秒 | **沿用既有 `defaultTsWindowSeconds = 300`**（`pkg/middleware/grpc/interceptor/internal_auth.go`），不另訂 | — |

### 0.3 重試次數與退避階梯

**我方 outbound → 商家**：`0s / 30s / 2m / 10m / 30m / 2h`（共 6 次）。
每次都用**同一個 `eventId`**，商家端必須冪等（§6.2.2）。

**商家 inbound → 我方**：我方對可重試失敗回 **5xx**、對永久失敗回 **4xx**——
這個分野沿用既有 `proxy.FailThirdPartyWebhook`（502）的理由逐字：
「消費者是第三方而不是我們的前端：5xx 對它是『等一下重送』，4xx 是『別再送了』」。
⚠️ **驗簽失敗一律 4xx**（重送不會變成通過），**我方 DB 暫時不可用一律 5xx**。

### 0.4 secret rotation 參數

| 欄位 | 語意 |
|---|---|
| `credential_ciphertext` / `credential_fingerprint` / `credential_last4` | **current**（一定存在） |
| `pending_ciphertext` / `pending_fingerprint` / `pending_last4` | **next**（輪替中才存在，三欄同進同出） |
| `rotation_started_at` | 重疊窗口的起點 |
| `credential_key_version` | 對應 `MERCHANT_ENCRYPTION_MASTER_KEY` / `_PREV` |

驗簽時**先試 current、再試 pending**（兩把都比對，常數時間比較）。
這是既有 `verifyInternalAuth` 的形狀逐字搬過來：`hmacEqual(cfg.secret, …)` 不過才試 `cfg.secretPrev`。
🔴 **重疊窗口結束時把 pending 升為 current 並清空 pending 三欄**——不清空就是「兩把 key 永久有效」。

### 0.5 本書新增的碼值與錯誤碼速查

| 類別 | 內容 | 落點 |
|---|---|---|
| `ShippingMethod` | `MERCHANT_DELIVERY(3005)` | 65 附錄 C.1（本設計只引用，不自行裁決） |
| `ShippingType` | 🔴 **`MERCHANT_DELIVERY(3654)`**（值域 SSOT 在 65 附錄 C.1.1／C.2，本設計只引用；68 拿的是另一個值 `3653 CVS_B2C`，🔴 **兩個值不是一個**，見 §5.1.9） | 不擴充則 3005 一單都建不出來；**光加值也不夠**，還要有 `seller_shipment_settings` 的 `3654` 列（§5.1.9 (a)） |
| `MerchantDeliveryJobStatus` | **新 enum，8 值**（§7.3.1） | 附錄 C |
| `MerchantIntegrationTier` | `L0_MANUAL` / `L1_WEBHOOK` / `L2_API` | 附錄 C |
| `MerchantEventType` | 商家原生值（`ACCEPTED`／`PICKED_UP`／`IN_TRANSIT`／`DELIVERED`／`REJECTED`／`UNDELIVERABLE`／`RETURNED`／`CANCEL_ACK`） | 附錄 C（**照抄不改**，D-4） |
| `AssetReferenceType` | `MERCHANT_POD`（POD 影像，走 secure 前綴） | 附錄 C |
| `FulfillmentExceptionCategory` | 6 個商家整合類 category（§7.6） | 附錄 C |
| 業務碼 | **本書不自行取號**，語意清單見 §6.1.5／§6.2.7，號碼由 **65 §6.7 連號追加** | 65 §6.7.4 |

---

## 1. 概述與目標

### 1.1 文件定位（履約層橫向能力；與帳務軸正交，D-7）

本書描述**一種配送方式的實作**：賣家用自有車隊或自簽承運商把貨送到買家手上，
平台不介入實體配送，只負責①把訂單交到商家手上②接收商家回報③把回報翻譯成 65 §7.1.2 的 normalized 值
④在證據不足時**不讓錢出去**。

🔴 **本書與帳務軸正交**：它不決定誰開發票、錢收給誰、要不要撥款。
同一套商家自送能力會被兩本書引用——66（Uber 不可用時的替代方案）與 67（定期配送**唯一**的配送方式）——
而那兩本的 `commerce_model` 不同。**本書任何一節都不得寫入「因為是商家自送所以…撥款／發票…」**。

### 1.1a 先讀基本設計書

> **本書的基本設計書：[69 商家自有出貨基本設計書](../基本設計書/69-商家自有出貨（Merchant%20Self-Delivery）基本設計書.md)**
> ——三種成熟度的能力矩陣、賣家配送能力的三件事（服務範圍／時段／承載量）、
> 🔴 **收貨 Gate 的證據表**（賣家自稱送達不能單獨通過）、降級與恢復的不對稱、
> **錢與發票怎麼走**（配送成本是賣家自己的；平台不在線 B 也不在線 C 上）。

共用骨架另讀 [65 物流履約平台基本設計書](../基本設計書/65-物流履約平台（Fulfillment%20Platform）基本設計書.md)
（六個正交維度、跨配送方式共通規則、營運工作台）。

⚠️ **基本設計書不含任何可施工的細節**（無 DDL、無端點、無述詞）⇒ **不得拿它當契約來源**。

### 1.2 設計目標

1. 讓一個**沒有任何系統**的賣家（L0）今天就能用商家自送出貨，不需要任何整合工作。
2. 讓一個**有 ERP／WMS** 的商家（L1／L2）用一份公開契約接上來，**不需要我方為他寫程式**。
3. 商家整合壞掉時，**降級成 L0 而不是壞掉**——訂單照樣履約，只是回報變成人工。
4. 收貨與撥款的證據鏈**不因為配送是自送而變寬鬆**。

### 1.3 非目標

- **不做承運商整合**：我方不與黑貓／新竹／宅配通等直接介接。商家自己找誰送是商家的事。
- **不做路線最佳化、不做車隊管理、不做司機 App**。
- **不做多趟／拆單**（一張訂單一個 job）。
- **不為商家自送新增 normalized 配送值**（65 §7.1.2 的 31 值是唯一輸入）。
- ⚠️ **L2 動態報價：契約做進去、預設關閉。**
  `merchant_delivery_profile` 的計價欄位與端點形狀完整定義，但
  🔴 **`pricing_mode = DYNAMIC_QUOTE` 需 ADM 逐商家開啟**——quote 落在**結帳熱路徑**上，
  一個第三方 3 秒逾時會直接拖慢每一次結帳。
  這是擴張—收縮的擴張半，`REMOVE-WHEN` 承諾見 §12.2。

### 1.4 與其他設計書／服務的關係

| 對象 | 關係 |
|---|---|
| **65 骨幹** | 值域、identity、金額、時間、API envelope、錯誤碼、MQ、排程器、媒體規約的唯一來源。本書只引用 |
| **66 Uber** | 平行的 provider 實作；**互為替代方案**，不互相依賴 |
| **67 定期配送** | 🔴 **單向依賴本書**：67 的物流只有商家自送。67 讀本書的 `merchant_service_zone`／`_window`／`_capacity_bucket` 做方案發布前置驗證（§5.1.7） |
| **68 7-11 B2C** | 無關係。兩者唯一交集是都新增了 `ShippingMethod` 值 |
| **06 Shipping** | 本書是它的擴充：新增一個 **`FulfillmentProviderAdapter`** 實作（65 §4.4.1；能力集合逐成熟度不同，見 §4.2a）與一組 3005 專屬的服務層 |
| **09 售後** | 商家拒單／無法配送 → C 軸 `33` → 既有履約失敗鏈（65 §6A.1.2 N1）→ aftersales 退款 |
| **21 運費** | 3005 的運費**不走 `shipping_fee_tiers`**，走 profile 的計價設定（§5.1.9 有分派點） |
| **49 媒體** | POD 影像走 objectKey，**新 refType ＋ secure 前綴**（§8.6） |
| **跨本決策的 rationale** | 集中在 65 的稽核附錄，本書不重述；本書只寫**結果**（每條結果都在它自己的章節內） |

---

## 2. 範圍與情境

### 2.1 系統情境圖（C4 Context）

```
  買家 App/Web ─┐                                        ┌─ 商家 ERP / WMS（第三方）
                │                                        │      ▲          │
  賣家 App/Web ─┼──▶ Gateway（唯一 HTTP 入口）           │      │ outbound │ inbound
                │      │                                 │      │ webhook  │ 事件回報
  ADM ──────────┘      │ gRPC                            │      │          ▼
                       ▼                                 └──────┴──── /webhook/merchant_delivery/events
              shipping-service ──────────────────────────────────▶（HTTPS，商家自報 endpoint）
                 └ merchant-delivery 模組
                       │
                       ├─ gRPC ─▶ order-service（訂單資訊）／media-service（POD 影像）
                       └─ MQ  ──▶ hawkr.shipping（既有 3 條 key，payload 擴充）
```

🔴 **圖上沒有 carrier**：商家找誰送、司機是誰，我方一無所知。這不是省略，是 §3.1 的硬約束來源。

### 2.2 In-scope 範圍

三種成熟度**全做**，但**分期上線**（§12.2）：
① L0 先上（零整合，賣家在 App 手動推）→ ② L1（outbound ＋ inbound webhook）→ ③ L2（OAuth2 ＋ 主動查詢 ＋ quote 契約）。
分期的依據是**每一期都能獨立交付價值**：L0 上線當天賣家就能用商家自送出貨。

### 2.3 Out-of-scope

- 商家端的實作（我方只給契約與 sandbox）
- 逆向物流的商家整合（退貨走既有 aftersales 流程；商家自送的逆向在 65 §7.1.2.4 ⑤ 明列**不經過** `43`～`56`）
- 跨賣家的併單配送

### 2.4 角色與身分模型

| 角色 | 鍵 | 說明 |
|---|---|---|
| 賣家 | **`store_id`**（65 §3.9 一） | profile、job、憑證全部掛在 `store_id` |
| 買家 | **`buyer_id`** | 收件資訊來自訂單快照，不另存 |
| 登入 principal | `account_id` | 只用於 JWT 與通知投遞（`notifyBuyerAccountId`／`sellerAccountId`），**不得當業務鍵** |
| **商家 ERP** | `merchant_client_id`（本書新增） | 🔴 **它是第三方 client，不是內部服務** ⇒ **不得**使用 65 §5.2 的 internal HMAC（那是 service-to-service，密鑰全叢集共用一把）。兩套機制的密鑰粒度不同：internal 是**全叢集一把**，merchant 是**每商家一把** |

🔴 **`merchant_client_id` 一律解析成 `store_id` 之後才進業務層**。
任何 SQL 或 gRPC 的參數都不得直接帶 `merchant_client_id`——它是認證層的東西。

### 2.5 上線順序（3005 是純新增）

3005 沒有存量資料要處理——這一點與 66（Uber 有骨架、缺資料列）**不同**。

🔴 **上線順序受 65 §7.7.4.4 拘束，不得顛倒**：
先擴 `chk_shipping_method` CHECK ＋ CodeMeta → 再建 `shipping_method_configs` 列 → 才開 flag。
反過來做的話 DB CHECK 會先擋住寫入（`23514`），而**那個錯誤會被 outbox 靜默吞掉**。

---

## 3. 約束與前提

### 3.1 產品硬約束

🔴 **賣家自稱送達不能單獨通過收貨 Gate——L0／L1／L2 三級皆同。**

這條約束的技術根源寫在 §2.1 的圖上：**我方與承運商之間沒有任何直接通道**。
所有「送達了」的訊息，不論經過幾層 API、帶幾張照片，來源都是同一個利害關係人。
⇒ 收貨 Gate 只接受**買家確認**或 **ADM 裁決**（§7.4）。

### 3.2 商家能力約束

| # | 要求 | 適用 | 不滿足的後果 |
|---|---|---|---|
| M1 | endpoint 必須是 **HTTPS**，且**不得**解析到 private／loopback／link-local IP | L1／L2 | 🔴 **SSRF**：商家自報 endpoint 是使用者可控的出站目標。既有 `media-service-egress`／`product-import-egress`（issue #1696）就是為這個形狀而存在 |
| M2 | 能驗證 HMAC-SHA256 簽章（raw body） | L1／L2 | 無法確認訊息來自我方 |
| M3 | 能處理 secret rotation（同時接受兩把 key） | L1／L2 | 輪替時整段中斷 |
| M4 | **對同一個 `eventId` 冪等** | L1／L2 | 我方重試 6 次 ⇒ 商家出 6 次貨 |
| M5 | 回報事件帶**單調遞增的 `sequence`** | L1／L2 | 亂序事件無法合併（§5.3） |
| M6 | 提供 health check 端點（`GET`，回 200 即可） | L1／L2 | 失聯偵測只能靠業務事件的缺席，訊號延遲一整個 SLA |
| M7 | OAuth2 `client_credentials` ＋ 我方 rate limit ＋ 通過 sandbox 自測清單 | L2 | 不得升 L2 |

⚠️ **M1 的檢查點是「每次呼叫前重新解析並比對」不是「登記時檢查一次」**——
DNS rebinding 讓「登記時是公網 IP、呼叫時是內網 IP」完全可行。

### 3.3 資料治理約束

🔴 **溫層不能用未知值默認常溫。** profile 的溫層是**明列的集合**（`3201`／`3202`／`3203` 的子集），
空集合 ⇒ 該商家不可用（類 B，65 §7.7.4.1），**不是**「預設常溫」。
理由：默認常溫會讓冷凍商品被排進一台沒有冷凍設備的車，而那個錯誤在資料層完全看不出來。

**服務區資料要標來源**：`merchant_service_zone.source` ∈ `MERCHANT_DECLARED`／`ADM_VERIFIED`／`IMPORTED`。
67 的方案發布前置驗證會讀它，而「商家自己說的」與「營運核實過的」在爭議時的責任歸屬不同。

### 3.4 工程約束

→ 65 §3.9 一～九，逐條適用，本書不重述。特別要記住的三條：
金額對外一律 `string`；時間存傳 UTC、顯示層轉台北；狀態拼寫 `CANCELLED`（雙 L），商家原生值照抄。

### 3.5 假設

| # | 假設 | 若不成立 |
|---|---|---|
| A1 | 商家的配送範圍在台灣本島／離島，不跨境 | 服務區模型要加國別維度 |
| A2 | 一張訂單一個 job（不拆單） | `merchant_delivery_job` 與 `shipments` 的 1:1 要改成 1:N |
| A3 | 商家的 endpoint 數量級是「每個賣家一個」，不是「每個門市一個」 | profile 要加門市維度（67 的 capacity bucket 已預留 `site_ref`） |

### 3.6 三條本設計特有的判定

**① 🔴 L2 的「API event ＋ reconciliation」不得單獨成立為撥款證據。**

L2 的 carrier event **仍然是商家轉述的**（我方沒有與 carrier 的直接通道，§2.1）⇒
它與 L0 的「賣家自稱」在信任鏈上**同源**，只是延遲較短。
⇒ **L0／L1／L2 三級的收貨 Gate 規則完全相同**（§7.4），不得因為「有串 carrier」而放寬。

**② onboarding profile 八項的 owner 服務 → `shipping-service`。**

判準是「誰讀它」：profile 的消費者是①建單（`createShipmentFromOrder`）②費率計算
③結帳可用性驗證（`hasAnyEnabledConfig` 的 3005 等價物）④服務區／時段（67 讀）——
**全部在 shipping-service 內或以 shipping-service 為 gRPC 對端**。
單體 seller 模組持有的是店面身分，不持有配送能力。
⚠️ 🔴 **畫面歸屬 ≠ 資料歸屬**：profile 的**編輯畫面**在賣家後台、與 seller 的其他設定並列（65A），
但資料的 owner 仍是 shipping-service——**不得因為畫面在哪就把表建到 seller 模組**。

**③ 失聯回查的觸發條件與間隔見 §0.2。**

🔴 **與 Uber 側（5–10 分鐘）刻意不同，不得對齊**：Uber 有外送員逐分變化的位置，商家自送沒有；
用 5 分鐘輪詢一個一天只變 3 次的狀態，只會把商家的 rate limit 吃掉。

---

## 4. 架構

### 4.1 解法總綱（outbound 通知 ＋ inbound 事件 ＋ 失聯回查三條路）

```
① outbound：建單同 tx 落 merchant_outbound_delivery → 派送器（lease）→ 商家 endpoint
              └ 6 次退避後仍失敗 ⇒ 降級 L0 ＋ 建 case
② inbound： 商家 → Gateway /webhook/merchant_delivery/events → gRPC(raw bytes)
              → shipping-service：驗簽 → 去重 → 順序合併 → 推 job 軸 ＋ 推 C 軸
③ 回查：   最後一則事件靜默超過門檻 → 回查排程（lease，僅 L2）→ 收斂或建 case ＋ 降級
```

🔴 **三條路互為備援，但備援的終點只有一個：降級成 L0**。
**不得** fallback 到其他 provider（黑貓／超商／Uber）——那會換掉買家在結帳時看到的配送方式與運費。

### 4.2 元件視圖（C4 Component，全部在 shipping-service 內）

| 元件 | 職責 | 邊界 |
|---|---|---|
| `MerchantProfileService` | profile 的讀寫與**版本化**；對外只吐 `profile_version` | 唯一寫入路徑 |
| `MerchantCredentialService` | 憑證加解密、fingerprint、rotation 狀態機、**驗簽** | 🔴 **密鑰不出這個元件**：對外只有 `verify(keyId, rawBody, headers) → storeId` |
| `MerchantOutboundDispatcher` | 讀 `merchant_outbound_delivery`、打商家 endpoint、退避、放棄 | **lease**（65 §3.9 九：向外投遞一律不得 level-triggered） |
| `MerchantEventIngestor` | 驗簽 → 去重 → 順序合併 → 落 `merchant_event_inbox` → 推 job／C 軸 | 唯一 inbound 入口 |
| `MerchantReconcileScheduler` | 失聯回查（僅 L2）；收斂不了就發事件建 case | **lease** |
| `MerchantPodCollector` | POD 影像 finalize ＋ link；**fail-closed** | 見 §8.3 |
| `MerchantDeliveryAdapter` | 🔴 **`FulfillmentProviderAdapter` 的第 4 個實作** | 見 §4.2a：**能力集合隨成熟度而變**，不是固定的一組方法 |

🔴 **`MerchantDeliveryAdapter` 不是「只做建單」的策略物件。**
outbound 通知／inbound 事件／失聯回查／POD 抓取**一律掛在 adapter 的 capability 上**（§4.2a），
**不得**落成 `ShippingService` 上的具名方法——那會讓 3005 的行為散回 service 層，
與 65 §4.4.1 的 provider 抽象直接衝突。

🔴 **`MerchantEventIngestor` 推 C 軸時必須走 `ShippingService` 的中央狀態分派點**
（既有 private `updateStatus`，需開一個 package-visible 方法）——
那一句同時做①樂觀鎖 CAS ②`shipment_status_logs` ③三條 MQ 分支的選擇。
繞過它自己 UPDATE，就會出現「狀態變了但沒有人收到事件」。

### 4.2a 🔴 capability 對應

> 🔴 **本書是三本裡唯一「同一個 provider 有三組不同能力集合」的**——L0 Manual／L1 Webhook／L2 API。
> 這正是 65 §4.4.1b 那段紅字要求「**capability 的解析要帶 context，不能只看 `supportedMethod()`**」的來源：
> `3002`／`3003`／`3004` 的能力集合是 per-method 常數，**`3005` 是 per-store**（依 `merchant_delivery_profile.tier`）。
> ⇒ **adapter 的每個 getter 都接受解析 context，回傳值不得快取成 per-method 單例。**
> 少了這件，§0.1 的三成熟度矩陣在程式裡表達不出來，只能退回「在 service 裡 `if tier`」——🔴 **那是字面判斷，明令禁止**。

⚠️ **本書的「能力」一詞已有兩個既有含義**（§0.1 的成熟度能力矩陣、§6.1 表頭的 `CapOrderFulfill` ＝ 46 號賣家授權）
⇒ **provider capability 一律加前綴**，不得只寫「能力」。

**本 provider（`3005 MERCHANT`）的 provider capability 集合**（引自 65 §4.4.1b 那一欄，**不得在本書另寫一份**）：

| provider capability | L0 Manual | L1 Webhook | L2 API | adapter method | 依據（§0.1 對應列） |
|---|:---:|:---:|:---:|---|---|
| `create()` | ❌ | ❌ | ❌ | — | 三級皆無外部建單 API；**建單純本地**（見下方紅字） |
| `label()` | ❌ | ❌ | ❌ | — | 商家自送無標籤概念 |
| `dispatch()` | ❌ | ✅ | ✅ | `dispatch().notify(ctx)` | §0.1「我方 outbound 通知商家（`order.ready_for_fulfillment`）」：L0 🚫 無端點 |
| `cancel()` | ❌ | ✅ | ✅ | `cancel().request(ctx)` | §0.1「取消需商家 ACK」：**L0 是 🚫（同步生效）** |
| `track()` | ❌ | ❌ | ✅ | `track().fetch(ctx)` | §0.1「我方主動查詢商家（失聯回查）」：🔴 **L1 也是 🚫——對方沒有查詢端點** |
| `returns()` | ❌ | ❌ | ❌ | — | 三級皆無；退貨走 aftersales（§2.3 Out-of-scope） |
| `pod()` | ❌ | ✅ | ✅ | `pod().collect(ctx)` | §0.1「POD」：L0 是**賣家上傳**（我方端點，非 provider）；L1／L2 是商家帶 metadata ＋ 我方抓回自存 |
| `onboard()` | ❌ | ❌ | ❌ | — | profile 是**我方**資料，無外部 API 可問 |
| `inbound()` | ❌ | ✅ | ✅ | `inbound().verify(...)`／`parse(...)` | §0.1「商家 inbound 回報事件（驗簽＋去重＋順序合併）」 |
| `storeLocator()` | ❌ | ❌ | ❌ | — | 無門市概念 |

🔴 **`create()` 三級皆 `empty()`，但 3005 當然建得出單。**
這是 65 §4.4.1a 兩層模型最重要的一個實例，也是本表最容易被誤讀的一格：
`empty()` 說的是「**商家那邊沒有一支 API 讓我們建單**」，不是「這件事做不到」。
建單這個 domain action 的 `requiresProviderCall = false`，它只寫我方的 `shipments` 與 job 軸。

**每個 domain action 的 `requiresProviderCall`（逐成熟度，這一欄是本書與 66 形狀最不同的地方）**：

| domain action | `allowedActions[]` 出現在 | L0 | L1 | L2 | capability `empty()` 時的行為 |
|---|---|:---:|:---:|:---:|---|
| `handover` | job `ACCEPTED` | **false** | **false** | **false** | 🔴 三級皆為本地寫入（`M-2` ＝ 骨幹 `S-3`）⇒ **`dispatch()` 為 empty 完全不影響它** |
| `cancel` | job `ACCEPTED` | **false**（🔴 **L0 同步走 `41`**） | **true**（需商家 ACK） | **true** | L0：`cancel()` empty ＋ `requiresProviderCall=false` ⇒ **照常 enabled**；L1／L2：`cancel()` present ⇒ 打商家 → `CANCEL_ACK` → job `CANCELLED` ＋ C 軸 `41` |
| `report_progress`／`report_delivered`／`report_exception` | job `PICKED_UP`／`IN_TRANSIT` | **false** | **false** | **false** | 賣家在我方 App 回報（`M-3`／`M-4`／`M-5`）⇒ 與 provider 無關，三級恆 enabled（L1／L2 是降級退路） |
| `upload_pod` | job `DELIVERY_REPORTED` | **false**（賣家上傳） | **false**（商家已帶，賣家仍可補） | **false** | 🔴 **`pod()` 的 present／empty 不決定這顆按鈕**——它決定的是「我方要不要主動去抓」 |
| `admin_resolve` | job `REJECTED`／`UNDELIVERABLE`／`RETURNED` | **false** | **false** | **false** | ADM 介入，與 provider 無關 |

**逐操作的 endpoint 與 failure behavior**（🔴 **第三張表，缺它前兩張表對施工者不完整**——
capability 與 action 都列了，卻沒說「打哪一支、失敗怎麼辦」，那正是「留給 Coding AI 自己猜」）：

| domain action | 對外端點 | 適用成熟度 | 失敗行為（權威節） |
|---|---|:---:|---|
| `handover` | **`M-2` ＝ 骨幹 `S-3`** `POST /app/shipments/:order_no/handover`（`handoverType = SELF_DEPART` ⇒ C 軸 `930`）。🔴 **本書不另建** | L0／L1／L2 | `9527 SHIPMENT_HANDOVER_NOT_ALLOWED`（狀態不允許）。⚠️ `930` 屬 `ShipmentStatusMapper.IN_DELIVERY` ⇒ `930→31` 合法；🔴 **若 `930` 被移出該集合，`930→31` 會被拒且只有一行 `unknownTransition` warn**（§7.8 B1）——那是**靜默失效**，不是錯誤 |
| `report_progress` | `M-3` `POST /app/shipments/:order_no/merchant_delivery/progress` | L0／L1L2 降級 | `9520`（狀態不允許）；🔴 **必掛 `RequireIdempotencyKey()`**，而該 middleware 現況把重放碼寫死成**發票的兩個碼** ⇒ 物流端點會吐發票錯誤碼，修法在 65 §6.7 群 I（`9560`／`9561`），**列為上線前置** |
| `report_delivered` | `M-4` `POST /app/shipments/:order_no/merchant_delivery/delivered` | L0／L1L2 降級 | `MERCHANT_DELIVERY_POD_REQUIRED`（400）；🔴 `podPhotoKeys` **只收 objectKey**，驗「非 http 開頭、無 `..`、前綴白名單」，**禁收 URL 回存**。🔴 **只寫 C 軸 `11`，不碰 E 軸**（§7.4） |
| `report_exception` | `M-5` `POST /app/shipments/:order_no/merchant_delivery/exceptions` | L0／L1L2 降級 | C 軸 → `33`、job → `REJECTED`；🔴 `REJECTED` **不是終態**，aging > 2h ⇒ 退款鏈沒接上（§7.3.1） |
| `upload_pod` | **無獨立端點**——是 `M-4` 的 body 欄位（`podPhotoKeys`／`signerName`／`pinCode`） | L0／L1／L2 | POD 缺漏**不改變狀態**（§0.2 的 24h 是**軟窗口**：逾時只記 counter ＋ 通知賣家）。L1／L2 的抓取失敗另走 §8.3 **fail-closed**：重試 3 次 ＋ 建 case `MERCHANT_POD_MISSING` |
| `cancel` | 🔴 **本書沒有賣家側 cancel 端點**——L0 走**骨幹既有**的 `B-3 /app/orders/:order_no/cancellations`（買家）或 `S-4 /app/orders/:order_no/seller_cancellations`（賣家）；L1／L2 由 outbound `cancel.requested` ＋ 商家 `CANCEL_ACK` 事件推進；L2 另有**商家側** `§6.2.4 #5 POST /app/shipments/:order_no/merchant_delivery/cancel`（對第三方公開，不是賣家 UI） | L0 同步 `41`／L1L2 走 `992` | `9521`（已有處理中的取消）／`9522`（已無法取消）。🔴 **`cancel.requested` 的 outbound payload／ACK／退避定義必須與 `order.ready_for_fulfillment` 同形（§6.2.2）**——兩者共用同一條派送器與同一份退避階梯（§0.3），不得各寫一套 |
| `admin_resolve` | **骨幹 `A-4` → `A-5`**（maker-checker，65 §6.4.2） | L0／L1／L2 | `9571`（案件狀態已變更，請重新載入）／`9572`～`9575`（覆核群）。🔴 差異要寫進 case 的 `proposed_action_codes`：**拒單全額退款 vs 無法配送不自動退款**（§7.3.1） |

🔴 **`cancel` 那一列是本表最重要的一格**：§7.3.2 的 `allowedActions[]` 對 job `ACCEPTED` 列了 `cancel`，
而 §6.1 的 `M-1`～`M-6` **沒有任何一支 cancel 端點** ⇒ 若不指名骨幹的 `B-3`／`S-4`，
前端會拿到一個 `enabled=true` 卻**沒有 `href` 可給**的 action。
⚠️ 這正是 65 §6.2.1 硬約束 1 要防的形狀，只是方向相反：那條防「有 URL 但要前端自己推導」，
這裡是「有 action 但後端組不出 URL」。

⚠️ **§7.3.2 的 `allowedActions[]` 欄是以商家原生事件為列（隱含 L1／L2），對 L0 不適用**
L0 沒有商家原生事件 ⇒ **L0 的 `allowedActions[]` 由賣家在我方 App 的操作序推導**，
取值範圍與上表相同，只是入邊全部來自 `M-1`～`M-5`。

⚠️ **action 代號一律用 65 附錄 C.2 的 `FulfillmentAction` 值域**（`snake_case`），本書不再自列；
🔴 **`cancel` 這個值在本書與 66 是同一個 action，只是 `requiresProviderCall` 不同**——
🔴 **不得因為行為不同就另取名字**（例如 `cancel_request`）——action 值域是跨本共用的。

**取消走哪一條軸，依成熟度分流**（`992` 的適用欄是 `3002／3003／3004／3005（限 L1／L2）`，65 §7.1.2.1）：

- **L1／L2**：走 `992 CANCEL_PENDING`，出邊 ＝ 商家 `CANCEL_ACK` → job `CANCELLED` ＋ C 軸 `41`
- **L0**：**同步走 `41`**（純人工，沒有對象可等）

🔴 **L0 不得寫入 `992`**——那會造出一個沒有人推得走的狀態。

### 4.3 部署拓撲與 Port（🔴 本書不新增服務）

落在既有 `shipping-service`（**k8s 覆寫值** HTTP `9930` / gRPC `9931`；application 預設為 `9928`／`9929`，見 06 §9.1 —— 🔴 **兩個層級都要對得上**）。
完整推導見 **65 附錄 G.8**。三條旁證與那條反面論據（對外第三方 API 讓爆炸半徑變大）
的處置：**照 68 的 adapter 形狀做 package 邊界 ＋ 對外面走 Gateway namespace**。

**入站跨服務邊：無新增。** Gateway → shipping-service 的 gRPC 邊已存在
⇒ 65 §6.1.5 的三件（NetworkPolicy／env／註解）**不觸發**。

🔴 **出站邊有一條新的**：shipping-service → 商家 endpoint（公網）。
shipping-service **沒有 egress policy** ⇒ **SSRF 防護 100% 是應用層責任**（§8.1），
不得指望 NetworkPolicy 兜底。

### 4.4 命名／契約規約

→ 65 §3.9 四。本書只補一條：**商家事件名 `ACCEPTED`／`PICKED_UP`／`IN_TRANSIT`／`DELIVERED` 是 provider 原生值**，
照抄存 `merchant_delivery_job.provider_event`，**不進任何一條軸的值域**（D-4）。

### 4.5 對外契約的版本化策略

商家整合 API 走**擴張—收縮**（《DB Migration 相容性規範（expand-contract）》的形狀）：
- URL 帶版本：`/webhook/merchant_delivery/v1/events`
- 加欄位：不升版；刪欄位／改型別／改語意：升版並**同時提供 v1 與 v2**
- v1 的移除需要 `REMOVE-WHEN` 承諾（§12.2）：**所有 L1／L2 商家的 `api_version` 都已是 v2**，
  這個條件由 `merchant_delivery_profile.api_version` 的 `COUNT(*) WHERE api_version='v1'` = 0 機械判定

🔴 **不得以「商家很少」為由直接切**——外部整合的相容窗口不是我方能決定的。

### 4.6 靜默陷阱表

| # | 陷阱 | 症狀 | 防線 |
|---|---|---|---|
| A1 | 驗簽前就用 body 裡的 `eventId` 去重 | 攻擊者送一則假事件把真事件的 `eventId` 佔掉 ⇒ 真事件被當重複丟棄 | 🔴 **驗簽必須在去重之前**（§6.2.3 的順序是規範不是描述） |
| A2 | 驗簽前做過 JSON parse／re-serialize | 偶發失敗，失敗率取決於欄位順序 ⇒ 測不出來、上線才炸 | Gateway 以 `bytes` 原樣轉發（§6.2.3） |
| A3 | POD 影像處理沿用既有 `processShipmentPhotos` 的 catch-all | 該方法逐字「失敗不影響主流程，本次視為未附照片」⇒ **證據靜默消失** | POD 走**獨立的 fail-closed 路徑**（§8.3） |
| A4 | 3005 沒有加進 `createShipmentFromOrder` 的 Uber 豁免分支 | 該處逐字 `!UBER_INSTANT.equals(method) && onboardingSetting == null` ⇒ throw `SHIPMENT_SELLER_NOT_ONBOARDED`，**每一筆 3005 都建不出單** | §5.1.9 第 2 列 |
| A5 | job 軸與 C 軸各自更新 | 兩個真相，必漂移 | 同一交易由 `MerchantDeliveryService` 一次寫（INV-M1） |
| A6 | 商家 endpoint 用 IP 白名單而我方直連（來源 IP 不固定） | 商家端 403，我方看到的是「重試耗盡」 | 見 §8.1 的出站約束 |
| A7 | 動到 `SlotDef`／`buildSlotKey` 時把 `shippingType = null` 的槽位也導進 `settingMap.get(...)` 分支 | 查的是字面 `"null:"` ⇒ 在 `seller_shipment_settings` 永遠查不到 ⇒ `available=false`、`onboardingStatus=""`，**且不報錯** | 🔴 `3005` 的 `shippingType` 是 `3654`（查得到），**但同一支程式碼要同時服務 3003**——改動前先讀 [06 §5.13](./06-物流微服務（Shipping%20Service）詳細設計書.md) 的分流規格 |

---

## 5. 資料設計

> 每張表逐一回答三問：**誰寫入／誰讀取／誰把它推走或刪除**。

### 5.1 資料模型

#### 5.1.1 `merchant_delivery_profile` ＋ `merchant_delivery_profile_version`

🔴 **必須版本化**，理由是**訂單要快照它**：一筆訂單成立時的服務區、SLA、計價與證明要求，
必須在半年後的爭議裁決時還讀得回來，而 profile 是可編輯的。
⇒ `merchant_delivery_job.profile_version_id` 指向不可變的 version 列。

```sql
-- 主表：一個賣家一列（可為空＝尚未開通）
CREATE TABLE merchant_delivery_profile (
  id                      BIGINT PRIMARY KEY,              -- TSID
  store_id                BIGINT NOT NULL,
  integration_tier        VARCHAR(16) NOT NULL DEFAULT 'L0_MANUAL',
  status                  VARCHAR(16) NOT NULL DEFAULT 'DRAFT',  -- DRAFT/ACTIVE/SUSPENDED/CLOSED
  current_version_id      BIGINT,                          -- → _version.id
  api_version             VARCHAR(8) NOT NULL DEFAULT 'v1',
  integration_degraded_at TIMESTAMPTZ,                      -- 降級 L0 的起點（§7.5）
  integration_degraded_reason VARCHAR(48),
  health_check_next_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),  -- 🔴 租約欄，非事實時戳
  last_health_checked_at  TIMESTAMPTZ,                      -- 事實時戳
  created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_mdp_tier   CHECK (integration_tier IN ('L0_MANUAL','L1_WEBHOOK','L2_API')),
  CONSTRAINT chk_mdp_status CHECK (status IN ('DRAFT','ACTIVE','SUSPENDED','CLOSED')),
  -- ACTIVE 必須有版本；沒有版本的 ACTIVE 等於「開通了但不知道開通了什麼」
  CONSTRAINT chk_mdp_active_has_version CHECK (status <> 'ACTIVE' OR current_version_id IS NOT NULL)
);
CREATE UNIQUE INDEX uq_mdp_store ON merchant_delivery_profile(store_id);
CREATE INDEX idx_mdp_health ON merchant_delivery_profile(health_check_next_at)
  WHERE status = 'ACTIVE' AND integration_tier IN ('L1_WEBHOOK','L2_API');
```

⚠️ **`health_check_next_at` 與 `last_health_checked_at` 是兩欄不是一欄**——
這是 `invoice_provider_accounts` 逐字寫下的教訓（「租約欄是 `health_check_next_at`，
不是 `last_health_checked_at`」）。拿事實時戳當租約，等於用業務事實當搶佔憑據。

**`_version` 表逐節落欄（需求書 §4.1 的八項）**

| 需求書項 | 欄位 | 🔴 注意 |
|---|---|---|
| 服務區域 | `service_zone_mode`（`ZIPCODE`／`POLYGON`／`RADIUS`）＋ 明細在 `merchant_service_zone` | `source` 必填（§3.3） |
| 溫層 | `supported_temps JSONB`（`["3201","3203"]`） | 🔴 **空陣列＝不可用，不是常溫**；`CHECK (jsonb_array_length(supported_temps) > 0)` |
| 計價 | `pricing_mode`（`FLAT`／`DISTANCE`／`TIER`／`FREE`／`DYNAMIC_QUOTE`）＋ `pricing_params JSONB` ＋ `quote_endpoint` | 金額欄一律 `BIGINT` 元（65 §3.9 二）；`DYNAMIC_QUOTE` 需 ADM 開啟 |
| SLA | `accept_sla_minutes`／`prep_minutes`／`delivery_windows JSONB`／`blackout_dates JSONB` | `accept_sla_minutes ≤ 120`（§0.2） |
| 包裹限制 | `max_items`／`max_weight_grams`／`max_dimension_sum_cm`／`prohibited_categories JSONB` | `0` 表示不限（沿用 `shipping_method_configs` 的既有語意） |
| 聯絡與取件 | `pickup_address`／`pickup_lat`／`pickup_lng`／`contact_name`／`contact_phone_cipher` | 🔴 **電話加密存 ＋ log 遮罩**（§8.6） |
| 證明 | `pod_requirements JSONB`（`{"photo":true,"signature":false,"pin":true,"carrierEvent":false}`） | 🔴 這裡宣告的是**收集什麼**，不是**證據等級**——後者由 §7.4 固定 |
| 技術 | `endpoint_url`／`health_endpoint_url`／`key_id`／`sig_algo`／`retry_policy JSONB`／`timeout_ms` | `sig_algo` 目前唯一值 `HMAC-SHA256`，但**留欄位**（未知值 fail-closed） |

版本表本身 append-only：`INSERT` 新版 → 更新主表 `current_version_id`（同一交易）。
**誰刪除**：不刪。profile `CLOSED` 後版本列保留（爭議裁決要用）。

#### 5.1.2 `merchant_integration_credential`

**形狀逐欄鏡射既有 `invoice_provider_accounts`**（`services/invoice-service/db/schema/schema.sql`），
因為那張表已經把「密文 ＋ 版本 ＋ 指紋 ＋ 末四碼 ＋ 輪替三聯」的每一個坑踩過一遍。

```sql
CREATE TABLE merchant_integration_credential (
  id                     BIGINT PRIMARY KEY,
  store_id               BIGINT NOT NULL,
  key_id                 VARCHAR(32) NOT NULL,          -- 商家在 header 帶的公開識別字
  secret_ciphertext      TEXT NOT NULL,                 -- AES-256-GCM base64
  secret_key_version     SMALLINT NOT NULL DEFAULT 1,   -- 對應 MERCHANT_ENCRYPTION_MASTER_KEY / _PREV
  secret_fingerprint     VARCHAR(64) NOT NULL,          -- HMAC-SHA256 hex
  secret_last4           VARCHAR(4) NOT NULL,
  pending_ciphertext     TEXT,
  pending_fingerprint    VARCHAR(64),
  pending_last4          VARCHAR(4),
  rotation_started_at    TIMESTAMPTZ,
  rotation_deadline_at   TIMESTAMPTZ,                   -- 重疊窗口終點（§0.4）
  oauth_client_id        VARCHAR(64),                   -- L2 才有
  oauth_secret_ciphertext TEXT,
  credential_updated_at  TIMESTAMPTZ,                   -- 🔴 只有 Put/Rotate 落地才寫
  revoked_at             TIMESTAMPTZ,
  created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_mic_rotation_triplet CHECK (
      (pending_ciphertext IS NULL AND pending_fingerprint IS NULL AND pending_last4 IS NULL)
   OR (pending_ciphertext IS NOT NULL AND pending_fingerprint IS NOT NULL AND pending_last4 IS NOT NULL)),
  CONSTRAINT chk_mic_rotation_deadline CHECK (
      (pending_ciphertext IS NULL) = (rotation_deadline_at IS NULL))
);
CREATE UNIQUE INDEX uq_mic_key_id ON merchant_integration_credential(key_id) WHERE revoked_at IS NULL;
CREATE UNIQUE INDEX uq_mic_store  ON merchant_integration_credential(store_id) WHERE revoked_at IS NULL;
```

🔴 **`credential_updated_at` 是獨立欄不是 `updated_at`**：健康檢查每分鐘改寫整列，
拿 `updated_at` 下發給賣家看「金鑰更新於」等於這個訊號永遠是雜訊
（`invoice_provider_accounts` 的 `credential_updated_at` 註解逐字記載了這個事故）。

⚠️ **欄位級 AES-GCM 一律走 `common-core` 的共用 helper `FieldCipher`**，🔴 **不得各自實作**
（現行三處各有一份：`media-service` 的 `AesStreamCipher`、`payuni-java-client` 的
`PayUniCryptoService`、Go 的 `pkg/crypto`）。
`FieldCipher` 必須與 Go `pkg/crypto.Cipher` **逐位元組相容**（同 `InternalAuthSigner` 與
`internal_auth.go` 的兩語言互簽先例）。

#### 5.1.3 `merchant_delivery_job`

**與 `shipments` 一對一**（`shipment_id` UNIQUE），只在 `shipping_method='3005'` 時存在。

```sql
CREATE TABLE merchant_delivery_job (
  id                  BIGINT PRIMARY KEY,
  shipment_id         BIGINT NOT NULL,
  order_no            VARCHAR(64) NOT NULL,
  store_id            BIGINT NOT NULL,
  profile_version_id  BIGINT NOT NULL,               -- 🔴 快照，不是 FK 到可變的 profile
  integration_tier    VARCHAR(16) NOT NULL,          -- 建立當下的成熟度（會變，要快照）
  status              VARCHAR(24) NOT NULL,          -- MerchantDeliveryJobStatus（§7.3.1）
  status_entered_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  provider_event      VARCHAR(32),                   -- 商家原生值，照抄（D-4）
  provider_message    TEXT,
  event_seq_watermark BIGINT NOT NULL DEFAULT 0,     -- §5.3
  accept_due_at       TIMESTAMPTZ,                   -- 建立時 = now + accept_sla_minutes
  last_event_at       TIMESTAMPTZ,                   -- 失聯偵測的基準
  reconcile_next_at   TIMESTAMPTZ,                   -- 🔴 回查排程的租約欄
  pod_due_at          TIMESTAMPTZ,
  created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_mdj_status CHECK (status IN
    ('PENDING_DISPATCH','NOTIFIED','ACCEPTED','IN_FULFILLMENT',
     'DELIVERY_REPORTED','REJECTED','CANCELLED','CLOSED'))
);
CREATE UNIQUE INDEX uq_mdj_shipment ON merchant_delivery_job(shipment_id);
CREATE INDEX idx_mdj_reconcile ON merchant_delivery_job(reconcile_next_at)
  WHERE status IN ('NOTIFIED','ACCEPTED','IN_FULFILLMENT');
CREATE INDEX idx_mdj_aging ON merchant_delivery_job(status, status_entered_at);
```

🔴 **為什麼 job 軸與 C 軸是兩條軸而不是一條**（這一節必須說服人，否則就是第二個真相）：

| C 軸表達不了的事實 | 為什麼重要 |
|---|---|
| `NOTIFIED`（已送出、商家未 ACK）vs `ACCEPTED`（商家已接單） | C 軸兩者都是 `91`，而**推走者完全不同**：前者是 outbound 派送器，後者是接單逾時偵測 |
| `REJECTED`（商家拒單） | C 軸只有 `ABNORMAL(33)`，而 `33` 同時承載十幾種配送異常 ⇒ 用它表達拒單，ADM 佇列就分不出「還沒出門」與「路上出事」 |
| `event_seq_watermark` | 純整合層事實，不屬於貨態 |

⚠️ **代價：兩條軸必須永不矛盾。** 由 INV-M1 保證（§5.5）：兩者**只由 `MerchantDeliveryService`
在同一交易寫入**，且 job → C 的映射是**全函式**（§7.3.2）。
🔴 **降級不是 job 狀態**：它是 `merchant_delivery_profile.integration_degraded_at`。
把降級做成 job 的一個值，映射就不再是全函式（降級時 C 軸可以停在任何值），
而不是全函式的映射沒有辦法寫成可機械檢查的斷言。

#### 5.1.4 `merchant_event_inbox`

```sql
CREATE TABLE merchant_event_inbox (
  id             BIGSERIAL PRIMARY KEY,
  store_id       BIGINT NOT NULL,                   -- 🔴 驗簽後才填
  event_key      VARCHAR(128) NOT NULL,             -- 商家的 eventId
  event_type     VARCHAR(32) NOT NULL,
  event_seq      BIGINT,
  shipment_id    BIGINT,
  raw_payload    JSONB NOT NULL,                    -- 遮罩後保存（§8.6）
  signature_ok   BOOLEAN NOT NULL,
  process_status VARCHAR(16) NOT NULL,              -- ACCEPTED/DUPLICATE/OUT_OF_ORDER/REJECTED
  process_note   TEXT,
  received_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT uq_mei_event UNIQUE (store_id, event_key)
);
CREATE INDEX idx_mei_cleanup ON merchant_event_inbox(received_at);
```

🔴 **去重鍵是 `(store_id, event_key)` 不是 `event_key`**：`event_key` 由商家自訂，
兩個商家撞號完全可能，而撞號的症狀是「B 商家的事件把 A 商家的吃掉」——靜默。
`store_id` 只有在驗簽通過後才知道 ⇒ **這條 UNIQUE 本身就要求驗簽先於去重**（§4.6 A1）。

**誰刪除**：`MerchantInboxCleanupScheduler`（GC，非 claim 形狀），
保留期 `MERCHANT_INBOX_RETENTION_DAYS`（預設 30，沿用既有 `DedupCleanupScheduler` 的形狀但保留期較長——
爭議窗口是 D+7 ＋ 人工，7 天不夠）。

#### 5.1.5 `merchant_outbound_delivery`

```sql
CREATE TABLE merchant_outbound_delivery (
  id              BIGINT PRIMARY KEY,
  store_id        BIGINT NOT NULL,
  shipment_id     BIGINT NOT NULL,
  event_id        VARCHAR(64) NOT NULL,             -- 我方產生的 UUID v4，重試時不變
  event_type      VARCHAR(32) NOT NULL,             -- order.ready_for_fulfillment / cancel.requested
  payload         JSONB NOT NULL,
  status          VARCHAR(16) NOT NULL DEFAULT 'PENDING',  -- PENDING/DELIVERED/ABANDONED
  attempt_count   SMALLINT NOT NULL DEFAULT 0,
  next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),      -- 🔴 租約欄兼退避欄
  last_http_status SMALLINT,
  last_error      TEXT,
  delivered_at    TIMESTAMPTZ,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT uq_mod_event UNIQUE (event_id),
  CONSTRAINT chk_mod_status CHECK (status IN ('PENDING','DELIVERED','ABANDONED'))
);
CREATE INDEX idx_mod_due ON merchant_outbound_delivery(next_attempt_at) WHERE status = 'PENDING';
```

**誰寫入**：`createShipmentFromOrder` 的 3005 分支，**與 `shipments` INSERT 同一交易**（例外 A）。
不同交易就會出現「單建了但通知從未產生」，而那是完全靜默的。

#### 5.1.6 `merchant_pod_artifact`

```sql
CREATE TABLE merchant_pod_artifact (
  id                BIGINT PRIMARY KEY,
  job_id            BIGINT NOT NULL,
  kind              VARCHAR(16) NOT NULL,   -- PHOTO/SIGNATURE/PIN/CARRIER_EVENT/TRACKING
  object_key        VARCHAR(512),           -- 🔴 只存 objectKey，禁 *_url 名存 key 值
  asset_id          BIGINT,
  tracking_no       VARCHAR(64),
  signer_name_cipher TEXT,                  -- 簽收人姓名（PII，加密）
  pin_verified      BOOLEAN,
  carrier_event_raw JSONB,
  source            VARCHAR(16) NOT NULL,   -- MERCHANT_EVENT / SELLER_UPLOAD / ADM
  collected_at      TIMESTAMPTZ NOT NULL,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_mpa_kind CHECK (kind IN ('PHOTO','SIGNATURE','PIN','CARRIER_EVENT','TRACKING')),
  CONSTRAINT chk_mpa_photo_key CHECK (kind <> 'PHOTO' OR object_key IS NOT NULL)
);
CREATE INDEX idx_mpa_job ON merchant_pod_artifact(job_id, kind);
```

🔴 **`source` 欄不是裝飾**：ADM 在 D+7 裁決時要知道這張照片是商家 API 送的、賣家在 App 補傳的、
還是營運自己補的。三者的證明力不同，而 §7.4 的表就是按這個分的。

#### 5.1.7 `merchant_service_zone` ／ `merchant_service_window` ／ `merchant_capacity_bucket`

🔴 **67 直接引用這三張表**（方案發布前置驗證），所以它們的形狀是**跨本契約**，改動要通知 67。

```sql
CREATE TABLE merchant_service_zone (
  id BIGINT PRIMARY KEY, profile_version_id BIGINT NOT NULL,
  zone_type VARCHAR(16) NOT NULL,          -- ZIPCODE/POLYGON/RADIUS
  zipcode VARCHAR(10), polygon_geojson JSONB, center_lat NUMERIC(9,6), center_lng NUMERIC(9,6), radius_m INT,
  source VARCHAR(16) NOT NULL,             -- MERCHANT_DECLARED/ADM_VERIFIED/IMPORTED（§3.3）
  CONSTRAINT chk_msz_type CHECK (zone_type IN ('ZIPCODE','POLYGON','RADIUS')),
  CONSTRAINT chk_msz_source CHECK (source IN ('MERCHANT_DECLARED','ADM_VERIFIED','IMPORTED'))
);
CREATE TABLE merchant_service_window (
  id BIGINT PRIMARY KEY, profile_version_id BIGINT NOT NULL,
  day_of_week SMALLINT NOT NULL,           -- 1..7（ISO）
  start_minute SMALLINT NOT NULL, end_minute SMALLINT NOT NULL,   -- 自當地 00:00 起的分鐘
  CONSTRAINT chk_msw_dow CHECK (day_of_week BETWEEN 1 AND 7),
  CONSTRAINT chk_msw_range CHECK (start_minute < end_minute)
);
CREATE TABLE merchant_capacity_bucket (
  id BIGINT PRIMARY KEY, store_id BIGINT NOT NULL,
  bucket_date DATE NOT NULL, window_id BIGINT NOT NULL, site_ref VARCHAR(32),
  capacity INT NOT NULL, reserved INT NOT NULL DEFAULT 0,
  CONSTRAINT chk_mcb_reserved CHECK (reserved >= 0 AND reserved <= capacity)
);
CREATE UNIQUE INDEX uq_mcb ON merchant_capacity_bucket(store_id, bucket_date, window_id, COALESCE(site_ref,''));
```

⚠️ **`start_minute`／`end_minute` 是「當地分鐘」不是 UTC**——配送窗是**營業時間語意**，
跨日光節約或跨時區都不改「早上 9 點到 12 點」這件事。
🔴 但**寫進 job 的 `accept_due_at` 是 UTC 絕對時間**（65 §3.9 三）。兩者的換算點只有一個：
建立 job 的那一句。

#### 5.1.8 `merchant_reconciliation_case`

🔴 **本表不建。**（本書對骨架的修正）

65 §5.5 已經裁決異常案件是**一張表**（`fulfillment_exception_cases`，
來源服務發事件、由該表的 owner 建案），理由逐字是「ADM 佇列要跨來源排序與分頁」。
該表由 **payment-service 的 Logistics Finance module** 持有，DDL 在
`services/java/payment-service/src/main/resources/db/migration/V9__add_logistics_finance_tables.sql`。

🔴 **不得再開一張商家專屬的案件表**——那正是 65 §5.5 否決的形狀。

⇒ 本書的做法：**沿用 `fulfillment_exception_cases`**，只新增 `category` 值（附錄 C）。
既有 CHECK **不需要擴充**：`source_service` 已含 `'shipping'`，
`subject_type` 用既有的 `'SHIPMENT'`（單筆問題）與 `'CONFIG'`（整合層問題）即可涵蓋。

**同時必辦兩件（65 §5.5 逐字的「兩件事不是一件」）**：
① 來源側 counter（同步、必留，不依賴事件送達）②案件（非同步，走 outbox）。
只做②＝把偵測搬到一條會掉東西的路徑上。

#### 5.1.9 既有表擴充

| # | 動作 | 漏了會怎樣 |
|---|---|---|
| 1 | 🔴 **本書不做 `chk_shipping_method`**（免除聲明，見下方 M6-1） | —（由 65 的 `V11` 負責；本書重做會刷掉它） |
| 1b | 🔴 **`chk_shipping_type` 擴到含 `'3654'`**（migration 取號規則見 §5.4：🔴 **取號前現查目錄**，不得在本書寫死版號） | `seller_shipment_settings` 的 `3654` 列寫入被 DB 拒絕（`23514`）⇒ §5.1.9 (a) 落不了地 ⇒ **3005 照樣一單都建不出來**。⚠️ 現行 `V1` 逐字 `CHECK (shipping_type IN ('3651','3652'))` |
| 2 | 🔴 `ShippingService.createShipmentFromOrder` 的建店豁免分支加 3005 | 該處逐字 `!ShippingMethod.UBER_INSTANT.value().equals(shippingMethod) && onboardingSetting == null` ⇒ **每一筆 3005 都拋 `SHIPMENT_SELLER_NOT_ONBOARDED`** |
| 3 | adapter registry 新增 `MerchantDeliveryAdapter`（`supportedMethod()` 回 `3005`） | 查無 adapter ⇒ `SHIPMENT_UNSUPPORTED_METHOD` |
| 4 | `shipping_method_configs` 新增 3 列（3005 × `3201`／`3202`／`3203`），🔴 **建立時 `enabled = false`**（65 §5.7；上線那一刻才切 `true`，見 §12.2 ⑥）。⚠️ **這一層今天還不成立，前置在 65 §5.7**：`ShippingFeeService` 載入設定快取時帶 `enabled = true` 的 filter ⇒ `enabled=false` 的列在快取裡與「不存在」完全一樣 ⇒ **那一行沒改，本層就退化成「不建列」而且看不出來**（落類 B 而不是類 A） | 三列不建 ⇒ `hasAnyEnabledConfig("3005")` 恆 false ⇒ 賣家永遠看不到這個槽位（且是靜默的，D-13 的原始形狀）。⚠️ 建了但沒切 `true` 時**畫面上一樣**，差別在落**類 B**（缺列＝配置缺陷，賣家已 onboarding 時還會建案件）或**類 A**（規則查得到、說現在不可用）——65 §5.7 逐字要的是後者 |
| 5 | `OnboardingService.SLOT_DEFINITIONS` 新增第 5 槽 | 賣家後台沒有這個選項 |
| 6 | `ShippingFeeService.lookupFee` 對 3005 的分派 | 落回 `shipping_fee_tiers` 查無 ⇒ 類 B「暫不可用」（**至少不是算錯錢**，但賣家會以為系統壞了） |

##### 🔴 M6-1｜`chk_shipping_method`：本設計**不得** `DROP/ADD`

`shipments.chk_shipping_method` 的擁有者是 **65**（它已一次擴成 5 值
`'3001','3002','3003','3004','3005'`）。

🔴 **CHECK 約束的擁有者只能有一個。** 本設計的 migration 再 `DROP/ADD` 一次
就會把前者的成果刷掉（**後跑的贏**）——寫成 3 值的話 `3004` 會被**靜默移除**，
而沒有任何東西會把它加回來，症狀是 `3004` 的每一筆寫入回 `23514` 並被 outbox 靜默吞掉。

##### 🔴 M6-2｜`chk_shipping_type`：本書**要做**，且 68 的 `V14` 必須是超集

與 `chk_shipping_method` 相反，`seller_shipment_settings.chk_shipping_type`
**必須由本設計擴到含 `'3654'`**（65 只指派值、不改這個約束）：

```sql
ALTER TABLE seller_shipment_settings DROP CONSTRAINT chk_shipping_type;
ALTER TABLE seller_shipment_settings ADD  CONSTRAINT chk_shipping_type
    CHECK (shipping_type IN ('3651', '3652', '3654'));   -- 3654 = MERCHANT_DELIVERY
```

🔴 **同一個約束若被多份 migration 各 `DROP/ADD` 一次，後跑的那一份必須是所有值的超集。**
68 的 `'3653'` 與本設計的 `'3654'` 都掛在這個約束上 ⇒ 後跑者漏掉任一個值，
就會把先跑者的成果刷掉，而症狀是**那一類的列從此再也寫不進去**，
且**不會有任何編譯錯或測試紅**（DB 回 `23514`，被 outbox 靜默吞掉）。

**🔴 `ShippingType.MERCHANT_DELIVERY`，Code ＝ `3654`**（65 附錄 C.1.1／C.2）。

> 🔴 **值域是骨幹的 SSOT，本設計只引用不得自行裁決**（同 65 §3.9「只准引用不得各寫一份」）。
> 🔴 **`3653 CVS_B2C`（68）與 `3654 MERCHANT_DELIVERY`（本設計）是兩個不同的值。**
> 兩者**不得共用同一個值**——共用的話 `buildSlotKey` 的第一段會相同，正好重演它要避免的撞號。
> 共用的是「**同一次 `業務類型碼.md` 變更與同一次 `generate.sh --all`**」，不是同一個值。

🔴 **沒有 `3654` 這個值，3005 會一單都建不出來**，而且是在走完上線順序之後才發現：
- `OnboardingService.mapShippingMethodToType` 逐字只認 `SEVEN_ELEVEN_STORE_PICKUP` → `SHIPPING_TYPE_CVS`、
  `TAQBIN` → `SHIPPING_TYPE_HOME`，**其餘 `return null`**
- `findActiveSetting` 拿到 null 就 `return null`
- `ShippingService.createShipmentFromOrder` 對「非 `UBER_INSTANT` 且 `onboardingSetting == null`」
  直接 `throw ShippingBusinessException(SHIPMENT_SELLER_NOT_ONBOARDED)` ——**3005 不是 3003，豁免不成立**

⚠️ **也不能硬塞進 `CVS_C2C`**：該值的 description 逐字是「超商 C2C 取貨流程」，
商家自送**兩類都不屬於**，塞進去會讓前端分組錯誤，還會與 68 的 B2C 兩槽**撞 slot key**
（`buildSlotKey(shippingType, goodsType)`，而 `Collectors.toMap` 的合併函式是 `(a,b)->a`「重複取第一筆」⇒ 靜默取到別人的設定）。

**⇒ 上線順序的第 0 步**：擴 `ShippingType`（＋`3654 MERCHANT_DELIVERY`）
＋ `mapShippingMethodToType` 對 `3005` 回 `SHIPPING_TYPE_MERCHANT`
＋ `SLOT_DEFINITIONS` 加槽：

```java
new SlotDef(ShippingMethod.MERCHANT_DELIVERY.value(), null,
            "商家自行配送", "賣家自有車隊或承運商", true, SHIPPING_TYPE_MERCHANT)
```

🔴 **`onboardingRequired = true`**（第 5 個參數）。
做完第 0 步才輪到「擴 CHECK ＋ CodeMeta → 建 config 列 → 開 flag」。
🔴 **必須同批補一支「3005 建單成功」的測試**——少了它，這個缺口會在 CI 全綠下流出。

🔴 **但「加了 `3654`」不等於「3005 建得出單」——這是本節最容易只做一半的一格。**
實讀 `OnboardingService.findActiveSetting`：跨過 `if (shippingType == null) return null;` 之後，
它還要 `selectOne(store_id = ?, shipping_type = '3654', onboarding_status = ACTIVE, enabled = true)`。
**`seller_shipment_settings` 裡沒有那一列的話，回的還是 `null`**，
`ShippingService.createShipmentFromOrder` 的
`!UBER_INSTANT.equals(shippingMethod) && onboardingSetting == null` 照樣成立
⇒ **照樣拋 `SHIPMENT_SELLER_NOT_ONBOARDED`**。⇒ 下面兩件是**必做**，不是二選一：

| # | 動作 | 只做另一件會怎樣 |
|---|---|---|
| **(a)** | `merchant_delivery_profile` 轉 `ACTIVE` 的**同一交易**內建立／更新 `seller_shipment_settings` 的 `3654` 列（`onboarding_status = '3602'`、`enabled = true`、`goods_type = NULL`） | 缺 (a) ⇒ 建單永遠 `SHIPMENT_SELLER_NOT_ONBOARDED`；就算補了 (b) 讓建單過關，`listSellerShippingMethods` 因 `settingMap.get("3654:") == null` 仍回 `available=false`／`needsOnboarding=true` ⇒ **賣家後台顯示「尚未建店」而單卻建得出來**，兩個畫面互相矛盾 |
| **(b)** | `ShippingService.createShipmentFromOrder` 的建店豁免分支加 `3005`（＝上表第 2 列） | 缺 (b) 時 (a) 已足以讓主線成立；(b) 的價值在**profile 已 ACTIVE 但 setting 列建失敗**的窗口期不讓既有訂單卡死 ⇒ 保留為第二道保險，**不得拿它取代 (a)** |

🔴 **(a) 會把一列非 PayUni 的資料放進一張 PayUni 形狀的表，三個欄位必須明訂填法**
（實讀 `V1__init_shipping_tables.sql` 的 `seller_shipment_settings`）：

| 欄位 | 現況 | `3654` 列怎麼填 |
|---|---|---|
| `chk_shipping_type` | `CHECK (shipping_type IN ('3651','3652'))` | 🔴 **必須含 `'3654'`**，否則寫入被 DB 拒絕（`23514`）。歸屬見下方 M6-2 |
| `mer_id` | `VARCHAR(32)` **NOT NULL**，**無 DEFAULT** | 🔴 自送沒有 PayUni 商家編號。**放寬為 nullable ＋ 綁 CHECK，不用哨兵值**（65 §5.7）。<br>DDL：`ALTER TABLE seller_shipment_settings ALTER COLUMN mer_id DROP NOT NULL;` ＋ `ADD CONSTRAINT chk_sss_mer_id_applicability CHECK ((shipping_type = '3654') = (mer_id IS NULL));`<br>⚠️ `NOT NULL → NULL` 是**放寬（expand）**，既有列全部有值、既有讀取路徑不受影響。<br>🔴 **不得改用哨兵值 `''`**：那會讓「沒有 merId」與「忘了填」在資料上完全一樣；上面那條 CHECK 讓兩者**在寫入當下就分得開**（反向也守住：非 `3654` 的列仍不得為 NULL） |
| `payuni_submit_status` | `NOT NULL DEFAULT '4301'` ＋ `CHECK IN ('4301'..'4304')` | 取 DEFAULT `'4301' NONE`。⚠️ 語意是「PayUni 建店尚未送出」，對自送**永遠為真且永遠不會前進** ⇒ 任何錨在 `payuni_submit_status` 的看板／告警**必須排除 `shipping_type='3654'`**，否則自送每一列都會被算成「建店卡住」 |

⚠️ `return_store_code`／`partner_id`／`eship_id`／`provider_status` 皆 nullable ⇒ 留 `NULL`，不需要特別處置。

🔴 **`3005` 槽走 `onboardingRequired = true` ＋ `SHIPPING_TYPE_MERCHANT`，`SlotDef` 不新增欄位。**

`listSellerShippingMethods` 對 `onboardingRequired = true` 的槽位走既有的
`settingMap.get(buildSlotKey("3654", null))` 分支 ⇒ **per-seller 判斷由既有程式碼提供**。

⚠️ **per-seller 的真實來源是 `merchant_delivery_profile.status = 'ACTIVE'`，而畫面讀的是
`seller_shipment_settings.onboarding_status`** ⇒ 兩者必須同步（上面 (a) 那一列的工作）。
🔴 **同步是單向的**：`merchant_delivery_profile` 是**唯一真實來源**，
`seller_shipment_settings` 的 `3654` 列是它的**投影**；
**禁止在 ADM 直接改那一列的 `onboarding_status`／`enabled`**——那會產生一個
「profile 說停用、畫面說可用」的狀態，而沒有任何路徑推得走它。

🔴 **`3653`（68）與 `3654`（本設計）是兩個不同的值**，`buildSlotKey` 的第一段因此不同
——這正是撞號問題的解法本身。

### 5.2 索引與查詢模式

| 述詞（誰在查） | 索引 |
|---|---|
| 派送器候選：`status='PENDING' AND next_attempt_at <= now()` | `idx_mod_due`（partial） |
| 回查候選：`status IN (NOTIFIED,ACCEPTED,IN_FULFILLMENT) AND reconcile_next_at <= now()` | `idx_mdj_reconcile`（partial） |
| 健康檢查候選：`status='ACTIVE' AND tier IN (L1,L2) AND health_check_next_at <= now()` | `idx_mdp_health`（partial） |
| aging 指標：`GROUP BY status` ＋ `max(now() - status_entered_at)` | `idx_mdj_aging` |
| 驗簽解析：`key_id = ? AND revoked_at IS NULL` | `uq_mic_key_id`（partial unique） |

⚠️ **partial index 的述詞必須與查詢逐字一致**——少列一個狀態不會壞掉，
只會讓那批列走全表掃描，**而那是無聲的**（`idx_ipa_health` 的註解逐字記載了這一條）。

### 5.3 序號／排序／一致性：`event_seq_watermark` 的語意

商家事件帶 `sequence`（單調遞增，per shipment）。我方的合併規則**只有三條**：

| 情況 | 處置 | 為什麼 |
|---|---|---|
| `seq > watermark` | 正常套用；`watermark = seq` | — |
| `seq <= watermark` | **落 inbox（`process_status='DUPLICATE'`）但不套用** | 重送或亂序的舊事件。**不是丟棄**——inbox 留痕才查得出「商家一直在重送」 |
| `seq` 跳號（`seq > watermark + 1`） | 🔴 **照樣套用並推進 watermark**，同時記 counter `merchant_event_seq_gap_total` | 跳號多半是商家沒發某一則中間事件（例如直接 `DELIVERED`）。**卡住等中間那則會讓整條鏈停住**，而那一則可能永遠不會來 |

🔴 **`watermark` 只前進不後退**，且與 `ShipmentStatusMapper.isValidTransition` **是兩道獨立的防線**：
watermark 擋「舊事件覆蓋新事件」，mapper 擋「合法事件推出非法轉移」。
兩者都要通過才寫入；只做前者會讓商家發一則 `seq=99` 的 `PICKED_UP` 把已送達的單子推回在途。

### 5.4 Schema 演進與遷移

Java Flyway，append-only。🔴 **取號前現查 migration 目錄取現況最高版號 +1，本書不寫死任何版號**：

```
ls services/java/shipping-service/src/main/resources/db/migration/ | sort -t_ -k1.2 -n
```

⚠️ **不得用 `ls` 的字典序**（它把 `V9` 排在 `V10` 之後，會漏看兩位數版號而撞號；
Flyway 撞版號 ＝ 服務起不來）。

🔴 **同一個 CHECK 約束的 `DROP` 與 `ADD` 必須在同一個檔**（Flyway 預設單檔單交易），
否則中間有一個不受約束的窗口。

### 5.5 不變式清單

| # | 不變式 | 落在哪 | 違反時 |
|---|---|---|---|
| **INV-M1** | job 軸與 C 軸永不矛盾（依 §7.3.2 的全函式） | 應用層單一寫入路徑 ＋ §13.2 變異測試 | 兩個真相 |
| **INV-M2** | 🔴 **POD 存在 ≠ 收貨 Gate 通過** | §7.4 的表；E 軸不因 POD 而轉態 | 賣家自己就能讓錢出去 |
| **INV-M3** | `supported_temps` 非空 | DB CHECK | 冷凍貨排進常溫車 |
| **INV-M4** | 憑證輪替三聯同進同出 | DB CHECK `chk_mic_rotation_triplet` | 半套輪替 ⇒ 驗簽偶發失敗 |
| **INV-M5** | 一個 shipment 至多一個 job | `uq_mdj_shipment` | 重複派工 |
| **INV-M6** | 商家 inbound 的 `store_id` 一律來自驗簽結果，**不得**來自 payload | 程式碼 ＋ §13.2 變異 | 任一商家可推進任一賣家的單 |
| **INV-M7** | `merchant_outbound_delivery` 與 `shipments` 同交易建立 | 例外 A | 單建了但通知從未產生 |

### 5.6 加密 master key

profile 的**電話**與 POD 的**簽收人姓名**走專屬的 `MERCHANT_ENCRYPTION_MASTER_KEY`。

🔴 **不得與 invoice 共用同一把**——共用會讓 master key 輪替引爆全表重算
（`pkg/crypto/hmac.go` 的註解逐字說明這一點）。
🔴 **key 名要進 git，值不進**（65 附錄 G.10）。

---

## 5A. 金額

3005 的運費由 profile 計價設定算出，仍受 65 §3.9 二拘束：**元的整數、`BIGINT`、對外 `string`、禁浮點**。
`DISTANCE` 模式的距離乘算一律先算「公尺 × 每公里費率」再 `ROUND_HALF_UP` 到元，
**不得**在中途用 `double` 保留小數。

---

## 6. API 與介面契約

> 🔴 **本章的號碼一律不自行取**：業務碼由 **65 §6.7 一次配完**，本書只列語意並引用既有碼。
> 65 §6.7.4 已訂下「66／67／68／69 定版時回到本表連號追加」的程序。

### 6.1 對外 REST（我方 Gateway，給賣家在我方 App／Web 用）

**掛載**：全部走 `/api/v1/app/*`，`sellerHandle(group, method, path, authz.CapOrderFulfill, handler)`。
理由：履約是「處理既有已付款訂單」⇒ 停權白名單（`suspendedWhitelist` 含 `CapOrderFulfill`）。
⚠️ **profile 的編輯**例外：它是「開通新能力」而非處理既有訂單 ⇒ `authz.CapProductManage`
（與既有 `shipping_onboardings` 群一致，該群逐字用 `CapProductManage`）。

🔴 **path param 名必須與同層既有路由一致**：`/app/shipments` 這一層既有全部是 `:order_no`
（`routes_shipping.go`），新端點一律 `:order_no`；`/admin/shipments` 這一層是 `:id`。
不同名會讓 gin **啟動即 panic**——那是 boot 失敗不是編譯錯。

#### 6.1.1 L0 人工履約端點

| # | Method | Path | 說明 |
|---|---|---|---|
| M-1 | POST | `/app/shipments/:order_no/merchant_delivery/accept` | 賣家接單（job `PENDING_DISPATCH\|NOTIFIED → ACCEPTED`） |
| M-2 | POST | `/app/shipments/:order_no/handover` | **既有 S-3（65 §6.3.1），本書不另建**。`handoverType = SELF_DEPART` ⇒ C 軸 `930` |
| M-3 | POST | `/app/shipments/:order_no/merchant_delivery/progress` | 回報在途（C 軸 `931`? ❌ → `31`）；body `{ occurredAt, note? }` |
| M-4 | POST | `/app/shipments/:order_no/merchant_delivery/delivered` | 回報送達 ＋ POD；body `{ occurredAt, podPhotoKeys?: [string], signerName?, pinCode?, trackingNo? }` |
| M-5 | POST | `/app/shipments/:order_no/merchant_delivery/exceptions` | 回報無法配送／退回（C 軸 `33`） |
| M-6 | GET | `/app/shipments/:order_no/merchant_delivery` | 賣家看這一筆的 job 詳情與 `allowedActions[]` |

🔴 **M-4 的 `podPhotoKeys` 只收 objectKey**（65 §3.9 十一）：驗「非 http 開頭、無 `..`、前綴白名單」。
**禁止收 URL 回存**；欄名存 key 就叫 `*Keys`。

**寫入端點一律掛 `middleware.RequireIdempotencyKey()`**（M-1／M-3／M-4／M-5）。
⚠️ 65 §6.1.4 已登記的缺口原樣適用：該 middleware 目前把重放衝突的業務碼寫死成發票的兩個碼
⇒ 直接沿用會讓**物流端點吐發票錯誤碼**。修法在 65 §6.7 群 I（9560／9561 ＋ 把碼對改成參數），
**本書不重複定義，但列為上線前置**。

#### 6.1.2 服務區／時段／承載／費率設定端點

| # | Method | Path | Capability |
|---|---|---|---|
| M-7 | GET／PUT | `/app/merchant_delivery/profile` | `CapProductManage` |
| M-8 | GET／PUT | `/app/merchant_delivery/service_zones` | `CapProductManage` |
| M-9 | GET／PUT | `/app/merchant_delivery/service_windows` | `CapProductManage` |
| M-10 | GET／PUT | `/app/merchant_delivery/capacity` | `CapOrderFulfill`（日常營運調整，不是開通） |
| M-11 | GET | `/app/merchant_delivery/pricing` ／ PUT 同路徑 | `CapProductManage` |

🔴 **67 的 publish 前置驗證讀 M-8／M-9／M-10 的同一份資料**（走 gRPC 不走 HTTP，§6.3）。
⇒ 這四支的欄位語意改動是**跨本破壞性變更**。

#### 6.1.3 onboarding 與健康狀態端點

| # | Method | Path | 說明 |
|---|---|---|---|
| M-12 | POST | `/app/merchant_delivery/activate` | `DRAFT → ACTIVE`；🔴 **fail-closed 的檢核清單逐項回**（缺哪一項就列哪一項，不得只回「尚未完成設定」） |
| M-13 | GET | `/app/merchant_delivery/health` | 賣家看整合健康：最後一次成功推送、最後一則商家事件、是否降級、降級原因 |
| M-14 | POST | `/app/merchant_delivery/credentials/rotate` | 產生 pending secret，**回應只給一次全文**，之後只給 `last4` |
| M-15 | POST | `/app/merchant_delivery/credentials/rotate/commit` | 提前結束重疊窗口 |

⚠️ **M-14 的「只給一次」要真的做到**：`secret_ciphertext` 是 write-only，
`credential_updated_at` 是賣家唯一能察覺「我的 key 被換過嗎」的訊號（§5.1.2 的紅字）。

#### 6.1.4 從 action matrix 反推補齊的端點（ADM 側）

| # | Method | Path | 說明 |
|---|---|---|---|
| M-16 | GET | `/admin/merchant_delivery/profiles` | 列出所有商家整合與健康狀態 |
| M-17 | GET | `/admin/merchant_delivery/profiles/:id` | 詳情（🔴 憑證只回 `last4`） |
| M-18 | POST | `/admin/merchant_delivery/profiles/:id/tier` | 升／降成熟度（L2 開通需 ADM，§3.2 M7） |
| M-19 | POST | `/admin/merchant_delivery/profiles/:id/degrade` ／ `/restore` | 手動降級與解除降級 |
| M-20 | POST | `/admin/shipments/:id/merchant_delivery/force_status` | 🔴 **maker-checker**（沿用 58 號既有實作與 `chk_invmu_two_person`，65 附錄 G.4） |

🔴 **全部掛 `admin` group**（已含 `PrivateNoStore()` ＋ `JWTAuth()` ＋ `requireAdmin()`），
**沒有任何一支掛 `adminNoAuth`**——掛錯等於那支端點沒有任何身分要求，而它長得跟其他 `/admin/` 路由一模一樣。

#### 6.1.5 錯誤碼與業務碼清單（**語意，號碼由 65 §6.7 追加**）

**可直接沿用 65 §6.7 既有碼（不新增）**：

| 情境 | 沿用 |
|---|---|
| 目前狀態不允許此操作 | **9520** `SHIPMENT_STATE_TRANSITION_REJECTED` |
| 回報交運不被允許（`930` allowed-from 不含目前狀態） | **9527** `SHIPMENT_HANDOVER_NOT_ALLOWED` |
| 商家回報了無法辨識的狀態 | **9526** `SHIPMENT_PROVIDER_STATUS_UNKNOWN` |
| 此配送方式尚未開放 | **9503** `FULFILLMENT_PROVIDER_NOT_ENABLED` |
| 設定缺失 ⇒ 此配送方式暫不可用 | **9510** `SHIPPING_OPTION_CONFIG_MISSING` |
| 查不到費率 | **9511** `SHIPPING_OPTION_NO_FEE_TIER` |
| 超過材積／重量限制 | **9512** `SHIPPING_OPTION_EXCEEDS_LIMIT` |
| 冪等鍵重放／併發 | **9560**／**9561** |

**需要 65 §6.7 連號追加的語意（10 條，本書不取號）**：

| # | Name（建議） | Message | 觸發 | 面向 |
|---|---|---|---|---|
| a | `MERCHANT_DELIVERY_PROFILE_NOT_ACTIVE` | 尚未完成商家自送設定 | profile 非 `ACTIVE` 仍嘗試建單或結帳 | 賣／買 |
| b | `MERCHANT_DELIVERY_PROFILE_INCOMPLETE` | 設定尚未填寫完整 | M-12 檢核未過（`data.missing[]` 逐項） | 賣 |
| c | `MERCHANT_DELIVERY_OUT_OF_SERVICE_ZONE` | 此地址不在配送範圍內 | 結帳時收件地址不在服務區 | 買／賣 |
| d | `MERCHANT_DELIVERY_TEMP_UNSUPPORTED` | 商家不支援此溫層 | 商品溫層不在 `supported_temps` | 買／賣 |
| e | `MERCHANT_DELIVERY_CAPACITY_FULL` | 所選配送時段已額滿 | `reserved >= capacity` | 買 |
| f | `MERCHANT_DELIVERY_JOB_NOT_FOUND` | 找不到配送任務 | 非 3005 或 job 不存在 | 賣／ADM |
| g | `MERCHANT_DELIVERY_POD_REQUIRED` | 請先上傳交付證明 | profile 要求 POD 而 M-4 沒帶 | 賣 |
| h | `MERCHANT_DELIVERY_CREDENTIAL_ROTATION_IN_PROGRESS` | 金鑰輪替進行中 | 重疊窗口內再次 rotate | 賣 |
| i | `MERCHANT_DELIVERY_INTEGRATION_DEGRADED` | 商家系統連線異常，已切換為人工回報 | 降級中仍呼叫 L2 專屬端點 | 賣／ADM |
| j | `MERCHANT_DELIVERY_ENDPOINT_NOT_ALLOWED` | 回報網址不被允許 | M-7 填入 private IP／非 HTTPS（§3.2 M1） | 賣 |

⚠️ **需要登記 `bizHTTPStatus` 的**：`f` → **404**；`b`／`g`／`j` → **400**；`a`／`i` → **403**
（🔴 **不得用 404／503**，會被讀成「此環境尚未部署」而永久隱藏入口）。
🔴 **`bizHTTPStatus` 一律 4xx**（Cloudflare 會把 origin 的 5xx body 整個換掉）。

### 6.2 🔴 商家整合 API 契約（對第三方公開）

#### 6.2.1 契約總則

| 項目 | 規約 |
|---|---|
| Transport | HTTPS only，TLS 1.2+ |
| 版本 | URL path 帶 `v1`（§4.5） |
| **回應形狀** | 🔴 **統一 envelope `{code, message, data, traceId}`，HTTP 200 ＋ 業務碼** |
| 時間 | ISO 8601 UTC（`YYYY-MM-DDTHH:mm:ssZ`） |
| 金額 | **字串**（元的整數） |
| ID | 字串（TSID／訂單編號） |
| rate limit | per-merchant `60 req/min`（突發 120），超過回 `429`（**transport 層，不是業務碼**） |
| sandbox | 獨立的 `key_id` 前綴 `sbx_`，資料不進正式庫 |

🔴 **「統一 envelope」是被機制逼出來的，不是偏好**：Gateway 的 `banDirectJSON` ruleguard
擋住 handler 自己 `c.JSON`，成功回應的出口只有 `OK`／`OKProto`／`Page`／`GrpcResult`，
四者的 `message` 都由 `pkg/envelope` 寫死。要用另一套形狀就得繞過 ruleguard。
⚠️ **代價要寫出來**：65 附錄 C.5.1 的「上調 A」（對外商家 API 走統一 envelope ⇒ +12 碼）
因此**成立**。號段餘裕要重算，觸發條件見 65 §6.7.4。

#### 6.2.2 Outbound：`order.ready_for_fulfillment`（我方 → 商家）

```
POST {merchant.endpoint_url}
Content-Type: application/json
X-Hawkr-Key-Id:    {key_id}
X-Hawkr-Timestamp: 1757140800            # unix 秒
X-Hawkr-Nonce:     {32 hex}
X-Hawkr-Signature: base64(HMAC-SHA256(secret, canonical))
X-Hawkr-Event-Id:  {uuid v4}             # 重試時不變
```

**canonical（逐字，LF 分隔，UTF-8）**：

```
canonical = keyId + "\n" + timestamp + "\n" + nonce + "\n" + sha256hex(rawBody)
```

⚠️ **第四段是 body 的 hash 不是 body 本身**——沿用既有 `InternalAuthCanonical` 的四段形狀，
但把「accountId」那一格換成 body hash，因為對外契約必須綁定內容（防止換 body 重放）。

**payload**

```json
{
  "eventId": "…", "eventType": "order.ready_for_fulfillment", "occurredAt": "2026-09-06T10:00:00Z",
  "apiVersion": "v1",
  "shipment": { "shipmentNo": "SHP-20260906-000123", "orderNo": "O…", "temperature": "3203" },
  "pickup":   { "contactName": "…", "phone": "…", "address": "…", "readyAt": "2026-09-06T11:00:00Z" },
  "dropoff":  { "recipientName": "…", "phone": "…", "address": "…", "zipcode": "…",
                "window": { "date": "2026-09-07", "startMinute": 540, "endMinute": 720 } },
  "items":    [ { "name": "…", "qty": "2", "weightGrams": "1200" } ],
  "requirements": { "photo": true, "signature": false, "pin": true },
  "callbackUrl": "https://api.example.com/webhook/merchant_delivery/v1/events"
}
```

**ACK 契約**：**回 2xx 即視為送達**（`status='DELIVERED'`）。回應 body 我方不解析。
非 2xx 或逾時 ⇒ 依 §0.3 退避重試，**同一個 `eventId`**。
6 次後 `status='ABANDONED'` ⇒ **降級 L0 ＋ 建 case**（category `MERCHANT_OUTBOUND_ABANDONED`）。

🔴 **`callbackUrl` 放進 payload 而不是叫商家硬編碼**：硬編碼的那一份會在我方換網域時全體壞掉，
而壞掉的形式是「商家一直打舊網址」——**我方完全看不到**。

#### 6.2.3 Inbound：商家回報事件（商家 → 我方）

```
POST /webhook/merchant_delivery/v1/events        # 🔴 top-level，非三 namespace
```

🔴 **落點決策（本書定版，需回灌 65 §3.9 五一行）**：
它與既有 `/webhook/payuni/notify`（`r.POST(WebhookPayUniNotify, …)`，**engine root**）同層同形。
65 §3.9 五 要求改掛 `/app`／`/admin` 的是**賣家與管理操作端點**，不是 **provider 回調**——
回調的呼叫者是第三方機器，身分由簽章承擔，`/app/*` 的整條 middleware 鏈
（`JWTAuth` → `sellerid` resolver → capability）對它全部不適用。硬掛過去只會逼出「第二種 JWT 來源」。

**處理順序（🔴 這個順序是規範，不是描述）**

```
1. Gateway：不解析、不重新序列化，raw body 以 bytes 透過 gRPC 轉給 shipping-service
2. 驗簽（current → pending 各試一次，常數時間比較）        ← 失敗即 4xx，不進 inbox
3. 時鐘檢查 |now - ts| <= 300s                            ← 失敗即 4xx
4. 由 key_id 解析出 store_id                               ← INV-M6：store_id 只從這裡來
5. 去重（uq_mei_event on (store_id, event_key)）
6. 順序合併（§5.3）
7. isValidTransition ＋ 中央狀態分派點寫入
```

**② 必須先於 ⑤**（§4.6 A1）；**② 必須拿 raw body**（G.2 逐字：任何在驗簽前做過 JSON parse／
重新序列化的實作都會偶發失敗，失敗率取決於欄位順序 ⇒ 測不出來、上線才炸）。

**事件型別**

| `eventType` | 帶什麼 | 我方處置 |
|---|---|---|
| `ACCEPTED` | `acceptedAt`, `estimatedDeliveryAt?` | job → `ACCEPTED`；**C 軸不變**（仍 `91`）。🔴 **idempotent 202**：重複送同一個 `eventId` 回相同結果，不報錯 |
| `PICKED_UP` | `seq`, `occurredAt`, `courierRef?` | job → `IN_FULFILLMENT`；C 軸 → `930` |
| `IN_TRANSIT` | `seq`, `occurredAt`, `note?` | C 軸 → `31` |
| `DELIVERED` | `seq`, `occurredAt`, `pod{...}` | job → `DELIVERY_REPORTED`；C 軸 → `11`。🔴 **不推 E 軸**（§7.4） |
| `REJECTED` | `reasonCode`, `reasonText` | job → `REJECTED`；C 軸 → `33` |
| `UNDELIVERABLE` | `reasonCode`, `attemptCount` | C 軸 → `33` |
| `RETURNED` | `returnedAt`, `reasonCode` | C 軸 → `33`（🔴 **不是 `51`／`53`**，見 §7.3.2 的紅字） |
| `CANCEL_ACK` | `cancelledAt` | job → `CANCELLED`；C 軸 → `41` |

🔴 **後四種是需求書零描述、本書補的**（§7.3.4）。少了它們，商家「送不到」時**沒有任何一條路**
把訂單推出去，而症狀是訂單永遠停在配送中。

**POD metadata**（隨 `DELIVERED`）

```json
"pod": { "photoUrls": [...], "signerName": "…", "pinVerified": true, "trackingNo": "…" }
```

⚠️ **`photoUrls` 是外部 URL（商家的），不是我方 objectKey**——我方必須**主動抓回自存**
（65 §3.9 十一對 Uber POD 的同一條理由：對方的保存期短於我方的爭議窗口）。
抓回後落 `merchant_pod_artifact.object_key`，**原始 URL 不入庫**（禁 `*_url` 名存值）。

**敏感資料遮罩**：`raw_payload` 入庫前把 `dropoff.phone`／`dropoff.address`／`signerName`／`pinCode`
以 `redact` 取代（§8.6）。

#### 6.2.4 L2：merchant-scoped REST API（7 個動作）

**掛載**：OAuth2 換得我方 JWT（`accountType = SELLER`，principal ＝ 該賣家 `account_id`）
⇒ **全部走 `/app/*` ＋ `sellerHandle(..., CapOrderFulfill, ...)`**，沿用既有能力層。
🔴 **不另建平行 authz**：46 號要求「賣家能不能做這件事」只有一個問答點；
另建一套對外授權等於重新鑿開那個洞（《賣家能力權限層規範》）。

| # | 動作 | Method / Path | 簽章 | 冪等 |
|---|---|---|---|---|
| 1 | `quote` | POST `/app/merchant_delivery/quotes` | Bearer JWT | 否（唯讀） |
| 2 | `accept` | POST `/app/shipments/:order_no/merchant_delivery/accept` | Bearer JWT | `Idempotency-Key` |
| 3 | `pickup` | POST `/app/shipments/:order_no/merchant_delivery/pickup` | Bearer JWT | `Idempotency-Key` |
| 4 | `status` | GET `/app/shipments/:order_no/merchant_delivery` | Bearer JWT | 否 |
| 5 | `cancel` | POST `/app/shipments/:order_no/merchant_delivery/cancel` | Bearer JWT | `Idempotency-Key` |
| 6 | `return` | POST `/app/shipments/:order_no/merchant_delivery/returns` | Bearer JWT | `Idempotency-Key` |
| 7 | `POD` | POST `/app/shipments/:order_no/merchant_delivery/pod` | Bearer JWT | `Idempotency-Key` |

⚠️ **2／3／5／6／7 與 §6.1.1 的 M-1～M-5 是同一組 handler**——L0 的賣家用瀏覽器 JWT 打，
L2 的商家用 client_credentials JWT 打。**兩個入口一套授權、一套狀態機**。
🔴 這正是「同一件事有兩個入口時，只有其中一個掛著既有守門」那個 bug class 的反面做法。

#### 6.2.5 認證

**L1（HMAC）**：§6.2.2 的 canonical ＋ §0.4 的 rotation。
**輪替程序**（六步，缺一即中斷）：
① 賣家在 M-14 產生 pending（我方回傳一次全文）→ ② 商家部署新 key →
③ 重疊窗口內兩把都驗得過 → ④ 商家確認 → ⑤ M-15 commit（pending 升 current、清空三聯）→
⑥ 未 commit 者由 `rotation_deadline_at` 到期自動 commit（**不是自動回滾**）。
🔴 **⑥ 選 commit 而非回滾的理由**：到期時商家多半已經在用新 key，回滾會讓正在運作的整合當場斷掉。

**L2（OAuth2 `client_credentials`）**：
`POST /api/v1/public/auth/merchant_delivery/token`（**匿名 namespace，與既有 `pubAuth` 群同層**）
```
{ "grantType": "client_credentials", "clientId": "...", "clientSecret": "..." }
→ data: { "accessToken": "...", "tokenType": "Bearer", "expiresIn": "3600", "scope": "merchant_delivery" }
```
🔴 **`/public/*` 的 deny-by-default 白名單一樣適用**：本端點回應逐欄明列，
並附契約測試同時斷言「PII 欄名不在回應」＋「欄位集合＝白名單」。

#### 6.2.6 失聯回查 `GET shipment reconcile`

**僅 L2**（L1 沒有查詢端點，§0.1）。

```
GET {merchant.base_url}/v1/shipments/{shipmentNo}
→ { "status": "IN_TRANSIT", "seq": 7, "occurredAt": "...", "pod": {...}? }
```

| 項 | 值 |
|---|---|
| 觸發條件 | `job.status IN (NOTIFIED, ACCEPTED, IN_FULFILLMENT)` **且** `now - last_event_at > max(2×accept_sla, 30 分)` |
| 間隔 | 30 分（`reconcile_next_at` 租約欄） |
| 收斂 | 回應的 `seq > watermark` ⇒ 走與 inbound 完全相同的 ⑥⑦ 步（**不另寫一套合併邏輯**） |
| 放棄 | 窗口 24h 內未收斂 ⇒ **降級 L0 ＋ 建 case**（category `MERCHANT_UNREACHABLE`） |

🔴 **回查是「重新確認同一件事」但仍走 lease 不走 level-triggered**——因為它**打第三方**，
而 65 §3.9 九 對「向外」的規則是形狀優先於語意：level-triggered 在多副本下會重複打對方的 API，
把對方的 rate limit 吃掉。

#### 6.2.7 錯誤碼表（給商家看的）

🔴 **與我方內部業務碼是同一套**（§6.2.1 的決策），但**商家只會遇到其中一個子集**，
公開文件只列這個子集 ＋ 對照的處置建議：

| 情境 | 碼 | 商家該怎麼做 |
|---|---|---|
| 簽章不符 | （追加）`MERCHANT_SIGNATURE_INVALID` | 檢查 canonical 組法與 key；**不要重送** |
| 時間戳超出容忍 | （追加）`MERCHANT_TIMESTAMP_SKEW` | 校時；**不要重送** |
| key 已撤銷／不存在 | （追加）`MERCHANT_KEY_UNKNOWN` | 聯絡平台；不要重送 |
| 重複事件 | **9560**／既有冪等碼 | 視為成功，不要重送 |
| 狀態不允許 | **9520** | 讀 `status` 端點對齊後再送 |
| 找不到配送任務 | （追加）`MERCHANT_DELIVERY_JOB_NOT_FOUND` | 確認 `shipmentNo` |
| 平台暫時不可用 | **HTTP 5xx** | **重送**（退避） |

⚠️ 三個 `MERCHANT_*` 碼一樣要 65 §6.7 追加號碼（本書不取號）。

#### 6.2.8 商家整合自測清單（sandbox）

| # | 案例 | 期望 |
|---|---|---|
| 1 | 收到 outbound 並回 200 | job 進 `NOTIFIED` |
| 2 | 同一 `eventId` 收兩次 | 商家端只出一次貨（M4） |
| 3 | 送 `ACCEPTED` 兩次 | 我方回相同結果（idempotent） |
| 4 | 簽章故意改一個 byte | 4xx，且事件**不進** inbox |
| 5 | 時間戳 -400 秒 | 4xx |
| 6 | `seq` 倒退 | 我方回成功但**不套用**（`DUPLICATE`） |
| 7 | `seq` 跳號 | 我方套用並記 gap |
| 8 | 輪替期間用新舊兩把 key 各送一則 | 兩則都通過 |
| 9 | `DELIVERED` 帶 POD 照片 URL | 我方抓回自存，回應帶 `podReceived: true` |
| 10 | 我方回 5xx | 商家退避重送 |

### 6.3 gRPC 介面

| RPC | 呼叫端 | 說明 |
|---|---|---|
| `GetMerchantDeliveryCapability(store_id, temp, zipcode)` | order-service（checkout） | 結帳可用性；🔴 **fail-closed**：查不到／逾時 ⇒ 類 C（整筆請求失敗），不得靜默排除 |
| `QuoteMerchantDelivery(...)` | order-service | `DYNAMIC_QUOTE` 開啟時才呼叫；**3s deadline** |
| `IngestMerchantDeliveryEvent(bytes raw_body, map<string,string> headers)` | gateway | §6.2.3 |
| `GetMerchantDeliveryWindows(store_id, date_from, date_to)` | **recurring-service（67）** | 方案發布前置驗證 |
| `ReserveMerchantCapacity(store_id, bucket_key, qty)` | order-service／recurring-service | 承載預約，單列 CAS |
| `ReleaseMerchantCapacity(store_id, bucket_key, qty, reason)` | order-service／recurring-service | **承載釋放，單列 CAS**。🔴 **缺這一支，67 §5.1.7 要求的「雙向釋放」做不到**——本地把 hold 標成 `RELEASED` 而**對方仍鎖著**，且**沒有任何錯誤訊息**（67 §5.5 的靜默陷阱 B3）。`reason` 值域 `EXPIRED`／`CANCELLED`／`RESCHEDULED`／`COMMIT_FAILED` |

🔴 **proto 型別**：response 側**沒有**「內部 RPC 可用原生數值」的豁免（65 §6.5.2）——
`capacity`／`reserved`／金額一律 `string`。`make proto-lint-numeric` 的走訪起點是 OUTPUT type，
不看該 RPC 是否對外。

### 6.4 非同步事件（MQ）

🔴 **本書新增 0 條 routing key。**

依 65 §6A.0 **R3**（能用既有 key 承載的一律擴充 payload，不開新 key）：
3005 的所有狀態變化都由既有 `shipment.status.changed`／`shipment.delivered` 承載，
而 65 §6A.1.1 **X4** 已經替這三條 key 加上了 `provider`／`previous_status`／`occurred_at`／`commerce_model`
四欄——**其中 `provider` 正是為了「66／69／68 共用同一條 key，consumer 要分辨來源」**。

⇒ 本書要辦的只有三件：
1. `ShippingEventPayload` 的 `provider` 欄在 3005 上填 `MERCHANT`（🔴 值域的 SSOT 是 **65 附錄 C.2** 的 `provider` 那一列＝`MERCHANT`／`TAQBIN`／`CVS_711`／`UBER_DIRECT`，**字串**）
1b. 🔴 **`commerce_model` 欄：本書不決定它的值，一律照抄 `orders` 的快照欄**（65 §5.1 的訂單快照六欄之一）。
   65 §4.6 把 `commerce_model` 列為 **66／67／68／69 共用 envelope 的必帶欄**
   （逐字理由：「消費端要據此決定『這筆要不要進撥款鏈』。**沒有它，consumer 只能猜**」），
   而本書被 **66** 與 **67** 共同引用。
   ⚠️ 66／67 兩本現行都是 `SELLER_DIRECT`（65 §0.6.1／§3.8b C-1）⇒ 這個欄位**實際只會出現一種值**。
   🔴 **不得因此改成硬填**——判準是「這個欄位的值由誰決定」，不是「現在有幾種值」：
   `commerce_model` 是 `orders` 的快照欄，**政策可以再變**（保留值仍在值域內），
   而硬填的那一版不會跟著變，且**填錯時完全靜默**（consumer 只會拿到一個看起來正常的錯值）。

   | 錯誤做法 | 為什麼錯 |
   |---|---|
   | 在 3005 上硬填任何常數（含 `SELLER_DIRECT`） | 🔴 **本欄的權威是 `orders` 的快照，不是配送方式**。硬填等於製造第二個真相；政策一變就漂移，而漂移**不會有任何東西轉紅** |
   | 因為「與帳務軸正交」就不填 | 65 §4.6 是**必帶欄**；缺欄時 settlement 落 `blocked_by = INPUT_UNAVAILABLE:commerce_model`（65 §6A 的未知處置），等於把每一筆自送訂單都推進人工佇列 |
   | 由 shipping-service 依 `shipping_method` 反推 | `3005` 與 `commerce_model` **沒有函數關係**（它是訂單的性質不是配送方式的性質）⇒ 反推是巧合成立，不是正確 |

   ⇒ **正解**：publisher 從該 shipment 對應 `orders` 的 `commerce_model` 快照欄讀出後原樣填入，
   讀不到就**不發事件並告警**，不得補預設值（`orders.commerce_model` 已 `DROP DEFAULT`，65 §5.2 逐字理由：
   留著 DEFAULT 等於「忘了設的 bug 會安靜地產出一張永遠不進撥款軸的訂單」）。

   🔴 **這與 §1.1「與帳務軸正交」不衝突，兩者講的是不同的事**：
   正交的意思是**本書不得依 `commerce_model` 改變自己的行為**（不得寫「因為是商家自送所以…撥款／發票…」），
   **不是**「本書不填這個欄位」。**填一個由別人決定的值，正是正交的表現**——
   它證明本書只是把訂單既有的事實透傳出去，沒有參與那個決定。
2. **通知模板**：買家「賣家已出發配送」與「已送達」兩則沿用既有 `shipment.status.changed` 模板，
   🔴 **`payload_filter` 一律寫 wire 值（`"930"`／`"11"`）不是 enum 名**——
   純字面字串比對、無型別轉換，寫錯不報錯、不進 DLQ，只是**沒有人收到**（65 附錄 G.3 的實際事故）
3. `tests/contract/golden/event_payload_keys.json`（**在 repo root**）同批更新

⚠️ **既有 gate 只驗「template 讀的 key ⊆ golden」，不驗「golden ⊆ publisher 實發」**
⇒ golden 手寫錯不會被抓（36 §3.3 的 seller 送達通知因此靜默 0 投遞）。
必須加一支 publisher-side 反向斷言。

**`topology.yaml`：零變更**（未新增 queue／exchange／binding）⇒
`infra/mq/tools/test_expand.py` 的資源數量 pin **不動**。
🔴 這句話要驗，不要假設：新增任何 queue 都會讓 pin 變（一個帶 `dlq:` 的新 queue ＝ queues +2、bindings +2）。

---

## 7. 執行時視圖

### 7.0 元件代號與排程器名冊

| 代號 | 名稱 | 服務 | 形狀 |
|---|---|---|---|
| **MD.out** | `MerchantOutboundDispatcher` | shipping | **lease**（租約 60s，§0.2） |
| **MD.rec** | `MerchantReconcileScheduler` | shipping | **lease**（租約 10 分＝1 tick） |
| **MD.hc** | `MerchantHealthCheckScheduler` | shipping | **lease**（租約 60s） |
| **MD.acc** | `MerchantAcceptTimeoutScheduler` | shipping | **lease**（租約 5 分） |
| **MD.gc** | `MerchantInboxCleanupScheduler` | shipping | **非 claim 形狀（GC）** |

🔴 每一支都必須有這兩支指標（65 §7.5.1 逐字，不是樣板文字）：
```
fulfillment_scheduler_claim_total{scheduler, result}
fulfillment_scheduler_last_success_timestamp_seconds{scheduler}
```
告警一律 **`absent()` ∨ `age > 3×cadence`**——`absent()` 那一半不是裝飾：
排程器從來沒跑過（DI 沒 wire）時，`age > 3T` 永遠不會響，因為根本沒有 series。

⚠️ **MD.gc 不得被拿來當 claim 的範例**（長得像正例的反例，65 附錄 G.7）。

### 7.1 主要場景（happy path）

```
訂單成立(3005) ─▶ createShipmentFromOrder：建 shipments(91) ＋ job(PENDING_DISPATCH)
                    ＋ merchant_outbound_delivery(PENDING)   ← 同一交易（例外 A）
   │
   ├─ L0：job 直接落 ACCEPTED（無 outbound）      ← 🔴 見 §7.3.1 的紅字
   └─ L1/L2：MD.out ─▶ 商家 endpoint ─2xx─▶ job=NOTIFIED
                            │
        商家 ACCEPTED  ─────▶ job=ACCEPTED（C 軸仍 91）
        商家 PICKED_UP ─────▶ job=IN_FULFILLMENT，C 軸 930
        商家 IN_TRANSIT ────▶ C 軸 31
        商家 DELIVERED+POD ─▶ job=DELIVERY_REPORTED，C 軸 11
                              ＋ shipment.delivered ─▶ order-service
                              ─▶ A 軸 40 ＋ auto_complete_at = now+60h ＋ E 軸 PENDING
   │
   └─ 🔴 到此為止，錢還沒有動。收貨 Gate 見 §7.4
       買家確認收貨（B-2）─▶ E 軸 BUYER_CONFIRMED ─▶ job=CLOSED
```

### 7.2 時序圖（正常 ＋ 失聯兩條）

```
賣家   平台(shipping)        商家ERP            買家
 │          │                  │                 │
 │  訂單成立│                  │                 │
 │          │──outbound(evt)──▶│                 │   ← 重試 6 次，同一 eventId
 │          │◀──── 200 ────────│                 │
 │          │◀── ACCEPTED ─────│                 │
 │          │◀── PICKED_UP ────│                 │   C:930
 │          │                  │                 │
 ┆   ┄┄ 失聯路徑 A（outbound 送不到）┄┄
 │          │──✗──✗──✗──✗──✗──✗│                 │
 │          │ ABANDONED ⇒ 降級 L0 ＋ case         │
 │◀─通知────│                  │                 │   賣家改用 M-1..M-5 手動推
 ┆   ┄┄ 失聯路徑 B（商家收了但不回報）┄┄
 │          │──── 回查(L2) ───▶│（無回應 ×48）    │
 │          │ 24h ⇒ 降級 L0 ＋ case                │
```

### 7.3 狀態機

#### 7.3.1 `merchant_delivery_job` 狀態值與轉換邊（本書自建，逐值答四問）

| 值 | 誰推進來 | 誰推走 | allowed-from | 卡住誰知道 |
|---|---|---|---|---|
| `PENDING_DISPATCH` | `createShipmentFromOrder`（**L1／L2 才落此值**） | **MD.out**（送達 2xx）／取消 | ∅（入口） | aging > 1h ⇒ MD.out 死了 |
| `NOTIFIED` | MD.out | 商家 `ACCEPTED` 事件／`REJECTED`／取消 | `PENDING_DISPATCH` | `accept_due_at` 過期 ⇒ **MD.acc** 建 case ＋ 降級 |
| `ACCEPTED` | 商家 `ACCEPTED`；**L0 由建單直接落此值** | 商家 `PICKED_UP`／賣家 M-2（`SELF_DEPART`）／`REJECTED`／取消 | `PENDING_DISPATCH`, `NOTIFIED`, ∅（L0 入口） | aging > `prep_minutes + 2h` ⇒ MD.rec／告警 |
| `IN_FULFILLMENT` | 商家 `PICKED_UP`／賣家 M-2／M-3 | 商家 `DELIVERED`／賣家 M-4／`UNDELIVERABLE`／`RETURNED`／取消 | `ACCEPTED` | aging > 24h ⇒ 告警 ＋ MD.rec |
| `DELIVERY_REPORTED` | 商家 `DELIVERED`／賣家 M-4 | **E 軸確認事件**（`order.receipt.confirmed`）⇒ `CLOSED` | `IN_FULFILLMENT` | aging > D+7+1h ⇒ 與 E 軸 `AUTO_CONFIRMED` 的 aging 同一條 |
| `REJECTED` | 商家 `REJECTED`／`UNDELIVERABLE`／`RETURNED`；賣家 M-5 | **履約失敗鏈**（65 §6A.1.2 **N1** `order.fulfillment.failed` → aftersales `cancelShipmentBySystem`）⇒ `CANCELLED` | `NOTIFIED`, `ACCEPTED`, `IN_FULFILLMENT` | aging > 2h ⇒ 退款鏈沒接上 |
| `CANCELLED` | 取消確認／履約失敗鏈 | — **終態 T1** | 全部非終態 | — |
| `CLOSED` | E 軸確認 | — **終態 T1** | `DELIVERY_REPORTED` | — |

🔴 **L0 直接落 `ACCEPTED` 不是特例而是必要**：L0 沒有 outbound 派送器
⇒ 若也落 `PENDING_DISPATCH`，**沒有任何東西會把它推走**（65 §3.11 第②問答不出來）。
這一格是本節最容易漏的形狀。

🔴 **`REJECTED` 不是終態**：它必須被推到 `CANCELLED`，否則錢收了、貨沒送、訂單沒取消。
推走它的是 65 已定義的 **N1** 事件鏈，**本書不另建第二條退款入口**
（refund 的唯一寫入路徑在 aftersales）。

**降級是欄位不是狀態**：`merchant_delivery_profile.integration_degraded_at`。
入邊＝MD.out 放棄／MD.rec 放棄／MD.hc 連續失敗／ADM 手動（M-19）；
出邊＝MD.hc 連續成功 3 次／ADM 手動解除。
🔴 **降級中 job 的每一條轉換邊照常存在**，只是推進者換成賣家在 App 操作。

#### 7.3.2 商家原生值 → 65 normalized 值的映射表

| 商家原生值（照抄） | job 軸 | **normalized（65 §7.1.2）** | `allowedActions[]`（65A 投影） |
|---|---|---|---|
| `ACCEPTED` | `ACCEPTED` | **不變**（`UNPROCESSED(91)`） | `handover`, `cancel` |
| `PICKED_UP` | `IN_FULFILLMENT` | **`HANDED_OVER(930)`** | `report_progress`, `report_delivered`, `report_exception` |
| `IN_TRANSIT` | `IN_FULFILLMENT` | **`IN_TRANSIT(31)`** | `report_delivered`, `report_exception` |
| `DELIVERED` | `DELIVERY_REPORTED` | **`PICKED_UP(11)`** | `upload_pod` |
| `REJECTED` | `REJECTED` | **`ABNORMAL(33)`** | `admin_resolve` |
| `UNDELIVERABLE` | `REJECTED` | **`ABNORMAL(33)`** | `admin_resolve` |
| `RETURNED` | `REJECTED` | **`ABNORMAL(33)`** | `admin_resolve` |
| `CANCEL_ACK` | `CANCELLED` | **`CANCELLED(41)`** | — |

🔴 **`COURIER_ASSIGNED(933)` 這條鏈在 3005 上不適用**：商家自送沒有我方看得見的外送員，
`933` 的「適用」欄逐字只有 `3003`。**交接一律用 `930`**。
🔴 **`RETURNED` 映 `33` 而不是 `51`／`53`**：65 §7.1.2.4 ⑤ 逐字把 `43`～`56` 列為 3005 不經過的值。
逆向在商家自送上不是一個獨立的貨態鏈，是一個需要 ADM 介入的異常。
⚠️ **這一格如果覺得不合理，正確做法是回 65 §7.1.2 過四問，不是在本書自己加值。**

**A-2 斷言（65 §7.1.2.7）在本表上的檢查**：不存在兩個原生值映到同一 normalized 值卻分屬不同述詞集合。
逐條核對——`REJECTED`／`UNDELIVERABLE`／`RETURNED` 三者都映 `33`，而 `33` ∈ `isAbnormal` ⇒ 同一集合 ✅。

**`992` 對 3005 的適用範圍**（65 §7.1.2.0b `M-3`）：
——**擴到 3005 但限 L1／L2**，L0 不適用（沒有對象可等）；
65 §7.1.2.1 的 `992` 列適用欄現為 `3002／3003／3004／3005（限 L1／L2）`，§6 的取消分支同步。
⇒ **L0 取消同步走 `41`；L1／L2 照常走 `992`**（出邊＝商家 `CANCEL_ACK` → job `CANCELLED` ＋ C 軸 `41`），
**不再走 ADM 人工過渡**。

**`991` 在 3005 上永不出現**：3005 建單是純本地寫入，不打第三方 ⇒ 沒有「建單結果未知」。

#### 7.3.3 終態清單

**T1（機器強制終態）**：job `CANCELLED`／`CLOSED`；C 軸沿用 65 的 `PICKED_UP(11)`／`CANCELLED(41)`。
**T2（業務終態）**：3005 上**沒有** C 軸的 T2 值（`43`／`46`／`56` 都不經過）。
**T3（必須有推走者）**：job 的其餘 6 值，逐一見 §7.3.1。

⚠️ **aging 指標必須排除 T1**，否則會產生一條永遠上升、沒有人該處理的曲線。

#### 7.3.4 失敗與退回分支（🔴 需求書零描述，本書補）

| 分支 | 觸發 | job | C 軸 | 錢怎麼走 |
|---|---|---|---|---|
| **商家拒單** | `REJECTED`（庫存不足／超出服務區／不接單） | `REJECTED` | `33` | N1 → aftersales `cancelShipmentBySystem` → 全額退款 |
| **無法配送** | `UNDELIVERABLE`（地址錯／收件人聯絡不上，`attemptCount` 用完） | `REJECTED` | `33` | 🔴 **不自動退款**：責任歸屬未定 ⇒ 建 case，ADM 裁決（可能是買家責任） |
| **退回商家** | `RETURNED` | `REJECTED` | `33` | 同上 |
| **接單逾時** | `accept_due_at` 過期 | 停在 `NOTIFIED` | `91` | 降級 L0 ＋ 通知賣家；賣家仍可 M-1 接單 |
| **outbound 放棄** | 6 次退避耗盡 | 停在 `PENDING_DISPATCH` | `91` | 降級 L0 ＋ case |

🔴 **「拒單全額退款」與「無法配送不自動退款」的差別要寫在 case 的 `proposed_action_codes` 裡**——
兩者在 ADM 佇列上長得一模一樣，靠人記得住是不可能的。

### 7.4 🔴 收貨 Gate 與 POD 信任層級（本書的核心規則）

| 證據 | L0 | L1 | L2 | **能否單獨通過收貨 Gate** |
|---|:---:|:---:|:---:|---|
| 賣家自稱送達（job → `DELIVERY_REPORTED`） | ✓ | ✓ | ✓ | 🔴 **不能** |
| POD（照片／簽收人／PIN） | ✓ | ✓ | ✓ | 🔴 **不能**——需與買家確認或 ADM 裁決併用 |
| verified callback（驗簽通過的商家事件） | — | ✓ | ✓ | 🔴 **不能**（同上；驗簽證明的是「訊息來自這個商家」，不是「貨真的到了」） |
| 可信 carrier event | — | — | ✓ | 🔴 **不能**（§3.6 第一條裁決：仍是商家轉述） |
| **買家確認收貨**（B-2） | ✓ | ✓ | ✓ | ✅ |
| **ADM 裁決**（65 A-17／A-18，maker-checker） | ✓ | ✓ | ✓ | ✅ |

**機制上怎麼保證**（不是靠紀律）：

1. `DELIVERED` 只寫 **C 軸** `11`，**不碰 E 軸**。E 軸的入邊只有三條：
   買家 B-2、60h AUTO（→ `AUTO_CONFIRMED`）、ADM 裁決。
2. 撥款資格的 **C3** 條件是 `receipt ∈ {BUYER_CONFIRMED, ADMIN_CONFIRMED}`——
   **`AUTO_CONFIRMED` 不在其中**（65 §7.1.1.2）。
3. `eligible_at` 只由 **D+7 鏈或買家提前確認**寫入，60h AUTO 一律不寫（65 §7.3.4）。

⇒ **賣家自稱送達 → 60h 後 A 軸自動完成 → E 軸 `AUTO_CONFIRMED` → 撥款仍然 BLOCKED**，
直到 D+7 的 ADM 裁決或買家自己確認。**這就是 D-14 的實質內容，而它已經在 65 的機制裡，本書不重複實作。**

⚠️ **POD 的價值是給 ADM 裁決用的，不是給 Gate 用的**。所以 POD 缺漏不改變狀態
（§0.2 是軟窗口），但會讓 D+7 的裁決缺證據 ⇒ `pod_due_at` 逾時要**通知賣家**而不是只記 counter。

⚠️ 🔴 **本節不適用於 `commerce_model = SELLER_DIRECT` 的撥款軸**——那條軸根本沒有 G 軸的列
（65 §7.1.1.1 的商流矩陣）。收貨 Gate 在那裡守的是**訂單完成與售後窗口**，不是撥款。
**本書不因為配送是自送就推論帳務走向**（§1.1）。

### 7.5 失效與重試

| 失效 | 偵測 | 處置 |
|---|---|---|
| outbound 送不到 | MD.out 的 `attempt_count` | 退避 6 次 → `ABANDONED` → 降級 ＋ case |
| inbound 亂序 | `event_seq_watermark` | 落 inbox `OUT_OF_ORDER`／`DUPLICATE`，不套用 |
| inbound 重複 | `uq_mei_event` | 回既有結果（idempotent 202） |
| 商家長時間中斷 | MD.hc 連續失敗 3 次 或 MD.rec 24h 未收斂 | 降級 L0 ＋ case ＋ 通知賣家 |
| 我方 DB 暫時不可用 | — | 對商家回 **5xx**（讓對方重送），**不要**回 200 假裝收到 |

🔴 **降級是自動的，恢復不是**：MD.hc 連續成功 3 次會清 `integration_degraded_at`，
但**已經降級期間產生的 job 不會自動切回**（它們的 `integration_tier` 是快照）。
⚠️ 這一條要寫進 Runbook，否則營運會以為「恢復了怎麼還在人工」。

### 7.6 失聯回查與 case 的生命週期

| 問題 | 答案 |
|---|---|
| **誰開** | shipping-service 發事件 → **payment-service（Logistics Finance module）** 建 `fulfillment_exception_cases`（65 §5.5 的形狀；見上方 §「本表不建」）。⚠️ **payment-service 側已持有表與建案路徑；shipping-service 側的「事件 → 建案」接線是本設計的上線前置** |
| **誰關** | ADM（65 §6.4 的四支端點：列出／認領／結案／駁回）。🔴 **那四支是本書的上線前置**，缺任一支就會產生只進不出的列 |
| **去重** | `dedup_key = 'MD:' + category + ':' + subject_ref`，錨在「同一個問題還開著」而不是「這個事件收過了」 |
| **卡住誰知道** | `fulfillment_exception_cases` 的 `idx_exc_aging` ＋ 65 §8.5 的 case aging 告警 |

**本書新增的 category（附錄 C）**：`MERCHANT_OUTBOUND_ABANDONED`／`MERCHANT_UNREACHABLE`／
`MERCHANT_ACCEPT_TIMEOUT`／`MERCHANT_SIGNATURE_FAILURE_BURST`／`MERCHANT_POD_MISSING`／
`MERCHANT_UNDELIVERABLE_LIABILITY`。

🔴 **這是 D-1 的義③（事件驅動建案 ＋ ADM 人工處置），不是常駐掃描器。**
本書**不建立**任何 `*ReconciliationJob`／`*BackfillCLI`／drift scanner（《後端服務責任邊界規範》的禁令）。
MD.rec 不是掃描器——它的候選集合是「有具體外部呼叫要做的那幾筆」，處理完就離開述詞。

### 7.7 並發與順序（同一 job 的多來源更新）

三個來源可能同時寫同一筆：①商家事件 ②賣家在 App 手動 ③ADM。
防線**三道，缺一不可**：

1. **`event_seq_watermark`**（只擋商家來源的亂序）
2. **`ShipmentStatusMapper.isValidTransition`**（擋非法轉移；🔴 `930` 屬 `IN_DELIVERY`，
   否則 `933→930`／`930→31` 會被拒——65 §7.1.2.5 的集合改動表逐字要求）
3. **樂觀鎖 CAS**（既有 `updateShipmentStatus(id, fromStatus, toStatus, …)`，`rows==0` 即放棄）

🔴 **`rows == 0` 的處置在本書是「記 log ＋ counter，不 rollback」**：C 軸是稽核側寫，
權威是「誰先寫進去誰贏」。**但 job 軸的 `rows == 0` 必須 rollback 整個交易**——
job 與 C 軸不同步就是 INV-M1 破了。**兩者的處置不同不是不一致，判準是「壞掉會不會讓證據或錢出錯」**。

⚠️ **`accept_due_at` 與既有 `orders.processing_deadline` 是兩個時鐘**，
不得互相冒充：後者是「賣家多久內要出貨，否則系統取消訂單」，前者是「商家多久內要接單」。
`accept_sla_minutes ≤ 120` 的上界就是為了讓前者永遠早於後者觸發。

### 7.8 靜默陷阱表

| # | 陷阱 | 防線 |
|---|---|---|
| B1 | `930` 被移出 `ShipmentStatusMapper.IN_DELIVERY` | `930 → 31` 被拒，日誌只有一行 `unknownTransition` warn（**不報錯**）⇒ §13.4 變異 V1 釘住它 |
| B2 | 商家事件推 E 軸 | §13.2 有一支專門的變異：把 `DELIVERED` 的處理改成順便寫 E 軸 ⇒ 必須轉紅 |
| B3 | POD 抓取沿用 best-effort catch | §8.3 fail-closed ＋ 測試 |
| B4 | `payload_filter` 寫 enum 名 | 契約測試比對 wire 值 |
| B5 | 降級後沒有人通知賣家 | 降級同時發通知（既有 `notification.order` `#` 綁定，零 topology 改動） |
| B6 | `merchant_delivery_profile` 改了但 job 讀 current 而非快照 | `profile_version_id` 為 `NOT NULL` ＋ 讀路徑一律走版本表 |

---

## 8. 跨切面關注點

### 8.1 安全與授權

| 面向 | 做法 |
|---|---|
| **SSRF**（🔴 本書最高風險） | 商家 endpoint 是使用者可控的出站目標。**每次呼叫前重新解析 DNS 並比對**：拒絕 private（10/8、172.16/12、192.168/16）、loopback、link-local（169.254/16，**含雲端 metadata**）、IPv6 ULA；只允許 `https`；禁止 redirect 跟隨 |
| per-merchant scope | JWT 的 principal 就是該賣家 ⇒ 所有查詢都被既有的 `store_id` 歸屬檢查擋住（`getAndVerifyShipment(orderNo, storeId)` 是既有形狀） |
| replay | `nonce` ＋ `±300s` 時間窗 ＋ `(store_id, event_key)` 去重 |
| 常數時間比較 | 沿用 `subtle.ConstantTimeCompare` 的形狀（Java 用 `MessageDigest.isEqual`） |
| IP allowlist（我方對商家） | ❌ **不做**：商家 endpoint 是動態的，靜態白名單無法表達 |

⚠️ **NetworkPolicy 幫不了忙**：shipping-service **沒有** egress policy，
而就算加了，也不可能列舉所有商家的 IP ⇒ **SSRF 防護 100% 是應用層責任**。

### 8.2 可觀測性

| 指標 | 型別 | 守什麼 |
|---|---|---|
| `fulfillment_state_age_seconds{axis="merchant_job",state}` | gauge（sql-exporter） | **每個非終態一條 aging**（65 D-17 的基準線） |
| `merchant_outbound_attempt_total{result}` | counter | 送不到 |
| `merchant_outbound_abandoned_total` | counter | 放棄門檻被打到 |
| `merchant_inbound_event_total{type,result}` | counter | result ∈ `applied`／`duplicate`／`out_of_order`／`sig_failed` |
| `merchant_event_seq_gap_total` | counter | 商家漏發事件 |
| `merchant_health_check_failure_total` | counter | 商家端故障 |
| `merchant_integration_degraded_rows` | gauge | 目前有幾家在降級 |
| `merchant_pod_missing_rows` | gauge | POD 逾期未到 |

🔴 **D-17 的告警四問，逐條回答**（每一條告警都要答）：
①**它什麼時候響**②**它什麼時候會靜默**③**響了誰去做什麼**④**它自己死掉誰知道**。
例：`merchant_integration_degraded_rows` 在**所有商家都被降級**時是一條高原線，
而「高原」不會觸發變化率型告警 ⇒ 必須用絕對閾值 ＋ `absent()`。

### 8.3 錯誤處理與降級

🔴 **商家整合故障 ⇒ 降級為 L0 人工，而不是 fallback 到其他 provider。**
換 provider 會改變買家在結帳時看到的配送方式與運費，那是**契約變更**不是容錯。

🔴 **POD 收集是 fail-closed，與既有出貨照片不同。**
既有 `processShipmentPhotos` 的 catch 逐字是「出貨照片處理失敗（不影響主流程，本次視為未附照片）」——
那對「出貨照片」是對的（它是附加資訊）；對 **POD 是錯的**（它是爭議裁決的唯一證據）。
⇒ POD 路徑：抓取失敗 ⇒ **不吞**，落 `merchant_pod_artifact` 的失敗記錄 ＋ counter ＋ 重試 3 次 ＋
仍失敗則建 case（`MERCHANT_POD_MISSING`）。**狀態照常推進**（貨確實到了），但證據缺口被記下來。

### 8.4 一致性與冪等

三個機制、三個不同的鍵，**不要混**：

| 機制 | 鍵 | 擋什麼 |
|---|---|---|
| `Idempotency-Key`（我方端點） | 呼叫端產生 | 同一個請求被送兩次 |
| `eventId`（inbound） | 商家產生 | 同一則事件被送兩次 |
| `sequence`（inbound） | 商家產生，per shipment | 舊事件覆蓋新事件 |

**idempotent 202 的語意**：`ACCEPTED` 重複送回 `200 + code 0 + 既有結果`，
**不是** 409。理由：商家的重試是我方契約要求的（M4），拿它當錯誤會逼商家寫特例。

### 8.5 設定與機密

**新增 secret 群組 `hawkr-merchant-delivery`**，key：
`MERCHANT_ENCRYPTION_MASTER_KEY`、`MERCHANT_ENCRYPTION_MASTER_KEY_PREV`、`MERCHANT_HMAC_LOOKUP_SECRET`。

七件事一件不能少（65 附錄 G.10）：
① `k8s/scripts/create-secrets.sh` 加群組（插在 `# 9. GHCR pull secret` 之前）
② `SCRIPT_REVISION` +1
③ `k8s/values/shipping-service.yaml` 加 `secretRef`（行內註解另起一行）
④ `docker-compose-local.yml` 加 `environment:` 三行
⑤ `.env.template` 加對應行（🔴 **零 gate**）
⑥ dev／prod 兩台各跑一次 `create-secrets.sh`
⑦ `k8s/SETUP.md` 同步（🔴 零 gate 且已經漂了）

🔴 **`MERCHANT_ENCRYPTION_MASTER_KEY` 不得標 `optional: true`**：
它是 fail-closed 的（沒有 key 就無法驗簽也無法解密憑證），
而 `optional` 會讓 `--live` 對「整支 Secret 缺席」只印 notice、不影響退出碼。
⚠️ 也**不能只靠宣告 gate**：`--from-literal=K=` 在 `.env` 缺 key 時仍會建立一個**空字串**的 key，
所有 gate 都認為它存在 ⇒ 服務端必須 fail-closed（空值即拒絕啟動）。

🔴 **啟動檢查（fail-closed）**：`MERCHANT_ENCRYPTION_MASTER_KEY` 缺席或為空字串
⇒ **服務拒絕啟動**。不得降級成「啟動後第一次加密才失敗」——那會讓一批 profile 以明文或半套狀態落庫。

### 8.6 PII 與 log 遮罩

| 值 | 分類 | 處置 |
|---|---|---|
| 買家地址／電話 | PII | outbound payload 必須帶（配送需要）；**log 一律遮罩**；`raw_payload` 入庫前遮罩 |
| POD 簽收人姓名 | PII | 加密欄 |
| **PIN 碼** | 🔴 **憑證**（單獨持有即可冒領） | **不落 log、不落 `raw_payload`**，只存 `pin_verified` 布林 |
| 商家 secret／`key_id` | 憑證 | secret 只存密文；`key_id` 可落 log（它是公開識別字） |

🔴 **遮罩清單是「值的清單」不是「欄位位置的清單」**（gateway `middleware/logger.go` 的 `sensitiveKeys`／`redactBody`／`redactQuery`）：
同一個值可從 body、query、path、header 進來，**漏掉任一條就等於沒遮**。
本書的三個值（`phone`／`pinCode`／`signerName`）要同時登記進 gateway `sensitiveKeys`
與 shipping-service 自己的遮罩器——**gateway 遮了不代表下游遮了**。

### 8.7 靜默陷阱表

| # | 陷阱 |
|---|---|
| C1 | 只在 gateway 遮罩，shipping-service 自己記一份完整 payload |
| C2 | `optional: true` 用在 fail-closed 的金鑰上 ⇒ 把 fail-closed 降級成靜默失能 |
| C3 | SSRF 檢查只在「登記 endpoint 時」做一次 ⇒ DNS rebinding 直接繞過 |
| C4 | 降級後的通知走 `reference_id_template = order_no` ⇒ 同一訂單第二則被 dedup index 靜默吞掉 |

---

## 9. 品質需求與 NFR

### 9.1 SLO／SLI

| SLI | 目標 |
|---|---|
| outbound 首次送達率（2xx / 總數） | ≥ 99%（商家端可用性決定，超出我方控制 ⇒ 只做觀測不做承諾） |
| outbound **最終**送達率（含重試） | ≥ 99.9% |
| inbound 事件處理延遲（收到 → 狀態寫入） | p95 < 2s |
| POD 到齊率（送達後 24h 內） | ≥ 95% |

### 9.2 容量與規模

以 dev 現況推估：3005 的 job 量 ≈ 該賣家訂單量。
inbound 事件約 4 則／job；outbound 1 則／job（＋重試）。
`merchant_event_inbox` 30 天保留 ⇒ 列數 ≈ 月訂單 × 4。

### 9.3 硬上限依賴治理

🔴 **商家端的 rate limit 是「對方的」上限** ⇒ 我方要有預算（65 附錄 G.9）。
接入前置三件缺一不得接上任何環境（**含 dev**）：
① 用量＋上限**兩個 series**（`merchant_outbound_attempt_total` ＋ profile 宣告的 `rate_limit_per_min`）
② 逼近上限與觸頂副作用**各一條**告警
③ 預算數字寫進本節（現況、單位成本、能撐多少、何時逼近）
＋ 補《硬上限依賴接入治理規範》§5 登記表一列。

⚠️ **我方對商家的 rate limit 也要登記**（60 req/min）：它用滿了會回 429 ⇒ 適用判準成立。

### 9.4 效能預算

`QuoteMerchantDelivery` 落在結帳熱路徑 ⇒ **3s deadline，逾時即類 C（整筆請求失敗，可重試）**。
🔴 **不得 fail-open 成「用預設運費」**——那會讓買家付到錯的錢。

### 9.5 資料保存與清理

| 對象 | 保留 | 誰刪 |
|---|---|---|
| `merchant_event_inbox` | 30 天 | MD.gc |
| `merchant_outbound_delivery` | 90 天（`DELIVERED`／`ABANDONED`） | MD.gc |
| POD 影像 | 🔴 **不隨 GC 刪**——`AssetReferenceType.MERCHANT_POD` 走獨立的保存政策 | 手動／未來的保存期政策 |
| profile version | 不刪 | — |

🔴 **POD 不能沿用 `SHIPPING(7104)` 這個 refType**：既有 `SHIPPING` 的引用在配送完成時會被
`labelService.unlinkLabel` 解除，media-service 的 GC 7 天後刪掉物件——
**那正好是爭議窗口（D+7 ＋ 人工裁決）最需要它的時候**。

---

## 12. 部署與維運

### 12.1 環境與設定

**新增設定必須同步五處**（漏任一處的後果都不同，逐條列出）：

| # | 位置 | 漏了會怎樣 |
|---|---|---|
| 1 | `.env.template` | 🔴 **零 gate** ——新環境照它建 `.env` 會少 key，接著落入第 2 列 |
| 2 | `k8s/scripts/create-secrets.sh`（憑證類）＋ `SCRIPT_REVISION` +1 | 靜態 gate exit=1 擋 PR；⚠️ **`--from-literal=K=` 在 `.env` 缺 key 時仍會建立空字串 key**，所有 gate 都認為它存在、pod 正常啟動、runtime 才 401 |
| 3 | `k8s/values/merchant-delivery` 相關 config／`secretRef` | 漏了 ⇒ key 變孤兒（只 warning 不擋），pod 讀不到值 |
| 4 | `docker-compose-local.yml` 的 `environment:` | 對齊 gate exit=1 擋 PR |
| 5 | `k8s/SETUP.md` 的 Secret key 表 | 🔴 **零 gate 且已知會漂**——「照 SETUP.md 重建環境」會漏掉本書的 key |

**本書需要的設定**

| key | 性質 | 說明 |
|---|---|---|
| 商家 API 的 HMAC 簽章金鑰（current／previous 一對） | 🔴 **憑證，fail-closed** | 缺值即**拒絕啟動**，不得標 `optional`——`--live` 對「整支 Secret 缺席但引用的都是 optional」只印 notice、不影響退出碼 |
| OAuth2 client 註冊資料 | 憑證 | per-merchant，存 secret manager reference 不落庫 |
| 失聯回查窗口／accept SLA／retry 階梯 | 一般 config | 值的 SSOT 在 §0.2；⚠️ **不要在 values 與設計書各寫一份** |

⚠️ **輪替期間的靜默失能**：某台機器的 `.env` 缺 `_PREV`，重跑 `create-secrets.sh` 會把
**叢集裡既有的那個 key 一併移走**（SSA 的語意是「宣告的即全部」）
⇒ 輪替前先確認兩台機器的 `.env` 都有 `_PREV`。

### 12.2 遷移步驟（同 66 §12.2 的形狀）

```
① CodeMeta：ShippingMethod +3005、新 enum、AssetReferenceType +MERCHANT_POD、錯誤碼追加
     → ./generate.sh --all → 四份前端拷貝 → 編譯驗證                    （一律 --all，不要自己判斷）
② Flyway（取號前現查目錄，§5.4）：**六張新表** ＋ `chk_shipping_type` 擴到含 `'3654'`（M6-2）
     🔴 **不含 `chk_shipping_method`**（M6-1：它的擁有者是 65；`DROP/ADD` 形式的 CHECK 只留最後寫的那一份）
③ shipping_method_configs 三列（3005 × 3201/3202/3203），建立時 enabled = false ＋ 費率分派點
④ 程式碼：**adapter 實作（十一個 capability，逐成熟度解析；`reportReady()` 對 3005 為 `empty`）**／建店豁免分支／`seller_shipment_settings` 的 `3654` 列同步（§5.1.9 (a)）
⑤ 通知模板列（payload_filter 用 wire 值）＋ golden ＋ 契約測試
⑥ 開 flag：先把 ③ 那三列 enabled 切成 true → L0 → 觀察 → L1 → 觀察 → L2
```

🔴 **①→②→③ 的順序不可換**：先擴 CHECK 才寫得進去。
🔴 **③ 的設定列走 `infra/seed-data/`，不寫進 migration**（《Java 微服務開發規範》：seed 資料禁止寫進 migration）——65 §5.8.3 ② 與 66 §12.2 步 1 同此機制。
⚠️ **③ 建列（`enabled=false`）本身並不會讓系統脫離 `CONFIG_MISSING`**——見下一段：
`ShippingFeeService` 的快取載入帶 `enabled = true` filter ⇒ `enabled=false` 的列對快取**與不存在完全等價**。
③ 的意義是「把列建好、等 ⑥ 切開關」，不是「這一步之後就不會落類 B」。
🔴 **④ 之前絕對不能把那三列切成 `enabled = true`**：程式碼還沒到位就開啟，D-13 描述的靜默 fallback 立刻變 live bug。
⚠️ **錨在「開啟」不在「建列」**——以 `enabled = false` 建立的列是惰性的（`hasAnyEnabledConfig("3005")` 仍為 false，賣家看不到槽位），
所以 ③ 可以、也應該早於 ④；真正不可逆的是 ⑥ 的第一個動作。

**`REMOVE-WHEN` 承諾**：
- `DYNAMIC_QUOTE` 的關閉旗標：`REMOVE-WHEN: 所有 ACTIVE profile 的 pricing_mode 皆非 DYNAMIC_QUOTE 已滿 90 天`
- v1 對外契約：`REMOVE-WHEN: SELECT count(*) FROM merchant_delivery_profile WHERE api_version='v1' AND status='ACTIVE' = 0`

### 12.3 健康檢查與優雅關閉

沿用 shipping-service 既有的 `/actuator/health/readiness`／`liveness` 分離
（readiness 含 db／redis／rabbit，liveness 只看 livenessState）。
MD.out／MD.rec 的關閉要等當前那一筆的 HTTP 呼叫結束（≤ 10s deadline）。

### 12.4 Runbook

| 症狀 | 先查 | 處置 |
|---|---|---|
| 商家說沒收到訂單 | `merchant_outbound_delivery` 的 `attempt_count`／`last_http_status` | 4xx ⇒ 商家端問題；逾時 ⇒ 查 endpoint DNS |
| 商家說送了但平台沒動 | `merchant_event_inbox` 的 `signature_ok`／`process_status` | `sig_failed` ⇒ 對 canonical；`DUPLICATE` ⇒ 對 `seq` |
| 大量驗簽失敗 | 是否在輪替窗口內 | 檢查 pending 三聯是否齊；🔴 **不要把 `_PREV` 從 `.env` 拿掉再跑 `create-secrets.sh`**——SSA 會把叢集裡既有的 key 一併移走 |
| POD 一直缺 | `merchant_pod_missing_rows` | 通知賣家；D+7 前補不上就走 ADM 裁決 |
| 降級解不掉 | `merchant_health_check_failure_total` | MD.hc 需連續成功 3 次；既有 job 不自動切回（§7.5） |

### 12.5 一次性資料處理的分界

本書**不新增**任何 `*ReconciliationJob`／`*BackfillCLI`／drift scanner（《後端服務責任邊界規範》）。
存量清整（例如降級期間卡住的 job）走**一次性維運 SQL**，且必須先跑 65 §3.11 的四問——
特別是「我要寫的這個值有沒有執行者」與「同一交易要不要補一列狀態歷程」。

---

## 13. 測試策略

### 13.1 分層與落點

| 層 | 落點 |
|---|---|
| 單元 | 驗簽 canonical、seq 合併、映射表、SSRF 判定 |
| 契約 | outbound payload ↔ 公開文件；inbound fixture ↔ sandbox 真實回應 |
| 整合（Java） | adapter 建單（`requiresProviderCall=false`，純本地）→ outbound row → inbound 事件 → 狀態鏈。🔴 **三成熟度各跑一次**：L0 無 outbound／無 inbound、L1 無回查、L2 全鏈 |
| 端到端（dev） | 三種成熟度各一條 |

### 13.2 契約測試

🔴 **fixture 一律不得手寫**（65 §13.3）。本書的兩側各有不同解法：

| 對象 | fixture 來源 |
|---|---|
| **我方 → 商家**（outbound payload） | 從**我方 producer 的原始碼**生（`MerchantOutboundPayload` 的欄位反射），公開文件反過來由它產出 |
| **商家 → 我方**（inbound） | 🔴 **對端是第三方，沒有原始碼可讀** ⇒ 用 **hermetic fake ＋ sandbox 真實回應存下來的 golden payload**。**不得**手寫一份「我覺得商家會送成這樣」 |
| MQ payload | **三方逐欄對拍**：producer 實發 ↔ consumer 宣告 ↔ 36 號 catalog。🔴 **fixture 一律從對端原始碼讀出欄名與型別**，禁止手寫「我覺得對方會發成這樣」——手寫 fixture ＝自己跟自己對，必然全綠。正例見 `services/invoice-service/internal/mq/producer_contract_test.go`（從 Java record 原始碼解析欄名，Java 檔搬家就 fail-loud） |

🔴 **fake 的鑑別力來自它拒絕什麼**：merchant fake 必須只接受唯一正解的簽章、
其餘一律回 4xx；且 `store_id` 與 `key_id` 要取**不同的值**，否則 INV-M6 填錯也會通過。

⚠️ **斷言的期望值一律硬寫字面值**，不得呼叫被測函式或它引用的常數——
否則期望值會跟著被測物一起變，那支測試驗不到任何東西。

### 13.3 覆蓋要求

三種成熟度各一條端到端；另外必測：亂序、重播、簽章失敗、`seq` 跳號、輪替窗口內新舊兩把 key、
outbound 6 次耗盡後降級、`REJECTED → CANCELLED` 的退款鏈接上。

### 13.4 鑑別力驗證（變異測試表）

| # | 改哪一行 | 預期哪一支轉紅 |
|---|---|---|
| V1 | 把 `930` 從 `ShipmentStatusMapper.IN_DELIVERY` 拿掉 | `930 → 31` 的轉移測試 |
| V2 | 把 `DELIVERED` 的處理加上「順便寫 E 軸 `BUYER_CONFIRMED`」 | 🔴 **收貨 Gate 測試**（INV-M2） |
| V3 | 把 `store_id` 改成從 payload 讀 | INV-M6 的跨商家測試 |
| V4 | 把去重移到驗簽之前 | 「假事件佔用 eventId」測試 |
| V5 | 把 `supported_temps` 的空陣列當成常溫 | INV-M3 |
| V6 | 把 POD 抓取改回 catch-all 吞例外 | §8.3 的 fail-closed 測試 |
| V7 | 把 job 的 `rows==0` 改成不 rollback | INV-M1 |
| V8 | L0 建單時落 `PENDING_DISPATCH` | 「L0 沒有推走者」測試（§7.3.1 的紅字） |

🔴 **每一條變異都要真的跑一次看它轉紅**。沒轉紅時三選一並附證據：
測試弱（缺哪個斷言）／變異無鑑別力（哪條冗餘防線接住，`檔案:符號名`）／派工單的預期錯了。

### 13.5 測試資料與環境防線

sandbox 的 `key_id` 前綴 `sbx_`，資料不進正式庫；測試商家 endpoint 只允許 `localhost` 的 fake
（🔴 **這個豁免只在 test profile 生效**，否則就是把 SSRF 防線關掉）。

### 13.6 與既有 CI gate 的關係

| gate | 本設計是否觸發 |
|---|---|
| CodeMeta drift（`--all` 逐位元） | ✅ 觸發（新 enum ＋ 錯誤碼 ＋ 四份前端拷貝） |
| `proto-generated-diff` ＋ numeric lint | ✅ 觸發（新 RPC） |
| MQ consumer ↔ topology 登記一致性 | ❌ 未新增 queue |
| `test_expand.py` 資源數量 pin | ❌ 不動（§6.4） |
| gateway JSON contract（Check B） | ✅ 觸發（新端點） |
| Swagger 註解 | ✅ 每支新端點都要 |
| 冪等覆蓋 AST gate | 🔴 **需新增一支對稱的**（既有那支的觀測窗只有 `routes_invoice.go` 這一個檔） |

---

## 14. 術語表

### 14.1 商家整合專有名詞

| 詞 | 意思 |
|---|---|
| **成熟度（tier）** | L0／L1／L2，描述商家系統能做到什麼，**不描述證據可信度** |
| **降級（degrade）** | 整合不可用時切回 L0 人工，**不是停用配送方式** |
| **重疊窗口** | 新舊兩把 secret 同時有效的期間 |
| **watermark** | 該 shipment 已套用的最大 `sequence` |
| **POD** | Proof of Delivery，交付證明。🔴 **它是給裁決用的證據，不是通過收貨 Gate 的鑰匙** |

### 14.2 同一件事的多個名字（job／shipment／delivery）

| 詞 | 指什麼 | 權威表 |
|---|---|---|
| **shipment** | 平台的配送單，**所有 provider 共用**的那一層 | `shipments`（C 軸） |
| **job** | 商家自送**特有**的整合任務 | `merchant_delivery_job` |
| **delivery** | 口語，指「這一趟配送」 | 🔴 **不要在識別字裡用它**——它同時可能指 shipment、job 或 Uber 的 `del_` 物件 |

---

## 附錄 C：CodeMeta 與碼表新增清單

> 🔴 **本附錄是碼值清單，不是施工單**。落碼流程照 65 附錄 C.6；
> 落碼流程照 65 附錄 C.6 一次做完（🔴 一律 `generate.sh --all`，不要自己判斷哪些輸出會變）。

**C-1 `ShippingMethod`**：`MERCHANT_DELIVERY(3005)` — **由 65 附錄 C.1 定案，本書只引用，不重複登記**。

**C-2 `ShippingType`**：✅ **新增 `MERCHANT_DELIVERY`，Code ＝ `3654`** —— 🔴 **由 65 附錄 C.1.1／C.2 定案，本書只引用不得自行裁決**（沿革與被推翻的三條理由見 §5.1.9）。

| Name | Value | Label | 給誰 |
|---|---|---|---|
| `CVS_B2C` | 3653 | 超商大宗寄倉 | 68 |
| **`MERCHANT_DELIVERY`** | **3654** | **商家自行配送** | **本書** |

🔴 **不是「兩本共用同一次擴充的同一個值」**——那個舊說法會直接重演 `buildSlotKey` 撞號。
兩本共用的是**同一次 `業務類型碼.md` 變更與同一次 `generate.sh --all`**（兩個值要同批進去，
各改一次會讓後跑的生成覆蓋前一個），**值本身各自獨立**。
⚠️ 連帶必須擴 `generate.sh` 的 `ADMIN_ENUMS`（該陣列目前沒有 `ShippingType`）——
這一次擴充同時服務 `3653` 與 `3654`，成本共用（65 附錄 C.1.1 連帶項第 5 條）。

**C-3 `MerchantDeliveryJobStatus`（新，8 值）**

| Name | Value | Label |
|---|---|---|
| PENDING_DISPATCH | 4501 | 待通知商家 |
| NOTIFIED | 4502 | 已通知商家 |
| ACCEPTED | 4503 | 商家已接單 |
| IN_FULFILLMENT | 4504 | 配送中 |
| DELIVERY_REPORTED | 4505 | 已回報送達 |
| REJECTED | 4506 | 商家無法履約 |
| CANCELLED | 4507 | 已取消 |
| CLOSED | 4508 | 已結案 |

號段 `4501~4519` 由 65 附錄 C.2 指派給 `MerchantDeliveryJobStatus`。
🔴 **本設計只引用不得自行擴段**：上表用掉 `4501~4508`，`4509~4519` 是**保留區**；
需要更多值時回 65 附錄 C.2 追加，**不得就地往 `4520` 撿**。
⚠️ 68 的 batch 狀態**不在這個號段內**——它沿用 65 §5.2.2 的 `chk_batch_status`、不新增 enum（65 附錄 C.2）。

**C-4 `MerchantIntegrationTier`（新，3 值）**：`L0_MANUAL`／`L1_WEBHOOK`／`L2_API`。

**C-5 `MerchantEventType`（新，8 值，provider 原生值照抄）**：
`ACCEPTED`／`PICKED_UP`／`IN_TRANSIT`／`DELIVERED`／`REJECTED`／`UNDELIVERABLE`／`RETURNED`／`CANCEL_ACK`。
⚠️ **它不是狀態軸**，只是「商家送來的字串的合法集合」。未知值 ⇒ **9526** ＋ counter ＋ case。

**C-6 `AssetReferenceType` 擴充**：`MERCHANT_POD`（🔴 **Code 取號前現查 `AssetReferenceType` 現況最大值**；既有 `OTHER(7199)`、
次大 `INVOICE_DOCUMENT(7120)`）。
🔴 **`Code` 決定 R2 上的實體儲存路徑**（該表的紅字逐字），
⇒ POD 走 **secure 前綴**（比照 `INVOICE_DOCUMENT` 經 `ServerUploadSecureAndFinalize` 寫入
`secure/invoice_document/`），只供授權下載端點以 `GetSecureAssetInternal` 取用。

**C-7 `FulfillmentExceptionCategory` 擴充（6 值）**：§7.6 列出的六個。

**C-8 錯誤碼**：§6.1.5（10 條）＋ §6.2.7（3 條）＝ **13 條語意**。
🔴 **號碼一律由 65 §6.7 統一指派，本設計不自行取號**。
⚠️ 本書的「統一 envelope」決策讓 65 附錄 C.5.1 的「上調 A」成立 ⇒ 號段餘裕要重算。

---

## 附錄 D：前端契約與交付對照（三 repo）

| repo | 要做的事 |
|---|---|
| `hawkr_seller`（賣家 App） | 商家自送 profile 設定頁、job 詳情與 `allowedActions[]`、M-1～M-5 的操作、POD 上傳、降級橫幅 |
| `hawkr_buyer`（買家 App） | 配送方式顯示「商家自行配送」；🔴 **禁止 `provider → 預設值` 的二元 fallback**（既有 `order_bottom_row.dart` 的 `_sevenElevenSteps` 是要改的形狀） |
| `hawkr-admin`（ADM） | M-16～M-20；case 佇列的商家類 category |
| `hawkr-website` | 買家端同 buyer App；賣家端同 seller App |

🔴 **CodeMeta 四份拷貝的比對嚴格度不同**：website／ADM 逐位元，兩個 Flutter 只比對 `(code, value, label)`
⇒ 只改 description 時 Flutter 的 gate 會過但拷貝其實已不同步。
🔴 **未知 normalized 值一律顯示「未知狀態」＋原碼，不猜測**（65 §7.7.4.5）。

畫面細規歸 **65A**，本附錄只列交付對照。

---

## 附錄 E：商家整合指南（對外文件的內容清單）

> 這一節不是設計書內容本身，而是列出「要另外交付給商家的公開文件」要包含什麼，
> 並指明它的 **SSOT 是本書 §6.2**（避免對外文件與設計書漂移）。

1. 快速開始（申請 → 拿 `key_id` ＋ secret → sandbox 自測 → 上線）
2. 簽章計算式**逐字**（canonical 四段、hash 演算法、編碼、時鐘容忍）＋ **三種語言的範例**
3. outbound payload schema ＋ 每個欄位的型別與可空性
4. inbound 事件表（8 種）＋ 每種的必填欄
5. 冪等與順序的三條規則（§8.4）
6. 錯誤碼表（§6.2.7）＋ 「哪些該重送、哪些不該」
7. secret rotation 的六步程序
8. rate limit 與 sandbox 的差異
9. 自測清單（§6.2.8 的 10 個案例）
10. 版本策略與棄用政策（§4.5）

🔴 **對外文件的每一段都要能指回本書的節號**。找不到出處的段落就是漂移的起點。
