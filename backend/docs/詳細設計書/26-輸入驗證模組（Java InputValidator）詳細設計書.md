# 輸入驗證規範（Input Validation）

> **本文件為驗證規則的唯一管理中心**。新增驗證規則時，先在本文件定義規則，確認後再實作程式碼。

## 1. 概述

### 1.1 目的

統一全微服務的輸入驗證機制，透過共用工具類 `InputValidator`（common-core）集中管理驗證邏輯與錯誤訊息，避免各微服務散落實作、訊息不一致。

### 1.2 設計原則

| 原則 | 說明 |
|------|------|
| **集中管理** | 所有驗證方法定義在 `InputValidator`（common-core），禁止微服務自建驗證邏輯 |
| **中文錯誤訊息** | `fieldName` 參數統一使用繁體中文（如 `"商品價格"`、`"收件人電話"`），前端/使用者可直接理解 |
| **驗證位置** | Service 層（首選）或 gRPC Service 層，禁止在 Converter/Entity 層驗證 |
| **失敗即停** | 驗證失敗拋出 `IllegalArgumentException`，由上層 catch 統一轉為 gRPC `INVALID_ARGUMENT` |
| **先定義後實作** | 新增驗證規則須先在本文件 §2.1 定義，review 通過後再寫程式碼 |

### 1.3 工作流程（新增驗證規則）

```
1. 在本文件 §2.1 新增一行（方法名、回傳型別、用途、範例）
2. 在 §2.3 定義驗證規則細節（合法值、邊界值、錯誤訊息）
3. Review 確認後，在 InputValidator.java 實作
4. 在 §3 對應微服務表格新增套用位置
5. 重新編譯 common-core + 受影響微服務
6. 測試：有效值 + 邊界值 + 錯誤值
```

---

## 2. 工具類：InputValidator

**位置**：`services/java/shared/common-core/src/main/java/io/github/will413028/hawkr/common/utils/validation/InputValidator.java`

### 2.1 方法清單

| 方法 | 回傳 | 用途 | 使用範例 |
|------|------|------|---------|
| `requireNotEmpty(value, fieldName)` | void | 非空字串驗證 | `requireNotEmpty(name, "收件人姓名")` |
| `requirePositiveCm(value, fieldName)` | int | 尺寸 cm（支援小數，四捨五入，≥ 1 cm） | `requirePositiveCm("15.50", "包裝長度")` |
| `requirePositiveInt(value, fieldName)` | int | 正整數（> 0） | `requirePositiveInt("3", "購買數量")` |
| `requireNonNegativeInt(value, fieldName)` | int | 非負整數（>= 0） | `requireNonNegativeInt("0", "庫存數量")` |
| `requirePositiveAmount(value, fieldName)` | long | 正整數金額（> 0，TWD） | `requirePositiveAmount("500", "商品價格")` |
| `requireNonNegativeAmount(value, fieldName)` | long | 非負金額（>= 0） | `requireNonNegativeAmount("0", "折扣金額")` |
| `requireMobile(value, fieldName)` | String | 台灣手機（09 開頭 10 碼） | `requireMobile("0912345678", "收件人電話")` |
| `requireAddress(value, fieldName)` | String | 地址（禁英文、必含「號」） | `requireAddress("中山路1號", "收件地址")` |
| `requireNoHtmlTags(value, fieldName)` | String | 拒絕含 HTML tag 的字串（純文字檢驗） | `requireNoHtmlTags(blockContent, "商品描述")` |
| `requireHashtagSet(rawList, fieldName)` | `List<NormalizedHashtag>` | hashtag 陣列校驗（normalize + 字元 + 長度 + 上限） | `requireHashtagSet(req.getHashtagsList(), "商品 hashtag")` |
| `displayLength(value)` | int | 計算顯示長度（半形/ASCII 算 1、中文/全形算 2，對齊 PayUni 計長規則） | `displayLength("陳A")` → 3 |
| `requireDisplayLengthBetween(value, min, max, fieldName)` | String | 顯示長度（半形1/全形2）必須落在 `[min, max]` 區間，否則拒絕 | `requireDisplayLengthBetween(name, 4, 10, "收件人姓名")` |
| `truncateByDisplayLength(value, max)` | String | 依顯示長度（半形1/全形2）截斷至上限 `max`，不破壞全形字（不切半個字） | `truncateByDisplayLength(goodsName, 20)` |

