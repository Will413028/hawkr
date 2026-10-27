package io.github.will413028.hawkr.aftersales.mq.payload;

import com.fasterxml.jackson.annotation.JsonAnyGetter;
import com.fasterxml.jackson.databind.PropertyNamingStrategies;
import com.fasterxml.jackson.databind.annotation.JsonNaming;

import java.util.Map;
import java.util.UUID;

/**
 * aftersales.case.* MQ 事件 Payload（11 種主流程 transition 共用同一形狀）。
 *
 * <p>Standard fields 由 publisher 統一填入；{@code extra} 為 caller 提供的額外欄位，
 * 例如 {@code shipping_method}（3001 宅配 / 3002 超商）、{@code deadline_at}（ISO 8601 倒數時間）。
 *
 * <p>⚠️ {@code extra} 的 key 必須由 caller 以 **snake_case** 提供 —— @JsonNaming 僅作用於
 * record component getter，不會替 Map 的 key 重命名。Jackson 對 {@link JsonAnyGetter} 標記的
 * Map 直接展開為頂層 JSON key。
 *
 * <p>{@code buyerId}：買家業務值（stint-scoped buyer_id）。{@code notifyBuyerAccountId}：
 * 供 notification-service 推播路由用的登入 account_id（buyer_id 反解而來，見
 * {@code AfterSalesEventPublisherImpl#resolveNotifyBuyerAccountId}）——buyer_id 8-service
 * rekey Phase 7，鏡像 order-service Phase 1 的 {@code notify_buyer_account_id} 手法。
 *
 * <p>對應 callsite 與 routing_key 對照見 {@code AfterSalesEventPublisher#publishCaseTransition}.
 */
@JsonNaming(PropertyNamingStrategies.SnakeCaseStrategy.class)
public record CaseTransitionPayload(
        String eventId,
        String caseNo,
        String orderNo,
        String buyerId,
        String notifyBuyerAccountId,
        String sellerAccountId,
        String storeId,
        String actor,
        String occurredAt,
        @JsonAnyGetter Map<String, Object> extra
) {
    public CaseTransitionPayload(
            String caseNo,
            String orderNo,
            String buyerId,
            String notifyBuyerAccountId,
            String sellerAccountId,
            String storeId,
            String actor,
            String occurredAt,
            Map<String, Object> extra
    ) {
        this(UUID.randomUUID().toString(), caseNo, orderNo, buyerId, notifyBuyerAccountId,
                sellerAccountId, storeId, actor, occurredAt, extra);
    }
}
