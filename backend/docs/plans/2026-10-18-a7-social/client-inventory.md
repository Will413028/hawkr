# A7-0 四個 client 的 social 呼叫點盤點

> 2026-10-22：「去向」是 2026-10-18 的快照。S1、S3（決定簡報）之後，legacy SocialService 8 支、「是否封鎖」RPC、im／review 的 port、`/app/blocks*` 留到 A6、代行路由留到 A10 這幾種去向不再成立，以 `../2026-10-18-a7-social.md` 的步驟為準。

快照：main `172260d31`，只讀。標記：【量】＝跑過指令或打開檔案看過；【推】＝讀程式推論。
縮寫：W=`website`、A=`admin`、BA=`buyer-app/lib`、SA=`seller-app/lib`。`packages/` 沒有任何 social 呼叫（見 §1）。
去向的 service 名稱依 brief D1：`SocialPublicService`、`SocialAppService`、`SocialAdminService`；封鎖依 D2 放 buyer／seller 模組；檢舉依 D5′ 放新的 `moderation` 模組。brief 沒有替 `moderation` 的 service 命名，下面寫 `ModerationAppService`／`ModerationAdminService` 都是【推】，要等 A7 的 proto 設計定名。

## 1. 指令與命中數（可重跑，cwd = hawkr 根目錄）

| # | 指令 | 命中行數 |
|---|---|---|
| W1 | `rg -n "(/public/posts\|/app/posts\|/app/sellers/posts\|/app/sellers/me/posts\|/forum\|/sellers/forum\|/public/hashtags\|/public/tags/\|/app/blocks\|/app/reports\|/comments/stream\|/sellers/posts)" website --glob '!node_modules' --glob '!.next' --glob '!**/*.md'` | 117（含註解與測試） |
| W2 | 相對路徑形式（`apiClient.public/authed` 會補前綴，W1 抓不到）：`rg -n 'apiClient\.(public\|authed)\.' website/lib/api/posts.ts website/lib/seller/forum-repo.ts` | 10 |
| W3 | 代行 BFF：`rg -n '/admin/sellers/\$\{numericSegment\(storeId\)\}/posts' website/app` ；`rg -n POSTS_ROOT website/lib/seller-data/delegated-repos.ts` | 6；2 |
| W4 | 找封鎖／檢舉入口：`rg -n "封鎖\|blockUser\|unblock\|submitReport\|reportPost\|/app/blocks\|/app/reports" website/lib website/features website/app website/components -g '*.ts' -g '*.tsx'`，只看到法務頁文字與 CRM `blocklist`，沒有任何 REST 呼叫 | 0 個 REST 呼叫 |
| A1 | `rg -n "/api/v1/admin/(posts\|reports)" admin/src` | 5 |
| B1 | `rg -n "(/public/posts\|/app/posts\|/posts\\b\|/sellers/posts\|/me/posts\|/forum\|/public/hashtags\|/hashtags\|/tags/\|/blocks\|/app/reports\|/reports\\b\|comments/stream\|/comments\\b\|/sellers/forum)" buyer-app/lib`（排除 `.g.dart`） | 59 |
| S1 | `rg -n "(/public/posts\|/app/posts\|/posts\\b\|/sellers/posts\|/me/posts\|/forum\|/public/hashtags\|/hashtags\|/tags/\|/blocks\|/app/reports\|/reports\\b\|comments/stream\|/comments\\b\|/sellers/forum\|/admin/sellers)" seller-app/lib`（排除 `.g.dart`、`.freezed`） | 57（seller-app 的 repository 用 `_pub`／`_app` 常數拼路徑，另跑 S2） |
| S2 | `rg -n "_pub/\|_app/(sellers\|posts)\|_base" seller-app/lib/features/post/data seller-app/lib/features/report/data` | 見 §5 表格 |
| P1 | `rg -il "social\|comment\|report\|block\|post" packages --glob '!**/gen/**' --glob '!**/*.g.dart'`，人工看過，只有 transport、twaddr、auth 測試的字面巧合 | 0 個 social 呼叫【量】 |
| T1 | 測試：W `rg -l "(/public/posts\|/app/posts\|/app/sellers/posts\|/app/sellers/me/posts\|/forum\|/public/tags\|isMine\|comments/stream)" website/tests website/lib website/features --glob '*test*' --glob '*spec*'`；BA `rg -l "(/public/posts\|/app/posts\|/app/blocks\|/app/reports\|/forum\|/public/hashtags)" buyer-app/test`；SA `rg -l "(/api/v1/(public\|app)/(posts\|sellers\|blocks\|reports\|hashtags\|tags)\|forum)" seller-app/test` | W 16、BA 16、SA 11 檔 |

命中行數含註解；下面的表只列真正發出請求的程式行。

### 各端摘要【量】

| 端 | 發請求的檔案 | 不重複的 REST 路徑（method＋path） | 備註 |
|---|---|---|---|
| website | 9 個（lib/social/repo、lib/sse/useCommentStream、lib/seller/forum-repo、lib/seller/repo、lib/api/posts、lib/post-tags/repo、lib/search/repo、lib/api/cached、lib/seller-data/delegated-repos）＋4 個代行 BFF route | 29 個＋SSE 1 條＋代行 6 條；其中 1 條是 gateway 沒有的 `DELETE …/comments/{id}/like` | 無封鎖、無檢舉、無封鎖名單 |
| admin | 2 | 4（`GET/PATCH /admin/posts`、`GET /admin/reports`、`POST /admin/reports/{id}/resolve`） | `reopen`、`GET /admin/reports/{id}` 沒有 client 呼叫 |
| buyer-app | 5（social_repository、comment_stream_service、block_repository、merchant_chat_tab、category_repository）；`merchant_home_tab` 另外直接打 dio 2 條 | 23 個＋SSE 1 條 | 有封鎖與檢舉；沒有「刪除留言」UI |
| seller-app | 5（post_repository、post_hashtag_repository、hashtag_repository、report_repository、block_repository） | 23 個（含 `/public/hashtags`，那是商品 hashtag） | 沒有 SSE（留言不即時）【量】；沒有讀單篇貼文 |

## 2. website

### 2.1 呼叫點

