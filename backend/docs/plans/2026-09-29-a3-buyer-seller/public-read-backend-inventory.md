# A3b 10d（公開讀取）backend 盤點

快照：main `af1b0dcc7`（2026-10-15）。唯讀。路徑：`G=backend/gateway/internal`、`H=backend/internal/httpapi`、`M=backend/internal/modules`。

## 0. 範圍（10d 擁有什麼）

來源：
- 計畫 `backend/docs/plans/2026-09-29-a3-buyer-seller.md:297`（10d）：GetStore、BatchGetStores、推薦（marketplace 由 client 組合）、`RecordProductView`、店家卡片、刪 `store_id` 轉址、buyer-app 以 `store_code` 導頁、seller-app `page_echo_tab.dart`；刪「對應的」`/public/sellers/*` 與 `/products/:id/view`。
- `step10-decision-brief.md:190`：被取代的路由列為 `/public/sellers/{id}`、`/stats`、`/rating`、`/recommended`、`/marketplace`、`/products/:id/view`；UI 同意項 B6、B11、B19。
- 計畫:202（10b D5）＋ `step10b-decision-brief.md:162`：沒有消費者的治理路由 `POST /app/violation_reports`、`GET /app/violation_reports/me`、`POST /public/food_safety_reports`、`GET /public/sellers/:id/violations/public`「留到 10d（B6 的 Connect 入口在那一刀）」。
- 計畫:153 B6（店面頁與商品頁「檢舉商家」用 `CreateReport`；website 匿名食安通報用 `SubmitFoodSafetyReport`）、:158 B11（卡片依狀態）、:166 B19（拿掉「已認證」徽章）、:164 照 SSOT 處理：「匿名瀏覽只給未登入者送 `RecordProductView`」「`PublicStore` 補 `seo_title`、`seo_description`」（**這一項已被 local/10e 的 ADR `2026-10-09-store-seo-platform-generated` 推翻**，見 §6）。
- 步驟 11（計畫:299）：刪 seller-service 與它在 `api/proto` 的 proto、盤點 CSV 的去向、改寫設計書 28、buyerserver `GetSubscriberDashboardStats`。K24（`step10c-decision-brief.md:49`）：只刪路由的 commit 不動 `rpc-servers.csv`；刪 gateway wrapper 時刪 `rpc-calls.csv` 的 gateway 列（10c 第 5 刀 `af1b0dcc7` 的先例：`GetProductViewStats` 的 gateway 列刪了、`rpc-servers.csv` 那列不動）；被刪 handler 是最後呼叫者的 seller-service RPC 留到步驟 11。

不在 10d（`/public/sellers/*` 底下但屬於別的模組，留著）：`/:id/products`（catalog，B）、`/:id/reviews`、`/:id/reviews/stats`（review，A6）、`/:id/posts`、`/:id/forum`、`/forum/:id/replies`（social，A7）。

## 1. Gateway 路由

`/public/sellers/*` 共 11 條、加 `/products/:id/view`、`/marketplace`；10d 刪除 6 條核心＋4 條零消費者治理路由。

### 1.1 10d 要刪的（核心 6）

