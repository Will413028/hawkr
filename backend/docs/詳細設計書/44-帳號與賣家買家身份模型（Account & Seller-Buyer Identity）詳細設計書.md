# 帳號與賣家買家身份模型（Account & Seller-Buyer Identity）詳細設計書

> **單體（遷移計畫 A2，2026-09-27 起）**：account 的部分搬進單體的 identity 模組（`internal/modules/identity`）。角色只存集合（`account_roles`，沒有 `account_type` bitmask；舊服務要的 bitmask 由 `internal/legacy` 投影）；`account_id` 過渡期由資料庫發號（IDENTITY），不再是兩位年份加亂數；刪除帳號當下清掉 email 與手機（T+30 的清除從未實作）。buyer 的部分是單體的 buyer 模組（`internal/modules/buyer`，遷移計畫 A3a），本文件的買家段落以它為準。seller 的部分（stint、`ResolvePrincipals`）仍是舊服務，遷移計畫 A3b 搬遷。差異與理由見 identity 模組的 `CARRYOVER.md`。

> 跨服務身份主題文件：描述 Hawkr 平台「登入身份（account）」「買家身份（buyer）」「賣家身份（seller）」三者如何建模、建立、角色升級、軟刪除、復活，以及在單人雙角色下如何用 `ResolvePrincipals` 維持跨角色守衛。
>
> 本文件涵蓋**賣家買家身份分離**（distributed-principal / incarnation）與**帳號角色軟刪除**。登入憑證機制（OAuth token 驗證、web cookie session、JWT、VerifyPhone、LinkConfirm、token refresh）不在本文件，見 [17 號](17-登入與帳號建立流程（Login%20&%20Account）詳細設計書.md)。

---

## 0. 文件元資訊 (Metadata)

| 項目 | 值 |
|---|---|
| 狀態 | `active` |
| 維護者 | Will |
| 對應程式碼 | `services/account-service/`（principal）、`internal/modules/buyer`（`buyer_id`）、`internal/modules/accountdeletion`（刪帳流程）、`services/seller-service/`（`store_id` / incarnation / ResolvePrincipals）、`gateway/internal/{handler,middleware,buyerid,storeid}`（還沒搬的路徑的身份解析）、跨角色守衛 `services/im-service`、`services/social-service`、`services/java/{cart,coupon,order}-service` |
| 相關 doc | [17 登入與帳號建立流程](17-登入與帳號建立流程（Login%20&%20Account）詳細設計書.md)（登入憑證機制）、[18 賣家實名認證](18-賣家實名認證（Merchant%20Certification）詳細設計書.md)（PayUni MerID 綁定源頭）、[30 Apple OAuth](30-Sign%20in%20with%20Apple%20認證（Apple%20OAuth）詳細設計書.md)、[39 買家服務](39-買家服務（Buyer%20Service）詳細設計書.md)（買家領域全貌）、[32 商家治理](32-商家治理與食安通報（Seller%20Governance%20&%20Food%20Safety）詳細設計書.md)（違規停權，非本文件的自助刪除）、[36 MQ 與事件總覽](36-MQ%20與事件總覽（MQ%20&%20Event%20Catalog）.md) |

---

## 1. 概述與目標 (Overview & Goals)

### 1.1 文件定位

本文件描述**身份的資料模型與生命週期**：三種身份（account / buyer / seller）各是什麼、如何建立、如何在單一帳號上疊加角色、如何軟刪除與復活、以及在「同一個人同時是買家與賣家」時如何避免自己對自己下單／發券／私訊。目標讀者為後端全體。**不涵蓋**登入憑證機制（OAuth／session／JWT，見 17）與實名認證細節（見 18）。

### 1.2 服務一句話定位

把「登入身份 `account_id`」與業務身份（賣家 `store_id`、買家 `buyer_id`）在資料層分離，使同一登入可扮演任意角色，且賣家刪店重開能取得全新的 PayUni 商店身份而不撞代理商一次性綁定上限。

### 1.3 設計目標 (Goals) 與 非目標 (Non-Goals)

| 類型 | 項目 |
|---|---|
| **Goal** | 每個帳號都是買家；賣家是另外申請的角色。買家不存成角色，登入不授予角色 |
| **Goal** | 帳號留墓碑、買家與賣家軟刪除（保留 row 供以 `buyer_id`／`store_id` 關聯的歷史），關閉 30 天後清 PII |
| **Goal** | 賣家刪店後重新開店 = 取得全新賣家身份（新 PayUni MerID），逃離 PayUni 代理商 `AGENT04017` 一次性 MerID 綁定上限 |
| **Goal** | 單人雙角色下，「不能對自己下單 / 發券 / 加購 / 私訊 / 封鎖」守衛仍正確 |
| **Non-Goal** | **不**引入 `store` 實體（一人多店 / 店鋪轉讓 / 多人共管）——見 [ADR 2026-06-29](../adr/2026-06-29-identity-defer-store-entity.md) |
| **Non-Goal** | **不**把 `account_id` 改為 TSID（維持 account-service 自有生成器，已知例外，見 §9.2） |
| **Non-Goal** | **不**處理「雙角色賣家復活後舊商品仍出現在自己下架分頁」的商品可見性 |

### 1.4 與其他設計書／服務的關係

- **上游**：[17 登入流程](17-登入與帳號建立流程（Login%20&%20Account）詳細設計書.md) 負責憑證驗證與 session；本文件承接其產出的 `account_id`、角色（SELLER）與視角（`Perspective`）。
- **下游**：[18 賣家實名認證](18-賣家實名認證（Merchant%20Certification）詳細設計書.md) 消費 `sellers.store_code` 推導 MerID；[36 MQ Catalog](36-MQ%20與事件總覽（MQ%20&%20Event%20Catalog）.md) 定義 `account.deleted` / `seller.deleted` 契約。
- **姊妹**：違規停權（治理 suspend）走 [32 商家治理](32-商家治理與食安通報（Seller%20Governance%20&%20Food%20Safety）詳細設計書.md)，與本文件的**自助刪除**互斥；兩者狀態機在 §7.5 並列。

---

## 2. 範圍與情境 (Context & Scope)

### 2.1 系統情境圖 (C4 Context)

```mermaid
flowchart LR
  Client["買家 / 賣家 App · Web"] -->|HTTPS Connect| MONO["單體<br/>identity · buyer · accountdeletion"]
  MONO -->|還沒搬的路徑，帶 X-Buyer-Id／X-Store-Id| GW[Gateway]
  GW -->|gRPC| SELLER["seller-service<br/>store_id store-stint"]
  LEGACY["Java 與還沒搬的 Go 服務"] -->|legacy gRPC :9941<br/>AccountService · BuyerService 子集| MONO
  SELLER -->|gRPC ResolvePrincipals| GUARD["im · social · cart · coupon · order<br/>自我操作守衛"]
  MONO -->|MQ account.deleted · buyer.*（legacy relay）| MQ[(RabbitMQ)]
  SELLER -->|MQ seller.deleted| MQ
  MONO --> DBM[(PG hawkr: identity / buyer / accountdeletion schema)]
  SELLER --> DBS[(PG: sellers / payuni_registrations)]
```

### 2.2 邊界與責任

| | 涵蓋 (In-scope) | 不涵蓋 (Out-of-scope，由誰負責) |
|---|---|---|
| account-service | `account_id` 生成、`account_type` bitmask、角色升級（AddRole）/移除（RemoveRole）、帳號軟刪、`auth_providers` 清理、`account.deleted` | 憑證驗證 / session（gateway + 17）、Apple revoke outbox 細節（見 17 §14） |
| buyer 模組（單體） | `buyer_id`（surrogate）、建帳時建立買家（訂閱 `AccountCreated`）、關閉時清除關聯資料、PII 清理 | 收藏 / 補貨 / 會員卡等買家領域（模組 `AGENTS.md`）；刪帳的編排（accountdeletion） |
| accountdeletion 模組（單體） | 刪帳申請、有未結訂單時等待、在同一個 transaction 關閉買家並刪除帳號 | 未結訂單的判斷（ordering，過渡期經 legacy 讀 Java） |
| seller-service | `store_id`、`store_code`、incarnation remint、PayUni MerID 推導、`ResolvePrincipals`、賣家軟刪 + outbox | 認證流程（見 18）、治理停權（見 32） |
| 守衛 5 服務 | 呼叫 `ResolvePrincipals` 做 resolve-to-principal 後比對 | 各自的下單 / 發券 / 私訊業務邏輯 |

