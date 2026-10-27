# 門市管理模組（Store Pickup Management）詳細設計書

---

### **1. 模組概述**

本模組擴展 Shipping Service，提供**買家常用門市管理**與**賣家退貨門市資訊擴充**兩大功能。

#### **1.1 功能範圍**

| 功能 | 說明 |
|------|------|
| 買家常用門市 CRUD | 買家可儲存、修改、查詢、刪除常用超商門市，支援預設門市設定（4 支買家端點） |
| 賣家退貨門市資訊擴充 | 退貨門市的代碼、名稱與地址一併由現有超商建店 API 收與回，不另設端點 |

#### **1.2 服務歸屬**

門市資料的 owner 是 **Shipping Service**：`buyer_store_pickups` 與
`seller_shipment_settings.return_store_*` 皆落在 `hawkr_shipping`，
由 `StoreMapService`／`OnboardingService`／`CvsShipmentStrategy` 共同消費。

Buyer Service 的地址管理只負責**宅配地址**，不得儲存門市資料；
門市代碼的加解密一律由 Shipping Service 對 PayUni 門市地圖 API 處理。

#### **1.3 系統架構**

```
買家 App                                      賣家 App
  │                                             │
  ├─ POST   /app/store_pickups                 ├─ POST /app/shipping_onboardings/cvs（現有，擴充參數）
  ├─ PUT    /app/store_pickups/{id}            ├─ GET  /app/shipping_onboardings（現有，擴充回應）
  ├─ GET    /app/store_pickups                 └─ GET  /app/shipping_onboardings/{id}（現有，擴充回應）
  └─ DELETE /app/store_pickups/{id}
  │                                             │
  └──────────── Gateway (HTTP ↔ gRPC) ──────────┘
                         │
                Shipping Service (gRPC)
                         │
                   PostgreSQL (hawkr_shipping)
                   ├─ buyer_store_pickups
                   └─ seller_shipment_settings（含退貨門市三欄）
```

---

### **2. 資料庫設計**

#### **2.1 買家常用門市表 buyer_store_pickups**

一個買家可以儲存多個常用門市，但預設門市最多一個。上限由應用層控制（預設 20 筆，可由配置參數調整）。

| **欄位名稱** | **類型** | **必填** | **說明 / 備註** |
|---|---|---|---|
| id | BIGINT | PK | TSID（`TsidUtil.generate()`） |
| buyer_id | BIGINT | YES | 買家 `buyer_id`（stint-scoped surrogate，非 JWT `account_id`；由 Gateway 解析後傳入） |
| store_code | VARCHAR(20) | YES | 門市代碼（PayUni 門市地圖回傳，6 碼） |
| store_name | VARCHAR(100) | YES | 門市名稱 |
| store_address | VARCHAR(200) | YES | 門市地址 |
| receiver_name | VARCHAR(50) | YES | 收件人姓名 |
| receiver_phone | VARCHAR(20) | YES | 收件人電話 |
| is_default | BOOLEAN | YES | DEFAULT FALSE，是否為預設門市 |
| created_at | TIMESTAMPTZ | YES | DEFAULT NOW() |
| updated_at | TIMESTAMPTZ | NO | 由 DB trigger 自動維護 |

**索引：**

- `idx_bsp_buyer (buyer_id)` — 依買家查詢門市列表
- `idx_bsp_buyer_default (buyer_id) WHERE is_default = TRUE` — 快速查找預設門市（Partial Index，確保邏輯唯一性檢查高效）

**資料保留語意：**

本表**不訂閱 `buyer.deleted` 做主動清除**（與 order／review／cart／aftersales-service 一致）。
買家軟刪除復活後會拿到新的 `buyer_id`，舊 stint 的常用門市因 `buyer_id` 不匹配而結構性不可見。
其中的 `receiver_name`／`receiver_phone` 屬 PII，會無限期留存但不可查——這是已知且可接受的限制。

**約束：**

- 無 DB 層唯一約束強制「一個買家只有一個預設」（由應用層處理，避免 race condition 導致的 constraint violation）
- 同一買家可以儲存同一門市代碼多次（不同收件人/電話）

**預設門市邏輯（應用層）：**

1. **新增時** `isDefault = true` → 先將該買家所有既有門市的 `is_default` 設為 `false`，再插入新門市
2. **修改時** `isDefault` 從 `false` → `true` → 同上，先清除舊預設
3. **修改時** `isDefault` 從 `true` → `false` → 直接更新，此時該買家無預設門市（合法狀態）
4. **刪除預設門市** → 該買家無預設門市（合法狀態，不自動指派新預設）

