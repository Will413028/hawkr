# B4-0 呼叫端盤點：誰在用 product-service（含 category）（唯讀，快照 dev `496b048e2`）

標記：【量】＝跑過指令；【推】＝讀程式推論。路徑縮寫：`G=backend/gateway/internal`、`H=G/handler`、`R=G/router`、`M=backend/internal`、`J=backend/services/java`。「去向」欄全部留空。所有指令從 repo 根目錄 `/Users/will/second-brain/.projects/personal/hawkr` 執行。

product-service 對外有兩個 gRPC service：`ProductService`（`backend/api/proto/product/product.proto`，41 支 rpc）與 `CategoryService`（`backend/api/proto/category/category.proto`，7 支 rpc）；`J/shared/java-proto` 下的副本與 `api/` 版逐位元相同【量】（`cmp`）。

## 0. 產生數字的指令與命中數

| # | 指令 | 命中 | 說明 |
|---|---|---|---|
| P1 | `rg -n '^\s*rpc ' backend/api/proto/product/product.proto backend/api/proto/category/category.proto` | 41＋7 | proto 的 rpc 清單 |
| P2 | `rg -n 'h\.(Product\|Category)\.[A-Z]' backend/gateway/internal/handler --glob '!*_test.go'` | 60 行 | gateway handler 呼叫 product／category gRPC 的所有呼叫點（§1、§2 逐列） |
| P3 | `rg -n 'c\.client\.[A-Z]' backend/gateway/internal/grpc/product_client.go backend/gateway/internal/grpc/category_client.go backend/gateway/internal/grpc/delegation_client.go` | — | gateway 包裝函式 → 實際 rpc 的對照（§1 的「rpc」欄依此） |
| P4 | `rg -n 'GET\|POST\|PUT\|PATCH\|DELETE' backend/gateway/internal/router/routes_product.go` 加 `routes_search.go`、`routes_seller.go:21`、`routes_share.go:12` | 52 條 | 非代行的 product 路由（§1） |
| P5 | `rg -n 'products' backend/gateway/internal/router/routes_delegated.go` | 7 條 | 代行 product 路由（§2，供 A10） |
| P6 | `awk '/^  \/[^ ]*:$/{p=$1} /^    (get\|post\|put\|patch\|delete):/{m=$1} /operationId:/{print NR, m, p, $2}' backend/api/openapi/openapi.yaml \| rg -i 'product\|categor\|hashtag\|origins\|seller-cities\|shipping_quotes\|certific\|search\|share_url'` | 51 個 product 相關（另 5 個命中是非 product：`searchOrders`、2 個 procurement proxy、2 個 assistant proxy） | §3 |
| P7 | `rg -n 'ambientShippingTemperature\|isAmbientOrUntyped' backend/gateway/internal/handler --glob '!*_test.go'` | 8 行（1 定義＋1 函式＋6 呼叫點） | §1b |
| P8 | `rg -n 'productconnect\|categoryconnect\|productservice\.\|categoryservice\.' backend/internal backend/cmd --glob '*.go' --glob '!*_test.go' --glob '!**/gen/**'` | — | 單體端（§4） |
| P9 | `rg -n 'ProductServiceGrpc\|CategoryServiceGrpc\|"product"' backend/services/java --glob '*.java' --glob '!**/target/**' --glob '!**/test/**'` | 2 個呼叫端服務（cart、order） | §5 |
| P10 | `rg -il product backend/services/java/{aftersales,shipping,coupon}-service/src/main --glob '*.java'` | 有檔，但逐檔看過無 product gRPC | §5 |
| P11 | `rg -n -o -U 'apiClient\.(public\|app\|admin)\.(get\|post\|put\|patch\|delete)\w*(<[^>]*>)?\(\s*[\x60"][^\x60"]*' website/lib website/features website/app website/components \| rg '(products\|categories\|hashtags\|sellers/)'` 加 `rg -n 'fetcher\|authedFetch' website/lib/products/seller-repo.ts website/lib/search/repo.ts` | 見 §6.1 | website |
| P12 | `rg -n '/api/v1/(admin\|public\|app)/(products\|categories\|product_certification\|hashtags)' admin/src --glob '!*.test.*' \| rg -v 'router\.push\|href'` | 10 行（含 3 行註解） | admin |
| P13 | `rg -n "['\"](/api/v1)?/?(public\|app)/(products\|categories\|hashtags\|search\|sellers/\|share_url\|shipping_quotes)" buyer-app/lib seller-app/lib --glob '!*.g.dart'` | buyer 18、seller 20（呼叫行） | §6.3、§6.4 |
| P14 | `rg -n 'certification_applications\|certifications/verify\|/related\|/rating\|origins/summary\|seller-cities\|product_certification\|products/cache\|batch-change-category' website admin buyer-app/lib seller-app/lib --glob '!node_modules' --glob '!.next' --glob '!*.g.dart' --glob '!*.test.*' --glob '!**/tests/**' --glob '!*.md'` | **0 個呼叫** | 認證、相關商品、評分、產地／城市聚合、快取清除、批次改分類在四個 client 都沒有呼叫端 |

