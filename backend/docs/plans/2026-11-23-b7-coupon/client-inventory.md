# B7-0 呼叫端盤點：誰在用 coupon-service（唯讀，快照 dev `df21e0d8a`）

標記：【量】＝跑過指令；【推】＝讀程式推論。路徑縮寫：`G=backend/gateway/internal`、`O=backend/services/java/order-service/src/main/java/io/github/will413028/hawkr/order`、`A=backend/services/java/aftersales-service/src/main/java/io/github/will413028/hawkr/aftersales`、`M=backend/internal`。「去向」欄全部留空。所有指令從 `/Users/will/second-brain/.projects/personal/hawkr`（repo 根目錄）執行。

## 0. 產生數字的指令與命中數

| # | 指令 | 命中 | 說明 |
|---|---|---|---|
| C1 | `rg -il coupon backend/gateway --glob '*.go' \| wc -l` | 23 檔 | gateway 含 coupon 字樣的 Go 檔（含 test） |
| C2 | `rg -i coupon backend/gateway --glob '*.go' --glob '!*_test.go' \| wc -l` | 260 行 | 非 test；其中 `handler/coupon.go` 132、`grpc/coupon_client.go` 41、`router/routes_coupon.go` 28、`router/constants.go` 17（`rg -c -i coupon backend/gateway --glob '*.go'` 的前幾名） |
| C3 | `rg -il coupon backend/internal backend/cmd --glob '*.go' --glob '!**/gen/**' --glob '!*_test.go' --glob '!**/codemeta/**' \| wc -l` | 9 檔 | 單體端；逐檔看過，全是名稱、註解或 `CouponApplied` 欄位，沒有任何 coupon RPC 呼叫（§4） |
| C4 | `rg -n 'CouponServiceGrpc' backend/services --glob '*.java' --glob '!**/target/**'`（排除 coupon-service 自己） | 2 檔 | 只有 `O/checkout/client/CheckoutCouponGrpcClient.java:41` 與 `A/grpc/client/CouponGrpcClient.java:39` |
| C4b | `rg -il coupon backend/services/java/cart-service --glob '*.java'` | 0 檔 | cart-service 完全沒有 coupon 引用 |
| C4c | `rg -il coupon backend/services/java --glob '*.java' --glob '!**/target/**' --glob '!coupon-service/**' --glob '!shared/**' \| wc -l` | 122 檔 | 名稱等級的引用（含 test、shipping/product 的 enum 與 schema 測試）；真正呼叫 coupon RPC 的只有 C4 的 2 檔，其餘見 §5 |
| C5 | `rg -n "app/coupons\|coupon_templates\|admin/coupons\|coupon_quotas" website --glob '!node_modules' --glob '!.next' \| wc -l` | 21 行 | website REST 路徑字面值（含 test 與註解） |
| C6 | 同上換成 `admin` | **0 行** | admin 前端沒有呼叫任何 coupon 端點 |
| C7 | `rg -n "app/coupons\|coupon_templates" buyer-app/lib --glob '!*.g.dart' \| wc -l` | 12 行 | buyer-app（含註解） |
| C8 | `rg -n "app/coupons\|coupon_templates\|admin/coupons" seller-app/lib --glob '!*.g.dart' \| wc -l` | 7 行 | seller-app（含註解） |
| C9 | `rg -n -i coupon packages --glob '!node_modules' \| wc -l` | 10 行 | 全部在 `packages/*/gen/hawkr/buyer/v1alpha1/buyer*` 生成檔（`coupon_applied` 欄位，`api/hawkr/buyer/v1alpha1/buyer.proto:293`）；沒有 coupon 的 proto 契約，也沒有手寫呼叫 |
| C10 | `rg -n "couponTemplateId" website buyer-app/lib seller-app/lib admin --glob '!node_modules' --glob '!.next' --glob '!*.g.dart' \| wc -l` | 9 行 | 結帳請求夾帶 coupon 的 client 端點 |
| C11 | `rg -n 'operationId' backend/api/openapi/openapi.yaml \| rg -i coupon` | 13 行（另有 2 個 operationId 不含 coupon 字樣：`adminDistributeTemplate`、`adminGrantCompensation`，以路徑確認） | OpenAPI coupon 操作 15 個（§3） |
| C12 | `rg -c -i coupon backend/api/openapi/openapi.yaml` | 84 | 檔內含 coupon 字樣的行數 |
| C13 | `rg -n 'CouponServiceGrpc\|pb/coupon\|proto/coupon' backend --glob '*.go' -l \| rg -v 'pb.go'` | 5 檔 | `gateway/internal/grpc/testing.go`、`gateway/internal/grpc/coupon_client.go`、`gateway/internal/handler/coupon.go`、`coupon_test.go`、`seller_store_code_test.go`；單體沒有 coupon client（`ls backend/internal/legacy` 沒有 `couponservice`） |