#### **2.2 賣家建店設定表 seller_shipment_settings**

退貨門市的三個欄位與建店設定同表，粒度為 per seller × shipping_type × goods_type：

| **欄位** | **類型** | **必填** | **說明 / 備註** |
|---|---|---|---|
| return_store_code | VARCHAR(20) | NO | 退貨門市代碼（6 碼） |
| return_store_name | VARCHAR(100) | NO | 退貨門市名稱 |
| return_store_address | VARCHAR(200) | NO | 退貨門市地址 |

建單時一次讀出三欄，不需 JOIN。

---

### **3. gRPC API 設計**

#### **3.1 買家常用門市 RPC**

所有新增 RPC 定義在 `api/proto/shipping/shipping.proto`。

| RPC | 說明 | 角色 |
|-----|------|------|
| `AddBuyerStorePickup` | 新增常用門市 | BUYER / SELLER_AND_BUYER |
| `UpdateBuyerStorePickup` | 修改常用門市 | BUYER / SELLER_AND_BUYER |
| `ListBuyerStorePickups` | 取得常用門市列表 | BUYER / SELLER_AND_BUYER |
| `DeleteBuyerStorePickup` | 刪除常用門市 | BUYER / SELLER_AND_BUYER |

##### 3.1.1 AddBuyerStorePickup

```protobuf
message AddBuyerStorePickupRequest {
    int64 buyer_id = 1;                // Gateway 解析後的現任 buyer_id（非 raw JWT account_id）
    string store_code = 2;            // 門市代碼（6 碼）
    string store_name = 3;            // 門市名稱
    string store_address = 4;         // 門市地址
    string receiver_name = 5;         // 收件人姓名
    string receiver_phone = 6;        // 收件人電話
    bool is_default = 7;              // 是否設為預設
}

message AddBuyerStorePickupResponse {
    int32 code = 1;
    string message = 2;
    BuyerStorePickupView data = 3;
}
```

##### 3.1.2 UpdateBuyerStorePickup

```protobuf
message UpdateBuyerStorePickupRequest {
    int64 buyer_id = 1;                // Gateway 解析後的現任 buyer_id（非 raw JWT account_id）
    string store_pickup_id = 2;       // 門市記錄 ID（TSID 數字字串）
    string store_code = 3;            // 門市代碼
    string store_name = 4;            // 門市名稱
    string store_address = 5;         // 門市地址
    string receiver_name = 6;         // 收件人姓名
    string receiver_phone = 7;        // 收件人電話
    bool is_default = 8;              // 是否設為預設
}

message UpdateBuyerStorePickupResponse {
    int32 code = 1;
    string message = 2;
    BuyerStorePickupView data = 3;
}
```

##### 3.1.3 ListBuyerStorePickups

```protobuf
message ListBuyerStorePickupsRequest {
    int64 buyer_id = 1;                // Gateway 解析後的現任 buyer_id（非 raw JWT account_id）
}

message ListBuyerStorePickupsResponse {
    int32 code = 1;
    string message = 2;
    repeated BuyerStorePickupView data = 3;
}
```

##### 3.1.4 DeleteBuyerStorePickup

```protobuf
message DeleteBuyerStorePickupRequest {
    int64 buyer_id = 1;                // Gateway 解析後的現任 buyer_id（非 raw JWT account_id）
    string store_pickup_id = 2;       // 門市記錄 ID（TSID 數字字串）
}

message DeleteBuyerStorePickupResponse {
    int32 code = 1;
    string message = 2;
}
```

##### 3.1.5 BuyerStorePickupView（共用回應結構）

```protobuf
message BuyerStorePickupView {
    string id = 1;                    // TSID 數字字串
    string store_code = 2;            // 門市代碼
    string store_name = 3;            // 門市名稱
    string store_address = 4;         // 門市地址
    string receiver_name = 5;         // 收件人姓名
    string receiver_phone = 6;        // 收件人電話
    string is_default = 7;            // "true" / "false"
    string created_at = 8;            // ISO 8601
    string updated_at = 9;            // ISO 8601
}
```

#### **3.2 賣家退貨門市（掛在既有建店 Proto）**

賣家退貨門市不佔用獨立 RPC，語意掛在既有建店 Message 上：

##### 3.2.1 OnboardCvsShippingRequest

