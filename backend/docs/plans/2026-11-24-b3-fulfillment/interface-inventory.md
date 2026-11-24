# B3-0 介面盤點：Java shipping-service（唯讀，快照 dev `496b048e2`）

標記：【量】＝跑過指令或打開檔案逐行看過；【推】＝讀程式、proto 註解或方法名推論，沒有逐分支讀完或沒跑。路徑縮寫：`B=backend`、`J=backend/services/java/shipping-service/src/main/java/io/github/will413028/hawkr/shipping`、`R=backend/services/java/shipping-service/src/main/resources`、`T=backend/services/java/shipping-service/src/test/java/io/github/will413028/hawkr/shipping`、`P=backend/services/java/shared/java-proto/src/main/proto/shipping/shipping.proto`、`C=backend/services/java/shared/common-core/src/main/java/io/github/will413028/hawkr/common`、`J-ORD=backend/services/java/order-service/src/main/java/io/github/will413028/hawkr/order`、`J-AS=backend/services/java/aftersales-service/src/main/java/io/github/will413028/hawkr/aftersales`、`G=backend/gateway/internal`。
「去向」欄全部留空（本檔只盤點，不決定）。「Will 11-23 刪除清單?」依 `B/docs/plans/2026-09-24-go-modular-monolith.md` B3 段（第 356–374 行）的清單判斷：`yes`＝清單點名的服務或其唯一服務對象；`no`＝運費規則或出貨狀態；`未列名`＝清單沒點名，括號內是我的推論，由 Will 定。所有指令從 repo 根目錄 `/Users/will/second-brain/.projects/personal/hawkr` 執行。
`B/api/proto/shipping/shipping.proto` 與 `P` 逐字相同（各 1592 行；`diff <(sed 's/[[:space:]]//g' backend/api/proto/shipping/shipping.proto) <(sed 's/[[:space:]]//g' P)` 無輸出）【量】。

## 0. 數字與指令

| 項目 | 量到 | 指令 |
|---|---|---|
| gRPC RPC | **52**（單一 `ShippingService`）；`ShippingGrpcServiceImpl` 的 `public void` 也是 52，一一對應 | `rg -c '^\s*rpc ' P`＝52；`rg -c '^    public void ' J/grpc/ShippingGrpcServiceImpl.java`＝52【量】 |
| main Java 檔／行 | 144 檔／22,656 行（`ShippingGrpcServiceImpl` 2,575、`ShippingService` 2,433、`OnboardingService` 1,161） | `find backend/services/java/shipping-service/src/main -name '*.java' \| wc -l`、`… \| xargs cat \| wc -l`【量】 |
| 其中 Uber | 23 檔／5,439 行 | `find … -ipath '*uber*' -name '*.java' \| xargs cat \| wc -l`【量】 |
| test Java 檔／行／測試數 | 57 檔／11,210 行／393 個 `@Test`（含 `@ParameterizedTest`）；全模組含 pom、Dockerfile 共 220 檔／35,793 行 | `find …/src/test -name '*.java' \| wc -l`；`rg -c '@Test\|@ParameterizedTest' …/src/test` 加總【量】 |
| DB 表 | **19**（V1 建 9、V3 1、V5 1、V13 8），Flyway 15 支（V1–V15） | `rg -c 'CREATE TABLE' R/db/migration` 加總＝19；`ls R/db/migration \| wc -l`＝15【量】 |
| MQ consumer | **5** 個 `@RabbitListener`（各 1）；另有 1 個 Redis pub/sub listener | `rg -n '@RabbitListener' J/mq`【量】 |
| MQ publisher | 2 個類別：`ShippingEventPublisher` 8 個 public 方法、`ShipmentAnomalyEventPublisher` 1 個方法 | `rg -n 'public void publish' J/mq`【量】 |
| `@Scheduled` | **9** 支（`BuyerGrpcClient:45` 的 `@Scheduled` 是註解，不算） | `rg -n '@Scheduled' --glob '*.java' J`【量】 |
| 出站 gRPC | 4 個 client（order 1 方法、seller 2、buyer 3、media 5） | `rg -n 'public .* \w+\(' J/client`【量】 |
| 出站 HTTP | PayUni 物流 10 個 path 常數（shipping 實際用 9 個）＋標籤 PDF 二次 POST；Uber 6 個 endpoint＋OAuth | `rg -n 'PATH_' backend/services/java/shared/payuni-java-client/src/main/java/io/github/will413028/hawkr/payuni/shipping/PayUniShippingEndpoint.java`【量】 |
| Redis | 2 種用法（key `storemap:redirect:<id>`、channel `cvs:print:<shipTradeNo>`） | `rg -n 'KEY_PREFIX\|CHANNEL_PREFIX\|PatternTopic' J`【量】 |
| 呼叫 shipping RPC 的其他程式 | order-service 7 個 RPC、product-service 3、aftersales 5、invoice-service（Go）2、gateway 32（經 `G/grpc/shipping_client.go`）、payuni-webhook-service 1 | 對每個 RPC 的 PascalCase 與 camelCase 名各搜一次 `rg -l`，排除 shipping-service 自己與 `*.pb.go`；結果見 §1 呼叫端欄【量】 |
| 出貨狀態值 | `ShippingStatus` 28 值（兩位數 PayUni 原生 19、三位數平台自訂 9） | `rg -c '^\s+[A-Z_]+\("' C/utils/code/ShippingStatus.java`＝28【量】 |
| 配送異常類型 | `ShipmentAnomalyType` 11 值；shipping 只對其中 6 種有產生者（D5） | `rg -c '^\s+[A-Z_]+\("' C/utils/code/ShipmentAnomalyType.java`＝11【量】 |
| 履約動作 | `FulfillmentAction` 19 值 | `rg -c '^\s+[A-Z_]+\("' C/utils/code/FulfillmentAction.java`＝19【量】 |

## 1. gRPC RPC（52）

行號是 `J/grpc/ShippingGrpcServiceImpl.java` 的方法起點。授權：賣家端 RPC 都 `GrpcAuthContext.requireAccountType(SELLER, SELLER_AND_BUYER)`，買家端 `(BUYER, SELLER_AND_BUYER)`，admin 端 `(ADMIN)`，服務間 RPC 不檢（`rg -n 'requireAccountType' J/grpc/ShippingGrpcServiceImpl.java`【量】）。多數 RPC 把 `ShippingBusinessException` 轉成 `response.code != 0`（gRPC 狀態仍是 OK），其餘例外走 `GrpcErrorHandler`【量】。

### 1.1 賣家安排出貨（6）

| 項目 | file:line | 行為摘要 | 呼叫端 | 去向 | Will 11-23 刪除清單? |
|---|---|---|---|---|---|
| PrintCvsLabel | grpc:125 | 呼叫 `ShippingService.printCvsLabel`（`ShippingService:369`）：首次列印以 `label_print_status` 4101/4104→4102 的 CAS 搶佔，打 PayUni `/api/logistics/print_label`，在 Redis channel 等 webhook 回填三段碼；重印走 `handleReprintCvsLabel`。【推】`ShippingService:399-480` 只讀到結構 | gateway `handler/shipping.go`、`routes_shipping.go` | | yes（`ShipmentLabel*`、PayUni） |
| BatchPrintCvsLabel | grpc:189 | 逐筆 CAS＋PayUni 批次列印；刻意不放 `@Transactional`（`BatchPrintTransactionShapeTest` 釘住）【量】 | gateway `handler/shipping.go` | | yes |
| GenerateHomeLabelPdf | grpc:256 | 宅配取號：PayUni `/api/home_delivery/get_obt_number_pdf` → 二次 POST 下載 PDF → 上傳 media → 回 label_key【推】 | gateway | | yes |
| BatchGenerateHomeLabelPdf | grpc:297 | `BatchHomeLabelService` 並行 fan-out（`home-label-concurrency` 10、`batch.max-size` 50，`application.yml`）【量】 | gateway | | yes（`BatchHomeLabelService`） |
| CallTcat | grpc:369 | 呼叫黑貓叫件：`tcat_call_status` 4201/4204→4202 CAS → PayUni `/api/home_delivery/call_cat` → 成功寫 4203、`status=210`、`processing_deadline=NULL`（`ShipmentMapper:218-242`），發 `shipment.processing.cleared`（`ShippingService:1013`）【量】 | gateway | | yes（Tcat） |
| ConfirmCvsShipment | grpc:167 | 超商賣家確認出貨：已 921 冪等回傳；`allowedActionResolver.require(HANDOVER)`；`label_print_status` 必須 4103；終態拒絕；`updateShipmentStatus` CAS 到 921；出貨照片可選（`ShippingService:526-600`）【量】 | gateway | | 未列名（推：屬超商流程，921 狀態一併消失） |

### 1.2 查詢出貨單、追蹤（8）

