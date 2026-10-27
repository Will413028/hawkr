# A3a 步驟 3：buyer Connect API 設計

## 1. 範圍與結論

本階段只定義契約；handler、port、schema、生成物與 CodeMeta 投影在後續階段實作。前台只有 `BuyerAppService`（25 RPC），後台 `BuyerAdminService`（3 RPC）；自助刪帳號是帳號的操作，放在 `IdentityAppService.DeleteAccount`。盤點 §5 沒有匿名 buyer 路由，因此不建立 `BuyerPublicService`。ID 均為不透明 `string`；目前 buyer、address、product、store 的 bigint 在 wire 上用十進位字串，client 不解析。前後台訊息分開；後台目前沒有與前台完全相同的值型別需要 import。

從零設計，買家 profile、單筆地址、方案各自需要獨立版本；它們有獨立讀取與修改入口。收藏／追蹤／補貨是以關聯鍵識別的命令，不把舊 REST 的 offset、Redis、TGOS、CodeMeta 數字值或 `MemberLevel` 搬進新契約。帳號自助刪除按計畫決定 4a 補後端入口。圖片遵守 image ADR，只有 `Image` 輸出與 `avatar_staged_token` 輸入；buyer `UpdateAvatar` 現已接受 token（`buyer.go:240`，`internal/profile/profile.go:305-330`），可沿用 content 的 staged-token 邊界做法。

## 2. Gateway operation 對照

依盤點 §5 的 40 個 OpenAPI operation 加 2 個未列入 OpenAPI 的路由；另將盤點明指的兩條 TGOS 路由逐列列出（已包含在 40 個）。`a`＝新 RPC，`b`＝刪除，`c`＝留在別的模組／gateway。每列恰好一個去向。證據：盤點 §5、§5.1、§6、§8.1；operationId 取自 `backend/api/openapi/openapi.yaml`，未列入者取 `gateway/internal/router/openapi_coverage_test.go:74,77`。

