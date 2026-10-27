package io.github.will413028.hawkr.aftersales.mq.payload;

import com.fasterxml.jackson.databind.PropertyNamingStrategies;
import com.fasterxml.jackson.databind.annotation.JsonNaming;

import java.util.UUID;

/**
 * aftersales.case.auto_escalated MQ 事件 Payload。
 * §5.10 AUTO_ESCALATE（備援）：售後單因賣家處理逾時自動升級至 admin 仲裁。
 *
 * <p>{@code notifyBuyerAccountId}：見 {@link CaseTransitionPayload} 說明，buyer_id 8-service
 * rekey Phase 7 新增。
 */
@JsonNaming(PropertyNamingStrategies.SnakeCaseStrategy.class)
public record CaseAutoEscalatedPayload(
        String eventId,
        String caseNo,
        String orderNo,
        String buyerId,
        String notifyBuyerAccountId,
        String sellerAccountId,
        String storeId,
        String originalDeadlineHours,
        String actor,
        String reason,
        String occurredAt
) {
    public CaseAutoEscalatedPayload(
            String caseNo,
            String orderNo,
            String buyerId,
            String notifyBuyerAccountId,
            String sellerAccountId,
            String storeId,
            String originalDeadlineHours,
            String actor,
            String reason,
            String occurredAt
    ) {
        this(UUID.randomUUID().toString(), caseNo, orderNo, buyerId, notifyBuyerAccountId,
                sellerAccountId, storeId, originalDeadlineHours, actor, reason, occurredAt);
    }
}