```protobuf
message OnboardCvsShippingRequest {
    int64 store_id = 1;               // Gateway JWT 注入的賣家 store 代理鍵
    string goods_type = 2;            // 貨物類型 AMBIENT / FROZEN
    string return_store_code = 3;     // 退貨門市代碼（6 碼）
    string return_store_name = 4;     // 退貨門市名稱
    string return_store_address = 5;  // 退貨門市地址
}
```

##### 3.2.2 ShippingOnboardingView

```protobuf
message ShippingOnboardingView {
    // ... 現有欄位 1~13 不變 ...
    string return_store_name = 14;    // 退貨門市名稱
    string return_store_address = 15; // 退貨門市地址
}
```

> **向後相容**：舊版 Client 未帶／未讀這兩個欄位時視為空字串，server 必須容忍。

---

### **4. Gateway HTTP API 設計**

#### **4.1 買家常用門市 API**

| 方法 | 路徑 | 說明 | 權限 |
|------|------|------|------|
| POST | `/api/v1/app/store_pickups` | 新增常用門市 | BUYER(1) / SELLER_AND_BUYER(3) |
| PUT | `/api/v1/app/store_pickups/{store_pickup_id}` | 修改常用門市 | BUYER(1) / SELLER_AND_BUYER(3) |
| GET | `/api/v1/app/store_pickups` | 取得常用門市列表 | BUYER(1) / SELLER_AND_BUYER(3) |
| DELETE | `/api/v1/app/store_pickups/{store_pickup_id}` | 刪除常用門市 | BUYER(1) / SELLER_AND_BUYER(3) |

##### 4.1.1 新增常用門市

**Request Body：**

```json
{
    "storeCode": "916712",
    "storeName": "敦安門市",
    "storeAddress": "台北市大安區敦化南路一段236巷1號",
    "receiverName": "王小明",
    "receiverPhone": "0912345678",
    "isDefault": true
}
```

**Success Response：**

```json
{
    "code": 0,
    "message": "success",
    "data": {
        "id": "123456789",
        "storeCode": "916712",
        "storeName": "敦安門市",
        "storeAddress": "台北市大安區敦化南路一段236巷1號",
        "receiverName": "王小明",
        "receiverPhone": "0912345678",
        "isDefault": "true",
        "createdAt": "2026-03-11T10:30:00Z",
        "updatedAt": ""
    }
}
```

**錯誤情境：**

| code | 情境 |
|------|------|
| 400 | 必填欄位缺失、門市代碼格式錯誤 |
| 409 | 門市數量已達上限 |

##### 4.1.2 修改常用門市

**Request Body**：與新增相同（全量更新）。

**錯誤情境：**

| code | 情境 |
|------|------|
| 400 | 必填欄位缺失 |
| 404 | 門市記錄不存在或不屬於該買家 |

##### 4.1.3 取得常用門市列表

無 Query 參數。回傳該買家所有常用門市，預設門市排在最前面，其餘依建立時間倒序排列。

**Success Response：**

```json
{
    "code": 0,
    "message": "success",
    "data": [
        {
            "id": "123456789",
            "storeCode": "916712",
            "storeName": "敦安門市",
            "storeAddress": "台北市大安區敦化南路一段236巷1號",
            "receiverName": "王小明",
            "receiverPhone": "0912345678",
            "isDefault": "true",
            "createdAt": "2026-03-11T10:30:00Z",
            "updatedAt": ""
        }
    ]
}
```

##### 4.1.4 刪除常用門市

無 Request Body。刪除成功回傳 `{ "code": 0, "message": "success" }`。

**錯誤情境：**

| code | 情境 |
|------|------|
| 404 | 門市記錄不存在或不屬於該買家 |

#### **4.2 賣家退貨門市（掛在既有建店 API）**

退貨門市的讀寫一律走既有建店 API 的 Request／Response，不另設端點：

##### 4.2.1 超商建店請求（`POST /api/v1/app/shipping_onboardings/cvs`）

`OnboardCvsReq` DTO：

```go
type OnboardCvsReq struct {
    GoodsType          string `json:"goodsType" binding:"required"`
    ReturnStoreCode    string `json:"returnStoreCode" binding:"required"`
    ReturnStoreName    string `json:"returnStoreName"`    // 退貨門市名稱（選填）
    ReturnStoreAddress string `json:"returnStoreAddress"` // 退貨門市地址（選填）
}
```

**完整 Request Body 範例：**

