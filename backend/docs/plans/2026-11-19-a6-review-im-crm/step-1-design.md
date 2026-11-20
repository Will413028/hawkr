# A6-1 review 模組：實作設計 v2

- 對象：計畫 `../2026-11-19-a6-review-im-crm.md` 的步驟 A6-1。計畫的決定（A6-D4、D6、D7）、預設與不變量是上位規則，本檔只把它們落成模組的形狀；本檔與計畫衝突時，以計畫為準，並在 §9 列出本檔提議的計畫修改。
- 快照：dev `762c28e6`（2026-11-19）。
- 標記：【量】＝打開檔案或跑過指令；【推】＝讀程式後推論。舊行為的出處以 `domain-inventory.md` 的 `file:line` 為準；本檔補查的寫在行內。
- 路徑縮寫：`M=backend/internal`、`RV=backend/services/review-service`、`RVi=RV/internal`。

## 0. 前提題：這一步的程式之後會不會被重寫、搬走或刪除？

模組本體不會；它是終態。過渡物有三個，各自有退場點：訂單資格查詢的 `Orders` port 由 `M/legacy/orderservice` 實作（ordering 搬遷時換 in-process）；`M/legacy/reviewrelay`（A8 notification、catalog、ordering 搬遷時刪）；給 Java catalog 的評分聚合仍由事件全量覆寫（RD8，catalog 搬遷時改成 review 發布 `pub_*`）。

## 0.1 範圍

- 新模組 `M/modules/review`（`make new-module name=review from=review-service`）：schema、提交、回覆、讀取（公開、作者、店家）、統計、隱藏與還原 API、買家刪除的訂閱者、媒體引用同步、快取失效。
- `M/legacy/orderservice` 加 `OrderForReview`（只補 review 用的這一支；`GetOrderForAfterSales`、`HasOrderBetween` 屬 A6-2、A6-3）。
- `M/legacy/reviewrelay`（新）：`review.created`、`review.hidden` 照舊形狀送 `hawkr.review`。
- `M/platform/revalidate` 加 `ProductTag(productID int64)`（`"product:<id>"`，與舊 `pkg/revalidate/tags.go:9` 同字串）。
- ~~換掉 `M/legacy/reviewservice` 的 3 個呼叫端~~：移到 A6-6e（計畫 A6-D11，Will 2026-11-20）；A6-6e 前評價仍寫進舊服務，提早換會讀到空表。
- `CARRYOVER.md`、`AGENTS.md`、`cmd/hawkr` 註冊（`modules.go`、`trimmedColumns`、worker、事件訂閱）。

## 0.2 不在範圍

- Connect 與 handler（A6-5）；moderation 的評價對象（A6-4，本步只提供 `Hide`／`Restore` 根套件 API）；client（A6-6）；刪除舊 review-service 與 gateway 路由（A6-6e／A6-7）。
- 本步沒有對外入口；舊 review-service 照常服務 gateway。relay 只在測試中啟用（計畫不變量「並行期單一生效者」），`cmd/hawkr` 的事件訂閱與 relay worker 在 A6-6e 才接上。
- 特性測試：review 現在沒有 characterization 目錄【量】。骨架與情境在 A6-5 隨 Connect driver 一起建（REST driver 對舊 gateway、Connect driver 對單體，需要 handler 才有意義）。A6-1 到 A6-5 之間，舊行為的依據是 `domain-inventory.md` 與本檔 §3 的舊碼出處；A6-1 刻意改的行為（RD1–RD10）在特性測試裡標成預期差異。

## 0.3 決策總表（都已由計畫、ADR 或單體規則決定，這裡只寫落點）

