# A7-0 介面盤點：social-service（唯讀，快照 main `172260d31`）

> 2026-10-22：「去向」是 2026-10-18 的快照。S1、S3（決定簡報）之後，legacy SocialService 8 支、「是否封鎖」RPC、im／review 的 port、`/app/blocks*` 留到 A6、代行路由留到 A10 這幾種去向不再成立，以 `../2026-10-18-a7-social.md` 的步驟為準。

標記：【量】＝跑過指令或打開檔案看過；【推】＝讀程式推論。路徑縮寫：`B=backend`、`G=backend/gateway/internal`、`S=backend/services/social-service/internal`、`M=backend/internal`、`CSV=backend/docs/plans/2026-09-24-go-modular-monolith`。
方法名（Connect）是我依 A7-D1…D5′ 擬的名字【推】，不是已決定的契約；去向的「種類」用 `scripts/check-migration-inventory.py:5-11` 的詞彙（rpc-servers：api／http／removed；events：call／in-tx-event／job／removed）。

## 0. 數字與指令（和簡報的差異先講）

| 項目 | 量到 | 簡報 | 指令 |
|---|---|---|---|
| SocialService RPC | **34** | 34 | `awk -F, '$1=="SocialService" && $3=="social-service"' CSV/rpc-servers.csv \| wc -l`＝34；`rg -c '^\s*rpc ' B/api/proto/social/social.proto`＝34；Go server：`S/grpc/handler.go` 25 支＋`handler_moderation.go` 9 支（`rg -n '^func \(s \*ContentServer\) [A-Z]'`）＝34 【量】 |
| 呼叫 social RPC 的 call site | gateway 34 列／41 處（`social_client.go` 36＋`delegation_client.go` 5）；seller-service 1 | gateway 34、seller-service 1 | `awk -F, '$2=="social-service"' CSV/rpc-calls.csv \| wc -l`＝35；gateway `call_sites` 加總＝41（`awk -F, '$1=="gateway" && $2=="social-service"{s+=$5} END{print s}'`）；`rg -n 'c\.client\.\w+\(' G/grpc/social_client.go`＝36 行＋`delegation_client.go` 5 行 【量】 |
| events 列 | **14**（producer 12、consumer 3，`counter.delta` 同時是兩邊） | 14 | `awk -F, 'NR>1 && ($3=="social-service"\|\|$4=="social-service")' CSV/events.csv \| wc -l`＝14 【量】 |
| 表 | **10** | 10 | `rg -c 'CREATE TABLE' services/social-service/db/schema/schema.sql`＝10（cms_post、reports、user_blocks、moderation_actions、cms_comment、cms_comment_like、cms_post_like、cms_seller_forum、cms_hashtag、cms_post_hashtag）【量】 |
| 舊 REST operation（簡報口徑：Post13＋Comment7＋Forum6＋Moderation5＋AdminModeration4＋AdminPost2＋PostHashtag2＋Hashtag2＋CommentRealtime1） | **42** | 42 | `python3 -I`＋PyYAML 讀 `B/api/openapi/openapi.yaml`，數 `paths.*.{get,post,put,patch,delete}` 的 `tags` 落在上述九個 tag 的 operation＝42（各 tag 數與簡報一致；全檔 489 個 operation）【量】 |
| **其中真的由 social 服務的 REST** | **40**（含 1 條 SSE） | 42 | 同上，去掉 tag `Hashtag` 的 2 個：`GET /public/hashtags`、`GET /public/hashtags/popular` 在 `G/router/routes_product.go:35-36` 註冊，handler `G/handler/hashtag.go:29,88` 呼叫 `h.Product.SearchHashtags`／`GetPopularHashtags`（`hashtag.go:47,101`），是 **product-service** 的商品 hashtag，不碰 social 【量】。**差 2：簡報的 42 把 product 的 2 個算進來了** |
| 代行貼文路由 | **6** | 6 | `G/router/routes_delegated.go:107-113`（註解 107、路由 108-113）；不在 openapi.yaml，登記在 `G/router/openapi_coverage_test.go:47,48,52,57,58,59` 的 `routesNotInSpec` 【量】 |
| SSE | 1（含在上面 40／42 內，tag `CommentRealtime`） | 1 | `GET /public/posts/{post_id}/comments/stream` 【量】 |
| `maxRESTOperations` | 489 | — | `G/router/openapi_coverage_test.go:28`；若 A7-4 刪 40 條社群 → 449；若誤刪簡報口徑的 42 條 → 447（會多刪 product hashtag 2 條）【推】 |

**若 3 支封鎖 RPC 要留 legacy（見 §4.3），A7-4 能刪的 REST 是 37 條，不是 40 條**（`/app/blocks*` 3 條要活到 A6）。

---

## 1. RPC（34 支）

呼叫方欄：G＝gateway。`G/grpc/social_client.go` 的 client 方法行（`c.client.X(`）；handler 行是 `G/handler/*.go` 呼叫 `h.Social.*` 的行。單體 `M/legacy/*` 沒有任何程式呼叫 social（`rg -n -i 'social' M --glob '!*_test.go'` 只有註解與依賴表，見 §6）【量】。seller-service 的 1 個 call site：`services/seller-service/internal/client/social/client.go:29`（`GetPostList`）← `services/seller-service/internal/usecase/certification.go:523`（`HasPublishedPost`）。

