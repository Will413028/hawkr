# A8 事實查核：U8 / U10 / U12

查核日 2026-11-23，分支 dev，唯讀。每條結論標明證據等級：「已讀碼」＝讀過該行；「推論」＝由讀碼推得、沒有跑過。

## U8：client 是否對 `type`／`refType`／`data.*` 做互動分支

衍生來源（已讀碼）：`backend/gateway/internal/handler/notification_inbox.go`
- `:21-33` `eventTypeToDisplayType`：`event_type` 大寫並把 `.` 換成 `_`，`type` 才變成 `ORDER_PAID` 這種字串。
- `:35-67` `deriveRefTypeAndID`：優先用 `data.refType`＋`data.refId`；否則由舊鍵 `order_id`→`ORDER`、`conversation_id`→`CHAT`、`certification_id`→`MERCHANT_VERIFICATION`、`post_id`→`POST` 推出。
- `:69-87` `protoToInboxDTO`：回傳 `type`、`refType`、`refId`、原樣的 `data` map。

### 分支清單（file:line、欄位、做什麼）

分類：N＝點擊導航；R＝依 type 觸發資料重讀等副作用；S＝抑制；D＝只影響顯示。

**website**（`website/lib/notifications/resolve.ts`）
| 位置 | 欄位 | 動作 | 類 |
|---|---|---|---|
| `:39-50` isOrderNotification | type（`order`、`order_*`、含 `shipment`／`payment`）、refType（ORDER_REF_TYPES） | 判定訂單類 | N／D |
| `:60-69` isChatNotification | type、refType（CHAT_REF_TYPES） | 判定聊天 | N |
| `:82-95` isPostNotification | type（`post_*`、含 `comment`）、refType | 判定貼文／留言 | N |
| `:97-141` resolvePostId／resolveCommentId | refType、refId、data 多把候選鍵 | 解貼文 id、要高亮的留言 id | N |
| `:175-215` resolveChatTargetId／resolveChatHref | data.roomId 等、refId、role | 賣家 `?chat=`、買家 `/account/messages/<id>` | N |
| `:217-233` resolveOrderNo | data.orderNo／order_no／orderSn 等 7 把鍵，退而求其次 refId | 取訂單號 | N |
| `:235-263` resolveBuyerName／resolveBuyerId | data.buyerName 等 | 賣家訂單通知 body 追加「買家：…」（`:361` 附近 appendMeta） | D |
| `:300-325` isInvoiceNotification／resolveSellerInvoiceHref | type 以 `invoice.` 開頭；含 `seller_tax_profile`→稅務身分頁；含 `complaint`→申訴列表；其餘→訂單詳情 | 賣家發票通知導向，且必須排在訂單分支之前 | N |
| `:327-360` resolveNotification | 上列組合＋`actionUrl` | 最終 href；解不出就不可點 | N |

website 其他：`website/components/shared/NotificationToastBridge.tsx:196,206,272,293`（`type==="message"` 不彈；`type==="order"` 決定 toast kind，D）；`website/lib/notifications/filters.ts:10-19` 與 `website/features/account/screens/notifications/NotificationsClient.tsx:174` 依偏好的 `category`（伺服器給的）做篩選，不依 type。`website/lib/notifications/stream-item.ts:27-36` 只把 SSE 項目正規化成與 REST 同形。