| # | 路由 | 註冊 | handler | 呼叫的 gRPC（gateway wrapper） | openapi（`backend/api/openapi/openapi.yaml`） | 本機現況 |
|---|---|---|---|---|---|---|
| R1 | GET `/public/sellers/recommended` | `G/router/routes_seller.go:18` | `handler/recommended_sellers.go:21` `GetRecommendedSellers` | `SellerService.GetRecommendedSellers`（`grpc/seller_client.go:184`） | `:31417` getRecommendedSellers | 壞：單體 `sellerserver` 沒實作（Unimplemented） |
| R2 | GET `/public/sellers/:id` | `routes_seller.go:23`（`OptionalJWTAuth`） | `handler/seller.go:133` `GetSellerInfo`（DTO `PublicSellerInfo` :49、`toPublicSellerInfoDTO` :76） | `GetSeller`（`GetSellerByStoreID` :78）＋ fire-and-forget `RecordStoreVisit`（:158，經 `h.BuyerID.CurrentBuyerID`） | `:31493` getSellerInfo | GetSeller 可用（只認 store_id）；RecordStoreVisit Unimplemented，靜默失敗 |
| R3 | GET `/public/sellers/:id/stats` | `routes_seller.go:24` | `seller.go:263` `GetSellerStats`（DTO `SellerStats` :109） | `GetSellerStats`（`GetSellerStatsByStoreID` :85） | `:31881` getSellerStats | 可用（total_* 一直是 0） |
| R4 | GET `/public/sellers/:id/rating` | `routes_seller.go:25` | `seller.go:226` `GetSellerRating`（DTO `SellerRating` :213） | `GetSellerStats`（同上） | `:31746` getSellerRating | 可用 |
| R5 | GET `/public/marketplace` | `routes_marketplace.go:13`（`routes.go:92` 呼叫 `registerMarketplaceRoutes`） | `handler/marketplace.go:29` `GetMarketplace` | `GetRecommendedSellers`（:58）、`Social.GetPostList`、`Product.GetPublicProductList(sales_desc)`、`GetSellerBatch`（經 `enrichPostsWithSellerInfo` :227） | `:30156` getMarketplace | 賣家區塊壞（Unimplemented，best-effort 回空），貼文與商品可用 |
| R6 | POST `/public/products/:id/view` | `routes_product.go:27`（常數 `ProductsView` `constants.go:72`） | `handler/product_view.go:18` `RecordProductView`（永遠回 200、錯誤吞掉） | `RecordProductView`（:172） | `:31145` recordProductView | 壞：Unimplemented（被吞，client 看不出） |

### 1.2 10d 要刪的（零消費者治理 4，10b D5 移交）

| # | 路由 | 註冊 | handler | gRPC（`grpc/governance_client.go`） | openapi | 消費者 |
|---|---|---|---|---|---|---|
| R7 | GET `/public/sellers/:id/violations/public` | `routes_governance.go:58`（`publicSellers` group :54） | `handler/governance.go:403` | `GetPublicSellerViolations`（:38） | `:31919` | 0（ADR `2026-10-18-seller-governance-consolidated` 公開違規列表刪除） |
| R8 | POST `/public/food_safety_reports` | `routes_governance.go:46-53`（`DualWindowRateLimit` 5/10min＋10/h） | `governance.go:365` | `CreateFoodSafetyReport`（:30） | `:30016` | 0；Connect `SellerReportPublicService.SubmitFoodSafetyReport` 已在（`H/ratelimit.go:62` `tierReportSubmit`） |
| R9 | POST `/app/violation_reports` | `routes_governance.go:62-66` | `governance.go:274` | `CreateViolationReport`（:18） | `:29423` | 0；Connect `SellerReportAppService.CreateReport` |
| R10 | GET `/app/violation_reports/me` | 同上 | `governance.go:318` | `ListMyViolationReports`（:24） | `:29488` | 0；Connect `ListMyReports` |

驗證零消費者：`rg -n "violation_reports|food_safety_reports|violations/public" website buyer-app/lib seller-app/lib`（排除 admin 與 gen）無命中；admin 只用 `/admin/violation_reports`、`/admin/food_safety_reports/*`（10e 範圍）。單體 `sellerserver` 對這 4 支都是 Unimplemented。

### 1.3 留下的 `/public/sellers/*`（別的模組）

`/:id/products`（`routes_seller.go:26`，`seller.go:307`）、`/:id/reviews`、`/:id/reviews/stats`（:27-28，`order_review.go:329,392`）、`/:id/posts`（`routes_post.go:29`，`post.go:71`）、`/:id/forum`、`/forum/:id/replies`（`routes_post.go:87,89`）。

