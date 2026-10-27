package io.github.will413028.hawkr.aftersales.mq.payload;

import com.fasterxml.jackson.databind.PropertyNamingStrategies;
import com.fasterxml.jackson.databind.annotation.JsonNaming;

import java.util.UUID;

/**
 * aftersales.awaiting.buyer.refund.page MQ 事件 Payload。
 * ATM 退款建立後通知買家前往 PayUni 操作頁面。共用於兩條流程：售後案件退款
 * （{@code caseNo} 有值）與賣家出貨前取消退款（{@code caseNo} 恆為 null，走 cancelId）。
 *
 * <p>{@code eventId} 由 publisher 端自動產生 UUID v4，供 consumer 端做冪等保護
 * （36 §7.4 Idempotency Conformance Contract）。
 * <p>{@code buyerId}：買家業務值（訂單 buyer_id），notification-service 的
 * inbox/device_tokens 讀寫路徑皆以此為鍵（buyer/read-write id-space mismatch fix，
 * 2026-08-06）。
 * <p>{@code notifyBuyerAccountId}：buyer_id 反解出的登入 account_id（issue #1626 遺留
 * 欄位）。notification_templates 對這兩個 event type 目前仍指向此欄位；待 PR 2
 * （Task 6，本 PR 合併並確認 buyer_id 已上線後才會執行）完成後才會改指向 buyer_id。
 */
@JsonNaming(PropertyNamingStrategies.SnakeCaseStrategy.class)
public record AwaitingBuyerRefundPagePayload(
        String eventId,
        String orderNo,
        String caseNo,
        String refundTaskId,
        String occurredAt,
        String buyerId,
        String notifyBuyerAccountId
) {
    public AwaitingBuyerRefundPagePayload(String orderNo, String caseNo, String refundTaskId,
                                           String occurredAt, String buyerId, String notifyBuyerAccountId) {
        this(UUID.randomUUID().toString(), orderNo, caseNo, refundTaskId, occurredAt, buyerId, notifyBuyerAccountId);
    }
}
