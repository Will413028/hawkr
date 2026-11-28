# B3-0 呼叫端盤點：誰在用 shipping-service（唯讀，快照 dev `496b048e2`）

標記：【量】＝跑過指令；【推】＝讀程式推論。路徑縮寫：`J=backend/services/java`、`O=J/order-service/src/main/java/io/github/will413028/hawkr/order`、`A=J/aftersales-service/src/main/java/io/github/will413028/hawkr/aftersales`、`P=J/product-service/src/main/java/io/github/will413028/hawkr/product`、`S=J/shipping-service/src/main/java/io/github/will413028/hawkr/shipping`、`G=backend/gateway/internal`、`CC=J/shared/common-core/src/main/java/io/github/will413028/hawkr/common`、`D=backend/docs`。「去向」欄一律留空，由決策簡報填。

## 0. 產生數字的指令與命中數

| # | 指令 | 命中 | 說明 |
|---|---|---|---|
| S1 | `rg -l 'ShippingGrpcClient' J --glob '*.java' --glob '!**/target/**' --glob '!**/test/**' --glob '!shipping-service/**'` | 14 檔 | 三個 client 類別本身（order、aftersales、product 各一）加使用它們的類別；cart、coupon、payment 沒有 shipping client |
| S2 | 同上但 `--glob '**/test/**'` | 39 檔 | 全是 mock 或 stub，B3 刪 client 時一起改 |
| S3 | `rg -n 'shippingGrpcClient\.[a-zA-Z]+\(' J --glob '*.java' --glob '!**/test/**' --glob '!**/target/**'` | 19 行 | 非 client 類別的主程式呼叫點，逐行列在 §1.1 |
| S4 | `rg -n '^\s+rpc ' J/shared/java-proto/src/main/proto/shipping/shipping.proto` | 52 條 RPC | `ShippingService`，proto 1592 行；`backend/api/proto/shipping/shipping.proto` 與 `backend/pkg/proto/shipping/` 是同一份的 Go 側副本 |
| S5 | `find S -name '*.java'`、`find J/shipping-service/src/test -name '*.java'` | 144 主程式、57 測試 | shipping-service 本身；Flyway migration 15 支，建 19 張表（§1.5） |
| S6 | `rg -l -i 'shipping\|shipment\|logistics' J/{order,aftersales,product,cart,coupon,payment}-service/src/main --glob '*.java'` | 88 檔 | 字樣命中，**含大量「收件地址」（`shipping_address`）誤中**，不等於呼叫端；分服務檔數 order 36、aftersales 24、product 22、coupon 5、cart 1、payment 0 |
| S7 | `rg -l -i uber J --glob '*.java' --glob '!**/target/**' --glob '!**/test/**'` | 105 檔 | Uber 字樣的主程式檔（全服務含 shared），本步刪除範圍（計畫 B3「刪除 Uber 即時配送」） |
| S8 | `rg -l -i 'shipping\|shipment' G --glob '*.go' --glob '!**_test.go'` | 46 檔 | gateway 非 test；分類見 §4 |
| S9 | `rg -n '(sellerHandle\|sellerHandleAuthed)\(\|\.(GET\|POST\|PUT\|DELETE)\(' G/router/routes_shipping.go` | 31 行 | `routes_shipping.go` 共 115 行，含賣家 11 條出貨動作與異常、5 條建店、買家 5 條門市、後台 6 條、2 條頂層門市回調 |
| S10 | `rg -n '^  /(api/v1/(admin\|app)/(shipments\|shipping_\|store_\|seller_shipment)\|store_map)' backend/api/openapi/openapi.yaml` | 30 條 path | 另有 `/api/v1/app/checkout/shipping_fee`、`/api/v1/app/orders/{order_no}/logistics`、`/api/v1/admin/after_sales/{case_no}/retry_print_return_label` 三條不在此型樣內（§4.4） |
| S11 | `rg -l -i 'shipping\|shipment\|logistic' backend/internal backend/cmd --glob '*.go' --glob '!**/gen/**' --glob '!**_test.go' --glob '!**/codemeta/**' --glob '!**/*.sql.go' --glob '!**/uitext/**'` | 22 檔 | 單體端；逐檔看過，沒有任何 shipping gRPC 呼叫、沒有 `internal/modules/fulfillment`（§3） |
| S12 | `rg -l -i 'shipping\|shipment\|uber\|logistics\|tcat\|labelprint\|fulfillment' CC/utils/code` | 50 檔 | common-core 的碼值 enum 與 helper，CodeMeta 生成物（§5） |
| S13 | `rg -l "shipment\.(created\|delivered)\|hawkr.shipping" backend/services/notification-service/db/migrations` | 18 檔 | 通知模板綁在 `hawkr.shipping` exchange 上（§1.2 關鍵耦合 K3） |
| S14 | `rg -l -i 'shipping-service\|payuni-webhook\|uber' .github backend/docker-compose*.yml backend/Makefile backend/dev.sh backend/infra backend/scripts backend/monitoring backend/.env.template` | 8 檔 | `docker-compose-local.yml`、`infra/seed-data/06-shipping.sql`、`dev.sh`、`infra/mq/topology/topology.yaml`、`.env.template`、監控的 `load-balance.json`、`business-alerts.yml`、`service-alerts.yml`；`.github` 沒有命中（【量】） |
| S15 | `git log --diff-filter=D --name-only -- 'backend/docs/detailed-design/6[6-9]*' …` | 3 次刪除 | 設計書 66、68、69 在 `2c4928740`（2026-10-29）刪除，67 在 `a7d37b97e`，70 在 `d876df987`（2026-11-23）；現在只剩 65（§6） |

## 1. Java 端

### 1.1 shipping gRPC 呼叫點（三個 client、19 行呼叫）

| 呼叫端 | 位置 | RPC | 用途 | 去向 |
|---|---|---|---|---|
| order | `O/grpc/OrderGrpcService.java:135` | `GetShipmentsByOrderNos` | 買家訂單列表帶物流狀態 | |
| order | `O/grpc/OrderGrpcService.java:151` | `GetShipmentByOrderNo` | 訂單詳情帶物流資訊（追蹤號、狀態） | |
| order | `O/grpc/OrderGrpcService.java:282` | `BatchGetProviderStatusByOrderIds`（`isReturn` 參數） | 賣家訂單列表取 provider 狀態碼 | |
| order | `O/checkout/ShippingFeeGrpcFetcher.java:45` | `GetShippingFeeConfigs` | 全量拉運費級距與方式限制，進 `ShippingFeeCacheClient`（5 分鐘 TTL、`@CircuitBreaker shipping-fee-service`） | |
| order | `O/checkout/CheckoutService.java:996、1256、1282` | `GetSellerShippingConfig` | 結帳驗證賣家是否開通該配送方式（`requireSellerOnboarded`）與取寄件郵遞區號推導路線（`getSenderPostalCode`） | |
| order | `O/checkout/ShippingUberQuoteAssessor.java:65、89` | `GetUberQuoteForCheckout`、`ValidateUberQuoteForCheckout` | Uber 報價與驗證，整支刪 | |
| aftersales | `A/grpc/AfterSalesGrpcService.java:1161、1629、1766` | `BatchGetProviderStatusByOrderIds(…, true)` | 售後列表與詳情帶退貨物流狀態 | |
| aftersales | `A/service/AfterSaleResolutionService.java:158、168` | `CreateReturnCvsShipment`、`CreateReturnHomeShipment` | 平台建退貨物流單；B9 刪（買家自寄） | |
| aftersales | `A/service/AfterSaleLabelRetryService.java:114、220` | `RetryReturnPrintLabel` | 退貨標籤重試；B9 刪 | |
| aftersales | `A/grpc/client/ShippingGrpcClient.java:139` | `GetSellerReturnConfig` | client 有方法，主程式**零呼叫**（【量】`rg 'getSellerReturnConfig' A` 只有 client 自己） | |
| product | `P/grpc/ProductGrpcService.java:1395` | `CalculateShippingOptions` | 賣家建商品時的運費預覽（`/app/shipping_quotes`） | |
| product | `P/service/ProductService.java:1016` | `GetSellerShippingConfig` | 商品建立／更新時查賣家已開通的方式 | |
| product | `P/service/ProductService.java:2142` | `ValidateProductShipping` | 商品尺寸重量對所有已開通方式的限制（錯誤碼 4016） | |
| shipping 反向 | `S/client/{Order,Seller,Buyer,Media}GrpcClient.java` | `order.GetOrderForShipping`、`seller.GetSellerShippingProfile`（兩處）、buyer 批次查 meta、media | shipping 呼叫別人，非本盤點對象，但決定 Go fulfillment 要往哪些 port 讀 | |
| invoice-service（Go） | `backend/services/invoice-service/internal/client/shipping/client.go:89` | `GetShipmentsByOrderNos` | `BatchGetShipmentStatus`，發票用的出貨狀態；shipping 退場時要改接 | |