```json
{
    "goodsType": "AMBIENT",
    "returnStoreCode": "916712",
    "returnStoreName": "敦安門市",
    "returnStoreAddress": "台北市大安區敦化南路一段236巷1號"
}
```

> **向後相容**：`returnStoreName` 和 `returnStoreAddress` 為選填。舊版前端不傳這兩個欄位時，後端正常建店，門市名稱與地址保持為空。

##### 4.2.2 建店設定回應（`GET /api/v1/app/shipping_onboardings`）

`ShippingOnboardingViewResp` DTO：

```go
type ShippingOnboardingViewResp struct {
    // ... 現有欄位不變 ...
    ReturnStoreName    string `json:"returnStoreName"`    // 退貨門市名稱
    ReturnStoreAddress string `json:"returnStoreAddress"` // 退貨門市地址
}
```

**回應範例（建店列表中的一筆）：**

```json
{
    "onboardingId": "987654321",
    "shippingType": "CVS_C2C",
    "goodsType": "AMBIENT",
    "onboardingStatus": "ACTIVE",
    "returnStoreCode": "916712",
    "returnStoreName": "敦安門市",
    "returnStoreAddress": "台北市大安區敦化南路一段236巷1號",
    "labelCompanyName": "範例鮮",
    "enabled": "true",
    "createdAt": "2026-03-01T08:00:00Z",
    "updatedAt": "2026-03-11T10:30:00Z"
}
```

##### 4.2.3 賣家修改退貨門市的操作流程

賣家修改退貨門市時，前端重新呼叫 `POST /api/v1/app/shipping_onboardings/cvs`，帶入完整參數（含更新後的退貨門市資訊）。後端判斷該 goods_type 的建店設定已為 `ACTIVE` 狀態，走**本地更新路徑**（不重新送 PayUni 建店 API）。

```
賣家修改退貨門市
    │
    ▼
POST /app/shipping_onboardings/cvs
    { goodsType, returnStoreCode, returnStoreName, returnStoreAddress }
    │
    ▼
OnboardingService.onboardCvs()
    ├─ 查詢現有建店設定
    ├─ onboardingStatus == ACTIVE → 僅更新本地 DB
    │   UPDATE seller_shipment_settings
    │   SET return_store_code = ?, return_store_name = ?, return_store_address = ?
    │   WHERE store_id = ? AND goods_type = ?
    └─ 回傳更新後的 ShippingOnboardingView
```

---

### **5. 業務規則**

#### **5.1 買家常用門市**

| # | 規則 |
|---|------|
| 1 | 單一買家最多儲存 **20** 筆常用門市（上限由 `application.yml` 配置 `hawkr.shipping.store-pickup.max-per-buyer`，預設 20） |
| 2 | 預設門市最多一個。新增/修改時若 `isDefault=true`，自動將該買家其他門市的 `isDefault` 設為 `false` |
| 3 | 刪除預設門市後，不自動指派新預設（買家需手動設定） |
| 4 | 同一買家可以儲存相同門市代碼的多筆記錄（不同收件人/電話情境） |
| 5 | 門市代碼不做 PayUni API 驗證（信任前端從門市地圖取得的資料） |
| 6 | 列表排序：預設門市最前，其餘依 `created_at DESC` |

#### **5.2 賣家退貨門市**

| # | 規則 |
|---|------|
| 1 | 退貨門市**按 goods_type 分開管理**（AMBIENT 和 FROZEN 各自獨立），因為建店設定的粒度是 per goods_type |
| 2 | 退貨門市資訊透過**現有建店 API** 設定/修改，不新增獨立 API |
| 3 | `returnStoreName` 和 `returnStoreAddress` 為**選填**欄位，首次建店時可以只傳 `returnStoreCode`，之後再補填完整資訊 |
| 4 | 修改退貨門市（ACTIVE 狀態重送建店 API）**僅更新本地 DB，不同步 PayUni**（詳見 §6） |
| 5 | 退貨門市**只適用 7-ELEVEN C2C**（`3651`）。同一 `store_id` 不得同時存在 `3651`（C2C）與 `3653`（B2C 大宗寄倉）的 ACTIVE 建店列——此排他性由 `seller_shipment_settings` 的部分唯一索引在**資料層**擋住（見 [68 號](68-7-ELEVEN%20大宗寄倉%EF%BC%88B2C%EF%BC%89詳細設計書.md) §5.4），不是靠 UI 隱藏 |
| 6 | 已開通 B2C 的賣家沒有可設定的退貨門市：B2C 建單不傳 `storeId`／`refundStoreId`，退貨去向由母子廠商代號與指定物流中心決定 |

