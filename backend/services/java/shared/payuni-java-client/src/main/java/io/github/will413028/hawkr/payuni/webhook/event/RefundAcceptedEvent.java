package io.github.will413028.hawkr.payuni.webhook.event;

/**
 * 退款受理通知事件（ATM async webhook）
 *
 * PayUni ATM 退款頁面版「受理」通知：買家完成頁面填寫後 PayUni 立即發；
 * 等同 sync return URL payload 的 async 副本（§6.6）。隨後才會在實際匯款完成時
 * 再發一筆 {@link RefundNotifyEvent}（§6.7 匯款結果，含 RefundStatus / BankDate）。
 *
 * payload 特徵：
 * - 含 {@code EstimatedRefundDate}（PayUni 預計扣款日期）
 * - **不含** {@code RefundStatus}（僅「結果」通知才有）
 * - {@code Status} 通常為 SUCCESS
 *
 * @param merId               商家編號
 * @param merTradeNo          商店退款編號
 * @param tradeNo             PayUni UNi 序號
 * @param status              PayUni 狀態（通常 SUCCESS）
 * @param message             PayUni 訊息
 * @param refundAmt           退款金額
 * @param paymentType         支付工具（2=ATM / 3=超商代碼）
 * @param estimatedRefundDate 預計扣款日期 YYYY-MM-DD
 */
public record RefundAcceptedEvent(
        String merId,
        String merTradeNo,
        String tradeNo,
        String status,
        String message,
        String refundAmt,
        String paymentType,
        String estimatedRefundDate
) implements PayUniWebhookEvent {
}
