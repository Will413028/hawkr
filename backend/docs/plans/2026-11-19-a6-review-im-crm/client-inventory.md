# A6-0 四個 client 的 review／IM／CRM 呼叫點盤點

快照：dev `94796248`（A7 收尾後），只讀。標記：【量】＝跑過指令或打開檔案看過；【推】＝讀程式推論。
縮寫：W=`website`、A=`admin`、BA=`buyer-app/lib`、SA=`seller-app/lib`。路徑都相對 hawkr 根目錄。
格式沿用 `../2026-10-18-a7-social/client-inventory.md`。範圍是 gateway 上的三組舊 REST：IM（`/app/chat/*` 與 WebSocket）、review（評價）、CRM（`/app/sellers/me/customers*`、`/app/sellers/me/tags*`）。
去向的 service 名稱（`ImAppService`、`ReviewPublicService` 等）A6 還沒定案，下面不替它們命名，只寫「去向待 A6 proto 設計」；能從 gateway 路由表直接判斷的對應關係才寫。

gateway 上屬於這三組的路由，用來核對 client 有沒有打到不存在的路徑【量：`rg -n "ChatPath|Crm|Review" backend/gateway/internal/router/routes_chat.go routes_crm.go routes_review.go routes_seller.go`】：

| 組 | 路由 | 數 |
|---|---|---|
| IM | `POST /app/chat/ws-ticket`、`GET /app/chat/ws`（WebSocket）、`POST /app/chat/messages`、`GET /app/chat/history/{targetId}`、`GET /app/chat/conversations`、`GET /app/chat/unread`、`PATCH /app/chat/messages`、`PATCH /app/chat/conversations/{targetId}`、`PATCH /app/chat/conversations` | 9（`routes_chat.go:15-27`） |
| review | `GET /public/products/{id}/reviews`、`GET /public/products/{id}/rating`、`POST /app/reviews`、`GET /app/reviews/{id}`、`GET /app/me/reviews`、`GET /public/sellers/{id}/reviews`、`GET /public/sellers/{id}/reviews/stats`、`GET /app/sellers/me/reviews`、`GET /app/sellers/me/reviews/stats`、`POST /app/sellers/reviews/{id}/reply` | 10（`routes_review.go`、`routes_product.go:25`、`routes_seller.go:24-39`） |
| CRM | customers 列表／lookup／meta／search-by-tags／詳情 GET／詳情 PATCH／orders／coupons；tags 列表／建立／改／刪；客戶標籤 GET／POST／DELETE | 15（`routes_crm.go:19-43`） |

## 1. 指令與命中數（可重跑，cwd = hawkr 根目錄）

排除項（每條都帶）：`--glob '!node_modules' --glob '!.next' --glob '!**/*.md' --glob '!**/generated.ts' --glob '!**/*.g.dart' --glob '!**/*.freezed.dart' --glob '!**/code_meta.dart' --glob '!**/gen/**'`；測試另帶 `--glob '!**/*test*' --glob '!**/*spec*'`。下表指令省略這兩串，完整版在 `scratchpad/cnt.sh`（本次執行用，不入庫）。命中行數含註解，不等於呼叫點數；呼叫點以各節表列出的 file:line 為準。

| # | 指令（rg -n，省略排除項） | 命中行數 |
|---|---|---|
| W1 | `rg -n 'app/chat\|/chat/(conversations\|history\|messages\|unread\|online\|ws)\|ws-ticket' website/lib website/features website/components website/app` | 15 |
| W2 | `rg -n '(/reviews\|app/reviews\|me/reviews\|/rating\b)' website/lib/reviews website/lib/api website/features website/app/api` | 43（多為 SEO、canonical 的 `/reviews` 分頁路徑，非請求） |
| W3 | `rg -n 'me/customers\|sellers/me/tags' website/lib website/features` | 36 |
| W4 | `rg -n 'new WebSocket\(' website/lib website/features website/components`（另查 `EventSource`：只有 `lib/sse/client.ts` 的註解，通知 SSE 不在 A6 範圍） | 1 |
| A1 | `rg -n 'app/chat\|admin/(reviews\|chat\|customers)\|me/customers\|me/reviews\|/public/products/[^ ]*/reviews' admin/src` | 0 |
| B1 | `rg -n 'app/chat\|/chat/(conversations\|history\|messages\|unread\|ws)\|ws-ticket\|WebSocket\.connect' buyer-app/lib` | 25 |
| B2 | `rg -n '(/reviews\|app/reviews\|me/reviews\|/rating\b)' buyer-app/lib` | 14 |
| B3 | `rg -n '/app/blocks\|/app/reports' buyer-app/lib` | 8（含註解；請求只有 2 行，見 §6） |
| S1 | `rg -n 'app/chat/\|ws-ticket\|WebSocket\.connect' seller-app/lib` | 22 |
| S2 | `rg -n '(/reviews\|me/reviews\|/rating\b)' seller-app/lib` | 14 |
| S3 | `rg -n 'me/customers\|me/tags' seller-app/lib` | 27 |
| S4 | `rg -n '/app/blocks\|/app/reports' seller-app/lib` | 4（請求 2 行） |
| P1 | `rg -il 'app/chat\|ws-ticket\|/reviews\|me/customers\|app/blocks\|app/reports' packages` | 0 個檔案【量】；`packages/` 沒有任何 review／IM／CRM 呼叫 |
| G1 | 核對 gateway 沒有封鎖與檢舉路由：`rg -n 'blocks\|reports' backend/gateway/internal/router --glob '!*_test.go'` | 1（`seller_route_assert.go:47` 的英文註解 reports，無關）；`rg -n '/app/(blocks\|reports)' backend/api/openapi/openapi.yaml` 0 |
| T1 | 測試檔（見 §7 各表）：W 聊天 18、評價 8、CRM 4；BA 聊天 11、評價 12、舊封鎖／檢舉 5；SA 聊天 17、CRM 3、舊封鎖／檢舉 4；A 0 | 見 §7 |

### 各端摘要【量】