## 1. gateway REST 路由（`G/router/routes_coupon.go`，共 15 條；路徑常數在 `G/router/constants.go:262-290`）

app group 前綴 `/api/v1/app`、admin group 前綴 `/api/v1/admin`（OpenAPI 路徑可佐證，§3）。gateway 在每條路由先解析身分再呼叫 gRPC：買家用 `resolveBuyerIDFor`（account→現任 buyer_id）、賣家用 `resolveStoreIDFor`（account→store_id）、admin 帶 JWT account_id；admin 路由由 `requireAdmin` group 層保護（`routes_coupon.go:53` 註解）。

| route/call | caller file | coupon RPC used | 去向 |
|---|---|---|---|
| `GET /api/v1/app/coupons`（無 view＝買家；`?view=seller`＝賣家看買家持券）；`JWTAuth`＋`sellerViewGuard(CapOrderFulfill)`；賣家分支呼叫前 `resolveStoreIDFor`，買家分支要求 `IsBuyer(accountType)` | `G/router/routes_coupon.go:26`；`G/handler/coupon.go:260` `GetCoupons`、`:290` `handleListMyCoupons`（RPC 在 `:296`）、`:332` `handleListBuyerCouponsForSeller`（RPC 在 `:343`）；`G/grpc/coupon_client.go:50`、`:101` | ListMyCoupons／ListBuyerCouponsForSeller | |
| `GET /api/v1/app/coupons/available`（`store_id`、`order_amount` 必填；**只有 JWTAuth，沒有 IsBuyer 檢查**） | `routes_coupon.go:27`；`handler/coupon.go:391`（RPC 在 `:410`）；`grpc/coupon_client.go:64` | ListAvailableCoupons | |
| `POST /api/v1/app/coupons/{template_id}/claim`（要求 IsBuyer） | `routes_coupon.go:29`；`handler/coupon.go:452`（RPC 在 `:476`）；`grpc/coupon_client.go:57` | BuyerClaimCoupon | |
| `POST /api/v1/app/coupon_templates`（`sellerHandle(CapMarketing)`；body 可帶 `buyerAccountIds`（經 `CurrentBuyerID` 轉 buyer_id）與 `buyerIds`（只驗正整數），合併去重） | `routes_coupon.go:43`；`handler/coupon.go:511`（RPC 在 `:549`）；`grpc/coupon_client.go:73` | SellerCreateTemplate | |
| `GET /api/v1/app/coupon_templates`（`CapMarketing`） | `routes_coupon.go:45`；`handler/coupon.go:593`（RPC 在 `:604`）；`grpc/coupon_client.go:80` | SellerListTemplates | |
| `GET /api/v1/app/coupon_templates/{template_id}`（`CapMarketing`） | `routes_coupon.go:47`；`handler/coupon.go:640`（RPC 在 `:657`）；`grpc/coupon_client.go:87` | SellerGetTemplateDetail | |
| `POST /api/v1/app/coupon_templates/{template_id}/revoke`（只有 JWTAuth，不掛 capability，「收縮型，停權仍可自清」） | `routes_coupon.go:49`；`handler/coupon.go:694`（RPC 在 `:716`）；`grpc/coupon_client.go:94` | SellerRevokeTemplate | |
| `POST /api/v1/admin/coupon_templates` | `routes_coupon.go:58`；`handler/coupon.go:743`（RPC 在 `:782`；`buyerAccountIds` 已經過 `resolveTargetBuyerIDs`，V4 migration 註解說沒有，現已補上）；`grpc/coupon_client.go:110` | AdminCreateTemplate | |
| `POST /api/v1/admin/coupon_templates/{template_id}/publish` | `routes_coupon.go:60`；`handler/coupon.go:827`（RPC 在 `:836`）；`grpc/coupon_client.go:117` | AdminPublishTemplate | |
| `POST /api/v1/admin/coupon_templates/{template_id}/disable` | `routes_coupon.go:62`；`handler/coupon.go:859`（RPC 在 `:868`）；`grpc/coupon_client.go:124` | AdminDisableTemplate | |
| `POST /api/v1/admin/coupon_templates/{template_id}/issue`（`buyerAccountIds` 必填，經 `resolveTargetBuyerIDs`；回應只在 `immediateDistribute=true` 時帶 `distributedCount`） | `routes_coupon.go:64`；`handler/coupon.go:892`（RPC 在 `:915`）；`grpc/coupon_client.go:131` | AdminIssueCoupons | |
| `POST /api/v1/admin/coupon_templates/{template_id}/distribute` | `routes_coupon.go:66`；`handler/coupon.go:950`（RPC 在 `:959`）；`grpc/coupon_client.go:138` | AdminDistributeTemplate | |
| `POST /api/v1/admin/coupons/compensations`（`buyerAccountId`、`amount`、`applicableStoreId`、`reason` 必填；`buyerAccountId` 經 `resolveTargetBuyerID`） | `routes_coupon.go:75`；`handler/coupon.go:987`（RPC 在 `:1012`）；`grpc/coupon_client.go:145` | AdminGrantCompensationCoupon | |
| `POST /api/v1/admin/coupons/{instance_id}/void` | `routes_coupon.go:77`；`handler/coupon.go:1049`（RPC 在 `:1066`）；`grpc/coupon_client.go:152` | AdminVoidInstance | |
| `PUT /api/v1/admin/coupon_quotas/{store_id}` | `routes_coupon.go:86`；`handler/coupon.go:1091`（RPC 在 `:1108`）；`grpc/coupon_client.go:159` | AdminUpdateSellerQuota | |