### 2.3 角色與身份模型

三種身份，**一個人（登入）→ 一個 principal → 各角色一個 aggregate**：

| 身份 | 主鍵 | 生成 | 何時建立 | 與 `account_id` 的關係 |
|---|---|---|---|---|
| **account（登入 principal）** | `accounts.account_id` (BIGINT PK) | account-service 自有生成器（YY 年份 + 8 碼 `crypto/rand`，如 `2612345678`） | OAuth 首次建帳 | — |
| **buyer** | `buyers.buyer_id` (BIGINT PK，surrogate，資料庫發號) + `buyers.account_id`（**唯一**） | 建帳時建立，一個帳號一生只有一個 `buyer_id` | 建帳的同一個 transaction（`AccountCreated`） | **獨立**：`buyer_id ≠ account_id`，**不得假設兩值相等**。買家只隨帳號刪除而關閉，不重新授予 |
| **seller** | `sellers.store_id` (BIGINT PK，TSID，`NEVER mutated`) ＋ `sellers.account_id`（非唯一，與 buyer 對稱） | **所有**新建 `store_id`（含首次 `CreateSeller` 與重新入駐）一律 mint 新 TSID；同一列另鑄 `store_code`（MerID 推導鍵，見 §5.1） | 賣家角色 grant / `CreateSeller` | **獨立**：`store_id ≠ account_id`，**不得假設兩值相等**。同一 `account_id` 可累積多筆歷史 stint，`is_current` 標記現行那筆 |

**角色與視角分開**（ADR `2026-09-30-account-created-provisions-buyer.md` 的 W5 修訂）：

- 角色：identity 的 `account_roles` 只存要另外申請的 SELLER，由 seller 開店時授予、關店時移除；移除 SELLER 不刪帳號。「是買家」由「帳號活著」推導，不存成角色。管理員是另一種 principal（`admins`），不是角色。
- 視角：Connect 沒有視角（ADR `2026-10-07-connect-without-perspective`）。`identity.proto` 的 `Account.roles` 只有 `ROLE_SELLER`，`GetSession` 不回視角；「正在看哪個工作區」是 client 的 UI 狀態（website 以儲存的工作區與是否有賣家角色決定，有賣家角色且儲存的是賣家才是賣家，否則買家）。`X-Active-Role` 只有舊 REST、SSE、AI chat 與 chat ws-ticket 送，是舊 gateway 選擇以買家或賣家身分動作的提示（`internal/legacy` 推導 `X-Login-Role`：提示 SELLER 而且帳號有 SELLER 時才是賣家）。視角不是授權（role-as-claim，見 §8.1），不儲存。
- 舊契約：舊服務要的 `account_type` bitmask（1 買家、2 賣家、3 兩者、4 管理員）、`X-Account-Type`、`X-Login-Role` 由 `internal/legacy/accounttype` 在邊界合成，存活的帳號一律帶買家位元。

---

## 3. 約束與前提 (Constraints & Assumptions)

| 類型 | 項目 |
|---|---|
| 技術約束 | Go 服務 schema 走 **Atlas**（declarative，`schema.sql` 為 SSOT，migration 由 `migrate diff` 生成）；回應走統一 **Envelope** `{code,message,data}`；前端可控業務 ID 一律 string 防 TSID/BIGINT 精度遺失 |
| 技術約束（例外） | PK 紅線為 TSID，但 `account_id` 是 account-service 自有生成器（**非 TSID、無撞號重試**），`sellers.store_code` 是 seller-service 自有生成器（YY 年份 + 8 碼 `crypto/rand`，靠 `uq_sellers_store_code` + 最多 5 次撞號重試保證唯一）；`store_id` / `buyer_id` 才用 `pkg/tsid` |
| 外部約束 | PayUni 代理商 MerID `HAWK{store_code}` 為**一次性綁定**：同一 MerID 重複申請回 `AGENT04017`。這是 incarnation 存在的唯一 forcing function |
| 組織約束 | 單人 / 小團隊維運；**上線前不留技術債** |
| 既有系統約束 | Redis 無 HA（session / blacklist）；k3s + Traefik；RabbitMQ topic exchange |
| **假設 (Assumptions)** | (1) 一個 account 同時至多一個現行 seller stint（**1:1**，非多店）→ 失真則觸發 store 實體拆分（見 [ADR 2026-06-29](../adr/2026-06-29-identity-defer-store-entity.md)）。(2) 一個 account 一生只有一個買家，與帳號同生命週期；只有賣家走 store-stint。(3) `ResolvePrincipals` 與 `buyerid.Resolver`／`storeid.Resolver` 都在做**真實解析**：`buyer_id`／`store_id` 與 `account_id` 全面分離，任何依賴「兩值相等」的推導（含守衛比對、測試 fixture）都是錯的 |

---

## 4. 架構 (Architecture)

### 4.1 解法總綱 (Solution Strategy)

採 **distributed principal + per-role aggregate**（見 [ADR 2026-07-07](../adr/2026-07-07-identity-distributed-principal.md)）：

1. **account-service 維持純 principal**：`account_id` 語意不退化為「某角色下的人」。
2. **各角色服務自己持有 aggregate id**：buyer 用 `buyer_id`、seller 用 `store_id`，兩者都以 `account_id` 錨回 principal。`buyers.account_id` **唯一**（一個帳號一生一個買家）；`sellers.account_id` **非唯一**（同一 account 可累積多筆歷史 stint，`is_current` 標記現行那筆）。
3. **賣家 incarnation（insert-new-stint）**：重新入駐時在**同一 tx** 內 `RetireCurrentStint`（舊列 `is_current=false`）→ `Create` 新列（remint `store_id` TSID、另鑄新 `store_code`、`is_current=true`）。新 `store_code` 使 MerID `HAWK{new_store_code}` 是 PayUni 沒看過的值 → 逃 `AGENT04017`。`store_code` 撞號時整個 tx 重跑，最多 5 次（`uq_sellers_store_code`）。
4. **買家與帳號同生命週期**：建帳時建立唯一的買家，帳號活著 ⇔ 買家活著；後台只能刪賣家角色或整個帳號，買家只隨帳號刪除而關閉（有未結訂單時先進 `PENDING_DELETION`），關閉當下清除本地關聯資料。刪除的帳號留墓碑、不會再建，所以買家沒有重新授予，也沒有 insert-new-stint。
5. **跨角色守衛 resolve-to-principal**：任何「不能對自己 X」的比對，先把 seller 端 id 經 `ResolvePrincipals` 還原成 principal，再與操作者 account_id 比對。買家端無對稱守衛需求（`buyerid.Resolver` 目的是身份解析轉發，非自我操作防護）。

### 4.2 元件視圖 (C4 Component)

