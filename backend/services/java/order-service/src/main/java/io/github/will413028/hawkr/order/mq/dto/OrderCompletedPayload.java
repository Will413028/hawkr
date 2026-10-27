package io.github.will413028.hawkr.order.mq.dto;

import com.fasterxml.jackson.databind.PropertyNamingStrategies;
import com.fasterxml.jackson.databind.annotation.JsonNaming;

import java.util.UUID;

/**
 * order.completed MQ 事件 Payload
 * 訂單完成後發布，供下游（撥款/行銷等）消費。
 *
 * <p>{@code eventId} 由 publisher 端自動產生 UUID v4
 * （36 §7.4 Idempotency Conformance Contract）。
 *
 * <p><b>{@code storeId} / {@code store_id}</b>：店家的 {@code store_id}
 * surrogate（原始值，非登入 account_id）。crm-service 的 {@code RecordOrder} 統計與本分支新增
 * 的 {@code GetOrderAggregates} reconcile RPC 皆以此欄位 key 統計資料，必須維持 store_id，
 * 不可帶入解析後的登入 account_id（否則會與 store_id-keyed 的儲存/查詢路徑不一致）。
 *
 * <p><b>{@code notifySellerAccountId} / {@code notify_seller_account_id}</b>：由
 * store_id 解析而來的登入 account_id，僅供 notification-service 的
 * {@code order.completed.merchant} 模板路由使用（{@code target_user_field} 指向此欄位）。
 *
 * <p><b>{@code buyerId} / {@code buyer_id}</b>：買家的 {@code buyer_id}
 * 業務值（原始值，非登入 account_id）。
 *
 * <p><b>{@code notifyBuyerAccountId} / {@code notify_buyer_account_id}</b>：由
 * buyer_id 解析而來的登入 account_id，僅供 notification-service 的
 * {@code order.completed.buyer} 模板路由使用（{@code target_user_field} 指向此欄位；
 * buyer_id 8-service rekey Phase 1，鏡像上方 notifySellerAccountId）。
 */
@JsonNaming(PropertyNamingStrategies.SnakeCaseStrategy.class)
public record OrderCompletedPayload(
        String eventId,
        String orderNo,
        String buyerId,
        String notifyBuyerAccountId,
        String storeId,
        String notifySellerAccountId,
        String totalActualAmount,
        String confirmType,
        String completedAt
) {
    public OrderCompletedPayload(
            String orderNo,
            String buyerId,
            String notifyBuyerAccountId,
            String storeId,
            String notifySellerAccountId,
            String totalActualAmount,
            String confirmType,
            String completedAt
    ) {
        this(UUID.randomUUID().toString(), orderNo, buyerId, notifyBuyerAccountId,
                storeId, notifySellerAccountId, totalActualAmount, confirmType, completedAt);
    }
}