| 項目 | file:line | 行為摘要 | 呼叫端 | 去向 | Will 11-23 刪除清單? |
|---|---|---|---|---|---|
| GetShipmentByOrderNo | grpc:431 | `shipmentMapper.selectOne(eq order_no)`，**沒有 `is_return=false` 條件**（`ShippingService:1264`）；找不到回 NOT_FOUND；成功回 `ShipmentDetailView`（`buildDetailView`，grpc:1091，含 `allowed_actions`）【量】 | order-service `OrderGrpcService:151`（經 `ShippingGrpcClient:61`）；invoice-service Go `internal/client/shipping/client.go` | | no |
| GetShipmentsByOrderNos | grpc:410 | `.in(order_no, list)` 全撈，空 list 回空，**無筆數上限**；同 order_no 的正向與退貨單都回（`ShippingService:1273`）【量】 | order-service `OrderGrpcService:135`；invoice-service Go（`client.go`、`client_test.go`） | | no |
| BatchGetProviderStatusByOrderIds | grpc:1883 | `findProviderStatusByOrderNos(orderNos, isReturn)` 回 map（key→provider_status、provider_message）；null key 跳過；無授權檢查【量】 | order-service `OrderGrpcService:282`；aftersales `AfterSalesGrpcService:1161`（`isReturn=true`） | | no（出貨狀態讀取） |
| QueryShipmentStatus | grpc:467 | 以 shipment_no 取單，`syncAndTouch`（`ShippingService:1377`）：有 `TrackCapability` 且有 `ship_trade_no` 才對 provider 查詢，合法轉移才寫，`finally` 一律 `touchLastReconciledAt`；回 `ShipmentDetailView`【量】 | **無 production 呼叫端**（Java、Go、gateway 皆無）【量】 | | 未列名（推 yes：PayUni 主動查詢） |
| GetShipmentTracking | grpc:490 | 買家：`getBuyerTrackingByOrderNo`（`ShippingService:1322`）：`is_return=false` 取單；`buyerId` 不符回 `SHIPMENT_PERMISSION_DENIED`；`syncAndTouch`（讀取會寫入）【量】 | gateway `handler/shipping.go` | | 未列名（推：狀態讀取保留，provider 同步消失） |
| GetSellerShipmentTracking | grpc:514 | 賣家版：以 `order_no + is_return` 取單，`storeId` 不符拒絕；同樣 `syncAndTouch`（`ShippingService:1350`）【量】 | gateway `handler/shipping.go`、`routes_shipping.go` | | 未列名（同上） |
| LookupShipTradeNo | grpc:1840 | 以 order_no/is_return 取最近一筆有 `ship_trade_no` 的單（`ShippingService:1285`）；供 webhook 模擬器【推】 | payuni-webhook-service `WebhookSimulatorController` | | 未列名（推 yes：隨 payuni-webhook-service 刪） |
| ListBusinessDays | grpc:1494 | 賣家限定；`holidayService.getNextBusinessDays(7)`（台北今日起 60 天內取 7 個非假日，回日期與「一二三…」星期字）【量】 | gateway `handler/shipping.go`、`routes_shipping.go:64` | | yes（`HolidayService`；假日已歸 `platform/bizcalendar`） |

### 1.3 賣家建店（物流開通，6）

| 項目 | file:line | 行為摘要 | 呼叫端 | 去向 | Will 11-23 刪除清單? |
|---|---|---|---|---|---|
| OnboardCvsShipping | grpc:540 | 賣家限定；`OnboardingService`：首次提交 PayUni `/api/logistics/setting`，ACTIVE 再送只更新本地；寫 `seller_shipment_settings`＋`seller_shipment_onboarding_logs`，`payuni_submit_status` 4301→4302→4303/4304【推】（`OnboardingService` 1,161 行只讀了結構與狀態寫入點） | gateway | | yes（`OnboardingService`） |
| OnboardHomeDeliveryShipping | grpc:569 | 同上，宅配（`/api/home_delivery/setting`），地址一旦送審即鎖（`addressLocked`，`OnboardingService:1037`）【量】 | gateway | | yes |
| ListShippingOnboardings | grpc:602 | 列賣家設定，`buildOnboardingView`（grpc:1032）【推】 | gateway | | yes |
| GetShippingOnboarding | grpc:634 | 單筆設定【推】 | gateway | | yes |
| ToggleShippingOnboarding | grpc:664 | 切 `enabled`，不通知 PayUni（proto 註解）【推】 | gateway | | yes |
| ListSellerShippingMethods | grpc:694 | `onboardingService.listSellerShippingMethods(storeId)` 回 `SellerShippingMethodSlot`（method、goodsType、available、onboardingId）【量】 | gateway `handler/shipping.go` | | yes |

### 1.4 買家常用門市與門市地圖（7）

| 項目 | file:line | 行為摘要 | 呼叫端 | 去向 | Will 11-23 刪除清單? |
|---|---|---|---|---|---|
| AddBuyerStorePickup | grpc:874 | 買家限定；`BuyerStorePickupService` 寫 `buyer_store_pickups`（預設門市旗標）【推】 | gateway | | yes |
| UpdateBuyerStorePickup | grpc:907 | 同上【推】 | gateway | | yes |
| ListBuyerStorePickups | grpc:958 | 同上【推】 | gateway | | yes |
| DeleteBuyerStorePickup | grpc:988 | 同上【推】 | gateway | | yes |
| GetStoreMapParams | grpc:738 | 取 `StoreLocatorCapability.buildRedirectParams`（加密參數、簽章、return URL），供 WebView Form POST 到 PayUni `/api/logistics/ship_map`【量】 | gateway `handler/store_map.go` | | yes（`StoreMapService`） |
| GetStoreMapRedirectForm | grpc:843 | 以 opaque id 從 Redis `getAndDelete`（key `storemap:redirect:<id>`、TTL 10 分，`StoreMapRedirectFormStore:27,29`）取回表單，一次性消費【量】 | gateway `handler/store_map.go` | | yes |
| DecryptStoreMapResult | grpc:781 | 解 PayUni 門市選擇回調（`parseResult`）【推】 | gateway `handler/store_map.go` | | yes |

### 1.5 運費與商品配送驗證（5，服務間）

| 項目 | file:line | 行為摘要 | 呼叫端 | 去向 | Will 11-23 刪除清單? |
|---|---|---|---|---|---|
| GetShippingFeeConfigs | grpc:1189 | 回記憶體快取的全部 `shipping_method_configs`（含 `commission_rate`、`commission_min`）與 `shipping_fee_tiers`；數值全以字串傳；快取啟動載入、每 3,600,000 ms 重載（`ShippingFeeService:66`）【量】 | order-service `ShippingFeeGrpcFetcher:45`（circuit breaker `shipping-fee-service`）；order 只取 configs 的 7 欄，**丟掉 commission 兩欄**【量】 | | no（運費規則；但 order 的 `ShippingFeeGrpcFetcher` 在刪除清單） |
| LookupShippingFee | grpc:1237 | method/temp/zone 任一為空回錯；`dimensionSum` 轉 int（預設 0）；`lookupFee` 找級距，附 `estimated_days`、`settlement_days`【量】 | **無 production 呼叫端**【量】 | | no |
| GetSellerShippingConfig | grpc:1297 | `storeId<=0` 回錯；對 `listSellerShippingMethods` 的每個 slot 附上該 setting 的寄件郵遞區號、城市、區（供運費路線推導）【量】 | order-service `CheckoutService:996,1256,1282`；product-service `ProductService`（經 `ShippingGrpcClient:92`） | | yes（資料來自 `OnboardingService`；結帳與商品兩端依賴它，見 §11） |
| CalculateShippingOptions | grpc:1361 | 長寬高重量缺一回錯；算 `dimensionSum`、最長邊；`storeId>0` 時以賣家已開通 slot 過濾；只考慮 `TAQBIN`＋`SEVEN_ELEVEN_STORE_PICKUP`（`ShippingFeeService:211` 寫死）；對每個溫層逐一檢查三個上限、查級距；7-11 強制 `DOMESTIC` 區；回溫層分組【量】 | product-service `ProductGrpcService:1395`（再經 gateway `handler/shipping_options.go`） | | no（運費計算；溫層待 B4/B6 移除） |
| ValidateProductShipping | grpc:1530 | `ShippingFeeService.validateProductShipping`（`:278-392`）：逐 SKU 產生拒絕原因 `METHOD_NOT_AVAILABLE`／`METHOD_NOT_ONBOARDED`／`TEMPERATURE_NOT_ONBOARDED`／`METHOD_NOT_SUPPORTED`／`DIMENSION_SUM_EXCEEDED`／`SINGLE_SIDE_EXCEEDED`／`WEIGHT_EXCEEDED`，訊息文字寫死在 Java（繁中）【量】 | product-service `ProductService`（經 `ShippingGrpcClient:117`） | | no |

### 1.6 逆物流（退貨，4）

| 項目 | file:line | 行為摘要 | 呼叫端 | 去向 | Will 11-23 刪除清單? |
|---|---|---|---|---|---|
| CreateReturnCvsShipment | grpc:1634 | `ShippingService.createReturnCvsShipment`（`:1053`）：建 `is_return=true` 的單（與正向單同 `order_no`）、PayUni 退貨便、非同步列印（`scheduleAsyncCvsPrint`）；失敗落地失敗殼（990）【推】 | aftersales `AfterSaleResolutionService:158` | | 未列名（推 yes：PayUni 建單） |
| CreateReturnHomeShipment | grpc:1683 | 同上宅配（`:1173`）【推】 | aftersales `AfterSaleResolutionService:168` | | 未列名（推 yes） |
| GetSellerReturnConfig | grpc:1734 | 讀賣家退貨設定（`seller_shipment_settings` 的 return_* 欄，`ShippingService:1226`）【推】 | aftersales `ShippingGrpcClient:143`（wrapper；沒找到 production 呼叫點） | | 未列名（推 yes） |
| RetryReturnPrintLabel | grpc:1804 | 以 ship_trade_no 重列印退貨標籤（`ShippingService:1137`）【推】 | aftersales `AfterSaleLabelRetryService:114,220` | | 未列名（推 yes） |

### 1.7 Admin 查詢（3）、配送異常（7）