| 端 | 發請求的檔案 | 不重複的 REST（method＋path） | 備註 |
|---|---|---|---|
| website | IM：`lib/chat/repo.ts`、`lib/chat/ws.ts`；review：`lib/reviews/repo.ts`、`lib/api/index.ts`；CRM：`lib/seller/repo.ts` | IM 7＋WS 1；review 6（公開商品、公開店家兩支、我的、送出、店家回覆）；CRM 14 | 無封鎖／檢舉入口；`GET /app/chat/online` 的 client 函式存在但整個 website 沒有呼叫者，gateway 也沒有這條路由 |
| admin | 無 | 0 | 只透過商品列表 REST 與 `SellerPublicService.GetStore` 讀 `reviewCount`、`rating`（§3） |
| buyer-app | `chat/data/chat_repository.dart`、`review/data/review_repository.dart`、`product/data/product_repository.dart`、`moderation/data/block_repository.dart`、`moderation/data/legacy_report_repository.dart` | IM 8＋WS 1；review 4（公開評價、評分、我的、送出）；舊封鎖／檢舉 2 | 沒有 CRM；沒有店家回覆評價 |
| seller-app | `chat/data/chat_repository.dart`、`chat/data/chat_picker_remote_service.dart`、`product/data/product_repository.dart`、`order/data/order_repository.dart`、`customer/data/customer_repository.dart`、`coupon/data/coupon_repository.dart`、`report/data/{block,report}_repository.dart` | IM 8＋WS 1；review 3；CRM 15（含 coupon 的 lookup）；舊封鎖／檢舉 2 | 沒有店家回覆評價的 UI【量：`rg -n "reviews/.*reply\|replyReview" seller-app/lib` 0 筆】 |

## 2. website

### 2.1 呼叫點

| 檔案:行 | 請求 | 畫面／功能 | 去向 |
|---|---|---|---|
| `W/lib/chat/ws.ts:108` | `POST /app/chat/ws-ticket`（`apiClient.authed`） | 鑄單次 WS 票；`useChat.ts`、`useChatUnreadStream.ts`、`NotificationToastBridge.tsx:22` 經 `getChatSocket()` 觸發 | 待 A6 proto；WS 協定見 §2.4 |
| `W/lib/chat/ws.ts:110,168` | WebSocket `GET /api/v1/app/chat/ws?ticket=` | 即時訊息、已讀、打字、在線、未讀 | 同上 |
| `W/lib/chat/repo.ts:175` | `POST /app/chat/messages`（`sendMessage`） | 送訊息（`lib/hooks/useChat.ts:331,379`） | IM |
| `W/lib/chat/repo.ts:216` | `GET /app/chat/history/{targetId}?page&size`（`fetchChatHistory`） | 聊天室歷史（`ThreadClient.tsx`、`ConversationSidebar.tsx`） | IM |
| `W/lib/chat/repo.ts:247` | `GET /app/chat/conversations`（`fetchConversations`） | 對話列表（`useChat.ts:99`） | IM |
| `W/lib/chat/repo.ts:265` | `GET /app/chat/unread`（`fetchUnreadChatCount`） | 未讀徽章（`lib/chat/unread-context.tsx:5`、`ChatBubble.tsx`、`SellerCenterShell.tsx:16`） | IM |
| `W/lib/chat/repo.ts:277` | `PATCH /app/chat/messages`（`markMessagesRead`） | 逐則已讀 | IM |
| `W/lib/chat/repo.ts:287` | `PATCH /app/chat/conversations/{targetId}`（`markConversationRead`） | 進入對話自動已讀（背景寫入，`W/lib/api/client.ts:48-49` 在停權時靜默略過） | IM |
| `W/lib/chat/repo.ts:296` | `PATCH /app/chat/conversations`（`markAllChatRead`） | 全部已讀 | IM |
| `W/lib/chat/repo.ts:303` | `GET /app/chat/online`（`fetchOnlineUsers`） | **沒有呼叫者**【量：`rg -n fetchOnlineUsers website` 只有定義與 `docs/plans/2026-11-15-remove-storybook/knip-baseline.json:1270`】，gateway 與 openapi 都沒有這條 | 刪除 |
| `W/lib/reviews/repo.ts:206` | `GET /public/products/{id}/reviews`（`fetchProductReviews`，`page`、`size`、`rating`） | 商品頁評價（`lib/api/cached.ts:302`、`ProductReviewsPanel.tsx:201`、`StorefrontProductPreviewDrawer.tsx:133`） | review |
| `W/lib/reviews/repo.ts:291-296` | `GET /app/me/reviews`（`fetchMyReviews`） | 我的評價（`lib/hooks/useMyReviews.ts:72`、`features/account/screens/reviews/ReviewsClient.tsx`） | review |
| `W/lib/reviews/repo.ts:313` | `POST /app/reviews`（`submitOrderItemReview`） | 送出評價（`features/account/screens/orders/ReviewClient.tsx:156`；先 `uploadImageFile` 傳圖，`REVIEW_PHOTO_LIMIT=4`，`:92`） | review |
| `W/lib/reviews/repo.ts:336-337` | `POST /app/sellers/reviews/{id}/reply`（`replyReview`，body `{content}`） | 賣家回覆（`ProductReviewsPanel.tsx:61`） | review |
| `W/lib/api/index.ts:310` | `GET /public/sellers/{id}/reviews`（`fetchSellerReviews`） | 公開店面評價頁（`features/sellers/reviews/ReviewsClient.tsx:43`；`lib/api/cached.ts:605`） | review |
| `W/lib/api/index.ts:345` | `GET /public/sellers/{id}/reviews/stats`（`fetchSellerReviewStats`） | 店家評分（`lib/api/cached.ts:275`、`StorefrontProductPreviewDrawer.tsx:131`） | review |
| `W/lib/seller/repo.ts:261` | `GET /app/sellers/me/customers/lookup` | 優惠券買家選擇器、客戶查詢（`useSellerCustomerLookup.ts`） | CRM |
| `W/lib/seller/repo.ts:386` | `GET /app/sellers/me/customers`（`customer_type`、tag、keyword、sort） | 客戶列表（`CustomersClient.tsx`、`CouponBuyerPicker.tsx`；`lib/hooks/useSellerCustomers.ts`） | CRM |
| `W/lib/seller/repo.ts:432` | `GET /app/sellers/me/customers/{buyerId}` | 客戶詳情（`useCustomerDetail.ts`） | CRM |
| `W/lib/seller/repo.ts:551-552` | `PATCH /app/sellers/me/customers/{buyerId}`（body `{customerType}`） | 設一般／VIP／黑名單（`CustomerDetailPanel.tsx:29-31`） | CRM |
| `W/lib/seller/repo.ts:479,495,506` | `GET`／`POST`／`DELETE /app/sellers/me/customers/{buyerId}/tags[/{tagId}]` | 客戶標籤（`CustomerTagEditor.tsx`） | CRM |
| `W/lib/seller/repo.ts:514,522,535,542` | `GET`／`POST /app/sellers/me/tags`；`PATCH`／`DELETE /app/sellers/me/tags/{tagId}` | 標籤池（`lib/hooks/useSellerTags.ts`） | CRM |