| # | 題 | 落點 | 出處 |
|---|---|---|---|
| E1 | ID | 評價 `id` UUIDv7；`buyer_id`、`store_id`、`product_id`、`order_item_id` bigint（外部 ID） | 計畫前提「ID 型別」；ADR `2026-09-25-row-identity-by-data-kind` |
| E2 | 公開可讀 | 一個述詞（未刪除、無未還原的隱藏），所有公開讀取與單筆讀取共用；作者與店家的讀取另有入口 | RD10；計畫不變量 |
| E3 | 封鎖過濾 | 觀看者封鎖作者（買家↔買家任一方向）或封鎖店家 → 該觀看者看不到；店家封鎖買家不影響；統計不過濾；過濾在 SQL 述詞內、keyset 之前 | A6-D4；RD1 |
| E4 | 隱藏 | `review.hides` 紀錄表，含還原；只有 review 寫；部分唯一索引保證同一則最多一筆未還原 | RD3；10e R12；照 `social.hides`（`M/modules/social/db/schema.sql:236-261`） |
| E5 | 買家刪帳 | 刪除評價、釋放圖片引用、重算並經 relay 發聚合 | A6-D6；RD4 |
| E6 | 退款 | 照舊：只放行訂單狀態 `"50"` | A6-D7 (a) |
| E7 | 媒體 | 照 social 的 `mediaref`：transaction 外 `Finalize`，transaction 內寫列與排同步 job | `M/modules/social/internal/mediaref/mediaref.go`；單體規則 5 |
| E8 | 分頁 | AIP-158 keyset `(created_at DESC, id DESC)`，不回總數 | 計畫「預設」 |
| E9 | 快取失效 | 寫入同 transaction 排 `revalidate` job（舊碼是 commit 後 best-effort） | `M/modules/social/internal/readcache` |
| E10 | 詞典 | `M/platform/textfilter`（A7 已建，唯一詞典） | 計畫 A6 範圍 |
| E11 | 長度 | 內容與回覆以 rune 計，上限 1000；proto 註明（A6-5） | RD2 |

## 0.4 查證過的現況【量】

- 依賴表 `review → ordering, catalog, buyer, seller, media`（`M/arch/modules.go:28`）；`lockorder.Modules` 已有 `review`（位於 aftersales 之後、im 之前，`M/platform/lockorder/lockorder.go:35-62`）。`M/modules` 沒有 ordering、catalog，所以 review 對它們的需要走根套件的 port，由 `cmd/hawkr` 以 legacy client 實作（前例 `accountdeletion.Orders`）。
- `buyer.IsBlockingStore`（`M/modules/buyer/buyer.go:579`）、`IsBlockedBetween`（`:584`）；`buyer.pub_blocked_buyers()`、`buyer.pub_blocked_stores()`、`buyer.pub_inactive_buyers()` 在 `M/modules/buyer/db/pub.sql`；seller 沒有「店家封鎖買家」的 `pub_*`，A6-D4 也不需要。
- `buyer.BatchDisplay`（`buyer.go:232-235`）對已刪帳買家回 `Deleted=true`、名稱與頭像 nil（舊 buyer-service 回墓碑字串）；墓碑文字由 client 呈現層決定（A6-5／A6-6），模組只回 `Deleted`。
- `buyer.BuyerDeletionRequested`、`buyer.BuyerDeleted`（`M/modules/buyer/events.go:25-28`）；social 兩個都訂（`M/modules/social/social.go:625-638`：沒有開啟中訂單時只發 `BuyerDeleted`）。accountdeletion 不直接呼叫 social，經 buyer 的同 transaction 事件（`M/modules/accountdeletion/internal/request/request.go:361-393`）。
- `M/legacy/orderservice` 沒有 `GetOrderForReview` 方法，生成碼有（`M/legacy/gen/order/orderconnect/order.connect.go:561`）；現有方法全部把錯誤包成一般 `fmt.Errorf`，不分 NotFound／Unavailable。Java 在訂單不存在時回 body 內 `code=NOT_FOUND`（`OrderGrpcService.java:1172-1198`）。
- `M/legacy/reviewservice.StoreRatingStats(ctx, []int64≤100) → map[int64]{Count, Average}`，只回有可見評價的店，失敗回錯誤（fail-closed），回應驗證 count≥1、avg∈[0,5]。
- 舊事件：exchange `hawkr.review`，routing key `review.created`／`review.hidden`；所有 ID 是 JSON number；golden 只有 `review.created` 的 key 清單（`backend/tests/contract/golden/event_payload_keys.json:1419-1436`），`review.hidden` 沒有。Java product 的 `ReviewStatsConsumer` 要求 `product_avg_rating`、`product_review_count` 是 JSON number，否則進 DLQ。

## 1. 資料模型（`review/db/schema.sql`，Atlas）

