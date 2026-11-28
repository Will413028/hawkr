# **API 設計規範**

> **適用範圍（2026-09-26 起）**：對外契約改為 Protobuf＋Connect（ADR `docs/adr/2026-09-26-external-contract-protobuf-connect.md`），新的對外 API 一律依〈API 設計規範（Connect）〉。本規範只適用舊 gateway 的 REST（`/api/v1`），模組搬進單體時隨它的路由退場。§3.1、§14.1 的原則已寫進〈API 設計規範（Connect）〉；§2.4、§4 對 Connect 仍然適用到 identity 搬進單體為止。

## **一、設計原則**

本專案 API 設計採用 **Pragmatic REST（工程化 REST）**，目標為：

* **明確安全域（Security Domain）**  
* **清晰資源語意（Resource-first）**  
* **嚴格授權邊界（Gateway 與 Service 分工）**  
* **穩定契約（不可破壞既有回應結構）**  
* **不過度設計，但保留未來升級空間**


## **二、整體架構模型**

### **2.1 標準服務模型**

**Client** → **Gateway**（REST 對外入口）→ **gRPC Services**（內部微服務）

### **2.2 例外服務模型（HTTP 原生端點）**

以下類型允許由微服務提供 HTTP Controller：

* 多媒體串流（Streaming）  
* 檔案下載（Binary）  
* Webhook / Callback  
* 必須支援 Range / 206 的端點

**Client** → **Gateway**（或指定路由）→ **HTTP Controller Service**（例如 Media Service）

#### **2.2.1 架構圖**

程式碼片段

```mermaid
graph TD  
    %% 定義樣式  
    classDef public fill:\#e1f5fe,stroke:\#01579b,stroke-width:2px;  
    classDef secure fill:\#e8f5e9,stroke:\#2e7d32,stroke-width:2px;  
    classDef exception fill:\#fff3e0,stroke:\#ef6c00,stroke-width:2px;  
    classDef gateway fill:\#f3e5f5,stroke:\#7b1fa2,stroke-width:2px;

    Client((Client App/Web)):::public

    subgraph "DMZ / Public Zone"  
        Gateway\["API Gateway\<br/\>(AuthN, Role Check, Envelope)"\]:::gateway  
    end

    subgraph "Private Network (VPC)"  
        direction TB  
          
        %% 標準路徑  
        AuthService\[Auth Service\<br/\>(gRPC)\]:::secure  
        OrderService\[Order Service\<br/\>(gRPC)\]:::secure  
        ProductService\[Product Service\<br/\>(gRPC)\]:::secure  
          
        %% 例外路徑  
        MediaService\[Media/File Service\<br/\>(HTTP Controller)\]:::exception  
    end

    %% 連線關係  
    Client \-- "REST / JSON\<br/\>(Standard)" \--\> Gateway  
    Client \-- "Streaming / Binary\<br/\>(Exception)" \--\> Gateway  
      
    Gateway \-- "gRPC (Metadata: account\_id, role)" \--\> AuthService  
    Gateway \-- "gRPC" \--\> OrderService  
    Gateway \-- "gRPC" \--\> ProductService  
      
    Gateway \-- "HTTP Stream / Proxy" \--\> MediaService

    %% 註解  
    note1\[Phase 1 Security:\<br/\>Network Isolation\<br/\>Security Groups\]  
      
    style note1 fill:\#fff,stroke:\#333,stroke-dasharray: 5 5
```

### **2.3 責任分配**

* **Gateway（強制）**  
  * 驗證 JWT（Authentication）  
  * 域級角色門檻檢查  
  * 傳遞身份至 gRPC（metadata）  
  * 將 gRPC 錯誤轉換為 REST Envelope  
* **gRPC Service（強制）**  
  * 不得假設呼叫一定來自 Gateway  
  * 必須做方法級角色門檻檢查  
  * 僅處理業務邏輯  
* **HTTP 原生 Controller（例外服務）**
  * 必須遵守 Namespace 規範
  * 必須做角色門檻或 service identity 驗證
  * JSON 類型回應必須使用統一 Envelope
  * Streaming / Binary 可例外

