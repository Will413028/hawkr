# B7-0 測試盤點：Java coupon-service 的 integration 測試（唯讀，快照 dev `df21e0d8a`）

標記：【量】＝跑過指令或打開檔案逐行看過；【推】＝只憑方法名或檔頭推論，沒讀斷言。路徑縮寫：`T=backend/services/java/coupon-service/src/test/java/io/github/will413028/hawkr/coupon`。「去向」欄全部留空。所有指令從 `/Users/will/second-brain/.projects/personal/hawkr` 執行。本盤點沒有跑任何測試（Testcontainers 要 Docker，本機不跑；依 hawkr 規則交給 CI）。

## 0. 數字與指令

| 項目 | 量到 | 指令 |
|---|---|---|
| 測試檔 | 18 | `find backend/services/java/coupon-service/src/test -name '*.java' \| wc -l` 【量】 |
| 測試方法總數 | **59**（`@Test`，沒有 `@ParameterizedTest`） | `rg -n -A3 '@Test\|@ParameterizedTest' backend/services/java/coupon-service/src/test --no-heading \| rg 'void ' \| wc -l`＝59【量】；`rg -c '@Test\|@ParameterizedTest' …` 加總是 60，多的 1 是 `IssuerIdentityMigrationIT.java:38` 的 `@Testcontainers` 註解行 |
| integration 測試（Testcontainers／`@Tag("integration")`／`@SpringBootTest`） | **3 個方法，1 個類別**（`IssuerIdentityMigrationIT`）；`@SpringBootTest` 命中 0 | `rg -n 'Testcontainers\|@SpringBootTest\|@DataJpaTest\|@MybatisTest\|PostgreSQLContainer\|@Tag' backend/services/java/coupon-service/src/test`：只命中 `migration/IssuerIdentityMigrationIT.java:8,10,37,38,39,42,108,116`【量】 |
| 其餘 56 個 | 純單元測試（Mockito 風格，沒有起 Spring context、沒有連 DB／RabbitMQ／Redis） | 同上，其餘檔案無 Testcontainers 命中【量】；Mockito 使用是從檔頭與 import 推（【推】，未對每檔 grep `@Mock`） |
| 預設會不會跑 integration | **預設不跑**：pom `excludedGroups=integration`（`backend/services/java/coupon-service/pom.xml:32-33`）；surefire `includes` 另加 `**/*IT.java`（`pom.xml:243-250`），所以 IT 只有在覆寫 `excludedGroups` 時才會執行 | `rg -n 'excludedGroups\|\*IT' backend/services/java/coupon-service/pom.xml`【量】 |
| 執行指令（只跑這支 IT） | `cd backend/services/java && ./mvnw -B -pl coupon-service -am test -Dgroups=integration -DexcludedGroups= -Dtest=IssuerIdentityMigrationIT`（需要 Docker 與 `postgres:16-alpine` 映像）；**本機不跑**（hawkr 規則），由 CI 跑 | 指令由 `.github/workflows/backend-java-tests.yml:73` 的全 reactor 指令 `./mvnw test -B -fae -Dgroups=integration -DexcludedGroups=` 縮小而來【推：`-pl`、`-Dtest` 的組合沒有實際執行驗證】 |
| CI 位置 | `.github/workflows/backend-java-tests.yml` 的 `integration-test` job（Testcontainers，真 PostgreSQL）；全 reactor 的 integration 數下限 100（`:83` 的檢查），coupon 這 3 個算在其中 | `rg -n 'integration' .github/workflows/backend-java-tests.yml`【量】 |

## 1. integration 測試（3，全在 `IssuerIdentityMigrationIT`）

類別層級：`T/migration/IssuerIdentityMigrationIT.java:37-39`（`@Tag("integration")`、`@Testcontainers`）；共用容器 `POSTGRES`（`postgres:16-alpine`，`:42-45`）；`@BeforeAll migrateAndConnect`（`:49-58`）以 Flyway 從空 DB 跑完 V1–V6 後開一條 JDBC 連線；輔助 `insertTemplate`（`:67-91`）直接 INSERT `coupon_templates`（`type='1'`、`target_type='1'`、`start_at=NOW()`、`end_at=NOW()+1 day`）。

