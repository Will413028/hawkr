# Review Service 詳細設計書

> 對應程式碼：`services/review-service/`（Go：`internal/{grpc,usecase,domain,repository,client,mq,config}`、`db/{schema,queries,migrations}`）、`api/proto/review/review.proto`、`gateway/internal/{handler,grpc,router}` 的 review / order_review 檔
> 對應 MQ 事件：見 [36-MQ 與事件總覽](./36-MQ%20與事件總覽（MQ%20%26%20Event%20Catalog）.md) §`hawkr.review`

---

## 1. 服務概述與責任邊界

### 1.1 服務職責

review-service 是訂單商品評論的專責微服務，負責「買家對已完成訂單中單一商品項目」評論的完整生命週期：

- 提交評論（資格驗證、重複防護、圖片永久化、商品快照落地）
- 查詢評論（商品公開列表、賣家收到列表、買家自身歷史、單筆詳情）
- 評論統計（賣家層級星級分布、商品層級平均分與總數）
- 內容審核隱藏（B-135，由 social-service 內部 RPC 觸發）
- 賣家回覆評論（#1174：賣家對自己商品收到的評論張貼一則官方回覆，可編輯覆蓋）
- 評論事件發布（`review.created` / `review.hidden`，供 order / seller / product 服務同步聚合與快照）

一筆 `order_item_reviews` row = 一個買家對一個 `order_item` 的評論。**評論以 `order_item_id` 為唯一鍵**，一個訂單項目最多一則評論。

### 1.2 上游 / 下游依賴

| 方向 | 對象 | 用途 | 連線方式 |
|------|------|------|---------|
| 上游（呼叫 review） | Gateway | 對外 REST → gRPC | gRPC（外部使用者，經 JWT） |
| 上游（呼叫 review） | social-service | `HideReview`（B-135 內容審核） | gRPC（`x-internal-service`） |
| 下游（review 呼叫） | order-service | `GetOrderForReview`：訂單資格 + 商品項目 + 權威 product_id | gRPC `ORDER_SERVICE_ADDR` |
| 下游（review 呼叫） | media-service | `FinalizeAndLink`：評論圖片 staged → permanent + asset_reference | gRPC（`x-internal-service`） |
| 下游（review 呼叫） | 單體 buyer 模組（legacy gRPC） | `BatchGetBuyerDisplay`：批量取買家顯示名 / 頭像（已註銷者回墓碑，#2830） | gRPC `BUYER_SERVICE_*` |
| 下游（review 呼叫） | hawkr-website | `/api/revalidate`：SSR cache bust（product / seller tag） | HTTP（`go-pkg/revalidate`） |
| 事件消費者 | order-service | `review.created` → markReviewed + `orders.review` 快照 | MQ |
| 事件消費者 | product-service | `review.created` / `review.hidden` → 商品 rating / review_count 聚合（#1156） | MQ |

### 1.3 非職責範圍

- **不負責**商品本身的 rating / review_count 持久化 → 由 product-service 消費 `review.created` / `review.hidden` 自行落地
- **不負責**賣家 rating 的彙總與顯示 → 店家頁與推薦排行向 review 讀取（評分不存在 seller 模組）
- **不負責**訂單 `is_reviewed` 旗標與 `order_items.review` 快照 → 由 order-service 消費事件落地
- **不負責**評論圖片儲存 → 委派 media-service（review 只持有最終 objectKey 快照，49 號設計書）
- **不提供**買家評論本體的「修改 / 刪除」對外 RPC（買家評論不可變；`deleted_at` 為內部軟刪除欄位，目前無對外刪除入口）。賣家回覆（reply）為例外：屬賣家對自己內容的官方回覆，**可編輯覆蓋**，但**不提供刪除回覆**（#1174）

### 1.4 系統架構圖

```mermaid
flowchart TB
    subgraph External[對外]
        Buyer[買家 App / Web]
        Public[匿名訪客]
    end
    GW[Gateway :9900]
    subgraph Review[review-service]
        H[gRPC Handler :10033]
        UC[ReviewUseCase]
        Repo[(order_item_reviews<br/>PostgreSQL)]
        Pub[MQ Publisher]
    end
    Social[social-service]
    Order[order-service]
    Media[media-service]
    BuyerSvc[單體 buyer 模組]
    Web[hawkr-website]
    MQ{{hawkr.review<br/>Topic Exchange}}
    OrderC[order-service consumer]
    ProductC[product-service consumer]

    Buyer -->|JWT REST| GW
    Public -->|REST| GW
    GW -->|gRPC| H
    Social -->|HideReview gRPC internal| H
    H --> UC
    UC --> Repo
    UC -->|GetOrderForReview| Order
    UC -->|FinalizeAndLink| Media
    UC -->|BatchGetBuyerDisplay| BuyerSvc
    UC -->|revalidate bust| Web
    UC --> Pub
    Pub -->|review.created / review.hidden| MQ
    MQ --> OrderC
    MQ --> ProductC
```

### 1.5 名詞定義

| 名詞 | 說明 |
|------|------|
| 訂單商品評論（OrderItemReview） | 買家對一個 `order_item` 的評分 + 文字 + 圖片，唯一鍵 `order_item_id` |
| 權威 product_id | 由 order-service 回傳的訂單項目 product_id，**忽略客戶端傳入值**（防偽造） |
| 快照欄位 | `sku_name` / `item_image_key` / `price`，下單當時商品狀態的副本，避免評論頁回查 order |
| 評論期限（Review Window） | 訂單完成後 30 天內可評論（`ReviewWindowDays`） |
| 隱藏（hidden） | 平台審核將評論標記 `hidden_at`，全站查詢一律排除（B-135） |
| 軟刪除（deleted） | `deleted_at` 內部欄位，查詢一律排除（目前無對外刪除入口） |

### 1.6 角色與權限邊界