【量】cart-service、coupon-service、payment-service、media-service 主程式沒有 shipping RPC 呼叫（S1 的 14 檔不含它們）。【量】Go 側除 gateway 與 invoice-service 外沒有 import shipping pb（`rg -l 'shipping/grpc\|proto/shipping\|shippingpb' backend --glob '*.go'` 只有 gateway、invoice-service）。ai-service 的 `shipping_method`、`shipping_status` 取自 order 的詳情，不是 shipping RPC。

### 1.2 訂單與配送單的狀態耦合（關鍵耦合點）

資料流（【量】讀 `O/mq/ShippingEventConsumer.java`、`S/mq/*`、`CC/mq/MqConstants.java`）：

| 編號 | 方向 | 通道 | 內容與耦合 | 去向 |
|---|---|---|---|---|
| K1 | order → shipping | MQ `hawkr.order` `order.paid`，shipping 佇列 `shipping.order.paid`（`S/mq/OrderPaidConsumer`） | 付款成功才建配送單（`createShipmentFromOrder`，內部叫 PayUni 取號）。**配送單的建立由付款事件驅動**，不是 checkout 同步建 | |
| K2 | order → shipping（兜底） | `O/scheduler/ShipmentCreationBackstopScheduler`（144 行）發 `order.shipment.create_requested` → `S/mq/ShipmentCreateRequestedConsumer`；超過重試上限發 `order.shipment.creation_failed` → aftersales `ShipmentCreationFailedConsumer` 系統取消＋退款 | 付款後超過寬限期仍無 `orders.shipment_no` 就重試建單、放棄就取消退款。**自寄模式沒有建單失敗這回事**（賣家回填追蹤號），計畫列為直接刪（B3） | |
| K3 | shipping → notification | `hawkr.shipping` `shipment.created`；`notification.shipping` 佇列綁 `#` | **賣家的「新訂單」推播綁在 `shipment.created` 上**（migration `20260531000000_rebind_order_placed_to_shipment_created.sql`：原綁 `order.paid`，因建單失敗賣家收到推播卻無單可操作而改綁）；`shipment.delivered` 有買家、賣家兩條模板；`shipment.anomaly.detected` 賣家模板。B3 若不再發 `shipment.created`，賣家新訂單推播會消失（【推】）。notification 現況：舊 `services/notification-service` 消費 `hawkr.shipping` 整個 exchange；單體 notification 的 `internal/arch/modules.go` 註解寫「還沒搬的（…ordering、fulfillment…）隨各自搬遷時加（A8 D13）」 | |
| K4 | shipping → order | MQ `shipment.created`→`order.shipping.created`；`shipment.status.changed`→`order.shipping.status`；`shipment.delivered`→`order.shipping.delivered`；`shipment.label.updated`→`order.shipping.label`；`shipment.photo.updated`→`order.shipping.photo`；`shipment.processing.cleared`→`order.shipment.processing.cleared`（皆 `O/mq/ShippingEventConsumer`，490 行） | 更新 `orders.shipment_no`、`shipping_status`（PayUni 狀態碼字串）、`label_url`、`label_expire_at`、`shipment_photo_keys`，並以 CAS 推進 `orders.status`：91/92/21/920→`PENDING_SHIPMENT(20)`、31/210/921→`IN_TRANSIT(30)`、32→`WAITING_PICKUP(32)`、33/43/44/46/51~56/81/82→`SHIPPING_ABNORMAL(35)`、送達→`DELIVERED(40)` 並算 `auto_complete_at`、寫 outbox。合法前置由 `O/util/OrderShippingProgress`（單調前進、異常恢復）統一產生 | |
| K5 | shipping → aftersales | `shipment.processing.timeout`→`aftersales.shipment.processing.timeout`（`A/mq/ProcessingTimeoutConsumer` → `cancelShipmentBySystem(orderNo, SHIPMENT_TIMEOUT)`）；`shipment.label.updated`→`aftersales.shipping.label`；`shipment.return.status_changed`→`aftersales.shipping.return.status`；`shipment.anomaly.detected` | 賣家逾期未出貨由 `S/scheduler/ProcessingDeadlineScheduler`（每 5 分鐘，租約在 `shipments.processing_deadline`）發事件，aftersales 取消＋退款。**這是自寄模式仍然需要的機制（計畫：逾期未出貨由伺服器狀態機決定）**，但發送端與消費端分屬 B3、B9 | |
| K6 | order → shipping | `order.receipt.confirmed`→`shipping.order.receipt.confirmed`（`S/mq/OrderReceiptConfirmedConsumer`） | 買家確認收貨或排程自動完成後，shipping 把 `shipments.receipt_confirmed` 標 TRUE | |
| K7 | order → aftersales | `OrderEventPublisher.publishShipmentCreationFailed` | 見 K2 | |
| K8 | order 本地 | `orders` 欄 `shipment_no`、`shipping_status`、`shipping_status_desc`、`label_url`、`label_expire_at`、`shipment_photo_keys`、`shipment_create_attempts`、`shipment_create_next_retry_at`、`delivered_at`（`O/entity/OrderEntity.java`） | 計畫 B8：「訂單上的物流投影欄與付款副本刪除」；B3 期間 Java order 仍讀寫它們 | |

【推】B3 在 B8 之前：Java order、aftersales、notification 在 B3 完成後仍活著。Go fulfillment 要嘛維持 K1、K4、K5、K6 的 MQ 與 gRPC 外觀（由 `internal/legacy/*` 型 adapter 與 relay 承接，單體其他模組的 A 階段都是這樣做），要嘛 B3 同時改 Java order／aftersales。計畫與 ADR 都沒寫選哪一種（§8 Q1）。

`shipping_status` 值域是 PayUni 狀態碼（`ShippingStatus` 91、92、21、31、32、33、11、98… 見 §5）；`order.proto` 的 `shipping_status`、`ShipmentDetailView`、ai-service 的 `shipping_status` 都吃這組碼，B8 已寫「不再是 PayUni 狀態碼字串」，B3 自寄狀態機要決定自己的值域。

### 1.3 結帳運費與免運（B6/B8 需要的規則）

位置（【量】）：`O/checkout/CheckoutService.java`（1700+ 行，字樣 185 處）、`CheckoutValidator.java`（71）、`ShippingFeeCacheClient.java`、`ShippingFeeGrpcFetcher.java`、`OrderGrpcService`（`CheckoutValidate` 回 `AvailableShippingOption`：`fee`、`fee_payer`、`free_shipping_threshold`、`tentative_free_shipping_eligible`，約 `CheckoutService.java:225-250`）、`CheckoutShippingFee`（`CheckoutService.java:558`）。

| 規則 | 現況（平台串接時代） | 來源 | 自寄後（【推】） | 去向 |
|---|---|---|---|---|
| 運費金額 | `shipping_fee_tiers`（方式×溫層×區域×材積級距，PayUni 合約費率）＋ `shipping_method_configs`（材積、重量上限、預計天數、撥款天數），由 shipping-service 擁有，order 以 gRPC 拉進 L2 快取 | 設計書 21 §0.2「單一來源」 | ADR D2：只用賣家自訂固定規則，不報價；兩張平台表與 `ShippingFeeGrpcFetcher`、`ShippingFeeCacheClient`、材積堆疊（`CheckoutValidator` 的 `pkg.totalDimSum` 等）都沒有依據。**賣家的固定規則存在哪裡沒寫**（目前只有 SKU 上的 `product_sku.shipping_method` JSONB：`type`、`temperature`、`freeThreshold`、`feePayer`，無金額欄） | |
| 誰負擔 | `feePayer`（`ShippingFeePayer` 3101 買家／3102 賣家）逐 SKU，`feePayer=SELLER` 才有免運資格 | `20` §4.7 | 保留（賣家自寄，平台不收也不負擔運費） | |
| 免運判定 | 逐商品獨立；`freeThreshold<=0` 且賣家負擔＝無條件免運；`freeThreshold>0` 以優惠券分攤後實付判定（折扣後判定）；任一商品達標整筆免運 | ADR `2026-09-18-checkout-free-shipping-and-degradation.md` 決策一、二；`20` §4.7 | 決策一、二是**產品語意**，與物流商無關，【推】照舊 | |
| 兩段判定 | Validate 回 tentative（折扣前）、Confirm 以折扣後 final；`CheckoutPreview`→`Confirm` 金額快照比對（`comparePriceSnapshot`） | `20` §4.11 | 保留，屬 B8 | |
| 快取與降級 | lazy load、5 分鐘 TTL、circuit breaker、fail-open stale（決策三、四；ADR `2026-09-18-shipping-fee-cache-no-warmup-no-scheduler.md`） | 兩份 ADR | 同進程呼叫後快取與熔斷沒有存在的理由（決策三、四的前提是跨服務 gRPC）；ADR 要標 superseded 或修訂（【推】） | |
| 補貼歸屬 | 賣家主辦／平台主辦／雙方共同三型態，六個量 `buyer_shipping_charge`、`provider_actual_cost`、`seller_logistics_charge`、`seller_subsidy`、`platform_subsidy`、`reconciliation_adjustment` | ADR `2026-09-11-free-shipping-subsidy-attribution.md`（狀態 accepted，**後果 3 寫「實作缺口：另一波」**） | 【量】`rg -l 'platform_subsidy\|seller_subsidy' backend --glob '*.java' --glob '*.go' --glob '*.sql'` 只命中 2 檔：`CheckoutValidator.java:226`（註解）與 payment-service `V9__add_logistics_finance_tables.sql`，**補貼欄位從未在 order 落地**；它的前提（平台吸收或補貼物流成本）已由 `logistics-finance-domain-ownership` 的 superseded 狀態（`2026-11-15`）推翻。【推】本 ADR 應標 superseded，B3 不實作補貼欄位 | |
| 路線與區域 | `ShippingArea`、`ShippingRoute`、`DeliveryZoneKey`（離島、本島、`toDeliveryZone`）、`CheckoutService.resolveRoute`、`getSenderPostalCode` | `21` §1 | 沒有區域費率就不需要（【推】） | |
| 溫層 | `deliveryTemp` 三值（`ShippingTemperature` 3201 常溫、3202/3203 低溫），gateway 補常溫 | ADR `2026-10-27-general-ecommerce-scope-cut` D8、D9；`shipping_ambient.go` | B4 擁有刪除；B6 計畫列「不帶溫層欄位」 | |