| 檔案:行 | 請求 | 畫面／功能 | 去向 |
|---|---|---|---|
| `W/lib/api/posts.ts:71` | `GET /public/posts`（`page`、`page_size`、`tag`） | 首頁動態、feed、搜尋頁、MegaMenu、`/api/public/tab-bootstrap`（`fetchPublicPostsTyped`；呼叫者 `features/feed/FeedPage.tsx:50`、`features/search/SearchPage.tsx:34`、`lib/hooks/useFeedPosts.ts:169`、`features/home/server/home-tab-bootstrap-route.ts:188,263`、`app/api/public/tab-bootstrap/v1/head/route.ts:45`） | `SocialPublicService.ListPosts`（server component 用 Connect 的 server transport） |
| `W/lib/api/posts.ts:70` | `GET /public/sellers/{id}/posts` | 公開店面「貼文」分頁（`features/sellers/posts/SellerPostsPage.tsx:52`） | `SocialPublicService.ListPosts`（`seller` 篩選）或 `ListSellerPosts` |
| `W/lib/api/posts.ts:140` | `GET /public/posts/{id}` | feed 深連結載入單篇（`features/feed/FeedParamController.tsx:98`） | `SocialPublicService.GetPost` |
| `W/lib/api/cached.ts:534` | `GET /public/posts`（快取包裝的 endpoint 標籤） | 首頁快取 `getCachedPublicPosts` | 同 ListPosts |
| `W/lib/social/repo.ts:140` | `GET /public/posts/{id}/comments` | 貼文留言列表（`useSocial.ts:137,176`） | `SocialPublicService.ListComments` |
| `W/lib/social/repo.ts:155` | `GET /public/posts/comments/{id}/replies` | 展開回覆（`useSocial.ts:649`） | `SocialPublicService.ListReplies` |
| `W/lib/sse/useCommentStream.ts:229`（`SseClient`，`lib/sse/client.ts`） | SSE `GET /public/posts/{id}/comments/stream`；事件 `ready`（`bufferMiss`）、`comment.batch/created/replied/updated/deleted/like_changed`、heartbeat | 即時留言（`useSocial.ts:492`） | `SocialPublicService.StreamComments`（server streaming，Connect 在瀏覽器要用 fetch 串流；事件 oneof 對應上列六種；`bufferMiss` 改成 resume 失敗時的明確訊息） |
| `W/lib/social/repo.ts:178` | `POST /app/posts/{id}/comments`（`content`、`clientRequestId`） | 送出留言（`useSocial.ts:224`） | `SocialAppService.CreateComment`（身分由伺服器推導，D1） |
| `W/lib/social/repo.ts:209` | `POST /app/posts/comments/{id}/replies` | 回覆（`PostComments.tsx:172`） | `SocialAppService.ReplyToComment`（或 `CreateComment` 帶 `parent`） |
| `W/lib/social/repo.ts:239` | `PUT /app/posts/comments/{id}` | 編輯留言（`PostComments.tsx:344,609`） | `SocialAppService.UpdateComment`（`update_mask`） |
| `W/lib/social/repo.ts:224` | `DELETE /app/posts/comments/{id}` | 刪除留言（`useSocial.ts:259`、`PostComments.tsx:315`） | `SocialAppService.DeleteComment` |
| `W/lib/social/repo.ts:284-290` | `POST` 與 `DELETE /app/posts/comments/{id}/like`（`wasLiked` 時送 DELETE） | 留言按讚（`useSocial.ts:294`、`PostComments.tsx:298`） | `SocialAppService.LikeComment`／`UnlikeComment`（明確的兩個動作，不再 toggle；理由見 §7 發現 2） |
| `W/lib/social/repo.ts:253` | `POST /app/posts/{id}/like`（toggle） | 貼文按讚（`useSocial.ts:581`；`lib/api/posts.ts:159-185`；`PostCardV2.tsx:79`、`PostDetailViewer.tsx:124`、`HomeStageFeedCard.tsx:58`） | `SocialAppService.LikePost`／`UnlikePost` |
| `W/lib/social/repo.ts:265` | `POST /app/posts/{id}/share` | 分享計數（`lib/api/posts.ts:176`、`PostDetailViewer.tsx:318`） | `SocialAppService.SharePost`（計數 RPC；若 A7 決定不保留分享計數則刪） |
| `W/lib/seller/forum-repo.ts:144` | `GET /public/sellers/{id}/forum` | 公開店面「留言板」＋SSR（`features/sellers/messages/SellerMessagesPage.tsx:50`、`SellerMessageTab.tsx:44`、`useSellerForum.ts:129,169`） | `SocialPublicService.ListForumComments` |
| `W/lib/seller/forum-repo.ts:160` | `GET /public/sellers/forum/{id}/replies` | 展開討論區回覆（`useSellerForum.ts:197`） | `SocialPublicService.ListForumReplies` |
| `W/lib/seller/forum-repo.ts:173` | `POST /app/sellers/{id}/forum` | 發表討論區留言（`useSellerForum.ts:232`） | `SocialAppService.CreateForumComment` |
| `W/lib/seller/forum-repo.ts:184` | `POST /app/sellers/forum/{id}/reply` | 回覆（`useSellerForum.ts:279`） | `SocialAppService.ReplyToForumComment` |
| `W/lib/seller/forum-repo.ts:195` | `PUT /app/sellers/forum/{id}` | 編輯（`useSellerForum.ts:317`） | `SocialAppService.UpdateForumComment` |
| `W/lib/seller/forum-repo.ts:205` | `DELETE /app/sellers/forum/{id}` | 刪除（`useSellerForum.ts:346`） | `SocialAppService.DeleteForumComment` |
| `W/lib/seller/forum-repo.ts:215-219` | `POST`／`DELETE /app/posts/comments/{id}/like`（借用貼文留言的讚端點，檔內有 `TODO(seller-forum)`） | 討論區按讚（`useSellerForum.ts:371`） | `SocialAppService.LikeForumComment`／`UnlikeForumComment`（討論區留言自己的 RPC，刪掉借用） |
| `W/lib/seller/repo.ts:977` | `POST /app/sellers/posts` | 賣家中心新增貼文（`PostComposeClient.tsx:468`） | `SocialAppService.CreatePost`（D1：店主管理在 SocialAppService；需 `CapMarketing`，D3） |
| `W/lib/seller/repo.ts:988` | `PUT /app/sellers/posts/{id}` | 編輯（`PostComposeClient.tsx:451`） | `SocialAppService.UpdatePost`（需 `CapMarketing`） |
| `W/lib/seller/repo.ts:998` | `POST /app/sellers/posts/{id}/publish`、`…/unpublish` | 發布／下架（`SellerPostsClient.tsx:289`、`postPublishActions.ts`、`PostPublishControl.tsx:70`） | `SocialAppService.PublishPost`（`CapMarketing`）、`UnpublishPost`（`CapContentContract`） |
| `W/lib/seller/repo.ts:1002` | `DELETE /app/sellers/posts/{id}` | 刪除（`SellerPostsClient.tsx:255`） | `SocialAppService.DeletePost`（`CapContentContract`） |
| `W/lib/seller/repo.ts:1007` | `GET /app/sellers/posts/{id}` | 編輯頁、深連結（`PostEditClient.tsx:27`、`SellerPostsClient.tsx:243`） | `SocialAppService.GetMyPost`（`CapContentContract`，D3：停權也讀得到） |
| `W/lib/seller/repo.ts:1025` | `GET /app/sellers/me/posts` | 賣家中心動態列表（`SellerPostsClient.tsx:205`；`lib/seller-data/scope.tsx:218` 把 `sellerPostRepo` 當預設 repo） | `SocialAppService.ListMyPosts`（`CapContentContract`） |
| `W/lib/post-tags/repo.ts:145` | `GET /public/tags/suggest` | 貼文標籤建議（`PostComposeClient.tsx:280`） | `SocialPublicService.SuggestPostTags` |
| `W/lib/post-tags/repo.ts:175` | `GET /public/tags/trending` | MegaMenu、footer 熱門話題（`MegaMenuPanel.tsx:121`、`lib/api/cached.ts:620`、`FooterCategoryColumns.tsx:117`） | `SocialPublicService.ListTrendingPostTags` |
| `W/lib/search/repo.ts:234` | `GET /public/tags/trending`（舊的搜尋頁貼文標籤，檔內註解說已被取代） | `fetchTrendingHashtags`；`rg` 找不到呼叫者【量：`rg -n fetchTrendingHashtags website` 只有定義與 import 型別】 | 刪除：沒有呼叫者，且與 post-tags/repo.ts 重複 |
| `W/lib/search/repo.ts:284`、`W/lib/api/cached.ts:597` | `GET /public/hashtags/popular` | 搜尋頁、市集側欄、MegaMenu、首頁搜尋（`MegaMenuPanel.tsx:37`、`MarketProductsListPage.tsx:39`、`HomeCompactSearchExperience.tsx:112`、`SearchHashtagNav.tsx:18`、`PreviewListingPage.tsx:212`） | **不是 social**：檔內註解說是「商品 hashtag，依上架商品數排序」，與貼文標籤是分開的命名空間。去向 catalog／discovery 的 hashtag RPC【推】，A7 要和 backend 切片確認 openapi 的 `Hashtag` tag 兩支（`searchHashtags`、`getPopularHashtags`）是不是真的屬於 social |
| `W/app/(delegated)/adm/api/posts/[storeId]/route.ts:26,39` | `GET`／`POST /admin/sellers/{storeId}/posts`（gateway `routes_delegated.go:107,108`） | 代行工作區的貼文列表與新增 | **留在代行（delegation）流程**：A7 不動 BFF 路徑；後端要決定代行貼文由誰呼叫 `SocialAppService`（brief 的「social → delegation 代行稽核改成事件」）。標示 legacy 直到 delegation 搬遷【推】 |
| `W/app/(delegated)/adm/api/posts/[storeId]/[postId]/route.ts:20,32` | `GET`／`PATCH /admin/sellers/{storeId}/posts/{postId}`（`routes_delegated.go:109,110`） | 代行檢視與編輯 | 同上 |
| `…/[postId]/publish/route.ts:22`、`…/unpublish/route.ts:19` | `POST /admin/sellers/{storeId}/posts/{postId}/publish`、`…/unpublish`（`routes_delegated.go:111,112`） | 代行發布／下架 | 同上 |
| `W/lib/seller-data/delegated-repos.ts:36,521-600` | `POSTS_ROOT = "/posts"` 加上代行 fetcher | 代行 `SellerPostRepo` 實作 | 同上；`sellerPostRepo` 的介面（`lib/seller/repo.ts:913`）是兩邊共用，改成 Connect 時要保留這個介面縫 |

