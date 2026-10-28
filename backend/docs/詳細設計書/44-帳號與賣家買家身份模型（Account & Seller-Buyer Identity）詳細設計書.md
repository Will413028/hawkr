# 帳號與賣家買家身份模型（Account & Seller-Buyer Identity）詳細設計書

> 本文涵蓋兩塊還在運作的內容：（1）賣家身份的資料模型（stint、`store_id`、`store_code`、`ResolvePrincipals`），實作在單體的 seller 模組（`internal/modules/seller`）；seller 模組沒有自己的規格檔，本文在規格檔建立前是它的身份規格。（2）gateway 的買家與店家身份解析，以及跨角色自我操作守衛（im、social、cart、coupon、order）。
>
> 單體的帳號與買家不在本文：identity 見 `docs/modules/identity.md`，buyer 見 `docs/modules/buyer.md`，刪帳流程見 `docs/modules/accountdeletion.md`。seller 模組的業務行為（profile、KYC、PayUni、治理、關店與清理）以 `internal/modules/seller/AGENTS.md` 為準。
>
> 刪除時機：賣家身份的部分在 seller 規格檔建立後搬進去；gateway 的身份解析與守衛服務搬進單體時，其餘刪除。
>
> 登入憑證機制（OAuth token 驗證、web cookie session、JWT、VerifyPhone、LinkConfirm、token refresh）不在本文，見 [17 號](17-登入與帳號建立流程（Login%20&%20Account）詳細設計書.md)。

---

## 0. 文件元資訊 (Metadata)

| 項目 | 值 |
|---|---|
| 狀態 | `active` |
| 維護者 | Will |
| 對應程式碼 | `internal/modules/seller`（stint、`store_id`／`store_code`）、`internal/legacy/sellerserver`（`ResolvePrincipals` 等 legacy 讀取）、`gateway/internal/{middleware,buyerid,storeid}`（還沒搬的路徑的身份解析）、跨角色守衛 `services/im-service`、`services/social-service`、`services/java/{cart,coupon,order}-service` |
| 相關 doc | [17 登入與帳號建立流程](17-登入與帳號建立流程（Login%20&%20Account）詳細設計書.md)（登入憑證機制）、[18 賣家實名認證](18-賣家實名認證（Merchant%20Certification）詳細設計書.md)（PayUni MerID 綁定源頭）、[32 商家治理](32-商家治理與食安通報（Seller%20Governance%20&%20Food%20Safety）詳細設計書.md)（違規停權）、[36 MQ 與事件總覽](36-MQ%20與事件總覽（MQ%20&%20Event%20Catalog）.md)；單體模組規格 `docs/modules/{identity,buyer,accountdeletion}.md`、`internal/modules/seller/AGENTS.md` |

---

## 1. 概述與目標 (Overview & Goals)

### 1.1 文件定位

本文描述**賣家身份的資料模型與生命週期**，以及依賴它的 gateway 身份解析與跨角色守衛：賣家身份（`store_id`）如何建立、關閉與重新入駐，以及在「同一個人同時是買家與賣家」時如何避免自己對自己下單／發券／私訊。目標讀者為後端全體。**不涵蓋**登入憑證機制（見 17）、實名認證細節（見 18）、單體 identity／buyer／accountdeletion 的行為（見上列模組規格）、seller 模組其餘業務行為（見 `internal/modules/seller/AGENTS.md`）。

### 1.2 一句話定位

把「登入身份 `account_id`」與賣家身份 `store_id` 在資料層分離，使賣家關店後重新入駐能取得全新的 PayUni 商店身份而不撞代理商一次性綁定上限。

### 1.3 設計目標 (Goals) 與 非目標 (Non-Goals)

| 類型 | 項目 |
|---|---|
| **Goal** | 賣家重新入駐 = 新增一個 stint，取得全新 `store_id` 與 `store_code`（新 PayUni MerID），逃離 PayUni 代理商 `AGENT04017` 一次性 MerID 綁定上限；舊 stint 保留供以 `store_id` 關聯的歷史 |
| **Goal** | 單人雙角色下，「不能對自己下單 / 發券 / 加購 / 私訊 / 封鎖」守衛仍正確 |
| **Non-Goal** | **不**引入 `store` 實體（一人多店 / 店鋪轉讓 / 多人共管）——見 [ADR 2026-06-29](../adr/2026-06-29-identity-defer-store-entity.md) |