| RPC | 角色 | 授權依據 | Namespace |
|-----|------|---------|-----------|
| `GetProductReviews` | 匿名 / 任意 | 無（公開）；`viewer_id` 僅用於封鎖過濾 | `/public/products/{id}/reviews` |
| `SubmitOrderReview` | BUYER | Gateway 由 JWT `x-account-id` 解析出現任 `buyer_id` 後注入 | `/app/reviews` |
| `GetReview` | 登入使用者 | Gateway JWT | `/app/reviews/{id}` |
| `ListMyOrderReviews` | BUYER | Gateway 解析出的現任 `buyer_id` | `/app/me/reviews` |
| `GetSellerOrderReviews`（賣家自看） | SELLER | Gateway 解析出的現任 `store_id` | `/app/sellers/me/reviews` |
| `GetSellerOrderReviewStats` | SELLER | JWT | `/app/sellers/me/reviews/stats` |
| `GetSellerOrderReviews`（公開賣家頁） | 匿名 | path `id`（賣家 `store_id`） | `/public/sellers/{id}/reviews` |
| `ReplyReview`（賣家回覆，#1174） | SELLER | Gateway 解析並注入現任 `store_id` + `requireSeller`（accountType 2/3）+ usecase 驗證 `review.store_id == 操作者的 store_id` | `/app/sellers/reviews/{id}/reply` |
| `HideReview` | 內部服務 | `x-internal-service`（由 social-service AdminResolveReport 呼叫） | 無對外路由 |

> 身分驗證僅在 Gateway 進行（[Java 微服務開發規範](../開發規範/Java%20微服務開發規範.md) §5.1）。review-service 信任 Gateway 注入的 `buyer_id` / `store_id`，不自行驗證外部使用者。

---

## 2. 狀態模型與業務規則

評論本身**無多階段狀態機**（買家評論本體不可變），但有三組正交的可見性 / 資格判定。賣家回覆（reply）為評論的**附屬可變欄位**（`reply_content` / `replied_at`），可由賣家編輯覆蓋，不影響評論本體與任何聚合，詳見 §2.6。

### 2.1 評論可見性模型

| 狀態欄位 | 值 | 對查詢的影響 |
|---------|----|------|
| `deleted_at` | NULL / 時間戳 | 非 NULL → 所有列表 / 統計排除 |
| `hidden_at` | NULL / 時間戳 | 非 NULL → 所有列表 / 統計排除（B-135） |

```mermaid
stateDiagram-v2
    [*] --> Visible: SubmitOrderReview 成功
    Visible --> Hidden: HideReview（B-135 審核）
    Visible --> Deleted: 軟刪除（內部，目前無對外入口）
    Hidden --> Hidden: HideReview 重複呼叫（idempotent，不重發 event）
    note right of Hidden
        hidden_at / deleted_at 非 NULL
        → 列表 + 統計一律排除
    end note
```

> ⚠️ **例外**：`GetReview`（單筆詳情，`GetReviewByID`）**不**過濾 `hidden_at` / `deleted_at`，會回傳已隱藏評論。`ReplyReview` 的 0-rows 分流依賴這個行為來區分 404／403／409。

### 2.1a 作者顯示名的三種狀態

`order_item_reviews` **沒有名字快照欄**，作者名一律即時向單體 buyer 模組解析。
買家註銷後仍必須顯示得出來 —— 這是 #2830 的核心。三種狀態必須分得開：

| 狀態 | `user_name` | `user_deleted` | 公開列表顯示 |
|---|---|---|---|
| 一般買家 | 真實暱稱 | `false` | `maskUsername` 遮罩後的暱稱（例：`王**`） |
| 已註銷買家 | 墓碑字串「已刪除帳號」 | `true` | **原樣顯示，不遮罩** |
| 解析失敗／未設暱稱 | 空字串 | `false` | 「匿名買家」，**不遮罩** |

🔴 **`ReviewInfo.user_deleted`（proto 欄位 15）存在的唯一理由，就是讓 Gateway
不必對 `user_name` 做字串比對**。遮罩的目的是不讓匿名訪客看到某個真人的完整
暱稱；墓碑與「匿名買家」都是我方常數、不是任何人的個資，遮它們不保護任何人，
只會產生「已刪***」「匿名**」這種既不好讀、又讓同一個買家在不同畫面顯示不同
字串的結果。

實作：review-service `review_usecase.go`（兩個顯示路徑填 `BuyerDeleted`）→
`internal/grpc/mapper.go` → Gateway `maskBuyerDisplayName`
（`gateway/internal/handler/order_review.go`，測試 `TestMaskBuyerDisplayName`）。

⚠️ `review.created` 事件的買家名是**建立當下的快照**（那時買家必然還在），
刻意不帶 `deleted` —— 買家日後註銷不影響已送出的通知。

### 2.2 評論資格規則（SubmitOrderReview 入口檢查）

依序檢查，任一不過即回對應錯誤碼（實作：`ReviewUseCase.SubmitItemReview`）：

| 順序 | 檢查 | 不通過錯誤 |
|------|------|-----------|
| 0 | 內容過濾（profanity，`go-pkg/moderation.Check`） | `ErrModerationRejected`（業務碼 `ModerationRejected`） |
| 1 | order-service `GetOrderForReview` 成功 | `ErrOrderNotFound`（404） |
| 2 | 訂單狀態 = 完成（`"50"`） | `ErrNotEligible`（412） |
| 3 | 訂單買家 == 請求買家 | `ErrNotYourOrder`（403） |
| 4 | 訂單完成 +30 天內（`ReviewWindowDays`） | `ErrReviewExpired`（412） |
| 5 | `order_item_id` 屬於該訂單（取權威 product_id） | `ErrItemNotInOrder`（400） |
| 6 | `order_item_id` 尚未評論過 | `ErrAlreadyReviewed`（409） |