| 項目 | file:line | 行為摘要 | 呼叫端 | 去向 | Will 11-23 刪除清單? |
|---|---|---|---|---|---|
| ListShipmentsForAdmin | grpc:1916 | admin 限定；`ShippingAdminQueryService.listShipments`：分頁（預設 20、上限 100）、`abnormal_only`（7 個狀態值，`ShippingAdminQueryService:42-50`）、provider_status 精確、status 多值、is_return、method、store_id、order_no、時間區間、4 種排序【量】 | gateway `handler/admin_shipping.go` | | no（出貨狀態讀取） |
| GetShipmentForAdmin | grpc:1935 | admin 單筆詳情（含 sender/receiver snapshot）【推】 | gateway `handler/admin_shipping.go` | | no |
| ListSellerShipmentSettingsForAdmin | grpc:1962 | admin 列賣家建店設定【推】 | gateway `handler/admin_shipping.go` | | yes（`OnboardingService` 資料） |
| ListSellerShipmentAnomalies | grpc:2098 | 賣家：`selectSellerAnomalies`，anomaly 由 `shipments` 以 SQL `CASE` 現場推導（`ShipmentAnomalyQueryMapper:30-90`）再 LEFT JOIN ack 表；頁大小上限 100【量】 | gateway `handler/shipment_anomaly.go` | | yes（`ShipmentAnomalyService`） |
| GetSellerShipmentAnomalyStats | grpc:2141 | 依類型計數、依 severity 加總【量】 | gateway | | yes |
| AcknowledgeShipmentAnomaly | grpc:2186 | 賣家：驗 `store_id` 歸屬，upsert ack（select 再 insert/update，非原子，`ShipmentAnomalyService.upsertAck`）【量】 | gateway | | yes |
| IgnoreShipmentAnomaly | grpc:2221 | 同上，action=IGNORED【量】 | gateway | | yes |
| ListShipmentAnomaliesForAdmin | grpc:2256 | admin 版列表【量】 | gateway | | yes |
| AcknowledgeShipmentAnomalyForAdmin | grpc:2300 | admin 版 ack（`actor_type=ADMIN`）【量】 | gateway | | yes |
| IgnoreShipmentAnomalyForAdmin | grpc:2335 | 同上 IGNORED【量】 | gateway | | yes |

### 1.8 Uber（6）

| 項目 | file:line | 行為摘要 | 呼叫端 | 去向 | Will 11-23 刪除清單? |
|---|---|---|---|---|---|
| GetUberQuoteForCheckout | grpc:2468 | 結帳報價：`UberQuoteService` 打 Uber `delivery_quotes`，寫 `uber_quotes`；先 buyer `ValidateAddress`（舊單體 `legacy/buyerserver/server.go:122` 為它回 9700 假值）【推】 | order-service `ShippingUberQuoteAssessor:65` | | yes（Uber） |
| ValidateUberQuoteForCheckout | grpc:2497 | 確認時以 payload hash 驗報價未過期未被改（`UberQuotePayloadHash`）【推】 | order-service `ShippingUberQuoteAssessor:89` | | yes |
| ListSellerUberDeliveries | grpc:1987 | 賣家 Uber 配送列表【推】 | 無 production 呼叫端【量】 | | yes |
| GetSellerUberDelivery | grpc:2014 | 單筆【推】 | 無 | | yes |
| ListUberDeliveriesForAdmin | grpc:2050 | admin 列表【推】 | 無 | | yes |
| GetUberDeliveryForAdmin | grpc:2069 | admin 單筆【推】 | 無 | | yes |

## 2. DB 表與 Flyway

資料庫 `hawkr_shipping`（`application.yml`），主鍵 ID 用 TSID（`id-type: INPUT`）【量】。

| 表 | 來源 migration | 主要欄位與約束（【量】讀 DDL） | 對應功能 | 去向 | Will 11-23 刪除清單? |
|---|---|---|---|---|---|
| `shipments` | V1:17；V2、V4、V7、V9、V10、V11、V12、V15 改 | PK `id`；`shipment_no`（唯一）、`order_no`、`store_id`（V4 由 `seller_account_id` 改名）、`buyer_id`（V9 改名）、`shipping_method`（CHECK 現為 3001–3003、3005，V15）、`status`（VARCHAR(4)）、`provider_status`（32）、`provider_message`、`provider_status_time`、`ship_trade_no`、`uber_delivery_id`、`cvs_store_code`、`label_print_status` 4101–4104、`tcat_call_status` 4201–4204、`label_object_key`／`label_asset_id`／`label_expire_at`、`partner_id`／`odno`／`validation_no`（超商三段碼）、`receipt_confirmed`、`sender_snapshot`／`receiver_snapshot`／`shipment_photos`（JSONB）、`is_return`、`last_reconciled_at`、`processing_deadline`、`expected_delivery_date`（V2）、租約欄（V10）、`status_entered_at`（V11，aging 唯一錨）、取消欄與回查欄（V11）、provider 錯誤欄（V12）；唯一索引：`shipment_no`、`order_no WHERE is_return=FALSE`；`BEFORE UPDATE` trigger 設 `updated_at=NOW()`（V1:115-120） | 出貨單主表 | | 部分保留（狀態欄）；PayUni／Uber／Tcat／標籤欄 yes |
| `shipment_status_logs` | V1:130；V11 加欄 | `shipment_id`、`from_status`、`to_status`、`provider_status`、`trigger_source`、`raw_payload`、`subject_type`、`actor_type`、`actor_account_id`、`correlation_id` | 狀態歷程 | | no |
| `shipping_webhook_dedup` | V1:150 | `event_key`（唯一）、`ship_trade_no`、`process_status`、`processed_at`；`DedupCleanupScheduler` 保留 7 天【量】 | webhook 去重（PayUni、Uber 共用） | | 未列名（ADR D3 要求 inbox 並以事件 ID 去重） |
| `seller_shipment_settings` | V1:167；V4 改名 | 賣家寄件人、退貨地址、超商退貨門市、`mer_id`（NOT NULL，PayUni 子商店）、`partner_id`、`eship_id`、`onboarding_status` 3601–3605、`enabled`、`payuni_submit_status` 4301–4304；唯一 `(store_id, shipping_type, COALESCE(goods_type,''))` | 物流開通 | | yes |
| `seller_shipment_onboarding_logs` | V1:236 | 開通歷程 | 物流開通 | | yes |
| `shipping_method_configs` | V1:260；V1 內 seed 5 筆 | 唯一 `(shipping_method, delivery_temp)`；`max_dimension_sum_cm`、`max_single_side_cm`、`max_weight_grams`、`estimated_days`、`settlement_days`、`commission_rate NUMERIC(5,4)`、`commission_min`、`enabled`。seed：3002×{3201,3203}、3001×{3201,3202,3203} | 運費規則（尺寸上限） | | no |
| `shipping_fee_tiers` | V1:306；seed 32 筆 | 唯一 `(shipping_method, delivery_temp, delivery_zone, max_dimension_cm)`；`fee BIGINT`；`max_dimension_cm=0` 表示無上限；seed：7-11 兩筆統一價（65、165）、黑貓常溫 3 區×4 級距、冷藏與冷凍各 3 區×3 級距 | 運費規則（費率） | | no |
| `buyer_store_pickups` | V1:375；V8 改名 | 買家常用超商門市，`store_code` 6 碼 | 門市 | | yes |
| `holidays` | V1:405 | `holiday_date`（唯一）、`name`、`year`。**repo 內沒有任何 INSERT**（D6）【量】 | 假日 | | yes |
| `shipment_anomaly_acknowledgments` | V3；V6 加 `actor_type`、`seller_id`→`actor_id` | seller／admin 各一列 | 異常處置 | | yes |
| `payuni_audit_logs` | V5 | PayUni 出站稽核（`trade_no`、`refund_no`…）；`PayUniAuditCleanupScheduler` 預設保留 2555 天（7 年）【量】 | PayUni 稽核 | | yes（PayUni） |
| `uber_account`、`uber_organization`、`uber_store_mapping`、`uber_quotes`、`uber_deliveries`、`uber_pod_artifacts`、`packing_policy`、`package_plan` | V13（8 表）；V14 補唯一索引 | Uber 帳號、組織、門市映射、報價、配送、送達憑證、裝箱 | Uber | | yes（Uber） |

migration 歷史【量】：V1 初始（9 表＋3 個 `updated_at` trigger＋`update_updated_at_column()`）、V2 預計送達日、V3 異常 ack、V4 `seller_account_id`→`store_id`（shipments 把舊 `store_id` 先改成 `cvs_store_code`）、V5 PayUni 稽核、V6 ack 加 actor、V7 刪 `label_url`、V8、V9 改名、V10 租約欄、V11 物流履約平台地基（CHECK 放寬到 3004/3005、`status_entered_at`、取消欄、log 加 subject/actor）、V12 provider 錯誤欄、V13 Uber 8 表、V14 Uber 唯一索引、V15 刪 3004。

## 3. MQ

### 3.1 Consumer（5）