### **2.4 Gateway 職責邊界（強制）**

Gateway 定位為**薄代理層（Thin Proxy）**，僅負責協議轉換與基礎設施橫切關注點，**禁止承載業務邏輯**。

**允許的職責：**

| 職責 | 說明 |
|------|------|
| HTTP ↔ gRPC 協議轉換 | 接收 HTTP 請求，呼叫對應 gRPC RPC，回傳結果 |
| JWT 認證 + Header 注入 | 驗證 Token、注入 `x-account-id` / `x-account-type` |
| 角色門檻 | `requireSeller` / `requireAdmin` 等域級檢查 |
| CORS / Rate Limit / Tracing | 基礎設施橫切關注點 |
| Request / Response 欄位映射 | JSON ↔ Proto 的簡單欄位對應（不含業務判斷） |
| 靜態 HTML 渲染 | **僅限**無業務判斷的固定模板（如純 Form POST 轉發） |

**禁止的行為：**

| 禁止行為 | 正確做法 |
|---------|---------|
| URL / 連結建構（return URL、redirect URL、CDN URL） | 微服務組裝完整 URL，Gateway 透傳 |
| Deep Link 建構（決定 URL Scheme 或組裝 Deep Link） | 微服務根據角色建構完整 Deep Link |
| 業務狀態映射（gRPC 狀態碼 → 業務語意文字） | 微服務在 gRPC Response 回傳最終語意 |
| 條件式 RPC 分流（依角色 / 狀態呼叫不同 RPC） | 定義獨立 RPC（§4 角色門檻），或微服務內部處理 |
| 含業務判斷的 HTML 渲染（角色判斷、URL Scheme 選擇） | 微服務回傳完整 redirectUrl 或 HTML 片段，Gateway 僅嵌入 |
| 業務預設值填充（為缺失欄位填入業務預設值） | 微服務決定業務預設值 |
| 業務錯誤碼轉換（gRPC error → 特定業務錯誤碼） | 微服務直接在 gRPC Status 回傳最終錯誤碼 |

**外部回調（Webhook / Return URL）標準模式：**

```
外部服務 → Gateway（HTTP 接收）→ 微服務（gRPC：解密 + 業務處理 + 組裝最終結果）
                                   ↓
                             Gateway 透傳結果（JSON / redirectUrl / HTML）
```

禁止 Gateway 自行解密、組裝業務回應、或決定導向目標。

**判斷標準：** 不確定時，問三個問題：
1. 這段邏輯是否需要理解業務語意？ → 需要 → 放微服務
2. 換一個完全不同的業務場景，這段邏輯是否仍然適用？ → 不適用 → 放微服務
3. 如果業務規則變更，是否需要改 Gateway？ → 需要 → 放微服務


## **三、Security Domain（Namespace）**

所有對外 API 僅允許以下三個 Namespace：

* /api/v1/public/\*  
* /api/v1/app/\*  
* /api/v1/admin/\*

### **3.1 public**

* 可匿名  
* 不得回傳敏感資訊

「敏感資訊」在 public namespace 的可執行定義 —— **deny-by-default 白名單**：

* public DTO 一律**逐欄明列**，禁止把 app／admin 的 DTO 直接拿來共用，也禁止用 struct copy／反射把上游 model 整包吐出去。漏一欄只會少顯示東西；共用一個型別會多洩漏東西。
* 加一個欄位到 public DTO 之前，先回答：**「這是這個店家／這件商品的營業資訊，還是某個人的個人資料？」** 後者一律不進 public。典型不得公開者：email、手機／電話（含以「聯絡電話」為名但實際填入本人已驗證手機者）、生日、性別、街道級地址、身分證／銀行帳號、帳號與治理內部狀態（accountType、認證流程記錄 id、停權原因碼）。
* 地址類的公開精度**止於行政區**（city + district）。街道級地址的用途是寄退件與物流開通，不是店面展示；個人賣家填的常常就是住家。
* public DTO 必須有契約測試斷言「PII 欄名不在回應中」＋「欄位集合與白名單一致」（範例：`gateway/internal/handler/seller_public_pii_test.go`）。**只擋已知欄名不夠**——欄位集合斷言才擋得住日後上游長胖時靜默流出的新欄位。