| # | RPC | 呼叫方 file:line | 到它的 REST | 去向（種類: 說明） |
|---|---|---|---|---|
| 1 | GetPostList | G social_client.go:52（handler post.go:40 `GetLatestPosts`、post.go:78 `GetSellerPosts`）；seller-service client.go:29 | `GET /public/posts`、`GET /public/sellers/{id}/posts` | api: `SocialPublicService.ListPosts`（可帶 store 篩選）。seller-service 的呼叫端刪除（單體 seller 已刪前置條件，`M/modules/seller/CARRYOVER.md:143`；簡報稱「沒人用」，我量到 legacy seller-service 內仍有 caller，但那是已被單體取代的程式，A7-5 隨 seller-service 目錄一起處理） |
| 2 | GetPostDetail | G social_client.go:65（post.go:115 `GetPostDetail`）、:78（post.go:523 `GetMyPostDetail`）、delegation_client.go:308（admin_delegated_post.go:506 `readPost`，被 :197 get、:322 update、:415／:467 publish／unpublish 共用）、sse_stream.go:51（SSE 的 404 預檢） | `GET /public/posts/{id}`、`GET /app/sellers/posts/{id}`（3a-① 例外）、SSE 預檢、代行 GET／PATCH／publish／unpublish | api: `SocialPublicService.GetPost`＋`SocialAppService.GetPost`（店主版，與 List 回同一個 Post 訊息；刪 3a-① 例外，計畫 A7 段末行）；http（legacy gRPC）: 代行用，活到 A10 |
| 3 | CreatePost | social_client.go:110（post.go:157）；delegation_client.go:317（admin_delegated_post.go:252） | `POST /app/sellers/posts`；代行 `POST /admin/sellers/{store_id}/posts` | api: `SocialAppService.CreatePost`（`CapMarketing`）；legacy gRPC 活到 A10 |
| 4 | UpdatePost | social_client.go:133（post.go:218）；delegation_client.go:326（admin_delegated_post.go:354） | `PUT /app/sellers/posts/{id}`；代行 `PATCH …/posts/{post_id}` | api: `SocialAppService.UpdatePost`（`CapMarketing`）；legacy gRPC 活到 A10 |
| 5 | DeletePost | social_client.go:140（post.go:256） | `DELETE /app/sellers/posts/{id}` | api: `SocialAppService.DeletePost`（`CapContentContract`，收縮類放行） |
| 6 | UpdateSellerPostStatus | social_client.go:190（`UpdateMyPostStatus`；post.go:322 `updateMyPostPublishStatus` ← post.go:279 `PublishMyPost`、:298 `UnpublishMyPost`）；delegation_client.go:335（admin_delegated_post.go:482 `writePostStatus` ← :418 publish、:470 unpublish） | `POST /app/sellers/posts/{id}/publish`（`CapMarketing`）、`…/unpublish`（`CapContentContract`）；代行 publish／unpublish | api: `SocialAppService.PublishPost`／`UnpublishPost`（兩個動作能力類別不同，不要合成一支「設狀態」）；legacy gRPC 活到 A10 |
| 7 | TogglePostLike | social_client.go:88（post.go:364） | `POST /app/posts/{id}/like` | api: `SocialAppService.LikePost`／`UnlikePost`（toggle 改成冪等的 set／unset；貼文讚永遠是本人身分，`G/handler/social_caller.go:35-37`）【推：toggle 是否保留要 A7-2 定】 |
| 8 | SharePost | social_client.go:150（post.go:413） | `POST /app/posts/{id}/share` | api: `SocialAppService.SharePost`。server 端從未讀 userID（`post.go:398-403` 註解）；是否改成匿名 public 是設計題【推】 |
| 9 | ReportPost | social_client.go:161（post.go:462） | `POST /app/sellers/posts/{id}/reports`（買家檢舉卻掛在 `/sellers/` 下） | removed: 與 SubmitReport 重複，併入 `moderation` 的 `SubmitReport`（target=POST） |
| 10 | ListMyPosts | social_client.go:173（post.go:499）；delegation_client.go:294（admin_delegated_post.go:146） | `GET /app/sellers/me/posts`（`CapMarketing`）；代行 `GET …/posts` | api: `SocialAppService.ListMyPosts`（D3：停權也讀得到，改掛 `CapContentContract` 讀取）；legacy gRPC 活到 A10 |
| 11 | AdminListPosts | social_client.go:201（post.go:558） | `GET /admin/posts`（`routes_admin.go:16`） | api: `SocialAdminService.ListPosts`（要在 `platform/adminperm` 加權限，目前沒有 social／moderation 權限，`rg -i 'moderat\|posts' M/platform/adminperm` 無） |
| 12 | AdminModeratePost | social_client.go:212（post.go:603） | `PATCH /admin/posts/{id}`（`routes_admin.go:17`） | api: `moderation` 的隱藏／恢復內容；作者是店家時改成 seller 措施「內容下架」（D5′），不再直接 `HidePost` |
| 13 | SearchHashtags | social_client.go:386（post_hashtag.go:51） | `GET /public/tags/suggest` | api: `SocialPublicService.SuggestTags` |
| 14 | GetTrendingHashtags | social_client.go:396（post_hashtag.go:108） | `GET /public/tags/trending` | api: `SocialPublicService.ListTrendingTags` |
| 15 | GetCommentList | social_client.go:234（post.go:642 `GetPostComments`）、:296（post.go:679 `GetCommentReplies`，同一支 RPC 以 parent 篩選） | `GET /public/posts/{id}/comments`、`GET /public/posts/comments/{id}/replies` | api: `SocialPublicService.ListComments`（parent 為請求欄位；帶 D4 的 `viewer_can_*`、`is_mine`） |
| 16 | CreateComment | social_client.go:247（post.go:732 `CreatePostComment`、:788 `ReplyPostComment`） | `POST /app/posts/{id}/comments`、`POST /app/posts/comments/{id}/replies` | api: `SocialAppService.CreateComment`（回覆＝帶 parent，兩條 REST 併成一支）；身分由伺服器推導（A7-D1） |
| 17 | UpdateComment | social_client.go:262（post.go:841） | `PUT /app/posts/comments/{id}` | api: `SocialAppService.UpdateComment`（`CapMarketing` 當店家身分） |
| 18 | DeleteComment | social_client.go:274（post.go:884） | `DELETE /app/posts/comments/{id}` | api: `SocialAppService.DeleteComment`（收縮類放行；店主刪別人留言） |
| 19 | ToggleCommentLike | social_client.go:285（post.go:922） | `POST /app/posts/comments/{id}/like` | api: `SocialAppService.LikeComment`／`UnlikeComment`（同 #7） |
| 20 | GetSellerForum | social_client.go:312（seller_forum.go:89） | `GET /public/sellers/{id}/forum` | api: `SocialPublicService.ListForumEntries` |
| 21 | GetForumReplies | social_client.go:325（seller_forum.go:127） | `GET /public/sellers/forum/{id}/replies` | api: 併入 `ListForumEntries`（parent 篩選） |
| 22 | CreateForumComment | social_client.go:338（seller_forum.go:178） | `POST /app/sellers/{id}/forum` | api: `SocialAppService.CreateForumEntry` |
| 23 | ReplyForumComment | social_client.go:350（seller_forum.go:230） | `POST /app/sellers/forum/{id}/reply` | api: 併入 `CreateForumEntry`（帶 parent；回覆只限店家，規則留在伺服器） |
| 24 | UpdateForumComment | social_client.go:362（seller_forum.go:283） | `PUT /app/sellers/forum/{id}` | api: `SocialAppService.UpdateForumEntry` |
| 25 | DeleteForumComment | social_client.go:374（seller_forum.go:326） | `DELETE /app/sellers/forum/{id}` | api: `SocialAppService.DeleteForumEntry` |
| 26 | SubmitReport | social_client.go:408（moderation.go:184） | `POST /app/reports` | api: `moderation` 的 `ModerationAppService.SubmitReport`（所有 target 型別；對象是店家時直接建立 seller 通報，D5′） |
| 27 | ListMyReports | social_client.go:421（moderation.go:221） | `GET /app/reports/me` | api: `ModerationAppService.ListMyReports` |
| 28 | BlockUser | social_client.go:431（moderation.go:258） | `POST /app/blocks/{userId}` | api: buyer 的個人封鎖／seller 的店家封鎖（A7-D2，從留言封鎖送留言參照）；**另 http（legacy gRPC）保留舊簽章到 A6**（聊天、評價入口仍送內部編號，簡報 D2 REAL 列） |
| 29 | UnblockUser | social_client.go:437（moderation.go:294） | `DELETE /app/blocks/{userId}` | 同 #28 |
| 30 | ListBlocks | social_client.go:443（moderation.go:327） | `GET /app/blocks` | 同 #28（buyer、seller 各自列出；legacy 版活到 A6） |
| 31 | AdminListReports | social_client.go:453（moderation.go:354） | `GET /admin/reports` | api: `ModerationAdminService.ListReports`（平台佇列，依 10e 第 3 刀 D7 投影 seller＋moderation 事件） |
| 32 | AdminGetReport | social_client.go:465（moderation.go:383） | `GET /admin/reports/{id}` | api: `ModerationAdminService.GetReport` |
| 33 | AdminResolveReport | social_client.go:471（moderation.go:429） | `POST /admin/reports/{id}/resolve` | api: `ModerationAdminService.ResolveReport`（`WARN_USER`／`BAN_USER` 對店家改走措施；對買家 BAN 在帳號有未關閉的店時拒絕） |
| 34 | AdminReopenReport | social_client.go:483（moderation.go:476） | `POST /admin/reports/{id}/reopen` | api: `ModerationAdminService.ReopenReport` |