**B6 cart 需要的運費規則**（【推】，依下列檔案）：

1. cart-service 本身沒有運費邏輯（【量】主程式只有 `grpc/client/ProductGrpcClient.java:39` 註解提到 `shipping_method` JSONB，以及 `CartGrpcService.java:140` 回 `delivery_temps`）。Java cart 只把 SKU 支援的溫層帶出去。
2. 購物車畫面的配送方式、運費、免運進度**不來自 cart**，來自 `CheckoutValidate`（order）：website `features/cart/components/ShippingMethodGroup.tsx`、`useShippingOptions.ts`、`lib/checkout/free-shipping.ts`，buyer-app `features/cart/presentation/widgets/shipping_section.dart` 都讀 `available_shipping_options`（`fee`、`fee_payer`、`free_shipping_threshold`、`tentative_free_shipping_eligible`）。所以 B6 cart 需要的是「某賣家某批商品可選的配送方式、各自運費、免運門檻與預判」這個查詢，它在 B8 ordering 的 checkout 裡，B6 不能自己算。
3. 單體 `internal/arch/modules.go` 現況：`"cart": {"catalog","buyer","seller"}`，**沒有 `fulfillment` 邊**；`"ordering"`、`"catalog"`、`"aftersales"`、`"invoice"`、`"finance"` 都有。若購物車要顯示運費或免運進度而不經 ordering，要加 `cart→fulfillment` 邊並寫入決定（§8 Q4）。
4. 計畫 B6 的延後項：`cart.proto` 的 `delivery_temp`／`溫層`、order 的 `deliveryTemp`，加上 client 五處（website `lib/checkout/draft.ts`、`lib/checkout/repo.ts`、`features/cart/cartUiStore.ts`、`features/checkout/screens/CheckoutClient.tsx`，buyer-app `features/cart/data/cart_repository.dart`、`checkout_payment_sheet.dart`）。B3 刪溫層值域前必須先確認 B6 不在其後（`ShippingTemperature` 值域是 B3、B4、B6 的共同刪除條件，計畫 B3 與 B4 都有條文）。
5. 結帳硬依賴：`CheckoutService.shippingFee()` 註明「硬依賴 Shipping Service」，`CheckoutValidator` 在方式不可用時回 `SHIPPING_METHOD_UNAVAILABLE(4008)`、`SELLER_SHIPPING_INCOMPLETE(4010)`、`SHIPPING_ALL_METHODS_EXCEED_LIMITS(4016)`；這三個錯誤碼的存續取決於賣家規則的模型（§8 Q2）。

### 1.4 其他 Java 檔

| 項目 | 位置 | 說明 | 去向 |
|---|---|---|---|
| Uber 評估與守門 | `O/checkout/UberOrderEligibilityGuard.java`（62）、`ShippingUberQuoteAssessor.java`、`CheckoutService.uberQuote`（308）等 | 本步刪（Uber） | |
| 商品配送欄 | `P/entity/ProductSkuEntity`、`P/config/typehandler/ShippingMethodItemListTypeHandler`、`P/dto/request/ShippingMethodDTO`（`type`、`temperature`、`freeThreshold`、`feePayer`）、`P/converter/*` | catalog 的 SKU 配送設定；B4 搬時決定留在 catalog 或歸 fulfillment（modules.go 有 `catalog→fulfillment`） | |
| 訂單政策 | `O/checkout/OrderPolicyResolver.java`：`MERCHANT_DELIVERY(3005)` 必然 `SELLER_MANAGED`；`O/service/OrderService.java:359、395` | 現行 Java 已有 `fulfillment_mode` 快照欄；3005 目前只用於 RFQ（採購助理）訂單，設計書 65 §1.4 | |
| 配送單動作政策 | `S/provider/action/FulfillmentActionPolicy.java:78-84` | `MERCHANT_DELIVERY` 的 `CANCEL`、`DISPATCH`、`TRACK`、`VIEW_POD` 四個動作；`S/service/ShippingService.java:168` 的 3005 | |
| 排程（shipping，7 支） | `S/scheduler/`：`ProcessingDeadlineScheduler`（5 分鐘，**自寄仍需要**）、`ShipmentStatusReconcileScheduler`、`ShipmentLabelReconcileScheduler`、`PayUniSubmitStuckAlertScheduler`、`TcatCallStuckAlertScheduler`、`PayUniAuditCleanupScheduler`、`DedupCleanupScheduler` | 計畫列「貨態與標籤對帳排程」直接刪；其餘 PayUni 專屬 | |
| 排程（order、aftersales） | `O/scheduler/ShipmentCreationBackstopScheduler`（直接刪）；`A/scheduler/ReturnStuckAlertScheduler`（B9 刪）、`ReturnTwentyFourHourWarnScheduler` | | |
| shared PayUni 物流 client | `J/shared/payuni-java-client/…/payuni/shipping/**`（`ShippingClient`、`PayUniShippingEndpoint`、請求回應類 20 餘檔）、`webhook/event/ShippingPrintEvent`、`ShippingStatusEvent`、`WebhookType`、`PayUniWebhookParser` 的物流分支 | 整包 PayUni 物流呼叫端；與 B2 刪 PayUni 金流的範圍交界（payuni-java-client 的金流部分屬 B2） | |
| shared common-core | `CC/mq/MqConstants.java`（`ROUTING_KEY_SHIPMENT_*`、`QUEUE_ORDER_SHIPPING_*`、`QUEUE_SHIPPING_*`、`EXCHANGE_SHIPPING_DLX` 等約 30 個常數）、`CC/mq/events/ShipmentCreatedEvent`、`ShipmentDeliveredEvent`、`CC/mq/payload/ShipmentAnomalyDetectedPayload`、`CC/webhook/UberWebhookVerifier`、`CC/providercall/*`（`ProviderCall*`） | 事件常數是 Java 之間的 MQ 契約，Java 端最後一個消費者退場才能刪（B8、B9） | |
| payment-service 測試殘留 | `J/payment-service/src/test/**/mq/{OutboxEventFactoryExchangeTest,OutboxProducerExchangeContractTest,PaymentOutboxExchangeContractTest,…}`、`persistence/OutboxExchangeSchemaContractTest` 共 6 檔字樣命中 | 【推】只是 exchange 名稱字串，B2 收尾處理；payment-service 主程式零命中（S6） | |
| 設計書提到的 `HolidayService` | `S/service/HolidayService`、`holidays` 表、4 支 RPC（`ImportHolidays`、`ImportHolidaysFromContent`、`GetHolidayCoverage`、`GetHolidaysInternal`） | 計畫 B3「假日」條文：隨 Java shipping 退場刪；`ListBusinessDays` RPC 仍有 gateway 路由 `/app/shipping_business_days` | |

### 1.5 shipping-service 的資料表（Flyway 15 支，建 19 張，【量】`rg -oh 'CREATE TABLE( IF NOT EXISTS)? [a-z_.]+' S/../../resources/db/migration`）

`shipments`、`shipment_status_logs`、`shipping_webhook_dedup`、`seller_shipment_settings`、`seller_shipment_onboarding_logs`、`holidays`、`shipping_fee_tiers`、`shipping_method_configs`、`buyer_store_pickups`、`package_plan`、`packing_policy`、`shipment_anomaly_acknowledgments`、`payuni_audit_logs`、`uber_account`、`uber_organization`、`uber_deliveries`、`uber_pod_artifacts`、`uber_quotes`、`uber_store_mapping`。【推】自寄主線留下概念上的 `shipments`（含追蹤號、承運商）、`shipment_status_logs`、webhook inbox（由 `platform/inbox` 取代 `shipping_webhook_dedup`）。其餘隨 PayUni、Uber、超商、門市刪。（19 張＝18 張 `IF NOT EXISTS` 加無該子句的 `payuni_audit_logs`。）