教訓：`GET /api/v1/public/sellers/{id}` 曾與 `GET /api/v1/app/sellers/me` 共用同一個 `SellerInfo` DTO，於是網頁每一個店面頁背後都掛著店主的 email、已驗證手機、生日、性別與寄退件街道地址，匿名帶一個 store\_id 即可取得。規則（本節「不得回傳敏感資訊」＋第十節「每個 endpoint 必須對應唯一 DTO」）當時都已存在，缺的是可執行的判準與機械保證。

### **3.2 app**

* 必須登入  
* 允許角色：BUYER / SELLER

### **3.3 admin**

* 必須 ADMIN

### **3.4 角色路徑規範**

#### **禁止事項**

* 禁止將角色作為 **Namespace 層級** 路徑：`/api/v1/buyer/*`、`/api/v1/seller/*`、`/api/v1/merchant/*`
* 禁止 RPC 風格 /getXxx、/doXxx、/createXxx

#### **角色子路徑（允許）**

當同一資源需要區分買家與賣家視角時，**使用 `/seller/` 子路徑分離**，確保每條路由 1:1 對應一個 gRPC RPC：

```
/api/v1/app/after_sales              ← 買家視角（預設）
/api/v1/app/seller/after_sales       ← 賣家視角
```

**規則**：

| 規則 | 說明 |
|------|------|
| 買家為預設視角 | `/api/v1/app/{resource}` 不帶前綴，預設為買家操作 |
| 賣家使用 `/seller/` 前綴 | `/api/v1/app/seller/{resource}` 明確標示賣家操作 |
| 每條路由對應一個 RPC | Gateway 不做 if/else 角色分流，每個 handler 1:1 對應一個 gRPC RPC |
| Middleware 分組掛載 | 賣家路由組統一掛 `requireSeller()`，買家路由組掛 `JWTAuth()` |

**範例**：

```
// 買家
GET  /api/v1/app/after_sales                      → GetMyAfterSaleCases
POST /api/v1/app/after_sales                      → CreateAfterSaleCase
POST /api/v1/app/after_sales/:case_no/cancel      → CancelAfterSaleCase

// 賣家
GET  /api/v1/app/seller/after_sales               → GetSellerAfterSaleCases
POST /api/v1/app/seller/after_sales/:case_no/resolve → ResolveAfterSaleCase
```

> **歷史相容**：Order Service 使用 `?view=seller` 模式切換視角（早期設計），維持現狀不改。新服務一律採用路徑分離模式。



## **四、角色模型（Phase 1）**

JWT 必須包含：account\_id, account\_type（以 code 值傳遞）

### **帳戶型態 Code 值定義**

| Code | 名稱 | 說明 |
| ----- | ----- | ----- |
| 1 | BUYER | 買家 |
| 2 | SELLER | 賣家 |
| 3 | SELLER\_AND\_BUYER | 同時擁有買家與賣家權限 |
| 4 | ADMIN | 管理員 |

### **權限涵蓋規則**

SELLER\_AND\_BUYER (code=3) 同時涵蓋 BUYER 與 SELLER 權限：

* API 限制 BUYER (code=1) → code 1 與 code 3 皆可通過
* API 限制 SELLER (code=2) → code 2 與 code 3 皆可通過
* API 限制 ADMIN (code=4) → 僅 code 4 可通過

### **4.1 Gateway 域級門檻**

| Namespace | 允許 Code |
| ----- | ----- |
| public | (匿名 / 全部) |
| app | 1 (BUYER) / 2 (SELLER) / 3 (SELLER\_AND\_BUYER) |
| admin | 4 (ADMIN) |

### **4.2 gRPC 方法級門檻**