入口型別驗證（gRPC handler `SubmitOrderReview`）：`order_no`/`order_item_id`/`buyer_id` 必填、`rating ∈ [1,5]`、`content ≤ 1000`、`staged_tokens ≤ 4`、`price` 可解析為整數。

> 訂單完成狀態碼 `"50"`、評分區間、AssetReferenceType `7115`（ORDER_REVIEW）等代碼引用 [代碼管理（CodeMeta）](../代碼管理（CodeMeta）/README.md)，不在本書複製完整表格。

### 2.3 重複評論防護（雙層）

1. **應用層**：`GetByOrderItemID` 查到既有 → `ErrAlreadyReviewed`
2. **DB 層**：`order_item_reviews_item_unique UNIQUE (order_item_id)`，並發 race 時 `Insert` 捕捉 `23505` → `domain.ErrAlreadyReviewed`（`review_repo.go` Insert）

兩層之間存在 check-then-insert 時間窗，但 DB unique 為最終守門，並發第二筆必拿 409，天然冪等。

### 2.4 內容審核隱藏（B-135）

- 由 social-service `AdminResolveReport` 對 `REVIEW` 目標執行 `HIDE_TARGET` 時，gRPC 呼叫 `HideReview`
- 落地：`UPDATE ... SET hidden_at = NOW(), hidden_reason = $2 WHERE id = $1 AND deleted_at IS NULL AND hidden_at IS NULL`（idempotent，已隱藏不重複更新）
- 隱藏後重算 seller + product 聚合，發 `review.hidden` event 供下游覆寫（`ReviewUseCase.HideReview`）

### 2.5 圖片快照與防 GC

- 提交前圖片以 staged token 形式存在 media-service，**有 GC 時效**
- 提交時先產生 `reviewID`（TSID）作為 `refID`，逐張呼叫 `FinalizeAndLink(token, refType="7115", refID)` 永久化並建立 asset_reference（`ReviewUseCase.SubmitItemReview`）
- 任一張 finalize 失敗 → 整筆評論建立失敗（圖片是評論的一部分，不允許半套）

---

### 2.6 賣家回覆

一則評論對應 0 或 1 則賣家回覆，寫在評論本體的 `reply_content` / `replied_at` 兩欄（無獨立 reply 表）。

| 規則 | 內容 |
|------|------|
| 關係 | 一則評論對應 **0 或 1** 則賣家回覆（1:1），對齊 proto 單一 `ReviewInfo.reply` |
| 權限 | 僅該評論的賣家（`order_item_reviews.store_id == 操作者的現任 store_id`）可回覆；Gateway `requireSeller`（accountType 2/3）+ usecase 擁有權驗證雙層 |
| 可變性 | **可編輯覆蓋**：賣家可再次回覆覆蓋舊內容並更新 `replied_at`（顯示最後回覆時間）。每次（含覆蓋）都重跑 `moderation.Check`，防「先乾淨後改髒」繞審 |
| 可回覆條件 | 評論須存在、屬該賣家、且**未被隱藏 / 軟刪除**（`hidden_at IS NULL AND deleted_at IS NULL`）。已隱藏 / 刪除評論不可回覆 |
| 已回 / 未回判斷 | **唯一判斷源 = `replied_at IS NOT NULL`**（DB CHECK 保證 `reply_content` 與 `replied_at` 同生同滅） |
| 內容限制 | `content` 非空、≤ `MaxContentLength`（1000，複用買家評論限制） |
| 不刪回覆 | 不提供刪除回覆 API；schema 不留 `reply_deleted_at` |

**reply 在各 surface 的 wire 形狀**（公開賣家頁與賣家管理共用 `GetSellerOrderReviews`）：

| 端點 | RPC | DTO / 輸出 | reply wire 形狀 |
|------|-----|-----------|----------------|
| `GET /public/products/{id}/reviews`（核心） | GetProductReviews | `ReviewListItem`（顯式 DTO） | camelCase `reply:{content,repliedAt}`，未回 null |
| `GET /public/sellers/{id}/reviews` | GetSellerOrderReviews | `PublicSellerReviewItem`（顯式 DTO） | camelCase 同上 |
| `GET /app/sellers/me/reviews`（賣家管理） | GetSellerOrderReviews | 直傳 proto `OrderReviewInfo` | snake_case `reply.created_at`（跟隨該 surface 既有風格）|
| `GET /app/me/reviews`（買家自評） | ListMyOrderReviews | 直傳 proto `OrderReviewInfo` | snake_case 同上 |
| `GET /app/reviews/{id}`（單則） | GetReview | 直傳 proto `ReviewInfo` | snake_case 同上 |

> **wire 欄名一致性**：DTO surface（核心 + 公開賣家頁）的 reply 用 camelCase 子結構 `{content, repliedAt}`；直傳 proto 的 surface（賣家管理 / 買家自評 / 單則）reply 跟隨該 surface 既有 snake_case 全欄風格。前端各 page 本就對不同 surface 有各自 normalize。

> **ReplyInfo 欄位 mapping**：`ReplyInfo.created_at` ⇔ `Review.RepliedAt`（DTO surface 對外欄名 `repliedAt`）。`ReplyInfo.id` 在 1:1 reply 設計下無業務意義（無獨立 reply PK），mapper 填空字串、DTO 子結構**不輸出 id**（僅 `{content, repliedAt}`）。已回 / 未回一律以 `RepliedAt` 非零（直傳 proto surface 則 `reply` 非 null）判斷。

**範圍邊界（明訂不做）**：

- **不發 MQ event**：reply 不改 rating / review_count / 可見集合，既有 consumer（order `markReviewed`、product rating 聚合）都不需要它。
- **不觸發 revalidate**：賣家回覆不改任何聚合，**不** bust product / seller tag。（對照：`SubmitItemReview` 與 `HideReview` 改聚合，兩者都 bust。）
- **不發買家通知**：買家主動瀏覽時看到回覆。
- **order-service `orders.review` JSONB 快照不含 reply**：該快照屬 order-service 範疇。
- **不提供刪除回覆 API**（見上表）。