```mermaid
flowchart TB
  subgraph GW["gateway"]
    MW["middleware: resolveIdentity<br/>applyIdentity · pickActiveRole"]
    H["handler: auth / buyer / seller"]
  end
  subgraph ACCT["account-service"]
    AUC["usecase: OAuth resolve · AddRole · RemoveRole"]
    AREPO["repo: accounts · auth_providers · apple_revoke_outbox"]
  end
  subgraph SELLER["seller-service"]
    SUC["usecase: Create/Revive(remint) · ResolvePrincipals · DeleteSellerAccount"]
    SW["worker: seller_deleted_outbox"]
    SREPO["repo: sellers · payuni_registrations"]
  end
  subgraph MONO["單體"]
    ID["identity: 建帳 · 發布 AccountCreated"]
    BUC["buyer: OnAccountCreated · BeginDeletion · CloseForDeletion · CleanupPII"]
    DEL["accountdeletion: Request · CompletePending"]
    BREPO["buyer schema: buyers(+6 關聯表)"]
  end
  H --> MW
  MW -->|gRPC| AUC
  MW -->|CreateSeller| SUC
  AUC --> AREPO
  SUC --> SREPO
  SUC --> SW
  ID -->|同一個 transaction| BUC
  DEL -->|同一個 transaction| BUC
  BUC --> BREPO
```

| 模組/package | 責任 | 依賴 |
|---|---|---|
| `account-service/internal/usecase` | OAuth 解析、AddRole/RemoveRole、軟刪 + auth_providers 清理 | accounts / auth_providers repo |
| `seller-service/internal/usecase` | `Create`（首次入駐與 insert-new-stint 重新入駐，remint `store_id` + `store_code`）、`ResolvePrincipals`、DeleteSellerAccount + outbox | sellers / payuni repo |
| `internal/modules/buyer` | `OnAccountCreated`（建帳的 transaction 內建立買家）、`BeginDeletion`／`CloseForDeletion`（只由 accountdeletion 在鎖住帳號列的 transaction 內呼叫）、`CleanupPII` | 自己的 `buyer` schema；media（頭像參照） |
| `internal/modules/accountdeletion` | 刪帳申請、等待未結訂單、完成時關閉買家並刪帳 | identity、buyer、ordering port |
| `gateway/internal/middleware` | 身份解析、`pickActiveRole`、subset 防呆、下游 header 注入 | account/buyer/seller client |
| `gateway/internal/buyerid` | `Resolver.CurrentBuyerID`：呼叫者自己讀單體轉發帶的 `X-Buyer-Id`；查別的帳號呼叫單體 legacy `BuyerService.GetBuyerProfile`。沒有快取 | 單體的 legacy listener |

### 4.3 部署拓撲與 Port

| 服務 | HTTP | gRPC | 對外暴露 |
|---|---|---|---|
| gateway | 9900 | - | 對外唯一入口 |
| 單體（identity、buyer） | 8080 | 9941（legacy：AccountService、BuyerService 的子集，只給還沒搬的服務） | 對外入口（Connect） |
| seller-service | 10021 | 10020 | ClusterIP |

> `ResolvePrincipals` 是 **seller-service 內部 gRPC**，被 im/social（Go）與 cart/coupon/order（Java）呼叫，不經 gateway、不對外。

### 4.4 命名／契約規約

| 契約 | 規則 | 出處 |
|---|---|---|
| PayUni MerID | `HAWK{store_code}` | `seller-service/internal/domain/payuni.go` `FormatMerID(storeCode)` |
| IM room id | `b{buyer_id}_s{store_id}`——兩者皆為**現行 stint 的 surrogate**，不得用 raw `account_id` 組 | `im-service .../usecase/message.go` `BuildRoomID` |
| 舊契約的 account_type bitmask | buyer=1 / seller=2 / both=3 / admin=4，由單體在邊界合成 | `internal/legacy/accounttype` |
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

**`accounts`（account-service，principal 源頭）**

| 欄位 | 型別 | 說明 |
|---|---|---|
| `account_id` | BIGINT PK | YY 年份 + 8 碼 crypto/rand（**非 TSID**） |
| `email` | VARCHAR | 身份唯一鍵（partial unique，見 5.2） |
| `phone` | VARCHAR | **非** unique（#944，多帳號可共用） |
| `status` | VARCHAR | `active` / `inactive`（軟刪同時設 inactive）/ `suspended`（治理停權，見 [32](32-商家治理與食安通報（Seller%20Governance%20&%20Food%20Safety）詳細設計書.md)；端點見 [17](17-登入與帳號建立流程（Login%20&%20Account）詳細設計書.md)） |
| `account_type` | INT | 角色 bitmask（1/2/3/4） |
| `created_at` / `updated_at` / `deleted_at` | TIMESTAMPTZ | `deleted_at` = 軟刪 tombstone |

> `firebase_uid` / `apple_uid` / `apple_refresh_token` / `nickname` / `avatar` 欄位**已於 main 移除**（migrations `20260506002822`、`20260227130621`）；身份鍵改走 `auth_providers`。

**`auth_providers`（account-service，登入身份鍵）**

| 欄位 | 型別 | 說明 |
|---|---|---|
| `id` | BIGINT PK | 同 account_id 生成器 |
| `account_id` | BIGINT | FK → accounts |
| `provider` / `provider_sub_id` | VARCHAR | `google` / `apple` + provider subject；**UNIQUE(provider, provider_sub_id)** = 登入查詢鍵 |
| `email` / `email_verified` / `raw_payload` / `last_used_at` | — | `raw_payload` 存 Apple refresh_token（AES-GCM）供 revoke outbox |

**`buyer.buyers`（單體的 buyer 模組，`internal/modules/buyer/db/schema.sql`）**

| 欄位 | 型別 | 說明 |
|---|---|---|
| `buyer_id` | BIGINT **PK**（`GENERATED ALWAYS AS IDENTITY`） | 買家身份，永不改變；與 `account_id` 是不同的值 |
| `account_id` | BIGINT NOT NULL（**UNIQUE** `buyers_account`） | 登入身份錨點；一個帳號一生一個買家，關閉的列也保留 |
| `status` | TEXT | `ACTIVE` / `PENDING_DELETION` / `CLOSED`（CHECK）；`CLOSED` ⇔ `deleted_at` 有值 |
| `deleted_at` / `pii_cleared_at` | TIMESTAMPTZ | 關閉時記 `deleted_at`；PII 清理只作用於 `CLOSED` 的列 |
| `version` | BIGINT | profile 的樂觀鎖（etag） |
| （profile 欄位） | — | 暱稱、電話、性別、生日、城市、職業、頭像；`phone` 是買家自己維護的聯絡電話，不從帳號複製 |

> **與賣家的差異（刻意）**：買家沒有停權狀態，也沒有 stint 輪替：不需要 `is_current`，`account_id` 直接 UNIQUE。
>
> **關聯表以 `buyer_id` 為鍵**（模組內 FK）：`shipping_addresses`、`favorites`、`seller_subscriptions`、`restock_subscriptions`、`visit_history`、`memberships`。關閉買家時在同一個 transaction 刪除地址、收藏（逐筆發 `FavoriteRemoved`）、追蹤、瀏覽與補貨訂閱，並排入釋放頭像參照的 job；會員紀錄保留。

**`sellers`（seller-service，可分離身份）**

| 欄位 | 型別 | 說明 |
|---|---|---|
| `store_id` | BIGINT **PK**（TSID） | **賣家 aggregate 身份**（stint surrogate）。穩定不變（`NEVER mutated`）；跨服務的賣家歸屬鍵一律用它 |
| `store_code` | BIGINT NOT NULL（`uq_sellers_store_code`） | **PayUni MerID 的推導鍵**：`HAWK{store_code}`（`domain.FormatMerID`）。YY 年份 + 8 碼 `crypto/rand`（`domain.GenerateStoreCode`），**非 TSID**；建立時撞號則整個交易重跑，最多 5 次。每個新 stint 各自鑄新值 ⇒ 重新入駐必得 PayUni 沒看過的 MerID |
| `account_id` | BIGINT（**非唯一**） | 登入 principal 的外鍵；同一 account 可有多筆 stint（`is_current` 標當前） |
| `is_current` | BOOLEAN NOT NULL | 每個 account 至多一筆 `true`（`uq_sellers_account_current`，見 5.2） |
| `status` | VARCHAR(20) | `active` / `closed`（自助刪除）/ `suspended`（admin 停權）/ `suspended_governance`（治理停權，見 32）。`chk_sellers_active_is_current`：`status='active'` 蘊含 `is_current` |
| `deleted_at` / `pii_cleared_at` / `payuni_account_unlinked_at` | TIMESTAMPTZ | 軟刪 + PII 清理標記 |
| `current_certification_id` | BIGINT | 認證關聯（見 18） |

