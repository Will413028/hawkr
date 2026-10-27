package io.github.will413028.hawkr.payment.application.dto;

/**
 * 建立退款請求 DTO
 *
 * @param tradeNo        原交易編號
 * @param amount         退款金額（新台幣元）
 * @param reason         退款原因
 * @param idempotencyKey 冪等鍵（AfterSales 傳入 case_no 或 cancel_id），可為 null
 */
public record CreateRefundRequest(
        String tradeNo,
        long amount,
        String reason,
        String idempotencyKey
) {
    /**
     * 向下相容建構子（無冪等鍵）
     */
    public CreateRefundRequest(String tradeNo, long amount, String reason) {
        this(tradeNo, amount, reason, null);
    }

    /**
     * 是否有提供冪等鍵
     */
    public boolean hasIdempotencyKey() {
        return idempotencyKey != null && !idempotencyKey.isEmpty();
    }
}
