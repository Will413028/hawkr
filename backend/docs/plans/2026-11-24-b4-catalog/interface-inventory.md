# B4-0 介面盤點：Java product-service（唯讀，快照 dev `496b048e2`）

標記：【量】＝跑過指令或打開檔案逐行看過；【推】＝讀方法名、檔頭或片段推論，沒逐分支讀。路徑縮寫：`B=backend`、`J=backend/services/java/product-service/src/main/java/io/github/will413028/hawkr/product`、`R=backend/services/java/product-service/src/main/resources`、`P=backend/api/proto/product/product.proto`、`C=backend/api/proto/category/category.proto`、`G=backend/gateway/internal`、`MQ=backend/services/java/shared/common-core/src/main/java/io/github/will413028/hawkr/common/mq/MqConstants.java`、`CSV=backend/docs/plans/2026-09-24-go-modular-monolith`。
「去向」欄全部留空（本檔只盤點，不決定）。「裁切」欄標出屬於範圍裁切、改寫時不帶的項目（溫層、產銷履歷、有機標章、商品認證；依據 `B/docs/plans/2026-09-24-go-modular-monolith.md:368-379` 的 B4 延後項）：**刪**＝整項屬裁切；**部分**＝項目保留但其中的溫層或認證欄位／分支屬裁切；空白＝不屬裁切。產地（`origin`）不是產銷履歷，不標裁切。
所有指令從 repo 根目錄 `/Users/will/second-brain/.projects/personal/hawkr` 執行。`P` 與 `backend/services/java/shared/java-proto/src/main/proto/product/product.proto` 去空白後逐行相同【量】（`diff <(sed 's/[[:space:]]//g' …)` 無輸出）。

## 0. 數字與指令

| 項目 | 量到 | 指令 |
|---|---|---|
| gRPC RPC | **46**：`ProductService` 39＋`CategoryService` 7；其中屬裁切 **11**（`VerifyCertification` 與 10 支認證申請／審核），部分屬裁切 3（`CreateProduct`、`UpdateProduct`、`CalculateShippingOptions`，另 `GetProductList` 的篩選欄位） | `rg -c '^\s*rpc ' backend/api/proto/product/product.proto`＝39、`… category/category.proto`＝7【量】 |
| main Java 檔／行數 | 123 檔／21,724 行；`ProductService.java` 3,015 行、`ProductGrpcService.java` 2,941 行、`ProductCertificationApplicationService.java` 1,549 行、`GovCertificationService.java` 912 行 | `find backend/services/java/product-service/src/main -name '*.java' \| wc -l`、`… \| xargs cat \| wc -l`、`wc -l`【量】 |
| MQ consumer | **8 個 `@RabbitListener` 方法、7 個類別**（`FavoriteCountConsumer` 有 2 個）；屬裁切 1（`CertificationVerifyConsumer`） | `rg -n '@RabbitListener' backend/services/java/product-service/src/main`【量】 |
| MQ publisher | **5 個發布方法**（restock 1、認證 4）；屬裁切 4。發布走 `obsRabbitPublisher.publish`（`J/mq/ProductEventPublisher.java:117`），不是 `convertAndSend` | `rg -n 'public void publish\w*' J/mq/ProductEventPublisher.java`【量】 |
| `@Scheduled` | **2**（`IdempotencyCleanupJob`、`FavoriteEventDedupCleanupJob`） | `rg -n '@Scheduled' backend/services/java/product-service/src/main`【量】 |
| gRPC outbound | **4 個服務、12 支 RPC**：seller 4（`BatchGetSellerMeta`、`MatchCertificateHolder`、`ListPublicStoreIDsByCity`、`ListPublicStoresByCity`）、buyer 2、shipping 3、media 4（`FinalizeImages`、`LinkImages`、`UnlinkImages`、`ReleaseReferences`）；另有政府認證查驗 HTTP（屬裁切） | `rg -n '\.(\w+)\(request\)\|stub\.\w+\(' J/grpc/client`、逐檔讀 `J/grpc/client/*.java` 的 public 方法【量】 |
| DB 表（現存） | **12**：`category`、`product`、`product_sku`、`idempotency_request`、`stock_transaction`、`hashtag`、`product_hashtag`、`favorite_count_event_dedup`、`sales_count_event_dedup`、`product_certification_application`、`product_certification_review_event`、`store_exposure`；已刪 1（`product_suspension_snapshot`，V10 建、V20 刪）。屬裁切 2 表＋2 欄 | `rg -n 'CREATE TABLE\|DROP TABLE' R/db/migration`【量】 |
| Flyway migration | 22 支（V1–V22） | `ls R/db/migration \| wc -l`【量】 |
| Redis key 群 | **7**：`prod:detail:*`、`prod:list:*`、`prod:origins:summary:*`、`cat:all`；屬裁切 3（`gov:certv2:*`、`gov:cert:circuit:*`、`cert:reverify:lease:*`） | `rg -n '"[a-z_]+:[a-z_:]*' J \| rg -i 'prefix\|key\|cache'`【量】 |
| 呼叫 product-service 的 client | gateway（`G/grpc/product_client.go`、`category_client.go`、`delegation_client.go`）、單體 `B/internal/legacy/productservice`（3 支 RPC）與 `B/internal/legacy/categoryservice`（1 支）、Java cart-service（2 支）、Java order-service（3 支有呼叫端＋2 支死方法）、`B/services/ai-service`（4 支） | 每支 RPC 名逐一 `rg -l "\.<Rpc>\(" --glob '*.go' --glob '!**/gen/**' --glob '!*_test.go'` 與 Java `rg -n '\.<rpc>\('`【量】；與 `CSV/rpc-calls.csv` 的 product-service 列相符（抽查 38-41、55、60-61、487-501）【量】 |