其他 gateway 細節：
- `G/router/constants.go:272` 的 `CouponsReceive = "/:template_id/receive"` 沒有任何路由使用（`rg -n CouponsReceive backend/gateway` 只命中定義）；buyer-app 的 `wallet_repository.dart:38` 註解仍寫 `/receive`，實作已用 `/claim`（`wallet_remote_service.dart:42`）。
- gRPC 連線：`G/grpc/coupon_client.go:22-23` 讀 `COUPON_SERVICE_HOST`（預設 127.0.0.1）、`COUPON_SERVICE_GRPC_PORT`（預設 9951）；`G/router/constants.go:487` `ServiceCoupon = "coupon"`。
- 21 個 coupon-service RPC 中，gateway 的 client 只包 16 支（`coupon_client.go` 16 個方法）；沒有被 gateway 包的 5 支是 ValidateCoupon、ReserveCoupon、ConsumeCoupon、ReleaseCoupon、GrantSellerCompensationCoupon，這 5 支只有 Java 服務呼叫（§5）。
- 回應轉換：`templateInfoToMap`（`handler/coupon.go:1123-1166`）、`instanceInfoToMap`（`:1168-1190`）把 proto 欄位映射成 REST JSON，並用 `h.MediaBaseURL` 組賣家 logo URL。

## 2. gateway 其他和 coupon 有關的檔案（不呼叫 coupon RPC，只是透傳字串或鍵名）

| route/call | caller file | coupon RPC used | 去向 |
|---|---|---|---|
| 結帳 DTO 的 `couponTemplateId` 欄位：`CheckoutPreviewRequest`（`:339-349`）、`CheckoutConfirmRequest`（`:351-366`），兩個 handler 內的匿名 request struct（`:715`、`:802`），組 proto 時 `CouponTemplateId: req.CouponTemplateID`（`:736`、`:831`）；路由 `POST /app/checkout/preview`（`:696`）、`POST /app/checkout/confirm`（`:776`）。優惠券的 validate／reserve 都在 order-service，不在 gateway | `G/handler/checkout.go:345`、`:365`、`:715`、`:736`、`:802`、`:831` | 無（交給 order-service 的 Preview／Confirm，再由它呼叫 ValidateCoupon／ReserveCoupon） | |
| 售後決策 `POST …/after_sales/{case}/resolve` 的 `couponAmount`、`couponMinSpend`、`couponMessage`（resolutionType=4）；`ResolveAfterSaleCaseRequest`（`:30-38`）、`ResolveResultResponse`（`:160-162`）、轉 proto（`:727-729`）；路由 `routes_aftersales.go:42`（`CapOrderFulfill`） | `G/handler/aftersales.go:34-36`、`:160-162`、`:270-272`、`:727-729` | 無（交給 aftersales-service，再由它呼叫 GrantSellerCompensationCoupon） | |
| 訂單回應的 `couponInstanceId` | `G/handler/order.go:40`、`:296` | 無 | |
| 通知偏好鍵 `coupon`（與 `coupon.expiring`／`coupon.issued` 事件對應的偏好類別） | `G/handler/notification_preference.go:198` | 無 | |
| 開發用清快取：`coupon` → `coupon:tpl:*`、`coupon:avail:*` | `G/cache/redis.go:21`、`G/handler/dev_cache.go:13-40` | 無（直接動 coupon-service 的 Redis key） | |
| 日誌遮罩白名單鍵（`coupontemplateid`、`couponamount`、`couponcount`、`couponminspend`） | `G/middleware/logger.go:189`、`:215` | 無 | |