| 項目 | file:line | queue／binding／payload | 行為摘要 | 去向 | Will 11-23 刪除清單? |
|---|---|---|---|---|---|
| OrderPaidConsumer | `J/mq/OrderPaidConsumer.java:38` | queue `shipping.order.paid`，綁 `hawkr.order`／`order.paid`；讀 `order_no`；manual ack | 空 `order_no` 直接 ack 丟棄；呼叫 `createShipmentFromOrder`（`ShippingService:265`）；失敗走 `MqErrorHandler.handleFailure`（重試後進 `shipping.order.paid.dlq`）【量】 | | yes（建單走 PayUni）；事件本身是出貨單建立的起點 |
| ShipmentCreateRequestedConsumer | `J/mq/ShipmentCreateRequestedConsumer.java:38` | queue `shipping.shipment.create_requested`，綁 `order.shipment.create_requested`（order 的 `ShipmentCreationBackstopScheduler` 發）；讀 `order_no` | 同上，呼叫同一個 `createShipmentFromOrder`【量】 | | yes（order 的 backstop 在清單） |
| OrderReceiptConfirmedConsumer | `J/mq/OrderReceiptConfirmedConsumer.java:39` | queue `shipping.order.receipt.confirmed`，綁 `order.receipt.confirmed`；讀 `order_no`、`confirm_type` | 只做 `UPDATE shipments SET receipt_confirmed=TRUE WHERE order_no=? AND receipt_confirmed=FALSE`；任何例外 `basicNack(requeue=false)`（`:72`），不經 `MqErrorHandler`【量】 | | 未列名 |
| ShippingWebhookConsumer | `J/mq/ShippingWebhookConsumer.java:52` | queue `shipping.payuni.webhook`，綁 `payuni.events`／`payuni.shipping.status`；PayUni 表單欄位 `ShipTradeNo`、`ShipStatus`、`ShipStatusDesc`、`ShipStatusTime`（及列印三段碼欄位） | 1) 無 `ShipTradeNo` ack 丟棄；2) 找不到 inbound capability 走相容入口 `processWebhookStatusUpdate`（**不去重**）；3) `dedupKey` 查 `shipping_webhook_dedup`，已有則跳過；4) `isLabelEvent` 則 `processWebhookPrint`（寫三段碼、920、發 `label.updated`、`cvsPrintWaitingRegistry.complete` 發 Redis）；否則 `applyInboundStatus`（`ShippingService:1472`，`@Transactional`）；5) 處理後才 `recordDedup`；失敗走 `MqErrorHandler`【量】 | | yes（PayUni）；inbox＋去重是 ADR D3 保留項 |
| UberWebhookConsumer | `J/mq/UberWebhookConsumer.java:61` | queue `shipping.uber.webhook`，綁 `payuni.events`／`uber.shipping.status` | 驗簽在 payuni-webhook-service；此處去重（共用 dedup 表）＋ `UberWebhookCorrelation` 對應 delivery＋狀態寫入【推】 | | yes（Uber） |
| （Redis）CvsPrintWaitingRegistry | `J/service/CvsPrintWaitingRegistry.java:79`、`J/config/CvsPrintRedisConfig.java:30` | pub/sub pattern `cvs:print:*` | 同步等待 PayUni 列印回調的跨 pod 通知【量】 | | yes |

queue 拓樸另登記在 `B/infra/mq/topology/topology.yaml`（`RabbitListenerTopologyRegistrationTest` 守每個 queue 都登記）；`shipping.*.dlq` 經 exchange `shipping.webhook.dlx`【量】。`hawkr.shipping.dlx` exchange 在 `J/config/RabbitMqConfig.java:61` 與 topology 宣告，**沒有任何 queue 用它當 DLX**（`rg 'hawkr.shipping.dlx' B/infra B/services` 只命中宣告）【量】。

### 3.2 Publisher（exchange `hawkr.shipping`，topic）

| 項目 | file:line | routing key／eventType | payload 欄位 | 觸發 | 下游 consumer | 去向 | Will 11-23 刪除清單? |
|---|---|---|---|---|---|---|---|
| publishShipmentCreated | `J/mq/ShippingEventPublisher.java:30` | `shipment.created`／`SHIPMENT_CREATED` | `ShipmentCreatedPayload`：eventId、orderNo、shipmentNo、shipTradeNo、shippingMethod、status、labelUrl、processingDeadline、sellerAccountId、buyerId、totalActualAmount、buyerNickname、buyerAvatarKey、storeId（皆字串）；失敗拋 `INTERNAL_ERROR` | `createShipmentFromOrder` 步驟 7（`ShippingService:1948-1998`） | order `order.shipping.created`；notification（`notification.shipping` 綁 `#`，`topology.yaml:277-282`） | | 未列名（出貨單建立事件，需重寫） |
| publishShipmentStatusChanged | `:48` | `shipment.status.changed` | `ShippingEventPayload`：eventId、orderNo、shipmentNo、shippingMethod、status、statusCode、statusDesc、labelKey、shipTradeNo、labelExpireAt、isReturn、sellerName、sellerAvatarKey、buyerId、notifyBuyerAccountId、sellerAccountId、providerStatus、providerMessage、storeId；失敗拋例外 | `updateStatusWithParsedTime`（`ShippingService:2024`）非退貨、非 PICKED_UP | order `order.shipping.status`（推進訂單狀態）；notification | | no（出貨狀態） |
| publishShipmentDelivered | `:64` | `shipment.delivered` | 同 `ShippingEventPayload` | 轉到 11 PICKED_UP（非退貨）；同時刪標籤（`labelService.unlinkLabel`） | order `order.shipping.delivered`（→ DELIVERED，設 `auto_complete_at`）；notification | | no |
| publishReturnShipmentStatusChanged | `:56` | `shipment.return.status_changed` | 同上 | `is_return=true` 的任何狀態轉移 | aftersales `aftersales.shipping.return.status` | | 未列名（退貨物流） |
| publishLabelUpdated | `:89` | `shipment.label.updated` | `LabelUpdatedPayload`：eventId、orderNo、shipmentNo、partnerId、odno、validationNo、isReturn；**吞例外只 log** | 列印 webhook 回填三段碼（`ShippingService:1736`） | order `order.shipping.label`；aftersales `aftersales.shipping.label` | | yes（超商標籤） |
| publishShipmentPhotoUpdated | `:72` | `shipment.photo.updated` | `ShipmentPhotoPayload`：eventId、orderNo、shipmentPhotoKeys；**吞例外** | 出貨照片寫入（`ShippingService:1815`） | order `order.shipping.photo` | | 未列名 |
| publishProcessingCleared | `:108` | `shipment.processing.cleared` | `Map.of("orderNo")`，**無 eventId、手拼 JSON**；**吞例外** | 賣家完成處理（叫件 `ShippingService:1013`、列印回填 `:1756`）與逾期掃描（`ProcessingDeadlineScheduler:116`） | order `order.shipment.processing.cleared`（清 `orders.processing_deadline`） | | 未列名 |
| publishProcessingTimeout | `:133` | `shipment.processing.timeout` | `Map.of(orderNo, sellerAccountId(字串), buyerId(字串))`，**無 eventId**；**吞例外** | `ProcessingDeadlineScheduler:120-130` | aftersales `aftersales.shipment.processing.timeout` → `ProcessingTimeoutConsumer` → `SellerCancelService.cancelShipmentBySystem`（系統取消＋退款）【量】 | | 未列名（「逾期未出貨」狀態機的輸出，需重寫） |
| publishAnomalyDetected | `J/mq/ShipmentAnomalyEventPublisher.java:65` | `shipment.anomaly.detected`／`SHIPMENT_ANOMALY_DETECTED` | `ShipmentAnomalyDetectedPayload`（`C/mq/payload`）：eventId（隨機 UUID，`:77`）、orderNo、shipmentId、sellerAccountId、buyerAccountId、anomalyType、anomalyTypeLabel、severity、detectedAt、storeId；先 gRPC 解析賣家與買家 account（解不到帶 0）；fire-and-forget | 5 個呼叫點：`ShippingService:1704`、`ShipmentLabelReconcileScheduler:188,222`、`ShipmentStatusReconcileScheduler:164,183`（另含 `providerStatusAnomalyOf` 路徑）、`TcatCallStuckAlertScheduler:119` | notification `notification.shipping`（模板 `shipment_anomaly`，賣家強制開）；aftersales 的 `ReturnStuckAlertScheduler` 也發同 key；**沒有 aftersales 消費者**【量】 | | yes（`ShipmentAnomalyService`＋排程） |

order 側 6 條 shipping queue 都在 `J-ORD/mq/ShippingEventConsumer.java`（`:75,121,158,209,265,390`）。訂單狀態對應（`:251-260`）：91／92／21／920→PENDING_SHIPMENT(20)；210／921／31→IN_TRANSIT(30)；32→WAITING_PICKUP(32)；33／43／44／46／51–56／81／82→SHIPPING_ABNORMAL(35)；98／22／41 不推進；delivered→DELIVERED(40)＋`auto_complete_at`【量】。

### 3.3 shipping 事件的上游（order 發出，shipping 收）

`order.paid`（`J-ORD/mq/OrderEventPublisher`、`PaymentEventHandler`）、`order.shipment.create_requested`（`ShipmentCreationBackstopScheduler`：每 5 分鐘掃已付款無出貨單且過 30 分寬限的訂單，最多 5 次、間隔 30 分，超過取消退款，`order-service/application.yml:131-141`）、`order.receipt.confirmed`（`OrderCompletionTxHandler`）【量】。

## 4. `@Scheduled`（9）與其他背景機制