## 1. gRPC server

共同行為：handler 以 envelope（`code`／`message`）回業務失敗【推】（抽查 `calculateShippingOptions`，`J/grpc/ProductGrpcService.java:1341-1400`）。角色檢查在 handler 內呼叫 `GrpcAuthContext`；表中「授權」欄是 `awk` 掃 `requireAccountType|requireSellerOrDelegated` 的結果【量】，「無」代表 handler 內沒有角色檢查（只靠 gateway 與內部 interceptor）。

### 1.1 `ProductService`：賣家寫入與讀取（`J/grpc/ProductGrpcService.java`）

| RPC | file:line（handler／proto） | 授權 | 行為摘要 | 呼叫端 | 裁切 | 去向 |
|---|---|---|---|---|---|---|
| `CreateProduct` | `:128`／`P:15` | `requireSellerOrDelegated(PRODUCTS_WRITE)`（`:129`）【量】 | 必填 idempotency key（`IdempotencyService`，`idempotency_request`）；建 product＋SKU、hashtag、media finalize/link、分類反正規化、`searchable_text`；SKU 的 `shipping_method` 帶溫層、product 帶 `certifications`【推】 | `G/handler/seller_product.go`、`G/grpc/delegation_client.go`（代操作）【量】 | 部分（溫層、認證欄位；gateway 的 `shipping_ambient.go` 補常溫） | |
| `UpdateProduct` | `:240`／`P:18` | 同上（`:241`）【量】 | 樂觀鎖 `version`（V16/V17）；`update_certifications` 旗標全量取代認證（`P:322`）；SKU 變動回寫 `stock_qty`、價格區間；補貨時發 `product.restock`（`J/service/ProductService.java:243`）【推】 | 同上【量】 | 部分 | |
| `GetProduct` | `:340`／`P:21` | 無；以 request 的 `store_id` 判斷擁有者（`:342`）【量】 | 賣家／內部詳情，含未上架【推】 | `G/handler/product.go`、`seller_product.go`、`seller_sku.go`、`delegation_client.go`、ai-service `client/product/adapter.go`【量】 | 部分（回應的認證與溫層欄位） | |
| `GetPublicProduct` | `:417`／`P:24` | 無【量】 | 買家詳情；`ProductPublicVisibilityPolicy` ＋ `store_exposure` 判斷 `TEMPORARILY_UNAVAILABLE`（見 IT `publicDetailReflectsExposure`）；Redis `prod:detail:{id}:{onboarding_sig}`【推】 | `G/handler/product.go`、`share.go`；order-service `ProductGrpcClient.java:133`（**無呼叫端**）【量】 | 部分 | |
| `GetProductList` | `:464`／`P:27` | 無【量】 | 公開／賣家／admin 列表；公開視角以 `store_exposure` 過濾；篩選含分類、價格、產地、賣家城市（呼叫 seller `ListPublicStoreIDsByCity`，`ProductService.java:1112`）、已收藏／已訂閱（呼叫 buyer，`:1128`、`:1154`）、`delivery_temp_filter`（`P:468`）、`certification_types`（`P:478`）；Redis `prod:list:*`【推】 | `G/grpc/product_client.go`、`delegation_client.go`、單體 `B/internal/legacy/productservice/search.go:39`（procurement 用）、ai-service `adapter.go`、`usecase/recommend_usecase.go`【量】 | 部分（溫層與認證篩選、卡片的 `delivery_temp`／`certifications`，`P:623-631`） | |
| `GetProductForAdmin` | `:531`／`P:30` | ADMIN（`:535`）【量】 | admin 詳情【推】 | `G/handler/admin_product.go`【量】 | 部分 | |
| `UpdateProductStatus` | `:585`／`P:33` | ADMIN（`:589`）【量】 | admin 改狀態（強制下架）【推】；B4 計畫要求改成 seller 的商品限制措施（`go-modular-monolith.md` B4「商品限制措施」） | `G/handler/admin_product.go`【量】 | | |
| `UpdateSellerProductStatus` | `:655`／`P:36` | `requireSellerOrDelegated(PRODUCTS_PUBLISH)`（`:656`）【量】 | 賣家上下架；首次上架寫 `first_published_at`（V15）【推】 | `G/handler/seller_product.go`、`delegation_client.go`【量】 | | |
| `BatchGetSellerProductsForDelegation` | `:742`／`P:48` | `requireSellerOrDelegated(PRODUCTS_READ)`（`:743`）【量】 | admin 代操作批次讀取【推】 | `G/handler/admin_delegated_product.go`（經 `delegation_client.go`）【量】 | | |
| `BatchUpdateSellerProductStatus` | `:813`／`P:40` | 代操作時 `DelegationContextValidator.validateBatch`；否則 `requireAccountType(SELLER)`（`:816-822`）【量】 | 批次上下架【推】 | 只有 `G/grpc/delegation_client.go:282`（admin 代操作，`admin_delegated_product.go:779`）；賣家自己的路徑無 client【量】 | | |
| `DeleteProduct` | `:1985`／`P:86` | SELLER（`:1988`）【量】 | 軟刪；連帶撤回認證申請（測試 `ProductCertificationWithdrawOnDeleteTest`）、release media references【推】 | `G/handler/seller_product.go`【量】 | 部分（撤回認證分支） | |
| `CalculateShippingOptions` | `:1341`／`P:91` | SELLER（`:1345`）【量】 | 轉呼叫 shipping `CalculateShippingOptions`（`:1395`），以 `temperature_groups` 回傳；寄件地區永遠由空字串郵遞區號解析（見 defects CD6）【量】 | `G/handler/shipping_options.go`【量】 | 部分（回應以溫層分組，`P:849-867`） | |
| `PurgeProductCache` | `:965`／`P:52` | ADMIN（`:968`）【量】 | 依 pattern 清 `prod:detail:*`／`prod:list:*`（`:988-1026`）【量】 | `G/handler/product.go`【量】 | | |
| `GetRelatedProducts` | `:1899`／`P:81` | 無【量】 | 相關商品【推】 | `G/handler/product.go`【量】 | | |
| `ListProductsByCategoryForAdmin` | `:2150`／`P:138` | ADMIN（`:2153`）【量】 | 分類下的商品【推】 | `G/handler/admin_category_products.go`【量】 | | |
| `BatchChangeProductCategoryForAdmin` | `:2186`／`P:142` | ADMIN（`:2189`）【量】 | 批次改分類、重算 `category_lv1/lv2` 與 `cat_l*_name` 快照【推】 | 同上【量】 | | |
| `SearchHashtags` | `:2224`／`P:147` | 無【量】 | hashtag 自動完成，數量讀取時只算可曝光店的 ACTIVE 商品（IT `hashtagAutocompleteCountsOnlyExposableStores`）【量】 | `G/handler/hashtag.go`【量】 | | |
| `GetPopularHashtags` | `:2263`／`P:152` | 無【量】 | 熱門 hashtag（`PopularHashtagService`）【推】 | `G/handler/hashtag.go`【量】 | | |
| `GetPublicOriginsSummary` | `:2298`／`P:157` | 無【量】 | 產地聚合，Redis `prod:origins:summary:{cat}:{limit}`（`J/constant/CacheKeyConstants.java:78`）【量】 | `G/handler/origins.go`【量】 | | |
| `GetPublicSellerCitiesSummary` | `:2342`／`P:159` | 無【量】 | 呼叫 seller `ListPublicStoresByCity`（`ProductService.java:2863`）後算每城市商品數【推】 | `G/handler/seller_city_summary.go`【量】 | | |