## 3. OpenAPI（`backend/api/openapi/openapi.yaml`，15 個 coupon 操作、14 個路徑）

指令：`rg -n -i coupon backend/api/openapi/openapi.yaml`（84 行）；路徑行號如下。

| route/call | caller file | coupon RPC used | 去向 |
|---|---|---|---|
| `adminUpdateCouponQuota`：`PUT /api/v1/admin/coupon_quotas/{store_id}` | `openapi.yaml:1088-1090` | AdminUpdateSellerQuota | |
| `adminCreateCouponTemplate`：`POST /api/v1/admin/coupon_templates` | `openapi.yaml:1135-1137` | AdminCreateTemplate | |
| `adminDisableCouponTemplate`：`POST …/{template_id}/disable` | `openapi.yaml:1183-1185` | AdminDisableTemplate | |
| `adminDistributeTemplate`：`POST …/{template_id}/distribute`（描述寫「為所有已發行但未領取的目標帳號自動建立券實例」，實作只寫 `coupon_claims`、不建 instance） | `openapi.yaml:1222-1226` | AdminDistributeTemplate | |
| `adminIssueCouponsHandler`：`POST …/{template_id}/issue` | `openapi.yaml:1269-1271` | AdminIssueCoupons | |
| `adminPublishCouponTemplate`：`POST …/{template_id}/publish` | `openapi.yaml:1318-1320` | AdminPublishTemplate | |
| `adminGrantCompensation`：`POST /api/v1/admin/coupons/compensations`（operationId 與 handler 名 `AdminGrantCompensation` 一致） | `openapi.yaml:1363-1366` | AdminGrantCompensationCoupon | |
| `adminVoidCouponInstance`：`POST /api/v1/admin/coupons/{instance_id}/void`（`handler/coupon.go:1039` 的描述寫 status 0／1 都可作廢，SQL 只允許 0） | `openapi.yaml:1405-1407` | AdminVoidInstance | |
| `listCouponTemplates`：`GET /api/v1/app/coupon_templates` | `openapi.yaml:10646-10648` | SellerListTemplates | |
| `createCouponTemplate`：`POST /api/v1/app/coupon_templates` | `openapi.yaml:10714` | SellerCreateTemplate | |
| `getCouponTemplateDetail`：`GET /api/v1/app/coupon_templates/{template_id}` | `openapi.yaml:10760-10762` | SellerGetTemplateDetail | |
| `revokeCouponTemplate`：`POST …/{template_id}/revoke` | `openapi.yaml:10813-10815` | SellerRevokeTemplate | |
| `getCoupons`：`GET /api/v1/app/coupons` | `openapi.yaml:10865-10867` | ListMyCoupons（`view=seller` 分支＝ListBuyerCouponsForSeller） | |
| `listAvailableCoupons`：`GET /api/v1/app/coupons/available` | `openapi.yaml:10949-10951` | ListAvailableCoupons | |
| `claimCoupon`：`POST /api/v1/app/coupons/{template_id}/claim` | `openapi.yaml:11021-11023` | BuyerClaimCoupon | |

OpenAPI 另有以下 schema 含 coupon：`AdminCreateCouponTemplateRequest`（`:20988`）、`AdminIssueCouponsRequest`／`Response`（`:21157`、`:21169`）、`AdminUpdateCouponQuotaRequest`（`:21904`）、`AdminVoidCouponRequest`（`:21911`）、`ClaimCouponResponse`（`:23096`）、`CouponInstanceResponse`（`:23183`）、`CouponTemplateDetailResponse`／`CouponTemplateResponse`（`:23227`、`:23236`）、`CreateCouponTemplateRequest`（`:23374`）；結帳與售後請求的 `couponTemplateId`（`:22767`、`:22995`）、`couponInstanceId`（`:25286`）、`couponAmount`／`couponMinSpend`／`couponMessage`（`:27692-27696`）。刪路由時要同步改（`scripts/gateway-json-contract-check.py`、`gateway/internal/router/openapi_coverage_test.go`，見 `backend/AGENTS.md`）。