**buyer-app**
| 位置 | 欄位 | 動作 | 類 |
|---|---|---|---|
| `lib/features/notification/presentation/notifications_screen.dart:265-301` `_navigateToDetail` | refType（ORDER／CHAT／AFTERSALE／AFTER_SALE／REFUND）、refId | 點擊導訂單詳情、聊天室、售後詳情；缺 refId 退到列表 | N |
| 同檔 `:605-720` `_iconConfig` | type 約 40 個精確值，再 refType，再 title 關鍵字 | 圖示與顏色 | D |
| 同檔 `:796-835` | refType、type 含 ORDER、data.category、data 多把鍵 | 取訂單號、判定訂單類（顯示用） | D |
| `lib/features/notification/data/push/destination.dart:49-212` `NotificationDestinationResolver.resolve` | payload 的 type／refType／eventType，約 30 個值，另取 order_id／caseNo／conversation_id／postId／productId／refId | 推播點擊導訂單、售後、聊天室、優惠券、貼文、商品、無動作；未知回 null | N |
| 同檔 `statusRefresh`；`router.dart:117-132` `_dispatchStatusRefresh` | 由 type 決定 order／afterSale | 導航後呼叫 `OrderStatusNotifier`／`AfterSaleStatusNotifier.handlePushNotification(payload)` 重讀狀態 | R |
| `lib/features/notification/data/push/manager.dart:348` | type=`chat_message` | 使用者已在該對話室時不彈前景 banner | S |
| `manager.dart:453`、`background_handler.dart:69-94` | type、data.content_preview | 取事件鍵；聊天 body 後備句 | D |

**seller-app**
| 位置 | 欄位 | 動作 | 類 |
|---|---|---|---|
| `lib/features/notification/presentation/notifications_screen.dart:233-283` | refType（ORDER／CHAT／AFTERSALE／AFTER_SALE／REFUND／DELEGATION）、refId、data | 點擊導訂單、聊天室、售後；DELEGATION 走 `navigateDelegation(context, data)` | N |
| `lib/features/notification/data/delegation_navigation.dart:62-110` `resolveDelegationTarget` | data.resourceType（order／storefront／product／post 四值）、resourceName、resourceId | 第二層判別：依被操作資源跳訂單詳情、店面頁、商品列表、貼文動態；解不出不動作。refId 在此是 audit_log_id，不能用來開頁 | N |
| `lib/features/notification/data/push/router.dart:45-153` `navigate` | type／refType／eventType，約 35 個值；data.caseNo、conversation_id、url | 訂單、售後、聊天、錢包、商品 tab、貼文、代行；default 時用 `data.url` 開外部連結 | N |
| `router.dart:48-66`、`seller_status_push.dart:9-14`、`manager.dart:459,501` `isSellerStatusPushType` | type 為 PAYUNI_STATUS、SELLER_REVIEW_STATUS、CERTIFICATION_* | 導到錢包，並重讀店家、認證、收款帳戶、開通摘要 | R／N |
| 同檔 `notifications_screen.dart:559-610` | type 精確值、refType、title | 圖示（同買家） | D |