| class#method | what it asserts | 去向 |
|---|---|---|
| `IssuerIdentityMigrationIT#adminRow_withIssuerAccountId_isAccepted`（`:92-95`） | 套完 V1–V6 的 schema 上，`issuer_type='1'`（ADMIN）、`issuer_account_id=777`、`issuer_store_id=NULL` 的 INSERT 不丟例外（`assertDoesNotThrow`）。證明 V6 把 `issuer_store_id` 放寬為可 NULL，且新欄位 `issuer_account_id` 存在。不測 CHECK（從未加過，見 defects.md）。 | deleted: Flyway V1-V6 schema retires with the service; replaced by schema CHECK and UNIQUE integration tests (step B7-2; BD31) |
| `IssuerIdentityMigrationIT#sellerRow_withIssuerStoreId_isAccepted`（`:97-100`） | `issuer_type='2'`（SELLER）、`issuer_account_id=NULL`、`issuer_store_id=888` 的 INSERT 不丟例外。 | deleted: Flyway V1-V6 schema retires with the service; replaced by schema CHECK and UNIQUE integration tests (step B7-2; BD31) |
| `IssuerIdentityMigrationIT#v6Backfill_migratesPreExistingDirtyDataFromV5Schema`（`:114-190`） | 另起第二個 Testcontainers Postgres：Flyway 只 migrate 到 target `5`，用 JDBC 在舊 schema 插入兩列（template 2001＝ADMIN、`issuer_store_id=777`；2002＝SELLER、`issuer_store_id=888`），再 migrate 到底套 V6；斷言 2001 的 `issuer_account_id=777` 且 `issuer_store_id IS NULL`；2002 的 `issuer_account_id IS NULL` 且 `issuer_store_id=888`。也就是只驗 V6 的 backfill `UPDATE … WHERE issuer_type='1'`。 | deleted: Flyway V1-V6 schema retires with the service; replaced by schema CHECK and UNIQUE integration tests (step B7-2; BD31) |

這 3 個 IT 都不碰任何 RPC、MQ、排程或鎖券流程。

## 2. 沒有 integration 覆蓋的行為（盤點結論，供 characterization 規劃用）

`rg -l <符號> backend/services/java/coupon-service/src/test` 的命中數（【量】）：

| 符號 | 命中檔數 | 說明 |
|---|---|---|
| `reserveCoupon`、`consumeCoupon`、`releaseCoupon`、`validateCoupon` | 0、0、0、0 | 券的生命週期（鎖／核銷／釋放）完全沒有測試，連單元層都沒有 |
| `batchClaimForBuyers`、`incrementIssuedCount`、`selectAvailableForBuyer` | 0、0、0 | 領券、發行上限、買家可見性 SQL 都沒有測試 |
| `voidInstance`、`grantSellerCompensationCoupon`、`adminIssueCoupons`、`adminDistributeTemplate` | 0、0、0、0 | admin 與 aftersales 入口沒有測試 |
| `AutoReleaseJob`、`AutoExpireJob` | 0、0 | 對帳與過期排程沒有測試（只有 `CouponExpiringWarningJobTest`） |
| `calculateDiscount` | 0 | 折扣計算沒有測試 |
| `claimCoupon`、`revokeByTemplate`、`checkQuota` | 各 1 | 命中的是方法名片段（`buyerClaimCoupon_usesRequestFields…`、`revokeTemplate_*`、`CouponTemplateServiceIssuerIdentityTest` 等），不是對這些 service 方法本身的行為測試【推】 |

整體：59 個測試幾乎全是「身分重鍵（account_id→buyer_id／store_id）與 issuer 欄位拆分」的回歸測試，以及 MQ consumer 的 payload 解析、停權連動、自發自用防護、通知欄位。**沒有任何一個測試涵蓋 reserve／consume／release 的並發與冪等**，這是 B7 characterization 要新建的部分（對應 order-service 結帳鎖券與 aftersales 補償，見 interface-inventory.md §1.2、§1.5）。

## 3. 其餘 56 個單元測試（附錄；斷言內容是依方法名與檔頭推論的，標【推】，未逐一讀斷言）