## 4. 單體（`backend/internal`、`backend/cmd`）既有的 coupon 接點

結論：單體沒有 coupon 模組、沒有 coupon legacy client、沒有任何 coupon RPC 呼叫【量】：`ls backend/internal/modules` 沒有 `coupon`；`ls backend/internal/legacy` 沒有 `couponservice`。

| route/call | caller file | coupon RPC used | 去向 |
|---|---|---|---|
| 依賴表已預留：`coupon` 依賴 identity、buyer、seller；`ordering` 依賴 cart、coupon、…；`aftersales` 依賴 ordering、coupon、… | `M/arch/modules.go:25-27`；模組清單 `:44` | 無（宣告而已） | |
| lockorder 全域順序已含 `"coupon"` | `M/platform/lockorder/lockorder.go:45` | 無 | |
| `CouponApplied` 欄位（買家消費紀錄：`discount > 0` 推得，不呼叫 coupon） | `M/legacy/orderservice/orderservice.go:39`、`:165`；`M/httpapi/buyer/ports.go:35`；`M/httpapi/buyer/membership.go:205`；`backend/cmd/hawkr/buyer_ports.go:46`；對外契約 `api/hawkr/buyer/v1alpha1/buyer.proto:293` | 無 | |
| 事件轉發註解（單體把 seller 事件轉到 RabbitMQ 給 Java coupon 等消費者；coupon 搬進單體後這條轉發要改成模組事件） | `backend/cmd/hawkr/seller_events.go:25`；`M/legacy/sellerrelay/relay.go:14` | 無（轉發 `seller.suspended`／`seller.unsuspended`，coupon-service 是消費者） | |
| `M/legacy/accountservice/server.go:2` 註解把 coupon 列為 account legacy server 的呼叫方之一 | 同左 | 無（coupon-service 呼叫 `AccountService.GetAccountInfo`、`GetAccountInfoBatch` 由此實作） | |
| `buyer.deleted` 的來源：buyer 模組事件經 `M/legacy/buyerrelay` 轉 RabbitMQ 【推，未讀 relay 內容】 | `backend/internal/legacy/buyerrelay`（目錄存在） | 無（coupon-service 消費） | |
| 單體 CRM 模組：優惠券數不做、不建 coupon legacy client（W3） | `backend/internal/modules/crm/CARRYOVER.md:23` | 無（原本 crm-service 呼叫 `ListBuyerCouponsForSeller`，A6 已刪該需求；`CSV/rpc-calls.csv:80` 的 crm→coupon 列已過期） | |
| banner 動作型別 `coupon`（測試輸入） | `M/modules/content/internal/banner/banner_test.go:32` | 無 | |
| 碼值 enum（CodeMeta 生成物，含 CouponTemplateStatus、CouponInstanceStatus、CouponType 等） | `M/platform/codemeta/code_meta.go:365-452`、`:2861-2890` | 無 | |

## 5. 其他 Java 服務呼叫 coupon-service