> ⚠️ **`store_id`、`store_code`、`account_id` 是三個不同的值，不得互相代入**：對外的賣家歸屬鍵是 `store_id`，PayUni MerID 只認 `store_code`，登入 principal 是 `account_id`。

**`payuni_registrations`（seller-service）**

| 欄位 | 型別 | 說明 |
|---|---|---|
| `account_id` | BIGINT NOT NULL | **非 unique**（incarnation 後同一 account 可有舊+新兩列） |
| `store_id` | BIGINT **NOT NULL UNIQUE** | 該筆註冊屬於哪一個賣家 stint；一個 stint 至多一筆註冊 |
| `payuni_mer_id` | VARCHAR(50) | 實際送 PayUni 的 MerID，值為 `HAWK{該 stint 的 store_code}` |

### 5.2 索引與查詢模式

| Index | 查詢路徑 |
|---|---|
| `idx_accounts_email_active` UNIQUE ON accounts(email) **WHERE deleted_at IS NULL** | email 身份唯一（軟刪後釋放 email 供同 email 重新註冊） |
| `idx_auth_providers_provider_sub` UNIQUE(provider, provider_sub_id) | 登入時以 (provider, sub) 找帳號 |
| `sellers_pkey` PRIMARY KEY(store_id) | `ResolvePrincipals` LEFT JOIN、所有以 `store_id` 為鍵的賣家查詢 |
| `uq_sellers_store_code` UNIQUE(store_code) | MerID 唯一性的 DB 防線；建立賣家時撞號重試（最多 5 次）靠它判定 |
| `uq_sellers_account_current` UNIQUE(account_id) **WHERE is_current** | 保證每個 account 至多一筆現行 stint；「找某人現在的店」走這條 |
| `payuni_registrations_store_id_key` UNIQUE(store_id) | 一個賣家 stint 至多一筆 PayUni 註冊；唯一性掛在 `store_id` **不在** `account_id`（同一 account 重新入駐後會有舊+新兩列） |
| `buyer.buyers_account` UNIQUE(account_id) | 一個帳號一生一個買家；由登入者找自己的買家走這條。建帳事件重送或重建違反約束時回錯誤讓建帳回滾，不補建 |

### 5.3 序號／排序／一致性

- `account_id` = `YY×10^8 + crypto/rand(10^8)`，`CreateAccount` 為純 INSERT **無 `ON CONFLICT` 無重試**（撞號直接回錯，見 §9.2）。同一生成器亦用於 `auth_providers` 的 id：`AuthProviderLink.ID` 經 gRPC `AuthProviderInfo.Id` 對外曝光，不適用 tsid 的不可讀格式，故維持 `crypto/rand`。`auth_audit_log`／`web_sessions` 的 id 與 `apple_revoke_outbox` 的 id（純內部識別碼，未對外曝光）改用 `pkg/tsid`，三者共用同一個 fx-managed generator instance；temp-bind JWT 的 `jti` 同樣改用 `pkg/tsid`，但因 jti 從未寫入資料表、無唯一性檢查，改用獨立的私有 generator instance，不與上述三者共用（純 mutex-guarded ms+seq、無 node-ID，多副本部署時跨 process 仍可能撞號）。
- `store_id` 用 `pkg/tsid`（42-bit ms + 22-bit random，時間排序）；`store_code` 用 `domain.GenerateStoreCode`（YY + 8 碼 `crypto/rand`），**兩者不同生成器，不可互換**。`buyer_id` 由資料庫發號（IDENTITY，row-identity ADR 的過渡規則）。
- **重新入駐原子性**：seller `Create` 的重新入駐路徑整段跑在 `WithTx`——`GetByAccountIDForUpdate`（row lock 序列化併發入駐）→ `RetireCurrentStint` → `Create`（新 `store_id`、新 `store_code`）→ 寫 `seller.onboarded` outbox 列，同 tx 提交；外部 account-info 取號在 tx 外。`store_code` 撞號（`uq_sellers_store_code` 的 `ErrDuplicate`）時**整個交易重跑**，最多 5 次。

---

## 6. API 與介面契約 (API & Interface Contracts)

### 6.1 對外 REST（經 Gateway）

登入 / 建帳 endpoint 見 [17](17-登入與帳號建立流程（Login%20&%20Account）詳細設計書.md)。本文件關切的**身份 / 生命週期** endpoint：

| Method + Path | 角色 | 說明 | 回應 |
|---|---|---|---|
| `BuyerAppService.GetProfile`（Connect） | 帳號 | 取自己的買家 profile（純讀，不補建） | 管理員 → `permission_denied`；存活的帳號沒有現任買家是不變量被破壞 → `internal` |
| `IdentityAppService.DeleteAccount`（Connect） | 帳號 | 自助刪除整個帳號（含買家），經 accountdeletion | `DELETED` 或 `PENDING_ORDERS`；有 SELLER → `failed_precondition` |
| `DELETE /app/sellers/me` | Seller | 賣家自助刪除 | `{code:0}`；已在刪除中 9204；有未結訂單 9205 |
| `POST /app/auth/active-role` | web | 切換並持久化 active role | 未持有該角色 → 403（見 17 §3） |

> Admin 面的帳號生命週期端點（`GET /admin/accounts`、`GET /admin/accounts/:id`、`POST /admin/accounts/:id/suspend`、`POST /admin/accounts/:id/unsuspend`、`DELETE /admin/accounts/:id`）SSOT 在 [17](17-登入與帳號建立流程（Login%20&%20Account）詳細設計書.md)「帳號停權與解除停權」——停權語意與 `accounts.status` / blacklist 的關係綁在一起，拆開描述必然漂移。

### 6.2 gRPC（服務間）

| RPC | 服務 | 契約 |
|---|---|---|
| `RemoveRole(account_id, Role)` | account（單體 legacy） | 移除 SELLER；不刪帳號（帳號活著就是買家）。`account_fully_deleted` 恆為 false |
| `ResolvePrincipals(ResolvePrincipalsRequest{repeated int64 ids})` | seller | 回 `{int32 code, string message, map<int64,int64> principals}`；把 `store_id` → principal `account_id`，非 `store_id` 的 id 原樣回傳 |
| `SelfDeleteSeller(account_id)` | seller | 賣家軟刪主流程（§7） |
| `CreateSeller(...)` | seller | 賣家角色 grant 時建立 seller profile（觸發 onboarding） |

`ResolvePrincipals` 訊息（`api/proto/seller/seller.proto` 的 `message ResolvePrincipalsRequest` / `ResolvePrincipalsResponse`，Java mirror 同步）：

```proto
message ResolvePrincipalsRequest  { repeated int64 ids = 1; }
message ResolvePrincipalsResponse { int32 code = 1; string message = 2; map<int64, int64> principals = 3; }
```

> **精度例外**：`principals` map 的 value（account_id）**刻意保持 int64**（不字串化），因為此 RPC 純內部（im/social/cart/coupon/order → seller）、不經 gateway、無 JS `>2^53` 精度風險——已在 `scripts/proto/numeric-allowlist.txt` 標 §6.2 #3 例外。

### 6.3 非同步事件 (MQ)