## 1. gateway REST 路由（非代行）

路由在 `R/routes_product.go`（`registerProductRoutes`），路徑常數 `R/constants.go:52-92`（Product／Category）、`:170`（`ShippingQuotesPath`）、`:173-197`（Admin Product／Certification）、`:226`（Search）、`:404`（`ShareURLPath`）。前綴：public `/api/v1/public`、app `/api/v1/app`、admin `/api/v1/admin`。`R/constants.go:468` 的 `ServiceProduct` 與 `G/proxy/reverse_proxy.go:145` 的 `"product"` 反代設定存在，但沒有任何路由用 `proxyToPath(ServiceProduct, …)`，全走 gRPC【量】（`rg -n 'ServiceProduct' backend/gateway --glob '!*_test.go'` 只命中定義）。

### 1a. 路由 → handler → rpc

| route | 路由行 | handler（定義；gRPC 呼叫行） | rpc | 標記 | 去向 |
|---|---|---|---|---|---|
| `GET /public/products` | `routes_product.go:21` | `H/product.go:257`；`:302` `GetPublicProductListFiltered` | GetProductList | 【量】 | |
| `GET /public/products/:id` | `:22` | `H/product.go:710`；`:730` `GetProduct`（登入）／`:732` `GetPublicProduct` | GetProduct／GetPublicProduct | 【量】 | |
| `GET /public/products/:id/brief` | `:23` | `H/product.go:808`；`:816` | GetPublicProduct | 【量】 | |
| `GET /public/products/:id/skus` | `:24` | `H/seller_sku.go:51`；`:65` | GetProduct | 【量】 | |
| `GET /public/products/:id/rating` | `:25` | `H/product.go:897`；`:905` | GetPublicProduct | 【量】 | |
| `GET /public/products/:id/related` | `:26` | `H/product.go:933`；`:941` | GetRelatedProducts | 【量】 | |
| `GET /public/hashtags` | `:34` | `H/hashtag.go:29`；`:47` | SearchHashtags | 【量】 | |
| `GET /public/hashtags/popular` | `:35` | `H/hashtag.go:88`；`:101` | GetPopularHashtags | 【量】 | |
| `GET /public/origins/summary` | `:39` | `H/origins.go:33`；`:45` | GetPublicOriginsSummary | 【量】 | |
| `GET /public/seller-cities/summary` | `:40` | `H/seller_city_summary.go:30`；`:31` | GetPublicSellerCitiesSummary | 【量】 | |
| `GET /public/categories` | `:46` | `H/category.go:56`；`:57` | GetCategoryTree | 【量】 | |
| `GET /public/categories/tree` | `:47` | `H/category.go:78`；`:79` | GetCategoryTree | 【量】 | |
| `GET /public/categories/:id` | `:48` | `H/category.go:132`；`:138` | GetCategoryTree（整棵樹再挑） | 【量】 | |
| `GET /public/categories/:id/path` | `:49` | `H/category.go:199`；`:205` | GetCategoryTree | 【量】 | |
| `GET /public/categories/:id/children` | `:50` | `H/category.go:164`；`:170` | GetChildCategories | 【量】 | |
| `GET /app/products`（`sellerHandle(CapProductManage)`，下同） | `:65` | `H/seller_product.go:349`；`:363` | GetProductList | 【量】 | |
| `POST /app/products` | `:66` | `H/seller_product.go:448`；`:535` | CreateProduct | 【量】 | |
| `GET /app/products/:id` | `:67` | `H/seller_product.go:706`；`:726` | GetProduct | 【量】 | |
| `PUT /app/products/:id` | `:68` | `H/seller_product.go:567`；`:675` | UpdateProduct | 【量】 | |
| `PUT /app/products/:id/status` | `:69` | `H/seller_product.go:809`；`:832` | UpdateSellerProductStatus | 【量】 | |
| `DELETE /app/products/:id` | `:70` | `H/seller_product.go:758`；`:775` | DeleteProduct | 【量】 | |
| `GET /app/products/:id/skus` | `:71` | `H/seller_sku.go:86`；`:103` | GetProduct | 【量】 | |
| `GET /app/products/certifications/verify` | `:72` | `H/product.go:1614`；`:1634` | VerifyCertification | 【量】 | |
| `GET /app/products/:id/certification_applications` | `:74` | `H/seller_product_certification.go:140`；`:150` | ListProductCertificationApplications | 【量】 | |
| `POST /app/products/:id/certification_applications` | `:75` | `H/seller_product_certification.go:193`；`:209` | SubmitProductCertification | 【量】 | |
| `POST /app/products/:id/certification_applications/:application_id/withdraw` | `:76` | `H/seller_product_certification.go:246`；`:267` | WithdrawProductCertification | 【量】 | |
| `POST /app/categories` | `:83` | `H/category.go:237`；`:248` | CreateCategory | 【量】 | |
| `GET /app/categories/tree` | `:84` | `H/category.go:102`；`:108` | GetCategoryTree | 【量】 | |
| `POST /app/shipping_quotes`（`sellerHandleAuthed`） | `:88` | `H/shipping_options.go:57`；`:86` | CalculateShippingOptions | 【量】 | |
| `GET /app/hashtags` | `:94` | `H/hashtag.go:29`（同 public） | SearchHashtags | 【量】 | |
| `GET /admin/products` | `:102` | `H/admin_product.go:38`；`:72` | GetProductList | 【量】 | |
| `DELETE /admin/products/cache` | `:103` | `H/product.go:1296`；`:1322` | PurgeProductCache | 【量】 | |
| `GET /admin/products/:id` | `:104` | `H/admin_product.go:130`；`:137` | GetProductForAdmin | 【量】 | |
| `PUT /admin/products/:id/status` | `:105` | `H/admin_product.go:166`；`:179` | UpdateProductStatus | 【量】 | |
| `GET /admin/product_certification_applications` | `:110` | `H/admin_product_certification.go:339`；`:346` | ListProductCertificationApplicationsForAdmin | 【量】 | |
| `GET /admin/product_certification_applications/:id` | `:111` | `…:384`；`:395` | GetProductCertificationApplicationForAdmin | 【量】 | |
| `GET …/:id/audit_logs` | `:112` | `…:417`；`:428` | ListProductCertificationAuditLogsForAdmin | 【量】 | |
| `POST …/:id/approve` | `:113` | `…:464`；`:466` | ApproveProductCertificationForAdmin | 【量】 | |
| `POST …/:id/reject` | `:114` | `…:486`；`:488` | RejectProductCertificationForAdmin | 【量】 | |
| `POST …/:id/revoke` | `:115` | `…:508`；`:510` | RevokeProductCertificationForAdmin | 【量】 | |
| `POST …/:id/reverify` | `:116` | `…:530`；`:547` | ReverifyProductCertificationForAdmin | 【量】 | |
| `GET /admin/categories/tree` | `:119` | `H/category.go:271`；`:277` | GetCategoryTree | 【量】 | |
| `POST /admin/categories` | `:120` | `H/category.go:299`；`:310` | CreateCategoryForAdmin | 【量】 | |
| `PATCH /admin/categories/:id` | `:121` | `H/category.go:334`；`:350` | UpdateCategoryForAdmin | 【量】 | |
| `DELETE /admin/categories` | `:122` | `H/category.go:373`；`:384` | BatchSoftDeleteCategoryForAdmin | 【量】 | |
| `POST /admin/categories/:id/restore` | `:123` | `H/category.go:407`；`:418` | RestoreCategoryForAdmin | 【量】 | |
| `GET /admin/categories/:id/products` | `:124` | `H/admin_category_products.go:24`；`:33` | ListProductsByCategoryForAdmin | 【量】 | |
| `POST /admin/products/batch-change-category` | `:125` | `H/admin_category_products.go:84`；`:96` | BatchChangeProductCategoryForAdmin | 【量】 | |
| `GET /public/search/products` | `routes_search.go:16` | `H/search.go:114`；`:124` | GetProductList（名稱叫 ViaES，實際打 product-service 列表） | 【量】 | |
| `GET /public/search/suggest` | `routes_search.go:17` | `H/search.go:178`；`:192` | GetProductList | 【量】 | |
| `GET /public/sellers/:id/products` | `routes_seller.go:21` | `H/seller.go:29`；`:42` | GetProductList | 【量】 | |
| `GET /app/share_url`（type=product 分支） | `routes_share.go:12` | `H/share.go:43` → `getProductShareURL` `:60`；`:61` | GetPublicProduct | 【量】 | |