| route/call | caller file | coupon RPC used | 去向 |
|---|---|---|---|
| 結帳 Preview（驗券，不鎖） | `O/checkout/CheckoutService.java:692-694`（呼叫 `CheckoutCouponGrpcClient.validateCoupon`，`O/checkout/client/CheckoutCouponGrpcClient.java:57-84`；`valid=false` 時拋 `COUPON_NOT_AVAILABLE`，連線例外往上拋，硬依賴） | ValidateCoupon | |
| 結帳 Confirm 建單前（再驗一次） | `O/checkout/CheckoutService.java:952-955`；折扣取回後做 `allocateShareDiscount`（`:958`）與運費減免判定 | ValidateCoupon | |
| 結帳 Confirm 建單後鎖券：`createOrderFromCheckout` 成功後呼叫 `reserveCoupon(templateId,buyerId,orderNo)`，成功才 `updateCouponInstanceId(orderNo,instanceId)`；**任何例外**→`cancelOrder`＋`releaseStockAfterCancel` 補償（補償本身失敗只記 log）→拋 `COUPON_LOCK_FAILED` | `O/checkout/CheckoutService.java:1073-1090`；`O/checkout/client/CheckoutCouponGrpcClient.java:92-114`；`O/service/OrderService.java:600-616`（`updateCouponInstanceIdIfAbsent` CAS） | ReserveCoupon | |
| 付款成功後建立核銷任務：`PaymentEventHandler` 在處理付款事件時，若訂單有 `coupon_instance_id` 就插入 `coupon_consumption_tasks`（status PENDING，`next_retry_time=now`） | `O/mq/PaymentEventHandler.java:196-211`；表在 order-service 的 DB（`CouponConsumptionTaskEntity`、`CouponConsumptionTaskRepository`） | 無（只寫任務） | |
| 核銷 worker：每 45 秒（`fixedDelay=45000, initialDelay=20000`）撈任務、claim 租約 135 秒，呼叫 `consumeCoupon`；呼叫成功就 `markDone`；超過 `max_retry_count` 轉人工；`CheckoutCouponGrpcClient.consumeCoupon` **自己 try/catch 吞掉所有例外與 `success=false`**，所以 worker 的 `catch` 永遠進不去，任務一律被標 DONE（見 defects.md） | `O/worker/CouponConsumptionWorker.java:49-96`；`O/checkout/client/CheckoutCouponGrpcClient.java:125-153` | ConsumeCoupon | |
| 買家取消訂單：`cancelOrder` 後 best-effort `releaseCouponForOrder` | `O/grpc/OrderGrpcService.java:498`、`:1452-1463` | ReleaseCoupon | |
| 售後內部取消訂單（CAS 成功後）：`releaseCouponForOrder` | `O/grpc/OrderGrpcService.java:2085`（同 `:1452`） | ReleaseCoupon | |
| 逾時取消訂單：`OrderExpiryCancellationService` 取消後 best-effort 釋放，註解寫「失敗由 Coupon AutoReleaseJob 兜底」 | `O/service/OrderExpiryCancellationService.java:168-175`；取消資訊由 `O/mq/PaymentEventHandler.java:395-410`（`CancelInfo` 帶 `couponInstanceId`）提供 | ReleaseCoupon | |
| 發票快照：`discount_funding` 恆為空字串，因為 `CouponValidateResult` 不帶 `coupon_templates.issuer_type`（58 號 U-7）；呼叫端要 fail-closed | `O/grpc/OrderGrpcService.java:2148-2152`（註解） | 無（缺少來自 coupon 的欄位） | |
| 賣家停權連動（order-service 取消賣家 PENDING_PAYMENT 訂單；與 coupon-service 的模板停用是同一事件的兩個消費者） | `O/mq/SellerSuspendedConsumer.java:30-36`（註解）、`O/config/RabbitMqConfig.java` | 無（但 `seller.suspended` 後被取消的訂單會經 ReleaseCoupon 路徑還券，【推】） | |
| 售後補償：賣家決策 resolutionType=4 時，`handleCouponCompensation` 在 `@Transactional` 內呼叫 coupon，檢查 `resp.code`（≠0 拋 `AFTERSALE_COUPON_COMPENSATION_FAILED`），成功後 INSERT `after_sale_resolutions`、`closeByCouponCompensation`（PENDING→CLOSED）、寫稽核、發 `aftersales.case.coupon_compensated` 事件；gRPC deadline 10 秒；`couponMessage` 傳過去但 coupon-service 不讀 | `A/service/AfterSaleResolutionService.java:334-398`；`A/grpc/client/CouponGrpcClient.java:27-48`；入口 `A/grpc/AfterSalesGrpcService.java:396-409` | GrantSellerCompensationCoupon | |
| 對帳（由 coupon-service 呼叫 order-service，反向依賴） | `backend/services/java/coupon-service/src/main/java/io/github/will413028/hawkr/coupon/scheduler/AutoReleaseJob.java:74`；`…/grpc/client/OrderGrpcClient.java:81-96` | 無（呼叫 Order.GetOrderForAfterSales；搬進單體後是 coupon→ordering 的反向依賴，與 `M/arch/modules.go:26` 的 `ordering→coupon` 方向相反） | |
| MEMBER 資格（coupon-service 呼叫 order-service） | `…/coupon/grpc/client/OrderGrpcClient.java:45-72` | 無（呼叫 Order.GetBuyerMerchantSummary，同上反向依賴） | |
| 單元測試中的耦合（只改變期望值，不是 production 呼叫） | `O/…/worker/TaskWorkerClaimLeaseTest.java`（24 行 coupon）、`OrderExpiryCancellationServiceTest.java`、`A/…/AfterSaleResolutionServiceTest.java:100`、`:121`（grantFailed／grantSuccess 兩個 coupon 測試）等 | 無 | |
| shipping-service／product-service 的 coupon 字樣 | `backend/services/java/shipping-service/src/main/java/io/github/will413028/hawkr/shipping/mapper/ShipmentAnomalyQueryMapper.java`、`backend/services/java/product-service/src/main/java/io/github/will413028/hawkr/product/mq/SellerExposureChangedConsumer.java` 等 | 無（欄位名或註解，非呼叫） | |

