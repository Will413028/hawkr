# A6-0 盤點（領域、資料、機制切片）：`review-service`、`im-service`、`crm-service` → review／im／crm 模組

唯讀盤點，快照 `94796248`，2026-11-19。路徑相對於 `backend/`，`RV=services/review-service`、`IM=services/im-service`、`CR=services/crm-service`，`X/internal` 簡寫 `RVi`、`IMi`、`CRi`，`G=gateway/internal`，`M=internal/modules`。
決策來源：遷移計畫 `docs/plans/2026-09-24-go-modular-monolith.md` 的 A6 條（第 207–222 行）、`docs/plans/2026-10-18-a7-social/decision-brief.md` §1.2、§4（A6-0 到 A6-7 草案）。A7 的格式前例：`docs/plans/2026-10-18-a7-social/domain-inventory.md`。
標記：【量】＝打開檔案或跑過指令；【推】＝讀程式推論；【猜】＝估計。本檔只盤「領域、資料、機制」；RPC／REST／client 呼叫點是別的切片。

---

## 0. 摘要

- 7 張表（review 1、im 2、crm 4；`rg -c 'CREATE TABLE' X/db/schema/*.sql` = 1／2／4【量】），35 支 migration（7／12／16，`ls X/db/migrations/*.sql | wc -l`【量】），0 個 extension，68 個 sqlc 查詢（15／28／25，`cat X/db/queries/*.sql | grep -c '^-- name:'`【量】）。全部沒有跨表 FK，唯一例外是 `crm_customer_tags.tag_id REFERENCES crm_tags ON DELETE CASCADE`（`CR/db/schema/schema.sql:39`【量】）。
- 去向（建議）：`review` 1 張（`order_item_reviews`）；`im` 2 張（`conversations`、`messages`）；`crm` 4 張（`crm_customers`、`crm_tags`、`crm_customer_tags`、`crm_customer_ledger`），其中 `crm_customers.customer_type = 'blocklist'` 是一個沒人讀的標籤（見 CD1、Q2）。依賴表早已預留三列：`review→ordering,catalog,buyer,seller,media`、`im→ordering,buyer,seller,media`、`crm→ordering,coupon,buyer,identity`（`internal/arch/modules.go:28-30`【量】）；**`crm` 沒有 `seller`、`im` 與 `review` 沒有 `moderation`／`identity`**，Q2、Q6 要動這張表。
- 對外面：review 9 支 RPC、im 13 支（含 `ChatStream`）、crm 14 支，其中 crm 的 `GetSellerStats`、`GetSellerDashboardGrowth` 已 removed（`docs/plans/2026-09-24-go-modular-monolith/rpc-servers.csv:233-234`、`:537-545`、`:260-272`、`:223-236`【量】）。REST（由 `api/openapi/openapi.yaml` 的 path 與 method 計【量】）：評價 9 條 operation（`/app/reviews`、`/app/reviews/{id}`、`/app/me/reviews`、`/app/sellers/me/reviews`、`…/stats`、`/app/sellers/reviews/{id}/reply`、`/public/products/{id}/reviews`、`/public/sellers/{id}/reviews`、`…/stats`）；聊天 9 條（`conversations` get＋patch、`conversations/{targetId}`、`history/{targetId}`、`messages` post＋patch、`unread`、`ws`、`ws-ticket`）；CRM 17 條（`customers*`、`tags*`）。
- 事件：`events.csv` 中 review 當生產者 6 列（`review.created` 4 個消費者、`review.hidden` 2 個）；im 當生產者 1 列（`chat.message.sent`）、當消費者 4 列；crm 當消費者 3 列、不生產（`awk -F, '$3=="review-service"' events.csv | wc -l` = 6；`$3=="im-service"` = 1、`$4=="im-service"` = 4；`$4=="crm-service"` = 3【量】）。
- **最重要的發現**（細節見 §6）：
  1. **平台「隱藏聊天訊息」漏在對話列表**：`HideMessage` 只標 `messages.hidden_at`，讀訊息時用 `TombstoneContent` 遮蔽，但對話列表的 `last_message_content` 與推播預覽不遮蔽，被隱藏的違規內容仍出現在對話列表（ID1）。
  2. **在線狀態訂閱沒有授權**：任何已連線的人送 `presence_sub` 就能訂閱任意 (userID, role) 的上線與最後上線時間（ID2）。
  3. **三條刪除事件走同一個 `RedactDeletedAccount`**：`buyer.deleted` 與 `seller.deleted` 都以 account 為單位，抹除該 account 所有 buyer 與 store 身分寄出的訊息，不限於被刪除的那個身分（ID3，【推】）。
  4. **CRM 的「黑名單」只是標籤**：`customer_type='blocklist'` 全程沒有任何程式讀它做阻擋；寫入端不驗值、也不驗這位買家是不是這間店的客戶（CD1、CD2）。
  5. **評價的封鎖過濾只做在商品評論列表**，賣家評論列表與評分統計不過濾；過濾發生在分頁之後，`total` 不變（RD1）；評價沒有「恢復」路徑、買家刪帳沒有處理（RD3、RD4）。
- **WebSocket 的伺服器端在 gateway**（`G/handler/chat.go` 565 行、`chat_ws_ticket.go`、`chat_ws_connlimit.go`），im-service 只提供 gRPC bidi `ChatStream` 與 in-memory hub；hub 沒有跨 pod 傳遞機制（沒有 Redis pub/sub，`rg 'Subscribe|PSubscribe' IMi` 無結果【量】），單 pod 才正確（ID4，【推】）。
- 沒有 etag：三個服務都沒有 ETag 或 version 欄（`rg -i 'etag|version' X/db/schema` 無結果【量】）；評價回覆「可編輯覆蓋」是 last-write-wins（`RV/db/queries/reviews.sql:104-112`）。
- 沒有 outbox：review 事件直接同步 publish、`_ =` 丟棄錯誤（`RVi/usecase/review_usecase.go:272`、`:408`）；im 事件在 `go` 協程內以 `context.Background()` 發布（`IMi/usecase/message.go` SendMessage 末段）。

---

## 1. 領域與資料

### 1.1 表

來源：`X/db/schema/*.sql`；sqlc 查詢在 `X/db/queries/`，生成碼在 `Xi/repository/postgres/gen/`。