下表是為了讓 characterization 規劃知道哪些行為已經有「Java 端的期望」可對照；`what it asserts` 皆為【推】。

| class#method | what it asserts【推】 | 去向 |
|---|---|---|
| `event/CouponEventPublisherTest#publishCouponIssued_carriesTheResolvedAccountId`（`:24`） | `coupon.issued` 事件帶解析出的登入 account_id（`notify_buyer_account_id`） | deleted: `coupon.issued` has no production caller (BD27); `VoucherExpiring` event tests are new (step B7-4) |
| `event/CouponEventPublisherTest#publishCouponIssued_unresolvedBuyerIsNotEchoedAsAnAccount`（`:36`） | buyer 解析不到時事件帶 0，不拿 buyer_id 冒充 account | deleted: `coupon.issued` has no production caller (BD27); `VoucherExpiring` event tests are new (step B7-4) |
| `service/CouponTemplateServiceIssuerIdentityTest#createTemplate_writesIssuerAccountId_leavesIssuerStoreIdNull`（`:52`） | admin 模板只寫 `issuer_account_id`，`issuer_store_id` 為 null | rewritten as module issuer-identity integration test (step B7-2) |
| `…#createCompensationTemplate_writesIssuerAccountId_leavesIssuerStoreIdNull`（`:67`） | 同上，補償模板（admin） | rewritten as module issuer-identity integration test (step B7-2) |
| `…#createSellerTemplate_writesIssuerStoreId_leavesIssuerAccountIdNull`（`:79`） | 賣家模板只寫 `issuer_store_id` | rewritten as module issuer-identity integration test (step B7-2) |
| `…#createSellerCompensationTemplate_writesIssuerStoreId_leavesIssuerAccountIdNull`（`:93`） | 賣家補償模板只寫 `issuer_store_id` | rewritten as module issuer-identity integration test (step B7-2) |
| `mq/BuyerDeletedConsumerTest#validBuyerIdVoidsAndAcks`（`:46`） | 合法 `buyerId` 會呼叫 `voidAllUnusedByOwner` 並 ack | rewritten as in-process `BuyerDeleted` subscriber same-tx test; malformed-payload cases deleted because typed events cannot be malformed (step B7-4; D8) |
| `…#missingBuyerIdAcksWithoutVoiding`（`:57`） | 缺 `buyerId` 只 ack，不作廢 | rewritten as in-process `BuyerDeleted` subscriber same-tx test; malformed-payload cases deleted because typed events cannot be malformed (step B7-4; D8) |
| `…#zeroBuyerIdAcksWithoutVoiding`（`:68`） | `buyerId=0` 只 ack，不作廢 | rewritten as in-process `BuyerDeleted` subscriber same-tx test; malformed-payload cases deleted because typed events cannot be malformed (step B7-4; D8) |
| `mq/SellerSuspendedConsumerTest#disableTemplatesUsesStoreIdNotAccountId`（`:38`） | 停權用 payload 的 `store_id`，不用 `accountId` | rewritten as in-process `StoreSuspended`/`StoreUnsuspended` subscriber same-tx test; payload-drop cases deleted (step B7-4; D8) |
| `…#missingStoreIdDropsMessage`（`:49`） | 缺 `store_id` 丟棄並 ack | rewritten as in-process `StoreSuspended`/`StoreUnsuspended` subscriber same-tx test; payload-drop cases deleted (step B7-4; D8) |
| `mq/SellerUnsuspendedConsumerTest#restoreTemplatesUsesStoreIdNotAccountId`（`:33`） | 同上，解除停權 | rewritten as in-process `StoreSuspended`/`StoreUnsuspended` subscriber same-tx test; payload-drop cases deleted (step B7-4; D8) |
| `…#missingStoreIdDropsMessage`（`:44`） | 同上 | rewritten as in-process `StoreSuspended`/`StoreUnsuspended` subscriber same-tx test; payload-drop cases deleted (step B7-4; D8) |
| `mq/dto/SellerSuspensionEventPayloadTest#fromJson_parsesStoreIdSeparatelyFromAccountId`（`:22`） | `store_id` 與 `accountId` 分開解析 | deleted: no JSON payload parsing with in-process typed events (D8) |
| `…#fromJson_missingStoreId_returnsNull`（`:35`） | 缺 `store_id` 回 null | deleted: no JSON payload parsing with in-process typed events (D8) |
| `service/CouponTemplateServiceRevokeTemplateTest#revokeTemplate_passesSellerIssuerTypeToMapper`（`:44`） | 收回時傳 `issuer_type=SELLER` 給 mapper | rewritten as campaign end integration test per D5 (step B7-2) |
| `…#revokeTemplate_returnsFalse_whenMapperAffectsZeroRows`（`:56`） | mapper 影響 0 列回 false | rewritten as campaign end integration test per D5 (step B7-2) |
| `service/CouponTemplateSuspensionTest#disableTemplatesBySuspensionWithActiveTemplates`（`:31`） | 有 ACTIVE 模板時回傳停用數 | rewritten as suspension/unsuspension subscriber integration test (step B7-4; D8) |
| `…#disableTemplatesBySuspensionNoopWhenNoneEligible`（`:44`） | 無符合模板回 0 | rewritten as suspension/unsuspension subscriber integration test (step B7-4; D8) |
| `…#restoreTemplatesFromSuspensionRestoresFlaggedTemplates`（`:56`） | 還原有旗標的模板 | rewritten as suspension/unsuspension subscriber integration test (step B7-4; D8) |
| `…#restoreTemplatesFromSuspensionSkipsAdminDisabledTemplates`（`:69`） | 不還原非停權造成的停用 | rewritten as suspension/unsuspension subscriber integration test (step B7-4; D8) |
| `service/CouponTemplateServiceCompensationScopeTest#createCompensationTemplate_nullStore_rejectedAndNotPersisted`（`:54`） | 補償券 `applicable_store_id` 為 null 被拒、不落庫 | rewritten as compensation-grant integration test via `couponserver` incl. idempotency on `source_id` (step B7-4; D4) |
| `…#createCompensationTemplate_nonPositiveStore_rejected`（`:66`） | 非正整數被拒 | rewritten as compensation-grant integration test via `couponserver` incl. idempotency on `source_id` (step B7-4; D4) |
| `…#createCompensationTemplate_validStore_persistsScopedTemplate`（`:79`） | 合法 store 落庫並限店 | rewritten as compensation-grant integration test via `couponserver` incl. idempotency on `source_id` (step B7-4; D4) |
| `grpc/CouponGrpcServiceSellerIdentityTest#sellerCreateTemplate_usesRequestStoreId_notGrpcAuthContext`（`:76`） | 賣家建立用請求的 `store_id`，不讀 `GrpcAuthContext` | rewritten as Connect handler/characterization test, identity from session, module API calls, failures fail the request (step B7-3) |
| `…#sellerListTemplates_usesRequestStoreId_notGrpcAuthContext`（`:115`） | 同上，列表 | rewritten as Connect handler/characterization test, identity from session, module API calls, failures fail the request (step B7-3) |
| `…#sellerGetTemplateDetail_usesRequestStoreId_notGrpcAuthContext`（`:148`） | 同上，詳情 | rewritten as Connect handler/characterization test, identity from session, module API calls, failures fail the request (step B7-3) |
| `…#sellerGetTemplateDetail_rejectsAdminIssuedTemplate_evenWhenIssuerStoreIdCoincidentallyMatches`（`:192`） | admin 模板即使數值碰巧相符也回不存在 | rewritten as Connect handler/characterization test, identity from session, module API calls, failures fail the request (step B7-3) |
| `…#sellerRevokeTemplate_usesRequestStoreId_notGrpcAuthContext`（`:230`） | 同上，收回 | rewritten as Connect handler/characterization test, identity from session, module API calls, failures fail the request (step B7-3) |
| `…#listBuyerCouponsForSeller_usesRequestStoreId_notGrpcAuthContext`（`:259`） | 同上，查買家持券 | deleted: RPC deleted (BD33) |
| `…#adminUpdateSellerQuota_readsStoreIdField`（`:289`） | 配額更新讀 `store_id` 欄位 | rewritten as Connect handler/characterization test, identity from session, module API calls, failures fail the request (step B7-3) |
| `grpc/CouponGrpcServiceBuyerIdentityTest#listMyCoupons_usesRequestFields_notGrpcAuthContext`（`:68`） | 買家 RPC 讀 request 的 `buyer_id`／`account_id`，不讀 context | rewritten as Connect handler/characterization test, identity from session, module API calls, failures fail the request (step B7-3) |
| `…#listAvailableCoupons_splitsAccountIdAndBuyerIdToCorrectDownstreamClients`（`:102`） | `account_id` 給 Account、`buyer_id` 給 Order | rewritten as Connect handler/characterization test, identity from session, module API calls, failures fail the request (step B7-3) |
| `…#buyerClaimCoupon_usesRequestFields_notGrpcAuthContext`（`:143`） | 領券讀 request 欄位 | rewritten as Connect handler/characterization test, identity from session, module API calls, failures fail the request (step B7-3) |
| `grpc/CouponGrpcServiceIssuerIdentityResponseTest#adminTemplate_populatesIssuerAccountId_leavesIssuerStoreIdEmpty`（`:105`） | proto 回應 admin 模板只填 `issuer_account_id` | rewritten as Connect handler/characterization test, identity from session, module API calls, failures fail the request (step B7-3) |
| `…#sellerTemplate_populatesIssuerStoreId_leavesIssuerAccountIdEmpty`（`:126`） | 賣家模板只填 `issuer_store_id` | rewritten as Connect handler/characterization test, identity from session, module API calls, failures fail the request (step B7-3) |
| `grpc/CouponGrpcServiceStoreCodeTest#listMyCoupons_carriesSellerStoreCode`（`:82`） | `store_code` 隨賣家 meta 帶出 | rewritten as Connect handler/characterization test, identity from session, module API calls, failures fail the request (step B7-3) |
| `…#listMyCoupons_leavesStoreCodeBlankWhenSellerLookupDegrades`（`:166`） | 賣家查詢降級時 `store_code` 為空字串 | deleted: silent degrade removed, read failure fails the request (D3, BD28) |
| `grpc/CouponGrpcServiceSelfIssueTest#grandfathered_differentSellerAndBuyer_allowed`（`:21`） | 不同人允許 | rewritten as Connect handler/characterization test, identity from session, module API calls, failures fail the request (step B7-3) |
| `…#grandfathered_sellerTargetsSelf_blocked`（`:32`） | 發給自己被擋 | rewritten as Connect handler/characterization test, identity from session, module API calls, failures fail the request (step B7-3) |
| `…#diverged_sellerSideOnly_storeIdResolvesToOwnPrincipal_stillBlocked`（`:44`） | 賣家側分岔仍被擋 | rewritten as Connect handler/characterization test, identity from session, module API calls, failures fail the request (step B7-3) |
| `…#diverged_buyerSideOnly_buyerIdResolvesToSellerOwnAccountId_stillBlocked`（`:72`） | 買家側分岔仍被擋 | rewritten as Connect handler/characterization test, identity from session, module API calls, failures fail the request (step B7-3) |
| `…#diverged_bothSidesButDifferentAccounts_allowed`（`:85`） | 兩側都分岔但不同人允許 | rewritten as Connect handler/characterization test, identity from session, module API calls, failures fail the request (step B7-3) |
| `…#sellerResolvePrincipalsRpcFails_propagatesForOuterCatchAllToHandle`（`:100`） | 賣家側 RPC 失敗往外拋 | rewritten as Connect handler/characterization test, identity from session, module API calls, failures fail the request (step B7-3) |
| `…#buyerResolvePrincipalsRpcFails_propagatesForOuterCatchAllToHandle`（`:112`） | 買家側 RPC 失敗往外拋 | rewritten as Connect handler/characterization test, identity from session, module API calls, failures fail the request (step B7-3) |
| `…#sellerStoreUnresolved_failsClosed`（`:136`） | 賣家解析不到拒絕發券 | rewritten as Connect handler/characterization test, identity from session, module API calls, failures fail the request (step B7-3) |
| `…#designatedBuyerUnresolved_failsClosed`（`:150`） | 買家解析不到拒絕發券 | rewritten as Connect handler/characterization test, identity from session, module API calls, failures fail the request (step B7-3) |
| `grpc/CouponGrpcServiceTargetAccountLookupTest#sellerGetTemplateDetail_resolvesDivergedBuyerIdToCurrentAccountId_beforeAccountLookup`（`:117`） | 詳情先把 buyer_id 解析成 account_id 再查帳號 | rewritten as Connect handler/characterization test, identity from session, module API calls, failures fail the request (step B7-3) |
| `…#sellerGetTemplateDetail_doesNotLookUpAccountByBuyerId_whenUnresolved`（`:156`） | 解析不到不拿 buyer_id 查帳號 | rewritten as Connect handler/characterization test, identity from session, module API calls, failures fail the request (step B7-3) |
| `…#sellerGetTemplateDetail_doesNotLookUpAccountByBuyerId_whenResolvePrincipalsThrows`（`:182`） | 解析 RPC 例外時不查帳號 | rewritten as Connect handler/characterization test, identity from session, module API calls, failures fail the request (step B7-3) |
| `…#sellerGetTemplateDetail_deletedBuyerShowsTombstone_notBlank`（`:207`） | 已註銷買家顯示墓碑字串 | rewritten as Connect handler/characterization test, identity from session, module API calls, failures fail the request (step B7-3) |
| `…#sellerGetTemplateDetail_nicknameMustNotComeFromAccountInfo`（`:232`） | 暱稱不得取自 AccountInfo | rewritten as Connect handler/characterization test, identity from session, module API calls, failures fail the request (step B7-3) |
| `grpc/client/GrpcTargetsConfigTest#applicationYml_declaresAllRequiredGrpcTargets`（`:48`） | `application.yml` 宣告 seller／account／order／buyer 四個 target（**讀 yml 檔**，不是 integration） | deleted: gRPC targets config goes with the service |
| `scheduler/CouponExpiringWarningJobTest#execute_publishesResolvedNotifyBuyerAccountId_whenDiverged`（`:55`） | 預警事件帶解析出的 account | rewritten as River periodic job test with fake clock across UTC day boundary (step B7-4; D7) |
| `…#execute_publishesZeroNotifyBuyerAccountId_whenResolveThrows`（`:79`） | 解析例外時帶 0 | deleted: in-process event carries no resolved account, notification resolves (D7, A8) |
| `…#execute_publishesZeroNotifyBuyerAccountId_whenBuyerIsUnresolved`（`:99`） | 解析不到時帶 0 | deleted: in-process event carries no resolved account, notification resolves (D7, A8) |