- 這些都經 `resolveStorefrontSellerID`（`seller.go:180`）以 legacy `GetSeller(StoreId)` 解析。**單體的 `sellerserver.GetSeller` 只以 `store_id` 查（`StoreWithSanction(store_id)`），不認 `store_code`**；gateway 註解與 swag `@Param`（「store_id 或 store_code」）以及 `seller_storefront_resolve_test.go` 的 `TestResolveStorefrontSellerID_ResolvesStoreCode`（mock）都已過期。店面頁改成 `/sellers/{store_code}` 之後，client 必須用 `GetStoreResponse.store.id` 打這些子路由。
- 它們不檢查「可公開曝光」：任何沒關的店都能以連號 `store_id` 讀商品、評價、貼文（GetSeller 只濾 CLOSED）。見 §5 Q3。
- 移除 `/:id` 的 GET 後，gin tree 仍有 `/:id/...` 與靜態 `forum`、`recommended`（後者也刪），沒有 wildcard 名稱衝突（`routes_governance.go:55-57` 的註解隨 R7 一起刪）。

### 1.4 `maxRESTOperations`

- 現值 `508`：`G/router/openapi_coverage_test.go:27`，斷言在 :170-173（大於上限失敗、小於也失敗，要求鎖新低點）。openapi 實算 508 ops（yaml 解析 get/post/put/patch/delete）。
- 10d 只刪核心 6：**508 → 502**；連零消費者治理 4：**508 → 498**。
- local/10e 刪 16 個 admin op（main 508 → 10e 492，兩邊沒有交集）。兩者都合併後：492 − 6 = 486，或 492 − 10 = **482**。後合併的那一邊要重算。

### 1.5 隨路由一起改的 gateway 檔

- router：`routes_seller.go`（public 區塊 :14-29 只剩 products、reviews 兩組，或併到別處）、`routes_product.go:27`、`routes_marketplace.go`（整檔）＋`routes.go:92`、`routes_governance.go`（public food safety、public sellers、app violation_reports 區塊與檔頭註解 :14-18）、`constants.go`（`SellersIDPath`、`SellersStats`、`SellersRating`、`SellersRecommended`、`ProductsView` 視使用情況；`SellersPath` 仍被 pricebook、post 用）。
- handler：`seller.go`（`PublicSellerInfo`、`toPublicSellerInfoDTO`、`SellerStats`、`SellerRating`、三個 handler；`syncutil`、`identity`、`context` import 可能變成未使用）、`recommended_sellers.go`、`marketplace.go`（**`enrichPostsWithSellerInfo` 定義在 :227，`post.go:54,97` 仍在用，要搬到 post.go**）、`product_view.go`、`governance.go` 的 4 個 handler 與只給它們用的 DTO（`toPublicViolationDTOs`、`toViolationReportDTO` 視 admin 是否共用）。
- grpc wrapper 變成沒有呼叫者：`seller_client.go` 的 `GetSellerStatsByStoreID`（:85）、`RecordStoreVisit`（:158）、`RecordProductView`（:172）、`GetRecommendedSellers`（:184）；`governance_client.go` 的 `CreateViolationReport`、`ListMyViolationReports`、`CreateFoodSafetyReport`、`GetPublicSellerViolations`（:18-44）。`GetSellerByStoreID`、`GetSellerBatch` 仍有呼叫者（resolver、`share.go:101,167`、`admin_seller.go:250`、`chat.go:147`、post 的 enrichment）。
- middleware：`middleware/ratelimit.go:106` `exemptExact` 的 `/api/v1/public/products/:id/view`；`ratelimit_test.go:31,79` 對應的斷言。`ratelimit_authfail_test.go:161,199,218,258` 在測試內自己註冊 `/api/v1/public/marketplace` 當樣本，不依賴真路由（可留，或換成仍存在的公開路由避免誤導）。
- 測試：`seller_test.go` 的 6 個 `TestGetSellerInfo_*`（留 `TestGetSellerProducts_*` 2 個）、`seller_response_shape_test.go` 的 `TestSellerShape_GetSellerInfo_Success/InvalidID`、`seller_public_pii_test.go`（整檔；設計書 65:8718 引用它）、`recommended_sellers_test.go`（整檔）、`marketplace_test.go`（整檔；其中 `TestGetMarketplace_EnrichPairsLogoKeyAndURL`、`TestGetMarketplace_DoesNotUseAccountIDAsMissingStoreIDFallback` 測的是 `enrichPostsWithSellerInfo`，覆蓋要移到 post 測試，`post_test.go:213` 已有部分）、`governance_test.go`（`toPublicViolationDTOs` 那段）。`testutil_test.go` 的 `mockSellerServiceClient` 實作整個 `pb.SellerServiceClient`，proto 的 RPC 還在就要留。
- `G/../docs/swagger.yaml`（swag 生成，先例每刀都重生：`/public/marketplace` :50584、`/public/sellers/recommended` :52355 等）。
- 快取：gateway 沒有針對這些路由的 Cache-Control 測試（`rg "public/sellers|marketplace" G/router/*cache*` 無命中）。
- `scripts/legacy-endpoints-baseline.json:95` `backend/gateway/internal/handler/marketplace_test.go`（company-domain 2）要隨刪檔移除（數量下降也會讓 `check-legacy-endpoints.py` 失敗，要 `--update`）。其他被刪的檔不在 baseline。`backend/scripts/gateway-json-contract-baseline.json` 沒有這些路由或 DTO 的列。
- 單體的 `internal/legacy/gatewayproxy`、`internal/httpapi` 沒有這些路由的特例（`rg` 無命中）。