gateway 包裝函式只存在、沒有 handler 呼叫：`GetProductList`、`GetPublicSellerProducts`、`ListProducts`、`GetProductDetail`、`ListSellerProducts`（非 Hashtags 版）、`SearchAdminProducts`（非 Hashtags 版）【推】（P2 的 60 行裡沒有它們）。

### 1b. 溫層補值與常溫過濾（`H/shipping_ambient.go`，檔頭註解寫「計畫 B4 刪除」）

定義：`H/shipping_ambient.go:5` `ambientShippingTemperature = "3201"`；`:8` `isAmbientOrUntyped`。

| 呼叫點 | 所在函式 | 作用 | 打到的服務 | 標記 | 去向 |
|---|---|---|---|---|---|
| `H/seller_product.go:485` | `CreateSellerProduct` | 建商品時每個配送方式補 `Temperature=3201` | product（CreateProduct） | 【量】 | |
| `H/seller_product.go:613` | `UpdateSellerProduct` | 同上（更新） | product（UpdateProduct） | 【量】 | |
| `H/admin_delegated_product.go:1100` | `shippingMethodProto`（代行建立／更新共用） | 同上；供 A10 | product | 【量】 | |
| `H/shipping_options.go:101` | `CalculateShippingOptions` | 回應只留常溫選項 | product（CalculateShippingOptions） | 【量】 | |
| `H/shipping.go:501` | `OnboardCvsShipping` | 超商建店補 `GoodsType=3201` | shipping-service（非 product） | 【量】 | |
| `H/shipping.go:598` | `ListShippingOnboardings` | 只列常溫／未分溫層 | shipping-service | 【量】 | |
| `H/shipping.go:750` | `ListSellerShippingMethods` | 只列常溫／未分溫層 | shipping-service | 【量】 | |