| Gateway method / path（operationId） | 去向與依據 |
|---|---|
| GET `/app/buyers/me`（getMemberProfile） | a `GetProfile`；website、buyer-app 使用；seller-app 的誤用在 A3b 改讀 seller profile（盤點 D14）。 |
| PUT `/app/buyers/me`（updateMemberProfile） | a `UpdateProfile`；profile etag。 |
| POST `/app/buyers/me`（ensureMyBuyerProfile） | b；沒有 client，登入時 `GetSession`／入口確保 profile（計畫「不搬」）。 |
| DELETE `/app/buyers/me`（未列規格） | a `IdentityAppService.DeleteAccount`；決定 4a 與 App Store 刪帳要求，現無 client。刪的是帳號（步驟 8 加上 seller），所以放在 identity 的 service。 |
| PUT `/app/buyers/me/avatar`（updateBuyerAvatar） | a `UpdateAvatar`；website、buyer-app 使用，seller-app 誤用於 A3b 移走。 |
| GET `/app/buyers/me/addresses`（getAddressList） | a `ListAddresses`。 |
| POST `/app/buyers/me/addresses`（addAddress） | a `CreateAddress`。 |
| GET `/app/buyers/me/addresses/{id}`（getAddress） | a `GetAddress`。 |
| PUT `/app/buyers/me/addresses/{id}`（updateAddress） | a `UpdateAddress`。 |
| DELETE `/app/buyers/me/addresses/{id}`（deleteAddress） | a `DeleteAddress`。 |
| GET `/app/address/suggestions`（addressSuggestions） | b；TGOS 無可用自有帳號，決定 9a 移除 buyer/seller 共用入口。 |
| GET `/app/address/validate`（addressValidate） | b；同上；Java shipping 的舊 `ValidateAddress` 需在步驟 4 以明確停用錯誤維持 legacy 契約，並標出最後一個 caller 後移除。 |
| GET `/app/buyers/me/favorites`（getFavoriteList） | a `ListFavorites`；product、seller 顯示在 httpapi 補。 |
| POST `/app/buyers/me/favorites`（addFavorite） | a `AddFavorite`。 |
| GET `/app/buyers/me/favorites/count`（getFavoriteCount） | b；四端都沒有呼叫（`website/lib/favorites/repo.ts:9` 註明不呼叫）。收藏頁要的總數由 `ListFavorites.total_size` 提供。 |
| GET `/app/buyers/me/favorites/{id}`（checkFavorite） | a `CheckFavorite`。 |
| DELETE `/app/buyers/me/favorites/{id}`（removeFavorite） | a `RemoveFavorite`。 |
| POST `/app/buyers/me/subscriptions`（subscribeSeller） | a `FollowStore`。 |
| DELETE `/app/buyers/me/subscriptions/{store_id}`（unsubscribeSeller） | a `UnfollowStore`。 |
| GET `/app/buyers/me/subscriptions/{store_id}`（checkSubscription） | a `CheckFollow`。 |
| GET `/app/buyers/me/subscriptions`（未列規格） | a `ListFollows`；website、buyer-app 使用。 |
| GET `/app/buyers/me/followers`（getFollowersList） | a `ListFollowers`；website、seller-app 需要；授權賣家角色且只查自己店。 |
| GET `/app/buyers/me/saves`（getSavedItems） | b；收藏中心的合併列表只為舊路由存在（模組 `AGENTS.md`：不新增 GetSavedItems 形狀）。website 的收藏頁本來就把商品與店家分成兩組（`FavoritesClient.tsx:79,83`），改呼叫 `ListFavorites`、`ListFollows`，分頁標籤的數字用兩者的 `total_size`。 |
| GET `/app/buyers/me/member-cards`（getMemberCards） | a `ListMemberCards`；buyer-app／website 使用。 |
| GET `/app/buyers/me/member-cards/{seller_id}/records`（getSpendingRecords） | a `ListSpendingRecords`；order 提供紀錄。 |
| GET `/app/buyers/me/browse-history`（getVisitList） | a `ListVisits`。 |
| POST `/app/buyers/me/browse-history`（recordVisit） | a `RecordVisit`；唯一寫入瀏覽事件的 RPC。 |
| DELETE `/app/buyers/me/browse-history`（clearVisitHistory） | a `ClearVisits`。 |
| GET `/app/buyers/me/restock_subscriptions`（listMyRestockSubscriptions） | a `ListRestockSubscriptions`；website 使用，即使 buyer-app 尚未呼叫。 |
| POST `/app/products/{id}/restock_subscriptions`（subscribeRestock） | a `SubscribeRestock`。 |
| DELETE `/app/products/{id}/restock_subscriptions`（unsubscribeRestock） | a `UnsubscribeRestock`。 |
| GET `/app/buyers/me/assistant/quota`（proxyGetBuyerAssistantQuota） | b；四端無 caller，計畫「不搬」；ai-service 的內部 `GetBuyerEntitlement` 仍保留於步驟 4 legacy。 |
| GET `/app/sellers/me/customers/lookup`（lookupBuyer） | c CRM A6；需 buyer legacy `GetBuyerProfileByBuyerID`／`BatchGetBuyerMetaByBuyerIDs`，手機精確查 buyer 的 phone（決定 10a）。 |
| GET `/app/sellers/me/customers/{buyerId}/meta`（getBuyerMeta） | c CRM A6；需 buyer legacy `GetBuyerProfileByBuyerID`／`BatchGetBuyerMetaByBuyerIDs`。 |
| GET `/app/sellers/me/dashboard`（getSellerDashboard） | c seller A3b／CRM A6；需 buyer legacy `GetSubscriberDashboardStats`／`BatchGetSubscriberCounts`，見 §4 缺口。 |
| GET `/admin/buyers/stats`（getAdminBuyerStats） | a admin `GetBuyerStats`。 |
| DELETE `/admin/accounts/{id}`（deleteAccount） | c A3b 步驟 8 的跨 seller／buyer／identity 原子 use case；需 accountdeletion `Request` 管理員路徑與 legacy `DeleteBuyerAccount` 到切換。 |
| GET `/admin/agent_entitlements/{buyer_id}`（proxyGetAdminAgentEntitlement） | b；沒有後台頁面，計畫「不搬」；ai-service 的內部權益 RPC 保留。 |
| POST `/admin/agent_entitlements/{buyer_id}/assign`（proxyAssignAdminAgentEntitlement） | b；沒有後台頁面，`AssignMembership` 不做對外 admin RPC。 |
| GET `/admin/membership_plans`（proxyListMembershipPlans） | a admin `ListMembershipPlans`。 |
| PATCH `/admin/membership_plans/{plan_code}`（proxyUpdateMembershipPlan） | a admin `UpdateMembershipPlan`；etag。 |
| PUT `/app/buyers/me/seller_binding`（bindSeller） | b；建店與 SELLER 授權在 seller A3b 同一交易，計畫「不搬」。 |

## 3. RPC 清單