### 1.6 `store_id` 轉址刪除的 backend 後果（容易漏）

`handler/share.go`：
- `getPostShareURL`（:93-104）：解析 store_code 失敗時退回 `store_id`，註解寫明依賴「網站 canonical redirect 導到 store_code 版」。轉址刪除後這條退路產生死連結。
- `getSellerShareURL`（:167-207）：`StoreCode` 空時退回 store_id（同樣的依賴）；而且對任何沒關的店都產生分享資料（店名、評分、logo），不看可曝光。

## 2. 單體的 Connect 實作

### 2.1 狀態

| RPC | 契約 | handler | 接線 | 授權 | 限流 | 快取 | 測試 |
|---|---|---|---|---|---|---|---|
| `SellerPublicService.GetStore` | `api/hawkr/seller/v1alpha1/seller.proto:123`（`NO_SIDE_EFFECTS`） | `H/seller/public.go:83` | `H/server.go:261,270` | `*PublicService` 命名規則＝匿名（`server.go:333` 的命名檢查） | `tierPublic` | GET → `public, no-cache`＋ETag、304 | `H/seller/public_test.go`（`TestGetStoreReadsByStoreCode…`、`…StatsDistinguishAFailedReadFromZero`、`…RejectsAMalformedCode…`、`…ReadInParallelEachUnderItsOwnBudget`）、`H/public_services_test.go:91`（快取）、`H/seller/public_integration_test.go:33-41`、模組 `M/seller/public_integration_test.go`（12 個） |
| `BatchGetStores` | `:131`（最多 100、刻意部分結果） | `public.go:113` | 同上 | 匿名 | `tierPublic` | 同上 | `public_test.go`（Maps…、ValidatesIDs、StatusValues）、integration :45 |
| `RecordProductView` | `:145`（POST、只計訪客、120/分/IP） | `public.go:147` | 同上 | 匿名；有 principal 時回成功不寫入 | `H/ratelimit.go:39` `tierProductView`（visitor 120、ssr 2400、ci 60），:61 | `private, no-store` | `public_test.go`（CountsOnlyGuests、RejectsABadID…）、`public_services_test.go:120-163`（POST、限流、額度低於 tierPublic）、integration :56,62 |
| `DiscoveryPublicService.SuggestRecommendedSellers` | `api/hawkr/discovery/v1alpha1/discovery.proto:18` | `H/discovery/public.go` | `server.go:256,260` | 匿名 | `tierPublic` | GET 快取 | `H/discovery/public_test.go`（8 個）、`public_services_test.go:96` |
| `SellerReportPublicService.SubmitFoodSafetyReport`、`SellerReportAppService.CreateReport`/`ListMyReports` | `report.proto:13-31` | — | `server.go:279` | 公開／登入 | `tierReportSubmit` | — | （8g，不在本盤點細查） |