| 項目 | file:line | 週期（預設）與行為 | 去向 | Will 11-23 刪除清單? |
|---|---|---|---|---|
| ProcessingDeadlineScheduler.scanExpiredShipments | `J/scheduler/ProcessingDeadlineScheduler.java:75` | `fixedDelay 300000`、`initialDelay 90000`，`hawkr.shipping.processing-deadline.enabled`（預設開）。`findExpiredUnprocessedShipments`（`ShipmentMapper:445-457`：`processing_deadline < now`、`is_return=FALSE`、`status<>'41'`、且 `(3002 且 label_print_status<>4103) 或 (3001 且 tcat_call_status<>4203)`，每批 50）→ 每筆 lease（`leaseProcessingDeadline` 推到 now+5 分）→ 發 `processing.cleared` → 批次 `batchGetSellerMeta` 解 accountId → 發 `processing.timeout` → `clearProcessingDeadline`。deadline 由建單設：超商 `now+72h`（`CvsShipmentStrategy:133`）、宅配「3 個工作日 23:59:59 台北」（`HomeShipmentStrategy:160-166`）；賣家叫件或列印成功時設 NULL【量】 | | 未列名（「逾期未出貨」；predicate 與 deadline 來源都綁 PayUni） |
| ShipmentStatusReconcileScheduler.reconcile | `J/scheduler/ShipmentStatusReconcileScheduler.java:77` | `fixedDelay 600000`，`reconcile.enabled`。候選：`status NOT IN (11,41)`、`shipping_method<>'3003'`、有 `ship_trade_no`、`created_at >= now-7d`、`last_reconciled_at` 為空或早於 15 分前，每批 100；`claimForReconcile` 租約；`queryAndSyncStatus`；再發異常：status 33／44、32 卡 7 天、超過預計送達日且 provider 3 天無更新【量】 | | yes（貨態對帳） |
| ShipmentLabelReconcileScheduler | `J/scheduler/ShipmentLabelReconcileScheduler.java:99` | `fixedDelay 600000`；撈 `label_print_status IN (4102,4104)` 且過 5 分寬限、`max-age-days 7`，向 PayUni 取三段碼補成功，或超過 48h 發 `CVS_LABEL_UNPRINTED`、無三段碼發 `PAYUNI_LGS_NO_CODE`【推】 | | yes（標籤對帳） |
| TcatCallStuckAlertScheduler | `J/scheduler/TcatCallStuckAlertScheduler.java:80` | `fixedDelay 600000`；`tcat_call_status=4202` 且過 3 分寬限、`max-age 24h`，租約後發 `TCAT_CALL_FAILED`【量】 | | yes（Tcat） |
| PayUniSubmitStuckAlertScheduler | `J/scheduler/PayUniSubmitStuckAlertScheduler.java:47` | `fixedDelay 600000`；`seller_shipment_settings` 送審卡住告警（`max-age-hours 24`）【推】 | | yes（`OnboardingService`） |
| DedupCleanupScheduler | `J/scheduler/DedupCleanupScheduler.java:28` | cron `0 0 3 * * ?`，刪 `shipping_webhook_dedup` 7 天前（`:20`）【量】 | | 未列名（隨 dedup 表） |
| PayUniAuditCleanupScheduler | `J/scheduler/PayUniAuditCleanupScheduler.java:36` | cron `0 30 3 * * ?`，保留 2555 天【量】 | | yes（PayUni） |
| ShippingFeeService.refresh | `J/service/ShippingFeeService.java:66` | `fixedRate 3600000` 重載運費快取；`@PostConstruct` 先載一次；`ComponentReadiness` 以 `cacheLoaded` 回報就緒【量】 | | no |
| HolidayService.scheduledRefresh | `J/service/HolidayService.java:62` | cron `0 0 3 1 * *`（UTC）重載假日快取【量】 | | yes |

order 側相關排程（不在 shipping，但屬狀態機）：`AutoCompleteScheduler`（送達後 `auto-complete-hours` 預設 60 小時，每小時掃，批 100；`order-service/application.yml:126-128`）、`ShipmentCreationBackstopScheduler`（§3.3）【量】。

## 5. 出站呼叫

| 對象 | 呼叫 | file:line | 用途 | 去向 | Will 11-23 刪除清單? |
|---|---|---|---|---|---|
| order（gRPC，`ORDER_SERVICE_GRPC_PORT` 9915） | `GetOrderForShipping` | `J/client/OrderGrpcClient.java:39` | 建單時取收件人、金額、store_id、配送方式、尺寸（`OrderShippingInfo`） | | 未列名（出貨單建立仍需訂單資料） |
| seller（gRPC，舊單體 `LEGACY_SELLER_GRPC_PORT` 9941） | `BatchGetSellerMeta`、`GetSellerShippingProfile` | `J/client/SellerGrpcClient.java:35,47` | storeId→accountId 解析（事件 payload）；寄件 contact_email（舊單體 `internal/legacy/sellerserver/server.go:403`，回物流 MerId 隨 Java shipping 退場刪） | | 未列名 |
| buyer（gRPC，舊單體 9941） | `BatchGetBuyerDisplayByBuyerIDs`、`ResolvePrincipals`、`ValidateAddress` | `J/client/BuyerGrpcClient.java:83,105,144` | 建單事件的買家暱稱頭像、買家 accountId、Uber 地址驗證 | | 未列名；`ValidateAddress` 是 Uber 專用 |
| media（gRPC，`MEDIA_SERVICE_GRPC_PORT` 9937） | `ServerUploadAndFinalize`、`ServerUploadSecureAndFinalize`、`FinalizeImages`、`LinkImages`、`UnlinkImages` | `J/client/MediaGrpcClient.java:33,68,92,113,134` | 標籤 PDF／圖片、出貨照片、Uber POD 上傳與關聯 | | yes（標籤、Uber）；出貨照片未列名 |
| PayUni HTTP | `/api/logistics/trade`、`/api/home_delivery/trade`、`/api/home_delivery/get_obt_number_pdf`、`/api/logistics/print_label`、`/api/home_delivery/call_cat`、`/api/logistics/query`、`/api/logistics/setting`、`/api/home_delivery/setting`、`/api/logistics/ship_map`（表單由前端 POST）；`PATH_HOME_REFUND` 只在 `payuni-java-client` 的 `ShippingClient:131`，shipping 沒呼叫 | `backend/services/java/shared/payuni-java-client/.../PayUniShippingEndpoint.java:17-53` | 超商／宅配建單、標籤、叫件、查詢、開通 | | yes |
| PayUni 標籤 PDF 二次 POST | 以 actionUrl 解析表單欄位再 POST，結果寫稽核 | `J/service/ShipmentLabelService.java:225-300` | 下載黑貓託運單 PDF | | yes |
| Uber HTTP（`uber.enabled` 預設 false） | E1 `delivery_quotes`、E2 建單、E3 列表、E4 取單、E6 取消、E7 POD；OAuth `https://auth.uber.com/oauth/v2/token`（記憶體快取 token，刻意不放 Redis） | `J/provider/uber/UberApiClient.java:34-38,126,166,184,201,216,237`、`UberOAuthTokenProvider.java:29,64` | Uber Direct | | yes |

## 6. Redis

| key／channel | 用途 | file:line | 去向 | Will 11-23 刪除清單? |
|---|---|---|---|---|
| `storemap:redirect:<id>`（TTL 10 分，`getAndDelete` 一次性） | 門市地圖中轉表單（#1394 反射 XSS 修復） | `J/service/StoreMapRedirectFormStore.java:27,29,43,54` | | yes |
| pub/sub `cvs:print:<shipTradeNo>`（pattern `cvs:print:*`） | 列印 RPC 同步等 webhook 的跨 pod 通知 | `J/service/CvsPrintWaitingRegistry.java:28,60`、`J/config/CvsPrintRedisConfig.java:30` | | yes |
| `uber_create_delivery:<orderNo>:<leg>` | **不是 Redis**：是送給 Uber 的 create idempotency key（`PRECOMMIT_KEY_PREFIX`）【量】 | `J/provider/uber/UberDirectAdapter.java:92,671` | | yes |

## 7. Provider 抽象（vendor-neutral port 的現況，ADR D3 的起點）

`FulfillmentProviderAdapter`（`J/provider/FulfillmentProviderAdapter.java`，138 行）以 `supportedMethod()` 註冊，每個 capability 回 `Optional`：`create`、`label`、`dispatch`、`cancel`、`track`、`returns`、`pod`、`onboard`、`inbound`、`storeLocator`，另有 `pendingMigration()` 與 `requiresSellerConfirmation()`【量】。capability 介面在 `J/provider/capability/`（12 檔）：`CreateCapability.createFromOrder`＋`requiresOnboarding`；`LabelCapability`（單筆、批次、`BatchConstraints`、三段碼）；`DispatchCapability`；`CancelCapability`（`CancelOutcome(accepted, terminal, message)`）；`TrackCapability.query`；`InboundCapability`（`parseStatusEvent`、`isLabelEvent`、`parseLabelEvent`、`dedupKey`，輸入 `InboundPayload`）；`ReturnCapability`；`PodCapability`；`OnboardCapability`；`StoreLocatorCapability`【量】。

| 實作 | 方法 | file | 去向 | Will 11-23 刪除清單? |
|---|---|---|---|---|
| `PayUniCvsAdapter` | 3002 | `J/provider/payuni/PayUniCvsAdapter.java:74`（492 行；`cancel` 永久 empty，取消走 992） | | yes |
| `PayUniHomeAdapter` | 3001 | `J/provider/payuni/PayUniHomeAdapter.java:84`（546 行） | | yes |
| `UberDirectAdapter` | 3003 | `J/provider/uber/UberDirectAdapter.java:83`（741 行） | | yes |
| 3005 `MERCHANT_DELIVERY` | **沒有 adapter**；`ShippingMethod` 說明是「RFQ 訂單的站外業者自行出貨，平台不安排物流」（`C/utils/code/ShippingMethod.java:18`）；只出現在指標白名單（`ShippingService:164-168`）與 `FulfillmentActionPolicy` 的 map【量】 | | 未列名 |
| `FulfillmentProviderRegistry`、`FulfillmentContext`、`AllowedActionResolver`（312 行）、`FulfillmentActionPolicy`（240 行）、`ActionAvailability` | `allowedActions[]` 計算：以真的 `ShipmentStatusMapper.isValidTransition` 與 adapter 能力回答「這顆按鈕現在能不能按」，附 `disabled_reason_code` | | 未列名 |

另有 `provider/create/*ShipmentStrategy`（Cvs 147、Home 193、Uber 69 行），被 adapter 的 `create` 委派，不是死碼【量】。

## 8. 出貨單狀態機

### 8.1 狀態值（`ShippingStatus` 28 值，`C/utils/code/ShippingStatus.java:15-42`）

- 兩位數（PayUni 原生，19 個，皆在映射表 `J/service/ShipmentStatusMapper.java:35-55`）：91 未處理、92 處理中、21 待出貨、22 物流驗收、31 配送中、32 待取貨、33 異常、11 已取貨（終態）、41 已取消（終態）、43 賠償、44 包裹遺失、46 包裹拋棄、51 一般退貨、52 買家未取、53 廠退、55 賣家未取、56 已轉宅配退回、81 門市關轉、82 待轉宅配退回。
- 三位數（平台自訂，9 個）：210 已叫件、920 已取得出貨便代碼、921 賣家確認出貨、930 已交運（Uber）、933 已指派外送員（Uber）、990 建單失敗、991 建單結果未知（Uber）、992 取消處理中、993 理賠受理中【量】。
- order 側註解（`J-ORD/mq/ShippingEventConsumer.java:258`）提到的 98 不在 `ShippingStatus` 內【量】。