### 1.4 與其他設計書／服務的關係

- **上游**：[17 登入流程](17-登入與帳號建立流程（Login%20&%20Account）詳細設計書.md) 負責憑證驗證與 session；本文承接其產出的 `account_id` 與角色（SELLER）。
- **下游**：[18 賣家實名認證](18-賣家實名認證（Merchant%20Certification）詳細設計書.md) 消費 `store_code` 推導 MerID；[36 MQ Catalog](36-MQ%20與事件總覽（MQ%20&%20Event%20Catalog）.md) 定義 `account.deleted` / `seller.deleted` 契約。
- **姊妹**：違規停權走 [32 商家治理](32-商家治理與食安通報（Seller%20Governance%20&%20Food%20Safety）詳細設計書.md)；停權由措施在讀取時推導，不是 stint 的狀態（見 `internal/modules/seller/AGENTS.md`）。

---

## 2. 範圍與情境 (Context & Scope)

### 2.1 系統情境圖 (C4 Context)

```mermaid
flowchart LR
  Client["買家 / 賣家 App · Web"] -->|HTTPS Connect| MONO["單體<br/>identity · buyer · seller · accountdeletion"]
  MONO -->|還沒搬的路徑，帶 X-Buyer-Id／X-Store-Id| GW[Gateway]
  LEGACY["Java 與還沒搬的 Go 服務<br/>im · social · cart · coupon · order 守衛"] -->|legacy gRPC :9941<br/>SellerService.ResolvePrincipals · AccountService · BuyerService 子集| MONO
  MONO -->|MQ account.deleted · seller.deleted · buyer.*（legacy relay）| MQ[(RabbitMQ)]
  MONO --> DBM[(PG hawkr: identity / buyer / seller / accountdeletion schema)]
```

### 2.2 邊界與責任

| | 涵蓋 (In-scope) | 不涵蓋 (Out-of-scope，由誰負責) |
|---|---|---|
| 單體 identity、buyer、accountdeletion | （不在本文） | 帳號、角色、買家與刪帳流程：`docs/modules/{identity,buyer,accountdeletion}.md` |
| 單體 seller 模組 | `store_id`、`store_code`、stint 的建立與重新入駐、PayUni MerID 推導鍵、legacy `ResolvePrincipals` | 認證流程（見 18）、治理停權（見 32）、其餘業務行為（`internal/modules/seller/AGENTS.md`） |
| 守衛 5 服務 | 呼叫 `ResolvePrincipals` 做 resolve-to-principal 後比對 | 各自的下單 / 發券 / 私訊業務邏輯 |
| gateway | 還沒搬的路徑的身份解析（`buyerid`／`storeid` resolver）、`requireSeller` 能力檢查、下游 header 注入 | 憑證驗證（見 17） |

### 2.3 角色與身份模型

一個人（登入）→ 一個 principal → 各角色一個 aggregate。帳號（`account_id`）與買家（`buyer_id`）由單體擁有，規格見 `docs/modules/identity.md`、`docs/modules/buyer.md`；本文擁有賣家這一列：

| 身份 | 主鍵 | 生成 | 何時建立 | 與 `account_id` 的關係 |
|---|---|---|---|---|
| **seller** | `seller.stores.store_id`（`BIGINT GENERATED ALWAYS AS IDENTITY`，建立後不變）＋ `seller.stores.account_id`（非唯一） | 每個新 stint 由資料庫發新 `store_id`；同一列另鑄 `store_code`（MerID 推導鍵，見 §5.1） | 申請開店（`Apply`，與授予 SELLER 角色在同一個 transaction） | **獨立**：`store_id ≠ account_id`，**不得假設兩值相等**。同一 `account_id` 可累積多個 stint，目前的 stint 是 `status <> 'CLOSED'` 的那一列 |

`account_id`、`buyer_id`、`store_id` 是三個不同的值，**不得假設任兩者相等**。角色只有 SELLER，由開店時授予、關店時移除；視角不是授權（role-as-claim，見 §8.1），不儲存。

