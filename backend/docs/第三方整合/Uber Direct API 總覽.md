# Uber Direct API 官方文檔（統整版）

> **最後更新**：2026-09-14（Stage 8B Step 5P-0 Provider Contract Reconciliation）
> **涵蓋範圍**：認證、Organizations、報價、建單、查狀態、取消、POD、webhook 驗簽與事件、Robocourier、Refund Submission
> **消費者**：[66 號 Uber Direct 即時配送整合詳細設計書](../詳細設計書/66-Uber%20Direct%20即時配送整合詳細設計書.md)、[70 號 物流財務結算詳細設計書](../詳細設計書/70-物流財務結算（Logistics%20Settlement%20%26%20Receivable）詳細設計書.md)

---

## 0. 來源權威序（🔴 先讀這一節，否則整份檔會被誤用）

**本檔的每一條事實都標來源等級。等級不同，能不能寫進 `shipping_method_configs`／能不能當契約，結論完全不同。**

| 等級 | 來源 | 效力 |
|---|---|---|
| **T1** | `developer.uber.com` 現行 guides ＋ **`uber/uber-direct-sdk` 的現行 `openapi.yaml`**（Uber 自己維護與發佈的 SDK spec） | **API protocol / schema / security 的最高權威**。與任何其他來源衝突時，T1 勝 |
| **T1-c** | `developer.uber.com/docs/deliveries/changelog` | 同 T1，且是「某能力何時開始存在」的唯一權威 |
| **T2** | Uber 台灣業務／PE 文件《**Uber Direct 開發指南 v2.4**》；Uber 針對本帳號的**書面**確認 | 台灣在地細節、營運門檻、帳號層設定的來源。**不得覆蓋 T1 的 protocol/schema/security**；但**帳號層 billing／商業設定不得反過來用 T1 的通用文件自行覆蓋** |
| **T3** | 已 merge 的 Hawkr Design／ADR／PO 裁決 | 我方決策 |
| **T4** | 歷史稽核、推導 | 只能當 radar，不得當證據 |

🔴 **T2 已知的三類問題（實查坐實，不是懷疑）**：
① **拼寫錯誤**——PDF p.60 的 `MERCHANT_TYPE_GROERY`，T1 是 `MERCHANT_TYPE_GROCERY`；
② **過度嚴格**——PDF p.27 宣稱 CreateOrg「以下參數皆為必填」，T1 的 `required` 只有 `name` ＋ `billing_type`（＋`parent_organization_id`）；
③ **過時**——PDF p.80 把補償描述成「聯繫 Uber 客服申請」，而 T1-c 記載 **2025-10-27 已上線 Refund Submission API**。
⇒ **引用 PDF 的任何一條之前，先在本檔找對應的 T1 欄位。**

⚠️ **本檔不是全文翻譯**，實查來源列在 §13。沒列到的代表**還沒查**，不代表「官方沒有」。

🔴 **本輪（2026-09-14）的取證管道限制，誠實標註**：`developer.uber.com/docs/deliveries/api-reference/*`
是前端渲染頁，`curl` 取回 404、WebFetch 只取得導覽列 ⇒ **欄位級事實改由 `uber/uber-direct-sdk` 的
`src/deliveries/openapi.yaml`（Direct API）與 `src/organizations/openapi.yaml`（Organizations API v1.0.1）取得**。
兩者是 Uber 官方帳號維護的現行 spec，本檔視為 T1；但**「與線上 reference 逐位元同步」未經證實** ⇒
欄位語意有疑義時仍以 sandbox 實測為終局。

---

## 目錄