| # | 表（schema） | 主鍵／ID | 值得注意的欄位 | 約束與索引 | 寫入（file:line） | 讀取（file:line） | 去向 |
|---|---|---|---|---|---|---|---|
| 1 | `order_item_reviews`（`RV/db/schema/schema.sql`，18 行） | `id BIGINT`，TSID（`RVi/usecase/review_usecase.go:190`） | `order_no`、`order_item_id`、`buyer_id`、`store_id`、`product_id DEFAULT 0`；`rating`；`content`；`image_keys TEXT[]`；快照 `sku_name`、`item_image_key`、`price`；`deleted_at`（**無寫入者**）；`hidden_at/hidden_reason`；`reply_content/replied_at` | `rating BETWEEN 1 AND 5`；`UNIQUE(order_item_id)`；`(reply_content IS NULL) = (replied_at IS NULL)`；四個索引；**無 content 長度 CHECK、無 image_keys 張數 CHECK** | 新增 `review_repo.go:24`（`reviews.sql` InsertReview，唯一鍵衝突轉 `ErrAlreadyReviewed`）；隱藏 `:184`（`reviews.sql:99-102`，不可還原）；回覆 `:194`（`reviews.sql:104-112`，每次覆蓋 `replied_at`）；`deleted_at` 在非測試、非生成碼中沒有任何 `SET deleted_at`（【量】`rg 'deleted_at *=' RV/db/queries` 無結果） | 商品列表 `:145`／計數 `:160`／統計 `:169`；賣家列表 `:74`／統計 `:96`／批次評分 `:114`；我的評論 `:127`；單筆 `:50`（**不過濾 hidden**，`reviews.sql:86-89`）；除單筆外都帶 `deleted_at IS NULL AND hidden_at IS NULL`（`reviews.sql:17-97`） | **review**。`buyer_id`、`store_id`、`product_id` 是別模組的 ID（單體內 bigint，見 §5）；`product_id DEFAULT 0` 的哨兵拿掉（NOT NULL＋驗證，M5）；`image_keys` 改為對 `media` 的引用（M6）；`deleted_at` 無寫入者就不留；`hidden_*` 改為隱藏紀錄（Q3） |
| 2 | `conversations`（`IM/db/schema/schema.sql`） | `id BIGINT` TSID；邏輯鍵 `room_id VARCHAR(64)`＝`<buyerID>_<sellerID>`（`IMi/usecase/message.go:234-253` `BuildRoomID`／`ParseRoomID`） | `buyer_id`、`store_id`（DEFAULT 0）；`last_message_id/content/type/preview`；`buyer_unread_count`、`seller_unread_count`；`order_no`、`order_binding_verified_at`（停權賣家 gating 用） | `UNIQUE INDEX(room_id)`；索引 `updated_at DESC`、`buyer_id`、`store_id`；**room_id 與 (buyer_id, store_id) 兩份資料無一致性約束** | `UpsertConversation`（`query.sql:1`，由 `message.go:684`）；未讀 +1／重置（`query.sql:40-62`，`message.go:645-650,950-1030`）；`BindOrder`（`query.sql:180`，`message.go:640-644`）；脫敏 `RedactConversationLastMessageBySender`（`query.sql:161`） | 列表 `ListConversationsByBuyer/BySeller`（`query.sql:28,34`）；`GetConversationByRoomID`（`:24`） | **im**。`room_id` 字串拿掉，改 `UNIQUE(buyer_id, store_id)`；未讀數見 Q5；`last_message_*` 是反正規化，見 ID1；`order_no` 綁定語意見 Q1 |
| 3 | `messages`（同檔） | `id BIGINT` TSID；`seq_id BIGINT`（每 room 遞增，由 advisory lock 產生） | `sender_id`、`receiver_id`（**無 kind**，買家用 buyer_id、店家用 store_id，同一欄兩種 ID 空間）；`type`（1 文字／2 圖片，無 CHECK）；`content`（≤4096 CHECK）；`preview_text`；`is_read`；`deleted_at`（脫敏用）、`hidden_at/hidden_reason` | `content` 長度 CHECK；`UNIQUE(room_id, seq_id)`；索引 `room_id`、`(room_id, created_at DESC)`、`sender_id`、`receiver_id`；**`type` 無 CHECK、`room_id` 無 FK** | 建立 `message.go:388-683` 經 `CreateWithLock`（`repository/postgres/message.go:31-`，`pg_advisory_xact_lock(hashtext(room_id))`，`query.sql:64-67`）；已讀 `query.sql:99,114,119,124`；隱藏 `HideMessage`（`query.sql:143-146`，冪等、不可還原）；脫敏 `RedactMessagesBySender`（`query.sql:148`，`deleted_at` 設值＋`content=''`） | 歷史 `ListMessages`（`query.sql:86`，`ORDER BY id DESC LIMIT/OFFSET`）／`ListMessagesBefore`（`:92`）；同步 `ListMessagesAfterSeq`（`:136`）；圖片 id `ListImageMessageIDsBySender`（`:173`） | **im**。`sender`／`receiver` 帶 (kind, id)（`sender_kind TEXT CHECK`）；`type` 加 CHECK；隱藏紀錄見 Q3；`seq_id` 機制見 M3 |
| 4 | `crm_customers`（`CR/db/schema/schema.sql`） | `id BIGINT` TSID | `store_id`、`buyer_id`；`customer_type`（`normal/vip/blocklist`，**無 CHECK**）；`total_spent`、`order_count`（由 ledger 推導的快取）；`nickname`、`phone`（買家個資的快取） | `UNIQUE(store_id, buyer_id)`；索引 `store_id`、`(store_id, customer_type)` | 首單 upsert `UpsertCustomerOnOrder`（`crm_customers.sql:66-82`）；退款 `ApplyRefundDelta`（`:101-106`，無 GREATEST 下限）；改分類 `UpdateCustomerStatus`（`:39-42`，0 列不報錯）；快取回寫 `UpdateCustomerCache`（`:44-`，由 `CRi/usecase/crm.go:184` 的 `SafeGo` 非同步呼叫）；抹除 `RedactCustomerPIIByBuyerID`（`:108-`） | 列表／計數 `crm_customers.sql:6-31`（排序無次序鍵，同值時分頁不穩）；單筆 `:1`；依標籤搜尋 `:87-95`；跨店分類 `GetCustomerTypesByBuyer`（`:97-99`） | **crm**。`total_spent`／`order_count` 是否留表見 M7；`nickname`／`phone` 快取不搬（M9）；`customer_type` 的 blocklist 見 Q2 |
| 5 | `crm_tags` | `id BIGINT` TSID | `store_id`、`name VARCHAR(50)` | 只有 `store_id` 索引；**無 `UNIQUE(store_id, name)`** | `crm_tags.sql:16,21,26`（`CRi/usecase/crm.go:219-243`、`AddCustomerTags` `:256-276` 先 `GetTagByName` 再建） | `:1,6,11,45` | **crm**；加 `UNIQUE(store_id, name)`（CD3） |
| 6 | `crm_customer_tags` | `id BIGSERIAL` | `store_id`、`buyer_id`、`tag_id` | `UNIQUE(store_id,buyer_id,tag_id)`；`tag_id` FK `ON DELETE CASCADE`；索引 `(store_id,buyer_id)`、`tag_id`；**不驗 `tag.store_id = store_id`** | `crm_tags.sql:36,41` | `:30` | **crm**；主鍵改複合自然鍵，去 `BIGSERIAL`；同店約束用複合 FK `(store_id, tag_id)`（CD4） |
| 7 | `crm_customer_ledger` | `event_id VARCHAR(64)`（冪等鍵） | `store_id`、`buyer_id`、`event_type`（`order_completed`／`refund_completed_full`／`refund_completed_partial`）、`order_no`、`amount_delta`、`count_delta`、`occurred_at` | PK＝event_id；索引 `(store_id,buyer_id,occurred_at)`、`order_no`；`event_type` 無 CHECK | `AppendLedgerEntry`（`crm_customer_ledger.sql:1-5`，`ON CONFLICT DO NOTHING`、回 rows）；對帳鎖 `LockOrderForNetting`（`:12-`） | `SumLedgerAmountForOrder`（`:6`）、`HasFullRefundEntry`（`:9`） | **crm**（或取消，見 M7）；無讀取者算 `total_spent` 以外的用途【量】 |

附註：
- `order_item_reviews.image_keys`、`item_image_key`、`messages.content`（type=2 時存 object key）都是 `media` 的引用，現在由 `FinalizeAndLink`／`ReleaseReferences` 管理（M6）。
- 無 legacy 資料需要搬：系統從未上線（根目錄 AGENTS.md「上線前不受相容性約束」），新 baseline schema。

### 1.2 三個服務擁有的資料與不變量（程式強制處）

| 不變量 | 強制處 | 備註 |
|---|---|---|
| 一個訂單項目最多一則評價 | DB `UNIQUE(order_item_id)`；usecase 先查再插（`RVi/usecase/review_usecase.go:181-187`） | DB 為最終保證；競態由唯一鍵錯誤轉 `ErrAlreadyReviewed`（`:227-229`）【量】 |
| 只有已完成（狀態 `"50"`）訂單可評；評價期限 30 天 | `review_usecase.go:144-155`；`RVi/domain/review.go:16-17` | `CompletedAt` 與 `Status` 來自 Java order 的 `GetOrderForReview`（`RVi/client/order/client.go:30`）；退款或售後後仍可評（RD5，【推】） |
| 評價者必須是訂單買家；`order_item_id` 必須屬於該訂單；`product_id` 一律取訂單的權威值 | `review_usecase.go:147-149,157-178,217` | 客戶端傳的 `product_id`、`price`、`sku_name` 只部分被忽略：`price`、`sku_name` 仍取自請求（`:222-223`），不是訂單的值（RD6，【量】） |
| 內容 ≤1000、圖片 ≤4、評分 1–5 | handler `RVi/grpc/handler.go:125-140`；usecase `review_usecase.go:196-198`（超過 4 張**靜默截斷**，不是拒絕）；`domain/review.go:11-15` | 長度用 `len()`＝位元組，不是字元，與錯誤訊息「characters」不符（RD2）【量】 |
| 賣家只能回覆自己店的、未隱藏的評價 | SQL WHERE（`reviews.sql:104-112`）＋0 列補查分流（`review_usecase.go:372-387`） | 回覆不通知買家、不發事件、不 bust 快取（`:390-393` 註解）【量】 |
| 評價統計只算可見評價 | `reviews.sql:31-97` 的 `deleted_at IS NULL AND hidden_at IS NULL`；`HideReview` 後重算並發 `review.hidden`（`review_usecase.go:398-425`） | 統計查詢失敗時不發事件，聚合留舊值（`:238-243,405-412`）【量】 |
| 訊息只能由房間的兩方傳送；買賣不得同一人 | `IMi/usecase/message.go:388-440`（`ParseRoomID`→principal 嚴格解析→`buyerPrincipal==sellerPrincipal` 擋，`:374-440`） | fail-closed：解析不出任一方就拒絕（`:407-425`）【量】 |
| 訊息 ≤4096 字元（rune） | `message.go:393`；DB CHECK `char_length` | 兩處一致【量】 |
| 同一房間的 `seq_id` 嚴格遞增 | `pg_advisory_xact_lock(hashtext(room_id))`（`query.sql:64-67`）＋`UNIQUE(room_id, seq_id)` | 鎖是 32 位元雜湊，碰撞只造成多餘序列化，不破壞正確性【推】 |
| 房間讀取授權（只有房內的人讀得到） | `authorizeRoomAccess`（`message.go:268-285`）用於 WS 的同步幀；**`GetChatHistory` 以請求內的 `user_id`／`target_id` 組房名，不驗證呼叫者**（`IMi/grpc/server.go:211-278`），靠 gateway 注入 | 內部 gRPC 信任 gateway；單體內改為從 session 取（RD7，【推】） |
| 停權店家只能在「訂單關聯」的對話發訊 | `message.go:455-475`（`resolveOrderBinding`，`:707-739`）；旗標來自 gateway 注入 `X-Seller-Suspended`（`G/router/routes_chat.go:24-33`） | 單體內可直接問 seller（M12） |
| 已停業店家不能收訊 | `message.go:477-491`，**查詢失敗時放行**（degraded） | 見 RD8 |
| 帳號刪除後對方不能再發訊給他 | Redis 標記 `im:account:deleted:<id>`（`message.go:1040,1093-1099`，`Set(..., 0)` 無 TTL）＋`Exists` 檢查（`:511-516`） | 註解自承「不是合規不變量」，Redis 錯誤 fail-open（`:507-510`）；單體以 DB 為準（M4） |
| 客戶與店的關係唯一；訂單事件冪等 | `UNIQUE(store_id,buyer_id)`；`crm_customer_ledger.event_id` PK；`LockOrderForNetting` 序列化同單的訂單與退款（`CRi/usecase/crm.go:322-368,396-440`） | 全額退款先到時 `HasFullRefundEntry` 把後到的訂單金額記 0（`:326-334`）【量】 |
| CRM 所有查詢帶 `store_id` | SQL WHERE；gateway 先驗「這位買家對這間店有訂單」才允許改分類、標籤（`G/handler/seller_crm.go:24-50`） | 租戶隔離靠 gateway＋SQL 述詞；服務本身不驗關係（CD2） |