---

## 3. 資料庫設計

DB：`hawkr_review`（PostgreSQL）。schema 由 Atlas 管理（`db/schema/schema.sql` + `db/migrations/`），query 由 sqlc 生成（[PostgreSQL 設計規範與最佳實踐](../開發規範/PostgreSQL%20設計規範與最佳實踐.md)）。

### 3.1 order_item_reviews（唯一業務表）

| 欄位 | 型別 | 約束 | 說明 |
|------|------|------|------|
| `id` | BIGINT | PK | TSID（評論 ID，`pkg/tsid`） |
| `order_no` | VARCHAR(64) | NOT NULL | 訂單號 |
| `order_item_id` | BIGINT | NOT NULL, UNIQUE | 訂單商品項目 ID（唯一鍵，防重複評論） |
| `buyer_id` | BIGINT | NOT NULL | 買家業務鍵（buyers.buyer_id surrogate，非登入 account_id） |
| `store_id` | BIGINT | NOT NULL | 賣家 store 代理鍵（sellers.store_id surrogate，非登入 account_id） |
| `product_id` | BIGINT | NOT NULL DEFAULT 0 | 商品 ID（聚合 + 連結用） |
| `rating` | SMALLINT | NOT NULL CHECK (1–5) | 評分 |
| `content` | TEXT | NULL | 文字（≤1000） |
| `image_keys` | TEXT[] | NOT NULL DEFAULT '{}' | 圖片永久 objectKey（≤4；原 `image_urls`，49 號設計書 §5.3） |
| `sku_name` | VARCHAR(255) | NOT NULL DEFAULT '' | 商品名稱快照 |
| `item_image_key` | TEXT | NOT NULL DEFAULT '' | 商品圖快照 objectKey（原 `item_image_url`，49 號設計書 §5.3） |
| `price` | BIGINT | NOT NULL DEFAULT 0 | 單價快照（元，整數） |
| `created_at` | TIMESTAMPTZ | NOT NULL DEFAULT CURRENT_TIMESTAMP | 建立時間（UTC） |
| `deleted_at` | TIMESTAMPTZ | NULL | 軟刪除 |
| `hidden_at` | TIMESTAMPTZ | NULL | 平台隱藏（B-135） |
| `hidden_reason` | TEXT | NULL | 隱藏原因 |
| `reply_content` | TEXT | NULL | 賣家回覆內容（#1174，與 `replied_at` 同生同滅）|
| `replied_at` | TIMESTAMPTZ | NULL | 賣家回覆時間（#1174；**已回 / 未回唯一判斷源**）|

> 表級 CHECK（#1174）：`CHECK ((reply_content IS NULL) = (replied_at IS NULL))` 保證回覆內容與時間同生同滅，杜絕「有時間沒內容」半套狀態。

> 時間欄位 `TIMESTAMPTZ`，符合 [Java 微服務開發規範](../開發規範/Java%20微服務開發規範.md) §2.3 / [PostgreSQL 設計規範與最佳實踐](../開發規範/PostgreSQL%20設計規範與最佳實踐.md)。金額 `price` 用 `BIGINT`（元，整數），符合 [Java 微服務開發規範](../開發規範/Java%20微服務開發規範.md) §2.2 金額規範。

### 3.2 索引

| 索引 | 欄位 | 用途 |
|------|------|------|
| `order_item_reviews_pkey` | (id) | 主鍵 |
| `order_item_reviews_item_unique` | (order_item_id) | 唯一，重複評論防護 |
| `idx_order_item_reviews_order_no` | (order_no) | 按訂單查已評論項目 |
| `idx_order_item_reviews_buyer` | (buyer_id, created_at DESC) | 買家評論歷史分頁 |
| `idx_order_item_reviews_seller` | (store_id, created_at DESC) | 賣家評論列表分頁 |
| `idx_order_item_reviews_product` | (product_id, created_at DESC) | 商品評論列表分頁 |

### 3.3 product_id legacy（DEFAULT 0）

`product_id` 為 `NOT NULL DEFAULT 0`。**舊評論可能為 0**（該欄位加入前建立，或提交時 order-service 未回傳有效值）。新評論一律用 order-service 回傳的權威值（§5.1），不會再產生 0。

🔴 **Gateway 映射時 `product_id = 0` 一律留空**（`PublicSellerReviewItem.productId` 與 `ReviewListItem.productId` 皆然），否則前端會渲染出 `/products/0` 的死連結。

---

## 4. 對外 / 對內介面定義

> **型別合規（[Java 微服務開發規範](../開發規範/Java%20微服務開發規範.md) §2.1：前端可控 request id 一律 string）**：經 Gateway 暴露給前端的 request body / path 業務 ID 為 **string**（TSID > 2^53）。涉及：`SubmitOrderReview`(`order_item_id`/`product_id`)、`GetProductReviews`/`GetReviewStats`(`product_id` path)、`GetReview`(`review_id` path)、`GetSellerOrderReviews`/`GetSellerOrderReviewStats`(`store_id`：admin path 前端可控；賣家自查時由 Gateway 以 JWT FormatInt 帶入)。`buyer_id` / `viewer_id` / `ReplyReviewRequest.store_id`（#1174）由 JWT 注入維持 int64（[Java 微服務開發規範](../開發規範/Java%20微服務開發規範.md) §2.1 例外①）。review-service gRPC handler 邊界 ParseInt（DB bigint 不變）。前端 hawkr_buyer 評論提交 `order_item_id` 以 `.toString()` 送字串（Dart int 為 64-bit，本身不丟精度，但後端契約改 string 後須送字串 body）。

### 4.1 Gateway 對外 REST 映射