## 2. 代行 product 路由（供 A10，只盤點）

路由在 `R/routes_delegated.go:95-101`（`sellers := "/sellers/:store_id"`，`:85`），前綴 `/api/v1/admin`，以 `delegatedHandle(…, authz.AdminCapProducts*)` 註冊；7 條都**不在 openapi.yaml**，`R/openapi_coverage_test.go:46-54` 以 `notAnnotatedDelegated` 豁免【量】。gRPC 包裝在 `G/grpc/delegation_client.go:217-282`。`H/admin_delegated_common.go` 與 `H/delegation/` 不直接呼叫 product gRPC；`H/delegation/orchestrator.go:340,355` 把資源型別 `"product"` 對到 capability，`H/delegation/diff.go:76-133` 是 product 欄位的差異投影【量】。

| route | 路由行 | handler（定義；gRPC 呼叫行） | rpc | 標記 | 去向 |
|---|---|---|---|---|---|
| `GET /admin/sellers/:store_id/products`（products.read） | `:95` | `H/admin_delegated_product.go:249`；`:268` | GetProductList | 【量】 | |
| `POST …/products`（products.write） | `:96` | `:326`；`:394` | CreateProduct | 【量】 | |
| `POST …/products/batch-unpublish`（products.publish） | `:97` | `:688`；`:748` BatchGetSellerProductsForDelegation、`:779` | BatchGetSellerProductsForDelegation＋BatchUpdateSellerProductStatus | 【量】 | |
| `GET …/products/:product_id`（products.read） | `:98` | `:308` → `readProduct` `:840`；`:844` | GetProduct | 【量】 | |
| `PATCH …/products/:product_id`（products.write） | `:99` | `:442`；`:588` | UpdateProduct（先經 `readProduct` 讀前像） | 【量】 | |
| `POST …/products/:product_id/publish` | `:100` | `:607` → `changeDelegatedProductStatus` `:616`；`:663` | UpdateSellerProductStatus | 【量】 | |
| `POST …/products/:product_id/unpublish` | `:101` | `:612` → 同上 | UpdateSellerProductStatus | 【量】 | |