* 每個 gRPC method 必須標註 Allowed Account Type。
* 帳戶型態不符必須拒絕。
* 驗證方法使用 Account Type Code，並遵守上述權限涵蓋規則。
* Gateway 與微服務的 x-account-type header 一律傳遞 code 值（整數字串）。

## **五、內部呼叫與 SERVICE\_ONLY 規範**

內部呼叫分為兩類：

### **5.1 USER\_SCOPED（代表使用者）**

* 攜帶 account\_id, account\_type
* 只服務於內部服務 SERVICE 時不需要再驗證權限（已在 Caller 被驗證過）
* 視為使用者操作延伸

### **5.2 SERVICE\_ONLY（純系統內部）— Phase 1 定稿規則**

#### **5.2.1 Phase 1 安全前提（強制）**

目前所有微服務均部署於內網，且必須由網路層保證：

* gRPC 端口不可被外部網路存取  
* 服務間連線必須限制為最小可達性（不可 any-to-any）  
* 非必要服務不得直接存取其他服務的 gRPC 端口  
* 需由 Security Group / NetworkPolicy / VPC 規則實作限制

~~**在上述前提成立時，SERVICE\_ONLY 類型方法可不實作 service identity 與 allowlist。**~~

> ⚠️ **此豁免已不再適用（#1366，2026-07 執行）**：明文 `x-internal-service` 可被同 namespace 任意 pod 偽造，「網路層保證最小可達性」在 flip 前並不成立（`deny-cross-namespace` 以 `podSelector: {}` 放行全 ns）。service identity 已改由 **HMAC 簽章**強制實作、可達性由 **per-service NetworkPolicy** 收斂。詳見 **詳細設計書 52 號《微服務間內部授權》**。

#### **5.2.2 風險說明**

若出現以下任一情境，將產生內部端點被繞過與橫向移動風險：

* 微服務不再保證位於同一內網  
* 出現跨 VPC / 跨網段 / 混合雲部署  
* 出現外部直連微服務（非透過 Gateway）  
* 服務間 direct call 且無最小可達性限制  
* 第三方系統直接存取內部端點

#### **5.2.3 觸發升級條件（✅ 已於 #1366 執行，見詳細設計書 52 號）**

原列升級條件已由 #1366 全面實作（不再是「若…則」的待辦，而是現行機制）：

1. **Service Identity**：✅ 以 **HMAC-SHA256 簽章**（單一共享密鑰 `hawkr-internal-auth`、canonical 綁 accountId、時間窗防重放）取代 mTLS / internal token，兩端 server interceptor 驗簽（52 號 §3/§5）。
2. **Allowed Callers allowlist**：✅ 以 **per-service NetworkPolicy ingress 白名單** 收斂網路可達性，移除 allow-all-same-ns（52 號 §7）。
3. **高風險方法**：退款（`CreateRefund` 已 internal-only）、金流回寫等由 HMAC + netpol 雙軌守門；歸屬魔數哨兵（`storeId=0`）改為明確內部 RPC（52 號 §8）。



## **六、身份傳遞規範**

### **6.1 傳遞範圍**

* Gateway → HTTP 原生 Controller（例外服務）  
* Gateway → gRPC（內部呼叫）

### **6.2 傳遞方式**
Gateway → gRPC 必須透過 metadata 傳遞：

* X-Account-Id
* X-Account-Type 使用者分類（1/2/3/4）
* X-Trace-Id
* X-Request-Id

### **6.3 補充說明**
1. gRPC 僅信任內部來源。
2. 第一階段上線授權判斷僅以 x-account-type 作為方法級門檻依據；x-role 僅保留未來細粒度權限（RBAC）擴充使用，現階段不得單獨作為授權依據。


## **七、Resource 命名規範**

### **7.1 複數名詞**

* /products
* /orders
* /sellers
* /coupons

### **7.2 ID 表達**

* /products/{product\_id}  
* 規範：snake\_case、小寫

### **7.3 巢狀資源**

* /products/{product\_id}/reviews  
* /sellers/{seller\_id}/products
* 規範：僅表示從屬關係。



## **八、HTTP Method 規範**