所有 App 方法須登入；profile、地址、收藏、追蹤、瀏覽、補貨、會員卡及刪帳只給帳號（每個帳號都是買家，買家不是角色，W5；管理員回 permission_denied），`ListFollowers` 須 SELLER 角色及 caller 自己的 store。Admin 三支都須管理員。讀取方法在 proto 標 `NO_SIDE_EFFECTS`，不得寫入或記錄瀏覽。未列出的讀取錯誤一律依 Connect 規範 §八（`not_found`、`invalid_argument`、`unavailable` 等），資源不屬 caller 時與不存在同為 `not_found`。新增／取消收藏、追蹤、補貨及清除瀏覽在重送時維持最終狀態且不重複事件；`RecordVisit` 每次都產生事件，不能自動重送。`Update*` 以 etag 防重複覆蓋；`DeleteAccount` 回目前狀態，重送安全（`IDEMPOTENT`）。

| RPC | request → response、顯示欄來源與特別錯誤 |
|---|---|
| `GetProfile`、`UpdateProfile`、`UpdateAvatar` | caller → `Profile`；buyer 的暱稱、電話、性別、生日、城市、職業、狀態，media object key 組 `Image`。Update 帶資源 etag／mask；換圖帶 staged token。9203 只限制新增收藏／追蹤，不限制 profile。 |
| `ListAddresses`、`GetAddress`、`CreateAddress`、`UpdateAddress`、`DeleteAddress` | caller 與 address id／page → buyer 地址；清單按更新時間與地址 ID 做 keyset。修改要 etag，Delete 選填；越權 `not_found`。 |
| `ListFavorites`、`CheckFavorite`、`AddFavorite`、`RemoveFavorite` | product id／page → buyer 關聯、時間／count／bool；清單商品名、圖、最低／最高價、評分、可買狀態由 product，店名、短碼、logo 由 seller port 補；補貨標記由 buyer `BatchCheckRestock`（`website/lib/favorites/repo.ts:29-65`）。新增於 pending 回 `failed_precondition`。 |
| `ListFollows`、`CheckFollow`、`FollowStore`、`UnfollowStore` | store id／page → buyer 關聯、時間／bool；店名、圖、營業狀態由 seller port。Follow 先驗證店家存在、未停權且非自己店；pending 回 `failed_precondition`。 |
| `ListFollowers` | seller caller／page → buyer `Subscribers` 與 `BatchDisplay`；CLOSED 回 `deleted=true`、名稱與圖省略，文字由 client 呈現。seller-service 現在以 port 解析自己 store，A3b 換 seller 模組；不延到 A3b，因 seller-app／website 已有 caller。 |
| `ListMemberCards` | page → order 的店家累計消費額、seller 名／logo／封面／短碼、crm VIP；buyer 僅提供 caller buyer id（`gateway/internal/handler/member_card.go:16-34`）。 |
| `ListSpendingRecords` | store id／page → order 的訂單號、金額、退款、優惠券標記、時間與累計消費／退款額；無此 store 的紀錄為空（`website/lib/member-cards/repo.ts:34-60`）。 |
| `ListVisits`、`RecordVisit`、`ClearVisits` | page／product id → buyer 最後瀏覽時間，product 補名稱、圖、最低／最高價、評分與可用狀態，seller 補名稱、短碼與 logo，buyer 補貨標記；只有 RecordVisit 寫入（`website/lib/visits/repo.ts:14-49`）。 |
| `ListRestockSubscriptions`、`SubscribeRestock`、`UnsubscribeRestock` | page／product id → buyer 待通知關聯與時間，product／seller 補顯示；取消與通知後不留終態歷史。 |
| identity `DeleteAccount` | caller → `DELETED` 或 `PENDING_ORDERS`；有 SELLER 角色回 `failed_precondition`＋`LocalizedMessage` 提示先關店；查訂單不可用回 `unavailable`，不得推定無訂單。 |
| admin `GetBuyerStats` | 空 request → buyer 總數；buyer `StatsForAdmin`。 |
| admin `ListMembershipPlans`、`UpdateMembershipPlan` | 空／方案＋mask → buyer 固定種子方案；方案碼、名稱、月回答額度、月價格、是否可買與版本均來自 buyer。無效方案 `invalid_argument` 或 `not_found`（見 §5），衝突 `aborted`＋`VERSION_CONFLICT`。 |