### 1.3 碰到但不擁有的資料

| 資料 | 擁有者 | 三個服務怎麼碰 | A6 後 |
|---|---|---|---|
| 訂單：狀態、完成時間、項目、買家、店家 | ordering（Java order-service，過渡經 `M/legacy/orderservice`） | review 讀 `GetOrderForReview`（`RVi/client/order/client.go:30`）；im 讀 `GetOrderParties`（`IMi/client/order/client.go:49`，實呼叫 `GetOrderForAfterSales`）、`HasOrderBetween`（`:80`）；crm 讀 `GetOrderByNo`（`CRi/client/order/client.go:51`，實呼叫 `GetOrderForAfterSales`） | 計畫「orderservice 補 3 支方法」（`go-modular-monolith.md` A6 條）＝`GetOrderForReview`、`GetOrderForAfterSales`、`HasOrderBetween`（`rpc-calls.csv:755,662-663,81`【量】），目前 `internal/legacy/orderservice` 只有 `HasOpenOrders`、`MerchantSummary` 等，**這三支都不在**（`rg 'GetOrderForReview|HasOrderBetween' internal/legacy` 無結果【量】） |
| 買家名稱、頭像、註銷旗標、principal | buyer | review `BatchGetBuyerDisplayByBuyerIDs`（`RVi/client/buyer/client.go:28`）；im 同批次與 `ResolvePrincipals`、`ListBuyerIDsByAccount`（`IMi/client/buyer/client.go:45,70,127`）；crm `GetBuyerProfileByBuyerID`、`BatchGetBuyerMetaByBuyerIDs`、`GetAddressList`（`CRi/client/buyer/client.go:32,54,78`） | `buyer` 公開 API（依賴表已有）；principal 解析整個消失（單體內 ID 即業務鍵） |
| 店家名稱、logo、store_code、停業旗標、principal | seller | review `BatchGetSellerMeta`、`ResolvePrincipals`（`RVi/client/seller/client.go:59,94`）；im `GetSellerBatch`、`GetSeller`、`ResolvePrincipals`、`ListStoreIDsByAccount`（`IMi/client/seller/client.go:50,148,173,193`） | `seller` 公開 API；`store_code` 只為了組網站快取 tag（M10） |
| 帳號電話、Email | identity（account-service） | crm `GetAccountInfo`／`GetAccountInfoBatch`（`CRi/client/account/client.go:31,48`） | `identity` 公開 API；**賣家看得到買家電話與 Email 要不要保留是產品題**（Q8） |
| 優惠券數 | coupon（Java） | crm `ListBuyerCouponsForSeller`（`CRi/client/coupon/client.go:33`）→ `GetCustomer` 的 `CouponCount`（`CRi/usecase/crm.go:104-111`） | 計畫的「coupon legacy client」；依賴表 crm→coupon 已有 |
| 商品基本資料 | catalog（Java product-service） | review 只存 `product_id`；商品評分聚合由 product-service **消費 `review.created/hidden` 覆寫**（`events.csv`、`replicas.csv:27`） | catalog 是否保留自己的 rating 副本見 Q4 |
| 媒體資產與引用 | media | review `FinalizeAndLink`（`RVi/usecase/review_usecase.go:201`）；im 同（`IMi/usecase/message.go` 圖片分支）＋`ReleaseChatReferences`（`:1087`）；review 沒有 Release | `media` 模組已在單體 |
| 評價隱藏、聊天訊息隱藏 | review、im | social 經 legacy client 呼叫 `HideReview`（`RVi/grpc/handler.go:275`）、`HideMessage`（`IMi/grpc/server.go:386`）（`rpc-calls.csv:792,797`） | `moderation` 加對象種類（Q3；現況見 §5） |
| 賣家端評分 | seller（`sellers.rating`） | seller-service 消費 `review.created/hidden` 並呼叫 `GetSellerOrderReviewStats`（`rpc-calls.csv:770`；`replicas.csv:31`） | seller 模組是否留 rating 副本（Q4）；`M/legacy` 的 `reviewservice.StoreRatingStats` 被 discovery 與 seller 使用（`cmd/hawkr/discovery.go:21-69`、`cmd/hawkr/seller.go:71`），A6 時改接 review 模組 |

---

## 2. 沿用的機制（沿用盤點的原料）

「從零設計」一欄是一行答案；「建議去向」是 A6 的建議落點。