**要保留的 legacy gRPC SocialService（單體實作，`M/legacy/socialserver` 之類的新目錄）**：代行 5 支（#2、#3、#4、#6、#10；`G/grpc/delegation_client.go:288-336`，handler `G/handler/admin_delegated_post.go:146,252,354,482,506`）＋封鎖 3 支（#28–30）＝**34 支裡留 8 支**，另加一支**新的**「是否封鎖」（im `message.go:497,809`、review `handler.go:43` 用；現有任何 legacy proto 都沒有這支）。其餘 26 支在 A7-4 隨 gateway 的 social client 一起刪。【推】
CSV 的 `disposition` 要填：#9、其餘 `api:`；留 legacy 的 8 支建議填 `api:` 並在說明註明「legacy gRPC 至 A10／A6」，以免 `check-migration-inventory.py` 在 A7-5 刪目錄時卡住【推】。
`rpc-calls.csv` 的 `example_path` 行號已過期（例：BlockUser 記 social_client.go:407，實際 :431；代行 5 支記 delegation_client.go:339–380，實際 :294–335 的 `c.client.*` 行，檔案只有 359 行）；call_sites 總數（41）仍對。【量】

---

## 2. 事件（14 列逐項驗證）

共通：`pkg/mq/exchanges.go:13`（`hawkr.content`）、`:58-66`（content routing key）、`:75`（buyer.deleted）、`:86`（seller.deleted）、`:106-110`（social 三個 queue）。Publisher：`S/mq/publisher.go:73-188`（薄包裝）→ `pkg/mq/publisher.go:309-347` 的 `Publish(ctx, ExchangeContent, key, event)`。Payload 型別 `pkg/mq/events.go`（行號下表）；`BaseEvent`（`events.go:74-82`）：`event_id`、`event_type`、`version`、`timestamp`、`source`、`correlation_id`、`metadata`。

| # | routing key（exchange） | producer 發送點 | consumer 點 | payload 欄位（wire） | 去向 |
|---|---|---|---|---|---|
| 1 | comment.created（content）→ notification | `S/usecase/comment.go:248` | notification `services/notification-service/internal/worker/consumer.go:44,99-104`（queue `notification.content`，綁 `#`，`topology.yaml:314-319`）；依 DB template 分派，self-skip 在 `template_handler.go:110-114`（`commenterId`／`commenterPrincipalId`） | `events.go:209-222`：commentId、postId、commenterId、commenterPrincipalId、commenterName、postAuthorId、postAuthorStoreId、postAuthorType、contentPreview、parentCommentId?、clientRequestId?、comment?（`CommentSnapshot` `:192-206`） | in-tx-event（social 模組事件）＋legacy relay 到 RabbitMQ 給 notification（前例 `M/legacy/buyerrelay/relay.go:304-307`、`M/legacy/sellerrelay/relay.go:316-318`），wire 形狀不變，A8 刪 relay |
| 2 | comment.created → gateway SSE | 同上 | `G/mq/consumer.go:162-170`（每連線一條 exclusive queue `gateway.sse.<pod>.<id>`，`:220-242`）；`G/realtime/comment_channel.go:34-44` | 同 #1（fat snapshot，gateway 直接分派不回查） | removed（gateway 與 SSE 在 A7-4 刪）；即時留言改成 server streaming，由 social 模組內部直接推（in-process，不經 MQ） |
| 3 | comment.replied → notification | `S/usecase/comment.go:277` | 同 #1；self-skip key `replierId`／`replierPrincipalId`（`template_handler.go:112-113`） | `events.go:269-283`：replyToUserId、replyToUserStoreId、replyToUserType、replierId、replierPrincipalId、replierName、postId、commentId、contentPreview、parentCommentId?、clientRequestId?、comment? | 同 #1（relay 給 notification） |
| 4 | comment.replied → gateway SSE | 同上 | 同 #2 | 同 #3 | removed（同 #2） |
| 5 | comment.updated → gateway SSE | `S/usecase/comment.go:309` | 同 #2 | `events.go:309-315`：commentId、postId、content、updatedAt | removed（只有 SSE 消費，`SocialEventType.java:20` 也寫「無 notification consumer」）；streaming 內部推 |
| 6 | comment.deleted → gateway SSE | `S/usecase/comment.go:346` | 同 #2 | `events.go:327-332`：commentId、postId、parentCommentId | removed（同 #5） |
| 7 | comment.like_changed → notification | `S/usecase/comment.go:403`（LIKE）、`:427`（UNLIKE） | 同 #1；self-skip `userId`（`template_handler.go:114`）；template 只在 action=LIKE 通知 | `events.go:343-356`：commentId、postId、likeCount、action、userId、likerKind、commentAuthorId、commentAuthorStoreId、likerPrincipalId、likerName、commentAuthorType | 同 #1 |
| 8 | comment.like_changed → gateway SSE | 同上 | 同 #2 | 同 #7 | removed（同 #2） |
| 9 | counter.delta（content）→ social 自己 | `S/usecase/post.go:573,596`、`comment.go:218,221,337,340,396,415` | `S/worker/di.go:117-122`（queue `social.counter.delta`，`topology.yaml:844-848`）→ `S/worker/counter_delta.go:31-80`；對帳 `counter_reconcile.go:35-52`（每 10 分鐘） | `events.go:378-384`：entityType、entityId、field、delta | removed: 單體內改成與寫入同一個 transaction 更新計數（或 in-tx 投影），不要自己發給自己；對帳工作改 River 週期 job。注意此 key 也會被 notification 的 `#` 綁定收到（`consumer.go:99-104`），因無 template 被略過（`template_handler.go:37` 的 debug log）【量】 |
| 10 | post.created → notification | `S/usecase/post.go:213`（對每個追蹤者各發一則；追蹤者從 buyer `GetSubscribersList` 取，`post.go:208`） | 同 #1 | `events.go:170-177`：post_id、store_id、follower_id、seller_name、seller_avatar_key、content_preview | 同 #1；fan-out 改為 River job 取 `buyer.Subscribers`（keyset，`M/modules/buyer/buyer.go:395` 之外的新 API），legacy relay 保持一則一追蹤者的 wire |
| 11 | post.hidden_by_platform → notification | `S/usecase/post_hider.go:73` | 同 #1（template 建立於 `notification-service/db/migrations/20260709120000_content_hidden_seller_template.sql`） | `events.go:430-436`：seller_id、store_id、post_id、reason | 同 #1，**觸發點改成 seller 的 `MeasureCreated`（種類＝內容下架）**：social 不再寫隱藏狀態，內容的可見性由 `seller.pub_content_takedowns` 在讀取時推導（D5′；簡報 `d3-read-model-brief.md` §3）；A8 notification 以 relay 形狀訂閱，只 `Enqueue`，併入措施通知，一個事實一則（P3，Will 2026-10-18） |
| 12 | post.liked → notification | `S/usecase/post.go:584` | 同 #1 | `events.go:251-257`：seller_id、store_id、likerName、post_id | 同 #1 |
| 13 | buyer.deleted（hawkr.buyer）→ social | 單體 relay `M/legacy/buyerrelay/relay.go:304-307`（模組事件 `buyer.BuyerDeleted`，`M/modules/buyer/events.go:25`） | `S/worker/di.go:103-109`（queue `social.buyer.deleted`，`topology.yaml:832-837`）→ `S/worker/buyer_deleted.go:13-44` | `events.go:484-488`：accountId、buyerId | in-tx-event（relay 式）: social `events.Subscribe(buyer.BuyerDeleted)`，handler 只 `Enqueue` 清除 job，不改自己的表；可見性由 `buyer.pub_*`（已刪除）在讀取時擋住，實體刪除由 job 執行（P2）；relay 繼續服務 im、crm、notification 等舊消費者 |
| 14 | seller.deleted（hawkr.seller）→ social | 單體 relay `M/legacy/sellerrelay/relay.go:316-318`（模組事件 `seller.StoreRetired`，`M/modules/seller/events.go:31`；relay 的 `storeRetired`） | `S/worker/di.go:110-116`（queue `social.seller.deleted`，`topology.yaml:838-843`）→ `S/worker/seller_deleted.go:40-110` | `events.go:478-482`：accountId、store_id | in-tx-event（relay 式）: social 訂閱 `seller.StoreRetired`，handler 只 `Enqueue` 清除 job；可見性由 `seller.pub_*`（已退場）在讀取時擋住（P2）。注意 `seller_deleted.go` 不清 `cms_seller_forum`（`rg -n -i forum S/worker/seller_deleted.go` 無命中）【量】，CSV 說「cleanup seller posts/comments/media」，討論區留言板沒清，新模組要決定 |