| Method | Path | Gateway Handler | gRPC RPC | 角色 |
|--------|------|-----------------|----------|------|
| GET | `/public/products/{id}/reviews` | `GetProductReviews`（review.go） | `GetProductReviews` | 匿名 |
| GET | `/public/sellers/{id}/reviews` | `GetPublicSellerReviews`（order_review.go） | `GetSellerOrderReviews` | 匿名 |
| GET | `/public/sellers/{id}/reviews/stats` | `GetPublicSellerReviewStats` | `GetSellerOrderReviewStats` | 匿名 |
| POST | `/app/reviews` | `SubmitOrderItemReview` | `SubmitOrderReview` | BUYER |
| GET | `/app/reviews/{id}` | `GetReview` | `GetReview` | 登入 |
| GET | `/app/me/reviews` | `ListMyOrderReviews` | `ListMyOrderReviews` | BUYER |
| GET | `/app/sellers/me/reviews` | `GetSellerOrderReviews` | `GetSellerOrderReviews` | SELLER |
| GET | `/app/sellers/me/reviews/stats` | `GetSellerOrderReviewStats` | `GetSellerOrderReviewStats` | SELLER |
| POST | `/app/sellers/reviews/{id}/reply` | `ReplyReview`（order_review.go） | `ReplyReview` | SELLER（#1174）|

### 4.2 gRPC Proto（ReviewService）

定義於 `api/proto/review/review.proto`。RPC 清單：

| RPC | Request | Response | 說明 |
|-----|---------|----------|------|
| `GetProductReviews` | `GetProductReviewsRequest`（含 `viewer_id` 封鎖過濾） | `ProductReviewsResponse`（`ReviewInfo[]` + `ReviewStats`） | 商品公開評論 |
| `GetReview` | `GetReviewRequest` | `ReviewResponse`（`ReviewInfo`） | 單筆 |
| `SubmitOrderReview` | `SubmitOrderReviewRequest` | `SubmitOrderReviewResponse` | 提交（buyer） |
| `ListMyOrderReviews` | `ListMyOrderReviewsRequest` | `ListMyOrderReviewsResponse`（`OrderReviewInfo[]`） | 買家歷史 |
| `GetSellerOrderReviews` | `GetSellerOrderReviewsRequest` | `GetSellerOrderReviewsResponse`（`OrderReviewInfo[]`） | 賣家收到 / 公開賣家頁 |
| `GetSellerOrderReviewStats` | `GetSellerOrderReviewStatsRequest` | `GetSellerOrderReviewStatsResponse` | 賣家統計 |
| `ReplyReview`（#1174） | `ReplyReviewRequest`（`review_id` string / `store_id` int64 JWT / `content`） | `ReviewResponse`（含 `ReviewInfo.reply`） | 賣家回覆評論（SELLER，可編輯覆蓋） |
| `HideReview` | `HideReviewRequest` | `HideReviewResponse` | 內部審核（social-service） |

> 上表即 `ReviewService` 的**完整** RPC 集合。`ReviewInfo.like_count` / `reply_count` 為早期商品評論模型的遺留欄位，**無人填也無人讀**——已回 / 未回一律以 `reply` 是否存在判斷。`ReviewStats` message 由 `GetProductReviews` 回傳並直接對外輸出。

**`OrderReviewInfo` 欄位**：

```protobuf
message OrderReviewInfo {
    string id = 1;
    string order_no = 2;
    string order_item_id = 3;
    string buyer_id = 4;
    string store_id = 5;
    int32 rating = 6;
    string content = 7;
    repeated string image_keys = 8;   // objectKey 陣列，原 image_urls（49 號設計書 §5.3）
    string sku_name = 9;
    string item_image_key = 10;       // objectKey，原 item_image_url（49 號設計書 §5.3）
    string price = 11;
    string created_at = 12;
    string buyer_name = 13;
    string buyer_avatar = 14;
    string product_id = 15;   // 商品 ID（前端連結商品頁用；0 由 Gateway 映射成空）
    ReplyInfo reply = 16;     // 賣家回覆（null = 未回覆）
}
```

> 數字 / ID 欄位對外一律 `string`（[Java 微服務開發規範](../開發規範/Java%20微服務開發規範.md) §2.1）；`rating` 為該節例外（Response `int32` 慣例沿用，避免破壞既有前端）。

### 4.3 依賴的外部 gRPC

| Client | RPC | 取得 | 失敗降級 |
|--------|-----|------|---------|
| order（`ORDER_SERVICE_ADDR`，預設 `order-service:9915`） | `GetOrderForReview` | 訂單資格 + 商品項目 + 權威 product_id + 快照 | → `ErrOrderNotFound`，評論失敗 |
| media（`MEDIA_SERVICE_HOST:GRPC_PORT`，預設 `media-service:9937`） | `FinalizeAndLink`（`x-internal-service`） | 圖片永久化 | 失敗 → 評論失敗 |
| buyer（`BUYER_SERVICE_HOST:GRPC_PORT`，預設 `127.0.0.1:9941`，指向單體的 legacy gRPC） | `BatchGetBuyerDisplay` | 買家顯示名 / 頭像 | best-effort，失敗只記 log，欄位留空；**已註銷買家回墓碑字串而非查不到**（#2830） |

### 4.4 MQ 發布事件

詳見 §6 與 [36-MQ 與事件總覽](./36-MQ%20與事件總覽（MQ%20%26%20Event%20Catalog）.md)。

| Event | Exchange | Routing Key | 觸發 | Payload 重點 |
|-------|----------|-------------|------|------|
| `REVIEW_CREATED` | `hawkr.review` | `review.created` | 評論提交成功 | review 完整快照 + seller 聚合（avgRating/totalCount）+ product 聚合（productAvgRating/productReviewCount）+ buyerNickname/Avatar |
| `REVIEW_HIDDEN` | `hawkr.review` | `review.hidden` | 隱藏成功（B-135） | reviewID + sellerAccountID + productID + 重算後 seller / product 聚合 |

### 4.5 錯誤碼