> **計長規則（PayUni 對齊）**：PayUni 物流欄位長度以「英文字母/半形算 1 位、中文字/全形字符算 2 位」計算。`displayLength` 以 code point 判定：`<= 0x7F`（ASCII）算 1，其餘算 2。`requireDisplayLengthBetween` / `truncateByDisplayLength` 皆以此為基準。

### 2.2 錯誤訊息格式

所有錯誤訊息格式為：`{fieldName} {驗證失敗描述}`

| 驗證失敗場景 | 錯誤訊息範例 |
|-------------|-------------|
| 空值 | `收件人姓名 為必填欄位` |
| 數字格式不合法 | `包裝長度 格式不合法: abc` |
| 值 <= 0 | `商品價格 必須大於 0` |
| 值 < 0 | `庫存數量 不可為負數` |
| 尺寸四捨五入後為 0 | `包裝長度 四捨五入後必須至少為 1 cm` |
| 手機格式錯誤 | `收件人電話 必須為 09 開頭的 10 碼數字: 123` |
| 地址含英文 | `收件地址 僅能使用繁體中文填寫，不可包含英文` |
| 地址缺門牌 | `收件地址 格式不完整，必須包含門牌號碼（號）` |
| 顯示長度過短 | `收件人姓名 長度過短，至少需 4 個字（中文 1 字算 2、英文 1 字算 1）` |
| 顯示長度過長 | `收件人姓名 長度過長，最多 10 個字` |

### 2.3 驗證規則定義

#### R1: requirePositiveCm — 尺寸（cm）

| 項目 | 規則 |
|------|------|
| 輸入 | 字串，支援整數或小數（如 `"30"`、`"15.50"`） |
| 處理 | `Double.parseDouble` → 四捨五入為整數 |
| 有效值 | 四捨五入後 ≥ 1（即原始值 ≥ 0.5） |
| 拒絕值 | null、空字串、非數字、<= 0、四捨五入後為 0（如 `"0.4"`） |

#### R2: requirePositiveInt — 正整數

| 項目 | 規則 |
|------|------|
| 輸入 | 字串，純整數（如 `"3"`、`"100"`） |
| 有效值 | > 0 |
| 拒絕值 | null、空字串、非整數、0、負數 |

#### R3: requireNonNegativeInt — 非負整數

| 項目 | 規則 |
|------|------|
| 輸入 | 同上 |
| 有效值 | >= 0 |
| 拒絕值 | null、空字串、非整數、負數 |

#### R4: requirePositiveAmount — 正整數金額（TWD）

| 項目 | 規則 |
|------|------|
| 輸入 | 字串，純整數（如 `"500"`、`"1200"`） |
| 有效值 | > 0 |
| 拒絕值 | null、空字串、非整數、0、負數 |
| 備註 | 單位新台幣元，不接受小數 |

#### R5: requireNonNegativeAmount — 非負金額

| 項目 | 規則 |
|------|------|
| 同 R4 | 但允許 0（如折扣金額可為 0） |

#### R6: requireMobile — 台灣手機號碼

| 項目 | 規則 |
|------|------|
| 格式 | `09` 開頭，共 10 碼純數字 |
| 正則 | `^09\d{8}$` |
| 有效值 | `0912345678` |
| 拒絕值 | `12345`、`02-12345678`、`091234567`（9 碼）、`09123456789`（11 碼） |

#### R7: requireAddress — 地址（PayUni 物流規範）

| 項目 | 規則 |
|------|------|
| 僅繁體中文 | 禁止英文字母（PayUni 規定） |
| 必含「號」 | 門牌號碼為最基本的地址結構 |
| 格式 | 縣市/區/路(街)/段/巷/弄/號/(樓) |
| 有效值 | `台北市大安區安和路一段1號4樓` |
| 拒絕值 | `ABC Company 1號`（含英文）、`中山路一段`（無號） |
| 禁止簡寫 | `嘉義縣中埔鄉同仁42-4號` ❌ → `嘉義縣中埔鄉同仁村42-4號` ✅ |
| 禁止附加資訊 | 不可包含公司名稱等非地址內容 |

#### R8: requireNoHtmlTags — 純文字（拒絕 HTML tag）

