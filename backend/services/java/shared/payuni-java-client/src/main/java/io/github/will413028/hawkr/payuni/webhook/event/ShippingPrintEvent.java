package io.github.will413028.hawkr.payuni.webhook.event;

/**
 * 超商列印結果通知事件（ApiType=Print）
 * <p>
 * PayUni 超商出貨單列印完成後回傳，含交貨便代碼。
 *
 * @param merId        商家編號
 * @param shipTradeNo  物流交易編號
 * @param partnerId    出貨便母代碼（3 碼）
 * @param odno         出貨單編號（8 碼）
 * @param validationNo C2C 驗證碼（4 碼）
 * @param goodsType    寄件型態
 * @param lgsType      物流類型
 * @param shipType     通路類別
 */
public record ShippingPrintEvent(
        String merId,
        String shipTradeNo,
        String partnerId,
        String odno,
        String validationNo,
        String goodsType,
        String lgsType,
        String shipType
) implements PayUniWebhookEvent {
}