沒有 website 呼叫者的 gateway 路由【量：`rg -n "customers/.*/(meta\|coupons\|orders)\|search-by-tags\|me/reviews/stats\|app/sellers/me/reviews\|app/reviews/\\$" website/lib website/features` 0 筆；`W/lib/seller/repo.ts:558` 註解說客戶訂單走 `lib/orders/repo.ts`，不是 CRM 的 `/orders`】：CRM `meta`、`search-by-tags`、`orders`、`coupons`；review `GET /app/reviews/{id}`、`GET /app/sellers/me/reviews`、`…/stats`。

### 2.2 入口（畫面與 BFF）

| 項目 | file:line | 現況【量】 |
|---|---|---|
| 聊天頁 | `W/app/[locale]/(account)/account/messages/page.tsx`、`…/messages/[targetId]/page.tsx`；賣家 `W/app/[locale]/(seller)/seller-center/messages/page.tsx`、`…/[targetId]/page.tsx`；元件 `features/account/screens/messages/{MessagesClient,ThreadClient}.tsx`、`features/messages/components/ConversationSidebar.tsx` | 聊天室沒有檢舉、封鎖入口（見 §6） |
| 聊天浮動入口、未讀 | `W/components/shared/ChatBubble.tsx`、`RootDocument.tsx:28`（`ChatUnreadProvider`）、`SellerCenterShell.tsx:16`、`features/seller/components/SellerChatButton`（有 `SellerChatButton.unit.test.ts`） | 全站掛載，WS 在登入後常駐 |
| 評價頁 | `W/app/[locale]/(account)/account/reviews/page.tsx`、`(public)/sellers/[id]/(storefront)/reviews/page.tsx`、`(seller)/seller-center/preview/[id]/reviews/page.tsx` | 評價列表沒有檢舉、封鎖入口 |
| 賣家中心評價 | `W/features/seller-center/products/ProductReviewsPanel.tsx`（讀的是公開商品評價列表，不是 `/app/sellers/me/reviews`） | 回覆走 `replyReview` |
| CRM 頁 | `W/features/seller-center/customers/{CustomersClient,CustomerDetailPanel,CustomerTagEditor}.tsx`、`features/seller-center/coupons/CouponBuyerPicker.tsx` | 見 §2.3、§6 |
| BFF | 沒有專屬的 chat／review／CRM BFF route；瀏覽器經 `W/app/api/v1/[...path]/route.ts` 同源反代到 gateway；該檔 `:80` 把含 `/reviews` 的路徑排除在 E2E proxy 快取外 | 改成 Connect 後要確認反代規則與 E2E proxy 的路徑清單 |
| CSP | `W/next.config.ts:47`（`withWebSocketSiblings`，`W/lib/security/csp.ts:54`）把 `https` 的 connect 來源補成 `wss` | WS 搬走或改協定時一起改 |

### 2.3 手寫的 enum／標籤表

| 位置 | 內容 | 去向 |
|---|---|---|
| `W/lib/seller/repo.ts:276,330-332`（`SellerCustomerType`、`normalizeCustomerType`） | `normal \| vip \| blocklist`；未知值一律當 normal | CRM 新契約的 enum 進共用文案目錄與 `@hawkr/api`；刪手寫型別 |
| `W/features/seller-center/customers/CustomersClient.tsx:37-40,52-54,61-62`（`TYPE_TABS`、`TYPE_BADGE`、`customerTypeLabel`） | 全部／一般／VIP／黑名單 | 同上 |
| `W/features/seller-center/customers/CustomerDetailPanel.tsx:29-31` | 一般／VIP／黑名單加 `loadingLabel`（「加入黑名單中…」） | 同上 |
| `W/features/seller-center/coupons/CouponBuyerPicker.tsx:39,41-44` | 同一份標籤第三次出現（`CUSTOMER_TYPE_LABEL`） | 同上；三處合併 |
| `W/lib/chat/types.ts:6-20`（`ChatMessageType`，尾巴有 `\| string`） | `text \| image \| chat \| notify \| system \| chat_ack \| read_receipt \| typing …`：把 WS 事件種類與訊息內容種類放在同一個 union | IM proto 把「訊息種類」與「即時事件」分成兩個 enum；`\| string` 的後備要拿掉 |
| `W/lib/chat/preview.ts:15-83`（`messagePreview`） | 以內容字串偵測圖片與卡片，回「傳送了一張圖片」「傳送了一張卡片」；三端各一份（BA `chat/presentation/chat_preview.dart`、SA 同名檔） | 伺服器給 `last_message_kind`，文案進共用目錄；偵測邏輯刪除 |
| `W/lib/reviews/repo.ts:139-160` | 讀 `imageKeys` 與衍生欄位，id 同時讀 `id`／`reviewId`（註解說各端點欄名不同） | 新契約單一欄名；刪相容讀取 |

### 2.4 即時連線

- `W/lib/chat/ws.ts`（440 行）：單例 `getChatSocket()`；連線前先 `POST /app/chat/ws-ticket`（`:108`），WS URL 只帶 `?ticket=`（`:110`，`buildWsUrl` 由 `tests/chat-ws.test.ts:60-80` 鎖）。
- 心跳：每 15 秒送 `{type:'ping'}`，30 秒沒收到任何 frame 判定半開並重連（`ws.ts:15-20,66-72`）。
- 重連：full-jitter backoff（上限 1、2、4、8、16 秒，最多 5 次），之後等 focus／online／visibility 事件才再試（`ws.ts:21-24,80,194-`）；`disconnect()` 抑制自動重連（`:136`）。票是單次，所以每次重連都重鑄。
- 收的事件（`ws.ts:388-419` 的 `switch`）：`pong`／`heartbeat`、`typing`／`start_typing`／`stop_typing`、`read_receipt`／`chat_ack`、`presence`、`unread_changed`，其餘當聊天訊息。送的事件：`ping`、`presence_sub`／`presence_unsub`（`:240,244`）、打字。
- 送訊息一律走 HTTP `POST /app/chat/messages`，WS 只收（`useChat.ts:331,379`）【推：三端相同，見 BA `chat_repository.dart:624` 註解】。
- 【推】Connect 沒有雙向串流在瀏覽器的原生支援，WS 是否保留、還是改 server streaming 加 unary 送訊息，是 A6 要決定的契約問題。

## 3. admin

沒有任何 review、IM、CRM 的 REST 或 WebSocket 呼叫【量：A1 0 筆；`admin/tests` 0 檔】。與 A6 有關的只有兩處讀取：