加總：3（IT）＋56（上表：2+4+3+2+2+2+2+4+3+7+3+2+2+9+5+1+3＝56）＝59。

## 4. 其他服務中與 coupon 耦合的測試（不在 coupon-service，列出來供 B7 characterization 對照；未跑）

| class#method | what it asserts【推】 | 去向 |
|---|---|---|
| `backend/services/java/aftersales-service/src/test/java/io/github/will413028/hawkr/aftersales/service/AfterSaleResolutionServiceTest#couponCompensation_grantFailed_throwsAndDoesNotCloseCase`（`:100`） | coupon 回非 0 時拋 `AFTERSALE_COUPON_COMPENSATION_FAILED`，案件不關閉 | rewritten as Java aftersales unit test passing `source_id` (step B7-4; D4) |
| `…AfterSaleResolutionServiceTest#couponCompensation_grantSuccess_insertsAndClosesCase`（`:121`） | 發券成功後寫決議並關案 | rewritten as Java aftersales unit test passing `source_id` (step B7-4; D4) |
| `backend/services/java/order-service/src/test/java/io/github/will413028/hawkr/order/worker/TaskWorkerClaimLeaseTest.java`（24 行 coupon 字樣） | 核銷 worker 的 claim 租約行為（`CouponConsumptionWorker`，`CLAIM_LEASE_SEC=135`） | kept as is (Java order worker lease behavior, unchanged until B8) |
| `backend/services/java/order-service/src/test/java/io/github/will413028/hawkr/order/service/OrderExpiryCancellationServiceTest.java`（8 行 coupon 字樣） | 逾時取消後釋放券的呼叫 | kept as is until B8; Java retry added in B7-4 per spikes.md gets its own unit test |
