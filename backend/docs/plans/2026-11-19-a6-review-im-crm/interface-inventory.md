# A6-0 介面盤點：review-service、im-service、crm-service（唯讀，快照 dev `94796248`）

標記：【量】＝跑過指令或打開檔案看過；【推】＝讀程式推論。路徑縮寫：`B=backend`、`G=backend/gateway/internal`、`R=backend/services/review-service/internal`、`I=backend/services/im-service/internal`、`C=backend/services/crm-service/internal`、`M=backend/internal`、`CSV=backend/docs/plans/2026-09-24-go-modular-monolith`。
Connect 方法名是我擬的名字【推】，不是已決定的契約；去向的「種類」沿用 `scripts/check-migration-inventory.py:5-11` 的詞彙（rpc-servers：api／http／removed；events：call／in-tx-event／job／removed）。所有指令從 `backend/` 執行。
兩個前提和 A7 盤點不同：social-service 已刪（`ls B/services` 沒有它），所以 `HideReview`、`HideMessage` 的呼叫端已不存在；gateway 與 openapi.yaml 已沒有 `/app/blocks*`、`/app/reports`（§4.3）。

## 0. 數字與指令（和簡報的差異先講）

| 項目 | 量到 | 簡報 | 指令 |
|---|---|---|---|
| RPC 總數 | **34**（review 9、im 13、crm 12）；CSV 另有 crm 的 `GetSellerStats`、`GetSellerDashboardGrowth` 兩列，已標 `removed`（A3b 步驟 10c 刪除），不計 | 36（含已刪的 2 支） | `rg -c '^\s*rpc ' api/proto/{review,im,crm}/*.proto`（在 `backend/`）＝9／13／12 【量，主 session 2026-11-19 核對】 |
| 非測試 Go 行數 | **9,036**（review 2,168、im 4,269、crm 2,599），含 `cmd/grpc-healthcheck` 與 DI 檔；**不含** sqlc 生成檔。含生成檔是 11,238 | 約 9,000 | `find B/services/<svc> -name '*.go' ! -name '*_test.go' ! -path '*/gen/*' \| xargs cat \| wc -l`；去掉 `! -path '*/gen/*'` 得 2,788／5,040／3,410 【量】。簡報的「約 9,000」成立，口徑是不含生成碼 |
| 測試行數 | 13,692（review 2,962、im 6,383、crm 4,347） | — | 同上，改成 `-name '*_test.go'` 【量】。im 的測試比正式碼還多，characterization 的素材多 |
| 表 | **7**（review 1、im 2、crm 4） | — | `rg -c 'CREATE TABLE' B/services/<svc>/db/schema/schema.sql`：review `order_item_reviews`；im `conversations`、`messages`；crm `crm_customers`、`crm_tags`、`crm_customer_tags`、`crm_customer_ledger`（與 `CSV/db-tables.csv:18,20,22` 的 4／2／1 相符）【量】 |
| 呼叫這三個服務 RPC 的 call site | CSV 30 列：gateway 26 列（review 7＋im 8＋crm 11），其餘 4 列：seller-service 1、buyer-service（已不存在，CSV 過期）1、social-service（已刪）2 | — | `awk -F, '$2~/^(review\|im\|crm)-service$/' CSV/rpc-calls.csv \| wc -l`＝30 列 【量】；gateway 實際另有單體 legacy client 的 3 個呼叫端（§5.3），CSV 沒有收單體的 caller |
| events 列 | **14**（producer 7、consumer 7）；不同 routing key 9 個 | — | `awk -F, 'NR>1 && ($3~/(review\|im\|crm)-service/\|\|$4~/(review\|im\|crm)-service/)' CSV/events.csv \| wc -l`＝14 【量】 |
| 舊 REST operation | **33**（Chat 9、Review 4、Seller tag 的評價 5、SellerCRM 15）；含 1 條 WebSocket | — | `python3 -I` 加 PyYAML 讀 `B/api/openapi/openapi.yaml`，列 tag `Chat`、`Review`、`SellerCRM` 與 path 含 `/reviews` 的 `Seller` operation（全檔現在 379 個 operation，A7-4 後已不是 489）【量】 |
| 其中真的打到這三個服務 | **29**：Chat 8（7 條 unary＋WS 1）、Review 9、CRM 11；另 4 條 CRM 路徑（`lookup`、`meta`、`orders`、`coupons`）打 account／buyer／order／coupon，不碰 crm；`ws-ticket` 只在 gateway＋Redis | — | 見 §4；`rg -n 'h\.(Crm\|IM\|Review\|OrderReview)\.' G/handler --glob '!*_test.go'` 【量】 |
| 舊服務還在跑的 Java 呼叫端 | **0**（Java 沒有任何 gRPC client 呼叫 review／im／crm）；Java 只當**事件消費者**（order、product）與**事件發布者**（order、aftersales） | — | `rg -n 'ReviewServiceGrpc\|IMServiceGrpc\|CrmServiceGrpc' B/services/java --glob '*.java'` 無命中；`rg -l 'review-service\|im-service\|crm-service' B/services/java --glob '!*/target/*'` 只命中註解與 MQ 常數 【量】 |

## 1. RPC（36 支）

呼叫方欄：G＝gateway。「單體」＝`M/` 的 legacy client。Handler 行是這三個服務自己的 gRPC server 方法。去向欄的方法名【推】。

### 1.1 ReviewService（9 支，`R/grpc/handler.go`）

