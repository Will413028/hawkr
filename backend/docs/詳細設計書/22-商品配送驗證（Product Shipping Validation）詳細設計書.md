# 商品配送驗證（Product Shipping Validation）詳細設計書

---

### 1. 功能概述

#### 1.1 文件定位

本文件描述商品建立／更新時對 SKU 配送方式的前置驗證契約：shipping-service 的 `ValidateProductShipping` RPC，以及 product-service 在寫入 DB 前的呼叫與失敗處置。

驗證必須在寫入 DB 前完成——SKU 帶著賣家未開通的配送方式或超出材積限制的規格落庫後，買家瀏覽與加入購物車都不會有異常，一路到結帳才會出現「無可用配送方式」。

#### 1.2 驗證要達成的效果

- 商品建立／更新時即時驗證 SKU 的配送方式是否合法
- 驗證失敗回傳**明確錯誤訊息**（含具體限制值），前端可即時提示賣家

#### 1.3 涉及服務

| 服務 | 責任 |
|------|---------|
| Shipping Service | 提供 `ValidateProductShipping` RPC |
| Product Service | 建立／更新商品時呼叫驗證 RPC 並處理結果 |

---

### 2. Shipping Service — ValidateProductShipping RPC

#### 2.1 與 CalculateShippingOptions 的語意差異

shipping-service 有兩支語意不同的 RPC，不可互相替代：

| | `CalculateShippingOptions` | `ValidateProductShipping` |
|---|---|---|
| 輸入 | 單一商品維度 | 多個 SKU，各自的配送方式與材積 |
| 輸出 | 只回可用選項，被過濾掉的不出現 | 逐項回驗證結果 ＋ 拒絕原因 ＋ 具體限制值 |
| 用途 | 結帳／前端展示可用配送選項 | 商品建立／更新的前置驗證 |

前者回傳的空集合無法分辨「全部不可用」與「某一項為什麼不可用」，因此不能用來產生給賣家看的錯誤訊息。

#### 2.2 Proto 定義

```protobuf
rpc ValidateProductShipping(ValidateProductShippingRequest) returns (ValidateProductShippingResponse);

// 請求：每個 SKU 各自的配送方式 + 材積
message ValidateProductShippingRequest {
    int64 store_id = 1;                                   // 賣家 store 代理鍵（內部服務呼叫）
    repeated ValidateShippingItem items = 2;              // 待驗證的 SKU 配送項目
}

message ValidateShippingItem {
    string sku_name = 1;              // SKU 名稱（用於錯誤訊息識別）
    string shipping_method = 2;       // 5 值：3001 TAQBIN / 3002 SEVEN_ELEVEN_STORE_PICKUP / 3003 UBER_INSTANT / 3004 7-11 B2C 大宗寄倉 / 3005 MERCHANT_DELIVERY
                                      // 🔴 一律用 value 形式（3001~3005）與 DB 的 chk_shipping_method 一致，不用英文名
    string temperature = 3;           // 3 值：3201 AMBIENT / 3202 CHILLED / 3203 FROZEN
                                      // 🔴 與 shipping_method 同一條規則：一律用 value 形式（3201~3203），不用英文名。
                                      // 依據＝`ShippingTemperature` CodeMeta（`AMBIENT("AMBIENT","3201",...)`），
                                      // 且 06 §3.7 的 `shipments.delivery_temp` 存的就是 value（3201/3202/3203）
    string length_cm = 4;
    string width_cm = 5;
    string height_cm = 6;
    string weight_grams = 7;          // 已轉換為公克
}

// 回應
message ValidateProductShippingResponse {
    int32 code = 1;
    string message = 2;
    bool all_valid = 3;                                    // 全部通過 = true
    repeated ValidateShippingResult results = 4;           // 各項目的驗證結果
}

message ValidateShippingResult {
    string sku_name = 1;
    string shipping_method = 2;
    string temperature = 3;
    bool valid = 4;
    string rejection_reason = 5;      // 拒絕原因代碼（見下表）
    string rejection_message = 6;     // 人類可讀的拒絕訊息（繁體中文）
    string max_dimension_sum_cm = 7;  // 材積上限（L+W+H），僅在 DIMENSION_SUM_EXCEEDED 時有值
    string max_single_side_cm = 8;    // 單邊上限，僅在 SINGLE_SIDE_EXCEEDED 時有值
    string max_weight_grams = 9;      // 重量上限，僅在 WEIGHT_EXCEEDED 時有值
}
```