| Method | 用途 |
| ----- | ----- |
| **GET** | 查詢 |
| **POST** | 建立 / 業務動作 |
| **PUT** | 全量更新 |
| **PATCH** | 部分更新 |
| **DELETE** | 刪除（需標示是否軟刪） |



## **九、Action Endpoint**

格式：POST /{resource}/{id}/{action}  
範例：

* POST /api/v1/app/orders/{order\_id}/cancel  
* POST /api/v1/admin/products/{product\_id}/approve



## **十、View / Projection**

* GET /api/v1/public/products/{product\_id}  
* GET /api/v1/app/products/{product\_id}/edit  
* GET /api/v1/admin/products/{product\_id}

每個 endpoint 必須對應唯一 DTO。**不同 Namespace 的同一資源尤其要分開**：public 與 app 各自一個型別，禁止「公開端點直接回自我讀取的 DTO」——那條捷徑的代價是店主個人資料隨每一次公開查詢外流（見 §3.1 教訓）。



## **十一、Query 規範**

範例：  
?page=1\&page\_size=20\&sort=created\_at\_desc\&q=keyword

* **命名**：snake\_case、小寫。  
* **時間格式**：一律使用 ISO 8601 (UTC)。  
  * 格式規範：YYYY-MM-DDTHH:mm:ssZ  
  * 禁止使用 Timestamp 或其他格式。



## **十二、統一回應格式（Envelope）**

**成功：通用**

``` JSON
{  
  "code": 0,  
  "message": "success",  
  "data": {}  
}
```

**成功：具有分頁屬性**

``` JSON
{  
  "code": 0,  
  "message": "success",  
  "data": {  
      "records": \[...\],  
      "total": "100",  
      "page": "1",  
      "pageSize": "20"  
  }  
}
``` 

**註：** page 從 1 開始，pageSize 最大 500；total, page, pageSize 皆為 **整數字串**。  
**錯誤**

``` JSON
{  
  "code": 404,  
  "message": "product not found",  
  "traceId": "a1b2c3d4e5"  
}
``` 

**註：** 為符合 JSON camelCase 規範，此處使用 traceId。

* traceId：錯誤時強制回傳，便於前端報修 
* 不得新增額外欄位

### message 來源策略（強制）

| 情境 | message 來源 | 說明 |
|------|-------------|------|
| 成功（code=0） | 固定 `"success"` | Gateway **禁止**自訂成功訊息（如「商品狀態更新成功」），一律使用 `"success"` |
| 錯誤（code≠0） | 透傳 Service 端的 `resp.Message` | Gateway **禁止**覆寫或自訂錯誤訊息，由微服務統一管理錯誤文案 |

**原則**：Gateway 僅負責 HTTP 轉發與格式轉換，不負責業務文案。訊息文案的唯一來源是微服務端。

**機械保證**：成功回應只能經由 `response.OK` / `response.OKProto` / `response.Page` / `response.GrpcResult`（code=0 分支）送出，四者的 `message` 都由 `pkg/envelope` 固定為 `"success"`——**不存在**「把下游成功訊息透傳到 `message`」的 helper（曾有 `response.OKMsg`，但它的 message 參數在實作上一直是被丟棄的，2026-08-22 已刪除）。要傳語意（`approved` / `rejected` / 收件人數之類）就在 `data` 開一個具名欄位，不要塞進 `message`——那是字串，前端得剖析它才拿得到值。`c.JSON` 等繞道由 ruleguard `banDirectJSON` 擋住。



## **十三、Streaming / Binary 例外**

當 Content-Type 非 JSON 時：

* 不使用 Envelope  
* 允許 video/\*、audio/\*、binary、206 Partial Content  
* 錯誤時（如 404, 500）必須回傳標準 JSON Envelope



## **十四、gRPC 錯誤轉換**

* 使用正確 gRPC status code  
* Gateway 必須轉換為 REST Envelope  
* 不得暴露內部 stack trace



### **14.1 越權回應策略（防枚舉 anti-enumeration，強制）**

當**已通過身分驗證**的使用者，存取一個**存在但不屬於自己**的資源（ownership mismatch）時，回應碼依資源敏感度決定：