| # | 機制 | 位置與做法 | 從零設計會怎麼做 | 建議去向 |
|---|---|---|---|---|
| M1 | **ID 方案**：評價、對話、訊息、客戶、標籤用 TSID bigint；`crm_customer_tags` 用 `BIGSERIAL`；`messages.id` 同時是游標 | `RVi/usecase/review_usecase.go:45,190`；`IM/db/schema/schema.sql` | Java 不呼叫這三個服務擁有的實體 ID（只有 `order_item_id`、`order_no` 是外部鍵，`idrefs.csv:102-106`），自己的實體可用 UUIDv7（row-identity ADR）；`buyer_id`／`store_id` 維持 bigint 直到最後一個 Java 消費者退場 | 三模組實體 UUIDv7；`crm_customer_tags` 複合自然鍵；訊息游標用 `(created_at, id)` 或 `seq` |
| M2 | **房名 `room_id = <buyerID>_<sellerID>` 字串**，同時是 WS 路由、授權、鎖鍵、對外 ID | `IMi/usecase/message.go:234-253`；授權 `:268-285`；鎖 `query.sql:64-67` | 對話是 `(buyer_id, store_id)` 的唯一列，對外 ID 是對話 UUID；授權由 session 的角色決定，不靠字串解析 | 刪除字串；`conversations` 改 `UNIQUE(buyer_id, store_id)` |
| M3 | **訊息順序**：每房 advisory lock＋`seq_id`；客戶端用 `SyncMessages(lastSeq)` 補洞；WS 送滿 channel 就丟 | `repository/postgres/message.go:31-`；`message.go:995-1006`；`usecase/hub.go:176-181`（`default:` 丟訊息） | 單一 DB 用 `UPDATE conversations SET next_seq = next_seq+1 RETURNING` 取序（鎖在對話列而不是 advisory hash）；補洞以序號游標保留 | im：序號機制改為對話列計數；補洞語意留（Q5 的串流游標） |
| M4 | **Redis 的全部用途**：`block:out:/block:in:`（讀 `pkg/blocklist`，**寫在 social**）；`im:account:deleted:<id>`（寫入無 TTL）；`im:sync:<userID>` 速率限制；gateway 的 WS ticket（`PutWSTicket`，30 秒 TTL，`G/handler/chat_ws_ticket.go:18-22,66-72`）與連線計數（in-memory，`G/handler/chat_ws_connlimit.go:5-41`）。review 與 im 啟動連不上 Redis 只警告、封鎖 fail-open（`RVi/infrastructure/redis.go:18-22`、`IMi/infrastructure/redis.go:20-24`）。**review、crm 沒有其他 Redis key**（`rg 'rdb\.|redis\.' RVi CRi -g '*.go' -g '!*_test.go' -g '!**/gen/**'` 只有 handler.go:43 的 `IsBlocked`【量】） | 封鎖用 buyer／seller 模組的 DB 查詢（A7 已建，見 §5）；刪除標記不需要（同 tx 刪除或標記帳號狀態，查 identity）；速率限制用單體 `httpapi` 既有限流；WS ticket 是 gateway 的事 | 全刪（封鎖→buyer/seller API；刪除標記→查 identity／buyer 的刪除狀態；速率→`httpapi`） |
| M5 | **`product_id DEFAULT 0`、`store_id DEFAULT 0` 哨兵** | `RV/db/schema/schema.sql`（`product_id`）、`IM/db/schema/schema.sql`（`buyer_id`、`store_id`） | NOT NULL 且驗證存在，無哨兵 | review／im 新 schema |
| M6 | **媒體參照**：review 提交時 `FinalizeAndLink`（staged token→object key，ref 用預先產生的 reviewID）；評價**從不釋放**；im 圖片訊息同；脫敏時 `ReleaseChatReferences` | `RVi/usecase/review_usecase.go:192-208`；`IMi/usecase/message.go:525-545`、`:1073-1089` | 單體內與寫列同 tx 呼叫 `media` 模組；隱藏／下架的內容引用保留至保留期後 | review＋im＋media。review 的「不釋放」在刪帳（RD4）與隱藏時要有決定 |
| M7 | **CRM 的 ledger＋快取列雙寫**：`crm_customer_ledger`（insert-only，`event_id` 冪等）＋`crm_customers.total_spent/order_count` 以增量更新，訂單與退款以 `LockOrderForNetting` 序列化，全額退款先到時後到訂單記 0 | `CRi/usecase/crm.go:282-445`；`crm_customer_ledger.sql:1-12` | 已有 `ordering` 資料時，`total_spent`／`order_count` 是訂單的聚合：讀時 `SUM` 或由 `ordering` 發布 `pub_*`；若保留投影，事件進同 tx 訂閱而不是 MQ consumer。**但 order 仍在 Java，這個投影在 A6 時必須是 consumer（見 M8）** | crm：A6 保留 ledger＋投影（資料在 Java），表與事件形狀寫進 CARRYOVER；ordering 搬遷時退場（Q7） |
| M8 | **MQ 消費者**：crm 消費 `order.completed`、`aftersales.refund.completed`、`buyer.deleted`；im 消費 `buyer.deleted`、`seller.deleted`、`account.deleted`、`seller.suspended`；review 沒有消費者 | `CRi/worker/consumer.go:27,131,184`、`CRi/worker/di.go`；`IMi/worker/{buyer,seller,account}_deleted.go`、`seller_suspended.go`、`worker/di.go:15-36` | 單體內 `buyer.deleted`／`seller.deleted`／`account.deleted` 是同 tx 事件訂閱；Java 發的 `order.completed`／`aftersales.refund.completed` 要有 legacy consumer（前例 `M/legacy/buyerconsumer`、`M/legacy/rabbitmq`），ordering 搬進單體後換成單體事件 | crm：legacy consumer（A6-5）；im、crm 的刪除事件→單體事件訂閱 |
| M9 | **個資快取與讀時補名稱**：crm 存 `nickname`／`phone` 並在讀取時回寫；im 的對話列表讀時補名稱頭像；review 讀時補買家名稱＋註銷旗標 | `CRi/usecase/crm.go:118-197`、`crm_customers.sql:44-`；`IMi/usecase/message.go:771-902`；`RVi/usecase/review_usecase.go:319-337` | 只讀時補，不存個資副本（刪帳後 PII 不留在別人的表）；搜尋顧客的需求用 buyer 的搜尋 API | crm 去 `nickname`／`phone` 欄與回寫（Q8）；im、review 維持讀時補 |
| M10 | **網站快取失效（revalidate）**：評價建立／隱藏後對商品與店家 tag `BustAsync`；店家 tag 還要多一次 RPC 解 `store_code` | `RVi/usecase/review_usecase.go:64-115,277-278,424` | `platform/revalidate`（單體已有）；`store_code` 由 seller 公開 API 一次給出 | review，用 `platform/revalidate`；`ResolveStoreCode` 隨 seller API 取代 |
| M11 | **事件發布（fat event，無 outbox）**：`review.created`（含統計、買家暱稱、通知收件 account）、`review.hidden`（含重算後聚合）、`chat.message.sent` | `RVi/mq/publisher.go:30-52`、`RVi/mq/di.go:19-29`（連不上 RabbitMQ 就 nil、事件停用）；`IMi/mq/chat_event_publisher.go`；`message.go` 末段 `go uc.chatPublisher.Publish…(context.Background(), …)` | 單體規則 4、5：下層通知上層用 `events.Publish`（tx 內）；副作用 `jobs.Enqueue`。聚合由資料擁有者發布 `pub_*` 或呼叫 API，不用「全量重算覆寫」事件 | A6-3 relay（前例 `M/legacy/sellerrelay`、`socialrelay`）維持舊形狀給 Java order、product、seller、notification；單體內消費者直接訂閱 |
| M12 | **封鎖過濾（`pkg/blocklist`）**：review 在 handler 層、分頁之後過濾商品評論列表；im 在送訊與列表時 `IsBlocked(sender, receiver)` | `RVi/grpc/handler.go:39-50`；`IMi/usecase/message.go:497-499,809-815` | 單體內 `buyer.IsBlockedBetween`、`buyer.IsBlockingStore`、`seller.StoreBlocksBuyer`（見 §5）；過濾進 SQL 述詞（keyset 之前），避免 filter-after-page | A6-4：換掉 `pkg/blocklist`，刪 Redis 寫入端依賴（Q1、Q6） |
| M13 | **詞典過濾（`pkg/moderation.Check`）**：review 提交與回覆、im 文字訊息；crm 不用 | `RVi/usecase/review_usecase.go:135,365`；`IMi/usecase/message.go:493-496` | 單體只留 `platform/textfilter` 一個詞典（計畫 A6 條）；是否保留預先過濾見 A7 Q12 | `internal/platform/textfilter`；刪 `pkg/moderation` |
| M14 | **principal 解析**：im 在每次送訊嚴格解析雙方 principal（兩次 RPC）；review 解析賣家 principal 才發 notification；im 事件收件人用 principal | `IMi/usecase/message.go:395-441,577-584`；`RVi/usecase/review_usecase.go:268-271` | 單體內業務鍵（buyer_id、store_id）就是身分，事件帶 (kind, id)，由 notification 自己解析收件匣身分（同 A7 M18） | 不搬；A6-3 relay 維持舊事件欄位（含 `notifySellerAccountID`） |
| M15 | **in-memory hub＋presence**：`clients map[userID]…`、presence 訂閱、`lastSeen`、`broadcast` channel（容量 256）；`Run()` 單 goroutine 處理註冊；送滿就丟 | `IMi/usecase/hub.go:11-348`、`IMi/app/app.go:27-33` | 見 Q4（通道選擇與扇出）；presence 要有授權（ID2）；單 pod 假設要寫下來或用 DB／Redis 通知 | im（Q4） |
| M16 | **ChatStream**：gateway→im 的 gRPC bidi，gateway 再轉成 WebSocket；frame 編解碼在 `chat_stream.go`；`SellerSuspended` 事件讓 hub 強制斷線 | `IMi/grpc/chat_stream.go:25-251`；`G/handler/chat.go`；`IMi/worker/seller_suspended.go:25-50` | ADR D7：預設 server streaming 收＋unary 送；瀏覽器要雙向才保留 WebSocket 為第二套契約（Q4） | im（Q4） |
| M17 | **CRM 標籤的「依名稱新增」**：`AddCustomerTags` 逐名查、不存在就建，單筆失敗只記 log 並仍回成功 | `CRi/usecase/crm.go:256-276` | 一個 SQL `INSERT … ON CONFLICT` 建標籤＋關聯，同 tx，錯誤上拋 | crm（CD3、CD5） |
| M18 | **兩套分頁**：page/pageSize＋`COUNT`（review 上限 500、crm、im 對話列表 20/100）；訊息歷史用 `before_id` 游標＋`LIMIT/OFFSET` 的舊 `ListMessages` | `RVi/domain/review.go:18-19`；`IMi/usecase/message.go:741-760`；`crm_customers.sql:6-31` | AIP-158 keyset（`platform/pagination`）；總數是否保留見 Q9 | 三模組 keyset |
| M19 | **一次性回填與 ops**：review 的 `ops backfill` 在 `review_usecase.go:235-237` 註解提及，但倉庫中無對應命令（`rg -i backfill RV` 只命中註解【量】）；crm 無 | `RVi/usecase/review_usecase.go:235-237` | 從未上線，沒有要回填的資料 | 不搬 |

---

## 3. CodeMeta 值域與錯誤碼

用到的 CodeMeta／bizcode（`rg -o --no-filename 'bizcode\.[A-Za-z]+|codemeta\.[A-Za-z]+' X/internal X/db -g '*.go' -g '!*_test.go' -g '!**/gen/**' | sort | uniq -c`，【量】）：review 只有 `bizcode.ModerationRejected`；im 有 6 個 bizcode；crm 無。三個服務**沒有用任何 `codemeta.*` 值域**；值域都是服務自己的字串或魔術數字。