### 8.2 轉移規則（`ShipmentStatusMapper.isValidTransition`，`J/service/ShipmentStatusMapper.java:149-215`，【量】逐行讀）

1. 同狀態不可轉；終態（11、41）不可再轉。
2. 任何非終態都可轉到 992 取消處理中，**除了從 990**。
3. 從 992 出去：任何狀態皆可（全值域唯一允許回退）。
4. 從 990 出去：只能到 91（Admin 重建）或 41（Admin 取消）。
5. 出貨前集合（91、92、21、22、991，`:67`）→ 任何狀態。
6. 本地處理完成集合（210、920、921、933，`:83`）→ 終態、異常、配送中；另允許 920→921。
7. 配送中集合（31、32、930，`:96`）→ 終態、異常、其他配送中，禁止倒退。
8. 異常集合（`ShippingStatusRules.isAbnormal`：33、43、44、46、51、52、53、55、56、81、82、993）→ 異常互轉、終態、回配送中，禁止倒退。
9. 其餘走 `unknownTransition` warn 並拒絕。

寫入一律走 `updateShipmentStatus` 的 `WHERE id=? AND status=?` CAS（`ShipmentMapper:63`），`status_entered_at` 只在狀態真的變動時更新；`provider_*` 欄用 `COALESCE` 不抹掉 provider 說過的話；列印、叫件成功的 SQL 用 `CASE` 擋終態被復活（`ShipmentMapper:96-110,218-242`）。未知 provider 碼：保存 raw、發告警指標、不推進（`persistUnknownProviderStatus`、`mapPayUniStatus` fail-closed）【量】。

### 8.3 誰寫狀態（shipping 內，【量】`rg 'updateShipmentStatus|updateStatus|setStatus' J`）

| 轉移 | 觸發 | file:line |
|---|---|---|
| →91（建單成功） | `order.paid`／backstop 重試 → adapter 建單 | `ShippingService:342`、`provider/create/*` |
| →990（建單失敗殼，獨立交易落地） | PayUni 建單 retry 後仍失敗；退貨建單失敗 | `ShippingService:1842-1900,1919` |
| 91→920 | 列印 webhook 回填三段碼（CAS 擋終態） | `ShippingService:1675-1760`、`ShipmentMapper:96-110` |
| 920→921 | 賣家確認出貨 | `ShippingService:526-600` |
| 91／21→210 | 賣家叫黑貓成功 | `ShippingService:918-1020`、`ShipmentMapper:218-242` |
| 其他（22、31、32、33、11、41、43–56、81、82） | provider webhook（`applyInboundStatus`）或對帳／追蹤查詢（`syncAndTouch`） | `ShippingService:1472,1377` |
| 992、993 | **shipping 內沒有任何程式碼寫入**（只在轉移規則與文件出現）；991、930、933 只有 Uber 路徑寫 | `rg -n 'CANCEL_PENDING\|CLAIM_PENDING' J` 排除 mapper／policy 後無寫入【量】 |

### 8.4 計時與跨服務（逾期未出貨、自動完成、異常）

| 規則 | 擁有者 | 數值與行為 |
|---|---|---|
| 賣家逾期未出貨 | shipping `ProcessingDeadlineScheduler` → aftersales | 超商 72 小時（自然日）、宅配 3 個工作日（用 `holidays`，表是空的，見 D6）；逾期且未取號／未叫件 → `shipment.processing.timeout` → aftersales 系統取消＋退款；3003、3005 不計時【量】 |
| 建單失敗兜底 | order `ShipmentCreationBackstopScheduler` | 付款後 30 分未建單 → 重發 `order.shipment.create_requested`，最多 5 次，仍失敗取消退款【量】 |
| 送達後自動完成 | **order** `AutoCompleteScheduler` | 收到 `shipment.delivered`（11）→ 訂單 DELIVERED(40)＋`auto_complete_at=now+60h`；排程每小時掃，有進行中的售後案件則跳過【量】 |
| 買家確認收貨 | order 發 `order.receipt.confirmed`；shipping 只標 `receipt_confirmed=TRUE`，無讀者（D8） | 【量】 |
| 配送異常 | shipping 偵測 → `shipment.anomaly.detected` → **notification only**（賣家強制通知）；**沒有任何程式把異常轉成售後案件**；order 側把 33／43／44／46／51–56／81／82 映成訂單 SHIPPING_ABNORMAL(35) | 異常清單 UI 是 SQL 現場推導＋ack 表，與事件偵測是兩套判斷（D3、D11）【量】（`aftersales` 消費 shipping 的只有 return.status、label、processing.timeout） |
| 「異常」定義 | 4 份不一致：`ShippingStatusRules.isAbnormal`（12 值）、`ShipmentAnomalyRules.abnormalShipmentStatusValues`（2 值：33、44）、`ShippingAdminQueryService:42-50`（7 值，含 990）、order `ShippingEventConsumer:257`（33／43／44／46／51–56／81／82） | D11【量】 |

## 9. 運費規則模型（B3「保留」的部分）

【量】現況模型，全在 `ShippingFeeService`（`J/service/ShippingFeeService.java`，474 行）：

- **平台統一費率，不是賣家自訂**。`shipping_method_configs`（method × temp）放尺寸、重量上限與天數；`shipping_fee_tiers`（method × temp × zone × 材積級距）放費用。快取鍵 `method:temp:zone`，材積＝長＋寬＋高（cm），依 `max_dimension_cm` 升冪取第一個 `max=0` 或 `dimensionSum <= max` 的級距（`lookupFee`，`:155-168`；排序 `:101`）。
- 配送區 `DeliveryZoneKey`：8101 本島、8102 本島↔離島、8103 離島互寄；7-11 一律 8101（`:241-243`）。
- 溫度欄：`delivery_temp`（`ShippingTemperature` 3201 常溫、3202 冷藏、3203 冷凍）在 configs、tiers 兩張表都是 NOT NULL 並進唯一鍵；seed 的 5＋32 筆全部按溫層分；`CalculateShippingOptions` 回傳依溫層分組；`ValidateProductShipping` 有 `TEMPERATURE_NOT_ONBOARDED`。gateway 目前在賣家端補常溫（`G/handler/shipping_ambient.go`）。
- **免運門檻與誰付運費不在 shipping-service**：在 product-service 的 `product_sku.shipping_method` JSONB `[{type, temperature, freeThreshold, feePayer}]`（`ProductSkuEntity:86-96`；`feePayer` 3101 買家／3102 賣家），免運判定在 order-service `CheckoutValidator.checkFreeShipping`，以 `unit_price × qty − share_discount`（折扣後）對門檻判定，折扣後失去資格要用級距原始運費 `rawFee`（`CheckoutService:700-715`）。
- `commission_rate`／`commission_min`／`settlement_days`：shipping 會回傳，order 的 `ShippingFeeGrpcFetcher:45-60` 只取到 `settlement_days`，**commission 兩欄沒有任何讀者**（`rg -n -i 'commissionRate\|getCommission' order-service/src/main product-service/src/main` 無命中）。
- 與 ADR D2「賣家自訂的固定規則」的差距：shipping-service 內**沒有**賣家層級的運費規則表；現有的是平台級距＋SKU JSON 內的門檻與付款方。賣家自訂規則需要新模型，不是搬舊表。

## 10. 已發現的缺陷（編號，附 file:line）