---

## 3. 約束與前提 (Constraints & Assumptions)

| 類型 | 項目 |
|---|---|
| 外部約束 | PayUni 代理商 MerID `HAWK{store_code}` 為**一次性綁定**：同一 MerID 重複申請回 `AGENT04017`。這是每個 stint 都鑄新 `store_code` 的唯一 forcing function |
| 技術約束 | `store_code` 是 seller 模組自有生成器（西元年後兩碼 × 10^8 ＋ 8 位 `crypto/rand`），靠 `stores_store_code` 唯一索引與最多 5 次撞號重試保證唯一；`store_id` 由資料庫發號 |
| **假設 (Assumptions)** | (1) 一個 account 同時至多一個目前的 stint（**1:1**，非多店）→ 失真則觸發 store 實體拆分（見 [ADR 2026-06-29](../adr/2026-06-29-identity-defer-store-entity.md)）。(2) `ResolvePrincipals` 與 `buyerid.Resolver`／`storeid.Resolver` 都在做**真實解析**：`buyer_id`／`store_id` 與 `account_id` 全面分離，任何依賴「兩值相等」的推導（含守衛比對、測試 fixture）都是錯的 |

---

## 4. 架構 (Architecture)

### 4.1 解法總綱 (Solution Strategy)

採 **distributed principal + per-role aggregate**（見 [ADR 2026-07-07](../adr/2026-07-07-identity-distributed-principal.md)）：

1. **登入身份維持純 principal**：`account_id` 語意不退化為「某角色下的人」。
2. **各角色模組自己持有 aggregate id**：賣家用 `store_id`，以 `account_id` 錨回 principal。`seller.stores.account_id` **非唯一**（同一 account 可累積多個 stint）。
3. **賣家 insert-new-stint**（見 [ADR 2026-07-11](../adr/2026-07-11-identity-store-stint-symmetry.md)）：帳號關店後再申請開店，新增一列，帶新 `store_id` 與新 `store_code`；舊列保留。新 `store_code` 使 MerID `HAWK{new_store_code}` 是 PayUni 沒看過的值 → 逃 `AGENT04017`。
4. **跨角色守衛 resolve-to-principal**：任何「不能對自己 X」的比對，先把 seller 端 id 經 `ResolvePrincipals` 還原成 principal，再與操作者 account_id 比對。買家端無對稱守衛需求（`buyerid.Resolver` 目的是身份解析轉發，非自我操作防護）。

### 4.2 元件視圖 (C4 Component)

| 模組/package | 責任 | 依賴 |
|---|---|---|
| `internal/modules/seller/internal/onboarding` | `Apply`：鎖帳號列、確認沒有目前的 stint、建立 stint（remint `store_id` ＋ `store_code`）、授予 SELLER，同一個 transaction | identity 公開 API |
| `internal/legacy/sellerserver` | legacy `SellerService` 的讀取方法，含 `ResolvePrincipals`、`GetCurrentStoreID` | seller 模組公開 API |
| `gateway/internal/middleware`（`InjectBuyerIDHeader`／`InjectStoreIDHeader`） | 反向代理路徑上解析營運鍵並注入 `x-buyer-id`／`x-store-id`，外部帶進來的同名 header 先刪 | `buyerid`／`storeid` resolver |
| `gateway/internal/buyerid`、`gateway/internal/storeid` | `Resolver.CurrentBuyerID`／`CurrentStoreID`：呼叫者自己時讀單體轉發帶的身份；查別的帳號時呼叫單體的 legacy 讀取。沒有快取 | 單體的 legacy listener |

### 4.3 部署拓撲與 Port

| 服務 | HTTP | gRPC | 對外暴露 |
|---|---|---|---|
| gateway | 9900 | - | 對外唯一入口 |
| 單體 | 8080 | 9941（legacy：AccountService、BuyerService、SellerService 的子集，只給還沒搬的服務） | 對外入口（Connect） |

> `ResolvePrincipals` 是單體 legacy listener 的內部 gRPC，被 im／social（Go）與 cart／coupon／order（Java）呼叫，不經 gateway、不對外。


### 4.4 命名／契約規約