| 值域 | 用到的值 | 儲存處 | 單體對應 |
|---|---|---|---|
| 訂單狀態（review 的資格） | `OrderStatusCompleted = "50"`（`RVi/domain/review.go:17`）；CodeMeta `OrderStatus` | 不存；每次問 Java order | 由 ordering 的公開型別（搬遷時）；A6 過渡只取「已完成」一個值，常數放 review 的 legacy 轉接層 |
| `customer_type` | `normal`／`vip`／`blocklist`（`CRi/domain/customer_types.go:3-10`） | `crm_customers.customer_type`，**無 CHECK**（`CR/db/schema/schema.sql`） | Go 型別＋CHECK；`blocklist` 去留見 Q2 |
| 訊息種類 | 1 文字、2 圖片（`messages.type`，無 CHECK；proto `toProtoMessageType`，`IMi/grpc/server.go`） | `messages.type INTEGER` | `message_type TEXT CHECK`；proto enum |
| 登入角色（買家／賣家） | `accountTypeBuyer=1`、`accountTypeSeller=2`（`IMi/usecase/message.go:256-259`） | 不存 | `sender_kind` TEXT CHECK |
| CRM 事件型別、退款結果 | ledger `event_type` 三值；`FULL_REFUND`／`PARTIAL_REFUND`（`CRi/worker/consumer.go:150-163`） | `crm_customer_ledger.event_type`，無 CHECK | Java 的事件值，legacy consumer 邊界轉成 Go 型別 |

**錯誤碼**（`pkg/errors/bizcode/bizcode.go`、各服務 `usecase`）：

| 碼 | 名稱 | 出處 | 用途 | 單體去向 |
|---|---|---|---|---|
| 9201 | `SelfMessageNotAllowed` | `bizcode.go:175`；`IMi/usecase/message.go:360` | 傳訊息給自己 | `ErrorReason`（im） |
| 9202 | `BlockedMessageNotAllowed` | `bizcode.go:176`；`message.go:497-499` | 被封鎖不能傳訊 | `ErrorReason`；Q1 的例外要定訊息 |
| 9206 | `ReceiverAccountDeleted` | `bizcode.go:182`；`message.go:511-516` | 對方已刪帳 | `ErrorReason` |
| 9207 | `StoreClosedMessageNotAllowed` | `bizcode.go:183`；`message.go:477-491` | 店家已停業 | `ErrorReason`；與 Q1 的「訂單相關聯絡」並存要定順序 |
| 9210 | `ModerationRejected` | `bizcode.go:186`；`message.go:493-496`；`review_usecase.go:30,135` | 詞典命中 | 與 social 共用 `ErrorReason` |
| 9240 | `SellerSuspendedOperationBlocked` | `bizcode.go:202`；`message.go:469-472` | 停權賣家的非訂單訊息 | ADR `2026-09-25-api-error-model.md` D10：改 `permission_denied`＋`SELLER_SUSPENDED` |
| （無碼，`pkgerrors.New*`） | `ErrAlreadyReviewed`、`ErrOrderNotFound`、`ErrNotEligible`、`ErrNotYourOrder`、`ErrReviewExpired`、`ErrItemNotInOrder`、`ErrReviewNotFound`、`ErrNotYourReview`、`ErrReviewNotReplyable` | `RVi/usecase/review_usecase.go:23-35` | 評價的業務錯誤 | 各自 `ErrorReason`；`ErrOrderNotFound` 把「查詢失敗」也吞成「找不到」（`:139-143`，RD9） |

---

## 4. 背景作業、排程、外部 provider

| # | 項目 | 位置 | 做法 | 去向 |
|---|---|---|---|---|
| B1 | crm 消費 `order.completed`（exchange `hawkr.order`，queue `crm.order.completed`） | `CRi/worker/consumer.go:27-80`、`worker/di.go` | 壞 JSON 吞掉（`return nil`，`:30-33`）；ID 解析失敗重試；缺 `order_no`／`event_id` 回錯（會重試到上限）；ID 為 0 吞掉 | legacy consumer；錯誤分類統一（CD6） |
| B2 | crm 消費 `aftersales.refund.completed`（exchange `aftersales.events`） | `consumer.go:131-182`、`:99-129` | 壞資料標 `mq.Permanent`；`GetOrderByNo` 找不到標 permanent；成功路徑要回頭問 Java order | legacy consumer；ordering／aftersales 搬進後換單體事件 |
| B3 | crm、im 消費 `buyer.deleted` | `consumer.go:184-202`；`IMi/worker/buyer_deleted.go:20-34` | crm 抹 `nickname`／`phone`；im 呼叫 `RedactDeletedAccount` | 單體事件訂閱（M8） |
| B4 | im 消費 `seller.deleted`、`account.deleted`、`seller.suspended` | `IMi/worker/seller_deleted.go`、`account_deleted.go`、`seller_suspended.go` | 前兩者與 B3 呼叫同一個函式（ID3）；`seller.suspended` 以 `CurrentStoreID(accountID)` 換 hub key 後 `DisconnectUser` | 單體事件訂閱；停權斷線改 Q4 的通道機制 |
| B5 | review 發布 `review.created`／`review.hidden`（exchange `hawkr.review`） | `RVi/mq/publisher.go`、`RVi/usecase/review_usecase.go:233-275,398-425` | 同步 publish，錯誤丟棄；統計取不到就不發；消費者為 notification、order（`order.review.created`）、product（`product.review.stats`）、seller（`seller.review.created/hidden`） | A6-3 relay（M11）；單體內消費者（seller、catalog 的 rating、notification）改訂閱或讀 pub |
| B6 | im 發布 `chat.message.sent` | `IMi/mq/chat_event_publisher.go`；`message.go` 末段 | `go` 協程＋`context.Background()`，無 deadline 與追蹤；消費者 notification | A6-3 relay；notification（A8）搬遷前維持形狀 |
| B7 | WebSocket／ChatStream | 見 M15、M16；gateway ticket 與連線限制 | in-memory hub；每 user 最多 10 條連線（pod 內計數）；ticket 在 Redis 30 秒 | Q4 |
| B8 | 排程／River | 三個服務**沒有任何排程**（`rg -i 'ticker|cron|time.After|outbox.Scheduler' RVi IMi CRi -g '*.go' -g '!*_test.go' -g '!**/gen/**'` 只命中 hub、client 的逾時【量】） | — | 無；crm 若保留投影不需要 job |
| B9 | 外部 provider | review／im／crm **沒有第三方 provider**（無 PayUni、AMEGO、R2、FCM 呼叫；圖片經 media 服務，推播經 notification 消費事件）【量】（`external-systems.csv:19` 只列 review 的網站 revalidate） | — | 單體規則 5 不適用；revalidate 是 first-party |
| B10 | Redis | 見 M4 | 封鎖讀取、刪帳標記、同步幀速率限制、WS ticket | 全刪或交給 gateway／`httpapi` |

---

## 5. 與單體的耦合