| # | 缺陷 | 證據 |
|---|---|---|
| D1 | `getShipmentByOrderNo` 用 `selectOne(eq order_no)` 沒有 `is_return=false`；退貨單與正向單同 `order_no`（唯一索引只限 `is_return=FALSE`），退貨單建立後這支會因兩筆而丟 MyBatis 的 too-many-results；建單冪等檢查（`ShippingService:273`）與 gRPC（grpc:431）都走它 | `J/service/ShippingService.java:1264-1268,273`；`R/db/migration/V1__init_shipping_tables.sql:85-86`【量】（行為後果【推】，沒跑） |
| D2 | `publishProcessingTimeout`、`publishProcessingCleared`、`publishLabelUpdated`、`publishShipmentPhotoUpdated` 全部吞例外只 log；`ProcessingDeadlineScheduler` 在 publish 之後無條件 `clearProcessingDeadline`（`:134`），類別 javadoc 宣稱維持 at-least-once；MQ 發布失敗時逾期取消事件永久遺失、無告警 | `J/mq/ShippingEventPublisher.java:72-125,133-150`；`J/scheduler/ProcessingDeadlineScheduler.java:116-134`【量】 |
| D3 | `PACKAGE_LOST`（32 卡 7 天）用 `shipments.updated_at` 當 aging，而 V1 trigger 在任何 UPDATE 都重設 `updated_at`，`claimForReconcile` 每 ≥15 分就 UPDATE 一次（`last_reconciled_at`）→ 排程路徑在對帳窗口內永遠判不到；同一規則的 SQL 版（異常清單）在停止對帳（7 天後）才會成立 → 事件與清單矛盾。V11 自己寫了「禁用 updated_at 當 aging 基準」並建了 `status_entered_at` | `J/scheduler/ShipmentStatusReconcileScheduler.java:160-162`；`J/mapper/ShipmentAnomalyQueryMapper.java:60`；`R/db/migration/V1__init_shipping_tables.sql:115-120`；`J/mapper/ShipmentMapper.java:328`；V11 註解【量】（交互作用【推】） |
| D4 | 同一張異常單每次對帳（≥15 分）都重發 `shipment.anomaly.detected`，`eventId` 每次新 UUID，下游無法以 eventId 去重；對帳窗口 7 天 | `J/scheduler/ShipmentStatusReconcileScheduler.java:139-185`；`J/mq/ShipmentAnomalyEventPublisher.java:77`【量】（重複通知【推】） |
| D5 | `ShipmentAnomalyType` 11 值：4 個 UBER 值在 shipping 內沒有產生者，SQL `CASE` 也不產出 UBER 值；`RETURN_STUCK` 只由 aftersales 發；shipping 只產生其餘 6 種 | `J/mapper/ShipmentAnomalyQueryMapper.java:160`（註解自承）；`rg 'ShipmentAnomalyType\.UBER_' J` 無寫入呼叫【量】 |
| D6 | `holidays` 表 repo 內無任何 INSERT（V1 只建表）；全新 DB 時每天都是工作天，宅配 3 個工作日 deadline 與 `ListBusinessDays` 把週末算進去。B3 計畫文字提到的 4 支匯入 RPC（`ImportHolidays` 等）在 proto 與全 repo 都不存在（`rg -l 'ImportHolidays\|GetHolidaysInternal\|GetHolidayCoverage' backend --glob '!docs/**'` 無結果） | `R/db/migration/V1__init_shipping_tables.sql:405-419`；`J/service/HolidayService.java:46-121`；`J/provider/create/HomeShipmentStrategy.java:160`；計畫 `go-modular-monolith.md:371`【量】 |
| D7 | 逾期未出貨掃描條件只涵蓋 3002 與 3001；3003、3005 永不逾期。賣家自寄要沿用這條規則時 predicate 必須重寫 | `J/mapper/ShipmentMapper.java:445-457`【量】 |
| D8 | `receipt_confirmed` 只有寫入（consumer＋`markReceiptConfirmed`），shipping 內與 proto 都沒有讀者 | `J/mapper/ShipmentMapper.java:28-38`、`J/entity/ShipmentEntity.java:96`；`rg -n -i 'receiptConfirmed\|receipt_confirmed' P J` 只有寫入與欄位定義【量】 |
| D9 | webhook 去重是「先查再寫」且在處理之後才寫，非同一交易；並發重複事件都會通過 `isDuplicate`，第二個 insert 撞唯一鍵走重試。「無 inbound capability」的相容路徑（`:82-92`）完全不去重 | `J/mq/ShippingWebhookConsumer.java:82-92,104,124,138,152-165`【量】（後果【推】） |
| D10 | 狀態寫入與事件發布是雙寫：`applyInboundStatus`（`@Transactional`）在交易內先 publish 再 commit，commit 失敗會留下已發出的事件（反向：`publishShipmentStatusChanged` 失敗會拋例外使整筆回滾，此方向安全）。無 outbox；order 側同類路徑已改成 outbox（`ShippingEventConsumer:408-482`） | `J/service/ShippingService.java:1471-1500,2024-2100`【量】（後果【推】） |
| D11 | 「異常」有 4 份定義互不一致（§8.4） | `C/utils/code/helper/ShippingStatusRules.java:25-39`；`C/utils/code/helper/ShipmentAnomalyRules.java`；`J/service/ShippingAdminQueryService.java:42-50`；`J-ORD/mq/ShippingEventConsumer.java:257`【量】 |
| D12 | 狀態 990 的出口「Admin 重建／Admin 取消」與 `FulfillmentAction` 的 `RESUBMIT`、`CANCEL`、`ARRANGE_RETURN` 等 19 個動作只有「可用性計算」（`AllowedActionResolver`），shipping 的 52 個 RPC 沒有對應的執行端點；992、993 在 shipping 內無寫入者 | `P` 全部 rpc 名；`rg -n 'CANCEL_PENDING\|CLAIM_PENDING\|RESUBMIT' J`（排除 mapper／policy／Onboarding）【量】 |
| D13 | 出貨單號 `generateShipmentNo` 有 4 份複本（`ShippingService:1904` 與 Cvs／Home／Uber strategy），都是 `"S" + TSID % 1e9 補 9 位`，撞號靠唯一索引失敗；V1 欄位註解寫的格式是 `SHP-yyyyMMdd-seq6`，與實作不符 | `J/service/ShippingService.java:1904`、`J/provider/create/CvsShipmentStrategy.java:143`、`HomeShipmentStrategy.java:189`、`UberShipmentStrategy.java:65`；V1 欄位註解【量】 |
| D14 | `OrderReceiptConfirmedConsumer` 任何例外一律 `basicNack(requeue=false)` 直入 DLQ，不重試；同模組另外 3 個走 `MqErrorHandler` 重試的 consumer 行為不同 | `J/mq/OrderReceiptConfirmedConsumer.java:70-75`；`OrderPaidConsumer.java:67`【量】 |
| D15 | `CalculateShippingOptions` 只迴圈 `TAQBIN` 與 `SEVEN_ELEVEN_STORE_PICKUP`（寫死），3003、3005 不會出現在選項；`ValidateProductShipping` 另有自己的判斷路徑 | `J/service/ShippingFeeService.java:211`【量】 |
| D16 | 級距查詢依賴 `ORDER BY max_dimension_cm ASC`，`max_dimension_cm=0`（無上限）排在最前；若同一組鍵下 0 與非 0 並存，0 會吞掉所有較小級距，目前 seed 只有 7-11 單筆 0，靠資料巧合 | `J/service/ShippingFeeService.java:101,163`【推】 |
| D17 | 「超過預計送達日」比較用 `LocalDate.now(UTC)`，`expected_delivery_date` 是台北日（`HolidayService` 與 deadline 計算都用台北） | `J/scheduler/ShipmentStatusReconcileScheduler.java:179`【量】（偏差最多 8 小時） |
| D18 | 追蹤類 RPC（`GetShipmentTracking`、`GetSellerShipmentTracking`、`QueryShipmentStatus`）是讀取，卻會寫入（provider 查詢、更新狀態與 `last_reconciled_at`） | `J/service/ShippingService.java:1301-1376,1377-1425`【量】 |
| D19 | proto 註解與實際不符：`ActionAvailability.action` 寫「27 值」，`FulfillmentAction` 實際 19 值 | `P:426`；`C/utils/code/FulfillmentAction.java:15-33`【量】 |
| D20 | 無 production 呼叫端的 RPC：`QueryShipmentStatus`、`LookupShippingFee`、4 個 Uber 賣家／admin RPC；`GetSellerReturnConfig` 在 aftersales 只有 client wrapper | §1 呼叫端欄【量】 |
| D21 | 驗證訊息與使用者面文字寫死在 Java（繁中、溫層名稱），與 repo 現行「文字由共用文案目錄或模組 `internal/text` 提供」規則不一致 | `J/service/ShippingFeeService.java:300-345`【量】 |
| D22 | `T/` 根目錄 7 支（`Cvs*Test`、`HomeDeliverySettingTest`、`DecryptTest`）不是 JUnit，是帶 `main()` 的手動腳本，連 PayUni sandbox，硬寫 `MER_ID=HAWK9` 與假金鑰；`mvn test` 不會跑，卻佔 test 目錄 | `T/CvsTradeTest.java:12-30` 等；`rg -c '@Test'` 這 7 檔為 0【量】 |
| D23 | `hawkr.shipping.dlx` exchange 宣告了沒有人用 | `J/config/RabbitMqConfig.java:61`【量】 |
| D24 | `createShipmentFromOrder` 整個方法 `@Transactional`（`:264`），內含 gRPC（order）與對 PayUni 的 HTTP 建單；PayUni 建成功後若最後 `insert` 失敗，重試會對 PayUni 再建一張（`ProviderCreateFailedException` 只涵蓋 provider 失敗） | `J/service/ShippingService.java:264-350`【推】 |

## 11. 其他服務裡「與 shipping 的邊界」的接點（B3 不遷，但要同步改）

| 位置 | 內容 | 備註 |
|---|---|---|
| `J-ORD/checkout/ShippingFeeGrpcFetcher.java`、`ShippingFeeCacheClient.java` | 取 `GetShippingFeeConfigs` 做本地運費快取 | 在 Will 刪除清單（order 的 `ShippingFeeGrpcFetcher`） |
| `J-ORD/checkout/CheckoutService.java:996,1256,1282` | 結帳以 `GetSellerShippingConfig` 取賣家郵遞區號推導配送區 | 依賴 `OnboardingService` |
| `J-ORD/checkout/ShippingUberQuoteAssessor.java`、`UberOrderEligibilityGuard.java` | Uber 報價 | Uber 刪除 |
| `J-ORD/scheduler/ShipmentCreationBackstopScheduler.java` | 見 §3.3 | 在清單 |
| product-service `ProductService`、`ProductGrpcService:1395` | `ValidateProductShipping`、`GetSellerShippingConfig`、`CalculateShippingOptions` | B4 catalog 範圍 |
| aftersales `AfterSaleResolutionService:158,168`、`AfterSaleLabelRetryService`、`ProcessingTimeoutConsumer`、`ShipmentLabelConsumer` | 退貨建單、重印、逾期系統取消、標籤事件 | aftersales 範圍 |
| invoice-service（Go）`internal/client/shipping/client.go` | `GetShipmentsByOrderNos`、`GetShipmentByOrderNo` 取狀態 | 舊 Go 服務 |
| gateway `routes_shipping.go`、`handler/shipping.go`、`store_map.go`、`admin_shipping.go`、`shipment_anomaly.go`、`shipping_ambient.go`、`shipping_options.go` | 32 個 RPC 的 REST 入口，加 `CalculateShippingOptions`（經 product） | 與 `client-inventory.md` 對照 |
| payuni-webhook-service | `PayUniWebhookController`（`payuni.shipping.status`／`payuni.shipping.print` routing）、`UberWebhookController`、`WebhookSimulatorController`（呼叫 `LookupShipTradeNo`） | 整個刪除（計畫） |
| `B/internal/legacy/sellerserver/server.go:403`、`legacy/buyerserver/server.go:122` | 為 Java shipping 保留的 `GetSellerShippingProfile`、`ValidateAddress`（9700） | 隨 Java shipping 退場刪 |
| `B/internal/platform/bizcalendar`、`legacy/bizcalendarrelay` | 假日現在的擁有者；relay 只推給 invoice-service，不推 shipping | `holidays` 無寫入入口（D6） |