| 契約 | 規則 | 出處 |
|---|---|---|
| PayUni MerID | `HAWK{store_code}` | `internal/modules/seller/payuni.go` `FormatPayUniMerID(storeCode)` |
| IM room id | `b{buyer_id}_s{store_id}`——兩者皆為**現行 stint 的 surrogate**，不得用 raw `account_id` 組 | `im-service .../usecase/message.go` `BuildRoomID` |
| MQ | exchange `hawkr.account` rk `account.deleted`；exchange `hawkr.seller` rk `seller.deleted` | §6.3 |
| 跨服務身份欄位 | 單體轉給 gateway 時帶 `X-Account-Id`（登入 principal）與已解析的 `X-Buyer-Id`、`X-Store-Id`；gateway 的 `buyerid`／`storeid` resolver 回答呼叫者自己時讀 header，再填入下游 gRPC 欄位 | §6.2、§8.3 |

⚠️ **`*AccountId` 這個名字不決定它裝什麼——判斷任一處一律看值不看名**：

| 值的來源 | 實際語意 | 命名是否正確 |
|---|---|---|
| `ResolvePrincipals` / `resolveAccountIdForStore` / `resolveNotifyBuyerAccountId` 的反解結果 | 登入 `account_id` | 正確 |
| 直接取自 `buyerId()` / `storeId()` | stint surrogate | **名字是債**，值是對的 |

現存的命名債（值正確、只會誤導讀的人）：payment `RescueRefundConsumer.serializeRescueCompletedPayload`
把 `buyer_id` surrogate 放進名為 `buyerAccountId` 的 wire key（因為 `notification_templates.target_user_field`
沿用該名）。
🔴 **改名必須 publisher ＋ `notification_templates` 同一批動**，否則 template 讀不到 key ＝ 靜默 0 投遞。
可繼續使用 `account_id` 的位置登記在 #2345；buyer 範圍的結論在 buyer 模組的 `CARRYOVER.md`。

---

## 5. 資料設計 (Data Design)

### 5.1 資料模型

帳號（`identity.accounts`）與買家（`buyer.buyers`）的表由單體擁有，見 `docs/modules/identity.md` §9 與 `internal/modules/buyer/db/schema.sql`。賣家的表由 seller 模組擁有，全部欄位以 `internal/modules/seller/db/schema.sql` 為準；本文只規定身份相關的欄位語意：

**`seller.stores`（一列是一個 stint）**

| 欄位 | 型別 | 說明 |
|---|---|---|
| `store_id` | BIGINT **PK**（`GENERATED ALWAYS AS IDENTITY`） | **賣家 aggregate 身份**（stint surrogate）。建立後不變；跨服務的賣家歸屬鍵一律用它 |
| `store_code` | BIGINT NOT NULL（`stores_store_code` 唯一，範圍 1–9999999999） | **PayUni MerID 的推導鍵**：`HAWK{store_code}`。西元年後兩碼 × 10^8 ＋ 8 位 `crypto/rand`，**非 `store_id`**；每個新 stint 各自鑄新值 ⇒ 重新入駐必得 PayUni 沒看過的 MerID |
| `account_id` | BIGINT NOT NULL（**非唯一**） | 登入 principal；建立後不變。同一 account 可有多個 stint |
| `status` | TEXT NOT NULL | `ACTIVE` / `PENDING_DELETION`（隨帳號刪除進入等待）/ `CLOSED`。停權不存在這裡，由措施在讀取時推導 |

> ⚠️ **`store_id`、`store_code`、`account_id` 是三個不同的值，不得互相代入**：對外的賣家歸屬鍵是 `store_id`，PayUni MerID 只認 `store_code`，登入 principal 是 `account_id`。

**`seller.payuni_registrations`**：`store_id` 是主鍵（一個 stint 至多一筆註冊），`mer_id` 為 `HAWK{該 stint 的 store_code}`。其餘欄位見 [18](18-賣家實名認證（Merchant%20Certification）詳細設計書.md) 與 schema。

### 5.2 索引與查詢模式