## 3. OpenAPI 操作（`backend/api/openapi/openapi.yaml`，51 個）

行號為 `operationId` 所在行。

| operationId | method path | 行 | 去向 |
|---|---|---|---|
| batchSoftDeleteCategoryAdmin | DELETE /api/v1/admin/categories | 810 | |
| createCategoryAdmin | POST /api/v1/admin/categories | 856 | |
| getAdminCategoryTree | GET /api/v1/admin/categories/tree | 892 | |
| updateCategoryAdmin | PATCH /api/v1/admin/categories/{id} | 936 | |
| listProductsByCategory | GET /api/v1/admin/categories/{id}/products | 985 | |
| restoreCategoryAdmin | POST /api/v1/admin/categories/{id}/restore | 1047 | |
| listAdminProductCertificationApplications | GET /api/v1/admin/product_certification_applications | 5984 | |
| getAdminProductCertificationApplication | GET …/{id} | 6083 | |
| approveAdminProductCertification | POST …/{id}/approve | 6136 | |
| listAdminProductCertificationAuditLogs | GET …/{id}/audit_logs | 6209 | |
| rejectAdminProductCertification | POST …/{id}/reject | 6283 | |
| reverifyAdminProductCertification | POST …/{id}/reverify | 6356 | |
| revokeAdminProductCertification | POST …/{id}/revoke | 6429 | |
| searchAdminProducts | GET /api/v1/admin/products | 6502 | |
| batchChangeProductCategory | POST /api/v1/admin/products/batch-change-category | 6584 | |
| purgeProductCache | DELETE /api/v1/admin/products/cache | 6629 | |
| getAdminProductDetail | GET /api/v1/admin/products/{id} | 6677 | |
| updateAdminProductStatus | PUT /api/v1/admin/products/{id}/status | 6730 | |
| createCategory | POST /api/v1/app/categories | 10320 | |
| getSellerCategoryTree | GET /api/v1/app/categories/tree | 10368 | |
| listSellerProducts | GET /api/v1/app/products | 13824 | |
| createSellerProduct | POST /api/v1/app/products | 13895 | |
| verifyCertification | GET /api/v1/app/products/certifications/verify | 13943 | |
| deleteSellerProduct | DELETE /api/v1/app/products/{id} | 14009 | |
| getSellerProductDetail | GET /api/v1/app/products/{id} | 14059 | |
| updateSellerProduct | PUT /api/v1/app/products/{id} | 14111 | |
| listSellerProductCertificationApplications | GET /api/v1/app/products/{id}/certification_applications | 14172 | |
| submitSellerProductCertification | POST 同上 | 14256 | |
| withdrawSellerProductCertification | POST …/{application_id}/withdraw | 14329 | |
| getSellerProductSkus | GET /api/v1/app/products/{id}/skus | 14408 | |
| updateSellerProductStatus | PUT /api/v1/app/products/{id}/status | 14461 | |
| getShareURL | GET /api/v1/app/share_url（product 只是其中一種 type） | 17428 | |
| calculateShippingOptions | POST /api/v1/app/shipping_quotes | 18539 | |
| getCategoryList | GET /api/v1/public/categories | 19158 | |
| getCategoryTree | GET /api/v1/public/categories/tree | 19191 | |
| getCategoryByID | GET /api/v1/public/categories/{id} | 19218 | |
| getCategoryChildren | GET /api/v1/public/categories/{id}/children | 19262 | |
| getCategoryPath | GET /api/v1/public/categories/{id}/path | 19302 | |
| searchHashtags | GET /api/v1/public/hashtags | 19342 | |
| getPopularHashtags | GET /api/v1/public/hashtags/popular | 19383 | |
| getOriginsSummary | GET /api/v1/public/origins/summary | 19426 | |
| getProductList | GET /api/v1/public/products | 19468 | |
| getProductDetailByID | GET /api/v1/public/products/{id} | 19602 | |
| getProductInfoByID | GET /api/v1/public/products/{id}/brief | 19646 | |
| getProductRating | GET /api/v1/public/products/{id}/rating | 19690 | |
| getRelatedProducts | GET /api/v1/public/products/{id}/related | 19734 | |
| listSellerSkus | GET /api/v1/public/products/{id}/skus | 19774 | |
| searchProductsViaES | GET /api/v1/public/search/products | 19855 | |
| getSearchSuggestions | GET /api/v1/public/search/suggest | 19916 | |
| getSellerCitiesSummary | GET /api/v1/public/seller-cities/summary | 19962 | |
| getSellerProducts | GET /api/v1/public/sellers/{id}/products | 19993 | |