**不在 CSV 的社群事件／通道（搜尋結果）**
- `rg -n 'Publish\(|PSubscribe|Subscribe\(' S gateway/internal pkg --glob '!*_test.go'` 找不到 Redis pub/sub；**SSE 全走 RabbitMQ**（`G/mq/consumer.go`），Redis 在 social 只放封鎖集合（`rg -n 'rdb\.|redis\.' S` 命中 blocklist 與 client 建立）【量】。
- social 的訂閱只有 3 個（buyer.deleted、seller.deleted、counter.delta，`S/worker/di.go:101-124`）；`M/legacy/accountservice/events.go:14-15` 的註解說 social 還訂閱 `account.deleted`，**程式與 `events.csv:39-42` 都沒有**（stale 註解）【量】。
- `pkg/mq/events.go` 與 `MqConstants.java:320-324` 另有 `comment.added`／舊 `post.created` 常數，Java 端無任何使用（`rg -l 'ROUTING_KEY_COMMENT_ADDED' services/java --glob '*.java'` 只有定義檔）【量】。
- `hawkr.content` exchange 不是 social 專用：`product.viewed` 也在上面（`pkg/mq/exchanges.go` `RoutingKeyProductViewed`；seller-service 訂閱 `services/seller-service/internal/mq/di.go:62-68`），所以 notification 的 `notification.content` queue 綁 `#` 會收到非 social 事件。刪 social 不能連 exchange 一起刪【量】。
- 排程貼文發布成功不發 `post.created`（`rg -n 'PublishPostCreated' S --glob '!*_test.go'` 只有 `post.go:213`，在 `CreatePost` 裡）；而 `CreatePost` 在 `publishStatus` 為草稿時也會對追蹤者發（`post.go:196-215` 沒有檢查 `publishStatus`）【量；後果是推論】。新模組要決定「發布」才是 `post.created` 的觸發點，這是行為差異，需要 characterization。

---

## 3. social-service 發出的呼叫

預設位址：`S/config/config.go:128-156`：account（host `account-service`，port 9941）、buyer（`127.0.0.1:9941`）、seller（`127.0.0.1:9941`）、media `:9937`、product `:9913`、review `:10033`、im `:9933`；單體 legacy gRPC `:9941`（`B/Makefile:254`）。account host 預設不是 127.0.0.1（簡報寫「三個預設 9941」指 port，成立）【量】。