| Index | 查詢路徑 |
|---|---|
| `stores` PRIMARY KEY(store_id) | `ResolvePrincipals`、所有以 `store_id` 為鍵的賣家查詢 |
| `stores_store_code` UNIQUE(store_code) | MerID 唯一性的 DB 防線；建立 stint 時撞號重試靠它判定 |
| `stores_current_account` UNIQUE(account_id) **WHERE status <> 'CLOSED'** | 保證每個 account 至多一個目前的 stint；「找某人現在的店」走這條 |
| `stores_account` (account_id) | 以帳號找全部的 stint |

### 5.3 序號／排序／一致性

- `store_id` 由資料庫發號；`store_code` 由 seller 模組生成，**兩者不同來源，不可互換**。
- **開店原子性**：`Apply` 在同一個 transaction 內依序：鎖 identity 帳號列（`LockAccount`）→ 確認沒有目前的 stint、帳號沒有生效中的停權或終止合作 → 建立 stint（`INSERT … ON CONFLICT DO NOTHING`，`store_code` 撞號時換一個再試，最多 5 次，不使 transaction 失效）→ 授予 SELLER（`source_id` = 新 `store_id`）→ 發 `StoreOpened`。帳號等待刪除時 identity 拒絕授予，整筆回滾。

---

## 6. API 與介面契約 (API & Interface Contracts)

### 6.1 對外 API

登入 / 建帳 endpoint 見 [17](17-登入與帳號建立流程（Login%20&%20Account）詳細設計書.md)。賣家**關店**的對外方法是 Connect `SellerAppService.CloseMyStore`（契約以 `api/hawkr/seller/v1alpha1/seller.proto` 為準：店家有未結訂單回 `failed_precondition`；停權中也可以關店）。

> 帳號刪除（Connect `IdentityAppService.DeleteAccount`）見 `docs/modules/accountdeletion.md`。Admin 面的帳號生命週期端點（`GET /admin/accounts`、`GET /admin/accounts/:id`、`POST /admin/accounts/:id/suspend`、`POST /admin/accounts/:id/unsuspend`、`DELETE /admin/accounts/:id`）SSOT 在 [17](17-登入與帳號建立流程（Login%20&%20Account）詳細設計書.md)「帳號停權與解除停權」。

### 6.2 gRPC（服務間）

| RPC | 提供者 | 契約 |
|---|---|---|
| `ResolvePrincipals(ResolvePrincipalsRequest{repeated int64 ids})` | 單體 legacy `SellerService` | 回 `{int32 code, string message, map<int64,int64> principals, repeated int64 unresolved_ids}`；把**存活**（未關閉）stint 的 `store_id` → principal `account_id`。已關閉或不存在的 id **不在** `principals`、只出現在 `unresolved_ids`；不把 id 原樣回填。非正數 id 視為不存在 |
| `GetCurrentStoreID(account_id)` | 單體 legacy `SellerService` | 帳號目前的 `store_id`；沒有目前的店回 `NotFound`（gateway `storeid.Resolver` 只認 gRPC `NotFound`，不解析業務碼字串） |

`ResolvePrincipals` 訊息欄位以 `api/proto/seller/seller.proto` 為準（Java mirror 同步）。

> **精度例外**：`principals` map 的 value（account_id）**刻意保持 int64**（不字串化），因為此 RPC 純內部（im/social/cart/coupon/order → 單體）、不經 gateway、無 JS `>2^53` 精度風險——已在 `scripts/proto/numeric-allowlist.txt` 標 §6.2 #3 例外。

### 6.3 非同步事件 (MQ)

| 項目 | `account.deleted` | `seller.deleted` |
|---|---|---|
| Exchange / RK | `hawkr.account` / `account.deleted` | `hawkr.seller` / `seller.deleted` |
| **發送條件** | 帳號刪除時（只經 accountdeletion）由單體的 legacy relay 發 | stint **退場**（關店後的第一段清理，`StoreRetired`）時由 legacy relay 轉成；關店本身（`StoreClosed`）不發，因為關店在退場前可逆 |
| 冪等 key | accountId（consumer 皆冪等） | event_id |
| 消費端 | buyer 頭像 media 引用釋放 / im / seller / notification scrub PII（coupon 已遷至 `buyer.deleted`；social 由 `buyer.deleted`、`seller.deleted` 依身分種類清理，不消費本事件） | product 下架、social／im／notification 等刪除店家資料 |

