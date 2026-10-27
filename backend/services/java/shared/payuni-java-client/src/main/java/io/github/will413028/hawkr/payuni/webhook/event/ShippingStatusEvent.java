package io.github.will413028.hawkr.payuni.webhook.event;

/**
 * 物流貨態通知事件
 *
 * @param merId           商家編號
 * @param shipTradeNo     物流交易編號
 * @param shipStatus      物流狀態碼
 * @param shipStatusTime  狀態變更時間
 * @param shipType        物流類型
 * @param lgsType         通路類型（B2C, C2C, HOME, C2B）
 * @param message         訊息
 */
public record ShippingStatusEvent(
        String merId,
        String shipTradeNo,
        String shipStatus,
        String shipStatusTime,
        String shipType,
        String lgsType,
        String message
) implements PayUniWebhookEvent {
}