| 呼叫 | 發送點 file:line（client → 使用處） | 單體現況 | 去向 |
|---|---|---|---|
| account `GetAccountInfo`／`GetAccountInfoBatch` | `S/client/account/client.go:71,52`；介面 `S/usecase/account_client.go:22-23` | `M/legacy/accountservice/server.go:71,86` | **removed: 沒有 production 呼叫端**（`rg -n 'GetAccountInfo' S --glob '!client/**' --glob '!*_test.go'` 只剩介面宣告）【量】 |
| account `SuspendAccount` | `S/client/account/client.go:88` ← `S/usecase/dispatcher.go:147`（`AccountBanDispatcher.BanUser`） | `M/legacy/accountservice/server.go:140` | in-process: `moderation` 呼叫 `identity` 的停權；帳號底下有未關閉的店時拒絕（D5′）；店家處分改走 seller 措施，不再 BAN 店主帳號 |
| delegation `RecordAuditIntent` | `S/client/account/delegation.go:39` ← `S/worker/post_schedule_worker.go:260` | `M/legacy/delegationservice/audit.go:48`；模組已有 `RecordDelegationAuditIntent`，`Origin=scheduled` 有整合測試（`M/modules/delegation/delegation_integration_scenarios_test.go:888-919`） | **簡報決定「改發事件」，但這是一個閘門不是通知，見 §7 發現 2**。需 A7-0 與 Will 再定 |
| delegation `ConfirmAuditResult` | `S/client/account/delegation.go:99` ← `post_schedule_worker.go:318,350` | `audit.go:128` | 同上 |
| buyer `GetBuyerProfileByBuyerID` | `S/client/buyer/client.go:36` ← `S/usecase/post.go:580`、`comment.go:441` | `M/legacy/buyerserver/server.go:109` | in-process: `buyer` 模組公開 API（顯示名稱、頭像） |
| buyer `GetSubscribersList` | `S/client/buyer/client.go:56`（`GetSubscriberIDs`）← `post.go:208` | `M/legacy/buyerserver/overlay.go:90` ← `buyer.SubscribersOffset`（`M/modules/buyer/buyer.go:395`，註明「僅提供 legacy social-service」） | in-process: `buyer.Subscribers`（keyset）；`overlay.go:90` 與 `SubscribersOffset` 在 A7-5 刪（`M/modules/buyer/CARRYOVER.md:85`） |
| buyer `BatchGetBuyerMetaByBuyerIDs` | `S/client/buyer/client.go:93` ← `S/grpc/handler.go:879`、`forum_snapshot.go:33`、`comment_snapshot.go:75` | `buyerserver/server.go:229` | in-process: `buyer` 批次顯示（留言寫入時快照＋讀取時覆寫，`replicas.csv:35`；從零設計是否保留快照要在沿用盤點回答） |
| buyer `ResolvePrincipals` | `S/client/buyer/client.go:111,137` ← `usecase/block.go:106`、`dispatcher.go:136` | `buyerserver/server.go:211` | removed: 封鎖移到 buyer／seller 模組後不再需要「內部編號→account」猜測（`block.go:94-115`）；停權走 identity |
| seller `GetSeller` | `S/client/seller/client.go:35` ← `S/usecase/post.go:202,652`、`comment.go:437` | `M/legacy/sellerserver/server.go:221` | in-process: `seller` 公開 API（店名、logo） |
| seller `BatchGetSellerMeta` | `S/client/seller/client.go:56` ← `S/grpc/handler.go:888`、`mapper.go:149`、`post.go:645`、`forum_snapshot.go:28`、`comment_snapshot.go:70` | `sellerserver/server.go:321` | in-process: `seller` 批次顯示；另 gateway 現在還在列表上以 seller 店名覆蓋（`G/handler/marketplace.go:225-290`，見 §4），新 API 直接回店名 |
| seller `ResolvePrincipals` | `S/client/seller/client.go:74,128` ← `post.go:583`、`post_hider.go:72`、`comment.go:144,236`、`block.go:97`、`dispatcher.go:127` | `sellerserver/server.go:337` | removed／改成事件欄位：通知收件人需要 account_id 的地方由 seller 模組 API 提供（事件 payload 的 `seller_id`、`postAuthorId` 欄位要在 relay 保持原值，見 §2 #1/#11/#12） |
| media `FinalizeImages`／`LinkImages`（`FinalizeAndLink`） | `S/client/media/client.go:33` ← `S/usecase/post.go:747` | 單體 `media` 模組，legacy 轉接 `M/legacy/mediaservice`（Java media-service 的轉接，B1 之前） | in-process: `media.Media`（arch 表已允許 `social`→`media`，`M/arch/modules.go:34`） |
| media `ReleaseReferences` | `S/client/media/client.go:40` ← `post.go:364`、`S/worker/seller_deleted.go:94` | 同上 | in-process: `media.Media`（分批 200 的限制來自 5s 固定 deadline，`seller_deleted.go:21-31`；單體內是否仍需要要在沿用盤點回答） |
| product `GetProductsByIDs` | `S/client/product/client.go:33` ← `S/usecase/post.go:630`、`grpc/handler.go:266`、`mapper.go:211` | `M/legacy/productservice/productservice.go:41` 呼叫 Java product-service 的同一支 RPC（單體還沒有 `catalog` 模組：`ls M/modules` 無 catalog） | legacy client: 以 `productservice.Client.Cards` 為 port 的實作；`catalog` 搬進單體後換成 in-process（arch 表已列 `social`→`catalog`） |
| im `HideMessage` | `S/client/im/client.go:31` ← `S/usecase/dispatcher.go:53` | im-service（A6 才搬）；單體無轉接 | legacy client: 由 `moderation` 呼叫（D5′），不是 social；A6 後 in-process |
| review `HideReview` | `S/client/review/client.go:31` ← `dispatcher.go:51` | `M/legacy/reviewservice`（需確認是否有 hide；本盤點沒開檔）【推】 | legacy client: 同上，由 `moderation` 呼叫 |
| Redis | `S/infrastructure/redis.go:16-25`（fail-open）；寫 `S/usecase/block.go:69,122`；讀 `S/grpc/blocklist_filter.go:19,34,49`（handler.go:114,780,810,972,998） | — | removed: 封鎖改存 buyer／seller 的 DB（A7-D2），讀取改 in-process；`pkg/blocklist` 在 A7-5 刪 |
| 網站快取失效（HTTP） | `S/usecase/post.go:79-83`（`bustPost`）、`post_hider.go:67-69`、`S/worker/post_schedule_worker.go:439-445`；設定 `WEB_REVALIDATE_BASE_URL`（`config.go:167`） | `M/platform/revalidate/revalidate.go` 已存在（content、recipe 在用） | in-process: `platform/revalidate`（tags 沿用 `pkg/revalidate/tags.go` 的 `PostTag`／`HashtagTags`）；後援／River job 依 backend/AGENTS.md 規則 4、5 |
| RabbitMQ | 見 §2 | — | 見 §2 |

排程發布 worker（不是 RPC，但是介面）：`S/worker/post_schedule_worker.go`（`publishOne` :127、租約、`cancelByRevocation` :422）→ 單體 River 週期 job；冪等鍵與租約語意要保留（characterization）。

---

## 4. gateway REST

### 4.1 計數指令

```
python3 -I <script>   # PyYAML 載入 backend/api/openapi/openapi.yaml；
                      # 對 paths.*.{get,post,put,patch,delete} 取 op['tags'] ∩ {Post,Comment,Forum,Moderation,
                      # AdminModeration,AdminPost,PostHashtag,Hashtag,CommentRealtime}  → 42
                      # 其中 tag=Hashtag 2 條 handler 呼叫 h.Product（hashtag.go:47,101）→ social 實際 40
```
各 tag：Post 13、Comment 7、Forum 6、Moderation 5、AdminModeration 4、AdminPost 2、PostHashtag 2、Hashtag 2（product）、CommentRealtime 1。腳本：同目錄的 `count-rest-ops.py`（在 `backend/` 執行：`python3 -I docs/plans/2026-10-18-a7-social/count-rest-ops.py`）。

共通中介層：public 群組 `OptionalJWTAuth`；app 群組 `JWTAuth`；admin 群組 `PrivateNoStore + JWTAuth + requireAdmin`（`G/router/routes.go:31`）。`injectSellerSuspension()`（`G/router/seller_capability.go:74-87`）只在 token 是賣家且停權時設 `X-Seller-Suspended` header 與 context，**不拒絕**；social 以 gRPC metadata 讀（`S/grpc/handler.go:30-33`）。`sellerHandle`（`seller_capability.go:101-105`）＝登記能力＋`requireSeller(cap)`（`:18-44`）。單體取代：能力由 seller 模組推導（`M/modules/seller/internal/model/capability.go:26,28`），不再靠 header。

gateway 側身分編排（每個需要的 handler 都先打單體 legacy gRPC）：`socialBuyerID`／`socialCaller`（`G/handler/social_caller.go:35-62`）→ `h.BuyerID.CurrentBuyerID`、`h.Store.CurrentStoreID`（`G/buyerid/resolver.go:39`、`G/storeid/resolver.go:38`）；`resolveStoreID`（`G/handler/resolve_seller.go:16-26`）；`resolveModerationUserID`（`G/handler/moderation.go:47-` ，依 JWT 的 `loginRole` 選 buyer 或 store 空間——A7-D1 要消滅的 active role）。

### 4.2 逐條（40 條 social；SSE 在最後）

「註冊」＝`G/router/` 檔案:行；「handler」＝`G/handler/` 檔案:行。RPC 編號對 §1。