---

### **6. PayUni 退貨門市同步策略**

> 🔴 **本節只適用 7-ELEVEN C2C（`3002`）。B2C 大宗寄倉（`3004`）不適用**——
> 見 [68 號](68-7-ELEVEN%20大宗寄倉%EF%BC%88B2C%EF%BC%89詳細設計書.md) §6.1.3：
> `storeId`（退貨門市代碼）**C2C 專用、B2C 不得傳**；`refundStoreId` 也**不傳**。
> B2C 的退貨去向由**母子廠商代號與指定物流中心**決定，不是由賣家設定的退貨門市。
>
> 排他性規則見 §5.2 規則 5–6：開通 B2C 後同一 `store_id` 不得再用門市交寄。
> 該規則只影響賣家的可用配送方式，`buyer_store_pickups` 與 `/api/logistics/ship_map` 不受影響。

#### **6.1 PayUni 退貨門市同步語意**

PayUni 超商建單 API（`/api/logistics/trade` v1.3）提供 `RefundStoreID` 選填參數：

| 行為 | 說明 |
|------|------|
| **有傳** `RefundStoreID` | **覆蓋**建店時設定的退貨門市，以本次傳入的為準 |
| **沒傳** `RefundStoreID` | 自動使用建店時設定的退貨門市 |

現有 `CvsShipmentStrategy` 在建單時已從 `seller_shipment_settings.return_store_code` 取值並傳入 `RefundStoreID`：

```java
// CvsShipmentStrategy：建單時帶入 RefundStoreID
onboardingSetting != null ? onboardingSetting.getReturnStoreCode() : null
```

因此，賣家修改退貨門市後，**下一次超商建單會自動帶上最新的退貨門市代碼**，無需額外同步 PayUni。PayUni 側的建店紀錄雖然保留舊的退貨門市代碼，但不影響實際物流行為。

---

### **7. 與現有流程的整合**

#### **7.1 結帳流程**

買家在結帳時選擇超商取貨，可以：
- 從常用門市列表中選取（調用 `ListBuyerStorePickups`）→ 帶入 `storeCode`、`storeName`、`receiverName`、`receiverPhone`
- 透過門市地圖重新選擇（現有 `GetStoreMapParams` 流程不變）

> **注意**：門市管理模組僅負責「儲存常用門市」，結帳時的門市選擇仍由前端決定帶入哪一組資訊到 `CreateOrderRequest`。

> 賣家已開通 7-ELEVEN 大宗寄倉（B2C，`3653`）時，該賣家不得再用門市交寄（§5.2 規則 5），
> 其可選配送方式不含 C2C 超商取貨。買家的常用門市清單跨賣家共用，不因此改變。

#### **7.2 超商建單流程**

`CvsShipmentStrategy` 建單時從 `seller_shipment_settings` 取得退貨門市代碼，傳入 PayUni `RefundStoreID`。`return_store_name` 與 `return_store_address` 不進建單請求（PayUni 只需代碼），僅供前端顯示賣家的退貨門市資訊。

#### **7.3 建店流程**

`OnboardCvsShipping` RPC 一併收退貨門市完整資訊，各時機行為如下：

| 時機 | 行為 |
|------|------|
| **首次建店** | 前端傳入 `returnStoreCode` + `returnStoreName` + `returnStoreAddress` → 送 PayUni 建店 API（僅用 code）+ 本地儲存全部 |
| **修改退貨門市** | 前端重送建店 API（ACTIVE 狀態）→ 僅更新本地 DB 的 3 個欄位 |
| **查詢建店設定** | 現有 `ListShippingOnboardings` / `GetShippingOnboarding` 回應自動包含退貨門市名稱與地址 |

---

### **8. Java 實作落點**

#### **8.1 Entity / Mapper**

| 類別 | 職責 |
|------|------|
| `BuyerStorePickupEntity` | 對應 `buyer_store_pickups` 表，`@TableId(type = IdType.INPUT)` |
| `BuyerStorePickupMapper` | 繼承 `BaseMapper<BuyerStorePickupEntity>` |
| `SellerShipmentSettingEntity` | 含 `returnStoreName`、`returnStoreAddress` 欄位 |

#### **8.2 Service**