```
review.reviews
  id              UUID PK                      -- UUIDv7（應用層）
  order_no        TEXT NOT NULL  CHECK 1..64
  order_item_id   BIGINT NOT NULL CHECK > 0    UNIQUE
  buyer_id        BIGINT NOT NULL CHECK > 0
  store_id        BIGINT NOT NULL CHECK > 0
  product_id      BIGINT NOT NULL CHECK > 0    -- 去掉 DEFAULT 0 哨兵（M5）
  rating          SMALLINT NOT NULL CHECK 1..5
  content         TEXT NULL  CHECK char_length ≤ 1000 且 trimtext.Check   -- 空＝NULL
  sku_name        TEXT NOT NULL CHECK char_length ≤ 255                  -- 取自訂單（RD6）
  item_image_key  TEXT NULL CHECK (item_image_key <> '')                   -- 取自訂單；訂單項目沒有圖時 NULL，不用空字串哨兵
  price           BIGINT NOT NULL CHECK ≥ 0                                -- 新台幣元，取自訂單（RD6）
  created_at      TIMESTAMPTZ NOT NULL
  reply_content   TEXT NULL CHECK char_length ≤ 1000 且 trimtext.Check
  replied_at      TIMESTAMPTZ NULL
  CHECK ((reply_content IS NULL) = (replied_at IS NULL))
  INDEX (product_id, created_at DESC, id DESC)、(store_id, created_at DESC, id DESC)、(buyer_id, created_at DESC, id DESC)

review.review_images
  review_id  UUID FK → reviews ON DELETE CASCADE
  position   SMALLINT CHECK 0..3        -- RD7：張數上限由 PK 範圍保證，不靠截斷
  asset_id   TEXT NOT NULL
  object_key TEXT NOT NULL
  PK (review_id, position)

review.hides                               -- 照 social.hides
  id, review_id FK, reason_category（PolicyCategory 子集）, reason_text, source_ref, admin_id,
  hidden_at, restored_at, restored_by, restore_reason；還原一致性 CHECK；
  partial UNIQUE (review_id) WHERE restored_at IS NULL

review.inactive_authors (buyer_id BIGINT PK)   -- 刪帳者的複本，照 social.inactive_authors
review.media_ref_sync / review.retired_assets   -- 照 social mediaref 的同步表
```

- 沒有 `deleted_at`：舊欄位沒有寫入者【量】；A6-D6 是實刪，評價沒有作者自刪的入口。
- 圖片拆表而不是 `TEXT[]`＋`cardinality` CHECK：媒體同步要 asset ID（舊碼只存 object key），而且「最多 4 張」以 `position` 範圍表達後沒有陣列與 asset 兩份清單對齊的問題。
- `order_no`、`sku_name` 等訂單快照只在建立時寫入，之後不變（評價不可編輯，舊行為）。

## 2. 讀取與公開可讀述詞

- 查詢用 sqlc（review 的讀取沒有 social 那種多入口組合，固定 6 支列表／統計查詢足夠；不引入 go-jet）。`sqlc.yaml` 的 schema 輸入加 `../../buyer/db/pub.sql`。
- 公開可讀 = `NOT EXISTS (未還原的 hide)`；所有公開查詢以同一段 SQL 片段開頭，守門測試以 golden 比對每支公開查詢都含它（social 的 `readpolicy` 做法的 sqlc 版）。
- 觀看者過濾（A6-D4）：公開列表多一個 `viewer_buyer_id` 參數，查詢分兩支（匿名、有觀看者），不用 `$v IS NULL OR …` 開關（social 的 generic plan 理由，`visibility.go:1-33`）：
  - `NOT EXISTS (buyer.pub_blocked_buyers() 任一方向 between viewer 與 r.buyer_id)`
  - `NOT EXISTS (buyer.pub_blocked_stores() WHERE blocker = viewer AND store_id = r.store_id)`