沒有 operation 的 product 路由：`GET /api/v1/app/hashtags`（`R/openapi_coverage_test.go:63` `notAnnotated`）與 §2 的 7 條代行路由【量】。

## 4. 單體已在呼叫 product-service 的地方

legacy client：`M/legacy/productservice/`（`productconnect.ProductServiceClient`，`productservice.go:37`）與 `M/legacy/categoryservice/categoryservice.go:43`（`categoryconnect`，同一個 `productAddr`）。

| 單體 client 方法 | rpc（呼叫行） | 經過的 port／adapter | 使用者 | 標記 | 去向 |
|---|---|---|---|---|---|
| `productservice.Cards`（`productservice.go:41`） | GetProductsByIDs（`:51`） | `buyerProducts`（`backend/cmd/hawkr/buyer_ports.go:11-13`）→ `M/httpapi/buyer/ports.go:45` `Products.Cards` | buyer httpapi（收藏、瀏覽紀錄等卡片） | 【量】 | |
| 同上 | 同上 | `socialCards`（`cmd/hawkr/social.go:46-48`）→ `M/httpapi/social/public.go:76` `Products.Cards` | social 公開貼文的商品卡 | 【量】 | |
| 同上 | 同上 | `socialProducts.VerifyProduct`（`cmd/hawkr/social.go:28-30`）→ `M/modules/social/internal/post/post.go:42` `ProductVerifier` | social 發文時驗證商品屬於該店 | 【量】 | |
| 同上 | 同上 | `sellerProducts.StoreIDs`（`cmd/hawkr/seller_events.go:39-41`）→ `M/modules/seller/internal/views/views.go:26` `ProductStores` | seller 瀏覽數歸店 | 【量】 | |
| 同上 | 同上 | 直接傳 `*productservice.Client` → `M/legacy/buyerrelay/relay.go:102-103` `Products.Cards`（`cmd/hawkr/main.go:513`） | buyerrelay worker | 【量】 | |
| `productservice.StoresWithPublicProducts`（`:71`） | ListStoreIDsWithPublicProductsInternal（`:74`） | 直接傳 client → `M/modules/discovery/internal/ranking/ports.go:21` `Products`（`cmd/hawkr/discovery.go:21`） | discovery 排名重算 | 【量】 | |
| `productservice.SearchProducts`（`search.go:39`） | GetProductList（`search.go:59`） | `cmd/hawkr/procurement.go:44-52` adapter | procurement worker 候選搜尋 | 【量】 | |
| `categoryservice.CategoryTree`（`categoryservice.go:47`） | GetCategoryTree（`:50`） | `taxonomy`（`cmd/hawkr/procurement.go:60-70`）→ `M/modules/procurement/internal/resolver/resolver.go:32` `TaxonomyReader` | procurement 分類解析 | 【量】 | |

建立 client 的地方：`cmd/hawkr/main.go:195`、`:415`、`:420`（category）、`cmd/hawkr/seller_commands.go:77`【量】。`M/httpapi` 沒有任何 handler 直接呼叫 product rpc，都經上表的 port【量】（P8）。`api/hawkr/` 下沒有 catalog／product 的 Connect 服務【量】（`rg -il 'product\|catalog' api --glob '*.proto'` 只命中其他領域的欄位）。

## 5. 其他 Java 服務呼叫 product-service gRPC

| 服務 | client 方法（定義） | rpc | 實際呼叫點 | 標記 | 去向 |
|---|---|---|---|---|---|
| cart | `J/cart-service/.../cart/grpc/client/ProductGrpcClient.java:61` `getProductSimple` | GetProductSimple（`:70`） | `J/cart-service/.../cart/service/CartService.java:120` | 【量】 | |
| cart | 同檔 `:86` 起 `batchGetSkuCartInfo` | BatchGetSkuCartInfo（`:96`） | `CartService.java:297` | 【量】 | |
| order | `J/order-service/.../order/grpc/client/ProductGrpcClient.java:76` `deductStock` | DeductStock（`:84`） | `order/service/OrderService.java:215`、`:1174` | 【量】 | |
| order | 同檔 `:92` `restoreStock` | RestoreStock（`:100`） | `OrderService.java:419`；`order/worker/StockCompensationWorker.java:146`、`:206` | 【量】 | |
| order | 同檔 `:149` `batchGetProductForCheckout` | BatchGetProductForCheckout（`:157`） | `order/checkout/CheckoutService.java:1126` | 【量】 | |
| order | 同檔 `:108` `getStock` | GetStock（`:114`） | 無呼叫端 | 【量】 | |
| order | 同檔 `:124` `getPublicProduct` | GetPublicProduct | 無呼叫端 | 【量】 | |