| usecase 錯誤 | `pkgerrors` 類型 | gRPC code / 對外 | 場景 |
|------|------|------|------|
| `ErrAlreadyReviewed` | Conflict | 409 | 已評論過 / unique 撞 |
| `ErrOrderNotFound` | NotFound | 404 | order-service 查不到 / 失敗 |
| `ErrNotEligible` | PreconditionFailed | 412 | 訂單非完成狀態 |
| `ErrNotYourOrder` | Forbidden | 403 | 訂單買家不符 |
| `ErrReviewExpired` | PreconditionFailed | 412 | 超過 30 天 |
| `ErrItemNotInOrder` | InvalidInput | 400 | item 不屬此訂單 |
| `ErrModerationRejected` | Business（`bizcode.ModerationRejected`） | 422 | 內容過濾命中（含 reply 覆蓋時）|
| `ErrReviewNotFound`（#1174） | NotFound | 404 | reply 目標評論不存在 |
| `ErrNotYourReview`（#1174） | Forbidden | 403 | reply 目標評論非該賣家 |
| `ErrReviewNotReplyable`（#1174） | Conflict | 409 | reply 目標評論已隱藏 / 軟刪除，不可回覆 |
| （type 驗證） | InvalidInput | 400 | rating / content / token 數 / price 格式；reply content 空 / 超長 |

> 錯誤碼以 [代碼管理（CodeMeta）/錯誤碼.md](../代碼管理（CodeMeta）/錯誤碼.md) 為唯一真實來源；`bizcode.ModerationRejected` 經 `generate.sh --go-errors` 生成（[Java 微服務開發規範](../開發規範/Java%20微服務開發規範.md) §6 與 [代碼管理（CodeMeta）](../代碼管理（CodeMeta）/README.md)）。

### 4.6 冪等與重試語意

- `SubmitOrderReview`：`order_item_id` unique 保證冪等（重複提交 → 409）
- `HideReview`：WHERE `hidden_at IS NULL` 保證冪等（重複隱藏 → 不更新、不重發 event）
- MQ event 為 best-effort，發布失敗只記 log（見 §8）

---

## 5. 業務流程詳細設計

### 5.1 提交評論（SubmitOrderReview）

實作：gRPC handler `SubmitOrderReview` → `ReviewUseCase.SubmitItemReview`。

```mermaid
flowchart TD
    A[handler 型別驗證<br/>rating/content/token/price] --> B[moderation.Check 內容過濾]
    B -->|命中| Z1[ErrModerationRejected 422]
    B -->|通過| C[order.GetOrderForReview]
    C -->|失敗| Z2[ErrOrderNotFound 404]
    C --> D{狀態=50?}
    D -->|否| Z3[ErrNotEligible 412]
    D -->|是| E{買家相符?}
    E -->|否| Z4[ErrNotYourOrder 403]
    E -->|是| F{完成+30天內?}
    F -->|否| Z5[ErrReviewExpired 412]
    F -->|是| G{item 屬訂單?<br/>取權威 product_id}
    G -->|否| Z6[ErrItemNotInOrder 400]
    G -->|是| H{已評論過?}
    H -->|是| Z7[ErrAlreadyReviewed 409]
    H -->|否| I[生成 reviewID TSID]
    I --> J[逐張 FinalizeAndLink 圖片]
    J -->|失敗| Z8[評論失敗]
    J --> K[Insert order_item_reviews]
    K -->|unique 撞| Z7
    K --> L[best-effort: 查 seller/product 聚合<br/>+ BatchGetBuyerDisplay]
    L -->|查詢成功| M[PublishReviewCreated]
    L -->|查詢失敗| N[跳過發 event 避免清零聚合]
    M --> O[revalidate product+seller cache]
    N --> O
    O --> P[回 review_id]
```

### 5.2 商品評論列表（GetProductReviews）

gRPC handler `GetProductReviews` → `ReviewUseCase.GetProductReviews`。分頁（預設 20，上限 500）+ 星級 / 有圖篩選 + 商品聚合 stats + 批量 buyer meta。`viewer_id != 0` 時於 handler 過濾與 viewer 互相封鎖的作者（D11：stats 不過濾，商品聚合不受個人封鎖影響）。

### 5.3 賣家評論（GetSellerOrderReviews / Stats）

gRPC handler `GetSellerOrderReviews` / `GetSellerOrderReviewStats` → `ReviewUseCase` 同名方法。公開賣家頁與賣家自看共用同 RPC（差別在 Gateway 是否帶 rating 篩選與是否經 JWT）。列表批量填 buyer meta；Stats 回星級分布（`GetSellerStats`，SQL `COUNT FILTER`）。

### 5.4 買家評論歷史（ListMyOrderReviews）

gRPC handler `ListMyOrderReviews` → `ReviewUseCase.ListMyOrderReviews`。僅查 `buyer_id`，不填 buyer meta（買家看自己無需暱稱填充）。隱藏 / 軟刪除排除（`ListBuyerReviews`，#1156 regression 修正）。

### 5.5 隱藏評論（HideReview，B-135）

gRPC handler `HideReview` → `ReviewUseCase.HideReview`。social-service 內部呼叫 → 更新 hidden → 重算 seller/product 聚合 → 發 `review.hidden` → revalidate。聚合查詢失敗則跳過發 event（避免清零，#1156）。

### 5.6 賣家回覆評論（ReplyReview）

Gateway `requireSeller` → `ReplyReview` RPC（`store_id` JWT 注入）→ handler 型別 / 長度驗證 → usecase：`moderation.Check` → `UPDATE ... SET reply_content, replied_at=NOW() WHERE id AND store_id AND hidden_at IS NULL AND deleted_at IS NULL RETURNING *` → RETURNING 有 row 即成功；0 rows 補查 `GetByID` 分流 404 / 403 / 409 。可編輯覆蓋（每次含覆蓋都重跑 moderation）。業務規則、wire 形狀與範圍邊界見 §2.6。**單表寫入、無下游聚合、不發 MQ event、不觸發 revalidate**。