| 項目 | `account.deleted` | `seller.deleted` |
|---|---|---|
| Exchange / RK | `hawkr.account` / `account.deleted` | `hawkr.seller` / `seller.deleted` |
| Payload | `{eventType, accountId}` | `{accountId}` |
| **發送條件** | 帳號刪除時（只經 accountdeletion）由單體的 legacy relay 發 | 賣家軟刪**無條件**發（不看 role 是否清空） |
| 冪等 key | accountId（consumer 皆冪等） | accountId |
| 可靠性 | best-effort（out-of-tx，失敗 Warn） | **transactional outbox**（同 tx 寫 pending row，worker drain，見 §7.3） |
| 消費端 | buyer 頭像 media 引用釋放 / im / seller / notification scrub PII（coupon 已遷至 `buyer.deleted`；social 由 `buyer.deleted`、`seller.deleted` 依身分種類清理，不消費本事件） | product ACTIVE→INACTIVE 下架 |

> **為何下架綁 `seller.deleted` 而非 `account.deleted`**：刪賣家角色時帳號仍在、不發 `account.deleted` → 商品不會下架；`seller.deleted` 無條件發，語意正確。

> **另有 `seller.profile.changed`（非本文件核心）**：seller-service 於 `UpdateProfile` / `UpdateLogo` / `UpdateCover` 成功後 best-effort emit（exchange `hawkr.seller`，payload `{event_id, event_type, store_id(string), occurred_at}`），cart-service consume 後 evict `seller:meta` 快取。屬**資料變更**類（非身份/刪除），故走 out-of-tx best-effort（非 outbox）；契約詳 [36 §3.6f](36-MQ%20與事件總覽（MQ%20&%20Event%20Catalog）.md) 與 [04 §3.2](04-購物車微服務（Cart%20Service）詳細設計書.md)。

### 6.4 即時通道 (WS/SSE)

> N/A — 本文件不擁有即時通道；IM WS 見 [14](14-即時通訊微服務（IM%20Service）詳細設計書.md)、SSE 見 [41](41-Realtime%20SSE%20推播平台（Realtime%20SSE%20Push%20Platform）詳細設計書.md)。

---

## 7. 執行時視圖 (Runtime View)

### 7.1 主要場景（happy path）：雙角色賣家刪店 → 重開（incarnation）

帳號有 SELLER 角色（每個帳號都是買家）。賣家在 App 內刪除賣家帳號：

1. `DELETE /app/sellers/me` → seller-service `GetByAccountIDForUpdate` 鎖列，檢查 `IsActive` 與未結訂單。
2. 同 tx：`UpdateStatus=deleted` + 寫 `seller_deleted_outbox` pending row。
3. tx 外：`RemoveSellerRole` → 單體 legacy `AccountService.RemoveRole`。移除 SELLER 不刪帳號（帳號活著就是買家）→ **account_id 不變**、不發 `account.deleted`。
4. outbox worker drain → 發 `seller.deleted` → product 下架該賣家全部 ACTIVE 商品。

日後同一帳號重新申請賣家：

5. seller `Create` 走重新入駐路徑：`GetByAccountIDForUpdate` 見 `closed` 列 → 同 tx `RetireCurrentStint`（舊列 `is_current=false`）→ `Create` 新列（**remint `store_id` TSID ＋ 鑄新 `store_code`**，`is_current=true`）→ 寫 `seller.onboarded` outbox 列。舊列與舊 `payuni_registrations` 列**保留**供稽核。
6. 認證通過 → `domain.FormatMerID(seller.StoreCode)` = `HAWK{new_store_code}` → PayUni 沒看過此 MerID → 綁定成功，逃 `AGENT04017`。

### 7.2 時序圖

```mermaid
sequenceDiagram
  autonumber
  participant C as Seller App
  participant GW as Gateway
  participant S as seller-service
  participant A as 單體 identity
  participant OB as outbox worker
  participant P as product-service

  C->>GW: DELETE /app/sellers/me
  GW->>S: gRPC SelfDeleteSeller(account_id)
  S->>S: GetByAccountIDForUpdate (lock)
  alt 未結訂單>0
    S-->>GW: 9205 SELLER_HAS_ACTIVE_ORDERS
  else
    S->>S: tx: status=deleted + insert seller_deleted_outbox
    S->>A: RemoveSellerRole(account_id)
    A->>A: 移除 SELLER，帳號保留
    OB->>S: drain pending row
    OB->>P: publish seller.deleted
    P->>P: 該賣家 ACTIVE→INACTIVE
    S-->>GW: {code:0}
  end
```

### 7.3 失效與重試

| 外部呼叫 | 逾時 | 重試 / 冪等 | 失敗降級 |
|---|---|---|---|
| `RemoveSellerRole`（seller→account，同步） | 前台路徑：gRPC 預設；**outbox worker 路徑：10s**（`sdoRoleRemovalTimeout`，見 §8.6 的 🚨） | 立即回 error 給前端；本地已 `deleted` 時可重打（idempotent） | outbox worker 每分鐘順便重打尚未確認移除的 role → ≤1 分鐘自動收斂（**逾時後走 `bumpAttempt` 指數退避**，不再永久佔住 goroutine） |
| `seller.deleted` publish | — | transactional outbox：同 tx 寫 pending，worker drain（1min tick、backoff 1min×5 / 1h cap、`recover()` per-tick） | RabbitMQ 不通則 row 留 pending，恢復後補發（at-least-once + 冪等 consumer） |
| `account.deleted` publish / Redis blacklist | — | out-of-tx best-effort | 失敗 Warn；下次登入靠 `oauth_common.Resolve` 的 `isLiveAccount` 防呆擋孤兒 link |
| `ResolvePrincipals`（守衛→seller） | 5s（Go client） | — | **Go（im/social）fail-open**（回原 id）；**Java（cart/coupon/order）fail-closed**（throw → SERVICE_UNAVAILABLE），見 §8.3 |
| 單體轉發前解析 `buyer_id`（入口→buyer） | — | 唯讀、不補建 | **fail-closed**：查詢失敗回 503 不轉發；gateway 的 `buyerid.Resolver` 查別的帳號時 RPC 失敗同樣擋下，不讓原始 `account_id` 漏傳 |

### 7.4 並發與順序

- **復活競態**：`GetByAccountIDForUpdate` 取 `FOR UPDATE` row lock 序列化同一帳號的併發復活；active seller 直接回 `您已經是賣家`。
- **建帳即建買家**：買家在建帳的 transaction 內建立，`buyers_account` UNIQUE 保證一個帳號一個買家，沒有登入時的併發補建。
- **auth_providers dual-write**：建帳與 provider link 同 tx；INSERT 唯一鍵衝突 → `GetByProviderSub` 判定同帳號（idempotent）/ 不同帳號（1305）。

### 7.5 連線／狀態生命週期

三個狀態機（自助刪除 + 治理停權並列）：

```mermaid
stateDiagram-v2
  direction LR
  state "account" as A {
    [*] --> active_a: OAuth 建帳
    active_a --> inactive_a: accountdeletion 刪除帳號（同一個 transaction 關閉買家）
    inactive_a --> [*]: 墓碑保留（同一個 Clerk 使用者不會再建帳號）
  }
  state "seller" as S {
    [*] --> active_s: 認證通過
    active_s --> closed_s: 自助刪除
    closed_s --> active_s: 重新入駐 (insert-new-stint：舊列 is_current→false, 新列 remint store_id + store_code)
    active_s --> suspended_s: 違規停權 (見 32)
    suspended_s --> active_s: 申訴成功
    closed_s --> [*]: T+30 PII 清理 (保留 row)
  }
  state "buyer" as B {
    [*] --> active_b: 建帳的同一 transaction（AccountCreated）
    active_b --> pending_b: 刪除帳號但有未結訂單
    active_b --> closed_b: 刪除帳號且無未結訂單 (+關聯資料清理, 帳號同交易刪除)
    pending_b --> closed_b: 訂單結案 (+關聯資料清理, 帳號同交易刪除)
    closed_b --> [*]: 不重新授予
  }
```