| 類別 | 資源性質 | 回應 | 理由 |
|------|---------|------|------|
| **A（敏感私密資源）** | 訂單、付款 / 退款交易、商品刪除、優惠券模板、媒體匯入 / 機敏資產、IM 對話 | **404 NOT\_FOUND**，且回應（code + message）必須與「資源不存在」**完全一致**、不可區分 | 防止以 403 vs 404 差異枚舉探測資源是否存在 |
| **C（公開或低敏感資源）** | 社群貼文 / 留言（貼文 ID 公開可見）、送貨地址、賣家違規申訴、評論提交 | **403 PERMISSION\_DENIED** | 資源存在性非敏感、或 ID 本身公開可見，回 404 反而誤導 |

**Tier A 強制清單**（越權一律回 404，回應與 not-found 一致）：

| 服務 | RPC / 端點 | 越權判斷 |
|------|-----------|---------|
| order | `GetOrder` / `GetSellerOrderDetail`（讀）<br>`CancelOrder` / `ConfirmReceipt`（buyer 寫）/ `ShipOrder`（seller 寫） | buyer / seller ownership mismatch — 一律 `throw OrderNotFoundException(orderNo)`，與 not-found 同例外路徑 |
| payment | `GetPayment` / `SyncPaymentStatus` / `ClosePayment` / `CreateRefund` / `GetRefund` / `GetPaymentHistory` | `verifyPaymentOwnership(ByOrderId)`；`GetRefund` 的「退款不存在」分支亦須與越權同回 `NOT_FOUND/"交易不存在"`，否則可用 `refund_no` 枚舉 |
| product | `DeleteProduct`（`GetProduct` / `UpdateSellerProductStatus` / `UpdateProduct` 已為 404） | `createdBy` mismatch |
| coupon | `SellerGetTemplateDetail` | `issuerAccountId` mismatch |
| media | 匯入 `bindStagedAsset` / `FinalizeImages` 轉正 / 機敏資產 secure_download | `account_id` mismatch |
| im | `GetHistory` / `MarkRead`（REST） | 非房間參與者 |

**實作要點**：
- 越權分支必須回傳與該資源「不存在」**相同的 code 與 message**（最穩健做法：直接走 not-found 例外路徑），**禁止**使用帶 ownership 線索的訊息（如「無權操作」「不屬於該帳號 token\_account\_id=...」）。
- **寫入型 RPC 同樣適用**：越權的 mutation（取消 / 確認收貨 / 出貨）不得回「無權操作」「狀態不允許」等可區分訊息，必須與 not-found 一致（否則 mutation 端點同樣是枚舉 oracle）。
- 純認證 / 角色門檻失敗（未登入、`account_type` 不符）仍回 401 / 403，**不適用**本策略。
- **能力憑證（bearer token）型路徑不在本策略約束**：media `bindStagedAsset` / `finalize` 的資源 id 內嵌於服務簽發的 staged_token（不可偽造、不可由外部枚舉），故「token 無效 / 過期」回 400 `INVALID_ARGUMENT`、帳號不匹配回 404 即可，無需與 not-found 完全一致（無枚舉面）。

**例外（Tier B，現況保留，待後續統一）**：aftersales（`AFTERSALE_NOT_OWNER` 9010）、shipping（`SHIPMENT_PERMISSION_DENIED` 5005）目前以專屬業務碼回應越權，仍可被與「不存在」區分；本期不變動，若日後納入統一須改走對應的 NOT\_FOUND 語意碼（經 CodeMeta）。

## **十五、版本與契約穩定性**

* 不得在同版本修改 DTO 結構  
* 可新增欄位（視為向下相容）  
* 不得刪除或改變欄位型別  
* 破壞性變更必須升級 API 版本（如 v2）



## **十六、Swagger 規範**

* 每個 endpoint 必須定義 Request / Response Model  
* Model 名稱符合 DTO 命名規範  
* 不得使用匿名 object  
* 時間欄位（Input/Output）：一律使用 ISO 8601 (UTC)，格式 YYYY-MM-DDTHH:mm:ssZ。



