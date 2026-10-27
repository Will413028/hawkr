package io.github.will413028.hawkr.payuni.webhook.event;

/**
 * 退款結果通知事件（ATM async webhook）
 *
 * PayUni 於 ATM 退款頁面版後，銀行實際到帳或退件時送回此 webhook。
 * 信用卡 / Apple Pay / Google Pay 退款為同步回應（不走此 webhook）。
 *
 * 參考 PayUni 退款 API 文件：
 * - Status（外層）：通知送達成功與否，業務上不使用
 * - RefundStatus（內層）：1=成功到帳 / 2=退件失敗
 * - BankDate：實際到帳日（YYYY-MM-DD）；Message：失敗原因描述
 *
 * @param merId        商家編號
 * @param merTradeNo   商店退款編號（對應 payment_refunds.refund_no 或由 payment service 解析映射）
 * @param tradeNo      PayUni 內部序號
 * @param refundStatus 退款狀態（1=成功 / 2=失敗）
 * @param refundAmt    退款金額
 * @param bankDate     銀行到帳日 YYYY-MM-DD（失敗時可能為 "-" 或空字串）
 * @param message      回應訊息（失敗時通常含退件原因）
 */
public record RefundNotifyEvent(
        String merId,
        String merTradeNo,
        String tradeNo,
        String refundStatus,
        String refundAmt,
        String bankDate,
        String message
) implements PayUniWebhookEvent {
}