### 2.2 指定的入口與邏輯

| 項目 | file:line | 現況【量】 | 對 A7 的意義 |
|---|---|---|---|
| 留言權限（刪除／編輯） | `W/features/feed/components/commentPermissions.ts:13-24` | `canEditComment = isMine`；`canDeleteComment = canModerate \|\| isMine` | D4：改讀每則留言的 `viewer_can_edit`／`viewer_can_delete`，整支刪掉 |
| `canModerate` 寫死 | `W/features/seller-center/posts/SellerPostDetailModal.tsx:195`（`<PostComments postId canModerate />`）；`PostComments.tsx:112,259,487,847` | 「在賣家中心」＝店主，公開頁的店主 `canModerate=false` | D4／C4：店主在公開頁也要看到刪除；由伺服器告知 |
| 留言框頭像、提示字 | `PostComments.tsx:896-904`（`canModerate ? sellerIdentity… : identity…`）、`:958` placeholder「寫下你的留言…」、`:226` 回覆 placeholder | 看「在哪個畫面」決定店名還是會員 | C2／C3：改讀貼文的 `viewer_actor` |
| 送出瞬間的暫時作者 | `W/lib/hooks/useSocial.ts:206`（`isMine: true`）、`:224` | optimistic 先放會員身分 | D4：optimistic 也改用 `viewer_actor` |
| 回覆按鈕 | `PostComments.tsx:752-758`（`isLoggedIn && !editing` 才顯示「回覆」） | 登入就顯示，回覆限店家時其他人按了必失敗 | D4：改 `viewer_can_reply` |
| 發表留言框 | `PostComments.tsx:947`（`isLoggedIn &&`） | 同上 | D4：改 `viewer_can_comment` |
| 賣家端不公開貼文的留言說明 | `SellerPostDetailModal.tsx:80`（`postCommentsAvailability(post, storePublic)`）、`:170-173`（平台下架卡片，顯示 `hiddenReason`） | 手寫 availability | C8′：改讀伺服器狀態；店主不公開時也要讀得到留言 |
| 停權閘門 | `W/features/seller-center/components/pageCapabilities.ts:32,40`（`/seller-center/posts` 需 `C.MARKETING`）、`PostPublishControl.tsx:23,45,70`（停權時發布方向整顆不渲染，下架保留）、`SellerPostsClient.tsx:178-183,445,583`（`isSuspended` 傳給 control） | 已有「收縮類放行」的按鈕層，但**頁面層仍要 MARKETING**：停權進不了列表（brief D3 的死結） | C1′／D3：`pageCapabilities.ts:40` 改成 `CONTENT_CONTRACT`（或列表頁拆權限）；列表頁加提示；`pageCapabilities.unit.test.ts` 要跟著改 |
| 封鎖 | （不存在） | website 沒有任何封鎖使用者的入口與封鎖名單頁【量，W4】 | C5：賣家中心留言選單要新增「封鎖此人」（新入口）；需要封鎖名單頁嗎？brief 說「你可以隨時在『封鎖名單』解除」，但 website 沒有這頁，要請 Will 決定是否新增【新問題，見 §7】 |
| 檢舉 | （不存在） | website 沒有檢舉入口；`W/lib/codemeta/generated.ts` 有 `Report*` 表但沒有任何 `.tsx` 消費【量：`rg "ReportStatus\|REPORT_" website -g '*.ts*'` 只有 generated.ts 與 next.config.ts 的 CSP 字樣】 | 不需搬遷；若 D5′ 要求 website 也能檢舉就是新功能，brief 沒寫 |
| 回覆分頁、深連結 | `PostComments.tsx:866-893`（`?post=&comment=` 通知深連結輪詢 3 秒） | 依賴留言 id 為 TSID 字串 | 新 API 的 id 型別要保持字串（`tests/social-comment-tsid.test.ts` 鎖著） |

### 2.3 手寫的 enum／標籤表

| 位置 | 內容 | 去向 |
|---|---|---|
| `W/lib/codemeta/generated.ts:518-530`（`PostPublishStatus` ＋ `POST_PUBLISH_STATUS_LABEL`） | 4401/4402/4403 → 未發布／已發布／已下架 | CodeMeta 凍結；A7 新 proto 的發布狀態 enum 進共用文案目錄，刪掉這段 |
| `W/lib/codemeta/generated.ts:1054-1070`、`2220-2290`（`ReportStatus`、`ReportTargetType`、`ReportReasonCode`、`ReportResolveAction` ＋標籤） | 檢舉狀態、對象、原因、處分標籤；website 內無 `.tsx` 消費，只有 admin 的同款生成檔有人用 | 同上，隨 D5′ 的 `moderation` proto 重做 |
| `W/features/seller-center/posts/postPublishActions.ts:42-66`（`POST_PUBLISH_ACTION_LABEL`、`POST_PUBLISH_PENDING_LABEL`、`POST_PUBLISH_CONFIRM_TEXT`、`…SCHEDULED_CONFIRM_TEXT`） | 動作與確認文案（註解說刻意不取 CodeMeta Label） | 進共用文案目錄；`postPublishActions.unit.test.ts` 跟著改 |
| `W/features/seller-center/posts/PostDraftBadge.tsx:2,27,49` | 用 `POST_PUBLISH_STATUS_LABEL` 組徽章 | 跟上列一起換 |
| `W/features/seller-center/posts/postHidden.ts:3` 與 `SellerPostDetailModal.tsx:170-173` | 平台下架判斷（`hiddenAt > 0`）與文案「此貼文已被平台下架」 | D5′／C9：平台下架變成措施「內容下架」，文案由措施頁負責；`hiddenReason` 欄位語意要重新定 |
| `W/features/seller-center/customers/CustomersClient.tsx:39,53`、`CustomerDetailPanel.tsx:30`、`coupons/CouponBuyerPicker.tsx:39,44`、`lib/seller/repo.ts:263,311,367,377,409` | CRM `blocklist`／「黑名單」 | **不是 social**；CRM 到 A6 才併入店家封鎖（C10）。A7 不動，列入 §6 清單 |