## 2. payuni-webhook-service（整個服務刪除）

【量】`find` 該服務：主程式 27 檔（`adapter/http` 3、`config` 3、`infrastructure` 7、`scheduler` 2、`service` 5、`uber` 6、啟動類 1）、測試 9 檔。範圍交界：

| 部分 | 位置 | 歸屬 | 去向 |
|---|---|---|---|
| PayUni 物流貨態與列印 webhook | `service/WebhookReceiveService.java:228-229`（`SHIPPING_STATUS`、`SHIPPING_PRINT`）、`EventKeyBuilder.java`（`{merId}:{shipTradeNo}:STATUS/PRINT:…`）、`infrastructure/mq/PayUniEventRouting.java:24-26`（兩者都路由到 `ROUTING_KEY_PAYUNI_SHIPPING_STATUS`） | B3 | |
| Uber webhook | `uber/*`（6 檔：`UberWebhookEvent`、`UberWebhookParser`、`UberWebhookProviderCode`、`UberWebhookReceiveService`、`UberWebhookSignatureConfig`、`UberWebhookSignatureException`）、`adapter/http/UberWebhookController.java`；轉發到 `ROUTING_KEY_UBER_SHIPPING_STATUS`，shipping 佇列 `shipping.uber.webhook` | B3（Uber 刪除） | |
| 開發用模擬器 | `adapter/http/WebhookSimulatorController.java`：「物流 / 退貨貨態」分頁，靠 `ShippingServiceGrpc.LookupShipTradeNo` 反查 | B3（含 gateway `routes_webhook_simulator.go`） | |
| 金流 webhook | 付款、退款分支、`WebhookInbox`、`OutboxPublisher` | B2（計畫：Stripe webhook 走 inbox 取代） | |
| 服務邊界 | gateway `proxy/reverse_proxy.go:175` 的 `"payuni-webhook"` 服務表項、`router/constants.go:31-32` 的 `WebhookPayUniNotify`、`ServicePayuniWebhook` | B2 與 B3 共有，誰最後刪誰收尾 | |

## 3. Go 單體（`backend/internal`、`backend/cmd`）

【量】S11 的 22 檔分類：

| 類別 | 位置 | 與 shipping 的關係 | 去向 |
|---|---|---|---|
| legacy sellerserver | `internal/legacy/sellerserver/server.go:175`（註解：舊契約 `payuni_mer_id` 與物流 `mer_id` 一律空字串）、`:403-430` `GetSellerShippingProfile`／`shippingProfile`（只有 `contact_email` 有讀者＝shipping-service）、`server_test.go:109-118`、`server_integration_test.go`（共 8 處 `MerId` 斷言） | 【量】`MerId` 已經是空字串，**計畫「隨 Java shipping 退場刪除」要刪的是欄位填值的註解、測試斷言與 `GetSellerShippingProfile`**；`api/proto/seller/seller.proto:58-60、278-286` 的 `GetSellerShippingProfile`、`SellerShippingProfile.mer_id`（欄 8）、`SellerMeta.payuni_mer_id`（欄 7）屬 proto，B2 也會碰 | |
| legacy buyerserver | `internal/legacy/buyerserver/server.go:122-124` | `ValidateAddress` 回 9700，註解「Java shipping 的 `UberQuoteService` 只把 `code!=0` 轉成 Uber 不可用…shipping 搬進單體或接上地理編碼時刪除」 | |
| legacy orderservice | `internal/legacy/orderservice/buyerorders.go`（`ShippingMethod`、`ShippingStatus`、`ShippingFee`；`ShippingFee` 驗非負）、`orderservice.go:514-538`（`PendingShipment` 計數） | 單體讀 Java order 詳情，吃 PayUni 狀態碼；B8 改 | |
| httpapi | `internal/httpapi/seller/dashboard.go:38、216-221`（`PendingShipment` 計數進 `OrderStatusCounts`） | 賣家儀表板，來源仍是 Java order | |
| assistant | `internal/modules/assistant/internal/buyeragent/{ports,tools}.go`、`cmd/hawkr/assistant.go:46-47`、`internal/modules/assistant/internal/turn/characterization_test.go` | 買家助理工具回 `shipping_method`、`shipping_status`、`shipping_status_desc`、`shipping_fee`；特性測試與 `docs/plans/2026-11-19-a9-agent/fixtures/assistant/*.json` 中 2 個 fixture 含這些字串（【量】`rg -l -i 'shipping_method|shipping_status|shipping_fee'`） | |
| 採購 | `internal/modules/procurement/**`（`rfq.sql.go`、`quote.sql.go`、`rfqhash`、`quote/accept.go:87`）等 | `ShippingAmount`（報價的運費）是報價金額欄，不是 fulfillment 呼叫；【推】與 B3 無關 | |
| buyer 模組 | `internal/modules/buyer/internal/store/shipping_addresses.sql.go`（24 處）、`address/address.go` | 買家的**收件地址**，非物流；ordering 結帳取收件人時用（`fulfillment` 依賴 `buyer` 的理由之一，【推】） | |
| seller 模組 | `internal/modules/seller/internal/model/capability.go:41-42、82` | `CapShippingSetup`（`CAP_SHIPPING_SETUP`）：物流建店、啟用與關閉配送方式，前置是 `ONBOARDING_COMPLETE`；gateway 的 `routes_shipping.go` 以它守 5 條建店路由。自寄後建店（PayUni 子商店）消失，能力是否保留以「賣家啟用／設定自己的配送方式」為意要決定（`seller.go:444` 對外導出） | |
| notification 模組 | `internal/modules/notification/internal/model/model.go:39-40、96-97` | `CategoryShipment`（`SHIPMENT`，雙方、交易性不可關）、`CategoryShipmentAnomaly`（`SHIPMENT_ANOMALY`，只有店家，收件匣、推播、email 皆不可關） | |
| delegation | `internal/modules/delegation/internal/session/query.go:72` | `fulfillmentAreaTerms = {"fulfillment","shipment","cancel"}`，代行營運的區域詞彙 | |
| 其他 | `internal/platform/stripe/redact.go:69`（`"shipping"` 遮罩鍵）、`cmd/hawkr/dev_seed.go:11`（註解：06 seed 的 `mer_id` 是 HAWK＋`store_code`）、`internal/modules/seller/internal/devseed/devseed.go`、`internal/modules/seller/internal/model/onboarding.go:113` | 與 shipping 無呼叫關係 | |

**模組依賴規則**（【量】`internal/arch/modules.go:8-45`）：

| 模組 | 可依賴 | 對 B3 的意義 |
|---|---|---|
| `fulfillment` | `buyer`、`seller`、`media` | 已列出（行 21）。B1 的 `PutSecure` 託運單 PDF 走 `media`；`payment` 不在其中，與 B2 separate charges and transfers 沒有交集（物流財務已刪） |
| `catalog` | `seller`、`buyer`、`media`、`fulfillment` | 商品的配送方式驗證、運費預覽走 fulfillment |
| `cart` | `catalog`、`buyer`、`seller` | **沒有 `fulfillment`**（見 §1.3 B6 需求 3） |
| `ordering` | `cart`、`coupon`、`catalog`、`fulfillment`、`payment`、`invoice`、`buyer`、`seller`、`identity`、`media` | 結帳運費、出貨狀態在 ordering 讀 fulfillment |
| `aftersales`、`invoice`、`finance` | 皆含 `fulfillment` | `finance` 在計畫 B10 已退場（【量】計畫 B10 標 `[x]`），但 `modules.go:31` 的 `finance` 條目仍在，B3 收尾或 B10 收尾要清（【推】） |
| `notification` | 不含 `fulfillment`（行 41-44 註解寫「隨各自搬遷時加」） | K3 的賣家新訂單推播改由誰發，決定要不要加這條邊 |

【量】`internal/arch` 其餘規則檔（`restricted_imports.go`、`storedcodes.go` 等）沒有 `fulfillment` 字樣（`rg -n fulfillment internal/arch` 只有 `modules.go` 行 21-44）。`internal/modules/fulfillment` 目錄不存在（`ls internal/modules`）。

## 4. gateway（`backend/gateway/internal`）

### 4.1 路由與 handler（S8 的 46 檔中屬 shipping 本體的）