清單用 AIP-158 的 `page_size`／`page_token`／`next_page_token`。buyer 的 Favorites、Follows、Visits、Restock、Subscribers 都已有時間＋ID keyset；地址需在 phase 2 新增更新時間＋地址 ID 的 keyset。預設 20、上限 100（`internal/model/model.go:145-195`；`pagination.PageSize`）；負數與無法解碼或請求參數指紋不符的 token 回 `invalid_argument`。order、crm 的頁面資料須由 adapter 轉成可穩定延續的 token，不把舊 offset 暴露給 client。只有 `ListFavorites`、`ListFollows` 回 `total_size`（收藏頁的分頁標籤）。已刪除的商品保留卡片，`ProductCard.deleted=true`、其他欄位不設；下架或售完是 `available=false`；店家關店或停權是 `StoreCard.active=false`。product 或 seller 讀不到時整個請求回 `unavailable`，不回部分資料（舊 REST 的 `degraded`／`productInfoUnavailable` 只為遠端呼叫存在，B 階段搬進單體後就沒有這個狀態，不放進契約）；瀏覽清單依舊契約只顯示 ACTIVE 商品時可過濾但必須在頁面補滿時繼續拉取 keyset，避免空洞。CLOSED buyer 的墓碑由 `deleted=true` 表示，文字不存於模組。

## 4. Ports 與 legacy RPC

步驟 2 的 buyer 模組不依賴 product、seller、order、crm；步驟 3 的 `internal/httpapi/buyer` 接下列 port。`backend/buf.gen.legacy.yaml` 目前只列 account、buyer、common、media、seller；product、order、crm 的 proto 尚未生成到單體，步驟 2 實作 httpapi 前須把對應 `api/proto/.../*.proto` 加入 `inputs.paths` 並生成，不能手改 stub。

| port / 用途 | 舊 gRPC RPC（`backend/api/proto`） | 現有、單體可呼叫？ |
|---|---|---|
| ProductCards：收藏、瀏覽、補貨 | `product.ProductService.GetProductsByIDs` | proto 與 Java handler 現有；單體尚無 product stub，須加 `api/proto/product/product.proto`。 |
| SellerDisplay／SellerExistence／OwnStore：店家卡、追蹤檢查、粉絲 caller | `seller.SellerService.GetSellerBatch`、`GetSeller`、`GetCurrentStoreID` | proto、seller stub 現有；A3b 後改 seller 公開 API。 |
| OrderMerchantSummary／OrderSpendingRecords | `order.OrderService.GetBuyerMerchantSummary`、`GetBuyerOrdersForSeller` | proto 與 Java handler 現有；單體尚無 order stub，須加 `api/proto/order/order.proto`。 |
| OpenOrders：刪帳前查該 buyer 的未結訂單 | `order.OrderService.GetMyOrderStatusCounts`（`order.proto:503`；以狀態計數，避免翻頁漏單） | proto 現有；單體尚無 order stub。未結狀態集合沿 ordering 的 SSOT，不在 buyer 寫死。 |
| BuyerVipStatuses：會員卡 | `crm.CrmService.GetBuyerVipStatuses` | proto 現有；單體尚無 crm stub，須加 `api/proto/crm/crm.proto`；A6 改 crm API。 |

步驟 4 的 buyer legacy server：計畫已有 Java 六支 `GetAddress`、`ValidateAddress`、`ListFavoritedProductIds`、`ListSubscribedSellerIds`、`BatchGetBuyerDisplayByBuyerIDs`、`ResolvePrincipals`；晚搬 Go 六支 `GetBuyerProfileByBuyerID`、`GetAddressList`、`BatchGetBuyerMetaByBuyerIDs`、`GetSubscribersList`、`ListBuyerIDsByAccount`、`GetBuyerEntitlement`；另有 gateway 商品疊加的 `BatchCheckFavorites`、`BatchCheckRestockSubscriptions`、`CheckFavorite`。**計畫步驟 4 缺 `GetSubscriberDashboardStats`、`BatchGetSubscriberCounts`、`DeleteBuyerAccount`**：seller dashboard、seller-service／discovery 仍需前兩支，後台 `DELETE /admin/accounts/{id}` 到 A3b 步驟 8 前仍需第三支；A3b 切換前不能少。`GetBuyerProfile` 原本的 gateway caller 已由 `X-Buyer-Id` 取代。`GET /app/sellers/me/customers/lookup` 的手機查詢須等 CRM A6 接 buyer phone lookup port；目前 module 只有 `ProfileByBuyerID`／`BatchMeta`，沒有 phone index lookup API。

## 5. 錯誤對照