| 項目 | 規則 |
|------|------|
| 用途 | 商品描述 `description.blocks[?type=text].content` 寫入前驗證；保證 `pure_description` 萃取結果為純文字 |
| 正則 | 字串中**任何子序列**匹配 `<\w[^>]*>`（startTag）或 `</\w+\s*>`（endTag）即拒絕 |
| 失敗錯誤碼 | `INVALID_DESCRIPTION_HTML_FORBIDDEN`（3014） |
| 失敗訊息 | `{fieldName} 不可包含 HTML 標籤` |
| 有效值 | 純中英文/數字/標點 |
| 拒絕值 | `<b>粗體</b>`、`<img src=...>`、`x < y > z`（含 `<` + 字母）|
| 注意 | 不主動 strip — 偵測到即拒絕，避免遮蓋上游 bug |

#### R9: requireHashtagSet — Hashtag 集合驗證

| 項目 | 規則 |
|------|------|
| 用途 | `CreateProductRequest.hashtags` / `UpdateProductRequest.hashtags` 寫入前完整校驗 |
| 輸入 | `List<String>`（賣家原始輸入；可含 `#`、空白、大小寫混雜） |
| 處理 pipeline（每 raw item）| trim → strip 開頭連續 `#` → Unicode NFC → 產生 canonical（lower）與 display（保留 case） |
| 拒絕條件（任一觸發即報錯） | 1) normalize 後為空字串 → `HASHTAG_EMPTY_AFTER_NORMALIZE`（3018）<br/>2) 字元含空白/`#`/emoji/標點/控制字元 → `HASHTAG_INVALID_CHAR`（3015）<br/>3) length（Unicode code point）超出 [1, 40] → `HASHTAG_LENGTH_EXCEEDED`（3016）<br/>4) canonical 去重後 list size > 10 → `HASHTAG_COUNT_EXCEEDED`（3017） |
| 有效字元 | 中文（含繁中、簡中、日韓表意字符若需擴充）/ ASCII letter / digit |
| 回傳 | `List<NormalizedHashtag> { canonical: String, display: String }` 依 raw 輸入序去重後保留 |
| 失敗訊息 | `{fieldName} 含不允許字元` / `{fieldName} 長度超出範圍` / `{fieldName} 數量超出上限` / `{fieldName} 規範化後為空` |
| 詳細規則 | 見 02-產品微服務設計書 §3.9 |

> `NormalizedHashtag` 為簡單 record（`record NormalizedHashtag(String canonical, String display) {}`），位於 `io.github.will413028.hawkr.product.domain.hashtag` 套件。本工具類僅做純規則檢驗，**不**負責 UPSERT hashtag 主表（屬於 Service 層職責）。

#### R10: requireDisplayLengthBetween — 顯示長度範圍驗證（PayUni 計長）

| 項目 | 規則 |
|------|------|
| 用途 | 對齊 PayUni 物流欄位的「英文 1 位、中文/全形 2 位」計長規則，驗證欄位顯示長度落在 `[min, max]` 區間。**前端可控欄位**（收件人姓名等）在寫入路徑驗證，從源頭杜絕 PayUni 建單失敗 |
| 計長 | `displayLength`：逐 char 累加，`ch <= 0x7F`（ASCII，含英文/數字/半形符號）算 1，其餘（中文/全形）算 2 |
| 輸入 | 任意字串 + `min` + `max`（顯示長度單位） |
| 有效值 | `min <= displayLength(value) <= max` |
| 拒絕值 | null、空字串、`displayLength < min`、`displayLength > max` |
| 失敗訊息 | `{fieldName} 長度過短，至少需 {min} 個字（中文 1 字算 2、英文 1 字算 1）` / `{fieldName} 長度過長，最多 {max} 個字` |
| 回傳 | 通過驗證的原字串 |

#### R11: truncateByDisplayLength — 顯示長度截斷（系統背景填寫）

| 項目 | 規則 |
|------|------|
| 用途 | **系統背景自動填寫**的欄位（商品名稱等，來源非前端可控、無法回頭要使用者改），送往 PayUni 前依顯示長度截斷至上限，避免超長導致建單失敗 |
| 計長 | 同 R10（半形 1、全形 2） |
| 輸入 | 任意字串 + `max`（顯示長度上限） |
| 處理 | 逐 char 累加顯示長度，達到 `max` 即停止；**不切半個全形字**（若加入下一個全形字會超過 `max` 則停在前一字） |
| 回傳 | 截斷後字串（未超長則原值回傳，null/空字串原樣回傳） |
| 備註 | 截斷屬「資料適配」非「驗證」，不拋例外 |