## 3. admin

### 3.1 呼叫點

| 檔案:行 | 請求 | 畫面 | 去向 |
|---|---|---|---|
| `A/src/app/admin/reports/page.tsx:145` | `GET /api/v1/admin/reports?status=&target_type=&reason_code=…` | 檢舉單列表（側欄 `components/Layout/Sidebar.tsx:33`） | D5′：後台佇列提升到平台層（10e D7），`ModerationAdminService.ListReports`【推】；與 seller 治理佇列整合，頁面形狀可能改 |
| `A/src/app/admin/reports/page.tsx:155` | `POST /api/v1/admin/reports/{id}/resolve`（`action`＝`HIDE_TARGET`／`WARN_USER`／`BAN_USER`／`DISMISS`、`note`） | 裁決對話框 | `ModerationAdminService.ResolveReport`。**D5′ 直接改這個畫面**：對象是店家時不能再 `WARN_USER`／`BAN_USER`，要走措施；對象是買家的 `BAN_USER` 在帳號底下有店時被拒 |
| （無呼叫） | `GET /admin/reports/{id}`（`adminGetReport`）、`POST /admin/reports/{id}/reopen`（`adminReopenReport`） | 【量：`rg -n "reopen\|admin/reports/" admin/src` 只有 list／resolve】 | 刪除 `GetReport`／`Reopen` 的 admin 入口或在 proto 保留但不做 UI；申覆重開由 D5′／10e 的措施申覆取代【推】 |
| `A/src/app/admin/posts/page.tsx:75-79` | `GET /api/v1/admin/posts?page=&page_size=&status=` | 貼文管理列表（`Sidebar.tsx:42`；wire 型別 `A/src/lib/posts/adminList.ts`） | `SocialAdminService.ListPosts` |
| `A/src/app/admin/posts/page.tsx:100` | `PATCH /api/v1/admin/posts/{id}`（`action: HIDE\|RESTORE`、`reason`、`reasonCode`） | 下架／恢復對話框（`:350-`） | **D5′ 衝擊**：店家貼文的平台下架要變成措施「內容下架」，`HIDE` 不能再是直接寫 `hidden_at`；`SocialAdminService.HidePost`／`RestorePost` 只留給買家內容，或整頁併入措施建立流程【推，brief 沒說後台貼文頁怎麼收】 |
| `A/src/app/admin/violation-reports/**`、`/admin/violations/new` | `/admin/violation_reports`、`/admin/food_safety_reports/*`、`/admin/violations` | 賣家違規／食安通報 | **不是 social**（seller 治理），列出只為排除；brief D5′ 第 3 點說店家層級檢舉併入這條線 |

### 3.2 手寫表與測試

| 位置 | 內容 | 去向 |
|---|---|---|
| `A/src/app/admin/reports/page.tsx:60-66` `STATUS_BADGE_VARIANT`、`:72-75` `ACTIONABLE_STATUSES`、`:83-88` `ACTION_DESCRIPTIONS`（註解說是操作說明不是碼表） | 狀態顏色、可裁決狀態、每個處分的副作用說明（文字含「寫入 hidden_at」「登入立即失效」） | 顏色表留 client；`ACTION_DESCRIPTIONS` 的副作用文字隨 D5′ 改寫後進共用文案目錄 |
| `A/src/app/admin/reports/page.tsx:9-20`、`A/src/app/admin/posts/page.tsx:9,418` | 從 `@/lib/codemeta` 取 `REPORT_*_LABEL`（`A/src/lib/codemeta/generated.ts`） | 同 W：換成 ui-text 目錄 |
| `A/src/app/admin/posts/page.tsx:31-35` `TAB_LABELS`、`:43-53` `PostStatusBadge`；`A/src/lib/posts/adminList.ts:20`（`AdminPostStatus = published\|hidden\|deleted`） | 貼文狀態字串（小寫，非 proto enum） | 新 proto enum 後進共用文案目錄 |
| 測試 | `A/tests/admin-post-list-media-contract.test.ts`（鎖 `/admin/posts` 欄名，含 `hidden_reason`）；`A/tests/codemeta-no-second-copy.test.ts:586-931`（鎖報告頁用 `ReportStatus` 六值，含 `APPEALED`）；`A/tests/measure-pages-wiring.test.ts:86` | 都要隨 A7 重寫或刪除 |

## 4. buyer-app

### 4.1 呼叫點