| 檔案:行 | 內容 | 去向 |
|---|---|---|
| `A/src/types/product.ts:66,90`、`A/src/app/admin/merchants/[id]/page.tsx:125-126,1209`、`A/src/app/admin/products/[id]/page.tsx:107` | 商品的 `rating`、`reviewCount` 字串，來自 `GET /api/v1/admin/products*`（`A/src/services/products.ts:13`、`merchants/[id]/page.tsx:280`），顯示用 | 商品列表搬進單體時決定由誰提供評分彙總【推】；不是 review 的 RPC 呼叫 |
| `A/src/app/admin/merchants/[id]/use-store-detail.ts:94-99` | `useStoreRating`：呼叫 `SellerPublicService.GetStore`，讀 `stats.rating_average` | 店家評分的來源是 seller 模組的 `StoreStats`；A6 review 搬進單體後要確認這個欄位由 review 的事件或 `pub_*` 更新【推，brief 未讀】 |

另外，admin 的檢舉佇列已經是 Connect（A7）：`A/src/app/admin/moderation/moderation-text.ts:32-37` 的 `TARGET_KIND_OPTIONS` 只有 `POST`、`COMMENT`、`FORUM_ENTRY`、`BUYER` 四種，`api/hawkr/moderation/v1alpha1/moderation.proto:29-36` 的 `ReportTargetKind` 同樣沒有 review 或聊天訊息。也就是說**後台目前沒有任何地方能處理評價或聊天訊息的檢舉**（見 §8 發現 2）。

## 4. buyer-app

### 4.1 呼叫點

| 檔案:行 | 請求 | 畫面／功能 | 去向 |
|---|---|---|---|
| `BA/features/chat/data/chat_repository.dart:606` | `POST /app/chat/ws-ticket` | 鑄票（`_mintWsTicket`） | IM |
| `…chat_repository.dart:275` | WebSocket `{host}/api/v1/app/chat/ws?ticket=`（`WebSocket.connect`，10 秒逾時） | 即時；`main.dart:250` 監聽新訊息，`chat_list_screen.dart:68` 駐留時 refCount＋1 | IM |
| `…:692` | `POST /app/chat/messages` | 送訊息（`chat_thread_notifier.dart`） | IM |
| `…:754`、`:817` | `GET /app/chat/history/{targetId}`（兩份：一般與較舊／分頁版） | 聊天室歷史 | IM；兩份合一 |
| `…:884` | `GET /app/chat/conversations` | 對話列表（`chat_list_screen.dart:53`；欄位解析 `chat_conversation_fields.dart`） | IM |
| `…:930`、`:945` | `GET /app/chat/unread`（兩處） | 全站未讀徽章（`shared/widgets/chat_icon_with_badge.dart`） | IM；兩處合一 |
| `…:1003` | `PATCH /app/chat/conversations/{targetId}` | 已讀（背景寫入，`core/networking/suspension_signal.dart:37,39` 停權時略過） | IM |
| `…:1021` | `PATCH /app/chat/conversations`（全部已讀） | 對話列表 | IM |
| `…:627,645` | 已讀回執、打字指示（經 WS 送出） | `chat_thread_screen.dart:683` | IM |
| `BA/features/review/data/review_repository.dart:68` | `GET /public/products/{skuId}/reviews`（`getProductReviews`） | `review_controller.dart:12`、`product_reviews_screen.dart:83` | review |
| `…review_repository.dart:90`、`:103` | 同一路徑（`getMerchantProductReviews`、`getMerchantProductReviewSummary`，沒帶分頁） | `merchant_item_detail_screen.dart:2183`、`settings/presentation/my_subscription_screen.dart:443` | review；三份合一 |
| `…review_repository.dart:173` | `GET /app/me/reviews`（`getMyReviews`；`findReviewByOrderItemIds` `:206` 以它反查） | `order_detail_screen.dart:2352,2390` | review |
| `…review_repository.dart:158`、`:247` | `POST /app/reviews`（`postMerchantProductReview`、`addReview`） | `order_review_screen.dart:197`、`review_controller.dart:37`、`rate_screen.dart` | review；兩份合一 |
| `BA/features/product/data/product_repository.dart:193` | `GET /public/products/{skuId}/rating`（`getMerchantProductGrades`） | `my_subscription_screen.dart:778` | review |

沒有任何 BA 呼叫者的 gateway 路由【量】：review 的店家端、`GET /app/reviews/{id}`、店家評價列表；CRM 全部。

### 4.2 入口

| 項目 | file:line | 現況 |
|---|---|---|
| 聊天路由 | `BA/core/routing/app_router.dart:274`（`/chat`）、`:276`（`/chat/room`，`extra` 帶 `merchantId`、`merchantName`、`conversationId`）、`:287`（`/chat/ai-assistant`，不屬 IM）；`route_access_policy.dart:19` 要求登入 | 入口：`seller/presentation/merchant_screen.dart:377`、`order/presentation/order_detail_screen.dart:729`、`notification/presentation/notifications_screen.dart:286`、推播 `notification/data/push/router.dart:96`、`main.dart:337` |
| 評價入口 | `BA/features/review/presentation/{order_review_screen,product_reviews_screen,rate_screen}.dart`；商品頁評價區 `product/presentation/merchant_item_detail_screen.dart:2380-2496` | 檢舉與封鎖按鈕見 §6 |

### 4.3 手寫的 enum／標籤表

| 位置 | 內容 | 去向 |
|---|---|---|
| `BA/features/review/presentation/rate_screen.dart:233-246`（`_ratingLabel`） | 1～5 星對應「非常不滿意」…「非常滿意」，預設「點擊評分」 | 共用文案目錄（W 沒有同款，所以是只有 BA 有的文案；SA 也沒有） |
| `BA/features/moderation/data/report_target_type.dart:14-`（`reportReasonsFor`）、`BA/core/codemeta/code_meta.dart` 的 `ReportTargetType`／`ReportReasonCode` | 聊天與評價檢舉的原因清單與標籤（舊 CodeMeta 值域，A7 已把社群檢舉換成 proto） | 隨 A6 的檢舉對象擴充一起換 |
| `BA/features/chat/presentation/chat_preview.dart:25-77`（`derivePreview`、`cardLabelForType`）、`chat/data/chat_model.dart:28-69`（`msgType==2` 當圖片、`type` 預設 `'chat'`） | 同 §2.3 | 同 §2.3 |
| `BA/features/moderation/data/report_block_codes.dart:17`（`reportBlockErrorMessage`） | 檢舉／封鎖的舊錯誤碼轉文字 | 新契約改用 `ErrorReason`，刪 |

