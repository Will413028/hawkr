package io.github.will413028.hawkr.payuni.webhook.event;

/**
 * 付款結果通知事件（UOP）
 *
 * @param merId        商家編號
 * @param merTradeNo   商店訂單編號
 * @param tradeNo      PayUni 交易編號
 * @param tradeStatus  交易狀態（0=處理中, 1=成功, 2=失敗）
 * @param tradeAmt     交易金額
 * @param payType      付款方式（Credit, ATM, ApplePay, GooglePay 等）
 * @param message      回應訊息
 * @param tradeDate    交易時間
 */
public record PaymentUopEvent(
        String merId,
        String merTradeNo,
        String tradeNo,
        String tradeStatus,
        String tradeAmt,
        String payType,
        String message,
        String tradeDate
) implements PayUniWebhookEvent {
}