aftersales、shipping、coupon、payment、media 沒有 product gRPC client【量】（P9 只命中 cart、order；P10 命中的檔是 enum、MQ payload 或欄位名）。

rpc 覆蓋：41＋7 支裡，除 order 的 `GetStock`（只有死包裝）外每支都至少有一個呼叫端【推】（合併 §1、§2、§4、§5）。

## 6. client 呼叫點

### 6.1 website（Next.js；`apiClient.public.*` 的路徑不帶 `/public` 前綴，`authedFetch` 帶 `/app`）

| 呼叫點 | 端點 | 標記 | 去向 |
|---|---|---|---|
| `website/lib/api/index.ts:182`、`:193` | GET /public/products | 【量】 | |
| `website/lib/api/index.ts:243` | GET /public/sellers/{id}/products | 【量】 | |
| `website/lib/api/index.ts:277` | GET /public/products/{id} | 【量】 | |
| `website/lib/api/index.ts:298`；`website/lib/products/seller-repo.ts:774` | GET /public/categories/tree | 【量】 | |
| `website/lib/cart/resolve.ts:55` | GET /public/products/{id} | 【量】 | |
| `website/lib/search/repo.ts:137` | GET /public/search/products | 【量】 | |
| `website/lib/search/repo.ts:153` | GET /public/search/suggest | 【量】 | |
| `website/lib/search/repo.ts:234-235` | GET /public/hashtags/popular | 【量】 | |
| `website/lib/products/seller-repo.ts:357`（root 預設 `/app/products`，`:342`） | GET /app/products | 【量】 | |
| `website/lib/products/seller-repo.ts:365` | GET /app/products/{id} | 【量】 | |
| `website/lib/products/seller-repo.ts:378` | GET /app/products/{id}/skus | 【量】 | |
| `website/lib/products/seller-repo.ts:390` | POST /app/products | 【量】 | |
| `website/lib/products/seller-repo.ts:404` | PUT /app/products/{id} | 【推】（method 在 init） | |
| `website/lib/products/seller-repo.ts:417` | PUT /app/products/{id}/status | 【推】 | |
| `website/lib/products/seller-repo.ts:428` | DELETE /app/products/{id} | 【量】 | |
| `website/lib/products/seller-repo.ts:759` | GET /app/categories/tree | 【量】 | |
| `website/lib/products/seller-repo.ts:792` | POST /app/categories | 【量】 | |
| `website/lib/products/seller-repo.ts:927-928` | POST /app/shipping_quotes | 【量】 | |
| `website/app/api/public/search/products/route.ts:65`、`:101`；`.../suggest/route.ts:18` | 同源 BFF，轉呼叫上面的 search／products repo | 【量】 | |
| `website/features/home/components/useHomeSearchQueries.ts:52`、`:71` | 打上一列的同源 BFF | 【量】 | |

代行（供 A10）：`website/app/(delegated)/adm/api/products/[storeId]/route.ts:25,38`、`[productId]/route.ts:22,34`、`publish/route.ts:21`、`unpublish/route.ts:18`、`batch-unpublish/route.ts:26` 轉呼叫 §2；瀏覽器端 `website/lib/seller-data/delegated-repos.ts:294,340,380,405`（root `:279`）與 `website/features/delegated-operations/products/repo.ts:273,296`【量】。

### 6.2 admin