## 12. Java 測試（shipping-service）

結論【量】：**shipping-service 沒有任何整合測試**（沒有 `@SpringBootTest`、Testcontainers、DB 或 broker；`rg -il 'testcontainers|SpringBootTest|DriverManager' T` 只命中 4 個檔，且全是註解自承「shipping-service 沒有 DB 測試基礎設施（testcontainers 只在 cart／aftersales／coupon）」），`src/test/resources` 不存在。57 檔裡 50 支是 JUnit 單元／契約（原始碼文字、SQL 字串、Mockito 斷言；其中 3 支是測試輔助類），7 支是手動 `main()` 腳本（D22）。「測試對象在刪除清單?」同 §1 的判斷。

| 測試類別（`T/…`） | 測試數 | 涵蓋 | 測試對象在刪除清單? |
|---|---|---|---|
| `service/ShipmentStatusMachineContractTest` | 24 | 狀態機新增邊真的走得通、五條 provider 鏈可達【量】 | no（出貨狀態） |
| `service/ShipmentStatusMapperTest` | 5 | 舊規則沒被改壞 | no |
| `service/CanonicalActionRuleParityTest` | 10 | 「可以做」（allowedActions）與「真的做」（端點）同一套規則（BLOCKER 3） | 未列名 |
| `provider/action/AllowedActionResolverTest` | 13 | `allowedActions[]` 計算，用真 mapper 與真 adapter | 未列名 |
| `service/ShippingFeeServiceValidationTest` | 7 | `validateProductShipping` 各拒絕原因 | no |
| `service/ShippingServiceTest` | 20 | status.changed 事件帶 buyerId（issue #626）等【推：其餘案例未逐一讀】 | 混合 |
| `service/BatchPrintTransactionShapeTest` | 2 | `batchPrintCvsLabel` 不得整段 `@Transactional` | yes |
| `service/BuyerStorePickupServiceTest` | 3 | 常用門市 | yes |
| `service/OnboardingSettingSelectionDeterminismTest` | 12 | 賣家寄件設定選列確定化 | yes |
| `service/StoreMapServiceTest` | 8 | 門市地圖參數 | yes |
| `service/StoreMapRedirectFormStoreTest` | 6 | Redis 中轉表單一次性消費、TTL | yes |
| `service/dto/ShippingEventPayloadTest` | 3 | 事件 payload 形狀 | no（事件） |
| `grpc/StoreMapRedirectFormHandlerTest` | 2 | 門市中轉 RPC | yes |
| `contract/ShippingPayloadGoldenContractTest` | 1 | publisher 實發 key ⊇ notification golden emitted_keys（issue #1529） | no（事件） |
| `contract/UberDeliveryExposureContractTest` | 4 | Uber 憑證外露面 | yes |
| `mq/RabbitListenerTopologyRegistrationTest` | 2 | 每個 `@RabbitListener` queue 登記於 topology.yaml | no（機制保留） |
| `mq/ShippingWebhookConsumerIngressTest` | 5 | webhook 匯流到 canonical ingress（BLOCKER 4），此路徑先前零測試 | 未列名（inbox 保留） |
| `mq/ShipmentAnomalyEventPublisherTest` | 6 | 異常 payload 帶已解析 accountId | yes |
| `mq/ShippingEventPublisherTest` | 1 | publisher | no |
| `mq/UberWebhookConsumerTest`、`mq/UberWebhookCorrelationTest` | 7、7 | Uber webhook 去重與對應 | yes |
| `mapper/PrintWebhookWritePathContractTest` | 5 | 列印回填不得復活終態；寫入路徑守衛 | yes |
| `mapper/ShipmentAnomalyQueryMapperSqlTest`、`mapper/ShipmentAnomalySeverityMappingTest` | 2、4 | 異常 SQL、severity 對 `ShipmentAnomalyType` 窮舉互斥 | yes |
| `mapper/UberDeliveryQueryMapperSqlTest` | 7 | Uber 查詢 SQL（字串斷言） | yes |
| `migration/ShippingMethodCheckContractTest` | 1 | `chk_shipping_method` 值域＝`ShippingMethod`（讀 V15） | 未列名 |
| `migration/UberDirectSchemaContractTest` | 18 | V13 Uber schema（原始碼文字斷言） | yes |
| `provider/FulfillmentCapabilityMigrationTest` | 29 | label／returns／storeLocator 搬進 adapter 後的行為 | 未列名（port 形狀可參考） |
| `provider/FulfillmentCapabilityWiringTest` | 22 | 每處被消滅的 `shipping_method` 字面分支，換上的 capability 做同一件事 | 未列名 |
| `provider/FulfillmentProviderRegistryTest` | 5 | adapter 註冊 | 未列名（port 保留） |
| `provider/payuni/PayUniInboundWireContractTest` | 5 | PayUni 貨態 notify 欄名＝provider 真會送的欄名 | yes |
| `provider/uber/` 9 支：`UberAddressStructureTest` 15、`UberAmountBoundaryContractTest` 8、`UberCapabilityBehaviourTest` 14、`UberCreateFailureMappingTest` 16、`UberInboundWireContractTest` 10、`UberOAuthTokenProviderTest` 5、`UberPackingPlannerTest` 14、`UberQuotePayloadHashTest` 11、`MediaUberPodStorageTest` 5 | 98 | Uber 地址、金額邊界、能力、建單失敗映射、wire、OAuth、裝箱、報價 hash、POD 儲存 | yes |
| 測試輔助（`provider/TestFulfillmentProviders`、`provider/uber/FakeUberTransport`、`provider/uber/TestUberAdapters`） | 0 | — | — |
| `scheduler/ProcessingDeadlineSchedulerTest`、`scheduler/ProcessingDeadlineSchedulerClaimTest` | 4、4 | 逾期事件帶解析後 accountId、批次解析只呼叫一次；租約搶佔 | 未列名（逾期未出貨） |
| `scheduler/ShipmentReconcileClaimTest` | 6 | 貨態與標籤對帳的 lease 閘門（issue #2294 L2、L3） | yes |
| `scheduler/TcatCallStuckAlertSchedulerClaimTest` | 4 | 黑貓卡住告警租約 | yes |
| `config/PayUniConfigProvisioningTest`、`config/UberConfigProvisioningTest` | 6、5 | 設定 provision | yes |
| `util/AddressNormalizerTest` | 20 | 地址正規化（寄件地址）【推：供宅配建單用】 | yes |
| 手動腳本：`CvsPrintLabelTest`、`CvsSettingTest`、`CvsShipMapTest`、`CvsShipmentWebhookTest`、`CvsTradeTest`、`DecryptTest`、`HomeDeliverySettingTest` | 0 | PayUni sandbox 手動呼叫（D22） | yes |

## 13. 開放問題（需要 Will 或後續簡報定）

1. 運費規則的新模型放在哪個模組、誰擁有：現況是平台級距（shipping）＋SKU JSON 的門檻與付款方（product）＋免運判定（order checkout）三處。ADR D2 的「賣家自訂固定規則」在現有程式裡沒有對應；溫層（B4／B6 移除）要從兩張表的唯一鍵拿掉。
2. 「出貨狀態」保留哪些值：28 值裡 PayUni 與 Uber 的多數失去來源；賣家自寄需要的是「未出貨、已出貨（填承運商與追蹤號）、已送達、已取消」加逾期與異常，值域與 `status` 欄是否沿用 `ShippingStatus` 數字碼，是 B3 的第一個設計決定。
3. 「逾期未出貨」的起算點與時限：現行是建單當下超商 72 小時／宅配 3 工作日，條件綁「未取號、未叫件」。自寄情境下起算點（付款後？）、時限、「已處理」的判準（賣家填追蹤號？）都要重定。
4. 「配送異常轉售後」：現行異常只通知賣家，沒有任何轉售後案件的程式（§8.4）。這是新需求，還是沿用 order 的 SHIPPING_ABNORMAL 加人工？
5. 自動完成與買家確認收貨的歸屬：現在由 order 的 `AutoCompleteScheduler`（60 小時）擁有，shipping 的 `receipt_confirmed` 無讀者；B3 的狀態機是否接管，還是留給 order 遷移。
6. 退貨物流（`is_return`）：ADR 沒提。aftersales 現在依賴 `CreateReturn*`、`RetryReturnPrintLabel`、`shipment.return.status_changed`；自寄情境下退貨是否也由買家回填追蹤號？D1 的 `order_no` 重複問題要在新模型避開。
7. 出貨單建立起點：`order.paid` ＋ order backstop（`order.shipment.create_requested`）在 Java 已有；單體內 order 尚未遷移，B3 與 order 的事件契約（誰發、誰收）要先對齊。
8. 保留的 port 範圍：ADR D3 要「建單、取標籤、貨態 webhook 先寫 inbox 並以事件 ID 去重」。現有 port 有 10 個 capability，其中 onboard、storeLocator、returns、pod、dispatch 隨刪除清單消失；inbox 是否沿用 `shipping_webhook_dedup`（先查後寫的 D9 不能照搬）。
9. `ShippingMethod` 3005 `MERCHANT_DELIVERY` 現有語意是「RFQ 站外業者自行出貨」（procurement），與新的賣家自寄是否同一個值。
10. B3 計畫文字有誤：假日段提到 4 支匯入 RPC，proto 與全 repo 都沒有（D6）；`HolidayService` 讀的表沒有寫入者，宅配期限在 Java 內本來就不準。
11. 賣家自寄的承運商清單：自由輸入還是列舉（沒有現成來源；`ShippingMethod` 只有 PayUni／Uber 的承運商）。