- 讀取入口：
  | API | 述詞 | 備註 |
  |---|---|---|
  | `ListProductReviews(product, viewer, filter{rating, withImages}, page)` | 公開＋觀看者 | rating 篩選驗證 1..5 或 0 |
  | `ListStoreReviews(store, viewer, filter, page)` | 公開＋觀看者 | 公開的賣家評價頁 |
  | `ListOwnStoreReviews(store, filter, page)` | 公開（店家自己的後台，不套觀看者） | 舊 `/app/sellers/me/reviews` |
  | `ListMyReviews(buyer, page)` | 作者本人：未隱藏 | 舊行為：隱藏的不給作者（保留；作者看自己被隱藏的評價是 A6-4 的申覆題，不在本步） |
  | `GetReview(id, viewer)` | 公開＋觀看者（RD10） | 作者與店家走上面各自的列表 |
  | `ProductStats(product)`、`StoreStats(store)`、`StoreRatingStats(stores≤100)` | 公開，不套觀看者 | 平均以 `ROUND(AVG, 2)`，同舊碼 |
- 買家名稱與頭像讀時由 httpapi 批次補（`buyer.BatchDisplay`），模組不存快照（M9）。

## 3. 寫入規則

### 3.1 提交 `Submit(ctx, SubmitInput{BuyerID, OrderNo, OrderItemID, Rating, Content, StagedTokens})`

順序（舊碼順序見 `RVi/usecase/review_usecase.go:133-281`；改動處標 ★）：

1. 輸入驗證：rating 1..5；content `trimtext.Trim` 後 ≤1000 rune（★RD2）；tokens ≤4；`price`、`sku_name`、`product_id` 不在輸入裡（★RD6，舊碼收 client 值）。
2. `textfilter` 命中 → `MODERATION_REJECTED`（同舊碼，在查訂單之前）。
3. `Orders.OrderForReview(orderNo)`：NotFound → `NotFound`；其他錯誤 → `Unavailable`（★RD9）。
4. 買家不符 → `NotFound`（與訂單不存在同一個回應，不洩漏別人訂單的存在與狀態；★A6-D10）；狀態不是 `"50"` → `FailedPrecondition`（A6-D7 (a)）。
5. 超過 `completed_at + 30 天` → `FailedPrecondition`（期限）。
6. 項目不在訂單 → `InvalidArgument`；`product_id`、`sku_name`、`price`、`item_image_key` 全取訂單項目（★RD6）。
7. transaction 外 `media.FinalizeList(tokens, RefTypeOrderReview)`。
8. `InTx`：插入評價與圖片列；撞 `UNIQUE(order_item_id)` → `AlreadyExists`（不在同 transaction 繼續下 SQL，單體規則 3）；排媒體同步 job；排 `revalidate`（product tag＋store tags）；`events.Publish(ReviewCreated{EventID, ReviewID, StoreID, ProductID, OccurredAt})`。
- 不再預先 `GetByOrderItemID`（唯一鍵是最終保證，舊碼的先查是多餘的往返）。
- 舊碼「先 link 後插入、失敗不回滾」的缺口由同步 job 消除：插入失敗時沒有任何 link。

### 3.2 回覆 `Reply(ctx, ReplyInput{Store seller.Store, ReviewID, Content})`

- 店家身分收已解析的 `seller.Store`，以 `seller.Authorize` 檢查 `CapMarketing`（舊 gateway 路由的能力，`routes_seller.go:37-38`），停權時 fail-closed（social 的做法）。
- `trimtext.Trim`、空 → `InvalidArgument`；≤1000 rune；`textfilter`。
- `UPDATE … WHERE id AND store_id AND 公開可讀 RETURNING`；0 列時補查分流（不存在 → NotFound；不是這家店 → PermissionDenied；已隱藏 → FailedPrecondition）。舊碼的已刪除分支因沒有 `deleted_at` 而消失。
- 可覆蓋（舊行為）；不發事件、不排失效（舊行為，`review_usecase.go:389-391`）。

### 3.3 隱藏與還原（給 A6-4 的 moderation）

- `Hide(ctx, HideInput{ReviewID, Category, Text, SourceRef, AdminID}) (Hide, error)`：冪等，已有未還原的回 `Created=false`；新隱藏時同 transaction 發 `ReviewAggregateChanged{ReviewID, StoreID, ProductID}` 並排 `revalidate`。
- `Restore(ctx, RestoreInput{ReviewID, AdminID, Reason})`：同上，發同一個事件。
- 舊碼重複隱藏也會重發 `review.hidden`（`review_usecase.go:399-425`）；新碼只在狀態真的改變時發。消費端是全量覆寫、冪等，少發不改變結果【推】。
- 會在 moderation 的 transaction 內被呼叫：只鎖 review 自己的列，順序在 seller 之後（lockorder 已滿足）。

