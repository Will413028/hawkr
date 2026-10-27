package io.github.will413028.hawkr.payuni.shipping.request;

/**
 * 物流單查詢請求 v1.1
 *
 * @param shipTradeNo 物流交易編號（B2C/C2C/HOME 時必填）
 * @param merTradeNo  商店訂單編號（選填）
 * @param lgsType     物流型態（B2C/C2C/HOME/C2B，必填）
 */
public record QueryShipmentRequest(
        String shipTradeNo,
        String merTradeNo,
        String lgsType
) {
}