| 對象 | review | im | crm | 單體現況（file:line）與 A6 做法 |
|---|---|---|---|---|
| identity | 無 | 無（只用 principal 解析） | `GetAccountInfo`（電話、Email） | `identity` 公開 API；crm 依賴表已有 identity；im、review 不需要 |
| buyer | 買家名稱頭像／註銷旗標；**封鎖**（Redis） | 名稱頭像、`ListBuyerIDsByAccount`、封鎖 | 檔案、預設地址、`GetBuyerVipStatuses` 的被呼叫端是 buyer-service（`rpc-calls.csv:48`） | `buyer.IsBlockedBetween`（`M/buyer/buyer.go:584`，買家↔買家任一方向）、`buyer.IsBlockingStore`（`:579`，買家封鎖店；註解寫明「給 A6 的 im、review」）【量】 |
| seller | 店家 meta、`store_code` | 店家 profile、停業旗標、`ListStoreIDsByAccount` | 無（Q2 要加） | `seller.StoreBlocksBuyer`（`M/seller/blocks.go:42-45`，店家封鎖買家）、`StoresBlockingBuyer`（`:49-52` 批次）、`ClearBlocksAgainstBuyer`（`:54-58`，刪帳清理）【量】；**`internal/arch/modules.go:30` 的 crm 沒有 seller** |
| moderation | 無（`HideReview` 被 social 經 legacy 呼叫） | 無（`HideMessage` 同） | 無 | `moderation` 的 `TargetKind` 只有 `POST/COMMENT/FORUM_ENTRY/BUYER`（`M/moderation/internal/model/model.go:27-43`，註解「聊天與評價對象在 A6」）。**加聊天與評價對象需要**：`TargetKind` 加兩值＋`cases_target_kind_check` migration；`Target` 結構（`model.go:45-`，內容用 `ContentID`、買家用 `BuyerID`，恰好填一欄）要新增或放寬欄位（對話訊息 ID、評價 ID 是 bigint，而貼文／留言是 UUID，Q3）；`Action.AllowedOn`（`model.go:107-118`）要決定 HIDE／WARN／BAN／TAKEDOWN 各自可用在哪（評價作者是買家，聊天內容的作者可買家或店家）；快照（`internal/snapshot`）要有評價與訊息的快照形狀；`handle` 的處置要呼叫 review／im 的隱藏 API（依賴表 `moderation` 要加 `review`、`im`；現在是 `social, seller, buyer, accountsuspension, media`，`modules.go:37`）；`pub_review_items` 不需改（`moderation/AGENTS.md:13`）。`moderation/CARRYOVER.md:47` 已寫「評價與聊天對象在 A6-4」 |
| social | 無 | 無 | 無 | 舊方向 `social-service → im/review` 已不存在（A7 未寫橋）；`dependency` 上 social 不依賴 review／im |
| notification | 消費 `review.created`（`events.csv`） | 消費 `chat.message.sent` | 無 | notification 依賴 review、im（`modules.go:42` 已列）；A8 搬遷前由 relay 維持形狀 |
| Java order | `GetOrderForReview`；消費 `review.created` 做「已評價」投影 | `GetOrderForAfterSales`、`HasOrderBetween` | `GetOrderForAfterSales`；發 `order.completed`、`aftersales.refund.completed` | `internal/legacy/orderservice` 要補這 3 支（現無，見 §1.3）；review relay 供 Java order 的投影 |
| Java product／catalog | 消費 `review.created/hidden` 覆寫 rating | 無 | 無 | `catalog` 搬進後由 catalog 讀 review 公開 API 或 `pub_*`（依賴表 `review→catalog`、`catalog` 不依賴 review，所以只能是 review 讀 catalog；catalog 的 rating 要由 review 的 pub 或 httpapi 組裝，Q4） |
| Java coupon | 無 | 無 | `ListBuyerCouponsForSeller` | coupon legacy client（計畫 A6 條）；依賴表 crm→coupon 已有 |
| media | `FinalizeAndLink` | 同＋`ReleaseChatReferences` | 無 | `media` 模組已在單體 |
| gateway | 評價 REST 9 條 | 聊天 REST 9 條＋WS | CRM REST 17 條，其中 `lookup`、`meta`、`orders`、`coupons` 是 gateway 組裝（`G/handler/seller_crm.go:84,202,849,933`） | 經單體轉給 gateway（過渡）；搬進後改 Connect |
| 舊 `pkg/` | `pkg/blocklist`、`pkg/moderation`、`pkg/revalidate`、`pkg/mq` | `pkg/blocklist`、`pkg/moderation`、`pkg/ratelimit`、`pkg/mq` | `pkg/mq` | A6 收尾刪 `pkg/blocklist`、`pkg/moderation`（計畫 A6 條；`pkg/blocklist` 的寫入端在 social，已於 A7 退場，【推】現在只剩 im、review 讀取端） |
| 單體內已有的舊轉接 | `M/legacy/reviewservice`（`StoreRatingStats`，`reviewservice.go:44`）被 discovery、seller 使用（`cmd/hawkr/discovery.go:21-69`、`cmd/hawkr/seller.go:71`、`cmd/hawkr/main.go:199,374`） | 無 | `M/legacy/crmservice`（`VIPStatuses`，`crmservice.go:29`）被 buyer 的 `Ports.CRM` 使用（`cmd/hawkr/main.go:195,292`） | 計畫要求刪除這兩個 legacy client（A6 條）；替換對象是 review／crm 模組的公開 API |

---

## 6. 可疑缺陷

編號：RD＝review、ID＝im、CD＝crm。「建議處置」三選一：修＋regression test／接受＋reason／等決定。

### 6.1 review

| # | 缺陷 | 證據 | 建議處置 |
|---|---|---|---|
| RD1 | 封鎖過濾只在商品評論列表，賣家評論列表、我的評論、評分統計不過濾；過濾發生在分頁之後，`total`、`page_size` 仍是未過濾的 | `RVi/grpc/handler.go:39-50`（只有 `GetProductReviews`）；`GetSellerOrderReviews`（`:196-222`）無 `IsBlocked`【量】；`RVi/usecase/review_usecase.go:302-345` 先 list 後 count | 等決定（Q1、Q6：買家對賣家評價的可見性要定模型）；定案後過濾進 SQL 述詞並加 regression test |
| RD2 | 內容與回覆長度用 `len()`（位元組）檢查 1000，訊息卻寫「characters」；一則中文評價最多約 333 字 | `RVi/grpc/handler.go:129`；`review_usecase.go:361`；`domain/review.go:14` | 修＋regression test（改 rune 計，數字由 Q9 的 proto 規則決定；與 im 的 `utf8.RuneCountInString` 一致，`message.go:393`） |
| RD3 | 隱藏後沒有恢復路徑：`HideReview` 條件 `hidden_at IS NULL`，沒有 `RestoreReview`；錯誤隱藏無法更正 | `reviews.sql:99-102`；`rg 'hidden_at *= *NULL' RV/db/queries` 無結果【量】 | 修（隨 Q3 的隱藏紀錄設計，含還原）＋regression test |
| RD4 | 買家刪帳不處理評價：review 沒有 `buyer.deleted` 消費者，評價內容與附圖留存且引用不釋放；讀取時以註銷旗標顯示墓碑名（`#2830`） | `events.csv` 中 `buyer.deleted` 的消費者沒有 review【量】；`RVi/usecase/review_usecase.go:331-333`；`RVi/` 無 `ReleaseReferences` 呼叫【量】 | 等決定（Q10：保留評價作為商品資訊、去識別，或刪除）；屬個資法題，先定政策 |
| RD5 | 先評價、後全額退款的評價仍計入評分；退款後新增評價本來就被擋（只放行狀態 50，退款改成 60／70） | `review_usecase.go:144-155`；`domain/review.go:17`；`OrderStatus.java:24-25`【量，主 session 2026-11-19 更正】 | 接受（A6-D7 (a)），延後到 ordering 搬遷 |
| RD6 | 「product_id 取權威值」只對 `product_id` 成立，`price`、`sku_name` 仍採請求值，賣家頁快照可被買家偽造 | `review_usecase.go:217` vs `:222-223`；`SubmitItemReviewInput`（`:120-130`） | 修＋regression test：快照全取訂單項目（`GetOrderForReview` 回的 `items`） |
| RD7 | 超過 4 張圖靜默截斷，而 handler 已先擋 >4 回 400，usecase 的截斷是死碼且與契約不一致 | `review_usecase.go:196-198` vs `RVi/grpc/handler.go:132-136` | 修（刪除死碼，DB 加張數 CHECK `cardinality(image_keys) <= 4`） |
| RD8 | 統計重算＋事件的競態：兩筆評價並發，各自重算聚合後發事件，消費者「全量覆寫」，後到的舊聚合可蓋過新聚合 | `review_usecase.go:233-275` 的註解自承「ops backfill 自癒」（`:236-237`）；消費者覆寫見 `replicas.csv:27,31` | 等決定（Q4：聚合由誰擁有；單體內改為讀時算或同 tx 更新，relay 階段的 Java 消費者要保證序或帶版本） |
| RD9 | `GetOrderForReview` 任何錯誤都回 `ErrOrderNotFound`（404），暫時性故障被誤報成「訂單不存在」 | `review_usecase.go:139-143` | 修＋regression test（NotFound 與 Unavailable 分流；單體 legacy client 要先解決） |
| RD10 | 單筆 `GetReview`（`GET /app/reviews/{id}`）不過濾 hidden，被平台隱藏的評價仍可由 ID 取回 | `reviews.sql:86-89`；`RVi/grpc/handler.go:72-88` | 修＋regression test（公開可讀述詞一致，除非呼叫者是作者或該店，Q3） |

### 6.2 im

