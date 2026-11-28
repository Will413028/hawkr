# B4-0 測試盤點：Java product-service 的測試方法（唯讀，快照 dev `496b048e2`）

標記：【量】＝跑過指令或打開檔案看過；【推】＝只憑方法名、`@DisplayName` 或檔名推論，沒讀斷言。路徑縮寫：`T=backend/services/java/product-service/src/test/java/io/github/will413028/hawkr/product`。「對照」欄在 B4-1 填：Go 端在哪一步以什麼測試保住該行為，或刪除理由（依計畫的決定 D1–D20）。「說明」欄是 `@DisplayName` 原文（沒有就空白），不是讀斷言的結果【推】。所有指令從 `/Users/will/second-brain/.projects/personal/hawkr` 執行。本盤點沒有跑任何測試（Testcontainers 要 Docker，本機不跑；依 hawkr 規則交給 CI）。

## 0. 數字與指令

| 項目 | 量到 | 指令 |
|---|---|---|
| 測試檔（含測試方法） | 51（另有基底類別 `T/integration/AbstractProductPostgresIT.java` 與 `src/test/resources/product-it/extensions.sql`） | `rg -c '@Test\|@ParameterizedTest' backend/services/java/product-service/src/test` 列出 51 檔【量】 |
| 測試方法總數 | **364**（`@Test`／`@ParameterizedTest` 後第一個 `void x(`）；有 `@DisplayName` 的 345 | 上式加總 364【量】；逐方法清單由 python 掃描產生，數量與 `rg -c` 加總一致【量】 |
| 名稱含認證或受限類別的測試檔 | 14 檔、181 個方法（檔名含 `Certification`、`GovCertification`、`RestrictedCategory`；認證屬範圍裁切，`RestrictedCategory` 不屬裁切，只因值域鎖空陣列一起標出）；§3 標「刪」的是檔名含 `Certification` 的 174 個方法【量】| 檔名比對【量】 |
| integration 測試 | **14 個方法、2 個類別**（`StoreExposureUpsertIT` 4、`StoreExposureVisibilityIT` 10），共用基底 `AbstractProductPostgresIT`（`@SpringBootTest(classes = MinimalConfig.class)`、`@Tag("integration")`，`:23-24`） | `rg -n 'Testcontainers\|@Tag\|@SpringBootTest' backend/services/java/product-service/src/test`【量】 |
| 其餘 350 個 | 單元測試（不起 Spring context、不連 DB／RabbitMQ／Redis）【推】（未逐檔 grep `@Mock`） | 同上，其餘檔案無命中【量】 |
| 預設會不會跑 integration | 預設不跑：pom `excludedGroups=integration`（`backend/services/java/product-service/pom.xml:34-35`）；surefire `includes` 加 `**/*IT.java`（`:248-255`） | `rg -n 'excludedGroups\|IT.java' backend/services/java/product-service/pom.xml`【量】 |
| CI 位置 | `.github/workflows/backend-java-tests.yml` 的 `integration-test` job（`:52`） | `rg -n 'integration' .github/workflows/backend-java-tests.yml`【量】 |

## 1. integration 測試（14）

兩個 IT 都只測 `store_exposure` 讀取模型與公開讀取面的過濾；B4 計畫不建這張表（改 join `seller.pub_exposable_stores(at)`），但它們斷言的**可見性規則**（公開列表、總數、摘要、hashtag 數量、詳情、`GetProductsByIDs` 都只算可曝光店）是新實作要保住的行為【推】。

| class#method | 說明（`@DisplayName`） | 對照 |
|---|---|---|
| `StoreExposureUpsertIT#missingRowIsNotExposable`（`T/integration/StoreExposureUpsertIT.java:36`） | 沒有列：isExposable 為 false | 刪：不建 `store_exposure`，改 join `seller.pub_exposable_stores(at)` |
| `StoreExposureUpsertIT#versionRules`（`T/integration/StoreExposureUpsertIT.java:42`） | 版本 5 寫入後收到版本 3 → 列不變；收到版本 5 重送 → 不變；版本 6 → 更新 | 刪：不建 `store_exposure`，改 join `seller.pub_exposable_stores(at)` |
| `StoreExposureUpsertIT#versionsAreScopedPerStore`（`T/integration/StoreExposureUpsertIT.java:60`） | 首次寫入 exposable=true 後 isExposable 為 true；版本只在同一家店之間比較 | 刪：不建 `store_exposure`，改 join `seller.pub_exposable_stores(at)` |
| `StoreExposureUpsertIT#suspensionSnapshotTableIsDropped`（`T/integration/StoreExposureUpsertIT.java:71`） | product_suspension_snapshot 已刪除 | 刪：不建該表 |
| `StoreExposureVisibilityIT#publicListHidesNonExposableStores`（`T/integration/StoreExposureVisibilityIT.java:110`） | 公開列表：只含可曝光店的 ACTIVE 商品，總數與分頁都以過濾後為準；沒有 store_exposure 列的店同樣隱藏 | 改寫：B4-3 integration（公開列表、總數、熱門與自動完成 hashtag、商品頁、批次卡片只算可曝光店的 active 商品；賣家與 admin 視角不過濾）；mutation `b4-3.json` 拿掉 join |
| `StoreExposureVisibilityIT#ownerViewIsNotFilteredByExposure`（`T/integration/StoreExposureVisibilityIT.java:125`） | 賣家看自己商品／admin 視角（不要求可曝光）：不可曝光的店的商品仍在列表與總數內 | 改寫：B4-3 integration（公開列表、總數、熱門與自動完成 hashtag、商品頁、批次卡片只算可曝光店的 active 商品；賣家與 admin 視角不過濾）；mutation `b4-3.json` 拿掉 join |
| `StoreExposureVisibilityIT#exposureFlipChangesListImmediately`（`T/integration/StoreExposureVisibilityIT.java:133`） | 店家由可曝光翻成不可曝光後，列表與總數立即不含它的商品 | 改寫：B4-3 integration（公開列表、總數、熱門與自動完成 hashtag、商品頁、批次卡片只算可曝光店的 active 商品；賣家與 admin 視角不過濾）；mutation `b4-3.json` 拿掉 join |
| `StoreExposureVisibilityIT#summariesCountOnlyExposableStores`（`T/integration/StoreExposureVisibilityIT.java:144`） | 產地摘要與店家公開商品數只計可曝光店的 ACTIVE 商品 | 改寫：B4-3 integration（店家公開商品數只算可曝光店）；產地摘要無 client 刪（D5） |
| `StoreExposureVisibilityIT#popularHashtagsCountOnlyExposableStores`（`T/integration/StoreExposureVisibilityIT.java:157`） | 熱門 hashtag 只計可曝光店的 ACTIVE 商品 | 改寫：B4-3 integration（公開列表、總數、熱門與自動完成 hashtag、商品頁、批次卡片只算可曝光店的 active 商品；賣家與 admin 視角不過濾）；mutation `b4-3.json` 拿掉 join |
| `StoreExposureVisibilityIT#briefCarriesExposureIntoPolicy`（`T/integration/StoreExposureVisibilityIT.java:174`） | brief 一次查詢帶回 storeExposable，Policy 對不可曝光或沒有列的店回 TEMPORARILY_UNAVAILABLE | 改寫：B4-3 integration（公開列表、總數、熱門與自動完成 hashtag、商品頁、批次卡片只算可曝光店的 active 商品；賣家與 admin 視角不過濾）；mutation `b4-3.json` 拿掉 join |
| `StoreExposureVisibilityIT#publicDetailReflectsExposure`（`T/integration/StoreExposureVisibilityIT.java:189`） | 公開詳情：不可曝光的店（含沒有列）回 TEMPORARILY_UNAVAILABLE 且不可購買；可曝光的店 PUBLIC 且可購買 | 改寫：B4-3 integration（公開列表、總數、熱門與自動完成 hashtag、商品頁、批次卡片只算可曝光店的 active 商品；賣家與 admin 視角不過濾）；mutation `b4-3.json` 拿掉 join |
| `StoreExposureVisibilityIT#hashtagAutocompleteCountsOnlyExposableStores`（`T/integration/StoreExposureVisibilityIT.java:208`） | hashtag 自動完成：數量在讀取時只計可曝光店的 ACTIVE 商品，不用預先計算的 product_count | 改寫：B4-3 integration（公開列表、總數、熱門與自動完成 hashtag、商品頁、批次卡片只算可曝光店的 active 商品；賣家與 admin 視角不過濾）；mutation `b4-3.json` 拿掉 join |
| `StoreExposureVisibilityIT#hashtagItemCountExcludesNonExposableStores`（`T/integration/StoreExposureVisibilityIT.java:231`） | 商品詳情／列表的 hashtag 數量在讀取時計算：不含不可曝光店與非 ACTIVE 的商品，沒有符合商品時為 0 | 改寫：B4-3 integration（公開列表、總數、熱門與自動完成 hashtag、商品頁、批次卡片只算可曝光店的 active 商品；賣家與 admin 視角不過濾）；mutation `b4-3.json` 拿掉 join |
| `StoreExposureVisibilityIT#getProductsByIdsHidesNonExposableStoresForBuyers`（`T/integration/StoreExposureVisibilityIT.java:258`） | getProductsByIds：includeInactive=false 只回可曝光店的 ACTIVE 商品；true 不受影響 | 改寫：B4-3 integration（公開列表、總數、熱門與自動完成 hashtag、商品頁、批次卡片只算可曝光店的 active 商品；賣家與 admin 視角不過濾）；mutation `b4-3.json` 拿掉 join |

## 2. 沒有測試覆蓋的行為（供 characterization 規劃）

`rg -l <符號> backend/services/java/product-service/src/test \| wc -l` 的命中檔數【量】：

| 符號 | 命中檔數 | 說明 |
|---|---|---|
| `DeductStock`／`RestoreStock`（gRPC handler） | 0／0 | 庫存 RPC 沒有 handler 測試；service 層 `deductStock` 3 檔、`restoreStock` 2 檔，都是快取清除與 `stock_qty` 回寫（`StockServiceCacheEvictionTest`、`ProductServiceUpdateProductStockQtyTest`、`ProductServiceSkuOptimisticLockTest`），沒有冪等或併發測試（defects CD1、CD2） |
| `getProductSimple`、`batchGetSkuCartInfo`、`batchGetProductForCheckout` | 0、0、0 | cart、checkout 用的三支內部 RPC 沒有測試 |
| `calculateShippingOptions` | 0 | 運費試算沒有測試（defects CD6） |
| `OrderCancelledConsumer`、`FavoriteCountService`、`ReviewStatsService` | 1（只是字串出現）、0、0 | 取消歸還、收藏數、評分更新沒有行為測試 |
| `IdempotencyCleanupJob`、`FavoriteEventDedupCleanupJob` | 0、0 | 排程沒有測試 |
| `getPublicProduct`、`getProductList` | 1、2 | 只有 IT 與 `ProductServiceListGuardTest` 間接觸及；排序、篩選、分頁、快取 key 沒有測試【推】 |

## 3. 全部測試方法（364）

依檔案分組；「裁切」欄：**刪**＝檔名屬認證流程（範圍裁切，改寫時不帶）【推】，其餘空白。