### 4.4 即時連線

`chat_repository.dart` 與 website 同協定、同參數：心跳 15 秒（`:120`）、存活逾時 30 秒（`:121`，`_livenessTimer` 在 `:19-20` 一帶）、重連最多 5 次（`:117`）、full-jitter（`_scheduleReconnect`，`:39-48,527`）、App 回前景重連（`:571`）；`_epoch` 防舊連線事件污染。WS 只收；送訊息、已讀走 HTTP（`:624` 註解），已讀回執與打字走 WS（`:627,645`）。
BA 另有 `startPolling`（`:552`）是空實作【量】，可刪。

## 5. seller-app

### 5.1 呼叫點

| 檔案:行 | 請求 | 畫面／功能 | 去向 |
|---|---|---|---|
| `SA/features/chat/data/chat_repository.dart:700` | `POST /api/v1/app/chat/ws-ticket` | 鑄票 | IM |
| `…:297`（URL 組在 `:127,129`） | WebSocket `/api/v1/app/chat/ws?ticket=` | 即時；`app.dart:20-21`、`core/session/post_login_side_effects.dart:3` 登入後建立 | IM |
| `…:746` | `POST /api/v1/app/chat/messages` | 送訊息 | IM |
| `…:860` | `GET /api/v1/app/chat/history/{targetId}` | 歷史 | IM |
| `…:950` | `GET /api/v1/app/chat/conversations` | 對話列表 | IM |
| `…:994` | `PATCH /api/v1/app/chat/messages`（`messageIds`） | 逐則已讀（BA 沒有這條，BA 走 WS 回執） | IM |
| `…:1012` | `GET /api/v1/app/chat/unread` | `features/dashboard/data/dashboard_chat_unread_provider.dart:3` | IM |
| `…:1076` | `PATCH /api/v1/app/chat/conversations/{id}` | 已讀 | IM |
| `…:1094` | `PATCH /api/v1/app/chat/conversations` | 全部已讀 | IM |
| `SA/features/chat/data/chat_picker_remote_service.dart:17` | `GET /api/v1/app/orders`（聊天附件挑訂單） | 不是 IM 路由，屬 order | 隨 order 契約 |
| `SA/features/product/data/product_repository.dart:144` | `GET /api/v1/public/products/{skuId}/reviews`（`getSellerProductReviewsPublic`） | 商品評價（`product/presentation/product_reviews_screen.dart:446`） | review |
| `…product_repository.dart:263` | `GET /api/v1/public/products/{id}/rating`（`getPublicProductRatingSummary`） | 商品預覽 | review |
| `SA/features/order/data/order_repository.dart:240` | `GET /api/v1/app/sellers/me/reviews`（`page`、`size`，翻頁找某訂單品項的評價） | 訂單詳情 | review（website 沒有這條呼叫，SA 是唯一消費者） |
| `SA/features/customer/data/customer_repository.dart:45` | `GET /app/sellers/me/customers`（`customer_type`、tag 篩選，`:23`） | `customer_management_screen.dart` | CRM |
| `…customer_repository.dart:70`、`:114` | `GET`／`PATCH /app/sellers/me/customers/{id}`（`:111-115` body `customerType`） | `customer_detail_screen.dart` | CRM |
| `…:91` | `GET …/customers/{id}/meta` | 詳情 | CRM（website 沒用） |
| `…:132`、`:152`、`:168` | `GET`／`POST`／`DELETE …/customers/{id}/tags[/{tagId}]` | 客戶標籤 | CRM |
| `…:184`、`:203`、`:220`、`:235` | `GET`／`POST /app/sellers/me/tags`；`PATCH`／`DELETE …/tags/{tagId}` | 標籤池 | CRM |
| `…:252`、`:279` | `GET …/customers/{id}/orders`、`…/coupons` | 詳情分頁（website 沒用） | CRM |
| `SA/features/coupon/data/coupon_repository.dart:175` | `GET /api/v1/app/sellers/me/customers/lookup` | 優惠券選買家 | CRM |

### 5.2 入口

| 項目 | file:line | 現況 |
|---|---|---|
| 聊天路由 | `SA/core/router/app_router.dart:209`（`/chat/thread`）、`:208`（AI 助手，不屬 IM） | 入口：`chat/presentation/chat_list_screen.dart:284`、`customer/presentation/customer_detail_screen.dart:273`、`notification/presentation/notifications_screen.dart:248`、推播 `notification/data/push/router.dart:118`、`order/presentation/shared_order_detail_screen.dart:460` |
| CRM 路由 | `SA/core/router/app_router.dart:106`（`/customers`）；入口 `dashboard/presentation/dashboard_screen.dart:503,581`、推播 `push/router.dart:135` | |
| 評價 | `SA/core/router/app_router.dart:134`（`/product/reviews`）；入口 `product/presentation/product_preview_screen.dart:990` | 沒有回覆評價的畫面 |

### 5.3 手寫的 enum／標籤表

| 位置 | 內容 | 去向 |
|---|---|---|
| `SA/features/customer/presentation/customer_management_screen.dart:313`（分頁 `(label: '黑名單', status: 'blocklist')`）、`:439` | CRM 篩選分頁 | CRM enum 進共用目錄 |
| `SA/features/customer/presentation/customer_detail_screen.dart:78,205,222-225,393`；`customer_model.dart:31`；`customer_repository.dart:23,110,314,329`；`customer_controller.dart:14` | `'normal'／'vip'／'blocklist'` 字串散在 6 處，`isBlacklisted` 由 `ct == 'blocklist'` 推；文案「設黑名單」「解除黑名單」「已加入黑名單」 | 同上；改用生成的 enum |
| `SA/features/chat/presentation/chat_preview.dart:26-78`（`derivePreview`、`cardLabelForType`）、`chat/data/chat_model.dart:21,56` | 同 §2.3 | 同 §2.3 |
| `SA/features/report/data/report_model.dart:13-`（`reportReasonsFor`）、`SA/core/codemeta/code_meta.dart` 的 `ReportTargetType`／`ReportReasonCode` | 聊天訊息檢舉的原因 | 同 BA |

### 5.4 即時連線

與 BA 同形：心跳 15 秒（`:149`）、存活 30 秒（`:150`）、重連最多 5 次（`:146`）、回前景重連（`:1116`）。三端（W、BA、SA）各自實作一份同樣的狀態機，常數靠註解對齊（`W/lib/chat/ws.ts:66` 註解「matched across web + buyer + seller」）。

## 6. 自 A7-4e 刪掉 `/app/blocks*`、`/app/reports*` 後壞掉的入口