### 1.2 `ProductService`：內部（庫存、購物車、結帳、單體）

| RPC | file:line | 授權 | 行為摘要 | 呼叫端 | 裁切 | 去向 |
|---|---|---|---|---|---|---|
| `DeductStock` | `:1095`／`P:57` | 無【量】 | `StockService.deductStock`（`J/service/StockService.java:108-147`）：先查 `(stock_tx_id, DEDUCT)` 是否存在，SKU 依 id 升序逐一條件扣減，不足丟例外整筆回滾，回寫 `product.stock_qty`，最後 `insertIfAbsent` 記錄；冪等檢查與寫入不是原子（見 CD1）【量】 | order-service `OrderService.java:215`、`:1174`（經 `ProductGrpcClient.java:84`）【量】 | | |
| `RestoreStock` | `:1181`／`P:60` | 無【量】 | `StockService.restoreStock`（`:153-194`）：沒有同 tx 的 DEDUCT 就 NOOP；歸還量用 caller 帶來的 items，不用 DEDUCT 記錄的 payload（見 CD2）；SKU 不存在略過【量】 | order-service `OrderService.java:419`、`worker/StockCompensationWorker.java:146`、`:206`；另由 `order.cancelled` 事件觸發（§2）【量】 | | |
| `GetStock` | `:1242`／`P:63` | 無【量】 | 批次讀 SKU 庫存【推】 | ai-service `adapter.go`；order-service `ProductGrpcClient.java:114`（**無呼叫端**）【量】 | | |
| `GetProductSimple` | `:1491`／`P:68` | 無【量】 | 加入購物車時查單一 SKU【推】 | cart-service `CartService.java:120`【量】 | | |
| `BatchGetSkuCartInfo` | `:1582`／`P:71` | 無【量】 | 購物車頁批次 SKU 資訊（含配送溫層 `delivery_temps`，`P:768`）【推】 | cart-service `CartService.java:297`【量】 | 部分（`delivery_temps`） | |
| `BatchGetProductForCheckout` | `:1709`／`P:75` | 無【量】 | 結帳批次讀商品＋SKU（含溫層，`P:787`、`:823`）【推】 | order-service `checkout/CheckoutService.java:1126`【量】 | 部分 | |
| `GetProductsByIDs` | `:1834`／`P:78` | 無【量】 | 批次卡片；`include_inactive=false` 只回可曝光店的 ACTIVE（IT `getProductsByIdsHidesNonExposableStoresForBuyers`）【量】 | 單體 `B/internal/legacy/productservice/productservice.go:41`（`Cards`）→ social、seller 彙總、buyerrelay、buyer 收藏卡片（`B/cmd/hawkr/social.go:31,49`、`seller_events.go:42`、`buyer_ports.go:14`、`B/internal/legacy/buyerrelay/relay.go:202`）【量】 | | |
| `ListStoreIDsWithPublicProductsInternal` | `:2377`／`P:166` | 無【量】 | 至少有一個公開商品的店家 ID 全集【推】 | 單體 `productservice.go:71`（`StoresWithPublicProducts`）→ discovery ranking port（`B/internal/modules/discovery/internal/ranking/ports.go:21`）【量】 | | |