> **PayUni 物流欄位約束常數**（避免硬編碼，見 [Java 微服務開發規範](../開發規範/Java%20微服務開發規範.md) §6）：位置 `services/java/shared/common-core/src/main/java/io/github/will413028/hawkr/common/utils/validation/PayUniShippingConstraints.java`，與 `InputValidator` 同套件，供 order（結帳驗證）／ shipping（建單截斷）兩端引用。
>
> | 常數 | 值（顯示長度） | 來源 | 處理方式 |
> |---|---|---|---|
> | `CVS_RECIPIENT_NAME_MIN` | 4（2 中文 / 4 英文） | 超商 7-11 Consignee（官方下限） | R10 驗證（前端可控） |
> | `CVS_RECIPIENT_NAME_MAX` | 10（5 中文） | 超商 7-11 Consignee | R10 驗證 |
> | `HOME_RECIPIENT_NAME_MAX` | 30 | 黑貓 Consignee | R10 驗證（min=1，僅上限） |
> | `CVS_SENDER_NAME_MAX` | 10 | 超商建店 ContactName（賣家寄件人） | R10 驗證（min=1，賣家開通物流設定） |
> | `HOME_SENDER_NAME_MAX` | 30 | 黑貓建店 SenderName（賣家寄件人/退貨收件人） | R10 驗證（min=1） |
> | `RECIPIENT_ADDRESS_MAX` | 120 | 黑貓 ConsigneeAddress | R10 驗證 |
> | `HOME_GOODS_NAME_MAX` | 20 | 黑貓 ProdDesc | R11 截斷（背景填寫） |
> | `CVS_GOODS_NAME_MAX` | 500 | 超商 ProdDesc | R11 截斷（背景填寫，寬鬆保險） |
>
> **約束來源**：PayUni 官方文件明定超商取件人姓名「最長 5 個中文字、最短至少 2 個中文字或 4 個英文字（限制長度 10）」、黑貓收件人姓名「限制長度 30（中文/全形算 2、英文算 1）」、商品名稱「最長 20 位」。`CVS_RECIPIENT_NAME_MIN=4` 為官方下限；低於此下限時 PayUni 回 `SHIP02114` 拒絕建單。
>
> **單字中文姓名邊界**：依官方「最短 2 個中文字」，單名（如「王」= 顯示長度 2）會被本驗證拒絕——此為 PayUni 規格要求（超商取件需核對身分，不接受單字姓名），非過度限制。買家結帳會在源頭收到明確提示要求補足。
>
> PayUni 約束細節見 [PayUni 物流官方 API 總覽](../第三方整合/PayUni%20物流官方%20API%20總覽.md) §4（建立超商物流單，Consignee/ProdDesc）/ §5（建立宅配單（黑貓），Consignee/ConsigneeAddress/ProdDesc）。

---

## 3. 各微服務驗證套用清單

### 3.1 Order Service（結帳建單）

| 驗證位置 | 欄位 | 驗證方法 | fieldName |
|---------|------|---------|-----------|
| CheckoutService.confirm | recipientName（超商取貨） | `requireDisplayLengthBetween(name, CVS_RECIPIENT_NAME_MIN, CVS_RECIPIENT_NAME_MAX, ...)` = `[4, 10]` | 收件人姓名 |
| CheckoutService.confirm | recipientName（宅配） | `requireDisplayLengthBetween(name, 1, HOME_RECIPIENT_NAME_MAX, ...)` = `[1, 30]`（先 `requireNotEmpty`） | 收件人姓名 |
| CheckoutService.confirm | recipientPhone | `requireMobile` | 收件人電話 |
| CheckoutService.confirm | shippingAddress（宅配時） | `requireAddress` + `requireDisplayLengthBetween(addr, 1, RECIPIENT_ADDRESS_MAX, ...)` = 上限 120 | 收件地址 |
| **OrderService.createOrder（建單收斂點最後防線）** | recipientName（依 shippingMethod）+ shippingAddress | 同 confirm 規則（超商 [4,10] / 宅配 ≤30；地址 ≤120）。**縱深防禦**：所有「帶收件人建單」路徑共同收斂點，防繞過 confirm 的入口（06 §2.5） | 收件人姓名 / 收件地址 |
| CheckoutService.confirm | shippingMethod | `requireNotEmpty` | 配送方式 |
| CheckoutService.confirm | deliveryTemp | `requireNotEmpty` | 配送溫層 |
| CheckoutService.resolveItems | quantity | `requirePositiveInt` | 購買數量 |
| CheckoutService.resolveItems | price | `requirePositiveAmount` | 商品價格 |
| CheckoutService.preview/confirm | discountAmount | `requireNonNegativeAmount` | 折扣金額 |
| PaymentEventHandler | amount | `> 0` 檢查（MQ 不拋異常，記錄並跳過） | — |