| 舊碼 | Connect code 與 details | 分支證據 |
|---|---|---|
| 9203 `BUYER_ACCOUNT_DELETING` | `failed_precondition`＋`LocalizedMessage(zh-TW)`；pending 不准新增收藏／追蹤。 | 四端在 buyer 呼叫上沒有依 9203 分支。 |
| 9204 `ACCOUNT_ALREADY_IN_DELETION` | 自助重複申請改回成功與目前的 `DeleteAccountResponse.state`；其他不允許動作才 `failed_precondition`＋`LocalizedMessage`。 | seller-app `lib/features/seller/data/seller_repository.dart:243` 僅註解提到舊碼；buyer 刪帳無 caller。 |
| 9400 `INVALID_PLAN_CODE` | 指派輸入用 `invalid_argument`＋`BadRequest(plan_code)`＋`LocalizedMessage`；以既有 plan_code 查／改不存在為 `not_found`＋`LocalizedMessage`。 | admin 方案頁沒有依 9400 分支；管理員指派入口刪除。 |
| 9700 地址查詢停用 | 新 API 沒有 TGOS RPC。legacy `BuyerService.ValidateAddress`（Java shipping 仍呼叫）固定回業務碼 9700，不回 gRPC 錯誤：shipping 把 code != 0 轉成「Uber 不可用」降級，`unavailable`／`unimplemented` 則會讓整個報價失敗（計畫決定 9 的後果），直到 shipping 搬遷或接上地理編碼。 | 兩 App 的 TGOS service 處理一般失敗／可手填，沒有依 9700 分支。 |
| 9233 賣家不存在（buyer 流程） | `not_found`＋`LocalizedMessage`（非自己店或不可見同形）；不加 buyer 專屬 reason。 | seller-app `lib/core/session/session_provider.dart:30,255`、website `lib/seller/repo.ts:36-47` 對 9233 的分支屬 **seller profile 登入／開店流程**，應由 A3b seller 契約決定；buyer 的 Follow 不共用此分支。 |

四端搜尋命令：`rg -n '9203|9204|9400|9700|9233' website/lib website/features admin/src buyer-app/lib seller-app/lib -g '*.ts' -g '*.tsx' -g '*.dart'`；命中其餘訂單取消原因與生成 CodeMeta，並非 buyer 錯誤處理。這五碼沒有在相同 Connect code 下需要額外分支的 buyer client，因此**不修改 `ErrorReason`**；既有 `VERSION_CONFLICT` 用於 etag 衝突。所有使用者文字只在 `LocalizedMessage`，`message` 不給 UI 顯示。

## 6. 模組要改的地方