| # | 缺陷 | 證據 | 建議處置 |
|---|---|---|---|
| ID1 | 被隱藏訊息仍出現在對話列表預覽與推播預覽：`HideMessage` 不更新 `conversations.last_message_*`，列表直接輸出 `LastMessageContent`；只有訊息歷史走 `TombstoneContent` | `IMi/usecase/message.go:55-62`（Tombstone）；`IMi/grpc/server.go` `toPbConversation` 直接取 `c.LastMessageContent`；`IMi/grpc/server.go:386-396`、`message.go:1032-1034` 只改 messages【量】 | 修＋regression test（隱藏時若是最後一則就同 tx 更新對話預覽；`preview_text` 也要遮） |
| ID2 | presence 訂閱無授權：任何連線者可 `presence_sub` 任意 (userID, role)，取得上線與 `lastSeen` | `IMi/usecase/client.go:54-61,116-125`；`hub.go:238-250` 沒有「與對方有對話」檢查【量】 | 修＋regression test（僅限已有對話的對方，或整個功能不做，Q4） |
| ID3 | 三條刪除事件走同一個 `RedactDeletedAccount`：以 account 為單位，抹除該 account 所有 buyer 與 store 身分寄出的訊息、並對這些 ID 設 Redis 標記；`buyer.deleted` 若該 account 還有店家身分，店家訊息也被抹除，反之亦然 | `IMi/worker/buyer_deleted.go:20-34`、`seller_deleted.go:14-34`、`account_deleted.go:14-34`；`message.go:1060-1110`（`mergeUniqueIDs(accountID, buyerIDs, storeIDs)`） | 修＋regression test（買家身分刪除只抹 buyer 身分；店家同；帳號刪除才全抹）。【推】一個 account 可同時有買家與賣家身分，未確認是否可只刪其一 |
| ID4 | hub 只在本 pod：`SendToUserRole` 查本地 map，對端連在別的 pod 時訊息只存 DB 不即時送；沒有跨 pod 傳遞 | `IMi/usecase/hub.go:143-186`；無 pub/sub（§0）；`message.go` 的 `hub.SendToUserRole` 後只有 `pushUnreadChanged` | 等決定（Q4：單體單進程時成立，水平擴展前要有通知機制；先把「單 pod」寫入規格） |
| ID5 | 停業守門在 profile 查詢失敗時放行（degraded），且 `else if err != nil` 的寫法讓「查到 profile 但另有 err」不記錄 | `message.go:477-491` | 等決定（Q1：單體內直接查 seller，失敗是錯誤而非降級） |
| ID6 | `GetChatHistory` 以請求的 `user_id`／`target_id` 組房名並「先試 room1 再試 room2」，呼叫者身分由 gateway 注入、服務不驗證；房名兩種組法讓角色模糊時可能讀到另一個身分空間的房 | `IMi/grpc/server.go:211-278`；對照 WS 路徑的 `authorizeRoomAccess`（`message.go:268-285`） | 修＋regression test（單體內從 session 取 (kind, id) 與對話 ID，不接受 `target_id` 猜房） |
| ID7 | `chat.message.sent` 在 `go` 協程內以 `context.Background()` 發布，無 deadline、無錯誤追蹤、關機時可能丟 | `message.go` SendMessage 末段（`go uc.chatPublisher.PublishChatMessageSent(context.Background(), event)`） | 修（單體：`events.Publish` 或 job，A8 前用 relay） |
| ID8 | 脫敏後的圖片：`RedactMessagesBySender` 只清 content（CDN key），OSS 實體靠 `ReleaseChatReferences` 釋放，SQL 註解自承「media-orphan deletion gap」 | `query.sql:148-160` 註解；`message.go:1073-1089` | 接受＋reason：單體內與 `media` 同 tx 解決（M6）；GC 實體刪除在 media 模組 |
| ID9 | `ListMessages` 以 `OFFSET` 與 `ORDER BY id DESC`，`GetChatHistory` 在 `beforeID=0` 時用 offset 0、之後用游標，兩種語意共存；ID 是 TSID 時與 `seq_id` 的順序不一定一致 | `IMi/usecase/message.go:741-760`；`query.sql:86-97` | 修（M18：keyset 以 `seq` 排序） |

### 6.3 crm

| # | 缺陷 | 證據 | 建議處置 |
|---|---|---|---|
| CD1 | `blocklist` 只是標籤，沒有任何阻擋效果：全 repo 非測試碼中讀 `customer_type` 的只有列表篩選、`GetBuyerVipStatuses`（只回 VIP 給 buyer 模組）與 gateway 顯示；店家把某買家設為 blocklist，這位買家仍可下單、聊天、評價 | `CRi/domain/customer_types.go:8-10`；`rg -i 'blocklist' CR gateway/internal -g '*.go' -g '!*_test.go' -g '!**/gen/**' -g '!*.pb.go'` 只命中常數與 swagger 註解【量】 | 等決定（Q2：併入 seller 的店家封鎖；刪 `blocklist` 值與「黑名單」分頁） |
| CD2 | 改分類不驗值也不驗客戶存在：gateway 只 `binding:"required"`，crm 直接存任意字串；SQL `UPDATE` 0 列不報錯，所以對沒有訂單關係的買家改分類回成功但無效；DB 無 CHECK | `G/handler/seller_crm.go:538-576`；`CRi/usecase/crm.go:199-205`；`crm_customers.sql:39-42`；schema 無 CHECK【量】 | 修＋regression test（值域驗證＋0 列回 NotFound；DB 加 CHECK） |
| CD3 | 標籤名稱沒有唯一約束，`AddCustomerTags` 是先查再建，並發會產生同名標籤；`GetTagByName` 的任何錯誤（含 DB 故障）都當成「不存在」而新建 | `CR/db/schema/schema.sql`（`crm_tags` 只有 `store_id` 索引）；`CRi/usecase/crm.go:256-276` | 修＋regression test（`UNIQUE(store_id, name)`、`INSERT … ON CONFLICT`，只對 NotFound 新建） |
| CD4 | `crm_customer_tags` 的 FK 只指 `crm_tags(id)`，資料庫不保證關聯的 `store_id` 與標籤所屬店相同；目前只有 `AddCustomerTags` 以「本店名稱」查標籤再關聯，入口擋得住（【推】沒有以 tag id 直接新增的 RPC，只讀了 `AddCustomerTags` 與 SQL） | `CR/db/schema/schema.sql:39`；`crm_tags.sql:36-44`；`CRi/usecase/crm.go:256-276` | 接受＋reason（入口只有名稱路徑）；新 schema 加複合 FK `(store_id, tag_id)`，不另修 |
| CD5 | `AddCustomerTags` 單筆失敗只記 warn 並仍回成功，整批部分成功無從得知 | `CRi/usecase/crm.go:262-276` | 修＋regression test（同 tx，任一失敗整批回錯） |
| CD6 | 錯誤分類不一致：`order.completed` 壞 JSON 回 `nil`（丟棄），`aftersales.refund.completed` 壞 JSON 回 `Permanent`；缺欄位前者重試、後者 permanent | `CRi/worker/consumer.go:30-33` vs `:134-136` | 修＋regression test（legacy consumer 統一 permanent／retry 分類） |
| CD7 | `total_spent` 可因退款先於訂單、或部分退款重複而變負；`ApplyRefundDelta` 無下限，`order_count` 全額退款不扣 | `crm_customers.sql:101-106`；`CRi/usecase/crm.go:389-445`（`count_delta=0`） | 等決定（Q7：是否保留投影；若保留，補不變量：`total_spent >= 0` 的語意與 `order_count` 是否扣） |
| CD8 | 賣家端 `GetCustomer` 回買家電話與 Email（跨 account-service 即時取），只要存在一筆訂單關係；沒有告知買家、沒有最小化 | `CRi/usecase/crm.go:63-76`；`G/handler/seller_crm.go:24-50` | 等決定（Q8，產品與個資政策） |
| CD9 | 列表排序無次序鍵，`total_spent`／`order_count` 相同時 `LIMIT/OFFSET` 分頁不穩；快取回寫是 `SafeGo` 的 fire-and-forget，讀取請求完成後才寫 | `crm_customers.sql:6-21`；`CRi/usecase/crm.go:176-194` | 修（keyset 加 `(sort_key, id)` 次序鍵；M9 取消快取後回寫自然消失） |
| CD10 | `RecordRefund` 在 tx 之外呼叫 Java order 取 `store_id`／`buyer_id`，order 暫時不可用就整筆重試；`order_not_found` 才 permanent | `CRi/usecase/crm.go:389-392`；`consumer.go:99-129` | 接受＋reason（過渡期事實；ordering 搬進後隨 M7 消失） |

---

## 7. 設計題（範圍沒回答的）

每題格式：前提／業界基準／建議，只列需要 Will 決定或需要 A6-0 計畫定案的項目。