| 檔案 | 行數 | 內容 | 去向 |
|---|---|---|---|
| `router/routes_shipping.go` | 115 | 賣家：`/app/shipments`（`batch_print_cvs_label`、`batch_generate_home_label`、`anomalies`、`anomalies/stats`、`anomalies/:id/:type/acknowledge|ignore`、`/:order_no/{print_cvs_label,generate_home_label,call_tcat,confirm,tracking}`）、`/app/shipping_onboardings`（`cvs`、`home_delivery`、`GET`、`GET /:id`、`toggle`）、`/app/shipping_methods`、`/app/shipping_business_days`；買家：`/app/store_map_sessions`、`/app/store_pickups`（POST、GET、PUT、DELETE）；後台：`/admin/shipments/anomalies`（含 acknowledge、ignore）、`/admin/shipments`、`/admin/shipments/:id`、`/admin/seller_shipment_settings`；頂層：`/store_map/redirect`、`/store_map/return` | |
| `handler/shipping.go` | 1321（19 個 handler） | 賣家出貨動作與建店、買家門市、`GetOrderLogistics`（`:1210`，買家物流追蹤） | |
| `handler/shipment_anomaly.go` | 519（7） | 賣家與後台的配送異常 | |
| `handler/admin_shipping.go` | 345（3） | 後台出貨單與賣家出貨設定 | |
| `handler/store_map.go` | 179（3） | 超商門市地圖；直接呼叫 `h.Shipping.GetStoreMapParams`、`GetStoreMapRedirectForm`、`DecryptStoreMapResult` | |
| `handler/shipping_options.go` | 127 | `/app/shipping_quotes` 賣家建商品運費預覽（`CalculateShippingOptions`，經 **product** client，不經 shipping client） | |
| `handler/fulfillment_actions.go` | 215 | 配送單動作可用性，按 `provider`（`ShippingMethod` 值，註解「3001 黑貓／3002 超商」） | |
| `handler/checkout.go:638` `CheckoutShippingFee` | — | 經 order client（`grpc/order_client.go:140`）；路由 `routes_checkout.go:16`、常數 `CheckoutShippingFee = "/shipping_fee"` | |
| `grpc/shipping_client.go` | 277（34 個方法） | gateway→shipping gRPC client；`grpc/di.go:19、41`、`handler/holder.go:23`、`handler/di.go:38`、`grpc/testing.go:158-160` | |
| `proxy/reverse_proxy.go:163` | — | 服務表項 `"shipping"`（`shipping-service`）、`:175` `"payuni-webhook"`；`router/constants.go:471` `ServiceShipping` | |
| `router/constants.go` | — | 常數段：PayUni Webhook、`StoreMap*`、`StorePickups*`、`OrderLogistics`、`Shipments*`、`ShippingOnboardings*`、`ShippingMethodsPath`、`ShippingBusinessDaysPath`、`CheckoutShippingFee`、`ShippingQuotesPath`、`AdminShipment*`、`WebhookSimulator*`（行 28-48、103、108-170、200-209、455-471） | |
| 路由守門 | `authz/capability.go`（`CapOrderFulfill` 守 11 條出貨動作；`CapShippingSetup` 守建店；`CapProductManage` 守列表與查詢）、`router/seller_capability_test.go`、`router/delegated_*` | 【推】能力對照表與代行營運路由斷言要同步改 | |
| 其他有 shipping 字樣的 handler | `admin_order.go`、`admin_delegated_order.go`、`admin_delegated_product.go:1100`、`order.go`、`product.go`、`seller_product.go:485、613`、`aftersales.go`、`admin_aftersales.go`、`invoice_*.go`、`notification_preference.go`、`search.go`、`ai_procurement_seller_response.go`、`vendor_portal_response_draft.go`、`realtime/seller_workspace_channel.go`、`middleware/{logger,ratelimit}.go` 等 | 多為訂單、商品欄位或 log 允許鍵，需逐檔判斷是否碰 shipping RPC（【量】`rg '\.Shipping\.'` 只命中 `store_map.go` 三處，其餘 shipping 呼叫都在 `shipping.go`、`admin_shipping.go`、`shipment_anomaly.go` 內的 `h.Shipping`／`GrpcErr(c,"shipping",…)`） | |
| `handler/shipping_ambient.go` | 10 | `ambientShippingTemperature = "3201"`、`isAmbientOrUntyped`；使用處：`shipping.go:501、598、750`、`shipping_options.go:101`、`seller_product.go:485、613`、`admin_delegated_product.go:1100`、測試 `shipping_ambient_test.go` | **B4 擁有，B3 不碰**（計畫 B4 延後項：程式註解寫明 B4 刪除；B3 刪 `shipping.go` 時要注意它引用此常數，兩步的順序要協調，見 §8 Q8） |

### 4.2 `routes_webhook_simulator.go` 與 `routes_payment.go`

| 檔案 | 內容 | 歸屬 |
|---|---|---|
| `router/routes_webhook_simulator.go`（31 行） | 只在 `APP_ENV` 為 local、dev、development 註冊 `GET /dev/webhook-simulator`、`POST …/send`、`GET …/lookup`，代理到 payuni-webhook-service（`FailClientFacing`） | 計畫 B3 明列刪除 |
| `router/routes_payment.go`（70 行） | `registerPayUniWebhookRoute`：`POST /webhook/payuni/notify`，路徑改寫為 `/webhooks/payuni/notify`，`FailThirdPartyWebhook`（502，唯一刻意維持 5xx 的代理路由）；`registerPaymentRedirectRoute`：`GET PaymentRedirectPath` → `h.RedirectToPayUni`；`/payment/return` 說明 | 計畫 B3 寫「routes_payment.go 裡的代理」刪除；【推】但付款導向與 return 是 B2 的 PayUni 金流，**同檔案兩個計畫步驟共有**，需確認 B2 是否已先刪（§8 Q9） |

### 4.3 gateway proto 與契約副本

【量】`backend/api/proto/shipping/shipping.proto` 1592 行、`backend/pkg/proto/shipping/{shipping.pb.go,shipping_grpc.pb.go}`；含 shipping 的其他 proto：`seller.proto`、`order.proto`、`coupon.proto`（【推】多為字樣）、`product.proto`、`ai.proto`、`aftersales.proto`、`invoice.proto`、`media.proto`。order.proto 的 shipping 相關 RPC：`GetOrderForShipping`（`:63`）、`CheckoutShippingFee`（`:87`）、`GetUberQuoteForCheckout`（`:157`）；aftersales.proto 的 `CancelShipment`（`:47`）；product.proto 的 `CalculateShippingOptions`（`:91`）。

### 4.4 openapi（`backend/api/openapi/openapi.yaml`，29613 行）

【量】S10 的 30 條 path，加 3 條：

- 賣家出貨：`/api/v1/app/shipments/{order_no}/{call_tcat,confirm,generate_home_label,print_cvs_label,tracking}`、`/app/shipments/batch_generate_home_label`、`/app/shipments/batch_print_cvs_label`
- 配送異常：`/api/v1/app/shipments/anomalies`、`…/stats`、`…/{shipment_id}/{anomaly_type}/{acknowledge,ignore}`；後台 `/api/v1/admin/shipments/anomalies`（含 acknowledge、ignore）
- 後台：`/api/v1/admin/shipments`、`/admin/shipments/{id}`、`/admin/seller_shipment_settings`
- 建店與設定：`/app/shipping_onboardings`、`…/cvs`、`…/home_delivery`、`…/{onboarding_id}`、`…/{onboarding_id}/toggle`、`/app/shipping_methods`、`/app/shipping_business_days`、`/app/shipping_quotes`
- 買家：`/app/store_map_sessions`、`/app/store_pickups`、`/app/store_pickups/{store_pickup_id}`、頂層 `/store_map/redirect`、`/store_map/return`
- 不在型樣內的三條：`/api/v1/app/checkout/shipping_fee`（`:10530`）、`/api/v1/app/orders/{order_no}/logistics`（`:13180`）、`/api/v1/admin/after_sales/{case_no}/retry_print_return_label`（`:434`，B9）

gateway 由 CI 對齊 openapi（`router/openapi_coverage_test.go`），所以刪路由與刪 path 必須同一個 commit。

## 5. CodeMeta 的物流值域

來源是 `D/codemeta/*.md`，由 `scripts/generate.sh` 生成（凍結，只服務舊契約）：

| 輸出 | 路徑 |
|---|---|
| Go（前端可見） | `backend/internal/platform/codemeta/code_meta.go` |
| Java（最終覆蓋輸出） | `CC/utils/code/*.java` |
| Flutter | `buyer-app/lib/core/codemeta/code_meta.dart`、`seller-app/lib/core/codemeta/code_meta.dart` |
| Web / Admin | `website/lib/codemeta/generated.ts`、`admin/src/lib/codemeta/generated.ts`（admin 走 allowlist） |
| 錯誤碼 | `backend/pkg/errors/bizcode/bizcode.go`（`--go-errors`，≥1000）、Java `GrpcResponseCode.java` |