### 1.3 `ProductService`：認證（全部屬裁切）

| RPC | file:line | 授權 | 行為摘要 | 呼叫端 | 裁切 | 去向 |
|---|---|---|---|---|---|---|
| `VerifyCertification` | `:2046`／`P:96` | SELLER（`:2049`）【量】 | 即時打政府 API 查證號（`GovCertificationService`，Redis `gov:certv2:*` 快取、`gov:cert:circuit:*` 斷路）【推】 | `G/handler/product.go`【量】 | 刪 | |
| `SubmitProductCertification` | `:2414`／`P:103` | SELLER 或 SELLER_AND_BUYER（`:2417`）【量】 | 建申請，`after commit` 發 `product.certification.verify.requested`（`ProductCertificationApplicationService.java:1425`）【量】 | `G/handler/seller_product_certification.go`【量】 | 刪 | |
| `ListProductCertificationApplications` | `:2465`／`P:106` | 同上（`:2469`）【量】 | 賣家看自己的申請【推】 | 同上【量】 | 刪 | |
| `WithdrawProductCertification` | `:2515`／`P:109` | 同上（`:2518`）【量】 | 撤回【推】 | 同上【量】 | 刪 | |
| `ListProductCertificationApplicationsForAdmin` | `:2562`／`P:114` | ADMIN（`:2566`）【量】 | 審核佇列【推】 | `G/handler/admin_product_certification.go`【量】 | 刪 | |
| `GetProductCertificationApplicationForAdmin` | `:2613`／`P:117` | ADMIN（`:2617`）【量】 | 單筆【推】 | 同上【量】 | 刪 | |
| `ListProductCertificationAuditLogsForAdmin` | `:2640`／`P:120` | ADMIN（`:2644`）【量】 | 讀 `product_certification_review_event`【推】 | 同上【量】 | 刪 | |
| `ApproveProductCertificationForAdmin` | `:2684`／`P:123` | ADMIN（共用 helper，`:2734`）【推】 | 核准，發 `product.certification.approved`（`:1487`）【量】 | 同上【量】 | 刪 | |
| `RejectProductCertificationForAdmin` | `:2697`／`P:126` | 同上【推】 | 駁回，發 `…rejected`（`:1488`）【量】 | 同上【量】 | 刪 | |
| `RevokeProductCertificationForAdmin` | `:2711`／`P:129` | 同上【推】 | 撤銷，發 `…revoked`（`:1489`）【量】 | 同上【量】 | 刪 | |
| `ReverifyProductCertificationForAdmin` | `:2730`／`P:133` | ADMIN（`:2773`）【推】 | 重新查驗，Redis lease `cert:reverify:lease:*`（`ProductCertificationApplicationService.java:1003`）【量】 | 同上【量】 | 刪 | |

### 1.4 `CategoryService`（`J/grpc/CategoryGrpcService.java`）