#### 2.3 拒絕原因代碼

| rejection_reason | 說明 | rejection_message 範例 |
|-----------------|------|----------------------|
| `NOT_ONBOARDED` | 賣家未開通此配送方式 | `賣家尚未開通 TAQBIN 配送方式` |
| `METHOD_NOT_SUPPORTED` | 配送方式+溫層組合不存在 | `SEVEN_ELEVEN_STORE_PICKUP 不支援 CHILLED 溫層` |
| `DIMENSION_SUM_EXCEEDED` | 材積超出上限 | `材積總和 180cm 超過 TAQBIN/AMBIENT 上限 150cm` |
| `SINGLE_SIDE_EXCEEDED` | 單邊長度超出上限 | `最長邊 50cm 超過 SEVEN_ELEVEN_STORE_PICKUP/AMBIENT 上限 45cm` |
| `WEIGHT_EXCEEDED` | 重量超出上限 | `重量 25000g 超過 TAQBIN/AMBIENT 上限 20000g` |

#### 2.4 驗證邏輯

```
對每個 ValidateShippingItem：
1. 查詢賣家配送開通狀態（SellerShipmentSettingEntity）
   → 未開通 → NOT_ONBOARDED

2. 查詢 shipping_method_configs 取得 (method, temp) 的限制
   → 找不到 → METHOD_NOT_SUPPORTED

3. 計算 dimensionSum = length + width + height
   計算 longestSide = max(length, width, height)
   （備註：length/width/height 已由 §2.7 parseDecimalToInt 處理為整數 cm，
     向 0 truncate；以下比較皆為整數運算，不存在浮點誤差問題）

4. 檢查材積：
   config.maxDimensionSumCm > 0 且 dimensionSum > maxDimensionSumCm
   → DIMENSION_SUM_EXCEEDED

5. 檢查單邊：
   config.maxSingleSideCm > 0 且 longestSide > maxSingleSideCm
   → SINGLE_SIDE_EXCEEDED

6. 檢查重量：
   config.maxWeightGrams > 0 且 weightGrams > maxWeightGrams
   → WEIGHT_EXCEEDED

7. 全部通過 → valid = true
```

**注意**：驗證邏輯與 `ShippingFeeService.calculateShippingOptions()` 的過濾邏輯一致（§ ShippingFeeService L186-194），但此處回傳拒絕原因而非靜默過濾。

#### 2.5 實作位置

| 層級 | 類別 | 職責 |
|------|------|------|
| Proto | `api/proto/shipping/shipping.proto` | RPC 與 message 契約 |
| gRPC Handler | `ShippingGrpcServiceImpl.validateProductShipping()` | 入參解析（§2.7）與錯誤碼映射 |
| Service | `ShippingFeeService.validateProductShipping()` | 逐項驗證，使用 configCache 與賣家開通狀態查詢 |

#### 2.6 重用現有基礎設施

- **configCache**（`ShippingFeeService`）：啟動時載入 + 每小時刷新的 `shipping_method_configs` 快取
- **OnboardingService**：`listSellerShippingMethods()` 查詢賣家開通狀態
- **無需新增 DB 表/欄位**

> 🔴 **配送方式解析的現行涵蓋範圍**：`mapShippingMethodToType` 走 `fulfillmentProviders.find(shippingMethod)` registry，
> 而 registry 內只有 `3001`（`PayUniHomeAdapter`）、`3002`（`PayUniCvsAdapter`）、`3003`（`UberDirectAdapter`）三個 adapter。
> ⇒ `3004`／`3005` 取不到建店設定，本 RPC 對這兩個值一律判為 `METHOD_NOT_ONBOARDED`，
> 即使賣家實際上有對應設定。缺口與修法追蹤於
> #3502。
>
> ⚠️ `configCache` 載入時 filter `enabled = true`，因此 `enabled=false` 的列在快取裡與「不存在」無法分辨——
> 兩者都會走到 `METHOD_NOT_AVAILABLE`。新增任何 method 的 config row 時要一併確認這個行為是否符合預期。

#### 2.7 輸入解析與錯誤回應契約

數字字串欄位（`length_cm` / `width_cm` / `height_cm` / `weight_grams`）的解析契約：

