# B4-0 疑似既有缺陷：Java product-service 及其呼叫端（唯讀，快照 dev `496b048e2`）

只列「讀程式時看到的疑似缺陷」與證據，**不寫處置**（去向留給 B4 決策）。標記：【量】＝逐行讀過 source 或跑過 `rg`；【推】＝由讀到的程式推論、沒有實際執行、沒有 test 佐證。嚴重度是依資料或金額影響的初判，不是決定。路徑縮寫同 interface-inventory.md（`J`、`R`、`P`、`G`、`MQ`）。
所有指令從 `/Users/will/second-brain/.projects/personal/hawkr` 執行。本檔沒有跑任何服務、測試或 API。測試覆蓋現況見 integration-tests.md §2：`DeductStock`／`RestoreStock` 的冪等、`OrderCancelledConsumer`、`FavoriteCountService`、`ReviewStatsService` 都沒有直接測試。

已排除（查過不是缺陷）：`requireAccountType(SELLER)` 不會擋 SELLER_AND_BUYER 帳號，`GrpcAuthContext.requireAccountType` 依「SELLER_AND_BUYER 涵蓋 BUYER 與 SELLER」規則放行（`backend/services/java/shared/common-core/src/main/java/io/github/will413028/hawkr/common/grpc/GrpcAuthContext.java:184-195`）【量】。

## 並發與冪等

CD1. **扣庫存的冪等檢查與記錄不是原子，同一 `stock_tx_id` 併發重送會扣兩次**（嚴重度：高，庫存少算、可能誤判售完）【量】
   - 證據：`J/service/StockService.java:111` 先 `existsByStockTxIdAndOperation(stockTxId, DEDUCT)`，沒有就逐 SKU 扣減（`:122-130`），最後才 `insertIfAbsent`（`:138`，`J/repository/StockTransactionRepository.java:26`）。兩個交易在 READ COMMITTED 下都看不到對方未 commit 的紀錄，都會扣減；SKU 的列鎖只讓兩次扣減排隊，不會擋第二次。第二個的 `insertIfAbsent` 遇衝突回 0，結果被忽略，仍回 SUCCESS。`restoreStock`（`:156`、`:189`）同形。
   - 觸發條件（推）：order-service 在 deadline 逾時後重試、或 `StockCompensationWorker` 與 `OrderCancelledConsumer` 同時對同一筆歸還（`OrderService.java:419`、`StockCompensationWorker.java:146/206`、`J/mq/OrderCancelledConsumer.java` 都呼叫 restore）。歸還重複會把庫存灌高。
   - 沒有測試：`rg -l 'deductStock|restoreStock' backend/services/java/product-service/src/test` 只命中快取清除與 stock_qty 回寫的 3 個單元測試，沒有併發或冪等測試【量】。

CD2. **歸還庫存用 caller 帶來的品項與數量，不用 DEDUCT 時記下的 payload**（嚴重度：中）【量】
   - 證據：`StockService.restoreStock` 只用 DEDUCT 紀錄當「是否扣過」的閘門（`:163`），實際歸還量來自參數 `items`（`:168-181`）；`stock_transaction.payload`（V1:209）有扣減時的 `[{sku_id, quantity}]` 但沒被讀。`OrderCancelledConsumer` 的 items 來自事件 JSON（`:54-59`）。
   - 影響（推）：呼叫端的品項或數量與扣減時不一致（例如部分取消、事件組錯），歸還量就跟著錯，且 RESTORE 一旦記錄就無法再修正。歸還時 SKU 不存在只記 warn 後繼續（`:174-179`），仍寫入 RESTORE 紀錄。

CD3. **收藏數是無集合的 ±1 計數器，事件順序顛倒或下限截斷後會永久漂移**（嚴重度：低）【推】
   - 證據：`J/service/FavoriteCountService.java` 以 `event_id` 去重後直接 `favorite_count + delta`；`-1` 的 SQL 帶 `WHERE favorite_count > 0`（`J/repository/ProductRepository.java:448-454`），計數為 0 時減法變 NOOP，但去重列已寫入。「先 removed 後 added」的亂序（兩條獨立 queue：`RabbitMqConfig.java:115-158`）會讓 removed 被吞掉、added 生效，最後多 1。沒有任何對帳把它校正回 buyer 端的真值。
   - 沒有測試：`rg -l FavoriteCountService backend/services/java/product-service/src/test` 無結果【量】。

CD4. **評分快照覆寫沒有版本比較，`review.created` 與 `review.hidden` 亂序會留下舊值**（嚴重度：低）【推】
   - 證據：`J/service/ReviewStatsService.java` 直接以事件帶的平均分與則數 `updateReviewStats`，沒有 `version` 或時間戳條件。兩個 routing key 共用一個 queue（`RabbitMqConfig.java:249-277`），單一 consumer 時大致有序，但重送（DLQ 回灌、`MqErrorHandler` 重試）會讓較舊的快照晚到並覆蓋新值。B4-0 查證：payload 沒有序號或版本欄，只有 `event_id`、`timestamp`（`backend/internal/legacy/reviewrelay/relay.go:136-178`）；聚合值在 relay job 執行時重讀（`:196-204`），不是事件發生時的快照，所以亂序只發生在兩個 job 重讀後交付順序顛倒時【量讀碼】。D3 改成 review 同步呼叫 `SetRating` 傳絕對值，不再依賴事件順序。