1. `buyer.buyers`、`buyer.shipping_addresses`、`buyer.membership_plans` 各加 `version bigint NOT NULL DEFAULT 1`；讀取回 etag，`UpdateProfile`、`UpdateAvatar`、`UpdateAddress`、`DeleteAddress`（帶 etag 時）、`UpdatePlan` 比對版本，條件式 `UPDATE ... WHERE version = $expected`，0 列分辨 `not_found`／`aborted`。傳輸層先取目前資源套 `update_mask`，模組在轉正圖片前檢查、最後由 SQL CAS 決定，回應取 `RETURNING`。新增的 `version` 與 proto 轉換雙向測試由 phase 2 補。
2. 地址是**獨立 aggregate root**：單筆地址有獨立讀／改／刪與穩定 ID，符合 row-identity ADR D1 的有身分子資料；buyer row 仍作為預設地址切換的序列化鎖，但不是所有地址的 etag。建立／更新為 default 時，同交易把舊 default 改為 false 並各自 bump version；刪掉 default 而遞補新 default 時，也 bump 新 default 版本。這能讓持有舊地址 etag 的 client 發現預設狀態已變。profile 頭像、profile 欄位、stint 狀態改變都 bump buyer version；方案欄位任一路徑改變都 bump plan version。收藏數／瀏覽原始計數不屬可編輯資源，不 bump。
3. `UpdateProfileInput`、`AddressInput`、`UpdatePlanInput` 目前沒有 ExpectedVersion；`DeleteAddress`、`UpdateAvatar` 沒有 etag 入參或 version 回傳，phase 2 一起擴充。`Profile`、`Address`、`Plan` 與讀取 SQL 要回 version。`Plan.MonthlyPriceTWD` 目前是 `int32`，wire 金額依 Connect 規範用 `int64`；phase 2 要在模組與 DB 對齊範圍或明確驗證轉換，不能截斷。刪帳申請改由 `accountdeletion.Request` 回傳 pending／completed 狀態；pending 後重送仍重新檢查訂單，不用 9204 表示重複。
4. `BatchDisplay` 對 CLOSED 已有 `Deleted=true`，`BatchMeta`／`ProfileByBuyerID` 不曝露 CLOSED PII；`ListRestock` 已有 keyset。收藏與追蹤各自回 `total_size`，模組需提供計數（收藏已有 `FavoriteCount`，追蹤要補）。CRM A6 手機查詢需 buyer 的 phone lookup 公開 API 與索引，A3a 不為 CRM 額外開前台 RPC。商品已刪與店家關閉的顯示是 httpapi adapter／呈現層處理，模組不存文案。
5. Proto enum 對 Go／CHECK 的映射：`BuyerStatus` ACTIVE／PENDING_DELETION（呼叫者自己的 profile 不會是 CLOSED，不對外），`Gender` MALE／FEMALE／OTHER，`AddressLabel` HOME／COMPANY／OTHER，會員的 status、source 沒有對外的 RPC，不進契約。名稱去型別前綴等於 Go 常數值，測試應驗證雙向映射並明列不對外的內部代碼。`DeleteAccountResponse.DeletionState` 的 DELETED／PENDING_ORDERS 分別對應關閉／待訂單完成的 use case 結果，它是流程的結果，不是資料庫欄位。`MemberLevel`、email、notifications 不出現在契約。
6. CodeMeta 投影在 phase 2／5 標擁有者：`業務狀態碼.md` 的 `RestockSubscriptionStatus` 由 buyer 決定退役（新模組只保留待通知關聯）；`內部技術碼.md` 的 `BuyerEventType` 由 buyer 擁有但舊 product 仍消費，保留舊投影到 product 搬遷；`MembershipEventType`／`MembershipBillingErrorCode` 的 buyer 計費部分是 D8 刪除，其他 Java／shared 使用者仍留 CodeMeta；`AssetReferenceType.BUYER_AVATAR` 由 media 擁有，`ProductEventType` 由 product 擁有，`OrderStatus` 由 order 擁有，不轉成 buyer 值域。錯誤碼 9203／9204／9400 是舊契約投影，於買家 REST 刪除後依 consumer 減少；9700 與 seller 共用、9233 屬 seller，不標 buyer owner。本階段不編輯 CodeMeta。

## 7. 刪帳號 use case

`IdentityAppService.DeleteAccount` 從 authenticated principal 取得 account_id，呼叫 `accountdeletion.Request`（`ACCOUNT_OWNER`）。`accountdeletion` 是擁有 `deletion_requests` 的流程管理模組：依 identity 角色判斷 SELLER 是否仍在（有則 `failed_precondition` 且不寫入），透過 buyer 公開 API 取得 current stint，於 transaction 前以 Orders port 查未結訂單（失敗回 `unavailable`，不寫入）。同一交易建立或重用流程狀態，並為每次受理追加不可變的申請紀錄、呼叫 buyer 的 `BeginDeletion` 或 `CloseForDeletion`，並在無其他角色時呼叫 `identity.DeleteAccount`。有未結訂單回 `PENDING_ORDERS` 且保留 BUYER；無未結訂單回 `DELETED`。重送 pending 使用同一申請；帳號已刪回 `DELETED`。River 每小時且啟動時掃描待辦申請並呼叫 `CompletePending`，即使事件早於申請提交也能收斂；步驟 4 的 `order.completed` consumer 先以 buyer 公開 API 從 buyer_id 解析 account_id，再呼叫 `accountdeletion.CompletePending` 以加速完成；A3b 加入 seller participant。App 入口 UI 尚未授權，後端只提供 RPC。

## 8. 未決

無新的 owner trade-off。order／crm 分頁 adapter 屬 phase 2 實作細節；若 Java RPC 無法以既有欄位保證穩定翻頁，應在實作前提出具體契約缺口，不以 offset token 假裝 keyset。

實作偏差：`ListSpendingRecords` 的 Java RPC 沒有 keyset 游標，也沒有 page size 上限；過渡 adapter 每次以 100 筆走完 offset 頁、核對 total 與重複訂單號，再按 `(order_time DESC, order_no DESC)` 排序並向外提供有 scope 指紋的 keyset token。總額由同一份全量紀錄加總；`OrderRepository` 的摘要與明細使用相同的 completed／partially refunded 狀態集合及金額欄位。來源有新增或刪除時，跨請求重新讀取仍可能改變頁面位置；order 搬入單體時應改由資料庫提供穩定 keyset。