| 類別 | 職責 |
|------|------|
| `BuyerStorePickupService` | 買家門市 CRUD + 預設邏輯 + 數量上限檢查 |
| `OnboardingService.onboardCvs()` | ACTIVE 狀態本地更新時，一併寫入 `return_store_name` + `return_store_address` |

#### **8.3 GrpcService**

`ShippingGrpcServiceImpl` 承載的門市相關 RPC 與權限：

| RPC | 權限 |
|-----|------|
| `AddBuyerStorePickup` | `requireAccountType(BUYER, SELLER_AND_BUYER)` |
| `UpdateBuyerStorePickup` | `requireAccountType(BUYER, SELLER_AND_BUYER)` |
| `ListBuyerStorePickups` | `requireAccountType(BUYER, SELLER_AND_BUYER)` |
| `DeleteBuyerStorePickup` | `requireAccountType(BUYER, SELLER_AND_BUYER)` |
| `OnboardCvsShipping` | 賣家權限不變，請求含退貨門市三欄 |
| `ListShippingOnboardings` / `GetShippingOnboarding` | 賣家權限不變，回應含退貨門市三欄 |

#### **8.6 配置參數**

```yaml
# application.yml
hawkr:
  shipping:
    store-pickup:
      max-per-buyer: ${STORE_PICKUP_MAX_PER_BUYER:20}
```

---

### **9. 資料表 DDL**

#### **9.1 buyer_store_pickups**

```sql
CREATE TABLE IF NOT EXISTS buyer_store_pickups (
    id                BIGINT PRIMARY KEY,
    buyer_id          BIGINT NOT NULL,
    store_code        VARCHAR(20) NOT NULL,
    store_name        VARCHAR(100) NOT NULL,
    store_address     VARCHAR(200) NOT NULL,
    receiver_name     VARCHAR(50) NOT NULL,
    receiver_phone    VARCHAR(20) NOT NULL,
    is_default        BOOLEAN NOT NULL DEFAULT FALSE,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_bsp_buyer ON buyer_store_pickups(buyer_id);
CREATE INDEX IF NOT EXISTS idx_bsp_buyer_default ON buyer_store_pickups(buyer_id)
    WHERE is_default = TRUE;

DROP TRIGGER IF EXISTS update_buyer_store_pickups_updated_at ON buyer_store_pickups;
CREATE TRIGGER update_buyer_store_pickups_updated_at
    BEFORE UPDATE ON buyer_store_pickups
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE buyer_store_pickups IS '買家常用門市表';
COMMENT ON COLUMN buyer_store_pickups.id IS 'TSID 主鍵';
COMMENT ON COLUMN buyer_store_pickups.store_code IS '門市代碼（PayUni 門市地圖回傳，6 碼）';
```

#### **9.2 seller_shipment_settings 的退貨門市欄位**

```sql
    return_store_code       VARCHAR(20),
    return_store_name       VARCHAR(100),
    return_store_address    VARCHAR(200),
```

> schema 變更一律走 Flyway `V{N+1}`，不得修改已套用的 migration（見
> [DB Migration 相容性規範](../開發規範/DB%20Migration%20相容性規範（expand-contract）.md)）。

---

### **10. Gateway 實作落點**

#### **10.1 買家門市 Handler**

買家門市 CRUD 的 4 個 Handler 住在 `gateway/internal/handler/store_pickup.go`（與 `shipping.go` 分開）。

🔴 **四個 handler 一律以 `resolveBuyerIDFor` 把 JWT `account_id` 解析成現任 `buyer_id`**
再填入 gRPC request 的 `buyer_id`，禁止直傳 raw `account_id`——直傳會讓軟刪除復活後的買家
讀到前一個 stint 的門市資料。

#### **10.2 建店 Handler**

| 檔案 | 責任 |
|------|------|
| `shipping.go` — `OnboardCvsReq` | 帶 `ReturnStoreName`、`ReturnStoreAddress` 欄位 |
| `shipping.go` — `ShippingOnboardingViewResp` | 回 `ReturnStoreName`、`ReturnStoreAddress` 欄位 |
| `shipping.go` — `OnboardCvsShipping()` | 把退貨門市三欄傳到 gRPC Request |
| `shipping.go` — `convertOnboardingView()` | 把退貨門市三欄映射到 Response DTO |

---

### **11. 附錄**

#### **11.1 相關文件**

- [物流微服務 (Shipping Service) 詳細設計書](06-物流微服務（Shipping%20Service）詳細設計書.md)
- [API 設計規範](../開發規範/API%20設計規範.md)