| 值域 | 位置（md） | 值 | 其他使用處（【量】`rg -l` 只列主程式） | 去向 |
|---|---|---|---|---|
| `ShippingMethod`（3001~3099） | `business-type-codes.md:102` | 3001 `TAQBIN` 黑貓、3002 `SEVEN_ELEVEN_STORE_PICKUP`、3003 `UBER_INSTANT`、3005 `MERCHANT_DELIVERY` | `orders.shipping_method`、`product_sku.shipping_method` JSONB `.type`；buyer-app `shipping_presentation.dart`、`logistics_track.dart`、`order_card.dart`、`after_sale_detail_screen.dart`；`internal/legacy/orderservice`、assistant | |
| `ShipmentAnomalyType` | `business-type-codes.md:577` | 11 值：`TCAT_CALL_FAILED`、`CVS_LABEL_UNPRINTED`、`PROVIDER_STATUS_ABNORMAL`、`PACKAGE_LOST`、`DELIVERY_OVERDUE`、`PAYUNI_LGS_NO_CODE`、`RETURN_STUCK`、`UBER_COURIER_UNASSIGNED`、`UBER_PICKUP_STUCK`、`UBER_CREATE_UNKNOWN_UNRESOLVED`、`UBER_POD_MISSING`（4 個 UBER 值） | seller-app `shipment_anomaly_presentation.dart`、`features/shipment_anomaly/data/shipment_anomaly_model.dart`、website `lib/shipping/anomalies.ts`、admin `app/admin/anomalies/page.tsx`；`shipment_anomaly_acknowledgments.anomaly_type` | |
| `ShipmentAnomalySeverity`、`ShipmentAnomalyAckAction` | `business-type-codes.md:626、644` | 嚴重度與處置動作；`ShipmentAnomalyRules.severityOf` 在 `CC/utils/code/helper/` | 同上 | |
| `UberDeliveryOutcome` | `business-type-codes.md:1687` | 5 值：`PROVIDER_UNDELIVERABLE`、`PROVIDER_CANCELED`、`SELLER_CANCELED`、`ADMIN_CANCELED`、`CREATE_FAILED` | 只有 CodeMeta 生成物（【量】buyer-app、seller-app、website、admin、Go、Java 共 6 個生成檔，無手寫使用） | |
| `ShippingTemperature`（3201~3299） | `business-type-codes.md:139` | 3201 常溫、3202、3203 低溫 | `orders.delivery_temp`、SKU JSONB `.temperature`；website/admin/兩 App 生成檔 | |
| 錯誤碼 9024 | `錯誤碼.md:485`（AfterSales 段 9001~9099） | `AFTERSALE_RETURN_NOT_SUPPORTED_FOR_UBER`，Uber 即時配送訂單暫不支援退貨退款 | 計畫補充範圍：`pkg/errors/bizcode` 的 9024 | |
| 同號段其他 | `錯誤碼.md:52-105`（Shipping 業務碼 5001~5099，49 筆，含 `SHIPMENT_*`、`SHIPPING_ONBOARDING_*`、`SHIPPING_GOODS_TYPE_INVALID` 等）、`:489-605`（物流履約業務碼 9500~9699，61 筆）、`:32-50` 內 4008、4010、4016 | B3 刪除或改寫時要核對哪些碼仍有主程式引用 | |
| 其他物流值域（計畫沒逐一列名） | `business-type-codes.md`：`ShippingType`（`:120`，3651~3654 超商／宅配）、`ShippingFeePayer`（`:157`，3101~3199）、`ShippingArea`（`:173`）、`ShippingRoute`（`:189`）、`ProviderSource`（`:661`）、`FulfillmentMode`（`:1445`）、`FulfillmentAction`（`:1481`）、`FulfillmentExceptionCategory`（`:1624`）、`FulfillmentCaseResolutionType`（`:1643`）、`ProviderFeeType`（`:1666`）、`CommerceModel`（`:1407`）；`business-status-codes.md`：`ShippingStatus`（`:319`）、`OnboardingStatus`（`:452`）、`LabelPrintStatus`（`:489`）、`TcatCallStatus`（`:508`）、`LogisticsReceivableStatus`（`:607`）；`internal-technical-codes.md`：`WebhookProvider`（`:185`）、`OnboardingLogAction`（`:392`）、`OnboardingLogResult`（`:409`）、`DeliveryZoneKey`（`:574`）、`ShippingEventType`（`:1009`）、`FulfillmentEventType`（`:1039`）、`LogisticsEventType`（`:1063`） | 計畫 B3 與 B10 只點名四個值域；`ShippingFeePayer`、`FulfillmentMode`、`CommerceModel` 在自寄主線**仍有用**（§1.3），其餘多數沒有消費者。逐值域的存續要決策 | |
| 生成腳本與文件 | `D/codemeta/scripts/generate.sh`（admin 的 `ADMIN_ENUMS` allowlist）、`README.md:241-243` 號段表（3001、3101、3201、3601~3699 屬物流）、`pitfalls-and-change-history.md` | 刪值域時要同步 allowlist 與號段表 | |

## 6. 設計書 65–70

【量】現況：66、68、69 於 `2c4928740`（2026-10-29）刪除，67 於 `a7d37b97e` 刪除，70 於 `d876df987`（2026-11-23）刪除；**目前 `backend/docs` 只剩 65**（基本、詳細各一）。計畫與 ADR 仍寫「設計書 69 `商家自有出貨` 已定義 `fulfillment_mode = SELLER_MANAGED`」，那份文件已不在工作樹，只能從 `git show 2c4928740^:'backend/docs/detailed-design/69-商家自有出貨（Merchant Self-Delivery）詳細設計書.md'` 讀（1903 行；【量】已讀其目錄與 §0、§7.3、§7.4）。實際要改的設計書因此是 65、06、20、21、22、23，以及提到 Uber、物流事件的 03、09、36、29、58、10、12、02、05（字樣數見 §6.4）。

### 6.1 設計書 69（已刪）內容摘要

- 定位：履約層的橫向能力，與帳務軸正交；落在既有 shipping-service，不新增服務。商家自送（`MERCHANT_DELIVERY`/3005）原為 RFQ 訂單的站外業者出貨，平台不安排物流、無外部 provider API。
- 三種成熟度：L0 Manual（賣家在我方 App／Web 手動推進貨態，**唯一途徑**）、L1 Webhook（商家 inbound 回報事件，驗簽、去重、順序合併）、L2 API（我方也可主動查詢商家、取消需商家 ACK）。**對 ADR D1 相關的只有 L0**。L1、L2（HMAC 商家整合 API、secret rotation、失聯回查、`merchant_delivery_job`、`event_seq_watermark`）屬對第三方商家的整合，作品集不需要。
- 狀態機 `merchant_delivery_job`：`PENDING_DISPATCH`（L1/L2 才落此值）→ `NOTIFIED` → `ACCEPTED`（**L0 由建單直接落此值**）→ `IN_FULFILLMENT` → `DELIVERY_REPORTED` → `CLOSED`（E 軸確認事件）；`REJECTED` 非終態，必須被推到 `CANCELLED`（走 65 §6A.1.2 的 N1 `order.fulfillment.failed` → aftersales `cancelShipmentBySystem`），不另建第二條退款入口。
- 收貨 Gate（§7.4，核心規則）：賣家自稱送達、POD、驗簽 callback、承運商事件，**都不能單獨通過收貨 Gate**；只有買家確認或 ADM 裁決能。`DELIVERED` 只寫 C 軸 `11`、不碰 E 軸；60h `AUTO_CONFIRMED` 不算撥款資格。對作品集：這條在 B2 撥款改成 separate charges and transfers 後怎麼落（誰觸發轉帳、與 `auto_complete_at` 的關係）沒有任何文件寫（§8 Q3）。
- 參數：`accept_sla_minutes` 預設 30、不得上修超過 120；POD 上傳窗口 24 小時（軟窗口，只通知）；失聯回查 24 小時；這些數字都屬 L1/L2 或撥款證據，ADR D1 的「逾期未出貨、逾期自動完成」要用的期限（目前 shipping 的 `processing_deadline`、order 的 `auto_complete_at`）在 69 沒有新定義。
- 其他：§5 資料模型 380 餘行（job、事件序號、secret）、§6.2 對第三方公開的商家整合 API 契約（約 210 行）、§12 部署、§13 測試、附錄 C CodeMeta 新增清單、附錄 E 商家整合指南。
- 結論（【推】）：69 實質可重用的是 L0 的狀態轉換與收貨 Gate 的語意；其餘 90% 以上是第三方整合，與 ADR 的 D3「物流 port＋模擬 adapter」不是同一件事（port 是平台呼叫物流商，69 的 L1/L2 是商家呼叫平台）。

### 6.2 設計書 65（現存）

詳細設計書 `65-fulfillment-platform-detailed-design.md`（429 行）：