| # | tag | method path（`/api/v1` 起） | 註冊 | handler | 中介／能力 | RPC | gateway 側編排／補資料 | 去向 |
|---|---|---|---|---|---|---|---|---|
| 1 | Post | GET /public/posts | routes_post.go:38 | post.go:31 `GetLatestPosts` | OptionalJWT | #1 | `socialBuyerID`（post.go:34）；`enrichPostsWithSellerInfo`（:50）→ `h.Seller.GetSellerBatch` 蓋掉店名／logo／store_code（marketplace.go:225-290）；blocklist 在 social 內 | removed: `SocialPublicService.ListPosts`，店名直接由 seller 模組 API 補 |
| 2 | Post | GET /public/sellers/{id}/posts | routes_post.go:29 | post.go:67 `GetSellerPosts` | OptionalJWT | #1 | `resolveStorefrontSellerID`（post.go:68→seller.go:176-203，`h.Seller.GetSellerByStoreID`，接受 store_id 或 store_code）；同上 enrich（:88） | removed: `ListPosts` 帶 store 篩選；store_code 解析移到 seller 模組 API |
| 3 | Post | GET /public/posts/{id} | routes_post.go:39 | post.go:104 `GetPostDetail` | OptionalJWT | #2 | `socialBuyerID`；`toPostDetailData` 組媒體 URL | removed: `SocialPublicService.GetPost` |
| 4 | Post | POST /app/posts/{id}/like | routes_post.go:52 | post.go:347 `TogglePostLike` | JWT | #7 | `socialCaller`（:359） | removed: `LikePost`／`UnlikePost` |
| 5 | Post | POST /app/posts/{id}/share | routes_post.go:53 | post.go:390 `SharePost` | JWT | #8 | 不解析身分（:398-403） | removed: `SharePost` |
| 6 | Post | POST /app/sellers/posts | routes_post.go:72 | post.go:140 `CreatePost` | JWT＋`CapMarketing` | #3 | `resolveStoreID`（:141） | removed: `SocialAppService.CreatePost` |
| 7 | Post | GET /app/sellers/posts/{id} | routes_post.go:73 | post.go:512 `GetMyPostDetail` | JWT＋`CapMarketing` | #2 | `resolveStoreID` | removed: `SocialAppService.GetPost`；**3a-① 例外路由**，ADR `2026-09-26-external-contract-protobuf-connect.md:109`（`maxRESTOperations` 492→493），與 A7 Connect Get 同時刪 |
| 8 | Post | PUT /app/sellers/posts/{id} | routes_post.go:74 | post.go:186 `UpdatePost` | JWT＋`CapMarketing` | #4 | `resolveStoreID` | removed: `UpdatePost` |
| 9 | Post | DELETE /app/sellers/posts/{id} | routes_post.go:75 | post.go:245 `DeletePost` | JWT＋`CapContentContract` | #5 | `resolveStoreID` | removed: `DeletePost` |
| 10 | Post | GET /app/sellers/me/posts | routes_post.go:76 | post.go:491 `ListMyPosts` | JWT＋`CapMarketing`（D3 要改成收縮讀取放行） | #10 | `resolveStoreID` | removed: `ListMyPosts` |
| 11 | Post | POST /app/sellers/posts/{id}/publish | routes_post.go:77 | post.go:279 `PublishMyPost` → :311 `updateMyPostPublishStatus` | JWT＋`CapMarketing` | #6 | `resolveStoreID` | removed: `PublishPost` |
| 12 | Post | POST /app/sellers/posts/{id}/unpublish | routes_post.go:78 | post.go:298 `UnpublishMyPost` | JWT＋`CapContentContract` | #6 | 同上 | removed: `UnpublishPost` |
| 13 | Post | POST /app/sellers/posts/{id}/reports | routes_post.go:82 | post.go:439 `ReportPost` | JWT | #9 | `resolveModerationUserID`（:451） | removed: 併入 `SubmitReport`（路徑錯置在 `/sellers/` 下） |
| 14 | PostHashtag | GET /public/tags/suggest | routes_post.go:21 | post_hashtag.go:35 | public | #13 | — | removed: `SuggestTags` |
| 15 | PostHashtag | GET /public/tags/trending | routes_post.go:22 | post_hashtag.go:97 | public | #14 | — | removed: `ListTrendingTags` |
| 16 | Comment | GET /public/posts/{id}/comments | routes_post.go:40 | post.go:630 `GetPostComments` | OptionalJWT | #15 | `socialCaller`（:637） | removed: `ListComments` |
| 17 | Comment | GET /public/posts/comments/{id}/replies | routes_post.go:41 | post.go:667 `GetCommentReplies` | OptionalJWT | #15 | `socialCaller`（:674） | removed: `ListComments`（parent 篩選） |
| 18 | Comment | POST /app/posts/{id}/comments | routes_post.go:56 | post.go:706 `CreatePostComment` | JWT＋`injectSellerSuspension` | #16 | `socialCaller`（:727） | removed: `CreateComment` |
| 19 | Comment | POST /app/posts/comments/{id}/replies | routes_post.go:57 | post.go:760 `ReplyPostComment` | JWT＋suspension header | #16 | `socialCaller`（:783） | removed: 併入 `CreateComment` |
| 20 | Comment | PUT /app/posts/comments/{id} | routes_post.go:58 | post.go:816 `UpdatePostComment` | JWT＋suspension header | #17 | `socialCaller`（:836） | removed: `UpdateComment` |
| 21 | Comment | DELETE /app/posts/comments/{id} | routes_post.go:59 | post.go:867 `DeletePostComment` | JWT＋suspension header | #18 | `socialCaller`（:879） | removed: `DeleteComment` |
| 22 | Comment | POST /app/posts/comments/{id}/like | routes_post.go:60 | post.go:905 `ToggleCommentLike` | JWT＋suspension header | #19 | `socialCaller`（:917） | removed: `LikeComment`／`UnlikeComment` |
| 23 | Forum | GET /public/sellers/{id}/forum | routes_post.go:88 | seller_forum.go:76 | OptionalJWT | #20 | `socialCaller` | removed: `ListForumEntries` |
| 24 | Forum | GET /public/sellers/forum/{id}/replies | routes_post.go:90 | seller_forum.go:114 | OptionalJWT | #21 | `socialCaller` | removed: 併入 `ListForumEntries` |
| 25 | Forum | POST /app/sellers/{id}/forum | routes_post.go:96 | seller_forum.go:153 | JWT＋suspension header | #22 | `socialCaller` | removed: `CreateForumEntry` |
| 26 | Forum | POST /app/sellers/forum/{id}/reply | routes_post.go:102 | seller_forum.go:205 | JWT＋suspension header | #23 | `socialCaller` | removed: 併入 `CreateForumEntry` |
| 27 | Forum | PUT /app/sellers/forum/{id} | routes_post.go:103 | seller_forum.go:258 | JWT（**沒有** suspension header） | #24 | `socialCaller` | removed: `UpdateForumEntry`（D3 要把編輯也歸入新增類） |
| 28 | Forum | DELETE /app/sellers/forum/{id} | routes_post.go:104 | seller_forum.go:309 | JWT | #25 | `socialCaller` | removed: `DeleteForumEntry` |
| 29 | Moderation | POST /app/reports | routes_moderation.go:24 | moderation.go:155 `SubmitReport` | JWT | #26 | `resolveModerationUserID`（:162）；證據 key 經 `resolveInputObjectKey`（:171-180，限 `public/` 前綴） | removed: `ModerationAppService.SubmitReport` |
| 30 | Moderation | GET /app/reports/me | routes_moderation.go:25 | moderation.go:208 | JWT | #27 | `resolveModerationUserID` | removed: `ListMyReports` |
| 31 | Moderation | POST /app/blocks/{userId} | routes_moderation.go:31 | moderation.go:242 | JWT | #28 | `resolveModerationUserID`；`userId` 是內部編號，歧義 | **保留**（gateway→單體 legacy gRPC）至 A6：聊天、評價入口仍用此路由（簡報 D2 REAL）；新入口走 buyer／seller Connect |
| 32 | Moderation | DELETE /app/blocks/{userId} | routes_moderation.go:32 | moderation.go:278 | JWT | #29 | 同上 | 同 #31 |
| 33 | Moderation | GET /app/blocks | routes_moderation.go:33 | moderation.go:314 | JWT | #30 | 同上 | 同 #31 |
| 34 | AdminModeration | GET /admin/reports | routes_moderation.go:38 | moderation.go:351 | admin | #31 | — | removed: `ModerationAdminService.ListReports` |
| 35 | AdminModeration | GET /admin/reports/{id} | routes_moderation.go:39 | moderation.go:377 | admin | #32 | — | removed: `GetReport` |
| 36 | AdminModeration | POST /admin/reports/{id}/resolve | routes_moderation.go:40 | moderation.go:413 | admin | #33 | adminID 取 JWT account | removed: `ResolveReport` |
| 37 | AdminModeration | POST /admin/reports/{id}/reopen | routes_moderation.go:41 | moderation.go:460 | admin | #34 | adminID 取 JWT account | removed: `ReopenReport` |
| 38 | AdminPost | GET /admin/posts | routes_admin.go:16 | post.go:554 | admin | #11 | — | removed: `SocialAdminService.ListPosts` |
| 39 | AdminPost | PATCH /admin/posts/{id} | routes_admin.go:17 | post.go:585 | admin | #12 | `operatorID` 取 JWT | removed: moderation 隱藏／措施 |
| 40 | CommentRealtime | GET /public/posts/{post_id}/comments/stream（SSE） | routes_post.go:43 | sse_stream.go:32 `StreamPostComments` | OptionalJWT（匿名可）；同一限流身分連線上限 5（`sse_stream.go:21` swag 註解） | #2（預檢 :51） | 預檢貼文存在；`socialCaller` 算 viewer（buyerId、storeId）供每條連線個人化 `isMine`（`G/realtime/comment_personalize.go`）；Hub／ringbuffer 在 `G/sse/`，Last-Event-ID 重送；訂閱 5 個 comment.* key | removed: 單體 server streaming RPC（ADR D7，續傳改請求內游標）；`G/sse`、`G/realtime/comment_*`、`G/mq/consumer.go` 的 comment 綁定隨之刪，但 notification 通知 SSE（`sse_stream.go:78` `StreamNotifications`）仍用同一個 Hub，**不能整包刪**【量】 |