| # | RPC | handler | 呼叫方 file:line | 到它的 REST | 去向（種類: 說明） |
|---|---|---|---|---|---|
| 1 | GetProductReviews | handler.go:23（封鎖過濾 :43） | G `review_service_client.go:49` ← `handler/review.go:262` | `GET /public/products/{id}/reviews` | api: `ReviewPublicService.ListProductReviews`（viewer 可選；封鎖過濾改查 buyer 的 `IsBlockedBetween`，見 §6） |
| 2 | GetReview | handler.go:72 | G `review_service_client.go:64` ← `handler/review.go:306` | `GET /app/reviews/{id}` | api: `ReviewAppService.GetReview` |
| 3 | ReplyReview | handler.go:90 | G `order_review_service_client.go:73` ← `handler/order_review.go:543` | `POST /app/sellers/reviews/{id}/reply` | api: `ReviewAppService.ReplyReview`（店家身分，`CapMarketing`） |
| 4 | SubmitOrderReview | handler.go:122 | G `order_review_service_client.go:48` ← `handler/order_review.go:70` | `POST /app/reviews` | api: `ReviewAppService.SubmitReview`；訂單資格改 in-process 呼叫 ordering 的「為評價讀訂單」，但 ordering 還是 Java（B8），所以暫走 `internal/legacy/orderservice` 補的 `GetOrderForReview`（§3 #1） |
| 5 | ListMyOrderReviews | handler.go:169 | G `order_review_service_client.go:54` ← `handler/order_review.go:126` | `GET /app/me/reviews` | api: `ReviewAppService.ListMyReviews` |
| 6 | GetSellerOrderReviews | handler.go:196 | G `order_review_service_client.go:60` ← `handler/order_review.go:339`（`/me/reviews`）、`:447`（公開店面頁） | `GET /app/sellers/me/reviews`、`GET /public/sellers/{id}/reviews` | api: `ReviewAppService.ListStoreReviews`＋`ReviewPublicService.ListStoreReviews`（兩條 REST 同一支 RPC，簡報要決定是否拆） |
| 7 | GetSellerOrderReviewStats | handler.go:224 | G `order_review_service_client.go:66` ← `handler/order_review.go:400`、`:486`；seller-service `services/seller-service/internal/client/review/client.go:29` ← `grpc/handler.go:133,358`（`GetSellerReviewStats`） | `GET /app/sellers/me/reviews/stats`、`GET /public/sellers/{id}/reviews/stats` | api: `…ListStoreReviewStats`（併入 #8 的統計？見 §7 U3）。seller-service 的呼叫端是舊程式，隨 seller-service 目錄刪除（replicas.csv:31 已標 `removed: seller 不存評分`）【量】 |
| 8 | ListStoreRatingStats | handler.go:247 | 單體 `M/legacy/reviewservice/reviewservice.go:66`（`StoreRatingStats`，上限 `MaxStoreIDs=100` :40）← `cmd/hawkr/discovery.go:50`（discovery 的 `Reviews` port，`:64`）、`:71`（店面頁 `httpseller.Ratings`，`M/httpapi/seller/public.go:39`）、`cmd/hawkr/seller.go:73`（legacy SellerService 的 `Ratings` port） | 無（只在單體內被讀） | in-process api: `review.Module.StoreRatingStats(ctx, storeIDs)`；三個 port 在組裝點改接它，刪 `M/legacy/reviewservice`（簡報 A6-1） |
| 9 | HideReview | handler.go:275（發 `review.hidden` :417） | **無**（唯一呼叫者 social-service 已刪；CSV rpc-calls.csv 的 social→review 那列過期）【量】 | 無 | api: in-process `review.Module.Hide/Restore`，由 moderation 呼叫（A6-4）；沒有「取消隱藏」，舊 RPC 只有 hide（D9 要求還原，要新加） |

### 1.2 IMService（13 支）

| # | RPC | handler | 呼叫方 file:line | 到它的 REST／WS | 去向 |
|---|---|---|---|---|---|
| 10 | SendMessage | `I/grpc/server.go:64`（`message.go` 封鎖 :497、詞典 :519、發事件 :678） | G `im_client.go:72` ← `handler/chat.go:194` | `POST /app/chat/messages` | api: `ChatAppService.SendMessage`（unary） |
| 11 | GetChatHistory | server.go:211 | G `im_client.go:86` ← `chat.go:246` | `GET /app/chat/history/{targetId}` | api: `ChatAppService.ListMessages`（keyset；封鎖過濾 `message.go:809` 改 in-process） |
| 12 | GetConversations | server.go:279 | G `im_client.go:93` ← `chat.go:319` | `GET /app/chat/conversations` | api: `ChatAppService.ListConversations` |
| 13 | GetUnreadCount | server.go:192 | G `im_client.go:100` ← `chat.go:376` | `GET /app/chat/unread` | api: `ChatAppService.GetUnreadCount` |
| 14 | MarkMessagesRead | server.go:313 | G `im_client.go:107` ← `chat.go:417` | `PATCH /app/chat/messages` | api: `ChatAppService.MarkMessagesRead` |
| 15 | MarkConversationRead | server.go:339 | G `im_client.go:114` ← `chat.go:456` | `PATCH /app/chat/conversations/{targetId}` | api: `ChatAppService.MarkConversationRead` |
| 16 | MarkAllRead | server.go:369 | G `im_client.go:121` ← `chat.go:486` | `PATCH /app/chat/conversations` | api: `ChatAppService.MarkAllRead` |
| 17 | ChatStream（雙向 stream） | `I/grpc/chat_stream.go:25` | G `im_client.go:79` ← `handler/chat_grpc.go:227`（WS 轉 gRPC stream） | `GET /app/chat/ws`（WebSocket） | api／http（待 D7 決定）: 見 §4.2 與 §7 U1。ADR D7 預設 server streaming＋unary 送出，例外候選是保留 WebSocket 並明列第二套契約（ADR `2026-09-26-external-contract-protobuf-connect.md:43`） |
| 18 | HideMessage | server.go:386 | **無**（唯一呼叫者 social-service 已刪）【量】 | 無 | api: in-process `chat.Module.HideMessage/Restore`，moderation 呼叫（A6-4） |
| 19 | BroadcastMessage | server.go:107 | **無** | 無 | removed: 沒有呼叫端 |
| 20 | IsUserOnline | server.go:141 | **無** | 無 | removed: 沒有呼叫端 |
| 21 | GetConnectionStats | server.go:160 | **無**（只有 `grpc-healthcheck` 與監控可能讀，我沒找到）【推】 | 無 | removed: 沒有呼叫端；連線數改走 metrics |
| 22 | DisconnectUser | server.go:175 | **無**（hub 內部 `usecase/hub.go:332` 由 `worker/seller_suspended.go:41` 直接用，不經 RPC） | 無 | removed: RPC 無呼叫端；強制斷線改 in-process（停權事件） |