> **為何下游刪除綁 `seller.deleted` 而非 `account.deleted`**：關店時帳號仍在、不發 `account.deleted`；`seller.deleted` 才是店家資料不可逆刪除的訊號。

### 6.4 即時通道 (WS/SSE)

> N/A — 本文件不擁有即時通道；IM WS 見 [14](14-即時通訊微服務（IM%20Service）詳細設計書.md)、SSE 見 [41](41-Realtime%20SSE%20推播平台（Realtime%20SSE%20Push%20Platform）詳細設計書.md)。

---

## 7. 執行時視圖 (Runtime View)

### 7.1 主要場景（happy path）：雙角色賣家關店 → 重新入駐

帳號有 SELLER 角色（每個帳號都是買家）。賣家在 App 內關店：

1. `SellerAppService.CloseMyStore`：鎖 identity 帳號列與帳號全部的 stint，檢查未結訂單；stint 轉 `CLOSED`，同一個 transaction 移除 SELLER 角色，發 `StoreClosed`。移除 SELLER 不刪帳號（帳號活著就是買家）→ **account_id 不變**、不發 `account.deleted`。
2. 退場（關店後 30 天，終止合作者取較晚者）發 `StoreRetired` → `seller.deleted`，下游刪除該店資料。

日後同一帳號重新申請開店：

3. `Apply`（§5.3）見沒有 `status <> 'CLOSED'` 的列 → 建立新列（**新 `store_id` ＋ 新 `store_code`**）→ 授予 SELLER。舊列與舊 `payuni_registrations` 列**保留**。
4. 認證通過 → MerID = `HAWK{new_store_code}` → PayUni 沒看過此 MerID → 綁定成功，逃 `AGENT04017`。

### 7.3 失效與重試

| 外部呼叫 | 逾時 | 重試 / 冪等 | 失敗降級 |
|---|---|---|---|
| `ResolvePrincipals`（守衛→單體） | 5s（Go client） | — | **Go（im/social）fail-open**（回原 id）；**Java（cart/coupon/order）fail-closed**（throw → SERVICE_UNAVAILABLE），見 §8.3 |
| 單體轉發前解析 `buyer_id`（入口→buyer） | — | 唯讀、不補建 | **fail-closed**：查詢失敗回 503 不轉發；gateway 的 `buyerid.Resolver` 查別的帳號時 RPC 失敗同樣擋下，不讓原始 `account_id` 漏傳 |

### 7.4 並發與順序

- **開店競態**：`Apply` 與 `CloseMyStore` 都先鎖 identity 帳號列，再鎖該帳號的 stint（鎖順序：帳號列先於店家列），序列化同一帳號的併發開店與關店；`stores_current_account` 是最後的資料庫防線。

### 7.5 狀態生命週期

stint 的狀態機（停權不是 stint 狀態，見 32）。帳號與買家的生命週期見 `docs/modules/identity.md` §4、`docs/modules/buyer.md` §2 與 `docs/modules/accountdeletion.md`。

```mermaid
stateDiagram-v2
  direction LR
  [*] --> ACTIVE: Apply
  ACTIVE --> CLOSED: 關店（賣家自助、終止合作、帳號刪除）
  ACTIVE --> PENDING_DELETION: 隨帳號刪除等待
  PENDING_DELETION --> CLOSED: 刪除流程關閉
  CLOSED --> ACTIVE: 恢復營業（終止合作撤銷或申覆成立，退場前）
  CLOSED --> [*]: 退場與個資清理（保留 row）
```

> **只有賣家走 insert-new-stint**：帳號再次申請開店另開新列、舊列保留，而且新列必須鑄新 `store_code`，否則新 MerID 會撞上舊綁定（`AGENT04017`）。買家與帳號同生命週期，不重新授予。關店的保留期、退場與清理規則以 `internal/modules/seller/AGENTS.md` 為準。
>
> ⚠️ **social-service 是唯一刻意不改鍵的下游**：`cms_comment.user_id` / `reports.reporter_id` / `user_blocks.blocker_id` 維持泛型命名，因為同一欄位可能裝賣家操作帶來的未 re-key 值，**不是單一 id space**。