前提【量】：gateway 路由表與 openapi 都沒有 `/app/blocks`、`/app/reports`（G1，0 筆；`backend/gateway/internal/router/routes_chat.go` 的聊天路由與 `routes_review.go` 沒有檢舉或封鎖）。`backend/docs/plans/2026-10-18-a7-social.md:14` 記載這是 Q1（Will 2026-10-18 接受：A6 完成 client 步驟前，聊天與評價的封鎖與檢舉在本機會失敗）。所以下列按鈕**今天按了一定回 404**【推：沒有起服務實測；依據是路由表沒有該路徑】。

### 6.1 壞掉的入口（逐一定位）

| 端 | 入口（畫面→按鈕） | 呼叫鏈 | 最終請求 |
|---|---|---|---|
| BA | 聊天室長按訊息→「檢舉此訊息」：`BA/features/chat/presentation/chat_thread_screen.dart:937-944`（`LegacyReportTarget(ReportTargetType.chatMessage, m.id)`） | `showReportSheet`（`moderation/presentation/report_sheet.dart:144-146`）→ `legacyReportRepositoryProvider` | `POST /app/reports`，`BA/features/moderation/data/legacy_report_repository.dart:28`（`targetType=CHAT_MESSAGE`） |
| BA | 聊天室長按訊息→「封鎖此使用者」：`chat_thread_screen.dart:955-959`（`LegacyBlockTarget(merchantId)`，對象是聊天對方店家） | `showBlockConfirmDialog`（`moderation/presentation/block_confirm_dialog.dart:42-43`）→ `blockRepositoryProvider.blockUser` | `POST /app/blocks/{userId}`，`BA/features/moderation/data/block_repository.dart:20` |
| BA | 商品頁評價卡右上「更多評價操作」→ 檢舉／封鎖：`BA/features/product/presentation/merchant_item_detail_screen.dart:2477-2496`（按鈕 label `更多評價操作` `:2481`；`reviewId` 取 `review['reviewId']` `:2380`，`reviewUserId` 取 `review['buyerId']` `:2381`） | `showReportBlockActionSheet`（`moderation/presentation/report_block_action_sheet.dart:23`）→ 同上兩條 | 檢舉 `POST /app/reports`（`targetType=REVIEW`，`:2486-2489`）、封鎖 `POST /app/blocks/{buyerId}`（`:2491`） |
| SA | 聊天室訊息選單→「檢舉訊息」：`SA/features/chat/presentation/chat_thread_screen.dart:1850-1861`（`ReportTargetType.chatMessage`） | `showReportSheet`（`SA/features/report/presentation/widgets/report_sheet.dart:20,74-75`）→ `reportRepositoryProvider.submitReport` | `POST /api/v1/app/reports`，`SA/features/report/data/report_repository.dart:54`（`_base` 在 `:24`） |
| SA | 聊天室訊息選單→「封鎖此人」：`chat_thread_screen.dart:1865-1884`（`userId: msg.senderId`） | `blockUserWithConfirm`（`SA/features/report/presentation/widgets/block_actions.dart:81-95`）→ `blockRepositoryProvider.blockUser` | `POST /api/v1/app/blocks/{userId}`，`SA/features/report/data/block_repository.dart:29`（`_base` 在 `:23`） |
| W | 無（website 的聊天、評價都沒有檢舉或封鎖入口）【量：`rg -n "封鎖\|檢舉\|舉報\|blockUser\|/blocks\|/reports" website/features website/lib website/components website/app`，排除 `lib/content`、`lib/codemeta`、法務頁後，只剩社群封鎖（A7 已改 Connect）與 `csp-report`】 | — | — |
| A | 無 | — | — |

三個壞掉的 Dart 檔都已被標成「A6 允許殘留」：BA `block_repository.dart:12`、`legacy_report_repository.dart:13`、`block_target.dart:94`，SA `block_repository.dart:27`、`block_actions.dart:80`。BA 另有守門測試把這兩個檔列成唯一允許的 `/app/blocks`、`/app/reports` 出現處（`BA test/architecture/dead_gateway_endpoint_guardrail_test.dart:79-90`，`max: 1`）；SA 沒有同款守門測試【量：`rg -n "app/blocks\|app/reports" seller-app/test` 只有 `core/networking/page_envelope_contract_test.dart:69-70` 鎖 `POST /app/reports` 回應形狀】。

### 6.2 CRM「黑名單」（沒有壞）

CRM 的 `blocklist` 只是 `PATCH /app/sellers/me/customers/{id}` 的 `customerType` 標籤，gateway 路由還在（`routes_crm.go:28`），所以**功能沒壞**，只是它與封鎖是兩個東西（A7 決定簡報 C10：A6 併入店家封鎖）。位置：

| 端 | file:line |
|---|---|
| W | `features/seller-center/customers/CustomersClient.tsx:37-40,52-54`、`CustomerDetailPanel.tsx:29-31`、`features/seller-center/coupons/CouponBuyerPicker.tsx:39-44`、`lib/seller/repo.ts:276,330-332,343,374,546-552` |
| SA | `features/customer/presentation/customer_management_screen.dart:313,439`、`customer_detail_screen.dart:78,205,222-225,393`、`customer/data/customer_repository.dart:23,110-115,314-329`、`customer_model.dart:31`、`customer_controller.dart:14` |
| BA | 沒有 |
| 說明文字（website 公開文件） | `W/lib/content/data.ts:427-432`、`W/lib/content/seller-guide-data.ts:482,494`（「設為一般、VIP或黑名單」） |

與 A7 已完成的店家封鎖並存的現況【量】：website 賣家中心已有封鎖名單頁（`W/features/seller-center/blocked-buyers/BlockedBuyersPage.tsx:40`、`sellerNav.tsx:134`、`lib/seller/block-buyer.ts`）與 seller-app 的 `BlockListScreen`（`SA/features/report/presentation/block_list_screen.dart`，用 `store_block_repository.dart`），所以賣家現在同時有「封鎖名單」（Connect）和「客戶黑名單」（CRM 標籤）兩個互不相干的概念。

## 7. 測試與 mock（編碼了這些 REST 形狀）