0. [來源權威序](#0-來源權威序先讀這一節否則整份檔會被誤用)
1. [通用規範](#1-通用規範)
2. [Direct API — 報價與建單](#2-direct-api--報價與建單)
3. [查詢配送狀態](#3-查詢配送狀態)
4. [取消](#4-取消)
5. [Proof of Delivery](#5-proof-of-delivery)
6. [Webhook](#6-webhook)
7. [狀態值域](#7-狀態值域)
8. [Organizations API（三層式架構）](#8-organizations-api三層式架構)
9. [Robocourier（sandbox 自動接單）](#9-robocouriersandbox-自動接單)
10. [Refund Submission API](#10-refund-submission-api)
11. [T2 台灣指南的營運契約](#11-t2-台灣指南的營運契約pdf-v24)
12. [與 66 號設計書的對照](#12-與-66-號設計書的對照)
13. [已實查來源](#13-已實查來源)

---

## 1. 通用規範

### 1.1 認證（T1）

| 項目 | 值 |
|---|---|
| 端點 | `POST https://auth.uber.com/oauth/v2/token` |
| 機制 | OAuth 2.0，`client_credentials` grant |
| 憑證來源 | Direct Dashboard 的 `client_id` / `client_secret` |
| scope | Direct API＝`eats.deliveries`；**Organizations API＝`direct.organizations`** |
| token 有效期 | **30 天**（2,592,000 秒） |
| 🔴 token 發放速率 | 逐字：「Client credential grant type requests will be rate limited to **100 requests per hour**.」 |

🔴 **token 必須快取**。每次請求重新取 token，光這一項就會撞 100/hr 上限。
⚠️ **兩個 scope 是分開的** ⇒ Organizations API 的 token 與 Direct API 的 token **不是同一個**，
快取要**按 scope 分槽**。拿 `eats.deliveries` 的 token 打 CreateOrg 會 401，而那個 401 看起來像憑證壞掉。

### 1.2 來源 IP / 網路（🔴 兩個方向必須分開談）

**這一節在 2026-09-14 被更正**：舊版把兩件事混成一句「Uber 不需要 IP 白名單」。

| 方向 | 事實 | 來源 | 我方處置 |
|---|---|---|---|
| **Uber → 我方（webhook inbound）** | Uber **不提供固定來源 IP／網域**，改以 signing key 認證（PDF p.14 逐字：「Uber 服務主機群不定時重整和擴增，我們無法提供您固定的 ip 位置或網域進行防火牆設定」） | T2 | **不得**對 webhook 端點做來源 IP 白名單；改以 §6.1 的驗簽為唯一閘門 |
| **我方 → Uber（outbound API）** | 認證指南全篇**未提及**要求呼叫端固定出口 IP；認證是純 OAuth bearer | T1（未提及） | 預設**不走 `egress-proxy`**（10 檔 §26） |

🔴 **不得從左欄推導右欄**：「Uber 沒有固定 IP 可以給你」與「Uber 不要求你有固定 IP」是**兩個不同的句子**，
而 PDF 那句話出現在 webhook security 的脈絡裡。
⚠️ 「文件未提及」不等於「一定不需要」：sandbox／正式環境實測若出現 403 或 IP 相關錯誤，
**一律收斂到既有 `egress-proxy`**，不得為 Uber 另開新 proxy 或新固定 IP。

### 1.3 兩組路徑家族（T1，容易搞混）

| 家族 | base | 我方是否使用 |
|---|---|---|
| **Direct（DaaS）** | `https://api.uber.com/v1/customers/{customer_id}/…` | ✅ **我方用這組** |
| **Organizations** | `https://api.uber.com/v1/direct/organizations…` | ✅ 使用（§8） |
| **Eats deliveries** | `https://api.uber.com/v1/eats/deliveries/orders/{order_id}` | ❌ 不使用 |
| **Eats orders** | `https://api.uber.com/v1/eats/orders/{order_id}/cancel` | ❌ 不使用 |

🔴 **`customer_id` 是路徑參數不是 header**，且 T1 逐字定義它是
**「Unique identifier for the organization. Either UUID or starts with `cus_`.」**
⇒ **`customer_id` ＝ Organization ID**（§8），**不是門市識別碼**。門市識別碼是 `external_store_id`。

### 1.4 Sandbox（T1）

官方逐字：「The sandbox environment is provided solely for functional testing and
integration validation. It is **not** designed or supported for load, performance, or
stress testing.」測試憑證不會產生真實配送；正式環境需通過 billing 審核。
sandbox 的完整 lifecycle 靠 **Robocourier**（§9）驅動。

---

## 2. Direct API — 報價與建單

### 2.1 端點總表（T1，路徑逐字取自現行 spec）

| # | 功能 | Method | 路徑（base `https://api.uber.com/v1`） | 副作用 |
|---|---|---|---|---|
| E1 | Create Quote | POST | `/customers/{customer_id}/delivery_quotes` | 否 |
| E2 | Create Delivery | POST | `/customers/{customer_id}/deliveries` | **是** |
| E3 | List Deliveries | GET | `/customers/{customer_id}/deliveries` | 否 |
| E4 | Get Delivery | GET | `/customers/{customer_id}/deliveries/{delivery_id}` | 否 |
| E5 | Update Delivery | POST | `/customers/{customer_id}/deliveries/{delivery_id}` | **是** |　→ 欄位與可編輯窗見 **§2.6a**
| E6 | Cancel Delivery | POST | `/customers/{customer_id}/deliveries/{delivery_id}/cancel` | **是** |
| E7 | Proof of Delivery | POST | `/customers/{customer_id}/deliveries/{delivery_id}/proof-of-delivery` | 否 |

✅ **七條路徑與 66 §4.5.2 逐字相符**（2026-09-14 以 T1 spec 覆核）。`delivery_id` 逐字「Always starts with `del_`」。

### 2.2 🔴 金額單位 — 全部是 cents（¹⁄₁₀₀ 幣別單位）

**T1 逐字（多處重複，非單一來源）**：

| 欄位 | 官方描述逐字 |
|---|---|
| `manifest_total_value` | 「Value in **cents ( ¹/₁₀₀ of currency unit )** of the items in the delivery. i.e.: `$10.99` => `1099`.」 |
| `manifest_items[].price` | 「(optional) Price in **cents ( ¹/₁₀₀ of currency unit )**.」 |
| `fee`（quote 與 delivery） | 「Amount in **cents (¹/₁₀₀ of currency unit)** that will be charged if this delivery is created.」 |
| `tip` | 「Amount in **cents ( ¹/₁₀₀ of currency unit )** … e.g.: `$5.00` => `500`.」 |
| refund `*_refund_amount` | 「… in **cents ( ¹/₁₀₀ of currency unit )**.」 |

🔴 **TWD 沒有豁免**：規則是「幣別單位的 ¹⁄₁₀₀」，不是「有小數的幣別才乘 100」。
T2 獨立佐證：PDF p.37 逐字「price 單位為"分"，因此價格請事先**乘以 100** 再傳送（例：金額為 120 元時，請傳送 12000）」；
PDF p.108 的 refund webhook 實例 `"currency_code": "TWD", "value": 7500` ＝ NT$75。

🔴 **我方的邊界規則（回灌 66 §6.5）**：
- **DB 與 domain 一律維持「元整數」**，不變。
- **只在 Uber adapter 邊界做一次換算**：出站 `× 100`、入站 `÷ 100`。
- 🔴 **Uber 的 minor unit 不得滲透進 domain／DB／MQ payload／對外 API**。
- 換算必須是一支具名函式並有契約測試；**不得寫成「TWD 係數 1」的恆等式**——那是本輪確認的缺陷。

| 欄位 | 型別 | 說明 |
|---|---|---|
| `currency` | string | 🔴 **T1 標 `deprecated`**（三字母小寫）。**不得在新程式碼使用** |
| `currency_type` | string | 三字母 ISO 4217 **大寫**。新程式碼一律用這個 |

### 2.3 Create Quote（T1）

**必填**：`pickup_address`、`dropoff_address`。

| 請求欄位 | 型別 | 備註 |
|---|---|---|
| `pickup_address` / `dropoff_address` | string | 🔴 **JSON 字串**（見 §2.4） |
| `pickup_latitude` / `pickup_longitude` / `dropoff_latitude` / `dropoff_longitude` | number | 提高定位精度 |
| `pickup_ready_dt` / `pickup_deadline_dt` / `dropoff_ready_dt` / `dropoff_deadline_dt` | RFC 3339 | 見 §2.6。🔴 **3003 V1 在報價階段就送 `pickup_ready_dt`**——報價當下算出的那個絕對時間**就是** `pickup_ready_at_snapshot` 的來源，`E2 Create Delivery` 讀同一份、不重算 |
| `pickup_phone_number` / `dropoff_phone_number` | string | 🔴 **pattern `^\+[0-9]+$`** |
| `manifest_total_value` | integer | **cents** |
| `external_store_id` | string | 🔴 見下方 |

🔴 **`external_store_id` 的兩條 T1 規則**：
① 逐字「if you utilize `external_store_id` in the Create Delivery process, you **MUST** also include this field in your Create Quote API calls.」
② 逐字「Merchants can search for this value in the dashboard, and it is also **visible on the billing details report** generated by the dashboard.」
⚠️ **長度上限**：T1-c 記載 2025-04-17「Character limit added to the `external_store_id` field」，
但現行 spec **沒有 `maxLength`** ⇒ **實際上限未知 ⇒ `SANDBOX_VERIFY`**。我方 `VARCHAR(64)` 暫不變更。

**回應**：

| 欄位 | 說明 |
|---|---|
| `kind` | `delivery_quote` |
| `id` | `dqt_…`，即建單時的 `quote_id` |
| `created` / `expires` | RFC 3339。🔴 官方範例為 **15 分鐘**後到期 |
| `fee` | **cents** |
| `currency_type` | ISO 4217 大寫 |
| `dropoff_eta` | 預估送達時間 |
| `duration` / `pickup_duration` | 分鐘 |
| `dropoff_deadline` | 送達窗結束 |

### 2.4 🔴 Structured Address（T1，本節取代舊版「樓層進 notes」的錯誤敘述）

`pickup_address` / `dropoff_address` 是**內含 JSON 的字串**（引號要跳脫）。T1 spec 的 example 逐字：

```
{"street_address":["30 Lincoln Center Plaza"],"city":"New York","state":"NY","zip_code":"10023","country":"US"}
```

`StructuredAddress` schema：

| 欄 | 型別 | T1 描述 |
|---|---|---|
| `street_address` | **array** | 逐字：「Array of street address elements. For example: **`["2000 Ocean Ave", "Floor 2"]`**」 |
| `city` | string | |
| `state` | string | |
| `zip_code` | string | |
| `country` | string | |

🔴 **樓層／房號放 `street_address` 的第二個元素，不是 `dropoff_notes`。**
T1 FAQ 逐字：「**Apartment or unit numbers are added as a separate value in the street address structure.**」
⚠️ 舊版本檔與 66 §6.5 曾寫「進 notes 不進 address」——**那是錯的，2026-09-14 更正**。

**台灣 mapping（T2，`SANDBOX_VERIFY`）**：PDF p.61 指定
**`state` ＝ 縣市**（例「台南市」）、**`city` ＝ 行政區**（例「中西區」）、`country` ＝ `TW`、`zip_code` ＝ 郵遞區號。
🔴 這個對映**與英語系直覺相反**（台灣的「市」落在 `state`），且**只有 T2 來源** ⇒ 必須 sandbox 實測確認 geocoding 成功率後才可定版。

**大樓名／社區／收件人稱謂**：PDF p.61 明示放 `dropoff_notes`，**不得放進 address**（會讓地理編碼失敗）。
⇒ 與樓層房號**分屬兩個去處**，不要混為一談。

### 2.5 Create Delivery（T1）

**必填**：`quote_id`、`pickup_address`、`pickup_name`、`pickup_phone_number`、`pickup_latitude`、`pickup_longitude`、
`dropoff_address`、`dropoff_name`、`dropoff_phone_number`、`dropoff_latitude`、`dropoff_longitude`、`manifest_items`。

**本輪新確認的選填欄位（舊版本檔標「官方未載」，現已取得 T1 定義）**：

| 欄位 | 型別 | T1 事實 |
|---|---|---|
| `pickup_business_name` | string | 顯示在外送員 App，**會覆蓋 `pickup_name`** |
| `pickup_notes` | string | **maxLength 280** |
| `dropoff_notes` | string | **maxLength 280**。逐字「accessible after the courier accepts the trip and before heading to the dropoff location」。example 為 `Second floor, black door to the right.` |
| `dropoff_seller_notes` | string | **maxLength 280**。商家額外指示 |
| `manifest_reference` | string | 顯示在外送員 App。🔴 見下方唯一性規則 |
| `external_id` | string | 逐字「To be used by **aggregators, POS systems, and other organization structures** which have an internal reference in addition to the `manifest_reference` used by your sub-account.」＋ **visible on the billing details report** |
| `manifest_total_value` | integer | cents |
| `tip` | integer | cents。🔴 逐字「The `fee` value in the Create Delivery response **includes the tip value**」 |
| `idempotency_key` | string | 🔴 見 §2.7 |
| `deliverable_action` | enum | 見 §2.8 |
| `undeliverable_action` | enum | 見 §2.8 |
| `pickup_verification` / `dropoff_verification` / `return_verification` | object | 見 §2.9 |
| `external_user_info` | object | |
| `requires_dropoff_signature` / `requires_id` | boolean | 🔴 **兩者皆 `deprecated`**，改用 `dropoff_verification` |

**`manifest_items[]`（`ManifestItem`）**：

| 欄 | 型別 | T1 事實 |
|---|---|---|
| `name` | string | 必要 |
| `quantity` | integer | minimum 1 |
| `size` | enum | `small` / `medium` / `large` / `xlarge`。🔴 逐字「**If nothing is specified, `small` will be applied by default.**」描述以「單手可拿／需購物袋／需雙手／42 吋電視」定義，**沒有 cm³ 數字** |
| `dimensions` | object | `length`／`height`／`depth`，**單位 cm**，minimum 1。🔴 逐字「`weight` is **required** when dimensions are provided」 |
| `weight` | integer | **單位 gram**，minimum 1 |
| `price` | integer | **cents** |
| `must_be_upright` | boolean | |
| `vat_percentage` | integer | 🔴 **特殊縮放**：逐字「12.5% => **1250000**」 |

✅ **`ManifestItemSize` 是描述文字 object 且未被 `ManifestItem.size` 引用** ⇒ 66 的 N4（「size 不是向量」）**T1 確認正確**。

**回應（`DeliveryResp`）重點**：

| 欄位 | T1 事實 |
|---|---|
| `id` | `del_…`。🔴 逐字「**The `id` field should be used for any identification purposes.**」 |
| `uuid` | 逐字「Alternative delivery identifier … **loses contextual information** … UUID v4 with `-` characters removed」。⚠️ **不是 API 主識別**；它的用途是與 Uber 營運端對話（T2 PDF p.63） |
| `tracking_url` | 逐字「can be used to track the courier during the delivery (**via an unauthenticated webpage**)」 |
| `status` | 7 值 enum（§7.1）；逐字「Always `pending` when the delivery is created」 |
| `courier` / `courier_imminent` | 見 §3 |
| `related_deliveries` | 🔴 見 §3.2 |
| `fee` / `currency_type` / `tip` | cents |
| `pickup_eta` / `dropoff_eta` / `pickup_ready` / `pickup_deadline` / `dropoff_ready` / `dropoff_deadline` | RFC 3339。⚠️ **這四個是回應欄，沒有 `_dt` 後綴**；請求欄是 §2.6 的 `*_dt`。欄名不同 ⇒ **不要假設回應會原樣回吐我方送出的 `pickup_ready_dt`**，要比對就逐欄比對 |
| `undeliverable_action` / `undeliverable_reason` | |
| `manifest` / `manifest_items` / `external_id` / `quote_id` / `batch_id` / `refund` / `return` | |

### 2.6 時間窗欄位（T1，本節關閉舊版的「官方未載」）

| 欄位 | T1 描述逐字要點 |
|---|---|
| `pickup_ready_dt` | 「Beginning of the window when an order **must be picked up**. **Must be less than 30 days in the future.**」<br/>🔴 同一份指南另有兩句**直接授權我方用法**的逐字：「**Send this field when preparation time is known.**」與「If you require prep-time, provide a `pickup_ready_dt` timestamp. **The courier will arrive around the `pickup_ready_dt`.**」 |
| `pickup_deadline_dt` | 「Must be at least **10 mins** later than `pickup_ready_dt` and at least **20 minutes** in the future from now.」 |
| `dropoff_ready_dt` | 「Must be **less than or equal to** `pickup_deadline_dt`.」 |
| `dropoff_deadline_dt` | 「Must be at least **20 mins** later than `dropoff_ready_dt` and must be **greater than or equal to** `pickup_deadline_dt`.」 |

🔴 **四個欄位全部是 optional**，且 T1 的 delivery-window 指南逐字給了 ASAP／Scheduled 兩套用法：

| 情境 | `pickup_ready_dt` | 其餘三欄 |
|---|---|---|
| **ASAP，無備貨時間** | 🔴 **留空**（逐字「otherwise leave blank」） | 🔴 **全部留空**（逐字「Leave Blank」） |
| **ASAP，有備貨時間** | `t0 + prep time` | 留空 |
| **Scheduled** | 至少 `t0 + 60 min`（或 `t0 + prep time`） | 四欄皆必填，各有上表的最小間距 |

**ASAP vs Scheduled 的分界（T1 逐字）**：ASAP ＝「Drop Off expectation is **< 90 minutes** in the future from order creation time」；
Scheduled ＝「**>= 90 minutes**」。

🔴 **Scheduled 是 provider capability，不是 3003 V1 的 runtime branch。**
本節保留 Scheduled 的官方契約是為了記錄「**Uber 支援什麼**」；我方 3003 V1 **只用 ASAP**。
送達預期 ≥ 90 分鐘時的處置是**結帳 eligibility 排除（method unavailable）**，
🔴 **不是**在 runtime 自動切成 Scheduled。
未來若要做「Uber 預約配送」，那是**另一個 capability／Product Flow**，獨立設計，不在 3003 V1 內。

🔴 **3003 V1 的用法 ＝ ASAP-with-prep**（上表第二列）：

| 欄位 | 3003 V1 行為 |
|---|---|
| `pickup_ready_dt` | 🔴 **送**，值 ＝ `pickup_ready_at_snapshot`（**報價當下凍結**的絕對時間） |
| `pickup_deadline_dt` | **OMIT** |
| `dropoff_ready_dt` | **OMIT** |
| `dropoff_deadline_dt` | **OMIT** |

⚠️ **T1 的兩句逐字（見上表 `pickup_ready_dt` 列）就是這個用法的授權**：
「Send this field when preparation time is known.」＝ **有備貨時間就送這個欄位**；
「The courier will arrive around the `pickup_ready_dt`.」＝ **外送員依這個時間抵達**。
後者正是我方要的行為：**建單早、取件晚**，courier 不會在賣家還在備貨時就到店枯等。

🔴 **禁止在 Create Delivery 當下重算這個值**：`E1 Create Quote` 與 `E2 Create Delivery` **必須送同一份 snapshot**。
用 `paid_at + prep_sla` 或新的 `now` 重算，會讓報價與建單的取件窗分岔——買家看到的 ETA 出自報價那一份，
而 Uber 依建單那一份排線。
⚠️ **這與「不得把 `seller_prep_sla` 當成 `pickup_ready_dt`」不是同一件事**：`pickup_ready_dt` **正是**由備貨時長推導的，
錯的是**推導的時機**，不是推導本身。快照欄位（`seller_prep_sla_snapshot`／`pickup_ready_at_snapshot`）
一旦 capture 成功即 **freeze**，見 66 號的 snapshot 設計。

**T2 的對應敘述（PDF p.38）——與 T1 一致，不是衝突**：p.38 逐字給的是**兩分支**：
「如為**即時訂單**，請輸入**現在時間 +5s**」／「如為**預約訂單**，請輸入**預計訂單撿貨完成時間**」。
🔴 我方 3003 V1 有備貨時間 ⇒ 落在**後者**；T2 的「預計訂單撿貨完成時間」**就是** `pickup_ready_at_snapshot` 的語意。
⚠️ 舊版本檔把 p.38 讀成「T2 說 +5s，與 T1 的『留空』衝突」——**那是只讀了前半句**，本節更正。
⚠️ T2 這裡的「預約訂單」是**台灣營運語彙**（＝有備貨時間），**不等於** T1 的 Scheduled（≥ 90 分鐘、四欄必填那一套）。

🔴 **T2 p.62 另有一段與本架構直接相關的官方建議**（全文見 §11.4）：Uber 台灣建議
「**於消費者結帳時，即建立預約訂單**（可設定 30 分鐘後配送），如此將不會產生報價逾期問題，
且可保留時間讓商家進行備貨與訂單處理」——**這正是 3003 V1 的「付款當下建單 ＋ 未來取件窗」**，
不是我方自創的用法。

### 2.6a Update Delivery（E5，T1）

`POST /v1/customers/{customer_id}/deliveries/{delivery_id}`　**有副作用**（§2.1 的 `E5`）。

🔴 **本節是 3003 V1「賣家提早備妥」的唯一自動路徑**，不是可有可無的增強。

**可更新欄位（T1 FAQ）**：

| 欄位 | 可更新 | 備註 |
|---|:---:|---|
| `pickup_ready_dt` | ✅ | 🔴 **3003 V1 實際會用的那一個**——賣家提早 Ready 時**前移**取件窗 |
| `pickup_deadline_dt` | ✅ | V1 不送（§2.6）⇒ 不更新 |
| `dropoff_ready_dt` | ✅ | 同上 |
| `dropoff_deadline_dt` | ✅ | 同上 |
| `dropoff_address`／經緯度 | ✅ | T2 p.41 亦載 |
| `pickup_notes`／`dropoff_notes`／`dropoff_seller_notes` | ✅ | T2 p.41 亦載 |
| `pickup_verification`／`dropoff_verification` | ✅ | T2 p.41 亦載 |

**可編輯窗（T1 FAQ）**：

| Delivery 階段 | 可否編輯 |
|---|:---:|
| Delivery created | ✅ |
| Pickup started | ✅ |
| Pickup imminent | ✅ |
| **Pickup complete** | ❌ |

⇒ 🔴 **取件完成之後就不能再改**，所以「賣家提早 Ready」的前移必須在 `pickup_complete` **之前**送出；
一旦錯過窗口，正確處置是**放棄前移**（Uber 照原約定時間取件），**不是**改走取消。

⚠️ **T2 p.41 對本端點只寫「可更新收貨地址經緯度、訂單備註、取貨收貨驗證方式」，未列時間窗欄位**——
但同頁自承「詳細可修改內容請見 API 文件」⇒ **T2 在此把權威讓給 T1**（§0 的等級規則），
時間窗欄位的可編輯性以 T1 FAQ 為準。

#### 🔴 3003 V1 的失敗處置（E5 不得升級為取消）

| 情形 | 處置 |
|---|---|
| `E5` 成功 | 取件窗前移，Uber 依新的 `pickup_ready_dt` 重新安排取件 |
| `E5` 失敗／逾時 | 🔴 **保留原 Delivery ＋ 保留原取件窗 ＋ bounded retry ＋ operational alert**。Uber 照**原**約定時間取件；買家 ETA 不變、**不向 Buyer 退款**、**不建第二筆 Delivery** |

🔴 **`Cancel + Recreate` 不是自動 fallback。**
它只在「有明確業務需要**且已確認原 Delivery 真的取消**」時由人工使用（取消的收斂鏈見 §4）。
⚠️ **T2 p.62 的括號註建議「可於 POS 建立取消及重新建立物流訂單功能」**——
那是 T2 給一般 POS 的營運建議；我方 3003 V1 **不採為自動路徑**，理由是自動 cancel+recreate 會在
「取消結果不明」時同時產生兩筆 provider leg 與兩筆成本，而 `E6` 的結果**本來就可能不明**（§4）。
🔴 這是**我方的設計裁決，不是 Uber 的規定**。

⚠️ **`E5` 之後 Uber 多久重新安排取件，官方未載** ⇒ `EXTERNAL_VERIFICATION_REQUIRED`（`EV-07`）。
它**不阻塞** implementation：前移失敗與前移延遲的處置相同（照原窗取件）。

---

### 2.7 🔴 冪等與唯一性（T1，本節關閉 66 的 `UP-17`）

| 機制 | T1 逐字 |
|---|---|
| `idempotency_key` | 「A key which is used to **avoid duplicate order creation** with identical idempotency keys **for the same account**. The key persists for a set time frame, **defaulting to 60 minutes**.」 |
| `manifest_reference` × `external_id` | 「the combination of this field with `external_id` **must be unique**; otherwise, **the delivery creation will not succeed**.」＋「If you can't ensure uniqueness for the `manifest_reference`, please include the `idempotency_key` in the request body and make sure it is unique.」 |

🔴 **對我方的三條結論**：
① **官方冪等鍵存在** ⇒ 66 `ADR-014` 的第 1 層（我方 pre-commit claim）**保留不動**，
官方鍵當**第二道**防線一併送出。**不得用它取代第 1 層**——60 分鐘 TTL 是 Uber 的，我方的 UNIQUE 是我們自己能保證的那一道。
② **60 分鐘 TTL 遠大於我方重試窗**（T-4 ＝ 15s × 3 ＋ 退避 ≈ 45s）⇒ 重試路徑完全落在保護窗內。
③ 🔴 **`991 CREATE_UNKNOWN` 的收斂手段因此改變**：帶**同一個 `idempotency_key` 重送 Create Delivery**，
60 分鐘內會取回原本那筆而不是建第二趟 ⇒ **比 List 候選比對可靠**（後者只能認出、不能指名，§3.3）。
⚠️ **超過 60 分鐘之後這條路失效** ⇒ `991` 的回查窗設計必須明確區分「≤60 分鐘」與「>60 分鐘」兩段。

#### 2.7.1 🔴 三個送出的識別字 ＋ 一個內部機制（R3；**R4 定版 local claim 的措辭**，PM Contract）

| # | 識別字／機制 | 類型 | 硬要求 |
|---|---|---|---|
| **A** | **local pre-commit claim** | **INTERNAL OWNERSHIP RECORD**（機制，不是 identifier） | claim-before-side-effect；由我方 DB `UNIQUE` / CAS 保證。✅ **其 `UNIQUE` 鍵就是 B 那份 persisted 值** |
| **B** | **`idempotency_key`**（送出） | **TECHNICAL DEDUPE IDENTITY** | immutable；side effect 前 persisted；per delivery leg 唯一；🔴 **重試與 replay 一律讀 persisted 原值，不得重算** |
| **C** | **`external_id`**（送出） | **BUSINESS CORRELATION IDENTITY** | deterministic；由**訂單識別 ＋ leg** 導出；正逆向可區分；stable；人讀得懂 |
| **D** | **`manifest_reference`**（送出） | **COURIER／PICKUP OPERATIONAL REFERENCE** | 不得含換行；🔴 與 `external_id` 的**組合**滿足官方唯一性規則 |

🔴 **A 與 B 共用同一份 persisted 值，這是刻意的，不是矛盾**：

```
local pre-commit claim record                  ← 機制（ownership / 併發）
   └─ keyed by  persisted technical dedupe key ← 同一份 immutable 值
          └─ 原值送出成 Uber idempotency_key    ← 協議欄位（protocol dedupe）
```

⚠️ **送出去的是那份 key value，不是 claim record 本身。**
🔴 **共享 key value ≠ 共享責任**：local 那層沒有 TTL、由我方 DB 保證；
provider 那層有 **60 分鐘 TTL 且在對方手上** ⇒ **兩層不得互相取代**。
⛔ **不得為 local claim 另開第四支 identity**（`local_claim_id`／`precommit_id`／`claim_uuid`）。

🔴 **`external_id` 與 `idempotency_key` 的語意角色不同，不得互換。**
把技術防重鍵直接當 `external_id` 送出，會讓帳單與 Dashboard 上出現一個**人看不懂、也對不回訂單**的字串；
反過來把 `external_id` 當冪等鍵用，則會在它為了可讀性改格式的那天**靜默失去防重能力**。
⚠️ **實際字串格式不在本檔鎖死**（屬 implementation review），本檔只定角色與唯一性；
語意範例 `<order_no>:FWD`／`<order_no>:RET` **僅示意，不得直接硬編碼**。
設計落點見 66 §6.3.2a-1。

### 2.8 交付方式（T1，本節關閉 66 的 `UP-23`）

**`deliverable_action`**（enum）：

| 值 | T1 描述逐字要點 |
|---|---|
| `deliverable_action_meet_at_door` | 「Meet at door delivery. **This is the default if DeliverableAction is not set.**」 |
| `deliverable_action_leave_at_door` | 「**Cannot** leave at door when **signature or ID** verification requirements are applied … **Photo confirmation of delivery will be automatically applied** as a requirement to complete drop-off.」 |

**`undeliverable_action`**（enum，逐字「**If not set then the default value is `return`**」）：

| 值 | 語意 |
|---|---|
| `return` | **預設**。送不到就把包裹送回取件點 |
| `leave_at_door` | 放門口。🔴 逐字「cannot be set to `leave at door` when **signature or PIN or ID** verification requirements are applied」；且會自動加上照片確認 |
| `discard` | 外送員自行處理／丟棄 |

⚠️ T1 另註：「Even if `deliverable_action` was set as `leave at door` and courier feels it is not okay then the package **can be returned** back to the pickup location.」
⇒ **即使選了放門口，仍可能產生返程** ⇒ 返程處理路徑不可省。

### 2.9 驗證與 PIN（T1，本節關閉 66 的 `UP-7`）

三個欄位 `pickup_verification`／`dropoff_verification`／`return_verification`，各自是 object：

| 子欄 | 適用 | 說明 |
|---|---|---|
| `picture` | pickup / dropoff / return | boolean。逐字「When set to true, **mandates the Courier to capture an image** as proof of delivery」 |
| `signature_requirement` | pickup / dropoff / return | object（`signature` boolean 已 `deprecated`） |
| `barcodes[]` | pickup / dropoff / return | `type` ∈ `CODE39`／`CODE39_FULL_ASCII`／`CODE128`／`QR` |
| `identification` | pickup / dropoff | object |
| **`pincode`** | 🔴 **只有 dropoff** | `{ "enabled": true }`；回應帶 `{ "enabled": true, "value": "1234" }` |

**PIN（T1 逐字）**：
- 「When set to true in POST requests, the delivery will require pincode entry at handoff. The pincode is **internally generated and shared by Uber**.」
- 「The pincode that the customer must present at dropoff.」
- 指南另載：**PIN 值會出現在 Create Delivery / Get Delivery / Update Delivery 的回應** `verification_requirements.pincode.value`，
  且 **Uber 會以簡訊主動告知收件人**（逐字範例：「Your PIN is 6066. Do not share your PIN with others…」）。
- 限制逐字：「Pincode verification is applicable **only during the dropoff** process and cannot be used during pickup」、
  「**cannot be used if the 'Leave-at-Door' option is selected**」。

⚠️ **T2 PDF p.68 宣稱有三種 PIN 型別**（`default` ＝ 收件人電話末四碼／`merchant_provided`／`random`）——
**T1 現行 spec 只有 `enabled` 一個旗標**，沒有型別欄位 ⇒ 標 **T2 / 可能過時 / `SANDBOX_VERIFY`**。

🔴 **買家端可見性（設計輸入）**：PIN 值**我方拿得到**（在建單回應裡），Uber 也會發簡訊。
⇒ 買家 UI 顯示 PIN 是**可行的**，不是靠 Uber 簡訊碰運氣。
⚠️ 但前提是 `dropoff_phone_number` 正確（T1 pattern `^\+[0-9]+$`）——號碼錯了 Uber 的簡訊就送不到，
而那時**買家唯一的 PIN 來源就是我方 UI**。

---

## 3. 查詢配送狀態

### 3.1 Get Delivery（E4，T1）

`GET /v1/customers/{customer_id}/deliveries/{delivery_id}`，回應欄位見 §2.5 回應表。

**外送員資訊 `courier`（`CourierInfo`）**：

| 欄 | T1 事實 |
|---|---|
| `name` | 「Courier's first name and last initial.」（例 `Alex I.`） |
| `phone_number` | 🔴 逐字「This is a **masked phone number that can only receive calls or SMS from the dropoff phone number**.」 |
| `vehicle_type` | `bicycle`／`car`／`van`／`truck`／`scooter`／`motorcycle`／`walker` |
| `location` | `{lat, lng}` |
| `img_href` | 具簽章與到期參數的 CDN URL |
| `rating` | 🔴 **已 `deprecated`** |

🔴 **`courier.phone_number` 只接受來自收件人號碼的來電** ⇒ **把它給賣家是沒有用的**。
賣家要聯繫外送員需要 T2 PDF p.83 的 `public_phone_info`（專線號碼 ＋ 轉接代碼，訂單完結後 30 分鐘失效）——
該欄位 **T1 spec 零命中** ⇒ 標 **T2 / `SANDBOX_VERIFY`**。同理 T2 PDF p.87 的 `sms_phone_number` 亦 T1 零命中。
🔴 **`public_phone_info` ＝ contact capability（撥打能力），≠ contact history／evidence（通聯紀錄）**——
Uber **沒有**提供任何 call-attempt log。子欄位、可撥打狀態窗與 `pin_code` 的憑證性質見 **§11.6**。

**`courier_imminent`**：boolean 欄位，外送員距離取貨或送達點 **1 分鐘**時為 true。
🔴 **它不是狀態**。把它當第 8 個狀態會同時造成兩個錯：多一個永不出現的狀態，且**漏讀「即將抵達」**。

⚠️ **T2 獨有的營運事實（T1 未載，`SANDBOX_VERIFY`）**：PDF p.47 指出
**外送員棄單換手時不會退回 `pending`、也不會發 webhook**，
判別方式是 `status = "pickup"` 且 **`courier = null`** ＝ 系統正在重新媒合。
⇒ 這是**只有主動回查看得到**的狀態，webhook 完全靜默。

### 3.2 🔴 `related_deliveries` 的形狀有歧義

| 來源 | 說法 |
|---|---|
| T1 spec | `related_deliveries: $ref: RelatedDelivery` —— **宣告成單一 object，不是 array**；description 也寫「the delivery identified in `related_deliveries`」（單數） |
| 66 號 N2（T3） | 宣稱是**陣列**（`id` ＋ `relationship`） |

⇒ 🔴 **`SANDBOX_VERIFY`，且 parser 必須同時容忍 object 與 array 兩種形狀**。
我方 `uber_deliveries.related_deliveries JSONB` 兩種都存得下 ⇒ **schema 不需改，改的是解析與斷言**。

**`relationship` enum（T1）**：`return` / `original`。
⚠️ **官方自己的 description 寫 `returned`**（逐字「`original` for the forward leg of the trip, and **`returned`** for the return leg」）
⇒ 🔴 **以 enum 為準，未知值 fail-closed**（不猜、不做前綴比對）。✅ 66 的 U-P 記載正確。

### 3.3 🔴 List Deliveries（E3）—— 可以**有界過濾**，不能**精確反查**

> 🔴 **2026-09-15 R3 更正**：本節原寫「query 參數只有三個」，**那已不是 current official fact**。
> 官方 changelog 2025-04-17 新增了三個，而 **SDK spec 尚未收錄**（見下方「spec 落後」）。

**現行可用的 query 參數**：

| 參數 | 等級 | 說明 |
|---|---|---|
| `external_store_id` | **T1-c** | changelog 2025-04-17 逐字新增。🔴 **V1 一賣家一個值** ⇒ 它等同「依賣家過濾」 |
| `start_dt` | **T1-c** | 同上，RFC 3339 時間窗起 |
| `end_dt` | **T1-c** | 同上，RFC 3339 時間窗迄 |
| `filter` | T1-B | 逐字：「Filter deliveries by delivery state. Valid values are: `pending`, `pickup`, `pickup_complete`, `dropoff`, `delivered`, `canceled`, `returned`, and **`ongoing`**.」 |
| `limit` | T1-B | 回傳筆數上限 |
| `Offset` | T1-B | 分頁位移（🔴 官方就是大寫 `O`） |

**changelog 2025-04-17 逐字**：「Added support for **new query parameters in the List Deliveries endpoint**:
`external_store_id`（Filter deliveries by external store identifier）、
`start_dt`／`end_dt`（Filter deliveries starting from / up to a specific date/time, RFC 3339）。」

🔴 **spec 落後 changelog，這是本檔 §0 那條警語的實例**：
`uber/uber-direct-sdk` 的現行 `openapi.yaml` **仍只列 `filter`／`limit`／`Offset`**，
`start_dt`／`end_dt` 在整份 spec **零命中**（2026-09-15 實查）。
⇒ **「spec 沒有」不等於「官方沒有」** ⇒ 查「有沒有某個能力」時
**changelog 與 spec 必須併查，且 changelog 較新者為準**。

🔴 **維持不變的結論**：**沒有 `external_id`／`manifest_reference`／`idempotency_key` 的 exact query。**
⇒ **66 `UP-9` 仍為負面關閉**：不能「用我方 id 指名取回那一筆」。
⚠️ **但它從「完全查不到」變成「查得到一個有界候選集合」** —— 這是 R3 的實質差異。

**回應形狀**：`{ data: DeliveryResp[], next_href, total_count }`。
`DeliveryResp` 是完整物件（含 `id`／`uuid`／`external_id`／`manifest`／`status`／時間）
⇒ 🔴 **嚴格本地比對在回應側做得到**。
⚠️ **`total_count` 官方標 `DEPRECATED`，逐字「Response is always -1」** ⇒
**不得用它判斷結果數量**；要數就自己數 `data[]` 並跟著 `next_href` 翻完，
否則「0 筆」可能只是沒翻頁。

⇒ `991 CREATE_UNKNOWN` 的收斂兩段式（規則全文在 66 §7.7）：
**≤60 分鐘** ＝ `idempotency_key` replay（協議層）；
**>60 分鐘** ＝ **bounded candidate search**（`external_store_id` ＋ 窄時間窗 ＋ 必要時 `filter`）
**＋ 嚴格本地比對**，**只有唯一且完整匹配才自動收斂**，0 筆或多筆一律進人工。
🔴 **禁止把 unbounded `filter=ongoing` 全掃當成設計。**

---

## 4. 取消

`POST /v1/customers/{customer_id}/deliveries/{delivery_id}/cancel`

- T1 現行 spec：**無 request body**；`200` 回傳被取消的 delivery 物件。
- T1 錯誤：`400 invalid_params`，其中一個具名 example 是 **`noncancelable_delivery`**。✅ 66 §4.5.4 記載正確。
- T1-c **2025-12-08**：「Added new optional fields for Cancel Delivery endpoint, allowing merchants to specify a **cancelation reason**.」
  ⇒ 現行 SDK spec 尚未反映該欄位 ⇒ **欄位名與值域 `SANDBOX_VERIFY`**。

🔴 **取消費用不在 API 契約內**（T1：取消回應與 delivery 物件皆無取消費欄位）。
**但「不可預判」是錯的**——T2 PDF p.43 給了可預期的規則：

| T2 規則 | 內容 | 來源頁 |
|---|---|---|
| 收費時點 | 逐字「**即於以下 status 取消將收取運費：`pickup_complete`、`dropoff`**」⇒ **產生訂單費用且不予退還**。🔴 **清單就是這兩個，是封閉的** | p.43 |
| 返程費 | 取件後取消依 `undeliverable_action`；`return` 會**另收返程運費，運費金額與去程運費相同** | 金額關係在 **p.38**；三分支行為在 p.43 |
| 建議 | 取消前先 `E4 Get Delivery` 確認狀態；`courier_imminent = true` 時建議不要取消 | p.43 |

🔴 **「不在收費清單」≠「絕對零成本」**：該清單講的是**正常去程運費**，未排除 ancillary／adjustment
（等待費、事後帳單修正）⇒ 最終 provider cost 一律以 provider refund／billing／statement 證據收斂。
逐字與完整推論見 **§11.3**。

⇒ 🔴 **兩件事必須拆開，不得互相冒充**：
① **風險可預判**（T2，`SANDBOX_VERIFY`）：取消前打一次 E4，依 `status` 判定「這次取消**會不會**產生費用」，
　結果寫進 case 供 `ADR-004` 的責任成因歸屬使用。**這是可以設計的。**
② **確切金額與對帳鍵**（**未證實**）：實際被收多少、出現在帳單哪一欄、對帳鍵是什麼——
　T1 只證實 `external_store_id` 與 `external_id` **會出現在 billing details report**，
　🔴 **沒有任何來源證實帳單「不含」完整 `del_…` ID**。
　⚠️ 舊稽核曾憑 PDF 截圖宣稱「帳單只有末五碼 ⇒ `uber_delivery_id` 對不上」——
　**那是從一張截圖推導的，本輪撤回**，改列為對帳鍵的 `EXTERNAL_VERIFICATION_REQUIRED`（需 Uber 提供真實帳單樣本）。

---

## 5. Proof of Delivery

`POST /v1/customers/{customer_id}/deliveries/{delivery_id}/proof-of-delivery`

🔴 **T1 逐字，本節是 POD 契約的唯一正確形式**：
「**If a delivery is created with any verification requirements** (e.g.: picture, signature or pincode),
the resulting image file is made available to you through our proof-of-delivery endpoint.」

⇒ **`delivered` 本身不保證有 POD。POD 的存在條件是「我方在建單時要求了驗證」。**
⚠️ 舊版本檔的敘述（「配送成功後 webhook 與 GetDelivery 會帶外部連結指向 POD」）讀起來像必然有 ⇒ 2026-09-14 更正。
✅ 66 已於 2026-09-13 自行更正（`INV-U5` 改為「我方要求了就該有」）⇒ **與本檔一致，無需再改**。

**請求（`PODReq`，兩欄皆 required）**：

| 欄 | enum |
|---|---|
| `waypoint` | `pickup` / `dropoff` / `return` |
| `type` | `picture` / `signature` / `pincode` |

**回應**：Base64 長字串，可轉 PNG。T1 逐字，圖檔內含：
Delivery Status／Delivery timestamp／**Uber Order ID**／**External Order ID**／Proof Type／Image
（signature 型另含 signer name 或 relationship，若啟用）。

**保存期限（T1，本節更正舊版）**：

| 通路 | 期限 |
|---|---|
| **API** | 逐字「The images are retained for **30 days**」 |
| **Dashboard** | **7 天** |

⚠️ 舊版記載「照片 7 天／簽名 30 天」**把通路差異誤讀成型別差異**。
🔴 **我方仍維持 7 天抓取窗（66 T-12）**——那是保守側且不需要改；但**理由要更正**：
不是「照片只留 7 天」，而是「Dashboard 只留 7 天、API 留 30 天，我方取 API 窗的前段以留出重試餘裕」。

**其他取得路徑**：Get Delivery 回應的 `dropoff.verification` 物件帶外部 URL；Dashboard 可下載。

---

## 6. Webhook

### 6.1 驗簽（T1）

| 項目 | 值 |
|---|---|
| header | **`x-uber-signature`**；Delivery Status／Courier Update 另接受舊版 `x-postmates-signature`。**Refund webhook 必須用 `x-uber-signature`** |
| 金鑰 | 🔴 **專屬 Webhook Signing Key**（建立 webhook 時發給），**不是** OAuth `client_secret` |
| 取得方式 | Dashboard → Developer → Webhooks → 該條目 → Edit |
| 演算法 | **HMAC-SHA256**，簽的是 message payload 本身 |

🔴 **驗簽必須在任何 JSON 解析之前**（raw body）。先 parse 再 re-serialize 會因欄位順序不同而
偶發失敗——那種失敗率取決於資料內容，**測不出來、上線才炸**（66 §6.3.1 A2）。

🔴 **T2 PDF p.14 的敘述不得採用**：該頁把機制簡化成「Direct Webhook 會在指定的 request header 屬性中
**放入約定的 Secret Key**，您可以在 WAF 建立安控規則」。
**那是 shared-secret 比對，不是 HMAC 簽章** ⇒ 標 **PDF OUTDATED / SIMPLIFIED**。
照它實作會得到一個**完全不驗 payload 完整性**的端點：任何知道該 key 的人可以偽造任意 body。
⇒ 正式契約維持六件：專屬 signing key ＋ `x-uber-signature` ＋ HMAC-SHA256 ＋ raw payload ＋ constant-time 比較 ＋ 驗簽先於 JSON parse。

⚠️ **webhook 只能在 Dashboard 手動建立、event types 手動勾選、signing key 手動複製**（T2 PDF p.13／p.15，T1 未提供對應 API）
⇒ **環境重建與金鑰輪替是人工步驟**，不得寫進自動化流程假裝它可重建。

### 6.2 事件 payload（T1）

| 欄位 | 說明 |
|---|---|
| `event_id` | 唯一識別碼；**未 ack 會重試** ⇒ 消費端必須冪等 |
| `event_time` | Unix 時間戳 |
| `event_type` | 事件分類 |
| `meta` | 事件型別專屬資訊 |
| `resource_href` | 取回完整狀態的端點 URL |

### 6.3 重試（來源：66 號 U-C／U-D，本輪未重新實查）

僅 `500`／`502`／`503`／`504` 與 timeout 會重試；**4xx 不重試**
⇒ 我方回 4xx ＝ 該事件永久丟失。上限 3 次、窗口約 100 秒（10／30／60〔／120〕秒）。

### 6.4 事件種類

| `kind` | 訂閱 | 備註 |
|---|---|---|
| `event.delivery_status` | ✅ | 核心（§7.1） |
| `event.courier_update` | ✅ | **每 20 秒一發** |
| `event.refund_request` | ✅ | §10 |
| `event.cpp_shopping_notification` | ❌ | Courier Pick & Pack 專屬，我方非代購 |

---

## 7. 狀態值域

🔴 **官方文件站有兩組 API 家族，各自一套 status，名字完全不同。我方整合的是 DaaS／Direct 家族。**

### 7.1 DaaS／Direct — `status`（**我方用這組**，T1）

| 值 | 官方描述（逐字） |
|---|---|
| `pending` | Delivery has been accepted but does not yet have a courier assigned |
| `pickup` | Courier is assigned and moving towards the pickup（`courier_imminent=true` 時為 1 分鐘內） |
| `pickup_complete` | Courier completed the pickup |
| `dropoff` | Courier is moving towards the dropoff（同上） |
| `delivered` | Courier has completed the delivery |
| `canceled` | Delivery has been canceled |
| `returned` | Delivery has been canceled and the `undeliverable_action` is return<br/>⚠️ **我方註記**：本值只描述 **physical disposition**（包裹回到商家），🔴 **我方 3003 V1 不以它自動觸發退款**——退款與否由**責任判定**決定，見 §11.3 與 66／70 號設計書 |
| `shopping_completed` | Courier completed shopping during pickup（**Only for Courier Pick & Pack**） |

🔴 **delivery 物件本身的 `status` enum 只有前 7 值**（T1 spec 實查，`shopping_completed` 不在內）
⇒ 66 的 `U-I`「7 值」**正確**；第 8 值只出現在 webhook 且屬我方未訂閱的模式 ⇒ 維持「容忍值」處置。

### 7.2 🔴 webhook 的兩個 `status`

**T1 事實**：`event.delivery_status` 的 payload **同時有頂層 `status` 與 `data.status`**，
且**兩者的官方描述逐字相同**：「Status of the delivery the event refers to.」
⇒ 🔴 **官方只證實兩者存在，沒有定義它們何時會不同。**

**T2 事實（PDF p.46）**：建議使用 `body.data.status`，理由是「可以保留 `canceled` 狀態出現的唯一性」，
並給出一張返程狀態對照表，其中 **#5 列兩者分歧**：

| 情境 | 原單 `data.status` | 原單 `status` |
|---|---|---|
| 取消並轉入返程 | **`returned`** | **`canceled`** |

⇒ 🔴 **設計規則（T1 ＋ T2 合成）**：
① **parser 必須同時讀取並保存兩個欄位**，不得只取其一——官方沒有定義語意差，
　只讀一個等於把行為押在一個沒有文件的欄位上。
② **狀態機的輸入取 `data.status`**（T2 建議，且它是 delivery 物件的狀態、與 Get Delivery 一致）。
③ **兩者不一致時記 counter ＋ 保留 raw**，不得靜默擇一。
④ 🔴 **返程分歧必須進 sandbox test matrix**（§9 的 return 案例）——
　若照頂層 `status` 實作，取消轉返程那一刻會讀到 `canceled` ⇒ 我方**提早寫入終態並進入責任判定與結算**，
　而包裹其實正在返程、且會再產生一筆返程運費（§11.3）。
　⚠️ **錯的是「終態時點」不是「有沒有退款」**：退款與否本來就由責任判定決定，
　但在包裹還在返程途中就宣告終態，會讓**尚未發生的返程成本落在結算之外**。

### 7.3 Eats deliveries 家族（**我方不用，列出以免抄錯**）

`order_status` ＝ `ACTIVE`／`COMPLETED`／`FAILED`；
`courier_trips[].status.status_code` ＝ `SCHEDULED`／`EN_ROUTE_TO_PICKUP`／`ARRIVED_AT_PICKUP`／`EN_ROUTE_TO_DROPOFF`／`ARRIVED_AT_DROPOFF`／`COMPLETED`／`FAILED`；
`return_status` ＝ `IN_PROGRESS`／`RETURNED`／`NONE`；
webhook `event_type = dapi.status_changed`，我方識別碼在 `meta.external_order_id`；
`cancellation_details.reason.reason_code` ＝ `CUSTOMER_CANCEL`／`MERCHANT_CANCEL`／`UBER_CANCEL`／`COURIER_CANCEL`。

---

## 8. Organizations API（三層式架構）

**base**：`https://api.uber.com/v1/direct/`　**scope**：`direct.organizations`
**spec 版本**：Organizations API **v1.0.1**（T1）

T1 逐字：「Uber's Organization API enables merchants to **create and manage sub-organizations nested below their given root organization**.」

### 8.1 三層對應

| Uber 層 | 識別碼 | 我方對應 |
|---|---|---|
| **Direct Account / Root Org** | Dashboard 的 Customer ID（＝ `parent_organization_id`） | `uber_account.organization_id` |
| **Organization（子組織）** | `organization_id`（**UUID**）＝ Direct API 路徑的 **`customer_id`** | `uber_organization.organization_id` |
| **Store / 取件點** | **`external_store_id`**（我方自訂字串） | `uber_store_mapping.external_store_id` |

🔴 **本節坐實兩件事**：
① **`customer_id` ＝ Organization ID**（Direct API 逐字「Unique identifier for the organization. Either **UUID** or starts with `cus_`」；
　CreateOrg 回應逐字「`organization_id`: Unique identifier of the organization represented as a **UUID**」）。
② **Store 層沒有自己的 `customer_id`**；Store 層唯一的識別碼是 `external_store_id`。

⚠️ **我方現況與此不符**：66 §5.1.3 把 `customer_id VARCHAR(64)`（註記 `cus_…`）放在 **`uber_store_mapping`（Store 層）**。
⇒ 詳見 §12 的 `PCR-06`，**層級歸屬涉及商業 ownership，本輪只列 Decision Input，不自行改**。

### 8.2 CreateOrg（T1）

`POST /v1/direct/organizations`

| 欄位 | required | T1 事實 |
|---|:---:|---|
| `info.name` | ✅ | 商業名稱。🔴 **不允許 URL 與 `/ \ : % < > # =` 這些字元** |
| `info.billing_type` | ✅ | `BILLING_TYPE_CENTRALIZED`／`BILLING_TYPE_DECENTRALIZED` |
| `info.merchant_type` | ❌ | 見 §8.3 |
| `info.point_of_contact` | ❌* | `email` ＋ `phone_details{phone_number, country_code, subscriber_number}`。🔴 *`DECENTRALIZED` 時 email 必填 |
| `info.address` | ❌* | `street1`／`street2`／`city`／`state`／`zipcode`／`country_iso2`。🔴 *`DECENTRALIZED` 時必填（錯誤訊息逐字：`street1 is required for BILLING_TYPE_DECENTRALIZED`） |
| `info.contract_type` | ❌ | `CONTRACT_TYPE_PARENT`（沿用母組織合約）／`CONTRACT_TYPE_CUSTOM` |
| `hierarchy_info.parent_organization_id` | ✅ | 逐字「This is the (**Root Org**) found via the `customer_id` displayed on the developer dashboard billing.」 |
| `options.onboarding_invite_type` | ❌ | `ONBOARDING_INVITE_TYPE_INVALID`（不寄信）／`ONBOARDING_INVITE_TYPE_EMAIL` |

🔴 **PDF p.27「以下參數皆為必填欄位」是錯的**：T1 的 `required` 只有 `name` ＋ `billing_type`（＋ `parent_organization_id`）。
我方 billing type 為 **CENTRALIZED** ⇒ `address`／`point_of_contact` **非必填**。

🔴 **`billing_type` 的 T1 警語**：「`BILLING_TYPE_DECENTRALIZED` requires **Uber's involvement to set up the billing policy before deliveries can be created**.」
⇒ 我方選 CENTRALIZED **不需要**這道前置。✅ 與 66 §1.6 的平台↔Uber 月結模型一致。

🔴 **`options.onboarding_invite_type` 是本輪的新發現**：T1 逐字
「Indicates a **webform should be sent to collect billing and tax information**. The webform will be sent to the POC specified in the request.」
⇒ **這就是 `customer_blocked` / 稅務表單那條路徑的前置觸發點**。
66 的 N6 記載「沒有事前查詢欄位，只能建單失敗後反推」**仍然正確**（確實沒有查詢欄位），
但**現在多了一個「主動觸發」的手段**：建組織時帶 `ONBOARDING_INVITE_TYPE_EMAIL` 讓 Uber 直接寄表單給賣家，
不必等第一筆建單失敗。⇒ 列入 66 的 onboarding 設計輸入。

**錯誤（T1 具名 example）**：
`parent organization is not a root organization`（🔴 **父層必須是 root** ⇒ 組織不能無限層巢狀）／
`parent_organization_id is invalid`／`billing_type is invalid`／`merchant_type is invalid`／
`street1 is required for BILLING_TYPE_DECENTRALIZED`。

### 8.3 `merchant_type` 值域（T1）

`MERCHANT_TYPE_RESTAURANT`／**`MERCHANT_TYPE_GROCERY`**／`MERCHANT_TYPE_LIQUOR`／`MERCHANT_TYPE_RETAIL`／
`MERCHANT_TYPE_ESSENTIALS`／`MERCHANT_TYPE_PHARMACY`／`MERCHANT_TYPE_SPECIALTY_FOOD`／`MERCHANT_TYPE_FLOWER`／`MERCHANT_TYPE_PET_SUPPLY`

🔴 **PDF p.60 寫的是 `MERCHANT_TYPE_GROERY`（少一個 C），那是 T2 的錯字。以 T1 為準。**
⚠️ 生鮮雜貨 ⇒ `MERCHANT_TYPE_GROCERY`。**酒類（`LIQUOR`）依 PDF p.60 於台灣不適用。**

### 8.4 GetOrg / InviteToOrg（T1）

| 功能 | Method / 路徑 |
|---|---|
| `GetOrg` | `GET /v1/direct/organizations/{organization_id}` |
| `InviteToOrg`（PDF 稱 AddMember） | `POST /v1/direct/organizations/{organization_id}/memberships/invite` |

`InviteToOrg` 的 `roles` **required**，值域 `ROLE_ADMIN`／`ROLE_EMPLOYEE`；
`user_details` 需 `email`／`first_name`／`last_name`（名字同樣禁 URL 與 `/ \ : % < > # =`）。
回應帶 `membership_id`（UUID）。

⚠️ **T2 PDF p.30 的測試環境注意事項**：sandbox 建立新 org 後，
**必須先用 InviteToOrg 把自己加進去**，才能在 Dashboard 切換到該 org 看資料。
（正式環境會自動把 Direct Account 的所有 user 加入。）⇒ 影響 sandbox 驗證步驟順序。

---

## 9. Robocourier（sandbox 自動接單）

**T1 逐字**：「a dedicated test automation tool built specifically for Uber Direct」，
**僅 test/sandbox 環境**，且「This feature **should not be used for stress testing**.」

在 **Create Delivery** 的 request body 加：

| mode | 行為 |
|---|---|
| `auto` | 固定 **30 秒**間隔跑完整趟（assignment → enroute → pickup imminent → pickup → dropoff imminent → dropoff），**總計約 2.5 分鐘** |
| `custom` | 逐事件指定時間戳：`enroute_for_pickup_at`／`pickup_imminent_at`／`pickup_at`／`dropoff_imminent_at`／`dropoff_at` |

請求片段（T1 範例）：`test_specifications` → `robo_courier_specification` → `mode`。

**限制（T1）**：無逐街道路徑規劃（外送員「瞬移」）／不支援 parking bay check-in／**不支援疊單**。

🔴 **這是 sandbox golden fixture 的唯一可行驅動方式**：
沒有它就跑不出 `pickup → pickup_complete → dropoff → delivered` 的完整 lifecycle，
而 66 §13.2 的 golden fixture 要求「跑完整一趟含取消與退回」。
⇒ 舊版本檔完全沒有這一節，屬**缺漏而非未載**。

---

## 10. Refund Submission API

🔴 **T1-c 2025-10-27 逐字**：「Added new API **Refund Submission API**. This API suite allows organizations to
**programmatically request refunds** for orders that meet specific criteria.」
（同一則另載欄位更名：`amount_e5` → `amount`。）

⇒ **T2 PDF p.80 把補償描述成「商家聯繫 Uber 客服申請補償，客服於系統操作退款」——標 `PDF HISTORICAL / OUTDATED`。**
⚠️ **不得在設計中寫「refund compensation 只能人工透過客服發起」。**

**`event.refund_request` webhook（T2 PDF p.80／p.108，T1 有對應 webhook 頁但本輪未取得欄位表）**：

| 欄位 | 說明 |
|---|---|
| `delivery_id` | 退款對應的 Uber 訂單 |
| `refund_fees[]` | `fee_code` ＝ `UBER_DELIVERY_FEE`／`PARTNER_FEE`／`PARTNER_TAX`；`value` 為 **cents** |
| `refund_order_items[]` | `party_at_fault` ＝ `UBER`／`Partner`；`reason`；`uber_refund_amount`／`partner_refund_amount`（cents） |
| `total_uber_refund` / `total_partner_refund` | cents |

**`reason` 值域（T2）**：`UBER_NEVER_RECEIVED_ORDER`／`UBER_ENTIRE_ORDER_WRONG`／`UBER_MISSING_ITEMS`／
`UBER_DAMAGED_ITEM`／`UBER_RETURN_TRIP_ISSUE`／`UBER_NEVER_PICK_UP`／`REFUND_REJECTED`。

🔴 **`party_at_fault` 直接對應 `ADR-004` 的責任成因軸**（`PROVIDER_CAUSED` vs 其他）
⇒ 是 70 號責任歸屬的**現成輸入**，不是要另外發明的判斷。
⚠️ **T1 的 Refund Submission API 欄位表本輪未取得** ⇒ `EXTERNAL_VERIFICATION_REQUIRED`。

---

## 11. T2 台灣指南的營運契約（PDF v2.4）

> 本章收錄《Uber Direct 開發指南 v2.4》(**T2**) 中**跨端點的營運事實**——它們不屬於任何單一 API 的 schema，
> 卻直接決定我方的**偵測、取消、對帳與聯絡**設計，塞進單一端點節會切斷該節的 T1 契約敘述。
> 🔴 **T2 不得覆蓋 T1 的 protocol／schema／security**（§0）；本章每一條都標明它與 T1 的關係。
> ⚠️ 本章是 **provider reference**，不是我方 Domain policy。凡是我方的處置一律逐字寫成「**我方 3003 V1**」。

### 11.1 主動回查頻率（p.47）

**T2 逐字**：「**建議每隔 5 或 10 分鐘**，針對狀態尚未完結的訂單進行主動更新，
**呼叫 Get Delivery API** 同步門市營運必要資料如 `status`, `pickup_eta`, `courier` 等。」

| 項 | 事實 |
|---|---|
| 對象端點 | 🔴 **`E4 Get Delivery`（逐筆）**，不是 `E3 List` |
| 頻率 | 5–10 分鐘 |
| 範圍 | **狀態尚未完結**的訂單 |
| 等級 | **T2 建議**，🔴 **不是 Uber 官方 SLA**，也不是契約義務 |

🔴 **它與 §3.3 的「禁止 unbounded `filter=ongoing` 全掃」並不衝突，而是互補**：
輪詢的對象是**我方已知 `delivery_id` 的逐筆 `E4`**；`E3` 只在 `991 CREATE_UNKNOWN` 的
bounded candidate search 時使用。**兩者不可互換。**

⚠️ **我方 3003 V1 的 `provider_poll_interval` 預設 5m 取得本條背書**，但它仍是 **configurable**，
可依 sandbox／production 實測調整；🔴 **不得寫成「Uber 規定 5 分鐘」**。

### 11.2 Rematch（外送員棄單換手）的偵測（p.47）

🔴 **這是 webhook 完全看不到的狀態**，偵測條件與判讀見 §3.1（不在此重複）。本節補 T2 的**完整軌跡**：

**T2 逐字**：「由於目前 webhook 在狀態更新至 `pickup` 後，**不會因為外送員轉單而退回 `pending` 狀態並發送 webhook**。」
「若想要具體抓出外送員棄單、Direct 正在重新媒合中的特定情境供門市決策，
**建議利用外送員 `courier` 欄位是否為 `null` 判斷**。」

**p.47 的 `E4` 軌跡表（逐字）**：

| 情境 | `status` | `courier` | `pickup_eta` |
|---|---|---|---|
| 媒合中 | `pending` | `null` | datetime |
| 外送員 A 接單，正在前往店家 | `pickup` | {外送員 A 資訊} | datetime |
| **外送員 A 放棄訂單，系統重新媒合** | **`pickup`** | **`null`** | datetime |
| 外送員 B 接單，正在前往店家 | `pickup` | {外送員 B 資訊} | datetime |

🔴 **兩個容易讀錯的地方**：
① **`pickup_eta` 在重新媒合期間仍然有值** ⇒ **不得**用「`pickup_eta` 消失」當偵測條件；
　真正的判別只有 `status = pickup` **AND** `courier = null` 這一個組合。
② 重新媒合**不會**讓 `status` 退回 `pending` ⇒ **不得**等 `pending` 再開始處理。

⚠️ **T2 未載的兩件事（`EXTERNAL_VERIFICATION_REQUIRED`）**：重新媒合最長會持續多久、
Uber 會不會自行放棄並給終態（`EV-02`）。⇒ 我方 3003 V1 因此需要**自己的 hard deadline**，
那是**我方的 platform policy**，🔴 **不得寫成 Uber 的 SLA**。

### 11.3 取消的收費邊界（p.43／p.38）

**T2 p.43 逐字（收費 status 清單是封閉的兩個）**：
「在外送員取件後取消，將會**產生訂單費用且不予退還**……**即於以下 status 取消將收取運費：`pickup_complete`、`dropoff`**。」

| status | 取消後是否產生正常去程運費 |
|---|---|
| `pending` | T2 未列入收費清單 |
| `pickup` | T2 未列入收費清單（⚠️ 但見下方警語） |
| **`pickup_complete`** | 🔴 **產生且不予退還** |
| **`dropoff`** | 🔴 **產生且不予退還** |

🔴 **「不在收費清單」只能推論到「尚無已確認的正常去程 delivery charge」，不得寫成絕對 0**：
T2 這張清單講的是**正常去程運費**，它沒有排除 ancillary／adjustment 類成本（等待費、調整、事後帳單修正）。
⇒ 任一情形的**最終** provider cost 一律以 **provider refund／billing／statement 證據**收斂，
**不以取消當下的 status 直接定案**（`EV-08`）。

**返程運費（p.38 逐字）**：「返程物流將另外收取返程運費，**運費金額與去程運費相同**。」
⚠️ **來源頁是 p.38 不是 p.43**——p.43 只說「將另外收取返程運費」，沒有給金額關係。

**取件後取消時外送員的下一步（p.43 逐字，依 `undeliverable_action`）**：

| 值 | 外送員 App 顯示的下一步 |
|---|---|
| `return` | 將商品**返送至商家**（另收返程運費） |
| `leave_at_door` | 將商品**放置門口** |
| `discard` | 外送員**自行處理**商品 |

🔴 **返程是 physical disposition，不是退款事件**：`returned` 只描述「包裹回到商家」，
**我方 3003 V1 不以它自動觸發退款**（見 §7.1 的註記）。

**T2 的取消建議（p.43）**：取消前先 `E4 Get Delivery` 確認最新狀態；
`status = pickup` 且 `courier_imminent = true` 時**非必要不要取消**（「增加外送員好感」）。

### 11.4 報價與建單的時間差處理（p.62）

**T2 描述的常見問題**：結帳時報價、賣家點出貨時才建單 ⇒ 「因自初次報價至商家點選出貨**通常超過 15 分鐘**，
要重新取得報價才能建立訂單」，而重新報價**可能變動**（「雖然 99% 以上之狀況……會相同，
但仍會有因系統最佳化路徑改變而產生不同價格的狀況」）。

**T2 給的三個建議（逐字要點）**：

| # | 建議 |
|---|---|
| ① | **進行商家溝通**：於使用頁面與條款指出報價可能變動 |
| ② | **對消費者收取固定物流費**（例：可配送距離內統一 80–100 元，或滿額免運），避免直接收實際運費 |
| ③ | 🔴 **調整建立訂單時間點，於消費者結帳時即建立預約訂單**（可設定 30 分鐘後配送），「如此將不會產生報價逾期問題，且可保留時間（如 30 分鐘）讓商家進行備貨與訂單處理」 |

🔴 **③ 正是我方 3003 V1 的形狀**：**付款當下建單 ＋ 未來取件窗**（`pickup_ready_dt` ＝ 報價時凍結的
`pickup_ready_at_snapshot`，見 §2.6）。⇒ 這不是我方自創的用法，而是**Uber 台灣自己的建議路徑**。

⚠️ **③ 的括號註建議「可於 POS 建立取消及重新建立物流訂單功能」——我方 3003 V1 不採為自動路徑**，
首選是 **`E5 Update Delivery` 前移取件窗**（§2.6a）。🔴 這是**我方的設計裁決，不是 Uber 的規定**。

### 11.5 `event.refund_request` 的責任輸入（p.80／p.108）

欄位表與 `reason` 值域見 **§10**（不在此重複）。本節只定它在責任判定中的**地位**：

| 欄位 | 能回答什麼 | 🔴 不能回答什麼 |
|---|---|---|
| `party_at_fault`（`UBER`／`Partner`） | 這筆 provider refund 由哪一側承擔 | **二值** ⇒ `Partner` **分不出**賣家／買家／平台 |
| `refund_fees[]`（`fee_code`／`value`） | provider 側實際退了哪些費用、多少 cents | 不表達我方對買家的退款決定 |

🔴 **它是責任判定的其中一個輸入，不是判定本身。**
我方 3003 V1 的責任分類需要**多個來源合成**（provider outcome／delivery attempt facts／
賣家與平台側事實／本欄），**單靠本欄不足以定案**；證據不足時的正確落點是
**未判定**（責任分類的值域與流程見 66／70 號設計書），🔴 **不是**直接推定某一方。

### 11.6 賣家聯繫外送員：`public_phone_info`（p.83）／`sms_phone_number`（p.87）

🔴 **T1 spec 對這兩個欄位零命中** ⇒ 全節標 **T2 / `SANDBOX_VERIFY`**。
`courier.phone_number` 為何不能給賣家，見 §3.1。

**`public_phone_info` 的子欄（p.83 逐字）**：

| 子欄 | 內容 |
|---|---|
| `phone_number` | 外送夥伴**聯繫專線號碼**（匿名轉接號） |
| `pin_code` | **轉接代碼** |
| `formatted_phone_number` | 上述兩者的**組合字串** |

**撥打方式（p.83 逐字）**：撥打專線號碼 → 聽到系統提示音後輸入轉接代碼 → 電話撥出。

**可撥打的狀態窗（p.83 逐字）**：已媒合外送夥伴尚未取貨／已取貨正在前往配送地點／
**配送完成或取消後的 30 分鐘內**／**返程完成或取消後的 30 分鐘內**。
⚠️ 同頁另載：「此為系統**加密過後**的電話號碼，將會於訂單完成或取消過後的 **30 分鐘後失效**」，
且「您的電話亦會經過加密後才顯示在外送夥伴通話介面上」。

🔴 **`pin_code` 是憑證，不是識別碼。** 任何持有「專線號碼 ＋ 轉接代碼」的人都能撥通該趟外送員
⇒ 我方 3003 V1 必須把它納入 **log 遮罩**，並限制只在有權限的賣家介面上呈現。

🔴 **`public_phone_info` ＝ contact capability（撥打能力），≠ contact history／evidence（通聯紀錄）。**
T2 p.83 提供的是**打得通的手段**，**沒有**提供任何 call-attempt log、通話時長或接通與否的回報。
⇒ 🔴 **不得**把「Uber 給了匿名號 ＋ PIN」寫成「Uber 提供聯繫紀錄」，
更不得把它當成責任判定的**聯繫證據來源**——那類證據若系統裡不存在，正確處置是承認不存在（§11.5）。

**`sms_phone_number`（p.87）**：Create Delivery 回應中的欄位，供**收件人**主動傳簡訊給外送員
（逐字：「此電話號碼**經過加密處理並隨機產生**，並非外送夥伴的真實電話號碼。收件人的真實手機號碼
亦不會顯示給外送夥伴」）。⇒ 它的對象是**買家**，與上面賣家用的 `public_phone_info` **是兩件事**。

---

## 12. 與 66 號設計書的對照

### 12.1 本輪確認的契約缺陷（T1 證據，需回灌）

| # | 缺陷 | 66 現況 | T1 事實 | 影響 |
|---|---|---|---|---|
| **PCR-01** | **金額單位** | §6.5 逐字「TWD 係數 1」 | 所有金額欄位為 **cents（¹⁄₁₀₀）**，多處逐字且附換算例 | `manifest_items[].price`／`manifest_total_value` 少 100 倍。**DB 仍維持元整數**，只在 adapter 邊界換算 |
| **PCR-02** | **樓層／房號的去處** | §6.5 紅字「進 notes 不進 address」 | `street_address` 是 **array**，example `["2000 Ocean Ave", "Floor 2"]`；FAQ 逐字「Apartment or unit numbers are added as a **separate value in the street address structure**」 | 樓層進 notes ⇒ 外送員 App 不顯示結構化樓層，且地理編碼拿不到完整地址 |
| **PCR-03** | **`related_deliveries` 形狀** | N2 斷言是**陣列** | spec 宣告為**單一 object** | parser 只吃 array ⇒ 逆向關聯建不起來。⇒ 兩形皆容忍 ＋ `SANDBOX_VERIFY` |
| **PCR-04** | **`currency` 已 deprecated** | §5.1.4 `uber_quotes.currency` | T1 標 `deprecated`，改用 `currency_type` | 低。新程式碼用 `currency_type`，欄名可不動 |
| **PCR-05** | **List 無我方 id 的 exact query** | `UP-9` 標「未查證」，`991` 收斂依賴它 | 無 `external_id`／`manifest_reference` 精確反查參數。<br/>🔴 **R3 更正**：另有 `external_store_id`／`start_dt`／`end_dt`（changelog 2025-04-17，spec 未收錄）⇒ **可做有界過濾** | `991` 兩段式：≤60 分 `idempotency_key` replay；>60 分 bounded candidate search ＋ 嚴格比對 ＋ 人工兜底（§3.3、66 §7.7） |
| **PCR-06** | **`customer_id` 的層級** | §5.1.3 放在 **store mapping**，註記只有 `cus_…` | `customer_id` ＝ **Organization ID**，UUID **或** `cus_` | 🔴 **Decision Input**（§12.4） |

### 12.2 本輪關閉的 external verification 項

| 項 | 舊狀態 | 本輪結論 |
|---|---|---|
| **`UP-7`** 台灣是否會出現 `pincode` 型 POD | sandbox 實測 | ✅ **關閉**：`pincode` 是 `dropoff_verification` 的正式子欄，**只能 dropoff、不能與 leave-at-door 併用**，值由 Uber 產生並回傳在建單／查詢回應。⚠️ PDF 的三種 PIN 型別 T1 零命中 ⇒ 保留為 `SANDBOX_VERIFY` |
| **`UP-9`** List 能否以我方 external id 反查 | 未查證 | ✅ **關閉（負面）**：不能。見 PCR-05 |
| **`UP-17`** 官方冪等機制 | 「完全沒有證據」 | ✅ **關閉（正面）**：`idempotency_key` 存在，**預設 60 分鐘**。🔴 當**第二道**防線，**不取代** `ADR-014` 第 1 層 |
| **`UP-23`** 交付方式 enum | 官方欄位名與值域未確認 | ✅ **關閉**：`deliverable_action`（預設 meet_at_door）／`undeliverable_action`（預設 return）／三個 verification 物件，值域見 §2.8／§2.9 |
| **`UP-24`** `pickup_ready_dt` 語意 | 「官方從未定義這個欄位」 | ✅ **關閉**：欄位存在且 optional，語意＝「取件窗開始」，`< 30 天`。🔴 **有備貨時間就送**（T1 逐字「Send this field when preparation time is known.」＋「The courier will arrive around the `pickup_ready_dt`.」）⇒ **3003 V1 ＝ ASAP-with-prep，只送 `pickup_ready_dt`，另三欄 OMIT**（§2.6）。⚠️ PDF `p.38` 的 `+5s` **只適用「即時訂單」分支**，同頁的「預約訂單 ⇒ 輸入預計訂單撿貨完成時間」才是我方情境 ⇒ **T1 與 T2 一致，非衝突** |
| POD 保存期 | 「官方未載」／66 記「照片 7 天、簽名 30 天」 | ✅ **關閉並更正**：**API 30 天／Dashboard 7 天**（通路差異，不是型別差異）。我方 7 天抓取窗**維持不變**，理由改寫 |
| Refund 是否只能人工 | 66 §11.3 E4 列為演進項 | ✅ **關閉**：**Refund Submission API 自 2025-10-27 存在** |
| 重量／體積／件數上限 | 66 標「官方未載」 | ⚠️ **部分**：T1 **確實沒有**任何上限數字；有數字的只有 T2。見 §12.3 |

### 12.3 🔴 承載限制必須分層，不得合成一張「Uber 官方限制」

| 參數 | **T1（現行官方）** | **T2（台灣業務指南 v2.4）** | **我方 platform policy** | 驗證 |
|---|---|---|---|---|
| `size` 值域 | `small`/`medium`/`large`/`xlarge`，**未指定時預設 `small`** | 各對應 2,359.7／20,322.5／42,032.8／92,177.2 cm³ | — | `SANDBOX_VERIFY` |
| 單筆件數 | **無上限數字** | 未傳 size ⇒ 全部視為 `small` ⇒ **超過 34 項無法媒合**；建議以 **30** 為判斷基準 | **M-4 ＝ 30 硬擋** | `NEEDS_ACCOUNT_MANAGER` |
| 總體積 | **無上限數字** | **82,369 cm³** | **M-2 ＝ 125,000 cm³** | 🔴 見下方 |
| 總重量 | **無上限數字**（`weight` 僅 minimum 1 g） | **20,000 g** | **M-1 ＝ 20,000 g** | `NEEDS_ACCOUNT_MANAGER` |
| 單邊長 | `dimensions` 僅 minimum 1 cm | — | **M-3 ＝ 60 cm** | — |

🔴 **M-2 必須處理**：我方門檻 **125,000 cm³ 比 T2 的 82,369 cm³ 寬 1.5 倍**。
⚠️ **但不得直接把 82,369 寫成「Uber 官方限制」**——T1 沒有這個數字，它可能是**台灣地區／本帳號的營運設定**，
而營運設定是可以隨帳號調整的。
⇒ **正確處置**：① `M-2` **不得再被描述成 provider-safe**；② 在取得帳號層書面確認前，
platform policy 取 **min(我方裁量, T2 值)** 作為 fail-closed 的保守側；③ 真值列入 `NEEDS_ACCOUNT_MANAGER`。
🔴 **後果是實的**：我方比 provider 寬 ⇒ 承載檢查放行 ⇒ 建單失敗 ⇒ **已收錢之後才走 `990` ＋ 向 Buyer 全額退款**。

### 12.4 ✅ Decision Input — 已於 2026-09-15（R2）由 PM 裁決 CLOSED

🔴 **最高原則：Provider does NOT redefine Hawkr Domain.**
Uber 的 Direct Account／Organization／Store 是 **provider-side model**，不是 Hawkr 的產品 Domain。

| # | 裁決 |
|---|---|
| **DI-1 CLOSED** | **1 Hawkr Seller ＝ 1 Uber Organization ＝ 1 active pickup profile ＝ 1 `external_store_id`**。<br/>🔴 **V1 明確不支援多門店**：「Organization 底下理論上可有多個 Store」**不等於**「Hawkr Seller 可建立多個 Store」 |
| **DI-2 CLOSED** | **`customer_id` ＝ Organization-level identity**，不是 Store-level |

**V1 OUT OF SCOPE（不得因 provider hierarchy 反向導入）**：
多門店／多出貨點／門店管理／門店庫存／門店 routing／店別 service hours／
買家選門店／賣家選出貨門店／訂單拆店／store selector／store-level inventory ownership。

**`external_store_id` 的 V1 語意**：Seller 唯一 pickup origin 在 provider 側的穩定識別
（概念形狀如 `HK-SELLER-<seller 業務鍵>`）。🔴 **它不是門店 ID，也不等於我方的 `seller_store_id`**
——後者是**賣家本身**的業務鍵（本 repo 的 `store_id` 指賣家不是門市）。

**Billing**：所有 Seller Organization 一律 **CENTRALIZED**；付款責任在 **Hawkr Direct Account → Uber**，
不是 Seller → Uber。🔴 **`Seller ＝ Organization` 不代表 Decentralized Billing**，70 號核心商流不變。

**未來 multi-origin（明確延後）**：若日後同一 Seller 需要多倉／3PL，那是新的 Domain 問題，
要另設計 `FulfillmentOrigin`，🔴 **本次不得預先導入**；第三方物流倉**不得被建模成 Seller 的第二間門店**。

⇒ 設計落點見 66 §5.1.2／§5.1.3a／§5.1.3b／§4.5.2a。

---

## 13. 已實查來源

### 13.1 本輪（2026-09-14，Stage 8B Step 5P-0）

| 來源 | 等級 | 取得了什麼 |
|---|---|---|
| `uber/uber-direct-sdk` → `src/deliveries/openapi.yaml`（OpenAPI 3.1.0，title `Direct API`） | **T1** | 七條端點路徑、Create Quote／Create Delivery 完整欄位、**cents 單位逐字**、`StructuredAddress`、`ManifestItem`、`DeliverableAction`／`UndeliverableAction`、三組 verification、`PincodeRequirement`、`idempotency_key`、`RelatedDelivery`、`CourierInfo`、`PODReq`、List 的三個 query 參數、7 值 status enum |
| `uber/uber-direct-sdk` → `src/organizations/openapi.yaml`（v1.0.1） | **T1** | Organizations base URL、三支端點、`required` 欄位、`BillingType`／`MerchantType`／`ContractType`／`Role`／`OnboardingInviteType` 值域、錯誤 example |
| `/docs/deliveries/guides/authentication` | **T1** | OAuth、兩個 scope、token 30 天、100 req/hr、無 IP 白名單敘述 |
| `/docs/deliveries/guides/delivery-window` | **T1** | 四個時間窗欄位的必填性、最小間距、ASAP／Scheduled 用法與 90 分鐘分界 |
| `/docs/deliveries/guides/proof-of-delivery` | **T1** | 五種驗證型別、**POD 以「有要求驗證」為前提**、API 30 天／Dashboard 7 天 |
| `/docs/deliveries/guides/pincode` | **T1** | PIN 請求形狀、值回傳在 API 回應、Uber 主動簡訊、只能 dropoff、不可與 leave-at-door 併用 |
| `/docs/deliveries/guides/robocourier` | **T1** | `test_specifications.robo_courier_specification.mode`、auto／custom、限制 |
| `/docs/deliveries/faq` | **T1** | structured address 完整範例、**apartment/unit 放 street_address** 逐字 |
| `/docs/deliveries/changelog` | **T1-c** | **Refund Submission API（2025-10-27）**、Cancel 新增 cancelation reason（2025-12-08）、`external_store_id` 字數限制（2025-04-17）、Business Location Management API（2025-11-17）、org `role_assignments`（2025-09-22） |
| `/docs/deliveries/daas/references/api/webhooks/delivery-status-webhook` | **T1** | webhook 頂層欄位、**頂層 `status` 與 `data.status` 兩者皆存在且描述相同**、8 值 status、`courier_imminent` |
| 《Uber Direct 開發指南 v2.4》（Uber 台灣業務提供，PDF，**108 頁**） | **T2** | 🔴 **已抽取全文並逐條引用**——不再是「業務口頭提供」等級的轉述：本檔每一條 T2 事實皆標 `p.NN`，現行覆蓋 **20 個相異頁**（`p.13`／`14`／`15`／`27`／`30`／`37`／`38`／`41`／`43`／`46`／`47`／`60`／`61`／`62`／`63`／`68`／`80`／`83`／`87`／`108`）。取得內容：三層式架構與上線流程、付款方式、`pickup_ready_dt` 的即時／預約兩分支（`p.38`）、Update Delivery 可改範圍（`p.41`）、取消收費 status 清單與返程運費金額關係（`p.43`／`p.38`）、返程狀態分歧表（`p.46`）、**5–10 分鐘主動回查建議與 rematch 軌跡表**（`p.47`）、材積與件數數字、台灣 structured address mapping（`p.61`）、**報價與建單時間差的三個建議**（`p.62`）、訂單編號用途（`p.63`）、PIN 三型別（`p.68`）、`public_phone_info`（`p.83`）／`sms_phone_number`（`p.87`）、對帳單欄位、現金代收、Hyperbatching |

### 13.2 仍未取得（⇒ 對應章節標 `SANDBOX_VERIFY` 或 `EXTERNAL_VERIFICATION_REQUIRED` 的原因）

- `developer.uber.com/docs/deliveries/api-reference/*` 的**線上渲染版**（前端渲染，本輪取不到；已改用官方 SDK spec）
- **Refund Submission API 的欄位表**與可用性條件
- **Business Location Management API**（2025-11-17 新增）—— 可能影響 WOK location 與 store 管理的設計
- `external_store_id` 的**實際字數上限**
- Cancel Delivery 的 **cancelation reason 欄位名與值域**
- **台灣地區可用性與費率**的書面確認
- **真實帳單（billing details report）樣本** —— 對帳鍵的唯一權威
- 帳號層的**承載上限**（§12.3）
- **Rematch 最長會持續多久／Uber 會不會自行放棄並給終態**（`EV-02`，§11.2）——T1／T2 皆未載
- **`E5 Update Delivery` 前移 `pickup_ready_dt` 後，Uber 多久重新安排取件**（`EV-07`，§2.6a）
- **pre-pickup 取消是否仍可能產生 ancillary／adjustment cost**（`EV-08`，§11.3）——
  T2 的收費 status 清單只涵蓋**正常去程運費**，未排除其他費目

🔴 **除「台灣地區可用性與費率的書面確認」外，上列皆不阻塞 implementation**：
它們影響的是 platform policy 的數值與收斂時機，不影響契約形狀。

---

## 附：T2（PDF v2.4）中與我方範圍無關、但已確認不採用的章節

現金代收（`dropoff_payment`，需 Uber 開通；遞交方式強制 meet_at_door）／
Hyperbatching 超級疊單（合約門檻：包裹 ≤1 kg、任一邊 ≤30 cm、單一取件窗 ≥30–50 單、非易腐、配送半徑 ≤3 km）
⇒ **兩者的門檻與我方情境（生鮮、單店少量、即時）直接衝突** ⇒ 66 §1.3 列為非目標**正確**。
