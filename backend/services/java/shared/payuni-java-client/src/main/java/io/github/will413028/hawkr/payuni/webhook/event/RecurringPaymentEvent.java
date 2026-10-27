package io.github.will413028.hawkr.payuni.webhook.event;

/**
 * PayUni 續期收款結果通知（page_id=306）。
 *
 * <p>這個事件只保存 provider 欄位的型別化快照；金額、狀態與週期邊界的業務判斷由
 * payment-service 的 provider adapter 負責，避免 webhook SDK 反向依賴 membership policy。</p>
 */
public record RecurringPaymentEvent(
        String merId,
        String periodTradeNo,
        String periodOrderNo,
        String thisPeriod,
        String tradeNo,
        String tradeStatus,
        String tradeAmt,
        String message
) implements PayUniWebhookEvent {
}