## 6. 前端與 App 的呼叫端（逐檔看過）

### 6.1 website

| route/call | caller file | coupon RPC used | 去向 |
|---|---|---|---|
| `GET /app/coupons/available?store_id&order_amount`（購物車結帳選券） | `website/lib/coupons/repo.ts:193`（`fetchAvailableCoupons`，URL 在 `:227`）；`website/lib/hooks/useCartCoupons.ts:48`（註解 `:40`）；用在 `website/features/cart/CartClient.tsx:564`；`website/features/account/screens/coupons/CouponsClient.tsx:23` 註解說明需要 store_id | ListAvailableCoupons | |
| `GET /app/coupons?status=`（買家券夾） | `website/lib/coupons/repo.ts:311`（`fetchMyCoupons`，URL 在 `:327`）；`website/lib/hooks/useAccount.ts:105`；`website/features/account/screens/coupons/CouponsClient.tsx:76` | ListMyCoupons | |
| `POST /app/coupons/{id}/claim` | `website/lib/coupons/repo.ts:339`（`claimCoupon`，URL 在 `:341`）；`website/lib/hooks/useAccount.ts:114` | BuyerClaimCoupon | |
| `GET /app/coupon_templates`（賣家列表） | `website/lib/seller/repo.ts:358`（`fetchSellerCoupons`，URL 在 `:370`）；`website/features/seller-center/coupons/SellerCouponsClient.tsx` | SellerListTemplates | |
| `GET /app/coupon_templates/{id}` | `website/lib/seller/repo.ts:378`（`fetchSellerCoupon`，URL 在 `:384`） | SellerGetTemplateDetail | |
| `POST /app/coupon_templates`（賣家建立；表單欄名為 snake，wire 為 camel，見 `repo.ts:217`、`:255`） | `website/lib/seller/repo.ts:437`（`createSellerCoupon`，URL 在 `:452`）；`website/features/seller-center/coupons/CouponFormClient.tsx`、`CouponBuyerPicker.tsx` | SellerCreateTemplate | |
| `POST /app/coupon_templates/{id}/revoke` | `website/lib/seller/repo.ts:481`（`deleteSellerCoupon`，URL 在 `:482`） | SellerRevokeTemplate | |
| 結帳請求夾 `couponTemplateId` | `website/lib/checkout/repo.ts:33`、`:399-400`；`website/features/checkout/screens/CheckoutClient.tsx:573`、`:763` | 無（透過 `/app/checkout/preview`、`/confirm`，由 order-service 呼叫 Validate／Reserve） | |
| 沒有任何 website 呼叫 `GET /app/coupons?view=seller`、`admin/*coupon*` | `rg -n "view.*seller" website/lib website/features \| rg -i coupon`＝0（`seller-center/customers/CustomerDetailPanel.tsx` 有 coupon 字樣，內容與此端點無關【推】） | 無 | |

### 6.2 admin

| route/call | caller file | coupon RPC used | 去向 |
|---|---|---|---|
| **沒有**呼叫任何 `/admin/coupon_templates`、`/admin/coupons`、`/admin/coupon_quotas` 的程式（C6＝0）；`admin/src/app/admin/notification-templates/page.tsx:88`、`:123` 只列 `coupon.issued` 通知模板名稱；`admin/src/features/invoices/admin-actions.ts:285`、`types.ts:667` 為發票註解；`admin/tests/dispute-detail-media-contract.test.ts` 為售後 fixture 的空字串欄位 | `admin/**`（`rg -c -i coupon admin --glob '!node_modules' --glob '!.next'` 共 4 檔） | 無（8 支 admin RPC 在 UI 端沒有呼叫端；實際操作靠 API 直打或尚無 UI【推】） | |

### 6.3 buyer-app（Flutter）