> **buyer 清理**只在 `ACTIVE/PENDING_DELETION → CLOSED` 觸發（地址、收藏、追蹤、瀏覽、補貨訂閱直接刪除，見 §5.1）；**`PENDING_DELETION` 不清**（未結訂單需地址）；訂單有地址快照，order 歷史一律保留（法遵）。
>
> **買家沒有 `closed_b → active_b`**：帳號與買家同生命週期，關閉的買家列保留（供訂單等以 `buyer_id` 關聯的歷史），不會重新啟用或另鑄 `buyer_id`。賣家側 `closed_s → active_s` 仍走 insert-new-stint。

### 7.6 買家場景：刪除帳號

買家與帳號同生命週期，沒有「只移除買家角色」或「重新授予」：

1. 帳號刪除（本人或後台）由 accountdeletion 模組協調；有 SELLER 時先拒絕。
2. 有未結訂單時買家進 `PENDING_DELETION`（仍可使用，新增收藏、追蹤除外）；`order.completed` 與每小時的對帳 job 呼叫 `CompletePending`，訂單結案後完成。
3. 完成時在同一個 transaction 關閉買家、清除本地關聯資料、刪除帳號（留墓碑）。
4. 存活的帳號沒有現任買家是不變量被破壞，回 internal 並記 error log。

> **只有賣家走 insert-new-stint**：重新入駐另開新列、舊列保留（`is_current=false`），而且新列必須鑄新 `store_code`，否則新 MerID 會撞上舊綁定（`AGENT04017`）。買家不重新授予。
>
> ⚠️ **social-service 是唯一刻意不改鍵的下游**：`cms_comment.user_id` / `reports.reporter_id` / `user_blocks.blocker_id` 維持泛型命名，因為同一欄位可能裝賣家操作帶來的未 re-key 值，**不是單一 id space**。

---

## 8. 跨切面關注點 (Crosscutting Concerns)

### 8.1 安全與授權

- **role-as-claim**：授權一律按 `account_id` ownership 與角色（SELLER）；工作區提示（舊 REST 的 `X-Active-Role`）**不是授權輸入**（否則是 IDOR）。每個帳號都能用買家的方法，有 SELLER 的帳號在買家視角仍能用賣家的方法。
- **角色閘門**：單體的 Connect 方法依 service 檢查（buyer 的 App 方法只給帳號，管理員回 `permission_denied`）；還沒搬的路徑由 gateway 的 `requireSeller`/`requireBuyer`/`requireAdmin` 依合成的 `account_type` 判定。
- **視角不能提權**：舊 REST 的 `X-Active-Role` 提示 SELLER 但帳號沒有 SELLER 時，舊 gateway 收到的 `X-Login-Role` 是買家。
- **跨角色自我操作守衛**：見 §8.3；resolve-to-principal 讓雙角色的 seller 端與 buyer 端被認出是同一人。
- **孤兒 link 防呆**：OAuth `isLiveAccount` 要求 `GetByID` 成功（`deleted_at IS NULL`）且 `!IsDeleted` 且 `!HasNoActiveRole`，跳過指向已軟刪/無 role 帳號的孤兒 link（防 2026-07-05 永久登入死結）。

### 8.2 可觀測性 (Observability)

| 訊號 | 內容 |
|---|---|
| metric | `AuthDualWriteTotal{provider, ok|err}`（auth_providers dual-write）；`seller_deleted_outbox` failed-counter |
| log | RemoveRole 失敗 Warn（帶 accountId）；存活帳號查不到現任買家記 error |
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
> **部署順序硬依賴（runbook 風險）**：order/cart/coupon 現為 fail-closed，seller-service 必須**先**完整 rollout，否則結帳/加購/發券全回 SERVICE_UNAVAILABLE。cart-service 因此成為對 seller-service 的新 SPOF。

**買家側：還沒搬的路徑，gateway 的買家身份一律經 `h.BuyerID.CurrentBuyerID` 解析（呼叫者自己讀單體帶來的 `X-Buyer-Id`），不得直送 raw `account_id`**。三組呼叫點：`notification.go`（`upsertDeviceToken`/`RemoveDeviceToken`）、`notification_inbox.go`（5 個 handler：`ListNotifications`/`GetUnreadCount`/`MarkNotificationAsRead`/`MarkAllNotificationsAsRead`/`DeleteNotifications`）、`resolve_seller.go` 的 `chatWireIdentity`（`chat.go` 8 個呼叫點共用）。⚠️ 第三組是**買家**聊天身份解析的實際位置，但檔名只提到 seller，新增買家面 handler 時容易漏審。

其他依賴降級：DB 不可用 → 交易 rollback；MQ 不可用 → seller.deleted 留 outbox pending；Redis 掛 → session 走 L1 stale-if-error（見 17 §3）。

### 8.4 一致性與冪等

- `GrantRole` / `RemoveRole`（SELLER）冪等，無變化即 no-op。
- 建立買家在建帳的 transaction 內；關閉買家（`CloseForDeletion`）已關閉時成功，重送安全。
- `account.deleted` / `seller.deleted` **at-least-once**，consumer 皆冪等去重。
- insert-new-stint 與關聯清理皆在單一 tx，失敗整段 rollback。

#### 8.4.1 角色授予的歸因與撤銷柵欄

**不變量：由 T 時刻的刪除所授權的撤銷，不得撤銷 T 之後才成立的授予。**

賣家角色的授予與撤銷跨兩個服務，且兩個方向的可靠性不同——授予是請求路徑上的同步呼叫，撤銷是 `seller_deleted_outbox` 的子步驟、退避重試最長數小時。撤銷因此可能在「刪除 → 重新入駐」之後才執行，把中間那次合法授予蓋掉；受害者是「有店、無角色」，被 `requireSeller` 擋在自己店外，且登入不再授予角色（見角色授予 ADR）所以沒有自癒路徑。

單一寫入路徑無法涵蓋這條跨服務落差，處理方式是把**因果來源**寫進授予、再讓撤銷指名同一個來源：

| | 內容 |
|---|---|
| 歸因欄 | `account_roles.source_id` — 造成該筆授予的外部實體：該次入駐的 `sellers.store_id`（stint surrogate，重新入駐必為新值）；`NULL` ＝未歸因。只有 SELLER 有這個柵欄，遷移計畫 A3b 步驟 11 拿掉 |
| 授予 | `BindSeller(account_id, source_id)`。已持有角色時**改指**當前來源，不新增列；`source_id` 為空時**不改指**——未歸因的授予任何撤銷都撤得掉，用空值覆蓋既有歸因等於默默拆掉柵欄 |
| 撤銷 | `RemoveRole(account_id, role, source_id)` → `... WHERE revoked_at IS NULL AND (source_id IS NULL OR source_id = @source_id)`。比對就在該句 UPDATE 內，無 check-then-act 窗口 |
| 柵欄擋下時 | `rows = 0`，回報成功讓呼叫端的 outbox 列收斂；移除 SELLER 不刪帳號 |
| 未歸因授予 | 維持柵欄出現前的行為（可被任何撤銷關閉），下一次帶來源的授予會把它補上歸因 |

**兩個子步驟的時間語意刻意相反**，這是本節最容易寫錯的地方：

- **發 MQ 事件＝edge-triggered**。事件陳述過去發生的事，payload 的 surrogate 必須是入列當時的快照；消費時重解析會與後續的重新入駐競態而改寫歷史。
- **角色同步＝進場 level-triggered、出場帶柵欄**。`seller.onboarded` 的授予子步驟在**執行當下**重讀帳號的當前 stint（`GetSellerByAccountID`，`is_current AND deleted_at IS NULL`），因此遲到的重試會授予給「現在真實存在的那次入駐」，查無當前 stint 則不授予；`seller.deleted` 的撤銷子步驟則指名它所關閉的那個 stint。