| RPC | file:line | 授權 | 行為摘要 | 呼叫端 | 裁切 | 去向 |
|---|---|---|---|---|---|---|
| `GetCategoryTree` | `:41`／`C:21` | 無【量】 | 整棵樹，Redis `cat:all`（`CategoryCacheService`）【推】 | `G/handler/category.go`；單體 `B/internal/legacy/categoryservice/categoryservice.go:47` → procurement taxonomy（`B/cmd/hawkr/procurement.go:62`）；ai-service 4 處【量】 | | |
| `GetChildCategories` | `:68`／`C:24` | 無【量】 | 子分類【推】 | `G/handler/category.go`【量】 | | |
| `CreateCategory` | `:99`／`C:27` | SELLER 或 ADMIN（`:101`）【量】 | 賣家可建分類【推】 | 同上【量】 | | |
| `CreateCategoryForAdmin` | `:122`／`C:32` | ADMIN（`:124`）【量】 | | 同上【量】 | | |
| `UpdateCategoryForAdmin` | `:145`／`C:35` | ADMIN（`:147`）【量】 | 改名觸發商品 `cat_l*_name` 重算（`ProductCascadeService`、`CategoryServiceRevalidateTest`）【推】 | 同上【量】 | | |
| `BatchSoftDeleteCategoryForAdmin` | `:178`／`C:38` | ADMIN（`:180`）【量】 | | 同上【量】 | | |
| `RestoreCategoryForAdmin` | `:207`／`C:41` | ADMIN（`:209`）【量】 | | 同上【量】 | | |

## 2. MQ consumer（`J/mq`，宣告在 `J/config/RabbitMqConfig.java`，每個 queue 都有 DLQ 綁到 `hawkr.product.dlx`）

| Consumer | file:line | exchange／routing key → queue | 行為摘要 | 目前的生產者 | 裁切 | 去向 |
|---|---|---|---|---|---|---|
| `OrderCancelledConsumer` | `OrderCancelledConsumer.java:39` | `hawkr.order`／`order.cancelled` → `QUEUE_PRODUCT_ORDER_CANCELLED`（`RabbitMqConfig.java:41-57`）【量】 | 讀 `stock_tx_id`＋`items`，呼叫 `restoreStock`【量】 | Java order-service（`OrderService.java`、`mq/PaymentEventHandler.java` 引用 `ROUTING_KEY_ORDER_CANCELLED`）【量】 | | |
| `SalesAccumulatedConsumer` | `SalesAccumulatedConsumer.java:59` | `hawkr.order`／`order.sales.accumulated`（`:70-86`）【量】 | 以 `order_no` 寫 `sales_count_event_dedup`，成功才逐 product `incrementSalesCount`，同一 transaction【量】 | Java order-service（同上）【量】；Go 無生產者（`rg -l '"order.sales.accumulated' --glob '*.go'` 無結果）【量】 | | |
| `FavoriteCountConsumer#added` | `FavoriteCountConsumer.java:47` | `hawkr.buyer`／`buyer.favorite.added`（`:115-131`）【量】 | `event_id` 去重後 `favorite_count + 1`（`FavoriteCountService.java`）【量】 | 單體 `B/internal/legacy/buyerrelay/relay.go:234`【量】 | | |
| `FavoriteCountConsumer#removed` | `FavoriteCountConsumer.java:55` | `hawkr.buyer`／`buyer.favorite.removed`（`:142-158`）【量】 | 去重後 `-1`，`WHERE favorite_count > 0`（`ProductRepository.java:448-454`）【量】 | `relay.go:247`【量】 | | |
| `ReviewStatsConsumer` | `ReviewStatsConsumer.java:47` | `hawkr.review`／`review.created`＋`review.hidden` → `product.review.stats`（`:245-277`）【量】 | 用事件帶的平均分與則數覆寫 `rating`／`review_count`，沒有版本比較（`ReviewStatsService.java`，見 CD4）【量】 | 單體 `B/internal/legacy/reviewrelay/relay.go`、`B/cmd/hawkr/review.go`【量】 | | |
| `SellerDeletedConsumer` | `SellerDeletedConsumer.java:44` | `hawkr.seller`／`seller.deleted`【量】 | `SellerProductOffShelfService.offShelfAllByMerchant`：先無條件蓋 `seller_deleted_at`，再批次改未啟用、清快取【量】 | 單體 `B/internal/legacy/sellerrelay/relay.go`【量】 | | |
| `SellerExposureChangedConsumer` | `SellerExposureChangedConsumer.java:42` | `hawkr.seller`／`seller.exposure.changed`（`:213-231`）【量】 | 以 `version` 單調 upsert `store_exposure`（IT `versionRules`）【量】 | 單體 `sellerrelay.OnStoreExposureChanged`（`B/cmd/hawkr/seller_events.go:34`、`relay.go:118`）；B4 計畫要求搬完後刪（`go-modular-monolith.md` B4 第 1 項）【量】 | | |
| `CertificationVerifyConsumer` | `CertificationVerifyConsumer.java:66` | `hawkr.product`／`product.certification.verify.requested`（`:304-318`）【量】 | 自產自銷：背景打政府 API 查驗【推】 | product-service 自己（`ProductEventPublisher.java:49`）【量】 | 刪 | |