指令：`rg -n 'IsUserOnline|GetConnectionStats|DisconnectUser|BroadcastMessage' B --glob '*.go' --glob '*.java' \| rg -v 'pb\.go|_grpc|/gen/|connect\.go|_test'` 只剩 im-service 自己的定義【量】。
im 有 5 支（#19–22 加 #18）在 CSV 沒有 caller；CSV 的 `disposition` 這三個服務 36 列**全空**，只有 crm 的 2 支寫了 `removed`（`CSV/rpc-servers.csv`，`GetSellerStats`、`GetSellerDashboardGrowth`）【量】。

### 1.3 CrmService（14 支，`C/grpc/handler.go`）

| # | RPC | handler | 呼叫方 file:line | 到它的 REST | 去向 |
|---|---|---|---|---|---|
| 23 | GetCustomerList | handler.go:31 | G `crm_client.go:55` ← `handler/seller_crm.go:456` | `GET /app/sellers/me/customers` | api: `CrmAppService.ListCustomers` |
| 24 | GetCustomerStats | handler.go:76 | G `crm_client.go:71` ← `seller_crm.go:509` | `GET /app/sellers/me/customers/{buyerId}` | api: `CrmAppService.GetCustomer` |
| 25 | UpdateCustomerStatus | handler.go:105 | G `crm_client.go:81` ← `seller_crm.go:557` | `PATCH …/customers/{buyerId}` | api: **併入店家封鎖**（`customerType` 的黑名單值，C10）；VIP 值另決 |
| 26 | CreateTag | handler.go:129 | G `crm_client.go:93` ← `seller_crm.go:622` | `POST …/tags` | api: `CrmAppService.CreateTag` |
| 27 | ListTags | handler.go:148 | G `crm_client.go:103` ← `seller_crm.go:584` | `GET …/tags` | api: `CrmAppService.ListTags` |
| 28 | UpdateTag | handler.go:168 | G `crm_client.go:112` ← `seller_crm.go:667` | `PATCH …/tags/{tagId}` | api: `CrmAppService.UpdateTag` |
| 29 | DeleteTag | handler.go:191 | G `crm_client.go:123` ← `seller_crm.go:703` | `DELETE …/tags/{tagId}` | api: `CrmAppService.DeleteTag` |
| 30 | AddCustomerTags | handler.go:210 | G `crm_client.go:133` ← `seller_crm.go:747` | `POST …/customers/{buyerId}/tags` | api: `CrmAppService.AddCustomerTags` |
| 31 | RemoveCustomerTag | handler.go:233 | G `crm_client.go:144` ← `seller_crm.go:789` | `DELETE …/customers/{buyerId}/tags/{tagId}` | api: `CrmAppService.RemoveCustomerTag` |
| 32 | GetCustomerTags | handler.go:343 | G `crm_client.go:155` ← `seller_crm.go:824` | `GET …/customers/{buyerId}/tags` | api: `CrmAppService.ListCustomerTags` |
| 33 | SearchCustomersByTags | handler.go:260 | G `crm_client.go:167` ← `seller_crm.go:1016` | `POST …/customers/search-by-tags` | api: `CrmAppService.SearchCustomers`（可併入 #23 的篩選，【推】） |
| 34 | GetBuyerVipStatuses | handler.go:311 | 單體 `M/legacy/crmservice/crmservice.go:29`（`VIPStatuses`）← `M/httpapi/buyer/membership.go:90`（port `M/httpapi/buyer/ports.go:63-65`，接線 `cmd/hawkr/main.go:195,292`）；CSV 記的 buyer-service 呼叫端（`services/buyer-service/.../crm_client.go:42`）目錄已不存在 | `GET` 買家的會員清單（httpapi/buyer） | in-process api: `crm.Module.VIPStatuses(ctx, buyerID, storeIDs)`；刪 `M/legacy/crmservice`（簡報 A6-3） |
| 35 | GetSellerStats | handler.go 已無此方法 | 無 | 無 | removed（CSV 已填：A3b 10c 刪除）；proto 殘留 |
| 36 | GetSellerDashboardGrowth | 同上 | 無 | 無 | removed（同上） |

【量】`rg -n '^func \(s \*CrmServer\) [A-Z]' B/services/crm-service/internal/grpc/handler.go` 只有 12 支（#23–34）；14 支裡有 2 支只在 proto 與 CSV。所以 **實作的 RPC 是 34 支，不是 36**；簡報的 36 是 proto／CSV 口徑【量】。

## 2. 事件（14 列逐項驗證）