去重：`RecordProductView` 刻意不去重（設計書 28 算總次數；`M/seller/CARRYOVER.md:224`，重新評估觸發：瀏覽數進排行或計費、或部署 CDN／bot management）。登入買家走 buyer `ProductViewed` → `cmd/hawkr/seller_events.go:20` `IngestProductView`（event_id 冪等）。

### 2.2 REST → Connect 欄位對照

**R2 `GET /public/sellers/:id`（`PublicSellerInfo`）→ `GetStore`（`PublicStore`＋`StoreStats`）**

| REST | Connect | 備註 |
|---|---|---|
| `storeId` | `store.id` | |
| `storeCode` | `store.store_code` | 也是請求鍵（REST 收 store_id） |
| `storeName`、`sellerName`（同值） | `store.store_name` | sellerName 是重複欄位 |
| `description`、`prize`、`websiteUrl` | 同名 | |
| `logoKey`＋`logoUrl`、`coverKey`＋`coverUrl` | `logo`、`cover_image`（`common.Image`） | |
| `city`、`district` | `town_code` | client 用 `hawkr_twaddr` / `@hawkr/twaddr` 解析 |
| `rating` | `stats.rating_average`（optional）＋`stats.review_count` | 讀取失敗不設，不是 0 |
| `seoTitle`、`seoDescription` | main：`seo_title`、`seo_description`；**local/10e：reserved，刪除** | §6 |
| `sellerType` | 無 | 沒有 client 讀（`rg sellerType buyer-app/lib/features/seller` 無命中） |
| `overallCertificationStatus` | 無 | B19 決定拿掉徽章（`merchant_screen.dart:265` 唯一讀者） |
| `foodBusinessRegNo` | 無（`PublicStore` 註解明寫「沒有食品業者登錄」） | REST 註解寫「法定應公開揭露」— 見 Q1 |
| `isProcessedFood` | 無 | 同上 |
| （寫入）`RecordStoreVisit` | 無 | 決定不搬（inventory P2 disposition b、CARRYOVER:225「來訪紀錄不搬」） |
| 對象範圍 | REST：任何沒關的店（含未公開）；Connect：只可曝光，其他一律 not_found | ADR 2026-10-03 |

**R3/R4 `stats`、`rating` → `GetStore.stats`**：`followers` → `follower_count`；`rating`、`reviewCount` → `rating_average`、`review_count`；`totalProducts/Orders/Sales` 一直是 0、無對應（不需要）；五星分佈（`fiveStarPct`…）REST 也沒填。無缺口。

**R1 `/recommended` → `SuggestRecommendedSellers`**：`storeId/storeCode/storeName/logo*/cover*` → `store.{id,store_code,name,logo,cover_image}`；`city/district` → `store.town_code`；`rating` → `store.rating_average`（＋`review_count`）；`followers` → `follower_count`（optional）；`description` 無（B3 決定卡片不帶簡介）；`certified` 刪除；`limit` → `max_results`（同為預設 10、上限 50）。另有 `store.status`（清單上恆為 OPEN）。無未決缺口。

**R5 `/marketplace`**：賣家區塊 → `SuggestRecommendedSellers`；貼文 → 仍是 REST `GET /public/posts`（social，A7，gateway 以 `GetSellerBatch` 補店名、logo、store_code）；熱門商品 → REST `GET /public/products?sort=sales_desc`（catalog）。client 組合（計畫明定）；backend 不需新 RPC。website 的 `features/home/server/home-tab-bootstrap-route.ts`、`lib/api/cached.ts` 是 BFF 端組合點（client 盤點範圍）。

**R6 `/view` → `RecordProductView`**：REST 收任意字串、永遠 200；Connect 對非數字 id 回 invalid_argument、超額回 resource_exhausted（RetryInfo）、寫入失敗回錯。client 要自己吞錯（fire-and-forget）。REST 是限流豁免；Connect 120/分/IP。

**R7–R10**：見 §1.2，Connect 等價物都已在（8g），沒有消費者，無資料對照需求。