### 3.2 Product Service（商品建立/更新）

| 驗證位置 | 欄位 | 驗證方法 | fieldName |
|---------|------|---------|-----------|
| ProductService.createProduct | price | `== null \|\| <= 0` → 拒絕 | 商品價格 |
| ProductService.createProduct/updateProduct | price（更新） | `!= null && <= 0` → 拒絕 | 商品價格 |
| ProductService.createProduct/updateProduct | stockQty | `!= null && < 0` → 拒絕 | 庫存數量 |
| ProductService.createProduct/updateProduct | weight | `!= null && <= 0` → 拒絕 | 商品重量 |
| ProductService.createProduct/updateProduct | lengthCm/widthCm/heightCm | `!= null && <= 0` → 拒絕 | 包裝長度/寬度/高度 |
| ProductGrpcService.calculateShippingOptions | length/width/height | `requirePositiveCm` | length/width/height |
| ProductService | unit | `CodeMetaValidator.requireValidCode` | unit |
| ProductService.createProduct/updateProduct | description.blocks[?type=text].content | `requireNoHtmlTags`（逐 block 套用） | 商品描述 |
| ProductService.createProduct/updateProduct | hashtags | `requireHashtagSet` | 商品 hashtag |

### 3.3 Shipping Service（物流建店 + 出貨 Strategy）

| 驗證位置 | 欄位 | 驗證方法 | fieldName |
|---------|------|---------|-----------|
| OnboardingService.onboardCvsShipping | senderName | `requireDisplayLengthBetween(1, CVS_SENDER_NAME_MAX)` = 上限 10 | 寄件人姓名 |
| OnboardingService.onboardCvsShipping | senderPhone | `requireMobile` | 寄件人電話 |
| OnboardingService.onboardHomeDelivery | senderName | `requireDisplayLengthBetween(1, HOME_SENDER_NAME_MAX)` = 上限 30 | 寄件人姓名 |
| OnboardingService.onboardHomeDelivery | returnName（選填） | `requireDisplayLengthBetween(1, HOME_SENDER_NAME_MAX)`（有值才驗） | 退貨收件人姓名 |
| OnboardingService.onboardHomeDelivery | senderPhone | `requireMobile` | 寄件人電話 |
| OnboardingService.onboardHomeDelivery | senderAddressDetail | `requireAddress` | 寄件詳細地址 |
| OnboardingService.onboardHomeDelivery | returnPhone（選填） | `requireMobile` | 退貨聯絡人電話 |
| OnboardingService.onboardHomeDelivery | returnAddressDetail（選填） | `requireAddress` | 退貨詳細地址 |
| Home/Cvs/UberShipmentStrategy | sellerAccountId/buyerAccountId/totalActualAmount | `ProtoConversionUtils.parseLongRequired` | — |
| HomeShipmentStrategy | goodsName（送 PayUni 前） | `truncateByDisplayLength(name, HOME_GOODS_NAME_MAX)` = 截斷至 20 | — |
| CvsShipmentStrategy | goodsName（送 PayUni 前） | `truncateByDisplayLength(name, CVS_GOODS_NAME_MAX)` = 截斷至 500 | — |
| ShipmentLabelService | assetId | `ProtoConversionUtils.parseLongRequired` | — |

> **背景填寫截斷說明**：商品名稱來自訂單商品（系統撈取，非前端可控、無法回頭要使用者改），故採 R11 截斷而非 R10 驗證拒絕。收件人姓名 / 地址為前端可控，採 R10 源頭驗證拒絕，讓使用者在結帳時即修正。送往 PayUni 的 goodsName 一律套 `truncateByDisplayLength`，不論來源是固定字串或動態商品名稱。

### 3.4 Payment Service