| 檔案:行 | 請求 | 畫面／功能 | 去向 |
|---|---|---|---|
| `BA/features/social/data/social_repository.dart:29-32` | `GET /public/sellers/{id}/posts`、`GET /public/posts` | `getPosts`：首頁動態預覽（`home/data/home_post_preview_repository.dart:24`）、貼文 feed（`social/presentation/post_feed_controller.dart:27,43`）、商家貼文頁 | `SocialPublicService.ListPosts` |
| `…social_repository.dart:178` | `GET /public/posts/{id}` | 貼文詳情（`post_detail_controller.dart:26`）；`seller/presentation/widgets/merchant_home_tab.dart:309` 另外直接用 dio 打同一條取 `comment_count` | `SocialPublicService.GetPost`；`merchant_home_tab` 的直打改走同一個 client |
| `…social_repository.dart:226` | `POST /app/posts/{id}/like` | 貼文按讚（`post_feed_controller.dart:87`、`post_detail_controller.dart:55`、`merchant_posts_feed_screen.dart:235`）；`merchant_home_tab.dart:269` 另直打 dio | `SocialAppService.LikePost`／`UnlikePost` |
| `…social_repository.dart:242,372` | `GET /public/posts/{id}/comments`（`getComments` 與 `getCommentsRaw` 兩份） | `post_detail_controller.dart:27`；`shared/widgets/comments_bottom_sheet.dart:204` | `SocialPublicService.ListComments`；兩份合一 |
| `…social_repository.dart:269,426` | `POST /app/posts/{id}/comments`（`addComment`、`addCommentRaw`） | `post_detail_controller.dart:36`；`comments_bottom_sheet.dart:320` | `SocialAppService.CreateComment` |
| `…social_repository.dart:286,476` | `POST /app/posts/comments/{id}/like`（`likeComment`、`likeCommentRaw`） | `post_detail_controller.dart:79`；`comments_bottom_sheet.dart:670` | `SocialAppService.LikeComment`／`UnlikeComment` |
| `…social_repository.dart:303` | `DELETE /app/posts/comments/{id}`（`deleteComment`） | **沒有任何呼叫者**【量：`rg -n "deleteComment" buyer-app/lib` 只有定義】 | C4 新入口時才會用；現在是死碼，A7 重寫時直接用新的 client |
| `…social_repository.dart:320,492` | `GET /public/posts/comments/{id}/replies`（`getCommentReplies`、`…Raw`） | 只有 `…Raw` 被 `comments_bottom_sheet.dart:283` 用；`getCommentReplies` 無呼叫者 | `SocialPublicService.ListReplies`；無呼叫者的那份刪除 |
| `…social_repository.dart:352,457` | `POST /app/posts/comments/{id}/replies`（`replyToComment`、`…Raw`） | 只有 `…Raw` 被 `comments_bottom_sheet.dart:320` 用 | `SocialAppService.ReplyToComment` |
| `BA/features/social/data/comment_stream_service.dart:72`（事件處理 `:121` 起） | SSE `GET {apiBaseUrl}/public/posts/{id}/comments/stream` | 即時留言（`comments_bottom_sheet.dart:136`；用自寫 `SseClient` 與 per-post `Last-Event-ID` 快取） | `SocialPublicService.StreamComments`（Dart Connect 的 server streaming） |
| `…social_repository.dart:559` | `POST /app/reports`（`submitReport`） | 檢舉面板（`moderation/presentation/report_sheet.dart:57`） | `ModerationAppService.SubmitReport`【推】 |
| `…social_repository.dart:596` | `GET /app/reports/me`（`getMyReports`） | 我的檢舉（`moderation/presentation/my_reports_screen.dart:43`，路由 `core/routing/app_router.dart:347`） | `ModerationAppService.ListMyReports` |
| `BA/features/moderation/data/block_repository.dart:20` | `POST /app/blocks/{userId}` | 封鎖確認（`block_confirm_dialog.dart:33`） | D2：買家發起，放 buyer 模組：`BuyerAppService.BlockBuyer`／`BlockSeller`（kind 明確，見 §7 發現 1） |
| `…block_repository.dart:39` | `DELETE /app/blocks/{userId}` | 解除（`blocked_users_screen.dart:72`） | `BuyerAppService.Unblock…` |
| `…block_repository.dart:62` | `GET /app/blocks`（只回 id、blocked_id、create_time_ms，沒名稱，`blocked_users_screen.dart:162`） | 封鎖名單頁（`blocked_users_screen.dart:43`，路由 `app_router.dart:351`，入口 `settings_screen.dart:585`） | `BuyerAppService.ListBlocks`（新回應要帶 kind＋名稱） |
| `BA/features/seller/presentation/widgets/merchant_chat_tab.dart:138` | `GET /public/sellers/{id}/forum` | 店家頁「留言板」 | `SocialPublicService.ListForumComments` |
| `…merchant_chat_tab.dart:172` | `GET /public/sellers/forum/{id}/replies` | 展開 | `SocialPublicService.ListForumReplies` |
| `…merchant_chat_tab.dart:218` | `POST /app/sellers/{id}/forum` | 發表 | `SocialAppService.CreateForumComment` |
| `…merchant_chat_tab.dart:210` | `POST /app/sellers/forum/{id}/reply` | 回覆 | `SocialAppService.ReplyToForumComment` |
| `…merchant_chat_tab.dart:242` | `PUT /app/sellers/forum/{id}` | 編輯 | `SocialAppService.UpdateForumComment` |
| `…merchant_chat_tab.dart:335` | `DELETE /app/sellers/forum/{id}` | 刪除（`isMine` 才有選單，`:885,923,957`） | `SocialAppService.DeleteForumComment` |
| `…merchant_chat_tab.dart:285` | `POST /app/posts/comments/{id}/like`（借用） | 討論區按讚 | `LikeForumComment`／`UnlikeForumComment` |
| `BA/features/seller/presentation/widgets/merchant_home_tab.dart:132` | `GET /public/sellers/{id}/posts` | 店家首頁貼文格 | `SocialPublicService.ListPosts` |
| `BA/features/category/data/category_repository.dart:237` | `GET /public/hashtags/popular` | 分類頁熱門標籤（`category_screen.dart:186`） | 商品 hashtag，非 social【推】；同 W |

### 4.2 指定的入口

| 項目 | file:line | 現況【量】 | 對 A7 的意義 |
|---|---|---|---|
| 從貼文封鎖店家（詳情） | `BA/features/social/presentation/post_detail_screen.dart:226-233`（`showReportBlockActionSheet`，`blockUserId: sellerId`） | 檢舉＋封鎖共用選單（`moderation/presentation/report_block_action_sheet.dart:17-80`，封鎖字串 `:71`「封鎖此使用者」） | D2：改送店家代碼，C7a 文案 |
| 從貼文封鎖店家（feed） | `merchant_posts_feed_screen.dart:562-571` | 同上，`blockUserId: sellerId` | 同上 |
| 從商品評價封鎖評價者 | `BA/features/product/presentation/merchant_item_detail_screen.dart:2567-2576`（`ReportTargetType.review`、`blockUserId: reviewUserId`） | 評價屬 review（A6） | **留 legacy 到 A6**：D2 說評價入口「送對方的內部編號（舊契約）」，legacy server 接住舊 RPC；但 kind＝買家，社群封鎖效果表的「買家→買家」 |
| 聊天封鎖 | `BA/features/chat/presentation/chat_thread_screen.dart:955-959`（`showBlockConfirmDialog(userId: merchantId)`）；檢舉 `:939`（`ChatMessage`） | 聊天屬 im（A6） | **留 legacy 到 A6**；封鎖寫入面在 A7 先搬到 buyer 模組，聊天只讀 |
| 封鎖確認文案 | `BA/features/moderation/presentation/block_confirm_dialog.dart:53-62`（標題、「封鎖後對方將無法傳送訊息給您，您也無法傳送訊息給對方。可隨時在『封鎖名單』解除。」） | 對買家與店家共用一句 | C7a／C7b：依對象 kind 分兩句 |
| 檢舉面板 | `BA/features/moderation/presentation/report_sheet.dart:57`（送出）、`:271`（`showReportSheet`） | 入口：貼文、評價、聊天訊息；**留言與討論區沒有檢舉入口**【量：`rg "showReport" buyer-app/lib` 只有 4 處（post_detail、merchant_posts_feed、merchant_item_detail、chat_thread）】 | 留言／討論區檢舉入口是否補，brief 沒寫【新問題】 |
| 留言權限、回覆按鈕 | `BA/shared/widgets/comments_bottom_sheet.dart:1005-1024,1165-1176`（每則留言一律有「回覆」）、`:342`（送出後本地補 `isMine`）、`:1285`（`isMine` 時換成自己的名字）；沒有刪除選項【量：該檔無 delete／canDelete】 | 登入就有回覆，無刪除 | D4／C4：改讀 `viewer_can_*`；C2／C3 留言框提示（目前 `:898` 只有「寫下回覆…」） |
| 討論區的刪除／編輯 | `merchant_chat_tab.dart:885,891,923,957`（`node.isMine` 才有長按選單） | 只有自己的 | 店主在 buyer-app 刪別人留言是 C4 新入口；討論區店主刪別人留言要看 `viewer_can_delete` |
| 封鎖名單頁 | `BA/features/moderation/presentation/blocked_users_screen.dart`；入口 `settings_screen.dart:585` | 列出 id，名稱缺 | 新 `ListBlocks` 要帶名稱與 kind |

### 4.3 手寫的 enum／標籤表

| 位置 | 內容 | 去向 |
|---|---|---|
| `BA/core/codemeta/code_meta.dart:1004`（`PostPublishStatus`）、`:2024`（`ReportStatus`）、`:4430`（`ReportTargetType`）、`:4471`（`ReportReasonCode`）等 | 生成檔 | CodeMeta 凍結，隨 A7 換 |
| `BA/features/moderation/data/report_target_type.dart:15-75`：`reportReasonsFor`（user 不含食安）、`reportReasonOptionLabel`（`other` 加「（請說明）」）、`reportReasonLabel`、`reportTargetTypeLabel`、`reportStatusLabel`（檢舉人視角：審核中／申訴中／已處理（已隱藏）…） | 手寫的狀態標籤，註解說「碼表 label 是治理後台視角」 | 進共用文案目錄（audience 變體）；`reportReasonsFor` 的適用規則改由 `moderation` proto 表達或留 client |
| `BA/features/moderation/presentation/report_sheet.dart`（版面文字）、`block_confirm_dialog.dart:53-91`（封鎖文案）、`report_block_action_sheet.dart:38,71`（「檢舉」「封鎖此使用者」） | 面板與選單字串 | 進 ARB／文案目錄（C7a／C7b 核可後） |