```mermaid
flowchart TD
    A[賣家 POST /app/sellers/reviews/:id/reply] --> B[Gateway requireSeller<br/>accountType 2/3]
    B -->|非賣家| Z0[403 seller access required]
    B -->|賣家| C[ReplyReview gRPC<br/>review_id + store_id JWT 注入]
    C --> D[handler 型別 / 長度驗證]
    D -->|content 空 / 超長| Z1[400 InvalidInput]
    D --> E[moderation.Check content]
    E -->|命中| Z2[422 ErrModerationRejected]
    E -->|通過| F[UPDATE order_item_reviews<br/>SET reply_content, replied_at=NOW()<br/>WHERE id AND store_id<br/>AND hidden_at IS NULL AND deleted_at IS NULL<br/>RETURNING *]
    F -->|RETURNING 有 row| G[mapper 填 reply<br/>回 ReviewResponse]
    F -->|0 rows| H[補查 GetByID<br/>不過濾 hidden 分流]
    H -->|不存在| Z3[404 ErrReviewNotFound]
    H -->|seller 不符| Z4[403 ErrNotYourReview]
    H -->|hidden / deleted| Z5[409 ErrReviewNotReplyable]
    G --> I[不觸發 revalidate]
    I --> J[回更新後 review 含 reply]
```

> ⚠️ **404 / 403 / 409 三態判斷源頭統一**：以 `UPDATE ... RETURNING` 的結果為準（WHERE 含擁有權 + 可見性）。0 rows 時才用一次**不過濾 hidden** 的 `GetByID` 區分「不存在 / 別人的 / 已隱藏不可回覆」。**不**讓不過濾 hidden 的 `GetByID` 成為可回覆性判斷依據。

---

## 6. 訊息佇列設計

| 項目 | 值 |
|------|----|
| Exchange | `hawkr.review`（Topic，`pkg/mq.ExchangeReview`） |
| DLX | `hawkr.review.dlx`（`ExchangeReviewDLX`，consumer 端 framework auto-wire） |
| Routing Key | `review.created`（`RoutingKeyReviewCreated`）/ `review.hidden`（`RoutingKeyReviewHidden`） |
| Publisher | `internal/mq/publisher.go` 薄 wrapper → `pkg/mq.Publisher`（publisher confirms + persistent） |
| 常數 SSOT | `pkg/mq/{events,exchanges}.go`（禁止字面值，[MQ 與事件管理規範](../開發規範/MQ%20與事件管理規範.md)） |

**消費者**（見 36 號 §`review.created` / `review.hidden`）：

| Consumer | Queue | 行為 | 冪等 |
|----------|-------|------|------|
| order-service `ReviewEventConsumer.onReviewCreated` | `order.review.created` | markReviewed + `orders.review` 快照（most-recent-wins） | 天然冪等；❌ 無 DLQ |
| product-service `ReviewStatsConsumer` | （`product.review.stats` + `.dlq`） | 商品 rating / review_count full-recompute（#1156） | full-recompute，✅ 有 DLQ |

### 6.1 業務冪等保證

下游採 **full-recompute 覆寫**（策略 4 變形）：event 攜帶權威重算值，consumer 直接 UPDATE，無 CAS 終態檢查、無 dedup 表，天然冪等。review-service 端保證「聚合查詢失敗則不發 event」，避免把 0 當權威覆寫既有正確聚合（#1156）。

---

## 7. 時序圖總覽

### 7.1 提交評論（含跨服務 + MQ）

```mermaid
sequenceDiagram
    participant B as 買家
    participant GW as Gateway
    participant R as review-service
    participant O as order-service
    participant M as media-service
    participant BU as 單體 buyer 模組
    participant MQ as hawkr.review
    participant DS as order/product consumer

    B->>GW: POST /app/reviews
    GW->>R: SubmitOrderReview(buyer_id 注入)
    R->>R: moderation.Check
    R->>O: GetOrderForReview(orderNo)
    O-->>R: eligibility + items(權威 product_id)
    R->>R: 驗資格(狀態/買家/期限/item/重複)
    R->>M: FinalizeAndLink(staged token×N)
    M-->>R: object key×N
    R->>R: Insert order_item_reviews
    R->>R: 查 seller/product 聚合
    R->>BU: BatchGetBuyerDisplay
    BU-->>R: nickname/avatar
    R->>MQ: PublishReviewCreated
    R-->>GW: review_id
    GW-->>B: 201 {review_id}
    MQ-->>DS: review.created
    DS->>DS: markReviewed / rating 聚合
```

### 7.2 公開賣家評價列表

```mermaid
sequenceDiagram
    participant V as 訪客
    participant GW as Gateway
    participant R as review-service
    participant BU as 單體 buyer 模組

    V->>GW: GET /public/sellers/{id}/reviews
    GW->>R: GetSellerOrderReviews(sellerAccountId)
    R->>R: ListBySellerID(排除 hidden/deleted)
    R->>BU: BatchGetBuyerDisplay
    BU-->>R: nickname/avatar
    R-->>GW: OrderReviewInfo[]（含 product_id）
    GW->>GW: map → PublicSellerReviewItem<br/>(productId, product_id=0 省略)
    GW-->>V: records[] 含 productId
    Note over V: 前端用 productId 連到 /products/{productId}
```

### 7.3 隱藏評論（B-135）

```mermaid
sequenceDiagram
    participant A as Admin
    participant S as social-service
    participant R as review-service
    participant MQ as hawkr.review
    participant DS as product consumer

    A->>S: AdminResolveReport(HIDE_TARGET, REVIEW)
    S->>R: HideReview(reviewId, reason) [internal]
    R->>R: UPDATE hidden_at(idempotent)
    R->>R: 重算 seller/product 聚合
    R->>MQ: PublishReviewHidden
    R-->>S: ok
    MQ-->>DS: review.hidden
    DS->>DS: full-recompute 覆寫聚合
```