## 3. legacy 去向（K24 格式）

| legacy RPC | 10d 後的呼叫者 | 10d 動作 | 等步驟 11 |
|---|---|---|---|
| `SellerService.GetRecommendedSellers` | 無（gateway 是唯一呼叫者；單體未實作） | 刪 gateway wrapper；刪 `rpc-calls.csv:572` | proto、seller-service 實作；`rpc-servers.csv:596`（已註 `api: discovery.RecommendedSellers…；對外在 A3b 步驟 8`，可補「gateway 呼叫者已刪（10d）」或不動） |
| `RecordProductView` | 無 | 刪 wrapper；刪 `rpc-calls.csv:585` | `rpc-servers.csv:594` |
| `RecordStoreVisit` | 無 | 刪 wrapper；刪 `rpc-calls.csv:586` | `rpc-servers.csv:592` |
| `GetSellerStats` | ai-service（`services/ai-service/internal/client/seller/adapter.go:37`，經單體 `sellerserver.GetSellerStats`） | 刪 gateway wrapper `GetSellerStatsByStoreID`；刪 `rpc-calls.csv:576` | 單體 legacy 實作留到 ai-service 搬遷；`rpc-servers.csv:553` 不動 |
| `CreateViolationReport`、`ListMyViolationReports`、`CreateFoodSafetyReport`、`GetPublicSellerViolations` | 無 | 刪 4 個 wrapper；刪 `rpc-calls.csv:562,580,559,571` | `rpc-servers.csv:598,599,600,604` |
| `GetSeller` | gateway（resolver、share、chat、admin、delegation）、account、im、social | 不動（`rpc-calls.csv:573` call_sites 4 可能要重算） | — |
| `GetSellerBatch` | gateway（post enrichment）、aftersales、buyer-service、im | 不動（`rpc-calls.csv:574`） | — |
| `SellerService.GetProductViewStats` | 無（10c 已處理） | — | `rpc-servers.csv:595`（K24） |

- 單體 `internal/legacy/sellerserver` 對上列 7 支本來就沒實作（`server.go` 只實作 GetSeller、ResolveSellerAccountID、GetCurrentStoreID、GetSellerBatch、BatchGetSellerMeta、ResolvePrincipals、ListStoreIDsByAccount、GetSellerStats、GetSellerShippingProfile、MatchCertificateHolder、ListPublicStore*ByCity、SearchSellerProspects ＋ 2 個 invite），所以 10d 不刪單體 legacy 程式碼。
- `internal/legacy/reviewservice.StoreRatingStats`、`buyer.SubscriberCounts` 仍由 Connect 的 `StatsSource` 使用，不受影響。
- Java 的 `services/java/shared/java-proto/.../seller.proto` 也宣告這些 RPC（只有宣告），步驟 11 一起處理。
- `rpc-calls.csv` 不記單體的呼叫者（10c 第 5 刀的駁回理由），判斷「沒有呼叫方」要另查程式碼。

## 4. 設計書

| 設計書 | 位置 | 內容 | 10d 動作建議 |
|---|---|---|---|
| 28 商品瀏覽統計 | :16、:58-66、:134-135、:151、:188、:207-209 | `POST /public/products/{id}/view` → gateway → seller-service | 步驟 11 整份改寫（計畫:299）；10d 至少把對外 API 指向 `SellerPublicService.RecordProductView`（10c 第 5 刀對 28 做過同類指向） |
| 53 Web 前端快取 | :312-316 | `getCachedSeller`／`getCachedSellerStats`／`getCachedRecommendedSellers` → REST 與 seller-service | 隨 website 改寫更新 |
| 45 首頁區塊 | :163 | `featured_sellers` → `getCachedRecommendedSellers` | 若 website 改名同步 |
| 02 產品 | :3019 §5.13c | `ListStoreIDsWithPublicProductsInternal` 的呼叫方寫 seller-service `GetRecommendedSellers`（餵兩條 REST） | 呼叫方已是 discovery 重算（`M/discovery/internal/ranking/recompute.go`），改指 discovery |
| 65 物流履約 | :8718 | 「全 repo `*_public_pii_test.go` 只有一支（`GET /public/sellers/{id}`）」 | 刪檔後這句失真 |
| 32 治理 | :176、:197、:201、:214、:273 | `app/violation_reports`、`public/food_safety_reports` 的 REST | 檔頭已標「§4 是舊 gateway REST，對外 API 以 Connect 重新設計」；local/10e 也在改 32 |
| 43 評價 | :108、:223、:301-302、:540 | `/public/sellers/{id}/reviews*` | 留著（A6） |
| 02 產品 | :374、:1039、:1047 | `/public/sellers/:id/products` | 留著（B） |
| 48 SEO | — | 賣家 SEO override 全鏈路 | local/10e 改（ADR 2026-10-09） |