| 輸入 | 處理 |
|---|---|
| 純整數字串（"18"、"600"） | 解析為對應整數 |
| 十進位字串（"18.00"、"18.5"、"0.6"） | 以 `BigDecimal.intValueExact()` 解析失敗則退回 `intValue()` truncate（向 0 取整）；caller 仍應保證業務輸入是非負合理範圍 |
| 科學記號（"5e2"、"1.8E3"） | `BigDecimal` 建構子合法解析（"5e2" = 500），與整數字串等效，視為合法 |
| 負數字串（"-18.00"） | 解析成功，但物理量負值無業務意義；本層**不在 helper 內額外驗證**（保持解析語意純淨），由 ShippingFeeService 業務邏輯各自處理（負 dim 走 SINGLE_SIDE_EXCEEDED / DIMENSION_SUM_EXCEEDED 等天然不通過）|
| 超 int 範圍（如 "9999999999"） | `BigDecimal.intValueExact()` 拋 `ArithmeticException` → 包成 `IllegalArgumentException`：「<field> 超出 int 範圍: <value>」。**禁止**靜默 narrowing overflow |
| 空字串 / null | 拋 `IllegalArgumentException`：「<field> 不可為空」 |
| 非數字（"abc"） | 拋 `IllegalArgumentException`：「<field> 格式不合法: abc」 |

> 設計動機：DB 商品 SKU 維度欄位多為 `numeric(8,2)`，序列化（`BigDecimal.toPlainString()`）會帶 trailing zero（"18.00"）。Proto 字串型別 contract 不要求一定整數；handler 應寬容處理避免 caller 端被迫多一層 round。

實作層級：由 `common-core/ProtoConversionUtils.parseDecimalToInt(String value, String fieldName)` 與 `parseDecimalToLong(String value, String fieldName)` 統一處理，shipping-service handler 一律使用此 helper（不可再用 `parseIntRequired` / `parseLongRequired`）。helper 行為：

```java
public static int parseDecimalToInt(String value, String fieldName) {
    if (value == null || value.isBlank()) {
        throw new IllegalArgumentException(fieldName + " 不可為空");
    }
    try {
        BigDecimal d = new BigDecimal(value.trim());
        // 先嘗試 exact 整數，失敗則 truncate（小數部分捨棄）
        try {
            return d.intValueExact();
        } catch (ArithmeticException overflowOrFraction) {
            // 區分：超 int 範圍 vs 含小數
            BigInteger big = d.toBigInteger();    // truncate
            if (big.bitLength() >= 32) {
                throw new IllegalArgumentException(fieldName + " 超出 int 範圍: " + value);
            }
            return big.intValue();
        }
    } catch (NumberFormatException e) {
        throw new IllegalArgumentException(fieldName + " 格式不合法: " + value, e);
    }
}
```

`parseDecimalToLong` 同邏輯，bit-length 改為 63、`intValueExact` 改 `longValueExact`、`bigInteger.longValue()`。

**錯誤回應契約**：

| 階段 | response.code | response.allValid | response.results |
|---|---|---|---|
| 全部 SKU 驗證完畢 | `SUCCESS (0)` | true / false | 完整 list（每 item 一筆） |
| 輸入解析失敗（非有效數字 / 必填空） | `INVALID_PARAM (400)` | false（default） | `[]`（default） |
| 業務例外（如 OnboardingService 拋 ShippingBusinessException） | 該例外的 code | false | `[]` |
| 其他未預期例外 | `INTERNAL_ERROR` | false | `[]` |

**Caller 對 response.code 的處理（強制）**：

- `SUCCESS` → 依 `allValid` / `results` 業務處理
- 非 `SUCCESS` → 視為「呼叫失敗」，**不可**僅看 `allValid` / `results` 判斷「無 violation」
  - product-service `ShippingGrpcClient.validateProductShipping` 必須檢查 `response.getCode()`，非 SUCCESS 拋 RuntimeException 給 caller 走 degraded 路徑
  - 🔴 這條規則的失效是**完全靜默的**：handler 若把 `INVALID_PARAM` 包成正常 response（`code=400, results=[]`），而 client 不檢 code 直接 return，product-service 會把空 `results` 當成「無 violation」放行，所有 shipping violation 都不會寫進 `product_import_item_error`，沒有任何錯誤訊息

---

### 3. Product Service — 整合驗證

#### 3.1 呼叫時機

| 方法 | 時機 | 說明 |
|------|------|------|
| `ProductService.createProduct()` | SKU Enum 驗證後、寫入 DB 前 | 全部 SKU 的配送方式一次驗證 |
| `ProductService.updateProduct()` | SKU Enum 驗證後、更新 DB 前 | 僅驗證有變更的 SKU |

#### 3.2 呼叫流程