共通：`B/pkg/mq/exchanges.go:54-55`（review key）、`:99-100,107-119`（queue 名）；拓樸 `B/infra/mq/topology/topology.yaml`（`hawkr.im` :18、`hawkr.review` :42、`crm.*` :151,157,623、`im.*` :171-189、`notification.im` :249、`notification.review` :297、`order.review.created` :434、`product.review.stats` :565）。CSV 的 `disposition` 14 列全空【量】。

| # | routing key（exchange） | producer 發送點 | consumer 點 | payload（wire） | 去向 |
|---|---|---|---|---|---|
| 1 | review.created（hawkr.review）→ notification | `R/usecase/review_usecase.go:272`（包裝 `R/mq/publisher.go:28`） | notification（queue `notification.review`，topology.yaml:297；賣家通知，DB template） | `B/pkg/mq/events.go:99-141`：reviewId、orderNo、orderItemId、storeId、notifySellerAccountId、buyerId、rating、content、imageKeys、createdAt、avgRating、totalCount、productId、productAvgRating、productReviewCount、buyerNickname、buyerAvatar | in-tx-event＋legacy relay 給 notification（前例 `M/legacy/sellerrelay/relay.go`、`socialrelay`），wire 形狀不變；A8 刪 relay |
| 2 | review.created → Java order | 同上 | `services/java/order-service/.../mq/ReviewEventConsumer.java:44`（queue `order.review.created`，`MqConstants.java:590`；topology.yaml:434-440；`markReviewed`＋review snapshot） | 同 #1 | in-tx-event＋relay（review relay 給 Java order，簡報 A6-1）；刪於 B8（order 搬進單體時改 in-process） |
| 3 | review.created → Java product | 同上 | `services/java/product-service/.../mq/ReviewStatsConsumer.java:47`（queue `product.review.stats`；`ReviewStatsService.java:34` 以權威值覆寫 rating／review_count） | 同 #1（用 productAvgRating、productReviewCount） | relay 給 Java product；刪於 B4 |
| 4 | review.created → seller-service | 同上 | `services/seller-service/internal/mq/di.go:48`（queue `seller.review.created`） | 同 #1 | removed: 單體 seller 沒有 rating 欄位，`M/legacy/sellerrelay/relay.go:12` 註解「review.created、review.hidden 的 consumer 單體不新增」，replicas.csv:31 已 removed【量】；舊 seller-service 的 consumer 隨目錄刪 |
| 5 | review.hidden → Java product | `R/usecase/review_usecase.go:417`（`R/mq/publisher.go:43`） | `ReviewStatsConsumer.java:47`（同 queue，綁 review.hidden，topology.yaml:572） | `events.go:143-165`：reviewId、storeId、productId、avgRating、totalCount、productAvgRating、productReviewCount | relay 給 Java product；與 #3 同 |
| 6 | review.hidden → seller-service | 同上 | `seller-service/internal/mq/di.go:57`（queue `seller.review.hidden`） | 同 #5 | removed（同 #4） |
| 7 | chat.message.sent（hawkr.im）→ notification | `I/usecase/message.go:678`（`go` 協程，best-effort；`I/mq/chat_event_publisher.go:29`；exchange 常數 :15、key :19） | notification（queue `notification.im`，topology.yaml:249；離線 FCM＋inbox，template `20260525000001_add_chat_message_templates.sql`） | `I/usecase/message.go:103-114`：message_id、conversation_id、sender_id、sender_name、sender_avatar、receiver_id、receiver_type、store_id、content_preview、sent_at | in-tx-event＋relay 給 notification，A8 刪；注意目前是 fire-and-forget 協程，in-tx 後要改成與寫入同交易 |
| 8 | order.completed（hawkr.order）→ crm | Java `order-service/.../mq/OrderOutboxPublisher.java`（payload `mq/dto/OrderCompletedPayload.java`） | `C/worker/di.go:19-21`（queue `crm.order.completed`，MaxRetries 3）→ `C/worker/consumer.go:27`（`orderCompletedHandler`）→ `usecase/crm.go:348` 的 RecordOrder | storeId／buyerId 為原始值（surrogate，不是 account_id；`OrderOutboxPublisher.java:136`） | 暫留 RabbitMQ consumer（Java 還在發，B8 前不能 in-process）：單體 consumer 先寫 inbox 再 `Enqueue` job（照 `M/legacy/buyerconsumer` 前例，其 `rabbitmq.go:13` 已有同 key 的單體 queue `hawkr.monolith.buyer.order.completed`）；crm 建自己的 `hawkr.monolith.crm.*` queue。B8 改 in-tx-event |
| 9 | aftersales.refund.completed（aftersales.events）→ crm | Java aftersales-service | `C/worker/di.go:26-28`（queue `crm.aftersales.refund.completed`）→ `consumer.go:131`（去重鍵 `refundDedupKey` :104，錯誤分類 :124） | 見 `consumer.go:131-180` | 同 #8；B9 改 in-tx-event |
| 10 | buyer.deleted（hawkr.buyer）→ crm | 單體 relay `M/legacy/buyerrelay/relay.go:307` | `C/worker/di.go:36-38`（queue `crm.buyer.deleted`）→ `consumer.go:184` | `{accountId, buyerId}` | in-tx-event: crm `events.Subscribe(buyer.BuyerDeleted)`，handler 只 `Enqueue` 清除 job（模式同 A7 social #13） |
| 11 | buyer.deleted → im | 同上 | `I/worker/di.go:34-36`（queue `im.buyer.deleted`，`pkg/mq/exchanges.go:109`）→ `worker/buyer_deleted.go` | 同 #10 | in-tx-event，同 #10 |
| 12 | account.deleted（hawkr.account）→ im | 單體 `M/legacy/rabbitmq`（`rabbitmq_test.go:26` 有 `hawkr.account\|account.deleted` 的轉發樣本）【推：實際發送點要對 accountdeletion 模組，未逐行追】 | `I/worker/di.go:41-43`（queue `im.account.deleted`，`exchanges.go:107`）→ `worker/account_deleted.go` | 見 `account_deleted.go` | in-tx-event: 訂閱 accountdeletion 的事件；先確認 buyer.deleted 與 account.deleted 對 im 的清除是否重複（#11、#12 都清聊天資料）【推】 |
| 13 | seller.deleted → im | 單體 relay `M/legacy/sellerrelay/relay.go:228`（`StoreRetired`） | `I/worker/di.go:48-50`（queue `im.seller.deleted`）→ `worker/seller_deleted.go` | `{accountId, store_id}` | in-tx-event: 訂閱 `seller.StoreRetired` |
| 14 | seller.suspended → im（強制斷 WS） | 單體 relay `sellerrelay/relay.go:246`（`StoreSuspended`） | `I/worker/di.go:55-57`（queue `im.seller.suspended`）→ `worker/seller_suspended.go:41`（`hub.DisconnectUser`） | `suspensionId` 等 | in-tx-event: 訂閱 `seller.StoreSuspended`，in-process 斷線（不是持久狀態，是即時副作用；hub 在記憶體，多 pod 時只斷到收到事件那個 pod，這是現狀）【推】 |