## 3. MQ publisher（`J/mq/ProductEventPublisher.java`，exchange `hawkr.product`，皆 after-commit、best-effort，失敗只記 log）

| 方法 | file:line | routing key | 呼叫點 | 目前的消費者 | 裁切 | 去向 |
|---|---|---|---|---|---|---|
| `publishRestockAfterCommit` | `:134` | `product.restock`（`MQ:76`）【量】 | `ProductService.java:243`（更新 SKU 補貨）、`StockService.java:247`（歸還庫存）【量】 | 單體 `B/internal/legacy/buyerconsumer/rabbitmq.go:14`（queue `hawkr.monolith.buyer.product.restock`）→ buyer 補貨通知 fan-out，再由 `buyerrelay/relay.go:301` 發 `product.restock.target`【量】 | | |
| `publishCertificationVerifyRequested` | `:49` | `product.certification.verify.requested` | `ProductCertificationApplicationService.java:1425`【量】 | 自己（§2 最後一列）【量】 | 刪 | |
| `publishCertificationApproved` | `:64` | `product.certification.approved`（`MQ:91`） | `…ApplicationService.java:1487`【量】 | **找不到消費者**（見 CD7）【量】 | 刪 | |
| `publishCertificationRejected` | `:70` | `product.certification.rejected`（`MQ:93`） | `:1488`【量】 | 同上【量】 | 刪 | |
| `publishCertificationRevoked` | `:76` | `product.certification.revoked`（`MQ:95`） | `:1489`【量】 | 同上【量】 | 刪 | |

`MQ:180-182` 另有 `product.created`／`product.updated`／`product.status.changed` 常數，product-service 沒有發布【推】（`rg -n 'ROUTING_KEY_PRODUCT_(CREATED\|UPDATED\|STATUS_CHANGED)' backend/services/java/product-service` 本次未跑）。

## 4. 排程

| Job | file:line | 行為 | 裁切 | 去向 |
|---|---|---|---|---|
| `IdempotencyCleanupJob` | `J/scheduler/IdempotencyCleanupJob.java:31`（cron `0 0 2 * * ?`） | `DELETE FROM idempotency_request WHERE created_at < NOW() - INTERVAL '24 hours'`（`J/repository/IdempotencyRepository.java:93`）【量】 | | |
| `FavoriteEventDedupCleanupJob` | `J/scheduler/FavoriteEventDedupCleanupJob.java:32`（cron `0 30 2 * * ?`，保留 30 天） | 刪 `favorite_count_event_dedup` 舊列（`FavoriteCountEventDedupRepository.java:37`）【量】 | | |

`sales_count_event_dedup` 與 `stock_transaction` 沒有清理排程【量】（`rg -n '@Scheduled' …` 只有上面兩支）。兩支都沒有 `@SchedulerLock`，多實例會重複執行（刪除是冪等的，無害）【推】。

## 5. gRPC outbound