### 7.4 賣家回覆評論（ReplyReview）

```mermaid
sequenceDiagram
    participant S as 賣家
    participant GW as Gateway
    participant R as review-service
    participant DB as order_item_reviews
    participant Web as hawkr-website

    S->>GW: POST /app/sellers/reviews/{id}/reply
    GW->>GW: requireSeller(accountType 2/3)
    GW->>R: ReplyReview(review_id, store_id 注入, content)
    R->>R: moderation.Check(content)
    R->>DB: UPDATE reply_content/replied_at<br/>WHERE id AND store_id<br/>AND not hidden AND not deleted RETURNING *
    alt RETURNING 有 row
        DB-->>R: updated review
        R->>R: 不觸發 revalidate（reply 不改聚合）
        R-->>GW: ReviewResponse(含 reply)
        GW-->>S: 200 reply 成功
    else 0 rows
        R->>DB: GetByID(不過濾 hidden) 分流
        R-->>GW: 404 / 403 / 409
        GW-->>S: 對應錯誤
    end
    Note over R,Web: reply 不改聚合、不發 MQ、不通知買家
```

---

## 8. 一致性、交易與補償設計

### 8.1 本地交易邊界

單筆 `Insert` 為單表寫入，無多表事務。圖片 finalize 在 Insert 之前（finalize 失敗則不寫評論）。

### 8.2 跨服務一致性策略

- review → order/product 的聚合同步**完全經 MQ event**，最終一致
- review-service 不持有下游聚合的真實來源，僅在 event payload 攜帶**重算後的權威值**（full-recompute），下游覆寫
- 「查詢聚合失敗則不發 event」保證不以 0 污染下游（#1156）

### 8.3 一致性章節（[後端服務責任邊界規範](../開發規範/後端服務責任邊界規範.md) 必寫）

**單一寫入路徑為唯一真實來源**：`order_item_reviews` 由 `SubmitOrderReview`（新增）、`HideReview`（隱藏）、`ReplyReview`（賣家回覆，#1174）三條 RPC 寫入；下游聚合由 MQ event 驅動，各服務自行落地。`ReplyReview` 為單表附屬欄寫入，**不觸發任何下游聚合 / event**（reply 不改 rating / count / 可見集合）。

**不引入補償任務基礎設施**：review-service **不做** reconciliation job / async reflow runner / backfill CLI / drift detection / sample 比對監測。

- MQ 發布失敗的最終一致由「下一筆評論 / 下一次隱藏自然攜新權威值覆寫」自癒；若需即時修補，由開發者戴維運帽子以 SQL / 一次性腳本處理（[後端服務責任邊界規範](../開發規範/後端服務責任邊界規範.md)），不寫進 application code。
- product_id=0 legacy 資料若需回填，屬一次性 ops SQL，不寫進 codebase（[後端服務責任邊界規範](../開發規範/後端服務責任邊界規範.md)，一次性腳本不留 codebase）。

**例外**：無（本服務無跨服務 saga unwind / Outbox / CDC 需求）。

**圖片欄位**：`order_item_reviews.image_keys` / `item_image_key` 一律存 objectKey，以 objectKey 為唯一真實來源（49 號設計書）。

### 8.4 補償與人工修復

- 聚合漂移（下游與 review 實際不一致）：以 review-service 為真實來源，重發事件或開發者 SQL 重算下游
- 隱藏 / 軟刪除後下游未同步：重放 `review.hidden` 或下游重算

---

## 9. 環境配置

### 9.1 Port / Node ID

| 項目 | 值 |
|------|----|
| gRPC Port | `GRPC_PORT`，預設 `10033` |
| HTTP Port（health/metrics/pprof） | `HTTP_PORT`，預設 `10032` |
| TSID | `pkg/tsid`（Go 服務無 Node ID Base，[Java 微服務開發規範](../開發規範/Java%20微服務開發規範.md) §3.3） |

### 9.2 環境變數

| 變數 | 預設 | 用途 |
|------|------|------|
| `ENV` | `development` | 環境 |
| `DB_HOST/PORT/USER/PASSWORD/NAME/SSLMODE` | localhost:5432 / postgres / hawkr_review / disable | DB（DSN TimeZone=UTC） |
| `GRPC_PORT` / `HTTP_PORT` | 10033 / 10032 | 監聽埠 |
| `ORDER_SERVICE_ADDR` | `order-service:9915` | order client |
| `MEDIA_SERVICE_HOST` / `MEDIA_SERVICE_GRPC_PORT` | `media-service` / `9937` | media client |
| `BUYER_SERVICE_HOST` / `BUYER_SERVICE_GRPC_PORT` | `127.0.0.1` / `9941` | buyer client（單體的 legacy gRPC） |
| `MEDIA_BASE_URL` | `""` | 評論圖 URL 前綴（原 `CDN_BASE_URL`；經 `pkg/mediaurl.BuildURL` 組出 Image Gateway URL，49 號設計書 §8.5） |
| `WEB_REVALIDATE_BASE_URL` / `WEB_REVALIDATE_SECRET` | `""` | SSR cache bust |
| `RABBITMQ_URL` | `amqp://guest:guest@localhost:5672/` | MQ |
| `REDIS_HOST/PORT/PASSWORD` | redis:6379 | 封鎖名單（B-135） |

> Config 載入走 `pkg/config.GetEnvOrDefault` + `Validate()`（[Java 微服務開發規範](../開發規範/Java%20微服務開發規範.md) §12）。fx DI：`config.Module`。

### 9.3 SSR cache bust（後端服務責任邊界 §9.3）

評論建立 / 隱藏改變 rating / reviewCount → 呼叫 `go-pkg/revalidate` BustAsync（product tag + seller tag）通知 hawkr-website 重驗證 SSR 快取。