## 5. 缺口與需要決定的事（不解）

- **Q1 食品業者登錄字號的公開揭露**：舊 DTO 註解寫 `foodBusinessRegNo`「法定應公開揭露」，`PublicStore` 刻意不帶（註解：「沒有…食品業者登錄」），`Store`（賣家）也把 `food_business_reg_no` reserved、改由 KYC `FoodBusiness` 承載。店面頁改讀 `GetStore` 後，前台不再有任何地方顯示登錄字號。要先確認法規要求（食安法網路販售揭露）與誰是 SSOT（ADR 2026-10-03 與 KYC ADR 都沒寫到前台揭露），再決定是否進 `PublicStore`。
- **Q2 合併順序與 SEO 欄位**：計畫:164 說「`PublicStore` 補 seo_*」，local/10e 的 ADR `2026-10-09-store-seo-platform-generated` 把它們 reserved、website 店面 metadata 改由 `lib/seo/store-summary.ts` 生成，並改了 `website/features/sellers/storefront/metadata.ts`、`lib/seo/metadata.ts`、`lib/api/index.ts`、`lib/query/keys.ts`。10d 的店面改寫正好在這幾個檔。要決定 10d 等 10e 合進 main 再開工，或在 10d 分支先合 local/10e。
- **Q3 留下的 `/public/sellers/:id/{products,reviews,reviews/stats,posts,forum}` 的鍵與可見性**：仍以連號 `store_id` 為鍵、不看可曝光（GetSeller 只濾 CLOSED），與 ADR 2026-10-03「公開讀取不接受 store_id、只回可曝光的店」的精神不一致；`resolveStorefrontSellerID` 的 store_code 分支在單體已失效（註解、swag、mock 測試過期）。選項：10d 只修註解與測試，可見性留到 catalog（B）、review（A6）、social（A7）搬遷；或 10d 在 resolver 加可曝光檢查（需要新的 legacy 讀取或改呼叫 `GetStore`）。
- **Q4 `share.go` 的 store_id 退路**：轉址刪除後 `getPostShareURL`、`getSellerShareURL` 的 store_id fallback 會產生死連結；`getSellerShareURL` 也不看可曝光。要決定改成失敗回錯、或照 ADR 對不可曝光的店不給分享資料。
- **Q5 討論區與貼文作者的顯示（client 打 `/public/sellers/{account/buyer id}`）**：buyer-app `merchant_chat_tab.dart:246`、`social_repository.dart:128`、`category_repository.dart:408`，seller-app `seller_repository.dart:60` `getUserProfileMap`（`page_echo_tab` 用）拿帳號或買家 ID 打 `/public/sellers/{id}` 取頭像，語意本來就錯（現在多半 404，靠 catch 回空）。`GetStore`／`BatchGetStores` 都不是「留言者的顯示資料」。後端沒有對應的 Connect 讀取（作者顯示應由 social 的回應帶）。要決定 10d 只刪呼叫（顯示預設頭像）還是在 social 的 REST 回應補作者顯示欄位（跨 A7）。
- **Q6 R7–R10 是否在 10d 刪**：10b D5 說留到 10d，但 10d 計畫文字沒列；它們綁 B6 的 UI 入口（website 匿名食安通報、店面與商品頁「檢舉商家」）。若 B6 的 UI 不在 10d 做，這 4 條零消費者路由仍可在 10d 照 B20「最後一個消費者已改完」直接刪。
- **G1 契約註解過期**：`discovery.proto:13-17` 的 service 註解說「回應裡的評價統計與排名來自同一份快照，不另查評價」，但 `RecommendedSeller` 註解與實作（`H/discovery/public.go:118` `RateCards`，10a 第 3 刀修訂）是 handler 批次讀 review。屬文件修正，不需決定。
- **G2 RecordProductView 錯誤語意變化**：REST 永遠 200；Connect 會回 invalid_argument／resource_exhausted／internal。client 必須 fire-and-forget；後端無需改。
- **G3 計畫文字與 brief 的路由清單不一致**：計畫:297 說刪「對應的 `/public/sellers/*`」，brief:190 列了 `/marketplace`（不在 `/public/sellers/*` 底下）。照 brief 刪 `/marketplace`，因為它的賣家區塊依賴已不存在的 RPC，貼文與商品已各有路由。

