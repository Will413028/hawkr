package io.github.will413028.hawkr.aftersales.mq.payload;

import com.fasterxml.jackson.databind.PropertyNamingStrategies;
import com.fasterxml.jackson.databind.annotation.JsonNaming;

import java.util.UUID;

/**
 * aftersales.refund.completed MQ 事件 Payload。
 * 退款完成事件，由 RefundResultConsumer / RefundTaskWorker / RefundSyncWorker / AtmRefundReconciler
 * 在退款成功後呼叫。
 *
 * <p>{@code refundOutcome}: FULL_REFUND / PARTIAL_REFUND，對應
 * {@link io.github.will413028.hawkr.common.utils.code.AfterSalePlatformDecisionOutcome} value（決定 order status 走 70 還是 60）。
 * 必填；publisher 與 consumer 皆 fail-loud（null/未知值拋例外，consumer 進 DLQ）。
 * <p>{@code refundAmount}: 元為單位（snake_case wire 為 long 序列化為 string）。
 * <p>{@code actor}: 固定 {@code "system"}——這個事件永遠是 payment webhook/worker 觸發，無人工路徑。
 * <p>{@code sellerAccountId}：store_id 解析出的登入 account_id。
 * <p>{@code buyerId}：買家業務值（訂單 buyer_id），notification-service 的 inbox/device_tokens
 * 讀寫路徑皆以此為鍵（buyer read-write id-space mismatch fix，2026-08-06）。
 * notification_templates 對此 event type 目前仍指向 {@code notifyBuyerAccountId}；待
 * PR 2（Task 6，本 PR 合併並確認 buyer_id 已上線後才會執行）完成後才會改指向 buyer_id。
 * <p>{@code notifyBuyerAccountId}：buyer_id 反解出的登入 account_id，供 notification-service
 * 推播路由用。{@code actor}、{@code sellerAccountId}、{@code notifyBuyerAccountId} 三個身份
 * 欄位是 issue #1626 新增——這個事件從建立起就沒有身份欄位，是 notification-service 持續
 * "cannot extract user_id" 的根因（buyer_id 8-service rekey Phase 7
 * `20260718180000_buyer_id_phase7_aftersales_notify_field.sql` 當時明確排除、這次補上）。
 */
@JsonNaming(PropertyNamingStrategies.SnakeCaseStrategy.class)
public record RefundCompletedPayload(
        String eventId,
        String caseNo,
        String orderNo,
        String refundOutcome,
        String refundAmount,
        String paymentMethod,
        String externalRefId,
        String occurredAt,
        String actor,
        String sellerAccountId,
        String storeId,
        String buyerId,
        String notifyBuyerAccountId
) {
    public RefundCompletedPayload(
            String caseNo,
            String orderNo,
            String refundOutcome,
            String refundAmount,
            String paymentMethod,
            String externalRefId,
            String occurredAt,
            String actor,
            String sellerAccountId,
            String storeId,
            String buyerId,
            String notifyBuyerAccountId
    ) {
        this(UUID.randomUUID().toString(), caseNo, orderNo, refundOutcome,
                refundAmount, paymentMethod, externalRefId, occurredAt,
                actor, sellerAccountId, storeId, buyerId, notifyBuyerAccountId);
    }
}
