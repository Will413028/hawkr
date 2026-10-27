package io.github.will413028.hawkr.aftersales.mq.payload;

import com.fasterxml.jackson.databind.PropertyNamingStrategies;
import com.fasterxml.jackson.databind.annotation.JsonNaming;

import java.util.UUID;

/**
 * aftersales.appeal.submitted MQ 事件 Payload。
 * 買家申訴提交，通知 seller + admin channel。
 *
 * <p>{@code notifyBuyerAccountId}：見 {@link CaseTransitionPayload} 說明，buyer_id 8-service
 * rekey Phase 7 新增。
 *
 * <p>{@code sellerAccountId}：store_id 解析出的賣家登入 account_id，供 notification-service
 * 推播路由用——鏡像 {@code CaseTransitionPayload}/{@code CaseAutoRejectedPayload}/
 * {@code CaseAutoEscalatedPayload}/{@code RefundCompletedPayload} 同一欄位的命名慣例。
 * 原欄位名 {@code sellerId} 容易誤讀為未解析的 store_id，issue #1626 一併正名。
 */
@JsonNaming(PropertyNamingStrategies.SnakeCaseStrategy.class)
public record AppealSubmittedPayload(
        String eventId,
        String caseNo,
        String orderNo,
        String buyerId,
        String notifyBuyerAccountId,
        String sellerAccountId,
        String storeId,
        String reason,
        String occurredAt
) {
    public AppealSubmittedPayload(
            String caseNo,
            String orderNo,
            String buyerId,
            String notifyBuyerAccountId,
            String sellerAccountId,
            String storeId,
            String reason,
            String occurredAt
    ) {
        this(UUID.randomUUID().toString(), caseNo, orderNo, buyerId, notifyBuyerAccountId,
                sellerAccountId, storeId, reason, occurredAt);
    }
}