CD5. **`sales_count_event_dedup` 與 `stock_transaction` 沒有清理，持續成長**（嚴重度：低）【量】
   - 證據：`rg -n '@Scheduled' backend/services/java/product-service/src/main` 只有 `IdempotencyCleanupJob`、`FavoriteEventDedupCleanupJob`；兩表（V11:10、V1:209）無其他刪除路徑（`rg -n 'DELETE FROM (sales_count_event_dedup|stock_transaction)' backend/services/java/product-service` 本次未跑，屬【推】）。

## 正確性

CD6. **運費試算的寄件地區永遠從空字串郵遞區號解析，取到的賣家資料被丟掉**（嚴重度：中，屬裁切項的鄰近程式）【量】
   - 證據：`J/grpc/ProductGrpcService.java` `calculateShippingOptions`：先 `sellerGrpcClient.getSellerMeta(storeId)`（`:1377`），接著 `String zipCode = (sellerMeta != null) ? "" : "";`，兩個分支都是空字串，再 `ShippingAreaResolver.resolve(zipCode)`；收件地區寫死 `MAIN_ISLAND`。所以每個賣家的路線（`ShippingRouteRules.resolve`）都相同，離島賣家的運費試算錯，且每次多一個無用的 seller RPC。
   - 呼叫端：`G/handler/shipping_options.go`（賣家編輯商品時的運費預覽）【量】。

CD7. **認證裁決事件（approved／rejected／revoked）沒有任何消費者**（嚴重度：低；整條屬裁切）【量】
   - 證據：發布點 `J/service/ProductCertificationApplicationService.java:1487-1489` → `J/mq/ProductEventPublisher.java:64-76`。消費端搜尋：`rg -l 'product.certification' --glob '*.go' backend` 無結果；Java 端只有 product-service 自己的 `verify.requested` 綁定（`RabbitMqConfig.java:304-318`）。notification-service 有 `product_certification_*` 模板（`backend/services/notification-service/db/migrations/20260826120000_product_certification_templates.sql`），但沒有 worker 讀這些事件（`rg -il certification backend/services/notification-service --glob '*.go'` 只命中 `template_contract_test.go`）。賣家收不到裁決通知。

CD8. **order-service 的 `ProductGrpcClient` 有兩個沒有呼叫端的方法**（嚴重度：無，死碼）【量】
   - 證據：`backend/services/java/order-service/src/main/java/io/github/will413028/hawkr/order/grpc/client/ProductGrpcClient.java:114`（`getStock`）、`:133`（`getPublicProduct`）；`rg -n 'productGrpcClient\.(getStock|getPublicProduct)' backend/services/java/order-service/src/main` 無結果。

CD9. **`GetProduct` 只靠 request 的 `store_id` 判斷擁有者，handler 內沒有角色或身分檢查**（嚴重度：中，視 gateway 是否一律覆寫 `store_id`）【推】
   - 證據：`J/grpc/ProductGrpcService.java:340-` 讀 `request.getStoreId()`（`:342`），授權掃描沒有命中任何 `GrpcAuthContext` 呼叫（interface-inventory §1 的 awk 結果）【量】。gateway（B4-0 查證）：`seller_product.go:726` 與 `seller_sku.go:103` 以 `resolveStoreID` 從登入身分推導；但 `seller_sku.go:58-63` 與 `product.go:724-731` 在賣家角色下呼叫 `CurrentStoreID`，**失敗時保留 account ID 當 `store_id` 傳給 `GetProduct`**，帳號 ID 與店家 ID 撞值時可讀到他店未上架商品【量讀碼，撞值後果為推】。D4 改由呼叫者推導店家、不收 `store_id`，一併消除。ai-service 帶的值未讀（D12 隨 A9 刪除）。同類的還有 `GetProductList` 的賣家視角（`ProductServiceListGuardTest` 有 3 個守門測試，未讀斷言）。

CD10. **`product_sku.id` 用 BIGSERIAL、`product.id` 與 `hashtag.id` 用應用層 TSID，同一聚合兩種 ID 方案**（嚴重度：無，設計觀察）【量】
   - 證據：V1:148（`id BIGSERIAL`）對 V1:54（`id BIGINT PRIMARY KEY -- TSID`）。SKU id 外流到 cart、order（`stock_transaction.payload`、`BatchGetSkuCartInfo`），可被枚舉。B4 計畫已要求商品 ID 換 UUID（content banner 一項），SKU 是否一起換未寫。
