package io.github.will413028.hawkr.order.mq.dto;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.databind.PropertyNamingStrategies;
import com.fasterxml.jackson.databind.annotation.JsonNaming;

import java.util.List;
import java.util.UUID;

/**
 * order.cancelled MQ 事件 Payload
 * 訂單取消後發布，供 Product Service 非同步歸還庫存與 Notification Service 推播。
 *
 * <p>{@code eventId} 由 publisher 端自動產生 UUID v4，供 consumer 端做冪等保護
 * （36 §7.4 Idempotency Conformance Contract）。
 *
 * <p>{@code buyerId} / {@code notifySellerAccountId} 為 notification-service
 * 路由買賣雙方的對象 id；{@code sellerName} / {@code sellerAvatarKey} 為買家推播
 * 文案上下文（design.md D3）。{@code notifySellerAccountId} 是**登入 account_id**（非 store_id），
 * 由 {@code storeId} 反解析而得，供 {@code order.cancelled.merchant} 模板
 * {@code target_user_field} 路由用。舊名 {@code sellerAccountId} 已於 Phase C（2026-08-06）移除
 * ——2026-08-05 Task 4 起雙寫、notification-service 模板同批切讀新名，Phase C 移除前已用「至少隔
 * 一次 release」+ 全 repo consumer 掃描確認無殘留讀取（詳見 PR 描述）。
 *
 * <p><b>{@code storeId}</b>：店家代理鍵原始值（{@code orders.store_id}），落 outbox 時
 * 由 {@code OrderService.insertCancelledOutbox} 寫入、派送時 enrich 讀它做賣家 meta 查詢
 * 與 principal 反解析。2026-08-05 之前這個值曾暫存在已移除的 {@code sellerAccountId} 欄位裡，
 * 造成同一個欄位在「outbox 階段 = store_id / wire 階段 = account_id」的雙義——本欄位就是為了拆掉
 * 那個雙義而加的（形狀對齊 OrderPaidPayload / OrderCompletedPayload，兩者本來就同時帶 store_id
 * 與解析後的 notify 欄位）。
 *
 * <p><b>{@code notifyBuyerAccountId}</b>：由 {@code buyerId}（買家業務值 buyer_id）
 * 經 BuyerGrpcClient.resolvePrincipals 反解析而得的登入 account_id，僅供
 * notification-service 的 {@code order.cancelled.buyer} 模板路由使用
 * （{@code target_user_field} 指向此欄位；buyer_id 8-service rekey Phase 1）。解析失敗時帶 0
 * （notification-service 端 skip），不阻斷本方法必須繼續往下發的 order.cancelled
 * （product-service 靠它做庫存歸還，不可因此中斷）。
 *
 * <p>{@code sellerAvatarKey} 為 objectKey（49 §6.3/S3：MQ payload 只帶 key、名 *Key；
 * 消費端 notification worker 組 URL，S5）。賣家未設頭像或取資料失敗則為 null。
 * （seller 批次改回 key 前值仍為 URL，worker BuildURL 對 http passthrough）
 *
 * <p>{@code cancelReason}（{@code OrderCancelReason.value()}，可為 null——買家自願取消未付款訂單
 * 不寫此欄位）：供 notification-service DB-driven 模板引擎以 {@code payload_filter} 篩出
 * 賣家停權專屬文案（12 §7.10 `order.cancelled.seller_suspended`），一般取消沿用既有文案。
 * 通用模板引擎（{@code matchesFilter}）直接讀 payload 任意 key，Go 端零改動。
 *
 * <p><b>{@code @JsonIgnoreProperties(ignoreUnknown = true)}（勿移除）</b>：本 record 是
 * {@code order_outbox.payload} 的反序列化目標，而 order_outbox 沒有清理機制——每次改動本 record
 * 的欄位，部署當下都必然有「用另一版形狀寫成、還沒派送」的 PENDING/FAILED row。本專案的
 * ObjectMapper 全是裸 {@code new ObjectMapper()}（全 repo 無任何 {@code FAIL_ON_UNKNOWN_PROPERTIES}
 * 或 {@code @JsonIgnoreProperties} 設定），預設嚴格：少了這個註解，只要 row 帶了本版沒宣告的 key，
 * {@code OrderOutboxPublisher.buildBody} 的 {@code readValue} 就丟
 * {@code UnrecognizedPropertyException}。該例外**不在 {@code enrichCancelled} 的 try/catch 內**，
 * 會一路傳到 {@code publish()} 外層 catch → {@code markFailed} → 指數退避 → 10 次後標 DEAD，
 * 那筆取消的庫存就永遠不會歸還（product-service 靠 order.cancelled 歸還）。這個保護正是本次
 * 移除 {@code sellerAccountId} 欄位能夠安全進行的原因——舊 row 裡殘留的 {@code seller_account_id}
 * key 會被此註解直接忽略，不會拋例外（見 {@code OrderOutboxPublisherCancelledEnrichTest}）。
 *
 * <p>作用方向要講清楚：它讓**本版之後**任何新增欄位的版本都能安全 rollback 回本版。它**不能**
 * 讓比本版更舊的版本讀懂本版新增的 {@code store_id}——那需要舊版自己也有這個註解。因此這個註解
 * 只增不減；移除它等於把未來每次欄位改動的 rollback 都變成單向門。
 */
@JsonIgnoreProperties(ignoreUnknown = true)
@JsonNaming(PropertyNamingStrategies.SnakeCaseStrategy.class)
public record OrderCancelledPayload(
        String eventId,
        String orderNo,
        String stockTxId,
        List<StockItemPayload> items,
        String occurredAt,
        Long buyerId,
        Long notifyBuyerAccountId,
        Long storeId,
        Long notifySellerAccountId,
        String sellerName,
        String sellerAvatarKey,
        String cancelReason
) {
    public OrderCancelledPayload(
            String orderNo,
            String stockTxId,
            List<StockItemPayload> items,
            String occurredAt,
            Long buyerId,
            Long notifyBuyerAccountId,
            Long storeId,
            Long notifySellerAccountId,
            String sellerName,
            String sellerAvatarKey,
            String cancelReason
    ) {
        this(UUID.randomUUID().toString(), orderNo, stockTxId, items, occurredAt,
                buyerId, notifyBuyerAccountId, storeId, notifySellerAccountId, sellerName,
                sellerAvatarKey, cancelReason);
    }

    @JsonNaming(PropertyNamingStrategies.SnakeCaseStrategy.class)
    public record StockItemPayload(String skuId, String quantity) {}
}