### 3.4 買家刪帳（A6-D6）

- 照 social 的兩段式（`social.go:625-638`、`purge_integration_test.go`）：訂閱 `buyer.BuyerDeletionRequested` 與 `buyer.BuyerDeleted`（兩個都訂：沒有開啟中訂單時只發後者），同 transaction 只做原子的狀態變更：`INSERT INTO review.inactive_authors (buyer_id) ON CONFLICT DO NOTHING` 並排一個 `review.purge_author` job（唯一鍵 buyer_id）。不在訂閱者內刪列：刪帳者的評價筆數沒有上限，而單體規則 4 限定同 transaction 訂閱者只做原子狀態變更。
- 公開可讀述詞加 `NOT EXISTS (review.inactive_authors WHERE buyer_id = r.buyer_id)`，統計同述詞：評價在刪帳申請的那個 transaction 起就看不到、不計分。不讀 `buyer.pub_inactive_buyers()`（`buyer/db/pub.sql:16-18` 限定只給每日對帳）。
- `review.purge_author` job：依 `id` 升序分批刪除該買家的評價（圖片列 CASCADE），每批記退場 asset、排媒體同步 job、排 `revalidate`。可重跑（從現況推導：還有列就刪）。
- 訂閱者在寫入 `inactive_authors` 時只發一個 `ReviewAuthorDeactivated{BuyerID}`（transaction 內工作量固定）；reviewrelay 的 job 在 transaction 外列出該買家評過的 (store, product)，逐組重算並送 `review.hidden`，Java 的聚合在刪帳當下就開始更新，不等 purge（Will 2026-11-19，審查第 9 項）。
- `Submit` 在第 4 步後查 `review.inactive_authors`，有列就 `FailedPrecondition`（刪除中的買家仍有開啟中訂單，之後完成的訂單不能再評）。

## 4. 事件與 relay

### 4.1 模組事件（`review/events.go`）

| 事件 | 時機 | 欄位 |
|---|---|---|
| `ReviewCreated` | 提交成功 | EventID、OccurredAt、ReviewID、StoreID、ProductID |
| `ReviewAggregateChanged` | 隱藏、還原 | EventID、OccurredAt、StoreID、ProductID（只表示「這組聚合要重送」，不指稱單則評價） |
| `ReviewAuthorDeactivated` | 作者進入刪帳（寫入 `inactive_authors`） | EventID、OccurredAt、BuyerID |

### 4.2 `M/legacy/reviewrelay`

- 照 `M/legacy/sellerrelay`：`OnReviewCreated(q)`／`OnReviewAggregateChanged(q)` 在同 transaction 排 job；worker 在 transaction 外組 payload，`rabbitmq.PublishArgs{EventID: 模組事件的 UUIDv7}`。
- `review.created`：讀評價列（已刪除就不送）、重算店與商品統計、`buyer.BatchDisplay` 補暱稱與頭像（刪帳者不帶）、`notify_seller_account_id` 由 `seller.Store` 的帳號 ID（0 表示查不到，舊行為）。
- `review.hidden`：重算後的店與商品聚合（`ReviewAggregateChanged` 觸發）；`review_id` 見 §9 第 2 項。
- 統計在 relay job 執行時讀當下的狀態重算，不在寫入 transaction 內算好存進 job：消費者是全量覆寫，最後一個執行的 job 送的是它執行當下的最新值，所以任何執行順序最後都收斂到正確值；在 transaction 內算好的值則可能因 job 亂序讓舊值最後送達。殘留窗口：兩個 job 並行時，先讀的較晚送達（RD8 接受的範圍，不帶版本）。計畫 RD8 的「重算與 relay 寫入同 tx」改成這段（§9 第 4 項）。
- wire 測試：照 sellerrelay 的 `wire_test.go`，以 `go/ast` 解析舊 `backend/pkg/mq/events.go` 的兩個 struct 逐欄比對 tag、型別、omitempty；golden key 清單照 socialrelay 的 `golden_test.go`，並補 `review.hidden` 的 golden 列。