| 驗證位置 | 欄位 | 驗證方法 | fieldName |
|---------|------|---------|-----------|
| PaymentApplicationService | amount | `requirePositiveAmount` | 付款金額 |
| PaymentApplicationService | buyerAccountId | `requireNotEmpty` | 買家帳號 |
| PaymentApplicationService | merId | `requireNotEmpty` | 商家編號 |
| PaymentApplicationService | orderNo | `requireNotEmpty` | 訂單編號 |
| PaymentApplicationService | providerTradeNo | `requireNotEmpty` | 金流交易編號 |
| PaymentApplicationService | applePayToken | `requireNotEmpty` | Apple Pay 付款憑證 |
| PaymentApplicationService | googlePayToken | `requireNotEmpty` | Google Pay 付款憑證 |

### 3.5 Coupon Service

| 驗證位置 | 欄位 | 驗證方法 | fieldName |
|---------|------|---------|-----------|
| CouponGrpcService.sellerCreateTemplateAndIssue | startAt | `ProtoConversionUtils.parseIso8601Required` | start_at |
| CouponGrpcService.sellerCreateTemplateAndIssue | endAt | `ProtoConversionUtils.parseIso8601Required` | end_at |
| CouponGrpcService.adminCreateTemplate | startAt | `ProtoConversionUtils.parseIso8601Required` | start_at |
| CouponGrpcService.adminCreateTemplate | endAt | `ProtoConversionUtils.parseIso8601Required` | end_at |
| CouponTemplateService.validateTemplateParams | startAt/endAt | null 檢查 + `startAt ≤ endAt` | start_at / end_at |
| CouponGrpcService.adminGrantCompensationCoupon | amount | `> 0` 檢查 | 補償金額 |

### 3.6 Media Service

| 驗證位置 | 欄位 | 驗證方法 | fieldName |
|---------|------|---------|-----------|
| SecureMediaController.secureDownload | assetId | `requireNotEmpty` | 資產識別碼 |

### 3.7 未套用（評估後保留原有機制）

| 微服務 | 原因 |
|--------|------|
| Cart Service | 用 `StatusRuntimeException` 處理驗證錯誤，改 `InputValidator` 會讓錯誤碼從 INVALID_ARGUMENT 降為 INTERNAL |
| Media AssetReferenceService | 用 `MediaServiceException` 處理，改了會從 400 降為 500 |

---

## 4. 重量轉換（WeightConverter）

### 4.1 運算順序

**先乘以單位換算係數，再四捨五入**，不得先四捨五入再換算——後者會讓所有小於 1 的重量值失真：

```
0.5 KG → 0.5 × 1000 = 500.0 → round = 500g
```

### 4.2 使用此轉換的路徑

| API | 用途 |
|-----|------|
| `POST /app/shipping_quotes` | 運費預估重量計算 |
| `POST /app/checkout/confirm` | 結帳運費計算 |
| `POST /app/products`（配送驗證） | 商品建立時重量驗證 |

### 4.3 尺寸小數支援

`shipping_quotes` 的 length/width/height 欄位支援小數字串（如 `"15.50"`），四捨五入為整數 cm，四捨五入後必須 ≥ 1 cm。

---

## 5. 禁止事項

| 禁止行為 | 正確做法 |
|---------|---------|
| 手寫 `if (x == null \|\| x.isEmpty()) throw ...` | `InputValidator.requireNotEmpty(x, "欄位中文名")` |
| 手寫 `if (qty <= 0) throw ...` | `InputValidator.requirePositiveInt(qty, "購買數量")` |
| 各服務自建驗證方法 | 使用 `InputValidator` 靜態方法 |
| fieldName 用英文 | 統一繁體中文（`"商品價格"` 非 `"price"`） |
| 在 Converter 層驗證 | 在 Service 層驗證（見 §1.2 驗證位置） |
| 直接寫程式碼不定義規則 | 先在本文件 §2.1 + §2.3 定義，review 後再實作 |

---

## 6. 新增驗證規則範本

新增規則時，複製以下範本到 §2.3：

```markdown
#### R{N}: {方法名} — {用途}

| 項目 | 規則 |
|------|------|
| 輸入 | {輸入格式描述} |
| 有效值 | {合法範圍} |
| 拒絕值 | {非法值列表} |
| 備註 | {其他注意事項} |
```

並在 §2.1 方法清單新增一行，§3 對應微服務表格新增套用位置。