| 端 | 檔案 | 編碼的內容 |
|---|---|---|
| W 聊天 | `tests/chat-ws.test.ts:60-80`（`buildWsUrl`）、`tests/chat-seqid-tsid.test.ts`、`tests/conversation-sidebar-range.test.ts`、`tests/chat-navigation-optimization.test.ts`、`tests/auth-state-source-guard.test.ts`、`tests/canonical-merchant-logo-normalizers.test.ts`、`tests/seller-link-canonical.test.ts`、`tests/media-objectkey-contract.test.ts:316`（`GET /app/chat/conversations` 的 `conversations[0]` 逐字）；`lib/chat/__tests__/{chatIdentity,list-range,timeline,ws-dispatch,ws-onstate,ws-unread}.unit.test.ts`；`components/shared/__tests__/{chat-bubble-prefetch,NotificationToastBridge.highwater,NotificationToastBridge.priming}…test.ts`；`features/seller/components/SellerChatButton.unit.test.ts` | wire 形狀與 WS 事件分派契約（共 18 檔） |
| W 聊天 e2e（Playwright，只能由 CI 跑） | `tests/chat-drawer-loading.spec.ts:37-72`（攔 `me/customers`、`me/tags`、`chat/conversations`、`chat/history/*`、`chat/conversations/*`、`chat/unread`、`chat/ws-ticket`）；`tests/e2e/protected-dynamic-routes.spec.ts:324-355,459-492,832-836`（`/app/chat/unread`、`POST ws-ticket`、`conversations`、`history/{id}`、`conversations/{id}`）；`tests/e2e/conversation-sidebar-range.spec.ts`；`tests/e2e/procurement-composer-pending.spec.ts:57-71,174-176`、`procurement-assistant-clarify.spec.ts`（9 處）、`procurement-publication-restore.spec.ts`（3）、`procurement-crop-disambiguation-interrupt.spec.ts`（3）：為了讓頁面載入而 mock 聊天的三條背景路由 | 路徑與回應形狀；採購助理的 e2e 也綁著 `/app/chat/*`（雖然被測的是 AI 助理），聊天契約改了這 4 檔也要改【量】 |
| W 評價 | `tests/my-reviews-normalize.test.ts:5,17,150`（`GET /app/me/reviews` 的 `OrderReviewRecord`）、`tests/review-id.test.ts`、`tests/page-envelope-shape.test.ts:19,56`（鎖 `lib/reviews/repo.ts` 讀 `data.records` 而非 `nested.reviews`）、`tests/buyer-avatar-contract.test.ts:34-71`（`GET /public/products/{id}/reviews` 的 `records[0]` 逐字）、`tests/seller-reviews-media-contract.test.ts:7`（`PublicSellerReviewItem`）、`tests/seller-preview-ssr-production.test.ts:85-97,541-542`（`reviews`、`reviews/stats` mock）、`tests/e2e-fake-session-upstream.test.ts:106`、`tests/media-objectkey-contract.test.ts:491` | wire 形狀契約（8 檔，不含 SEO 與路徑類測試） |
| W CRM | `tests/seller-coupon-repo.test.ts:163,201`（`/app/sellers/me/customers`、`…/lookup`）；`tests/e2e/protected-dynamic-routes.spec.ts:689-724`（`customers/{id}`、`…/tags`）；`tests/e2e/seller-summary-mobile-layout.spec.ts:112-135`；`tests/seller-record-mobile-disclosure-contract.test.ts` | 路徑與回應形狀 |
| W mock helper | `tests/e2e/helpers/mock-gateway.ts`、`proxy-public-gateway.ts` 不含 chat／reviews／customers【量：`rg -n "chat\|reviews\|customers" website/tests/e2e/helpers/mock-gateway.ts website/tests/e2e/helpers/proxy-public-gateway.ts` 0 筆】；`helpers/dynamic-route-catalog.ts:187,261,301` 只列頁面路由；`tests/e2e/site-stability.spec.ts`、`site-links.spec.ts` 是頁面路由 | 路徑散在各 spec，不在共用 helper，改契約時要逐檔 rg |
| A | 無 | — |
| BA 聊天 | `test/features/chat/{chat_conversation_fields,chat_message_avatar_key_contract,chat_preview,chat_repository_auth,chat_thread_notifier}_test.dart`、`chat_thread_notifier_test.mocks.dart`；`test/core/networking/suspension_test.dart`、`test/config/media_image_test.dart` | Dio adapter 假回應編碼 REST（11 檔）；沒有 WS 重連或心跳測試【量：`rg -n "reconnect\|livenessTimer\|ping" buyer-app/test/features/chat` 0 筆】 |
| BA 評價 | `test/features/review/{product_reviews_screen_layout,review_avatar_key_contract,review_submit_contract}_test.dart`、`test/features/settings/my_subscription_social_screen_test.dart`、`test/features/home/home_latest_posts_*`（命中 `/reviews` 字樣）、`test/shared/utils/avatar_wire_test.dart`、`test/features/seller/merchant_icon_policy_test.dart` | 12 檔，其中有實質契約的是前三個與 avatar_wire |
| BA 舊封鎖／檢舉 | `test/features/moderation/block_repository_test.dart:43-51`（`POST /app/blocks/42`）、`block_flows_test.dart:193-201`、`report_flows_test.dart:346-353`、`frosted_more_entry_points_test.dart:185-210`（鎖 `LegacyReportTarget`、`LegacyBlockTarget(userId: 'buyer-9')`）、`test/architecture/dead_gateway_endpoint_guardrail_test.dart:79-90,168-175` | 5 檔；A6 搬完要把這個守門改成「禁止任何 `/app/blocks`、`/app/reports`」 |
| SA 聊天 | `test/features/chat/{chat_list_redesign,chat_picker_architecture,chat_send_error_dispatch,chat_thread_notifier}_test.dart`、`test/core/networking/{dio_auth_interceptor_skip,dio_forbidden_write}_test.dart`、`test/core/session/session_provider_test.dart`、`test/features/media/{buyer_avatar_key_contract,protected_media_architecture}_test.dart`、`test/features/report/report_target_type_contract_test.dart`、`test/integration/hermetic_app_router_smoke_test.dart`、`test/shared/widgets/chat_icon_policy_test.dart` | 17 檔（含 AI 助理與商品 listing chat 的同名誤中，約 4 檔） |
| SA CRM | `test/features/customer/{customer_detail_more_menu,customer_repository_camel_parse,subscriber_architecture}_test.dart` | 3 檔；沒有鎖路徑的測試，`blocklist` 字串靠 `customer_repository_camel_parse_test` 間接鎖 |
| SA 評價／舊封鎖 | `test/core/networking/page_envelope_contract_test.dart:31`（`/public/products/{id}/reviews`）、`:69-70`（`POST /app/reports`）、`test/features/media/buyer_avatar_key_contract_test.dart:292`（`GET /public/products/{id}/reviews`） | SA 沒有評價與舊封鎖的專屬測試 |

## 8. 最重要的發現