另外 2 條（簡報 42 內、不屬於 social）：`GET /public/hashtags`、`GET /public/hashtags/popular`（`routes_product.go:35-36`，`handler/hashtag.go:29,88`，呼叫 product-service）；同族還有 `GET /app/hashtags`（`routes_product.go:95`，`CapProductManage`，在 `routesNotInSpec`）。去向屬於 catalog／product 的搬遷，與 A7 無關，A7 不得刪。

### 4.3 6 條代行貼文路由與 3a-① 例外

| # | 路由（`/api/v1/admin/sellers/{store_id}` 起；`routes_delegated.go`） | handler（`G/handler/admin_delegated_post.go`） | 能力 | RPC | 去向 |
|---|---|---|---|---|---|
| D1 | GET /posts（:108） | :131 `ListDelegatedPosts` | `AdminCapPostsRead` | #10（delegation_client.go:294） | 留在 gateway 到 A10；後端改打單體 legacy gRPC（`ListMyPosts`） |
| D2 | POST /posts（:109） | :210 `CreateDelegatedPost`（`orch.Orchestrate` :232） | `AdminCapPostsWrite` | #3（:317） | 同上 |
| D3 | GET /posts/:post_id（:110） | :190 `GetDelegatedPost` → `readPost` :502 | `AdminCapPostsRead` | #2（:308） | 同上 |
| D4 | PATCH /posts/:post_id（:111） | :278 `UpdateDelegatedPost`（`readPost` :322＋:354） | `AdminCapPostsWrite` | #2、#4（:326） | 同上 |
| D5 | POST /posts/:post_id/publish（:112） | :372 `PublishDelegatedPost` | `AdminCapPostsPublish` | #2、#6（:335） | 同上 |
| D6 | POST /posts/:post_id/unpublish（:113） | :430 `UnpublishDelegatedPost` | `AdminCapPostsPublish` | #2、#6 | 同上 |

- 能力檢查在 gateway：`requireCapability`（`G/grpc/delegation_client.go:56-70`）＋`delegatedHandle`（`G/router/delegated_capability.go:566-572`）；稽核由 `orch.Orchestrate`（gateway 的 delegation 編排）呼叫 account-service 的 `RecordAuditIntent`／`ConfirmAuditResult`，已被單體 `legacy/delegationservice` 取代（`server.go:1-8`）。這 6 條與 40 條無關，**不計入** openapi 42；`maxRESTOperations` 不計（`routesNotInSpec`）。
- 3a-① 例外：`GET /app/sellers/posts/:id`（#7 above），ADR 文末修訂 `2026-09-26-external-contract-protobuf-connect.md:103-110`，目前登記的唯一例外；刪除條件＝A7 Connect Get 落地。
- `maxRESTOperations`＝489（`openapi_coverage_test.go:28`）；僅計 openapi.yaml 的 operation，`routesNotInSpec`（含 6 條代行貼文）不計。

---

## 5. 其他消費者

| 對象 | 內容 | 位置 | 去向 |
|---|---|---|---|
| `pkg/blocklist` importers | social（寫：`S/usecase/block.go:69,122`；讀：`S/grpc/blocklist_filter.go:19,34,49`）、im-service（`services/im-service/internal/usecase/message.go:497` 送訊前雙向檢查，回業務碼 `BlockedMessageNotAllowed`；`:809` 過濾對話列表）、review-service（`services/review-service/internal/grpc/handler.go:43` 以 `ViewerId` 過濾評價作者）。`rg -n 'pkg/blocklist\|go-pkg/blocklist' B --glob '*.go'` 共這些，另有 `seller-service/internal/infrastructure/violation_public_cache.go:25` 只是註解提到 metrics 命名 | `B/pkg/blocklist/blocklist.go:20-21`（key `block:out:<id>`／`block:in:<id>`，`IsBlocked` :89 雙向，Redis 錯誤時 fail-open） | im／review 只換掉讀 Redis 那一行（改打單體新 legacy「是否封鎖」）；`pkg/blocklist` A7-5 刪 |
| 其他服務讀 social 資料 | 無：`rg -n 'cms_post\|cms_comment\|cms_seller_forum\|user_blocks\|hawkr_social\|moderation_actions\|cms_hashtag'` 排除 social 自己與 docs／生成碼後，只剩註解與 codemeta；`block:out`／`block:in` key 字面值只在 `pkg/blocklist` 【量】 | — | — |
| Java | `rg -l "SocialService\|SocialServiceGrpc\|import.*social" B/services/java -g '*.java' \| wc -l`＝**0**；`rg -l 'com\.hawkr\.social' B/services/java --glob '*.java' \| wc -l`＝0。`services/java/shared/java-proto/src/main/proto/social/social.proto` 與 `B/api/proto/social/social.proto` 逐位元相同（`diff -q` 無輸出）【量】。Java 只用 CodeMeta 生成的列舉（`ReportTargetType.java`、`ReportReasonCode.java`、`SocialEventType.java`、`DelegationAuditAction.java`）| — | 兩份 `social.proto`、`B/pkg/proto/social/*.pb.go`（第三份生成物，簡報沒列）A7-5 一併刪；CodeMeta 的 Report* 列舉隨 moderation 搬遷處理（凍結，不手改） |
| `B/scripts/proto/numeric-allowlist.txt` | 38 行 `hawkr.social.*`（`rg -c 'hawkr\.social\.' …`＝38） | — | A7-5 刪 |
| 部署／開發環境 | `B/docker-compose-local.yml:881-961`、`B/dev.sh:15,634,644`、`B/Makefile:15`、`.github/workflows/backend-go-checks.yml`、`backend-integration-tests.yml`、`B/scripts/init-databases.sh`、`topology.yaml:832-848` 的三個 social queue | — | A7-5 刪；`hawkr.content` exchange 與 `notification.content` 保留 |