| route/call | caller file | coupon RPC used | 去向 |
|---|---|---|---|
| `GET /app/coupons`，`status` 為整數（0／2） | `buyer-app/lib/features/wallet/data/wallet_remote_service.dart:30-37`（`fetchCoupons`）；`wallet_repository.dart:17`；`wallet_controller.dart` | ListMyCoupons | |
| `GET /app/coupons?status=0`（首頁券數量） | `buyer-app/lib/features/home/presentation/home_screen.dart:86-117`（`_loadCouponCount`，取 `data.records` 的長度；`status=0` 只取可用券；訪客略過） | ListMyCoupons | |
| `GET /app/coupons`（分類落地頁券數量；取 `data['data']` 當 List，`data` 其實是 Map，所以數量會是 0【推：回應格式讀自 gateway `handleListMyCoupons`，未實際呼叫】） | `buyer-app/lib/features/category/presentation/category_landing_screen.dart:148-160` | ListMyCoupons | |
| `GET /app/coupons/available`（購物車選券，帶 `order_amount`、`store_id`、page=1、page_size=20） | `buyer-app/lib/features/cart/data/cart_repository.dart:355-372`；UI `cart/presentation/widgets/coupon_selection_sheet.dart`、`coupon_helpers.dart`、`payment_details_section.dart`、`cart_controller.dart` | ListAvailableCoupons | |
| `POST /app/coupons/{templateId}/claim` | `buyer-app/lib/features/wallet/data/wallet_remote_service.dart:39-46`；測試 `test/features/wallet/coupon_claim_endpoint_contract_test.dart`（守 `/claim`）、`test/architecture/dead_gateway_endpoint_guardrail_test.dart`（2 命中） | BuyerClaimCoupon | |
| `GET /app/coupon_templates`（買家端「模板列表」，page／page_size）：這是**賣家路由**（`CapMarketing`），買家呼叫會被拒；`fetchCouponTemplates` 在 lib 內沒有其他呼叫端（`rg -n "fetchCouponTemplates" buyer-app/lib`只命中定義與 repository 包裝） | `buyer-app/lib/features/wallet/data/wallet_remote_service.dart:48-60`；`wallet_repository.dart:53-59` | SellerListTemplates（用錯路由，疑似死碼【推】） | |
| 結帳請求夾 `couponTemplateId` | `buyer-app/lib/features/cart/presentation/widgets/checkout_payment_sheet.dart:224`、`:303`、`:420`、`:1193` | 無（`/app/checkout/preview`、`/confirm`） | |

### 6.4 seller-app（Flutter）

| route/call | caller file | coupon RPC used | 去向 |
|---|---|---|---|
| `GET /api/v1/app/coupon_templates` | `seller-app/lib/features/coupon/data/coupon_repository.dart:19`（`_basePath`）、`:26-28`（`getSellerCoupons`） | SellerListTemplates | |
| `GET /api/v1/app/coupon_templates/{id}` | `coupon_repository.dart:62-64`（`getCouponDetail`） | SellerGetTemplateDetail | |
| `POST /api/v1/app/coupon_templates` | `coupon_repository.dart:98-100`（`createCoupon`）；UI `coupon_editor_screen.dart`、`add_coupon_screen.dart`、`coupon_controller.dart` | SellerCreateTemplate | |
| `POST /api/v1/app/coupon_templates/{id}/revoke` | `coupon_repository.dart:141-160`（`deleteCoupon`，POST `$_basePath/$templateId/revoke`）；`coupon_detail_screen.dart:20` 註解寫「後端沒有更新模板端點」 | SellerRevokeTemplate | |
| 其他 coupon 字樣（錢包、儀表板、通知、icon）不呼叫 coupon 端點 | `seller-app/lib/features/wallet/presentation/wallet_coupon_section.dart`、`dashboard_screen.dart`、`notification/*` 等（`rg -n "app/coupons\|coupon_templates\|admin/coupons" seller-app/lib --glob '!*.g.dart'` 的 7 行全在 `coupon_repository.dart` 與 `coupon_detail_screen.dart`） | 無 | |

### 6.5 packages

| route/call | caller file | coupon RPC used | 去向 |
|---|---|---|---|
| `packages/hawkr-api-ts`、`packages/hawkr_api` 沒有 coupon 契約；只有 `buyer.proto` 的 `coupon_applied`（消費紀錄）生成碼 | `packages/hawkr-api-ts/src/gen/hawkr/buyer/v1alpha1/buyer_pb.ts`、`packages/hawkr_api/lib/gen/hawkr/buyer/v1alpha1/buyer.pb.dart`、`buyer.pbjson.dart` | 無 | |