| 檔案 | method（行） | 說明（`@DisplayName`） | 裁切 | 對照 |
|---|---|---|---|---|
| `T/converter/ProductCertificationApplicationConverterAdminProductStateTest.java` | `softDeletedProductStillResolvesNameAndIsMarkedRemoved`（`:64`） | 商品已軟刪：名字仍取得到，且標成 REMOVED —— 這是 #2818 的症狀本身 | 刪 | 刪：範圍裁切（認證） |
| `T/converter/ProductCertificationApplicationConverterAdminProductStateTest.java` | `sellerDeletedProductIsAlsoRemoved`（`:78`） | 賣家帳號已刪（seller_deleted_at）：同樣算已移除 —— 商品有兩個軟刪戳 | 刪 | 刪：範圍裁切（認證） |
| `T/converter/ProductCertificationApplicationConverterAdminProductStateTest.java` | `livePublishedProductIsPublic`（`:90`） | 陰性：live 且上架中的商品標 PUBLIC，不得被標成已移除 | 刪 | 刪：範圍裁切（認證） |
| `T/converter/ProductCertificationApplicationConverterAdminProductStateTest.java` | `inactiveButPublishedIsTemporarilyUnavailable`（`:103`） | 陰性：下架中（曾公開）是 TEMPORARILY_UNAVAILABLE，不是已移除 —— 這種申請照常審 | 刪 | 刪：範圍裁切（認證） |
| `T/converter/ProductCertificationApplicationConverterAdminProductStateTest.java` | `missingProductRowLeavesNameEmptyButKeepsProductId`（`:116`） | 商品列查不到（實體刪除）：名字空字串、狀態不設，但 productId 仍在 | 刪 | 刪：範圍裁切（認證） |
| `T/converter/ProductEntityConverterOwnerTest.java` | `toEntityStampsOwnerAndAudit`（`:36`） | toEntity：storeId 與 createdBy 都 stamp 為請求 storeId（owner + audit 雙寫） |  | 改寫：B4-3 handler 測試，店家由呼叫者推導寫入（D4） |
| `T/converter/ProductProtoConverterCertificationClearTest.java` | `clearsWhenFlagSetAndListEmpty`（`:40`） | update_certifications=true ＋ 空清單 → DTO 給空 list（可清空），不是 null | 刪 | 刪：範圍裁切（認證） |
| `T/converter/ProductProtoConverterCertificationClearTest.java` | `keepsCurrentWhenNoFlagAndListEmpty`（`:51`） | 未帶旗標 ＋ 空清單 → 維持 null（不動認證），這是舊 client 的相容行為 | 刪 | 刪：範圍裁切（認證） |
| `T/converter/ProductProtoConverterCertificationClearTest.java` | `stillUpdatesWhenNoFlagButListNonEmpty`（`:60`） | 未帶旗標 ＋ 非空清單 → 仍然更新（product-service 與 gateway 分開部署的窗口） | 刪 | 刪：範圍裁切（認證） |
| `T/converter/ProductProtoConverterCertificationClearTest.java` | `replacesWhenFlagSetAndListNonEmpty`（`:77`） | 旗標 ＋ 非空清單 → 全量取代 | 刪 | 刪：範圍裁切（認證） |
| `T/converter/ProductProtoConverterStockZeroTest.java` | `zeroStockSurvivesConversion`（`:52`） | stockQty=\ |  | 刪：proto 字串數量轉換是 Java 的 wire 形狀；Connect 用 `int32`，B4-3 handler 測試涵蓋庫存 0 |
| `T/converter/ProductProtoConverterStockZeroTest.java` | `absentStockStaysNull`（`:62`） | stockQty=\ |  | 刪：proto 字串數量轉換是 Java 的 wire 形狀；Connect 用 `int32`，B4-3 handler 測試涵蓋庫存 0 |
| `T/converter/ProductProtoConverterStockZeroTest.java` | `positiveStockConverts`（`:70`） | 一般正值照常轉換 |  | 刪：proto 字串數量轉換是 Java 的 wire 形狀；Connect 用 `int32`，B4-3 handler 測試涵蓋庫存 0 |
| `T/converter/ProductProtoConverterStockZeroTest.java` | `garbageStockBecomesNull`（`:76`） | 非數字字串 → null，不拋例外（沿用 parseIntOrNull 既有約定） |  | 刪：proto 字串數量轉換是 Java 的 wire 形狀；Connect 用 `int32`，B4-3 handler 測試涵蓋庫存 0 |
| `T/converter/ProductSkuVersionConversionTest.java` | `entityVersionFlowsIntoDto`（`:38`） | Entity.version → DTO.version：不下發，client 就沒有東西可以在下次寫回時回送 |  | 刪：`version`／`expectedVersion` 改 etag（D4） |
| `T/converter/ProductSkuVersionConversionTest.java` | `nullEntityVersionStaysNull`（`:49`） | Entity.version 為 null（migration 之前的舊物件）→ DTO 給 null，不得變成字面 \ |  | 刪：`version`／`expectedVersion` 改 etag（D4） |
| `T/converter/ProductSkuVersionConversionTest.java` | `dtoVersionFlowsIntoProto`（`:61`） | DTO.version → proto version：Gateway 要靠它才有 skus[].version 可以吐給前端 |  | 刪：`version`／`expectedVersion` 改 etag（D4） |
| `T/converter/ProductSkuVersionConversionTest.java` | `protoExpectedVersionFlowsIntoDto`（`:75`） | proto expected_version → DTO.expectedVersion：漏了的話 client 送了也到不了 CAS |  | 刪：`version`／`expectedVersion` 改 etag（D4） |
| `T/converter/ProductSkuVersionConversionTest.java` | `emptyExpectedVersionBecomesNull`（`:82`） | proto expected_version 為空字串（舊 client）→ DTO 給 null，不得讓 parseLong 炸成 400 |  | 刪：`version`／`expectedVersion` 改 etag（D4） |
| `T/domain/ProductPublicVisibilityPolicyTest.java` | `distinguishesAbsentNeverPublishedAndRemovedProducts`（`:12`） |  |  | 改寫：B4-3 Public 商品頁測試（不存在、未上架、已刪除的區分；可曝光 fail closed）；B4-1 特性測試 |
| `T/domain/ProductPublicVisibilityPolicyTest.java` | `publicAndTemporarilyUnavailableProductsHaveNoVisibilityError`（`:22`） |  |  | 改寫：B4-3 Public 商品頁測試（不存在、未上架、已刪除的區分；可曝光 fail closed）；B4-1 特性測試 |
| `T/domain/ProductPublicVisibilityPolicyTest.java` | `missingDerivedStateFailsClosed`（`:28`） |  |  | 改寫：B4-3 Public 商品頁測試（不存在、未上架、已刪除的區分；可曝光 fail closed）；B4-1 特性測試 |
| `T/domain/ProductPublicationPolicyTest.java` | `firstActivationRecordsImmutablePublicationFact`（`:17`） |  |  | 改寫：B4-2 生命週期單元測試（`first_published_at` 只在首次上架寫入、下架保留商品頁、可曝光決定可購買，D16）；B4-1 特性測試 |
| `T/domain/ProductPublicationPolicyTest.java` | `nonActiveProductDoesNotInventPublicationHistory`（`:28`） |  |  | 改寫：B4-2 生命週期單元測試（`first_published_at` 只在首次上架寫入、下架保留商品頁、可曝光決定可購買，D16）；B4-1 特性測試 |
| `T/domain/ProductPublicationPolicyTest.java` | `sellerOffShelfPreservesPublicPageAsTemporarilyUnavailable`（`:38`） |  |  | 改寫：B4-2 生命週期單元測試（`first_published_at` 只在首次上架寫入、下架保留商品頁、可曝光決定可購買，D16）；B4-1 特性測試 |
| `T/domain/ProductPublicationPolicyTest.java` | `activeProductWithPublicationFactIsPublic`（`:45`） |  |  | 改寫：B4-2 生命週期單元測試（`first_published_at` 只在首次上架寫入、下架保留商品頁、可曝光決定可購買，D16）；B4-1 特性測試 |
| `T/domain/ProductPublicationPolicyTest.java` | `explicitRemovalWinsOverStatusAndStock`（`:52`） |  |  | 改寫：B4-2 生命週期單元測試（`first_published_at` 只在首次上架寫入、下架保留商品頁、可曝光決定可購買，D16）；B4-1 特性測試 |
| `T/domain/ProductPublicationPolicyTest.java` | `publicAvailabilitySeparatesStockFromUrlVisibility`（`:59`） |  |  | 改寫：B4-2 生命週期單元測試（`first_published_at` 只在首次上架寫入、下架保留商品頁、可曝光決定可購買，D16）；B4-1 特性測試 |
| `T/domain/ProductPublicationPolicyTest.java` | `nonExposableStoreHidesPurchaseButKeepsPublishedProductPage`（`:72`） |  |  | 改寫：B4-2 生命週期單元測試（`first_published_at` 只在首次上架寫入、下架保留商品頁、可曝光決定可購買，D16）；B4-1 特性測試 |
| `T/domain/ProductPublicationPolicyTest.java` | `storeExposableStateTable`（`:85`） |  |  | 改寫：B4-2 生命週期單元測試（`first_published_at` 只在首次上架寫入、下架保留商品頁、可曝光決定可購買，D16）；B4-1 特性測試 |
| `T/domain/ProductSellerCityFilterPolicyTest.java` | `usesAllCityStoresWhenNoOtherStoreFilterExists`（`:12`） |  |  | 刪：`seller_city_code` 篩選無 client |
| `T/domain/ProductSellerCityFilterPolicyTest.java` | `intersectsExplicitStoreFilterWithCompleteCityMembership`（`:18`） |  |  | 刪：`seller_city_code` 篩選無 client |
| `T/domain/ProductSellerCityFilterPolicyTest.java` | `returnsEmptyWhenCityHasNoMatchingStores`（`:24`） |  |  | 刪：`seller_city_code` 篩選無 client |
| `T/domain/ProductSellerCitySummaryPolicyTest.java` | `countsOnlyStoresWithPublicProductsAndKeepsPreviewBounded`（`:16`） |  |  | 刪：城市摘要無 client（D5） |
| `T/domain/ProductSellerCitySummaryPolicyTest.java` | `emptyStorefrontIsNeitherCountedNorPreviewed`（`:53`） | 有 0 件在架商品的店不計入 sellerCount，也不進 preview |  | 刪：城市摘要無 client（D5） |
| `T/domain/ProductSellerCitySummaryPolicyTest.java` | `explicitZeroCountsAsNoPublicProduct`（`:76`） |  |  | 刪：城市摘要無 client（D5） |
| `T/domain/ProductSellerCitySummaryPolicyTest.java` | `cityWhoseSellersAreAllEmptyStaysInSummaryWithZeroCounts`（`:93`） |  |  | 刪：城市摘要無 client（D5） |
| `T/domain/ProductSellerCitySummaryPolicyTest.java` | `cityWithNoSellersRemainsInSummaryWithZeroCounts`（`:108`） |  |  | 刪：城市摘要無 client（D5） |
| `T/domain/ProductSellerCitySummaryPolicyTest.java` | `malformedSellerStoreIdFailsClosedInsteadOfSilentlyUndercounting`（`:119`） |  |  | 刪：城市摘要無 client（D5） |
| `T/domain/text/DescriptionTextExtractorTest.java` | `extractList`（`:39`） | list block：依 item 順序串接每個 item 每個 run 的 text（含連結文字） |  | 改寫：B4-2 `searchable_text` 由描述區塊抽文字（D15、D17） |
| `T/domain/text/DescriptionTextExtractorTest.java` | `extractListEmpty`（`:51`） | list block：list_json 為空 / null 不納入、不拋例外 |  | 改寫：B4-2 `searchable_text` 由描述區塊抽文字（D15、D17） |
| `T/domain/text/DescriptionTextExtractorTest.java` | `extractListMalformed`（`:62`） | list block：list_json 格式錯誤容錯（後端 opaque，不阻斷） |  | 改寫：B4-2 `searchable_text` 由描述區塊抽文字（D15、D17） |
| `T/domain/text/DescriptionTextExtractorTest.java` | `extractMixed`（`:70`） | text / table / list / image 混合：依 blocks 順序串接、單一空格分隔 |  | 改寫：B4-2 `searchable_text` 由描述區塊抽文字（D15、D17） |
| `T/grpc/ProductGrpcServiceAdminCertificationProductLookupTest.java` | `adminDetailUsesIncludingDeletedLookup`（`:97`） | ADM 詳情補商品名走「含已軟刪」的查詢 | 刪 | 刪：範圍裁切（認證） |
| `T/grpc/ProductGrpcServiceAdminCertificationProductLookupTest.java` | `adminQueueListAlsoUsesIncludingDeletedLookup`（`:145`） | ADM 佇列列表也走「含已軟刪」的查詢——它是 #2818 最顯眼的 surface，且是另一個獨立 call site | 刪 | 刪：範圍裁切（認證） |
| `T/grpc/ProductGrpcServiceCreateSkuGuardTest.java` | `zeroSkusIsRejectedBeforeAnyWrite`（`:139`） | 零 SKU → INVALID_PARAM，且商品絕不能已經被建出來 |  | 改寫：B4-2 建立商品至少一個 SKU、失敗不留列（D18 沒有選項時是預設 SKU）；代行路徑在 B4-5d |
| `T/grpc/ProductGrpcServiceCreateSkuGuardTest.java` | `zeroSkusIsRejectedOnDelegatedPathToo`（`:153`） | 代行路徑走同一個守衛（收斂點不是只擋賣家本人） |  | 改寫：B4-2 建立商品至少一個 SKU、失敗不留列（D18 沒有選項時是預設 SKU）；代行路徑在 B4-5d |
| `T/grpc/ProductGrpcServiceCreateSkuGuardTest.java` | `oneSkuPassesTheGuard`（`:169`） | 控制組：帶一個 SKU 時確實建得成（證明上面兩測不是被別的必填檢查擋掉） |  | 改寫：B4-2 建立商品至少一個 SKU、失敗不留列（D18 沒有選項時是預設 SKU）；代行路徑在 B4-5d |
| `T/grpc/ProductGrpcServiceDelegationTest.java` | `getProduct_delegated_asksForOwnershipCheck`（`:88`） | 代行讀取單筆走 requireOwnership 分支（Q4），賣家本人路徑不走 |  | 改寫：B4-5d `CatalogDelegatedService` 測試（能力宣告、目標店家只來自 session、批次逐項成敗、FAILED 稽核）；`audit_log_id`、`version` 衝突、冪等鍵比對改由代行 interceptor、etag、`request_id` 承擔 |
| `T/grpc/ProductGrpcServiceDelegationTest.java` | `getProduct_delegated_notOwned_returns9275`（`:106`） | Q4 不符 ⇒ 9275（跨店讀取必須被擋，且不是靜默 200） |  | 改寫：B4-5d `CatalogDelegatedService` 測試（能力宣告、目標店家只來自 session、批次逐項成敗、FAILED 稽核）；`audit_log_id`、`version` 衝突、冪等鍵比對改由代行 interceptor、etag、`request_id` 承擔 |
| `T/grpc/ProductGrpcServiceDelegationTest.java` | `getProductList_delegated_withoutOwnerStoreId_returns9289`（`:125`） | 代行列表 owner_store_id 為零 ⇒ 9260（否則會落進公開列表分支拿到全站商品） |  | 改寫：B4-5d `CatalogDelegatedService` 測試（能力宣告、目標店家只來自 session、批次逐項成敗、FAILED 稽核）；`audit_log_id`、`version` 衝突、冪等鍵比對改由代行 interceptor、etag、`request_id` 承擔 |
| `T/grpc/ProductGrpcServiceDelegationTest.java` | `batchSnapshot_declaresProductsRead`（`:139`） | 代行讀取宣告 products.read；帶 products.publish 會被 S3 擋（批次第一跳的形狀） |  | 改寫：B4-5d `CatalogDelegatedService` 測試（能力宣告、目標店家只來自 session、批次逐項成敗、FAILED 稽核）；`audit_log_id`、`version` 衝突、冪等鍵比對改由代行 interceptor、etag、`request_id` 承擔 |
| `T/grpc/ProductGrpcServiceDelegationTest.java` | `batchUpdate_partialFailure_keepsTopLevelCodeZero`（`:184`） | 批次下架：逐項獨立成敗，頂層 code 恆為 0，audit_log_id 原樣回音 |  | 改寫：B4-5d `CatalogDelegatedService` 測試（能力宣告、目標店家只來自 session、批次逐項成敗、FAILED 稽核）；`audit_log_id`、`version` 衝突、冪等鍵比對改由代行 interceptor、etag、`request_id` 承擔 |
| `T/grpc/ProductGrpcServiceDelegationTest.java` | `batchUpdate_itemWithoutAuditLogId_rejectsWholeBatch`（`:233`） | 批次任一項缺 audit_log_id ⇒ 整批 9260（S1b 在批次落在逐項上） |  | 改寫：B4-5d `CatalogDelegatedService` 測試（能力宣告、目標店家只來自 session、批次逐項成敗、FAILED 稽核）；`audit_log_id`、`version` 衝突、冪等鍵比對改由代行 interceptor、etag、`request_id` 承擔 |
| `T/grpc/ProductGrpcServiceDelegationTest.java` | `batchUpdate_duplicateProductId_rejected`（`:253`） | 批次 product_id 重複 ⇒ 整批拒，一項都不執行 |  | 改寫：B4-5d `CatalogDelegatedService` 測試（能力宣告、目標店家只來自 session、批次逐項成敗、FAILED 稽核）；`audit_log_id`、`version` 衝突、冪等鍵比對改由代行 interceptor、etag、`request_id` 承擔 |
| `T/grpc/ProductGrpcServiceDelegationTest.java` | `updateSellerProductStatus_wrongCapability_denied`（`:274`） | 寫入路徑 capability 不符 ⇒ 9262，且不進 Service |  | 改寫：B4-5d `CatalogDelegatedService` 測試（能力宣告、目標店家只來自 session、批次逐項成敗、FAILED 稽核）；`audit_log_id`、`version` 衝突、冪等鍵比對改由代行 interceptor、etag、`request_id` 承擔 |
| `T/grpc/ProductGrpcServiceDelegationTest.java` | `updateSellerProductStatus_withoutAuditLogId_denied`（`:294`） | 寫入路徑缺 audit_log_id ⇒ 9260（S1b 對 .publish 必驗） |  | 改寫：B4-5d `CatalogDelegatedService` 測試（能力宣告、目標店家只來自 session、批次逐項成敗、FAILED 稽核）；`audit_log_id`、`version` 衝突、冪等鍵比對改由代行 interceptor、etag、`request_id` 承擔 |
| `T/grpc/ProductGrpcServiceDelegationTest.java` | `updateSellerProductStatus_versionConflict_returns9266`（`:311`） | 樂觀鎖衝突 ⇒ 9266（MyBatis-Plus 回 0 列不拋例外，不判就是 200 但沒寫） |  | 改寫：B4-5d `CatalogDelegatedService` 測試（能力宣告、目標店家只來自 session、批次逐項成敗、FAILED 稽核）；`audit_log_id`、`version` 衝突、冪等鍵比對改由代行 interceptor、etag、`request_id` 承擔 |
| `T/grpc/ProductGrpcServiceDelegationTest.java` | `updateProduct_idempotencyKeyMismatch_denied`（`:333`） | request 與代行語境的冪等鍵不同 ⇒ 9260（S5） |  | 改寫：B4-5d `CatalogDelegatedService` 測試（能力宣告、目標店家只來自 session、批次逐項成敗、FAILED 稽核）；`audit_log_id`、`version` 衝突、冪等鍵比對改由代行 interceptor、etag、`request_id` 承擔 |
| `T/grpc/ProductGrpcServiceGetProductOwnerTest.java` | `getProduct_usesRequestStoreId_notGrpcAuthContext`（`:55`） | 擁有權判斷讀 request.store_id（重開店 surrogate），忽略 GrpcAuthContext 的原始 JWT accountId |  | 改寫：B4-3 賣家讀取只用呼叫者推導的店家（D4，修 CD9）；跨店回 not_found |
| `T/grpc/ProductGrpcServiceStoresWithPublicProductsTest.java` | `returnsStoreIdsAsNumericStrings`（`:70`） | 回傳的 store_id 以數字字串輸出，順序與來源一致 |  | 改寫：B4-4 模組 API 測試（空集合是成功、查詢失敗回錯誤） |
| `T/grpc/ProductGrpcServiceStoresWithPublicProductsTest.java` | `emptySetIsASuccessfulAnswer`（`:84`） | 平台上沒有任何公開商品時回空集合且 code=0（與失敗可分辨） |  | 改寫：B4-4 模組 API 測試（空集合是成功、查詢失敗回錯誤） |
| `T/grpc/ProductGrpcServiceStoresWithPublicProductsTest.java` | `failureMustNotDegradeToEmptySet`（`:97`） | 查詢失敗必須回非零 code，不得降級成 code=0 的空集合 |  | 改寫：B4-4 模組 API 測試（空集合是成功、查詢失敗回錯誤） |
| `T/grpc/ProductGrpcServiceVerifyCertificationGuardTest.java` | `invalidCertNoIsRejectedBeforeReachingGovApi`（`:142`） | 含非白名單字元的 cert_no → INVALID_PARAM，且絕不能已經送去組 URL | 刪 | 刪：範圍裁切（認證） |
| `T/grpc/ProductGrpcServiceVerifyCertificationGuardTest.java` | `overlongCertNoIsRejected`（`:155`） | 超出長度上限的 cert_no 同樣擋下 | 刪 | 刪：範圍裁切（認證） |
| `T/grpc/ProductGrpcServiceVerifyCertificationGuardTest.java` | `blankCertNoKeepsExistingCode`（`:167`） | 空 cert_no 維持既有的 CERT_NO_REQUIRED，不被新守衛蓋掉 | 刪 | 刪：範圍裁切（認證） |
| `T/grpc/ProductGrpcServiceVerifyCertificationGuardTest.java` | `validCertNoStillPassesThrough`（`:178`） | 合法 cert_no 仍照常送去驗證（守衛不得過度收緊） | 刪 | 刪：範圍裁切（認證） |
| `T/grpc/ProductGrpcServiceVerifyCertificationGuardTest.java` | `nonVerifiableTypeIsNotRejectedByWhitelist`（`:193`） | 不可線上驗證的類型：帶特殊字元的 cert_no 仍回 3003，不被白名單改判成 400 | 刪 | 刪：範圍裁切（認證） |
| `T/integration/StoreExposureUpsertIT.java` | `missingRowIsNotExposable`（`:36`） | 沒有列：isExposable 為 false |  | 刪：不建 `store_exposure`，改 join `seller.pub_exposable_stores(at)` |
| `T/integration/StoreExposureUpsertIT.java` | `versionRules`（`:42`） | 版本 5 寫入後收到版本 3 → 列不變；收到版本 5 重送 → 不變；版本 6 → 更新 |  | 刪：不建 `store_exposure`，改 join `seller.pub_exposable_stores(at)` |
| `T/integration/StoreExposureUpsertIT.java` | `versionsAreScopedPerStore`（`:60`） | 首次寫入 exposable=true 後 isExposable 為 true；版本只在同一家店之間比較 |  | 刪：不建 `store_exposure`，改 join `seller.pub_exposable_stores(at)` |
| `T/integration/StoreExposureUpsertIT.java` | `suspensionSnapshotTableIsDropped`（`:71`） | product_suspension_snapshot 已刪除 |  | 刪：不建該表 |
| `T/integration/StoreExposureVisibilityIT.java` | `publicListHidesNonExposableStores`（`:110`） | 公開列表：只含可曝光店的 ACTIVE 商品，總數與分頁都以過濾後為準；沒有 store_exposure 列的店同樣隱藏 |  | 改寫：B4-3 integration（公開列表、總數、熱門與自動完成 hashtag、商品頁、批次卡片只算可曝光店的 active 商品；賣家與 admin 視角不過濾）；mutation `b4-3.json` 拿掉 join |
| `T/integration/StoreExposureVisibilityIT.java` | `ownerViewIsNotFilteredByExposure`（`:125`） | 賣家看自己商品／admin 視角（不要求可曝光）：不可曝光的店的商品仍在列表與總數內 |  | 改寫：B4-3 integration（公開列表、總數、熱門與自動完成 hashtag、商品頁、批次卡片只算可曝光店的 active 商品；賣家與 admin 視角不過濾）；mutation `b4-3.json` 拿掉 join |
| `T/integration/StoreExposureVisibilityIT.java` | `exposureFlipChangesListImmediately`（`:133`） | 店家由可曝光翻成不可曝光後，列表與總數立即不含它的商品 |  | 改寫：B4-3 integration（公開列表、總數、熱門與自動完成 hashtag、商品頁、批次卡片只算可曝光店的 active 商品；賣家與 admin 視角不過濾）；mutation `b4-3.json` 拿掉 join |
| `T/integration/StoreExposureVisibilityIT.java` | `summariesCountOnlyExposableStores`（`:144`） | 產地摘要與店家公開商品數只計可曝光店的 ACTIVE 商品 |  | 改寫：B4-3 integration（店家公開商品數只算可曝光店）；產地摘要無 client 刪（D5） |
| `T/integration/StoreExposureVisibilityIT.java` | `popularHashtagsCountOnlyExposableStores`（`:157`） | 熱門 hashtag 只計可曝光店的 ACTIVE 商品 |  | 改寫：B4-3 integration（公開列表、總數、熱門與自動完成 hashtag、商品頁、批次卡片只算可曝光店的 active 商品；賣家與 admin 視角不過濾）；mutation `b4-3.json` 拿掉 join |
| `T/integration/StoreExposureVisibilityIT.java` | `briefCarriesExposureIntoPolicy`（`:174`） | brief 一次查詢帶回 storeExposable，Policy 對不可曝光或沒有列的店回 TEMPORARILY_UNAVAILABLE |  | 改寫：B4-3 integration（公開列表、總數、熱門與自動完成 hashtag、商品頁、批次卡片只算可曝光店的 active 商品；賣家與 admin 視角不過濾）；mutation `b4-3.json` 拿掉 join |
| `T/integration/StoreExposureVisibilityIT.java` | `publicDetailReflectsExposure`（`:189`） | 公開詳情：不可曝光的店（含沒有列）回 TEMPORARILY_UNAVAILABLE 且不可購買；可曝光的店 PUBLIC 且可購買 |  | 改寫：B4-3 integration（公開列表、總數、熱門與自動完成 hashtag、商品頁、批次卡片只算可曝光店的 active 商品；賣家與 admin 視角不過濾）；mutation `b4-3.json` 拿掉 join |
| `T/integration/StoreExposureVisibilityIT.java` | `hashtagAutocompleteCountsOnlyExposableStores`（`:208`） | hashtag 自動完成：數量在讀取時只計可曝光店的 ACTIVE 商品，不用預先計算的 product_count |  | 改寫：B4-3 integration（公開列表、總數、熱門與自動完成 hashtag、商品頁、批次卡片只算可曝光店的 active 商品；賣家與 admin 視角不過濾）；mutation `b4-3.json` 拿掉 join |
| `T/integration/StoreExposureVisibilityIT.java` | `hashtagItemCountExcludesNonExposableStores`（`:231`） | 商品詳情／列表的 hashtag 數量在讀取時計算：不含不可曝光店與非 ACTIVE 的商品，沒有符合商品時為 0 |  | 改寫：B4-3 integration（公開列表、總數、熱門與自動完成 hashtag、商品頁、批次卡片只算可曝光店的 active 商品；賣家與 admin 視角不過濾）；mutation `b4-3.json` 拿掉 join |
| `T/integration/StoreExposureVisibilityIT.java` | `getProductsByIdsHidesNonExposableStoresForBuyers`（`:258`） | getProductsByIds：includeInactive=false 只回可曝光店的 ACTIVE 商品；true 不受影響 |  | 改寫：B4-3 integration（公開列表、總數、熱門與自動完成 hashtag、商品頁、批次卡片只算可曝光店的 active 商品；賣家與 admin 視角不過濾）；mutation `b4-3.json` 拿掉 join |
| `T/migration/RestrictedCategoryCodesSchemaContractTest.java` | `註解剝除器本身有效`（`:111`） | 剝除器本身有效：只出現在註解裡的字串不得進入 CODE |  | 刪：`restricted_category_codes` 不帶，受管制品類不放進分類（D8） |
| `T/migration/RestrictedCategoryCodesSchemaContractTest.java` | `欄位存在`（`:123`） | 欄位存在且型別／預設值正確（66 §5.1.9 ② 逐字） |  | 刪：`restricted_category_codes` 不帶，受管制品類不放進分類（D8） |
| `T/migration/RestrictedCategoryCodesSchemaContractTest.java` | `值域未定案時只允許空陣列`（`:132`） | 值域未定案 ⇒ CHECK 把它鎖成 fail-closed（沒有碼表 = 寫不進去） |  | 刪：`restricted_category_codes` 不帶，受管制品類不放進分類（D8） |
| `T/migration/RestrictedCategoryCodesSchemaContractTest.java` | `不得有回填UPDATE`（`:145`） | 不回填既有列（66 §5.4 逐字：空陣列即為正確初值） |  | 刪：`restricted_category_codes` 不帶，受管制品類不放進分類（D8） |
| `T/migration/RestrictedCategoryCodesSchemaContractTest.java` | `欄位有註解`（`:153`） | 欄位有 COMMENT，把『值域待 UP-4』留在 DB 裡而不只留在 repo 裡 |  | 刪：`restricted_category_codes` 不帶，受管制品類不放進分類（D8） |
| `T/migration/RestrictedCategoryCodesSchemaContractTest.java` | `entity已接線`（`:163`） | entity 側真的接上了：欄位存在且指名對應的 TypeHandler |  | 刪：`restricted_category_codes` 不帶，受管制品類不放進分類（D8） |
| `T/migration/RestrictedCategoryCodesSchemaContractTest.java` | `typeHandler未污染全域註冊表`（`:177`） | TypeHandler 不得標 @MappedTypes(List.class)（Java 微服務開發規範 §9.1） |  | 刪：`restricted_category_codes` 不帶，受管制品類不放進分類（D8） |
| `T/mq/CertificationVerifyConsumerTest.java` | `verifiedAndAutoApprovable_approves`（`:64`） | SUBMITTED ＋ 查驗有效 ＋ 免人工 → 回寫後自動核准並 ack | 刪 | 刪：範圍裁切（認證） |
| `T/mq/CertificationVerifyConsumerTest.java` | `duplicateMessageAfterApproval_isNoOp`（`:86`） | 重複訊息（申請已核准）→ 不查驗、不核准、直接 ack | 刪 | 刪：範圍裁切（認證） |
| `T/mq/CertificationVerifyConsumerTest.java` | `sameMessageTwice_approvesOnce`（`:100`） | 同一則訊息連投兩次 → 只核准一次 | 刪 | 刪：範圍裁切（認證） |
| `T/mq/CertificationVerifyConsumerTest.java` | `manualReviewRequired_doesNotApprove`（`:130`） | requiresManualReview=true ＋ 查驗有效 → 只回寫，不自動核准 | 刪 | 刪：範圍裁切（認證） |
| `T/mq/CertificationVerifyConsumerTest.java` | `govUnavailable_doesNotApprove`（`:147`） | 查驗結果 UNAVAILABLE → 不自動核准 | 刪 | 刪：範圍裁切（認證） |
| `T/mq/CertificationVerifyConsumerTest.java` | `govNotFound_doesNotApprove`（`:162`） | 查驗結果 NOT_FOUND → 不自動核准（留給 ADM 裁決） | 刪 | 刪：範圍裁切（認證） |
| `T/mq/CertificationVerifyConsumerTest.java` | `applicationMissing_acksAndSkips`（`:177`） | 申請不存在 → 丟棄（ack），不查驗 | 刪 | 刪：範圍裁切（認證） |
| `T/mq/CertificationVerifyConsumerTest.java` | `missingApplicationId_acksAndSkips`（`:189`） | payload 缺 application_id → 丟棄（ack），不進 retry | 刪 | 刪：範圍裁切（認證） |
| `T/mq/CertificationVerifyConsumerTest.java` | `verifyThrows_goesToRetry`（`:204`） | 查驗拋例外 → 走 MqErrorHandler 重投，不靜默吞掉 | 刪 | 刪：範圍裁切（認證） |
| `T/mq/CertificationVerifyConsumerTest.java` | `verifyThrowsAtMaxRetry_goesToDlq`（`:219`） | 重試已達上限時查驗再失敗 → reject 進 DLQ | 刪 | 刪：範圍裁切（認證） |
| `T/mq/ProductCertificationEventGoldenTest.java` | `approvedMatchesGolden`（`:44`） | approved payload 的 wire key 與 golden 完全一致（不多不少） | 刪 | 刪：範圍裁切（認證） |
| `T/mq/ProductCertificationEventGoldenTest.java` | `rejectedMatchesGolden`（`:51`） | rejected payload 的 wire key 與 golden 完全一致（含 reason_code／public_reason） | 刪 | 刪：範圍裁切（認證） |
| `T/mq/ProductCertificationEventGoldenTest.java` | `revokedMatchesGolden`（`:58`） | revoked payload 的 wire key 與 golden 完全一致（含 reason_code／public_reason） | 刪 | 刪：範圍裁切（認證） |
| `T/mq/ProductCertificationEventGoldenTest.java` | `neverEmitsInternalNote`（`:69`） | 裁決 payload 永遠不含 internal_note | 刪 | 刪：範圍裁切（認證） |
| `T/mq/ProductCertificationEventGoldenTest.java` | `verifyRequestedWireShape`（`:86`） | verify.requested 的 wire key 為 snake_case 且 id 皆為 JSON 字串 | 刪 | 刪：範圍裁切（認證） |
| `T/mq/SalesAccumulatedConsumerTest.java` | `firstEventAckAndEvict`（`:57`） | 首次事件 → accumulator 累計 + ack + evict cache |  | 改寫：B4-4 `catalogconsumer` 測試（首次累計、重送不累計、缺 `order_no` 進 DLQ） |
| `T/mq/SalesAccumulatedConsumerTest.java` | `duplicateEventAckNoEvict`（`:73`） | 重複事件（applyAll=false）→ ack、不 evict |  | 改寫：B4-4 `catalogconsumer` 測試（首次累計、重送不累計、缺 `order_no` 進 DLQ） |
| `T/mq/SalesAccumulatedConsumerTest.java` | `missingOrderNoRejectsToDlq`（`:86`） | 缺 order_no → reject 進 DLQ、不累計 |  | 改寫：B4-4 `catalogconsumer` 測試（首次累計、重送不累計、缺 `order_no` 進 DLQ） |
| `T/mq/SalesAccumulatorTest.java` | `firstEventAccumulates`（`:41`） | 首次事件：claim 成功 → 逐筆累計、回傳 true |  | 改寫：B4-4 銷量入口 `order_no` 冪等（mutation `b4-3.json` 重送測試） |
| `T/mq/SalesAccumulatorTest.java` | `duplicateEventSkips`（`:53`） | 重複事件：claim NO-OP → 不累計、回傳 false |  | 改寫：B4-4 銷量入口 `order_no` 冪等（mutation `b4-3.json` 重送測試） |
| `T/mq/SalesAccumulatorTest.java` | `sameOrderNoTwiceAccumulatesOnce`（`:64`） | 同 order_no 連續兩次：sales_count 只累計一次 |  | 改寫：B4-4 銷量入口 `order_no` 冪等（mutation `b4-3.json` 重送測試） |
| `T/mq/SellerDeletedConsumerTest.java` | `newFormatUsesStoreId`（`:42`） | 新格式：帶 store_id → 以 surrogate 下架 + ack |  | 刪：不訂閱 `seller.deleted`，可見性讀 `seller.pub_exposable_stores(at)`（D7） |
| `T/mq/SellerDeletedConsumerTest.java` | `rolloutWindowFallsBackToLegacySellerAccountId`（`:52`） | rollout-window 相容格式：無 store_id 但帶舊鍵 sellerAccountId → 仍以 surrogate 下架 + ack |  | 刪：不訂閱 `seller.deleted`，可見性讀 `seller.pub_exposable_stores(at)`（D7） |
| `T/mq/SellerDeletedConsumerTest.java` | `oldFormatFallsBackToAccountId`（`:62`） | 舊格式：store_id 與 sellerAccountId 皆無 → fallback accountId（in-flight 舊事件） |  | 刪：不訂閱 `seller.deleted`，可見性讀 `seller.pub_exposable_stores(at)`（D7） |
| `T/mq/SellerDeletedConsumerTest.java` | `missingBothIdsAcksWithoutOffShelf`（`:71`） | 兩者皆缺 → WARN + ack，不下架 |  | 刪：不訂閱 `seller.deleted`，可見性讀 `seller.pub_exposable_stores(at)`（D7） |
| `T/mq/SellerExposureChangedConsumerTest.java` | `contractBodyIsApplied`（`:50`） | 契約的 JSON 逐字：解析成功，寫入 (123, true, 7) 並 ack |  | 刪：不訂閱 `seller.exposure.changed`（B4-6 刪管線） |
| `T/mq/SellerExposureChangedConsumerTest.java` | `stringIdsAreAccepted`（`:58`） | store_id 與 exposure_version 為 string 也可 |  | 刪：不訂閱 `seller.exposure.changed`（B4-6 刪管線） |
| `T/mq/SellerExposureChangedConsumerTest.java` | `staleEventIsAcked`（`:66`） | 舊版本或重送（apply 回 false）仍 ack |  | 刪：不訂閱 `seller.exposure.changed`（B4-6 刪管線） |
| `T/mq/SellerExposureChangedConsumerTest.java` | `missingVersionGoesToDlq`（`:75`） | 缺 exposure_version → nack 進 DLQ，不寫入 |  | 刪：不訂閱 `seller.exposure.changed`（B4-6 刪管線） |
| `T/mq/SellerExposureChangedConsumerTest.java` | `missingExposableGoesToDlq`（`:83`） | 缺 exposable → nack 進 DLQ |  | 刪：不訂閱 `seller.exposure.changed`（B4-6 刪管線） |
| `T/mq/SellerExposureChangedConsumerTest.java` | `malformedBodiesGoToDlq`（`:91`） | 缺 store_id、exposable 非 boolean、版本非數字、store_id 非正數、event_type 不符、非 JSON → 全部 nack |  | 刪：不訂閱 `seller.exposure.changed`（B4-6 刪管線） |
| `T/mq/SellerExposureChangedConsumerTest.java` | `transientFailureIsRetriedNotDeadLettered`（`:113`） | 暫時性失敗（DB 例外）：第一次失敗不進 DLQ，ack 原訊息並重新發布重試（x-retry-count=1） |  | 刪：不訂閱 `seller.exposure.changed`（B4-6 刪管線） |
| `T/mq/SellerExposureChangedConsumerTest.java` | `exhaustedRetriesGoToDlq`（`:130`） | 重試用盡（x-retry-count=3）：basicReject 進 DLQ |  | 刪：不訂閱 `seller.exposure.changed`（B4-6 刪管線） |
| `T/service/CategoryServiceRevalidateTest.java` | `createCategoryRevalidatesNewCategory`（`:65`） | 新增分類：發 category:<新 id>（新分類進入分類樹） |  | 刪：分類不做即時 CRUD（D8），web 快取 revalidate 隨 REST 退場 |
| `T/service/CategoryServiceRevalidateTest.java` | `createCategoryAutoApplyDoesNotRevalidate`（`:83`） | 自動套用既有 ACTIVE 分類：什麼都沒改，不得 bust |  | 刪：分類不做即時 CRUD（D8），web 快取 revalidate 隨 REST 退場 |
| `T/service/CategoryServiceRevalidateTest.java` | `updateCategoryNameRevalidatesCategoryWithProducts`（`:97`） | 分類改名：category + cascade 受影響 product 走單次合併 POST |  | 刪：分類不做即時 CRUD（D8），web 快取 revalidate 隨 REST 退場 |
| `T/service/CategoryServiceRevalidateTest.java` | `updateCategoryNameWithNoProductsStillRevalidatesCategory`（`:116`） | 分類改名但底下無商品：仍要發 category（分類樹本身變了） |  | 刪：分類不做即時 CRUD（D8），web 快取 revalidate 隨 REST 退場 |
| `T/service/CategoryServiceRevalidateTest.java` | `updateCategoryNameToSameNameDoesNotRevalidate`（`:133`） | 改名成相同名稱：no-op，不得 bust |  | 刪：分類不做即時 CRUD（D8），web 快取 revalidate 隨 REST 退場 |
| `T/service/CategoryServiceRevalidateTest.java` | `softDeleteRevalidatesDeletedCategories`（`:145`） | 批次軟刪除：發被刪的每個 category（分類離開分類樹） |  | 刪：分類不做即時 CRUD（D8），web 快取 revalidate 隨 REST 退場 |
| `T/service/CategoryServiceRevalidateTest.java` | `restoreRevalidatesCategory`（`:162`） | 還原分類：發 category:<id>（分類回到分類樹） |  | 刪：分類不做即時 CRUD（D8），web 快取 revalidate 隨 REST 退場 |
| `T/service/GovCertificationServiceDetailedTest.java` | `badApiKeyThrowsUnavailable`（`:183`） | 金鑰失效（RS=ERROR、無 Data 欄）→ 拋不可用，且不得寫入任何快取 | 刪 | 刪：範圍裁切（認證） |
| `T/service/GovCertificationServiceDetailedTest.java` | `rsErrorWithEmptyDataIsStillUnavailable`（`:201`） | RS=ERROR 但帶空 Data：仍必須是不可用，不得塌成 NOT_FOUND | 刪 | 刪：範圍裁切（認證） |
| `T/service/GovCertificationServiceDetailedTest.java` | `missingRsIsUnavailable`（`:212`） | RS 欄位缺失同樣視為不可用 | 刪 | 刪：範圍裁切（認證） |
| `T/service/GovCertificationServiceDetailedTest.java` | `emptyDataIsNotFound`（`:221`） | RS=OK ＋ Data 空陣列才是真正的「查無」 | 刪 | 刪：範圍裁切（認證） |
| `T/service/GovCertificationServiceDetailedTest.java` | `echoMismatchThrows`（`:247`） | 回傳列是別人的編號（＝參數名被忽略回傳整個資料集）→ 拋不可用，不得判成有效 | 刪 | 刪：範圍裁切（認證） |
| `T/service/GovCertificationServiceDetailedTest.java` | `missingEchoFieldThrows`（`:258`） | 回聲欄位整個缺席 → 拋不可用（不得當成「沒有就算過」） | 刪 | 刪：範圍裁切（認證） |
| `T/service/GovCertificationServiceDetailedTest.java` | `anyMismatchedRowThrows`（`:267`） | 多列中只要有一列編號不符就拋（不得只查第一列） | 刪 | 刪：範圍裁切（認證） |
| `T/service/GovCertificationServiceDetailedTest.java` | `eachTypeChecksItsOwnEchoField`（`:281`） | 三支各自比對自己的回聲欄位（3702 Tracecode / 3703 Emblem_ID） | 刪 | 刪：範圍裁切（認證） |
| `T/service/GovCertificationServiceDetailedTest.java` | `matchingEchoPasses`（`:293`） | 編號一致時正常放行（防線不得過度收緊） | 刪 | 刪：範圍裁切（認證） |
| `T/service/GovCertificationServiceDetailedTest.java` | `tapPayloadIsTrimmed`（`:316`） | 3702：剔除 TraceCodelist / StoreInfo / ParentTraceCode，ProcessDetail 截前 5 筆 | 刪 | 刪：範圍裁切（認證） |
| `T/service/GovCertificationServiceDetailedTest.java` | `casRowsAreCappedButCountIsHonest`（`:342`） | 3703：列數截到上限，但 matchedCount 仍回報截斷前的真實筆數 | 刪 | 刪：範圍裁切（認證） |
| `T/service/GovCertificationServiceDetailedTest.java` | `organicTerminatedIsNotVerified`（`:366`） | 3701 Status=結束（已失效）→ TERMINATED，且 exists=false | 刪 | 刪：範圍裁切（認證） |
| `T/service/GovCertificationServiceDetailedTest.java` | `organicOtherNegativeStatuses`（`:381`） | 3701 Status=終止 → TERMINATED；Status=暫終 → SUSPENDED | 刪 | 刪：範圍裁切（認證） |
| `T/service/GovCertificationServiceDetailedTest.java` | `organicPassedButExpired`（`:393`） | 3701 Status=通過 但效期已過 → EXPIRED（實測 371 筆結束中有 292 筆過期，過期資料照樣回傳） | 刪 | 刪：範圍裁切（認證） |
| `T/service/GovCertificationServiceDetailedTest.java` | `organicValid`（`:404`） | 3701 Status=通過 ＋ 效期未過 → VALID | 刪 | 刪：範圍裁切（認證） |
| `T/service/GovCertificationServiceDetailedTest.java` | `organicUnknownStatus`（`:416`） | 3701 狀態值不在已知值域 → UNKNOWN（判讀不出來不等於無效，也不等於有效） | 刪 | 刪：範圍裁切（認證） |
| `T/service/GovCertificationServiceDetailedTest.java` | `tapUsesValidDateOnly`（`:431`） | 3702 無狀態欄：ValidDate 未過 → VALID，已過 → EXPIRED | 刪 | 刪：範圍裁切（認證） |
| `T/service/GovCertificationServiceDetailedTest.java` | `casFoundIsValid`（`:447`） | 3703 無狀態亦無效期：found 即 VALID，statusRaw / expiresOn 皆為 null | 刪 | 刪：範圍裁切（認證） |
| `T/service/GovCertificationServiceDetailedTest.java` | `organicMultiRowTakesBest`（`:460`） | 3701 兩列（同證書不同品項）取最有利的一列作為整體結論 | 刪 | 刪：範圍裁切（認證） |
| `T/service/GovCertificationServiceDetailedTest.java` | `organicExpiresOnTaipeiCalendarNotUtc`（`:517`） | 3701：台北已跨日、UTC 未跨日 → EXPIRED（用 UTC 會誤判成 VALID） | 刪 | 刪：範圍裁切（認證） |
| `T/service/GovCertificationServiceDetailedTest.java` | `tapExpiresOnTaipeiCalendarNotUtc`（`:529`） | 3702：台北已跨日、UTC 未跨日 → EXPIRED（ValidDate 走同一支效期比較） | 刪 | 刪：範圍裁切（認證） |
| `T/service/GovCertificationServiceDetailedTest.java` | `organicStillValidOnLastTaipeiDay`（`:546`） | 3701：台北到期日當天 23:30 仍是 VALID（效期當日有效，不得提早一天作廢） | 刪 | 刪：範圍裁切（認證） |
| `T/service/GovCertificationServiceDetailedTest.java` | `validUsesLongTtl`（`:564`） | VALID → TTL 7 天，且寫的是結構化 JSON 不是 \ | 刪 | 刪：範圍裁切（認證） |
| `T/service/GovCertificationServiceDetailedTest.java` | `notFoundUsesShortTtl`（`:577`） | NOT_FOUND → TTL 1 小時（打錯字不該被自己的錯誤快取擋住一整天） | 刪 | 刪：範圍裁切（認證） |
| `T/service/GovCertificationServiceDetailedTest.java` | `invalidStatesUseMediumTtl`（`:588`） | EXPIRED / TERMINATED / SUSPENDED → TTL 1 天 | 刪 | 刪：範圍裁切（認證） |
| `T/service/GovCertificationServiceDetailedTest.java` | `unknownIsNeverCached`（`:603`） | UNKNOWN → 完全不寫快取 | 刪 | 刪：範圍裁切（認證） |
| `T/service/GovCertificationServiceDetailedTest.java` | `cacheRoundTrip`（`:613`） | 寫進去的 JSON 讀回來要還原成同一個結論（背景寫、同步讀，跨路徑的唯一介面） | 刪 | 刪：範圍裁切（認證） |
| `T/service/GovCertificationServiceDetailedTest.java` | `readCachedResultNeverCallsGovApi`（`:632`） | readCachedResult 一律不發外部呼叫（§8.2 的結構保證） | 刪 | 刪：範圍裁切（認證） |
| `T/service/GovCertificationServiceDetailedTest.java` | `unsupportedTypesShortCircuit`（`:642`） | 不支援線上查驗的類型（3704/3705/3706）→ 直接回 null，不查 Redis 也不打 API | 刪 | 刪：範圍裁切（認證） |
| `T/service/GovCertificationServiceDetailedTest.java` | `corruptCacheIsTreatedAsMiss`（`:651`） | 快取內容壞掉 → 視為未命中，不得讓壞快取變成查驗結論 | 刪 | 刪：範圍裁切（認證） |
| `T/service/GovCertificationServiceDetailedTest.java` | `backgroundBypassesOpenCircuit`（`:670`） | circuit 開著時：同步路徑短路，背景路徑照打不誤 | 刪 | 刪：範圍裁切（認證） |
| `T/service/GovCertificationServiceDetailedTest.java` | `backgroundFailureDoesNotOpenCircuit`（`:689`） | 背景查驗失敗不得打開同步 circuit | 刪 | 刪：範圍裁切（認證） |
| `T/service/GovCertificationServiceDetailedTest.java` | `timeoutsDiffer`（`:701`） | 背景用 90 秒 read-timeout，同步維持 10 秒 | 刪 | 刪：範圍裁切（認證） |
| `T/service/GovCertificationServiceDetailedTest.java` | `syncTimeoutUnchanged`（`:715`） | 同步路徑仍是 10 秒（使用者在等畫面，不能讓他等 90 秒） | 刪 | 刪：範圍裁切（認證） |
| `T/service/GovCertificationServiceDetailedTest.java` | `forceRefreshNeverReadsResultCache`（`:748`） | 🔴 快取有值也照打政府 API，而且連讀都不讀那把 key | 刪 | 刪：範圍裁切（認證） |
| `T/service/GovCertificationServiceDetailedTest.java` | `backgroundPathStillHitsTheSameCache`（`:775`） | 對照組：同一份快取，背景路徑照樣命中並回舊答案（證明 stub 真的有效） | 刪 | 刪：範圍裁切（認證） |
| `T/service/GovCertificationServiceDetailedTest.java` | `forceRefreshStillWritesCache`（`:797`） | 仍然回寫快取（繞的是讀不是寫），否則同步路徑會繼續吃七天前的舊值 | 刪 | 刪：範圍裁切（認證） |
| `T/service/GovCertificationServiceDetailedTest.java` | `forceRefreshKeepsBackgroundTimeoutAndBypassesCircuit`（`:815`） | 沿用背景路徑的 90 秒 timeout，且 circuit 開著也照打 | 刪 | 刪：範圍裁切（認證） |
| `T/service/GovCertificationServiceDetailedTest.java` | `redisOutageDoesNotBreakForceRefresh`（`:833`） | Redis 失聯不影響：本來就不讀，寫不進去也只是少一次快取 | 刪 | 刪：範圍裁切（認證） |
| `T/service/GovCertificationServiceDetailedTest.java` | `legacyVerifySharesNewPrefix`（`:848`） | verify() 走的是 verifyDetailed 的內部實作：讀寫同一個新前綴 | 刪 | 刪：範圍裁切（認證） |
| `T/service/GovCertificationServiceDetailedTest.java` | `queryParamAndEchoFieldShareOneSource`（`:861`） | 查詢參數名與防線 2 的回聲欄位同源：參數名打錯時校驗欄位也會跟著錯，防線就白做了 | 刪 | 刪：範圍裁切（認證） |
| `T/service/GovCertificationServiceTest.java` | `acceptsWhitelistedCertNo`（`:131`） | 合法 cert_no（英數 / 連字號 / 底線）一律放行 | 刪 | 刪：範圍裁切（認證） |
| `T/service/GovCertificationServiceTest.java` | `rejectsNonWhitelistedCertNo`（`:152`） | 含非白名單字元的 cert_no 一律拒絕 | 刪 | 刪：範圍裁切（認證） |
| `T/service/GovCertificationServiceTest.java` | `rejectsNullAndOverlongCertNo`（`:158`） | null 與超過長度上限的 cert_no 也要拒絕 | 刪 | 刪：範圍裁切（認證） |
| `T/service/GovCertificationServiceTest.java` | `invalidCertNoRejectedBeforeCallingGovApi`（`:166`） | 非法 cert_no 在打政府 API 之前就被擋下，且不得誤觸 circuit breaker | 刪 | 刪：範圍裁切（認證） |
| `T/service/GovCertificationServiceTest.java` | `rejectionMessageDoesNotLeakApiKey`（`:179`） | 非法 cert_no 被拒時，例外訊息本身不得帶出 api_key | 刪 | 刪：範圍裁切（認證） |
| `T/service/GovCertificationServiceTest.java` | `catchAllMustNotLeakApiKey`（`:192`） | catch-all 分支：底層例外訊息含 api_key 時，不得把整個例外物件交給 logger | 刪 | 刪：範圍裁切（認證） |
| `T/service/GovCertificationServiceTest.java` | `catchAllStillOpensCircuitAndThrowsUnavailable`（`:211`） | catch-all 仍維持既有語意：轉成 GovApiUnavailableException 並觸發 circuit breaker | 刪 | 刪：範圍裁切（認證） |
| `T/service/GovCertificationServiceTest.java` | `validCertNoStillReachesGovApi`（`:231`） | 合法 cert_no 正常查到有效資料時仍回傳 true | 刪 | 刪：範圍裁切（認證） |
| `T/service/GovCertificationServiceTest.java` | `terminatedCertIsNotVerified`（`:251`） | Status=結束（已失效）→ 不算 VERIFIED，即使查得到資料 | 刪 | 刪：範圍裁切（認證） |
| `T/service/IdempotencyServiceHashUnavailableTest.java` | `storedHashEmpty`（`:76`） | 既有記錄的 hash 為空字串（calculateHash 降級的回傳值）→ 拒絕 |  | 刪：`idempotency_request` 不帶，改 `request_id`（D4） |
| `T/service/IdempotencyServiceHashUnavailableTest.java` | `storedHashNull`（`:86`） | 既有記錄的 hash 為 null（欄位 nullable）→ 拒絕 |  | 刪：`idempotency_request` 不帶，改 `request_id`（D4） |
| `T/service/IdempotencyServiceHashUnavailableTest.java` | `incomingHashEmpty`（`:94`） | 本次請求算不出 hash → 拒絕（不能拿一個空值去比對別人的記錄） |  | 刪：`idempotency_request` 不帶，改 `request_id`（D4） |
| `T/service/IdempotencyServiceHashUnavailableTest.java` | `hashMismatch`（`:102`） | hash 兩邊都有且不同 → 拒絕（既有行為） |  | 刪：`idempotency_request` 不帶，改 `request_id`（D4） |
| `T/service/IdempotencyServiceHashUnavailableTest.java` | `hashMatchStillReturnsCachedResult`（`:110`） | hash 相同 → 正常回傳先前結果（不得過度收緊，冪等本身要還能用） |  | 刪：`idempotency_request` 不帶，改 `request_id`（D4） |
| `T/service/IdempotencyServiceHashUnavailableTest.java` | `lookupIsScopedToStore`（`:121`） | 撈既有記錄必須帶 storeId：撞鍵時不得撈到另一個賣家的記錄 |  | 刪：`idempotency_request` 不帶，改 `request_id`（D4） |
| `T/service/IdempotencyServiceHashUnavailableTest.java` | `conflictMessageIsNotSwallowed`（`:137`） | 衝突例外的 message 真的傳得出去（3-arg 建構子） |  | 刪：`idempotency_request` 不帶，改 `request_id`（D4） |
| `T/service/ProductCertificationApplicationServiceEventTest.java` | `syncCacheHitApproval_publishesNothing`（`:156`） | 同步快取命中的即刻核准 → 不發 approved、不發 verify.requested | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceEventTest.java` | `backgroundApproval_publishesApprovedAsSystem`（`:173`） | 背景補課核准（approveBySystem）→ 發 approved，decidedBy=SYSTEM | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceEventTest.java` | `backgroundApprovalIneligible_publishesNothing`（`:192`） | 背景補課但資格不符（requiresManualReview）→ 不發任何事件 | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceEventTest.java` | `adminApproval_publishesApprovedAsAdmin`（`:203`） | ADM 人工核准 → 發 approved，decidedBy=ADMIN | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceEventTest.java` | `adminRejection_publishesRejected`（`:215`） | ADM 駁回 → 發 rejected，帶 reasonCode／publicReason | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceEventTest.java` | `adminRevocation_publishesRevoked`（`:233`） | ADM 撤銷 → 發 revoked，帶 reasonCode／publicReason | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceEventTest.java` | `sellerWithdraw_publishesNothing`（`:250`） | 賣家撤回 → 不發任何裁決事件 | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceEventTest.java` | `cacheMiss_publishesVerifyRequested`（`:271`） | 同步快取未命中（PENDING）→ 發 verify.requested，id 欄位全為 string | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceEventTest.java` | `manualReviewRequired_stillPublishesVerifyRequested`（`:294`） | requiresManualReview=true 仍要發 verify.requested，且旗標帶進 payload | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceEventTest.java` | `unsupportedType_publishesNothing`（`:306`） | 不支援線上查驗的類型（UNSUPPORTED）→ 不發 verify.requested | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceEventTest.java` | `sellerSubmit_publishesVerifyRequested`（`:317`） | 賣家單筆申請（PENDING）→ 也發 verify.requested | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceEventTest.java` | `recipientResolutionFails_publishesNothingAndPropagates`（`:342`） | 收件人解析失敗 → 不得發出任何 payload，且例外向外傳播讓交易 rollback | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceEventTest.java` | `recipientIsAccountIdNotStoreId`（`:360`） | 收件人是登入 principal（account_id），不是 store_id | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceEventTest.java` | `rejectedPayloadWireShape`（`:382`） | 駁回 payload 不含 internalNote，且 wire key 為 snake_case、id 為 JSON 字串 | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceEventTest.java` | `approvedPayloadOmitsReasonKeys`（`:414`） | 核准 payload 不帶 reason_code／public_reason | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceEventTest.java` | `verifyRequestedWireShape`（`:428`） | verify.requested wire key 為 snake_case、id 為 JSON 字串 | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `partialUpdateMustNotTouchAnyCertification`（`:168`） | 旗標 false ＋ 未帶認證 ⇒ 一筆都不能動（既有 APPROVED 不得被撤回） | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `emptyListWithoutFlagMustNotTouchAnyCertification`（`:183`） | 旗標 false ＋ 空清單（新 client 送空但沒設旗標）⇒ 同樣一筆都不能動 | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `legacyClientWithoutFlagStillCreatesApplication`（`:197`） | 旗標 false ＋ 認證非空（舊 App）⇒ 仍然要建立申請 | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `explicitClearWithdrawsApprovedAndRebuildsProjection`（`:208`） | 旗標 true ＋ 空陣列 ⇒ 這是唯一合法的『清空』，既有 APPROVED 轉 WITHDRAWN ＋ 重建投影 | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `submittingNewNumberKeepsOldApprovedPublic`（`:242`） | 送出同 type 的新號碼 B ⇒ 舊的 A 仍是 APPROVED、投影不動 | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `approvingNewNumberSupersedesOldAndProjectsOnlyNew`（`:255`） | B 核准後 ⇒ 投影只剩 B，A 轉 SUPERSEDED 且留下稽核事件 | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `supersedeMustHappenBeforeApprove`（`:306`） | 先把舊 APPROVED 轉 SUPERSEDED，再把本筆轉 APPROVED | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `staleExpectedVersionThrowsSoTransactionRollsBack`（`:337`） | expectedVersion 不符 ⇒ 拋 3026 讓交易 rollback（不得走 onNext + code 的既有慣例） | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `terminalStatusCannotBeApproved`（`:351`） | 終態不得再被核准 ⇒ 3027 | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `approveRebuildsProjection`（`:369`） | APPROVE（人工） | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `systemAutoApproveRebuildsProjection`（`:391`） | APPROVE（系統）：同步快取命中有效 ⇒ 插 SUBMITTED 後在同一交易內核准並重建投影 | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `supersedeInStep4WritesEvent`（`:430`） | SUPERSEDE（§6.3 Step 4 的併發副產物也要寫事件） | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `withdrawRebuildsProjectionOnlyFromApproved`（`:449`） | WITHDRAW：由 APPROVED 轉出才重建投影；由 SUBMITTED 轉出不動投影 | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `revokeRebuildsProjection`（`:480`） | REVOKE：標章必須立刻從公開投影消失 | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `revokeWithoutReasonCodeIsRejected`（`:500`） | 撤銷缺 reasonCode ⇒ 3028，且不得動到任何列 | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `unchangedProjectionIsNotRewritten`（`:511`） | approved 集合沒變 ⇒ 不覆寫投影、不 bump version | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `illegalEntriesAreDroppedWithoutThrowing`（`:538`） | 非法 type 與非法 certNo 各自被剔除，其他合法筆數照常進申請表 | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `concurrentConflictBecomesWarningNotError`（`:562`） | ON CONFLICT DO NOTHING 沒插進去 ⇒ 落進 certificationWarnings，不 rollback、不回 4xx | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `duplicateInputIsDeduped`（`:575`） | 同一次請求送兩筆完全相同的 (type, certNo) ⇒ 只建一筆，也不發警告 | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `sameValueOppositeOutcomeAcrossTypes`（`:598`） | 同一個 ISO 22000:2018：3705 保留、3701 剔除 | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `relaxedTypesAcceptRealWorldFormats`（`:621`） | HACCP／GMP 的空格與斜線編號必須保留（放寬類不是不驗，是換一組規則） | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `relaxedTypesStillRejectControlCharsAndAngleBrackets`（`:635`） | 放寬類仍擋控制字元與角括號 | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `relaxedLengthIsCountedInCharactersNotBytes`（`:653`） | 長度用字元數不是位元組：64 個中文字通過、65 個被擋 | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `relaxedTypesPersistNormalizedValue`（`:677`） | 放寬類：存進 DB 的必須是正規化（trim）後的值，不是原值 | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `strictTypesJudgeRawValueWithoutTrimming`（`:691`） | 嚴格類對原值判：前後空白直接判不合法，不先 trim 再放行 | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `singleSubmitReturnsBusinessCodeInsteadOfDropping`（`:707`） | 單筆 submit：非法 certNo 回業務碼，不走剔除 | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `singleSubmitAcceptsRelaxedTypeAndPersistsNormalized`（`:720`） | 單筆 submit：放寬類的合法編號要能送出，且存正規化後的值 | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `alreadyApprovedIsNoop`（`:749`） | Step 1：同組已 APPROVED ⇒ no-op，不再建申請、也不執行 Step 4 | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `resubmittingRejectedNumberRequiresManualReview`（`:765`） | Step 3：曾被 REJECTED 的號碼重送 ⇒ requiresManualReview=true（永不自動核准） | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `casTypeIsNeverAutoApproved`（`:794`） | 3703 CAS 即使查驗通過也不得自動核准 | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `unsupportedTypeSkipsVerification`（`:817`） | 3704 HACCP ⇒ UNSUPPORTED，且不讀查驗快取 | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `syncPathNeverCallsGovApi`（`:832`） | 同步階段只讀快取，絕不呼叫政府 API | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `matchedHolderIsAutoApproved`（`:904`） | 業者名稱屬於本賣家 ⇒ 照常自動核准（新條件不得誤傷正常賣家） | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `mismatchedHolderIsParkedForManualReview`（`:928`） | HOLDER_MISMATCH：賣家有已通過的名稱但對不上 ⇒ 不核准並轉人工 | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `sellerWithoutVerifiedNameIsParkedForManualReview`（`:943`） | SELLER_HAS_NO_VERIFIED_NAME：賣家無已通過 KYC ⇒ 不核准並轉人工 | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `lookupFailureFailsClosed`（`:957`） | LOOKUP_UNAVAILABLE：比對 RPC 失敗 ⇒ fail-closed，不核准並轉人工 | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `nullHolderNameSkipsRpcAndParks`（`:968`） | NO_HOLDER_NAME（null）：不核准並轉人工，且完全不呼叫 seller-service | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `blankHolderNameSkipsRpcAndParks`（`:977`） | NO_HOLDER_NAME（空字串）：不核准並轉人工，且完全不呼叫 seller-service | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `neverRejectsForAnyReason`（`:1008`） | 四種不放行的成因，沒有一種可以導向 REJECTED | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `foreignProductLooksMissing`（`:1035`） | 非本人商品一律當成不存在（防 IDOR 枚舉） | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `dedicatedEndpointReturnsTypedConflict`（`:1045`） | 專用端點的併發衝突回 typed 409（3029），不是靜默跳過 | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `withdrawingForeignApplicationLooksMissing`（`:1058`） | 撤回別人的申請 ⇒ 當成不存在（3025），不得洩漏它存在 | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `reverifyNeverChangesStatus`（`:1078`） | 查驗通過也不自動核准，只回寫查驗欄位 | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `govUnavailableNeverBecomesNotFound`（`:1101`） | 政府 API 不可用 ⇒ 落 UNAVAILABLE，絕不得落成 NOT_FOUND | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `unsupportedTypeSkipsExternalCall`（`:1118`） | 3704 等不支援線上驗證的類型 ⇒ 落 UNSUPPORTED，不打外部 API | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `reverifyBypassesResultCache`（`:1147`） | 🔴 reverify 走繞過結果快取的路徑，不得走會讀快取的背景路徑 | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `backgroundVerifyStillUsesCache`（`:1171`） | 反向：MQ 背景查驗仍走讀快取的路徑（配額只花在 ADM 手動觸發上） | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `cooldownBlocksRepeatedReverify`（`:1198`） | 🔴 冷卻窗內重複觸發 ⇒ 回 3030，且完全不打政府 API | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `cooldownMessageTellsAdminWhy`（`:1220`） | 被擋時訊息講得出「幾秒前查的」與「還要等多久」 | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `cooldownMessageReadsNaturallyAtZeroElapsed`（`:1242`） | 剛查完就再按（elapsed=0）⇒ 訊息說「剛剛」，不說「0 秒前」 | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `cooldownDoesNotBlockLegacyFirstVerify`（`:1264`） | 🔴 gov_verified_at 為 NULL（legacy 列首次查驗）不套用冷卻 | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `cooldownExpiresAndAllowsAgain`（`:1281`） | 冷卻窗過了就放行（不是永久鎖住） | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `leaseHeldByConcurrentAdminBlocks`（`:1304`） | 🔴 租約被別人持有（另一個查驗還在飛）⇒ 擋下，且不打政府 API | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `leaseIsAlwaysReleased`（`:1326`） | 查驗結束會釋放租約；即使查驗拋例外也要還 | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `redisOutageFailsOpen`（`:1356`） | Redis 失聯 ⇒ 租約 fail-open，功能不中斷 | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `backgroundVerifyNotSubjectToCooldown`（`:1379`） | 反向：MQ 背景查驗不套用冷卻（它讀快取，本來就不燒配額） | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `reverifyWritesFreshVerifiedAt`（`:1401`） | 回寫的 gov_verified_at 來自這一次查詢的 queriedAt，不是舊快取的時間 | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `delegatedWriteIsRecordedAsDelegatedAdmin`（`:1436`） | 代行寫入 ⇒ source = DELEGATED_ADMIN，稽核 actor_role = ADMIN | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `submitterIsNeverSubstitutedByStoreId`（`:1457`） | submitted_by_account_id 絕不可被 storeId 頂替 | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `reverifyReplayDoesNotCallGovApiAgain`（`:1481`） | reverify 重放 ⇒ 不再打政府 API | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `approveReplayRereadsCurrentStateAndDoesNotRerun`（`:1501`） | 核准重放 ⇒ 重讀當前狀態，且不重複執行狀態轉移 | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `successMarksIdempotencyWithApplicationId`（`:1520`） | 成功後要標記冪等成功，存的是申請 ID 不是整份快照 | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationApplicationServiceTest.java` | `failureMarksIdempotencyFailed`（`:1536`） | 失敗要標記冪等失敗（否則同一把 key 永遠卡在 IN_PROGRESS） | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationWithdrawOnDeleteTest.java` | `withdrawsSubmittedApplications`（`:97`） | 待審申請轉 WITHDRAWN，且來源狀態明寫 SUBMITTED（讓轉換對重複執行冪等） | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationWithdrawOnDeleteTest.java` | `neverTouchesApprovedApplications`（`:115`） | 🔴 不得碰 APPROVED：只能走 SUBMITTED-only 的查詢，不得改用會撈到已核准的那支 | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationWithdrawOnDeleteTest.java` | `writesSystemActorNotSeller`（`:129`） | 🔴 稽核事件的 actor 是 SYSTEM＋null，不是 SELLER（賣家刪的是商品，不是撤回申請） | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationWithdrawOnDeleteTest.java` | `noPendingApplicationsWritesNothing`（`:153`） | 沒有待審申請時直接返回，不寫任何事件 | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductCertificationWithdrawOnDeleteTest.java` | `doesNotWriteEventWhenTransitionAffectedNoRows`（`:164`） | 轉換回 0 列（被其他路徑搶先）時不寫事件 —— 事件必須對應真正發生的轉換 | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductRevalidationServiceTest.java` | `fieldEditEmitsProductOnly`（`:74`） | 欄位編輯：revalidateProduct 只含 product，不含 seller / home |  | 刪：Next.js tag revalidate 呼叫隨 Java 刪；Connect 讀取的快取策略在 B4-3 依 website 需求另定 |
| `T/service/ProductRevalidationServiceTest.java` | `batchFieldEditEmitsProductsOnly`（`:81`） | 庫存 / 改分類批次：revalidateProducts 只含每筆 product（去重），不反查賣家 |  | 刪：Next.js tag revalidate 呼叫隨 Java 刪；Connect 讀取的快取策略在 B4-3 依 website 需求另定 |
| `T/service/ProductRevalidationServiceTest.java` | `emptyBatchIsNoop`（`:90`） | 空批次：不發任何 revalidate 請求 |  | 刪：Next.js tag revalidate 呼叫隨 Java 刪；Connect 讀取的快取策略在 B4-3 依 website 需求另定 |
| `T/service/ProductRevalidationServiceTest.java` | `productSetChangeEmitsProductAndSeller`（`:99`） | 建立即上架 / 上下架跨 ACTIVE / 刪除前為 ACTIVE：revalidateProductAndSeller 含 product + seller |  | 刪：Next.js tag revalidate 呼叫隨 Java 刪；Connect 讀取的快取策略在 B4-3 依 website 需求另定 |
| `T/service/ProductRevalidationServiceTest.java` | `draftCreateEmitsSellerOnly`（`:108`） | 新建 DRAFT：revalidateSeller 只含 seller |  | 刪：Next.js tag revalidate 呼叫隨 Java 刪；Connect 讀取的快取策略在 B4-3 依 website 需求另定 |
| `T/service/ProductRevalidationServiceTest.java` | `batchOffShelfEmitsProductsAndSellers`（`:116`） | 賣家批次下架：revalidateProductsWithSellers 含每筆 product + 反查去重後的 seller |  | 刪：Next.js tag revalidate 呼叫隨 Java 刪；Connect 讀取的快取策略在 B4-3 依 website 需求另定 |
| `T/service/ProductRevalidationServiceTest.java` | `sellerTagsCoverBothStorefrontAndPdpCacheKeys`（`:142`） | 賣家 tag 同時含 store_id（PDP）與 store_code（店面頁），兩者皆不可少 |  | 刪：Next.js tag revalidate 呼叫隨 Java 刪；Connect 讀取的快取策略在 B4-3 依 website 需求另定 |
| `T/service/ProductRevalidationServiceTest.java` | `unresolvableStoreCodeDegradesToStoreIdOnly`（`:154`） | seller-service 解析不到 store_code：fail-soft 退回只發 store_id，不整批放棄 |  | 刪：Next.js tag revalidate 呼叫隨 Java 刪；Connect 讀取的快取策略在 B4-3 依 website 需求另定 |
| `T/service/ProductRevalidationServiceTest.java` | `categoryCreateOrRestoreEmitsCategoryOnly`（`:166`） | 分類新增 / 還原：revalidateCategory 只含 category |  | 刪：Next.js tag revalidate 呼叫隨 Java 刪；Connect 讀取的快取策略在 B4-3 依 website 需求另定 |
| `T/service/ProductRevalidationServiceTest.java` | `categoryBatchDeleteEmitsDistinctCategories`（`:173`） | 批次軟刪除：revalidateCategories 每筆 category 去重，單次 POST |  | 刪：Next.js tag revalidate 呼叫隨 Java 刪；Connect 讀取的快取策略在 B4-3 依 website 需求另定 |
| `T/service/ProductRevalidationServiceTest.java` | `emptyCategoryBatchIsNoop`（`:183`） | 空批次 / null 分類：不發任何 revalidate 請求 |  | 刪：Next.js tag revalidate 呼叫隨 Java 刪；Connect 讀取的快取策略在 B4-3 依 website 需求另定 |
| `T/service/ProductRevalidationServiceTest.java` | `categoryRenameEmitsCategoryAndProductsInOnePost`（`:195`） | 分類改名：category + 受影響 product 合成單次 POST，不共發 seller |  | 刪：Next.js tag revalidate 呼叫隨 Java 刪；Connect 讀取的快取策略在 B4-3 依 website 需求另定 |
| `T/service/ProductRevalidationServiceTest.java` | `categoryRenameWithNoProductsEmitsCategoryOnly`（`:207`） | 分類改名但底下無商品：只發 category，不發空的 product tag |  | 刪：Next.js tag revalidate 呼叫隨 Java 刪；Connect 讀取的快取策略在 B4-3 依 website 需求另定 |
| `T/service/ProductRevalidationServiceTest.java` | `categoryVariantsNeverEmitDomainCollectionTag`（`:219`） | 分類變體不得直接發 domain-collection tag categories |  | 刪：Next.js tag revalidate 呼叫隨 Java 刪；Connect 讀取的快取策略在 B4-3 依 website 需求另定 |
| `T/service/ProductRevalidationServiceTest.java` | `noVariantEmitsHomeTag`（`:235`） | 所有變體都不再發 legacy home tag |  | 刪：Next.js tag revalidate 呼叫隨 Java 刪；Connect 讀取的快取策略在 B4-3 依 website 需求另定 |
| `T/service/ProductServiceCertificationCascadeTest.java` | `softDeleteWithdrawsSubmittedCertificationApplications`（`:53`） | softDeleteProduct 連帶撤回該商品待審的認證申請 | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductServiceCertificationCascadeTest.java` | `withdrawHappensBeforeSoftDelete`（`:63`） | 撤回發生在 deleteById 之前 —— 兩者必須在同一事務內，且鎖住的是尚未被刪的列 | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductServiceCertificationCascadeTest.java` | `nonOwnerDeleteDoesNotTouchCertifications`（`:75`） | 非擁有者刪除被擋下時，不得動到任何認證申請 | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductServiceCertificationCascadeTest.java` | `alreadyDeletedProductDoesNotTouchCertifications`（`:90`） | 已刪除的商品重複刪除被擋下時，不得重複撤回（冪等性靠前置守衛而非下游） | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductServiceCertificationWiringTest.java` | `delegatedUpdateUsesDelegatedAdminSource`（`:120`） | 代行更新 ⇒ source = DELEGATED_ADMIN | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductServiceCertificationWiringTest.java` | `sellerUpdateUsesSellerSource`（`:129`） | 賣家自助更新 ⇒ source = SELLER | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductServiceCertificationWiringTest.java` | `submitterIsNullNotStoreId`（`:144`） | submittedByAccountId 必須是 null，不得被 storeId 頂替 | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductServiceCertificationWiringTest.java` | `gateInputsArePassedThroughUntouched`（`:160`） | 旗標與清單原樣傳給收斂點，ProductService 不得自行提前 return | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductServiceCertificationWiringTest.java` | `warningsAreSurfacedInOutcome`（`:172`） | 收斂點的 certificationWarnings 要出現在 ProductWriteOutcome | 刪 | 刪：範圍裁切（認證） |
| `T/service/ProductServiceDelegationGuardTest.java` | `getProduct_delegatedCrossStore_denied`（`:69`） | Q4：ACTIVE 商品跨店代行讀取 ⇒ 9275（既有述詞對 ACTIVE 一律放行，靠這一步擋） |  | 改寫：B4-5d 代行跨店讀取被拒、找不到與不屬本店不可區分；`expectedVersion` 改 etag |
| `T/service/ProductServiceDelegationGuardTest.java` | `getProduct_withoutOwnershipCheck_crossStoreSucceeds`（`:86`） | 鑑別力對照：同一筆 ACTIVE 商品在不做歸屬斷言時跨店讀得到 |  | 改寫：B4-5d 代行跨店讀取被拒、找不到與不屬本店不可區分；`expectedVersion` 改 etag |
| `T/service/ProductServiceDelegationGuardTest.java` | `getProduct_delegatedOwnStore_allowed`（`:99`） | 代行本店讀取照常通過 |  | 改寫：B4-5d 代行跨店讀取被拒、找不到與不屬本店不可區分；`expectedVersion` 改 etag |
| `T/service/ProductServiceDelegationGuardTest.java` | `updateSellerProductStatus_staleExpectedVersion_denied`（`:112`） | expectedVersion 與現況不符 ⇒ 9266，且不進到狀態轉換 |  | 改寫：B4-5d 代行跨店讀取被拒、找不到與不屬本店不可區分；`expectedVersion` 改 etag |
| `T/service/ProductServiceDelegationGuardTest.java` | `updateSellerProductStatus_casMiss_denied`（`:123`） | CAS 回 0 列 ⇒ 9266（MyBatis-Plus 版本不符只回 0，不拋例外） |  | 改寫：B4-5d 代行跨店讀取被拒、找不到與不屬本店不可區分；`expectedVersion` 改 etag |
| `T/service/ProductServiceDelegationGuardTest.java` | `batchSnapshots_notFoundAndNotOwnedAreIndistinguishable`（`:135`） | 批次快照：查不到與不屬本店都回 found=false，不區分（避免探測） |  | 改寫：B4-5d 代行跨店讀取被拒、找不到與不屬本店不可區分；`expectedVersion` 改 etag |
| `T/service/ProductServiceImageUpdateOrderTest.java` | `interleavedOrderPreserved`（`:51`） | 新圖拖曳插入既有圖之間 [A, 新, B] → 落地順序 = [A, 新, B]，不被重排到最後 |  | 改寫：B4-2 `product_images.position` 依請求順序落地（D17） |
| `T/service/ProductServiceImageUpdateOrderTest.java` | `pureReorderPreserved`（`:71`） | 純重排既有圖 [B, A] → [B, A]（regression：不呼叫 finalizeImages） |  | 改寫：B4-2 `product_images.position` 依請求順序落地（D17） |
| `T/service/ProductServiceListGuardTest.java` | `sellerOwnListExcludes`（`:53`） | 賣家看自己的商品（ownerStoreId 設、storeId 空）→ excludeSellerDeleted=true |  | 改寫：B4-3 列表視角測試（公開要求可曝光，賣家、admin 不要求）；`seller_deleted_at` 不帶 |
| `T/service/ProductServiceListGuardTest.java` | `publicSellerFilterDoesNotExclude`（`:63`） | 公開篩選某賣家（storeId 設）→ excludeSellerDeleted=false（flagged 已 INACTIVE，靠 status 過濾） |  | 改寫：B4-3 列表視角測試（公開要求可曝光，賣家、admin 不要求）；`seller_deleted_at` 不帶 |
| `T/service/ProductServiceListGuardTest.java` | `exposableStoreFilterAppliesToPublicViewOnly`（`:73`） | 公開視角（publicApi）→ requireExposableStore=true；賣家看自己、admin → false |  | 改寫：B4-3 列表視角測試（公開要求可曝光，賣家、admin 不要求）；`seller_deleted_at` 不帶 |
| `T/service/ProductServiceMediaReleaseTest.java` | `softDeleteReleasesMediaReferences`（`:39`） | softDeleteProduct 釋放 PRODUCT(7101) 與 PRODUCT_DESC(7102) 引用，ref_id=商品 id |  | 改寫：B4-2 刪除商品在同 transaction `MarkDeleted` 圖片（D17） |
| `T/service/ProductServiceOptimisticLockGuardTest.java` | `productVersionFieldCarriesVersionAnnotation`（`:87`） | ProductEntity.version 必須帶 @Version —— 少了它，本檔每一測都只是在驗 mock 自己的回傳值 |  | 刪：`version` 欄改 etag（D4），分類名稱 reflow 隨快照不帶（D20） |
| `T/service/ProductServiceOptimisticLockGuardTest.java` | `updateProductStatus_casMiss_throwsVersionConflict`（`:102`） | admin 改狀態：CAS 回 0 列 ⇒ 拋 9266，不回成功 |  | 刪：`version` 欄改 etag（D4），分類名稱 reflow 隨快照不帶（D20） |
| `T/service/ProductServiceOptimisticLockGuardTest.java` | `updateProductStatus_casHit_returnsBumpedVersion`（`:123`） | admin 改狀態：CAS 命中 ⇒ 回應帶的 version 是 +1 後的值（不可回舊值讓下一次寫必然衝突） |  | 刪：`version` 欄改 etag（D4），分類名稱 reflow 隨快照不帶（D20） |
| `T/service/ProductServiceOptimisticLockGuardTest.java` | `reflow_casMissThenRetry_succeeds`（`:145`） | reflow：第一次 CAS 未命中 ⇒ 重讀重算後再寫一次，成功即計入 |  | 刪：`version` 欄改 etag（D4），分類名稱 reflow 隨快照不帶（D20） |
| `T/service/ProductServiceOptimisticLockGuardTest.java` | `reflow_casMissTwice_throwsVersionConflict`（`:163`） | reflow：重試後仍未命中 ⇒ 拋 9266，不靜默把 searchable_text 停在舊分類名 |  | 刪：`version` 欄改 etag（D4），分類名稱 reflow 隨快照不帶（D20） |
| `T/service/ProductServiceOptimisticLockGuardTest.java` | `reflow_missingProduct_isSkipped`（`:180`） | reflow：id 不存在／已軟刪除仍是跳過，不算失敗也不拋 |  | 刪：`version` 欄改 etag（D4），分類名稱 reflow 隨快照不帶（D20） |
| `T/service/ProductServiceOptimisticLockGuardTest.java` | `createProduct_postInsertUpdateAffectsNoRow_throws`（`:191`） | 建立商品：補寫影響 0 列 ⇒ 拋 IllegalStateException 並標冪等 FAILED，不回 CREATED |  | 刪：`version` 欄改 etag（D4），分類名稱 reflow 隨快照不帶（D20） |
| `T/service/ProductServiceOwnerRekeyTest.java` | `getProductOwnerByStoreId`（`:68`） | getProduct：INACTIVE + 請求者 == store_id（≠ created_by）→ 視為 owner，可見 |  | 改寫：B4-3 擁有者只依呼叫者推導的店家（D4） |
| `T/service/ProductServiceOwnerRekeyTest.java` | `getProductCreatedByIsNotOwnership`（`:81`） | getProduct：INACTIVE + 請求者 == created_by（≠ store_id）→ 非 owner，拒絕 |  | 改寫：B4-3 擁有者只依呼叫者推導的店家（D4） |
| `T/service/ProductServiceOwnerRekeyTest.java` | `softDeleteOwnerByStoreId`（`:93`） | softDelete：請求者 == store_id → 通過擁有者驗證並軟刪 |  | 改寫：B4-3 擁有者只依呼叫者推導的店家（D4） |
| `T/service/ProductServiceOwnerRekeyTest.java` | `softDeleteCreatedByIsNotOwnership`（`:104`） | softDelete：請求者 == created_by（≠ store_id）→ 拒絕 |  | 改寫：B4-3 擁有者只依呼叫者推導的店家（D4） |
| `T/service/ProductServicePriceRangeClearTest.java` | `emptySkuSetClearsAllSixPriceFields`（`:73`） | SKU 集合為空 → 六個價格欄位全部算成 null（舊價格不得留在 entity 上） |  | 改寫：B4-2 `min_price`／`max_price` 同 transaction 重算（D19）；商品至少一個 SKU，空集合不出現 |
| `T/service/ProductServicePriceRangeClearTest.java` | `nullSkuSetClearsAllSixPriceFields`（`:88`） | SKU 集合為 null（同樣是「沒有 SKU」）→ 六欄同樣清空 |  | 改寫：B4-2 `min_price`／`max_price` 同 transaction 重算（D19）；商品至少一個 SKU，空集合不出現 |
| `T/service/ProductServicePriceRangeClearTest.java` | `allSixPriceFieldsCarryIgnoredUpdateStrategy`（`:100`） | 六個價格欄位都必須帶 updateStrategy = IGNORED，null 才寫得進 DB（#2546 真正的修復點） |  | 改寫：B4-2 `min_price`／`max_price` 同 transaction 重算（D19）；商品至少一個 SKU，空集合不出現 |
| `T/service/ProductServicePriceRangeClearTest.java` | `nonEmptySkuSetStillComputesRange`（`:115`） | 控制組：SKU 集合非空時照常算出區間（證明上面的 assertNull 不是 vacuously true） |  | 改寫：B4-2 `min_price`／`max_price` 同 transaction 重算（D19）；商品至少一個 SKU，空集合不出現 |
| `T/service/ProductServiceSellerDeletedGuardTest.java` | `getProductFlaggedOwnerInactiveThrowsNotFound`（`:46`） | getProduct：flagged + owner + 非 ACTIVE → NotFound（對復活賣家隱形） |  | 刪：`seller_deleted_at` 不帶（D7），刪除店家的商品由可曝光判定隱藏 |
| `T/service/ProductServiceSellerDeletedGuardTest.java` | `adminReactivateFlaggedThrows`（`:54`） | admin updateProductStatus：flagged 商品目標 ACTIVE → 拋 ProductBusinessException（禁復活） |  | 刪：`seller_deleted_at` 不帶（D7），刪除店家的商品由可曝光判定隱藏 |
| `T/service/ProductServiceSellerSuspensionActivationTest.java` | `suspendedSeller_throws9240`（`:66`） | 停權賣家（can_manage_products=false）→ 拋 ProductBusinessException 9240 |  | 改寫：B4-3 賣家 App 宣告 `seller.Cap*` 能力，停權回 `SELLER_SUSPENDED`（API 設計規範 §二），上架與建立 active 被擋、下架不擋 |
| `T/service/ProductServiceSellerSuspensionActivationTest.java` | `activeSeller_allowed`（`:77`） | 正常賣家（can_manage_products=true）→ 放行不拋例外 |  | 改寫：B4-3 賣家 App 宣告 `seller.Cap*` 能力，停權回 `SELLER_SUSPENDED`（API 設計規範 §二），上架與建立 active 被擋、下架不擋 |
| `T/service/ProductServiceSellerSuspensionActivationTest.java` | `canManageButNotAcceptOrders_allowed`（`:87`） | 可以管理商品但還不可接單（KYC 或 PayUni 未核准）→ 放行：上架閘只讀 can_manage_products |  | 改寫：B4-3 賣家 App 宣告 `seller.Cap*` 能力，停權回 `SELLER_SUSPENDED`（API 設計規範 §二），上架與建立 active 被擋、下架不擋 |
| `T/service/ProductServiceSellerSuspensionActivationTest.java` | `canAcceptOrdersButNotManageProducts_throws9240`（`:97`） | 可接單但不能管理商品（等待刪除或停權）→ 拋 9240：不以 can_accept_orders 放行 |  | 改寫：B4-3 賣家 App 宣告 `seller.Cap*` 能力，停權回 `SELLER_SUSPENDED`（API 設計規範 §二），上架與建立 active 被擋、下架不擋 |
| `T/service/ProductServiceSellerSuspensionActivationTest.java` | `sellerMetaQueryFails_failClosed503`（`:108`） | SellerMeta 查詢失敗（gRPC 例外）→ fail-closed 拋 ProductBusinessException 503，不放行 |  | 改寫：B4-3 賣家 App 宣告 `seller.Cap*` 能力，停權回 `SELLER_SUSPENDED`（API 設計規範 §二），上架與建立 active 被擋、下架不擋 |
| `T/service/ProductServiceSellerSuspensionActivationTest.java` | `draftToActive_suspendedSeller_blockedBeforeUpdate`（`:137`） | DRAFT→ACTIVE 且賣家停權 → 擋下，updateById 未被呼叫（backstop 在更新前生效） |  | 改寫：B4-3 賣家 App 宣告 `seller.Cap*` 能力，停權回 `SELLER_SUSPENDED`（API 設計規範 §二），上架與建立 active 被擋、下架不擋 |
| `T/service/ProductServiceSellerSuspensionActivationTest.java` | `activeToInactive_doesNotConsultSellerService`（`:151`） | ACTIVE→INACTIVE（下架，收縮型操作）→ 不查 SellerMeta（02 §5.10 步驟 5b 明文不擋） |  | 改寫：B4-3 賣家 App 宣告 `seller.Cap*` 能力，停權回 `SELLER_SUSPENDED`（API 設計規範 §二），上架與建立 active 被擋、下架不擋 |
| `T/service/ProductServiceSellerSuspensionActivationTest.java` | `createProduct_activeStatus_suspendedSeller_blockedBeforeInsert`（`:198`） | CreateProduct 以 ACTIVE 落地且賣家停權 → backstop 在 insert 前擋下，DB 無寫入 |  | 改寫：B4-3 賣家 App 宣告 `seller.Cap*` 能力，停權回 `SELLER_SUSPENDED`（API 設計規範 §二），上架與建立 active 被擋、下架不擋 |
| `T/service/ProductServiceSellerSuspensionActivationTest.java` | `createProduct_activeStatus_activeSeller_backstopPassesThrough`（`:218`） | CreateProduct 以 ACTIVE 落地但賣家正常 → 呼叫 backstop 放行，繼續走到 insert 前的下一步 |  | 改寫：B4-3 賣家 App 宣告 `seller.Cap*` 能力，停權回 `SELLER_SUSPENDED`（API 設計規範 §二），上架與建立 active 被擋、下架不擋 |
| `T/service/ProductServiceSkuOptimisticLockTest.java` | `skuVersionFieldCarriesVersionAnnotation`（`:129`） | ⓪ ProductSkuEntity.version 必須帶 @Version —— 少了它，本檔其餘每一測都是裝飾品 |  | 改寫：B4-2 SKU 寫入用 etag（D4）；庫存不由賣家表單覆寫帳本外的數量（D2），舊 client 相容分支不帶 |
| `T/service/ProductServiceSkuOptimisticLockTest.java` | `staleSkuVersionIsRejected`（`:144`） | ① 表單開著期間被扣了庫存 → 賣家帶舊 version 寫回，必須 409 而不是把庫存復活 |  | 改寫：B4-2 SKU 寫入用 etag（D4）；庫存不由賣家表單覆寫帳本外的數量（D2），舊 client 相容分支不帶 |
| `T/service/ProductServiceSkuOptimisticLockTest.java` | `clientExpectedVersionReachesTheCas`（`:164`） | ① 核心機制：送去 CAS 的 entity 必須帶 client 的 expectedVersion，不是剛讀到的新版本 |  | 改寫：B4-2 SKU 寫入用 etag（D4）；庫存不由賣家表單覆寫帳本外的數量（D2），舊 client 相容分支不帶 |
| `T/service/ProductServiceSkuOptimisticLockTest.java` | `casMissIsNotSilentSuccess`（`:179`） | ① rows=0 不得被當成成功：靜默不寫比報錯更糟（HTTP 200 但資料沒進去） |  | 改寫：B4-2 SKU 寫入用 etag（D4）；庫存不由賣家表單覆寫帳本外的數量（D2），舊 client 相容分支不帶 |
| `T/service/ProductServiceSkuOptimisticLockTest.java` | `productOnlyEditNeverTouchesSkuVersion`（`:195`） | ② 只改 product 其他欄位（skus 未帶）→ 一條 SKU CAS 都不做，扣過庫存也不會 409 |  | 改寫：B4-2 SKU 寫入用 etag（D4）；庫存不由賣家表單覆寫帳本外的數量（D2），舊 client 相容分支不帶 |
| `T/service/ProductServiceSkuOptimisticLockTest.java` | `absentExpectedVersionKeepsLegacyBehaviour`（`:207`） | ② 沒送 expectedVersion 的舊 client → 沿用既有語意（不比對），不得平白 409 |  | 改寫：B4-2 SKU 寫入用 etag（D4）；庫存不由賣家表單覆寫帳本外的數量（D2），舊 client 相容分支不帶 |
| `T/service/ProductServiceSkuOptimisticLockTest.java` | `versionAdvancesAfterSuccessfulCas`（`:223`） | CAS 成功後回傳的 entity 版本要 +1，才不會把舊版本再下發給 client |  | 改寫：B4-2 SKU 寫入用 etag（D4）；庫存不由賣家表單覆寫帳本外的數量（D2），舊 client 相容分支不帶 |
| `T/service/ProductServiceSkuOptimisticLockTest.java` | `nonNumericExpectedVersionIsRejected`（`:238`） | expectedVersion 不是數字 → 400 參數錯誤，不得靜默當成「沒帶」而放行 |  | 改寫：B4-2 SKU 寫入用 etag（D4）；庫存不由賣家表單覆寫帳本外的數量（D2），舊 client 相容分支不帶 |
| `T/service/ProductServiceStoresWithPublicProductsTest.java` | `reusesSharedPublicProductAggregate`（`:41`） | 沿用 findPublicProductCountsByStore 的結果，只取 store_id |  | 改寫：B4-4 模組 API 測試 |
| `T/service/ProductServiceStoresWithPublicProductsTest.java` | `dropsNullStoreIds`（`:52`） | store_id 為 null 的列直接丟棄，不得混進 filter 集合 |  | 改寫：B4-4 模組 API 測試 |
| `T/service/ProductServiceStoresWithPublicProductsTest.java` | `emptyAggregateYieldsEmptyList`（`:62`） | 平台上沒有任何公開商品時回空集合（不是 null） |  | 改寫：B4-4 模組 API 測試 |
| `T/service/ProductServiceUpdateProductStockQtyTest.java` | `productOnlyEditDoesNotWriteStockQty`（`:173`） | 純商品編輯（未帶 skus）→ stock_qty 不得進 UPDATE，否則會蓋掉編輯期間的庫存異動 |  | 改寫：B4-2 只改商品欄位不動庫存（D2） |
| `T/service/ProductServiceUpdateProductStockQtyTest.java` | `productOnlyEditStillReportsStockInResponse`（`:189`） | 純商品編輯的回應仍要帶既有庫存（不得因為不寫 DB 就把回應裡的 stock 變成 null） |  | 改寫：B4-2 只改商品欄位不動庫存（D2） |
| `T/service/ProductServiceUpdateProductStockQtyTest.java` | `skuEditStillWritesAggregatedStockQty`（`:204`） | 有帶 skus → stock_qty 照常寫入（本次修改不得把 SKU 編輯路徑的庫存維護一起關掉） |  | 改寫：B4-2 只改商品欄位不動庫存（D2） |
| `T/service/ProductServiceUpdateSkusEmptyListTest.java` | `emptyListIsNoOp`（`:68`） | skus 為空清單（前端只改名稱）→ 既有 SKU 原封不動，不呼叫任何刪除 |  | 改寫：B4-2 SKU 組合更新（D18）：不屬本商品的 SKU 拒絕整筆且不先寫、商品至少一個 SKU |
| `T/service/ProductServiceUpdateSkusEmptyListTest.java` | `nullListIsNoOp`（`:83`） | skus 為 null（同樣代表未提供）→ 既有 SKU 原封不動 |  | 改寫：B4-2 SKU 組合更新（D18）：不屬本商品的 SKU 拒絕整筆且不先寫、商品至少一個 SKU |
| `T/service/ProductServiceUpdateSkusEmptyListTest.java` | `listedSkusStillPruneTheRest`（`:95`） | 控制組：帶了 SKU 清單時，未列入的既有 SKU 仍然會被刪除（證明上面兩測不是 vacuously true） |  | 改寫：B4-2 SKU 組合更新（D18）：不屬本商品的 SKU 拒絕整筆且不先寫、商品至少一個 SKU |
| `T/service/ProductServiceUpdateSkusEmptyListTest.java` | `unknownSkuIdRejectsRequestAndKeepsExistingSkus`（`:118`） | #2544 送了不屬於本商品的 skuId → 拒絕整個請求，既有 SKU 一個都不能被刪 |  | 改寫：B4-2 SKU 組合更新（D18）：不屬本商品的 SKU 拒絕整筆且不先寫、商品至少一個 SKU |
| `T/service/ProductServiceUpdateSkusEmptyListTest.java` | `unknownSkuIdNeverDeletesExistingSkus`（`:137`） | #2544 核心不變式：非法 skuId 不得造成既有 SKU 被刪除（舊實作在此把 101/102 刪光） |  | 改寫：B4-2 SKU 組合更新（D18）：不屬本商品的 SKU 拒絕整筆且不先寫、商品至少一個 SKU |
| `T/service/ProductServiceUpdateSkusEmptyListTest.java` | `unknownSkuIdRejectsBeforeAnyWrite`（`:157`） | #2544 fail-fast：非法 id 混在合法 id 之後時，合法的那筆也不得先被寫入 |  | 改寫：B4-2 SKU 組合更新（D18）：不屬本商品的 SKU 拒絕整筆且不先寫、商品至少一個 SKU |
| `T/service/ProductServiceUpdateSkusEmptyListTest.java` | `resultIsNeverEmpty`（`:174`） | 結構性不變式：updateSkus 的結果集恆非空（商品永遠至少一個 SKU） |  | 改寫：B4-2 SKU 組合更新（D18）：不屬本商品的 SKU 拒絕整筆且不先寫、商品至少一個 SKU |
| `T/service/ProductServiceUpdateSkusEmptyListTest.java` | `blankIdMeansInsertNotRejection`（`:201`） | 控制組：新增用的空白 id（無 id = 新增 SKU）不得被誤判成非法 id |  | 改寫：B4-2 SKU 組合更新（D18）：不屬本商品的 SKU 拒絕整筆且不先寫、商品至少一個 SKU |
| `T/service/SellerProductOffShelfServiceTest.java` | `offShelfsActiveProductsWithSideEffects`（`:46`） | 有 ACTIVE 商品：批次下架為 INACTIVE，每筆扣 hashtag、清詳情快取，並清賣家+公開列表快取 |  | 刪：不複製店家刪除狀態（D7） |
| `T/service/SellerProductOffShelfServiceTest.java` | `stampsEvenWhenNoActiveProducts`（`:75`） | 無 ACTIVE 商品：不下架/不動 hashtag/不清商品詳情快取，回 0；但仍無條件蓋章 seller_deleted_at 且 bust 列表快取（含只剩 INACTIVE/DRAFT 的賣家） |  | 刪：不複製店家刪除狀態（D7） |
| `T/service/StockServiceCacheEvictionTest.java` | `deductEvictsOnlyAfterCommit`（`:89`） | deduct：交易內不 evict，afterCommit 後才清 detail + list（race-free） |  | 改寫：快取清除不帶（§7）；補貨事件只在店家可曝光時發 → B4-3 in-process 補貨事件測試 |
| `T/service/StockServiceCacheEvictionTest.java` | `restoreEvictsOnlyAfterCommit`（`:114`） | restore：與 deduct 共用同一 afterCommit 收斂入口（OrderCancelledConsumer 路徑） |  | 改寫：快取清除不帶（§7）；補貨事件只在店家可曝光時發 → B4-3 in-process 補貨事件測試 |
| `T/service/StockServiceCacheEvictionTest.java` | `fallbackEvictsImmediatelyWithoutTransaction`（`:138`） | 無 active transaction（防禦路徑）：fallback 直接 evict |  | 改寫：快取清除不帶（§7）；補貨事件只在店家可曝光時發 → B4-3 in-process 補貨事件測試 |
| `T/service/StockServiceCacheEvictionTest.java` | `restockNotificationPublishedWhenStoreExposable`（`:163`） | 補貨通知：店家可曝光時才發 |  | 改寫：快取清除不帶（§7）；補貨事件只在店家可曝光時發 → B4-3 in-process 補貨事件測試 |
| `T/service/StockServiceCacheEvictionTest.java` | `restockNotificationSuppressedWhenStoreNotExposable`（`:175`） | 補貨通知：店家不可曝光（含沒有列）不發 |  | 改寫：快取清除不帶（§7）；補貨事件只在店家可曝光時發 → B4-3 in-process 補貨事件測試 |
| `T/service/StoreExposureServiceTest.java` | `appliedEvictsCaches`（`:38`） | 有改到列：清這家店每個商品的詳情快取、列表快取，並通知前端快取失效 |  | 刪：同上 |
| `T/service/StoreExposureServiceTest.java` | `redeliveredSameVersionStillEvicts`（`:53`） | 同一版本重送（上一次寫入已提交、快取清除失敗）：仍清快取，補上沒做完的清除 |  | 刪：同上 |
| `T/service/StoreExposureServiceTest.java` | `staleDoesNotEvict`（`:73`） | 舊版本（沒改到列）：不清任何快取 |  | 刪：同上 |