| 目標.RPC | client file:line | 呼叫點 | 目前的伺服器 | 裁切 | 去向 |
|---|---|---|---|---|---|
| seller `BatchGetSellerMeta` | `J/grpc/client/SellerGrpcClient.java:135`、`:165`、`:188`（`batchGetSellerMetas`／`getSellerMetaOrThrow`／`getSellerMeta`） | `ProductService.java:1300`、`:1868`、`:2898`；`ProductRevalidationService.java:77`；`ProductGrpcService.java:1377`（結果沒被使用，CD6）；`ProductCertificationApplicationService.java:1461`【量】 | 單體 `B/internal/legacy/sellerserver/server.go`【量】 | | |
| seller `MatchCertificateHolder` | `SellerGrpcClient.java:82` | `ProductCertificationApplicationService.java:480`【量】 | `sellerserver/server.go`、`B/internal/modules/seller/kyc.go`【量】 | 刪 | |
| seller `ListPublicStoreIDsByCity` | `SellerGrpcClient.java:201-` | `ProductService.java:1112`（列表的賣家城市篩選）【量】 | `sellerserver/server.go`【量】 | | |
| seller `ListPublicStoresByCity` | `SellerGrpcClient.java:219-` | `ProductService.java:2863`【量】 | `sellerserver/server.go`【量】 | | |
| buyer `ListFavoritedProductIds` | `J/grpc/client/BuyerGrpcClient.java:73` | `ProductService.java:1128`【量】 | 單體 `B/internal/legacy/buyerserver/server.go`【量】 | | |
| buyer `ListSubscribedSellerIds` | `BuyerGrpcClient.java:111` | `ProductService.java:1154`【量】 | 同上【量】 | | |
| shipping `CalculateShippingOptions` | `J/grpc/client/ShippingGrpcClient.java:53` | `ProductGrpcService.java:1395`【量】 | Java shipping-service【量】 | 部分（溫層分組） | |
| shipping `GetSellerShippingConfig` | `ShippingGrpcClient.java:92` | `ProductService.java:1016`（onboarding 指紋，進快取 key）【推】 | Java shipping-service【推】 | | |
| shipping `ValidateProductShipping` | `ShippingGrpcClient.java:117` | `ProductService.java:2142`（建立／更新時驗 SKU 配送方式含溫層）【推】 | Java shipping-service【推】 | 部分 | |
| media `FinalizeImages` | `J/grpc/client/MediaGrpcClient.java:83` | `ProductService.java:1911`、`:1953`、`:2464`、`:2519`【量】 | media（單體 `B/internal/modules/media` 是否接手本 RPC 本次未查）【推】 | | |
| media `LinkImages` | `MediaGrpcClient.java:132` | `ProductMediaReferenceLinker.java:75`、`:86`；`ProductService.java:2554`、`:2565`、`:2587`、`:2622`【量】 | 同上【推】 | | |
| media `UnlinkImages` | `MediaGrpcClient.java:156` | `ProductService.java:2596`、`:2631`【量】 | 同上【推】 | | |
| media `ReleaseReferences` | `MediaGrpcClient.java:182` | `ProductService.java:2648`、`:2653`【量】 | 同上【推】 | | |
| 政府認證查驗（HTTP） | `J/service/GovCertificationService.java`、設定 `J/config/GovApiProperties.java`、`R/application.yml:95-102` | `VerifyCertification`、`CertificationVerifyConsumer`、reverify【推】 | 外部 | 刪 | |

## 6. DB 表（`R/db/migration`，DB `hawkr_product`）

| 表 | 建立／修改 | 值得注意的欄位與約束 | 寫入者 | 裁切 | 去向 |
|---|---|---|---|---|---|
| `category` | V1:21 | `parent_id` 自參照 FK；`status` 是 CodeMeta 字串（預設 `'9004'`）；`uk_category_name_parent`（V1:36）；軟刪 `deleted_at`【量】 | CategoryService | | |
| `product` | V1:54；V3（`stock_qty`、`favorite_count`、`shipping_methods` JSONB）、V6（`review_count`）、V7（`seller_deleted_at`）、V8/V9（`store_id`）、V15（`first_published_at`）、V16（`version`）、V22 刪 import 欄位 | `id` 是 TSID（應用層產生）；`status` VARCHAR(4) CodeMeta（4001–4004）；分類三層反正規化＋名稱快照；`searchable_text` trigram GIN（V1:109）；7 支 `WHERE deleted_at IS NULL AND status='4004'` 的部分索引；銷量、收藏、評分、則數、庫存都是反正規化計數，各有非負 CHECK（V3、V5、V6）；`certifications` JSONB＋GIN（V2）【量】 | ProductService、各 consumer | 部分（`certifications` 欄與 GIN；`shipping_methods` 內的 `temperature`） | |
| `product_sku` | V1:148；V17（`version`）、V19（`restricted_category_codes` JSONB） | `id` BIGSERIAL（與 product 的 TSID 不同方案）；`sale_price` CHECK `NULL OR (0 < sale_price < price)`；`stock_qty >= 0`；`shipping_method` JSONB `[{type, temperature, freeThreshold, feePayer}]`；`chk_product_sku_restricted_category_codes` 鎖成只能空陣列（V19:50）【量】 | ProductService、StockService | 部分（`shipping_method[].temperature`） | |
| `idempotency_request` | V1:183；V12–V14 | PK 改為 `(key, store_id)`（V14:21）；24 小時清理【量】 | IdempotencyService | | |
| `stock_transaction` | V1:209 | PK `(stock_tx_id, operation)`；`payload` JSONB；`status` 預設 `'8912'`；無清理【量】 | StockService | | |
| `hashtag` | V1:226；V21 刪 `product_count` | `uk_hashtag_name`；trigram GIN【量】 | HashtagService | | |
| `product_hashtag` | V1:252 | PK `(product_id, hashtag_id)`；`display_name`、`position`【量】 | HashtagService | | |
| `favorite_count_event_dedup` | V3:36 | PK `event_id`；30 天清理【量】 | FavoriteCountService | | |
| `sales_count_event_dedup` | V11:10 | PK `order_no`；無清理【量】 | SalesAccumulatedConsumer | | |
| `store_exposure` | V20:3 | PK `store_id`；`version > 0`；沒有列＝不可曝光【量】 | SellerExposureChangedConsumer；B4 計畫改為 SQL 內 join `seller.pub_exposable_stores(at)`，本表不建 | | |
| `product_certification_application` | V18:16 | 部分唯一索引 `uk_pca_pending`、`uk_pca_active`；CHECK `ck_pca_status`／`gov_state`／`cert_type BETWEEN 3701 AND 3706`【量】 | ProductCertificationApplicationService | 刪 | |
| `product_certification_review_event` | V18:100 | 稽核事件，無 FK【量】 | 同上 | 刪 | |