| 呼叫點 | 端點 | 標記 | 去向 |
|---|---|---|---|
| `admin/src/app/admin/products/page.tsx:72` | GET /admin/products | 【量】 | |
| `admin/src/app/admin/products/page.tsx:104` | PUT /admin/products/{id}/status | 【量】 | |
| `admin/src/services/products.ts:13` | GET /admin/products/{id} | 【量】 | |
| `admin/src/app/admin/page.tsx:45`；`admin/src/app/admin/analytics/page.tsx:66` | GET /admin/products（只取 total） | 【量】 | |
| `admin/src/app/admin/merchants/[id]/page.tsx:280`、`:328` | GET /admin/products?owner_id= | 【量】 | |
| `admin/src/app/admin/merchants/[id]/page.tsx:360` | PUT /admin/products/{id}/status | 【量】 | |
| `admin/src/app/admin/category-audit/page.tsx:62` | GET /admin/categories/tree | 【量】 | |
| `admin/src/app/admin/category-audit/page.tsx:70` | **PATCH /admin/categories**（body `{categoryIds, status}`）；gateway 只有 `PATCH /admin/categories/:id`（`routes_product.go:121`），此路徑沒有對應路由 | 【推】會 404 | |

`admin/src/app/admin/products/page.tsx:18` 的註解提到 `DELETE /admin/products/cache`，但沒有呼叫碼【量】。

### 6.3 buyer-app（Dio，baseUrl 已含 `/api/v1`）

| 呼叫點 | 端點 | 標記 | 去向 |
|---|---|---|---|
| `buyer-app/lib/features/product/data/product_repository.dart:31`；`home/data/home_product_repository.dart:30`；`category/data/category_repository.dart:97`；`product/presentation/search_screen.dart:199` | GET /public/products | 【量】 | |
| `product_repository.dart:56`、`:160` | GET /public/products/{id} | 【量】 | |
| `product_repository.dart:144` | GET /public/products/{id}/brief | 【量】 | |
| `product_repository.dart:73`；`seller/presentation/merchant_screen.dart:229`；`seller/presentation/widgets/merchant_shop_tab.dart:193` | GET /public/sellers/{id}/products | 【量】 | |
| `product_repository.dart:174` | GET /public/categories/{id}/path | 【量】 | |
| `category_repository.dart:56` | GET /public/categories/tree | 【量】 | |
| `category_repository.dart:129` | GET /public/hashtags/popular | 【量】 | |
| `category_repository.dart:240`；`search_screen.dart:273` | GET /public/search/products | 【量】 | |
| `category_repository.dart:282`；`product_repository.dart:189` | GET /public/search/suggest | 【量】 | |
| `product_repository.dart:292` | GET /app/share_url | 【量】 | |

### 6.4 seller-app（路徑帶完整 `/api/v1`）

| 呼叫點（`seller-app/lib/features/product/data/`） | 端點 | 標記 | 去向 |
|---|---|---|---|
| `product_repository.dart:24` | GET /app/products | 【量】 | |
| `product_repository.dart:60`；`product_upload_service.dart:276` | POST /app/products | 【推】（method 由包裝決定） | |
| `product_repository.dart:99`；`product_upload_service.dart:325` | PUT /app/products/{id} | 【推】 | |
| `product_repository.dart:114` | DELETE /app/products/{id} | 【量】 | |
| `product_repository.dart:122` | PUT /app/products/{id}/status | 【推】 | |
| `product_repository.dart:130`、`:240` | GET /app/products/{id}/skus | 【量】 | |
| `product_repository.dart:188` | GET /app/products/{id} | 【量】 | |
| `product_repository.dart:161` | POST /app/shipping_quotes | 【量】 | |
| `product_repository.dart:196`、`:205` | GET /app/categories/tree | 【量】 | |
| `product_repository.dart:214`、`:218`、`:222` | GET /public/categories/{id}/path | 【量】 | |
| `product_repository.dart:231` | POST /app/categories | 【量】 | |
| `hashtag_repository.dart:35` | GET /public/hashtags（不是 `/app/hashtags`） | 【量】 | |

### 6.5 沒有 client 呼叫的 gateway 路由（合併 §6.1–6.4 與 P14）

【量】四個 client 都沒有呼叫：商品認證 11 條（seller 申請 3＋verify 1＋admin 7）、`/public/products/{id}/related`、`/public/products/{id}/rating`、`/public/products/{id}/skus`、`/public/origins/summary`、`/public/seller-cities/summary`、`/app/hashtags`（seller-app 改打 `/public/hashtags`）、`/public/categories`、`/public/categories/{id}`、`/public/categories/{id}/children`、`DELETE /admin/products/cache`、admin 分類的 create／delete／restore／`{id}/products`、`batch-change-category`、`PATCH /admin/categories/{id}`（admin 打的是無 id 版）。website 是否經 Server Component 以其他包裝呼叫上述路徑【推】未逐檔排除，以 P11、P14 為準。