## 6. 與 local/10e 的重疊

`git diff --name-only main...local/10e` 共 447 個檔；merge-base `d700a7caa`（main 之後又有 3089784c3、cd9a170c0、af1b0dcc7）。10e 的 openapi 刪 16 個 admin op，與 10d 的 10 個 op 沒有交集。

同時會改的檔（衝突點）：
- `backend/api/openapi/openapi.yaml`、`G/router/openapi_coverage_test.go`（`maxRESTOperations`：10e 是 492，main 508）
- `G/handler/seller.go`（10e 刪 `PublicSellerInfo` 的 `SeoTitle`/`SeoDescription`；10d 刪整個 DTO）、`G/handler/seller_public_pii_test.go`（10e 改；10d 刪檔）
- `G/router/routes_governance.go`（10e 刪 admin suspensions；10d 刪 public／app 區塊）、`G/router/constants.go`
- `G/grpc/seller_client.go`、`G/grpc/governance_client.go`（兩邊都刪 wrapper）、`G/handler/testutil_test.go`
- `api/hawkr/seller/v1alpha1/seller.proto`（10e reserved `PublicStore` 的 seo_*，大改治理訊息）、`H/seller/public.go`（10e 刪 SEO 對應）、`internal/legacy/sellerserver/server.go`
- `scripts/legacy-endpoints-baseline.json`、`website/tests/e2e/helpers/mock-gateway.ts`
- website 店面：`features/sellers/storefront/metadata.ts`、`lib/seo/metadata.ts`、`lib/api/index.ts`、`lib/query/keys.ts`
- 文件：計畫 `2026-09-29-a3-buyer-seller.md`、設計書 32、48、ADR `2026-10-18-seller-governance-consolidated.md`
- 10e 不動 `rpc-*.csv`、設計書 28／53／02／65。

## 7. 指令

- 路由：`rg -n "public/sellers|/view\"|ProductsView|maxRESTOperations" backend --type go`；`sed` 讀 `routes_seller.go`、`routes_product.go`、`routes_governance.go`、`routes_marketplace.go`。
- openapi 計數：python `yaml.safe_load` 數 `paths` 的 get/post/put/patch/delete（main 508、merge-base 508、local/10e 492），set 差集得 10e 刪的 16 個。
- legacy server 實作清單：`rg -n "^func \(s \*Server\) [A-Z]" backend/internal/legacy/sellerserver`。
- CSV：`rg -n ",<RPC>," backend/docs/plans/2026-09-24-go-modular-monolith/rpc-*.csv`。
- 零消費者：`rg -n "violation_reports|food_safety_reports|violations/public" website buyer-app/lib seller-app/lib`。
- 設計書：`rg -n "public/sellers|public/marketplace|products/(\{id\}|:id)/view|RecordProductView|GetRecommendedSellers" backend/docs/詳細設計書`。
- 10e 重疊：`git diff --name-only main...local/10e`、`git diff main...local/10e -- <file>`。