## **十七、AI 開發強制條款**

1. 僅使用三種 namespace (public, app, admin)  
2. 不得使用角色作為路徑切分  
3. Action 必須使用 POST /{id}/{action}  
4. 每個 endpoint 必須標註 Allowed Account Type  
5. **URL Parameter & Query** 使用 snake\_case  
6. **JSON Body** 使用 camelCase  
7. 使用統一 Envelope  
8. Streaming 僅在例外條款下允許



## **十八、回應快取政策（Cache-Control）**

未列入下表的端點不寫 `Cache-Control`，沿用 HTTP 預設的啟發式快取。下列兩類必須明確標註：

| 類別 | 值 | 適用 | 現況實作 |
|------|-----|------|---------|
| **個人化／機敏回應** | `private, no-store` | 回應內容綁定登入者身分：AI 助手與 Hawkr 幫找採購（對話內容、候選廠商、報價金額）、報價單邀請確認、**認證端點（`/public/auth/*`、`/app/auth/*`）**——含 200 的身分回應與 401 的「無身分」回應 | `response.CacheControlPrivateNoStore`（Gateway 唯一來源），由 `middleware.PrivateNoStore()` 掛在路由群組上 |
| **刻意可快取的公開／條件式回應** | `public, max-age=N` 或 `private, max-age=N` + ETag | 公開統計、參考資料等可共享或可條件請求的內容 | 賣家城市統計（`public, max-age=300, stale-while-revalidate=3600`）、pricebook 空間面（`private, max-age=N` + ETag/304） |

強制規則：

* **`no-store` 不是 `no-cache`**：`no-cache` 允許快取寫入、只要求重新驗證；個人化回應要的是「連寫入都不允許」。
* **中介層必須排在認證／能力門檻之前**：`JWTAuth` 與 capability gate 以 abort 早退，排在其後的中介層不會執行，401／403 就會漏掉 header。
* **自己寫 SSE header 的 handler 必須引用同一個常數**：gin 的 `c.Header` 是 `Set` 語意，handler 後寫的值會覆蓋群組中介層，寫字面值等於埋一個會靜默漂移的第二來源。
* **免認證但機敏的單條路由逐條掛，不要掛整個 `public` 群組**：`public` 底下多數路由是刻意可快取的，整組覆蓋是效能回歸。
* **中介層的涵蓋範圍到 group 為止**：engine 全域鏈（`gin.Recovery()`、`CORS()`、分級限流）排在 group 之前，它們產生的 panic 500／preflight 204／429 不會帶到 header。這三者的 body 不含使用者內容，屬已知且可接受的例外；不要為了補這三個把中介層搬到全域——那會覆蓋刻意可快取的端點。

### 現況缺口（本節上線時尚未處理）

本節於 2026-08-21 隨 issue #2447 BE-01A 建立，當次只把 **AI 助手與 Hawkr 幫找採購**這一組收斂到 `private, no-store`；2026-09-05 隨 #1684 補上**認證端點**。**仍未做全 repo 掃描**，以下是已知仍不符本節第一列的位置，逐項處理前不得宣稱本節已全面落實：

| 位置 | 現況 | 說明 |
|------|------|------|
| `gateway/internal/sse/stream.go` | `no-cache` | 留言／通知 SSE hub。通知 topic 綁收件匣身分，屬個人化回應 |
| `pkg/csvutil/csv.go` | `no-cache` | 共用套件；匯出檔內容依呼叫端而定 |
| 其餘 cookie 認證的個人化 GET | 未設 | 未逐一盤點；auth 群組已收斂，其餘（訂單、收件匣、賣家後台等）仍未盤 |

## **終章**

本規範確保：

* 內網部署下不過度設計  
* 未來架構變更時可強制升級安全模型  
* Gateway 與 gRPC 邊界清晰  
* Streaming 合理例外  
* 契約穩定、AI 可遵循

**所有新 API 必須符合本規範。**  
**違反者應於 Code Review 階段退回修正。**