| 章節 | 行 | 假設 | 備註 | 去向 |
|---|---|---|---|---|
| 檔頭四塊表與「配套文件」「對應程式碼」 | 1-33 | 四塊權威實作都在 Java（含 payment-service 責任判定、帳本） | 整段要重寫 | |
| §1 訂單政策快照（order-service）、§1.1 欄位、§1.2 `OrderPolicyResolver` | 34-74 | `commerce_model`、`collection_model`、`invoice_model`、`fulfillment_mode` 等六欄快照 | `fulfillment_mode` 保留；收款模式與 B2 的收款開通有交集 | |
| §1.3 付款 MerID `resolvePaymentMerId` | 75-86 | PayUni MerID | 隨 B2 刪 | |
| §1.4 RFQ（採購助理）訂單 | 87-104 | 3005 `MERCHANT_DELIVERY` | 自寄成為主線後 3005 與一般訂單的差別要重判 | |
| §2 收貨確認軸（E 軸）§2.1、§2.2 | 105-138 | 確認收貨三來源、60h 自動確認 | 與撥款（B2）掛鉤部分要重寫 | |
| §3 provider adapter §3.1 硬規則、§3.2 `FulfillmentProviderAdapter` 與 capability、§3.3 `Registry` | 139-177 | 以 PayUni、Uber 的 capability 切分 | **ADR D3 的 port 以此為雛形**，要改寫成供應商中立 port（建單、取標籤、貨態 webhook inbox） | |
| §3.4 PayUni adapter | 178-190 | PayUni | 刪 | |
| §4 配送單擴充與動作可用性 §4.1 `shipments` 擴充欄（V11）、§4.2 狀態映射、§4.3 兩層模型、§4.4 `FulfillmentActionPolicy` | 191-245 | PayUni 狀態碼映射、Uber 動作 | 改寫成自寄狀態機 | |
| §5 責任判定案件（payment-service）§5.1 表、§5.2 `ResponsibilityCaseRules`、§5.3 Admin 端點 | 246-332 | 平台責任判定 | 【量】程式已刪（commit `5883f499e`、`125db5844`、`28a120564`），文件還在，**整節刪** | |
| §6 事件 | 333-352 | `order.fulfillment.failed`（N1）等 | 保留事件，改寫 | |
| §7 帳本與 provider 帳單列 §7.1、§7.2 | 353-413 | 雙分錄帳本 | 程式已刪，**整節刪** | |
| §8 錯誤碼 | 414-429 | 9501~9575 | 與 §5 §7 對應者刪 | |

基本設計書 `65-…基本設計書.md`（85 行）：§0（問題陳述）、§1（誰用）、§2 訂單政策快照、§3 收貨確認軸、§4 adapter 與兩層動作、§5 責任判定、§6 帳本與物流商帳單、§7 刻意不做。【推】§5、§6 刪，§7 補「平台代買標籤、超商、即時報價」。

### 6.3 設計書 06（物流微服務）、20、21、22、23

| 文件 | 範圍 | 去向（留空）備註 |
|---|---|---|
| `detailed-design/06-…詳細設計書.md`（5605 行） | §1 概述 1-144；§2.1 狀態機 147、§2.2 PayUni 映射 207、§2.3 建店狀態機 238、§2.4 外部呼叫併發 290、§2.5 PayUni 欄位防護 567；§3 資料庫 628-1043（§3.1 `shipments`、§3.3 `shipping_webhook_dedup`、§3.4 `seller_shipment_settings`、§3.6 `holidays`、§3.7 `shipping_fee_tiers`、§3.8 `shipping_method_configs`、§3.9 `buyer_store_pickups`）；§4 介面 1044-1771（§4.1 REST 映射、§4.2 proto 1135-1643、§4.4 發佈事件、§4.5 消費事件）；§5 流程 1772-4533（§5.1 背景建單、§5.2 超商列印、§5.3 宅配取號、§5.4 呼叫黑貓、§5.5 webhook、§5.6 列印 notify、§5.7 按需查詢、§5.8 送達後自動完成、§5.9/5.10 建店、§5.13 賣家可用配送方式、§5.14 門市地圖、§5.15 `CvsPrintWaitingRegistry`、§5.16 退貨物流、§5.17 商品配送驗證、§5.18 webhook service 整合、§5.19 reconciliation、§5.20 賣家逾期自動取消、§5.21 配送異常通知、§5.22 排程搶佔）；§6-§8 MQ、時序、一致性 4534-5306；§9 環境 5307；§10 `payuni_audit_logs` 5472；附錄 A enum 5595 | 【推】幾乎全篇平台串接。**自寄仍相關**：§2.1 的狀態機思路、§3.1 `shipments`、§5.8（送達後訂單自動完成）、§5.13（賣家可用配送方式）、§5.20（逾期自動取消，K5）、§8.5 排程搶佔；其餘整體刪除或改寫為新模組的設計書 |
| `basic-design/06-…基本設計書.md`（410 行） | §0、§1、§2 核心模型 72、§3 賣家要先開通 115、§4 主要流程 148（§4.1 黑貓、§4.2 7-11、§4.3 退貨物流、§4.4 逾期自動取消）、§5 錢與發票 274（§5.1 買家付款線 A、§5.2 發票）、§6 狀態 318、§7 出事 351、§8 頁面地圖 368、§9 刻意不做 394、§10 外部相依 403 | 【推】§3、§4.1~4.3、§5、§8 的超商與門市頁面、§10 刪；§4.4、§6 改寫 |
| `detailed-design/20-結帳模組…`（1242 行） | §1.5 範圍、§3.3 `CheckoutShippingFee` 275、§4.2 運費流程 581、§4.6 材積重量（最短邊堆疊法）697、§4.7 免運邏輯 723（規則保留，來源 ADR）、§4.8 溫層相容性驗證 818、§4.9 運費計算架構 828、§4.12 降級策略總覽 920、§4.13 和單與 `paymentMerId` 935、§7.2 `ShippingTemperature` 補充方法 1195、§7.3 `WeightUnit` 1207、§7.4 `ShippingAreaResolver` 1219、§7.5 Redis 快取鍵 1236 | 【推】§4.6、§4.8、§4.9、§4.12 的運費部分、§7.2~7.5 刪或改；§4.7 保留；§4.13 屬 B2 |
| `detailed-design/21-運費計算模組…`（699 行） | §0 概覽、§1 配送區域 73-120（三區域、PayUni 離島、店到店）、§2 DB 表 121-239（`shipping_method_configs`、`shipping_fee_tiers`）、§3 快取 240-319（L1、L2、結構、查找、重整）、§4 gRPC 320-438、§5 結帳整合 439-499、§6 產品服務整合 500-527、§7 common-core 不得持有運費資料 528、§8 時序 542-659、附錄 A PayUni 最新費率表 660-699 | 【推】**整份以 PayUni 費率為前提**，與 ADR D2 衝突；除 §0.1 表中「買家被收多少」的語意外全部要重寫 |
| `detailed-design/22-商品配送驗證…`（301 行） | §2 `ValidateProductShipping` RPC 27、§3 產品整合 214、§4 proto 契約落點 272 | 【推】尺寸重量對照的是 `shipping_method_configs` 限制；ADR 沒有限制表，B4 處理商品建立時是否仍驗 |
| `detailed-design/23-門市管理模組…`（611 行） | 全篇：§1 概述、§2 DB、§3 gRPC、§4 Gateway、§6 PayUni 退貨門市同步 439 | 【推】超商門市整份，ADR D4 不搬，整份刪 |

### 6.4 其他提到物流或 Uber 的設計書（字樣數 `rg -c -i`）

| 文件 | Uber／3003 | 物流相關 | 需處理的標題（【量】`rg -n '^#{2,4} …'`） |
|---|---|---|---|
| `detailed-design/03-訂單微服務` | 21 | 338 | 物流推進守門單調前進與異常恢復 `:225`、物流狀態碼 → 訂單狀態推進規則 `:267`、`DELIVERY_FAILED_NO_REFUND(54)` `:90`、物流編號 `:584`、「Shipping Service — 聚合物流 PayUni provider 狀態」`:1420`、「賣家 onboarding 狀態（結帳流程強制查）」`:1427`、Shipping Service 事件 `:1519`、運費來源說明 `:1635`、§5.5 物流事件消費 `:1898`、`shipment.label.updated` `:1908`、§5.6 出貨收貨照片 `:1945`、§7.7 配送失敗責任 `:2519`、§8.11 `ShipmentCreationBackstopScheduler` `:2799` |
| `detailed-design/09-售後微服務` | 24 | 429 | §2.3 取消出貨狀態機 `:428`、§2.5.3 配送失敗退款政策 `:595`、§3.6 `seller_cancel_requests` `:973`、§4.3.2 Shipping Service 新增 RPC `:1784`、§4.3.4b 批次 provider 狀態聚合 `:1926`、§5.3 退貨物流流程 `:2362`（多個子節）、§5.7 取消出貨流程 `:3893`；多數屬 B9 |
| `detailed-design/36-MQ 與事件總覽` | 22 | 127 | §3.3 物流領域 `:364`、Shipping Service 8 個發送方法 `:1271`、4 個 Consumer class `:1387` |
| `detailed-design/29-Admin 異常訂單管理` | 1 | 130 | 後台出貨單與異常 |
| `detailed-design/58-電子發票服務` | 7 | 104 | 發票查出貨狀態（K，invoice-service 的 `BatchGetShipmentStatus`） |
| `detailed-design/10-PayUni Webhook 服務` | 14 | 18 | 整份隨服務刪（計畫） |
| `detailed-design/02`、`05`、`12` | 0、2、1 | 101、14、27 | 02 的商品配送方式；05 的物流財務殘留；12 的通知模板 |
| `detailed-design/00-總覽`、`basic-design/00-總覽` | 0 | 14、6 | 服務與設計書索引 |
| 計畫補充 | | | 計畫明列設計書 03、06、09、58 描述 3003（Uber）的段落與 `D/runbooks/uber-quote-retention.md`、`D/runbooks/uber-quote-retention/` 一併刪；4 份 Uber ADR 標 superseded（`2026-09-11-uber-commerce-model-seller-direct`、`2026-09-11-uber-logistics-fee-charge-trigger`、`2026-09-14-uber-create-idempotency-precommit-key`、`2026-09-06-uber-webhook-slice-in-payuni-webhook-service`） |