**不在 CSV 的通道（搜尋結果）**
- `rg -n 'Publish\(|PSubscribe|Subscribe\(' B/services/{review,im,crm}-service --glob '!*_test.go'`：只有 RabbitMQ，沒有 Redis pub/sub；im 的 hub 是單 pod 記憶體（`I/usecase/hub.go`），gateway 的 WS 連線也是單 pod。多 pod 的跨節點投遞靠什麼，我沒找到，**目前就是單 pod**【推，要和 Will 確認部署是否單副本】。
- review 另有 website 的 on-demand revalidate（`R/usecase/review_usecase.go:68,278,422`，`go-pkg/revalidate`；`CSV/external-systems.csv:19`）：不是事件，是 HTTP 呼叫，去向要看單體現有的 `revalidate` 作法（content、social 的前例）【推】。
- `MqConstants.java:198` `ROUTING_KEY_REVIEW_POSTED = "review.posted"` 與 `common-core/.../ReviewPostedEvent.java` 是 Java 端的舊常數，我沒找到發布者【推；`rg -n 'review.posted' B --glob '!*/target/*'` 可複查】。

## 3. 這三個服務發出的呼叫

預設位址：review 的 order／seller／buyer／media client 在 `R/config/config.go`；im 在 `I/config/config.go`；crm 在 `C/config/config.go`。單體 legacy gRPC `:9941`。