## 5. 根套件 API（`review/review.go`）

```go
type Orders interface { OrderForReview(ctx, orderNo string) (OrderForReview, error) } // ErrOrderNotFound 與其他錯誤分流
type Deps struct { DB; Bus; Jobs; Media media.Media; Buyer *buyer.Module; Seller *seller.Module; Orders Orders; Now; NewID; Logger }
Submit, Reply, Hide, Restore
ListProductReviews, ListStoreReviews, ListOwnStoreReviews, ListMyReviews, GetReview
ProductStats, StoreStats, StoreRatingStats
OnBuyerDeletionRequested, OnBuyerDeleted
AddWorkers, PeriodicJobs（媒體同步）
```

`StoreRatingStats` 的契約與 `M/legacy/reviewservice` 相同（≤100、只回有可見評價的店、失敗回錯誤），discovery、seller 的 port 直接換實作，不改 port。

## 6. 錯誤原因

| 舊 | 新（`apperr` 分類＋`ErrorReason`，A6-5 加進 proto） |
|---|---|
| `ErrAlreadyReviewed` 409 | AlreadyExists `REVIEW_ALREADY_EXISTS` |
| `ErrOrderNotFound` 404（含暫時故障） | NotFound `ORDER_NOT_FOUND`；暫時故障 Unavailable（RD9） |
| `ErrNotEligible` 428 | FailedPrecondition `ORDER_NOT_REVIEWABLE` |
| `ErrReviewExpired` 428 | FailedPrecondition `REVIEW_WINDOW_EXPIRED` |
| `ErrNotYourOrder`、`ErrNotYourReview` 403 | PermissionDenied |
| `ErrItemNotInOrder` 400 | InvalidArgument（欄位 `order_item_id`） |
| `ErrReviewNotFound` 404 | NotFound |
| `ErrReviewNotReplyable` 409 | FailedPrecondition `REVIEW_NOT_REPLYABLE` |
| 9210 `ModerationRejected` | 與 social 共用的 `MODERATION_REJECTED` |

reason 名稱在 A6-5 以 proto 定案；本步模組只回 `apperr` 分類與 Go 端的 reason 常數。

## 7. 依賴表與註冊

- `M/arch/modules.go` 的 review 列不改（ordering、catalog 以 port 間接使用；arch 只限制 import）。
- `cmd/hawkr/modules.go` 登記 review；`trimmedColumns` 登記 `reviews.content`、`reviews.reply_content`、`hides.reason_text`；事件訂閱與 relay worker 照 §0.2 只在測試接。
- 買家刪除的訂閱者在 `cmd/hawkr` 註冊（它不是 relay，現在就接；舊 review-service 沒有刪帳行為，新模組的表在 A6-6e 前沒有讀者也沒有寫者，接上無副作用）。

## 8. 測試

- integration（`pg.NewDB`，真 Postgres，CI）：每條 RD 一個回歸測試（RD1 觀看者過濾在 keyset 前：封鎖的列不佔頁、不出現；RD2 1000 個中文字可過、1001 不行；RD3 隱藏後還原可見；RD4 刪帳申請後立即不可見、不計分，purge job 刪除列與圖片列並排同步，重跑結果相同；RD6 client 不能影響快照；RD7 第 5 張圖被 CHECK 擋；RD9 orderservice 暫時故障回 Unavailable；RD10 單筆讀取不給隱藏的）。
- 鎖順序：`accountdeletion_lockorder_integration_test.go` 加一個情境：買家刪除時 review 列在 buyer、seller 列之後才鎖（lockorder 檢查器同時強制）。
- relay：wire、golden、`TestRelayRetryEnqueuesOnePublishJobPerEvent` 的 review 版。
- mutation（在複本跑）：每條 RD 改回舊寫法（證據欄的 `file:line`），對應測試要失敗；另外刪掉公開述詞片段，守門測試要失敗。

## 9. 本檔提議的計畫修改（design-review 後交 Will 或直接改）