1. **聊天與評價的封鎖、檢舉全部 404，且 BA 與 SA 各有兩個壞入口加一個評價入口**【量：路由表、openapi 皆無】（§6.1）：BA 3 個按鈕（聊天檢舉、聊天封鎖、評價檢舉＋封鎖共用選單）、SA 2 個按鈕（聊天檢舉、聊天封鎖）；website 與 admin 沒有。BA 評價入口送的是 `review['buyerId']`（`merchant_item_detail_screen.dart:2381`），不是店家代碼；BA 聊天入口送路由參數 `merchantId`（`chat_thread_screen.dart:957`）；SA 聊天送 `msg.senderId`（`chat_thread_screen.dart:1879`）；兩者各是帳號 ID 或業務鍵，沒有追到後端確認【推】。A7 已把封鎖解析成「內容參照→被封鎖的人」（`backend/internal/httpapi/blocks/blocks.go:1-5` 寫明 A6 的聊天、評價出現第二個封鎖入口時升成編排模組），所以 A6 的 proto 要決定聊天與評價封鎖的「參照」是什麼（對話 id、評價 id），而不是再送使用者 id【推】。
2. **後台沒有任何地方能處理評價或聊天訊息的檢舉**：`ReportTargetKind` 只有 POST／COMMENT／FORUM_ENTRY／BUYER（`moderation.proto:29-36`），admin 的 `TARGET_KIND_OPTIONS` 同（`moderation-text.ts:32-37`）。client 要補檢舉入口（目前壞掉的就是這兩種對象），proto 就要新增 `REVIEW`、`CHAT_MESSAGE`（或「對話」），admin 的措施表 `actionsFor`（`moderation-text.ts:90-`）也要加對應動作（評價下架、聊天封鎖……）。這是 A6 對 moderation 的跨模組變更，brief 要涵蓋【推】。
3. **三端各有一份 WS 狀態機，常數靠註解對齊**：W `ws.ts`（440 行）、BA `chat_repository.dart`（1,119 行）、SA `chat_repository.dart`（1,157 行），心跳 15／30 秒、重連 5 次的 full-jitter 完全一致；測試只有 W 有分派與狀態測試（`lib/chat/__tests__/ws-*.unit.test.ts`），BA、SA 沒有重連測試【量】。若 A6 保留 WS 協定，應只定一份行為契約（事件種類的 proto enum 與重連參數放文件）；若改 Connect streaming，三端的狀態機都要重寫，是最大的 client 工作量。
4. **三端手寫了同一個聊天預覽偵測**（`messagePreview`／`derivePreview`，從內容字串猜是不是圖片或卡片）：W `lib/chat/preview.ts`、BA `chat_preview.dart:25`、SA `chat_preview.dart:26`；`ChatMessageType` 把 WS 事件和內容種類混在一起，還帶 `| string` 後備（`W/lib/chat/types.ts:6-20`）。伺服器應直接給 `last_message_kind`，這與 A7 的「client 不產生只由契約值決定的文字」規則直接衝突【推】。
5. **死碼與重複**【量】：W `fetchOnlineUsers`（打不存在的 `GET /app/chat/online`，整個 website 沒有呼叫者）；BA `chat_repository.dart` 的 `history`、`unread` 各兩份實作、`startPolling` 空實作；BA `review_repository.dart` 三個方法打同一條 GET、兩個方法打同一條 POST；SA 沒有 `GET /app/reviews/{id}`、`…/stats`、CRM `search-by-tags` 的呼叫者，W 沒有 CRM `meta`／`orders`／`coupons`。搬遷時這些路由可以不進新契約（待 A6 對照 gateway 清單確認，勿直接刪 gateway）。
6. **CRM 黑名單的標籤字串散在 W 3 處、SA 6 處**，且與已上線的店家封鎖名單（Connect）是兩個無關概念（§6.2）：賣家現在同時看到「封鎖名單」與「客戶黑名單」。C10 要併入封鎖時，兩端 UI 都要動；`customerType` 的 `normal \| vip \| blocklist` 要拆成「客戶分級（一般／VIP）」與「封鎖」兩件事【推，brief 決定】。
7. **採購助理的 e2e 綁著 `/app/chat/*`**（4 個 spec，`procurement-*.spec.ts`）：只是為了讓頁面載入而 mock 三條背景路由，但聊天契約改成 Connect 後這些 mock 會失效，容易被當成與聊天無關而漏改【量】。
8. **評價回覆只有 website 有 UI**：W `ProductReviewsPanel.tsx:61` 呼叫 `POST /app/sellers/reviews/{id}/reply`；SA、BA 都沒有；`GET /app/sellers/me/reviews` 只有 SA 的 `order_repository.dart:240` 用來翻頁找單一訂單品項的評價（最多翻 `maxPages` 頁）。新契約應提供「依訂單品項取評價」，不要沿用這個翻頁搜尋【推】。
9. **monolith 的 gateway proxy 現在負責轉 WS upgrade**（`backend/internal/legacy/gatewayproxy/proxy_test.go:189` 測 `GET /api/v1/app/chat/ws?ticket=t`）【量：只看了測試的存在，沒有讀 proxy 實作】；IM 搬進單體前，這條轉發要保持可用。

## 9. 還不確定的事

- A6 的 service／方法命名與是否保留 WebSocket（§2.4）：brief 還沒有決定，上面「去向」欄只寫模組分類。
- 聊天封鎖與評價封鎖的「參照」（對話 id、評價 id）與 A7 `blocks` handler 的衝突規則（買家封鎖自己的店等）是否沿用；見 §8 發現 1。
- `ReportTargetKind` 要不要新增 REVIEW、CHAT_MESSAGE，以及 admin 措施表怎麼擴（§8 發現 2）。
- admin 商品列表的 `rating`／`reviewCount` 與 `StoreStats.rating_average` 搬遷後由誰更新（§3）；我沒有讀 backend 的 seller／product 模組，這兩處是依 client 端欄位推論。
- BA、SA 的「動態分派」（Riverpod provider 以字串拼路徑、`extra` 傳 `merchantId`）沒有逐一排除，「無呼叫者」都是靜態 `rg` 結果；SA 的 CRM 呼叫是用 `apiGet`／`apiPost` 輔助函式，路徑字串在 `customer_repository.dart` 內，若有別處以相對路徑組字串，S3 抓不到。
- 沒有啟動任何 App、服務或測試：§6 的「404」依據是 gateway 路由表與 openapi 沒有該路徑，沒有實打驗證。
- `packages/` 為 0 筆是用 P1 的關鍵字搜尋；`packages/hawkr_auth` 等沒有逐檔人工看過。