| # | 呼叫 | 發送點 file:line（client → 使用處） | 單體現況 | 去向 |
|---|---|---|---|---|
| 1 | order `GetOrderForReview`（review） | `R/client/order/client.go:30` ← `R/usecase/review_usecase.go:139` | `M/legacy/orderservice` **沒有**這支（現有 7 個方法：`HasOpenOrders`、`MerchantSummary`、`SpendingRecords`、`HasOpenStoreOrders`、`StoreOrderStats`、`StorePeriodOrderStats`、`SellerOrderStatusCounts`，`orderservice.go:55-422`）【量】 | call: 補到 `M/legacy/orderservice`（簡報 A6-1）；B8 改 in-process |
| 2 | order `GetOrderForAfterSales`（im、crm） | im `I/client/order/client.go:53` ← `message.go:720`（`GetOrderParties`）；crm `C/client/order/client.go:51` ← `usecase/crm.go:390`（`GetOrderByNo`） | 沒有 | call: 補到 `orderservice`；兩個服務用同一支但取不同欄位（同一個 RPC 兩種用途，補之前要定介面，見 §7 U4） |
| 3 | order `HasOrderBetween`（im） | `I/client/order/client.go:84` ← `message.go:725,732`（封鎖後的「訂單相關聯絡」例外） | 沒有 | call: 補到 `orderservice`；簡報「orderservice 2 支」＝#2＋#3 對 im、加 review 的 #1＝**3 支**，與主計畫 A6 段「補 3 支」相符，但 crm 的 `GetOrderForAfterSales` 與 im 的是同一支，所以**不同的方法是 3 支**（#1、#2、#3）【量】 |
| 4 | coupon `ListBuyerCouponsForSeller`（crm） | `C/client/coupon/client.go:33` ← `usecase/crm.go:107`（`GetBuyerCouponCount`） | 無單體 coupon client（`ls M/legacy` 沒有 `couponservice`）【量】；gateway 的 `handler/seller_crm.go:951` 也用同一支 | call: 新增 `M/legacy/couponservice`（簡報 A6-3 的「coupon legacy client」）；B7 改 in-process |
| 5 | buyer `GetBuyerProfileByBuyerID`、`BatchGetBuyerMetaByBuyerIDs`、`GetAddressList`（crm） | `C/client/buyer/client.go:54,32,78` ← `usecase/crm.go:57,285,152,470,98` | `M/modules/buyer` 已有模組 API | in-process: `buyer` 公開 API（顯示名稱、頭像、預設地址、電話）；phone 查詢走 buyer 自己維護的聯絡電話（`M/modules/buyer/CARRYOVER.md:33`、`identity/CARRYOVER.md:58`） |
| 6 | account `GetAccountInfo`、`GetAccountInfoBatch`（crm） | `C/client/account/client.go:48,31` ← `usecase/crm.go:78,295` | `M/legacy/accountservice` | removed／in-process: 暱稱與電話改讀 buyer；`crm_customers.nickname/phone` 的 cache 要不要留是沿用盤點題（replicas.csv:33） |
| 7 | buyer `BatchGetBuyerDisplayByBuyerIDs`（review）、`BatchGetBuyerDisplayByBuyerIDs`／`ResolvePrincipals`（im 2 處）／`ListBuyerIDsByAccount`（im） | review `R/client/buyer/client.go:28` ← `review_usecase.go:252,325,463`；im `I/client/buyer/client.go:45,70,127` ← `message.go:415,597,843,914,1061` | buyer 模組 | in-process: `buyer` 批次顯示；`ResolvePrincipals` 的「account→現任 buyer」改用 `CurrentBuyerID`；`ListBuyerIDsByAccount`（含已關閉買家，im 清 PII 要這個語意，`buyer/CARRYOVER.md:96`）要保留或改成清除事件帶 buyerId【推】 |
| 8 | seller `BatchGetSellerMeta`、`ResolvePrincipals`（review）；`GetSeller`、`GetSellerBatch`、`ResolvePrincipals`（2）、`ListStoreIDsByAccount`（im） | review `R/client/seller/client.go:94,59` ← `review_usecase.go:100,270`（`ResolveStoreCode`、`ResolvePrincipal`）；im `I/client/seller/client.go:173,50,148,193` ← `message.go:427,478,607,854,929,1065` | seller 模組 | in-process: `seller` 公開 API（店名、logo、store→account 對應） |
| 9 | media `FinalizeImages`＋`LinkImages`（review、im）；`ReleaseReferences`（im） | review `R/client/media/client.go:34` ← `review_usecase.go:201`；im `I/client/media/client.go:36,50` ← `message.go:527,1089` | `M/modules/media` | in-process: `media.Media`（arch 表要加 `review`、`chat` → `media`，`M/arch/modules.go`）；im 的帳號刪除釋放引用保持 |
| 10 | Redis `pkg/blocklist.IsBlocked`（review、im） | review `R/grpc/handler.go:43`；im `I/usecase/message.go:497,809`（Redis client `I/infrastructure/redis.go`，fail-open） | **A7 起沒有人寫 `block:out:*`／`block:in:*`**（舊 social 已刪；寫入面搬到 buyer／seller 模組）【推：`rg -n 'blocklist.Add' B --glob '*.go' \| rg -v _test` 無命中，已量】 | in-process: `buyer.IsBlockedBetween`、`buyer.IsBlockingStore`、`seller.StoreBlocksBuyer`、`seller.StoresBlockingBuyer`（§6）；刪 `pkg/blocklist` |
| 11 | `pkg/moderation.Check`（詞典過濾，review、im） | review `R/usecase/review_usecase.go:135,365`；im `I/usecase/message.go:519` | 單體有 `internal/platform/textfilter`（社群用） | in-process: `platform/textfilter`；刪 `pkg/moderation`（`blocklist` 與 `moderation` 兩個 pkg 的刪除是 A6-7） |
| 12 | website revalidate（review） | `R/usecase/review_usecase.go:68`（`go-pkg/revalidate`） | 單體的作法（content、social） | call: 沿用單體的 revalidate 機制【推】 |
| 13 | Redis WS ticket（gateway，非 im 自己） | `G/handler/chat_ws_ticket.go:65`（`pkgredis.GenerateSessionToken`）、`G/middleware/chat_ws_auth.go:52`（`ConsumeWSTicket`，GETDEL，Redis 不可用時退 HMAC ticket `chat_ws_ticket_hmac.go`） | gateway 自己 | 隨聊天通道決定（§7 U1）；單體現有的 session 認證是否可直接用於 WS 升級，【推】要驗證 |

## 4. gateway 提供的 REST 與 WebSocket／SSE

### 4.1 計數

指令：`python3 -I` 讀 `B/api/openapi/openapi.yaml`，依 tag 與 path 篩選（§0）；路由註冊：`G/router/routes_chat.go`、`routes_review.go`、`routes_seller.go:24-25,37-39`、`routes_crm.go:19-43`、`G/router/constants.go:61-106,537-562`。

| 區塊 | operation | 打到 | 路由檔 |
|---|---|---|---|
| Chat | 9（7 unary、1 `ws-ticket`、1 WS） | im（8），ws-ticket 只在 gateway＋Redis | `routes_chat.go:15,25-32` |
| Review（tag `Review`） | 4 | review | `routes_review.go:16,26-27,37` |
| 評價（tag `Seller`，path 含 `/reviews`） | 5：`GET /public/sellers/{id}/reviews`、`…/reviews/stats`、`GET /app/sellers/me/reviews`、`…/me/reviews/stats`、`POST /app/sellers/reviews/{id}/reply` | review | `routes_seller.go:24-25,37-39` |
| SellerCRM | 15 | crm 11；`lookup` → `h.Account.GetAccountByPhone`（`seller_crm.go:120`，單體已一律回「找不到」）、`meta` → buyer（:225）、`orders` → order（:863）、`coupons` → coupon（:951） | `routes_crm.go:19-43` |
| 合計 | **33** | **29** 打到這三個服務 | |

**WebSocket 1 條，SSE 0 條**：`GET /api/v1/app/chat/ws`（`routes_chat.go:15`，認證用 ticket，`middleware.ChatWSAuth`；限流清單 `G/middleware/ratelimit.go:103`；連線上限 `handler/chat_ws_connlimit.go`）。review、crm 沒有串流。全檔 SSE 是 `NotificationRealtime`（通知）與 AI，不屬於這三個服務。