### 6.5 相關 ADR 狀態（【量】讀各檔頭）

| ADR | 狀態 | 與 B3 的關係 |
|---|---|---|
| `2026-11-15-fulfillment-seller-managed-shipping.md` | accepted | 本步的依據，D1-D4 |
| `2026-09-11-logistics-finance-domain-ownership.md` | **superseded by** 上一份 | 物流財務已刪 |
| `2026-09-18-checkout-free-shipping-and-degradation.md` | accepted | 決策一、二留；三、四（快取與降級）失去前提 |
| `2026-09-18-shipping-fee-cache-no-warmup-no-scheduler.md` | accepted | 失去前提 |
| `2026-09-11-free-shipping-subsidy-attribution.md` | accepted | 平台補貼與六個量的前提已推翻；Uber 成本語境 |
| 其他相關 | `2026-09-11-logistics-amount-semantics-separation.md`、`2026-09-11-logistics-cost-true-up.md`、`2026-09-11-uber-logistics-fee-charge-trigger.md`、`2026-09-06-fulfillment-docs-split-into-six.md`、`2026-09-12-return-leg-classifier-canonical.md` | 狀態未逐一讀，列為 B3 開工前核對（§8） |

## 7. 四個 client 的出貨相關介面（供計畫 UI 同意清單參考；非本盤點要求，只給量）

【量】`rg -l 'app/shipments|shipping_onboardings|shipping_methods|store_pickups|store_map|checkout/shipping_fee|shipping_quotes|/logistics|shipping_business_days|admin/shipments|seller_shipment_settings|shipments/anomalies'`（排除 test、gen）：website 15 檔、admin 2、buyer-app 9、seller-app 15。

- website：`app/(commerce)/store-map/return/route.ts`、`lib/checkout/store-pickups.ts`、`lib/logistics/repo.ts`、`lib/seller/{shipment,shipping}.ts`、`lib/shipping/anomalies.ts`、`lib/checkout/repo.ts`、`lib/products/seller-repo.ts`、`features/seller-center/{orders/SellerOrdersPage,products/components/SkuShippingEditor,products/adapters/productFormAdapter,invoices/jobs/SellerInvoiceJobPage}`、`features/account/screens/orders/TrackingClient.tsx`
- admin：`src/app/admin/anomalies/page.tsx`
- buyer-app：`features/cart/presentation/{store_map_webview_screen,store_pickup_form_screen}.dart`、`features/address/data/store_pickup_{repository,model}.dart`、`features/order/{data/logistics_track,presentation/widgets/order_logistics_section}.dart`
- seller-app：`features/order/data/payuni_logistics_service.dart`、`features/order/presentation/tcat_reservation_dialog.dart`、`order_card_widgets/order_bottom_row.dart`、`features/settings/{data/shipping_settings_*,data/cvs_shipping_mutation,presentation/shipping_settings/shipping_settings_screen,presentation/store_map_webview_screen}.dart`、`features/shipment_anomaly/**`、`core/router/app_router.dart`

## 8. 開放問題（需決策簡報回答）

| # | 問題 | 為什麼卡 |
|---|---|---|
| Q1 | B3 完成到 B8、B9 之前，Java order、aftersales、notification 怎麼繼續看到「出貨」？選項：Go fulfillment 提供 legacy gRPC（`GetShipmentByOrderNo`、`GetShipmentsByOrderNos`、`BatchGetProviderStatusByOrderIds`、`GetSellerShippingConfig` 等）與 legacy relay 重發 `shipment.*` 事件；或 B3 同時改 Java | 計畫與 ADR 都沒寫；K1、K4、K5、K6 全是跨服務契約（§1.2） |
| Q2 | 賣家「固定運費規則」的存放位置與形狀：SKU JSONB（現況，只有 `feePayer`、`freeThreshold`，沒有金額）、store 層級設定、或 fulfillment 自己的表；是否仍需配送方式、溫層、材積驗證 | ADR D2 只說固定規則；`CheckoutValidator`、`ProductService.validateProductShipping`、錯誤碼 4008、4010、4016 依賴這個答案 |
| Q3 | 逾期自動完成與買家確認收貨怎麼接 B2 的轉帳：`auto_complete_at` 目前由 `ShippingEventConsumer.onShipmentDelivered` 在送達時算；設計書 69 的收貨 Gate（自稱送達不可撥款，只有買家確認或後台裁決）在新模型要不要保留 | 69 已刪；B2 計畫不含「何時轉帳給賣家」的觸發事件定義（【推】需對照 B2 計畫 D1、D6） |
| Q4 | `cart→fulfillment` 依賴邊要不要加（`modules.go` 現沒有）；購物車的運費預覽走 ordering 的 `CheckoutValidate` 還是 fulfillment 的查詢 | 影響 B6 與 B8 的邊界（§1.3 B6 需求） |
| Q5 | K3：賣家新訂單推播在 `shipment.created` 消失後改綁 `order.paid` 還是由 fulfillment 在建立出貨單時發同名事件 | 現綁定理由（建單失敗不通知）在自寄模式不存在；需要 A8 的 notification 同意加邊 |
| Q6 | 免運補貼 ADR（`2026-09-11-free-shipping-subsidy-attribution.md`）是否標 superseded：【量】補貼欄位只存在於已刪物流財務的 V9 migration 與一行註解 | 前提（平台承擔物流成本）已被 2026-11-15 ADR 推翻 |
| Q7 | 物流 port 的最小形狀與模擬 adapter 範圍：ADR D3 寫「建單、取標籤、貨態 webhook 先寫 inbox 以事件 ID 去重」；自寄主線不經 port，port 在 B3 只被測試與模擬 adapter 使用，是否仍在 B3 實作（necessity-condition） | 全域規則要求新機制寫明必要條件；ADR 自己說「port 是否適合綠界要寫真實 adapter 才驗證」 |
| Q8 | `shipping_ambient.go` 的刪除次序：B4 擁有，但 B3 要刪的 `handler/shipping.go:501、598、750` 與 `shipping_options.go:101` 都引用它；順序需協調 | §4.1 |
| Q9 | `routes_payment.go` 的 PayUni webhook 代理與付款導向是 B2 還是 B3 先刪；`reverse_proxy.go` 的 `"payuni-webhook"` 表項誰最後刪 | 計畫 B2、B3 兩邊都寫到這個檔 |
| Q10 | 計畫與 ADR 引用的「設計書 69」已不存在，B3 開工前的「盤點設計書 65–70」實際範圍是 65、06、20、21、22、23（§6.3）；是否要先改計畫文字 | §6 |
| Q11 | 值域刪除範圍：計畫只點名 `ShippingMethod` 3003、`ShipmentAnomalyType` 4 值、`UberDeliveryOutcome`、`ShippingTemperature`、9024；§5 另列 `ShippingType`、`ShippingArea`、`ShippingRoute`、`ProviderSource`、`LabelPrintStatus`、`TcatCallStatus` 等沒有自寄用途的值域；同時 `ShippingMethod` 3001、3002 是否仍作為「承運商」的值 | 追蹤號與物流業者的表示法（自寄回填「物流業者」是自由文字還是列舉）沒有決定 |
| Q12 | 自寄狀態機的值域：目前 `orders.status`（20、30、32、35、40、54）與 `shipping_status`（PayUni 碼）都吃 PayUni 語意；B3 定新值域，B8 才改 `order.proto` 的 `shipping_status` | 「配送異常轉售後」在自寄下由誰判定異常（沒有貨態 webhook） |
| Q13 | `invoice-service`（Go，`client/shipping/client.go`）改接的時機：A5 invoice 計畫「等 B8」，但 shipping RPC 在 B3 就退場 | §1.1 最後一列 |
