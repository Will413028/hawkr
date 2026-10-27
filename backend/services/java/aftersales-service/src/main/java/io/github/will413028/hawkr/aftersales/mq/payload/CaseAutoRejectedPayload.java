package io.github.will413028.hawkr.aftersales.mq.payload;

import com.fasterxml.jackson.databind.PropertyNamingStrategies;
import com.fasterxml.jackson.databind.annotation.JsonNaming;

import java.util.UUID;

/**
 * aftersales.case.auto_rejected MQ 事件 Payload。
 * §5.10 AUTO_REJECT（預設）：售後單因賣家處理逾時系統代為拒絕。
 *
 * <p>{@code notifyBuyerAccountId}：見 {@link CaseTransitionPayload} 說明，buyer_id 8-service
 * rekey Phase 7 新增。
 */
@JsonNaming(PropertyNamingStrategies.SnakeCaseStrategy.class)
public record CaseAutoRejectedPayload(
        String eventId,
        String caseNo,
        String orderNo,
        String buyerId,
        String notifyBuyerAccountId,
        String sellerAccountId,
        String originalDeadlineHours,
        String actor,
        String reason,
        String occurredAt
) {
    public CaseAutoRejectedPayload(
            String caseNo,
            String orderNo,
            String buyerId,
            String notifyBuyerAccountId,
            String sellerAccountId,
            String originalDeadlineHours,
            String actor,
            String reason,
            String occurredAt
    ) {
        this(UUID.randomUUID().toString(), caseNo, orderNo, buyerId, notifyBuyerAccountId,
                sellerAccountId, originalDeadlineHours, actor, reason, occurredAt);
    }
}