### 4.2 聊天通道（ADR D7 的決定題，不在這裡決定）

現況：瀏覽器／App 連 gateway 的 WS → gateway `handler/chat_grpc.go:227` 開 gRPC 雙向 stream 到 im-service `ChatStream`（`I/grpc/chat_stream.go:25`）→ 記憶體 hub（`I/usecase/hub.go`）。即時投遞只在**單一 im pod**。client 兩端（`buyer-app/lib/features/chat/data/chat_repository.dart`、`seller-app/lib/features/chat/data/chat_repository.dart`、website 無聊天）用 WS 加 ticket。【量：`rg -l 'ws-ticket|chat/ws' website/src buyer-app/lib seller-app/lib` 命中兩個 repository 檔】

### 4.3 留在這三個區塊的封鎖與檢舉入口（A7 留下來的）

A7 的決定（Q1）是 A6 之前 im、review 不改。現況【量】：
- `B/api/openapi/openapi.yaml` 與 `G/router` **沒有** `/app/blocks*`、`/app/reports`（`rg -n 'blocks|reports' G/router --glob '!*_test.go'` 無命中；openapi 同）。
- 但 client 還在打：buyer-app `moderation/data/block_repository.dart:20`（`POST /app/blocks/$userId`，只給聊天與評價入口）、`legacy_report_repository.dart:28`（`POST /app/reports`）、`report_target_type.dart`；seller-app `report/data/block_repository.dart:23`、`report/data/report_repository.dart:24`。入口：`chat_thread_screen.dart:939,955`（buyer）、`:1857,1877`（seller）、`merchant_item_detail_screen.dart:2567`（評價檢舉與封鎖評價者）。
- 所以**目前聊天、評價的檢舉與封鎖在 dev 上打到不存在的路由**（A7 計畫 Q1 已接受本機失敗）。A6-4／A6-6 要補：moderation 目標型別、HTTP 入口與四端 client。

## 5. 其他消費者

### 5.1 Java（事件消費者，沒有 gRPC 呼叫）

| 服務 | 消費 | file |
|---|---|---|
| order-service | `review.created` → `order.review.created` queue | `services/java/order-service/.../mq/ReviewEventConsumer.java:44` |
| product-service | `review.created`、`review.hidden` → `product.review.stats` | `services/java/product-service/.../mq/ReviewStatsConsumer.java:47` |
| order-service（發布） | `order.completed`，crm 消費 | `OrderOutboxPublisher.java:136` |
| aftersales-service（發布） | `aftersales.refund.completed`，crm 消費 | CSV events.csv 的 `aftersales.refund.completed` 列 |

### 5.2 notification-service

消費 `chat.message.sent`（queue `notification.im`）、`review.created`（`notification.review`）。template 在 `services/notification-service/db/migrations/`（`20260525000001_add_chat_message_templates.sql`、`20260501000002_add_buyer_info_to_review_created_template.sql`、`20260802120000_review_created_store_id_rekey.sql`、`20260729140000_review_created_notify_field.sql`）。payload 欄位是 template 的契約（`worker/template_contract_test.go`）；relay 必須保持 wire 形狀。

### 5.3 單體的 legacy client（A6 要刪）

| client | 方法 | 呼叫端 |
|---|---|---|
| `M/legacy/reviewservice/reviewservice.go`（91 行；`ListStoreRatingStats`） | `StoreRatingStats` | `cmd/hawkr/main.go:199,374`（建立）；`cmd/hawkr/discovery.go:21,50,62-71`；`cmd/hawkr/seller.go:71-73`；`cmd/hawkr/main.go:260,319`（`sellerRatings`、`legacySellerRatings`） |
| `M/legacy/crmservice/crmservice.go`（55 行；`GetBuyerVipStatuses`） | `VIPStatuses` | `cmd/hawkr/main.go:195,292`（`httpbuyer.Ports{CRM: crm}`）← `M/httpapi/buyer/membership.go:90` |
| 型別生成 | `M/legacy/gen/review`、`M/legacy/gen/crm`（`reviewconnect`、`crmconnect`） | 兩個 client |
| 設定 | `REVIEW_SERVICE_ADDR`、`CRM_SERVICE_ADDR`（`cmd/hawkr/main.go:144-146,339`） | 刪 client 時一起刪；`internal/legacy/gen/im` 不存在，單體沒有 im client【量】 |

另有舊服務之間：seller-service（`services/seller-service/internal/client/review/client.go:29`，`grpc/handler.go:133,358`）呼叫 review 取評分，單體 seller 已沒有 rating，隨 seller-service 目錄刪。

## 6. 單體現況耦合