---

## 8. 跨切面關注點 (Crosscutting Concerns)


### 8.1 安全與授權

- **role-as-claim**：授權一律按 `account_id` ownership 與角色（SELLER）；工作區提示（舊 REST 的 `X-Active-Role`）**不是授權輸入**（否則是 IDOR）。每個帳號都能用買家的方法，有 SELLER 的帳號在買家視角仍能用賣家的方法。
- **角色閘門**：單體的 Connect 方法依 service 檢查（buyer 的 App 方法只給帳號，管理員回 `permission_denied`）；還沒搬的路徑由 gateway 的 `requireSeller(cap)`（依入口帶的 `X-Seller-Capabilities` 檢查能力，沒有能力集合一律拒絕）與 `requireAdmin()` 判定。
- **視角不能提權**：舊 REST 的 `X-Active-Role` 提示 SELLER 但帳號沒有 SELLER 時，舊 gateway 收到的 `X-Login-Role` 是買家。
- **跨角色自我操作守衛**：見 §8.3；resolve-to-principal 讓雙角色的 seller 端與 buyer 端被認出是同一人。

### 8.2 可觀測性 (Observability)

| 訊號 | 內容 |
|---|---|
| log | 守衛的 `ResolvePrincipals` 失敗或 id 不被認得時 Warn（帶 store_id） |
| trace | 沿用平台 OTel；跨服務 gRPC 帶 trace context |


### 8.3 錯誤處理與降級 (Failure Modes & Degradation)

`ResolvePrincipals` 的降級是本重構最關鍵的跨切面決策——**依守衛性質分 fail-open / fail-closed**：

| 守衛 | 服務 | 語言 | RPC 失敗 / code≠0 | 理由 |
|---|---|---|---|---|
| 自我私訊（room self-message） | im | Go | **fail-open**（回原 id，守衛放行） | UX 守衛，非防詐 |
| 自我封鎖（block-self，**兩端都 resolve**） | social | Go | **fail-open** | UX 守衛 |
| 自我加購（self-item） | cart | Java | **fail-closed**（throw → SERVICE_UNAVAILABLE） | 涉金流 / 反詐 |
| 自我發券（self-issue，僅 DESIGNATED） | coupon | Java | **fail-closed** | 反詐 |
| 自我下單（self-purchase, 4015） | order | Java | **fail-closed** | 反詐 |

> Java client 對 `code != 0` 也 throw（commit 58a53038），避免「拿到空 map → `getOrDefault` 退回原 id → 靜默比錯」。
>
> **部署順序硬依賴（runbook 風險）**：order/cart/coupon 現為 fail-closed，單體的 legacy listener 必須**先**就緒，否則結帳/加購/發券全回 SERVICE_UNAVAILABLE。cart-service 因此成為對單體 legacy listener 的 SPOF。

**買家側：還沒搬的路徑，gateway 的買家身份一律經 `h.BuyerID.CurrentBuyerID` 解析（呼叫者自己讀單體帶來的 `X-Buyer-Id`），不得直送 raw `account_id`**。三組呼叫點：`notification.go`（`upsertDeviceToken`/`RemoveDeviceToken`）、`notification_inbox.go`（5 個 handler：`ListNotifications`/`GetUnreadCount`/`MarkNotificationAsRead`/`MarkAllNotificationsAsRead`/`DeleteNotifications`）、`resolve_seller.go` 的 `chatWireIdentity`（`chat.go` 8 個呼叫點共用）。⚠️ 第三組是**買家**聊天身份解析的實際位置，但檔名只提到 seller，新增買家面 handler 時容易漏審。

其他依賴降級：DB 不可用 → 交易 rollback；Redis 掛 → session 走 L1 stale-if-error（見 17 §3）。

### 8.4 一致性與冪等

- `seller.deleted` **at-least-once**，consumer 皆冪等去重。
- 開店與關店各在單一 transaction（含角色授予與移除），失敗整段 rollback。

---

## 9. 品質需求與 NFR (Quality Requirements / SLO)

### 9.1 SLO / SLI