**admin**：無通知收件匣消費者。`rg -il notification admin/src` 只命中公告送出結果（`admin/src/lib/broadcast-result.ts`）、側欄模板頁連結（`Sidebar.tsx:176`）與代行稽核顯示，沒有讀 inbox 的 type／refType／data。**packages/**：`rg -il notification packages` 只命中 icons 套件，無消費者。

### 結論（U8）

1. 有 per-type 互動，不只是顯示：導航（N，約 14 個分支點）與資料重讀（R，3 處：buyer order／afterSale 的 `statusRefresh`、seller 的認證與收款狀態重讀）與抑制（S，1 處：聊天前景 banner）。純顯示（D）另有圖示、顏色、toast kind、買家名追加。
2. 「8–10 個呈現分類＋伺服器組好的文字＋一個 deep-link／ref 欄位」能涵蓋：所有 N 類（目的地可由伺服器直接給 route kind＋id＋必要參數；含代行通知的 resourceType 第二層判別與 website 發票三種靜態落點，前提是伺服器把「落點」算好給出，而不是讓 client 再判 type）；所有 D 類。
3. 不能涵蓋（需要契約額外攜帶，或保留穩定的 event type 列舉）：
   - R 類：client 需要知道「這則通知代表訂單／售後／認證收款狀態已變，請重讀」。分類與 deep-link 推不出來，要多一個 refresh hint，或保留事件鍵。
   - S 類：需要 chat 分類加 conversationId（deep-link 的 id 可供；但「是聊天」這件事要由分類或 kind 表達）。
   - 這些多半在推播（FCM）路徑，不在 inbox REST 路徑；inbox REST 路徑的消費者只做 N 與 D。
4. 停止條件（U8 出現 per-type 互動）：已觸發，但性質是「導航可被 deep-link 吸收、重讀副作用不能」。是否重開 D6 由主 session 決定；證據上建議的最小契約是「分類＋伺服器文字＋deep-link（kind＋id＋參數）＋refresh hint」，而不是只有前三者。
5. 附帶發現（推論，沒有實測）：gateway 把 `type` 轉成 `INVOICE_ISSUED_SELLER` 形式（`notification_inbox.go:31-33`），website `resolve.ts:300-304` 卻檢查小寫後 `startsWith("invoice.")`，REST 路徑上點分隔不會再存在，這個判斷看起來命中不了；SSE 路徑的 type 形式沒有查。搬遷時若沿用現有字串契約，要先確認這一點。

## U10：seller workspace 即時通道與通知串流是否共用

已讀碼證據：
- `backend/gateway/internal/realtime/seller_workspace_channel.go:14-34` 註解：publish-only channel，**不對應任何 SSE endpoint**；topic 與 NotificationChannel 同為 `notification:user:{storeId}`，事件落在賣家「已開著」的 `/app/notifications/stream` 連線（「不開第二條 stream」）；`BatchEventName` 復用 `notification.batch`，與通知事件在同一 (topic, batch) 窗口合併。
- 同檔 `:41-57`：`Name()="seller_workspace"`、`Exchange()=pkgmq.ExchangeSellerWorkspace`、`RoutingKeys()=seller.order.updated／seller.after_sale.updated`；`AuthorizeSubscribe` 一律回 `ErrForbidden`（不可直接訂閱）。
- 訂閱授權在 `backend/gateway/internal/handler/sse_stream.go`（`:49` 經 `resolveInboxUserType` 取角色，再用收件匣身分建 topic）。
- website：`website/lib/sse/useNotificationStream.ts:44-47` 型別註解寫明同一連線；`:71-72` 兩個 seller handler；`:136`、`:152` 呼叫；`:170` path 為 `/notifications/stream`。`website/lib/notifications/unread-context.tsx:63` 把賣家事件轉成 window 事件 `SELLER_WORKSPACE_EVENT`（`website/lib/notifications/events.ts:35`）；`:118,:124` 掛 handler；`:138` 呼叫 `useNotificationStream`。
- `website/lib/hooks/useSellerWorkspaceRealtime.ts:9-13` 註解：這個 hook **不開連線**，整個分頁唯一一條 stream 由 `NotificationUnreadProvider` 持有；`:78,:82` 只是監聽 window 事件，250ms debounce 後 invalidate seller orders／dashboard／orderCounts／afterSales／shipmentAnomalies（`:34-60`）。

消費者清單（全 client）：
- 唯一的 consumer 鏈：`useNotificationStream`（`unread-context.tsx:138`）→ window 事件 → `useSellerWorkspaceRealtime`，掛在 `website/features/seller-center/components/SellerCenterShell.tsx:77`；另有 `website/features/seller-center/orders/SellerOrdersPage.tsx`（2 處提及）與 `website/lib/hooks/useSellerDashboard.ts`（1 處提及）。
- admin、buyer-app、seller-app、packages：`rg 'notifications/stream|SELLER_WORKSPACE_EVENT|seller_order_updated|onSellerOrderUpdated'` 在這四處 0 命中。buyer-app／seller-app 的 `stream` 命中只是 AG-UI 與其他 SSE（`buyer-app/lib/core/networking/agui_sse.dart` 等），不是這條。
- 生產者（Java）：order-service `OrderEventPublisher.java`、`SellerOrderUpdatedPayload.java`，aftersales-service `AfterSalesEventPublisherImpl.java`、`SellerAfterSaleUpdatedPayload.java`；topology 在 `backend/infra/mq/topology/topology.yaml`；`backend/docs/plans/2026-09-24-go-modular-monolith/events.csv` 有列。

### 結論（U10）

共用，且是三層共用：同一條 HTTP 路徑 `/app/notifications/stream`、同一條 SSE 連線、同一個 hub topic `notification:user:{storeId}`（`notification.batch` 還會把兩類事件合批）。唯一的 consumer 是 website 賣家工作區（無 App、無 admin）。

A8-2 搬遷 `notifications/stream` 若只帶通知本身，賣家工作區會無聲失效（Hub 精確鍵 fan-out，查無訂閱者不會有錯誤或 log，見 channel 檔 `:23-27` 的歷史事故）。A8-2 至少要包含：
1. 同一個 SSE 端點與 hub，並讓 `seller_workspace` 的 publish 側（exchange、兩個 routing key、`ResolveTopic` 讀 `payload.sellerId`＝store_id、`BuildSSEEvent`）也進單體，或明文保留在 gateway 並確保兩者共用同一個 hub 實例。
2. topic 的身分空間仍是 store_id（賣家）／buyer_id（買家），而非 login account_id。
3. 批次事件名維持 `notification.batch`，內層事件混合 notification 與 seller 事件，client 的 `unpackBatch` 已依此運作。
4. 驗收要有賣家情境：發 `seller.order.updated`，在同一條連線上收到並讓 website 失效快取（只驗通知事件不夠）。

## U12：管理員代行（delegation）是否能呼叫 /app 通知路由

已讀碼證據：
- 代行的實體是 `/api/v1/admin/sellers/:store_id/...` 共 38 支，路由在 `backend/gateway/internal/router/routes_delegated.go` 與 `delegated_capability.go`，在 `backend/gateway/internal/middleware/delegated_routes.go:10-30` 以 mount 時登記。`rg -ci notif` 在 `routes_delegated.go`、`delegated_capability.go` 為 0：38 支中沒有通知路由。
- 代行 20 支走 operation credential，`admin` 群組另 14 支走 ADM session；代行身分是 actor（admin id＋ADMIN）注入 gRPC context，**不換發賣家身分**：`backend/gateway/internal/middleware/delegation_context.go:34-50`。代行群組沒有 JWT（`router/routes.go:31-37` 註解）。
- /app 通知路由：`router/routes_notification.go:10-27` 群組只掛 `middleware.JWTAuth()`，沒有 capability、沒有代行判斷；`:13-27` 含 device-token 三支、inbox（list／unread-count／read／read-all／delete）、`/stream`。
- 這些 handler 都先呼叫 `resolveInboxUserType`（`handler/notification_inbox.go:394-407`；呼叫點 `:102,:157,:197,:237,:282`，`notification.go:73,:137`，`sse_stream.go:49`）。它只接受 `loginRole` 1 或 2，其他回 false→400「invalid login role」。admin 的 loginRole 被 `sanitizeActiveRole` 收斂成 0（`middleware/auth.go:70-74`、註解 `:265-270` 寫明 admin 是「no active role」）。
- 偏好路由：賣家 `routes_seller.go:33-34` 掛 `sellerHandle(... authz.CapNotification ...)`，`requireSeller`（`router/seller_capability.go:17-42`）用 `IsSeller(accountType)`（`middleware/account_type.go:17-19`，只認 2、3），admin（type 4）回 403「seller access required」。買家 `routes_buyer.go:15-16` 只有 `JWTAuth()`，handler `notification_preference.go:124-139` 用 `CurrentBuyerID`，admin 無買家身分得 `ErrNoCurrentBuyer`→403。
- `CapNotification`（`gateway/internal/authz/capability.go:46-47`；單體 `internal/modules/seller/internal/model/capability.go:33-34,75`）的註解與 `seller.proto:306-307` 的「通知收件箱、偏好與裝置」是**停權賣家白名單**的語意（停權時仍能用），不是代行白名單。`CapNotification` 並非代行 capability；代行 capability 在 `delegated_capability.go` 另有一組。
- 單體 `backend/internal/httpapi`：`rpc.go:30-62` 的 `AudienceDelegated` 只認服務名以 `DelegatedService` 結尾的 RPC，憑證是 `X-HK-Operation-Session`；單體目前沒有通知模組，`/api/v1/app/notifications/*` 仍由舊 gateway 反代（`internal/httpapi/session.go:92` 只特判 device-token 路徑）。沒有找到把通知 RPC 放進代行名單的程式。

### 結論（U12，現況）

| 路由群組 | 代行（ADM 管理員）能否呼叫 | 依據 |
|---|---|---|
| `/app/notifications` inbox（list、unread-count、read、read-all、delete） | 否 | 無 JWT 的代行群組不掛這些路由；用 admin 的 JWT 打會被 `resolveInboxUserType`（loginRole=0）回 400 |
| `/app/notifications/device-token` | 否 | 同上，`notification.go:73,137` |
| `/app/notifications/stream` | 否 | `sse_stream.go:49` 同樣先過 `resolveInboxUserType` |
| `/app/sellers/me/notification-preferences` | 否 | `requireSeller` 對 admin 回 403 |
| `/app/buyers/me/notification-preferences` | 否 | admin 無買家身分，403 |

D9（代行者不得讀店家通知）與現況一致，是由「代行不換發賣家身分」加上「/app 通知 handler 都要求 loginRole 1 或 2」這兩個結構事實擋住，不是靠通知專屬的規則。搬遷時要守住的是這兩個事實：單體版 inbox RPC 的 audience 不得是 delegated，且不得因為 actor 是 admin 就放行 seller 身分的讀取。證據等級：已讀碼，沒有實際用 admin token 打過（要打需起本機 stack，這次未做）。

## 執行過的指令

- `rg -n "refType|refId|RefType|ref_type|ref_id" website admin buyer-app seller-app packages`（排除 node_modules、gen、*.g.dart、*.freezed.dart；去掉 refresh／refund 雜訊後約 60 行有效命中，涵蓋 buyer-app 與 seller-app 各 5 檔、website 3 檔）
- `rg -n "notifications/stream" website admin buyer-app seller-app packages -c`：website 8 檔、其餘 0
- `rg -n -c "SELLER_WORKSPACE_EVENT"`：website 5 檔（events.ts 1、unread-context.tsx 3、unread-context.unit.test.ts 3、useSellerWorkspaceRealtime.ts 4、其 unit test 2）；其餘 client 0
- `rg -n -c "onSellerOrderUpdated|onSellerAfterSaleUpdated|seller_order_updated|seller\.order\.updated"`：website 4 檔；其餘 client 0
- `rg -n -c "useSellerWorkspaceRealtime"`：website 6 檔
- `rg -n "seller_workspace|SellerWorkspace" -l`（全 repo，排除 md）：26 檔（gateway 3、Java 8、website 9、其他）
- `rg -n "CapNotification" backend --glob '*.go'`：14 命中（gateway authz、router、單體 seller 模組、測試）
- `rg -n "resolveInboxUserType\(" backend/gateway/internal/handler`：9 個呼叫點
- `rg -n "notifications" backend/internal/httpapi backend/internal/legacy/gatewayproxy`：非測試 1 命中（`session.go:92`）
- `rg -il notification admin/src`、`rg -il notification packages`：admin 8 檔皆非 inbox 消費者；packages 僅 icons
- 以 sed 讀 `notification_inbox.go`、`seller_workspace_channel.go`、`routes_notification.go`、`routes.go`、`delegated_routes.go`、`delegation_context.go`、`auth.go`、`seller_capability.go`、`resolve.ts`、兩支 App 的 `notifications_screen.dart`、`destination.dart`、`router.dart`、`delegation_navigation.dart`
- 未做：沒有起 stack、沒有用 admin token 實打、沒有查 SSE 路徑上 `type` 的實際字串形式