- **Q1 店家封鎖後「訂單相關聯絡」是否例外**（決定簡報 §4 A6 條）。事實：im 目前先查停權（有訂單例外，`message.go:455-475`）、再查停業、**再查封鎖且沒有訂單例外**（`:497-499`）；buyer 的 `IsBlockingStore` 與 seller 的 `StoreBlocksBuyer` 都是單向（見 §5）。前提題：這個例外要不要存在？業界（市集平台）通常封鎖不阻斷「進行中訂單」的溝通。建議：封鎖後只阻斷「無進行中訂單」的新對話，進行中訂單（`HasOrderBetween` 且訂單未結案）仍可通訊，並定義「進行中」＝Java order 的哪些狀態（需要 `GetOrderForAfterSales` 的狀態欄位）。同題決定：買家封鎖店家時，店家是否仍能回覆已存在的對話。
- **Q2 CRM「黑名單」併入 seller 的店家封鎖**（計畫 A6 條）。事實：CD1，`blocklist` 只是標籤；`modules.go:30` crm 沒有 `seller`。建議：刪除 `customer_type` 的 `blocklist` 值，CRM 分類只剩 `normal／vip`（或改成標籤）；「封鎖名單」由 seller 的 `ListBlockedBuyers` 提供，UI 分頁改名「封鎖名單」或拿掉（計畫文字已列兩選項，UI 改動要先取得同意）；若 CRM 畫面要顯示封鎖狀態，crm 依賴 seller 的唯讀 API（要改依賴表，需說明理由：CRM 客戶列表要標示是否被封鎖）。
- **Q3 moderation 加聊天與評價對象**。事實：§5 moderation 列。要定：(a) `Target` 如何容納 bigint 的訊息／評價 ID（貼文、留言是 UUID）：建議 review／im 的實體在新 schema 用 UUIDv7（M1），Target 的 `ContentID` 通用；(b) 隱藏紀錄放哪：建議 review／im 各自有「隱藏紀錄」表（原因、案件 ID、時間、可還原），不再用 `hidden_at/hidden_reason` 兩欄（RD3、ID1）；(c) 聊天的證據快照要含前後文幾則；(d) 評價的作者是買家，聊天內容作者可為買家或店家，`WARN/BAN` 對店家作者走 seller 措施（沿用 A7-D5′）；(e) 對話是私人內容，只有被檢舉的訊息與前後文能進後台，要寫明可見範圍。
- **Q4 IM 的即時通道與評分聚合歸屬**。通道：ADR `2026-09-26-external-contract-protobuf-connect.md` D7（第 43 行）預設 server streaming 收＋unary 送，瀏覽器要雙向才例外保留 WebSocket 並列為第二套契約，並指定「IM 在 A6 決定」；計畫文字說扇出與 A7-D10（留言即時推，A7 決定「不做」，gateway 留言 SSE 刪除）一起設計。建議：IM 是雙人對話，雙向的實質需求只有「送出」，送出用 unary，接收用 server streaming（游標欄位取代 `Last-Event-ID`，沿用 `seq_id` 補洞），不需要 WebSocket；扇出只有「對話雙方的連線」，單進程 in-memory 即可，水平擴展前加 DB `LISTEN/NOTIFY` 或不加（ID4，要寫下單 pod 假設）；presence 若不是產品需求就不做（ID2）。需量測：website 在瀏覽器端實際是否依賴雙向幀（`G/handler/chat.go` 的客戶端幀種類，未量）。聚合：商品與店家評分由誰擁有（RD8），建議 review 擁有、發布 `pub_*`，catalog 與 seller 讀取，不留「全量覆寫」副本；但 A6 期間 Java product 與 seller 的消費者仍在，relay 要維持事件形狀。
- **Q5 IM 的未讀數與已讀模型**。現況：兩欄計數＋`MarkMessagesAsRead`＋`RecalcUnreadForMessages`，三套寫法（`query.sql:40-62,99-135`）。建議：未讀＝`messages.seq > participant.last_read_seq`，單欄游標，不維護計數。
- **Q6 buyer／seller 封鎖在 review 的可見性**（RD1）。建議模型：買家封鎖賣家／買家 → 看不到對方的評價；店家封鎖買家 → 不影響評價的公開顯示（評價是對商品的公共資訊，聚合不受影響，沿用舊 D11 的「統計不過濾」）。需 Will 確認。
- **Q7 CRM 投影去留**（M7、CD7）。前提：ordering 在 Java，A6 時 `order.completed`／退款事件是唯一資料來源。建議：A6 以 legacy consumer 保留 ledger＋投影，並在 CARRYOVER 寫「ordering 搬遷時退場，改由 ordering 的 `pub_*` 讀」；不在 A6 重新設計成讀時聚合（資料不在單體）。
- **Q8 CRM 的買家個資**（CD8、M9）。建議：不存 `nickname`／`phone`，顧客搜尋走 buyer 的搜尋 API；賣家是否看得到買家電話／Email 是產品與個資決定（現況：有訂單關係即可見）。
- **Q9 API 形狀與分頁**：Connect 契約 keyset、總數是否保留（現況 review 上限 500，`domain/review.go:19`）；評價與訊息長度以 rune 計且寫進 proto 註解（RD2）。
- **Q10 買家刪帳後的評價**（RD4）：保留（去識別）或刪除；建議保留為商品資訊、作者顯示墓碑、圖片引用保留，需 Will 確認個資政策。
- **Q11 對話與客戶的建立時機**：現況對話在首則訊息時 upsert（`message.go:684`），客戶在首次訂單完成時 upsert（`UpsertCustomerOnOrder`）；建議維持，但 `store 沒有訂單關係的買家` 不存在於 CRM，Q2 的封鎖因此不依賴 CRM。

---

## 8. 量了什麼、推了什麼、沒涵蓋什麼

**量**（【量】；路徑相對 `backend/`，在 `/Users/will/second-brain/.projects/personal/hawkr/backend` 下執行）：
- `git rev-parse --short HEAD`（`94796248`，在 `services/` 下執行）。
- 表、migration、查詢數：`rg -c 'CREATE TABLE' services/{review,im,crm}-service/db/schema/*.sql`；`ls services/<x>-service/db/migrations/*.sql | wc -l`（7／12／16）；`cat services/<x>-service/db/queries/*.sql | grep -c '^-- name:'`（15／28／25）；`rg -c 'REFERENCES' services/crm-service/db/schema/*.sql`（1）。
- 事件與 RPC：`rg -i '<service>' docs/plans/2026-09-24-go-modular-monolith/events.csv`、`rpc-servers.csv`、`rpc-calls.csv`、`replicas.csv`、`idrefs.csv`、`db-tables.csv`、`external-systems.csv`。
- REST 數量：以 python 解析 `api/openapi/openapi.yaml` 的 path 與 method，篩 `/chat/`、`/reviews`、`/me/reviews`、`/sellers/me/(customers|tags)`（評價 9、聊天 9、CRM 17 條 operation）。
- 型別與用量：`rg -o --no-filename 'bizcode\.[A-Za-z]+|codemeta\.[A-Za-z]+' services/<x>-service/internal services/<x>-service/db -g '*.go' -g '!*_test.go' -g '!**/gen/**' | sort | uniq -c`；`rg -n 'rdb|redis' -i services/{im,review,crm}-service/internal -g '*.go' -g '!*_test.go' -g '!**/gen/**'`；`rg -n 'blocklist' -i services/crm-service gateway/internal -g '*.go' …`；`rg -n 'TombstoneContent|LastMessageContent' services/im-service/internal …`。
- 單體現況：`rg -n 'IsBlocked|Blocked\(' internal/modules/buyer/*.go internal/modules/seller/*.go`；`sed` 讀 `internal/modules/{buyer/buyer.go,seller/blocks.go,moderation/internal/model/model.go,moderation/CARRYOVER.md}`、`internal/arch/modules.go`、`cmd/hawkr/{main,discovery,seller}.go`、`internal/legacy/{reviewservice,crmservice,orderservice}`。
- 逐檔閱讀（全文或主要段落）：`services/review-service/internal/usecase/review_usecase.go`、`grpc/handler.go`、`mq/publisher.go`、`db/queries/reviews.sql`；`services/im-service/internal/usecase/{message,hub}.go`、`grpc/{server,chat_stream}.go`、`worker/*.go`、`db/queries/query.sql`；`services/crm-service/internal/usecase/crm.go`、`worker/consumer.go`、`db/queries/crm_customers.sql`、`crm_tags.sql`、`crm_customer_ledger.sql`；`gateway/internal/router/routes_{chat,crm,review}.go`、`handler/{seller_crm,chat_ws_ticket,chat_ws_connlimit}.go`（只讀片段）。

**推**（【推】，未用指令證實）：ID3（一個 account 可同時有買家與賣家身分且能只刪其一）、ID4（im 部署多 pod）、RD5（`GetOrderForReview` 是否帶退款資訊）、CD4（無以 tag id 直接新增的 RPC，只看了 `AddCustomerTags` 與 SQL）、§5 `pkg/blocklist` 寫入端已隨 A7 退場。

**沒涵蓋**：
- gateway 的 `G/handler/chat.go`（565 行）WS 幀協定與 origin 檢查、`G/handler/review.go`、`order_review.go` 的 REST 組裝與回應形狀（屬 RPC／REST 切片）。
- proto 契約與 Connect 設計（`api/` 尚無 review／im／crm 的 package）；website、admin、兩支 App 對這些 API 的消費端。
- Java order 對 `GetOrderForReview`／`GetOrderForAfterSales`／`HasOrderBetween` 的實際回應欄位、`order.completed` 的 producer 與 `review.created` 的 Java 消費者實作。
- 效能與資料量（對話列表 `COUNT`、`ListMessages` OFFSET、`SearchCustomersByTagIds` 的 GROUP BY）都沒量測。
- notification（A8）、catalog／discovery 對 review 的實際讀取形狀；`services/*/cmd/` 的 ops 子命令與測試檔（`*_test.go`）未讀。
- 沒跑測試、沒起容器、沒讀任何 secret 或 `.env`。