## 5. seller-app

### 5.1 呼叫點（全在 `SA/features/post/data/post_repository.dart`，常數 `_pub='/api/v1/public'`、`_app='/api/v1/app'`，`:40-41`）

| 檔案:行 | 請求 | 畫面／功能 | 去向 |
|---|---|---|---|
| `post_repository.dart:222` | `GET /app/sellers/me/posts` | 貼文列表（`post_controller.dart:13,19,29,69`；`widgets/page_home_tab.dart:156,181`；`PostRoutes.feed`） | `SocialAppService.ListMyPosts`（`CapContentContract`，停權進得去，D3／C1） |
| `:267` / `:272` | `POST /app/sellers/posts/{id}/publish`／`unpublish` | `post_controller.dart:64,66`；`page_home_feed_screen.dart:500-502`（`canPublish`＝`MARKETING`、`canUnpublish`＝`CONTENT_CONTRACT`，已逐能力判斷） | `PublishPost`（`CapMarketing`）、`UnpublishPost`（`CapContentContract`） |
| `:276` | `DELETE /app/sellers/posts/{id}` | `post_controller.dart:45`；`page_home_feed_screen.dart:248` | `DeletePost` |
| `:297` | `POST /app/sellers/posts` | 新增（`post_compose_screen.dart:205`） | `CreatePost`（`CapMarketing`） |
| `:533` | `PUT /app/sellers/posts/{id}` | 編輯（`post_edit_screen.dart:179`、`post_controller.dart:87`） | `UpdatePost` |
| `:342` | `GET /public/posts/{id}/comments`（`listComments`、`_listCommentsRaw`） | 留言預覽、feed 留言、底部抽屜（`page_home_tab.dart:676`、`page_home_feed_screen.dart:113`、`shared/widgets/comments_bottom_sheet.dart:132`） | D3／C8：店主讀自己貼文的留言**不能用公開端點**（不公開時讀不到），改 `SocialAppService.ListComments`（店主身分，`CapContentContract`）；公開貼文可共用 `SocialPublicService.ListComments` |
| `:420` | `GET /public/posts/comments/{id}/replies` | 展開（`page_home_feed_screen.dart:130`、`comments_bottom_sheet.dart:499`） | 同上，`ListReplies` |
| `:375` | `POST /app/posts/{id}/comments` | 店主留言（`page_home_feed_screen.dart:453`、`comments_bottom_sheet.dart:196`） | `SocialAppService.CreateComment`（`CapMarketing`） |
| `:405` | `POST /app/posts/comments/{id}/replies` | 店主回覆（`page_home_feed_screen.dart:445`、`comments_bottom_sheet.dart:169`） | `ReplyToComment`（`CapMarketing`） |
| `:389` | `POST /app/posts/comments/{id}/like` | `comments_bottom_sheet.dart:545,576` | `LikeComment`／`UnlikeComment`（`CapMarketing`，D3 新增類） |
| `:435` | `DELETE /app/posts/comments/{id}` | 店主刪留言（`page_home_feed_screen.dart:475`、`comments_bottom_sheet.dart:253`） | `DeleteComment`（`CapContentContract`） |
| `:443` | `GET /public/sellers/{id}/forum` | 「迴響」分頁（`widgets/page_echo_tab.dart:226`） | `SocialPublicService.ListForumComments`（店主也走這條；若店家不公開仍要看得到，D3 讀取放行 → 需要 App 版 `SocialAppService.ListForumComments`【推】） |
| `:457` | `GET /public/sellers/forum/{id}/replies` | `page_echo_tab.dart:270` | 同上 |
| `:471` | `POST /app/sellers/forum/{id}/reply` | 店主回覆（`page_echo_tab.dart:958`） | `ReplyToForumComment`（`CapMarketing`） |
| `:482` | `PUT /app/sellers/forum/{id}` | 編輯回覆（`page_echo_tab.dart:957`） | `UpdateForumComment`（`CapMarketing`） |
| `:489` | `DELETE /app/sellers/forum/{id}` | 刪（`page_echo_tab.dart:777`） | `DeleteForumComment`（`CapContentContract`） |
| `SA/features/post/data/post_hashtag_repository.dart:45` | `GET /api/v1/public/tags/suggest` | 貼文標籤自動完成（`post_hashtag_text_form_field.dart:135`） | `SocialPublicService.SuggestPostTags` |
| `SA/features/product/data/hashtag_repository.dart:35` | `GET /api/v1/public/hashtags` | 商品 hashtag（`shared/widgets/hashtag_autocomplete_popover.dart:140`） | 非 social【推】 |
| `SA/features/report/data/report_repository.dart:26` / `:71` | `POST /api/v1/app/reports`、`GET …/reports/me?page&page_size` | 檢舉面板（`widgets/report_sheet.dart:74`）、我的檢舉（`report_list_controller.dart:47`，路由 `app_router.dart:215`，入口 `profile_other_options_card.dart:86`） | `ModerationAppService.SubmitReport`／`ListMyReports`【推】 |
| `SA/features/report/data/block_repository.dart:26,37,48` | `POST`／`DELETE /api/v1/app/blocks/{userId}`、`GET /api/v1/app/blocks` | `block_actions.dart:50`、`report_list_controller.dart:141,209`、封鎖名單 `block_list_screen.dart`（路由 `app_router.dart:218`，入口 `profile_other_options_card.dart:98`） | D2：店家發起，放 seller 模組：`SellerAppService.BlockBuyer`／`Unblock…`／`ListBlocks`【推，名稱】 |
| `SA/features/post/data/post_repository.dart:494` | `GET /api/v1/app/share_url?type=post\|product` | 分享（`page_home_feed_screen.dart:215`、`page_shop_tab.dart:1056`） | **不是 social 的 42 個 operation**；跨商品與貼文的分享連結，屬 content／discovery【推】，A7 要有人認領 |

SA 沒有任何 `GET /public/posts` 或 `GET /public/posts/{id}`（單篇貼文只用列表資料）；沒有 SSE【量：S1 無 `comments/stream`，`rg -n "SseClient\|EventSource\|comments/stream" seller-app/lib` 只有 AI 助手與商品 listing 聊天】。

### 5.2 指定的入口