| 指標 (SLI) | 目標 (SLO) | 量測點 |
|---|---|---|
| `ResolvePrincipals` 延遲 | < 5s（client timeout） | RPC round-trip |

### 9.2 容量與規模 (Capacity & Scaling)

- seller 模組是賣家資料的唯一寫入者，1:1 假設下無 sharding 需求。

### 9.3 效能預算

- `ResolvePrincipals` 為批次 RPC（一次多 id），守衛必須一次送整批 id、不得逐筆呼叫；批次上限為 `MaxBatchSize`。

---

## 10. 部署與維運 (Deployment & Operations)

### 10.1 環境與設定

- 守衛服務以環境變數指向單體的 legacy listener（例：im 的 `SELLER_SERVICE_ADDR`，預設 `127.0.0.1:9941`；cart 的 `SELLER_SERVICE_HOST`）。

### 10.2 部署順序

🔴 **部署順序硬依賴**：單體（含 legacy listener 的 `ResolvePrincipals`）**必須先於** order／cart／coupon 完成 rollout。那三支對 `ResolvePrincipals` 是 fail-closed（§8.3），單體未就緒時結帳／加購／發券會**全數**回 `SERVICE_UNAVAILABLE`。

---

## 11. 測試策略 (Test Strategy)

| 層級 | 覆蓋 | 關鍵場景 |
|---|---|---|
| unit | 守衛 | im/social fail-open、cart/coupon/order fail-closed（RPC error + code≠0 兩路徑）；`buyerid.Resolver`／`storeid.Resolver` 讀 header 與查別的帳號、`ErrNoCurrentBuyer`／`ErrNoCurrentStore` 分類（含真實 gRPC wire path 迴歸測試）。🔴 **`storeid.Resolver` 只認真正的 gRPC `NotFound`，不得解析業務碼字串** |
| integration | seller 模組 + DB | `stores_store_code` / `stores_current_account` invariant；`ResolvePrincipals` 不解析已關閉的 stint（`TestResolvePrincipalsExcludesClosedStints`）；併發開店的鎖競爭 |
| integration | legacy | `internal/legacy/sellerserver` 的 `ResolvePrincipals`、`GetCurrentStoreID` 契約 |

---

## 12. 術語表 (Glossary)

| 術語 | 定義 |
|---|---|
| principal | 登入身份，`accounts.account_id`；所有角色 aggregate 的錨點 |
| `store_id` | 賣家 aggregate 身份（`seller.stores` PK，資料庫發號，不變）；跨服務的賣家歸屬鍵 |
| `store_code` | `seller.stores` 的獨立欄位（`stores_store_code` 唯一），**MerID 的唯一推導鍵**；每個新 stint 各自鑄新值 |
| `buyer_id` | 買家 aggregate 身份（`buyers` PK，資料庫發號）；跨服務的買家歸屬鍵，一個帳號一生一個。**不得假設等於 `account_id`**；規格見 `docs/modules/buyer.md` |
| stint | 賣家的一段生命週期（`ACTIVE` → `CLOSED`）。帳號再次申請開店另開新 stint，同一 `account_id` 可累積多個 |
| 目前的 stint | `status <> 'CLOSED'` 的那一列，每個 `account_id` 至多一個（`stores_current_account`） |
| insert-new-stint | 賣家重新入駐的手法：**新增一列**而非原地改寫；舊列保留供稽核 |
| MerID | PayUni 代理商商店識別碼 `HAWK{store_code}`，一次性綁定（重複回 `AGENT04017`） |
| ResolvePrincipals | 單體 legacy `SellerService` 的 RPC，把存活 stint 的 `store_id` → principal `account_id`；已關閉或不存在的 id 列在 `unresolved_ids` |
| resolve-to-principal | 守衛比對前先把 seller 端 id 還原成 principal，使雙角色的兩端被認出同一人 |


| 視角（舊契約 `loginRole`） | 請求以買家或賣家看；Connect 沒有，只有舊 REST 的 `X-Active-Role` 提示，由 `internal/legacy` 推導成 `X-Login-Role`，非授權輸入（`docs/modules/identity.md` §6） |
| role-as-claim | 授權按 account_id ownership 與角色，不綁當前視角 |