**授予點與順序**：`POST /app/sellers` **不由 gateway 授予角色**——授予必須指名 stint，而 stint surrogate 是在 seller-service 的建立交易內鑄造的，gateway 沒有任何時點拿得到。seller-service 於同一交易寫入 stint ＋ `seller.onboarded` outbox 列，交易後同步呼叫 `BindSeller`；**該同步呼叫只是延遲最佳化，保證來自 outbox 列的授予子步驟**——同步呼叫失敗只記 Warn，不得讓建店失敗。

### 8.5 設定與機密

- 無本重構專屬新 secret；Apple refresh_token 於 `auth_providers.raw_payload` 以 AES-GCM 加密（`RefreshTokenEncrypter`）。
- session / TTL / CSRF 等 env 見 [17](17-登入與帳號建立流程（Login%20&%20Account）詳細設計書.md)。

### 8.6 多副本排程搶佔語意（Scheduler Claim）

**規範全文**：[開發規範／多副本排程搶佔規範](../開發規範/多副本排程搶佔規範.md)。本節列本文件擁有的**兩支 outbox worker**（seller-service 的 `PayuniSyncWorker` 見 [18 號 §9](./18-賣家實名認證（Merchant%20Certification）詳細設計書.md)）。

**副本數前提（緊急度的判定輸入）**：`account-service` 與 `seller-service` 在 prod **都是單副本**（`k8s/values/overlays/prod/common.yaml:4` 的 `replicas: 1`，兩者的 per-service overlay 都不覆蓋 `replicas`；prod 只有 `cart`／`gateway`／`order`／`payment`／`payuni-webhook` 五支被覆寫成 `2`，全清單與查證方式見[規範 §〇](../開發規範/多副本排程搶佔規範.md)）。⇒ 本節處理的是[規範規則 ⑧](../開發規範/多副本排程搶佔規範.md)的**擴容前置條件**，不是今天穩態下的重複；唯一的例外是 rolling update 期間的雙 pod 窗口（chart 預設 `maxUnavailable: 0` / `maxSurge: 1`）——**每次部署都發生**。

| | **`apple_revoke_outbox`（account-service）** | **`seller_deleted_outbox`（seller-service）** |
|---|---|---|
| 排程 | `applerevoke.Scheduler`，ticker **每 5 分鐘** | `SellerDeletedOutboxScheduler`，ticker **每 1 分鐘** |
| 候選述詞 | `status = 'pending' AND next_attempt_at <= $1 ORDER BY next_attempt_at LIMIT 50` | 同左 |
| 副作用 | 解密 refresh token ＋ **Apple revoke HTTP**（`requestTimeout` 5 秒） | publish 生命週期事件 ＋ account-service 角色同步 gRPC——`seller.deleted` 走 **`RemoveSellerRole`**（另有 `HasActiveBuyer` 查詢，由單體的 legacy 回應），`seller.onboarded` 走 **`BindSeller`**（另有當前 stint 查詢，見 §8.4.1） |
| 搶佔閘門 | `ClaimAppleRevokeOutbox` | `ClaimSellerDeletedOutbox` |
| 租約欄 | **既有的 `next_attempt_at`**（不另加欄位） | 同左 |
| 租約長度 | **15 分鐘**（= tick 5 分 × 3） | **3 分鐘**（= tick 1 分 × 3） |
| 候選窗口寬度 | **∞**（pending row 一直是候選，直到 `MarkDone` / `MarkFailed`） | 同左 |

**閘門 SQL（兩支同形）**：

```sql
UPDATE <outbox_table>
   SET next_attempt_at = sqlc.arg(lease_until)
 WHERE id = sqlc.arg(id)
   AND status = 'pending'                                -- 非 PK 述詞（規則 ⑦）
   AND next_attempt_at <= sqlc.arg(claimable_before);    -- 與候選述詞一致
```

`rows = 0` ⇒ **一個副作用都不做**。account 側 claim 刻意放在 `Decrypt` 之前——audit append 是一次 DB 寫入，[規範規則 ①](../開發規範/多副本排程搶佔規範.md)不允許閘門與副作用之間夾任何其他 I/O。seller 側 claim 放在 `event_published_at` / `role_removed_at` 兩個子步驟**之前**，因為這道閘門要蓋住**整列的處理**；那兩個欄位是斷點續傳的進度記號，**不是**跨副本的互斥。授予子步驟刻意沒有進度記號：它是 level-triggered 且冪等，重跑的代價是一次 RPC，不可能出錯（§8.4.1）。

**租約長度的上下界**：

| | 下界（單筆最壞耗時） | 上界 |
|---|---|---|
| apple revoke | 一次本地解密 ＋ 一次 Apple HTTP，而該呼叫已被 `requestTimeout`（5 秒）封頂 ⇒ 15 分鐘有兩個數量級以上餘裕 | 候選窗口 ∞，且 Apple 呼叫**不會靜默失敗**（`RevokeRetryable` 餵給 `bumpAttempt`，由它寫自己的退避覆蓋租約）⇒ 租約只管崩潰窗口，取 3 個 tick |
| seller deleted | 一次 MQ publish（毫秒）＋ `HasActiveBuyer`（3 秒）＋ `RemoveSellerRole`（**`sdoRoleRemovalTimeout` 10 秒**）≈ **13 秒** ⇒ 3 分鐘有一個數量級餘裕 | 同左（兩個副作用的 error 都傳回 `bumpAttempt`） |

- **租約基準逐筆各自取**（不是 tick 起點）：取 tick 起點 ⇒ 第 k 筆的有效租約 = `租約 − 前 k−1 筆的耗時`；seller 側**每筆兩次 gRPC 往返、`sdoFetchBatch = 50`**，一個批次內就會變成負數（[規範 §三 第 6 點](../開發規範/多副本排程搶佔規範.md)）。CAS 上界 `claimable_before` 維持 tick 起點——那個方向讓可搶佔集合更小，是保守的。

🚨 **`RemoveSellerRole` 的 10 秒 deadline 是租約下界能成立的前提，不是順手加的韌性**
沒有 deadline 時（繼承 scheduler 的 cancel-only background ctx），account-service 若是「**掛住**」而非「拒絕」，那個 goroutine **永不返回**，整條 outbox drain 永久停擺；而且**「租約長度 > 單筆最壞耗時」在最壞耗時無界時根本無從論證**。`sdoRoleRemovalTimeout = 10 * time.Second` 讓單筆最壞耗時有一個可寫下來的數字（≈13 秒），3 分鐘的下界才是可證明的。
⇒ **任何把新的無界呼叫加進這個迴圈的改動，都會同時打破韌性與租約下界的論證，必須連 deadline 一起補。**（同一條規則的 Java 版見[規範 §四 出路 4 的硬性邊界](../開發規範/多副本排程搶佔規範.md)。）

🚨 **兩支候選查詢不得使用 `FOR UPDATE SKIP LOCKED`，它在這裡零保護**
兩支 `RunOnce` 都在**自動提交**下執行（傳裸 ctx 直接拿 pool，沒有交易），SELECT 一回傳 row lock 就釋放 ⇒ 兩個副本拿到**同一批**、各自做一遍副作用（[規範 §一 ②](../開發規範/多副本排程搶佔規範.md)）。互斥**只能**靠上面的 claim 閘門，不得把 `SKIP LOCKED` 當第二道保險，也不得在 doc comment 裡宣稱它提供多副本安全。

**例外 B 不適用（兩支皆然，不要拿它當捷徑）**：
1. **不是「結構上不可能」**：`next_attempt_at` 本身就是時間欄 ⇒ lease **零 schema 變更**就做得到，[規範 §四 例外 B 條件 1(a)](../開發規範/多副本排程搶佔規範.md) 直接不成立。
2. **不是「語意上只該發一次」**：兩支的設計都是 **at-least-once 投遞**（重試上限 5 次、指數退避 `2^attempt × 1 分` 上限 1 小時、耗盡標 terminal `failed`），與條件 1(b) 講的「一次性事件、已經發出去就是終態」相反。