所有跨表參照只有 `product.category*`→`category`、`product_sku.product_id`→`product`、`product_hashtag`→`product`／`hashtag` 有 FK；其餘（`store_id`、認證表）不建 FK【量】。

## 7. Redis

| Key | 定義 | TTL／用途 | 裁切 | 去向 |
|---|---|---|---|---|
| `prod:detail:{id}:{onboarding_sig}` | `J/constant/CacheKeyConstants.java:14`、`:35` | 詳情快取；寫入時以 pattern `prod:detail:{id}:*` 清【量】 | | |
| `prod:list:{store_id}:{query_hash}:{sig}` | `:20`、`:69` | 列表快取；賣家寫入時清該店 pattern【量】 | | |
| `prod:origins:summary:{cat}:{limit}` | `:78` | 產地聚合【量】 | | |
| `cat:all` | `:26`；`J/service/CategoryCacheService.java` | 整棵分類樹【量】 | | |
| `gov:certv2:*` | `J/service/GovCertificationService.java:77` | 政府查驗結果，TTL 7 天／1 小時／1 天（`application.yml:100-102`）【量】 | 刪 | |
| `gov:cert:circuit:*` | `GovCertificationService.java:79` | 斷路器狀態【量】 | 刪 | |
| `cert:reverify:lease:*` | `ProductCertificationApplicationService.java:1003` | reverify 互斥【量】 | 刪 | |

Redis 連線 `timeout: 500ms`（`application.yml:47`）【量】。

## 8. 設定（`R/application.yml`）

| 項目 | 行 | 內容 | 裁切 | 去向 |
|---|---|---|---|---|
| 埠 | `:2`、`:6` | HTTP 9912、gRPC 9913【量】 | | |
| DB | `:33-55` | `hawkr_product`；Flyway 直連 `DB_PORT`、應用走 `DB_POOL_PORT`【量】 | | |
| RabbitMQ／Redis | `:28-48` | 【量】 | | |
| gRPC 下游 | `:76-89` | media 9937；seller、buyer 都指向 9941（單體的 legacy gRPC）；shipping 9931【量】 | | |
| `hawkr.tsid` | `:91` | TSID 產生器節點設定【推】 | | |
| `hawkr.gov-api` | `:95-102` | 逾時與快取 TTL【量】 | 刪 | |
| `product.cleanup.*` | `:105-109` | 兩支排程的 cron【量】 | | |

## 9. 單體內的依賴（catalog 搬進單體後要換掉的 port）

| 消費模組 | port／接點 | 現在用的 RPC | 去向 |
|---|---|---|---|
| seller | `B/internal/modules/seller/internal/views/views.go:26` `ProductStores.StoreIDs`；`B/cmd/hawkr/seller_events.go:39-42` | `GetProductsByIDs`（`include_inactive=true`）【量】 | |
| social | `B/internal/modules/social/internal/post/post.go:42` `ProductVerifier`；`B/cmd/hawkr/social.go:28-49` | `GetProductsByIDs`【量】 | |
| discovery | `B/internal/modules/discovery/internal/ranking/ports.go:21` `Products.StoresWithPublicProducts` | `ListStoreIDsWithPublicProductsInternal`【量】 | |
| buyer（收藏卡片、補貨通知） | `B/cmd/hawkr/buyer_ports.go:11-14`、`B/internal/legacy/buyerrelay/relay.go:102`、`:202` | `GetProductsByIDs`；收 `product.restock`【量】 | |
| procurement | `B/cmd/hawkr/procurement.go:44-70` | `GetProductList`（搜尋）、`GetCategoryTree`【量】 | |
| review | 經 `reviewrelay` 發 `review.created/hidden` 給 product 更新評分【量】；review 模組本身是否直接查商品，本次未查【推】 | — | |
| content | banner 的 `product` 動作（`B/internal/modules/content/internal/banner/banner.go:70`）目前不驗商品；B4 計畫要求改為經 catalog 公開 API 確認、ID 換 UUID【量】 | — | |
| order／cart（仍在 Java） | 見 §1.2 | `DeductStock`、`RestoreStock`、`BatchGetProductForCheckout`、`GetProductSimple`、`BatchGetSkuCartInfo`【量】 | |
| 賣家／買家互動計數 | buyer 發收藏事件、order 發銷量事件、review 發評分事件（§2）；B4 計畫要求改成「事實發生的模組呼叫 catalog API」【量】 | — | |