---

## 6. 單體現況耦合

`rg -n -i 'social' B/internal --glob '!*_test.go' --glob '!legacy/gen/**' --glob '!httpapi/gen/**'`（排除 CARRYOVER／md 之外）：

| 位置 | 內容 | 性質 |
|---|---|---|
| `M/arch/modules.go:34` | `"social": {"im","review","catalog","buyer","seller","identity","media"}` | A7-1 要拿掉 im、review，新增 `moderation`（簡報 D5′） |
| `M/arch/modules.go:36` | `"delegation": {"identity","seller","ordering","catalog","social","media"}`，註解「授權、session、稽核與編排全由 delegation 擁有」 | 只是宣告：`rg -n 'modules/social' M` 沒有任何 import，`ls M/modules` 沒有 social 目錄；`modules/delegation` 目前不引用 social 【量】。social→delegation 會成環 |
| `M/arch/modules.go:41` | `notification` 依賴 `social` | A7 新增 `moderation` 時要補 |
| `M/legacy/delegationservice/server.go:1-8`、`audit.go:48,128` | 單體替 gateway 與 social-service 提供 DelegationService | social 刪除後只剩 gateway 用，A10 刪 |
| `M/legacy/accountservice/server.go:140`（`SuspendAccount` 註解「social 的 BAN_USER」）、`events.go:14-15` | 見 §3、§2 | `SuspendAccount` 的 social 呼叫端隨 A7 消失；註解 stale |
| `M/legacy/buyerserver/overlay.go:90-133`、`M/modules/buyer/buyer.go:395`、`internal/follow/follow.go:196`、`CARRYOVER.md:85`、`AGENTS.md:43` | `GetSubscribersList` 有界 offset 轉接，唯一呼叫方 social | A7-5 刪 |
| `M/legacy/sellerrelay/relay.go:14,52`、`buyerrelay/relay.go:304-307` | 註解提到 social 是消費者；relay 保留給其他舊消費者 | A7 後 social 不再是消費者，註解更新 |
| `M/platform/codemeta/code_meta.go:2208,4850-4896` | Report 狀態／target／原因的列舉，註解稱權威值域是 social-service 的 DB CHECK | 凍結；moderation 搬遷時依 ADR `codemeta-retirement` 處理 |
| `M/modules/delegation/db/schema.sql:29` | 註解：`scheduled_by_session_id` 在 social 存成 BIGINT | 排程貼文欄位遷移時要一致 |
| `M/modules/seller/CARRYOVER.md:143,226,237` | seller 不再詢問 social；`X-Seller-Suspended`／`X-Seller-Capabilities` 供 im、social | 單體內的 social 直接向 seller 查能力 |

單體沒有任何 social 的 Go 程式碼或 Connect proto：`ls B/../api/hawkr` 只有 admin、agent、buyer、common、content、discovery、identity、seller【量】，A7-2 要新增 `social`、`moderation`（以及 `buyer.proto`、`seller.proto` 的封鎖方法）。

---

## 7. 發現與不確定

1. **42 vs 40**：簡報的 42 含 product 的 2 個 `Hashtag` operation；social 真正服務的是 40 條（含 SSE）。`maxRESTOperations` 計算與 A7-4 的刪除清單要用 40（§0、§4.2）。
2. **代行稽核「改發事件」語意不夠**：現行是兩階段閘門——`RecordAuditIntent` 失敗就不發布（`post_schedule_worker.go:127-246`：intent 失敗→`releaseAfterFailure`；session 已撤銷（`bizcode.DelegationSessionRevoked`）→`cancelByRevocation` :422），發布後才 `ConfirmAuditResult`（:318,350）。事件是事後通知，無法擋發布、無法回傳「已撤銷」、無法回 `Replayed`／`AuditLogID`。模組依賴表只禁止 import，替代方案是 social 定義一個 port、在 `cmd/hawkr` 組裝點以 delegation 的 `RecordDelegationAuditIntent`／確認 API 實作（接線只在組裝點，`backend/AGENTS.md`「單體的結構」）【推；未跑 arch 測試驗證】。A7-0 定案前需 Will 確認。
3. **封鎖 REST 3 條不能在 A7-4 刪**：簡報 D2 的 REAL 列要求 legacy gRPC 接住 3 支舊封鎖 RPC，因此 gateway 的 `/app/blocks/{userId}`（POST／DELETE）與 `/app/blocks`（GET）要活到 A6；但簡報 A7-4 寫「刪 openapi.yaml 的 42 個 operation」。實際可刪 37；`maxRESTOperations` 489→452（若 40 條中 3 條留下）【推】。
4. **im／review 共用同一支「是否封鎖」會改語意**：review 現在也用雙向封鎖過濾評價作者（`review-service handler.go:43`），而簡報 D2 說店家封鎖「不影響評價」、買家封鎖店家不影響店家看評價。新的 legacy 查詢要能區分「個人封鎖（買家↔買家）」與「店家封鎖」，review 只能吃前者【推】；現有 proto 沒有這支 RPC，要新增 legacy proto。
5. **事件行為差異需 characterization**：`post.created` 在草稿建立時也會對追蹤者發、且排程或手動發布時不發（§2）；`seller.deleted` 不清討論區；`counter.delta` 的最終一致加 10 分鐘對帳在單體內應改成同交易。這些都是「照搬還是修正」的決定，屬沿用盤點。
6. **SSE 不能整包刪**：`G/sse`、`G/mq/consumer.go` 與 `G/realtime` 同時服務通知 SSE（`StreamNotifications`）；只刪 comment channel 與 5 個 comment.* 綁定。多實例單體的 streaming 扇出（process 內 vs 跨實例）簡報沒寫，A7-0 未涵蓋【未量】。
7. **seller-service → social**：簡報稱 `GetPostList`「沒人用」；實際 `certification.go:523` 有呼叫，但單體 seller 已刪除該前置條件（CARRYOVER.md:143），legacy seller-service 內的呼叫成為死碼；是否 legacy seller-service 仍會被啟動由別的 slice 判斷。
8. **未涵蓋**：四個 client（website、admin、buyer-app、seller-app）的呼叫點、10 張表的欄位／索引盤點、proto message 層的欄位對照、`G/handler` 內 DTO 轉換的欄位差異；`M/legacy/reviewservice` 是否已有 `HideReview` 沒開檔確認。