**行為退化**：pod 在 claim 與副作用之間崩潰 ⇒ 該筆延後**一個租約**（apple 15 分 / seller 3 分）才重來。租約不是鎖：到期後可能與原持有者重疊 ⇒ 回到「重複一次」，下游冪等仍是最終防線（`RemoveSellerRole` 本身冪等、`seller.deleted` / `account.deleted` 的 consumer 皆冪等去重，見 §8.4）。**at-least-once 契約不變。**

---

## 9. 品質需求與 NFR (Quality Requirements / SLO)

### 9.1 SLO / SLI

| 指標 (SLI) | 目標 (SLO) | 量測點 |
|---|---|---|
| `seller.deleted` 投遞 | 最終一致（outbox） | outbox pending 積壓 + failed-counter |
| `ResolvePrincipals` 延遲 | < 5s（client timeout） | RPC round-trip |

### 9.2 容量與規模 (Capacity & Scaling)

- `account_id` keyspace = 每年 10^8，**無撞號重試**；目前帳號量遠低於生日悖論風險門檻。**觸發**：帳號量成長到同年撞號機率不可忽略 → 補 collision-retry 或改共用 TSID（取捨待裁決，見 #3501）。
- seller-service 與單體的 buyer 模組各自是唯一寫入者，1:1 假設下無 sharding 需求。

### 9.3 效能預算

- `ResolvePrincipals` 為批次 RPC（一次多 id）+ 單一 LEFT JOIN，守衛必須一次送整批 id、不得逐筆呼叫。
- 買家關聯清理是關閉 transaction 內的幾個 DELETE，量級小（單一買家自有資料）。

---

## 10. 部署與維運 (Deployment & Operations)

### 10.1 環境與設定

- 單體（identity、buyer、accountdeletion）與 seller-service；DB DSN、Redis、MQ 走既有 env。
- 無本重構專屬新 env（session/CSRF 見 17）。

### 10.2 部署順序

🔴 **部署順序硬依賴**：seller-service（含 `ResolvePrincipals`）**必須先於** order／cart／coupon 完成 rollout。
那三支對 `ResolvePrincipals` 是 fail-closed（§8.3），seller-service 未就緒時結帳／加購／發券會**全數**
回 `SERVICE_UNAVAILABLE`——cart-service 因此成為對 seller-service 的 SPOF。

### 10.3 健康檢查與優雅關閉

- 各服務沿用平台 health/readiness；`seller_deleted_outbox` worker per-tick `recover()` 防單次 panic 拖垮。
- gateway session drain 見 17。

### 10.4 Runbook

- 卡住的 seller role 移除：outbox worker ≤1 分鐘自動重打 `RemoveSellerRole` 收斂；手動可重打 `DELETE /app/sellers/me`（idempotent）。
- PII 清理：seller 有 T+30 cron；buyer 是單體的每小時 River job（`CLOSED` 且 `deleted_at` 超過 30 天、`pii_cleared_at IS NULL`，以 `buyer_id` 分批，冪等）。

---

## 11. 測試策略 (Test Strategy)

| 層級 | 覆蓋 | 關鍵場景 |
|---|---|---|
| unit | usecase | GrantRole/RemoveRole 冪等；賣家重新入駐鑄新 `store_id` **與新 `store_code`**（含 `store_code` 撞號時整個交易重跑）；`ResolvePrincipals` 對非 `store_id` 的 id echo-back |
| unit | 守衛 | im/social fail-open、cart/coupon/order fail-closed（RPC error + code≠0 兩路徑）；`buyerid.Resolver`／`storeid.Resolver` 讀 header 與查別的帳號、`ErrNoCurrentBuyer`／`ErrNoCurrentStore` 分類（含真實 gRPC wire path 迴歸測試）。🔴 **`storeid.Resolver` 只認真正的 gRPC `NotFound`，不得解析業務碼字串**——它呼叫的是專屬、無業務碼 envelope 的 `GetCurrentStoreID`（見 [ADR 2026-08-19](../adr/2026-08-19-identity-current-store-id-rpc.md)）。|
| integration | repo + DB | `uq_sellers_store_code` / `uq_sellers_account_current` invariant；買家：建帳的 transaction 內建立、`buyers_account` UNIQUE、只清 CLOSED、PII 以 `buyer_id` 清理（buyer 與 accountdeletion 模組的 integration 測試）；`payuni_registrations` join on `store_id`；`GetPrincipalsByStoreIDs` LEFT JOIN；**testcontainers 真實 Postgres 併發測試**（insert-new-stint atomicity，pool pre-warm + 起跑訊號強制真實鎖競爭，斷言輸家拿到贏家的新 stint 而非未處理錯誤） |
| integration | outbox | `seller_deleted_outbox` 同 tx 寫入 + worker drain + 重試 |
| e2e | 端到端 | 雙角色刪賣家 → 重新入駐 → **新 MerID 與舊值不同**；刪帳號 → 有未結訂單時 `PENDING_ORDERS`、訂單結案後帳號與買家同時關閉；賣家 insert-new-stint 端到端（新列 + 舊列 `is_current` 翻轉 + gateway 轉發新 surrogate） |

---

## 12. 術語表 (Glossary)

| 術語 | 定義 |
|---|---|
| principal | 登入身份，`accounts.account_id`；所有角色 aggregate 的錨點 |
| `account_id` | principal 主鍵；account-service 自有生成器（YY + 8 碼 crypto/rand，非 TSID） |
| `store_id` | 賣家 aggregate 身份（`sellers` PK，TSID，`NEVER mutated`）；跨服務的賣家歸屬鍵 |
| `store_code` | `sellers` 的獨立欄位（`uq_sellers_store_code`），**MerID 的唯一推導鍵**；YY + 8 碼 `crypto/rand`，每個新 stint 各自鑄新值 |
| `buyer_id` | 買家 aggregate 身份（`buyers` PK，資料庫發號）；跨服務的買家歸屬鍵，一個帳號一生一個。**不得假設等於 `account_id`** |
| stint | 一個 `buyer_id`／`store_id` 對應的一段生命週期（active → pending_deletion/closed）。賣家重新入駐另開新 stint，同一 `account_id` 可累積多筆歷史 stint；買家每個帳號只有一段 |
| `is_current` | `sellers` 的布林欄位，每個 `account_id` 至多一筆 true（`uq_sellers_account_current` partial unique index 保證）；insert-new-stint 時舊列翻 false、新列設 true。買家不需要（`buyers.account_id` 唯一） |
| incarnation | 賣家刪店後重新入駐得到的那一段新 stint：新 `store_id` ＋ 新 `store_code` ⇒ 新 MerID |
| insert-new-stint | 賣家重新入駐時的手法：**新增一列**而非原地改寫，`RetireCurrentStint` + `Create` 包在同一 tx；舊列保留供稽核 |
| MerID | PayUni 代理商商店識別碼 `HAWK{store_code}`，一次性綁定（重複回 `AGENT04017`） |
| ResolvePrincipals | seller-service RPC，把 `store_id` → principal `account_id`；非 `store_id` 的 id 原樣回傳 |
| resolve-to-principal | 守衛比對前先把 seller 端 id 還原成 principal，使雙角色的兩端被認出同一人 |
| `account_type` | 舊契約的角色 bitmask（buyer=1 / seller=2 / both=3 / admin=4），由單體在邊界合成；單體內只有角色（SELLER） |
| 視角（舊契約 `loginRole`） | 請求以買家或賣家看；Connect 沒有，只有舊 REST 的 `X-Active-Role` 提示，由 `internal/legacy` 推導成 `X-Login-Role`，非授權輸入 |
| role-as-claim | 授權按 account_id ownership 與角色，不綁當前視角 |