1. **刪帳的接線**（A6-D9，Will 2026-11-19 選事件訂閱）：計畫不變量寫「accountdeletion 編排者在同 tx 呼叫 review、im 的刪除 API（不走事件）；依賴表 accountdeletion 加 review、im」，A6-1 也寫「`request.Deps` 加 `Review` 介面」。本檔改成訂閱 buyer 的同 transaction 事件：單體規則 4（下層通知上層用 `events.Publish`）、social 的前例（`social.go:625-638`）、accountdeletion 不必知道每個內容模組（依賴表不加 review、im）。鎖順序由 lockorder 檢查器強制，與直接呼叫等價。建議把計畫的不變量與 A6-1、A6-2a 的刪帳條改成事件訂閱。
2. **`review_id`**（已定，依證據）：舊 wire 是 int64，新評價 ID 是 UUIDv7。讀它的只有 notification 的 `review.created` 模板，放進推播 data 的 `reviewId`（`services/notification-service/db/migrations/20260703160000_1176_flipped_events_template_snake.sql:93`）；四端都沒有讀推播的 `reviewId`（`rg -n reviewId buyer-app/lib seller-app/lib admin/src` 只命中評價列表的 REST 欄位，2026-11-19）；Java order、product 不讀。所以兩個事件都送 `review_id: 0`：wire 型別不變，推播 data 的 `reviewId` 變成無意義的 0，沒有讀者。A8 notification 搬進單體時這個欄位隨 relay 刪除。
3. **A6-1 拆成子步驟**（每步一個 session）：
   - A6-1a 本設計書與開工 design-review。
   - A6-1b 模組本體：schema、提交、回覆、讀取、統計、隱藏與還原、`orderservice.OrderForReview`、RD1–RD10 的回歸測試與 mutation、`CARRYOVER.md`。
   - A6-1c 刪帳訂閱者、媒體同步、`revalidate.ProductTag`、`reviewrelay`（wire＋golden）、換掉 `legacy/reviewservice` 的呼叫端並刪除它。
4. **RD8 的文字**：計畫缺陷表 RD8 寫「單體內重算與 relay 寫入同 tx 以縮小窗口」；改成「relay job 執行時重算」（§4.2 的理由：全量覆寫的消費者在 job 時重算才收斂）。

## 10. 開工 design-review 的處理（2026-11-19，8 項）

| # | 嚴重度 | 發現 | 處理 |
|---|---|---|---|
| 1 | should | `Submit` 讀 `buyer.pub_inactive_buyers()`，違反 `buyer/db/pub.sql:16-18` 的用途限制 | 改：review 自己的 `inactive_authors` 複本（§3.4） |
| 2 | should | 訂閱者同 tx 無上限刪除 | 改：訂閱者只寫複本與排 purge job（§3.4） |
| 3 | should | relay 時重算與計畫 RD8「同 tx」矛盾 | 改：保留 job 時重算並寫出收斂理由，計畫 RD8 隨之改（§4.2、§9 第 4 項） |
| 4 | should | `review_id` 送 0 的論證漏了 notification 的 `reviewId` | 改：補證據（四端不讀推播 `reviewId`），結論不變（§9 第 2 項） |
| 5 | should | 刪帳改事件訂閱與計畫不變量衝突 | 交 Will（§9 第 1 項） |
| 6 | nit | `item_image_key` 空字串哨兵 | 改：NULL＋CHECK 非空（§1） |
| 7 | nit | 事件 ReviewID 選填、兩種語意 | 改：`ReviewAggregateChanged` 只帶 (store, product)（§4.1） |
| 8 | nit | 特性測試延後期間舊行為的依據 | 改：§0.2 註明 |

findings 8 (blocking 0)：fixed 7、recorded 0、raised 1、rejected 0。

第 2 輪（2026-11-19，只審修正）：

| # | 嚴重度 | 發現 | 處理 |
|---|---|---|---|
| 9 | should | §3.4 第 4 點在訂閱者 transaction 內對每個 (store, product) 發 `ReviewAggregateChanged`，組數約等於評價數，無上限的工作從刪列搬到發事件（第 2、7 項修正引起；第 2 項因此只修了一半） | 改（Will 2026-11-19 選單一事件，不跑第 3 輪）：§3.4、§4.1 |

累計 findings 9 (blocking 0)：fixed 8、recorded 0、raised 2（A6-D9、第 9 項，Will 皆已決）、rejected 0。