```
ProductService.createProduct(dto):
  1. 既有驗證（idempotency、unit CodeMetaValidator...）
  2. 收集所有 SKU 的配送方式 → 轉換重量為公克
  3. 呼叫 ShippingGrpcClient.validateProductShipping(storeId, items)
  4. 若 allValid = false → 拋出 IllegalArgumentException（帶拒絕訊息）
  5. toEntity → insert → 快取清除
```

#### 3.3 重量轉換

SKU 的重量以 `weight`（Decimal）+ `unit`（WeightUnit.value()）儲存，呼叫驗證 RPC 前需轉換為公克：

```java
// 使用 common-core WeightConverter（`docs/開發規範/Java 微服務開發規範.md` §7.1 強制規則）
long weightGrams = WeightConverter.convertToGrams(sku.getWeight(), WeightUnit.fromValue(sku.getUnit()));
```

#### 3.4 錯誤訊息格式

驗證失敗時，組合所有失敗項目的 `rejectionMessage`，以逗號分隔回傳：

```
SKU「規格A」的 TAQBIN/AMBIENT 驗證失敗：材積總和 180cm 超過上限 150cm; SKU「規格B」的 SEVEN_ELEVEN_STORE_PICKUP/AMBIENT 驗證失敗：賣家尚未開通此配送方式
```

#### 3.5 實作位置

| 層級 | 類別 | 職責 |
|------|------|------|
| 共用工具（common-core） | `ProtoConversionUtils` | 提供 `parseDecimalToInt(String, String)` / `parseDecimalToLong(String, String)`（見 §2.7）。本 RPC 的 dim／weight 欄位**一律用這兩支**，不得用 `parseIntRequired` / `parseLongRequired`（它們不接受小數字串） |
| gRPC Server | `ShippingGrpcServiceImpl.validateProductShipping` | dim/weight 以 `parseDecimalToInt` / `parseDecimalToLong` 解析 |
| gRPC Client | `ShippingGrpcClient.validateProductShipping`（product-service） | **必檢 `response.getCode()`**，非 SUCCESS 拋 RuntimeException |
| Service | `ProductService` | `createProduct()` / `updateProduct()` 中新增驗證呼叫 |
| Validator | `ProductPublishValidator.validateShippingEnabled`（product-service） | catch RuntimeException → log ERROR + degraded return；**禁止**僅依 `getAllValid()` / `getResultsCount()` 判斷 |

#### 3.6 邊界情況

| 情境 | 處理方式 |
|------|---------|
| SKU 無配送方式 | 跳過驗證（空陣列合法） |
| SKU 缺少材積欄位（length/width/height） | 跳過驗證（僅驗證有材積資料的 SKU） |
| Shipping Service 不可用 / 回 INVALID_PARAM | Client 拋 RuntimeException；Validator catch 後 log ERROR + degraded return，**不阻擋上架但記事件供查**（`ProductPublishValidator.validateShippingEnabled.degraded`） |
| 同一 SKU 有多個配送方式 | 每個配送方式各自驗證 |
| SKU dim/weight 字串含小數（"18.00"） | shipping-service handler 走 `parseDecimalToInt` truncate 為整數（見 §2.7） |

---

### 4. Proto 契約落點

| 檔案 | 內容 |
|------|------|
| `api/proto/shipping/shipping.proto` | `ValidateProductShipping` RPC ＋ 4 個 message |

**同步流程**（`docs/開發規範/Proto 管理規範.md` §3）：
1. 修改 `api/proto/shipping/shipping.proto`
2. `sh scripts/gen-go-proto.sh`
3. `rsync -av --delete api/proto/ services/java/shared/java-proto/src/main/proto/`
4. `cd services/java/shared/java-proto && mvn clean install`

---

### 5. 不在本契約內

| 項目 | 說明 |
|------|------|
| DB Schema | 材積限制的權威在 `shipping_method_configs` 表，本驗證只讀不寫 |
| Gateway | 驗證發生在微服務之間，不對外暴露 HTTP API |
| 前端 API | 驗證整合在既有的建立／更新商品 API 內，前端不需另外呼叫 |

---

### 6. 測試計畫

1. **Shipping Service**：呼叫 `ValidateProductShipping` RPC，驗證各拒絕原因正確回傳
2. **Product Service**：建立商品時帶入超出限制的材積，確認被拒絕並回傳明確訊息
3. **Product Service**：建立商品時帶入未開通的配送方式，確認被拒絕
4. **Product Service**：建立商品時帶入合法配送方式 + 材積，確認正常建立