- **buyer（A7-1b）**：`IsBlockedBetween(buyerA, buyerB)`（`M/modules/buyer/buyer.go:~583`，加入呼叫方 transaction，兩方向）、`IsBlockingStore(buyerID, storeID)`（`:579`）、`BlockBuyer`（`:549`）、`BlockStore`（`:559`）、`ListBlockedBuyers`（`:569`）、`ListBlockedStores`（`:574`）【量】。
- **seller（A7-1b）**：`StoreBlocksBuyer(storeID, buyerID)`（`M/modules/seller/blocks.go:41`）、`StoresBlockingBuyer(buyerID, storeIDs)`（`:47`，批次）、`BlockBuyer`（`:26`）、`ListBlockedBuyers`（`:36`）、`ClearBlocksAgainstBuyer`（`:53`）【量】。review 的「viewer 看不到被封鎖評價者」用 buyer↔buyer；聊天的買賣雙方封鎖用 buyer.`IsBlockingStore`＋seller.`StoreBlocksBuyer`；店家封鎖「訂單相關聯絡的例外」要 `HasOrderBetween`（§3 #3）。
- **moderation 目標型別**：現有 `TargetPost`、`TargetComment`、`TargetForumEntry`、`TargetBuyer`（`M/modules/moderation/moderation.go:85-88`）。**沒有評價、沒有聊天訊息**【量】，所以 A6-4 要新增兩種目標（含快照欄位、隱藏與還原 in-process）。依賴表：`"moderation": {"social","seller","buyer","accountsuspension","media"}`（`M/arch/modules.go:37`）；要加 `review`、`chat` 就得動 arch 表，也要看循環（review、chat 依賴 buyer、seller、media，moderation 依賴 review、chat，無環，【推】）。
- **`reviewqueue` 不是商品評價**：`M/modules/reviewqueue` 是後台待辦佇列（讀 seller、payment、moderation 的 `pub_review_items`，`M/arch/modules.go:38`、`reviewqueue.go`），與 review-service 的買家評價無關；新模組名要避開（建議 `review`＝買家評價，現名 `reviewqueue` 不衝突但容易混，命名要寫進 CARRYOVER）。
- **discovery**：`M/modules/discovery/ports.go:16-20` 的 `Reviews` port 和 `MaxRatingBatch`（＝100）等 review 搬進單體後換成 in-process。
- **platform**：`textfilter` 可取代 `pkg/moderation`；`events`（`events.Publish`／`Subscribe`）、`jobs`、`lockorder`（要加 review、chat、crm 的模組位置，`B/AGENTS.md` 規則 4）。
- **relay 前例**：`M/legacy/buyerrelay`、`sellerrelay`、`socialrelay`（`M/legacy/` 目錄）；crm 的 RabbitMQ consumer 前例是 `M/legacy/buyerconsumer/rabbitmq.go:13`。

## 7. 發現與不確定

**發現**
1. 簡報的 36 RPC 是 proto／CSV 口徑；crm 的 `GetSellerStats`、`GetSellerDashboardGrowth` 在 Go server 已無實作（只在 proto 與 CSV，disposition 已 `removed`），**實作的是 34 支**。
2. 36 支裡**沒有任何呼叫端的有 8 支**：review 的 `HideReview`（#9，social 已刪）、im 的 `HideMessage`、`BroadcastMessage`、`IsUserOnline`、`GetConnectionStats`、`DisconnectUser`（#18–22）、crm 的 2 支已刪。只有 `HideReview`、`HideMessage` 在 A6-4 要變成 in-process 的隱藏與還原，其餘直接 removed。需要真正對外的方法只剩 **review 7＋8（ListStoreRatingStats 在單體內用）、im 7＋stream 1、crm 11＋VIP 1**。
3. 目前 `pkg/blocklist` 沒有寫入者（A7 把寫入搬走了），im、review 的封鎖過濾是**空的 Redis 集合＋fail-open**，即上線前這兩處封鎖不生效，不是 bug 是 Q1 已接受的缺口，但 A6 的 characterization 不能以舊行為當基準（舊行為是失效狀態）【推，依 `rg blocklist.Add` 無命中】。
4. CRM `lookup`（以手機找顧客）目前一律回找不到（`identity/CARRYOVER.md:58,122`），A6-3 要接到 buyer 的聯絡電話（決定 10a）。
5. `chat.message.sent` 用 `go` 協程 best-effort 發（`I/usecase/message.go:678`），改成 in-tx-event 後語意變強（與寫入同交易），要列進沿用盤點。
6. 事件的 Java 消費者都在 B4（product）、B8（order）才消失，所以 review relay 要活到 B8，簡報已寫。
7. 三個 CSV 欄位過期：rpc-calls.csv 的 buyer-service→crm 與 social-service 兩列、`order-service` 的 `GetOrderForAfterSales` 被 crm 和 im 共用（同一支，不是兩支）。

**不確定**
- U1 聊天通道：D7 預設 server streaming＋unary 送出，connect-web 在瀏覽器不支援雙向串流；現況是 WS 雙向（含已讀、輸入中等？我只量到 `ChatStream` 一條雙向 stream，沒逐訊息種類列出）。需要 Will 決定（簡報 A6-2 的 Q3 扇出相關）。
- U2 多 pod：hub 在記憶體，單體多副本時 WS 投遞怎麼辦；目前部署是否單副本我沒查（只讀程式）。
- U3 `GetSellerOrderReviewStats`（#7）與 `ListStoreRatingStats`（#8）兩支統計是否合成一個 API，要看欄位差異，我沒逐欄比。
- U4 orderservice 補的 3 支方法的介面形狀（`GetOrderForReview`、`GetOrderForAfterSales`、`HasOrderBetween`）要對 Java 的 `OrderGrpcService.java` 驗證 proto 是否已有；我只確認單體 `orderservice.go` 沒有。
- U5 `account.deleted` 事件的實際發送點（#12）只在 `M/legacy/rabbitmq` 測試看到樣本，未追到 accountdeletion 的發送行。
- U6 `HideReview`／`HideMessage` 要不要新增「還原」要看 moderation D9；舊 RPC 沒有還原。
- 本盤點沒覆蓋：三個服務各自的 schema 欄位逐項沿用盤點（屬 A6-1 到 A6-3 的 CARRYOVER）、Java `order-service` 的 `GetOrderForReview` 實作、infra（compose、監控 `infra-alerts.yml`、`service-alerts.yml` 對三個服務的規則）、`pkg/proto/{review,im,crm}` 的刪除連動、website 對 review／crm 的呼叫點逐行（只確認 client 層入口）。