| 項目 | file:line | 現況【量】 | 對 A7 的意義 |
|---|---|---|---|
| 停權路由閘門 | `SA/core/router/route_capabilities.dart:63-66`（`PostRoutes.mediaPicker/compose/edit/feed` 全部 `_marketing`）；`:85-86`（`myReports`、`blockList` 為 `null`＝不需能力） | 停權進不了貼文列表（D3 死結）；封鎖名單、我的檢舉不受停權影響 | C1：`PostRoutes.feed` 改 `CONTENT_CONTRACT`（compose／edit／mediaPicker 維持 `MARKETING`）；`route_capabilities_test.dart` 跟著改 |
| 貼文頁內的逐能力判斷 | `page_home_feed_screen.dart:496-502` | 已分 `canPublish`／`canUnpublish` | 保留，與伺服器一致 |
| 留言選單：刪除、檢舉，**沒有封鎖** | `page_home_feed_screen.dart:1900`（`_isSelf = comment.isMine`）、`:1913`（註解「公開留言不帶留言者 id，目前沒有封鎖入口」）、`:1921`（delete）、`:1929`（report）、`:1941`（`ReportTargetType.postComment`） | 身分第 1 刀拿掉的入口 | C5：A7-4 恢復「封鎖此人」，送留言參照 |
| 討論區選單 | `widgets/page_echo_tab.dart:540-560`（只有「檢舉留言」，`ReportTargetType.sellerForum`、`:552`）；刪除 `:777` | 無封鎖 | C5：恢復封鎖 |
| 聊天封鎖／檢舉（留 legacy） | `SA/features/chat/presentation/chat_thread_screen.dart:1857`（檢舉，`chatMessage`）、`:1877`（`blockUserWithConfirm`） | 唯一還在用 `block_actions.dart` 的地方【量：`rg -n blockUserWithConfirm seller-app/lib`】 | **留 legacy 到 A6**；`block_actions.dart:15-40` 的 C6 文案要改 |
| 封鎖確認文案 | `SA/features/report/presentation/widgets/block_actions.dart:12-22`（「封鎖後，對方將無法傳送訊息或留言給你；你可以隨時在『封鎖名單』解除封鎖。」）、解除 `:26-35` | 說明寫成單向（brief 已指出） | C6 |
| CRM「黑名單」 | `SA/features/customer/presentation/customer_management_screen.dart:316`（分頁）、`:442`；`customer_detail_screen.dart:79,206,223-226,394`（設／解除黑名單）；`customer_repository.dart:38-39`（`customer_type` 篩選）、`:111-115`（`PATCH` body `customerType`）、`:314-329`、`customer_model.dart:31` | 只是 CRM 標籤，沒有封鎖效果【推】 | **留 legacy 到 A6**（C10）；不是 social 的呼叫，A7 不動 |
| 留言可用性（C8） | `SA/features/post/data/post_comments_availability.dart:8-44`（`storeNotPublic`、`notPublished`、`platformHidden`；用在 `page_home_tab.dart:669`、`page_home_feed_screen.dart:491`）、`post_hidden.dart:8-22` | 不公開時**不呼叫**留言 API，改顯示說明 | C8：不公開時也讀得到、刪得了；這個 enum 與判斷要重寫，文案進目錄 |

### 5.3 手寫的 enum／標籤表

| 位置 | 內容 | 去向 |
|---|---|---|
| `SA/core/codemeta/code_meta.dart`（`PostPublishStatus`、`ReportStatus`、`ReportTargetType`、`ReportReasonCode` 生成檔） | 生成 | 隨 A7 換 |
| `SA/features/report/data/report_model.dart:50-100`（`reportReasonsFor`、`reportReasonLabel`、`reportTargetTypeLabel`、`reportStatusLabel`） | 與 buyer-app 同款手寫（兩端文案刻意一致，註解寫「用字與買家 App 的我的檢舉一致」） | 兩端合併成共用文案目錄的一份 |
| `SA/features/post/data/post_publish.dart:20-45`（`kPostPublishLabel`、`kPostUnpublishLabel`、確認句、`kPostPlatformHiddenLabel`…） | 發布／下架文案 | 進共用文案目錄 |
| `SA/features/post/data/post_comments_availability.dart:15-20` | 四個說明句 | 進目錄（C8 改句） |
| `SA/features/report/presentation/widgets/block_actions.dart:12-35` | 封鎖／解除文案 | C6，進目錄 |

## 6. 在 im／review／chat／CRM 內、留 legacy 到 A6 的呼叫點

| 端 | 位置 | 內容 | 為何留 |
|---|---|---|---|
| BA | `chat/presentation/chat_thread_screen.dart:939`（檢舉聊天訊息）、`:955`（封鎖） | 聊天 | im 到 A6 |
| BA | `product/presentation/merchant_item_detail_screen.dart:2567`（評價檢舉＋封鎖評價者） | 評價 | review 到 A6 |
| SA | `chat/presentation/chat_thread_screen.dart:1857,1877` | 聊天 | im 到 A6 |
| SA | `customer/**`（CRM「黑名單」，見 §5.2） | CRM | crm 到 A6 |
| W | `features/seller-center/customers/**`、`coupons/CouponBuyerPicker.tsx`、`lib/seller/repo.ts:263-409`（`blocklist`） | CRM | 同上 |
| 全部 | `POST /app/reports` 的 `targetType` 為 `REVIEW`、`CHAT_MESSAGE`（BA／SA 的 report_sheet 共用） | 一個 API 同時收 social 與 im／review 的對象 | D5′：`moderation` 收件，隱藏 im／review 內容經 legacy client；client 端 API 呼叫不變，只換 service |

注意：封鎖的**寫入**面在 A7 搬到 buyer／seller 模組後，這些 legacy 入口要改呼叫新的 block RPC，但 `userId` 的語意（買家或店家）由這些 legacy 入口決定，A7 要把 kind 補進入口參數（見 §7 發現 1）。

## 7. 測試與 mock（編碼了 social REST 形狀）

| 端 | 檔案 | 編碼的內容 |
|---|---|---|
| W | `tests/e2e/helpers/mock-gateway.ts`、`tests/e2e/helpers/proxy-public-gateway.ts` | **不含 social 路徑**【量：`rg -n "posts\|comments\|forum\|blocks" website/tests/e2e/helpers/mock-gateway.ts` 0 筆】 |
| W | `tests/e2e/protected-dynamic-routes.spec.ts:1063,1078`（`GET /public/posts/{id}`）、`tests/e2e/reliability.spec.ts:738-751`（`/public/posts?page&pageSize`）、`tests/e2e/public-static-routes.spec.ts:354-355`、`tests/e2e/seller-post-media-resilience.spec.ts:70`、`tests/e2e/seller-dashboard-onboarding-actions.spec.ts:99`（`/app/sellers/me/posts`） | Playwright route 攔截的 social 路徑 |
| W | `tests/e2e-api-proxy.test.ts:300`（`/public/tags/trending`）、`tests/seller-preview-ssr-production.test.ts:101,529`（forum）、`tests/buyer-avatar-contract.test.ts`（留言 wire 形狀，import `useSocial`、`useCommentStream`）、`tests/social-comment-ismine.test.ts`、`tests/social-comment-tsid.test.ts`、`tests/media-objectkey-contract.test.ts:348`（forum record）、`tests/post-comment-heart-parity.test.ts`、`tests/post-hidden-badge.test.ts`、`tests/post-publish-status-badge.test.ts`、`tests/seller-post-publish-control.test.ts` | wire 形狀契約測試 |
| W | `lib/seller-data/seller-wire-contract.unit.test.ts:102,108`、`lib/seller/seller-post-detail.unit.test.ts:43,58`、`lib/seller-data/repo-injection.unit.test.ts`、`lib/api/session-ended.unit.test.ts:9,157-158`（連 `toggleCommentLike` 的 `POST`／`DELETE` 都測）、`features/feed/components/__tests__/commentPermissions.unit.test.ts`、`features/seller-center/components/__tests__/pageCapabilities.unit.test.ts`、`features/seller-center/posts/__tests__/{postHidden,postPublishActions}.unit.test.ts`、`SellerPostDetailModal.unit.test.ts`、`SellerPostsClient.unit.test.ts`、`PostEditClient.unit.test.ts`、`post-draft.unit.test.ts`、`compose-actions.unit.test.ts` | 單元測 |
| W | storybook：`rg -l "PostComments\|CommentsBottomSheet" website --glob '*.stories.*'` 0 筆 | 無 |
| A | `tests/admin-post-list-media-contract.test.ts`、`tests/codemeta-no-second-copy.test.ts`、`tests/measure-pages-wiring.test.ts:86` | 見 §3.2 |
| BA | `test/features/moderation/{block_repository,blocked_users_screen,block_model,report_record_model,report_status_label,report_target_type,report_block_codes,report_sheet_layout,my_reports_screen,frosted_more_entry_points}_test.dart`；`test/features/social/{post_detail_screen,merchant_posts_feed_screen,post_detail_not_found,post_comment_avatar_contract,post_contract_no_legacy_keys,social_model_tsid,social_repository_submit_report,social_repository_related_product_image}_test.dart`；`test/shared/widgets/comments_bottom_sheet_identity_test.dart:47,165,235`、`accessibility_control_inventory_test.dart`；`test/features/seller/merchant_chat_tab_ownership_test.dart:50,186,218,266-268`；`test/shared/utils/avatar_wire_test.dart:11`；`test/features/home/home_latest_posts_*_test.dart`；`test/features/category/category_repository_filter_contract_test.dart:163`；`test/integration/hermetic_app_smoke_test.dart:42-76`、`hermetic_home_product_flow_test.dart:56` | Dio adapter 假回應（`options.path`）編碼 REST；無即時留言的測試【量：`rg -n comment_stream buyer-app/test` 0 筆】 |
| BA | `test/architecture/dead_gateway_endpoint_guardrail_test.dart:17-26` | 禁止出現 `DELETE /app/posts/comments/{id}/like`、`/comments/count`、`/likes/count` 等 gateway 沒有的路由；A7 搬完要改成「禁止任何手寫 social REST」 |
| SA | `test/features/post/{post_repository_owner_reads,my_posts_product_enrichment,page_home_tab_sliver_grid}_test.dart`（`/api/v1/app/sellers/me/posts`）、`test/features/media/{buyer_avatar_key_contract,dead_endpoint_guard}_test.dart`（forum record、`/like_status`、`/like_count` 死端點）、`test/features/report/{block_model_contract,block_list_unblock_id,report_target_type_contract,report_block_error_dispatch,report_status_label}_test.dart`、`test/core/networking/page_envelope_contract_test.dart:70`（`POST /app/reports` 回應）、`test/features/post/{post_comments_availability,page_home_feed_comments_availability,page_home_feed_comment_actions,page_home_feed_management,page_echo_input,page_echo_empty_state,page_home_tab_preview_comments,page_home_tab_owner_surface,post_mutation_rollback,post_mutation_error_widget,post_compose_image_required,post_gif_upload_contract}_test.dart`、`test/core/router/route_capabilities_test.dart` | 同上；`route_capabilities_test.dart` 要隨 C1 改 |

## 8. 最重要的發現（brief 沒涵蓋或與 brief 描述不完全一致）

1. **封鎖的 `userId` 在三個入口有三種語意**【量】：buyer-app 從貼文送 `sellerId`（`post_detail_screen.dart:229`）、從評價送 `reviewUserId`（`merchant_item_detail_screen.dart:2573`）、從聊天送 `merchantId`（`chat_thread_screen.dart:957`）；seller-app 聊天送對方買家 id。全部打同一條 `POST /app/blocks/{userId}`，沒有 kind。D2 的「client 送參照」要求每個入口各自改成送「貼文參照／評價參照／對話參照」，A6 之前聊天與評價入口還得維持舊形狀，所以 A7 的新 block RPC 需要同時接受「kind＋id」（給 legacy 入口）與「資源參照」（給貼文／留言入口）【推】。
2. **website 現在就在打 gateway 沒有的路由**【量】：`DELETE /app/posts/comments/{id}/like`（`W/lib/social/repo.ts:284-290`、`forum-repo.ts:215-219`；`routes_post.go:60` 只註冊 POST）。`throwOnHttpError:false` 讓 404 被吞掉，使用者取消讚可能沒有生效；buyer-app 有守門測試禁止這個路由，website 沒有。A7 的 `UnlikeComment`（明確兩個動作）會順便修掉；不要沿用 toggle。
3. **website 沒有封鎖名單頁，也沒有任何封鎖／檢舉入口**【量，W4】。C5 說 website 賣家中心留言選單「新增」封鎖入口，而 C6 的說明文字要求使用者「可以隨時在『封鎖名單』解除封鎖」，所以 website 要新增一個封鎖名單頁（新畫面）或改 C6 文案；brief 沒有把這頁列入畫面改動，需要 Will 決定。
4. **admin 貼文管理頁（`admin/src/app/admin/posts/page.tsx:100`，`PATCH /admin/posts/{id}` HIDE／RESTORE）與 D5′ 衝突**：D5′ 說店家貼文的平台下架變成措施「內容下架」，但 admin 頁面目前直接寫隱藏。brief 只提後台「檢舉佇列」提升到平台層，沒說這頁怎麼收；`admin/tests/codemeta-no-second-copy.test.ts:586-931` 也鎖著 `ReportStatus` 六值。
5. **seller-app 與 website 的賣家端「讀留言」現在走公開端點**（`post_repository.dart:342,420`；`W/lib/social/repo.ts:140` 由 `SellerPostDetailModal` 經 `PostComments` 使用）。公開端點在店家不公開、貼文未發布或被下架時讀不到，所以兩端都用 `postCommentsAvailability` 在 client 端提前擋掉並顯示說明（SA `post_comments_availability.dart`、W `SellerPostDetailModal.tsx:80`）。C8／C8′ 要「不公開時也讀得到」，等於店主讀留言要改走 `SocialAppService`（店主身分），這是 brief 的 D1 沒明講的 service 切分：同一個「列出留言」在 Public 與 App 各有一份。
6. 其他：(a) seller-app 沒有即時留言（無 SSE），C8 之外不必為它做 streaming client；(b) buyer-app `social_repository.dart` 有 6 個無呼叫者或重複的方法（`getComments`／`getCommentsRaw`、`addComment`／`addCommentRaw`、`getCommentReplies`、`replyToComment`、`deleteComment`、`commentLikeCountRaw`／`commentLikedByMeRaw`），搬遷時直接刪；(c) `GET /app/share_url`（seller-app `post_repository.dart:494`）不在 42 個 operation 內，但依附貼文分享，需要有人認領；(d) openapi 的 `Hashtag` tag 兩支（`/public/hashtags`、`/public/hashtags/popular`）在 client 端註解都寫成「商品 hashtag」，不是貼文標籤，可能不屬 social；`/public/tags/*`（`PostHashtag`）才是貼文標籤；(e) `W/lib/search/repo.ts:221-240` 的 `fetchTrendingHashtags` 沒有呼叫者；(f) 代行貼文路由（`routes_delegated.go:107-113`）在 website 有 4 個 BFF route＋1 個 repo，admin 前端沒有。

## 9. 還不確定的事

- `moderation` 與封鎖 RPC 的 service／方法名稱：brief 只定 `Social*Service` 三個，其餘名稱都是【推】。
- 代行貼文（`/admin/sellers/{id}/posts`）在 A7 之後由誰承接：brief 提到「代行稽核改發事件」，沒說代行的貼文 CRUD 要不要改成 Connect；website 已有共用的 `SellerPostRepo` 介面可做縫。
- `Hashtag` 兩支 operation 的歸屬（見 §8 6(d)）。
- `POST /app/posts/{id}/share` 與 `GET /app/share_url` 是否保留。
- `rg` 命中行數含註解與測試，不等於呼叫點數；呼叫點以各表列出的 file:line 為準。我沒有執行任何 App 或測試，所以「無呼叫者」都是靜態 `rg` 結果，動態分派（例如 Riverpod provider 以字串組路徑）沒有逐一排除。
